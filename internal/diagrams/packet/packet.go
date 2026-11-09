// Package packet is the packet diagram.
//
// Ported from mermaid 12.2.0, diagrams/packet/diagram.ts, db.ts, parser.ts and
// renderer.ts, and the grammar parser/src/language/packet/packet.langium
// (MIT).
package packet

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:    "packet",
		New:   func(ctx *render.Context) render.DB { return newDB(ctx) },
		Parse: parseDiagram,
		Draw:  draw,
	})
}
