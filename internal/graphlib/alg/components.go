// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/graphlib/alg/components.js
// (MIT).

// Package alg is a port of the graph algorithms of dagre-d3-es 7.0.14
// (src/graphlib/alg).
package alg

import "github.com/bvolpato/mermaid-go-renderer/internal/graphlib"

// Components returns the connected components of g. Each component is a
// list of node ids.
func Components(g *graphlib.Graph) [][]string {
	visited := map[string]bool{}
	cmpts := [][]string{}
	var cmpt []string

	var dfs func(v string)
	dfs = func(v string) {
		if visited[v] {
			return
		}
		visited[v] = true
		cmpt = append(cmpt, v)
		for _, w := range g.Successors(v) {
			dfs(w)
		}
		for _, w := range g.Predecessors(v) {
			dfs(w)
		}
	}

	for _, v := range g.Nodes() {
		cmpt = []string{}
		dfs(v)
		if len(cmpt) > 0 {
			cmpts = append(cmpts, cmpt)
		}
	}

	return cmpts
}
