package dagre

// NestingGraphRun creates dummy border nodes for subgraphs, connects them
// with weighted edges, and adds a nesting root to ensure graph connectivity.
func NestingGraphRun(g *Graph) {
	root := AddDummyNode(g, "root", "", 0)
	rootNode := g.Node(root)
	rootNode.Dummy = "root"
	depths := treeDepths(g)

	maxDepth := 0
	for _, d := range depths {
		if d > maxDepth {
			maxDepth = d
		}
	}
	height := maxDepth - 1
	if height < 0 {
		height = 0
	}
	nodeSep := 2*height + 1

	g.label.NestingRoot = root
	g.label.NodeRankFactor = nodeSep

	// Multiply existing minlen by nodeSep
	for _, e := range g.Edges() {
		el := g.EdgeByKey(e)
		el.MinLen *= nodeSep
	}

	// Calculate total weight for keep-compact edges
	weight := sumWeights(g) + 1

	// Create border nodes and connect
	for _, child := range g.Children("") {
		nestingDFS(g, root, nodeSep, weight, height, depths, child)
	}
}

func nestingDFS(g *Graph, root string, nodeSep, weight, height int, depths map[string]int, v string) {
	children := g.Children(v)
	if len(children) == 0 {
		if v != root {
			g.SetEdgeVW(root, v, &EdgeLabel{Weight: 0, MinLen: nodeSep})
		}
		return
	}

	top := AddDummyNode(g, "border", "_bt", 0)
	topNode := g.Node(top)
	topNode.Dummy = "border"
	bottom := AddDummyNode(g, "border", "_bb", 0)
	bottomNode := g.Node(bottom)
	bottomNode.Dummy = "border"
	label := g.Node(v)

	g.SetParent(top, v)
	label.BorderTop = top
	g.SetParent(bottom, v)
	label.BorderBot = bottom

	for _, child := range children {
		nestingDFS(g, root, nodeSep, weight, height, depths, child)

		childNode := g.Node(child)
		childTop := child
		if childNode.BorderTop != "" {
			childTop = childNode.BorderTop
		}
		childBottom := child
		if childNode.BorderBot != "" {
			childBottom = childNode.BorderBot
		}
		thisWeight := weight
		if childNode.BorderTop != "" {
			// child is a compound; use normal weight
		} else {
			thisWeight = 2 * weight
		}
		minlen := 1
		if childTop == childBottom {
			depth := depths[v]
			minlen = height - depth + 1
			if minlen < 1 {
				minlen = 1
			}
		}

		g.SetEdgeVW(top, childTop, &EdgeLabel{Weight: thisWeight, MinLen: minlen, NestingEdge: true})
		g.SetEdgeVW(childBottom, bottom, &EdgeLabel{Weight: thisWeight, MinLen: minlen, NestingEdge: true})
	}

	if g.Parent(v) == "" {
		depth := depths[v]
		g.SetEdgeVW(root, top, &EdgeLabel{Weight: 0, MinLen: height + depth})
	}
}

func treeDepths(g *Graph) map[string]int {
	depths := make(map[string]int)
	var dfs func(string, int)
	dfs = func(v string, depth int) {
		children := g.Children(v)
		if len(children) > 0 {
			for _, child := range children {
				dfs(child, depth+1)
			}
		}
		depths[v] = depth
	}
	for _, v := range g.Children("") {
		dfs(v, 1)
	}
	return depths
}

func sumWeights(g *Graph) int {
	total := 0
	for _, e := range g.Edges() {
		total += g.EdgeByKey(e).Weight
	}
	return total
}

// NestingGraphCleanup removes the nesting root node and nesting edges.
func NestingGraphCleanup(g *Graph) {
	if g.label.NestingRoot != "" {
		g.RemoveNode(g.label.NestingRoot)
		g.label.NestingRoot = ""
	}
	for _, e := range g.Edges() {
		el := g.EdgeByKey(e)
		if el.NestingEdge {
			g.RemoveEdge(e)
		}
	}
}
