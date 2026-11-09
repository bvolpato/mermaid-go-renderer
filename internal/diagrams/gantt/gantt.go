// Package gantt is the gantt diagram.
//
// Ported from mermaid 12.2.0, diagrams/gantt/ganttDiagram.ts, ganttDb.js,
// ganttRenderer.js and the semantic actions of
// diagrams/gantt/parser/gantt.jison (MIT).
package gantt

import (
	grammar "github.com/bvolpato/mermaid-go-renderer/internal/grammar/gantt"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:    "gantt",
		New:   func(ctx *render.Context) render.DB { return NewDB(ctx) },
		Parse: parse,
		Draw:  draw,
	})
}

func parse(_ *render.Context, db render.DB, text string) error {
	return Parse(db.(*DB), text)
}

// Parse fills the db from the text of a gantt diagram: parser.parse(text)
// with the db as yy.
func Parse(db *DB, text string) error {
	_, err := grammar.Parse(text, db, performAction)
	return err
}

// performAction is the port of the semantic actions of gantt.jison. The
// values of the productions ($$) have no reader: mermaid ignores the result
// of the parse.
func performAction(yy any, s *jison.ParserState, production int) (any, error) {
	db := yy.(*DB)
	substr := func(start int) string { return js.StringSliceFrom(s.S(1), start) }
	switch production {
	case 1: // return $2;
		return s.V(2), nil
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
	case 8:
		db.SetWeekday("monday")
	case 9:
		db.SetWeekday("tuesday")
	case 10:
		db.SetWeekday("wednesday")
	case 11:
		db.SetWeekday("thursday")
	case 12:
		db.SetWeekday("friday")
	case 13:
		db.SetWeekday("saturday")
	case 14:
		db.SetWeekday("sunday")
	case 15:
		db.SetWeekend("friday")
	case 16:
		db.SetWeekend("saturday")
	case 17: // yy.setDateFormat($1.substr(11));$$=$1.substr(11);
		db.SetDateFormat(substr(11))
		s.Result = substr(11)
	case 18: // yy.enableInclusiveEndDates();$$=$1.substr(18);
		db.EnableInclusiveEndDates()
		s.Result = substr(18)
	case 19: // yy.TopAxis();$$=$1.substr(8);
		// The db has enableTopAxis and no TopAxis: the statement throws.
		return nil, &typeError{"yy.TopAxis is not a function"}
	case 20: // yy.setAxisFormat($1.substr(11));$$=$1.substr(11);
		db.SetAxisFormat(substr(11))
		s.Result = substr(11)
	case 21: // yy.setTickInterval($1.substr(13));$$=$1.substr(13);
		db.SetTickInterval(substr(13))
		s.Result = substr(13)
	case 22: // yy.setExcludes($1.substr(9));$$=$1.substr(9);
		db.SetExcludes(substr(9))
		s.Result = substr(9)
	case 23: // yy.setIncludes($1.substr(9));$$=$1.substr(9);
		db.SetIncludes(substr(9))
		s.Result = substr(9)
	case 24: // yy.setTodayMarker($1.substr(12));$$=$1.substr(12);
		db.SetTodayMarker(substr(12))
		s.Result = substr(12)
	case 27: // yy.setDiagramTitle($1.substr(6));$$=$1.substr(6);
		db.SetDiagramTitle(substr(6))
		s.Result = substr(6)
	case 28: // $$=$2.trim();yy.setAccTitle($$);
		v := js.TrimSpace(s.S(2))
		s.Result = v
		db.SetAccTitle(v)
	case 29: // $$=$2.trim();yy.setAccDescription($$);
		v := js.TrimSpace(s.S(2))
		s.Result = v
		db.SetAccDescription(v)
	case 30: // $$=$1.trim();yy.setAccDescription($$);
		v := js.TrimSpace(s.S(1))
		s.Result = v
		db.SetAccDescription(v)
	case 31: // yy.addSection($1.substr(8));$$=$1.substr(8);
		db.AddSection(substr(8))
		s.Result = substr(8)
	case 33: // yy.addTask($1,$2);$$='task';
		if err := db.AddTask(s.S(1), s.S(2)); err != nil {
			return nil, err
		}
		s.Result = "task"
	case 34: // $$ = $1;yy.setClickEvent($1, $2, null);
		s.Result = s.V(1)
		db.SetClickEvent(s.S(1))
	case 35: // $$ = $1;yy.setClickEvent($1, $2, $3);
		s.Result = s.V(1)
		db.SetClickEvent(s.S(1))
	case 36: // $$ = $1;yy.setClickEvent($1, $2, null);yy.setLink($1,$3);
		s.Result = s.V(1)
		db.SetClickEvent(s.S(1))
		db.SetLink(s.S(1), s.S(3))
	case 37: // $$ = $1;yy.setClickEvent($1, $2, $3);yy.setLink($1,$4);
		s.Result = s.V(1)
		db.SetClickEvent(s.S(1))
		db.SetLink(s.S(1), s.S(4))
	case 38: // $$ = $1;yy.setClickEvent($1, $3, null);yy.setLink($1,$2);
		s.Result = s.V(1)
		db.SetClickEvent(s.S(1))
		db.SetLink(s.S(1), s.S(2))
	case 39: // $$ = $1;yy.setClickEvent($1, $3, $4);yy.setLink($1,$2);
		s.Result = s.V(1)
		db.SetClickEvent(s.S(1))
		db.SetLink(s.S(1), s.S(2))
	case 40: // $$ = $1;yy.setLink($1, $2);
		s.Result = s.V(1)
		db.SetLink(s.S(1), s.S(2))
	}
	return nil, nil
}
