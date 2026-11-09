// Package sequence is the sequence diagram.
//
// Ported from mermaid 12.2.0, diagrams/sequence/sequenceDiagram.ts,
// sequenceDb.ts, sequenceRenderer.ts, svgDraw.js, actorBands.ts and the
// semantic actions of parser/sequenceDiagram.jison (MIT).
package sequence

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:  "sequence",
		New: func(ctx *render.Context) render.DB { return newDB(ctx) },
		Init: func(ctx *render.Context, _ render.DB) {
			initDiagram(ctx)
		},
		Parse: func(_ *render.Context, d render.DB, text string) error {
			sdb, ok := d.(*db)
			if !ok {
				return fmt.Errorf("sequence: unexpected db type %T", d)
			}
			return parse(sdb, text)
		},
		Draw: func(ctx *render.Context, d render.DB, _ string) error {
			sdb, ok := d.(*db)
			if !ok {
				return fmt.Errorf("sequence: unexpected db type %T", d)
			}
			return draw(ctx, sdb)
		},
	})
}

// initDiagram is the init(cnf) hook of the diagram definition: a truthy
// global wrap setting becomes the wrap setting of the sequence diagram.
func initDiagram(ctx *render.Context) {
	wrap := ctx.Cfg("wrap")
	if !js.Truthy(wrap) {
		return
	}
	seq := js.NewObj[any]()
	seq.Set("wrap", wrap)
	conf := js.NewObj[any]()
	conf.Set("sequence", seq)
	if _, err := ctx.ConfigState.SetConfig(conf); err == nil {
		// The db reads the config during the parse, before the pipeline
		// reads it again.
		ctx.Config = ctx.ConfigState.GetConfig()
	}
}
