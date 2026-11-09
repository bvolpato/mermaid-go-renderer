// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/phase2.crossOptimization.ts
// (MIT).

package swimlanes

import "strconv"

func crossOptCountCrossingsBetweenAdjacent(upper, lower []NodeId, edges []*EdgeRef) int {
	upperSet := make(map[NodeId]bool, len(upper))
	for _, id := range upper {
		upperSet[id] = true
	}
	lowerSet := make(map[NodeId]bool, len(lower))
	for _, id := range lower {
		lowerSet[id] = true
	}
	li := buildLayerIndex(lower)
	vs := []int{}
	for _, e := range edges {
		if upperSet[e.Src] && lowerSet[e.Dst] {
			vs = append(vs, li[e.Dst])
		}
	}
	return countInversions(vs)
}

func crossOptTotalCrossings(layers [][]NodeId, edges []*EdgeRef, rankOf map[NodeId]int) int {
	expanded := []*EdgeRef{}
	for _, e := range edges {
		ru, hasRu := rankOf[e.Src]
		rv, hasRv := rankOf[e.Dst]
		if !hasRu || !hasRv || ru == rv {
			continue
		}
		upper := e.Src
		lower := e.Dst
		rUpper := ru
		rLower := rv
		if ru > rv {
			upper = e.Dst
			lower = e.Src
			rUpper = rv
			rLower = ru
		}
		for L := rUpper; L < rLower; L++ {
			expanded = append(expanded, &EdgeRef{ID: e.ID + "@" + strconv.Itoa(L), Src: upper, Dst: lower, Ref: e.Ref})
		}
	}
	sum := 0
	for i := 0; i+1 < len(layers); i++ {
		sum += crossOptCountCrossingsBetweenAdjacent(layers[i], layers[i+1], expanded)
	}
	return sum
}

// optimizeRanksByCrossings is a greedy local search that tries to lift
// nodes to reduce crossings while preserving acyclicity (rank(v) >=
// rank(u)+1 for every edge u->v).
func optimizeRanksByCrossings(g *Graph, initialRank map[NodeId]int) map[NodeId]int {
	rankOf := make(map[NodeId]int, len(initialRank))
	for k, v := range initialRank {
		rankOf[k] = v
	}
	preds, _ := buildPredecessorSuccessorMaps(g, nil)

	laneOf := createTopLaneResolver(g)

	layers := buildMultitreeLayerOrder(g, rankOf, laneOf)
	best := crossOptTotalCrossings(layers, g.Edges, rankOf)
	const maxPasses = LAYERING_MAX_CROSSING_OPTIMIZATION_PASSES
	for pass := 0; pass < maxPasses; pass++ {
		changed := false
		nodesByRank := append([]NodeId{}, g.Nodes...)
		sortFunc(nodesByRank, func(a, b NodeId) float64 {
			return float64(rankOr0(rankOf, b) - rankOr0(rankOf, a))
		})
		for _, v := range nodesByRank {
			r := rankOr0(rankOf, v)
			if r == 0 {
				continue
			}
			lb := 0
			for _, u := range preds[v] {
				lb = max(lb, rankOr0(rankOf, u)+1)
			}
			if lb >= r {
				continue
			}
			old := r
			rankOf[v] = lb
			trialLayers := buildMultitreeLayerOrder(g, rankOf, laneOf)
			score := crossOptTotalCrossings(trialLayers, g.Edges, rankOf)
			if score < best {
				best = score
				changed = true
			} else {
				rankOf[v] = old
			}
		}
		if !changed {
			break
		}
	}
	return rankOf
}
