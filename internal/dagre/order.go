// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/order/index.js (MIT).

package dagre

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
)

// order applies heuristics to minimize edge crossings in the graph and sets
// the best order solution as an order attribute on each node.
//
// Pre-conditions:
//
//  1. Graph must be DAG.
//  2. Graph nodes must be objects with a "rank" attribute.
//  3. Graph edges must have the "weight" attribute.
//
// Post-conditions:
//
//  1. Graph nodes have an "order" attribute based on the results of the
//     algorithm.
func (l *layout) order(g *graphlib.Graph) {
	maxRank := orNaN(maxRank(g))
	downLayerGraphs := l.buildLayerGraphs(g, rangeStartEnd(1, maxRank+1), inEdges)
	upLayerGraphs := l.buildLayerGraphs(g, rangeStartEndStep(maxRank-1, -1, -1), outEdges)

	layering := initOrder(l, g)
	assignOrder(g, layering)

	bestCC := math.Inf(1)
	var best [][]string

	for i, lastBest := 0, 0; lastBest < 4; i, lastBest = i+1, lastBest+1 {
		layerGraphs := upLayerGraphs
		if i%2 != 0 {
			layerGraphs = downLayerGraphs
		}
		l.sweepLayerGraphs(layerGraphs, i%4 >= 2)

		layering = buildLayerMatrix(g)
		cc := l.crossCount(g, layering)
		if cc < bestCC {
			lastBest = 0
			best = cloneLayering(layering)
			bestCC = cc
		}
	}

	assignOrder(g, best)
}

// cloneLayering is _.cloneDeep for a layering matrix.
func cloneLayering(layering [][]string) [][]string {
	out := make([][]string, len(layering))
	for i, layer := range layering {
		out[i] = append([]string{}, layer...)
	}
	return out
}

func (l *layout) buildLayerGraphs(g *graphlib.Graph, ranks []float64, relationship relationship) []*graphlib.Graph {
	graphs := make([]*graphlib.Graph, len(ranks))
	for i, rank := range ranks {
		graphs[i] = l.buildLayerGraph(g, rank, relationship)
	}
	return graphs
}

func (l *layout) sweepLayerGraphs(layerGraphs []*graphlib.Graph, biasRight bool) {
	cg := graphlib.New(graphlib.Options{})
	for _, lg := range layerGraphs {
		root := lg.Graph().(*layerGraphLabel).root
		sorted := l.sortSubgraph(lg, root, cg, biasRight)
		for i, v := range sorted.vs {
			switch node := lg.Node(v).(type) {
			case *nodeLabel:
				node.order = float64(i)
			case *layerSubgraphLabel:
				// JavaScript sets order on the subgraph label. No code reads it.
			default:
				panic(typeError("Cannot set properties of undefined (setting 'order')"))
			}
		}
		addSubgraphConstraints(lg, cg, sorted.vs)
	}
}

func assignOrder(g *graphlib.Graph, layering [][]string) {
	for _, layer := range layering {
		for i, v := range layer {
			nodeOf(g, v).order = float64(i)
		}
	}
}
