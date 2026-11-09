// Ported from mermaid 12.2.0, rendering-util/layout-algorithms/elk/lineHops.ts
// (MIT).

package elk

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
	rugraph "github.com/bvolpato/mermaid-go-renderer/internal/ru/graph"
)

// JUMP_RADIUS is the radius of the arc drawn where one edge hops another.
// It is fixed rather than configurable: elk.lineHops exposes the style
// (arc or gap) but not the radius.
const JUMP_RADIUS = 6

// applyElkLineJumps draws a hop where two edges cross.
//
// It runs after the paint, because a hop is a property of the rendered path
// rather than of the layout: the crossings are only known once every edge
// has been emitted, and the fix is to rewrite the `d` of the edge that
// gives way.
func applyElkLineJumps(data4Layout *ru.LayoutData, context *rugraph.PaintContext, _ any) error {
	lineHops := js.Path(data4Layout.Config, "elk", "lineHops")
	if strictEq(lineHops, false) {
		return nil
	}

	edgeGeometries := []edges.EdgeGeom{}
	for _, edge := range data4Layout.Edges {
		// Array.isArray(edge.points) && edge.points.length >= 2
		if len(edge.Points) < 2 {
			continue
		}
		edgeGeometries = append(edgeGeometries, edges.EdgeGeom{
			ID:             edge.ID,
			Points:         edge.Points,
			Curve:          edge.Curve,
			ArrowTypeStart: edge.ArrowTypeStart,
			ArrowTypeEnd:   edge.ArrowTypeEnd,
		})
	}

	jumpStyle := "arc"
	if strictEq(lineHops, "gap") {
		jumpStyle = "gap"
	}
	edges.ApplyLineJumpsToSvg(context.Measure.LayoutGroups().EdgePaths, edgeGeometries, edges.LineJumpConfig{
		Enabled:    true,
		JumpRadius: JUMP_RADIUS,
		JumpStyle:  jumpStyle,
	})
	return nil
}
