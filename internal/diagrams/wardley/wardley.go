// Package wardley is the Wardley map diagram.
//
// Ported from mermaid 12.2.0, diagrams/wardley/wardleyDiagram.ts (MIT). The
// other files of the package port wardleyDb.ts, wardleyBuilder.ts,
// wardleyParser.ts and wardleyRenderer.ts, and the grammar
// parser/src/language/wardley/wardley.langium with its value converter.
package wardley

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:    "wardley",
		New:   func(ctx *render.Context) render.DB { return newDB(ctx) },
		Parse: parseDiagram,
		Draw:  draw,
	})
}
