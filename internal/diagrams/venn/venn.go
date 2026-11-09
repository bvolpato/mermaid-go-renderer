// Package venn is the venn diagram.
//
// Ported from mermaid 12.2.0, diagrams/venn/vennDiagram.ts (MIT). The other
// files of the package port vennDB.ts, vennTypes.ts, the semantic actions of
// parser/venn.jison and vennRenderer.ts. The layout library @upsetjs/venn.js
// is in internal/venn.
package venn

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:    "venn",
		New:   func(ctx *render.Context) render.DB { return NewDB(ctx) },
		Parse: parse,
		Draw:  draw,
	})
}

func parse(_ *render.Context, db render.DB, text string) error {
	d, ok := db.(*DB)
	if !ok {
		return fmt.Errorf("venn: unexpected db type %T", db)
	}
	return parseText(d, text)
}

func draw(ctx *render.Context, db render.DB, _ string) error {
	d, ok := db.(*DB)
	if !ok {
		return fmt.Errorf("venn: unexpected db type %T", db)
	}
	return drawVenn(ctx, d)
}
