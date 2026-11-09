// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/phase2.longestPath.ts (MIT).

package swimlanes

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// assignLayers_LongestPath is the classic longest-path layering with
// optional compaction and lane awareness. opts can be nil.
func assignLayers_LongestPath(gAcyclic *Graph, opts *LayeringOptions) *Layering {
	if opts == nil {
		opts = &LayeringOptions{}
	}
	g := normalizeGraph(gAcyclic)
	order := topoSortIfAcyclic(g)
	if order == nil {
		order = append([]NodeId{}, g.Nodes...)
		js.SortStrings(order)
	}
	compact := opts.CompactSingleInput.or(false)

	topLaneOf := createTopLaneResolver(g)

	rankOf := map[NodeId]int{}
	for _, v := range order {
		incAll := incoming(g, v)
		inc := incAll
		if opts.IgnoreCrossLaneEdges.truthy() {
			inc = nil
			for _, e := range incAll {
				laneSrc := topLaneOf(e.Src)
				laneDst := topLaneOf(v)
				if laneSrc == "" || laneDst == "" || laneSrc == laneDst {
					inc = append(inc, e)
				}
			}
		}
		if len(inc) == 0 {
			rankOf[v] = 0
		} else if compact && len(inc) == 1 {
			u := inc[0].Src
			// Only compact if predecessor is in a different lane than v.
			laneU := topLaneOf(u)
			laneV := topLaneOf(v)
			if laneU != laneV {
				rankOf[v] = rankOr0(rankOf, u)
			} else {
				rankOf[v] = rankOr0(rankOf, u) + 1
			}
		} else {
			mx := 0
			first := true
			for _, e := range inc {
				r := rankOr0(rankOf, e.Src) + 1
				if first || r > mx {
					mx = r
					first = false
				}
			}
			rankOf[v] = mx
		}
	}

	// Optional: revisit ranks using a greedy crossing reduction respecting
	// precedence constraints.
	if opts.OptimizeRanksByCrossings.or(false) {
		rankOf = optimizeRanksByCrossings(g, rankOf)
	}

	if opts.IgnoreCrossLaneEdges.truthy() {
		adjustCrossLaneSources(g, rankOf)
	}

	layers := buildMultitreeLayerOrder(g, rankOf, topLaneOf)

	return &Layering{Layers: layers, RankOf: rankOf, Dummy: js.NewSet[NodeId]()}
}
