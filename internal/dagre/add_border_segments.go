// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/add-border-segments.js
// (MIT).

package dagre

import "github.com/bvolpato/mermaid-go-renderer/internal/graphlib"

func (l *layout) addBorderSegments(g *graphlib.Graph) {
	var dfs func(v string)
	dfs = func(v string) {
		children := g.Children(v)
		node := nodeOf(g, v)
		for _, child := range children {
			dfs(child)
		}

		if node.hasMinRank {
			node.borderLeft = []string{}
			node.borderRight = []string{}
			for rank, maxRank := node.minRank, node.maxRank+1; rank < maxRank; rank++ {
				l.addBorderSegmentNode(g, &node.borderLeft, "borderLeft", "_bl", v, rank)
				l.addBorderSegmentNode(g, &node.borderRight, "borderRight", "_br", v, rank)
			}
		}
	}

	for _, v := range g.Children() {
		dfs(v)
	}
}

// addBorderSegmentNode is addBorderNode of add-border-segments.js. border
// is sgNode[prop], a sparse array indexed by rank.
func (l *layout) addBorderSegmentNode(g *graphlib.Graph, border *[]string, prop, prefix, sg string, rank float64) {
	label := newNodeLabel()
	label.width = 0
	label.height = 0
	label.rank = rank
	label.hasRank = true
	label.borderType = prop
	prev := sparseGet(*border, rank-1)
	curr := l.addDummyNode(g, "border", label, prefix)
	sparseSet(border, rank, curr)
	setParent(g, curr, sg, true)
	if prev != "" {
		edge := newEdgeLabel()
		edge.weight = 1
		setEdge(g, prev, curr, edge, "")
	}
}

// sparseGet is array[index] for a sparse array of node ids. The result is
// empty for a hole and for an index outside of the array.
func sparseGet(array []string, index float64) string {
	i, ok := arrayIndex(index)
	if !ok || i >= len(array) {
		return ""
	}
	return array[i]
}

// sparseSet is array[index] = v. An index that is not an array index sets a
// property that no code reads, so the port ignores it.
func sparseSet(array *[]string, index float64, v string) {
	i, ok := arrayIndex(index)
	if !ok {
		return
	}
	for len(*array) <= i {
		*array = append(*array, "")
	}
	(*array)[i] = v
}
