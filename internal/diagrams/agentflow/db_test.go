package agentflow

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/purify"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The golden file is the db of real mermaid at the moment when the renderer
// draws: tools/reference/gen-agentflow-db.mjs writes it with the browser
// oracle. The tests parse the same texts and compare everything.
//
// The values of both sides are dynamic JavaScript values (internal/js), so
// that the order of the keys of an object can count. It counts for the
// metadata and the props of an element, which come from the YAML loader.

type goldenFile struct {
	BaseConfig any           `json:"-"`
	Cases      []*goldenCase `json:"-"`
	API        []*goldenAPI  `json:"-"`
	Links      []any
	Commas     []any
	Proto      []any
	Shapes     []any
	Slots      []any
	Constants  *js.Obj[any]
}

// goldenCase is one case of the golden file: the fields that the test reads
// by name, and the whole record.
type goldenCase struct {
	rec  *js.Obj[any]
	Name string
	// Text is absent for a corpus fixture: Name is then its path.
	Text    string
	HasText bool
	// Fresh is true for a case that ran in a new page.
	Fresh bool
	// Site is the site config: the argument of mermaid.initialize.
	Site      *js.Obj[any]
	Error     string
	HasError  bool
	ErrorName string
	Type      string
	// Other is true when another diagram type drew the text.
	Other bool
}

type goldenAPI struct {
	rec  *js.Obj[any]
	Name string
	Site *js.Obj[any]
	Ops  []any
}

func str(o *js.Obj[any], key string) string {
	s, _ := o.Get(key).(string)
	return s
}

func list(o *js.Obj[any], key string) []any {
	l, _ := o.Get(key).([]any)
	return l
}

func loadGolden(tb testing.TB) *goldenFile {
	tb.Helper()
	f, err := os.Open(filepath.Join("testdata", "db.json.gz"))
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		tb.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		tb.Fatal(err)
	}
	v, err := js.ParseJSON(data)
	if err != nil {
		tb.Fatal(err)
	}
	doc := js.AsObj(v)
	g := &goldenFile{
		BaseConfig: doc.Get("baseConfig"),
		Links:      list(doc, "links"),
		Commas:     list(doc, "commas"),
		Proto:      list(doc, "proto"),
		Shapes:     list(doc, "shapes"),
		Slots:      list(doc, "slots"),
		Constants:  js.AsObj(doc.Get("constants")),
	}
	for _, c := range list(doc, "cases") {
		rec := js.AsObj(c)
		gc := &goldenCase{
			rec:       rec,
			Name:      str(rec, "name"),
			Fresh:     js.Truthy(rec.Get("fresh")),
			Site:      js.AsObj(rec.Get("site")),
			ErrorName: str(rec, "errorName"),
			Type:      str(rec, "type"),
			Other:     js.Truthy(rec.Get("otherDiagram")),
		}
		gc.Text, gc.HasText = rec.Get("text").(string)
		gc.Error, gc.HasError = rec.Get("error").(string)
		g.Cases = append(g.Cases, gc)
	}
	for _, c := range list(doc, "api") {
		rec := js.AsObj(c)
		g.API = append(g.API, &goldenAPI{rec: rec, Name: str(rec, "name"), Site: js.AsObj(rec.Get("site")), Ops: list(rec, "ops")})
	}
	return g
}

func caseText(tb testing.TB, c *goldenCase) string {
	tb.Helper()
	if c.HasText {
		return c.Text
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "..", filepath.FromSlash(c.Name)))
	if err != nil {
		tb.Fatal(err)
	}
	return string(data)
}

// capture is what a layout algorithm receives from the draw function of
// the diagram in one render: the state at the moment when the oracle dumps
// the db of upstream.
type capture struct {
	// warm says that the page has rendered a diagram before, so the
	// DOMPurify hooks of mermaid are installed.
	warm bool
	// pipeline says that the parser gets the text of the pipeline of
	// internal/render, and not the text that Diagram.fromText chooses.
	pipeline bool
	// code is the DiagramCode object of the text, as renderDiagram gives it
	// to Diagram.fromText.
	code config.DiagramCode
	// textToParse is the text that the parser received.
	textToParse string
	ctx         *render.Context
	db          *DB
	// data is the layout data, after ru.Render has added the id of the
	// diagram to the DOM ids.
	data *LayoutData
	// style is the text of the style element of the diagram.
	style    string
	hasStyle bool
}

var (
	// captures has the capture of each render that is under test, by the
	// time zone of the render: a pointer that only that render has.
	captures    sync.Map
	captureOnce sync.Once
	errCaptured = errors.New("agentflow test: the db is captured")
)

// layoutNames are the layouts of the full build of mermaid.
var layoutNames = []string{"dagre", "swimlane", "cose-bilkent", "elk", "elk.stress", "elk.force", "elk.mrtree", "elk.sporeOverlap", "elk.box", "elk.rectpacking"}

// sourceText is the choice of Diagram.fromText between the cleaned text and
// the text with its comments, for a db with preserveCommentsWhenParsing.
// The pipeline of internal/render does not make this choice yet: it parses
// the cleaned text. The capture of the test makes it, so that the test
// compares the db for the text that upstream parses.
func sourceText(code config.DiagramCode, cfg *js.Obj[any], d *DB) string {
	source := code.Cleaned
	if d.PreserveCommentsWhenParsing() && code.WithComments != "" {
		// config.maxTextSize ?? defaultConfig.maxTextSize
		var maxTextSize any = 50000.0
		if v := cfg.Get("maxTextSize"); !js.IsNullish(v) {
			maxTextSize = v
		}
		if !js.Compare(float64(js.UTF16Len(code.WithComments)), ">", maxTextSize) {
			source = code.WithComments
		}
	}
	return source
}

// installCapture changes the test binary so that a render that is under
// test stops when the draw function hands the layout data to the layout
// algorithm, as the oracle script does with upstream: every layout becomes
// a layout that captures the data, and the diagram type gets a draw
// function that runs the steps of draw up to ru.Render.
func installCapture() {
	captureOnce.Do(func() {
		for _, name := range layoutNames {
			ru.RegisterLayout(name, "", func(ctx *render.Context, data *ru.LayoutData, _ *dom.Node, _ string) error {
				if _, ok := captures.Load(ctx.Location); !ok {
					return &ru.NotPortedError{Kind: "layout", Name: data.LayoutAlgorithm}
				}
				return errCaptured
			})
		}
		def := NewDefinition()
		newDB, parseText, draw := def.New, def.Parse, def.Draw
		def.New = func(ctx *render.Context) render.DB {
			if c, ok := captures.Load(ctx.Location); ok && c.(*capture).warm {
				ctx.Sanitizer.RemoveScript("")
			}
			return newDB(ctx)
		}
		def.Parse = func(ctx *render.Context, db render.DB, text string) error {
			c, ok := captures.Load(ctx.Location)
			if !ok || c.(*capture).pipeline {
				return parseText(ctx, db, text)
			}
			cp := c.(*capture)
			d := db.(*DB)
			// The steps of Diagram.fromText that the pipeline does not have.
			if cp.code.FrontmatterLineOffset != 0 {
				d.SetFrontmatterLineOffset(cp.code.FrontmatterLineOffset)
			}
			cp.textToParse = render.EncodeEntities(sourceText(cp.code, ctx.Config, d)) + "\n"
			return parseText(ctx, db, cp.textToParse)
		}
		def.Draw = func(ctx *render.Context, db render.DB, text string) error {
			c, ok := captures.Load(ctx.Location)
			if !ok {
				return draw(ctx, db, text)
			}
			cp := c.(*capture)
			cp.ctx, cp.db = ctx, db.(*DB)
			if style := ctx.SVG.Select("style"); style != nil {
				cp.style, cp.hasStyle = style.TextContent(), true
			}
			// The first steps of draw.
			cp.data = layoutData(ctx, cp.db)
			return ru.Render(ctx, cp.data.LayoutData, ctx.GetDiagramElement())
		}
		render.Register(def)
	})
}

// outcome is the result of the steps of mermaid.render up to the draw
// function of the diagram.
type outcome struct {
	// cp is set when the diagram is an agentflow diagram and the parser
	// accepted it.
	cp *capture
	// otherType is the diagram type when it is not agentflow.
	otherType string
	// err is the error of the config, of the type detection or of the parser.
	err error
}

// renderToDraw runs render.Render for a text until the draw function of
// the diagram. site is the site config: the argument of mermaid.initialize.
func renderToDraw(text string, site *js.Obj[any], warm bool) outcome {
	return renderCapture(text, site, &capture{warm: warm})
}

func renderCapture(text string, site *js.Obj[any], cp *capture) outcome {
	installCapture()
	zone := time.FixedZone("agentflow-test", 0)
	// The DiagramCode object, as renderDiagram computes it.
	if resolved, err := config.Resolve(text, site); err == nil {
		cp.code = resolved.Source
		maxTextSize := 50000.0
		if v, ok := resolved.Config.Get("maxTextSize").(float64); ok {
			maxTextSize = v
		}
		if float64(js.UTF16Len(cp.code.Cleaned)) > maxTextSize {
			// The text is replaced by the text of the error diagram.
			cp.code = config.DiagramCode{}
		}
	}
	captures.Store(zone, cp)
	defer captures.Delete(zone)
	_, err := render.Render(text, render.Options{Config: site, Location: zone})
	var unsupported *render.UnsupportedError
	switch {
	case errors.Is(err, errCaptured) && cp.db != nil:
		return outcome{cp: cp}
	case errors.Is(err, errCaptured):
		return outcome{otherType: "another diagram type"}
	case errors.As(err, &unsupported):
		return outcome{otherType: unsupported.Type}
	case err == nil:
		return outcome{err: errors.New("the render did not reach the draw function of the diagram")}
	}
	return outcome{err: err}
}

// textHash is the textHash function of the oracle script: the length and
// two 32-bit hashes of the UTF-16 code units of a text.
func textHash(s string) string {
	h1, h2 := uint32(0x811c9dc5), uint32(0x9e3779b9)
	units := utf16.Encode([]rune(s))
	for _, unit := range units {
		u := uint32(unit)
		h1 = (h1 ^ u) * 16777619
		h2 = ((h2 + u) * 0x85ebca6b) ^ (h2 >> 13)
	}
	return fmt.Sprintf("%d:%x:%x", len(units), h1, h2)
}

// errorInfo is the name, the message, the hash and the mark of the
// JavaScript exception for an error of this port.
func errorInfo(err error) (name, message string, hash, mark any) {
	var ce *config.Error
	if errors.As(err, &ce) {
		return ce.Name, ce.Message, nil, nil
	}
	var ae *Error
	if errors.As(err, &ae) {
		if ae.Hash != nil {
			hash = obj(
				"text", "",
				"token", js.Null,
				"line", float64(ae.Hash.Line),
				"loc", obj(
					"first_line", float64(ae.Hash.Loc.FirstLine),
					"last_line", float64(ae.Hash.Loc.LastLine),
					"first_column", float64(ae.Hash.Loc.FirstColumn),
					"last_column", float64(ae.Hash.Loc.LastColumn),
				),
				"expected", []any{},
			)
		}
		if ae.Mark != nil {
			mark = obj("line", float64(ae.Mark.Line), "column", float64(ae.Mark.Column))
		}
		return ae.Name, ae.Message, hash, mark
	}
	var je *jison.Error
	if errors.As(err, &je) {
		if je.Hash != nil {
			h := js.NewObj[any]()
			h.Set("text", je.Hash.Text)
			if je.Hash.TokenIsNull {
				h.Set("token", js.Null)
			} else {
				h.Set("token", je.Hash.Token)
			}
			h.Set("line", float64(je.Hash.Line))
			if je.Hash.Loc != nil {
				h.Set("loc", obj(
					"first_line", float64(je.Hash.Loc.FirstLine),
					"last_line", float64(je.Hash.Loc.LastLine),
					"first_column", float64(je.Hash.Loc.FirstColumn),
					"last_column", float64(je.Hash.Loc.LastColumn),
				))
			}
			if je.Hash.Expected != nil {
				h.Set("expected", stringsJS(je.Hash.Expected))
			}
			hash = h
		}
		return je.Name, je.Message, hash, nil
	}
	var pe *purify.TypeError
	if errors.As(err, &pe) {
		return "TypeError", pe.Message, nil, nil
	}
	return "Error", err.Error(), nil, nil
}

// ---- conversion of the Go side to JavaScript values ----

// obj builds a JavaScript object from keys and values. A value that is nil
// (undefined) is left out, as JSON.stringify does.
func obj(pairs ...any) *js.Obj[any] {
	o := js.NewObj[any]()
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] != nil {
			o.Set(pairs[i].(string), pairs[i+1])
		}
	}
	return o
}

func stringsJS(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

// num is a number of the golden file: { $num: "NaN" } for a number that is
// not finite, as the oracle writes it.
func num(x float64) any {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return obj("$num", js.Str(x))
	}
	return x
}

// optNum is a number that is undefined when it is NaN.
func optNum(x float64) any {
	if x != x {
		return nil
	}
	return x
}

// optStr is a string that is undefined when it is empty.
func optStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func optFlag(set bool, v any) any {
	if !set {
		return nil
	}
	return v
}

// metadataJS is the metadata of an element: nil when it has none.
func metadataJS(m *js.Obj[any]) any {
	if m == nil {
		return nil
	}
	return jsonValue(m)
}

// jsonValue is JSON.parse(JSON.stringify(v)) with the number rule of the
// oracle: a property that is undefined is left out, undefined in an array
// is null, and a number that is not finite is { $num }.
func jsonValue(v any) any {
	switch x := v.(type) {
	case nil:
		return js.Null
	case float64:
		return num(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = jsonValue(e)
		}
		return out
	case *js.Obj[any]:
		if x == nil {
			return js.Null
		}
		out := js.NewObj[any]()
		x.Each(func(k string, e any) {
			if e != nil {
				out.Set(k, jsonValue(e))
			}
		})
		return out
	}
	return v
}

// withExtra adds the properties of Extra. They win over the fields: Extra
// has the JavaScript value of a property that the field cannot hold.
func withExtra(o *js.Obj[any], extra map[string]any) *js.Obj[any] {
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		o.Set(k, jsonValue(extra[k]))
	}
	return o
}

// nodeJSON is JSON.stringify of a node of the layout data, as upstream
// builds it in getData.
func nodeJSON(n *ru.Node) *js.Obj[any] {
	var cssStyles any
	if n.CSSStylesSet {
		styles := n.CSSStyles
		if styles == nil {
			styles = []string{}
		}
		cssStyles = stringsJS(styles)
	}
	compiled := n.CSSCompiledStyles
	if compiled == nil {
		compiled = []string{}
	}
	return withExtra(obj(
		"id", n.ID,
		"label", optFlag(n.HasLabel, n.Label),
		"labelStyle", n.LabelStyle,
		"labelType", n.LabelType,
		"parentId", optStr(n.ParentID),
		"padding", optNum(n.Padding),
		"cssStyles", cssStyles,
		"cssCompiledStyles", stringsJS(compiled),
		"cssClasses", optFlag(n.CSSClassesSet, n.CSSClasses),
		"dir", optStr(n.Dir),
		"domId", optStr(n.DomID),
		"look", optStr(n.Look),
		"link", optStr(n.Link),
		"linkTarget", optStr(n.LinkTarget),
		"tooltip", optStr(n.Tooltip),
		"metadata", metadataJS(n.Metadata),
		"isGroup", n.IsGroup,
		"shape", n.Shape,
		"wrappingWidth", optNum(n.WrappingWidth),
		"minWidth", optNum(n.MinWidth),
		"colorIndex", optNum(n.ColorIndex),
	), n.Extra)
}

// edgeJSON is JSON.stringify of an edge of the layout data.
func edgeJSON(e *ru.Edge) *js.Obj[any] {
	var animate any
	if e.HasAnimate {
		animate = e.Animate
	}
	return withExtra(obj(
		"id", e.ID,
		"isUserDefinedId", e.IsUserDefinedID,
		"start", e.Start,
		"end", e.End,
		"type", e.Type,
		"label", optFlag(e.HasLabel, e.Label),
		"labelType", e.LabelType,
		"labelpos", e.Labelpos,
		"thickness", optStr(e.Thickness),
		"minlen", optNum(e.Minlen),
		"classes", e.Classes,
		"arrowTypeStart", e.ArrowTypeStart,
		"arrowTypeEnd", e.ArrowTypeEnd,
		"arrowheadStyle", e.ArrowheadStyle,
		"cssCompiledStyles", stringsJS(e.CSSCompiledStyles),
		"labelStyle", stringsJS(e.LabelStyle),
		"style", stringsJS(e.Style),
		"pattern", optStr(e.Pattern),
		"look", optStr(e.Look),
		"animate", animate,
		"animation", optStr(e.Animation),
		"curve", optStr(e.Curve),
		"metadata", metadataJS(e.Metadata),
	), e.Extra)
}

// vertexJSON is JSON.stringify of a FlowVertex.
func vertexJSON(v *FlowVertex) *js.Obj[any] {
	var props any
	if v.Props != nil {
		props = jsonValue(v.Props)
	}
	return obj(
		"id", v.ID,
		"labelType", v.LabelType,
		"domId", v.DomID,
		"styles", stringsJS(v.Styles),
		"classes", stringsJS(v.Classes),
		"isConnector", optFlag(v.IsConnector, true),
		"text", optFlag(v.HasText, v.Text),
		"type", optFlag(v.HasType, v.Type),
		"dir", optFlag(v.HasDir, v.Dir),
		"props", props,
		"metadata", metadataJS(v.Metadata),
		"haveCallback", optFlag(v.HaveCallback, true),
		"link", optFlag(v.HasLink, v.Link),
		"linkTarget", optFlag(v.HasLinkTarget, v.LinkTarget),
	)
}

func verticesJSON(list []*FlowVertex) []any {
	out := make([]any, len(list))
	for i, v := range list {
		out[i] = vertexJSON(v)
	}
	return out
}

// rawEdgeJSON is JSON.stringify of a FlowEdge.
func rawEdgeJSON(e *FlowEdge) *js.Obj[any] {
	var interpolate, animate, animation, style any
	if e.Interpolate != nil {
		interpolate = jsonValue(e.Interpolate)
	}
	if e.Animate != nil {
		animate = jsonValue(e.Animate)
	}
	if e.Animation != nil {
		animation = jsonValue(e.Animation)
	}
	if e.Style != nil {
		style = stringsJS(e.Style)
	}
	return obj(
		"start", e.Start,
		"end", e.End,
		"type", optStr(e.Type),
		"text", e.Text,
		"labelType", e.LabelType,
		"classes", stringsJS(e.Classes),
		"isUserDefinedId", e.IsUserDefinedID,
		"interpolate", interpolate,
		"stroke", optStr(e.Stroke),
		"length", optNum(e.Length),
		"edgeSemantic", optStr(e.EdgeSemantic),
		"id", e.ID,
		"style", style,
		"animate", animate,
		"animation", animation,
		"metadata", metadataJS(e.Metadata),
	)
}

// subGraphJSON is JSON.stringify of a FlowSubGraph.
func subGraphJSON(sg *FlowSubGraph) *js.Obj[any] {
	return obj(
		"id", sg.ID,
		"nodes", stringsJS(sg.Nodes),
		"title", sg.Title,
		"classes", stringsJS(sg.Classes),
		"dir", sg.dirJS(),
		"labelType", sg.LabelType,
		"type", sg.Type,
		"metadata", metadataJS(sg.Metadata),
	)
}

func positionJSON(p *ElementPosition) *js.Obj[any] {
	return obj(
		"startLine", float64(p.StartLine),
		"startColumn", float64(p.StartColumn),
		"endLine", float64(p.EndLine),
		"endColumn", float64(p.EndColumn),
		"startIndex", float64(p.StartIndex),
		"endIndex", float64(p.EndIndex),
	)
}

func mappingJSON(m *ElementMapping) any {
	if m == nil {
		return js.Null
	}
	return obj("id", m.ID, "type", m.Type, "position", positionJSON(m.Position))
}

func diagnosticsJSON(list []*Diagnostic) []any {
	out := make([]any, len(list))
	for i, d := range list {
		var position any
		if d.Position != nil {
			position = positionJSON(d.Position)
		}
		out[i] = obj(
			"id", d.ID,
			"severity", d.Severity,
			"message", d.Message,
			"nodeId", optStr(d.NodeID),
			"edgeId", optStr(d.EdgeID),
			"position", position,
		)
	}
	return out
}

func semanticModelJSON(m *SemanticModel) *js.Obj[any] {
	vertices := make([]any, len(m.Vertices))
	for i, v := range m.Vertices {
		vertices[i] = obj(
			"id", v.ID,
			"label", optFlag(v.HasLabel, v.Label),
			"shape", optFlag(v.HasShape, v.Shape),
			"vertexKind", v.VertexKind,
			"metadata", metadataJS(v.Metadata),
		)
	}
	edges := make([]any, len(m.Edges))
	for i, e := range m.Edges {
		edges[i] = obj(
			"start", e.Start,
			"end", e.End,
			"id", e.ID,
			"label", optFlag(e.HasLabel, e.Label),
			"type", optStr(e.Type),
			"stroke", optStr(e.Stroke),
			"edgeSemantic", optStr(e.EdgeSemantic),
			"length", optNum(e.Length),
			"metadata", metadataJS(e.Metadata),
		)
	}
	subGraphs := make([]any, len(m.SubGraphs))
	for i, sg := range m.SubGraphs {
		subGraphs[i] = obj(
			"id", sg.ID,
			"nodes", stringsJS(sg.Nodes),
			"type", sg.Type,
			"title", sg.Title,
			"direction", sg.Direction,
			"metadata", metadataJS(sg.Metadata),
		)
	}
	connectors := make([]any, len(m.Connectors))
	for i, c := range m.Connectors {
		connectors[i] = obj(
			"id", c.ID,
			"title", optFlag(c.HasTitle, c.Title),
			"metadata", metadataJS(c.Metadata),
		)
	}
	return obj(
		"vertices", vertices,
		"edges", edges,
		"subGraphs", subGraphs,
		"connectors", connectors,
		"diagnostics", diagnosticsJSON(m.Diagnostics),
		"direction", optFlag(m.HasDirection, m.Direction),
	)
}

func tag(m *ElementMapping) any {
	if m == nil {
		return js.Null
	}
	return m.ID + "|" + m.Type
}

func tags(list []*ElementMapping) []any {
	out := make([]any, len(list))
	for i, m := range list {
		out[i] = tag(m)
	}
	return out
}

// dbState is the dbState function of the oracle script: the content of a
// db, without the layout data.
func dbState(d *DB) *js.Obj[any] {
	classes := []any{}
	d.GetClasses().Each(func(id string, def *render.ClassDef) {
		classes = append(classes, []any{id, obj("id", def.ID, "styles", stringsJS(def.Styles), "textStyles", stringsJS(def.TextStyles))})
	})
	subGraphs := []any{}
	for _, sg := range d.GetSubGraphs() {
		subGraphs = append(subGraphs, subGraphJSON(sg))
	}
	vertices := []any{}
	domIDs := []any{}
	d.GetVertices().Each(func(id string, v *FlowVertex) {
		vertices = append(vertices, []any{id, vertexJSON(v)})
		domIDs = append(domIDs, d.LookUpDomId(id))
	})
	rawEdges := []any{}
	for _, e := range d.GetEdges() {
		rawEdges = append(rawEdges, rawEdgeJSON(e))
	}
	tooltips := []any{}
	d.Tooltips().Each(func(id, tip string) { tooltips = append(tooltips, []any{id, tip}) })
	tools := []any{}
	for _, v := range d.GetTools() {
		tools = append(tools, v.ID)
	}
	mappings := d.GetElementMappings()
	mappingList := make([]any, len(mappings))
	for i, m := range mappings {
		mappingList[i] = mappingJSON(m)
	}
	stats := d.GetMappingStats()

	rec := obj(
		"classes", classes,
		"subGraphs", subGraphs,
		"vertices", vertices,
		"rawEdges", rawEdges,
		"tooltips", tooltips,
		"title", d.GetDiagramTitle(),
		"accTitle", d.GetAccTitle(),
		"accDescription", d.GetAccDescription(),
		"domIds", domIDs,
		"connectors", verticesJSON(d.GetConnectors()),
		"globalNodes", stringsJS(d.GlobalNodes()),
		"tools", tools,
		"diagnostics", diagnosticsJSON(d.GetDiagnostics()),
		"mappings", mappingList,
		"mappingStats", obj(
			"vertices", float64(stats.Vertices),
			"edges", float64(stats.Edges),
			"subgraphs", float64(stats.Subgraphs),
			"connectors", float64(stats.Connectors),
			"attachments", float64(stats.Attachments),
			"totalElements", float64(stats.TotalElements),
		),
		"semanticModel", semanticModelJSON(d.GetSemanticModel()),
	)
	if direction, ok := d.GetDirection(); ok {
		rec.Set("direction", direction)
	}
	defaultStyle, defaultInterpolate, hasInterpolate := d.DefaultLinkStyle()
	if defaultStyle != nil {
		rec.Set("defaultStyle", stringsJS(defaultStyle))
	}
	if hasInterpolate {
		rec.Set("defaultInterpolate", defaultInterpolate)
	}

	// The queries of the element mappings, for the positions of the mappings.
	lineSet := map[int]bool{}
	for _, m := range mappings {
		lineSet[m.Position.StartLine] = true
		lineSet[m.Position.EndLine] = true
		lineSet[m.Position.EndLine+1] = true
	}
	lines := make([]int, 0, len(lineSet))
	for l := range lineSet {
		lines = append(lines, l)
	}
	sort.Ints(lines)
	if len(lines) > 16 {
		lines = lines[:16]
	}
	onLine := []any{}
	for _, l := range lines {
		onLine = append(onLine, []any{float64(l), tags(d.GetElementsOnLine(l))})
	}
	rec.Set("onLine", onLine)
	atPosition := []any{}
	for i, m := range mappings {
		if i >= 24 {
			break
		}
		p := m.Position
		for _, at := range [][2]int{{p.StartLine, p.StartColumn}, {p.EndLine, p.EndColumn}, {p.StartLine, p.EndColumn}, {p.EndLine, p.StartColumn + 1}} {
			atPosition = append(atPosition, []any{float64(at[0]), float64(at[1]), tag(d.GetElementAtPosition(at[0], at[1]))})
		}
	}
	rec.Set("atPosition", atPosition)
	byID := []any{}
	seen := map[string]bool{}
	for _, m := range mappings {
		if seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		if len(byID) >= 24 {
			break
		}
		byID = append(byID, mappingJSON(d.GetElementById(m.ID)))
	}
	rec.Set("byId", byID)

	d.IndexNodes()
	depthFirstPos := []any{}
	for i := range d.GetSubGraphs() {
		if pos, ok := d.GetDepthFirstPos(i); ok {
			depthFirstPos = append(depthFirstPos, float64(pos))
		} else {
			depthFirstPos = append(depthFirstPos, js.Null)
		}
	}
	rec.Set("depthFirstPos", depthFirstPos)
	return rec
}

// ---- comparison ----

// orderedKeys are the properties whose value is data of the author: the
// order of the keys of such a value counts.
var orderedKeys = map[string]bool{"metadata": true, "props": true}

// diff returns a description of the first difference between two
// JavaScript values, or "". The order of the keys of an object counts when
// ordered is true, and under a property of orderedKeys.
func diff(path string, got, want any, ordered bool) string {
	switch w := want.(type) {
	case *js.Obj[any]:
		g, ok := got.(*js.Obj[any])
		if !ok || g == nil {
			return fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want))
		}
		wk, gk := w.Keys(), g.Keys()
		if ordered {
			if strings.Join(gk, "\x00") != strings.Join(wk, "\x00") {
				return fmt.Sprintf("%s: keys %q, want %q", path, gk, wk)
			}
		} else {
			for _, k := range wk {
				if !g.Has(k) {
					return fmt.Sprintf("%s.%s: missing, want %s", path, k, show(w.Get(k)))
				}
			}
			for _, k := range gk {
				if !w.Has(k) {
					return fmt.Sprintf("%s.%s: got %s, want it absent", path, k, show(g.Get(k)))
				}
			}
		}
		for _, k := range wk {
			if d := diff(path+"."+k, g.Get(k), w.Get(k), ordered || orderedKeys[k]); d != "" {
				return d
			}
		}
		return ""
	case []any:
		g, ok := got.([]any)
		if !ok {
			return fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want))
		}
		for i := 0; i < len(g) && i < len(w); i++ {
			if d := diff(path+"["+strconv.Itoa(i)+"]", g[i], w[i], ordered); d != "" {
				return d
			}
		}
		if len(g) != len(w) {
			return fmt.Sprintf("%s: length %d, want %d", path, len(g), len(w))
		}
		return ""
	}
	if !js.StrictEquals(got, want) {
		return fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want))
	}
	return ""
}

func show(v any) string {
	if v == nil {
		return "undefined"
	}
	s := js.StringifyJSON(v)
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}

// flatten is the flatten function of the generator: the leaves of a config
// by path. An object with no property is a leaf.
func flatten(v any, prefix []string, out map[string]any) {
	if o, ok := v.(*js.Obj[any]); ok && o != nil && o.Len() > 0 {
		o.Each(func(k string, e any) {
			flatten(e, append(append([]string{}, prefix...), k), out)
		})
		return
	}
	key, _ := json.Marshal(prefix)
	out[string(key)] = v
}

// wantConfig returns the leaves of the config of a case: the base config
// with the delta of the case.
func wantConfig(base map[string]any, delta *js.Obj[any]) map[string]any {
	out := make(map[string]any, len(base))
	for k, v := range base {
		out[k] = v
	}
	for _, p := range list(delta, "del") {
		key, _ := json.Marshal(pathOf(p))
		delete(out, string(key))
	}
	for _, entry := range list(delta, "set") {
		pair := entry.([]any)
		key, _ := json.Marshal(pathOf(pair[0]))
		out[string(key)] = pair[1]
	}
	return out
}

func pathOf(v any) []string {
	l, _ := v.([]any)
	out := make([]string, len(l))
	for i, e := range l {
		out[i], _ = e.(string)
	}
	return out
}

// diffConfig compares a config with the leaves that upstream has.
func diffConfig(got *js.Obj[any], want map[string]any) string {
	gotLeaves := map[string]any{}
	flatten(jsonValue(got), nil, gotLeaves)
	keys := map[string]bool{}
	for k := range gotLeaves {
		keys[k] = true
	}
	for k := range want {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		g, gok := gotLeaves[k]
		w, wok := want[k]
		switch {
		case !gok:
			return fmt.Sprintf("config%s: missing, want %s", k, show(w))
		case !wok:
			return fmt.Sprintf("config%s: got %s, want it absent", k, show(g))
		}
		if d := diff("config"+k, g, w, false); d != "" {
			return d
		}
	}
	return ""
}

// callbackNames are the functions that the page of the oracle defines. A
// click callback with another name fails there and records no call.
var callbackNames = map[string]bool{"callback": true, "noargs": true, "two": true, "cb": true, "other": true, "a.b.c": true}

// comparedKeys are the properties of a record of the golden file that
// compareCase and runCase compare beside the properties of dbState, and the
// properties that describe the case.
var comparedKeys = map[string]bool{
	"name": true, "text": true, "fresh": true, "site": true, "type": true,
	"textHash": true, "nodes": true, "edges": true, "other": true, "dataConnectors": true,
	"configDelta": true, "layoutData": true, "styleHash": true, "getTooltip": true, "callbacks": true,
	"defaultStyle": true, "defaultInterpolate": true, "direction": true,
}

// compareCase compares the db of this port with the dump of upstream. It
// returns the first difference, or "".
func compareCase(base map[string]any, c *goldenCase, cp *capture) string {
	if cp.ctx.Type != c.Type {
		return fmt.Sprintf("type: got %q, want %q", cp.ctx.Type, c.Type)
	}
	rec := c.rec
	if rec.Get("nodes") == nil {
		return "got an agentflow db, want none"
	}
	d := cp.db
	data := cp.data

	if got, want := textHash(cp.textToParse), str(rec, "textHash"); got != want {
		return fmt.Sprintf("checksum of the text for the parser: got %s, want %s", got, want)
	}

	nodes := make([]any, len(data.Nodes))
	for i, n := range data.Nodes {
		nodes[i] = nodeJSON(n)
	}
	if s := diff("nodes", nodes, rec.Get("nodes"), false); s != "" {
		return s
	}
	edges := make([]any, len(data.Edges))
	for i, e := range data.Edges {
		edges[i] = edgeJSON(e)
	}
	if s := diff("edges", edges, rec.Get("edges"), false); s != "" {
		return s
	}
	if s := diff("other", js.NewObj[any](), rec.Get("other"), false); s != "" {
		return s
	}
	if s := diff("dataConnectors", verticesJSON(data.Connectors), rec.Get("dataConnectors"), false); s != "" {
		return s
	}
	if s := diffConfig(data.Config, wantConfig(base, js.AsObj(rec.Get("configDelta")))); s != "" {
		return s
	}
	gotLayout := obj(
		"type", data.Type,
		"layoutAlgorithm", data.LayoutAlgorithm,
		"direction", data.Direction,
		"nodeSpacing", num(data.NodeSpacing),
		"rankSpacing", num(data.RankSpacing),
		"markers", stringsJS(data.Markers),
		"diagramId", data.DiagramID,
	)
	// Extra has the JavaScript value of a property that the field cannot hold.
	withExtra(gotLayout, data.Extra)
	if s := diff("layoutData", gotLayout, rec.Get("layoutData"), false); s != "" {
		return s
	}

	state := dbState(d)
	for _, key := range state.Keys() {
		if s := diff(key, state.Get(key), rec.Get(key), false); s != "" {
			return s
		}
	}
	for _, key := range []string{"defaultStyle", "defaultInterpolate", "direction"} {
		if !state.Has(key) && rec.Has(key) {
			return fmt.Sprintf("%s: got undefined, want %s", key, show(rec.Get(key)))
		}
	}
	if direction, _ := d.GetDirection(); data.Direction != direction {
		return fmt.Sprintf("layout data direction: got %q, want %q", data.Direction, direction)
	}

	getTooltip := []any{}
	for _, n := range data.Nodes {
		if tip, ok := d.GetTooltip(n.ID); ok {
			getTooltip = append(getTooltip, tip)
		} else {
			getTooltip = append(getTooltip, js.Null)
		}
	}
	if s := diff("getTooltip", getTooltip, rec.Get("getTooltip"), false); s != "" {
		return s
	}

	// Every property of the record must have a comparison.
	for _, key := range rec.Keys() {
		if !state.Has(key) && !comparedKeys[key] {
			return fmt.Sprintf("the test does not compare the property %q of the golden file", key)
		}
	}

	callbacks := []any{}
	d.GetVertices().Each(func(id string, v *FlowVertex) {
		if !v.HaveCallback {
			return
		}
		calls := []any{}
		for _, cb := range d.Callbacks() {
			if cb.ID == id && callbackNames[cb.FunctionName] {
				calls = append(calls, obj("fn", cb.FunctionName, "args", stringsJS(cb.Args)))
			}
		}
		callbacks = append(callbacks, obj("id", id, "calls", calls))
	})
	if rec.Has("callbacks") || len(callbacks) > 0 {
		if s := diff("callbacks", callbacks, rec.Get("callbacks"), false); s != "" {
			return s
		}
	}
	return ""
}

// result is the comparison of one case of the golden file.
type result struct {
	text string
	// skipped says why the case does not test this package, or "".
	skipped string
	// problem is the first difference of the db, or "".
	problem string
	// isError is true when upstream rejects the text.
	isError bool
	// styleProblem is the difference of the style text, or "".
	styleProblem string
}

var (
	goldenOnce    sync.Once
	goldenData    *goldenFile
	goldenResults []result
)

// runGolden parses every case of the golden file, one time, and compares it
// with the dump of upstream.
func runGolden(tb testing.TB) (*goldenFile, []result) {
	tb.Helper()
	goldenOnce.Do(func() {
		g := loadGolden(tb)
		base := map[string]any{}
		flatten(g.BaseConfig, nil, base)
		texts := make([]string, len(g.Cases))
		for i := range g.Cases {
			texts[i] = caseText(tb, g.Cases[i])
		}
		results := make([]result, len(g.Cases))
		var wg sync.WaitGroup
		sem := make(chan struct{}, runtime.NumCPU())
		for i := range g.Cases {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer wg.Done()
				defer func() { <-sem }()
				results[i] = runCase(base, g.Cases[i], texts[i])
			}(i)
		}
		wg.Wait()
		goldenData, goldenResults = g, results
	})
	return goldenData, goldenResults
}

// siteConfig is the argument of mermaid.initialize in the page of the
// oracle: { startOnLoad: false, ...site }.
func siteConfig(extra *js.Obj[any]) *js.Obj[any] {
	site := js.NewObj[any]()
	site.Set("startOnLoad", false)
	if extra != nil {
		extra.Each(func(k string, v any) { site.Set(k, js.Clone(v)) })
	}
	return site
}

func runCase(base map[string]any, c *goldenCase, text string) (r result) {
	r.text = text
	defer func() {
		if p := recover(); p != nil {
			buf := make([]byte, 2048)
			buf = buf[:runtime.Stack(buf, false)]
			r.problem = fmt.Sprintf("panic: %v\n%s", p, buf)
		}
	}()
	o := renderToDraw(text, siteConfig(c.Site), !c.Fresh)
	r.isError = c.HasError
	switch {
	case o.otherType != "":
		// Another diagram type, or no diagram at all: the text does not
		// reach this package.
		switch {
		case c.rec.Get("nodes") != nil:
			r.problem = fmt.Sprintf("got the diagram type %q, want an agentflow db", o.otherType)
		case c.Other:
			r.skipped = "another diagram type draws the text"
		case !c.HasError && c.Type != o.otherType:
			r.problem = fmt.Sprintf("type: got %q, want %q", o.otherType, c.Type)
		default:
			r.skipped = "the diagram type " + o.otherType + " is not agentflow"
		}
	case c.Other:
		r.problem = "got an agentflow diagram, want another diagram type"
	case c.HasError && o.err == nil:
		r.problem = fmt.Sprintf("got no error, want %s: %q", c.ErrorName, c.Error)
	case !c.HasError && o.err != nil:
		r.problem = fmt.Sprintf("got error %q, want none", o.err)
	case c.HasError:
		name, message, hash, mark := errorInfo(o.err)
		switch {
		case message != c.Error:
			r.problem = fmt.Sprintf("error message:\n got %s: %q\nwant %s: %q", name, message, c.ErrorName, c.Error)
		case name != c.ErrorName:
			r.problem = fmt.Sprintf("error name: got %s, want %s (%q)", name, c.ErrorName, message)
		default:
			wantHash := c.rec.Get("hash")
			if h := js.AsObj(wantHash); h != nil {
				// The token of a parse error at the end of the input is the
				// number of the end symbol upstream. The driver of the
				// grammar has it as text.
				if n, isNumber := h.Get("token").(float64); isNumber {
					h.Set("token", js.Str(n))
				}
			}
			if s := diff("error.hash", hash, wantHash, false); s != "" {
				r.problem = s
			} else if s := diff("error.mark", mark, c.rec.Get("mark"), false); s != "" {
				r.problem = s
			}
		}
	default:
		r.problem = compareCase(base, c, o.cp)
		want, hasHash := c.rec.Get("styleHash").(string)
		switch {
		case !hasHash:
		case !o.cp.hasStyle:
			r.styleProblem = "no style element"
		default:
			if got := textHash(o.cp.style); got != want {
				r.styleProblem = fmt.Sprintf("checksum of the style text: got %s, want %s", got, want)
			}
		}
	}
	return r
}

// TestDBGolden parses every case of the golden file and compares the whole
// db with the db of upstream: every property of every node and edge of
// getData, their order, the connectors, the vertices, the edges, the
// containers, the classes, the tooltips, the direction, the title, the
// accessibility values, the diagnostics, the element mappings and their
// queries, the semantic model and the config after the init hook. A case
// that upstream rejects must fail here too, with the same error name,
// message, hash and mark.
func TestDBGolden(t *testing.T) {
	g, results := runGolden(t)
	var total, ok, corpus, corpusOK, skipped, errorCases int
	for i, r := range results {
		c := g.Cases[i]
		total++
		isCorpus := !c.HasText
		if isCorpus {
			corpus++
		}
		if r.skipped != "" {
			skipped++
			continue
		}
		if r.problem != "" {
			t.Errorf("%s: %s\n--- text ---\n%s", c.Name, r.problem, clip(r.text))
			continue
		}
		ok++
		if isCorpus {
			corpusOK++
		}
		if r.isError {
			errorCases++
		}
	}
	t.Logf("cases: %d of %d agree, %d are not agentflow diagrams (corpus fixtures: %d of %d agree)",
		ok, total, skipped, corpusOK, corpus)
	t.Logf("cases that upstream rejects, with the same name, message, hash and mark: %d", errorCases)
	if corpus == 0 || corpusOK != corpus {
		t.Errorf("corpus fixtures: %d of %d agree", corpusOK, corpus)
	}
}

// TestPipelineText measures what the pipeline of internal/render changes:
// it gives the cleaned text to the parser, and no front matter offset.
// Upstream gives the text with its comments to a db with
// preserveCommentsWhenParsing (Diagram.fromText). The nodes and the edges
// that the layout receives must be the same for every corpus fixture, so
// the drawing does not change. The positions of the element mappings and
// of the diagnostics differ: the test logs how many cases have another db.
func TestPipelineText(t *testing.T) {
	g, _ := runGolden(t)
	// verdict is the comparison of one case: 0 for another db or no db,
	// 1 for the nodes and edges of upstream, 2 for the whole db.
	type verdict struct {
		level   int
		problem string
	}
	verdicts := make([]verdict, len(g.Cases))
	texts := make([]string, len(g.Cases))
	for i, c := range g.Cases {
		texts[i] = caseText(t, c)
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for i, c := range g.Cases {
		if c.rec.Get("nodes") == nil {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, c *goldenCase) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if p := recover(); p != nil {
					verdicts[i] = verdict{problem: fmt.Sprintf("panic: %v", p)}
				}
			}()
			o := renderCapture(texts[i], siteConfig(c.Site), &capture{warm: !c.Fresh, pipeline: true})
			if o.cp == nil {
				verdicts[i].problem = fmt.Sprintf("the pipeline text gives no db: %v", o.err)
				return
			}
			nodes := make([]any, len(o.cp.data.Nodes))
			for k, n := range o.cp.data.Nodes {
				nodes[k] = nodeJSON(n)
			}
			edges := make([]any, len(o.cp.data.Edges))
			for k, e := range o.cp.data.Edges {
				edges[k] = edgeJSON(e)
			}
			if diff("nodes", nodes, c.rec.Get("nodes"), false) != "" || diff("edges", edges, c.rec.Get("edges"), false) != "" {
				verdicts[i].problem = "the pipeline text gives other nodes or edges"
				return
			}
			verdicts[i].level = 1
			state := dbState(o.cp.db)
			for _, key := range state.Keys() {
				if diff(key, state.Get(key), c.rec.Get(key), false) != "" {
					return
				}
			}
			verdicts[i].level = 2
		}(i, c)
	}
	wg.Wait()

	var total, sameLayout, sameDB, corpus, corpusSameLayout int
	for i, c := range g.Cases {
		if c.rec.Get("nodes") == nil {
			continue
		}
		total++
		isCorpus := !c.HasText
		if isCorpus {
			corpus++
		}
		v := verdicts[i]
		if v.level == 0 {
			if isCorpus {
				t.Errorf("%s: %s", c.Name, v.problem)
			} else {
				t.Logf("%s: %s", c.Name, v.problem)
			}
			continue
		}
		sameLayout++
		if isCorpus {
			corpusSameLayout++
		}
		if v.level == 2 {
			sameDB++
		}
	}
	t.Logf("with the text of the pipeline: %d of %d cases have the nodes and edges of upstream (corpus fixtures: %d of %d), %d of %d have the whole db",
		sameLayout, total, corpusSameLayout, corpus, sameDB, total)
}

// TestStyleGolden compares the text of the style element of each case with
// upstream, by a checksum. The text has the CSS of the class definitions of
// the diagram (the Classes hook) and the style sheet of the diagram type
// for the config of the diagram.
func TestStyleGolden(t *testing.T) {
	g, results := runGolden(t)
	var total, ok, corpus, corpusOK int
	for i, r := range results {
		c := g.Cases[i]
		if _, hasHash := c.rec.Get("styleHash").(string); !hasHash || r.skipped != "" || r.problem != "" {
			continue
		}
		total++
		isCorpus := !c.HasText
		if isCorpus {
			corpus++
		}
		if r.styleProblem != "" {
			t.Errorf("%s: %s\n--- text ---\n%s", c.Name, r.styleProblem, clip(r.text))
			continue
		}
		ok++
		if isCorpus {
			corpusOK++
		}
	}
	t.Logf("style texts: %d of %d agree (corpus fixtures: %d of %d)", ok, total, corpusOK, corpus)
	if total == 0 {
		t.Skip("the golden file has no style checksums")
	}
}

func clip(s string) string {
	if len(s) > 600 {
		return s[:600] + "..."
	}
	return s
}

// TestDestructLink compares destructLink with upstream for the link texts
// of the golden file.
func TestDestructLink(t *testing.T) {
	g := loadGolden(t)
	d := NewDB(render.NewContext(render.ContextOptions{Config: js.AsObj(g.BaseConfig)}))
	for _, l := range g.Links {
		rec := js.AsObj(l)
		args := list(rec, "args")
		end, _ := args[0].(string)
		start, _ := args[1].(string)
		info := d.DestructLink(end, start)
		got := obj(
			"type", info.Type,
			"stroke", info.Stroke,
			"length", optNum(info.Length),
			"edgeSemantic", optStr(info.EdgeSemantic),
		)
		if s := diff("", got, rec.Get("result"), false); s != "" {
			t.Errorf("destructLink(%q, %q)%s", end, start, s)
		}
	}
	if len(g.Links) == 0 {
		t.Error("no link cases")
	}
	t.Logf("%d link cases", len(g.Links))
}

// TestStripLineTrailingCommas compares stripLineTrailingCommas with
// upstream for the bodies of the golden file.
func TestStripLineTrailingCommas(t *testing.T) {
	g := loadGolden(t)
	for _, c := range g.Commas {
		rec := js.AsObj(c)
		body := str(rec, "body")
		if got, want := stripLineTrailingCommas(body), str(rec, "result"); got != want {
			t.Errorf("stripLineTrailingCommas(%q):\n got %q\nwant %q", body, got, want)
		}
	}
	if len(g.Commas) == 0 {
		t.Error("no cases")
	}
	t.Logf("%d bodies", len(g.Commas))
}

// TestStripPrototypeKeys compares stripPrototypeKeys with upstream.
func TestStripPrototypeKeys(t *testing.T) {
	g := loadGolden(t)
	for _, c := range g.Proto {
		rec := js.AsObj(c)
		text := str(rec, "text")
		v, err := js.ParseJSON([]byte(text))
		if err != nil {
			t.Fatal(err)
		}
		if s := diff("", stripPrototypeKeys(v), rec.Get("result"), true); s != "" {
			t.Errorf("stripPrototypeKeys(%s)%s", text, s)
		}
	}
	if len(g.Proto) == 0 {
		t.Error("no cases")
	}
	t.Logf("%d values", len(g.Proto))
}

// shapeSink records the diagnostics of normaliseNodeShapes.
type shapeSink struct{ diags []any }

func (s *shapeSink) emit(kind, id, message string, ctx DiagnosticContext) {
	s.diags = append(s.diags, obj("kind", kind, "id", id, "message", message, "ctx", obj("nodeId", ctx.NodeID)))
}

func (s *shapeSink) EmitError(id, message string, ctx DiagnosticContext) {
	s.emit("error", id, message, ctx)
}

func (s *shapeSink) EmitWarning(id, message string, ctx DiagnosticContext) {
	s.emit("warning", id, message, ctx)
}

// TestNormaliseNodeShapes compares normaliseNodeShapes and resolveShapeAlias
// with upstream for the shapes of the golden file.
func TestNormaliseNodeShapes(t *testing.T) {
	g := loadGolden(t)
	for _, c := range g.Shapes {
		rec := js.AsObj(c)
		in := js.AsObj(rec.Get("node"))
		shape, hasShape := in.Get("shape").(string)
		node := ru.NewNode("n")
		node.Shape = shape
		node.IsGroup = js.Truthy(in.Get("isGroup"))
		sink := &shapeSink{diags: []any{}}
		normaliseNodeShapes([]*ru.Node{node}, sink)
		var gotShape, alias any = js.Null, js.Null
		if node.Shape != "" {
			gotShape = node.Shape
		}
		if hasShape {
			alias = resolveShapeAlias(shape)
		}
		got := obj("shape", gotShape, "alias", alias, "diags", sink.diags)
		if s := diff("", got, rec.Get("result"), false); s != "" {
			t.Errorf("normaliseNodeShapes(%s)%s", show(in), s)
		}
	}
	if len(g.Shapes) == 0 {
		t.Error("no cases")
	}
	t.Logf("%d shapes", len(g.Shapes))
}

// TestContainerSlot compares containerSlotCount and containerSlot with
// upstream for palettes of 0 to 16 colours.
func TestContainerSlot(t *testing.T) {
	g := loadGolden(t)
	n := 0
	for paletteLength, r := range g.Slots {
		row := r.([]any)
		if got := float64(containerSlotCount(paletteLength)); got != row[0] {
			t.Errorf("containerSlotCount(%d) = %v, want %v", paletteLength, got, row[0])
		}
		for i, want := range row[1:] {
			if got := float64(containerSlot(i, paletteLength)); got != want {
				t.Errorf("containerSlot(%d, %d) = %v, want %v", i, paletteLength, got, want)
			}
			n++
		}
	}
	if n == 0 {
		t.Error("no cases")
	}
	t.Logf("%d slots", n)
}

func setOf(m map[string]bool) []any {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return stringsJS(keys)
}

func sorted(v any) []any {
	l, _ := v.([]any)
	out := make([]string, len(l))
	for i, e := range l {
		out[i], _ = e.(string)
	}
	sort.Strings(out)
	return stringsJS(out)
}

// TestConstants compares the tables of the package with upstream.
func TestConstants(t *testing.T) {
	g := loadGolden(t)
	c := g.Constants
	d := NewDB(render.NewContext(render.ContextOptions{Config: js.AsObj(g.BaseConfig)}))
	kindClasses := make([]any, len(kinds))
	for i, k := range kinds {
		kindClasses[i] = kindClass(k)
	}
	aliases := map[string]bool{}
	for from, to := range shapeAliases {
		aliases[from+" "+to] = true
	}
	wantAliases := map[string]bool{}
	for _, p := range list(c, "aliases") {
		pair := p.([]any)
		wantAliases[pair[0].(string)+" "+pair[1].(string)] = true
	}
	checks := []struct {
		name      string
		got, want any
	}{
		{"defaultStyle", d.DefaultStyle(), c.Get("defaultStyle")},
		{"kinds", stringsJS(kinds), c.Get("kinds")},
		{"kindCount", float64(kindCount), c.Get("kindCount")},
		{"kindClass", kindClasses, c.Get("kindClass")},
		{"warnings", stringsJS(agentflowWarnings), c.Get("warnings")},
		{"emitted", stringsJS(ParserEmittedDiagnostics), c.Get("emitted")},
		{"reserved", stringsJS(ReservedDiagnostics()), c.Get("reserved")},
		{"aliases", setOf(aliases), setOf(wantAliases)},
		{"removed", setOf(removedShapes), sorted(c.Get("removed"))},
		{"allowed", setOf(allowedShapes), sorted(c.Get("allowed"))},
		{"defaultShape", defaultShape, c.Get("defaultShape")},
		{"skipKeys", setOf(semanticMetadataSkipKeys), sorted(c.Get("skipKeys"))},
		{"subroutineAliases", setOf(subroutineAliases), sorted(c.Get("subroutineAliases"))},
		{"domIdPrefix", mermaidDomIDPrefix, c.Get("domIdPrefix")},
	}
	for _, check := range checks {
		if s := diff(check.name, check.got, check.want, true); s != "" {
			t.Error(s)
		}
	}
}

// TestPrepareText checks the text preparation of agentflowParser.ts with
// cases that follow from the regular expression /}[^\S\n]*\n/g.
func TestPrepareText(t *testing.T) {
	cases := [][2]string{
		{"a}\nb", "a}\nb"},
		{"a}  \nb", "a}\nb"},
		{"a} \t\n\n  \n  b", "a}\n\n  \n  b"},
		{"a} b\n", "a} b\n"},
		{"a}", "a}"},
		{"a}  ", "a}  "},
		{"}}\n", "}}\n"},
		{"}\n}\n", "}\n}\n"},
		{"} \n} \n", "}\n}\n"},
		{"}  \n x", "}\n x"},
		{"}\r\nx", "}\nx"},
		{"}" + string([]rune{0xA0, 0x2028, 0xFEFF}) + "\nx", "}\nx"},
		{"} } \n", "} }\n"},
		{"no brace \n", "no brace \n"},
	}
	for _, c := range cases {
		if got := prepareText(c[0]); got != c[1] {
			t.Errorf("prepareText(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}
