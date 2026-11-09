// Package git is the gitGraph diagram.
//
// Ported from mermaid 12.2.0, diagrams/git/gitGraphDiagram.ts, gitGraphAst.ts,
// gitGraphParser.ts, gitGraphRenderer.ts and gitGraphTypes.ts, and the grammar
// parser/src/language/gitGraph/gitGraph.langium (MIT).
package git

import (
	"errors"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:  "gitGraph",
		New: func(ctx *render.Context) render.DB { return newDB(ctx) },
		Parse: func(_ *render.Context, db render.DB, text string) error {
			d, ok := db.(*DB)
			if !ok {
				return errors.New("gitGraph: unexpected db type")
			}
			return parse(d, text)
		},
		Draw: func(ctx *render.Context, db render.DB, _ string) error {
			d, ok := db.(*DB)
			if !ok {
				return errors.New("gitGraph: unexpected db type")
			}
			return draw(ctx, d)
		},
	})
}
