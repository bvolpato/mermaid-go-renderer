// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/graphlib/alg/dfs.js,
// preorder.js and postorder.js (MIT).

package alg

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
)

// dfs is a depth first traversal of g that starts at the nodes vs. The
// order is "pre" or "post". An undirected graph is traversed through the
// neighbors of each node, a directed graph through the successors.
func dfs(g *graphlib.Graph, vs []string, order string) ([]string, error) {
	navigation := g.Neighbors
	if g.IsDirected() {
		navigation = g.Successors
	}
	acc := []string{}
	visited := map[string]bool{}
	for _, v := range vs {
		if !g.HasNode(v) {
			return nil, fmt.Errorf("Graph does not have node: %s", v)
		}

		doDfs(v, order == "post", visited, navigation, &acc)
	}
	return acc, nil
}

func doDfs(v string, postorder bool, visited map[string]bool, navigation func(string) []string, acc *[]string) {
	if !visited[v] {
		visited[v] = true

		if !postorder {
			*acc = append(*acc, v)
		}
		for _, w := range navigation(v) {
			doDfs(w, postorder, visited, navigation, acc)
		}
		if postorder {
			*acc = append(*acc, v)
		}
	}
}

// Preorder returns the nodes that are reachable from vs in depth first
// preorder. It fails when a node of vs is not in the graph.
func Preorder(g *graphlib.Graph, vs []string) ([]string, error) {
	return dfs(g, vs, "pre")
}

// Postorder returns the nodes that are reachable from vs in depth first
// postorder. It fails when a node of vs is not in the graph.
func Postorder(g *graphlib.Graph, vs []string) ([]string, error) {
	return dfs(g, vs, "post")
}
