// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/options/Spacings.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package lgraph

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

const nodeTypeCount = int(NodeTypeBreakingPoint) + 1

// Spacings is the class Spacings: the spacing option that applies between
// two nodes, by their types.
type Spacings struct {
	graph      *LGraph
	horizontal [nodeTypeCount][nodeTypeCount]*prop.Property[float64]
	vertical   [nodeTypeCount][nodeTypeCount]*prop.Property[float64]
}

// NewSpacings is new Spacings(graph).
func NewSpacings(graph *LGraph) *Spacings {
	s := &Spacings{graph: graph}
	s.precalculateNodeTypeSpacings()
	return s
}

// Graph returns the graph whose options the spacings read.
func (s *Spacings) Graph() *LGraph { return s.graph }

func (s *Spacings) precalculateNodeTypeSpacings() {
	s.same(NodeTypeNormal, lopts.SpacingNodeNode, lopts.SpacingNodeNodeBetweenLayers)
	s.pair(NodeTypeNormal, NodeTypeLongEdge, lopts.SpacingEdgeNode, lopts.SpacingEdgeNodeBetweenLayers)
	s.pair(NodeTypeNormal, NodeTypeNorthSouthPort, lopts.SpacingEdgeNode, nil)
	s.pair(NodeTypeNormal, NodeTypeExternalPort, lopts.SpacingEdgeNode, nil)
	s.pair(NodeTypeNormal, NodeTypeLabel, lopts.SpacingNodeNode, lopts.SpacingNodeNodeBetweenLayers)
	s.same(NodeTypeLongEdge, lopts.SpacingEdgeEdge, lopts.SpacingEdgeEdgeBetweenLayers)
	s.pair(NodeTypeLongEdge, NodeTypeNorthSouthPort, lopts.SpacingEdgeEdge, nil)
	s.pair(NodeTypeLongEdge, NodeTypeExternalPort, lopts.SpacingEdgeEdge, nil)
	s.pair(NodeTypeLongEdge, NodeTypeLabel, lopts.SpacingEdgeNode, lopts.SpacingEdgeNodeBetweenLayers)
	s.same(NodeTypeNorthSouthPort, lopts.SpacingEdgeEdge, nil)
	s.pair(NodeTypeNorthSouthPort, NodeTypeExternalPort, lopts.SpacingEdgeEdge, nil)
	s.pair(NodeTypeNorthSouthPort, NodeTypeLabel, lopts.SpacingLabelNode, nil)
	s.same(NodeTypeExternalPort, lopts.SpacingPortPort, nil)
	s.pair(NodeTypeExternalPort, NodeTypeLabel, lopts.SpacingLabelPortVertical, lopts.SpacingLabelPortHorizontal)
	s.same(NodeTypeLabel, lopts.SpacingEdgeEdge, lopts.SpacingEdgeEdge)
	s.same(NodeTypeBreakingPoint, lopts.SpacingEdgeEdge, lopts.SpacingEdgeEdgeBetweenLayers)
	s.pair(NodeTypeBreakingPoint, NodeTypeNormal, lopts.SpacingEdgeNode, lopts.SpacingEdgeNodeBetweenLayers)
	s.pair(NodeTypeBreakingPoint, NodeTypeLabel, lopts.SpacingEdgeNode, lopts.SpacingEdgeNodeBetweenLayers)
	s.pair(NodeTypeBreakingPoint, NodeTypeLongEdge, lopts.SpacingEdgeNode, lopts.SpacingEdgeNodeBetweenLayers)
}

// same is nodeTypeSpacing(nt, vert[, horz]). A nil horz keeps the entry.
func (s *Spacings) same(nt NodeType, vert, horz *prop.Property[float64]) {
	s.vertical[nt][nt] = vert
	if horz != nil {
		s.horizontal[nt][nt] = horz
	}
}

// pair is nodeTypeSpacing(n1, n2, vert[, horz]). A nil horz keeps the entries.
func (s *Spacings) pair(n1, n2 NodeType, vert, horz *prop.Property[float64]) {
	s.vertical[n1][n2] = vert
	s.vertical[n2][n1] = vert
	if horz != nil {
		s.horizontal[n1][n2] = horz
		s.horizontal[n2][n1] = horz
	}
}

// HorizontalSpacing is getHorizontalSpacing(LNode, LNode).
func (s *Spacings) HorizontalSpacing(n1, n2 *LNode) float64 {
	return localSpacing(n1, n2, &s.horizontal)
}

// HorizontalSpacingByType is getHorizontalSpacing(NodeType, NodeType).
func (s *Spacings) HorizontalSpacingByType(nt1, nt2 NodeType) float64 {
	return prop.Get(s.graph, s.option(&s.horizontal, nt1, nt2))
}

// VerticalSpacing is getVerticalSpacing(LNode, LNode).
func (s *Spacings) VerticalSpacing(n1, n2 *LNode) float64 {
	return localSpacing(n1, n2, &s.vertical)
}

// VerticalSpacingByType is getVerticalSpacing(NodeType, NodeType).
func (s *Spacings) VerticalSpacingByType(nt1, nt2 NodeType) float64 {
	return prop.Get(s.graph, s.option(&s.vertical, nt1, nt2))
}

// option returns the option of a pair of types. Java fails with a
// NullPointerException for a pair without an option.
func (s *Spacings) option(table *[nodeTypeCount][nodeTypeCount]*prop.Property[float64], nt1, nt2 NodeType) *prop.Property[float64] {
	o := table[nt1][nt2]
	if o == nil {
		Bail("Spacings: no spacing option for the node types %v and %v", nt1, nt2)
	}
	return o
}

func localSpacing(n1, n2 *LNode, table *[nodeTypeCount][nodeTypeCount]*prop.Property[float64]) float64 {
	o := table[n1.Type][n2.Type]
	if o == nil {
		Bail("Spacings: no spacing option for the node types %v and %v", n1.Type, n2.Type)
	}
	s1 := IndividualOrDefault(n1, o)
	s2 := IndividualOrDefault(n2, o)
	return max(s1, s2)
}

// IndividualOrDefault is Spacings.getIndividualOrDefault(node, property): the
// value of the individual spacings of the node, or the value of its graph.
//
// The individual spacings of a node (the member individualSpacings of the
// JSON graph) are not ported. A node that has them stops the layout.
func IndividualOrDefault[T any](node *LNode, property *prop.Property[T]) T {
	if prop.Has(node, lopts.SpacingIndividual) {
		Bail("Spacings: individual spacings are not ported")
	}
	return prop.Get(node.Graph(), property)
}
