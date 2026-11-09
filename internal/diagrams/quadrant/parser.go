// Ported from mermaid 12.2.0, the semantic actions of
// diagrams/quadrant-chart/parser/quadrant.jison (MIT). The tables and the
// lexer are in internal/grammar/quadrant.

package quadrant

import (
	"errors"

	grammar "github.com/bvolpato/mermaid-go-renderer/internal/grammar/quadrant"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// lexTextObj is LexTextObj: the value of the nonterminal "text". It is a
// pointer, because one action changes the text of the object in place.
type lexTextObj struct {
	text string
	typ  string // "text" or "markdown"
}

// parseDiagram is parser.parse of the generated parser.
func parseDiagram(_ *render.Context, rdb render.DB, text string) error {
	_, err := grammar.Parse(text, rdb.(*DB), performAction)
	return err
}

// styles is the value of the nonterminal stylesOpt: a list of strings.
func styles(v any) []string {
	list, _ := v.([]string)
	return list
}

// performAction is parser.performAction of the generated parser.
func performAction(yy any, s *jison.ParserState, production int) (any, error) {
	db := yy.(*DB)
	textObj := func(i int) *lexTextObj {
		t, _ := s.V(i).(*lexTextObj)
		if t == nil {
			t = &lexTextObj{}
		}
		return t
	}
	switch production {
	case 23: // $$=$1
		s.Result = s.V(1)
	case 24: // $$=$1+''+$2
		s.Result = js.Concat(s.S(1), s.S(2))
	case 26: // $$ = $1 + $2;
		s.Result = js.Concat(s.S(1), s.S(2))
	case 27: // $$ = [$1.trim()]
		s.Result = []string{js.TrimSpace(s.S(1))}
	case 28: // $1.push($3.trim());$$ = $1;
		s.Result = append(styles(s.V(1)), js.TrimSpace(s.S(3)))
	case 29: // $$ = $1;yy.addClass($3,$5);
		s.Result = s.V(1)
		if err := db.AddClass(s.S(3), styles(s.V(5))); err != nil {
			return nil, err
		}
	case 37: // $$=[];
		s.Result = []string{}
	case 42: // $$=$2.trim();yy.setDiagramTitle($$);
		s.Result = js.TrimSpace(s.S(2))
		db.SetDiagramTitle(s.Result.(string))
	case 43: // $$=$2.trim();yy.setAccTitle($$);
		s.Result = js.TrimSpace(s.S(2))
		db.SetAccTitle(s.Result.(string))
	case 44: // $$=$2.trim();yy.setAccDescription($$);
		s.Result = js.TrimSpace(s.S(2))
		db.SetAccDescription(s.Result.(string))
	case 45: // $$=$1.trim();yy.setAccDescription($$);
		s.Result = js.TrimSpace(s.S(1))
		db.SetAccDescription(s.Result.(string))
	case 46: // yy.addSection($1.substr(8));$$=$1.substr(8);
		// The db has no addSection. The lexer has no rule for the token
		// "section", so the parser does not get here.
		return nil, errors.New("yy.addSection is not a function")
	case 47: // yy.addPoint($1, "", $3, $4, []);
		return nil, db.AddPoint(textObj(1), "", s.S(3), s.S(4), []string{})
	case 48: // yy.addPoint($1, $2, $4, $5, []);
		return nil, db.AddPoint(textObj(1), s.S(2), s.S(4), s.S(5), []string{})
	case 49: // yy.addPoint($1, "", $3, $4, $5);
		return nil, db.AddPoint(textObj(1), "", s.S(3), s.S(4), styles(s.V(5)))
	case 50: // yy.addPoint($1, $2, $4, $5, $6);
		return nil, db.AddPoint(textObj(1), s.S(2), s.S(4), s.S(5), styles(s.V(6)))
	case 51: // yy.setXAxisLeftText($2); yy.setXAxisRightText($4);
		db.SetXAxisLeftText(textObj(2))
		db.SetXAxisRightText(textObj(4))
	case 52: // $2.text += " ⟶ "; yy.setXAxisLeftText($2);
		t := textObj(2)
		t.text = js.Concat(t.text, " ⟶ ")
		db.SetXAxisLeftText(t)
	case 53: // yy.setXAxisLeftText($2);
		db.SetXAxisLeftText(textObj(2))
	case 54: // yy.setYAxisBottomText($2); yy.setYAxisTopText($4);
		db.SetYAxisBottomText(textObj(2))
		db.SetYAxisTopText(textObj(4))
	case 55: // $2.text += " ⟶ "; yy.setYAxisBottomText($2);
		t := textObj(2)
		t.text = js.Concat(t.text, " ⟶ ")
		db.SetYAxisBottomText(t)
	case 56: // yy.setYAxisBottomText($2);
		db.SetYAxisBottomText(textObj(2))
	case 57: // yy.setQuadrant1Text($2)
		db.SetQuadrant1Text(textObj(2))
	case 58: // yy.setQuadrant2Text($2)
		db.SetQuadrant2Text(textObj(2))
	case 59: // yy.setQuadrant3Text($2)
		db.SetQuadrant3Text(textObj(2))
	case 60: // yy.setQuadrant4Text($2)
		db.SetQuadrant4Text(textObj(2))
	case 64: // $$={text:$1, type: 'text'};
		s.Result = &lexTextObj{text: s.S(1), typ: "text"}
	case 65: // $$={text:$1.text+''+$2, type: $1.type};
		t := textObj(1)
		s.Result = &lexTextObj{text: js.Concat(t.text, s.S(2)), typ: t.typ}
	case 66: // $$={text: $1, type: 'text'};
		s.Result = &lexTextObj{text: s.S(1), typ: "text"}
	case 67: // $$={text: $1, type: 'markdown'};
		s.Result = &lexTextObj{text: s.S(1), typ: "markdown"}
	case 68: // $$=$1;
		s.Result = s.V(1)
	case 69: // $$=$1+''+$2;
		s.Result = js.Concat(s.S(1), s.S(2))
	}
	return nil, nil
}
