// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/phase2.laneAwareCompact.ts (MIT).

package swimlanes

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

func topoSortByGenerationIfAcyclic(g *Graph) []NodeId {
	indeg := buildInDegreeMap(g)
	adj := buildSortedSuccessorMap(g)
	frontier := sortedZeroInDegreeNodes(g, indeg)
	order := []NodeId{}

	for len(frontier) > 0 {
		nextFrontier := []NodeId{}
		for _, u := range frontier {
			order = append(order, u)
			for _, v := range adj[u] {
				indeg.Set(v, indeg.Get(v)-1)
				if indeg.Get(v) == 0 {
					nextFrontier = append(nextFrontier, v)
				}
			}
		}
		sortFunc(nextFrontier, func(a, b NodeId) float64 { return localeCompare(a, b) })
		frontier = nextFrontier
	}

	if len(order) == len(g.Nodes) {
		return order
	}
	return nil
}

// assignLayers_LaneAwareCompact is the lane-aware compact layering: one
// node per (layer, lane); inter-lane edges can stay on the same layer. opts
// can be nil.
func assignLayers_LaneAwareCompact(gAcyclic *Graph, opts *LayeringOptions) *Layering {
	if opts == nil {
		opts = &LayeringOptions{}
	}
	g := normalizeGraph(gAcyclic)
	var order []NodeId
	if opts.Direction == "LR" {
		order = topoSortByGenerationIfAcyclic(g)
	} else {
		order = topoSortIfAcyclic(g)
	}
	if order == nil {
		order = append([]NodeId{}, g.Nodes...)
		js.SortStrings(order)
	}

	// Determine a lane id for each node: the top-level parent id, or the node
	// id if there is none.
	topLaneOf := createTopLaneResolver(g)
	laneOf := func(id NodeId) string {
		if lane := topLaneOf(id); lane != "" {
			return lane
		}
		return id
	}

	rankOf := map[NodeId]int{}
	nextFree := map[string]int{}

	// Edge weight w(u,v) = 1 if same lane else 0, when ignoring cross-lane
	// constraints; otherwise 1 for all edges.
	edgeWeight := func(u, v NodeId) int {
		ignoreCrossLane := opts.IgnoreCrossLaneEdges.or(true)
		if ignoreCrossLane {
			if laneOf(u) == laneOf(v) {
				return 1
			}
			return 0
		}
		return 1
	}

	for _, v := range order {
		node := g.NodeById.Get(v)
		if node != nil && node.IsGroup {
			// Do not assign ranks/capacity to lane/group containers.
			continue
		}
		preds := incoming(g, v)
		base := 0
		if len(preds) > 0 {
			for _, e := range preds {
				u := e.Src
				ru := rankOr0(rankOf, u)
				base = max(base, ru+edgeWeight(u, v))
			}
		}
		lane := laneOf(v)
		nf := nextFree[lane]
		L := max(base, nf)
		rankOf[v] = L
		nextFree[lane] = L + 1
	}

	layers := buildLayersFromRanks(g, order, rankOf, true)

	return &Layering{Layers: layers, RankOf: rankOf, Dummy: js.NewSet[NodeId]()}
}
