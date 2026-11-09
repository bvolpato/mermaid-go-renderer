// Ported from mermaid 12.2.0, the semantic actions of
// diagrams/venn/parser/venn.jison (MIT).

package venn

import (
	"errors"
	"strings"

	grammar "github.com/bvolpato/mermaid-go-renderer/internal/grammar/venn"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// parseText runs the generated parser on text with db as parser.yy. The
// lexer reads and writes the indent mode of the db (grammar.LexYY).
func parseText(db *DB, text string) error {
	_, err := grammar.Parse(text, db, performAction)
	return err
}

var _ grammar.LexYY = (*DB)(nil)

// stringList is the value of an identifierList or of valueTokens.
func stringList(v any) []string {
	list, _ := v.([]string)
	return list
}

// unionChecks runs the start of the actions of the union statements.
func unionChecks(db *DB, identifierList []string) error {
	if len(identifierList) < 2 {
		return errors.New("union requires multiple identifiers")
	}
	return db.ValidateUnionIdentifiers(identifierList)
}

// indentedText runs the start of the actions of indentedTextTail.
func indentedText(db *DB) ([]string, error) {
	cs, ok := db.GetCurrentSets()
	if !ok {
		return nil, errors.New("text requires set")
	}
	return cs, nil
}

// performAction is parser.performAction of the generated parser.
func performAction(yy any, s *jison.ParserState, production int) (any, error) {
	db := yy.(*DB)
	str := func(i int) *string {
		v := s.S(i)
		return &v
	}
	num := func(i int) *float64 {
		v := js.ParseFloat(s.S(i))
		return &v
	}
	switch production {
	case 1: // return $3;
		if v := s.V(3); v != nil {
			return v, nil
		}
		return []any{}, nil
	case 2, 3, 4, 6: // $$ = []
		s.Result = []any{}
	case 5: // $1.push($2); $$ = $1
		list, _ := s.V(1).([]any)
		s.Result = append(list, s.V(2))
	case 7: // $$ = $1;
		s.Result = s.V(1)
	case 8: // yy.setDiagramTitle($1.substr(6)); $$ = $1.substr(6);
		db.SetDiagramTitle(js.StringSliceFrom(s.S(1), 6))
		s.Result = js.StringSliceFrom(s.S(1), 6)
	case 9: // yy.addSubsetData([$2], undefined, undefined); if (yy.setIndentMode) { yy.setIndentMode(true); }
		db.AddSubsetData([]string{s.S(2)}, nil, nil)
		db.SetIndentMode(true)
	case 10: // yy.addSubsetData([$2], $3, undefined); ...
		db.AddSubsetData([]string{s.S(2)}, str(3), nil)
		db.SetIndentMode(true)
	case 11: // yy.addSubsetData([$2], undefined, parseFloat($4)); ...
		db.AddSubsetData([]string{s.S(2)}, nil, num(4))
		db.SetIndentMode(true)
	case 12: // yy.addSubsetData([$2], $3, parseFloat($5)); ...
		db.AddSubsetData([]string{s.S(2)}, str(3), num(5))
		db.SetIndentMode(true)
	case 13: // if ($2.length < 2) { throw ... } if (yy.validateUnionIdentifiers) { ... } yy.addSubsetData($2, undefined, undefined); ...
		list := stringList(s.V(2))
		if err := unionChecks(db, list); err != nil {
			return nil, err
		}
		db.AddSubsetData(list, nil, nil)
		db.SetIndentMode(true)
	case 14: // ... yy.addSubsetData($2, $3, undefined); ...
		list := stringList(s.V(2))
		if err := unionChecks(db, list); err != nil {
			return nil, err
		}
		db.AddSubsetData(list, str(3), nil)
		db.SetIndentMode(true)
	case 15: // ... yy.addSubsetData($2, undefined, parseFloat($4)); ...
		list := stringList(s.V(2))
		if err := unionChecks(db, list); err != nil {
			return nil, err
		}
		db.AddSubsetData(list, nil, num(4))
		db.SetIndentMode(true)
	case 16: // ... yy.addSubsetData($2, $3, parseFloat($5)); ...
		list := stringList(s.V(2))
		if err := unionChecks(db, list); err != nil {
			return nil, err
		}
		db.AddSubsetData(list, str(3), num(5))
		db.SetIndentMode(true)
	case 17, 18, 19: // yy.addTextData($2, $3, undefined);
		db.AddTextData(stringList(s.V(2)), s.S(3), nil)
	case 20, 21: // yy.addTextData($2, $3, $4);
		db.AddTextData(stringList(s.V(2)), s.S(3), str(4))
	case 22: // $$ = $2;
		s.Result = s.V(2)
	case 23: // yy.addStyleData($2, $3);
		data, _ := s.V(3).([][2]string)
		db.AddStyleData(stringList(s.V(2)), data)
	case 24, 25, 26: // var cs = yy.getCurrentSets(); if (!cs) throw new Error('text requires set'); yy.addTextData(cs, $1, undefined);
		cs, err := indentedText(db)
		if err != nil {
			return nil, err
		}
		db.AddTextData(cs, s.S(1), nil)
	case 27, 28: // ... yy.addTextData(cs, $1, $2);
		cs, err := indentedText(db)
		if err != nil {
			return nil, err
		}
		db.AddTextData(cs, s.S(1), str(2))
	case 29: // $$ = [$1]
		field, _ := s.V(1).([2]string)
		s.Result = [][2]string{field}
	case 30: // $$ = [...$1, $3]
		list, _ := s.V(1).([][2]string)
		field, _ := s.V(3).([2]string)
		s.Result = append(append([][2]string{}, list...), field)
	case 31: // $$ = [$1, $3]
		s.Result = [2]string{s.S(1), s.S(3)}
	case 32: // $$ = $1;
		s.Result = s.V(1)
	case 33: // $$ = $1.join(' ');
		s.Result = strings.Join(stringList(s.V(1)), " ")
	case 34: // $$ = [$1];
		s.Result = []string{s.S(1)}
	case 35: // $1.push($2); $$ = $1;
		s.Result = append(stringList(s.V(1)), s.S(2))
	case 36, 37, 38, 39, 40: // $$ = $1;
		s.Result = s.V(1)
	case 41: // $$ = [$1]
		s.Result = []string{s.S(1)}
	case 42: // $$ = [...$1, $3]
		s.Result = append(append([]string{}, stringList(s.V(1))...), s.S(3))
	case 43, 44: // $$ = $1
		s.Result = s.V(1)
	}
	return nil, nil
}
