package usecase

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
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The golden file testdata/db.json.gz is the db of real mermaid and the
// layout data that the renderer hands to the layout, inside a real render:
// tools/reference/gen-usecase-db.mjs writes it with the browser oracle. The
// tests render the same texts up to the layout and compare everything.

type dbGolden struct {
	BaseConfig map[string]any `json:"baseConfig"`
	Cases      []dbCase       `json:"cases"`
}

type dbCase struct {
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
	// Data has the properties of the layout data other than the nodes,
	// the edges and the config.
	Data           map[string]any `json:"data"`
	DataKeys       []string       `json:"dataKeys"`
	Model          map[string]any `json:"model"`
	Title          string         `json:"title"`
	AccTitle       string         `json:"accTitle"`
	AccDescription string         `json:"accDescription"`
	UsecaseConfig  any            `json:"usecaseConfig"`
}

func loadDBGolden(tb testing.TB) *dbGolden {
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
	var g dbGolden
	if err := json.Unmarshal(data, &g); err != nil {
		tb.Fatal(err)
	}
	return &g
}

func dbCaseText(tb testing.TB, c *dbCase) string {
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
	var je *jsonError
	if errors.As(err, &je) {
		return "UsecaseJsonError", je.message
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
	case bool:
		return x
	case string:
		return wf(x)
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
		return stringList(x)
	case []UsecaseJsonRow:
		out := make([]any, len(x))
		for i, row := range x {
			out[i] = map[string]any{"key": wf(row.Key), "accessibleKey": wf(row.AccessibleKey), "value": wf(row.Value)}
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

// setNumber sets a property that is a number in the layout model: the
// JavaScript value of the property when the config had another type (it is
// in extra then), or the number when it is defined.
func setNumber(out map[string]any, key string, v float64, extra map[string]any) {
	if raw, ok := extra[key]; ok {
		if raw != nil {
			out[key] = generic(raw)
		}
		return
	}
	if !ru.IsUndef(v) {
		out[key] = v
	}
}

// nodeJSON is JSON.stringify of a layout node of this diagram.
func nodeJSON(n *ru.Node) map[string]any {
	out := map[string]any{
		"id":      wf(n.ID),
		"shape":   n.Shape,
		"isGroup": n.IsGroup,
	}
	if n.HasLabel {
		out["label"] = wf(n.Label)
	}
	if n.LabelType != "" {
		out["labelType"] = n.LabelType
	}
	if !ru.IsUndef(n.Padding) {
		out["padding"] = n.Padding
	}
	if n.Look != "" {
		out["look"] = n.Look
	}
	if !ru.IsUndef(n.ColorIndex) {
		out["colorIndex"] = n.ColorIndex
	}
	if n.CSSClassesSet {
		out["cssClasses"] = wf(n.CSSClasses)
	}
	if n.CSSStylesSet {
		out["cssStyles"] = stringList(n.CSSStyles)
	}
	if n.CSSCompiledStyles != nil {
		out["cssCompiledStyles"] = stringList(n.CSSCompiledStyles)
	}
	if n.Icon != "" {
		out["icon"] = wf(n.Icon)
	}
	if n.HasStereotype {
		out["stereotype"] = wf(n.Stereotype)
	}
	if n.ParentID != "" {
		out["parentId"] = wf(n.ParentID)
	}
	setNumber(out, "wrappingWidth", n.WrappingWidth, n.Extra)
	setNumber(out, "minWidth", n.MinWidth, n.Extra)
	if n.SpreadPorts {
		out["spreadPorts"] = true
	}
	if n.DomID != "" {
		out["domId"] = wf(n.DomID)
	}
	for k, v := range n.Extra {
		if k == "wrappingWidth" || k == "minWidth" {
			continue
		}
		out[k] = generic(v)
	}
	return out
}

// edgeJSON is JSON.stringify of a layout edge of this diagram.
func edgeJSON(e *ru.Edge) map[string]any {
	out := map[string]any{
		"id":                wf(e.ID),
		"start":             wf(e.Start),
		"end":               wf(e.End),
		"source":            wf(e.Source),
		"target":            wf(e.Target),
		"type":              e.Type,
		"arrowTypeStart":    e.ArrowTypeStart,
		"arrowTypeEnd":      e.ArrowTypeEnd,
		"pattern":           e.Pattern,
		"labelpos":          e.Labelpos,
		"classes":           wf(e.Classes),
		"style":             stringList(e.Style),
		"cssCompiledStyles": stringList(e.CSSCompiledStyles),
		"look":              e.Look,
		"thickness":         e.Thickness,
		"minlen":            e.Minlen,
		"isUserDefinedId":   e.IsUserDefinedID,
	}
	if e.HasAnimate {
		out["animate"] = e.Animate
	}
	if e.HasLabel {
		out["label"] = wf(e.Label)
	}
	if e.LabelType != "" {
		out["labelType"] = e.LabelType
	}
	if e.Animation != "" {
		out["animation"] = e.Animation
	}
	for k, v := range e.Extra {
		out[k] = generic(v)
	}
	return out
}

// dataJSON is JSON.stringify of the properties of the layout data other
// than the nodes, the edges and the config.
func dataJSON(data *ru.LayoutData) map[string]any {
	out := map[string]any{
		"type":            data.Type,
		"layoutAlgorithm": data.LayoutAlgorithm,
		"direction":       data.Direction,
		"markers":         stringList(data.Markers),
		"diagramId":       data.DiagramID,
	}
	setNumber(out, "nodeSpacing", data.NodeSpacing, data.Extra)
	setNumber(out, "rankSpacing", data.RankSpacing, data.Extra)
	for k, v := range data.Extra {
		if k == "nodeSpacing" || k == "rankSpacing" || v == nil {
			continue
		}
		out[k] = generic(v)
	}
	return out
}

// dataKeys are the names of the properties of the layout data of upstream,
// in the order in which getData and prepareUsecaseLayoutData set them.
var dataKeys = []string{
	"nodes", "edges", "config", "type", "layoutAlgorithm", "direction", "nodeSpacing", "rankSpacing",
	"actorFontSize", "actorFontFamily", "actorFontWeight", "usecaseFontSize", "usecaseFontFamily", "usecaseFontWeight",
	"diagramPadding", "useMaxWidth", "markers", "diagramId",
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
func wantConfig(base map[string]any, c *dbCase) map[string]any {
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

// compareDBCase compares the db and the layout data of this port with the
// dump of upstream. It returns the first difference, or "".
func compareDBCase(base map[string]any, c *dbCase, cp *capture) string {
	d, data := cp.db, cp.data
	if cp.ctx.Type != c.Type {
		return fmt.Sprintf("type: got %q, want %q", cp.ctx.Type, c.Type)
	}

	// The layout data.
	nodes := make([]any, len(data.Nodes))
	for i, n := range data.Nodes {
		nodes[i] = nodeJSON(n)
	}
	if s := jsonDiff("nodes", nodes, c.Nodes); s != "" {
		return s
	}
	edges := make([]any, len(data.Edges))
	for i, e := range data.Edges {
		edges[i] = edgeJSON(e)
	}
	if s := jsonDiff("edges", edges, c.Edges); s != "" {
		return s
	}
	gotConfig := map[string]any{}
	flatten(generic(data.Config), nil, gotConfig)
	if s := jsonDiff("config", gotConfig, wantConfig(base, c)); s != "" {
		return s
	}
	if s := jsonDiff("data", dataJSON(data), c.Data); s != "" {
		return s
	}
	if s := jsonDiff("dataKeys", stringList(dataKeys), stringList(c.DataKeys)); s != "" {
		return s
	}

	// The db.
	model := dumpModel(d.state)
	for key, want := range c.Model {
		if s := jsonDiff("model."+key, model[key], want); s != "" {
			return s
		}
	}
	if got := wf(d.GetDiagramTitle()); got != c.Title {
		return fmt.Sprintf("title: got %q, want %q", got, c.Title)
	}
	if got := wf(d.GetAccTitle()); got != c.AccTitle {
		return fmt.Sprintf("accTitle: got %q, want %q", got, c.AccTitle)
	}
	if got := wf(d.GetAccDescription()); got != c.AccDescription {
		return fmt.Sprintf("accDescription: got %q, want %q", got, c.AccDescription)
	}
	return jsonDiff("usecaseConfig", generic(d.GetConfig()), c.UsecaseConfig)
}

// dbResult is the comparison of one case of the golden file.
type dbResult struct {
	text string
	// problem is the first difference, or "".
	problem string
	// isError is true when upstream rejects the text.
	isError bool
}

// runDBCase renders one case up to the layout and compares it with upstream.
func runDBCase(base map[string]any, c *dbCase, text string) (r dbResult) {
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
		r.problem = compareDBCase(base, c, cp)
	case err == nil:
		r.problem = "the render did not reach the layout"
	case c.Error == nil:
		r.problem = fmt.Sprintf("got the error %q, want a db", err)
	default:
		name, message := errorMessage(err)
		if got := wf(message); got != *c.Error {
			r.problem = fmt.Sprintf("error message:\n got %q\nwant %q", got, *c.Error)
		} else if c.ErrorName != "" && name != c.ErrorName {
			r.problem = fmt.Sprintf("error name: got %q, want %q", name, c.ErrorName)
		}
	}
	return r
}

var (
	dbGoldenOnce    sync.Once
	dbGoldenData    *dbGolden
	dbGoldenResults []dbResult
)

// runDBGolden renders every case of the golden file, one time.
func runDBGolden(tb testing.TB) (*dbGolden, []dbResult) {
	tb.Helper()
	dbGoldenOnce.Do(func() {
		g := loadDBGolden(tb)
		base := map[string]any{}
		flatten(any(g.BaseConfig), nil, base)
		texts := make([]string, len(g.Cases))
		for i := range g.Cases {
			texts[i] = dbCaseText(tb, &g.Cases[i])
		}
		results := make([]dbResult, len(g.Cases))
		var wg sync.WaitGroup
		sem := make(chan struct{}, runtime.NumCPU())
		for i := range g.Cases {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer wg.Done()
				defer func() { <-sem }()
				results[i] = runDBCase(base, &g.Cases[i], texts[i])
			}(i)
		}
		wg.Wait()
		dbGoldenData, dbGoldenResults = g, results
	})
	return dbGoldenData, dbGoldenResults
}

// TestDBAgainstMermaid compares the db and the layout data of every case of
// the golden file with the dump of real mermaid: the fixtures of the corpus,
// seeded mutations of them and hand-written inputs. A case that upstream
// rejects must fail here with the same message.
func TestDBAgainstMermaid(t *testing.T) {
	g, results := runDBGolden(t)
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
			t.Errorf("%s: %s\n    text: %s", c.Name, r.problem, quoteText(r.text))
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
