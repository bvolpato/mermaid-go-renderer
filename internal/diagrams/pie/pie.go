// Package pie is the pie chart.
//
// Ported from mermaid 12.2.0, diagrams/pie/pieDiagram.ts, pieDb.ts,
// pieParser.ts and pieRenderer.ts, and the grammar
// parser/src/language/pie/pie.langium (MIT).
package pie

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:    "pie",
		New:   func(ctx *render.Context) render.DB { return newDB(ctx) },
		Parse: parseDiagram,
		Draw:  draw,
	})
}
