// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/order/init-order.js
// (MIT).

package dagre

import "github.com/bvolpato/mermaid-go-renderer/internal/graphlib"

// initOrder assigns an initial order value for each node by performing a
// DFS search that starts from the nodes in the first rank. Nodes get an
// order in their rank as they are first visited.
//
// This approach comes from Gansner, et al., "A Technique for Drawing
// Directed Graphs."
//
// It returns a layering matrix with an array per layer and each layer
// sorted by the order of its nodes.
func initOrder(l *layout, g *graphlib.Graph) [][]string {
	visited := map[string]bool{}
	simpleNodes := []string{}
	for _, v := range g.Nodes() {
		if len(g.Children(v)) == 0 {
			simpleNodes = append(simpleNodes, v)
		}
	}
	ranks := make([]float64, len(simpleNodes))
	for i, v := range simpleNodes {
		ranks[i] = nodeOf(g, v).rank
	}
	maxRank := orNaN(lodashMax(ranks))
	layers := make([][]string, len(rangeEnd(maxRank+1)))
	for i := range layers {
		layers[i] = []string{}
	}

	var dfs func(v string)
	dfs = func(v string) {
		if visited[v] {
			return
		}
		visited[v] = true
		node := nodeOf(g, v)
		layer := layerAt(layers, node.rank)
		*layer = append(*layer, v)
		for _, w := range g.Successors(v) {
			dfs(w)
		}
	}

	orderedVs := sortBy(l.state.Sort, simpleNodes, func(v string) sortKey {
		node := nodeOf(g, v)
		return sortKey{v: node.rank, defined: node.hasRank}
	})
	for _, v := range orderedVs {
		dfs(v)
	}

	return layers
}
