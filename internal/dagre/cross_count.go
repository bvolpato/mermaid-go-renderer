// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/order/cross-count.js
// (MIT).

package dagre

import "github.com/bvolpato/mermaid-go-renderer/internal/graphlib"

// crossCount takes a layering (an array of layers, each with an array of
// ordered nodes) and a graph and returns a weighted crossing count.
//
// Pre-conditions:
//
//  1. Input graph must be simple (not a multigraph), directed, and include
//     only simple edges.
//  2. Edges in the input graph must have assigned weights.
//
// Post-conditions:
//
//  1. The graph and layering matrix are left unchanged.
//
// This algorithm is derived from Barth, et al., "Bilayer Cross Counting."
func (l *layout) crossCount(g *graphlib.Graph, layering [][]string) float64 {
	cc := 0.0
	for i := 1; i < len(layering); i++ {
		cc += l.twoLayerCrossCount(g, layering[i-1], layering[i])
	}
	return cc
}

func (l *layout) twoLayerCrossCount(g *graphlib.Graph, northLayer, southLayer []string) float64 {
	// Sort all of the edges between the north and south layers by their
	// position in the north layer and then the south. Map these edges to
	// the position of their head in the south layer.
	southPos := make(map[string]int, len(southLayer))
	for i, v := range southLayer {
		southPos[v] = i
	}
	type entry struct {
		pos    float64
		hasPos bool
		weight float64
	}
	southEntries := []entry{}
	for _, v := range northLayer {
		outEdges := g.OutEdges(v)
		entries := make([]entry, len(outEdges))
		for i, e := range outEdges {
			pos, ok := southPos[e.W]
			entries[i] = entry{pos: float64(pos), hasPos: ok, weight: edgeOf(g, e).weight}
		}
		southEntries = append(southEntries, sortBy(l.state.Sort, entries, func(e entry) sortKey {
			return sortKey{v: e.pos, defined: e.hasPos}
		})...)
	}

	// Build the accumulator tree.
	firstIndex := 1
	for firstIndex < len(southLayer) {
		firstIndex <<= 1
	}
	treeSize := 2*firstIndex - 1
	firstIndex -= 1
	tree := make([]float64, treeSize)

	// Calculate the weighted crossings.
	cc := 0.0
	for _, entry := range southEntries {
		weightSum := 0.0
		if entry.hasPos {
			index := int(entry.pos) + firstIndex
			tree[index] += entry.weight
			for index > 0 {
				if index%2 != 0 {
					weightSum += tree[index+1]
				}
				index = (index - 1) >> 1
				tree[index] += entry.weight
			}
		}
		// An edge to a node outside of the south layer has an undefined
		// position. The JavaScript code then adds weight * 0.
		cc += float64(entry.weight * weightSum)
	}

	return cc
}
