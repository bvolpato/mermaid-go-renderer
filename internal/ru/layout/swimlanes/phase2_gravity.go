// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/phase2.gravity.ts (MIT).

package swimlanes

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// assignLayers_Gravity is the gravity-based layering algorithm that
// minimizes edge lengths while maintaining acyclicity. opts can be nil.
func assignLayers_Gravity(gAcyclic *Graph, opts *LayeringOptions) *Layering {
	if opts == nil {
		opts = &LayeringOptions{}
	}
	g := normalizeGraph(gAcyclic)
	// Initial ranks from longest path (gives feasible lower bounds).
	base := assignLayers_LongestPath(g, &LayeringOptions{
		CompactSingleInput:       opts.CompactSingleInput,
		IgnoreCrossLaneEdges:     opts.IgnoreCrossLaneEdges,
		OptimizeRanksByCrossings: opts.OptimizeRanksByCrossings,
	})
	rankOf := make(map[NodeId]int, len(base.RankOf))
	for k, v := range base.RankOf {
		rankOf[k] = v
	}

	topLaneOf := createTopLaneResolver(g)

	preds, succs := buildPredecessorSuccessorMaps(g, func(e *EdgeRef) bool {
		if opts.IgnoreCrossLaneEdges.truthy() {
			laneSrc := topLaneOf(e.Src)
			laneDst := topLaneOf(e.Dst)
			if laneSrc != "" && laneDst != "" && laneSrc != laneDst {
				return false
			}
		}
		return true
	})

	order := topoSortIfAcyclic(g)
	if order == nil {
		order = append([]NodeId{}, g.Nodes...)
	}
	revOrder := make([]NodeId, len(order))
	for i, v := range order {
		revOrder[len(order)-1-i] = v
	}

	// minSuccRank is Math.min(...s.map((w) => (rankOf[w] ?? 0) - 1)) for a
	// list that is not empty.
	minSuccRank := func(s []NodeId) int {
		ub := rankOr0(rankOf, s[0]) - 1
		for _, w := range s[1:] {
			ub = min(ub, rankOr0(rankOf, w)-1)
		}
		return ub
	}

	clampFeasible := func(v NodeId, desired int) int {
		// Lower bound from predecessors: max(rank[u] + 1).
		lb := 0
		for _, u := range preds[v] {
			lb = max(lb, rankOr0(rankOf, u)+1)
		}
		// Upper bound from successors: min(rank[w] - 1). Without successors
		// the bound is infinite upstream, and then it is max(lb, desired).
		var ub int
		if s := succs[v]; len(s) > 0 {
			ub = minSuccRank(s)
		} else {
			ub = max(lb, desired)
		}
		return min(max(desired, lb), ub)
	}

	// Iterative relaxation.
	const iters = LAYERING_GRAVITY_ITERATIONS
	relaxOrder := func(nodeOrder []NodeId) bool {
		changed := false
		for _, v := range nodeOrder {
			ps := preds[v]
			ss := succs[v]
			if len(ps) == 0 && len(ss) == 0 {
				continue
			}
			predAvg := float64(rankOr0(rankOf, v))
			if len(ps) > 0 {
				a := 0.0
				for _, u := range ps {
					a = a + float64(rankOr0(rankOf, u)) + 1
				}
				predAvg = a / float64(len(ps))
			}
			succAvg := float64(rankOr0(rankOf, v))
			if len(ss) > 0 {
				a := 0.0
				for _, w := range ss {
					a = a + float64(rankOr0(rankOf, w)) - 1
				}
				succAvg = a / float64(len(ss))
			}
			desired := int(js.Round((predAvg + succAvg) / 2))
			clamped := clampFeasible(v, desired)
			if current, ok := rankOf[v]; !ok || clamped != current {
				rankOf[v] = clamped
				changed = true
			}
		}
		return changed
	}

	for it := 0; it < iters; it++ {
		forwardChanged := relaxOrder(order)
		// The backward pass helps propagate upper bounds.
		backwardChanged := relaxOrder(revOrder)
		if !forwardChanged && !backwardChanged {
			break
		}
	}

	// Final feasibility fix-ups (ensure r(v) >= r(u)+1).
	for _, v := range order {
		lb := 0
		for _, u := range preds[v] {
			lb = max(lb, rankOr0(rankOf, u)+1)
		}
		if rankOr0(rankOf, v) < lb {
			rankOf[v] = lb
		}
	}
	for _, v := range revOrder {
		if s := succs[v]; len(s) > 0 {
			ub := minSuccRank(s)
			if rankOr0(rankOf, v) > ub {
				rankOf[v] = ub
			}
		}
	}

	layers := buildLayersFromRanks(g, order, rankOf, false)

	return &Layering{Layers: layers, RankOf: rankOf, Dummy: js.NewSet[NodeId]()}
}
