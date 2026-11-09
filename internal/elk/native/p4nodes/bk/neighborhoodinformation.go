// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p4nodes/bk/NeighborhoodInformation.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package bk

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// neighbor is the Pair<LNode, LEdge> of the neighbor lists: a node of another
// layer, with an edge that connects it.
type neighbor struct {
	first  *lgraph.LNode
	second *lgraph.LEdge
}

// NeighborhoodInformation is the class NeighborhoodInformation: the indices
// and the neighbors of the nodes, which do not change during the placement.
type NeighborhoodInformation struct {
	// nodeCount is the number of nodes of the graph.
	nodeCount int
	// layerIndex has the index of a layer at the id of the layer.
	layerIndex []int
	// nodeIndex has the index of a node in its layer at the id of the node.
	nodeIndex []int
	// leftNeighbors has for the id of a node its neighbors in the layers
	// before, each with an edge that connects the two nodes.
	leftNeighbors [][]neighbor
	// rightNeighbors is leftNeighbors for the layers after.
	rightNeighbors [][]neighbor
	// nodes has each node at its id. The Java class does not have it: the
	// port uses it to find an edge that leaves the layers of the graph.
	nodes []*lgraph.LNode
}

// cleanup is cleanup().
func (ni *NeighborhoodInformation) cleanup() {
	ni.layerIndex = nil
	ni.nodeIndex = nil
	ni.leftNeighbors = ni.leftNeighbors[:0]
	ni.rightNeighbors = ni.rightNeighbors[:0]
	ni.nodes = nil
}

// buildFor is NeighborhoodInformation.buildFor(graph). It gives each layer
// and each node an id.
func buildFor(graph *lgraph.LGraph) *NeighborhoodInformation {
	ni := &NeighborhoodInformation{}

	ni.nodeCount = 0
	for _, layer := range graph.Layers {
		ni.nodeCount += len(layer.Nodes)
	}

	// cache indexes of layers and of nodes
	lID := 0
	lIndex := 0
	ni.layerIndex = make([]int, len(graph.Layers))
	nID := 0
	ni.nodeIndex = make([]int, ni.nodeCount)
	ni.nodes = make([]*lgraph.LNode, 0, ni.nodeCount)
	for _, l := range graph.Layers {
		l.ID = lID
		lID++
		ni.layerIndex[l.ID] = lIndex
		lIndex++
		nIndex := 0
		for _, n := range l.Nodes {
			n.ID = nID
			nID++
			ni.nodeIndex[n.ID] = nIndex
			nIndex++
			ni.nodes = append(ni.nodes, n)
		}
	}

	// determine all left and right neighbors of the graph's nodes
	ni.leftNeighbors = make([][]neighbor, 0, ni.nodeCount)
	determineAllLeftNeighbors(ni, graph)
	ni.rightNeighbors = make([][]neighbor, 0, ni.nodeCount)
	determineAllRightNeighbors(ni, graph)

	return ni
}

// determineAllRightNeighbors gives the right neighbors of each node: the
// nodes of another layer that an outgoing edge leads to.
func determineAllRightNeighbors(ni *NeighborhoodInformation, graph *lgraph.LGraph) {
	for _, l := range graph.Layers {
		for _, n := range l.Nodes {
			result := []neighbor{}
			maxPriority := 0

			for _, port := range n.Ports {
				for _, edge := range port.OutgoingEdges {
					if edge.IsSelfLoop() || edge.IsInLayerEdge() {
						continue
					}
					ni.checkInGraph(edge.Target.Node())
					edgePrio := prop.Get(edge, lopts.PriorityStraightness)
					if edgePrio > maxPriority {
						maxPriority = edgePrio
						result = result[:0]
					}
					if edgePrio == maxPriority {
						result = append(result, neighbor{edge.Target.Node(), edge})
					}
				}
			}

			gwt.SortFunc(result, ni.neighborCompare)

			// The ids of the nodes count up in this order, so add(n.id,
			// result) appends.
			ni.rightNeighbors = append(ni.rightNeighbors, result)
		}
	}
}

// determineAllLeftNeighbors gives the left neighbors of each node: the nodes
// of another layer that an incoming edge comes from.
func determineAllLeftNeighbors(ni *NeighborhoodInformation, graph *lgraph.LGraph) {
	for _, l := range graph.Layers {
		for _, n := range l.Nodes {
			result := []neighbor{}
			maxPriority := 0

			for _, port := range n.Ports {
				for _, edge := range port.IncomingEdges {
					if edge.IsSelfLoop() || edge.IsInLayerEdge() {
						continue
					}
					ni.checkInGraph(edge.Source.Node())
					edgePrio := prop.Get(edge, lopts.PriorityStraightness)
					if edgePrio > maxPriority {
						maxPriority = edgePrio
						result = result[:0]
					}
					if edgePrio == maxPriority {
						result = append(result, neighbor{edge.Source.Node(), edge})
					}
				}
			}

			gwt.SortFunc(result, ni.neighborCompare)

			ni.leftNeighbors = append(ni.leftNeighbors, result)
		}
	}
}

// checkInGraph stops the layout for an edge that leads to a node outside the
// layers of the graph. The Java code reads its arrays at the id of each
// neighbor, and only the nodes of the layers have an id that is an index of
// these arrays.
func (ni *NeighborhoodInformation) checkInGraph(node *lgraph.LNode) {
	if node.ID < 0 || node.ID >= len(ni.nodes) || ni.nodes[node.ID] != node {
		lgraph.Bail("BKNodePlacer: an edge leads to a node outside the layers of the graph")
	}
}

// neighborCompare is NeighborComparator.compare: the order of the neighbors
// by their index in the layer.
func (ni *NeighborhoodInformation) neighborCompare(o1, o2 neighbor) int {
	cmp := ni.nodeIndex[o1.first.ID] - ni.nodeIndex[o2.first.ID]
	return gwt.RoundInt(gwt.Signum(float64(cmp)))
}
