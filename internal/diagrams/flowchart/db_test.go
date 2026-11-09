package flowchart

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
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
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The golden file is the db of real mermaid at the moment when the renderer
// draws: tools/reference/gen-flowchart-db.mjs writes it with the browser
// oracle. The tests parse the same texts and compare everything.

type goldenFile struct {
	BaseConfig   map[string]any `json:"baseConfig"`
	Cases        []goldenCase   `json:"cases"`
	Links        []goldenLink   `json:"links"`
	DefaultStyle string         `json:"defaultStyle"`
}

type goldenLink struct {
	Args   []*string      `json:"args"`
	Result map[string]any `json:"result"`
}

type goldenCase struct {
	Name string `json:"name"`
	// Text is absent for a corpus fixture: Name is then its path.
	Text *string `json:"text"`
	// Fresh is true for a case that ran in a new page.
	Fresh bool `json:"fresh"`
	// Site is the site config: the argument of mermaid.initialize.
	Site      map[string]any `json:"site"`
	Error     *string        `json:"error"`
	ErrorName string         `json:"errorName"`
	Type      string         `json:"type"`

	Nodes       []any `json:"nodes"`
	Edges       []any `json:"edges"`
	ConfigDelta *struct {
		Set [][]any    `json:"set"`
		Del [][]string `json:"del"`
	} `json:"configDelta"`
	// LayoutData has the properties that the draw function adds to the
	// layout data: type, layoutAlgorithm, direction, nodeSpacing,
	// rankSpacing, markers and diagramId.
	LayoutData         map[string]any `json:"layoutData"`
	Classes            []any          `json:"classes"`
	SubGraphs          []any          `json:"subGraphs"`
	Vertices           []any          `json:"vertices"`
	RawEdges           []any          `json:"rawEdges"`
	Tooltips           []any          `json:"tooltips"`
	Direction          *string        `json:"direction"`
	Title              string         `json:"title"`
	AccTitle           string         `json:"accTitle"`
	AccDescription     string         `json:"accDescription"`
	DomIDs             []any          `json:"domIds"`
	DefaultStyle       any            `json:"defaultStyle"`
	DefaultInterpolate any            `json:"defaultInterpolate"`
	GetTooltip         []any          `json:"getTooltip"`
	DepthFirstPos      []any          `json:"depthFirstPos"`
	// StyleHash is the checksum of the text of the style element.
	StyleHash *string `json:"styleHash"`
	// Callbacks has, for each node with a callback, the calls that a click
	// on its element makes. It is absent when no node has a callback.
	Callbacks []any `json:"callbacks"`
}

// callbackNames are the functions that the page of the oracle defines. A
// click callback with another name fails there and records no call.
var callbackNames = map[string]bool{"callback": true, "noargs": true, "two": true, "cb": true, "other": true, "a.b.c": true}

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
	var g goldenFile
	if err := json.Unmarshal(data, &g); err != nil {
		tb.Fatal(err)
	}
	return &g
}

func caseText(tb testing.TB, c *goldenCase) string {
	tb.Helper()
	if c.Text != nil {
		return *c.Text
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
	// warm says that the page has rendered a flowchart before, so the
	// DOMPurify hooks of mermaid are installed.
	warm bool
	ctx  *render.Context
	db   *DB
	// data is the layout data, after ru.Render has added the id of the
	// diagram to the DOM ids.
	data *ru.LayoutData
	// style is the text of the style element of the diagram.
	style    string
	hasStyle bool
}

var (
	// captures has the capture of each render that is under test, by the
	// time zone of the render: a pointer that only that render has.
	captures    sync.Map
	captureOnce sync.Once
	errCaptured = errors.New("flowchart test: the db is captured")
)

// layoutNames are the layouts of the full build of mermaid.
var layoutNames = []string{"dagre", "swimlane", "cose-bilkent", "elk", "elk.stress", "elk.force", "elk.mrtree", "elk.sporeOverlap", "elk.box", "elk.rectpacking"}

// installCapture changes the test binary so that a render that is under
// test stops when the draw function hands the layout data to the layout
// algorithm, as the oracle script does with upstream: every layout becomes
// a layout that captures the data, and the diagram types get a draw
// function that runs the steps of draw up to ru.Render.
func installCapture() {
	captureOnce.Do(func() {
		for _, name := range layoutNames {
			ru.RegisterLayout(name, "", func(ctx *render.Context, data *ru.LayoutData, _ *dom.Node, _ string) error {
				c, ok := captures.Load(ctx.Location)
				if !ok {
					return &ru.NotPortedError{Kind: "layout", Name: data.LayoutAlgorithm}
				}
				c.(*capture).data = data
				return errCaptured
			})
		}
		for _, id := range typeIDs {
			def := NewDefinition(id)
			newDB, draw := def.New, def.Draw
			def.New = func(ctx *render.Context) render.DB {
				if c, ok := captures.Load(ctx.Location); ok && c.(*capture).warm {
					ctx.Sanitizer.RemoveScript("")
				}
				return newDB(ctx)
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
				// The first steps of draw. The check of the labels is left
				// out: upstream fails for such a label in the layout.
				return ru.Render(ctx, layoutData(ctx, cp.db), ctx.GetDiagramElement())
			}
			render.Register(def)
		}
	})
}

// outcome is the result of the steps of mermaid.render up to the draw
// function of the diagram.
type outcome struct {
	// cp is set when the diagram is a flowchart and the parser accepted it.
	cp *capture
	// otherType is the diagram type when it is not a flowchart.
	otherType string
	// err is the error of the config, of the type detection or of the parser.
	err error
}

// renderToDraw runs render.Render for a text until the draw function of the
// flowchart. site is the site config: the argument of mermaid.initialize.
func renderToDraw(text string, site *js.Obj[any], warm bool) outcome {
	installCapture()
	zone := time.FixedZone("flowchart-test", 0)
	cp := &capture{warm: warm}
	captures.Store(zone, cp)
	defer captures.Delete(zone)
	_, err := render.Render(text, render.Options{Config: site, Location: zone})
	var unsupported *render.UnsupportedError
	switch {
	case errors.Is(err, errCaptured):
		return outcome{cp: cp}
	case errors.As(err, &unsupported):
		return outcome{otherType: unsupported.Type}
	case err == nil:
		return outcome{err: errors.New("the render did not reach the draw function of a flowchart")}
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

// errorMessage is error.message of the JavaScript exception.
func errorMessage(err error) (name, message string) {
	var ce *config.Error
	if errors.As(err, &ce) {
		return ce.Name, ce.Message
	}
	var fe *Error
	if errors.As(err, &fe) {
		return fe.Name, fe.Message
	}
	var je *jison.Error
	if errors.As(err, &je) {
		return je.Name, je.Message
	}
	return "Error", err.Error()
}

// ---- conversion of the Go side to JSON data ----

// generic converts a dynamic JavaScript value to the value that
// encoding/json gives for JSON.stringify of it: an undefined property is
// absent, undefined in an array is null, and a number that is not finite is
// { $num: "NaN" }, as the oracle writes it.
func generic(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case bool, string:
		return x
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return map[string]any{"$num": js.Str(x)}
		}
		return x
	case int:
		return float64(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = generic(e)
		}
		return out
	case []string:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = e
		}
		return out
	case *js.Obj[any]:
		if x == nil {
			return nil
		}
		out := map[string]any{}
		x.Each(func(k string, e any) {
			if e != nil {
				out[k] = generic(e)
			}
		})
		return out
	}
	if js.IsNullish(v) {
		return nil
	}
	panic(fmt.Sprintf("generic: unexpected %T", v))
}

func stringsJSON(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

// vertexJSON is JSON.stringify of a FlowVertex.
func vertexJSON(v *FlowVertex) map[string]any {
	out := map[string]any{
		"id":        v.ID,
		"labelType": v.LabelType,
		"domId":     v.DomID,
		"styles":    stringsJSON(v.Styles),
		"classes":   stringsJSON(v.Classes),
	}
	dyn := func(key string, value any) {
		if value != nil {
			out[key] = generic(value)
		}
	}
	dyn("text", v.Text)
	if v.HasType {
		out["type"] = v.Type
	}
	if v.HasDir {
		out["dir"] = v.Dir
	}
	if v.Props != nil {
		out["props"] = generic(v.Props)
	}
	if v.HaveCallback {
		out["haveCallback"] = true
	}
	if v.HasLink {
		out["link"] = v.Link
	}
	if v.HasLinkTarget {
		out["linkTarget"] = v.LinkTarget
	}
	dyn("icon", v.Icon)
	dyn("form", v.Form)
	dyn("pos", v.Pos)
	dyn("img", v.Img)
	dyn("constraint", v.Constraint)
	if v.HasAssetWidth {
		out["assetWidth"] = generic(v.AssetWidth)
	}
	if v.HasAssetHeight {
		out["assetHeight"] = generic(v.AssetHeight)
	}
	return out
}

// edgeJSON is JSON.stringify of a FlowEdge.
func edgeJSON(e *FlowEdge) map[string]any {
	out := map[string]any{
		"start":           e.Start,
		"end":             e.End,
		"type":            e.Type,
		"text":            e.Text,
		"labelType":       e.LabelType,
		"classes":         stringsJSON(e.Classes),
		"isUserDefinedId": e.IsUserDefinedID,
		"stroke":          e.Stroke,
		"id":              e.ID,
	}
	if e.Interpolate != nil {
		out["interpolate"] = generic(e.Interpolate)
	}
	if !math.IsNaN(e.Length) {
		out["length"] = e.Length
	}
	if e.Style != nil {
		out["style"] = stringsJSON(e.Style)
	}
	if e.Animate != nil {
		out["animate"] = generic(e.Animate)
	}
	if e.Animation != nil {
		out["animation"] = generic(e.Animation)
	}
	return out
}

// subGraphJSON is JSON.stringify of a FlowSubGraph.
func subGraphJSON(sg *FlowSubGraph) map[string]any {
	out := map[string]any{
		"id":        sg.ID,
		"nodes":     stringsJSON(sg.Nodes),
		"title":     sg.Title,
		"classes":   stringsJSON(sg.Classes),
		"labelType": sg.LabelType,
	}
	if sg.HasDir {
		out["dir"] = sg.Dir
	}
	if sg.Metadata != nil {
		out["metadata"] = generic(sg.Metadata)
	}
	return out
}

// dropUndefinedLike removes the properties of a node or an edge of
// upstream that the layout model of this port cannot tell from undefined:
// an empty string and false. The properties in keep have a field that says
// whether they are set. See NodeJSON.
func dropUndefinedLike(list []any, keep ...string) []any {
	out := make([]any, len(list))
	for i, item := range list {
		obj, ok := item.(map[string]any)
		if !ok {
			out[i] = item
			continue
		}
		clean := map[string]any{}
	next:
		for k, v := range obj {
			if v == "" || v == false {
				for _, name := range keep {
					if k == name {
						clean[k] = v
						continue next
					}
				}
				continue
			}
			clean[k] = v
		}
		out[i] = clean
	}
	return out
}

// ---- comparison ----

// diff returns a description of the first difference between two JSON
// values, or "".
func diff(path string, got, want any) string {
	if reflect.DeepEqual(got, want) {
		return ""
	}
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			break
		}
		keys := map[string]bool{}
		for k := range w {
			keys[k] = true
		}
		for k := range g {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			gv, gok := g[k]
			wv, wok := w[k]
			switch {
			case !gok:
				return fmt.Sprintf("%s.%s: missing, want %s", path, k, show(wv))
			case !wok:
				return fmt.Sprintf("%s.%s: got %s, want it absent", path, k, show(gv))
			}
			if d := diff(path+"."+k, gv, wv); d != "" {
				return d
			}
		}
	case []any:
		g, ok := got.([]any)
		if !ok {
			break
		}
		for i := 0; i < len(g) && i < len(w); i++ {
			if d := diff(path+"["+strconv.Itoa(i)+"]", g[i], w[i]); d != "" {
				return d
			}
		}
		if len(g) != len(w) {
			return fmt.Sprintf("%s: length %d, want %d", path, len(g), len(w))
		}
	}
	return fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want))
}

func show(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	if len(b) > 300 {
		return string(b[:300]) + "..."
	}
	return string(b)
}

// flatten is the flatten function of the generator: the leaves of a config
// by path. An object with no property is a leaf.
func flatten(v any, prefix []string, out map[string]any) {
	if obj, ok := v.(map[string]any); ok && len(obj) > 0 {
		for k, e := range obj {
			flatten(e, append(append([]string{}, prefix...), k), out)
		}
		return
	}
	key, _ := json.Marshal(prefix)
	out[string(key)] = v
}

// wantConfig returns the leaves of the config of a case: the base config
// with the delta of the case.
func wantConfig(base map[string]any, c *goldenCase) map[string]any {
	out := make(map[string]any, len(base))
	for k, v := range base {
		out[k] = v
	}
	for _, path := range c.ConfigDelta.Del {
		key, _ := json.Marshal(path)
		delete(out, string(key))
	}
	for _, entry := range c.ConfigDelta.Set {
		key, _ := json.Marshal(entry[0])
		out[string(key)] = entry[1]
	}
	return out
}

func toObj(m map[string]any) *js.Obj[any] {
	if m == nil {
		return nil
	}
	data, _ := json.Marshal(m)
	v, err := js.ParseJSON(data)
	if err != nil {
		panic(err)
	}
	return js.AsObj(v)
}

// compareCase compares the db of this port with the dump of upstream. It
// returns the first difference, or "".
func compareCase(base map[string]any, c *goldenCase, cp *capture) string {
	if cp.ctx.Type != c.Type {
		return fmt.Sprintf("type: got %q, want %q", cp.ctx.Type, c.Type)
	}
	if c.Nodes == nil {
		return "got a flowchart db, want none"
	}
	d := cp.db
	data := cp.data

	nodes := make([]any, len(data.Nodes))
	for i, n := range data.Nodes {
		nodes[i] = generic(NodeJSON(n))
	}
	if s := diff("nodes", nodes, dropUndefinedLike(c.Nodes, "label", "cssClasses")); s != "" {
		return s
	}
	edges := make([]any, len(data.Edges))
	for i, e := range data.Edges {
		edges[i] = generic(EdgeJSON(e))
	}
	if s := diff("edges", edges, dropUndefinedLike(c.Edges, "label", "animate")); s != "" {
		return s
	}

	gotConfig := map[string]any{}
	flatten(generic(data.Config), nil, gotConfig)
	if s := diff("config", gotConfig, wantConfig(base, c)); s != "" {
		return s
	}
	gotLayout := map[string]any{
		"type":            data.Type,
		"layoutAlgorithm": data.LayoutAlgorithm,
		"direction":       data.Direction,
		"nodeSpacing":     generic(data.NodeSpacing),
		"rankSpacing":     generic(data.RankSpacing),
		"markers":         stringsJSON(data.Markers),
		"diagramId":       data.DiagramID,
	}
	// Extra has the JavaScript value of a property that the field cannot hold.
	for k, v := range data.Extra {
		gotLayout[k] = generic(v)
	}
	if s := diff("layoutData", gotLayout, any(c.LayoutData)); s != "" {
		return s
	}

	classes := []any{}
	d.GetClasses().Each(func(id string, def *render.ClassDef) {
		classes = append(classes, []any{id, map[string]any{
			"id": def.ID, "styles": stringsJSON(def.Styles), "textStyles": stringsJSON(def.TextStyles),
		}})
	})
	if s := diff("classes", classes, c.Classes); s != "" {
		return s
	}

	subGraphs := []any{}
	for _, sg := range d.GetSubGraphs() {
		subGraphs = append(subGraphs, subGraphJSON(sg))
	}
	if s := diff("subGraphs", subGraphs, c.SubGraphs); s != "" {
		return s
	}

	vertices := []any{}
	domIDs := []any{}
	d.GetVertices().Each(func(id string, v *FlowVertex) {
		vertices = append(vertices, []any{id, vertexJSON(v)})
		domIDs = append(domIDs, d.LookUpDomId(id))
	})
	if s := diff("vertices", vertices, c.Vertices); s != "" {
		return s
	}
	if s := diff("domIds", domIDs, c.DomIDs); s != "" {
		return s
	}

	rawEdges := []any{}
	for _, e := range d.GetEdges() {
		rawEdges = append(rawEdges, edgeJSON(e))
	}
	if s := diff("rawEdges", rawEdges, c.RawEdges); s != "" {
		return s
	}
	defaultStyle, defaultInterpolate, hasInterpolate := d.DefaultLinkStyle()
	var gotStyle, gotInterpolate any
	if defaultStyle != nil {
		gotStyle = stringsJSON(defaultStyle)
	}
	if hasInterpolate {
		gotInterpolate = defaultInterpolate
	}
	if s := diff("defaultStyle", gotStyle, c.DefaultStyle); s != "" {
		return s
	}
	if s := diff("defaultInterpolate", gotInterpolate, c.DefaultInterpolate); s != "" {
		return s
	}

	tooltips := []any{}
	d.Tooltips().Each(func(id, tip string) { tooltips = append(tooltips, []any{id, tip}) })
	if s := diff("tooltips", tooltips, c.Tooltips); s != "" {
		return s
	}
	getTooltip := []any{}
	for _, n := range data.Nodes {
		if tip, ok := d.GetTooltip(n.ID); ok {
			getTooltip = append(getTooltip, tip)
		} else {
			getTooltip = append(getTooltip, nil)
		}
	}
	if s := diff("getTooltip", getTooltip, c.GetTooltip); s != "" {
		return s
	}

	direction, hasDirection := d.GetDirection()
	switch {
	case c.Direction == nil && hasDirection:
		return fmt.Sprintf("direction: got %q, want undefined", direction)
	case c.Direction != nil && (!hasDirection || direction != *c.Direction):
		return fmt.Sprintf("direction: got %q, want %q", direction, *c.Direction)
	}
	if data.Direction != direction {
		return fmt.Sprintf("layout data direction: got %q, want %q", data.Direction, direction)
	}
	if got := d.GetDiagramTitle(); got != c.Title {
		return fmt.Sprintf("title: got %q, want %q", got, c.Title)
	}
	if got := d.GetAccTitle(); got != c.AccTitle {
		return fmt.Sprintf("accTitle: got %q, want %q", got, c.AccTitle)
	}
	if got := d.GetAccDescription(); got != c.AccDescription {
		return fmt.Sprintf("accDescription: got %q, want %q", got, c.AccDescription)
	}

	callbacks := []any{}
	d.GetVertices().Each(func(id string, v *FlowVertex) {
		if !v.HaveCallback {
			return
		}
		calls := []any{}
		for _, cb := range d.Callbacks() {
			if cb.ID == id && callbackNames[cb.FunctionName] {
				calls = append(calls, map[string]any{"fn": cb.FunctionName, "args": stringsJSON(cb.Args)})
			}
		}
		callbacks = append(callbacks, map[string]any{"id": id, "calls": calls})
	})
	if c.Callbacks != nil || len(callbacks) > 0 {
		if s := diff("callbacks", callbacks, c.Callbacks); s != "" {
			return s
		}
	}

	d.IndexNodes()
	depthFirstPos := []any{}
	for i := range d.GetSubGraphs() {
		if pos, ok := d.GetDepthFirstPos(i); ok {
			depthFirstPos = append(depthFirstPos, float64(pos))
		} else {
			depthFirstPos = append(depthFirstPos, nil)
		}
	}
	if s := diff("depthFirstPos", depthFirstPos, c.DepthFirstPos); s != "" {
		return s
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
	// isError is true when upstream rejects the text, and sameMessage when
	// this port has the same message.
	isError     bool
	sameMessage bool
	// styleProblem is the difference of the style text, or "".
	styleProblem string
}

var (
	goldenOnce    sync.Once
	goldenData    *goldenFile
	goldenResults []result
)

// knownDivergences are the texts of the golden file for which this port
// has another result than upstream, each with the cause. The test fails
// when such a text agrees: remove it then.
var knownDivergences = map[string]string{
	"flowchart TD\nA@{ : }\n": "the YAML parser of internal/config rejects a flow mapping with an empty key; js-yaml reads { null: null }",
}

// runGolden parses every case of the golden file, one time, and compares it
// with the dump of upstream.
func runGolden(tb testing.TB) (*goldenFile, []result) {
	tb.Helper()
	goldenOnce.Do(func() {
		g := loadGolden(tb)
		base := map[string]any{}
		flatten(any(g.BaseConfig), nil, base)
		texts := make([]string, len(g.Cases))
		for i := range g.Cases {
			texts[i] = caseText(tb, &g.Cases[i])
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
				results[i] = runCase(base, &g.Cases[i], texts[i])
			}(i)
		}
		wg.Wait()
		goldenData, goldenResults = g, results
	})
	return goldenData, goldenResults
}

func runCase(base map[string]any, c *goldenCase, text string) (r result) {
	r.text = text
	defer func() {
		if p := recover(); p != nil {
			r.problem = fmt.Sprintf("panic: %v", p)
		}
	}()
	// The page of the oracle calls mermaid.initialize({ startOnLoad: false, ...site }).
	site := js.NewObj[any]()
	site.Set("startOnLoad", false)
	if extra := toObj(c.Site); extra != nil {
		extra.Each(func(k string, v any) { site.Set(k, v) })
	}
	o := renderToDraw(text, site, !c.Fresh)
	r.isError = c.Error != nil
	switch {
	case o.otherType != "":
		// Another diagram type, or no diagram at all: the text does not
		// reach this package.
		switch {
		case c.Nodes != nil:
			r.problem = fmt.Sprintf("got the diagram type %q, want a flowchart db", o.otherType)
		case c.Error == nil && c.Type != o.otherType:
			r.problem = fmt.Sprintf("type: got %q, want %q", o.otherType, c.Type)
		default:
			r.skipped = "the diagram type " + o.otherType + " is not a flowchart"
		}
	case c.Error != nil && o.err == nil:
		r.problem = fmt.Sprintf("got no error, want %s: %q", c.ErrorName, *c.Error)
	case c.Error == nil && o.err != nil:
		r.problem = fmt.Sprintf("got error %q, want none", o.err)
	case c.Error != nil:
		name, message := errorMessage(o.err)
		switch {
		case message == *c.Error:
			r.sameMessage = true
		case c.ErrorName == "YAMLException" && name == "YAMLException":
			// The YAML parser of this port has its own messages.
		default:
			r.problem = fmt.Sprintf("error message:\n got %s: %q\nwant %s: %q", name, message, c.ErrorName, *c.Error)
		}
	default:
		r.problem = compareCase(base, c, o.cp)
		switch {
		case c.StyleHash == nil:
		case !o.cp.hasStyle:
			r.styleProblem = "no style element"
		default:
			if got := textHash(o.cp.style); got != *c.StyleHash {
				r.styleProblem = fmt.Sprintf("checksum of the style text: got %s, want %s%s", got, *c.StyleHash, refStyleDiff(c, o.cp.style))
			}
		}
	}
	return r
}

// refStyleDiff describes the first difference between a style text and the
// style of the reference SVG of a corpus fixture, when the reference
// directory is present. The reference directory is not in the repository.
func refStyleDiff(c *goldenCase, got string) string {
	if c.Text != nil {
		return ""
	}
	rel := strings.TrimPrefix(c.Name, "testdata/corpus/")
	data, err := os.ReadFile(filepath.Join("..", "..", "..", ".parity", "ref", filepath.FromSlash(strings.TrimSuffix(rel, ".mmd")+".svg")))
	if err != nil {
		return ""
	}
	svg := string(data)
	start := strings.Index(svg, "<style>")
	end := strings.Index(svg, "</style>")
	if start < 0 || end < start {
		return ""
	}
	want := svg[start+len("<style>") : end]
	i := 0
	for i < len(got) && i < len(want) && got[i] == want[i] {
		i++
	}
	if i == len(got) && i == len(want) {
		return "\n(the serialized style of the reference SVG is equal)"
	}
	from := max(0, i-80)
	return fmt.Sprintf("\nfirst difference with the reference SVG at byte %d:\n got ...%s\nwant ...%s", i, clip(got[from:min(len(got), i+120)]), clip(want[from:min(len(want), i+120)]))
}

// TestDBGolden parses every case of the golden file and compares the whole
// db with the db of upstream: every property of every node and edge of
// getData, their order, the vertices, the edges, the subgraphs, the
// classes, the tooltips, the direction, the title, the accessibility values
// and the config after the init hook. A case that upstream rejects must
// fail here too, with the same message.
func TestDBGolden(t *testing.T) {
	g, results := runGolden(t)
	var total, ok, corpus, corpusOK, skipped, errorCases, sameMessage, known int
	for i, r := range results {
		c := &g.Cases[i]
		total++
		isCorpus := c.Text == nil
		if isCorpus {
			corpus++
		}
		if r.skipped != "" {
			skipped++
			continue
		}
		if reason, isKnown := knownDivergences[r.text]; isKnown {
			known++
			if r.problem == "" {
				t.Errorf("%s: agrees now. Remove it from knownDivergences (%s)", c.Name, reason)
			}
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
			if r.sameMessage {
				sameMessage++
			}
		}
	}
	t.Logf("cases: %d of %d agree, %d known divergences, %d are not flowcharts (corpus fixtures: %d of %d agree)",
		ok, total, known, skipped, corpusOK, corpus)
	t.Logf("cases that upstream rejects: %d, with the same message: %d (the others are YAML exceptions)", errorCases, sameMessage)
	if corpus == 0 || corpusOK != corpus {
		t.Errorf("corpus fixtures: %d of %d agree", corpusOK, corpus)
	}
}

// TestStyleGolden compares the text of the style element of each case with
// upstream, by a checksum. The text has the CSS of the class definitions of
// the diagram (the Classes hook) and the style sheet of the diagram type
// for the config of the diagram.
func TestStyleGolden(t *testing.T) {
	g, results := runGolden(t)
	var total, ok, corpus, corpusOK int
	for i, r := range results {
		c := &g.Cases[i]
		if c.StyleHash == nil || r.skipped != "" || r.problem != "" {
			continue
		}
		total++
		isCorpus := c.Text == nil
		if isCorpus {
			corpus++
		}
		if r.styleProblem != "" {
			t.Errorf("%s: %s", c.Name, r.styleProblem)
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
	d := NewDB(render.NewContext(render.ContextOptions{Config: toObj(g.BaseConfig)}))
	for _, l := range g.Links {
		str, start := "", ""
		if l.Args[0] != nil {
			str = *l.Args[0]
		}
		if l.Args[1] != nil {
			start = *l.Args[1]
		}
		info := d.DestructLink(str, start)
		got := map[string]any{"type": info.Type, "stroke": info.Stroke}
		if !math.IsNaN(info.Length) {
			got["length"] = info.Length
		}
		if s := diff("", got, any(l.Result)); s != "" {
			t.Errorf("destructLink(%q, %q)%s", str, start, s)
		}
	}
	if got := d.DefaultStyle(); got != g.DefaultStyle {
		t.Errorf("defaultStyle: got %q, want %q", got, g.DefaultStyle)
	}
	if len(g.Links) == 0 {
		t.Error("no link cases")
	}
}

// TestPrepareText checks the text preparation of flowParser.ts with cases
// that follow from the regular expression /}\s*\n/g.
func TestPrepareText(t *testing.T) {
	cases := [][2]string{
		{"a}\nb", "a}\nb"},
		{"a}  \nb", "a}\nb"},
		{"a} \t\n\n  \n  b", "a}\n  b"},
		{"a} b\n", "a} b\n"},
		{"a}", "a}"},
		{"a}  ", "a}  "},
		{"}}\n", "}}\n"},
		{"}\n}\n", "}\n}\n"},
		{"} \n} \n", "}\n}\n"},
		{"}  \n x", "}\n x"},
		{"no brace \n", "no brace \n"},
	}
	for _, c := range cases {
		if got := prepareText(c[0]); got != c[1] {
			t.Errorf("prepareText(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

// TestNodeJSON checks that the JSON conversion prints only the properties
// that are set.
func TestNodeJSON(t *testing.T) {
	n := ru.NewNode("a")
	if got := js.StringifyJSON(NodeJSON(n)); got != `{"id":"a"}` {
		t.Errorf("empty node: %s", got)
	}
	e := ru.NewEdge("e")
	if got := js.StringifyJSON(EdgeJSON(e)); got != `{"id":"e"}` {
		t.Errorf("empty edge: %s", got)
	}
	n.Label, n.HasLabel = "", true
	n.CSSStylesSet = true
	n.Padding = 8
	n.IsGroup = true
	got := js.StringifyJSON(NodeJSON(n))
	want := `{"id":"a","label":"","cssStyles":[],"padding":8,"isGroup":true}`
	if got != want {
		t.Errorf("node: got %s, want %s", got, want)
	}
	if !strings.Contains(js.StringifyJSON(EdgeJSON(&ru.Edge{ID: "e", HasAnimate: true, Length: math.NaN(), Minlen: 2, Width: math.NaN(), Height: math.NaN(), X: math.NaN(), Y: math.NaN(), Depth: math.NaN()})), `"animate":false`) {
		t.Error("edge: animate is missing")
	}
}
