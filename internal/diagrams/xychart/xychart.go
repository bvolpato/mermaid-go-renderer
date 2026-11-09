// Package xychart is the XY chart diagram: bar plots and line plots on a
// band axis or a linear axis, in vertical or horizontal orientation.
//
// Ported from mermaid 12.2.0, diagrams/xychart/xychartDiagram.ts,
// xychartDb.ts, xychartRenderer.ts, parser/xychart.jison and the chartBuilder
// directory (MIT).
package xychart

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(definition())
}

// definition is the diagram definition of xychartDiagram.ts.
func definition() *render.Definition {
	return &render.Definition{
		ID:  "xychart",
		New: func(ctx *render.Context) render.DB { return newDB(ctx) },
		Parse: func(_ *render.Context, d render.DB, text string) error {
			xy := d.(*db)
			if xy.initErr != nil {
				return xy.initErr
			}
			return parse(xy, text)
		},
		Draw: func(ctx *render.Context, d render.DB, _ string) error {
			return draw(ctx, d.(*db))
		},
	}
}
