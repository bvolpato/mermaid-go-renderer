package requirement

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/rutest"
)

// shapeCase is one case of tools/reference/probes/requirement-shapes.js
// with the result that upstream produced.
type shapeCase struct {
	Name       string          `json:"name"`
	Dir        string          `json:"dir"`
	Node       json.RawMessage `json:"node"`
	Config     *int            `json:"config"`
	CSS        *int            `json:"css"`
	HTML       string          `json:"html"`
	Width      *float64        `json:"width"`
	Height     *float64        `json:"height"`
	Random     int             `json:"randomCalls"`
	LabelStyle *string         `json:"labelStyle"`
	Intersect  []any           `json:"intersects"`
	Error      string          `json:"error"`
	// Nodes and Sizes are set for a case that draws every node of a
	// diagram together: the nodes, and the width and height of each.
	Nodes []json.RawMessage `json:"nodes"`
	Sizes [][]*float64      `json:"sizes"`
}

// shapeGolden is the golden file of the requirementBox shape. The config
// and the style sheet of a case are indexes into the lists of the file.
type shapeGolden struct {
	Configs []json.RawMessage `json:"configs"`
	Styles  []string          `json:"styles"`
	Cases   []shapeCase       `json:"cases"`
}

func loadShapeGolden(tb testing.TB) *shapeGolden {
	tb.Helper()
	f, err := os.Open(filepath.Join("testdata", "requirementbox.json.gz"))
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		tb.Fatal(err)
	}
	defer zr.Close()
	var g shapeGolden
	if err := json.NewDecoder(zr).Decode(&g); err != nil {
		tb.Fatal(err)
	}
	return &g
}

// shapeRays are the points from which the probe calls intersect.
var shapeRays = [][2]float64{{-500, -300}, {500, -300}, {500, 300}, {-500, 300}, {0, -400}, {0, 400}, {-400, 0}, {400, 0}, {-37, 290}, {123, -45}, {1000, 10}, {-3, -1000}}

// shapeNode builds the layout node of a case. The model of a layout node
// has a numeric field for `type`. The type of a requirement or of an
// element is a string, which the db keeps in Extra.
func shapeNode(o *js.Obj[any]) *ru.Node {
	node := ru.NodeFromJS(o)
	if o != nil && o.Has("type") {
		if _, isNumber := o.Get("type").(float64); !isNumber {
			node.Type = ru.Undef
			node.Set("type", o.Get("type"))
		}
	}
	return node
}

// runShapeCase replays one case: it builds the same page, calls
// ru.InsertNode with the same node and config, and compares the markup, the
// size of the node, its label style, the intersections and the number of
// random draws with upstream. A case with several nodes draws them
// together with ctx.All, as a layout does, and compares the markup, the
// sizes and the number of random draws. The status is "exact",
// "equivalent" (numbers within 0.01), "different" or "error".
func runShapeCase(g *shapeGolden, c *shapeCase) (status, detail string) {
	defer func() {
		if p := recover(); p != nil {
			status, detail = "error", fmt.Sprintf("panic: %v", p)
		}
	}()
	if c.Config == nil || c.CSS == nil {
		// Upstream failed before it had a config.
		return "error", "the golden case has no config: " + c.Error
	}
	cfgVal, err := js.ParseJSON(g.Configs[*c.Config])
	cfg := js.AsObj(cfgVal)
	if err != nil || cfg == nil {
		return "error", fmt.Sprintf("bad config in golden: %v", err)
	}
	var nodeVal any
	if c.Nodes == nil {
		if nodeVal, err = js.ParseJSON(c.Node); err != nil {
			return "error", fmt.Sprintf("bad node in golden: %v", err)
		}
	}
	ctx := render.NewContext(render.ContextOptions{Config: cfg})
	ctx.Type = "requirement"
	// The probe builds its own svg content: a style element and one group.
	ctx.SVG.RemoveChildren()
	style := ctx.Doc.CreateElement("style")
	style.Text(g.Styles[*c.CSS])
	ctx.SVG.AppendChild(style)
	group := ctx.SVG.Append("g").Attr("class", "nodes")

	exact := true
	checkNum := func(what string, got float64, want *float64) string {
		if want == nil {
			if !ru.IsUndef(got) {
				return fmt.Sprintf("%s: got %v, want undefined", what, got)
			}
			return ""
		}
		if got != *want {
			exact = false
			if !(math.Abs(got-*want) <= 0.01) {
				return fmt.Sprintf("%s: got %v, want %v", what, got, *want)
			}
		}
		return ""
	}
	// checkMarkup compares the markup of the group and the number of
	// random draws. The shapes must draw as many random numbers as
	// upstream: the next number of the generator is then the same.
	checkMarkup := func() string {
		got := group.InnerHTML()
		if got != c.HTML {
			exact = false
			verdict, diff := parity.CompareSVG("<svg>"+got+"</svg>", "<svg>"+c.HTML+"</svg>", 0.01)
			if verdict == parity.Different {
				return "markup: " + diff.String()
			}
		}
		ref := render.NewContext(render.ContextOptions{Config: cfg})
		for i := 0; i < c.Random; i++ {
			ref.Random()
		}
		if ctx.Random() != ref.Random() {
			return fmt.Sprintf("the number of random draws is not %d", c.Random)
		}
		return ""
	}

	if c.Nodes != nil {
		// Every node of a diagram, drawn together as a layout draws them.
		nodes := make([]*ru.Node, len(c.Nodes))
		for i, raw := range c.Nodes {
			v, err := js.ParseJSON(raw)
			if err != nil {
				return "error", fmt.Sprintf("bad node in golden: %v", err)
			}
			nodes[i] = shapeNode(js.AsObj(v))
		}
		err := ctx.All(len(nodes), func(i int) error {
			_, err := ru.InsertNode(ctx, group, nodes[i], &ru.ShapeRenderOptions{Config: cfg, Dir: c.Dir})
			return err
		})
		if err != nil {
			return "error", err.Error()
		}
		if d := checkMarkup(); d != "" {
			return "different", d
		}
		if len(c.Sizes) != len(nodes) {
			return "error", "the golden case has not one size for each node"
		}
		for i, node := range nodes {
			if len(c.Sizes[i]) != 2 {
				return "error", "bad size in golden"
			}
			if d := checkNum(fmt.Sprintf("nodes[%d].width", i), node.Width, c.Sizes[i][0]); d != "" {
				return "different", d
			}
			if d := checkNum(fmt.Sprintf("nodes[%d].height", i), node.Height, c.Sizes[i][1]); d != "" {
				return "different", d
			}
		}
		if exact {
			return "exact", ""
		}
		return "equivalent", ""
	}

	node := shapeNode(js.AsObj(nodeVal))
	err = ctx.All(1, func(int) error {
		_, err := ru.InsertNode(ctx, group, node, &ru.ShapeRenderOptions{Config: cfg, Dir: c.Dir})
		return err
	})
	if c.Error != "" {
		if err == nil {
			return "different", "upstream fails: " + c.Error
		}
		if err.Error() != c.Error {
			return "different", fmt.Sprintf("error %q, want %q", err.Error(), c.Error)
		}
		return "exact", ""
	}
	if err != nil {
		return "error", err.Error()
	}

	if d := checkMarkup(); d != "" {
		return "different", d
	}
	if d := checkNum("node.width", node.Width, c.Width); d != "" {
		return "different", d
	}
	if d := checkNum("node.height", node.Height, c.Height); d != "" {
		return "different", d
	}
	if c.LabelStyle != nil && node.LabelStyle != *c.LabelStyle {
		return "different", fmt.Sprintf("node.labelStyle: got %q, want %q", node.LabelStyle, *c.LabelStyle)
	}
	node.X, node.Y = 0, 0
	for i, want := range c.Intersect {
		pair, ok := want.([]any)
		if !ok || len(pair) != 2 {
			return "different", fmt.Sprintf("intersect ray %d: upstream gives %v", i, want)
		}
		if node.Intersect == nil {
			return "different", "node.intersect is not set"
		}
		p := node.Intersect(ru.Point{X: shapeRays[i][0], Y: shapeRays[i][1]})
		wx, _ := pair[0].(float64)
		wy, _ := pair[1].(float64)
		if p.X != wx || p.Y != wy {
			exact = false
			if !(math.Abs(p.X-wx) <= 0.01 && math.Abs(p.Y-wy) <= 0.01) {
				return "different", fmt.Sprintf("intersect ray %d: got (%v, %v), want (%v, %v)", i, p.X, p.Y, wx, wy)
			}
		}
	}
	if exact {
		return "exact", ""
	}
	return "equivalent", ""
}

// knownShapeDivergences are the cases of the golden file for which this
// port has another result than upstream, each with the cause. The test
// fails when such a case agrees: remove it then.
var knownShapeDivergences = map[string]string{
	// addText gives each label the width calculateTextWidth(text, config)
	// + 50, and an HTML label has it as its max-width. calculateTextWidth
	// measures the text in the default font of the browser, a serif font:
	// the font family of the default config ends with a semicolon, so the
	// browser does not accept it. For a Han character Chromium takes the
	// fallback font from the serif list of the system, in which the
	// character is 16 px wide at 16 px. The font library of this port
	// (internal/font) has one fallback list for every font, in which it is
	// 16.328125 px wide. So the max-width of the name is 99px and not 98px.
	"requirementBox/cjk-neo": "the width of Han text in a serif font: internal/font has no serif CJK fallback",
}

// TestShapeAgainstMermaid replays the golden file of the shape
// requirementBox: cases recorded from the real insertNode of mermaid in
// Chromium by tools/reference/gen-requirement-shapes.mjs.
func TestShapeAgainstMermaid(t *testing.T) {
	g := loadShapeGolden(t)
	counts := map[string]int{}
	groups := map[string][2]int{}
	seen := map[string]bool{}
	skipped := 0
	for i := range g.Cases {
		c := &g.Cases[i]
		status, detail := runShapeCase(g, c)
		cause, known := knownShapeDivergences[c.Name]
		ok := status == "exact" || status == "equivalent"
		if known {
			seen[c.Name] = true
		}
		switch {
		case rutest.SkipComparison(c.HTML):
			// The case has text, and this machine does not have the fonts
			// of the reference machine: only an error fails the test.
			skipped++
			if status == "error" {
				t.Errorf("%s: %s\n    %s", c.Name, status, detail)
			}
		case known:
			if ok {
				t.Errorf("%s: %s, but it is a known divergence (%s): remove it from knownShapeDivergences", c.Name, status, cause)
			} else {
				status = "known-divergence"
			}
		case !ok:
			t.Errorf("%s: %s\n    %s", c.Name, status, detail)
		}
		counts[status]++
		group := "synthetic"
		switch {
		case strings.HasPrefix(c.Name, "corpus/"):
			group = "corpus"
		case strings.HasPrefix(c.Name, "together/"):
			group = "together"
		}
		n := groups[group]
		n[0]++
		if ok {
			n[1]++
		}
		groups[group] = n
	}
	for name := range knownShapeDivergences {
		if !seen[name] {
			t.Errorf("knownShapeDivergences has %q, which is not a case of the golden file", name)
		}
	}
	t.Logf("%d cases: exact=%d equivalent=%d known-divergence=%d different=%d error=%d",
		len(g.Cases), counts["exact"], counts["equivalent"], counts["known-divergence"], counts["different"], counts["error"])
	for _, group := range []string{"synthetic", "corpus", "together"} {
		t.Logf("%s: %d cases, %d exact or equivalent", group, groups[group][0], groups[group][1])
	}
	rutest.RequireFonts(t, skipped)
}
