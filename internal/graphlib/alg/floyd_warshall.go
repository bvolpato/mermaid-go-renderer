// Ported from dagre-d3-es 7.0.14,
// dagre-d3-es/src/graphlib/alg/floyd-warshall.js (MIT).

package alg

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// FloydWarshall finds the shortest paths between all pairs of nodes. A nil
// weightFn gives every edge the weight 1. A nil edgeFn follows the out-edges
// of each node. The result maps v to w to the entry for the path from v to w.
func FloydWarshall(g *graphlib.Graph, weightFn WeightFunc, edgeFn EdgeFunc) *js.Obj[*js.Obj[*PathEntry]] {
	if weightFn == nil {
		weightFn = defaultWeightFunc
	}
	if edgeFn == nil {
		edgeFn = func(v string) []graphlib.Edge { return g.OutEdges(v) }
	}
	return runFloydWarshall(g, weightFn, edgeFn)
}

func runFloydWarshall(g *graphlib.Graph, weightFn WeightFunc, edgeFn EdgeFunc) *js.Obj[*js.Obj[*PathEntry]] {
	results := js.NewObj[*js.Obj[*PathEntry]]()
	nodes := g.Nodes()

	for _, v := range nodes {
		row := js.NewObj[*PathEntry]()
		results.Set(v, row)
		row.Set(v, &PathEntry{Distance: 0})
		for _, w := range nodes {
			if v != w {
				row.Set(w, &PathEntry{Distance: math.Inf(1)})
			}
		}
		for _, edge := range edgeFn(v) {
			w := edge.V
			if edge.V == v {
				w = edge.W
			}
			d := weightFn(edge)
			row.Set(w, &PathEntry{Distance: d, Predecessor: v, HasPredecessor: true})
		}
	}

	for _, k := range nodes {
		rowK := results.Get(k)
		for _, i := range nodes {
			rowI := results.Get(i)
			for _, j := range nodes {
				ik := rowI.Get(k)
				kj := rowK.Get(j)
				ij := rowI.Get(j)
				altDistance := ik.Distance + kj.Distance
				if altDistance < ij.Distance {
					ij.Distance = altDistance
					ij.Predecessor = kj.Predecessor
					ij.HasPredecessor = kj.HasPredecessor
				}
			}
		}
	}

	return results
}
