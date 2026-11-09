// Package journey is the user journey diagram.
//
// Ported from mermaid 12.2.0, diagrams/user-journey/journeyDiagram.ts (MIT).
// The other files of the package port journeyDb.js, the semantic actions of
// parser/journey.jison, svgDraw.js and journeyRenderer.ts.
package journey

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:    "journey",
		New:   func(ctx *render.Context) render.DB { return NewDB(ctx) },
		Init:  initDiagram,
		Parse: parse,
		Draw:  draw,
	})
}

// NewDB returns an empty journey db. The common db sanitizes its texts with
// the sanitizer and the config of ctx at the time of the call, as
// commonDb.ts does with getConfig().
func NewDB(ctx *render.Context) *DB {
	db := &DB{}
	db.Sanitize = ctx.SanitizeText
	return db
}

// rendererState is the renderer of one render, or the error that upstream
// throws when the module of the renderer loads.
type rendererState struct {
	renderer *renderer
	err      error
}

const rendererKey = "journey.renderer"

// stateOf returns the renderer state of ctx. The first call stands in for
// the load of the journeyRenderer.ts module, which reads the config.
func stateOf(ctx *render.Context) *rendererState {
	return ctx.Value(rendererKey, func() any {
		r, err := newRenderer(ctx)
		return &rendererState{renderer: r, err: err}
	}).(*rendererState)
}

// initDiagram is the init function of the diagram definition:
// renderer.setConf(cnf.journey) and db.clear().
func initDiagram(ctx *render.Context, db render.DB) {
	state := stateOf(ctx)
	if state.err == nil {
		state.renderer.setConf(ctx.Config.Get("journey"))
	}
	if d, ok := db.(*DB); ok {
		d.Clear()
	}
}

func parse(ctx *render.Context, db render.DB, text string) error {
	d, ok := db.(*DB)
	if !ok {
		return fmt.Errorf("journey: unexpected db type %T", db)
	}
	// An error of the module load or of init stops upstream before the parse.
	if state := stateOf(ctx); state.err != nil {
		return state.err
	}
	return parseText(d, text)
}

func draw(ctx *render.Context, db render.DB, _ string) error {
	d, ok := db.(*DB)
	if !ok {
		return fmt.Errorf("journey: unexpected db type %T", db)
	}
	state := stateOf(ctx)
	if state.err != nil {
		return state.err
	}
	return state.renderer.draw(d)
}
