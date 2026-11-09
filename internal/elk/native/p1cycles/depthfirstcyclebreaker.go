// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p1cycles/DepthFirstCycleBreaker.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package p1cycles has the cycle breakers of the layered algorithm: the
// first phase, which reverses edges until the graph has no cycle.
package p1cycles

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("DepthFirstCycleBreaker", func() lgraph.Processor { return NewDepthFirstCycleBreaker() })
}

// DepthFirstCycleBreaker is the phase DepthFirstCycleBreaker: it walks the
// graph depth first and reverses each edge that leads back to a node of the
// current path.
type DepthFirstCycleBreaker struct {
	sources           []*lgraph.LNode
	visited           []bool
	active            []bool
	edgesToBeReversed []*lgraph.LEdge
}

// NewDepthFirstCycleBreaker is new DepthFirstCycleBreaker().
func NewDepthFirstCycleBreaker() *DepthFirstCycleBreaker { return &DepthFirstCycleBreaker{} }

// Process is process(graph, monitor).
func (b *DepthFirstCycleBreaker) Process(graph *lgraph.LGraph, ctx *lgraph.Context) {
	nodes := graph.LayerlessNodes

	// initialize values for the algorithm
	nodeCount := len(nodes)

	b.sources = nil
	b.visited = make([]bool, nodeCount)
	b.active = make([]bool, nodeCount)
	b.edgesToBeReversed = nil

	index := 0
	for _, node := range nodes {
		// The node id is used as index into our arrays
		node.ID = index
		if !hasIncomingEdges(node) {
			b.sources = append(b.sources, node)
		}
		index++
	}

	// From every source node start a DFS
	for _, source := range b.sources {
		b.dfs(source)
	}

	// Start more DFS runs for all nodes that have not been visited yet. These
	// must be part of a cycle since they are not source nodes
	for i := 0; i < nodeCount; i++ {
		if !b.visited[i] {
			b.dfs(nodes[i])
		}
	}

	// Reverse "back edges"
	for _, edge := range b.edgesToBeReversed {
		edge.Reverse(graph, true)
		prop.Set(graph, iprops.Cyclic, true)
	}

	// Cleanup
	b.sources = nil
	b.visited = nil
	b.active = nil
	b.edgesToBeReversed = nil
}

// hasIncomingEdges tells that node.getIncomingEdges() is not empty.
func hasIncomingEdges(node *lgraph.LNode) bool {
	for _, port := range node.Ports {
		if len(port.IncomingEdges) != 0 {
			return true
		}
	}
	return false
}

func (b *DepthFirstCycleBreaker) dfs(n *lgraph.LNode) {
	b.checkIndex(n)
	if b.visited[n.ID] {
		return
	}

	// We're now visiting the node, and it's active
	b.visited[n.ID] = true
	b.active[n.ID] = true

	// The walk is over node.getOutgoingEdges(): the outgoing edges of each
	// port in order. No edge changes before the walk ends.
	for _, port := range n.Ports {
		for _, out := range port.OutgoingEdges {
			// Ignore self loops
			if out.IsSelfLoop() {
				continue
			}

			target := out.Target.Node()
			b.checkIndex(target)

			// If the edge connects to an active node, we have found a path
			// from said active node back to itself since active nodes are on
			// our current path. That's a backward edge and needs to be
			// reversed
			if b.active[target.ID] {
				b.edgesToBeReversed = append(b.edgesToBeReversed, out)
			} else {
				b.dfs(target)
			}
		}
	}

	// We're leaving this node
	b.active[n.ID] = false
}

// checkIndex stops the layout for a node whose id is not an index of the
// arrays: an edge leads to a node that is not a layerless node of the graph.
// The JavaScript code extends its arrays for such a node.
func (b *DepthFirstCycleBreaker) checkIndex(n *lgraph.LNode) {
	if n == nil || n.ID < 0 || n.ID >= len(b.visited) {
		lgraph.Bail("DepthFirstCycleBreaker: an edge leads to a node that is not a layerless node of the graph")
	}
}
