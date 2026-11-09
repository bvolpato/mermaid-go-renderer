// Ported from mermaid 12.2.0, diagrams/xychart/parser/xychart.jison: the
// semantic actions (MIT). The tables and the lexer are in
// internal/grammar/xychart.

package xychart

import (
	"fmt"

	grammar "github.com/bvolpato/mermaid-go-renderer/internal/grammar/xychart"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// parse runs the generated parser on the text, with the db as yy.
func parse(d *db, text string) error {
	_, err := grammar.Parse(text, d, performAction)
	return err
}

// The values of the nonterminals:
//
//	text, and each element of commaSeparatedTexts and bandData: normalTextType
//	commaSeparatedTexts, bandData: []normalTextType
//	dataPoint: parsedDataPoint
//	dataPoints, plotData: []parsedDataPoint
//	alphaNum, alphaNumToken and the tokens: string

// valueError reports a semantic value of a type that the grammar cannot
// produce. It stands for the TypeError that JavaScript would throw.
func valueError(production, index int, v any) error {
	return &typeError{fmt.Sprintf("xychart parser: production %d: unexpected value $%d of type %T", production, index, v)}
}

// performAction is the port of parser.performAction.
func performAction(yy any, s *jison.ParserState, production int) (any, error) {
	d := yy.(*db)

	text := func(i int) (normalTextType, error) {
		t, ok := s.V(i).(normalTextType)
		if !ok {
			return t, valueError(production, i, s.V(i))
		}
		return t, nil
	}
	dataPoints := func(i int) ([]parsedDataPoint, error) {
		p, ok := s.V(i).([]parsedDataPoint)
		if !ok {
			return nil, valueError(production, i, s.V(i))
		}
		return p, nil
	}

	switch production {
	case 5: // yy.setOrientation($1);
		d.setOrientation(s.S(1))
	case 9: // yy.setDiagramTitle($2.text.trim());
		t, err := text(2)
		if err != nil {
			return nil, err
		}
		d.SetDiagramTitle(js.TrimSpace(t.text))
	case 12: // yy.setLineData({text: '', type: 'text'}, $2);
		data, err := dataPoints(2)
		if err != nil {
			return nil, err
		}
		d.setLineData(normalTextType{text: "", typ: "text"}, data)
	case 13: // yy.setLineData($2, $3);
		t, err := text(2)
		if err != nil {
			return nil, err
		}
		data, err := dataPoints(3)
		if err != nil {
			return nil, err
		}
		d.setLineData(t, data)
	case 14: // yy.setBarData({text: '', type: 'text'}, $2);
		data, err := dataPoints(2)
		if err != nil {
			return nil, err
		}
		d.setBarData(normalTextType{text: "", typ: "text"}, data)
	case 15: // yy.setBarData($2, $3);
		t, err := text(2)
		if err != nil {
			return nil, err
		}
		data, err := dataPoints(3)
		if err != nil {
			return nil, err
		}
		d.setBarData(t, data)
	case 16: // $$=$2.trim();yy.setAccTitle($$);
		v := js.TrimSpace(s.S(2))
		s.Result = v
		d.SetAccTitle(v)
	case 17: // $$=$2.trim();yy.setAccDescription($$);
		v := js.TrimSpace(s.S(2))
		s.Result = v
		d.SetAccDescription(v)
	case 18: // $$=$1.trim();yy.setAccDescription($$);
		v := js.TrimSpace(s.S(1))
		s.Result = v
		d.SetAccDescription(v)
	case 19: // $$ = $2
		s.Result = s.V(2)
	case 20: // $$ = [$1, ...$3]
		first, ok := s.V(1).(parsedDataPoint)
		if !ok {
			return nil, valueError(production, 1, s.V(1))
		}
		rest, err := dataPoints(3)
		if err != nil {
			return nil, err
		}
		list := make([]parsedDataPoint, 0, len(rest)+1)
		s.Result = append(append(list, first), rest...)
	case 21: // $$ = [$1]
		first, ok := s.V(1).(parsedDataPoint)
		if !ok {
			return nil, valueError(production, 1, s.V(1))
		}
		s.Result = []parsedDataPoint{first}
	case 22: // $$ = { value: Number($1), label: $2 }
		s.Result = parsedDataPoint{value: js.Number(s.S(1)), label: s.S(2)}
	case 23: // $$ = { value: Number($1), label: '' }
		s.Result = parsedDataPoint{value: js.Number(s.S(1)), label: ""}
	case 24, 25: // yy.setXAxisTitle($1);
		t, err := text(1)
		if err != nil {
			return nil, err
		}
		d.setXAxisTitle(t)
	case 26: // yy.setXAxisTitle({type: 'text', text: ''});
		d.setXAxisTitle(normalTextType{typ: "text", text: ""})
	case 27: // yy.setXAxisBand($1);
		categories, ok := s.V(1).([]normalTextType)
		if !ok {
			return nil, valueError(production, 1, s.V(1))
		}
		d.setXAxisBand(categories)
	case 28: // yy.setXAxisRangeData(Number($1), Number($3));
		d.setXAxisRangeData(js.Number(s.S(1)), js.Number(s.S(3)))
	case 29: // $$ = $2
		s.Result = s.V(2)
	case 30: // $$ = [$1, ...$3]
		first, err := text(1)
		if err != nil {
			return nil, err
		}
		rest, ok := s.V(3).([]normalTextType)
		if !ok {
			return nil, valueError(production, 3, s.V(3))
		}
		list := make([]normalTextType, 0, len(rest)+1)
		s.Result = append(append(list, first), rest...)
	case 31: // $$ = [$1]
		first, err := text(1)
		if err != nil {
			return nil, err
		}
		s.Result = []normalTextType{first}
	case 32, 33: // yy.setYAxisTitle($1);
		t, err := text(1)
		if err != nil {
			return nil, err
		}
		d.setYAxisTitle(t)
	case 34: // yy.setYAxisTitle({type: "text", text: ""});
		d.setYAxisTitle(normalTextType{typ: "text", text: ""})
	case 35: // yy.setYAxisRangeData(Number($1), Number($3));
		d.setYAxisRangeData(js.Number(s.S(1)), js.Number(s.S(3)))
	case 39: // $$={text:$1, type: 'text'};
		s.Result = normalTextType{text: s.S(1), typ: "text"}
	case 40: // $$={text: $1, type: 'text'};
		s.Result = normalTextType{text: s.S(1), typ: "text"}
	case 41: // $$={text: $1, type: 'markdown'};
		s.Result = normalTextType{text: s.S(1), typ: "markdown"}
	case 42: // $$=$1;
		s.Result = s.V(1)
	case 43: // $$=$1+''+$2;
		s.Result = js.Concat(s.S(1), "", s.S(2))
	}
	return nil, nil
}
