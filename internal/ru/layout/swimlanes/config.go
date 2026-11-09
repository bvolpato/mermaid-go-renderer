// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/config.ts (MIT).

package swimlanes

// Edge routing and spacing constants (EDGE_ROUTING).
const (
	// EDGE_ROUTING_EDGE_GAP is the spacing between parallel edges in the same
	// corridor (px).
	EDGE_ROUTING_EDGE_GAP = 12
	// EDGE_ROUTING_LANE_MARGIN is the horizontal offset from a lane boundary
	// to the corridor center (px).
	EDGE_ROUTING_LANE_MARGIN = 20
)

// PRECISION_EPSILON is the epsilon for floating-point comparisons
// (PRECISION.EPSILON).
const PRECISION_EPSILON = 1e-6

// Layer assignment constants (LAYERING).
const (
	// LAYERING_GRAVITY_ITERATIONS is the number of iterations of the
	// gravity-based layering.
	LAYERING_GRAVITY_ITERATIONS = 8
	// LAYERING_MAX_CROSSING_OPTIMIZATION_PASSES is the maximum number of
	// passes of the crossing-based rank optimization.
	LAYERING_MAX_CROSSING_OPTIMIZATION_PASSES = 4
	// LAYERING_DEFAULT_COMPACT_SINGLE_INPUT says whether single-input nodes
	// are compacted by default.
	LAYERING_DEFAULT_COMPACT_SINGLE_INPUT = true
)

// Coordinate assignment constants (COORDINATES).
const (
	// COORDINATES_DEFAULT_LAYER_GAP is the default vertical gap between
	// layers (px).
	COORDINATES_DEFAULT_LAYER_GAP = 100
	// COORDINATES_DEFAULT_NODE_GAP is the default horizontal gap between
	// nodes (px).
	COORDINATES_DEFAULT_NODE_GAP = 40
)
