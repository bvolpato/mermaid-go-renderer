package routing_test

import (
	"os"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops/loopstest"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops/routing"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func TestSelfLoopRouter(t *testing.T) {
	loopstest.RunProcessor(t, "SelfLoopRouter", func() lgraph.Processor { return routing.NewSelfLoopRouter() })
}

// hasSelfLoops tells that the self loop router has a node of the graph to
// route.
func hasSelfLoops(graph *lgraph.LGraph) bool {
	for _, layer := range graph.Layers {
		for _, node := range layer.Nodes {
			if node.Type == lgraph.NodeTypeNormal && prop.Has(node, iprops.SelfLoopHolder) {
				return true
			}
		}
	}
	return false
}

// TestSelfLoopRouterStops checks the cases that the port does not cover: the
// polyline and the spline self loop router, and a label manager. The router
// must stop the layout for a graph that has self loops to route, and must
// not stop it for a graph without.
func TestSelfLoopRouterStops(t *testing.T) {
	changes := []struct {
		name  string
		apply func(graph *lgraph.LGraph)
		// always tells that the layout stops without self loops too.
		always bool
	}{
		{"POLYLINE", func(graph *lgraph.LGraph) { prop.Set(graph, lopts.EdgeRouting, core.EdgeRoutingPolyline) }, false},
		{"SPLINES", func(graph *lgraph.LGraph) { prop.Set(graph, lopts.EdgeRouting, core.EdgeRoutingSplines) }, false},
		{"label manager", func(graph *lgraph.LGraph) { prop.SetByID(graph, "org.eclipse.elk.labelManager", "manager") }, true},
	}
	withLoops, without := 0, 0
	for _, c := range loopstest.Cases(t) {
		if withLoops >= 12 && without >= 3 {
			break
		}
		for _, run := range c.Runs {
			for _, step := range run.Steps {
				if step.Proc != "SelfLoopRouter" || step.Before == nil {
					continue
				}
				// build makes the graph of the step again for each change.
				build := func() *lgraph.LGraph {
					state, err := run.State(*step.Before)
					if err != nil {
						t.Fatal(err)
					}
					built, err := nativetest.Build(state)
					if err != nil {
						t.Fatalf("%s: %v", c.Name, err)
					}
					return built.Graphs[step.Graph]
				}
				loops := hasSelfLoops(build())
				if loops {
					withLoops++
				} else {
					without++
				}
				for _, change := range changes {
					graph := build()
					change.apply(graph)
					err := nativetest.Process(routing.NewSelfLoopRouter(), graph, lgraph.NewContext())
					switch {
					case loops || change.always:
						if err == nil || !strings.Contains(err.Error(), "is not ported") {
							t.Errorf("%s, %s: the router did not stop the layout: %v", c.Name, change.name, err)
						}
					case err != nil:
						t.Errorf("%s, %s: the router stopped the layout of a graph without self loops: %v", c.Name, change.name, err)
					}
				}
			}
		}
	}
	t.Logf("%d graphs with self loops and %d graphs without, each with %d changes", withLoops, without, len(changes))
	if (withLoops == 0 || without == 0) && os.Getenv("ELK_PHASE_CASE") == "" {
		t.Fatalf("the goldens have %d steps of the self loop router for a graph with self loops and %d for a graph without, want both", withLoops, without)
	}
}
