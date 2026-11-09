// Package radar is the radar chart.
//
// Ported from mermaid 12.2.0, diagrams/radar/diagram.ts, db.ts, parser.ts and
// renderer.ts, and the grammar parser/src/language/radar/radar.langium (MIT).
package radar

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:    "radar",
		New:   func(ctx *render.Context) render.DB { return newDB(ctx) },
		Parse: parseDiagram,
		Draw:  draw,
	})
}
