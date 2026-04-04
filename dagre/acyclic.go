package dagre

// AcyclicRun removes cycles from the graph by reversing back-edges found via DFS.
func AcyclicRun(g *Graph) {
	fas := dfsFAS(g)
	for _, e := range fas {
		label := g.EdgeByKey(e)
		g.RemoveEdge(e)
		label.ForwardName = e.Name
		label.Reversed = true
		g.SetEdgeNamed(e.W, e.V, UniqueID("rev"), label)
	}
}

func dfsFAS(g *Graph) []Edge {
	var fas []Edge
	stack := make(map[string]bool)
	visited := make(map[string]bool)

	var dfs func(string)
	dfs = func(v string) {
		if visited[v] {
			return
		}
		visited[v] = true
		stack[v] = true
		for _, e := range g.OutEdges(v) {
			if stack[e.W] {
				fas = append(fas, e)
			} else {
				dfs(e.W)
			}
		}
		delete(stack, v)
	}

	for _, v := range g.Nodes() {
		dfs(v)
	}
	return fas
}

// AcyclicUndo restores reversed edges to their original direction.
func AcyclicUndo(g *Graph) {
	for _, e := range g.Edges() {
		label := g.EdgeByKey(e)
		if label.Reversed {
			g.RemoveEdge(e)
			label.Reversed = false
			fwd := label.ForwardName
			label.ForwardName = ""
			g.SetEdgeNamed(e.W, e.V, fwd, label)
		}
	}
}
