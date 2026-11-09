// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/rank/index.js (MIT).

package dagre

import "github.com/bvolpato/mermaid-go-renderer/internal/graphlib"

// rank assigns a rank to each node in the input graph that respects the
// "minlen" constraint of the edges between nodes.
//
// This basic structure is derived from Gansner, et al., "A Technique for
// Drawing Directed Graphs."
//
// Pre-conditions:
//
//  1. Graph must be a connected DAG.
//  2. Graph nodes must be objects.
//  3. Graph edges must have "weight" and "minlen" attributes.
//
// Post-conditions:
//
//  1. Graph nodes have a "rank" attribute. Ranks can start at any index
//     (also a negative one). A later stage fixes them up.
func (l *layout) rank(g *graphlib.Graph) {
	switch graphOf(g).ranker {
	case "network-simplex":
		l.networkSimplexRanker(g)
	case "tight-tree":
		tightTreeRanker(g)
	case "longest-path":
		longestPathRanker(g)
	default:
		l.networkSimplexRanker(g)
	}
}

// longestPathRanker is a fast and simple ranker, but the results are far
// from optimal.
func longestPathRanker(g *graphlib.Graph) {
	longestPath(g)
}

func tightTreeRanker(g *graphlib.Graph) {
	longestPath(g)
	feasibleTree(g)
}

func (l *layout) networkSimplexRanker(g *graphlib.Graph) {
	networkSimplex(g)
}
