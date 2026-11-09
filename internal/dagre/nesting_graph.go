// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/nesting-graph.js
// (MIT).

package dagre

import "github.com/bvolpato/mermaid-go-renderer/internal/graphlib"

// nestingGraphRun creates dummy nodes for the tops and bottoms of
// subgraphs, adds edges that keep all nodes of a subgraph between these
// borders, and makes the graph connected.
//
// Through the minlen property it also keeps nodes and subgraph border nodes
// on different ranks.
//
// Pre-conditions:
//
//  1. Input graph is a DAG.
//  2. Edges in the input graph have a minlen attribute.
//
// Post-conditions:
//
//  1. Input graph is connected.
//  2. Dummy nodes are added for the tops and bottoms of subgraphs.
//  3. The minlen attribute of edges is adjusted so that nodes do not get
//     placed on the same rank as subgraph border nodes.
//
// The nesting graph idea comes from Sander, "Layout of Compound Directed
// Graphs."
func (l *layout) nestingGraphRun(g *graphlib.Graph) {
	root := l.addDummyNode(g, "root", newNodeLabel(), "_root")
	depths := treeDepths(g)
	depthValues := make([]float64, 0, len(depths))
	for _, d := range depths {
		depthValues = append(depthValues, d)
	}
	height := orNaN(lodashMax(depthValues)) - 1 // Note: depths is an Object not an array
	nodeSep := float64(2*height) + 1

	graphOf(g).nestingRoot = root

	// Multiply minlen by nodeSep to align nodes on non-border ranks.
	for _, e := range g.Edges() {
		edge := edgeOf(g, e)
		edge.minlen = float64(edge.minlen * nodeSep)
	}

	// Calculate a weight that is sufficient to keep subgraphs vertically compact.
	weight := sumWeights(g) + 1

	// Create border nodes and link them up.
	for _, child := range g.Children() {
		l.nestingDfs(g, root, nodeSep, weight, height, depths, child)
	}

	// Save the multiplier for node layers for later removal of empty border
	// layers.
	graphOf(g).nodeRankFactor = nodeSep
}

func (l *layout) nestingDfs(g *graphlib.Graph, root string, nodeSep, weight, height float64, depths map[string]float64, v string) {
	children := g.Children(v)
	if len(children) == 0 {
		if v != root {
			label := newEdgeLabel()
			label.weight = 0
			label.minlen = nodeSep
			setEdge(g, root, v, label, "")
		}
		return
	}

	top := l.addBorderNode(g, "_bt")
	bottom := l.addBorderNode(g, "_bb")
	label := nodeOf(g, v)

	setParent(g, top, v, true)
	label.borderTop = top
	setParent(g, bottom, v, true)
	label.borderBottom = bottom

	for _, child := range children {
		l.nestingDfs(g, root, nodeSep, weight, height, depths, child)

		childNode := nodeOf(g, child)
		childTop := child
		if childNode.borderTop != "" {
			childTop = childNode.borderTop
		}
		childBottom := child
		if childNode.borderBottom != "" {
			childBottom = childNode.borderBottom
		}
		thisWeight := float64(2 * weight)
		if childNode.borderTop != "" {
			thisWeight = weight
		}
		minlen := height - depthOf(depths, v) + 1
		if childTop != childBottom {
			minlen = 1
		}

		topLabel := newEdgeLabel()
		topLabel.weight = thisWeight
		topLabel.minlen = minlen
		topLabel.nestingEdge = true
		setEdge(g, top, childTop, topLabel, "")

		bottomLabel := newEdgeLabel()
		bottomLabel.weight = thisWeight
		bottomLabel.minlen = minlen
		bottomLabel.nestingEdge = true
		setEdge(g, childBottom, bottom, bottomLabel, "")
	}

	if parent, ok := g.Parent(v); !ok || parent == "" {
		rootLabel := newEdgeLabel()
		rootLabel.weight = 0
		rootLabel.minlen = height + depthOf(depths, v)
		setEdge(g, root, top, rootLabel, "")
	}
}

func depthOf(depths map[string]float64, v string) float64 {
	d, ok := depths[v]
	if !ok {
		return nan
	}
	return d
}

func treeDepths(g *graphlib.Graph) map[string]float64 {
	depths := map[string]float64{}
	var dfs func(v string, depth float64)
	dfs = func(v string, depth float64) {
		children := g.Children(v)
		for _, child := range children {
			dfs(child, depth+1)
		}
		depths[v] = depth
	}
	for _, v := range g.Children() {
		dfs(v, 1)
	}
	return depths
}

func sumWeights(g *graphlib.Graph) float64 {
	acc := 0.0
	for _, e := range g.Edges() {
		acc = acc + edgeOf(g, e).weight
	}
	return acc
}

func nestingGraphCleanup(g *graphlib.Graph) {
	graphLabel := graphOf(g)
	g.RemoveNode(graphLabel.nestingRoot)
	graphLabel.nestingRoot = ""
	for _, e := range g.Edges() {
		edge := edgeOf(g, e)
		if edge.nestingEdge {
			g.RemoveEdgeObj(e)
		}
	}
}
