// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/rank/feasible-tree.js
// (MIT).

package dagre

import "github.com/bvolpato/mermaid-go-renderer/internal/graphlib"

// treeNode is the label of a node of the tight tree.
type treeNode struct {
	low       float64
	lim       float64
	parent    string
	hasParent bool
}

// treeEdge is the label of an edge of the tight tree. cutvalue is NaN
// until it is assigned.
type treeEdge struct {
	cutvalue float64
}

func newTreeEdge() *treeEdge { return &treeEdge{cutvalue: nan} }

func treeNodeOf(t *graphlib.Graph, v string) *treeNode {
	n, _ := t.Node(v).(*treeNode)
	return n
}

func treeEdgeOf(t *graphlib.Graph, v, w string) *treeEdge {
	e, _ := t.Edge(v, w).(*treeEdge)
	return e
}

// feasibleTree constructs a spanning tree with tight edges and adjusts the
// ranks of the input nodes to achieve this. A tight edge has a length that
// matches its "minlen" attribute.
//
// The basic structure for this function is derived from Gansner, et al.,
// "A Technique for Drawing Directed Graphs."
//
// Pre-conditions:
//
//  1. Graph must be a DAG.
//  2. Graph must be connected.
//  3. Graph must have at least one node.
//  5. Graph nodes must have a "rank" property that respects the "minlen"
//     property of incident edges.
//  6. Graph edges must have a "minlen" property.
//
// Post-conditions:
//
//   - Graph nodes have their rank adjusted so that all edges of the tree
//     are tight.
//
// Returns a tree (undirected graph) that is constructed using only "tight"
// edges.
func feasibleTree(g *graphlib.Graph) *graphlib.Graph {
	t := graphlib.New(graphlib.Options{Undirected: true})

	// Choose arbitrary node from which to start our tree.
	start := g.Nodes()[0]
	size := g.NodeCount()
	t.SetNode(start, &treeNode{})

	for tightTree(t, g) < size {
		edge, ok := findMinSlackEdge(t, g)
		if !ok {
			// edge is undefined in JavaScript.
			panic(typeError("Cannot read properties of undefined (reading 'v')"))
		}
		delta := slack(g, edge)
		if !t.HasNode(edge.V) {
			delta = -slack(g, edge)
		}
		shiftRanks(t, g, delta)
	}

	return t
}

// tightTree finds a maximal tree of tight edges and returns the number of
// nodes in the tree.
func tightTree(t, g *graphlib.Graph) int {
	var dfs func(v string)
	dfs = func(v string) {
		for _, e := range g.NodeEdges(v) {
			edgeV := e.V
			w := edgeV
			if v == edgeV {
				w = e.W
			}
			if !t.HasNode(w) && !truthy(slack(g, e)) {
				t.SetNode(w, &treeNode{})
				setEdge(t, v, w, newTreeEdge(), "")
				dfs(w)
			}
		}
	}

	for _, v := range t.Nodes() {
		dfs(v)
	}
	return t.NodeCount()
}

// findMinSlackEdge finds the edge with the smallest slack that is incident
// on the tree and returns it.
func findMinSlackEdge(t, g *graphlib.Graph) (graphlib.Edge, bool) {
	return minBy(g.Edges(), func(e graphlib.Edge) float64 {
		if t.HasNode(e.V) != t.HasNode(e.W) {
			return slack(g, e)
		}
		return nan
	})
}

func shiftRanks(t, g *graphlib.Graph, delta float64) {
	for _, v := range t.Nodes() {
		node := nodeOf(g, v)
		node.rank += delta
		node.hasRank = true
	}
}
