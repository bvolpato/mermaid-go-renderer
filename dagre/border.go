package dagre

// AddBorderSegments adds border nodes for each rank of compound nodes.
func AddBorderSegments(g *Graph) {
	var dfs func(string)
	dfs = func(v string) {
		children := g.Children(v)
		for _, child := range children {
			dfs(child)
		}

		node := g.Node(v)
		if node == nil {
			return
		}
		if node.HasMinRank {
			node.BorderLeft = make([]string, node.MaxRank+1)
			node.BorderRight = make([]string, node.MaxRank+1)
			for rank := node.MinRank; rank <= node.MaxRank; rank++ {
				addBorderNode(g, "borderLeft", "_bl", v, node, rank)
				addBorderNode(g, "borderRight", "_br", v, node, rank)
			}
		}
	}

	for _, v := range g.Children("") {
		dfs(v)
	}
}

func addBorderNode(g *Graph, prop, prefix, sg string, sgNode *NodeLabel, rank int) {
	var prev string
	if prop == "borderLeft" {
		if rank-1 >= 0 && rank-1 < len(sgNode.BorderLeft) {
			prev = sgNode.BorderLeft[rank-1]
		}
	} else {
		if rank-1 >= 0 && rank-1 < len(sgNode.BorderRight) {
			prev = sgNode.BorderRight[rank-1]
		}
	}

	curr := AddDummyNode(g, "border", prefix, rank)
	g.Node(curr).BorderType = prop
	g.Node(curr).Dummy = "border"
	g.Node(curr).Rank = rank
	g.Node(curr).HasRank = true

	if prop == "borderLeft" {
		if rank < len(sgNode.BorderLeft) {
			sgNode.BorderLeft[rank] = curr
		}
	} else {
		if rank < len(sgNode.BorderRight) {
			sgNode.BorderRight[rank] = curr
		}
	}

	g.SetParent(curr, sg)
	if prev != "" {
		g.SetEdgeVW(prev, curr, &EdgeLabel{Weight: 1, MinLen: 1})
	}
}
