// Package timeline is the timeline diagram.
//
// Ported from mermaid 12.2.0, diagrams/timeline/timeline-definition.ts (MIT).
// The other files of the package port timelineDb.js, the semantic actions of
// parser/timeline.jison, svgDraw.js, timelineRenderer.ts and
// timelineRendererVertical.ts.
package timeline

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:    "timeline",
		New:   func(ctx *render.Context) render.DB { return NewDB(ctx) },
		Parse: parse,
		Draw:  draw,
	})
}

// NewDB returns an empty timeline db. The common db sanitizes its texts
// with the sanitizer and the config of ctx at the time of the call, as
// commonDb.ts does with getConfig().
func NewDB(ctx *render.Context) *DB {
	db := newDB()
	db.common.Sanitize = ctx.SanitizeText
	return db
}

func parse(_ *render.Context, db render.DB, text string) error {
	d, ok := db.(*DB)
	if !ok {
		return fmt.Errorf("timeline: unexpected db type %T", db)
	}
	return parseText(d, text)
}

// draw is rendererSelector.draw: the direction of the db selects the
// renderer.
func draw(ctx *render.Context, db render.DB, _ string) error {
	d, ok := db.(*DB)
	if !ok {
		return fmt.Errorf("timeline: unexpected db type %T", db)
	}
	r := &renderer{ctx: ctx, conf: ctx.Config}
	if d.GetDirection() == "TD" {
		return r.drawVertical(d)
	}
	return r.draw(d)
}
