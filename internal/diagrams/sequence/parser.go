// Ported from mermaid 12.2.0, the semantic actions of
// diagrams/sequence/parser/sequenceDiagram.jison (see
// internal/grammar/sequencediagram/actions.js.txt) (MIT).

package sequence

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/sequencediagram"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// parse is parser.parse(text) with yy = db.
func parse(d *db, text string) error {
	_, err := sequencediagram.Parse(text, d, performAction)
	return err
}

// The semantic values of the grammar are:
//
//	string   the text of a token, and the value of the title statements
//	int      a line type or a placement
//	*param   an object
//	[]any    an array
//	*parsedMessage  the result of yy.parseMessage

// asParam returns $i as an object.
func asParam(s *jison.ParserState, i int) (*param, error) {
	p, ok := s.V(i).(*param)
	if !ok || p == nil {
		return nil, fmt.Errorf("sequence parser: $%d is not an object", i)
	}
	return p, nil
}

// asArray returns $i as an array.
func asArray(s *jison.ParserState, i int) ([]any, error) {
	a, ok := s.V(i).([]any)
	if !ok {
		return nil, fmt.Errorf("sequence parser: $%d is not an array", i)
	}
	return a, nil
}

// asMessage returns $i as the result of parseMessage.
func asMessage(s *jison.ParserState, i int) (*parsedMessage, error) {
	m, ok := s.V(i).(*parsedMessage)
	if !ok || m == nil {
		return nil, fmt.Errorf("sequence parser: $%d is not a message", i)
	}
	return m, nil
}

// asInt returns $i as a number.
func asInt(s *jison.ParserState, i int) (int, error) {
	n, ok := s.V(i).(int)
	if !ok {
		return 0, fmt.Errorf("sequence parser: $%d is not a number", i)
	}
	return n, nil
}

// signalTypes maps the productions of signaltype to the line type.
var signalTypes = map[int]int{
	80:  lineSolidOpen,
	81:  lineDottedOpen,
	82:  lineSolid,
	83:  lineSolidTop,
	84:  lineSolidBottom,
	85:  lineStickTop,
	86:  lineStickBottom,
	87:  lineSolidTopDotted,
	88:  lineSolidBottomDotted,
	89:  lineStickTopDotted,
	90:  lineStickBottomDotted,
	91:  lineSolidArrowTopReverse,
	92:  lineSolidArrowBottomReverse,
	93:  lineStickArrowTopReverse,
	94:  lineStickArrowBottomReverse,
	95:  lineSolidArrowTopReverseDotted,
	96:  lineSolidArrowBottomReverseDotted,
	97:  lineStickArrowTopReverseDotted,
	98:  lineStickArrowBottomReverseDotted,
	99:  lineBidirectionalSolid,
	100: lineDotted,
	101: lineBidirectionalDotted,
	102: lineSolidCross,
	103: lineDottedCross,
	104: lineSolidPoint,
	105: lineDottedPoint,
}

// block is the action of loop, rect, opt and break: the start object goes
// to the front of $3 and the end object to the back.
func block(d *db, s *jison.ParserState, startType string, startSignal int, endType string, endSignal int) error {
	doc, err := asArray(s, 3)
	if err != nil {
		return err
	}
	out := make([]any, 0, len(doc)+2)
	out = append(out, &param{typ: startType, text: d.parseMessage(s.S(2)), signalType: startSignal})
	out = append(out, doc...)
	out = append(out, &param{typ: endType, signalType: endSignal})
	s.Result = out
	return nil
}

// section is the action of the option, and and else sections:
// $$ = $1.concat([{...}, $4]).
func section(d *db, s *jison.ParserState, typ string, signal int) error {
	doc, err := asArray(s, 1)
	if err != nil {
		return err
	}
	out := make([]any, 0, len(doc)+2)
	out = append(out, doc...)
	out = append(out, &param{typ: typ, text: d.parseMessage(s.S(3)), signalType: signal})
	out = append(out, s.V(4))
	s.Result = out
	return nil
}

// participant is the action of the participant statements.
func participant(d *db, s *jison.ParserState, draw string, withDescription bool) error {
	p, err := asParam(s, 2)
	if err != nil {
		return err
	}
	p.draw, p.hasDraw = draw, true
	p.typ = "addParticipant"
	if withDescription {
		p.description = d.parseMessage(s.S(4))
	}
	s.Result = p
	return nil
}

// actorText is the action of the links, link, properties and details
// statements: $$ = [$2, {type, actor: $2.actor, text: $3}].
func actorText(s *jison.ParserState, typ string) error {
	p, err := asParam(s, 2)
	if err != nil {
		return err
	}
	text, err := asMessage(s, 3)
	if err != nil {
		return err
	}
	s.Result = []any{p, &param{typ: typ, actor: p.actor, text: text}}
	return nil
}

// performAction is parser.performAction.
func performAction(yy any, s *jison.ParserState, production int) (any, error) {
	d, ok := yy.(*db)
	if !ok {
		return nil, fmt.Errorf("sequence parser: yy is not the sequence db")
	}
	switch production {
	case 3: // yy.apply($2);return $2;
		if err := d.apply(s.V(2)); err != nil {
			return nil, err
		}
		if doc, ok := s.V(2).([]any); ok && doc != nil {
			return doc, nil
		}
		return []any{}, nil
	case 4, 8, 9, 10, 14: // $$ = []
		s.Result = []any{}
	case 5, 11: // $1.push($2);$$ = $1
		doc, err := asArray(s, 1)
		if err != nil {
			return nil, err
		}
		s.Result = append(doc, s.V(2))
	case 6, 12: // $$ = $2
		s.Result = s.V(2)
	case 7, 13: // $$ = $1
		s.Result = s.V(1)
	case 16: // $2.type='createParticipant'; $$=$2;
		p, err := asParam(s, 2)
		if err != nil {
			return nil, err
		}
		p.typ = "createParticipant"
		s.Result = p
	case 17:
		doc, err := asArray(s, 3)
		if err != nil {
			return nil, err
		}
		out := make([]any, 0, len(doc)+2)
		out = append(out, &param{typ: "boxStart", boxData: d.parseBoxData(s.S(2))})
		out = append(out, doc...)
		out = append(out, &param{typ: "boxEnd"})
		s.Result = out
	case 19:
		s.Result = &param{
			typ:           "sequenceIndex",
			sequenceIndex: js.Number(s.S(2)), hasSequenceIndex: true,
			sequenceIndexStep: js.Number(s.S(3)), hasSequenceIndexStep: true,
			sequenceVisible: true, signalType: lineAutonumber,
		}
	case 20:
		s.Result = &param{
			typ:           "sequenceIndex",
			sequenceIndex: js.Number(s.S(2)), hasSequenceIndex: true,
			sequenceIndexStep: 1, hasSequenceIndexStep: true,
			sequenceVisible: true, signalType: lineAutonumber,
		}
	case 21:
		s.Result = &param{typ: "sequenceIndex", sequenceVisible: false, signalType: lineAutonumber}
	case 22:
		s.Result = &param{typ: "sequenceIndex", sequenceVisible: true, signalType: lineAutonumber}
	case 23:
		p, err := asParam(s, 2)
		if err != nil {
			return nil, err
		}
		s.Result = &param{typ: "activeStart", signalType: lineActiveStart, actor: p.actor}
	case 24:
		p, err := asParam(s, 2)
		if err != nil {
			return nil, err
		}
		s.Result = &param{typ: "activeEnd", signalType: lineActiveEnd, actor: p.actor}
	case 30: // yy.setDiagramTitle($1.substring(6));$$=$1.substring(6);
		title := js.StringSliceFrom(s.S(1), 6)
		d.SetDiagramTitle(title)
		s.Result = title
	case 31: // yy.setDiagramTitle($1.substring(7));$$=$1.substring(7);
		title := js.StringSliceFrom(s.S(1), 7)
		d.SetDiagramTitle(title)
		s.Result = title
	case 32: // $$=$2.trim();yy.setAccTitle($$);
		v := js.TrimSpace(s.S(2))
		s.Result = v
		d.SetAccTitle(v)
	case 33: // $$=$2.trim();yy.setAccDescription($$);
		v := js.TrimSpace(s.S(2))
		s.Result = v
		d.SetAccDescription(v)
	case 34: // $$=$1.trim();yy.setAccDescription($$);
		v := js.TrimSpace(s.S(1))
		s.Result = v
		d.SetAccDescription(v)
	case 35:
		return nil, block(d, s, "loopStart", lineLoopStart, "loopEnd", lineLoopEnd)
	case 36:
		return nil, block(d, s, "rectStart", lineRectStart, "rectEnd", lineRectEnd)
	case 37:
		return nil, block(d, s, "optStart", lineOptStart, "optEnd", lineOptEnd)
	case 38:
		return nil, block(d, s, "altStart", lineAltStart, "altEnd", lineAltEnd)
	case 39:
		return nil, block(d, s, "parStart", lineParStart, "parEnd", lineParEnd)
	case 40:
		return nil, block(d, s, "parStart", lineParOverStart, "parEnd", lineParEnd)
	case 41:
		return nil, block(d, s, "criticalStart", lineCriticalStart, "criticalEnd", lineCriticalEnd)
	case 42:
		return nil, block(d, s, "breakStart", lineBreakStart, "breakEnd", lineBreakEnd)
	case 44:
		return nil, section(d, s, "option", lineCriticalOption)
	case 46:
		return nil, section(d, s, "and", lineParAnd)
	case 48:
		return nil, section(d, s, "else", lineAltElse)
	case 49, 54:
		return nil, participant(d, s, "participant", true)
	case 50, 55:
		return nil, participant(d, s, "participant", false)
	case 51, 56:
		return nil, participant(d, s, "actor", true)
	case 52, 57:
		return nil, participant(d, s, "actor", false)
	case 53: // $2.type='destroyParticipant'; $$=$2;
		p, err := asParam(s, 2)
		if err != nil {
			return nil, err
		}
		p.typ = "destroyParticipant"
		s.Result = p
	case 58: // $$ = [$3, {type:'addNote', placement:$2, actor:$3.actor, text:$4}];
		placement, err := asInt(s, 2)
		if err != nil {
			return nil, err
		}
		p, err := asParam(s, 3)
		if err != nil {
			return nil, err
		}
		text, err := asMessage(s, 4)
		if err != nil {
			return nil, err
		}
		s.Result = []any{p, &param{typ: "addNote", placement: placement, actor: p.actor, text: text}}
	case 59:
		// Coerce actor_pair into a [to, from, ...] array:
		// $2 = [].concat($3, $3).slice(0, 2), then the actor of each.
		text, err := asMessage(s, 4)
		if err != nil {
			return nil, err
		}
		var pair []*param
		switch v := s.V(3).(type) {
		case *param:
			pair = []*param{v, v}
		case []any:
			for _, e := range v {
				if p, ok := e.(*param); ok {
					pair = append(pair, p)
				}
			}
		}
		if len(pair) != 2 || pair[0] == nil || pair[1] == nil {
			return nil, fmt.Errorf("sequence parser: $3 is not an actor pair")
		}
		s.Result = []any{s.V(3), &param{
			typ:       "addNote",
			placement: placementOver,
			actors:    []string{pair[0].actor, pair[1].actor},
			text:      text,
		}}
	case 60:
		return nil, actorText(s, "addLinks")
	case 61:
		return nil, actorText(s, "addALink")
	case 62:
		return nil, actorText(s, "addProperties")
	case 63:
		return nil, actorText(s, "addDetails")
	case 66: // $$ = [$1, $3];
		s.Result = []any{s.V(1), s.V(3)}
	case 67: // $$ = $1;
		s.Result = s.V(1)
	case 68:
		s.Result = placementLeftOf
	case 69:
		s.Result = placementRightOf
	case 70, 71, 72, 73, 74, 75:
		return nil, signal(s, production)
	case 76:
		s.Result = &param{typ: "addParticipant", actor: s.S(1), config: s.S(2), hasConfig: true}
	case 77: // $$ = $2.trim();
		s.Result = js.TrimSpace(s.S(2))
	case 78, 79:
		s.Result = &param{typ: "addParticipant", actor: s.S(1)}
	case 106: // $$ = yy.parseMessage($1.trim().substring(1))
		s.Result = d.parseMessage(js.StringSliceFrom(js.TrimSpace(s.S(1)), 1))
	default:
		if t, ok := signalTypes[production]; ok {
			s.Result = t
		}
	}
	return nil, nil
}

// signal is the action of the signal productions.
func signal(s *jison.ParserState, production int) error {
	from, err := asParam(s, 1)
	if err != nil {
		return err
	}
	// The positions of the signal type, of the receiver and of the text.
	typeAt, toAt, textAt := 2, 4, 5
	switch production {
	case 73:
		typeAt = 3
	case 74:
		typeAt, toAt, textAt = 3, 5, 6
	case 75:
		toAt, textAt = 3, 4
	}
	signalType, err := asInt(s, typeAt)
	if err != nil {
		return err
	}
	to, err := asParam(s, toAt)
	if err != nil {
		return err
	}
	msg, err := asMessage(s, textAt)
	if err != nil {
		return err
	}
	m := &param{typ: "addMessage", from: from.actor, to: to.actor, signalType: signalType, msg: msg}
	switch production {
	case 70:
		m.activate = true
		s.Result = []any{from, to, m,
			&param{typ: "activeStart", signalType: lineActiveStart, actor: to.actor}}
	case 71:
		s.Result = []any{from, to, m,
			&param{typ: "activeEnd", signalType: lineActiveEnd, actor: from.actor}}
	case 72:
		m.activate = true
		m.centralConnection = lineCentralConnection
		s.Result = []any{from, to, m,
			&param{typ: "centralConnection", signalType: lineCentralConnection, actor: to.actor}}
	case 73:
		m.activate = false
		m.centralConnection = lineCentralConnectionReverse
		s.Result = []any{from, to, m,
			&param{typ: "centralConnectionReverse", signalType: lineCentralConnectionReverse, actor: from.actor}}
	case 74:
		m.activate = true
		m.centralConnection = lineCentralConnectionDual
		s.Result = []any{from, to, m,
			&param{typ: "centralConnection", signalType: lineCentralConnection, actor: to.actor},
			&param{typ: "centralConnectionReverse", signalType: lineCentralConnectionReverse, actor: from.actor}}
	default:
		s.Result = []any{from, to, m}
	}
	return nil
}
