// Package architecture is the architecture diagram.
//
// Ported from mermaid 12.2.0, diagrams/architecture/architectureDiagram.ts
// and architectureDetector.ts (MIT). The other files of the package port
// architectureDb.ts, architectureTypes.ts, architectureParser.ts,
// architectureIcons.ts, architectureRenderer.ts and svgDraw.ts, and the
// grammar parser/src/language/architecture/architecture.langium with its
// value converter.
//
// The renderer lays the diagram out with the fcose layout of cytoscape.
// The part of architectureRenderer.ts that talks to cytoscape
// (layoutArchitecture) and architectureSeed.ts run as JavaScript in
// internal/cytoscape.
package architecture

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:    "architecture",
		New:   func(ctx *render.Context) render.DB { return NewDB(ctx) },
		Parse: parse,
		Draw:  drawDiagram,
	})
}

func drawDiagram(ctx *render.Context, rdb render.DB, text string) error {
	db, ok := rdb.(*ArchitectureDB)
	if !ok {
		return errNotArchitectureDB
	}
	return draw(ctx, db, text)
}
