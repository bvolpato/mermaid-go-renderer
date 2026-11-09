// Package cosebilkent is a Go port of the cose-bilkent layout as mermaid
// 12.2.0 runs it for a mindmap: cytoscape-cose-bilkent 4.1.0 with cose-base
// 1.0.3 and layout-base 1.0.2 (all MIT license).
//
// The port covers a flat graph that is a forest, with at most one node
// without edges: the tree of a mindmap is such a graph. For another graph
// the JavaScript reduces trees, scatters the nodes with random numbers or
// tiles nodes. Those paths are not ported: Run reports the reason, and
// the caller runs the JavaScript.
//
// This file is ported from cytoscape-cose-bilkent 4.1.0 (MIT license):
// src/Layout/index.js.
package cosebilkent

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"
)

// NodeInput is a node of cytoscape as the extension reads it.
type NodeInput struct {
	ID string
	// Width and Height are layoutDimensions() of the node.
	Width, Height float64
	// X and Y are position() of the node before the layout.
	X, Y float64
}

// EdgeInput is an edge of cytoscape: its id and the ids of its ends.
type EdgeInput struct {
	ID, Source, Target string
}

// Position is the centre of a node after the layout.
type Position struct {
	X, Y float64
}

// maxNodes bounds the graphs that the port lays out. The JavaScript recurses
// once for each level of a tree.
const maxNodes = 5000

// Options are the observers of a run.
type Options struct {
	// Trace receives the state of the layout at the points that the oracle
	// records. nil for none.
	Trace func(stage string, values []float64)
	// Cancelled tells that the caller does not wait for the result. The
	// layout then stops with a reason.
	Cancelled func() bool
}

// Run is _CoSELayout.prototype.run up to the positions that it gives to
// cytoscape: the centre of each node, in the order of nodes.
//
// reason is not empty when the port does not cover the graph.
func Run(nodes []NodeInput, edges []EdgeInput, opts Options) (positions []Position, reason string) {
	defer bail.Recover(&reason)

	if len(nodes) > maxNodes {
		bail.Out("cose-bilkent: more than %d nodes", maxNodes)
	}

	idToLNode := make(map[string]*Node, len(nodes))
	layout := newLayout()
	layout.trace = opts.Trace
	layout.cancelled = opts.Cancelled
	trace := opts.Trace

	gm := layout.newGraphManagerForRun()

	root := gm.addRoot()
	// processChildrenList(this.root, this.getTopMostNodes(nodes), layout): a
	// node of mermaid has no parent, so each node is a top node.
	order := make([]*Node, len(nodes))
	for i, theChild := range nodes {
		w, h := theChild.Width, theChild.Height
		if !(w >= 0 && h >= 0) || math.IsInf(w, 0) || math.IsInf(h, 0) || math.IsInf(theChild.X, 0) || math.IsInf(theChild.Y, 0) {
			bail.Out("cose-bilkent: a node with a size or a position that is not a finite number")
		}
		// parseFloat of a number: the sign of a zero goes away.
		if w == 0 {
			w = 0
		}
		if h == 0 {
			h = 0
		}
		theNode := root.addNode(newNode(layout.graphManager, PointD{theChild.X - float64(w/2), theChild.Y - float64(h/2)}, PointD{w, h}))
		// Attach id to the layout node
		theNode.id = theChild.ID
		theNode.hasID = true

		// Map the layout node
		if _, dup := idToLNode[theChild.ID]; dup {
			bail.Out("cose-bilkent: two nodes with the same id")
		}
		idToLNode[theChild.ID] = theNode

		if math.IsNaN(theNode.rect.x) {
			theNode.rect.x = 0
		}

		if math.IsNaN(theNode.rect.y) {
			theNode.rect.y = 0
		}
		order[i] = theNode
	}

	for _, edge := range edges {
		sourceNode := idToLNode[edge.Source]
		targetNode := idToLNode[edge.Target]
		if sourceNode == nil || targetNode == nil {
			bail.Out("cose-bilkent: an edge to a node that does not exist")
		}
		if sourceNode != targetNode && len(sourceNode.getEdgesBetween(targetNode)) == 0 {
			e1 := gm.addEdge(layout.newEdge(), sourceNode, targetNode)
			e1.id = edge.ID
		}
	}

	if trace != nil {
		trace("cose.graph", layout.traceGraph())
	}

	layout.runLayout() // Run cose layout

	if trace != nil {
		trace("cose.final", layout.traceRects())
	}

	positions = make([]Position, len(order))
	for i, lNode := range order {
		positions[i] = Position{X: lNode.getRect().getCenterX(), Y: lNode.getRect().getCenterY()}
	}
	return positions, ""
}

// newGraphManagerForRun is CoSELayout.newGraphManager.
func (l *Layout) newGraphManagerForRun() *GraphManager {
	gm := newGraphManager(l)
	l.graphManager = gm
	return gm
}
