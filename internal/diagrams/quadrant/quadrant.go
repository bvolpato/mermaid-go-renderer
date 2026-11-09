// Package quadrant is the quadrant chart.
//
// Ported from mermaid 12.2.0, diagrams/quadrant-chart/quadrantDiagram.ts,
// quadrantDb.ts, quadrantBuilder.ts, quadrantRenderer.ts and utils.ts, and the
// semantic actions of diagrams/quadrant-chart/parser/quadrant.jison (MIT).
package quadrant

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:    "quadrantChart",
		New:   func(ctx *render.Context) render.DB { return newDB(ctx) },
		Parse: parseDiagram,
		Draw:  draw,
	})
}
