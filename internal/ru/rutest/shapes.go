// Package rutest has test helpers for the unified renderer. They replay
// cases that were recorded from upstream mermaid in Chromium.
package rutest

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// ShapeCase is one case of tools/reference/probes/shapes.js with the
// result that upstream produced.
type ShapeCase struct {
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Dir       string          `json:"dir"`
	Node      json.RawMessage `json:"node"`
	Config    json.RawMessage `json:"config"`
	CSS       string          `json:"css"`
	HTML      string          `json:"html"`
	Width     *float64        `json:"width"`
	Height    *float64        `json:"height"`
	Random    int             `json:"randomCalls"`
	Shape     string          `json:"shape"`
	Intersect []any           `json:"intersects"`
	Error     string          `json:"error"`
}

// shapeRays are the points from which probes/shapes.js calls intersect.
var shapeRays = [][2]float64{{-500, -300}, {500, -300}, {500, 300}, {-500, 300}, {0, -400}, {0, 400}, {-400, 0}, {400, 0}, {-37, 290}, {123, -45}, {1000, 10}, {-3, -1000}}

// LoadShapeCases reads a golden file written by tools/reference/gen-shapes.mjs
// (JSON, or gzip-compressed JSON when the name ends in .gz).
func LoadShapeCases(t testing.TB, path string) []ShapeCase {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("no shape golden file: %v", err)
	}
	defer f.Close()
	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		defer gz.Close()
		r = gz
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	cases, err := DecodeShapeCases(data)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return cases
}

// DecodeShapeCases reads the content of a golden file. The file is either
// an array of cases, or the compact form of tools/reference/shape-golden.mjs:
// an object with the lists configs and styles, and cases that hold an index
// into each list in place of the config and the style sheet.
func DecodeShapeCases(data []byte) ([]ShapeCase, error) {
	if trimmed := strings.TrimLeft(string(data[:min(len(data), 64)]), " \t\r\n"); strings.HasPrefix(trimmed, "[") {
		var cases []ShapeCase
		err := json.Unmarshal(data, &cases)
		return cases, err
	}
	var file struct {
		Configs []json.RawMessage            `json:"configs"`
		Styles  []string                     `json:"styles"`
		Cases   []map[string]json.RawMessage `json:"cases"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	cases := make([]ShapeCase, len(file.Cases))
	for i, fields := range file.Cases {
		var index int
		if raw, ok := fields["config"]; ok && json.Unmarshal(raw, &index) == nil {
			if index < 0 || index >= len(file.Configs) {
				return nil, fmt.Errorf("case %d: config index %d is out of range", i, index)
			}
			fields["config"] = file.Configs[index]
		}
		if raw, ok := fields["css"]; ok && json.Unmarshal(raw, &index) == nil {
			if index < 0 || index >= len(file.Styles) {
				return nil, fmt.Errorf("case %d: style index %d is out of range", i, index)
			}
			text, _ := json.Marshal(file.Styles[index])
			fields["css"] = text
		}
		full, err := json.Marshal(fields)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(full, &cases[i]); err != nil {
			return nil, fmt.Errorf("case %d: %w", i, err)
		}
	}
	return cases, nil
}

// ShapeResult is the outcome of one case.
type ShapeResult struct {
	Name   string
	Status string // "exact", "equivalent", "different", "error", "not-ported"
	Detail string
}

// reText finds text content in markup: characters that are not white space
// between two tags.
var reText = regexp.MustCompile(`>[^<>]*[^<>\s][^<>]*<`)

// SkipComparison reports whether a test must not compare its result with
// the result of upstream that has the given markup. That is so when the
// markup has text and this machine does not have the system fonts of the
// macOS reference machine (fonttest.IsMacOSReference): the sizes and the
// positions of such a result come from the widths of the text in Chromium
// on that machine. A result with no text does not depend on the fonts, and
// every machine compares it.
//
// A test that skips a comparison still fails for the status "error", which
// is an error or a panic of the Go code where upstream has a result. It
// counts the comparisons that it skipped and gives the count to
// RequireFonts at its end.
func SkipComparison(markup string) bool {
	return !fonttest.IsMacOSReference() && reText.MatchString(markup)
}

// RequireFonts ends a test that skipped comparisons, see SkipComparison:
// it marks the test as skipped when the count is not 0.
func RequireFonts(t testing.TB, skipped int) {
	t.Helper()
	if skipped > 0 {
		t.Logf("%d cases with text were not compared with upstream", skipped)
		fonttest.RequireMacOSReference(t)
	}
}

// RunShapeCase replays one case: it builds the same page, calls
// ru.InsertNode with the same node and config, and compares the markup, the
// node size and the intersections with upstream.
func RunShapeCase(c ShapeCase) (res ShapeResult) {
	res.Name = c.Name
	defer func() {
		if p := recover(); p != nil {
			res.Status, res.Detail = "error", fmt.Sprintf("panic: %v", p)
		}
	}()
	cfgVal, err := js.ParseJSON(c.Config)
	if err != nil || js.AsObj(cfgVal) == nil {
		return ShapeResult{c.Name, "error", fmt.Sprintf("bad config in golden: %v", err)}
	}
	cfg := js.AsObj(cfgVal)
	nodeVal, err := js.ParseJSON(c.Node)
	if err != nil {
		return ShapeResult{c.Name, "error", fmt.Sprintf("bad node in golden: %v", err)}
	}
	ctx := render.NewContext(render.ContextOptions{Config: cfg})
	ctx.Type = c.Type
	if ctx.Type == "" {
		ctx.Type = "flowchart-v2"
	}
	// The probe removes the <g> that appendDivSvgG adds and builds its own
	// svg content: a style element and one group.
	ctx.SVG.RemoveChildren()
	style := ctx.Doc.CreateElement("style")
	style.Text(c.CSS)
	ctx.SVG.AppendChild(style)
	g := ctx.SVG.Append("g").Attr("class", "nodes")

	node := ru.NodeFromJS(js.AsObj(nodeVal))
	_, err = ru.InsertNode(ctx, g, node, &ru.ShapeRenderOptions{Config: cfg, Dir: c.Dir})
	if c.Error != "" {
		if err != nil {
			return ShapeResult{c.Name, "exact", ""}
		}
		return ShapeResult{c.Name, "different", "upstream fails: " + c.Error}
	}
	if err != nil {
		if _, ok := err.(*ru.NotPortedError); ok {
			return ShapeResult{c.Name, "not-ported", err.Error()}
		}
		return ShapeResult{c.Name, "error", err.Error()}
	}

	exact := true
	got := g.InnerHTML()
	if got != c.HTML {
		exact = false
		verdict, diff := parity.CompareSVG("<svg>"+got+"</svg>", "<svg>"+c.HTML+"</svg>", 0.01)
		if verdict == parity.Different {
			return ShapeResult{c.Name, "different", "markup: " + diff.String()}
		}
	}
	checkNum := func(what string, got float64, want *float64) string {
		if want == nil {
			if !ru.IsUndef(got) {
				return fmt.Sprintf("%s: got %v, want undefined", what, got)
			}
			return ""
		}
		if got != *want {
			exact = false
			if math.Abs(got-*want) > 0.01 {
				return fmt.Sprintf("%s: got %v, want %v", what, got, *want)
			}
		}
		return ""
	}
	if d := checkNum("node.width", node.Width, c.Width); d != "" {
		return ShapeResult{c.Name, "different", d}
	}
	if d := checkNum("node.height", node.Height, c.Height); d != "" {
		return ShapeResult{c.Name, "different", d}
	}
	node.X, node.Y = 0, 0
	for i, want := range c.Intersect {
		pair, ok := want.([]any)
		if !ok || len(pair) != 2 || node.Intersect == nil {
			continue
		}
		p := node.Intersect(ru.Point{X: shapeRays[i][0], Y: shapeRays[i][1]})
		wx, _ := pair[0].(float64)
		wy, _ := pair[1].(float64)
		if p.X != wx || p.Y != wy {
			exact = false
			if math.Abs(p.X-wx) > 0.01 || math.Abs(p.Y-wy) > 0.01 {
				return ShapeResult{c.Name, "different", fmt.Sprintf("intersect ray %d: got (%v, %v), want (%v, %v)", i, p.X, p.Y, wx, wy)}
			}
		}
	}
	if exact {
		return ShapeResult{c.Name, "exact", ""}
	}
	return ShapeResult{c.Name, "equivalent", ""}
}

// RunShapeGolden replays every case of the golden files and fails the test
// for cases that differ. It logs a summary of each file. Cases of shapes
// that are not ported yet are counted and do not fail the test.
//
// On a machine without the system fonts of the macOS reference machine, a
// case with text fails the test only for an error, and the test is marked
// as skipped after the last file. See SkipComparison.
func RunShapeGolden(t *testing.T, paths ...string) {
	t.Helper()
	skipped := 0
	for _, path := range paths {
		cases := LoadShapeCases(t, path)
		counts := map[string]int{}
		for _, c := range cases {
			r := RunShapeCase(c)
			counts[r.Status]++
			skip := SkipComparison(c.HTML)
			if skip {
				skipped++
			}
			if r.Status == "error" || r.Status == "different" && !skip {
				t.Errorf("%s: %s\n    %s", r.Name, r.Status, r.Detail)
			}
		}
		t.Logf("%s: %d cases: exact=%d equivalent=%d different=%d error=%d not-ported=%d",
			path, len(cases), counts["exact"], counts["equivalent"], counts["different"], counts["error"], counts["not-ported"])
	}
	RequireFonts(t, skipped)
}
