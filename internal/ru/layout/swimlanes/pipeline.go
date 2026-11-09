// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/pipeline.ts (MIT).

package swimlanes

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// LayoutOptions are the options of sugiyamaLayout.
type LayoutOptions struct {
	// Layering. CompactSingleInput defaults to true for compact swimlanes.
	CompactSingleInput       optBool
	IgnoreCrossLaneEdges     optBool
	OptimizeRanksByCrossings optBool
	AutomaticLaneOrdering    optBool
	// Coordinates. A gap that is undefined is NaN.
	LayerGap float64
	NodeGap  float64
	// Direction (for proper spacing calculation), or "" when it is
	// undefined.
	Direction string
}

// LayoutResult is the result of sugiyamaLayout.
type LayoutResult struct {
	Acyclic     *Graph
	Reversed    []*EdgeRef
	Layering    *Layering
	Ordered     *OrderedLayers
	Coordinates *Coordinates

	// The fields below are intermediate results that upstream does not
	// return. The tests compare them with upstream phase by phase.

	// Normalized is the graph after normalizeGraph.
	Normalized *Graph
	// LaneOrder is the lane order of the automatic lane ordering, or nil.
	LaneOrder []string
	// InitialLayers are the layers before the dummy nodes were added.
	InitialLayers [][]NodeId
	// GraphWithDummies is the graph with the dummy nodes of long edges.
	GraphWithDummies *Graph
}

// sugiyamaLayout runs the Sugiyama phases on a graph view.
func sugiyamaLayout(g *Graph, opts *LayoutOptions) *LayoutResult {
	ignoreCrossLaneEdges := opts.IgnoreCrossLaneEdges.or(true)
	optimizeRanksByCrossings := opts.OptimizeRanksByCrossings.or(true)
	g0 := normalizeGraph(g)
	var laneOrder []string
	if opts.AutomaticLaneOrdering.truthy() {
		laneOrder = optimizeTopLaneOrder(g0, AUTOMATIC_LANE_ORDERING_RESTARTS)
	}

	// Phase 1: cycle removal
	cycleRes := removeCycles_DFS(g0)
	gAcyclic := cycleRes.Acyclic

	// Phase 2: layering
	var layering *Layering
	if ignoreCrossLaneEdges {
		layering = assignLayers_LaneAwareCompact(gAcyclic, &LayeringOptions{
			CompactSingleInput:   optOf(opts.CompactSingleInput.or(LAYERING_DEFAULT_COMPACT_SINGLE_INPUT)),
			IgnoreCrossLaneEdges: optTrue,
			Direction:            opts.Direction,
		})
	} else {
		layering = assignLayers_Gravity(gAcyclic, &LayeringOptions{
			CompactSingleInput:       optOf(opts.CompactSingleInput.or(LAYERING_DEFAULT_COMPACT_SINGLE_INPUT)),
			IgnoreCrossLaneEdges:     optFalse,
			OptimizeRanksByCrossings: optOf(optimizeRanksByCrossings),
		})
	}
	initialLayers := layering.Layers
	properLayering, graphWithDummies := makeProperLayering(layering, gAcyclic)
	// Phase 3: ordering
	ordered := orderLayers(properLayering, graphWithDummies, &OrderOptions{LaneOrder: laneOrder})

	// Phase 4: coordinates
	coordinates := assignCoordinates(ordered, graphWithDummies, &CoordOptions{
		LayerGap:  opts.LayerGap,
		NodeGap:   opts.NodeGap,
		LaneGap:   ru.Undef,
		Direction: opts.Direction,
		LaneOrder: laneOrder,
	})

	return &LayoutResult{
		Acyclic:          gAcyclic,
		Reversed:         cycleRes.Reversed,
		Layering:         properLayering,
		Ordered:          ordered,
		Coordinates:      coordinates,
		Normalized:       g0,
		LaneOrder:        laneOrder,
		InitialLayers:    initialLayers,
		GraphWithDummies: graphWithDummies,
	}
}
