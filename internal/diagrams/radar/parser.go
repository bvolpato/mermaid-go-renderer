// Ported from mermaid 12.2.0, diagrams/radar/parser.ts (MIT).

package radar

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// parseDiagram is parser.parse of parser.ts.
func parseDiagram(_ *render.Context, rdb render.DB, text string) error {
	ast, err := parse(text)
	if err != nil {
		return err
	}
	return populate(ast, rdb.(*DB))
}

// populate is populate(ast) of parser.ts.
func populate(ast *Radar, db *DB) error {
	ast.Populate(db)
	// Here we can add specific logic between the AST and the DB
	db.SetAxes(ast.Axes)
	if err := db.SetCurves(ast.Curves); err != nil {
		return err
	}
	db.SetOptions(ast.Options)
	return nil
}
