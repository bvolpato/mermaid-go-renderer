// Ported from mermaid 12.2.0, the semantic actions of
// diagrams/user-journey/parser/journey.jison (MIT).

package journey

import (
	grammar "github.com/bvolpato/mermaid-go-renderer/internal/grammar/journey"
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
	case 1: // return $2;
		if v := s.V(2); v != nil {
			return v, nil
		}
		return []any{}, nil
	case 2: // $$ = []
		s.Result = []any{}
	case 3: // $1.push($2);$$ = $1
		list, _ := s.V(1).([]any)
		s.Result = append(list, s.V(2))
	case 4: // $$ = $2
		s.Result = s.V(2)
	case 5: // $$ = $1
		s.Result = s.V(1)
	case 6, 7: // $$=[];
		s.Result = []any{}
	case 8: // yy.setDiagramTitle($1.substr(6));$$=$1.substr(6);
		db.SetDiagramTitle(js.StringSliceFrom(s.S(1), 6))
		s.Result = js.StringSliceFrom(s.S(1), 6)
	case 9: // $$=$2.trim();yy.setAccTitle($$);
		v := js.TrimSpace(s.S(2))
		s.Result = v
		db.SetAccTitle(v)
	case 10: // $$=$2.trim();yy.setAccDescription($$);
		v := js.TrimSpace(s.S(2))
		s.Result = v
		db.SetAccDescription(v)
	case 11: // $$=$1.trim();yy.setAccDescription($$);
		v := js.TrimSpace(s.S(1))
		s.Result = v
		db.SetAccDescription(v)
	case 12: // yy.addSection($1.substr(8));$$=$1.substr(8);
		db.AddSection(js.StringSliceFrom(s.S(1), 8))
		s.Result = js.StringSliceFrom(s.S(1), 8)
	case 13: // yy.addTask($1, $2);$$='task';
		db.AddTask(s.S(1), s.S(2))
		s.Result = "task"
	}
	return nil, nil
}
