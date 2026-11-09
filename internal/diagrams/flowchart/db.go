// Ported from mermaid 12.2.0, diagrams/flowchart/flowDb.ts (MIT).

package flowchart

import (
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

const mermaidDomIDPrefix = "flowchart-"

// DB is FlowDB: the data of a parsed flowchart.
type DB struct {
	render.CommonDB
	ctx *render.Context

	vertexCounter int
	// config is the config that the db keeps: getConfig() when the db is
	// created, and again for each node with a text.
	config    *js.Obj[any]
	diagramID string
	vertices  *js.Map[string, *FlowVertex]
	edges     []*FlowEdge
	// defaultInterpolate and defaultStyle are edges.defaultInterpolate and
	// edges.defaultStyle. hasDefaultInterpolate is false, and defaultStyle
	// is nil, when the property is undefined.
	defaultInterpolate    string
	hasDefaultInterpolate bool
	defaultStyle          []string
	classes               *js.Map[string, *render.ClassDef]
	subGraphs             []*FlowSubGraph
	subGraphLookup        *js.Map[string, *FlowSubGraph]
	tooltips              *js.Map[string, string]
	subCount              int
	firstGraphFlag        bool
	direction             string
	hasDirection          bool
	// version is "gen-2": as in graph.
	version     string
	secCount    int
	posCrossRef map[int]int

	// callbacks stands for the functions that upstream runs after the
	// render to add the click listeners (funs).
	callbacks []ClickCallback
}

// NewDB returns a db in the state after the constructor of FlowDB: clear()
// and setGen('gen-2').
func NewDB(ctx *render.Context) *DB {
	d := &DB{ctx: ctx, secCount: -1, posCrossRef: map[int]int{}}
	d.CommonDB.Sanitize = func(text string) string {
		return ctx.Sanitizer.SanitizeText(text, d.getConfig())
	}
	d.Clear("gen-2")
	d.SetGen("gen-2")
	return d
}

// getConfig is getConfig() of diagramAPI. The pipeline keeps the current
// config in the context.
func (d *DB) getConfig() *js.Obj[any] {
	return d.ctx.Config
}

// sanitizeText is sanitizeText(txt): common.sanitizeText(txt, this.config).
func (d *DB) sanitizeText(txt string) (string, error) {
	return d.ctx.Sanitizer.SanitizeTextErr(txt, d.config)
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

// loadMetadata is the step of addVertex that reads the shape data: the text
// between "@{" and "}" is YAML.
func loadMetadata(metadata string) (any, error) {
	var yamlData string
	// detect if shapeData contains a newline character
	if !strings.Contains(metadata, "\n") {
		yamlData = "{\n" + metadata + "\n}"
	} else {
		yamlData = metadata + "\n"
	}
	return config.LoadYAML(yamlData)
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

// toNumber is Number(v). An array or an object converts to a string first.
func toNumber(v any) float64 {
	switch v.(type) {
	case []any, *js.Obj[any]:
		return js.Number(js.ToString(v))
	}
	return js.ToNumber(v)
}

// labelTrim is doc.label?.trim(). The second result is false when upstream
// throws: the label is not nullish and has no trim method.
func labelTrim(label any) (trimmed any, ok bool) {
	if js.IsNullish(label) {
		return nil, true
	}
	if s, isString := label.(string); isString {
		return js.TrimSpace(s), true
	}
	return nil, false
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
}

// AddVertex is addVertex(id, textObj, type, style, classes, dir, props,
// metadata): the parser calls it when it finds a node definition.
func (d *DB) AddVertex(id string, a VertexArgs) error {
	if id == "" || len(js.TrimSpace(id)) == 0 {
		return nil
	}
	// Extract the metadata from the shapeData, the syntax for adding metadata for nodes and edges is the same
	// so at this point we don't know if it's a node or an edge, but we can still extract the metadata
	var doc any
	hasDoc := false
	if a.Metadata != nil {
		v, err := loadMetadata(*a.Metadata)
		if err != nil {
			return err
		}
		doc = v
		hasDoc = v != nil
	}

	// Check if this is metadata for an already-declared subgraph
	// (e.g. `sub1@{ view: collapsed }`). The id refers to a subgraph, so
	// route the metadata onto the subgraph instead of creating a vertex.
	if subGraph, ok := d.subGraphLookup.Lookup(id); ok && js.Truthy(doc) {
		var old any
		if subGraph.Metadata != nil {
			old = subGraph.Metadata
		}
		subGraph.Metadata = spread(old, doc)
		return nil
	}

	// Check if this is an edge
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
		vertex.Text = stripQuotes(txt)
	} else if vertex.Text == nil {
		vertex.Text = id
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
		// adds no own property.
		props.Each(func(k string, v any) {
			if k != "__proto__" {
				vertex.Props.Set(k, v)
			}
		})
	}

	if hasDoc {
		if js.IsNullish(doc) {
			return typeError("Cannot read properties of null (reading 'shape')")
		}
		if shape := prop(doc, "shape"); js.Truthy(shape) {
			s, isString := shape.(string)
			if !isString {
				return typeError("doc.shape.toLowerCase is not a function")
			}
			if s != js.ToLowerCase(s) || strings.Contains(s, "_") {
				return newError("No such shape: " + s + ". Shape names should be lowercase.")
			} else if !isValidShape(s) {
				return newError("No such shape: " + s + ".")
			}
			vertex.Type, vertex.HasType = s, true
		}

		label := prop(doc, "label")
		if js.Truthy(label) {
			vertex.Text = label
			vertex.LabelType = sanitizeNodeLabelType(prop(doc, "labelType"))
		}
		// clearText is the step that an icon and an image share: a node
		// that shows only its id as the text gets no text.
		clearText := func() error {
			trimmed, ok := labelTrim(label)
			if !ok {
				return typeError("doc.label?.trim is not a function")
			}
			if !js.Truthy(trimmed) && js.StrictEquals(vertex.Text, id) {
				vertex.Text = ""
			}
			return nil
		}
		if icon := prop(doc, "icon"); js.Truthy(icon) {
			vertex.Icon = icon
			if err := clearText(); err != nil {
				return err
			}
		}
		if form := prop(doc, "form"); js.Truthy(form) {
			vertex.Form = form
		}
		if pos := prop(doc, "pos"); js.Truthy(pos) {
			vertex.Pos = pos
		}
		if img := prop(doc, "img"); js.Truthy(img) {
			vertex.Img = img
			if err := clearText(); err != nil {
				return err
			}
		}
		if constraint := prop(doc, "constraint"); js.Truthy(constraint) {
			vertex.Constraint = constraint
		}
		if w := prop(doc, "w"); js.Truthy(w) {
			vertex.AssetWidth, vertex.HasAssetWidth = toNumber(w), true
		}
		if h := prop(doc, "h"); js.Truthy(h) {
			vertex.AssetHeight, vertex.HasAssetHeight = toNumber(h), true
		}
	}
	return nil
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

// edgeAt is this.edges[pos] for a position of the grammar. The grammar
// gives the positions as strings, so upstream reads a property of the
// array: only a canonical index finds an edge.
func (d *DB) edgeAt(pos string) *FlowEdge {
	i, ok := js.IsArrayIndex(pos)
	if !ok || int(i) >= len(d.edges) {
		return nil
	}
	return d.edges[i]
}

// UpdateLinkInterpolate is updateLinkInterpolate(positions, interpolate):
// it updates the line interpolation algorithm of links. A position is
// "default" or the text of a number.
func (d *DB) UpdateLinkInterpolate(positions []string, interpolate string) error {
	for _, pos := range positions {
		if pos == "default" {
			d.defaultInterpolate, d.hasDefaultInterpolate = interpolate, true
		} else {
			edge := d.edgeAt(pos)
			if edge == nil {
				return typeError("Cannot set properties of undefined (setting 'interpolate')")
			}
			edge.Interpolate = interpolate
		}
	}
	return nil
}

// UpdateLink is updateLink(positions, style): it updates links with a
// style. A position is "default" or the text of a number.
//
// Upstream checks the bounds only for a position that is a number, and the
// grammar gives strings. So a position without an edge is a TypeError.
func (d *DB) UpdateLink(positions []string, style []string) error {
	for _, pos := range positions {
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
	d.diagramID = ""
	d.subGraphs = nil
	d.subGraphLookup = js.NewMap[string, *FlowSubGraph]()
	d.subCount = 0
	d.tooltips = js.NewMap[string, string]()
	d.firstGraphFlag = true
	d.version = ver
	d.config = d.getConfig()
	d.ClearCommon()
}

// SetGen is setGen(ver).
func (d *DB) SetGen(ver string) {
	if ver == "" {
		ver = "gen-2"
	}
	d.version = ver
}

// DefaultStyle is defaultStyle().
func (d *DB) DefaultStyle() string {
	return "fill:#ffa;stroke: #f66; stroke-width: 3px; stroke-dasharray: 5, 5;fill:#ffa;stroke: #666;"
}

// uniq is the uniq function of addSubGraph: it removes the duplicates and
// the white space from the items of a subgraph, and takes the direction
// statement out. An item is a string or a *dirStmt.
//
// The store of upstream for the ids that it has seen is a plain object, and
// two ids collide with its prototype. An assignment to __proto__ adds no
// property, so that id is never a duplicate. The id hasOwnProperty hides the
// method that the next lookup calls, so the next id is a TypeError.
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
				return nil, "", false, typeError("prims[type].hasOwnProperty is not a function")
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

// AddSubGraph is addSubGraph(_id, list, _title). An item of list is a
// string, a list of strings, or a direction statement. The result is the id
// of the subgraph.
func (d *DB) AddSubGraph(_id *FlowText, list []any, _title *FlowText) (string, error) {
	if _id == nil {
		return "", typeError("Cannot read properties of undefined (reading 'text')")
	}
	id, hasID := js.TrimSpace(_id.Text), true
	if _title == nil {
		return "", typeError("Cannot read properties of undefined (reading 'text')")
	}
	title := _title.Text
	if _id == _title && strings.ContainsFunc(_title.Text, js.IsWhiteSpace) {
		hasID = false
	}

	nodeList, dir, hasDir, err := uniq(flat(list))
	if err != nil {
		return "", err
	}
	if !hasDir {
		if js.Truthy(js.Path(d.getConfig(), "flowchart", "inheritDir")) {
			if direction, ok := d.GetDirection(); ok {
				dir, hasDir = direction, true
			} else if v := d.getConfig().Get("direction"); !js.IsNullish(v) {
				dir, hasDir = js.ToString(v), true
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
	title, err = d.sanitizeText(title)
	if err != nil {
		return "", err
	}
	d.subCount = d.subCount + 1

	subGraph := &FlowSubGraph{
		ID:        id,
		Nodes:     nodeList,
		Title:     js.TrimSpace(title),
		Classes:   []string{},
		Dir:       dir,
		HasDir:    hasDir,
		LabelType: sanitizeNodeLabelType(_title.Type),
	}

	// Remove the members in the new subgraph if they already belong to another subgraph.
	// A subgraph never contains itself, so a redeclaration nested in its own body flattens.
	nodes := []string{}
	for _, nodeID := range d.MakeUniq(subGraph, d.subGraphs) {
		if nodeID != id {
			nodes = append(nodes, nodeID)
		}
	}
	subGraph.Nodes = nodes

	// A repeated id merges its members into the first declaration, which keeps its
	// title, direction and position, so every layout sees a single subgraph.
	var existing *FlowSubGraph
	for _, sg := range d.subGraphs {
		if sg.ID == id {
			existing = sg
			break
		}
	}
	if existing != nil {
		existing.Nodes = append(existing.Nodes, subGraph.Nodes...)
	} else {
		d.subGraphs = append(d.subGraphs, subGraph)
	}
	// The lookup points at the subgraph that is drawn, so classes and metadata such as
	// `view: collapsed` apply whichever declaration they follow.
	if existing != nil {
		d.subGraphLookup.Set(id, existing)
	} else {
		d.subGraphLookup.Set(id, subGraph)
	}
	return id, nil
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

// GetSubGraphs is getSubGraphs().
func (d *DB) GetSubGraphs() []*FlowSubGraph {
	return d.subGraphs
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

// destructStartLink is destructStartLink(_str).
func destructStartLink(_str string) FlowLink {
	str := js.UTF16Units(js.TrimSpace(_str))
	typ := "arrow_open"

	if len(str) > 0 {
		switch str[0] {
		case '<':
			typ = "arrow_point"
			str = str[1:]
		case 'x':
			typ = "arrow_cross"
			str = str[1:]
		case 'o':
			typ = "arrow_circle"
			str = str[1:]
		}
	}

	stroke := "normal"

	if containsUnit(str, '=') {
		stroke = "thick"
	}

	if containsUnit(str, '.') {
		stroke = "dotted"
	}

	return FlowLink{Type: typ, Stroke: stroke, Length: undef}
}

func containsUnit(units []rune, unit rune) bool {
	for _, u := range units {
		if u == unit {
			return true
		}
	}
	return false
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

	// dropFirst is line.slice(1).
	dropFirst := func() {
		if len(line) > 0 {
			line = line[1:]
		}
	}
	if len(str) > 0 {
		switch str[len(str)-1] {
		case 'x':
			typ = "arrow_cross"
			if str[0] == 'x' {
				typ = "double_" + typ
				dropFirst()
			}
		case '>':
			typ = "arrow_point"
			if str[0] == '<' {
				typ = "double_" + typ
				dropFirst()
			}
		case 'o':
			typ = "arrow_circle"
			if str[0] == 'o' {
				typ = "double_" + typ
				dropFirst()
			}
		}
	}

	stroke := "normal"
	length := len(line) - 1

	if len(line) > 0 && line[0] == '=' {
		stroke = "thick"
	}

	if len(line) > 0 && line[0] == '~' {
		stroke = "invisible"
	}

	dots := countChar('.', line)

	if dots != 0 {
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

		if startInfo.Type == "arrow_open" {
			// -- xyz -->  - take arrow type from ending
			startInfo.Type = info.Type
		} else {
			// x-- xyz -->  - not supported
			if startInfo.Type != info.Type {
				return FlowLink{Type: "INVALID", Stroke: "INVALID", Length: undef}
			}

			startInfo.Type = "double_" + startInfo.Type
		}

		if startInfo.Type == "double_arrow" {
			startInfo.Type = "double_arrow_point"
		}

		startInfo.Length = info.Length
		return startInfo
	}

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
