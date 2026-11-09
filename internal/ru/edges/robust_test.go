package edges_test

import (
	"math"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
)

// mulberry32 is the seeded generator of the reference oracle.
type mulberry32 uint32

func (m *mulberry32) next() float64 {
	*m += 0x6d2b79f5
	t := uint32(*m)
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return float64(float64(t^(t>>14)) / 4294967296)
}

// TestNoPanic feeds degenerate and random edges to the exported functions.
// A function may return an error, as upstream throws. It must not panic.
func TestNoPanic(t *testing.T) {
	rnd := mulberry32(0x6e6f7061)
	special := []float64{0, -0.0, 1, -1, 1e-9, 1e9, math.NaN(), math.Inf(1), math.Inf(-1), 5, 10.5}
	coord := func() float64 {
		if rnd.next() < 0.25 {
			return special[int(rnd.next()*float64(len(special)))]
		}
		return (rnd.next() - 0.3) * 300
	}
	pick := func(list []string) string { return list[int(rnd.next()*float64(len(list)))] }
	arrows := []string{"", "none", "arrow_point", "arrow_circle", "arrow_cross", "arrow_barb", "arrow_barb_neo", "aggregation", "extension", "composition", "dependency", "lollipop", "only_one", "requirement_arrow", "bogus"}
	curves := []string{"", "linear", "basis", "cardinal", "bumpX", "bumpY", "catmullRom", "monotoneX", "monotoneY", "natural", "step", "stepAfter", "stepBefore", "rounded", "bogus"}
	looks := []string{"", "classic", "neo", "handDrawn"}
	layouts := []string{"", "dagre", "elk", "swimlane"}
	texts := []string{"", "a", "two<br/>lines", "**md** `x`", "<script>alert(1)</script>", "fa:fa-car x", "é中\U0001F600", "a < b & c"}

	cfg := js.NewObj[any]()
	cfg.Set("fontFamily", "trebuchet ms")
	cfg.Set("themeVariables", js.NewObj[any]())
	for i := 0; i < 400; i++ {
		ctx := render.NewContext(render.ContextOptions{Config: cfg})
		ctx.Type = "flowchart-v2"
		root := ctx.SVG.Append("g")
		if err := edges.InsertMarkers(ctx, root, []string{"point", "circle", "cross", "barbNeo", "only_one_neo", "requirement_contains_neo"}, "flowchart-v2", "my-svg"); err != nil {
			t.Fatal(err)
		}
		edgePaths := root.Insert("g").Attr("class", "edgePaths")
		edgeLabels := root.Insert("g").Attr("class", "edgeLabels")

		e := ru.NewEdge(pick([]string{"", "e1", "x-to-label", "a b\"<"}))
		n := int(rnd.next() * 7)
		if rnd.next() < 0.1 {
			n = 0
		}
		for k := 0; k < n; k++ {
			e.Points = append(e.Points, ru.Point{X: coord(), Y: coord()})
		}
		e.ArrowTypeStart, e.ArrowTypeEnd = pick(arrows), pick(arrows)
		e.Curve, e.Look = pick(curves), pick(looks)
		e.Label, e.LabelType = pick(texts), pick([]string{"", "text", "markdown"})
		e.StartLabelLeft, e.EndLabelRight = pick(texts), pick(texts)
		e.StartLabelRight, e.EndLabelLeft = pick(texts), pick(texts)
		e.Pattern, e.Thickness = pick([]string{"", "solid", "dotted", "dashed"}), pick([]string{"", "normal", "thick", "invisible"})
		e.Style = []string{pick([]string{"", "stroke:red", "stroke:", "fill:none;stroke: #fff", ";;;"})}
		e.StyleUndefined = rnd.next() < 0.2
		e.LabelStyle = []string{pick([]string{"", "color:red", "nocolon", ":"})}
		e.CSSCompiledStyles = []string{pick([]string{"", "stroke:blue", "animation: x"})}
		e.Animate, e.Animation = rnd.next() < 0.2, pick([]string{"", "fast", "slow"})
		e.ShowPoints, e.IsLabelEdge = rnd.next() < 0.2, rnd.next() < 0.3
		e.ToCluster, e.FromCluster = pick([]string{"", "", "c1", "nope"}), pick([]string{"", "", "c1", "nope"})
		e.X, e.Y = coord(), coord()

		node := func() *ru.Node {
			switch int(rnd.next() * 3) {
			case 0:
				return nil
			case 1:
				return ru.NewNode("n")
			}
			nd := ru.NewNode("n")
			b := ru.Bounds{X: coord(), Y: coord(), Width: coord(), Height: coord()}
			nd.Intersect = func(p ru.Point) ru.Point { return ru.IntersectRect(b, p) }
			return nd
		}
		clusterDb := func(id string) (ru.Bounds, bool) {
			return ru.Bounds{X: coord(), Y: coord(), Width: coord(), Height: coord()}, id == "c1"
		}

		if rnd.next() < 0.7 {
			if rnd.next() < 0.5 {
				_, _ = edges.InsertEdgeLabel(ctx, edgeLabels, e)
			} else {
				_ = edges.InsertEdgeLabels(ctx, edgeLabels, []*ru.Edge{e, e})
			}
		}
		paths, err := edges.InsertEdge(ctx, edgePaths, e, clusterDb, "flowchart-v2", node(), node(), pick([]string{"my-svg", "my-svg", ""}), rnd.next() < 0.5, pick(layouts))
		if err != nil {
			paths = nil
		}
		_ = edges.PositionEdgeLabel(ctx, e, paths)
		_, _ = edges.ResolveEdgeLabelPosition(e, paths)

		geoms := []edges.EdgeGeom{
			{ID: e.ID, Points: e.Points, Curve: e.Curve, ArrowTypeStart: e.ArrowTypeStart, ArrowTypeEnd: e.ArrowTypeEnd},
			{ID: "other", Points: []ru.Point{{X: coord(), Y: coord()}, {X: coord(), Y: coord()}, {X: coord(), Y: coord()}}},
		}
		jump := edges.LineJumpConfig{Enabled: true, JumpRadius: coord(), JumpStyle: pick([]string{"arc", "gap", ""})}
		_ = edges.FindEdgeIntersections(geoms)
		_ = edges.ProcessEdgesWithJumps(geoms, jump)
		edges.ApplyLineJumpsToSvg(edgePaths, geoms, jump)
		edgePaths.SelectAll("path").Attr("data-points", pick([]string{"", "!!!", "e30=", "W3sieCI6MX1d", "bnVsbA=="}))
		edges.ApplyLineJumpsToSvg(edgePaths, geoms, jump)

		_ = edges.GenerateRoundedPath(e.Points, coord())
		_ = edges.ApplyMarkerOffsetsToPoints(e.Points, e.ArrowTypeStart, e.ArrowTypeEnd)
		_ = edges.Intersection(ru.Bounds{X: coord(), Y: coord(), Width: coord(), Height: coord()}, ru.Point{X: coord(), Y: coord()}, ru.Point{X: coord(), Y: coord()})
		edges.Clear(ctx)
		_ = edges.PositionEdgeLabel(ctx, e, paths)
		_ = root.OuterHTML()
	}
}

// TestNilArguments checks the calls with nil elements and nil nodes.
func TestNilArguments(t *testing.T) {
	cfg := js.NewObj[any]()
	cfg.Set("fontFamily", "trebuchet ms")
	ctx := render.NewContext(render.ContextOptions{Config: cfg})
	e := ru.NewEdge("e")
	e.Label = "x"
	e.Points = []ru.Point{{X: 0, Y: 0}, {X: 10, Y: 10}}
	if _, err := edges.InsertEdge(ctx, nil, e, nil, "t", nil, nil, "id", false, ""); err == nil {
		t.Error("InsertEdge into a nil element: want an error")
	}
	e.StartLabelLeft = "s"
	if _, err := edges.InsertEdgeLabel(ctx, nil, e); err == nil {
		t.Error("InsertEdgeLabel into a nil element: want an error")
	}
	if err := edges.InsertEdgeLabels(ctx, nil, []*ru.Edge{e}); err == nil {
		t.Error("InsertEdgeLabels into a nil element: want an error")
	}
	_ = edges.InsertMarkers(ctx, nil, []string{"point"}, "t", "id")
	_ = edges.PositionEdgeLabel(ctx, e, nil)
	edges.ApplyLineJumpsToSvg(nil, nil, edges.LineJumpConfig{Enabled: true})
	edges.AddEdgeMarkers(ctx, nil, "arrow_point", "arrow_point", "", "id", "t", true, "red")
	if got := edges.GetLabelStyles(nil); got != "" {
		t.Errorf("GetLabelStyles(nil) = %q", got)
	}
}
