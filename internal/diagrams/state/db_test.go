package state

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The golden file is the db of real mermaid after the parse and at the
// moment when the renderer draws: tools/reference/gen-state-db.mjs writes
// it with the browser oracle. The tests parse the same texts and compare
// everything, with the order of the properties.
//
// The file is read with js.ParseJSON, which keeps the order of the
// properties. A case is an object with these properties:
//
//	name          the path of a corpus fixture, or the name of another case
//	text          the text, absent for a corpus fixture
//	fresh         true for a case that ran in a new page
//	site          the site config: the argument of mermaid.initialize
//	idCounter     the counter of generateId before the render
//	error         the message when mermaid rejects the text
//	type          the diagram type
//	parsed        the db after the parse (see snapshot)
//	drawn         the db after the first steps of the draw function
//	nodes, edges  getData().nodes and getData().edges
//	configDelta   the config, as the difference from baseConfig
//	dataDirection, layout, nodeSpacing, rankSpacing, markers
//	              the other properties of the layout data
//	stateConfig   db.getConfig()
//	api           { version, ops }: calls of the functions of a new db
//	apiResults    for each call the value or the error, and the snapshot
//	              of the db after the call

func loadGolden(tb testing.TB) *js.Obj[any] {
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
	return js.AsObj(v)
}

func caseText(tb testing.TB, c *js.Obj[any]) string {
	tb.Helper()
	if text, ok := c.Get("text").(string); ok {
		return text
	}
	name, _ := c.Get("name").(string)
	data, err := os.ReadFile(filepath.Join("..", "..", "..", filepath.FromSlash(name)))
	if err != nil {
		tb.Fatal(err)
	}
	return string(data)
}

// parsed is the result of the steps of mermaid.render up to the draw
// function of the diagram.
type parsed struct {
	ctx *render.Context
	// typ is the diagram type.
	typ string
	// db is nil when the diagram type is not a state diagram.
	db *DB
}

const maxTextExceeded = "graph TB;a[Maximum text size in diagram exceeded];style a fill:#faa"

// parseText runs the steps of render.Render before the draw function:
// mermaid.initialize(site), processAndSetConfigs, Diagram.fromText. warm
// says that the page has rendered a state diagram before, so the DOMPurify
// hooks of mermaid are installed. idCounter is the counter of generateId
// before the render.
func parseText(text string, site *js.Obj[any], warm bool, idCounter int) (*parsed, error) {
	resolved, err := config.Resolve(text, site)
	if err != nil {
		return nil, err
	}
	cfg := resolved.Config
	code := resolved.Source
	if v, ok := cfg.Get("maxTextSize").(float64); ok && float64(js.UTF16Len(code.Cleaned)) > v {
		code = config.DiagramCode{Raw: maxTextExceeded, Cleaned: maxTextExceeded}
	}
	// The page makes one draw from Math.random before the render.
	ctx := render.NewContext(render.ContextOptions{Config: cfg, ConfigState: resolved.State, RandomSkip: 1})
	ctx.Title = resolved.Title
	for i := 0; i < idCounter; i++ {
		ctx.Counter("utils.generateId")
	}
	if warm {
		ctx.Sanitizer.RemoveScript("")
	}
	diagramType, err := config.DetectType(code.Cleaned, cfg)
	if err != nil {
		return nil, err
	}
	ctx.Type = diagramType
	p := &parsed{ctx: ctx, typ: diagramType}
	def, ok := render.Lookup(diagramType)
	if !ok {
		return p, nil
	}
	db := def.New(ctx)
	if def.Init != nil {
		def.Init(ctx, db)
		ctx.Config = ctx.ConfigState.GetConfig()
	}
	if resolved.Title != "" {
		if ts, ok := db.(render.TitleSetter); ok {
			ts.SetDiagramTitle(resolved.Title)
		}
	}
	if err := def.Parse(ctx, db, render.EncodeEntities(code.Cleaned)+"\n"); err != nil {
		return nil, err
	}
	p.db, _ = db.(*DB)
	return p, nil
}

// errorMessage is error.message of the JavaScript exception.
func errorMessage(err error) string {
	var ce *config.Error
	if errors.As(err, &ce) {
		return ce.Message
	}
	var se *Error
	if errors.As(err, &se) {
		return se.Message
	}
	var je *jison.Error
	if errors.As(err, &je) {
		return je.Message
	}
	return err.Error()
}

// ---- conversion of the Go side to JavaScript values ----

// stateJSON is a State as the JavaScript object that addState creates.
func stateJSON(st *State) *js.Obj[any] {
	return newObj(
		"stmt", st.Stmt,
		"id", st.ID,
		"descriptions", anyStrings(st.Descriptions),
		"type", st.Type,
		"doc", st.Doc,
		"note", noteValue(st.Note),
		"classes", anyStrings(st.Classes),
		"styles", anyStrings(st.Styles),
		"textStyles", anyStrings(st.TextStyles),
	)
}

// noteValue is the note of a state: undefined for no note.
func noteValue(note *js.Obj[any]) any {
	if note == nil {
		return nil
	}
	return note
}

// snapshot is the state of the db, as the snapshot function of
// tools/reference/state-db-page.js builds it.
func snapshot(d *DB) *js.Obj[any] {
	states := []any{}
	d.GetStates().Each(func(id string, st *State) {
		states = append(states, []any{id, stateJSON(st)})
	})
	relations := []any{}
	for _, r := range d.GetRelations() {
		o := newObj("id1", r.ID1, "id2", r.ID2)
		if r.HasRelationTitle {
			o.Set("relationTitle", r.RelationTitle)
		}
		relations = append(relations, o)
	}
	classes := []any{}
	d.GetClasses().Each(func(id string, def *render.ClassDef) {
		classes = append(classes, []any{id, newObj(
			"id", def.ID,
			"styles", anyStrings(def.Styles),
			"textStyles", anyStrings(def.TextStyles),
		)})
	})
	links := []any{}
	d.GetLinks().Each(func(key any, link *Link) {
		links = append(links, []any{key, newObj("url", link.URL, "tooltip", link.Tooltip)})
	})
	return newObj(
		"rootDoc", d.rootDoc,
		"states", states,
		"relations", relations,
		"classes", classes,
		"links", links,
		"direction", d.GetDirection(),
		"title", d.GetDiagramTitle(),
		"accTitle", d.GetAccTitle(),
		"accDescription", d.GetAccDescription(),
		"dividerCnt", float64(d.dividerCnt),
		"startEndCount", float64(d.startEndCount),
	)
}

func objList(list []*js.Obj[any]) []any {
	out := make([]any, len(list))
	for i, o := range list {
		out[i] = o
	}
	return out
}

// ---- comparison ----

// diff returns a description of the first difference between two
// JavaScript values, with the order of the properties, or "". got is first
// copied as JSON.stringify sees it: a property that is undefined is absent.
func diff(path string, got, want any) string {
	return diffValues(path, clone(got), want)
}

func diffValues(path string, got, want any) string {
	switch w := want.(type) {
	case *js.Obj[any]:
		g, ok := got.(*js.Obj[any])
		if !ok || g == nil {
			break
		}
		gk, wk := g.Keys(), w.Keys()
		for i := 0; i < len(gk) && i < len(wk); i++ {
			if gk[i] != wk[i] {
				return fmt.Sprintf("%s: property %d is %q, want %q (got %v, want %v)", path, i, gk[i], wk[i], gk, wk)
			}
		}
		if len(gk) != len(wk) {
			return fmt.Sprintf("%s: properties %v, want %v", path, gk, wk)
		}
		for _, k := range wk {
			if d := diffValues(path+"."+k, g.Get(k), w.Get(k)); d != "" {
				return d
			}
		}
		return ""
	case []any:
		g, ok := got.([]any)
		if !ok {
			break
		}
		for i := 0; i < len(g) && i < len(w); i++ {
			if d := diffValues(path+"["+strconv.Itoa(i)+"]", g[i], w[i]); d != "" {
				return d
			}
		}
		if len(g) != len(w) {
			return fmt.Sprintf("%s: length %d, want %d", path, len(g), len(w))
		}
		return ""
	}
	if reflect.DeepEqual(got, want) {
		return ""
	}
	return fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want))
}

func show(v any) string {
	s := js.StringifyJSON(v)
	if v == nil {
		s = "undefined"
	}
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}

// generic converts a JavaScript value to the value that encoding/json
// gives for its JSON text: the order of the properties is lost.
func generic(v any) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = generic(e)
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
	return v
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
func wantConfig(base map[string]any, c *js.Obj[any]) map[string]any {
	out := make(map[string]any, len(base))
	for k, v := range base {
		out[k] = v
	}
	delta := js.AsObj(c.Get("configDelta"))
	del, _ := delta.Get("del").([]any)
	for _, path := range del {
		key, _ := json.Marshal(generic(path))
		delete(out, string(key))
	}
	set, _ := delta.Get("set").([]any)
	for _, entry := range set {
		pair := entry.([]any)
		key, _ := json.Marshal(generic(pair[0]))
		out[string(key)] = generic(pair[1])
	}
	return out
}

// diffConfig compares the leaves of two configs, without the order.
func diffConfig(got, want map[string]any) string {
	keys := map[string]bool{}
	for k := range got {
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
		g, gok := got[k]
		w, wok := want[k]
		switch {
		case !gok:
			return fmt.Sprintf("config%s: missing, want %v", k, w)
		case !wok:
			return fmt.Sprintf("config%s: got %v, want it absent", k, g)
		case !reflect.DeepEqual(g, w):
			return fmt.Sprintf("config%s: got %v, want %v", k, g, w)
		}
	}
	return ""
}

// nodeFields is a layout node as the properties of the node object of
// upstream. The layout model has no separate state for "undefined": an
// empty string, a false boolean and a NaN number stand for it, except for
// label, cssStyles and cssClasses. So the properties with those values are
// left out here, and dropUndefinedLike removes them from the object of
// upstream.
func nodeFields(n *ru.Node) map[string]any {
	out := map[string]any{"id": n.ID}
	str := func(key, v string) {
		if v != "" {
			out[key] = v
		}
	}
	num := func(key string, v float64) {
		if v == v {
			out[key] = v
		}
	}
	flag := func(key string, v bool) {
		if v {
			out[key] = true
		}
	}
	list := func(key string, v []string) {
		if v != nil {
			out[key] = generic(anyStrings(v))
		}
	}
	str("labelStyle", n.LabelStyle)
	str("shape", n.Shape)
	if n.HasLabel {
		out["label"] = n.Label
	}
	if n.CSSClassesSet {
		out["cssClasses"] = n.CSSClasses
	}
	list("cssCompiledStyles", n.CSSCompiledStyles)
	if n.CSSStylesSet {
		list("cssStyles", n.CSSStyles)
	}
	str("dir", n.Dir)
	str("domId", n.DomID)
	flag("isGroup", n.IsGroup)
	num("colorIndex", n.ColorIndex)
	num("padding", n.Padding)
	num("rx", n.Rx)
	num("ry", n.Ry)
	str("look", n.Look)
	str("labelType", n.LabelType)
	str("parentId", n.ParentID)
	flag("centerLabel", n.CenterLabel)
	str("position", n.Position)
	list("description", n.Description)
	num("wrappingWidth", n.WrappingWidth)
	num("minWidth", n.MinWidth)
	for k, v := range n.Extra {
		out[k] = generic(v)
	}
	return out
}

// edgeFields is a layout edge as the properties of the edge object of
// upstream. See nodeFields.
func edgeFields(e *ru.Edge) map[string]any {
	out := map[string]any{"id": e.ID}
	str := func(key, v string) {
		if v != "" {
			out[key] = v
		}
	}
	str("start", e.Start)
	str("end", e.End)
	str("arrowhead", e.Arrowhead)
	str("arrowTypeEnd", e.ArrowTypeEnd)
	if !e.StyleUndefined && len(e.Style) == 1 {
		str("style", e.Style[0])
	}
	if e.LabelStyleIsText {
		str("labelStyle", e.LabelStyleText)
	}
	if e.HasLabel {
		out["label"] = e.Label
	}
	str("arrowheadStyle", e.ArrowheadStyle)
	str("labelpos", e.Labelpos)
	str("labelType", e.LabelType)
	str("thickness", e.Thickness)
	str("classes", e.Classes)
	str("look", e.Look)
	str("pattern", e.Pattern)
	for k, v := range e.Extra {
		out[k] = generic(v)
	}
	return out
}

// dropUndefinedLike removes the properties of a node or an edge of
// upstream that the layout model cannot tell from undefined: an empty
// string and false. The properties in keep have a field that says whether
// they are set.
func dropUndefinedLike(item any, keep ...string) map[string]any {
	obj, _ := generic(item).(map[string]any)
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
	return clean
}

// compareCase compares the db of this port with the dump of upstream. It
// returns the first difference, or "".
func compareCase(base map[string]any, c *js.Obj[any], p *parsed) string {
	wantType, _ := c.Get("type").(string)
	if p.typ != wantType {
		return fmt.Sprintf("type: got %q, want %q", p.typ, wantType)
	}
	if c.Get("nodes") == nil {
		// Upstream rendered another diagram type.
		if p.db != nil {
			return "got a state db, want none"
		}
		return ""
	}
	if p.db == nil {
		return "no state db"
	}
	d := p.db
	if s := diff("parsed", snapshot(d), c.Get("parsed")); s != "" {
		return s
	}

	data, err := layoutData(p.ctx, d)
	if err != nil {
		return "draw: " + errorMessage(err)
	}
	if s := diff("nodes", objList(d.nodes), c.Get("nodes")); s != "" {
		return s
	}
	if s := diff("edges", objList(d.edges), c.Get("edges")); s != "" {
		return s
	}
	if s := diff("drawn", snapshot(d), c.Get("drawn")); s != "" {
		return s
	}

	// The layout data that the unified renderer receives.
	wantNodes, _ := c.Get("nodes").([]any)
	if len(data.Nodes) != len(wantNodes) {
		return fmt.Sprintf("layout nodes: length %d, want %d", len(data.Nodes), len(wantNodes))
	}
	for i, n := range data.Nodes {
		got, want := nodeFields(n), dropUndefinedLike(wantNodes[i], "label", "cssClasses")
		if !reflect.DeepEqual(got, want) {
			return fmt.Sprintf("layout nodes[%d]: got %v, want %v", i, got, want)
		}
	}
	wantEdges, _ := c.Get("edges").([]any)
	if len(data.Edges) != len(wantEdges) {
		return fmt.Sprintf("layout edges: length %d, want %d", len(data.Edges), len(wantEdges))
	}
	for i, e := range data.Edges {
		got, want := edgeFields(e), dropUndefinedLike(wantEdges[i], "label")
		if !reflect.DeepEqual(got, want) {
			return fmt.Sprintf("layout edges[%d]: got %v, want %v", i, got, want)
		}
	}

	gotConfig := map[string]any{}
	flatten(generic(data.Config), nil, gotConfig)
	if s := diffConfig(gotConfig, wantConfig(base, c)); s != "" {
		return s
	}
	if got, want := generic(d.GetConfig()), generic(c.Get("stateConfig")); !reflect.DeepEqual(got, want) {
		return fmt.Sprintf("stateConfig: got %v, want %v", got, want)
	}
	if s := diff("dataDirection", data.Direction, c.Get("dataDirection")); s != "" {
		return s
	}
	if s := diff("layout", data.LayoutAlgorithm, c.Get("layout")); s != "" {
		return s
	}
	if s := diff("nodeSpacing", data.NodeSpacing, toNumber(c.Get("nodeSpacing"))); s != "" {
		return s
	}
	if s := diff("rankSpacing", data.RankSpacing, toNumber(c.Get("rankSpacing"))); s != "" {
		return s
	}
	if s := diff("markers", anyStrings(data.Markers), c.Get("markers")); s != "" {
		return s
	}
	if data.Type != wantType {
		return fmt.Sprintf("layout data type: got %q, want %q", data.Type, wantType)
	}
	return ""
}

// callAPI calls a function of the db by its upstream name. An argument that
// is undefined is nil. The result is the return value as a JavaScript
// value: nil for undefined.
func callAPI(d *DB, op string, args []any) (any, error) {
	arg := func(i int) any {
		if i < len(args) {
			return args[i]
		}
		return nil
	}
	str := func(i int) string {
		s, _ := arg(i).(string)
		return s
	}
	switch op {
	case "addState":
		return nil, d.addState(str(0), arg(1), arg(2), arg(3), arg(4), arg(5), arg(6), arg(7))
	case "addRelation":
		return nil, d.AddRelation(arg(0), arg(1), arg(2))
	case "addStyleClass":
		d.addStyleClass(str(0), arg(1))
	case "setCssClass":
		return nil, d.setCssClass(str(0), str(1))
	case "setStyle":
		d.setStyle(str(0), str(1))
	case "setTextStyle":
		d.setTextStyle(str(0), str(1))
	case "addDescription":
		return nil, d.addDescription(str(0), str(1))
	case "addLink":
		d.AddLink(arg(0), arg(1), arg(2))
	case "setDirection":
		d.SetDirection(str(0))
	case "getDirection":
		return d.GetDirection(), nil
	case "setRootDoc":
		doc, _ := arg(0).([]any)
		return nil, d.SetRootDoc(doc)
	case "extract":
		return nil, d.Extract(arg(0))
	case "trimColon":
		return d.trimColon(str(0)), nil
	case "cleanupLabel":
		return d.cleanupLabel(str(0)), nil
	case "getDividerId":
		return d.getDividerID(), nil
	case "startIdIfNeeded":
		return d.startIDIfNeeded(str(0)), nil
	case "startTypeIfNeeded":
		return d.startTypeIfNeeded(str(0), arg(1)), nil
	case "endIdIfNeeded":
		return d.endIDIfNeeded(str(0)), nil
	case "endTypeIfNeeded":
		return d.endTypeIfNeeded(str(0), arg(1)), nil
	case "getData":
		data, err := d.GetData()
		if err != nil {
			return nil, err
		}
		return newObj("nodes", objList(d.nodes), "edges", objList(d.edges), "direction", data.Direction), nil
	case "clear":
		d.clear(js.Truthy(arg(0)))
	case "getState":
		if st := d.GetState(str(0)); st != nil {
			return stateJSON(st), nil
		}
	case "getStates":
		return snapshot(d).Get("states"), nil
	case "getRelations":
		return snapshot(d).Get("relations"), nil
	case "getClasses":
		return snapshot(d).Get("classes"), nil
	case "getLinks":
		return snapshot(d).Get("links"), nil
	case "stateDomId":
		counter, _ := arg(1).(float64)
		return stateDomID(str(0), int(counter), str(2)), nil
	default:
		return nil, fmt.Errorf("callAPI: unknown function %s", op)
	}
	return nil, nil
}

// compareAPI runs the calls of an api case on a new db and compares each
// result and the state of the db after each call.
func compareAPI(c *js.Obj[any], p *parsed) string {
	api := js.AsObj(c.Get("api"))
	version, _ := api.Get("version").(float64)
	ops, _ := api.Get("ops").([]any)
	results, _ := c.Get("apiResults").([]any)
	if len(results) != len(ops) {
		return fmt.Sprintf("the golden file has %d results for %d calls", len(results), len(ops))
	}
	d := NewDB(p.ctx, int(version))
	for i, item := range ops {
		call, _ := item.([]any)
		op, _ := call[0].(string)
		args := make([]any, len(call)-1)
		for j, a := range call[1:] {
			// A null argument stands for undefined.
			if !js.IsNullish(a) {
				args[j] = a
			}
		}
		where := fmt.Sprintf("call %d (%s)", i, op)
		want := js.AsObj(results[i])
		value, err := callAPI(d, op, args)
		wantError, isError := want.Get("error").(string)
		switch {
		case isError && err == nil:
			return fmt.Sprintf("%s: no error, want %q", where, wantError)
		case isError:
			if got := errorMessage(err); got != wantError {
				return fmt.Sprintf("%s: error %q, want %q", where, got, wantError)
			}
		case err != nil:
			return fmt.Sprintf("%s: error %q, want none", where, errorMessage(err))
		default:
			if value == nil {
				value = js.Null
			}
			if s := diff(where+": value", value, want.Get("value")); s != "" {
				return s
			}
		}
		if s := diff(where+": snapshot", snapshot(d), want.Get("snapshot")); s != "" {
			return s
		}
	}
	return ""
}

// toNumber is the number that the renderer computes from a config value:
// the oracle dumps `conf?.nodeSpacing || 50`, which can be a text.
func toNumber(v any) any {
	if s, ok := v.(string); ok {
		return js.Number(s)
	}
	return v
}

// result is the comparison of one case of the golden file.
type result struct {
	name string
	text string
	// problem is the first difference, or "".
	problem string
	// isError is true when upstream rejects the text.
	isError bool
	// hasDB is true when upstream dumped a state db.
	hasDB bool
	// calls is the number of db calls of an api case.
	calls int
	// corpus is true for a fixture of the corpus.
	corpus bool
}

// runGolden parses every case of the golden file and compares it with the
// dump of upstream.
func runGolden(tb testing.TB) []result {
	tb.Helper()
	g := loadGolden(tb)
	base := map[string]any{}
	flatten(generic(g.Get("baseConfig")), nil, base)
	cases, _ := g.Get("cases").([]any)
	results := make([]result, 0, len(cases))
	for _, item := range cases {
		c := js.AsObj(item)
		name, _ := c.Get("name").(string)
		text := caseText(tb, c)
		_, hasText := c.Get("text").(string)
		r := result{name: name, text: text, corpus: !hasText}
		fresh, _ := c.Get("fresh").(bool)
		idCounter, _ := c.Get("idCounter").(float64)
		// The page of the oracle calls mermaid.initialize({ startOnLoad: false, ...site }).
		site := js.NewObj[any]()
		site.Set("startOnLoad", false)
		if extra := js.AsObj(c.Get("site")); extra != nil {
			extra.Each(func(k string, v any) { site.Set(k, v) })
		}
		wantError, isError := c.Get("error").(string)
		r.isError = isError
		r.hasDB = c.Get("nodes") != nil
		if results, ok := c.Get("apiResults").([]any); ok {
			r.calls = len(results)
		}

		p, err := parseText(text, site, !fresh, int(idCounter))
		switch {
		case isError && err == nil:
			// The dump stops before the layout: the draw function of
			// upstream can fail there.
			if _, derr := layoutData(p.ctx, p.db); derr != nil {
				err = derr
			}
			if err == nil {
				r.problem = fmt.Sprintf("no error, want %q", wantError)
			} else if got := errorMessage(err); got != wantError {
				r.problem = fmt.Sprintf("error %q, want %q", got, wantError)
			}
		case isError:
			if got := errorMessage(err); got != wantError {
				r.problem = fmt.Sprintf("error %q, want %q", got, wantError)
			}
		case err != nil:
			r.problem = "error " + strconv.Quote(errorMessage(err)) + ", want none"
		case c.Get("api") != nil:
			r.problem = compareAPI(c, p)
		default:
			r.problem = compareCase(base, c, p)
		}
		results = append(results, r)
	}
	return results
}

// TestDBGolden compares the db with the dump of real mermaid for every case
// of the golden file.
func TestDBGolden(t *testing.T) {
	results := runGolden(t)
	var total, failed, corpus, corpusFailed, withDB, withError, apiCases, calls int
	for _, r := range results {
		total++
		if r.calls > 0 {
			apiCases++
			calls += r.calls
		}
		if r.corpus {
			corpus++
		}
		if r.hasDB {
			withDB++
		}
		if r.isError {
			withError++
		}
		if r.problem == "" {
			continue
		}
		failed++
		if r.corpus {
			corpusFailed++
		}
		if failed <= 25 {
			text := r.text
			if r.corpus {
				text = ""
			}
			t.Errorf("%s: %s\n%s", r.name, r.problem, indent(text))
		}
	}
	t.Logf("%d cases (%d corpus fixtures, %d with a db dump, %d that mermaid rejects, %d with %d calls of the db): %d agree, %d differ (%d corpus fixtures differ)",
		total, corpus, withDB, withError, apiCases, calls, total-failed, failed, corpusFailed)
}

func indent(text string) string {
	if text == "" {
		return ""
	}
	return "    " + strings.ReplaceAll(strings.TrimRight(text, "\n"), "\n", "\n    ")
}
