// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/laneOrdering.ts (MIT).

package swimlanes

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// AUTOMATIC_LANE_ORDERING_RESTARTS is the number of restarts of the
// automatic lane ordering.
const AUTOMATIC_LANE_ORDERING_RESTARTS = 8

// WeightedLaneEdge is the number of edges between two lanes.
type WeightedLaneEdge struct {
	A      string
	B      string
	Weight int
}

type candidateOrder struct {
	order          []string
	cost           int
	sourceDistance int
}

// hashString is the FNV-1a hash of the UTF-16 code units of input.
func hashString(input string) uint32 {
	hash := uint32(2166136261)
	for _, unit := range utf16.Encode([]rune(input)) {
		hash ^= uint32(unit)
		hash *= 16777619
	}
	return hash
}

// mulberry32 is a seeded random generator. Upstream adds to the state
// without a wrap; the operations that read it wrap to 32 bits.
func mulberry32(seed uint32) func() float64 {
	state := seed
	return func() float64 {
		state += 0x6d2b79f5
		t := state
		t = (t ^ (t >> 15)) * (t | 1)
		t ^= t + (t^(t>>7))*(t|61)
		return float64(t^(t>>14)) / 4294967296
	}
}

func deterministicShuffle(order []string, seed uint32) []string {
	shuffled := append([]string{}, order...)
	random := mulberry32(seed)
	for i := len(shuffled) - 1; i > 0; i-- {
		j := int(math.Floor(random() * float64(i+1)))
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	return shuffled
}

func sourceDistance(order []string, sourceIndex map[string]int) int {
	distance := 0
	for index, laneId := range order {
		at, ok := sourceIndex[laneId]
		if !ok {
			at = index
		}
		d := index - at
		if d < 0 {
			d = -d
		}
		distance += d
	}
	return distance
}

func laneArrangementCost(order []string, weights []*WeightedLaneEdge) int {
	position := make(map[string]int, len(order))
	for index, laneId := range order {
		position[laneId] = index
	}

	cost := 0
	for _, w := range weights {
		ai, okA := position[w.A]
		bi, okB := position[w.B]
		if !okA || !okB {
			continue
		}
		d := ai - bi
		if d < 0 {
			d = -d
		}
		cost += w.Weight * d
	}
	return cost
}

func buildWeightedLaneEdges(g *Graph) []*WeightedLaneEdge {
	sourceOrder := buildTopLaneOrder(g)
	if len(sourceOrder) < 2 {
		return []*WeightedLaneEdge{}
	}

	sourceIndex := make(map[string]int, len(sourceOrder))
	for index, laneId := range sourceOrder {
		sourceIndex[laneId] = index
	}
	topLaneOf := createTopLaneResolver(g)
	weights := js.NewMap[string, *WeightedLaneEdge]()

	for _, edge := range g.Layout.Edges {
		if edge.IsLayoutOnly {
			continue
		}
		src := edge.Start
		dst := edge.End
		if src == "" || dst == "" || !g.NodeById.Has(src) || !g.NodeById.Has(dst) {
			continue
		}

		laneA := topLaneOf(src)
		laneB := topLaneOf(dst)
		if laneA == "" || laneB == "" || laneA == laneB {
			continue
		}

		ia, okA := sourceIndex[laneA]
		ib, okB := sourceIndex[laneB]
		if !okA || !okB {
			continue
		}

		a, b := laneB, laneA
		if ia <= ib {
			a, b = laneA, laneB
		}
		key := a + "\x00" + b
		if existing, ok := weights.Lookup(key); ok {
			existing.Weight++
		} else {
			weights.Set(key, &WeightedLaneEdge{A: a, B: b, Weight: 1})
		}
	}

	return weights.Values()
}

func greedySwitch(startOrder []string, weights []*WeightedLaneEdge, sourceIndex map[string]int) *candidateOrder {
	order := append([]string{}, startOrder...)
	cost := laneArrangementCost(order, weights)
	changed := true
	sweeps := 0
	maxSweeps := max(1, len(order))

	for changed && sweeps < maxSweeps {
		changed = false
		sweeps++
		for i := 0; i+1 < len(order); i++ {
			order[i], order[i+1] = order[i+1], order[i]
			nextCost := laneArrangementCost(order, weights)
			if nextCost < cost {
				cost = nextCost
				changed = true
			} else {
				order[i], order[i+1] = order[i+1], order[i]
			}
		}
	}

	return &candidateOrder{
		order:          order,
		cost:           cost,
		sourceDistance: sourceDistance(order, sourceIndex),
	}
}

func isBetterCandidate(candidate, best *candidateOrder) bool {
	if candidate.cost != best.cost {
		return candidate.cost < best.cost
	}
	return candidate.sourceDistance < best.sourceDistance
}

func seedForRestart(sourceOrder []string, weights []*WeightedLaneEdge, restartIndex int) uint32 {
	sorted := append([]*WeightedLaneEdge{}, weights...)
	sortFunc(sorted, func(a, b *WeightedLaneEdge) float64 {
		if a.A == b.A {
			return localeCompare(a.B, b.B)
		}
		return localeCompare(a.A, b.A)
	})
	parts := make([]string, len(sorted))
	for i, w := range sorted {
		parts[i] = w.A + ":" + w.B + ":" + strconv.Itoa(w.Weight)
	}
	weightSignature := strings.Join(parts, "|")
	return hashString(strings.Join(sourceOrder, "|") + "#" + weightSignature + "#" + strconv.Itoa(restartIndex))
}

// optimizeTopLaneOrder orders the lanes so that the edges between lanes
// are short. restarts is the number of restarts: a negative number stands
// for undefined, which is AUTOMATIC_LANE_ORDERING_RESTARTS.
func optimizeTopLaneOrder(g *Graph, restarts int) []string {
	sourceOrder := buildTopLaneOrder(g)
	if len(sourceOrder) < 2 {
		return sourceOrder
	}

	weights := buildWeightedLaneEdges(g)
	if len(weights) == 0 {
		return sourceOrder
	}

	sourceIndex := make(map[string]int, len(sourceOrder))
	for index, laneId := range sourceOrder {
		sourceIndex[laneId] = index
	}
	best := greedySwitch(sourceOrder, weights, sourceIndex)
	if restarts < 0 {
		restarts = AUTOMATIC_LANE_ORDERING_RESTARTS
	}

	for i := 0; i < restarts; i++ {
		seed := seedForRestart(sourceOrder, weights, i)
		start := deterministicShuffle(sourceOrder, seed)
		candidate := greedySwitch(start, weights, sourceIndex)
		if isBetterCandidate(candidate, best) {
			best = candidate
		}
	}

	return best.order
}
