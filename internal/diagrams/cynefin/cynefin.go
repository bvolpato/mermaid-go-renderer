// Package cynefin is the Cynefin framework diagram.
//
// Ported from mermaid 12.2.0, diagrams/cynefin/cynefinDiagram.ts (MIT). The
// other files of the package port cynefinDb.ts, cynefinParser.ts,
// cynefinBoundaries.ts and cynefinRenderer.ts, and the grammar
// parser/src/language/cynefin/cynefin.langium.
package cynefin

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:    "cynefin",
		New:   func(ctx *render.Context) render.DB { return newDB(ctx) },
		Parse: parseDiagram,
		Draw:  draw,
	})
}
