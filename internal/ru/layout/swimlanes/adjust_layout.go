// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/adjustLayout.ts (MIT).

package swimlanes

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
	rugraph "github.com/bvolpato/mermaid-go-renderer/internal/ru/graph"
)

// applySwimlaneLineJumps draws a hop where two edges cross. It runs after
// the paint, on the drawn edge paths.
func applySwimlaneLineJumps(data4Layout *ru.LayoutData, context *rugraph.PaintContext, _ any) error {
	lineHopsConfig := js.Path(data4Layout.Config, "swimlane", "lineHops")
	if b, ok := lineHopsConfig.(bool); ok && !b {
		return nil
	}

	jumpStyle := "arc"
	if s, ok := lineHopsConfig.(string); ok && s == "gap" {
		jumpStyle = "gap"
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

	edges.ApplyLineJumpsToSvg(context.Measure.LayoutGroups().EdgePaths, edgeGeometries, edges.LineJumpConfig{
		Enabled:    true,
		JumpRadius: 6,
		JumpStyle:  jumpStyle,
	})
	return nil
}
