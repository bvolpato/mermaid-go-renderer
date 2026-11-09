// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/graphlib/alg/tarjan.js and
// find-cycles.js (MIT).

package alg

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
)

// Tarjan returns the strongly connected components of g.
func Tarjan(g *graphlib.Graph) [][]string {
	type entry struct {
		onStack bool
		lowlink float64
		index   float64
	}
	index := 0.0
	stack := []string{}
	visited := map[string]*entry{}
	results := [][]string{}

	var dfs func(v string)
	dfs = func(v string) {
		e := &entry{onStack: true, lowlink: index, index: index}
		index++
		visited[v] = e
		stack = append(stack, v)

		for _, w := range g.Successors(v) {
			if visited[w] == nil {
				dfs(w)
				e.lowlink = min(e.lowlink, visited[w].lowlink)
			} else if visited[w].onStack {
				e.lowlink = min(e.lowlink, visited[w].index)
			}
		}

		if e.lowlink == e.index {
			cmpt := []string{}
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				visited[w].onStack = false
				cmpt = append(cmpt, w)
				if v == w {
					break
				}
			}
			results = append(results, cmpt)
		}
	}

	for _, v := range g.Nodes() {
		if visited[v] == nil {
			dfs(v)
		}
	}

	return results
}

// FindCycles returns the strongly connected components of g that have a
// cycle: components with more than one node, and single nodes with a
// self-loop.
func FindCycles(g *graphlib.Graph) [][]string {
	res := [][]string{}
	for _, cmpt := range Tarjan(g) {
		if len(cmpt) > 1 || (len(cmpt) == 1 && g.HasEdge(cmpt[0], cmpt[0])) {
			res = append(res, cmpt)
		}
	}
	return res
}
