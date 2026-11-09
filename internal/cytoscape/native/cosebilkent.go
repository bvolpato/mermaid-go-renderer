// The cose-bilkent layout of a mindmap. The statements are those of mermaid
// 12.2.0, rendering-util/layout-algorithms/cose-bilkent/cytoscape-setup.ts
// and layout.ts (createCytoscapeInstance, extractPositionedNodes,
// extractPositionedEdges), with the parts of cytoscape 3.34.0 (MIT license)
// that they reach.

package native

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/cosebilkent"
	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/cy"
	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"
)

// Node is a node of the cose-bilkent layout.
type Node struct {
	ID string
	// Width and Height are the size that the layout gives to the node.
	Width, Height float64
	// X and Y are the position before the layout.
	X, Y float64
}

// Edge is an edge of the cose-bilkent layout.
type Edge struct {
	ID, Start, End string
}

// PositionedNode is a node after the layout. X and Y are the centre.
type PositionedNode struct {
	ID   string
	X, Y float64
}

// PositionedEdge is an edge after the layout: the points of the renderer
// scratch of cytoscape. A value that cytoscape did not compute is NaN.
type PositionedEdge struct {
	ID, Source, Target string
	StartX, StartY     float64
	MidX, MidY         float64
	EndX, EndY         float64
}

// CoseBilkentResult has the nodes and the edges in the order of the input.
type CoseBilkentResult struct {
	Nodes []PositionedNode
	Edges []PositionedEdge
}

// nodeSize is the width and the height of a node in the style of cytoscape:
// mermaid sets no size, so each node is the default ellipse of 30 by 30. The
// size that the layout uses comes from layoutDimensions, which mermaid
// replaces.
const nodeSize = 30.0

// CoseBilkent is executeCoseBilkentLayout of mermaid without the validation.
//
// reason is not empty when the port does not cover the input: res is then
// nil, and the caller runs the JavaScript. This is the case for a graph that
// is not a forest, for two or more nodes without edges, for an edge from a
// node to itself, for two edges between the same two nodes, and for an input
// on which cytoscape throws.
func CoseBilkent(env *Env, nodes []Node, edges []Edge) (res *CoseBilkentResult, reason string) {
	defer bail.Recover(&reason)

	// cy.add: an id is a string that is not empty, and no two elements have
	// the same id. An edge needs its two nodes.
	ids := make(map[string]int, len(nodes)+len(edges))
	for i, n := range nodes {
		if _, dup := ids[n.ID]; dup || n.ID == "" {
			bail.Out("cose-bilkent: an element that cytoscape does not add")
		}
		ids[n.ID] = i
	}
	type pair struct{ a, b int }
	pairs := make(map[pair]bool, len(edges))
	for _, e := range edges {
		if _, dup := ids[e.ID]; dup || e.ID == "" {
			bail.Out("cose-bilkent: an element that cytoscape does not add")
		}
		ids[e.ID] = -1
	}
	for _, e := range edges {
		s, okS := ids[e.Start]
		t, okT := ids[e.End]
		if !okS || !okT || s < 0 || t < 0 {
			bail.Out("cose-bilkent: an element that cytoscape does not add")
		}
		// findEdgeControlPoints: an edge from a node to itself is a loop, and
		// the edges between the same two nodes are a bundle of curves. An
		// edge that is alone between its nodes is straight.
		if s == t {
			bail.Out("cose-bilkent: an edge from a node to itself")
		}
		p := pair{min(s, t), max(s, t)}
		if pairs[p] {
			bail.Out("cose-bilkent: two edges between the same nodes")
		}
		pairs[p] = true
	}

	in := make([]cosebilkent.NodeInput, len(nodes))
	for i, n := range nodes {
		// n.layoutDimensions = () => ({ w: nodeData.width, h: nodeData.height })
		in[i] = cosebilkent.NodeInput{ID: n.ID, Width: n.Width, Height: n.Height, X: n.X, Y: n.Y}
	}
	inEdges := make([]cosebilkent.EdgeInput, len(edges))
	for i, e := range edges {
		inEdges[i] = cosebilkent.EdgeInput{ID: e.ID, Source: e.Start, Target: e.End}
	}
	var opts cosebilkent.Options
	if env != nil {
		opts = cosebilkent.Options{Trace: env.Trace, Cancelled: env.Cancelled}
	}
	trace := opts.Trace
	positions, reason := cosebilkent.Run(in, inEdges, opts)
	if reason != "" {
		return nil, reason
	}

	// nodes.layoutPositions gives each node its position, and cy.fit asks for
	// the bounding box of all elements. That request makes the renderer
	// compute the points of each edge, from the positions of the layout.
	res = &CoseBilkentResult{
		Nodes: make([]PositionedNode, len(nodes)),
		Edges: make([]PositionedEdge, len(edges)),
	}
	for i, n := range nodes {
		res.Nodes[i] = PositionedNode{ID: n.ID, X: positions[i].X, Y: positions[i].Y}
	}
	for i, e := range edges {
		src := positions[ids[e.Start]]
		tgt := positions[ids[e.End]]
		rs := cy.StraightEdge(cy.Point{X: src.X, Y: src.Y}, cy.Point{X: tgt.X, Y: tgt.Y}, nodeSize, nodeSize, nodeSize, nodeSize)
		res.Edges[i] = PositionedEdge{
			ID: e.ID, Source: e.Start, Target: e.End,
			StartX: rs.StartX, StartY: rs.StartY,
			MidX: rs.MidX, MidY: rs.MidY,
			EndX: rs.EndX, EndY: rs.EndY,
		}
	}
	if trace != nil {
		values := make([]float64, 0, 2*len(nodes)+6*len(edges))
		for _, n := range res.Nodes {
			values = append(values, n.X, n.Y)
		}
		for _, e := range res.Edges {
			values = append(values, e.StartX, e.StartY, e.MidX, e.MidY, e.EndX, e.EndY)
		}
		trace("cy.result", values)
	}
	return res, ""
}
