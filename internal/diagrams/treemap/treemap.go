// Package treemap is the treemap diagram.
//
// Ported from mermaid 12.2.0, diagrams/treemap/diagram.ts (MIT). The other
// files of the package port db.ts, parser.ts, utils.ts and renderer.ts, and
// the grammar parser/src/language/treemap/treemap.langium with its value
// converter.
package treemap

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:      "treemap",
		New:     func(ctx *render.Context) render.DB { return NewDB(ctx) },
		Parse:   parse,
		Draw:    draw,
		Classes: getClasses,
	})
}

func parse(_ *render.Context, db render.DB, text string) error {
	d, ok := db.(*DB)
	if !ok {
		return fmt.Errorf("treemap: unexpected db type %T", db)
	}
	return parseText(d, text)
}

func draw(ctx *render.Context, db render.DB, _ string) error {
	d, ok := db.(*DB)
	if !ok {
		return fmt.Errorf("treemap: unexpected db type %T", db)
	}
	return drawTreemap(ctx, d)
}

// getClasses is renderer.getClasses(_text, diagramObj).
func getClasses(_ *render.Context, db render.DB, _ string) *js.Map[string, *render.ClassDef] {
	d, ok := db.(*DB)
	if !ok {
		return nil
	}
	return d.GetClasses()
}
