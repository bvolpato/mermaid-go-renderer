// Package railroad is the railroad diagrams: the diagram types railroad,
// railroadEbnf, railroadAbnf and railroadPeg. Each type has its own syntax
// for the rules of a grammar. The four types share the db, the renderer and
// the styles.
//
// Ported from mermaid 12.2.0, diagrams/railroad/railroadDiagram.ts,
// ebnfDiagram.ts, abnfDiagram.ts and pegDiagram.ts, and the four detectors
// (MIT). The other files of the package port railroadDb.ts, railroadTypes.ts,
// railroadRenderer.ts, the parsers in parser/, and the four Langium grammars
// of the parser package with their value converters.
package railroad

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	// The ids are the ids of railroadDetector.ts, ebnfDetector.ts,
	// abnfDetector.ts and pegDetector.ts.
	for id, parse := range map[string]func(db *DB, input string) error{
		"railroad":     parseRailroadText,
		"railroadEbnf": parseEbnfText,
		"railroadAbnf": parseAbnfText,
		"railroadPeg":  parsePegText,
	} {
		render.Register(&render.Definition{
			ID:  id,
			New: func(ctx *render.Context) render.DB { return NewDB(ctx) },
			Parse: func(_ *render.Context, db render.DB, text string) error {
				d, ok := db.(*DB)
				if !ok {
					return fmt.Errorf("railroad: unexpected db type %T", db)
				}
				return parse(d, text)
			},
			Draw: func(ctx *render.Context, db render.DB, _ string) error {
				d, ok := db.(*DB)
				if !ok {
					return fmt.Errorf("railroad: unexpected db type %T", db)
				}
				return draw(ctx, d)
			},
		})
	}
}
