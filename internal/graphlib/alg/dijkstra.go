// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/graphlib/alg/dijkstra.js and
// dijkstra-all.js (MIT).

package alg

import (
	"fmt"
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// PathEntry is one entry of a shortest path result: the distance from the
// source, and the predecessor on the shortest path. HasPredecessor is false
// for the source and for a node that is not reachable.
type PathEntry struct {
	Distance       float64
	Predecessor    string
	HasPredecessor bool
}

// WeightFunc returns the weight of an edge.
type WeightFunc func(e graphlib.Edge) float64

// EdgeFunc returns the edges to follow from a node.
type EdgeFunc func(v string) []graphlib.Edge

func defaultWeightFunc(graphlib.Edge) float64 { return 1 }

// Dijkstra finds the shortest paths from source to all other nodes. A nil
// weightFn gives every edge the weight 1. A nil edgeFn follows the out-edges
// of each node. The result has one entry for each node, in node order. It
// fails for a negative edge weight.
func Dijkstra(g *graphlib.Graph, source string, weightFn WeightFunc, edgeFn EdgeFunc) (*js.Obj[*PathEntry], error) {
	if weightFn == nil {
		weightFn = defaultWeightFunc
	}
	if edgeFn == nil {
		edgeFn = func(v string) []graphlib.Edge { return g.OutEdges(v) }
	}
	return runDijkstra(g, source, weightFn, edgeFn)
}

func runDijkstra(g *graphlib.Graph, source string, weightFn WeightFunc, edgeFn EdgeFunc) (*js.Obj[*PathEntry], error) {
	results := js.NewObj[*PathEntry]()
	pq := newPriorityQueue()
	var v string
	var vEntry *PathEntry

	updateNeighbors := func(edge graphlib.Edge) error {
		w := edge.W
		if edge.V != v {
			w = edge.V
		}
		wEntry := results.Get(w)
		weight := weightFn(edge)
		distance := vEntry.Distance + weight

		if weight < 0 {
			return fmt.Errorf("dijkstra does not allow negative edge weights. Bad edge: %s -> %s Weight: %s", edge.V, edge.W, js.Str(weight))
		}

		if wEntry == nil {
			return fmt.Errorf("dijkstra: edge %s -> %s leads to a node that is not in the graph", edge.V, edge.W)
		}
		if distance < wEntry.Distance {
			wEntry.Distance = distance
			wEntry.Predecessor = v
			wEntry.HasPredecessor = true
			if err := pq.decrease(w, distance); err != nil {
				return err
			}
		}
		return nil
	}

	for _, v := range g.Nodes() {
		distance := math.Inf(1)
		if v == source {
			distance = 0
		}
		results.Set(v, &PathEntry{Distance: distance})
		pq.add(v, distance)
	}

	for pq.size() > 0 {
		v = pq.removeMin()
		vEntry = results.Get(v)
		if math.IsInf(vEntry.Distance, 1) {
			break
		}

		for _, edge := range edgeFn(v) {
			if err := updateNeighbors(edge); err != nil {
				return nil, err
			}
		}
	}

	return results, nil
}

// DijkstraAll runs Dijkstra from every node. The result maps each source
// node to its Dijkstra result.
func DijkstraAll(g *graphlib.Graph, weightFn WeightFunc, edgeFn EdgeFunc) (*js.Obj[*js.Obj[*PathEntry]], error) {
	acc := js.NewObj[*js.Obj[*PathEntry]]()
	for _, v := range g.Nodes() {
		res, err := Dijkstra(g, v, weightFn, edgeFn)
		if err != nil {
			return nil, err
		}
		acc.Set(v, res)
	}
	return acc, nil
}
