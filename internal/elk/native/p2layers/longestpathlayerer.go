// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p2layers/LongestPathLayerer.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p2layers

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
)

func init() {
	lgraph.Register("LongestPathLayerer", func() lgraph.Processor { return NewLongestPathLayerer() })
}

// LongestPathLayerer is the phase LongestPathLayerer: the layer of a node is
// given by the longest path from the node to a sink.
type LongestPathLayerer struct {
	layeredGraph *lgraph.LGraph
	nodeHeights  []int
	// depth is the depth of the recursion of visit. It is not in the Java
	// class: it finds a cycle, where the JavaScript code fails with a stack
	// overflow.
	depth int
}

// NewLongestPathLayerer is new LongestPathLayerer().
func NewLongestPathLayerer() *LongestPathLayerer { return &LongestPathLayerer{} }

// Process is process(thelayeredGraph, monitor).
func (l *LongestPathLayerer) Process(thelayeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	l.layeredGraph = thelayeredGraph
	nodes := l.layeredGraph.LayerlessNodes

	// initialize values required for the computation
	l.nodeHeights = make([]int, len(nodes))
	index := 0
	for _, node := range nodes {
		// the node id is used as index for the nodeHeights array
		node.ID = index
		l.nodeHeights[index] = -1
		index++
	}

	// process all nodes
	l.depth = 0
	for _, node := range nodes {
		l.visit(node)
	}

	// empty the list of unlayered nodes
	clear(nodes)
	l.layeredGraph.LayerlessNodes = nodes[:0]

	// release the created resources
	l.layeredGraph = nil
	l.nodeHeights = nil
}

// visit puts the node in a layer after each node that its outgoing edges
// lead to, and returns the height of the node: the number of nodes on the
// longest path from the node to a sink.
func (l *LongestPathLayerer) visit(node *lgraph.LNode) int {
	if node == nil || node.ID < 0 || node.ID >= len(l.nodeHeights) {
		// The JavaScript code reads past the end of its array here.
		lgraph.Bail("LongestPathLayerer: an edge leads to a node that is not a layerless node of the graph")
	}
	height := l.nodeHeights[node.ID]
	if height >= 0 {
		// the node was already visited (the case height == 0 should never
		// occur)
		return height
	}
	if l.depth > len(l.nodeHeights) {
		lgraph.Bail("LongestPathLayerer: the graph has a cycle")
	}
	l.depth++
	maxHeight := 1
	for _, port := range node.Ports {
		for _, edge := range port.OutgoingEdges {
			targetNode := edge.Target.Node()

			// ignore self-loops
			if node != targetNode {
				targetHeight := l.visit(targetNode)
				maxHeight = max(maxHeight, targetHeight+1)
			}
		}
	}
	l.depth--
	l.putNode(node, maxHeight)
	return maxHeight
}

// putNode puts the node in the layer with the given height. The last layer
// has the height 1.
func (l *LongestPathLayerer) putNode(node *lgraph.LNode, height int) {
	// add layers so as to guarantee that number of layers >= height
	for i := len(l.layeredGraph.Layers); i < height; i++ {
		layers := append(l.layeredGraph.Layers, nil)
		copy(layers[1:], layers)
		layers[0] = lgraph.NewLayer(l.layeredGraph)
		l.layeredGraph.Layers = layers
	}

	// layer index = number of layers - height
	layers := l.layeredGraph.Layers
	node.SetLayer(layers[len(layers)-height])
	l.nodeHeights[node.ID] = height
}
