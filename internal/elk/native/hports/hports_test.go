package hports_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/compound/compoundtest"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/hports"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// sharedMinimum restores an object identity that a stored state does not
// have. ElkGraphImporter copies the properties of one ElkNode to the LNode
// and to its nested LGraph. So for an ElkNode that has the option
// nodeSize.minimum, both have the same KVector object.
// HierarchicalNodeResizingProcessor changes that vector of the graph in
// place, and the node then has the new value too. nativetest.Build makes one
// vector for each holder. This wrapper gives a graph and its parent node one
// vector again, then it runs the processor.
//
// The two have one vector in elkjs when the ELK node of the case has the
// option (without it, each of the two has its own copy of the default
// value) and the graph of the node has a horizontal direction (for a
// vertical direction, GraphTransformer gave the node a new vector with the
// coordinates exchanged).
//
// TODO(core): store the identity of shared property values in the state.
type sharedMinimum struct {
	lgraph.Processor
	// explicit has the identifiers of the ELK nodes with the option.
	explicit map[string]bool
}

func (s sharedMinimum) Process(graph *lgraph.LGraph, ctx *lgraph.Context) {
	if node := graph.ParentNode; node != nil && prop.Has(node, lopts.NodeSizeMinimum) && prop.Has(graph, lopts.NodeSizeMinimum) {
		origin, isElk := prop.Get(node, iprops.Origin).(lgraph.OriginRef)
		if isElk && s.explicit[origin.ElkIdentifier()] && !prop.Get(node.Graph(), lopts.Direction).IsVertical() {
			prop.Set(graph, lopts.NodeSizeMinimum, prop.Get(node, lopts.NodeSizeMinimum))
		}
	}
	s.Processor.Process(graph, ctx)
}

// explicitMinimum returns the identifiers of the nodes of the ELK graph of
// a case that have the option nodeSize.minimum.
func explicitMinimum(t *testing.T, c *nativetest.Case) map[string]bool {
	type elkNode struct {
		ID            string         `json:"id"`
		LayoutOptions map[string]any `json:"layoutOptions"`
		Children      []*elkNode     `json:"children"`
	}
	root := &elkNode{}
	if err := json.Unmarshal(c.In, root); err != nil {
		t.Fatalf("%s: %v", c.Name, err)
	}
	out := map[string]bool{}
	var walk func(n *elkNode)
	walk = func(n *elkNode) {
		for key := range n.LayoutOptions {
			if key == "nodeSize.minimum" || strings.HasSuffix(key, ".nodeSize.minimum") {
				out[n.ID] = true
			}
		}
		for _, child := range n.Children {
			walk(child)
		}
	}
	walk(root)
	return out
}

func TestHierarchicalNodeResizingProcessor(t *testing.T) {
	nativetest.RunProcessor(t, "HierarchicalNodeResizingProcessor", func() lgraph.Processor { return hports.NewHierarchicalNodeResizingProcessor() })
}

func TestInvertedPortProcessor(t *testing.T) {
	nativetest.RunProcessor(t, "InvertedPortProcessor", func() lgraph.Processor { return hports.NewInvertedPortProcessor() })
}

func TestHierarchicalPortConstraintProcessor(t *testing.T) {
	nativetest.RunProcessor(t, "HierarchicalPortConstraintProcessor", func() lgraph.Processor { return hports.NewHierarchicalPortConstraintProcessor() })
}

func TestHierarchicalPortDummySizeProcessor(t *testing.T) {
	nativetest.RunProcessor(t, "HierarchicalPortDummySizeProcessor", func() lgraph.Processor { return hports.NewHierarchicalPortDummySizeProcessor() })
}

func TestHierarchicalPortPositionProcessor(t *testing.T) {
	nativetest.RunProcessor(t, "HierarchicalPortPositionProcessor", func() lgraph.Processor { return hports.NewHierarchicalPortPositionProcessor() })
}

func TestHierarchicalPortOrthogonalEdgeRouter(t *testing.T) {
	nativetest.RunProcessor(t, "HierarchicalPortOrthogonalEdgeRouter", func() lgraph.Processor { return hports.NewHierarchicalPortOrthogonalEdgeRouter() })
}

// TestHierarchyGoldens checks the processors against the extra goldens of
// tools/reference/gen-elk-hierarchy.mjs: graphs with hierarchical ports on
// each side and with each port constraint, port labels, minimum sizes,
// content alignment and inverted ports, which mermaid does not make.
func TestHierarchyGoldens(t *testing.T) {
	cases, err := compoundtest.LoadCases(filepath.Join("testdata", "hports-steps.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	plain := compoundtest.Factory
	compoundtest.RunCases(t, cases, map[string]compoundtest.CaseFactory{
		"HierarchicalNodeResizingProcessor": func(c *nativetest.Case) lgraph.Processor {
			return sharedMinimum{hports.NewHierarchicalNodeResizingProcessor(), explicitMinimum(t, c)}
		},
		"InvertedPortProcessor":                plain(func() lgraph.Processor { return hports.NewInvertedPortProcessor() }),
		"HierarchicalPortConstraintProcessor":  plain(func() lgraph.Processor { return hports.NewHierarchicalPortConstraintProcessor() }),
		"HierarchicalPortDummySizeProcessor":   plain(func() lgraph.Processor { return hports.NewHierarchicalPortDummySizeProcessor() }),
		"HierarchicalPortPositionProcessor":    plain(func() lgraph.Processor { return hports.NewHierarchicalPortPositionProcessor() }),
		"HierarchicalPortOrthogonalEdgeRouter": plain(func() lgraph.Processor { return hports.NewHierarchicalPortOrthogonalEdgeRouter() }),
	})
}
