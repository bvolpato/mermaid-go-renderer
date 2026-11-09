// The records that Run gives to its trace function. The debug glue of
// internal/cytoscape (testdata/trace.js) records the same values from the
// JavaScript, and the tests compare them bit for bit.

package fcose

import "math"

// traceNodes returns the nodes of all graphs in the order of
// LGraphManager.getAllNodes, without the cache of that function.
func (l *Layout) traceNodes() []*Node {
	var list []*Node
	for _, g := range l.graphManager.getGraphs() {
		list = append(list, g.getNodes()...)
	}
	return list
}

// traceGraph is the graph that the extension built: the rectangle of each
// node, and the indexes of the ends, the ideal length and the elasticity of
// each edge.
func (l *Layout) traceGraph() []float64 {
	nodes := l.traceNodes()
	index := make(map[*Node]int, len(nodes))
	out := []float64{float64(len(nodes))}
	for i, n := range nodes {
		index[n] = i
		out = append(out, n.rect.x, n.rect.y, n.rect.width, n.rect.height)
	}
	for _, g := range l.graphManager.getGraphs() {
		for _, e := range g.getEdges() {
			out = append(out, float64(index[e.source]), float64(index[e.target]), e.idealLength, e.edgeElasticity)
		}
	}
	// The edges between two graphs.
	for _, e := range l.graphManager.edges {
		out = append(out, float64(index[e.source]), float64(index[e.target]), e.idealLength, e.edgeElasticity)
	}
	return out
}

// traceRects is the rectangle of each node.
func (l *Layout) traceRects() []float64 {
	var out []float64
	for _, n := range l.traceNodes() {
		out = append(out, n.rect.x, n.rect.y, n.rect.width, n.rect.height)
	}
	return out
}

// tracePositions is head followed by the left top corner of each node.
func (l *Layout) tracePositions(head []float64) []float64 {
	out := head
	for _, n := range l.traceNodes() {
		out = append(out, n.rect.x, n.rect.y)
	}
	return out
}

// traceForces is the forces on each node, before the nodes move.
func (l *Layout) traceForces() []float64 {
	out := []float64{l.totalIterations}
	for _, n := range l.traceNodes() {
		out = append(out, n.springForceX, n.springForceY, n.repulsionForceX, n.repulsionForceY, n.gravitationForceX, n.gravitationForceY)
	}
	return out
}

// traceIteration tells whether the oracle records the current iteration: the
// first 20, and then each 50th.
func (l *Layout) traceIteration() bool {
	return l.totalIterations <= 20 || math.Mod(l.totalIterations, 50) == 0
}
