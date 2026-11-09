// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/graphlib/alg/prim.js (MIT).

package alg

import (
	"errors"
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
)

// ErrNotConnected is the error of Prim for a graph that is not connected.
var ErrNotConnected = errors.New("Input graph is not connected")

// Prim returns a minimum spanning tree of the connected graph g as a new
// directed graph without labels. Each tree edge goes from a node to its
// parent in the tree.
func Prim(g *graphlib.Graph, weightFunc WeightFunc) (*graphlib.Graph, error) {
	result := graphlib.New(graphlib.Options{})
	parents := map[string]string{}
	pq := newPriorityQueue()
	var v string

	updateNeighbors := func(edge graphlib.Edge) error {
		w := edge.V
		if edge.V == v {
			w = edge.W
		}
		pri, ok := pq.priority(w)
		if ok {
			edgeWeight := weightFunc(edge)
			if edgeWeight < pri {
				parents[w] = v
				if err := pq.decrease(w, edgeWeight); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if g.NodeCount() == 0 {
		return result, nil
	}

	for _, v := range g.Nodes() {
		pq.add(v, math.Inf(1))
		result.SetNode(v)
	}

	// Start from an arbitrary node.
	if err := pq.decrease(g.Nodes()[0], 0); err != nil {
		return nil, err
	}

	init := false
	for pq.size() > 0 {
		v = pq.removeMin()
		if parent, ok := parents[v]; ok {
			if err := result.SetEdgeObj(graphlib.Edge{V: v, W: parent}); err != nil {
				return nil, err
			}
		} else if init {
			return nil, ErrNotConnected
		} else {
			init = true
		}

		for _, edge := range g.NodeEdges(v) {
			if err := updateNeighbors(edge); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}
