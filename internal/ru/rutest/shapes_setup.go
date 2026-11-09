package rutest

import (
	"fmt"
	"math"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// ShapeSetup prepares the context of a shape case before the node is
// inserted. It stands for what a probe does in the page before it calls
// insertNode, for example mermaid.registerIconPacks.
type ShapeSetup func(ctx *render.Context, c ShapeCase) error

// RunShapeCaseSetup is RunShapeCase with a set-up step. The comparison is
// the same: the markup, the node size and the intersections. The second
// result is the markup that the shape made, for a test that looks at a
// part of it.
func RunShapeCaseSetup(c ShapeCase, setup ShapeSetup) (res ShapeResult, got string) {
	res.Name = c.Name
	defer func() {
		if p := recover(); p != nil {
			res.Status, res.Detail = "error", fmt.Sprintf("panic: %v", p)
		}
	}()
	cfgVal, err := js.ParseJSON(c.Config)
	if err != nil || js.AsObj(cfgVal) == nil {
		return ShapeResult{c.Name, "error", fmt.Sprintf("bad config in golden: %v", err)}, ""
	}
	cfg := js.AsObj(cfgVal)
	nodeVal, err := js.ParseJSON(c.Node)
	if err != nil {
		return ShapeResult{c.Name, "error", fmt.Sprintf("bad node in golden: %v", err)}, ""
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

	if setup != nil {
		if err := setup(ctx, c); err != nil {
			return ShapeResult{c.Name, "error", fmt.Sprintf("set-up: %v", err)}, ""
		}
	}

	node := ru.NodeFromJS(js.AsObj(nodeVal))
	_, err = ru.InsertNode(ctx, g, node, &ru.ShapeRenderOptions{Config: cfg, Dir: c.Dir})
	if c.Error != "" {
		if err != nil {
			return ShapeResult{c.Name, "exact", ""}, ""
		}
		return ShapeResult{c.Name, "different", "upstream fails: " + c.Error}, g.InnerHTML()
	}
	if err != nil {
		if _, ok := err.(*ru.NotPortedError); ok {
			return ShapeResult{c.Name, "not-ported", err.Error()}, ""
		}
		return ShapeResult{c.Name, "error", err.Error()}, ""
	}

	exact := true
	got = g.InnerHTML()
	if got != c.HTML {
		exact = false
		verdict, diff := parity.CompareSVG("<svg>"+got+"</svg>", "<svg>"+c.HTML+"</svg>", 0.01)
		if verdict == parity.Different {
			return ShapeResult{c.Name, "different", "markup: " + diff.String()}, got
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
		return ShapeResult{c.Name, "different", d}, got
	}
	if d := checkNum("node.height", node.Height, c.Height); d != "" {
		return ShapeResult{c.Name, "different", d}, got
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
				return ShapeResult{c.Name, "different", fmt.Sprintf("intersect ray %d: got (%v, %v), want (%v, %v)", i, p.X, p.Y, wx, wy)}, got
			}
		}
	}
	if exact {
		return ShapeResult{c.Name, "exact", ""}, got
	}
	return ShapeResult{c.Name, "equivalent", ""}, got
}

// RunShapeGoldenSetup is RunShapeGolden with a set-up step for each case.
func RunShapeGoldenSetup(t *testing.T, path string, setup ShapeSetup) {
	t.Helper()
	cases := LoadShapeCases(t, path)
	counts := map[string]int{}
	skipped := 0
	for _, c := range cases {
		r, _ := RunShapeCaseSetup(c, setup)
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
	RequireFonts(t, skipped)
}
