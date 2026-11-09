// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/phase2.crossLaneAdjust.ts (MIT).

package swimlanes

// adjustCrossLaneSources is a heuristic that pushes nodes with only
// cross-lane outgoing edges downward, so that same-lane successors have room
// to appear above them.
func adjustCrossLaneSources(g *Graph, rankOf map[NodeId]int) {
	topLaneOf := createTopLaneResolver(g)

	nodesByRank := append([]NodeId{}, g.Nodes...)
	sortFunc(nodesByRank, func(a, b NodeId) float64 {
		if d := rankOr0(rankOf, a) - rankOr0(rankOf, b); d != 0 {
			return float64(d)
		}
		return localeCompare(a, b)
	})
	for _, v := range nodesByRank {
		laneV := topLaneOf(v)
		if laneV == "" {
			continue
		}
		var outEdges []*EdgeRef
		for _, e := range g.Edges {
			if e.Src == v {
				outEdges = append(outEdges, e)
			}
		}
		if len(outEdges) == 0 {
			continue
		}
		hasSameLaneSucc := false
		crossLaneCount := 0
		for _, e := range outEdges {
			laneDst := topLaneOf(e.Dst)
			if laneDst == "" || laneDst == laneV {
				hasSameLaneSucc = true
			} else {
				crossLaneCount++
			}
		}
		if crossLaneCount == 0 || hasSameLaneSucc {
			continue
		}

		crossLaneIncoming := 0
		hasSameLanePred := false
		for _, e := range g.Edges {
			if e.Dst != v {
				continue
			}
			laneSrc := topLaneOf(e.Src)
			if laneSrc == "" {
				continue
			}
			if laneSrc == laneV {
				hasSameLanePred = true
			} else {
				crossLaneIncoming++
			}
		}
		if crossLaneIncoming > 0 || !hasSameLanePred {
			continue
		}
		current := rankOr0(rankOf, v)
		target := current + crossLaneCount
		// Ensure we still respect predecessor constraints.
		lb := 0
		for _, e := range g.Edges {
			if e.Dst == v {
				lb = max(lb, rankOr0(rankOf, e.Src)+1)
			}
		}
		newRank := max(current, lb, target)
		if newRank != current {
			rankOf[v] = newRank
		}
	}
}
