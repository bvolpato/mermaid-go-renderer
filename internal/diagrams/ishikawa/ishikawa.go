// Package ishikawa is the ishikawa (fishbone) diagram.
//
// Ported from mermaid 12.2.0, diagrams/ishikawa/ishikawaDiagram.ts (MIT). The
// other files of the package port ishikawaDb.ts, ishikawaTypes.ts, the
// semantic actions of parser/ishikawa.jison and ishikawaRenderer.ts.
package ishikawa

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:    "ishikawa",
		New:   func(ctx *render.Context) render.DB { return NewDB(ctx) },
		Parse: parse,
		Draw:  draw,
	})
}

func parse(_ *render.Context, db render.DB, text string) error {
	d, ok := db.(*DB)
	if !ok {
		return fmt.Errorf("ishikawa: unexpected db type %T", db)
	}
	return parseText(d, text)
}

func draw(ctx *render.Context, db render.DB, _ string) error {
	d, ok := db.(*DB)
	if !ok {
		return fmt.Errorf("ishikawa: unexpected db type %T", db)
	}
	return drawIshikawa(ctx, d)
}
