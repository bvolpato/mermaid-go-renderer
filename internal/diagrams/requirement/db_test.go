package requirement

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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The golden file is the db of real mermaid at the moment when the renderer
// calls the layout: tools/reference/gen-requirement-db.mjs writes it with
// the browser oracle. The tests render the same texts and compare
// everything. Arrays and maps are compared in order. The order of the keys
// of an object is not compared.
//
// The tests of this package link no layout. They register a layout of their
// own under every name that mermaid has, as
// tools/reference/requirement-db-page.js does in the browser: the draw
// function of the renderer then runs up to the layout, and the layout keeps
// the layout data that it receives.

// capture is what a test keeps of one render.
type capture struct {
	// warm says that the page has made a render before, so the DOMPurify
	// hooks of mermaid are installed.
	warm bool
	// stop makes the layout fail with errCaptured, as the layout of the
	// oracle does.
	stop bool

	db *DB
	// requirementsBefore and elementsBefore are the dumps before the draw
	// function runs.
	requirementsBefore []any
	elementsBefore     []any
	// data is the layout data that the layout received.
	data *ru.LayoutData
}

var errCaptured = errors.New("layout data captured")

// captures has the capture of each render that wants one. The key is the
// time zone of the render: a pointer that the test makes for the render and
// that the pipeline hands to the context.
var captures sync.Map

// newCapture registers a capture and returns the time zone that selects it.
func newCapture(c *capture) *time.Location {
	loc := time.FixedZone("UTC", 0)
	captures.Store(loc, c)
	return loc
}

func captureOf(ctx *render.Context) *capture {
	if c, ok := captures.Load(ctx.Location); ok {
		return c.(*capture)
	}
	return nil
}

func init() {
	for name, algorithm := range map[string]string{
		"dagre": "", "swimlane": "", "cose-bilkent": "", "elk": "elk.layered",
		"elk.stress": "elk.stress", "elk.force": "elk.force", "elk.mrtree": "elk.mrtree",
		"elk.sporeOverlap": "elk.sporeOverlap", "elk.box": "elk.box", "elk.rectpacking": "elk.rectpacking",
	} {
		ru.RegisterLayout(name, algorithm, testLayout)
	}

	// The definition of the diagram, with hooks for the captures.
	def, _ := render.Lookup("requirement")
	newDB, drawDiagram := def.New, def.Draw
	def.New = func(ctx *render.Context) render.DB {
		if c := captureOf(ctx); c != nil && c.warm {
			ctx.Sanitizer.RemoveScript("")
		}
		return newDB(ctx)
	}
	def.Draw = func(ctx *render.Context, db render.DB, text string) error {
		if c := captureOf(ctx); c != nil {
			c.db = db.(*DB)
			c.requirementsBefore = requirementsJSON(c.db)
			c.elementsBefore = elementsJSON(c.db)
		}
		return drawDiagram(ctx, db, text)
	}
}

// testLayout keeps the layout data. Then it draws the nodes in a group,
// together as a layout of upstream does and without positions, so that a
// render runs the shape of this package.
func testLayout(ctx *render.Context, data *ru.LayoutData, svg *dom.Node, _ string) error {
	if c := captureOf(ctx); c != nil {
		c.data = data
		if c.stop {
			return errCaptured
		}
	}
	nodes := svg.Select("g").Append("g").Attr("class", "nodes")
	return ctx.All(len(data.Nodes), func(i int) error {
		_, err := ru.InsertNode(ctx, nodes, data.Nodes[i], &ru.ShapeRenderOptions{Config: data.Config, Dir: data.Direction})
		return err
	})
}

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

	RequirementsBefore []any  `json:"requirementsBefore"`
	ElementsBefore     []any  `json:"elementsBefore"`
	RendererGetClasses string `json:"rendererGetClasses"`
	Nodes              []any  `json:"nodes"`
	Edges              []any  `json:"edges"`
	Other              any    `json:"other"`
	ConfigDelta        *struct {
		Set [][]any    `json:"set"`
		Del [][]string `json:"del"`
	} `json:"configDelta"`
	DataDirection     string   `json:"dataDirection"`
	DataType          string   `json:"dataType"`
	Layout            string   `json:"layout"`
	NodeSpacing       any      `json:"nodeSpacing"`
	RankSpacing       any      `json:"rankSpacing"`
	Markers           []any    `json:"markers"`
	DiagramID         string   `json:"diagramId"`
	DataKeys          []string `json:"dataKeys"`
	Requirements      []any    `json:"requirements"`
	Elements          []any    `json:"elements"`
	Relationships     []any    `json:"relationships"`
	Classes           []any    `json:"classes"`
	Direction         string   `json:"direction"`
	Title             string   `json:"title"`
	AccTitle          string   `json:"accTitle"`
	AccDescription    string   `json:"accDescription"`
	RequirementConfig any      `json:"requirementConfig"`
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

// errorMessage is error.name and error.message of the JavaScript exception.
func errorMessage(err error) (name, message string) {
	var ce *config.Error
	if errors.As(err, &ce) {
		return ce.Name, ce.Message
	}
	var te typeError
	if errors.As(err, &te) {
		return "TypeError", string(te)
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

// requirementsJSON is [...db.getRequirements().entries()] as JSON data.
func requirementsJSON(d *DB) []any {
	out := []any{}
	d.GetRequirements().Each(func(key string, r *Requirement) {
		out = append(out, []any{key, map[string]any{
			"name":          r.Name,
			"type":          r.Type,
			"requirementId": r.RequirementID,
			"text":          r.Text,
			"risk":          r.Risk,
			"verifyMethod":  r.VerifyMethod,
			"cssStyles":     stringsJSON(r.CSSStyles),
			"classes":       stringsJSON(r.Classes),
		}})
	})
	return out
}

// elementsJSON is [...db.getElements().entries()] as JSON data.
func elementsJSON(d *DB) []any {
	out := []any{}
	d.GetElements().Each(func(key string, e *Element) {
		out = append(out, []any{key, map[string]any{
			"name":      e.Name,
			"type":      e.Type,
			"docRef":    e.DocRef,
			"cssStyles": stringsJSON(e.CSSStyles),
			"classes":   stringsJSON(e.Classes),
		}})
	})
	return out
}

// nodeJSON is a layout node as JSON data: every property that is set. The
// model has no separate state for an undefined string, an undefined
// boolean or an undefined list: "", false and nil stand for them.
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
	flag := func(key string, v bool) {
		if v {
			out[key] = true
		}
	}
	if n.HasLabel {
		out["label"] = n.Label
	}
	if n.CSSClassesSet {
		out["cssClasses"] = n.CSSClasses
	}
	if n.CSSStylesSet {
		out["cssStyles"] = stringsJSON(n.CSSStyles)
	}
	if n.CSSCompiledStyles != nil {
		out["cssCompiledStyles"] = stringsJSON(n.CSSCompiledStyles)
	}
	if n.HasStereotype {
		out["stereotype"] = n.Stereotype
	}
	if n.Description != nil {
		out["description"] = stringsJSON(n.Description)
	}
	str("shape", n.Shape)
	str("look", n.Look)
	str("domId", n.DomID)
	str("parentId", n.ParentID)
	str("labelStyle", n.LabelStyle)
	str("labelType", n.LabelType)
	str("dir", n.Dir)
	str("link", n.Link)
	str("linkTarget", n.LinkTarget)
	str("tooltip", n.Tooltip)
	str("icon", n.Icon)
	str("img", n.Img)
	str("pos", n.Pos)
	str("position", n.Position)
	str("constraint", n.Constraint)
	num("colorIndex", n.ColorIndex)
	num("padding", n.Padding)
	num("width", n.Width)
	num("height", n.Height)
	num("x", n.X)
	num("y", n.Y)
	num("rx", n.Rx)
	num("ry", n.Ry)
	num("type", n.Type)
	flag("isGroup", n.IsGroup)
	flag("haveCallback", n.HaveCallback)
	for k, v := range n.Extra {
		out[k] = generic(v)
	}
	return out
}

// edgeJSON is a layout edge as JSON data: every property that is set. The
// arrow types of an edge of this diagram are always strings.
func edgeJSON(e *ru.Edge) map[string]any {
	out := map[string]any{
		"id":             e.ID,
		"arrowTypeStart": e.ArrowTypeStart,
		"arrowTypeEnd":   e.ArrowTypeEnd,
	}
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
	if e.HasLabel {
		out["label"] = e.Label
	}
	if !e.StyleUndefined {
		out["style"] = stringsJSON(e.Style)
	}
	if e.LabelStyle != nil {
		out["labelStyle"] = stringsJSON(e.LabelStyle)
	}
	// An undefined start or end is "".
	str("start", e.Start)
	str("end", e.End)
	str("classes", e.Classes)
	str("labelpos", e.Labelpos)
	str("thickness", e.Thickness)
	str("type", e.Type)
	str("pattern", e.Pattern)
	str("look", e.Look)
	str("labelType", e.LabelType)
	str("arrowhead", e.Arrowhead)
	str("arrowheadStyle", e.ArrowheadStyle)
	str("curve", e.Curve)
	str("interpolate", e.Interpolate)
	str("stroke", e.Stroke)
	str("text", e.Text)
	str("startLabelRight", e.StartLabelRight)
	str("endLabelLeft", e.EndLabelLeft)
	num("minlen", e.Minlen)
	num("length", e.Length)
	num("width", e.Width)
	num("height", e.Height)
	for k, v := range e.Extra {
		out[k] = generic(v)
	}
	return out
}

// goldenConfig returns the config of a case: the base config with the
// delta of the case applied.
func goldenConfig(base map[string]any, c *goldenCase) map[string]any {
	var clone func(v any) any
	clone = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			out := make(map[string]any, len(x))
			for k, e := range x {
				out[k] = clone(e)
			}
			return out
		case []any:
			out := make([]any, len(x))
			for i, e := range x {
				out[i] = clone(e)
			}
			return out
		}
		return v
	}
	cfg := clone(base).(map[string]any)
	if c.ConfigDelta == nil {
		return cfg
	}
	for _, path := range c.ConfigDelta.Del {
		m := cfg
		for i, key := range path {
			if i == len(path)-1 {
				delete(m, key)
				break
			}
			next, ok := m[key].(map[string]any)
			if !ok {
				break
			}
			m = next
		}
	}
	for _, entry := range c.ConfigDelta.Set {
		pathAny, _ := entry[0].([]any)
		m := cfg
		for i, k := range pathAny {
			key, _ := k.(string)
			if i == len(pathAny)-1 {
				m[key] = entry[1]
				break
			}
			next, ok := m[key].(map[string]any)
			if !ok {
				next = map[string]any{}
				m[key] = next
			}
			m = next
		}
	}
	return cfg
}

// firstDiff returns the path of the first difference of two JSON values,
// or "".
func firstDiff(path string, got, want any) string {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s: got %s, want an object", path, short(got))
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
			if !gok {
				return fmt.Sprintf("%s.%s: missing, want %s", path, k, short(wv))
			}
			if !wok {
				return fmt.Sprintf("%s.%s: got %s, want no property", path, k, short(gv))
			}
			if d := firstDiff(path+"."+k, gv, wv); d != "" {
				return d
			}
		}
		return ""
	case []any:
		g, ok := got.([]any)
		if !ok {
			return fmt.Sprintf("%s: got %s, want an array", path, short(got))
		}
		for i := 0; i < len(g) && i < len(w); i++ {
			if d := firstDiff(fmt.Sprintf("%s[%d]", path, i), g[i], w[i]); d != "" {
				return d
			}
		}
		if len(g) != len(w) {
			return fmt.Sprintf("%s: got %d elements, want %d", path, len(g), len(w))
		}
		return ""
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Sprintf("%s: got %s, want %s", path, short(got), short(want))
	}
	return ""
}

func short(v any) string {
	data, _ := json.Marshal(v)
	s := string(data)
	if len(s) > 300 {
		s = s[:300] + "..."
	}
	return s
}

// dumpDB returns the content of a capture in the form of the golden file
// (see tools/reference/requirement-db-page.js).
func dumpDB(c *capture) map[string]any {
	d, data := c.db, c.data
	nodes := make([]any, len(data.Nodes))
	for i, n := range data.Nodes {
		nodes[i] = nodeJSON(n)
	}
	edges := make([]any, len(data.Edges))
	for i, e := range data.Edges {
		edges[i] = edgeJSON(e)
	}
	relationships := []any{}
	for _, r := range d.GetRelationships() {
		relationships = append(relationships, map[string]any{"type": r.Type, "src": r.Src, "dst": r.Dst})
	}
	classes := []any{}
	d.GetClasses().Each(func(key string, class *RequirementClass) {
		classes = append(classes, []any{key, map[string]any{
			"id":         class.ID,
			"styles":     stringsJSON(class.Styles),
			"textStyles": stringsJSON(class.TextStyles),
		}})
	})
	return map[string]any{
		"type":               data.Type,
		"requirementsBefore": c.requirementsBefore,
		"elementsBefore":     c.elementsBefore,
		"nodes":              nodes,
		"edges":              edges,
		"other":              generic(data.Extra["other"]),
		"config":             generic(data.Config),
		"dataDirection":      data.Direction,
		"layout":             data.LayoutAlgorithm,
		"nodeSpacing":        generic(data.NodeSpacing),
		"rankSpacing":        generic(data.RankSpacing),
		"markers":            stringsJSON(data.Markers),
		"diagramId":          data.DiagramID,
		"relationships":      relationships,
		"classes":            classes,
		"direction":          d.GetDirection(),
		"title":              d.GetDiagramTitle(),
		"accTitle":           d.GetAccTitle(),
		"accDescription":     d.GetAccDescription(),
		"requirementConfig":  generic(d.GetConfig()),
	}
}

// goldenDump is the expected dump of a case, in the form of dumpDB.
func goldenDump(g *goldenFile, c *goldenCase) map[string]any {
	return map[string]any{
		"type":               c.DataType,
		"requirementsBefore": c.RequirementsBefore,
		"elementsBefore":     c.ElementsBefore,
		"nodes":              c.Nodes,
		"edges":              c.Edges,
		"other":              c.Other,
		"config":             goldenConfig(g.BaseConfig, c),
		"dataDirection":      c.DataDirection,
		"layout":             c.Layout,
		"nodeSpacing":        c.NodeSpacing,
		"rankSpacing":        c.RankSpacing,
		"markers":            c.Markers,
		"diagramId":          c.DiagramID,
		"relationships":      c.Relationships,
		"classes":            c.Classes,
		"direction":          c.Direction,
		"title":              c.Title,
		"accTitle":           c.AccTitle,
		"accDescription":     c.AccDescription,
		"requirementConfig":  c.RequirementConfig,
	}
}

func siteConfig(tb testing.TB, site map[string]any) *js.Obj[any] {
	tb.Helper()
	if site == nil {
		return nil
	}
	data, err := json.Marshal(site)
	if err != nil {
		tb.Fatal(err)
	}
	v, err := js.ParseJSON(data)
	if err != nil {
		tb.Fatal(err)
	}
	return js.AsObj(v)
}

// renderToLayout runs render.Render for a text up to the layout, which
// stops the render. It returns the capture, or the error of the render.
func renderToLayout(text string, site *js.Obj[any], warm bool) (*capture, error) {
	c := &capture{warm: warm, stop: true}
	loc := newCapture(c)
	defer captures.Delete(loc)
	_, err := render.Render(text, render.Options{Config: site, Location: loc})
	if errors.Is(err, errCaptured) {
		return c, nil
	}
	if err == nil {
		return nil, errors.New("the draw function did not call a layout")
	}
	return nil, err
}

// layoutDataKeys are the properties of the layout data that the layout of
// upstream receives. A property that is not in this list would be a part
// of the layout data that the dump does not compare.
var layoutDataKeys = []string{"nodes", "edges", "other", "config", "direction", "type", "layoutAlgorithm", "nodeSpacing", "rankSpacing", "markers", "diagramId"}

// checkGolden checks the properties of the golden record that describe how
// upstream works, and that the Go model relies on.
func checkGolden(c *goldenCase) string {
	if !reflect.DeepEqual(c.DataKeys, layoutDataKeys) {
		return fmt.Sprintf("layout data keys %v, want %v", c.DataKeys, layoutDataKeys)
	}
	// The renderer has no getClasses function.
	if c.RendererGetClasses != "undefined" {
		return fmt.Sprintf("typeof renderer.getClasses is %q", c.RendererGetClasses)
	}
	if c.Type != "requirement" {
		return fmt.Sprintf("diag.type %q", c.Type)
	}
	// getData makes the requirements and the elements the nodes: after the
	// draw function, the maps of the db hold the node objects. When two
	// entries have the same name, the later node has that name too.
	entries := append(append([]any{}, c.Requirements...), c.Elements...)
	if len(entries) != len(c.Nodes) {
		return fmt.Sprintf("%d requirements and elements, %d nodes", len(entries), len(c.Nodes))
	}
	for i, entry := range entries {
		pair, ok := entry.([]any)
		if !ok || len(pair) != 2 {
			return fmt.Sprintf("entry %d is not a pair", i)
		}
		if d := firstDiff(fmt.Sprintf("entry[%d]", i), pair[1], c.Nodes[i]); d != "" {
			return "the db does not hold the node: " + d
		}
	}
	return ""
}

// compareCase renders the text of a case and compares the result with the
// golden record. It returns a description of the first difference, or "".
func compareCase(tb testing.TB, g *goldenFile, c *goldenCase) string {
	tb.Helper()
	text := caseText(tb, c)
	site := js.NewObj[any]()
	site.Set("startOnLoad", false)
	if s := siteConfig(tb, c.Site); s != nil {
		s.Each(func(k string, v any) { site.Set(k, v) })
	}
	cp, err := renderToLayout(text, site, !c.Fresh)
	var unsupported *render.UnsupportedError
	if errors.As(err, &unsupported) {
		// The text is a diagram of another type, which this test does not
		// link. Upstream has the type "---" for front matter that is not
		// closed: its parser throws.
		if unsupported.Type == "---" && c.Error != nil && strings.HasPrefix(*c.Error, "Diagrams beginning with --- are not valid.") {
			return ""
		}
		return fmt.Sprintf("the diagram type is %q", unsupported.Type)
	}
	if c.Error != nil {
		if err == nil {
			return fmt.Sprintf("no error, want the error %q", *c.Error)
		}
		name, message := errorMessage(err)
		if message != *c.Error {
			return fmt.Sprintf("error %q, want %q", message, *c.Error)
		}
		if c.ErrorName != "" && name != c.ErrorName {
			return fmt.Sprintf("error name %q, want %q", name, c.ErrorName)
		}
		return ""
	}
	if err != nil {
		return fmt.Sprintf("error %q, want no error", err.Error())
	}
	if d := checkGolden(c); d != "" {
		return "golden record: " + d
	}
	return firstDiff("", dumpDB(cp), goldenDump(g, c))
}

// knownDivergences are the cases in which this port differs from upstream
// on purpose. The value is the first difference of the case: the test
// fails when the case has another difference, or none.
var knownDivergences = map[string]string{
	// The front matter has state.nodeSpacing and state.rankSpacing as the
	// strings "30" and "40". Upstream copies the strings to the layout
	// data. The model of the layout data holds numbers.
	"extra368": `.nodeSpacing: got 30, want "30"`,
}

// TestDBAgainstMermaid compares the db and the layout data with those of
// real mermaid for every case of the golden file: every fixture of the
// requirement family of the corpus, mutations of the fixtures and
// hand-written inputs. A case that mermaid rejects must fail with the same
// error message.
func TestDBAgainstMermaid(t *testing.T) {
	if def, ok := render.Lookup("requirement"); !ok || def.Classes != nil {
		t.Fatal("the definition must exist and must have no Classes function: the renderer of upstream has no getClasses")
	}
	g := loadGolden(t)
	diffs := make([]string, len(g.Cases))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for i := range g.Cases {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if p := recover(); p != nil {
					diffs[i] = fmt.Sprintf("panic: %v", p)
				}
			}()
			diffs[i] = compareCase(t, g, &g.Cases[i])
		}(i)
	}
	wg.Wait()
	counts := map[string]int{}
	fails := map[string]int{}
	rejected := map[string]int{}
	known := 0
	for i := range g.Cases {
		c := &g.Cases[i]
		kind := "extra"
		switch {
		case strings.Contains(c.Name, "#mutant"):
			kind = "mutant"
		case strings.HasPrefix(c.Name, "testdata/"):
			kind = "corpus"
		case strings.HasPrefix(c.Name, "site"):
			kind = "site"
		}
		counts[kind]++
		if c.Error != nil {
			rejected[kind]++
		}
		d := diffs[i]
		if want, ok := knownDivergences[c.Name]; ok {
			known++
			fails[kind]++
			if d != want {
				t.Errorf("%s: known divergence: got the difference %q, want %q", c.Name, d, want)
			}
			continue
		}
		if d != "" {
			fails[kind]++
			t.Errorf("%s: %s", c.Name, d)
		}
	}
	for _, kind := range []string{"corpus", "mutant", "extra", "site"} {
		t.Logf("%s: %d cases (%d that mermaid rejects), %d equal", kind, counts[kind], rejected[kind], counts[kind]-fails[kind])
	}
	t.Logf("known divergences: %d", known)
}
