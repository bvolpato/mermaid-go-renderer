// Ported from mermaid 12.2.0, the semantic actions of
// diagrams/timeline/parser/timeline.jison (MIT).

package timeline

import (
	grammar "github.com/bvolpato/mermaid-go-renderer/internal/grammar/timeline"
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
	case 3: // yy.setDirection('LR');
		db.SetDirection("LR")
	case 4: // yy.setDirection('TD');
		db.SetDirection("TD")
	case 5: // $$ = []
		s.Result = []any{}
	case 6: // $1.push($2);$$ = $1
		list, _ := s.V(1).([]any)
		s.Result = append(list, s.V(2))
	case 7: // $$ = $2
		s.Result = s.V(2)
	case 8: // $$ = $1
		s.Result = s.V(1)
	case 9, 10: // $$=[];
		s.Result = []any{}
	case 11: // yy.getCommonDb().setDiagramTitle($1.substr(6));$$=$1.substr(6);
		db.GetCommonDb().SetDiagramTitle(js.StringSliceFrom(s.S(1), 6))
		s.Result = js.StringSliceFrom(s.S(1), 6)
	case 12: // $$=$2.trim();yy.getCommonDb().setAccTitle($$);
		v := js.TrimSpace(s.S(2))
		s.Result = v
		db.GetCommonDb().SetAccTitle(v)
	case 13: // $$=$2.trim();yy.getCommonDb().setAccDescription($$);
		v := js.TrimSpace(s.S(2))
		s.Result = v
		db.GetCommonDb().SetAccDescription(v)
	case 14: // $$=$1.trim();yy.getCommonDb().setAccDescription($$);
		v := js.TrimSpace(s.S(1))
		s.Result = v
		db.GetCommonDb().SetAccDescription(v)
	case 15: // yy.addSection($1.substr(8));$$=$1.substr(8);
		db.AddSection(js.StringSliceFrom(s.S(1), 8))
		s.Result = js.StringSliceFrom(s.S(1), 8)
	case 18: // yy.addTask($1,0,'');$$=$1;
		db.AddTask(s.S(1), 0, "")
		s.Result = s.V(1)
	case 19: // yy.addEvent($1.substr(2));$$=$1;
		if err := db.AddEvent(js.StringSliceFrom(s.S(1), 2)); err != nil {
			return nil, err
		}
		s.Result = s.V(1)
	}
	return nil, nil
}
