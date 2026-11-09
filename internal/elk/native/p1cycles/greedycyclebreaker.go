// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p1cycles/GreedyCycleBreaker.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p1cycles

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("GreedyCycleBreaker", func() lgraph.Processor { return NewGreedyCycleBreaker() })
}

// GreedyCycleBreaker is the phase GreedyCycleBreaker: it puts the nodes in a
// linear order with few edges that point left (sinks to the right, sources
// to the left, then the node with the largest out flow), and reverses the
// edges that point left.
type GreedyCycleBreaker struct {
	indeg  []int
	outdeg []int
	mark   []int
	// sources and sinks are the linked lists of the Java class: a node is
	// added at the end and removed at the start.
	sources []*lgraph.LNode
	sinks   []*lgraph.LNode

	random *gwt.Random

	// chooser is chooseNodeWithMaxOutflow of a subclass, or nil for the
	// method of this class.
	chooser func(nodes []*lgraph.LNode) *lgraph.LNode
}

// NewGreedyCycleBreaker is new GreedyCycleBreaker().
func NewGreedyCycleBreaker() *GreedyCycleBreaker { return &GreedyCycleBreaker{} }

// Process is process(layeredGraph, monitor).
func (b *GreedyCycleBreaker) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	nodes := layeredGraph.LayerlessNodes

	// initialize values for the algorithm (sum of priorities of incoming
	// edges and outgoing edges per node, and the ordering calculated for each
	// node)
	unprocessedNodeCount := len(nodes)
	b.indeg = make([]int, unprocessedNodeCount)
	b.outdeg = make([]int, unprocessedNodeCount)
	b.mark = make([]int, unprocessedNodeCount)
	// The lists are empty here: dispose clears them. A layout that stopped
	// in this method can leave nodes.
	b.sources = b.sources[:0]
	b.sinks = b.sinks[:0]

	index := 0
	for _, node := range nodes {
		// the node id is used as index for the indeg, outdeg, and mark arrays
		node.ID = index

		for _, port := range node.Ports {
			// calculate the sum of edge priorities
			for _, edge := range port.IncomingEdges {
				// ignore self-loops
				if edge.Source.Node() == node {
					continue
				}

				priority := prop.Get(edge, lopts.PriorityDirection)
				if priority > 0 {
					b.indeg[index] += priority + 1
				} else {
					b.indeg[index] += 1
				}
			}

			for _, edge := range port.OutgoingEdges {
				// ignore self-loops
				if edge.Target.Node() == node {
					continue
				}

				priority := prop.Get(edge, lopts.PriorityDirection)
				if priority > 0 {
					b.outdeg[index] += priority + 1
				} else {
					b.outdeg[index] += 1
				}
			}
		}

		// collect sources and sinks
		if b.outdeg[index] == 0 {
			b.sinks = append(b.sinks, node)
		} else if b.indeg[index] == 0 {
			b.sources = append(b.sources, node)
		}
		index++
	}

	// The ids are indices now. An edge to a node that is not a layerless node
	// of the graph would read the arrays at the id of that node.
	for _, node := range nodes {
		for _, port := range node.Ports {
			for _, edge := range port.IncomingEdges {
				b.checkEndpoint(nodes, edge.Source)
			}
			for _, edge := range port.OutgoingEdges {
				b.checkEndpoint(nodes, edge.Target)
			}
		}
	}

	// next rank values used for sinks and sources (from right and from left)
	nextRight, nextLeft := -1, 1

	// assign marks to all nodes
	var maxNodes []*lgraph.LNode
	b.random = prop.Get(layeredGraph, iprops.Random)

	for unprocessedNodeCount > 0 {
		// sinks are put to the right --> assign negative rank, which is later
		// shifted to positive
		for len(b.sinks) != 0 {
			sink := b.sinks[0]
			b.sinks = b.sinks[1:]
			b.mark[sink.ID] = nextRight
			nextRight--
			b.updateNeighbors(sink)
			unprocessedNodeCount--
		}

		// sources are put to the left --> assign positive rank
		for len(b.sources) != 0 {
			source := b.sources[0]
			b.sources = b.sources[1:]
			b.mark[source.ID] = nextLeft
			nextLeft++
			b.updateNeighbors(source)
			unprocessedNodeCount--
		}

		// while there are unprocessed nodes left that are neither sinks nor
		// sources...
		if unprocessedNodeCount > 0 {
			maxOutflow := math.MinInt32

			// find the set of unprocessed node (=> mark == 0), with the
			// largest out flow
			for _, node := range nodes {
				if b.mark[node.ID] == 0 {
					outflow := b.outdeg[node.ID] - b.indeg[node.ID]
					if outflow >= maxOutflow {
						if outflow > maxOutflow {
							maxNodes = maxNodes[:0]
							maxOutflow = outflow
						}
						maxNodes = append(maxNodes, node)
					}
				}
			}

			// randomly select a node from the ones with maximal outflow and
			// put it left
			var maxNode *lgraph.LNode
			if b.chooser != nil {
				maxNode = b.chooser(maxNodes)
			} else {
				maxNode = b.chooseNodeWithMaxOutflow(maxNodes)
			}
			b.mark[maxNode.ID] = nextLeft
			nextLeft++
			b.updateNeighbors(maxNode)
			unprocessedNodeCount--
		}
	}

	// shift negative ranks to positive; this applies to sinks of the graph
	shiftBase := len(nodes) + 1
	for index = 0; index < len(nodes); index++ {
		if b.mark[index] < 0 {
			b.mark[index] += shiftBase
		}
	}

	// reverse edges that point left
	for _, node := range nodes {
		// A reversed edge can add a port to a node, and it moves between the
		// edge lists of two ports. So the walk is over copies, as in the Java
		// code.
		ports := append([]*lgraph.LPort(nil), node.Ports...)
		for _, port := range ports {
			outgoingEdges := append([]*lgraph.LEdge(nil), port.OutgoingEdges...)

			// look at the node's outgoing edges
			for _, edge := range outgoingEdges {
				targetIx := edge.Target.Node().ID
				if b.mark[node.ID] > b.mark[targetIx] {
					edge.Reverse(layeredGraph, true)
					prop.Set(layeredGraph, iprops.Cyclic, true)
				}
			}
		}
	}

	b.dispose()
}

// checkEndpoint stops the layout when the node of the port is not the
// layerless node with its id.
func (b *GreedyCycleBreaker) checkEndpoint(nodes []*lgraph.LNode, port *lgraph.LPort) {
	if port == nil || port.Node() == nil {
		lgraph.Bail("GreedyCycleBreaker: an edge has no node at one end")
	}
	n := port.Node()
	if n.ID < 0 || n.ID >= len(nodes) || nodes[n.ID] != n {
		lgraph.Bail("GreedyCycleBreaker: an edge leads to a node that is not a layerless node of the graph")
	}
}

// chooseNodeWithMaxOutflow is chooseNodeWithMaxOutflow(nodes): it selects one
// of the nodes at random.
func (b *GreedyCycleBreaker) chooseNodeWithMaxOutflow(nodes []*lgraph.LNode) *lgraph.LNode {
	if b.random == nil || len(nodes) == 0 {
		// The Java code throws here.
		lgraph.Bail("GreedyCycleBreaker: no random generator, or no node to choose from")
	}
	return nodes[b.random.NextInt(len(nodes))]
}

func (b *GreedyCycleBreaker) dispose() {
	b.indeg = nil
	b.outdeg = nil
	b.mark = nil
	b.sources = nil
	b.sinks = nil
}

// updateNeighbors updates the edge counts of the neighbors of a node that got
// its mark, and adds each neighbor that became a source or a sink to the
// list.
func (b *GreedyCycleBreaker) updateNeighbors(node *lgraph.LNode) {
	for _, port := range node.Ports {
		// port.getConnectedEdges(): the incoming edges, then the outgoing
		// edges.
		for pass := 0; pass < 2; pass++ {
			edges := port.IncomingEdges
			if pass == 1 {
				edges = port.OutgoingEdges
			}
			for _, edge := range edges {
				connectedPort := edge.Source
				if edge.Source == port {
					connectedPort = edge.Target
				}
				endpoint := connectedPort.Node()

				// exclude self-loops
				if node == endpoint {
					continue
				}

				priority := prop.Get(edge, lopts.PriorityDirection)
				if priority < 0 {
					priority = 0
				}
				index := endpoint.ID
				if b.mark[index] == 0 {
					if edge.Target == connectedPort {
						b.indeg[index] -= priority + 1
						if b.indeg[index] <= 0 && b.outdeg[index] > 0 {
							b.sources = append(b.sources, endpoint)
						}
					} else {
						b.outdeg[index] -= priority + 1
						if b.outdeg[index] <= 0 && b.indeg[index] > 0 {
							b.sinks = append(b.sinks, endpoint)
						}
					}
				}
			}
		}
	}
}
