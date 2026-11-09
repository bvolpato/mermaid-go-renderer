package graph_test

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/graph"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/rutest"
)

// malformedData builds layout data with missing and odd values from a
// seeded generator.
func malformedData(rnd func() float64) *ru.LayoutData {
	ids := []string{"", "a", "b", "c", "10", "2", "constructor", "__proto__", "a"}
	shapes := []string{"", "rect", "squareRect", "roundedRect", "noSuchShape", "roundedWithTitle", "divider", "noteGroup"}
	labels := []string{"", "Label", "<b>x</b> & **y**", "a\nb<br/>c", "fa:fa-car"}
	values := []float64{math.NaN(), 0, -1, 12.5, 250, math.Inf(1)}
	pick := func(list []string) string { return list[int(rnd()*float64(len(list)))] }
	num := func() float64 { return values[int(rnd()*float64(len(values)))] }
	data := &ru.LayoutData{Type: "flowchart-v2", DiagramID: "my-svg", Direction: pick([]string{"", "TB", "LR", "x"})}
	if rnd() < 0.7 {
		data.Markers = []string{"point", "circle", "cross"}
	}
	for i, n := 0, int(rnd()*7); i < n; i++ {
		node := ru.NewNode(pick(ids))
		node.Shape, node.Label, node.ParentID = pick(shapes), pick(labels), pick(ids)
		node.IsGroup = rnd() < 0.3
		node.Look = pick([]string{"", "classic", "neo", "handDrawn"})
		node.Padding, node.Width, node.Height = num(), num(), num()
		if rnd() < 0.3 {
			node.LabelType = "markdown"
		}
		if rnd() < 0.3 {
			node.Dir = "LR"
		}
		if rnd() < 0.2 {
			node.ClusterNode = true
		}
		data.Nodes = append(data.Nodes, node)
	}
	for i, n := 0, int(rnd()*5); i < n; i++ {
		edge := ru.NewEdge(pick(ids))
		edge.Start, edge.End = pick(ids), pick(ids)
		edge.Label = pick(labels)
		edge.IsLayoutOnly = rnd() < 0.1
		if rnd() < 0.3 {
			edge.StartLabelLeft, edge.EndLabelRight = "1", "*"
		}
		if rnd() < 0.5 {
			edge.Points = []ru.Point{{X: num(), Y: num()}, {X: num(), Y: num()}}
		}
		data.Edges = append(data.Edges, edge)
	}
	return data
}

// TestMalformedLayoutData feeds layout data with missing and odd values to
// CreateGraphWithElements and to layouts made with
// CreateCommonLayoutRenderer. The functions must return, with or without
// an error: they must not panic and must not hang.
func TestMalformedLayoutData(t *testing.T) {
	var g rutest.GraphGolden
	rutest.LoadGolden(t, "testdata/graph.json.gz", &g)
	cfg, err := rutest.PatchedConfig(g.BaseConfig, nil)
	if err != nil {
		t.Fatal(err)
	}
	seed := uint32(0x67726166)
	rnd := func() float64 {
		seed += 0x6d2b79f5
		x := seed
		x = (x ^ (x >> 15)) * (x | 1)
		x ^= x + (x^(x>>7))*(x|61)
		return float64(x^(x>>14)) / 4294967296
	}
	failing := errors.New("phase failed")
	definitions := []graph.Definition{
		{RunLayoutCore: func(d *ru.LayoutData, _ *graph.RenderContext) (any, error) { return toyCore(d), nil }},
		{RunLayoutCore: func(*ru.LayoutData, *graph.RenderContext) (any, error) { return nil, nil }},
		{},
		{
			MeasureLayout: func(*ru.LayoutData, *graph.RenderContext) (graph.Measure, error) { return nil, nil },
			RunLayoutCore: func(*ru.LayoutData, *graph.RenderContext) (any, error) { return nil, nil },
		},
		{RunLayoutCore: func(*ru.LayoutData, *graph.RenderContext) (any, error) { return nil, failing }},
		{
			PrepareLayout: func(*ru.LayoutData, *graph.RenderContext) (any, error) { return nil, failing },
			RunLayoutCore: func(*ru.LayoutData, *graph.RenderContext) (any, error) { return nil, nil },
		},
		{
			RunLayoutCore: func(d *ru.LayoutData, _ *graph.RenderContext) (any, error) { return toyCore(d), nil },
			PaintOptions: &graph.PaintOptions{
				GetNodes:    func(*ru.LayoutData, *graph.PaintContext) []*ru.Node { return nil },
				GetEdgeNode: func(string, *ru.Edge, *graph.PaintContext) *ru.Node { return nil },
				IsCluster:   func(*ru.Node, *graph.PaintContext) bool { return false },
			},
		},
	}
	count := 0
	for i := 0; i < 150; i++ {
		data := malformedData(rnd)
		data.Config = cfg
		def := definitions[i%len(definitions)]
		detached := i%11 == 0
		count++
		done := make(chan string, 1)
		go func() {
			defer func() {
				if p := recover(); p != nil {
					done <- fmt.Sprintf("panic: %v", p)
				}
			}()
			ctx := rutest.NewProbeContext(js.AsObj(js.Clone(cfg)), g.CSS[0], "flowchart-v2")
			root := ctx.SVG.Append("g")
			if i%2 == 0 {
				element := root
				if detached {
					element = nil
				}
				_, _ = graph.CreateGraphWithElements(ctx, element, data, graph.CreateGraphOptions{UnwrapGroupLabels: i%4 == 0})
			} else {
				_ = graph.CreateCommonLayoutRenderer(def)(ctx, data, ctx.SVG, "")
			}
			done <- ""
		}()
		select {
		case msg := <-done:
			if msg != "" {
				t.Errorf("layout data %d: %s", i, msg)
			}
		case <-time.After(30 * time.Second):
			t.Fatalf("layout data %d: no result after 30 s", i)
		}
	}
	t.Logf("%d malformed layouts", count)
}

// TestPaintLayoutDataErrors checks the errors that stand for the TypeErrors
// of upstream.
func TestPaintLayoutDataErrors(t *testing.T) {
	var g rutest.GraphGolden
	rutest.LoadGolden(t, "testdata/graph.json.gz", &g)
	cfg, err := rutest.PatchedConfig(g.BaseConfig, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := rutest.NewProbeContext(cfg, g.CSS[0], "flowchart-v2")
	rc := graph.RenderContext{Ctx: ctx, Element: ctx.SVG.Append("g")}
	data := &ru.LayoutData{Config: cfg}
	if err := graph.PaintLayoutData(data, &graph.PaintContext{RenderContext: rc}, nil); err == nil {
		t.Error("PaintLayoutData without a measure result: no error")
	}
	groups := graph.CreateLayoutElementGroups(rc.Element)
	if err := graph.PaintLayoutData(data, &graph.PaintContext{RenderContext: rc, Measure: groups}, nil); err != nil {
		t.Errorf("PaintLayoutData with empty layout data: %v", err)
	}
	if err := graph.CreateCommonLayoutRenderer(graph.Definition{})(ctx, data, ctx.SVG, ""); err == nil {
		t.Error("a layout without RunLayoutCore: no error")
	}
}
