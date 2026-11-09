// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/acyclic.js (MIT).

package dagre

import "github.com/bvolpato/mermaid-go-renderer/internal/graphlib"

func (l *layout) acyclicRun(g *graphlib.Graph) {
	var fas []graphlib.Edge
	if graphOf(g).acyclicer == "greedy" {
		fas = greedyFAS(g, func(e graphlib.Edge) float64 { return edgeOf(g, e).weight })
	} else {
		fas = dfsFAS(g)
	}
	for _, e := range fas {
		label := edgeOf(g, e)
		g.RemoveEdgeObj(e)
		label.forwardName = e.Name
		label.reversed = true
		setEdge(g, e.W, e.V, label, l.state.UniqueID("rev"))
	}
}

func dfsFAS(g *graphlib.Graph) []graphlib.Edge {
	fas := []graphlib.Edge{}
	stack := map[string]bool{}
	visited := map[string]bool{}

	var dfs func(v string)
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

func acyclicUndo(g *graphlib.Graph) {
	for _, e := range g.Edges() {
		label := edgeOf(g, e)
		if label.reversed {
			g.RemoveEdgeObj(e)

			forwardName := label.forwardName
			label.reversed = false
			label.forwardName = ""
			setEdge(g, e.W, e.V, label, forwardName)
		}
	}
}
