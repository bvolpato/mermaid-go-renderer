// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/rank/util.js (MIT).

package dagre

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
)

// longestPath initializes ranks for the input graph with the longest path
// algorithm. Nodes are pushed to the lowest layer possible, which leaves the
// bottom ranks wide and edges longer than necessary. The result is a good
// start for other rankers.
//
// Pre-conditions:
//
//  1. Input graph is a DAG.
//  2. Input graph node labels can be assigned properties.
//
// Post-conditions:
//
//  1. Each node has an (unnormalized) "rank" property.
func longestPath(g *graphlib.Graph) {
	visited := map[string]bool{}

	var dfs func(v string) float64
	dfs = func(v string) float64 {
		label := nodeOf(g, v)
		if visited[v] {
			return label.rank
		}
		visited[v] = true

		outEdges := g.OutEdges(v)
		ranks := make([]float64, 0, len(outEdges))
		for _, e := range outEdges {
			ranks = append(ranks, dfs(e.W)-edgeOf(g, e).minlen)
		}
		rank, ok := lodashMin(ranks)

		if !ok || math.IsInf(rank, 1) {
			rank = 0
		}

		label.rank = rank
		label.hasRank = true
		return rank
	}

	for _, v := range g.Sources() {
		dfs(v)
	}
}

// slack returns the amount of slack of the given edge: the difference
// between the length of the edge and its minimum length.
func slack(g *graphlib.Graph, e graphlib.Edge) float64 {
	return nodeOf(g, e.W).rank - nodeOf(g, e.V).rank - edgeOf(g, e).minlen
}
