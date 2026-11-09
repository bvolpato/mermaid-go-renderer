package er

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

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The golden file is the db of real mermaid and the layout data that the
// renderer hands to the layout: tools/reference/gen-er-db.mjs writes it with
// the browser oracle. The tests render the same texts up to the layout and
// compare everything.

type goldenFile struct {
	BaseConfig map[string]any `json:"baseConfig"`
	Cases      []goldenCase   `json:"cases"`
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

	EntitiesBefore []any    `json:"entitiesBefore"`
	SubgraphDepth  *float64 `json:"subgraphDepth"`
	Nodes          []any    `json:"nodes"`
	Edges          []any    `json:"edges"`
	ConfigDelta    *struct {
		Set [][]any    `json:"set"`
		Del [][]string `json:"del"`
	} `json:"configDelta"`
	DataDirection  string   `json:"dataDirection"`
	DataType       string   `json:"dataType"`
	Layout         string   `json:"layout"`
	Markers        []any    `json:"markers"`
	DiagramID      string   `json:"diagramId"`
	DataKeys       []string `json:"dataKeys"`
	Entities       []any    `json:"entities"`
	Relationships  []any    `json:"relationships"`
	Classes        []any    `json:"classes"`
	SubGraphs      []any    `json:"subGraphs"`
	Direction      string   `json:"direction"`
	Title          string   `json:"title"`
	AccTitle       string   `json:"accTitle"`
	AccDescription string   `json:"accDescription"`
	ErConfig       any      `json:"erConfig"`
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

// errorMessage is the name and the message of the JavaScript exception.
func errorMessage(err error) (name, message string) {
	var ce *config.Error
	if errors.As(err, &ce) {
		return ce.Name, ce.Message
	}
	var je *jison.Error
	if errors.As(err, &je) {
		return je.Name, je.Message
	}
	var te typeError
	if errors.As(err, &te) {
		return "TypeError", string(te)
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
		return stringsJSON(x)
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

// entityJSON is JSON.stringify of an EntityNode.
func entityJSON(e *EntityNode) map[string]any {
	attributes := make([]any, len(e.Attributes))
	for i, a := range e.Attributes {
		attribute := map[string]any{"type": a.Type, "name": a.Name, "comment": a.Comment}
		if a.Keys != nil {
			attribute["keys"] = stringsJSON(a.Keys)
		}
		attributes[i] = attribute
	}
	out := map[string]any{
		"id":         e.ID,
		"label":      e.Label,
		"attributes": attributes,
		"alias":      e.Alias,
		"shape":      e.Shape,
		"look":       e.Look,
		"cssClasses": e.CSSClasses,
		"cssStyles":  stringsJSON(e.CSSStyles),
		"labelType":  e.LabelType,
	}
	if e.CSSCompiledStyles != nil {
		out["cssCompiledStyles"] = stringsJSON(e.CSSCompiledStyles)
	}
	if e.HasColorIndex {
		out["colorIndex"] = float64(e.ColorIndex)
	}
	return out
}

// entitiesJSON is JSON.stringify of [...db.getEntities().entries()].
func entitiesJSON(d *DB) []any {
	out := []any{}
	d.GetEntities().Each(func(name string, e *EntityNode) {
		out = append(out, []any{name, entityJSON(e)})
	})
	return out
}

// subGraphJSON is JSON.stringify of an ErSubGraph.
func subGraphJSON(sg *ErSubGraph) map[string]any {
	out := map[string]any{
		"id":        sg.ID,
		"nodes":     stringsJSON(sg.Nodes),
		"title":     sg.Title,
		"classes":   stringsJSON(sg.Classes),
		"labelType": sg.LabelType,
	}
	if sg.CSSStyles != nil {
		out["cssStyles"] = stringsJSON(sg.CSSStyles)
	}
	if sg.HasDir {
		out["dir"] = sg.Dir
	}
	return out
}

// nodeJSON is JSON.stringify of a layout node, for the properties that the
// ER db sets. A property that the node model cannot tell from undefined (an
// empty string, false) is absent: see dropUndefinedLike.
func nodeJSON(n *ru.Node) map[string]any {
	out := map[string]any{"id": n.ID}
	str := func(key, v string) {
		if v != "" {
			out[key] = v
		}
	}
	num := func(key string, v float64) {
		if !ru.IsUndef(v) {
			out[key] = v
		}
	}
	if n.HasLabel {
		out["label"] = n.Label
	}
	str("labelStyle", n.LabelStyle)
	str("labelType", n.LabelType)
	str("parentId", n.ParentID)
	num("padding", n.Padding)
	if n.CSSCompiledStyles != nil {
		out["cssCompiledStyles"] = stringsJSON(n.CSSCompiledStyles)
	}
	if n.CSSStylesSet {
		out["cssStyles"] = stringsJSON(n.CSSStyles)
	}
	if n.CSSClassesSet {
		out["cssClasses"] = n.CSSClasses
	}
	str("shape", n.Shape)
	str("dir", n.Dir)
	if n.IsGroup {
		out["isGroup"] = true
	}
	str("look", n.Look)
	num("colorIndex", n.ColorIndex)
	str("domId", n.DomID)
	num("width", n.Width)
	num("height", n.Height)
	num("x", n.X)
	num("y", n.Y)
	for k, v := range n.Extra {
		out[k] = generic(v)
	}
	return out
}

// edgeJSON is JSON.stringify of a layout edge, for the properties that the
// ER db sets.
func edgeJSON(e *ru.Edge) map[string]any {
	out := map[string]any{"id": e.ID}
	str := func(key, v string) {
		if v != "" {
			out[key] = v
		}
	}
	str("type", e.Type)
	str("curve", e.Curve)
	str("start", e.Start)
	str("end", e.End)
	if e.HasLabel {
		out["label"] = e.Label
	}
	str("labelpos", e.Labelpos)
	str("thickness", e.Thickness)
	if !e.ClassesUndefined {
		out["classes"] = e.Classes
	}
	if !e.StyleUndefined {
		out["style"] = stringsJSON(e.Style)
	}
	str("arrowTypeStart", e.ArrowTypeStart)
	str("arrowTypeEnd", e.ArrowTypeEnd)
	str("pattern", e.Pattern)
	str("look", e.Look)
	str("labelType", e.LabelType)
	for k, v := range e.Extra {
		out[k] = generic(v)
	}
	return out
}

// dropUndefinedLike removes the properties of a node or an edge that the
// layout model of this port cannot tell from undefined: an empty string and
// false. The properties in keep have a field that says whether they are
// set, or are in Extra with their JavaScript value.
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
// values, or "". The order of the elements of a list counts.
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

// compareCase compares the db and the layout data of this port with the
// dump of upstream. It returns the first difference, or "".
func compareCase(base map[string]any, c *goldenCase, cp *capture) string {
	d, data := cp.db, cp.data
	if cp.ctx.Type != c.Type {
		return fmt.Sprintf("type: got %q, want %q", cp.ctx.Type, c.Type)
	}

	// The db before the draw function.
	if s := diff("entitiesBefore", cp.entitiesBefore, c.EntitiesBefore); s != "" {
		return s
	}
	if c.SubgraphDepth == nil || float64(cp.subgraphDepth) != *c.SubgraphDepth {
		return fmt.Sprintf("subgraphDepth: got %d, want %v", cp.subgraphDepth, c.SubgraphDepth)
	}

	// The layout data.
	nodes := make([]any, len(data.Nodes))
	for i, n := range data.Nodes {
		nodes[i] = nodeJSON(n)
	}
	if s := diff("nodes", dropUndefinedLike(nodes, "label", "cssClasses", "alias"), dropUndefinedLike(c.Nodes, "label", "cssClasses", "alias")); s != "" {
		return s
	}
	edges := make([]any, len(data.Edges))
	for i, e := range data.Edges {
		edges[i] = edgeJSON(e)
	}
	if s := diff("edges", dropUndefinedLike(edges, "label", "classes"), dropUndefinedLike(c.Edges, "label", "classes")); s != "" {
		return s
	}
	gotConfig := map[string]any{}
	flatten(generic(data.Config), nil, gotConfig)
	if s := diff("config", gotConfig, wantConfig(base, c)); s != "" {
		return s
	}
	if data.Direction != c.DataDirection {
		return fmt.Sprintf("layout data direction: got %q, want %q", data.Direction, c.DataDirection)
	}
	if data.Type != c.DataType {
		return fmt.Sprintf("layout data type: got %q, want %q", data.Type, c.DataType)
	}
	if data.LayoutAlgorithm != c.Layout {
		return fmt.Sprintf("layout: got %q, want %q", data.LayoutAlgorithm, c.Layout)
	}
	if s := diff("markers", stringsJSON(data.Markers), c.Markers); s != "" {
		return s
	}
	if data.DiagramID != c.DiagramID {
		return fmt.Sprintf("diagramId: got %q, want %q", data.DiagramID, c.DiagramID)
	}
	// The renderer of upstream sets no spacing in the layout data.
	for _, key := range c.DataKeys {
		switch key {
		case "nodes", "edges", "other", "config", "direction", "type", "layoutAlgorithm", "markers", "diagramId":
		default:
			return fmt.Sprintf("layout data: upstream has the property %q", key)
		}
	}
	if !ru.IsUndef(data.NodeSpacing) || !ru.IsUndef(data.RankSpacing) || len(data.Extra) > 0 {
		return fmt.Sprintf("layout data: got nodeSpacing %v, rankSpacing %v, extra %v, want them undefined", data.NodeSpacing, data.RankSpacing, data.Extra)
	}

	// The db after the draw function.
	if s := diff("entities", entitiesJSON(d), c.Entities); s != "" {
		return s
	}
	relationships := []any{}
	for _, r := range d.GetRelationships() {
		relationships = append(relationships, map[string]any{
			"entityA": r.EntityA,
			"roleA":   r.RoleA,
			"entityB": r.EntityB,
			"relSpec": map[string]any{"cardA": r.RelSpec.CardA, "cardB": r.RelSpec.CardB, "relType": r.RelSpec.RelType},
		})
	}
	if s := diff("relationships", relationships, c.Relationships); s != "" {
		return s
	}
	classes := []any{}
	d.GetClasses().Each(func(id string, def *EntityClass) {
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
	if got := d.GetDirection(); got != c.Direction {
		return fmt.Sprintf("direction: got %q, want %q", got, c.Direction)
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
	if s := diff("erConfig", generic(d.GetConfig()), c.ErConfig); s != "" {
		return s
	}
	return ""
}

// result is the comparison of one case of the golden file.
type result struct {
	text string
	// problem is the first difference, or "".
	problem string
	// isError is true when upstream rejects the text.
	isError bool
}

// runCase renders one case up to the layout and compares it with upstream.
func runCase(base map[string]any, c *goldenCase, text string) (r result) {
	r.text = text
	r.isError = c.Error != nil
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
	cp := &capture{warm: !c.Fresh, stop: true}
	loc := newCapture(cp)
	defer captures.Delete(loc)
	_, err := render.Render(text, render.Options{Config: site, Location: loc})

	switch {
	case errors.Is(err, errCaptured):
		if c.Error != nil {
			r.problem = fmt.Sprintf("got a db, want the error %q", *c.Error)
			return r
		}
		r.problem = compareCase(base, c, cp)
	case err == nil:
		r.problem = "the render did not reach the layout"
	case c.Error == nil:
		r.problem = fmt.Sprintf("got the error %q, want a db", err)
	default:
		name, message := errorMessage(err)
		if message != *c.Error {
			r.problem = fmt.Sprintf("error message: got %q, want %q", message, *c.Error)
		} else if c.ErrorName != "" && name != c.ErrorName {
			r.problem = fmt.Sprintf("error name: got %q, want %q", name, c.ErrorName)
		}
	}
	return r
}

var (
	goldenOnce    sync.Once
	goldenData    *goldenFile
	goldenResults []result
)

// runGolden renders every case of the golden file, one time.
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

// TestDBAgainstMermaid compares the db and the layout data of every case of
// the golden file with the dump of real mermaid: the fixtures of the corpus,
// seeded mutations of them and hand-written inputs. A case that upstream
// rejects must fail here with the same message.
func TestDBAgainstMermaid(t *testing.T) {
	g, results := runGolden(t)
	var corpus, corpusOK, other, otherOK, errorCases int
	for i, r := range results {
		c := &g.Cases[i]
		isCorpus := c.Text == nil
		if isCorpus {
			corpus++
		} else {
			other++
		}
		if r.isError {
			errorCases++
		}
		if r.problem != "" {
			t.Errorf("%s: %s\n    text: %s", c.Name, r.problem, strings.ReplaceAll(r.text, "\n", "\\n"))
			continue
		}
		if isCorpus {
			corpusOK++
		} else {
			otherOK++
		}
	}
	t.Logf("corpus fixtures: %d of %d agree; mutations and hand-written inputs: %d of %d agree; %d of all cases are errors upstream",
		corpusOK, corpus, otherOK, other, errorCases)
}
