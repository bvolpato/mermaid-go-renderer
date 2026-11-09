// Ported from mermaid 12.2.0, diagrams/architecture/architectureParser.ts
// (MIT).

package architecture

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// populateDb is populateDb(ast, db).
func populateDb(ast *Architecture, db *ArchitectureDB) error {
	ast.Populate(db)
	for _, group := range ast.Groups {
		if err := db.AddGroup(group.ID, group.Icon, group.In, group.Title); err != nil {
			return err
		}
	}
	for _, service := range ast.Services {
		if err := db.AddService(service.ID, service.Icon, service.In, service.Title, service.IconText); err != nil {
			return err
		}
	}
	for _, service := range ast.Junctions {
		if err := db.AddJunction(service.ID, service.In); err != nil {
			return err
		}
	}
	for _, edge := range ast.Edges {
		err := db.AddEdge(ArchitectureEdge{
			LhsID:    edge.LhsID,
			LhsDir:   edge.LhsDir,
			LhsInto:  edge.LhsInto,
			LhsGroup: edge.LhsGroup,
			RhsID:    edge.RhsID,
			RhsDir:   edge.RhsDir,
			RhsInto:  edge.RhsInto,
			RhsGroup: edge.RhsGroup,
			Title:    edge.Title,
		})
		if err != nil {
			return err
		}
	}
	for _, alignment := range ast.Alignments {
		members := append([]string{}, alignment.Members...)
		if err := db.AddLayoutHint(&ArchitectureLayoutHint{Direction: alignment.Direction, Members: members}); err != nil {
			return err
		}
	}
	return nil
}

// parse is parser.parse(input): it parses the text and fills the db.
func parse(_ *render.Context, rdb render.DB, input string) error {
	ast, err := parseAST(input)
	if err != nil {
		return err
	}
	db, ok := rdb.(*ArchitectureDB)
	if !ok {
		return errNotArchitectureDB
	}
	return populateDb(ast, db)
}
