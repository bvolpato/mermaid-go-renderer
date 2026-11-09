// Package block is the block diagram: the diagram type block (the keywords
// block and block-beta).
//
// Ported from mermaid 12.2.0, diagrams/block/blockDiagram.ts and
// blockDetector.ts (MIT). The other files of the package port blockTypes.ts,
// blockDB.ts, layout.ts, renderHelpers.ts, blockRenderer.ts and the semantic
// actions of parser/block.jison. The detector is in internal/config, and
// styles.ts is in internal/styles. blockUtils.ts is not ported: nothing
// imports it.
//
// The blocks and the edges are drawn by the unified renderer (internal/ru).
// The shapes register themselves there: a program must link the shape
// packages too.
package block

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:  "block",
		New: func(ctx *render.Context) render.DB { return NewDB(ctx) },
		Parse: func(_ *render.Context, db render.DB, text string) error {
			d, ok := db.(*DB)
			if !ok {
				return fmt.Errorf("block: unexpected db type %T", db)
			}
			return parse(d, text)
		},
		Draw: func(ctx *render.Context, db render.DB, _ string) error {
			d, ok := db.(*DB)
			if !ok {
				return fmt.Errorf("block: unexpected db type %T", db)
			}
			return draw(ctx, d)
		},
		Classes: func(_ *render.Context, db render.DB, _ string) *js.Map[string, *render.ClassDef] {
			d, ok := db.(*DB)
			if !ok {
				return nil
			}
			return getClasses(d)
		},
	})
}
