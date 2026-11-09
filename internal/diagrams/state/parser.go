// Ported from mermaid 12.2.0, the semantic actions of
// diagrams/state/parser/stateDiagram.jison (see
// internal/grammar/statediagram/actions.js.txt) (MIT).

package state

import (
	"fmt"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/statediagram"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// parse is parser.parse(text) with yy = db.
func parse(d *DB, text string) error {
	if d.initErr != nil {
		return d.initErr
	}
	_, err := statediagram.Parse(text, d, performAction)
	return err
}

// The semantic values of the grammar are:
//
//	string         the text of a token, the value of an accessibility
//	               statement, 'nl' for an empty line
//	[]any          a document: the values of its lines
//	*js.Obj[any]   a statement

func valueError(i int, want string) error {
	return fmt.Errorf("state parser: $%d is not %s", i, want)
}

// docValue returns $i, which is a document.
func docValue(s *jison.ParserState, i int) ([]any, error) {
	doc, ok := s.V(i).([]any)
	if !ok {
		return nil, valueError(i, "a document")
	}
	return doc, nil
}

// statement returns $i, which is a statement object.
func statement(s *jison.ParserState, i int) (*js.Obj[any], error) {
	stmt, ok := s.V(i).(*js.Obj[any])
	if !ok || stmt == nil {
		return nil, valueError(i, "a statement")
	}
	return stmt, nil
}

// performAction is the performAction function of the generated parser.
func performAction(yy any, s *jison.ParserState, production int) (any, error) {
	d := yy.(*DB)
	switch production {
	case 3: // start -> SD document
		// yy.setRootDoc($2); return $2;
		doc, err := docValue(s, 2)
		if err != nil {
			return nil, err
		}
		if err := d.SetRootDoc(doc); err != nil {
			return nil, err
		}
		return doc, nil

	case 4: // document -> /* empty */
		s.Result = []any{}

	case 5: // document -> document line
		// if($2 !='nl'){ $1.push($2); $$ = $1 }
		line := s.V(2)
		if text, ok := line.(string); !ok || text != "nl" {
			doc, err := docValue(s, 1)
			if err != nil {
				return nil, err
			}
			s.Result = append(doc, line)
		}

	case 6: // line -> SPACE statement
		s.Result = s.V(2)

	case 7: // line -> statement
		s.Result = s.V(1)

	case 8: // line -> NL
		s.Result = "nl"

	case 12: // statement -> idStatement
		s.Result = s.V(1)

	case 13: // statement -> idStatement DESCR
		stateStmt, err := statement(s, 1)
		if err != nil {
			return nil, err
		}
		stateStmt.Set("description", d.trimColon(s.S(2)))
		s.Result = stateStmt

	case 14: // statement -> idStatement --> idStatement
		s.Result = newObj("stmt", "relation", "state1", s.V(1), "state2", s.V(3))

	case 15: // statement -> idStatement --> idStatement DESCR
		relDescription := d.trimColon(s.S(4))
		s.Result = newObj("stmt", "relation", "state1", s.V(1), "state2", s.V(3), "description", relDescription)

	case 19: // statement -> COMPOSIT_STATE STRUCT_START document STRUCT_STOP
		s.Result = newObj("stmt", "state", "id", s.V(1), "type", "default", "description", "", "doc", s.V(3))

	case 20: // statement -> STATE_DESCR AS ID
		id := s.S(3)
		var description any = js.TrimSpace(s.S(1))
		// $3.match(':')
		if strings.Contains(s.S(3), ":") {
			parts := strings.Split(s.S(3), ":")
			id = parts[0]
			description = []any{description, parts[1]}
		}
		s.Result = newObj("stmt", "state", "id", id, "type", "default", "description", description)

	case 21: // statement -> STATE_DESCR AS ID STRUCT_START document STRUCT_STOP
		s.Result = newObj("stmt", "state", "id", s.V(3), "type", "default", "description", s.V(1), "doc", s.V(5))

	case 22: // statement -> FORK
		s.Result = newObj("stmt", "state", "id", s.V(1), "type", "fork")

	case 23: // statement -> JOIN
		s.Result = newObj("stmt", "state", "id", s.V(1), "type", "join")

	case 24: // statement -> CHOICE
		s.Result = newObj("stmt", "state", "id", s.V(1), "type", "choice")

	case 25: // statement -> CONCURRENT
		s.Result = newObj("stmt", "state", "id", d.getDividerID(), "type", "divider")

	case 26: // statement -> note notePosition ID NOTE_TEXT
		s.Result = newObj(
			"stmt", "state",
			"id", js.TrimSpace(s.S(3)),
			"note", newObj("position", js.TrimSpace(s.S(2)), "text", js.TrimSpace(s.S(4))),
		)

	case 29: // statement -> acc_title acc_title_value
		// $$=$2.trim();yy.setAccTitle($$);
		value := js.TrimSpace(s.S(2))
		s.Result = value
		// setAccTitle sanitizes the text, which can throw.
		if _, err := d.sanitizeText(value); err != nil {
			return nil, err
		}
		d.SetAccTitle(value)

	case 30: // statement -> acc_descr acc_descr_value
		// $$=$2.trim();yy.setAccDescription($$);
		value := js.TrimSpace(s.S(2))
		s.Result = value
		if _, err := d.sanitizeText(value); err != nil {
			return nil, err
		}
		d.SetAccDescription(value)

	case 31: // statement -> acc_descr_multiline_value
		// $$=$1.trim();yy.setAccDescription($$);
		value := js.TrimSpace(s.S(1))
		s.Result = value
		if _, err := d.sanitizeText(value); err != nil {
			return nil, err
		}
		d.SetAccDescription(value)

	case 32: // statement -> CLICK idStatement STRING STRING NL
		s.Result = newObj("stmt", "click", "id", s.V(2), "url", s.V(3), "tooltip", s.V(4))

	case 33: // statement -> CLICK idStatement HREF STRING NL
		s.Result = newObj("stmt", "click", "id", s.V(2), "url", s.V(4), "tooltip", "")

	case 34, 35: // classDefStatement -> classDef CLASSDEF_ID CLASSDEF_STYLEOPTS, classDef DEFAULT CLASSDEF_STYLEOPTS
		s.Result = newObj("stmt", "classDef", "id", js.TrimSpace(s.S(2)), "classes", js.TrimSpace(s.S(3)))

	case 36: // styleStatement -> style STYLE_IDS STYLEDEF_STYLEOPTS
		s.Result = newObj("stmt", "style", "id", js.TrimSpace(s.S(2)), "styleClass", js.TrimSpace(s.S(3)))

	case 37: // cssClassStatement -> class CLASSENTITY_IDS STYLECLASS
		s.Result = newObj("stmt", "applyClass", "id", js.TrimSpace(s.S(2)), "styleClass", js.TrimSpace(s.S(3)))

	case 38: // direction -> direction_tb
		d.SetDirection("TB")
		s.Result = newObj("stmt", "dir", "value", "TB")

	case 39: // direction -> direction_bt
		d.SetDirection("BT")
		s.Result = newObj("stmt", "dir", "value", "BT")

	case 40: // direction -> direction_rl
		d.SetDirection("RL")
		s.Result = newObj("stmt", "dir", "value", "RL")

	case 41: // direction -> direction_lr
		d.SetDirection("LR")
		s.Result = newObj("stmt", "dir", "value", "LR")

	case 44, 45: // idStatement -> ID, idStatement -> EDGE_STATE
		s.Result = newObj("stmt", "state", "id", js.TrimSpace(s.S(1)), "type", "default", "description", "")

	case 46, 47: // idStatement -> ID STYLE_SEPARATOR ID, idStatement -> EDGE_STATE STYLE_SEPARATOR ID
		s.Result = newObj(
			"stmt", "state",
			"id", js.TrimSpace(s.S(1)),
			"classes", []any{js.TrimSpace(s.S(3))},
			"type", "default",
			"description", "",
		)
	}
	return nil, nil
}
