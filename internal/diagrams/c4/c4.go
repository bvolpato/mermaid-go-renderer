// Package c4 is the C4 diagram: C4Context, C4Container, C4Component,
// C4Dynamic and C4Deployment.
//
// Ported from mermaid 12.2.0, diagrams/c4/c4Diagram.ts (MIT). The other files
// of the package port c4Db.ts, the semantic actions of parser/c4Diagram.jison,
// c4Renderer.ts, c4ShapeAdapter.ts and svgDraw.ts.
//
// The renderer of mermaid 12.2.0 lays the elements out on the legacy grid
// and draws each element with a shape of the unified renderer (person,
// rounded, cylinder, h-cyl or fr-rect) and the stacked C4 label. The shapes
// come from internal/ru. label_helper.go has the C4 label helper, which the
// label helper of internal/ru calls for a node that has a stereotype.
package c4

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	// The shapes that the C4 adapter uses register themselves: person,
	// rounded, cylinder and fr-rect (set1), and h-cyl (set2).
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set1"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set2"
)

func init() {
	// The label helper of the unified renderer calls the C4 label helper
	// for a node that has a stereotype.
	ru.C4LabelHelper = c4LabelHelper
	render.Register(&render.Definition{
		ID:    "c4",
		New:   func(ctx *render.Context) render.DB { return newDB(ctx) },
		Init:  initDiagram,
		Parse: parse,
		Draw:  draw,
	})
}

// initDiagram is the init hook of the diagram definition:
//
//	init: ({ c4, wrap }) => { renderer.setConf(c4); db.setWrap(wrap); }
func initDiagram(ctx *render.Context, d render.DB) {
	database := d.(*db)
	c4, _ := js.Clone(ctx.Cfg("c4")).(*js.Obj[any])
	database.rs.setConf(c4)
	database.setWrap(ctx.Cfg("wrap"))
}
