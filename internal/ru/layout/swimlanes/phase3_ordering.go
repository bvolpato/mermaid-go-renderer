// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/phase3.ordering.ts (MIT).
//
// totalCrossings of upstream is not ported: nothing in the layout calls it,
// and the build of mermaid does not contain it.

package swimlanes

import (
	"math"
)

// SweepDirection is "down" or "up".
type SweepDirection = string

func median(values []int) float64 {
	n := len(values)
	if n == 0 {
		return math.Inf(1)
	}
	a := append([]int{}, values...)
	sortFunc(a, func(x, y int) float64 { return float64(x - y) })
	if n%2 == 1 {
		return float64(a[(n-1)/2])
	}
	return 0.5 * float64(a[n/2-1]+a[n/2])
}

func barycenter(values []int) float64 {
	if len(values) == 0 {
		return math.Inf(1)
	}
	s := 0
	for _, v := range values {
		s += v
	}
	return float64(s) / float64(len(values))
}

// neighborPositionsFor returns, for every target node, the positions of
// its neighbors in the fixed layer. A node that is not a target has no
// entry.
func neighborPositionsFor(targetNodes []NodeId, fixedIndex map[NodeId]int, edges []*EdgeRef, direction SweepDirection) map[NodeId][]int {
	neighborPositions := make(map[NodeId][]int, len(targetNodes))
	for _, v := range targetNodes {
		neighborPositions[v] = []int{}
	}
	for _, e := range edges {
		if direction == "down" {
			if at, ok := fixedIndex[e.Src]; ok {
				if list, ok := neighborPositions[e.Dst]; ok {
					neighborPositions[e.Dst] = append(list, at)
				}
			}
		} else if at, ok := fixedIndex[e.Dst]; ok {
			if list, ok := neighborPositions[e.Src]; ok {
				neighborPositions[e.Src] = append(list, at)
			}
		}
	}
	return neighborPositions
}

func currentOrderTieBreak(a, b NodeId, currentLayerIndex map[NodeId]int) float64 {
	ia := currentLayerIndex[a]
	ib := currentLayerIndex[b]
	if ia != ib {
		return float64(ia - ib)
	}
	return localeCompare(a, b)
}

func orderingCountCrossingsBetweenAdjacent(upper, lower []NodeId, edges []*EdgeRef) int {
	// Filter edges between these two layers.
	upperSet := make(map[NodeId]bool, len(upper))
	for _, id := range upper {
		upperSet[id] = true
	}
	lowerSet := make(map[NodeId]bool, len(lower))
	for _, id := range lower {
		lowerSet[id] = true
	}
	upperIndex := buildLayerIndex(upper)
	lowerIndex := buildLayerIndex(lower)
	type pair struct{ u, v int }
	pairs := []pair{}
	for _, e := range edges {
		if upperSet[e.Src] && lowerSet[e.Dst] {
			pairs = append(pairs, pair{u: upperIndex[e.Src], v: lowerIndex[e.Dst]})
		}
	}
	// Sort by u, count inversions in v.
	sortFunc(pairs, func(a, b pair) float64 {
		if a.u == b.u {
			return float64(a.v - b.v)
		}
		return float64(a.u - b.u)
	})
	vs := make([]int, len(pairs))
	for i, p := range pairs {
		vs[i] = p.v
	}
	return countInversions(vs)
}

// sortByHeuristic sorts a subset of nodes by their median score relative to
// a fixed layer.
func sortByHeuristic(nodes []NodeId, neighborPositions map[NodeId][]int, currentLayerIndex map[NodeId]int) []NodeId {
	out := append([]NodeId{}, nodes...)
	sortFunc(out, func(a, b NodeId) float64 {
		sa := median(neighborPositions[a])
		sb := median(neighborPositions[b])
		if sa == sb {
			return currentOrderTieBreak(a, b, currentLayerIndex)
		}
		if math.IsInf(sa, 0) {
			return 1
		}
		if math.IsInf(sb, 0) {
			return -1
		}
		return sa - sb
	})
	return out
}

// reorderLayer reorders the target layer by the positions of the neighbors
// in the fixed layer. topLaneOf and laneOrder can be nil.
func reorderLayer(fixedLayer, targetLayer []NodeId, edges []*EdgeRef, direction SweepDirection, topLaneOf func(id NodeId) string, laneOrder []string) []NodeId {
	fixedIndex := buildLayerIndex(fixedLayer)
	currIndex := buildLayerIndex(targetLayer)
	neighborPositions := neighborPositionsFor(targetLayer, fixedIndex, edges, direction)

	// If no lane info, fall back to flat reorder (original behavior).
	if topLaneOf == nil || len(laneOrder) == 0 {
		return sortByHeuristic(targetLayer, neighborPositions, currIndex)
	}

	// Partition target layer nodes by lane. The lane "" is the null lane.
	byLane := map[string][]NodeId{}
	for _, id := range targetLayer {
		lane := topLaneOf(id)
		byLane[lane] = append(byLane[lane], id)
	}

	// Reorder nodes within each lane independently.
	result := []NodeId{}

	// First, place lane-grouped nodes in lane order.
	for _, lane := range laneOrder {
		nodesInLane := byLane[lane]
		if len(nodesInLane) == 0 {
			continue
		}
		sorted := sortByHeuristic(nodesInLane, neighborPositions, currIndex)
		result = append(result, sorted...)
	}

	// Then, handle null-lane nodes (long-edge dummies without a parent).
	// Compute their barycenter and insert them adjacent to the lane whose
	// center position is closest to their barycenter.
	nullNodes := byLane[""]
	if len(nullNodes) > 0 {
		// Sort null-lane nodes by their barycenter across the full layer.
		sorted := sortByHeuristic(nullNodes, neighborPositions, currIndex)

		// For each null-lane node, find the best insertion position based on
		// its connections to nodes already in the result.
		for _, nid := range sorted {
			// The barycenter position of this node's neighbors in the fixed
			// layer.
			bc := barycenter(neighborPositions[nid])

			// Insert before the first node whose fixed-layer neighbor position
			// is greater than this node's barycenter.
			bestIdx := len(result) // default: append at end
			if !math.IsInf(bc, 0) {
				for i, rid := range result {
					rBc := barycenter(neighborPositions[rid])
					if bc < rBc {
						bestIdx = i
						break
					}
				}
			}
			next := make([]NodeId, 0, len(result)+1)
			next = append(next, result[:bestIdx]...)
			next = append(next, nid)
			next = append(next, result[bestIdx:]...)
			result = next
		}
	}

	return result
}

// transposeImprove swaps adjacent nodes of the current layer while that
// lowers the number of crossings. hasNext is false when next is undefined.
// topLaneOf can be nil.
func transposeImprove(upper, current []NodeId, edges []*EdgeRef, next []NodeId, hasNext bool, topLaneOf func(id NodeId) string) []NodeId {
	best := append([]NodeId{}, current...)
	upperSet := make(map[NodeId]bool, len(upper))
	for _, id := range upper {
		upperSet[id] = true
	}
	layerSet := make(map[NodeId]bool, len(current))
	for _, id := range current {
		layerSet[id] = true
	}
	var nextSet map[NodeId]bool
	if hasNext {
		nextSet = make(map[NodeId]bool, len(next))
		for _, id := range next {
			nextSet[id] = true
		}
	}

	var edgesIn []*EdgeRef
	for _, e := range edges {
		if upperSet[e.Src] && layerSet[e.Dst] {
			edgesIn = append(edgesIn, e)
		}
	}
	var edgesOut []*EdgeRef
	if hasNext {
		for _, e := range edges {
			if layerSet[e.Src] && nextSet[e.Dst] {
				edgesOut = append(edgesOut, e)
			}
		}
	}

	crossingScore := func(order []NodeId) int {
		score := orderingCountCrossingsBetweenAdjacent(upper, order, edgesIn)
		if hasNext {
			score += orderingCountCrossingsBetweenAdjacent(order, next, edgesOut)
		}
		return score
	}

	// Precompute lane membership for the same-lane check.
	var laneOf map[NodeId]string
	if topLaneOf != nil {
		laneOf = make(map[NodeId]string, len(current))
		for _, id := range current {
			laneOf[id] = topLaneOf(id)
		}
	}

	improved := true
	bestScore := crossingScore(best)
	for improved {
		improved = false
		for i := 0; i+1 < len(best); i++ {
			// Only swap nodes in the same lane (or both null-lane).
			if laneOf != nil {
				laneA := laneOf[best[i]]
				laneB := laneOf[best[i+1]]
				if laneA != laneB {
					continue // never swap across lane boundaries
				}
			}

			prev := bestScore
			best[i], best[i+1] = best[i+1], best[i]
			nextScore := crossingScore(best)
			if nextScore < prev {
				bestScore = nextScore
				improved = true
			} else {
				best[i], best[i+1] = best[i+1], best[i]
			}
		}
	}
	return best
}

// OrderOptions are the options of orderLayers. LaneOrder is nil when it is
// undefined.
type OrderOptions struct {
	LaneOrder []string
}

// orderLayers is Sugiyama phase 3: lane-aware median sweeps plus adjacent
// transpose improvements. opts can be nil.
func orderLayers(layering *Layering, gWithDummies *Graph, opts *OrderOptions) *OrderedLayers {
	// Start with a deterministic initial order per layer (preserve the given
	// order).
	layers := make([][]NodeId, len(layering.Layers))
	for i, l := range layering.Layers {
		layers[i] = append([]NodeId{}, l...)
	}
	edges := gWithDummies.Edges

	// Compute the lane order for lane-aware crossing minimization
	// (Siebenhaller Lemma 4.4).
	topLaneOf := createTopLaneResolver(gWithDummies)
	var preferred []string
	if opts != nil {
		preferred = opts.LaneOrder
	}
	laneOrder := resolveTopLaneOrder(gWithDummies, preferred)

	// layerAt is layers[i], which is undefined outside the list.
	layerAt := func(i int) ([]NodeId, bool) {
		if i < 0 || i >= len(layers) {
			return nil, false
		}
		return layers[i], true
	}

	// Perform top-down / bottom-up sweeps.
	for s := 0; s < 3; s++ {
		// Top-down: reorder layer i based on neighbors in layer i-1.
		for i := 1; i < len(layers); i++ {
			layers[i] = reorderLayer(layers[i-1], layers[i], edges, "down", topLaneOf, laneOrder)
			next, hasNext := layerAt(i + 1)
			layers[i] = transposeImprove(layers[i-1], layers[i], edges, next, hasNext, topLaneOf)
		}
		// Bottom-up: reorder layer i based on neighbors in layer i+1.
		for i := len(layers) - 2; i >= 0; i-- {
			layers[i] = reorderLayer(layers[i+1], layers[i], edges, "up", topLaneOf, laneOrder)
			next, hasNext := layerAt(i - 1)
			layers[i] = transposeImprove(layers[i+1], layers[i], edges, next, hasNext, topLaneOf)
		}
	}

	return &OrderedLayers{Layers: layers}
}
