// Ported from mermaid 12.2.0, the semantic actions of
// diagrams/ishikawa/parser/ishikawa.jison (MIT).

package ishikawa

import (
	grammar "github.com/bvolpato/mermaid-go-renderer/internal/grammar/ishikawa"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// parseText runs the generated parser on text with db as parser.yy.
func parseText(db *DB, text string) error {
	_, err := grammar.Parse(text, db, performAction)
	return err
}

// performAction is parser.performAction of the generated parser.
func performAction(yy any, s *jison.ParserState, production int) (any, error) {
	db := yy.(*DB)
	switch production {
	case 6, 7: // return yy;
		return db, nil
	case 15: // yy.addNode($1.length, $2.trim());
		db.AddNode(js.UTF16Len(s.S(1)), js.TrimSpace(s.S(2)))
	case 16: // yy.addNode(0, $1.trim());
		db.AddNode(0, js.TrimSpace(s.S(1)))
	}
	return nil, nil
}
