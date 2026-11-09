// Package requirement is the requirement diagram: the diagram type
// requirement (the keywords requirementDiagram and requirement).
//
// Ported from mermaid 12.2.0, diagrams/requirement/requirementDiagram.ts and
// requirementRenderer.ts (MIT). The other files of the package port
// requirementDb.ts, types.ts, the semantic actions of
// parser/requirementDiagram.jison and the shape
// rendering-util/rendering-elements/shapes/requirementBox.ts.
//
// The diagram is drawn by the unified renderer (internal/ru). The edges
// with their markers and the layouts register themselves there: a program
// must link those packages too.
package requirement

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	render.Register(&render.Definition{
		ID:  "requirement",
		New: func(ctx *render.Context) render.DB { return NewDB(ctx) },
		Parse: func(_ *render.Context, db render.DB, text string) error {
			d, ok := db.(*DB)
			if !ok {
				return fmt.Errorf("requirement: unexpected db type %T", db)
			}
			return parse(d, text)
		},
		Draw: func(ctx *render.Context, db render.DB, _ string) error {
			d, ok := db.(*DB)
			if !ok {
				return fmt.Errorf("requirement: unexpected db type %T", db)
			}
			return draw(ctx, d)
		},
		// The renderer of upstream has no getClasses function, so the
		// classDef statements make no CSS rules. They become inline styles
		// of the nodes.
	})
	ru.RegisterShape("requirementBox", requirementBox)
}

// typeError is a TypeError of JavaScript.
type typeError string

func (e typeError) Error() string { return string(e) }
