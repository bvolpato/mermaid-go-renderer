// Package sankey is the sankey diagram.
//
// Ported from mermaid 12.2.0, diagrams/sankey/sankeyDiagram.ts (MIT). The
// other files of the package port sankeyDB.ts, sankeyUtils.ts, the semantic
// actions of parser/sankey.jison and sankeyRenderer.ts.
package sankey

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:    "sankey",
		New:   func(ctx *render.Context) render.DB { return NewDB(ctx) },
		Parse: parse,
		Draw:  draw,
	})
}

// parse is the parse function that sankeyDiagram.ts installs: it prepares
// the text and then runs the generated parser.
func parse(_ *render.Context, db render.DB, text string) error {
	d, ok := db.(*DB)
	if !ok {
		return fmt.Errorf("sankey: unexpected db type %T", db)
	}
	return parseText(d, PrepareTextForParsing(text))
}

func draw(ctx *render.Context, db render.DB, _ string) error {
	d, ok := db.(*DB)
	if !ok {
		return fmt.Errorf("sankey: unexpected db type %T", db)
	}
	return drawSankey(ctx, d)
}
