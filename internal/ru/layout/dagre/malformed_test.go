package dagre

import (
	"fmt"
	"math"
	"runtime/debug"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
	rugraph "github.com/bvolpato/mermaid-go-renderer/internal/ru/graph"
)

// TestMalformedInput feeds broken layout data to the layout: the recorded
// layout data of real diagrams with random damage, and DOM stubs that
// return arbitrary sizes. The layout may return an error. It must not
// panic and must not hang.

// prng is the mulberry32 generator.
type prng struct{ a uint32 }

func (p *prng) next() float64 {
	p.a += 0x6d2b79f5
	t := p.a
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return float64(t^(t>>14)) / 4294967296
}

func (p *prng) intn(n int) int        { return int(p.next() * float64(n)) }
func (p *prng) chance(c float64) bool { return p.next() < c }

func pickOf[T any](p *prng, list []T) T { return list[p.intn(len(list))] }

var oddSizes = []float64{math.NaN(), 0, -5, 1e6, 1e-9, math.Inf(1), 17.5, 40, 120.25}

// damage changes the layout data in place.
func damage(data *ru.LayoutData, config *js.Obj[any], p *prng) {
	ids := []string{"", "nowhere", "constructor", "__proto__", "1"}
	for _, node := range data.Nodes {
		ids = append(ids, node.ID)
	}
	anyID := func() string { return pickOf(p, ids) }
	// The nil entries come last, so that the other changes need not look
	// for them.
	nilNode, nilEdge := false, false
	defer func() {
		if nilNode {
			data.Nodes = append(data.Nodes, nil)
		}
		if nilEdge {
			data.Edges = append(data.Edges, nil)
		}
	}()
	for n := 1 + p.intn(4); n > 0; n-- {
		switch p.intn(14) {
		case 0:
			if len(data.Nodes) > 0 {
				i := p.intn(len(data.Nodes))
				data.Nodes = append(data.Nodes[:i:i], data.Nodes[i+1:]...)
			}
		case 1:
			if len(data.Nodes) > 0 {
				// An unknown parent, the node itself, or any node: a
				// parent cycle is possible.
				pickOf(p, data.Nodes).ParentID = anyID()
			}
		case 2:
			if len(data.Nodes) > 1 {
				a, b := pickOf(p, data.Nodes), pickOf(p, data.Nodes)
				a.ParentID, b.ParentID = b.ID, a.ID
			}
		case 3:
			if len(data.Nodes) > 0 {
				c := *pickOf(p, data.Nodes)
				data.Nodes = append(data.Nodes, &c)
			}
		case 4:
			edge := ru.NewEdge(pickOf(p, []string{"", "e", "L_new", "1"}))
			edge.Start, edge.End = anyID(), anyID()
			if p.chance(0.4) {
				edge.End = edge.Start
			}
			if p.chance(0.5) {
				edge.Label, edge.HasLabel = "label", true
			}
			data.Edges = append(data.Edges, edge)
		case 5:
			if len(data.Edges) > 0 {
				edge := pickOf(p, data.Edges)
				edge.Start, edge.End = anyID(), anyID()
			}
		case 6:
			if len(data.Edges) > 0 {
				pickOf(p, data.Edges).Minlen = pickOf(p, []float64{0, -1, math.NaN(), 3, 0.5})
			}
		case 7:
			if len(data.Nodes) > 0 {
				node := pickOf(p, data.Nodes)
				node.ClusterNode = true
			}
		case 8:
			if len(data.Nodes) > 0 {
				node := pickOf(p, data.Nodes)
				node.IsGroup = !node.IsGroup
			}
		case 9:
			if p.chance(0.5) {
				nilNode = true
			} else {
				nilEdge = true
			}
		case 10:
			data.Direction = pickOf(p, []string{"", "XX", "LR", "BT", "tb"})
			data.Type = pickOf(p, []string{"", "flowchart-v2", "requirement", "stateDiagram"})
		case 11:
			if len(data.Edges) > 0 {
				edge := pickOf(p, data.Edges)
				edge.ID = pickOf(p, []string{"", "dup", "dup"})
				edge.Labelpos = pickOf(p, []string{"", "c", "l", "x"})
			}
		case 12:
			flowchart := js.AsObj(config.Get("flowchart"))
			margin := js.NewObj[any]()
			margin.Set("top", pickOf[any](p, []any{-3.0, math.NaN(), "7", 1e9, nil}))
			margin.Set("bottom", pickOf[any](p, []any{-3.0, math.NaN(), "7", 0.0, nil}))
			flowchart.Set("subGraphTitleMargin", margin)
		case 13:
			data.NodeSpacing = pickOf(p, oddSizes)
			data.RankSpacing = pickOf(p, oddSizes)
			config.Set("nodeSpacing", pickOf[any](p, []any{nil, "x", -1.0, js.Null}))
		}
	}
}

// oddOps are DOM stubs that write arbitrary sizes.
func oddOps(p *prng) *ops {
	size := func() float64 {
		if p.chance(0.7) {
			return 10 + math.Floor(p.next()*100)
		}
		return pickOf(p, oddSizes)
	}
	return &ops{
		createLayoutElementGroups: func(*scope, *dom.Node, rugraph.CreateLayoutElementGroupsOptions) *rugraph.LayoutElementGroups {
			return &rugraph.LayoutElementGroups{}
		},
		insertMeasuredNode: func(_ *scope, _ *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) error {
			node.Width, node.Height = size(), size()
			return nil
		},
		insertEdgeLabel: func(_ *scope, _ *dom.Node, edge *ru.Edge) error {
			edge.Width, edge.Height = size(), size()
			return nil
		},
		updateNodeBounds: func(_ *scope, node *ru.Node, _ *dom.Node) {
			node.Width, node.Height = size(), size()
		},
		setNodeElem: func(*scope, *dom.Node, *ru.Node) {},
		insertCluster: func(_ *scope, _ *dom.Node, node *ru.Node) error {
			node.Width, node.Height, node.Diff = size(), size(), pickOf(p, []float64{math.NaN(), 0, -8, 3.5})
			return nil
		},
		positionNode: func(*scope, *ru.Node) {},
		insertEdge: func(_ *scope, _ *dom.Node, edge *ru.Edge, clusterDb edges.ClusterDb, _ string, _, _ *ru.Node, _ string) (*edges.Paths, error) {
			if err := clusterLookups(edge, clusterDb); err != nil {
				return nil, err
			}
			return &edges.Paths{OriginalPath: edge.Points, HasOriginalPath: true}, nil
		},
		positionEdgeLabel: func(*scope, *ru.Edge, *edges.Paths) error { return nil },
	}
}

// runAll runs the phases of the layout as the common renderer does.
func runAll(data *ru.LayoutData, config *js.Obj[any], p *prng) error {
	st := newTestState()
	st.ops = oddOps(p)
	ctx := &render.Context{ID: data.DiagramID, Config: config}
	st.ctx = ctx
	ctx.Value(stateKey, func() any { return st })
	Clear(ctx)
	rc := &rugraph.RenderContext{Ctx: ctx}
	prepared, err := PrepareLayoutForDagre(ctx, data)
	if err != nil {
		return err
	}
	rc.PreparedLayout = prepared
	measure, err := MeasureDagreLayout(data, rc)
	if err != nil {
		return err
	}
	if _, err := RunDagreLayoutCore(data, rc); err != nil {
		return err
	}
	pc := &rugraph.PaintContext{RenderContext: *rc, Measure: measure}
	clusterDb := paintOptions.ClusterDb(pc)
	for _, node := range paintOptions.GetNodes(data, pc) {
		_ = paintOptions.SkipNode(node, pc)
		_ = paintOptions.IsCluster(node, pc)
	}
	// The layout phase replaced the edges of the layout data.
	for _, edge := range data.Edges {
		paintOptions.GetEdgeNode(edge.Start, edge, pc)
		paintOptions.GetEdgeNode(edge.End, edge, pc)
		if err := clusterLookups(edge, clusterDb); err != nil {
			return err
		}
	}
	return nil
}

func TestMalformedInput(t *testing.T) {
	var file recordFile
	loadJSONGz(t, "testdata/layout-record.json.gz", &file)

	const rounds = 6
	p := &prng{a: 0x6d616c66} // "malf"
	cases, failures, panics := 0, 0, 0
	for _, fx := range file.Fixtures {
		input, ok := fx["input"].(map[string]any)
		if !ok {
			continue
		}
		for round := 0; round < rounds; round++ {
			data, config := inputData(input)
			damage(data, config, p)
			cases++
			type result struct {
				err   error
				panic string
			}
			done := make(chan result, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						done <- result{panic: fmt.Sprintf("%v\n%s", r, debug.Stack())}
					}
				}()
				done <- result{err: runAll(data, config, p)}
			}()
			select {
			case res := <-done:
				if res.panic != "" {
					panics++
					if panics <= 5 {
						t.Errorf("%s round %d: panic: %s", decStr(fx, "name"), round, res.panic)
					}
				} else if res.err != nil {
					failures++
				}
			case <-time.After(60 * time.Second):
				t.Fatalf("%s round %d: the layout hangs", decStr(fx, "name"), round)
			}
		}
	}
	t.Logf("%d damaged inputs: %d errors, %d panics", cases, failures, panics)
}
