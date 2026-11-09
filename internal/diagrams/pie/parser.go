// Ported from mermaid 12.2.0, diagrams/pie/pieParser.ts (MIT).

package pie

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// parseDiagram is parser.parse of pieParser.ts.
func parseDiagram(_ *render.Context, rdb render.DB, text string) error {
	ast, err := parse(text)
	if err != nil {
		return err
	}
	return populateDb(ast, rdb.(*DB))
}

// populateDb is populateDb of pieParser.ts.
func populateDb(ast *Pie, db *DB) error {
	ast.Populate(db)
	db.SetShowData(ast.ShowData)
	for _, section := range ast.Sections {
		if err := db.AddSection(D3Section{Label: section.Label, Value: section.Value}); err != nil {
			return err
		}
	}
	return nil
}
