package xychart

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// The golden file testdata/db.json.gz has what real mermaid 12.2.0 has in the
// xychart db after it renders a chart in Chromium. The script
// tools/reference/gen-xychart-db.mjs writes it.

// capture receives the db of each render, after the draw function.
var capture struct {
	sync.Mutex
	fn func(d *db)
}

func TestMain(m *testing.M) {
	// Wrap the draw function so that a test can read the db of a render.
	def := definition()
	drawFn := def.Draw
	def.Draw = func(ctx *render.Context, d render.DB, text string) error {
		err := drawFn(ctx, d, text)
		capture.Lock()
		fn := capture.fn
		capture.Unlock()
		if fn != nil {
			fn(d.(*db))
		}
		return err
	}
	render.Register(def)
	os.Exit(m.Run())
}

type goldenCase struct {
	Name           string          `json:"name"`
	Text           *string         `json:"text"`
	Type           string          `json:"type"`
	Error          *string         `json:"error"`
	Drawable       json.RawMessage `json:"drawable"`
	ChartConfig    json.RawMessage `json:"chartConfig"`
	ThemeConfig    json.RawMessage `json:"themeConfig"`
	Data           json.RawMessage `json:"data"`
	Title          *string         `json:"title"`
	AccTitle       *string         `json:"accTitle"`
	AccDescription *string         `json:"accDescription"`
	SVG            *string         `json:"svg"`
}

func loadGolden(t testing.TB) []goldenCase {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "db.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	var cases []goldenCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

// caseText returns the diagram text of a case. The text of a corpus fixture
// is in the corpus.
func caseText(t testing.TB, c *goldenCase) string {
	t.Helper()
	if c.Text != nil {
		return *c.Text
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "..", filepath.FromSlash(c.Name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// encNum is the function enc of xychart-db-page.js for a number.
func encNum(x float64) any {
	switch {
	case math.IsNaN(x):
		return map[string]any{"$num": "NaN"}
	case math.IsInf(x, 1):
		return map[string]any{"$num": "Infinity"}
	case math.IsInf(x, -1):
		return map[string]any{"$num": "-Infinity"}
	case x == 0 && math.Signbit(x):
		return map[string]any{"$num": "-0"}
	}
	return x
}

// undefined marks a value that enc leaves out of an object.
type undefined struct{}

// enc is the function enc of xychart-db-page.js for a dynamic value.
func enc(v any) any {
	switch x := v.(type) {
	case nil:
		return undefined{}
	case float64:
		return encNum(x)
	case int:
		return encNum(float64(x))
	case string, bool:
		return x
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = encElem(enc(e))
		}
		return out
	case []string:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = e
		}
		return out
	case *js.Obj[any]:
		out := map[string]any{}
		x.Each(func(k string, e any) { setProp(out, k, e) })
		return out
	}
	if js.IsNullish(v) {
		return nil // null
	}
	panic(fmt.Sprintf("enc: unexpected type %T", v))
}

// encElem is the form of an array element: undefined has a marker.
func encElem(v any) any {
	if _, ok := v.(undefined); ok {
		return map[string]any{"$undefined": true}
	}
	return v
}

// setProp sets a property, except for the value undefined.
func setProp(o map[string]any, key string, v any) {
	e := enc(v)
	if _, ok := e.(undefined); ok {
		return
	}
	o[key] = e
}

func encDrawable(shapes []drawableElem) any {
	out := make([]any, len(shapes))
	for i := range shapes {
		s := &shapes[i]
		data := []any{}
		switch s.typ {
		case "rect":
			for _, r := range s.rects {
				o := map[string]any{}
				setProp(o, "x", r.x)
				setProp(o, "y", r.y)
				setProp(o, "width", r.width)
				setProp(o, "height", r.height)
				setProp(o, "fill", r.fill)
				setProp(o, "strokeWidth", r.strokeWidth)
				setProp(o, "strokeFill", r.strokeFill)
				data = append(data, o)
			}
		case "text":
			for _, r := range s.texts {
				o := map[string]any{}
				setProp(o, "text", r.text)
				setProp(o, "x", r.x)
				setProp(o, "y", r.y)
				setProp(o, "fill", r.fill)
				setProp(o, "verticalPos", r.verticalPos)
				setProp(o, "horizontalPos", r.horizontalPos)
				setProp(o, "fontSize", r.fontSize)
				setProp(o, "rotation", r.rotation)
				data = append(data, o)
			}
		case "path":
			for _, r := range s.paths {
				o := map[string]any{}
				setProp(o, "path", r.path)
				setProp(o, "fill", r.fill)
				setProp(o, "strokeWidth", r.strokeWidth)
				setProp(o, "strokeFill", r.strokeFill)
				data = append(data, o)
			}
		}
		out[i] = map[string]any{
			"groupTexts": enc(s.groupTexts),
			"type":       s.typ,
			"data":       data,
		}
	}
	return out
}

func encAxisData(a *axisDataType) any {
	o := map[string]any{"type": a.typ, "title": a.title}
	if isBandAxisData(a) {
		o["categories"] = enc(a.categories)
	} else {
		o["min"] = encNum(a.min)
		o["max"] = encNum(a.max)
	}
	return o
}

func encChartData(d *xyChartData) any {
	plots := make([]any, len(d.plots))
	for i, p := range d.plots {
		data := make([]any, len(p.data))
		for j, datum := range p.data {
			data[j] = []any{datum.x, encElem(enc(datum.y))}
		}
		o := map[string]any{"type": p.typ, "title": p.title, "data": data}
		if isBarPlot(p) {
			o["fill"] = p.fill
		} else {
			o["strokeFill"] = p.strokeFill
			o["strokeWidth"] = encNum(p.strokeWidth)
			if p.hasPointLabels {
				o["pointLabels"] = enc(p.pointLabels)
			}
		}
		plots[i] = o
	}
	return map[string]any{
		"xAxis": encAxisData(d.xAxis),
		"yAxis": encAxisData(d.yAxis),
		"title": d.title,
		"plots": plots,
	}
}

// diff returns the path and the values of the first difference between two
// JSON values, or "" when they are equal. Numbers are equal when their bits
// are equal.
func diff(path string, got, want any) string {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s: got %#v, want an object", path, got)
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
			if !gok || !wok {
				return fmt.Sprintf("%s.%s: got %#v (present %v), want %#v (present %v)", path, k, gv, gok, wv, wok)
			}
			if d := diff(path+"."+k, gv, wv); d != "" {
				return d
			}
		}
		return ""
	case []any:
		g, ok := got.([]any)
		if !ok {
			return fmt.Sprintf("%s: got %#v, want an array", path, got)
		}
		if len(g) != len(w) {
			return fmt.Sprintf("%s: got %d elements, want %d", path, len(g), len(w))
		}
		for i := range w {
			if d := diff(fmt.Sprintf("%s[%d]", path, i), g[i], w[i]); d != "" {
				return d
			}
		}
		return ""
	case float64:
		g, ok := got.(float64)
		if !ok || math.Float64bits(g) != math.Float64bits(w) {
			return fmt.Sprintf("%s: got %#v, want %v", path, got, js.Str(w))
		}
		return ""
	}
	if got != want {
		return fmt.Sprintf("%s: got %#v, want %#v", path, got, want)
	}
	return ""
}

func decode(t testing.TB, raw json.RawMessage) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

var reStyle = regexp.MustCompile(`(?s)<style>.*?</style>`)

// stripStyle is the replacement that xychart-db-page.js does on the SVG.
func stripStyle(svg string) string {
	done := false
	return reStyle.ReplaceAllStringFunc(svg, func(m string) string {
		if done {
			return m
		}
		done = true
		return "<style></style>"
	})
}

// firstDiff shows the place where two strings start to differ.
func firstDiff(got, want string) string {
	i := 0
	for i < len(got) && i < len(want) && got[i] == want[i] {
		i++
	}
	lo := max(i-80, 0)
	return fmt.Sprintf("at byte %d\n got: …%s\nwant: …%s", i, got[lo:min(i+120, len(got))], want[lo:min(i+120, len(want))])
}

// knownDivergence lists the golden cases that the port does not reproduce,
// with the cause. There is none at present.
var knownDivergence = map[string]string{}

// compareCase renders a golden case and returns the differences from what
// real mermaid produced.
//
// The drawable elements and the SVG have positions that come from the text
// widths of Chromium on the macOS reference machine. A machine with other
// fonts does not compare them.
func compareCase(t *testing.T, c *goldenCase, last **db) []string {
	t.Helper()
	var diffs []string
	add := func(format string, args ...any) { diffs = append(diffs, fmt.Sprintf(format, args...)) }
	text := caseText(t, c)
	reference := fonttest.IsMacOSReference()
	*last = nil
	result, err := render.Render(text, render.Options{})
	if c.Type != "xychart" {
		// Upstream fails before the db exists: no diagram type, or a syntax
		// error.
		if c.Error == nil {
			t.Fatalf("the golden case has no type and no error")
		}
		if err == nil {
			add("no error, want %q", *c.Error)
		}
		return diffs
	}
	if c.Error != nil {
		if err == nil {
			add("no error, want %q", *c.Error)
			return diffs
		}
		// The parser has the messages of the generated parser.
		if strings.HasPrefix(*c.Error, "Parse error") || strings.HasPrefix(*c.Error, "Lexical error") {
			if err.Error() != *c.Error {
				add("error:\n got: %q\nwant: %q", err.Error(), *c.Error)
			}
		}
	} else if err != nil {
		add("error %v, want none", err)
		return diffs
	}
	d := *last
	if c.Drawable != nil {
		if d == nil {
			add("the draw function did not run")
			return diffs
		}
		if df := diff("drawable", encDrawable(d.drawable), decode(t, c.Drawable)); df != "" && reference {
			add("%s", df)
		}
	}
	if c.Error != nil {
		return diffs
	}
	if df := diff("chartConfig", enc(d.getChartConfig()), decode(t, c.ChartConfig)); df != "" {
		add("%s", df)
	}
	if df := diff("themeConfig", enc(d.getChartThemeConfig()), decode(t, c.ThemeConfig)); df != "" {
		add("%s", df)
	}
	if df := diff("data", encChartData(d.getXYChartData()), decode(t, c.Data)); df != "" {
		add("%s", df)
	}
	if got := d.GetDiagramTitle(); got != *c.Title {
		add("title: got %q, want %q", got, *c.Title)
	}
	if got := d.GetAccTitle(); got != *c.AccTitle {
		add("accTitle: got %q, want %q", got, *c.AccTitle)
	}
	if got := d.GetAccDescription(); got != *c.AccDescription {
		add("accDescription: got %q, want %q", got, *c.AccDescription)
	}
	if got := stripStyle(result.SVG); got != *c.SVG && reference {
		add("svg: %s", firstDiff(got, *c.SVG))
	}
	return diffs
}

// TestGoldenDB compares the db, the drawable elements of the chart builder
// and the SVG with real mermaid for every golden case. Numbers are compared
// bit for bit.
//
// A machine without the system fonts of the macOS reference machine compares
// the errors and the db, but not the drawable elements and the SVG. Then the
// test is marked as skipped.
func TestGoldenDB(t *testing.T) {
	cases := loadGolden(t)
	var last *db
	capture.Lock()
	capture.fn = func(d *db) { last = d }
	capture.Unlock()
	defer func() {
		capture.Lock()
		capture.fn = nil
		capture.Unlock()
	}()

	failed, known, errors := 0, 0, 0
	for i := range cases {
		c := &cases[i]
		if c.Error != nil {
			errors++
		}
		t.Run(c.Name, func(t *testing.T) {
			diffs := compareCase(t, c, &last)
			if len(diffs) == 0 {
				return
			}
			if reason, ok := knownDivergence[c.Name]; ok {
				known++
				t.Skipf("known divergence (%s):\n%s", reason, strings.Join(diffs, "\n"))
			}
			failed++
			t.Error(strings.Join(diffs, "\n"))
		})
	}
	t.Logf("%d golden cases (%d with an error upstream): %d failed, %d known divergences", len(cases), errors, failed, known)
	fonttest.RequireMacOSReference(t)
}

// TestGoldenReference checks the golden file against the reference renders:
// the SVG of a corpus fixture in the golden file is the reference SVG. It
// shows that the page of the generator has the state of the reference page.
func TestGoldenReference(t *testing.T) {
	cases := loadGolden(t)
	checked := 0
	for i := range cases {
		c := &cases[i]
		if c.Text != nil || c.SVG == nil {
			continue
		}
		rel := strings.TrimSuffix(strings.TrimPrefix(c.Name, "testdata/corpus/"), ".mmd") + ".svg"
		ref, err := os.ReadFile(filepath.Join("..", "..", "..", ".parity", "ref", filepath.FromSlash(rel)))
		if err != nil {
			// The reference directory is generated and can be absent.
			continue
		}
		checked++
		if got := stripStyle(string(bytes.TrimRight(ref, "\n"))); got != *c.SVG {
			t.Errorf("%s: the golden SVG is not the reference SVG: %s", c.Name, firstDiff(*c.SVG, got))
		}
	}
	if checked == 0 {
		t.Skip("no reference renders in .parity/ref")
	}
	t.Logf("%d fixtures checked", checked)
}
