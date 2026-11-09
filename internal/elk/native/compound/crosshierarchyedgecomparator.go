// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/compound/CrossHierarchyEdgeComparator.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package compound

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
)

// CrossHierarchyEdgeComparator is the class CrossHierarchyEdgeComparator: it
// orders the segments of a cross-hierarchy edge from the start segment to the
// end segment.
type CrossHierarchyEdgeComparator struct {
	// graph is the top-level compound graph.
	graph *lgraph.LGraph
}

// NewCrossHierarchyEdgeComparator is new CrossHierarchyEdgeComparator(graph)
// for the top-level compound graph.
func NewCrossHierarchyEdgeComparator(graph *lgraph.LGraph) *CrossHierarchyEdgeComparator {
	return &CrossHierarchyEdgeComparator{graph: graph}
}

// Compare is compare(edge1, edge2).
func (c *CrossHierarchyEdgeComparator) Compare(edge1, edge2 *CrossHierarchyEdge) int {
	if edge1.Type() == lenum.PortTypeOutput && edge2.Type() == lenum.PortTypeInput {
		return -1
	} else if edge1.Type() == lenum.PortTypeInput && edge2.Type() == lenum.PortTypeOutput {
		return 1
	}
	level1 := hierarchyLevel(edge1.Graph(), c.graph)
	level2 := hierarchyLevel(edge2.Graph(), c.graph)
	if edge1.Type() == lenum.PortTypeOutput {
		// from deeper level to higher level
		return level2 - level1
	}
	// from higher level to deeper level
	return level1 - level2
}

// hierarchyLevel is hierarchyLevel(nestedGraph, topLevelGraph): the number of
// hierarchy levels between the two graphs. A larger number is a deeper graph.
func hierarchyLevel(nestedGraph, topLevelGraph *lgraph.LGraph) int {
	currentGraph := nestedGraph
	level := 0
	for {
		if currentGraph == topLevelGraph {
			return level
		}
		if currentGraph == nil {
			lgraph.Bail("CrossHierarchyEdgeComparator: a node of the hierarchy has no graph")
		}
		currentNode := currentGraph.ParentNode
		if currentNode == nil {
			// the given node is not an ancestor of the graph node
			lgraph.Bail("CrossHierarchyEdgeComparator: IllegalArgumentException, the graph is not below the top-level graph")
		}
		currentGraph = currentNode.Graph()
		level++
	}
}
