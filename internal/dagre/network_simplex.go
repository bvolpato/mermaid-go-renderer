// Ported from dagre-d3-es 7.0.14,
// dagre-d3-es/src/dagre/rank/network-simplex.js (MIT).

package dagre

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib/alg"
)

// networkSimplex assigns ranks to each node in the input graph and
// iteratively improves the ranking to reduce the length of edges.
//
// Pre-conditions:
//
//  1. The input graph must be a DAG.
//  2. All nodes in the graph must have an object value.
//  3. All edges in the graph must have "minlen" and "weight" attributes.
//
// Post-conditions:
//
//  1. All nodes in the graph have a "rank" attribute that the network
//     simplex algorithm optimized. Ranks start at 0.
//
// A rough sketch of the algorithm is as follows:
//
//  1. Assign initial ranks to each node with the longest path algorithm.
//  2. Construct a feasible tight tree. In a tight tree all edges have no
//     slack (difference between the length of an edge and its minlen).
//  3. Iteratively find edges that have negative cut values. A negative cut
//     value means that the edge can be removed and a new tree edge can be
//     added to produce a more compact graph.
//
// Much of the algorithms here are derived from Gansner, et al., "A
// Technique for Drawing Directed Graphs."
func networkSimplex(g *graphlib.Graph) {
	g = simplify(g)
	longestPath(g)
	t := feasibleTree(g)
	initLowLimValues(t)
	initCutValues(t, g)

	for {
		e, ok := leaveEdge(t)
		if !ok {
			break
		}
		f := enterEdge(t, g, e)
		exchangeEdges(t, g, e, f)
	}
}

// initCutValues initializes cut values for all edges in the tree.
func initCutValues(t, g *graphlib.Graph) {
	vs, err := alg.Postorder(t, t.Nodes())
	if err != nil {
		panic(err)
	}
	if len(vs) > 0 {
		vs = vs[:len(vs)-1]
	}
	for _, v := range vs {
		assignCutValue(t, g, v)
	}
}

// parentKey returns the parent of a tree node as a property key. A node
// without a parent has the key "undefined", as an undefined value has in
// JavaScript when it is used as a node id.
func parentKey(label *treeNode) string {
	if !label.hasParent {
		return "undefined"
	}
	return label.parent
}

func assignCutValue(t, g *graphlib.Graph, child string) {
	childLab := treeNodeOf(t, child)
	parent := parentKey(childLab)
	edge := treeEdgeOf(t, child, parent)
	cutvalue := calcCutValue(t, g, child)
	edge.cutvalue = cutvalue
}

// calcCutValue returns the cut value for the edge between the child and its
// parent, given the tight tree, its graph and a child in the graph.
func calcCutValue(t, g *graphlib.Graph, child string) float64 {
	childLab := treeNodeOf(t, child)
	parent := parentKey(childLab)
	// True if the child is on the tail end of the edge in the directed graph.
	childIsTail := true
	// The graph's view of the tree edge we're inspecting.
	graphEdge := edgeOfVW(g, child, parent)
	// The accumulated cut value for the edge between this node and its parent.
	cutValue := 0.0

	if graphEdge == nil {
		childIsTail = false
		graphEdge = edgeOfVW(g, parent, child)
	}

	cutValue = graphEdge.weight

	for _, e := range g.NodeEdges(child) {
		isOutEdge := e.V == child
		other := e.V
		if isOutEdge {
			other = e.W
		}

		if !childLab.hasParent || other != childLab.parent {
			pointsToHead := isOutEdge == childIsTail
			otherWeight := edgeOf(g, e).weight

			if pointsToHead {
				cutValue += otherWeight
			} else {
				cutValue += -otherWeight
			}
			if isTreeEdge(t, child, other) {
				otherCutValue := treeEdgeOf(t, child, other).cutvalue
				if pointsToHead {
					cutValue += -otherCutValue
				} else {
					cutValue += otherCutValue
				}
			}
		}
	}

	return cutValue
}

// initLowLimValues assigns the low and lim values of a depth first
// traversal from the first node of the tree.
func initLowLimValues(tree *graphlib.Graph) {
	root := tree.Nodes()[0]
	dfsAssignLowLim(tree, map[string]bool{}, 1, root, "", false)
}

func dfsAssignLowLim(tree *graphlib.Graph, visited map[string]bool, nextLim float64, v string, parent string, hasParent bool) float64 {
	low := nextLim
	label := treeNodeOf(tree, v)

	visited[v] = true
	for _, w := range tree.Neighbors(v) {
		if !visited[w] {
			nextLim = dfsAssignLowLim(tree, visited, nextLim, w, v, true)
		}
	}

	label.low = low
	label.lim = nextLim
	nextLim++
	if hasParent && parent != "" {
		label.parent = parent
		label.hasParent = true
	} else {
		// TODO should be able to remove this when we incrementally update low lim
		label.parent = ""
		label.hasParent = false
	}

	return nextLim
}

func leaveEdge(tree *graphlib.Graph) (graphlib.Edge, bool) {
	for _, e := range tree.Edges() {
		if tree.EdgeObj(e).(*treeEdge).cutvalue < 0 {
			return e, true
		}
	}
	return graphlib.Edge{}, false
}

func enterEdge(t, g *graphlib.Graph, edge graphlib.Edge) graphlib.Edge {
	v := edge.V
	w := edge.W

	// For the rest of this function we assume that v is the tail and w is
	// the head, so if we don't have this edge in the graph we should flip
	// it to match the correct orientation.
	if !g.HasEdge(v, w) {
		v = edge.W
		w = edge.V
	}

	vLabel := treeNodeOf(t, v)
	wLabel := treeNodeOf(t, w)
	tailLabel := vLabel
	flip := false

	// If the root is in the tail of the edge then we need to flip the logic
	// that checks for the head and tail nodes in the candidates function
	// below.
	if vLabel.lim > wLabel.lim {
		tailLabel = wLabel
		flip = true
	}

	candidates := []graphlib.Edge{}
	for _, edge := range g.Edges() {
		if flip == isDescendant(treeNodeOf(t, edge.V), tailLabel) &&
			flip != isDescendant(treeNodeOf(t, edge.W), tailLabel) {
			candidates = append(candidates, edge)
		}
	}

	f, ok := minBy(candidates, func(edge graphlib.Edge) float64 {
		return slack(g, edge)
	})
	if !ok {
		// f is undefined in JavaScript.
		panic(typeError("Cannot read properties of undefined (reading 'v')"))
	}
	return f
}

func exchangeEdges(t, g *graphlib.Graph, e, f graphlib.Edge) {
	v := e.V
	w := e.W
	t.RemoveEdge(v, w)
	setEdge(t, f.V, f.W, newTreeEdge(), "")
	initLowLimValues(t)
	initCutValues(t, g)
	updateRanks(t, g)
}

func updateRanks(t, g *graphlib.Graph) {
	// The JavaScript code looks for the first node v with !g.node(v).parent.
	// Labels of g have no parent property, so this is the first node.
	nodes := t.Nodes()
	root := nodes[0]
	if nodeOf(g, root) == nil {
		panic(typeError("Cannot read properties of undefined (reading 'parent')"))
	}
	vs, err := alg.Preorder(t, []string{root})
	if err != nil {
		panic(err)
	}
	vs = vs[1:]
	for _, v := range vs {
		parent := parentKey(treeNodeOf(t, v))
		edge := edgeOfVW(g, v, parent)
		flipped := false

		if edge == nil {
			edge = edgeOfVW(g, parent, v)
			flipped = true
		}

		node := nodeOf(g, v)
		if flipped {
			node.rank = nodeOf(g, parent).rank + edge.minlen
		} else {
			node.rank = nodeOf(g, parent).rank + -edge.minlen
		}
		node.hasRank = true
	}
}

// isTreeEdge reports whether the edge is in the tree.
func isTreeEdge(tree *graphlib.Graph, u, v string) bool {
	return tree.HasEdge(u, v)
}

// isDescendant reports whether the node is a descendant of the root node
// per the assigned low and lim attributes in the tree.
func isDescendant(vLabel, rootLabel *treeNode) bool {
	return rootLabel.low <= vLabel.lim && vLabel.lim <= rootLabel.lim
}
