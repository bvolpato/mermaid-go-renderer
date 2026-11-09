// Ported from mermaid 12.2.0, the semantic actions of
// diagrams/sankey/parser/sankey.jison (MIT).

package sankey

import (
	"strings"

	grammar "github.com/bvolpato/mermaid-go-renderer/internal/grammar/sankey"
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
	case 7:
		// const source = yy.findOrCreateNode($1.trim().replaceAll('""', '"'));
		// const target = yy.findOrCreateNode($3.trim().replaceAll('""', '"'));
		// const value = parseFloat($5.trim());
		// yy.addLink(source,target,value);
		source := db.FindOrCreateNode(strings.ReplaceAll(js.TrimSpace(s.S(1)), `""`, `"`))
		target := db.FindOrCreateNode(strings.ReplaceAll(js.TrimSpace(s.S(3)), `""`, `"`))
		value := js.ParseFloat(js.TrimSpace(s.S(5)))
		db.AddLink(source, target, value)
	case 8, 9, 11: // $$=$1;
		s.Result = s.V(1)
	case 10: // $$=$2;
		s.Result = s.V(2)
	}
	return nil, nil
}
