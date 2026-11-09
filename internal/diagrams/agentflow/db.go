// Ported from mermaid 12.2.0, diagrams/agentflow/agentflowDb.ts (MIT).

package agentflow

import (
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

const mermaidDomIDPrefix = "agentflow-"

// semanticMetadataSkipKeys is SEMANTIC_METADATA_SKIP_KEYS: the metadata keys
// that are presentation only. The semantic model does not have them.
var semanticMetadataSkipKeys = map[string]bool{
	"shape":     true,
	"view":      true,
	"icon":      true,
	"img":       true,
	"form":      true,
	"pos":       true,
	"w":         true,
	"h":         true,
	"class":     true,
	"style":     true,
	"labelType": true,
}

// subroutineAliases is SUBROUTINE_ALIASES: the shape ids that mark a node
// as a tool definition.
var subroutineAliases = map[string]bool{
	"subroutine":       true,
	"subprocess":       true,
	"subproc":          true,
	"framed-rectangle": true,
	"tool":             true,
}

// subgraphTypeToShape is SUBGRAPH_TYPE_TO_SHAPE: the cluster shape of each
// container type.
var subgraphTypeToShape = map[string]string{
	"flow": "flowGroup",
}

// DB is AgentFlowDB: the data of a parsed agentflow diagram.
type DB struct {
	render.CommonDB
	ctx *render.Context

	vertexCounter int
	// config is the config that the db keeps: getConfig() when the db is
	// created, and again for each node with a text.
	config   *js.Obj[any]
	vertices *js.Map[string, *FlowVertex]
	edges    []*FlowEdge
	// defaultInterpolate and defaultStyle are edges.defaultInterpolate and
	// edges.defaultStyle. hasDefaultInterpolate is false, and defaultStyle
	// is nil, when the property is undefined.
	defaultInterpolate    string
	hasDefaultInterpolate bool
	defaultStyle          []string
	classes               *js.Map[string, *render.ClassDef]
	subGraphs             []*FlowSubGraph
	subGraphLookup        *js.Map[string, *FlowSubGraph]
	globalNodes           *js.Set[string]
	connectors            *js.Map[string, *FlowVertex]
	tooltips              *js.Map[string, string]
	subCount              int
	firstGraphFlag        bool
	direction             string
	hasDirection          bool
	// version is "gen-2": as in graph.
	version     string
	secCount    int
	posCrossRef map[int]int
	diagramID   string

	// The element mappings. Agentflow reports source positions, so it
	// parses the text with its comments, and it gets the number of lines of
	// the front matter. The positions of the grammar then are positions in
	// the source of the author.
	frontmatterLineOffset int
	elementMappings       []*ElementMapping
	// bareVertexMappings has the vertex mappings of a bare `id` reference
	// (no label or shape brackets). A `@{ ... }` block after it changes
	// the type of such a mapping to "attachment".
	bareVertexMappings map[*ElementMapping]bool

	// diagnostics are the warnings and the errors that the db found during
	// the parse and in getData.
	diagnostics []*Diagnostic

	// postParseValidationRun is true after the post-parse validators ran.
	postParseValidationRun bool

	// callbacks stands for the functions that upstream runs after the
	// render to add the click listeners (funs).
	callbacks []ClickCallback
}

// NewDB returns a db in the state after the constructor of AgentFlowDB:
// clear() and setGen('gen-2').
func NewDB(ctx *render.Context) *DB {
	d := &DB{ctx: ctx, secCount: -1, posCrossRef: map[int]int{}}
	d.CommonDB.Sanitize = func(text string) string {
		return ctx.Sanitizer.SanitizeText(text, d.getConfig())
	}
	d.Clear("gen-2")
	d.SetGen("gen-2")
	return d
}

// PreserveCommentsWhenParsing is the property preserveCommentsWhenParsing
// of the db: Diagram.fromText then gives the text with its `%%` comments to
// the parser, so that the positions of the grammar are positions in the
// source.
func (d *DB) PreserveCommentsWhenParsing() bool { return true }

// getConfig is getConfig() of diagramAPI. The pipeline keeps the current
// config in the context.
func (d *DB) getConfig() *js.Obj[any] {
	return d.ctx.Config
}

// sanitizeText is sanitizeText(txt): common.sanitizeText(txt, this.config).
func (d *DB) sanitizeText(txt string) (string, error) {
	return d.ctx.Sanitizer.SanitizeTextErr(txt, d.config)
}

// toStringErr is the string conversion of a value of the YAML metadata
// (String(v), v.toString()). An element of an array that is an object with
// an own property toString has no conversion: a value of the metadata is
// never a function, and the valueOf of an object gives no primitive.
func toStringErr(v any) (string, error) {
	switch x := v.(type) {
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			if js.IsNullish(e) {
				continue
			}
			s, err := toStringErr(e)
			if err != nil {
				return "", err
			}
			parts[i] = s
		}
		return strings.Join(parts, ","), nil
	case *js.Obj[any]:
		if x != nil && x.Has("toString") {
			return "", typeError("Cannot convert object to primitive value")
		}
	}
	return js.ToString(v), nil
}

// sanitizeValue is sanitizeText(v) for a value of the YAML metadata that is
// truthy. The value can be a number, a boolean, an array or an object:
// DOMPurify converts it to a string.
func (d *DB) sanitizeValue(v any) (string, error) {
	if s, ok := v.(string); ok {
		return d.sanitizeText(s)
	}
	removesScript := false
	if sanitize.GetEffectiveHtmlLabels(d.config) {
		level := d.config.Get("securityLevel")
		switch {
		case level == "antiscript" || level == "strict" || level == "sandbox":
			removesScript = true
		case level != "loose":
			// breakToPlaceholder(text): s.replace(lineBreakRegex, '#br#')
			return "", typeError("s.replace is not a function")
		}
	}
	// DOMPurify.sanitize(dirty): stringifyValue(dirty) for a value that is
	// not a string. For an object it calls the first toString function of
	// the prototype chain, so an own property toString that is no function
	// has no effect: the text is "[object Object]". The toString of an
	// array joins its elements.
	var text string
	if _, isObject := v.(*js.Obj[any]); isObject {
		text = "[object Object]"
	} else {
		var err error
		text, err = toStringErr(v)
		if err != nil {
			return "", err
		}
	}
	if text == "" {
		if removesScript {
			// removeScript sets up the hooks of DOMPurify also for a text
			// that is empty after the conversion.
			d.ctx.Sanitizer.RemoveScript("")
		}
		return "", nil
	}
	return d.sanitizeText(text)
}

// sanitizeNodeLabelType is sanitizeNodeLabelType(labelType).
func sanitizeNodeLabelType(labelType any) string {
	switch labelType {
	case "markdown", "string", "text":
		return labelType.(string)
	}
	return "markdown"
}

// stripQuotes is the step "strip quotes if string starts and ends with a
// quote": txt.substring(1, txt.length - 1).
func stripQuotes(txt string) string {
	if strings.HasPrefix(txt, `"`) && strings.HasSuffix(txt, `"`) {
		if len(txt) < 2 {
			// substring(1, 0) is substring(0, 1).
			return txt
		}
		return txt[1 : len(txt)-1]
	}
	return txt
}

// SetDiagramId is setDiagramId(svgElementId): the id of the svg element is
// the prefix of the DOM ids, so that they are unique across the diagrams of
// a page.
func (d *DB) SetDiagramId(svgElementID string) {
	d.diagramID = svgElementID
}

// LookUpDomId is lookUpDomId(id): the DOM id for an id of the graph
// definition, with the prefix of the diagram when it is set.
func (d *DB) LookUpDomId(id string) string {
	for _, vertex := range d.vertices.Values() {
		if vertex.ID == id {
			if d.diagramID != "" {
				return d.diagramID + "-" + vertex.DomID
			}
			return vertex.DomID
		}
	}
	if d.diagramID != "" {
		return d.diagramID + "-" + id
	}
	return id
}

// prop is doc?.key and doc.key for a value that is not nullish.
func prop(doc any, key string) any {
	v, _ := js.GetProperty(doc, key)
	return v
}

// spread is { ...a, ...b }: the own enumerable properties of both values.
func spread(values ...any) *js.Obj[any] {
	out := js.NewObj[any]()
	for _, v := range values {
		if o, ok := v.(*js.Obj[any]); ok && o == nil {
			continue
		}
		keys, ok := js.OwnKeys(v)
		if !ok {
			continue
		}
		for _, k := range keys {
			out.Set(k, prop(v, k))
		}
	}
	return out
}

// metadataOrNil is the metadata of an element as a spread operand: nil when
// the element has none.
func metadataOrNil(m *js.Obj[any]) any {
	if m == nil {
		return nil
	}
	return m
}

// isValidShape is isValidShape(shape): `shape in shapes`. The operator also
// finds the properties of Object.prototype. "constructor" is the one that
// passes the lowercase check before.
func isValidShape(shape string) bool {
	return ru.IsValidShape(shape) || shape == "constructor"
}

// VertexArgs are the optional arguments of addVertex. The zero value is
// "all undefined".
type VertexArgs struct {
	// TextObj is the text, or nil.
	TextObj *FlowText
	// Type is the shape of the grammar, or "".
	Type string
	// Style and Classes are nil when undefined.
	Style   []string
	Classes []string
	// Dir is the direction, or "".
	Dir string
	// Props is nil when undefined. The default is an empty object.
	Props *js.Obj[any]
	// Metadata is the shape data, or nil.
	Metadata *string
	// MetadataLoc is the location of the shape data in the text, or nil.
	MetadataLoc *Location
}

// Location is the location object of the grammar (JisonLocation): lines
// start at 1 and columns at 0.
type Location struct {
	FirstLine   int
	FirstColumn int
	LastLine    int
	LastColumn  int
}

// AddVertex is addVertex(id, textObj, type, style, classes, dir, props,
// metadata, metadataLoc): the parser calls it when it finds a node
// definition.
func (d *DB) AddVertex(id string, a VertexArgs) error {
	if id == "" || len(js.TrimSpace(id)) == 0 {
		return nil
	}
	// Extract the metadata from the shapeData, the syntax for adding metadata for nodes and edges is the same
	// so at this point we don't know if it's a node or an edge, but we can still extract the metadata
	var doc any
	if a.Metadata != nil {
		v, err := d.loadMetadata(*a.Metadata, a.MetadataLoc)
		if err != nil {
			return err
		}
		doc = stripPrototypeKeys(v)
	}

	// Resolve shape alias (section 4.3.2) before any other shape handling. Keep the
	// authored name around: the lowercase style rule in the vertex branch must
	// judge what the user typed, not an internal id like `roundedRect` that
	// alias resolution produces (`task` to `roundedRect` used to trip it).
	authoredShape, hasAuthoredShape := "", false
	if js.Truthy(doc) {
		if shape, ok := prop(doc, "shape").(string); ok {
			authoredShape, hasAuthoredShape = shape, true
			resolved := resolveShapeAlias(authoredShape)
			if resolved != "" && resolved != authoredShape {
				// A doc with a string property shape is an object.
				js.AsObj(doc).Set("shape", resolved)
			}
		}
	}

	// Check if this is a subgraph (e.g. my_flow@{ view: "collapsed" })
	if subGraph, ok := d.subGraphLookup.Lookup(id); ok && js.Truthy(doc) {
		subGraph.Metadata = spread(metadataOrNil(subGraph.Metadata), doc)
		return nil
	}

	// Check if this is a connector declared via the `connector` keyword:
	// attach metadata to the connector record so applicability rules
	// (section 10) apply on the connector kind.
	if connector, ok := d.connectors.Lookup(id); ok && js.Truthy(doc) {
		connector.Metadata = spread(metadataOrNil(connector.Metadata), doc)
		return nil
	}

	// Reserve 'connectors' as a declaration group ID for forward compat
	// (the synthesised connectors group was removed in v0.8.1, but the
	// id stays reserved). The subgraph branch above runs first, so a
	// user-declared `flow connectors[...]` still owns the id.
	if id == "connectors" {
		return nil
	}

	// Check if this is an edge: route metadata to the edge instead of
	// creating a placeholder vertex. Presentation keys map to dedicated edge
	// fields; all authored metadata is carried faithfully onto `edge.metadata`
	// (which keys are valid on an edge is a semantic concern handled
	// downstream, issue #64).
	for _, edge := range d.edges {
		if edge.ID != id {
			continue
		}
		if v := prop(doc, "animate"); v != nil {
			edge.Animate = v
		}
		if v := prop(doc, "animation"); v != nil {
			edge.Animation = v
		}
		if v := prop(doc, "curve"); v != nil {
			edge.Interpolate = v
		}
		if js.Truthy(doc) {
			edge.Metadata = spread(metadataOrNil(edge.Metadata), doc)
		}
		return nil
	}

	vertex, ok := d.vertices.Lookup(id)
	if !ok {
		vertex = &FlowVertex{
			ID:        id,
			LabelType: "text",
			DomID:     mermaidDomIDPrefix + id + "-" + strconv.Itoa(d.vertexCounter),
			Styles:    []string{},
			Classes:   []string{},
		}
		d.vertices.Set(id, vertex)
	}
	d.vertexCounter++

	if a.TextObj != nil {
		d.config = d.getConfig()
		txt, err := d.sanitizeText(js.TrimSpace(a.TextObj.Text))
		if err != nil {
			return err
		}
		vertex.LabelType = a.TextObj.Type
		// strip quotes if string starts and ends with a quote
		vertex.Text, vertex.HasText = stripQuotes(txt), true
	} else if !vertex.HasText {
		vertex.Text, vertex.HasText = id, true
	}
	if a.Type != "" {
		vertex.Type, vertex.HasType = a.Type, true
	}
	if a.Style != nil {
		vertex.Styles = append(vertex.Styles, a.Style...)
	}
	if a.Classes != nil {
		vertex.Classes = append(vertex.Classes, a.Classes...)
	}
	if a.Dir != "" {
		vertex.Dir, vertex.HasDir = a.Dir, true
	}
	props := a.Props
	if props == nil {
		props = js.NewObj[any]()
	}
	if vertex.Props == nil {
		vertex.Props = props
	} else {
		// Object.assign(vertex.props, props). An assignment to __proto__
		// adds no own property: it only changes a property that the object
		// already has, from Object.fromEntries.
		props.Each(func(k string, v any) {
			if k != "__proto__" || vertex.Props.Has(k) {
				vertex.Props.Set(k, v)
			}
		})
	}

	if js.Truthy(doc) {
		vertex.Metadata = spread(metadataOrNil(vertex.Metadata), doc)
		if shape := prop(doc, "shape"); js.Truthy(shape) {
			// An unknown shape stays a hard error, matching flowchart: accepting
			// something flowchart rejects is the divergence this grammar exists to
			// avoid. What it gains here is a source position, so an editor can
			// point at the `@{ ... }` block instead of just blanking the diagram.
			//
			// A non-string value (e.g. `shape: 123`) is an ordinary unknown shape,
			// not an uncaught TypeError on `.toLowerCase` (issue #83).
			if !hasAuthoredShape {
				// authoredShape ?? doc.shape is a value that is not a string.
				return d.positionedShapeError("No such shape: "+js.StringifyJSON(shape)+".", a.MetadataLoc)
			}
			authored := authoredShape
			// doc.shape is the resolved shape: a string.
			resolved, _ := shape.(string)
			if authored != js.ToLowerCase(authored) || strings.Contains(authored, "_") {
				return d.positionedShapeError("No such shape: "+authored+". Shape names should be lowercase.", a.MetadataLoc)
			} else if !isValidShape(resolved) {
				return d.positionedShapeError("No such shape: "+resolved+".", a.MetadataLoc)
			}
			vertex.Type, vertex.HasType = resolved, true
		}

		if label := prop(doc, "label"); js.Truthy(label) {
			// Sanitize like the bracket-label and edge-label paths do. Not
			// currently exploitable (`createText` sanitizes before writing and the
			// finished SVG is DOMPurify'd), but the metadata path should not be the
			// one place a label reaches the DB unsanitized.
			text, err := d.sanitizeValue(label)
			if err != nil {
				return err
			}
			vertex.Text, vertex.HasText = text, true
			vertex.LabelType = sanitizeNodeLabelType(prop(doc, "labelType"))
		}
		// `icon`, `img`, `form`, `pos`, `w` and `h` are deliberately not promoted
		// onto the vertex. Agentflow's shape catalogue (section 4.3) is the six domain
		// shapes; the icon and image shapes these keys select are outside it, so
		// the shape they produced was replaced with the default and reported as
		// SHAPE_UNSUPPORTED before it ever reached the renderer. They stay
		// available as ordinary metadata on `vertex.metadata`.
	}
	return nil
}

// positionedShapeError is positionedShapeError(message, metadataLoc): a
// shape error with a hash in the form of jison, so that the position is
// readable as data.
func (d *DB) positionedShapeError(message string, metadataLoc *Location) *Error {
	e := newError(message)
	if metadataLoc == nil {
		return e
	}
	line := metadataLoc.FirstLine + d.frontmatterLineOffset
	e.Hash = &ErrorHash{
		Line: line - 1,
		Loc: ErrorLoc{
			FirstLine:   line,
			LastLine:    metadataLoc.LastLine + d.frontmatterLineOffset,
			FirstColumn: metadataLoc.FirstColumn,
			LastColumn:  metadataLoc.LastColumn,
		},
	}
	return e
}

// AddSingleLink is addSingleLink(_start, _end, type, id): the parser calls
// it when it finds a link. id is "" when it is undefined.
func (d *DB) AddSingleLink(start, end string, link *FlowLink, id string) error {
	edge := &FlowEdge{
		Start:           start,
		End:             end,
		Text:            "",
		LabelType:       "text",
		Classes:         []string{},
		IsUserDefinedID: false,
		Length:          undef,
	}
	if d.hasDefaultInterpolate {
		edge.Interpolate = d.defaultInterpolate
	}
	if link == nil {
		return typeError("Cannot read properties of undefined (reading 'text')")
	}

	if linkTextObj := link.Text; linkTextObj != nil {
		text, err := d.sanitizeText(js.TrimSpace(linkTextObj.Text))
		if err != nil {
			return err
		}
		// strip quotes if string starts and ends with a quote
		edge.Text = stripQuotes(text)
		edge.LabelType = sanitizeNodeLabelType(linkTextObj.Type)
	}

	edge.Type = link.Type
	edge.Stroke = link.Stroke
	if link.Length > 10 {
		edge.Length = 10
	} else {
		edge.Length = link.Length
	}
	if link.EdgeSemantic != "" {
		edge.EdgeSemantic = link.EdgeSemantic
	}

	userID := id != ""
	if userID {
		for _, e := range d.edges {
			if e.ID == id {
				userID = false
				break
			}
		}
	}
	if userID {
		edge.ID = id
		edge.IsUserDefinedID = true
	} else {
		existingLinks := 0
		for _, e := range d.edges {
			if e.Start == edge.Start && e.End == edge.End {
				existingLinks++
			}
		}
		if existingLinks == 0 {
			edge.ID = utils.GetEdgeId(edge.Start, edge.End, utils.EdgeIdOptions{Counter: 0, Prefix: "L"}, "")
		} else {
			edge.ID = utils.GetEdgeId(edge.Start, edge.End, utils.EdgeIdOptions{Counter: existingLinks + 1, Prefix: "L"}, "")
		}
	}

	var maxEdges any = 500.0
	if v := d.config.Get("maxEdges"); !js.IsNullish(v) {
		maxEdges = v
	}
	if js.Compare(float64(len(d.edges)), "<", maxEdges) {
		d.edges = append(d.edges, edge)
	} else {
		return newError("Edge limit exceeded. " + strconv.Itoa(len(d.edges)) + " edges found, but the limit is " + js.ToString(d.config.Get("maxEdges")) + `.

Initialize mermaid with maxEdges set to a higher number to allow more edges.
You cannot set this config via configuration inside the diagram as it is a secure config.
You have to call mermaid.initialize.`)
	}
	return nil
}

// AddLink is addLink(_start, _end, linkData).
func (d *DB) AddLink(start, end []string, linkData *FlowLink) error {
	id := ""
	if linkData != nil && linkData.HasID {
		id = strings.Replace(linkData.ID, "@", "", 1)
	}

	// for a group syntax like A e1@--> B & C, only the first edge should have a userDefined id
	// the rest of the edges should have auto generated ids
	for _, s := range start {
		for _, e := range end {
			// use the id only for last node in _start and first node in _end
			isLastStart := s == start[len(start)-1]
			isFirstEnd := e == end[0]
			var err error
			if isLastStart && isFirstEnd {
				err = d.AddSingleLink(s, e, linkData, id)
			} else {
				err = d.AddSingleLink(s, e, linkData, "")
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// edgeAt is this.edges[pos] for a position: the text of a number (the
// grammar gives the positions as strings) or a number. Upstream reads a
// property of the array, so only a canonical index finds an edge.
func (d *DB) edgeAt(pos any) *FlowEdge {
	var key string
	switch p := pos.(type) {
	case string:
		key = p
	case float64:
		key = js.Str(p)
	default:
		return nil
	}
	i, ok := js.IsArrayIndex(key)
	if !ok || int(i) >= len(d.edges) {
		return nil
	}
	return d.edges[i]
}

// outOfBounds is the error of updateLink and updateLinkInterpolate for a
// position that is a number and that is not below the number of edges.
func (d *DB) outOfBounds(pos any) error {
	n, ok := pos.(float64)
	if !ok || !(n >= float64(len(d.edges))) {
		return nil
	}
	return newError("The index " + js.Str(n) + " for linkStyle is out of bounds. Valid indices for linkStyle are between 0 and " +
		strconv.Itoa(len(d.edges)-1) + ". (Help: Ensure that the index is within the range of existing edges.)")
}

// UpdateLinkInterpolate is updateLinkInterpolate(positions, interpolate):
// it updates the line interpolation algorithm of links. A position is
// "default", the text of a number, or a number (a float64).
//
// Upstream checks the bounds only for a position that is a number, and the
// grammar gives strings. So a position of the grammar without an edge is a
// TypeError.
func (d *DB) UpdateLinkInterpolate(positions []any, interpolate string) error {
	for _, pos := range positions {
		if pos == "default" {
			d.defaultInterpolate, d.hasDefaultInterpolate = interpolate, true
			continue
		}
		// Same bounds message as `updateLink`. Without it an out-of-range index
		// surfaced as `TypeError: Cannot set properties of undefined`.
		if err := d.outOfBounds(pos); err != nil {
			return err
		}
		edge := d.edgeAt(pos)
		if edge == nil {
			return typeError("Cannot set properties of undefined (setting 'interpolate')")
		}
		edge.Interpolate = interpolate
	}
	return nil
}

// UpdateLink is updateLink(positions, style): it updates links with a
// style. A position is "default", the text of a number, or a number (a
// float64).
func (d *DB) UpdateLink(positions []any, style []string) error {
	for _, pos := range positions {
		if err := d.outOfBounds(pos); err != nil {
			return err
		}
		if pos == "default" {
			d.defaultStyle = style
		} else {
			edge := d.edgeAt(pos)
			if edge == nil {
				return typeError("Cannot set properties of undefined (setting 'style')")
			}
			edge.Style = style
			// if edges[pos].style does have fill not set, set it to none
			if len(edge.Style) > 0 {
				hasFill := false
				for _, s := range edge.Style {
					if strings.HasPrefix(s, "fill") {
						hasFill = true
						break
					}
				}
				if !hasFill {
					// A new array: upstream gives the same array to every
					// position, and its first push sets the fill for all.
					edge.Style = append(append(make([]string, 0, len(style)+1), style...), "fill:none")
				}
			}
		}
	}
	return nil
}

// AddClass is addClass(ids, _style).
func (d *DB) AddClass(ids string, _style []string) {
	joined := strings.Join(_style, ",")
	joined = strings.ReplaceAll(joined, `\,`, "§§§")
	joined = strings.ReplaceAll(joined, ",", ";")
	joined = strings.ReplaceAll(joined, "§§§", ",")
	style := strings.Split(joined, ";")
	for _, id := range strings.Split(ids, ",") {
		classNode, ok := d.classes.Lookup(id)
		if !ok {
			classNode = &render.ClassDef{ID: id, Styles: []string{}, TextStyles: []string{}}
			d.classes.Set(id, classNode)
		}

		for _, s := range style {
			if strings.Contains(s, "color") {
				newStyle := strings.Replace(s, "fill", "bgFill", 1) // .replace('color', 'fill');
				classNode.TextStyles = append(classNode.TextStyles, newStyle)
			}
			classNode.Styles = append(classNode.Styles, s)
		}
	}
}

// SetDirection is setDirection(dir): the parser calls it when it finds a
// graph definition. It stores the direction of the chart.
func (d *DB) SetDirection(dir string) {
	d.direction, d.hasDirection = js.TrimSpace(dir), true

	// /.*</ and the other patterns match when the character is in the text.
	if strings.Contains(d.direction, "<") {
		d.direction = "RL"
	}
	if strings.Contains(d.direction, "^") {
		d.direction = "BT"
	}
	if strings.Contains(d.direction, ">") {
		d.direction = "LR"
	}
	if strings.Contains(d.direction, "v") {
		d.direction = "TB"
	}
	if d.direction == "TD" {
		d.direction = "TB"
	}
}

// SetClass is setClass(ids, className): the parser calls it for a class
// statement. ids is a comma separated list of ids.
func (d *DB) SetClass(ids, className string) {
	for _, id := range strings.Split(ids, ",") {
		if vertex, ok := d.vertices.Lookup(id); ok {
			vertex.Classes = append(vertex.Classes, className)
		}
		for _, edge := range d.edges {
			if edge.ID == id {
				edge.Classes = append(edge.Classes, className)
				break
			}
		}
		if subGraph, ok := d.subGraphLookup.Lookup(id); ok {
			subGraph.Classes = append(subGraph.Classes, className)
		}
	}
}

// SetTooltip is setTooltip(ids, tooltip). The caller does not call it for
// a tooltip that is undefined.
func (d *DB) SetTooltip(ids, tooltip string) error {
	tooltip, err := d.sanitizeText(tooltip)
	if err != nil {
		return err
	}
	for _, id := range strings.Split(ids, ",") {
		key := id
		if d.version == "gen-1" {
			key = d.LookUpDomId(id)
		}
		d.tooltips.Set(key, tooltip)
	}
	return nil
}

// splitArgs is functionArgs.split(/,(?=(?:(?:[^"]*"){2})*[^"]*$)/): it
// splits at each "," that an even number of double quotes follows.
func splitArgs(functionArgs string) []string {
	quotesAfter := make([]int, len(functionArgs)+1)
	for i := len(functionArgs) - 1; i >= 0; i-- {
		quotesAfter[i] = quotesAfter[i+1]
		if functionArgs[i] == '"' {
			quotesAfter[i]++
		}
	}
	var out []string
	start := 0
	for i := 0; i < len(functionArgs); i++ {
		if functionArgs[i] == ',' && quotesAfter[i+1]%2 == 0 {
			out = append(out, functionArgs[start:i])
			start = i + 1
		}
	}
	return append(out, functionArgs[start:])
}

// setClickFun is setClickFun(id, functionName, functionArgs). functionArgs
// is nil when it is undefined.
func (d *DB) setClickFun(id, functionName string, functionArgs *string) {
	if !js.StrictEquals(d.getConfig().Get("securityLevel"), "loose") {
		return
	}
	var argList []string
	if functionArgs != nil {
		/* Splits functionArgs by ',', ignoring all ',' in double quoted strings */
		argList = splitArgs(*functionArgs)
		for i := range argList {
			item := js.TrimSpace(argList[i])
			/* Removes all double quotes at the start and end of an argument */
			/* This preserves all starting and ending whitespace inside */
			if strings.HasPrefix(item, `"`) && strings.HasSuffix(item, `"`) {
				item = js.Substr(item, 1, js.UTF16Len(item)-2)
			}
			argList[i] = item
		}
	}

	/* if no arguments passed into callback, default to passing in id */
	if len(argList) == 0 {
		argList = append(argList, id)
	}

	if vertex, ok := d.vertices.Lookup(id); ok {
		vertex.HaveCallback = true
		d.callbacks = append(d.callbacks, ClickCallback{ID: id, FunctionName: functionName, Args: argList})
	}
}

// SetLink is setLink(ids, linkStr, target): the parser calls it when it
// finds a link. It adds the URL to the vertex data. target is nil when it
// is undefined.
func (d *DB) SetLink(ids, linkStr string, target *string) {
	for _, id := range strings.Split(ids, ",") {
		if vertex, ok := d.vertices.Lookup(id); ok {
			vertex.Link, vertex.HasLink = utils.FormatUrl(linkStr, d.config)
			vertex.LinkTarget, vertex.HasLinkTarget = "", false
			if target != nil {
				vertex.LinkTarget, vertex.HasLinkTarget = *target, true
			}
		}
	}
	d.SetClass(ids, "clickable")
}

// GetTooltip is getTooltip(id). The second result is false when the id has
// no tooltip.
func (d *DB) GetTooltip(id string) (string, bool) {
	return d.tooltips.Lookup(id)
}

// SetClickEvent is setClickEvent(ids, functionName, functionArgs): the
// parser calls it when it finds a click definition. functionArgs is nil
// when it is undefined.
func (d *DB) SetClickEvent(ids, functionName string, functionArgs *string) {
	for _, id := range strings.Split(ids, ",") {
		d.setClickFun(id, functionName, functionArgs)
	}
	d.SetClass(ids, "clickable")
}

// Callbacks returns the click callbacks that bindFunctions would install.
// A static SVG has no event listeners, so this port only records them.
func (d *DB) Callbacks() []ClickCallback {
	return d.callbacks
}

// GetDirection is getDirection(). The second result is false when the
// direction is undefined.
func (d *DB) GetDirection() (string, bool) {
	if !d.hasDirection {
		return "", false
	}
	return js.TrimSpace(d.direction), true
}

// GetVertices is getVertices(): the nodes that the parser found.
func (d *DB) GetVertices() *js.Map[string, *FlowVertex] {
	return d.vertices
}

// GetEdges is getEdges(): the links that the parser found.
func (d *DB) GetEdges() []*FlowEdge {
	return d.edges
}

// DefaultLinkStyle returns edges.defaultStyle and edges.defaultInterpolate.
// The style is nil, and hasInterpolate is false, when the property is
// undefined.
func (d *DB) DefaultLinkStyle() (style []string, interpolate string, hasInterpolate bool) {
	return d.defaultStyle, d.defaultInterpolate, d.hasDefaultInterpolate
}

// IsToolDefinition is isToolDefinition(vertex): true when the vertex is a
// tool definition. Its shape is then "subroutine" or one of the aliases.
func (d *DB) IsToolDefinition(vertex *FlowVertex) bool {
	return vertex.HasType && subroutineAliases[vertex.Type]
}

// GetTools is getTools(): every vertex that is a tool definition.
func (d *DB) GetTools() []*FlowVertex {
	tools := []*FlowVertex{}
	for _, vertex := range d.vertices.Values() {
		if d.IsToolDefinition(vertex) {
			tools = append(tools, vertex)
		}
	}
	return tools
}

// GetClasses is getClasses(): the class definitions that the parser found.
func (d *DB) GetClasses() *js.Map[string, *render.ClassDef] {
	return d.classes
}

// Tooltips returns the tooltips by id.
func (d *DB) Tooltips() *js.Map[string, string] {
	return d.tooltips
}

// Clear is clear(ver): it clears the db so that a new graph can be parsed.
func (d *DB) Clear(ver string) {
	d.vertices = js.NewMap[string, *FlowVertex]()
	d.classes = js.NewMap[string, *render.ClassDef]()
	d.edges = nil
	d.defaultInterpolate, d.hasDefaultInterpolate = "", false
	d.defaultStyle = nil
	d.callbacks = nil
	d.subGraphs = nil
	d.subGraphLookup = js.NewMap[string, *FlowSubGraph]()
	d.globalNodes = js.NewSet[string]()
	d.connectors = js.NewMap[string, *FlowVertex]()
	d.subCount = 0
	d.tooltips = js.NewMap[string, string]()
	d.firstGraphFlag = true
	d.version = ver
	d.config = d.getConfig()
	d.diagramID = ""
	d.vertexCounter = 0
	d.direction, d.hasDirection = "", false
	d.secCount = -1
	d.posCrossRef = map[int]int{}
	d.frontmatterLineOffset = 0
	d.elementMappings = nil
	d.bareVertexMappings = map[*ElementMapping]bool{}
	d.diagnostics = nil
	d.postParseValidationRun = false
	d.ClearCommon()
}

// SetGen is setGen(ver).
func (d *DB) SetGen(ver string) {
	if ver == "" {
		ver = "gen-2"
	}
	d.version = ver
}

// DefaultStyle is defaultStyle(): the colours of `linkStyle default` of the
// legacy renderer, the same as in the flowchart db.
func (d *DB) DefaultStyle() string {
	return "fill:#ffa;stroke: #f66; stroke-width: 3px; stroke-dasharray: 5, 5;fill:#ffa;stroke: #666;"
}

// uniq is the uniq function of addSubGraph: it removes the duplicates and
// the white space from the items of a container, and takes the direction
// statement out. An item is a string or a *dirStmt.
//
// The store of upstream for the ids that it has seen is a plain object, and
// two ids collide with its prototype. An assignment to __proto__ adds no
// property, so that id is never a duplicate. The id hasOwnProperty hides the
// method that the next lookup calls, so the next id is a TypeError. The
// message has the name of the variable in the build of mermaid: the bundler
// renames `type` to `type2` in this function.
func uniq(a []any) (nodeList []string, dir string, hasDir bool, err error) {
	seen := map[string]bool{}
	nodeList = []string{}
	for _, item := range a {
		switch x := item.(type) {
		case *dirStmt:
			dir, hasDir = x.value, true
		case string:
			if js.TrimSpace(x) == "" {
				continue
			}
			if seen["hasOwnProperty"] {
				return nil, "", false, typeError("prims[type2].hasOwnProperty is not a function")
			}
			if x != "__proto__" {
				if seen[x] {
					continue
				}
				seen[x] = true
			}
			nodeList = append(nodeList, x)
		}
	}
	return nodeList, dir, hasDir, nil
}

// flat is list.flat(): it flattens the items of a document by one level.
func flat(list []any) []any {
	out := make([]any, 0, len(list))
	for _, item := range list {
		switch x := item.(type) {
		case []string:
			for _, s := range x {
				out = append(out, s)
			}
		case []any:
			out = append(out, x...)
		default:
			out = append(out, item)
		}
	}
	return out
}

// AddSubGraph is addSubGraph(_id, list, _title, type). An item of list is a
// string, a list of strings, or a direction statement. _id and _title are
// nil when they are undefined, and typ is "" when it is undefined. The
// result is the id of the container.
func (d *DB) AddSubGraph(_id *FlowText, list []any, _title *FlowText, typ string) (string, error) {
	id, hasID := "", false
	if _id != nil {
		id, hasID = js.TrimSpace(_id.Text), true
	}
	title := ""
	if _title != nil {
		title = _title.Text
	}
	if _id != nil && _id == _title && _title.Text != "" && strings.ContainsFunc(_title.Text, js.IsWhiteSpace) {
		hasID = false
	}

	nodeList, dir, hasDir, err := uniq(flat(list))
	if err != nil {
		return "", err
	}
	// dirValue is the direction when it is not a string: the direction of
	// the config can have any type.
	var dirValue any
	if !hasDir {
		if js.Truthy(js.Path(d.getConfig(), "flowchart", "inheritDir")) {
			if direction, ok := d.GetDirection(); ok {
				dir, hasDir = direction, true
			} else if v := d.getConfig().Get("direction"); !js.IsNullish(v) {
				dir, hasDir = js.ToString(v), true
				if _, isString := v.(string); !isString {
					dirValue = v
				}
			}
		}
	}

	if d.version == "gen-1" {
		for i := range nodeList {
			nodeList[i] = d.LookUpDomId(nodeList[i])
		}
	}

	if !hasID {
		id = "subGraph" + strconv.Itoa(d.subCount)
	}
	// An undefined title is the empty text here: title = title || "".
	title, err = d.sanitizeText(title)
	if err != nil {
		return "", err
	}
	d.subCount = d.subCount + 1

	// A subgraph is never a member of itself. In-block edge endpoints are
	// folded into the member list, so `a --> myFlow` written inside
	// `flow myFlow ... end` would otherwise make the flow its own member and
	// self-parent, crashing the renderer with graphlib's "would create a
	// cycle" (issue #70). Globally scoped nodes (`global ... end`, issue #80)
	// are exempt from membership the same way: referencing one inside a
	// flow must not adopt it.
	nodes := []string{}
	for _, nodeID := range nodeList {
		if nodeID != id && !d.globalNodes.Has(nodeID) {
			nodes = append(nodes, nodeID)
		}
	}
	var titleType any
	if _title != nil {
		titleType = _title.Type
	}
	if typ == "" {
		typ = "flow"
	}
	subGraph := &FlowSubGraph{
		ID:        id,
		Nodes:     nodes,
		Title:     js.TrimSpace(title),
		Classes:   []string{},
		Dir:       dir,
		HasDir:    hasDir,
		DirValue:  dirValue,
		LabelType: sanitizeNodeLabelType(titleType),
		Type:      typ,
	}

	// Remove the members in the new subgraph if they already belong to another subgraph
	subGraph.Nodes = d.MakeUniq(subGraph, d.subGraphs)

	// Duplicate subgraph IDs merge children into the first occurrence
	// (structural). Identifier-namespace diagnostics moved to semantics (#64).
	existingPos := d.getPosForId(id)
	if existingPos != -1 {
		// Duplicate subgraph ID: merge children into the existing entry.
		// First occurrence wins for hierarchy position.
		existing := d.subGraphs[existingPos]
		for _, nodeID := range subGraph.Nodes {
			found := false
			for _, n := range existing.Nodes {
				if n == nodeID {
					found = true
					break
				}
			}
			if !found {
				existing.Nodes = append(existing.Nodes, nodeID)
			}
		}
		// Last occurrence wins for properties
		if subGraph.Title != "" {
			existing.Title = subGraph.Title
		}
		if subGraph.HasDir && js.Truthy(subGraph.dirJS()) {
			existing.Dir, existing.HasDir, existing.DirValue = subGraph.Dir, true, subGraph.DirValue
		}
		if subGraph.Type != "" {
			existing.Type = subGraph.Type
		}
		d.subGraphLookup.Set(id, existing)
	} else {
		d.subGraphs = append(d.subGraphs, subGraph)
		d.subGraphLookup.Set(id, subGraph)
	}
	return id, nil
}

// AddGlobal is addGlobal(list): it registers every node id of a
// `global ... end` block as globally scoped. Such a node has no parent, also
// when a `flow ... end` block names it. The edges of the block are ordinary
// top-level edges, and the block has no container.
func (d *DB) AddGlobal(list []any) {
	for _, item := range flat(list) {
		// Skip non-id statement values (e.g. `direction` markers); node ids
		// arrive as plain strings.
		s, ok := item.(string)
		if !ok {
			continue
		}
		id := js.TrimSpace(s)
		if id != "" {
			d.globalNodes.Add(id)
		}
	}
	// Flows parsed before this block already claimed the ids. Release them
	// so the exemption is independent of declaration order.
	for _, subGraph := range d.subGraphs {
		nodes := []string{}
		for _, nodeID := range subGraph.Nodes {
			if !d.globalNodes.Has(nodeID) {
				nodes = append(nodes, nodeID)
			}
		}
		subGraph.Nodes = nodes
	}
}

// GlobalNodes returns the ids of the globally scoped nodes, in the order in
// which the parser found them.
func (d *DB) GlobalNodes() []string {
	return d.globalNodes.Values()
}

// getPosForId is getPosForId(id).
func (d *DB) getPosForId(id string) int {
	for i, subGraph := range d.subGraphs {
		if subGraph.ID == id {
			return i
		}
	}
	return -1
}

// indexNodes2 is indexNodes2(id, pos).
func (d *DB) indexNodes2(id string, pos int) (result bool, count int) {
	nodes := d.subGraphs[pos].Nodes
	d.secCount = d.secCount + 1
	if d.secCount > 2000 {
		return false, 0
	}
	d.posCrossRef[d.secCount] = pos
	// Check if match
	if d.subGraphs[pos].ID == id {
		return true, 0
	}

	count = 0
	posCount := 1
	for count < len(nodes) {
		childPos := d.getPosForId(nodes[count])
		// Ignore regular nodes (pos will be -1)
		if childPos >= 0 {
			res, resCount := d.indexNodes2(id, childPos)
			if res {
				return true, posCount + resCount
			}
			posCount = posCount + resCount
		}
		count = count + 1
	}

	return false, posCount
}

// GetDepthFirstPos is getDepthFirstPos(pos). The second result is false
// when the position is undefined.
func (d *DB) GetDepthFirstPos(pos int) (int, bool) {
	v, ok := d.posCrossRef[pos]
	return v, ok
}

// IndexNodes is indexNodes().
func (d *DB) IndexNodes() {
	d.secCount = -1
	if len(d.subGraphs) > 0 {
		d.indexNodes2("none", len(d.subGraphs)-1)
	}
}

// AddConnector is addConnector(textObj, titleObj): the parser calls it for
// a `connector <id>["Title"]` declaration. The connector is a vertex with
// isConnector, so that it goes through the renderer with the vertices, and
// the db keeps a separate index of the connectors. The result is the id.
func (d *DB) AddConnector(textObj, titleObj *FlowText) (string, error) {
	if textObj == nil {
		return "", nil
	}
	id := js.TrimSpace(textObj.Text)
	if id == "" {
		return "", nil
	}

	title := ""
	if titleObj != nil {
		title = js.TrimSpace(titleObj.Text)
	}
	resolvedTitle := id
	if len(title) > 0 {
		var err error
		resolvedTitle, err = d.sanitizeText(title)
		if err != nil {
			return "", err
		}
	}

	connector, ok := d.connectors.Lookup(id)
	if !ok {
		connector = &FlowVertex{
			ID:          id,
			LabelType:   "text",
			DomID:       mermaidDomIDPrefix + id + "-" + strconv.Itoa(d.vertexCounter),
			Styles:      []string{},
			Classes:     []string{},
			IsConnector: true,
			Text:        resolvedTitle,
			HasText:     true,
		}
		d.connectors.Set(id, connector)
		d.vertices.Set(id, connector)
		d.vertexCounter++
	} else if len(title) > 0 {
		connector.Text, connector.HasText = resolvedTitle, true
	}
	return id, nil
}

// GetSubGraphs is getSubGraphs(): the live list of the containers.
func (d *DB) GetSubGraphs() []*FlowSubGraph {
	return d.subGraphs
}

// GetConnectors is getConnectors(): the connectors declared with the
// `connector` keyword.
func (d *DB) GetConnectors() []*FlowVertex {
	return d.connectors.Values()
}

// FirstGraph is firstGraph() and lex.firstGraph(): true for the first call
// after the db is cleared. The lexer of the grammar calls it.
func (d *DB) FirstGraph() bool {
	if d.firstGraphFlag {
		d.firstGraphFlag = false
		return true
	}
	return false
}

// computeEdgeSemantic is computeEdgeSemantic(type, stroke): the canonical
// semantic of an edge for the type and the stroke that destructLink gives.
//
//	-->  arrow_point + normal  sequence
//	-.-  arrow_open  + dotted  reference
//	--x  arrow_cross + normal  failure
//
// The result is "" (undefined) for the other combinations.
func computeEdgeSemantic(typ, stroke string) string {
	if typ == "arrow_point" && stroke == "normal" {
		return "sequence"
	}
	if typ == "arrow_cross" && stroke == "normal" {
		return "failure"
	}
	if typ == "arrow_open" && stroke == "dotted" {
		return "reference"
	}
	return ""
}

// destructStartLink is destructStartLink(_str). Only `-->`, `-.-` and `--x`
// are valid in v0.8.1. A start link (the first half of `a -- xyz -->`) has
// only the stroke: the type comes from the end link.
func destructStartLink(_str string) FlowLink {
	str := js.TrimSpace(_str)
	typ := "arrow_open"

	stroke := "normal"
	if strings.Contains(str, ".") {
		stroke = "dotted"
	}

	return FlowLink{Type: typ, Stroke: stroke, Length: undef}
}

// countChar is countChar(char, str).
func countChar(char rune, str []rune) int {
	count := 0
	for _, u := range str {
		if u == char {
			count++
		}
	}
	return count
}

// destructEndLink is destructEndLink(_str).
func destructEndLink(_str string) FlowLink {
	str := js.UTF16Units(js.TrimSpace(_str))
	// str.slice(0, -1)
	line := str
	if len(line) > 0 {
		line = line[:len(line)-1]
	}
	typ := "arrow_open"

	if len(str) > 0 {
		switch str[len(str)-1] {
		case 'x':
			typ = "arrow_cross"
		case '>':
			typ = "arrow_point"
		case '-', '.':
			// `-.-` form: the trailing character is `-` (or `.`), the operator
			// is non-directional. Keep `arrow_open` so the renderer draws no
			// arrowhead and we route to the reference semantic.
			typ = "arrow_open"
			line = str
		}
	}

	stroke := "normal"
	length := len(line) - 1

	dots := countChar('.', line)
	if dots > 0 {
		stroke = "dotted"
		length = dots
	}

	return FlowLink{Type: typ, Stroke: stroke, Length: float64(length)}
}

// DestructLink is destructLink(_str, _startStr). _startStr is "" when the
// link has no start part.
func (d *DB) DestructLink(_str, _startStr string) FlowLink {
	info := destructEndLink(_str)
	if _startStr != "" {
		startInfo := destructStartLink(_startStr)

		if startInfo.Stroke != info.Stroke {
			return FlowLink{Type: "INVALID", Stroke: "INVALID", Length: undef}
		}

		// -- xyz -->  - take arrow type from ending
		startInfo.Type = info.Type
		startInfo.Length = info.Length
		startInfo.EdgeSemantic = computeEdgeSemantic(startInfo.Type, startInfo.Stroke)
		return startInfo
	}

	info.EdgeSemantic = computeEdgeSemantic(info.Type, info.Stroke)
	return info
}

// Exists is exists(allSgs, _id).
// Todo optimizer this by caching existing nodes
func (d *DB) Exists(allSgs []*FlowSubGraph, _id string) bool {
	for _, sg := range allSgs {
		for _, n := range sg.Nodes {
			if n == _id {
				return true
			}
		}
	}
	return false
}

// MakeUniq is makeUniq(sg, allSubgraphs): the nodes of sg that are in none
// of the subgraphs.
func (d *DB) MakeUniq(sg *FlowSubGraph, allSubgraphs []*FlowSubGraph) []string {
	res := []string{}
	if len(sg.Nodes) == 0 {
		return res
	}
	// One set for all lookups. The result is the same as exists() for each node.
	member := map[string]bool{}
	for _, other := range allSubgraphs {
		for _, n := range other.Nodes {
			member[n] = true
		}
	}
	for _, id := range sg.Nodes {
		if !member[id] {
			res = append(res, id)
		}
	}
	return res
}

// DefaultConfig is defaultConfig(): the flowchart section of the default
// config.
func (d *DB) DefaultConfig() any {
	return d.ctx.ConfigState.DefaultSection("flowchart")
}
