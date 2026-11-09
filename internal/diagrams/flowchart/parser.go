// Ported from mermaid 12.2.0, diagrams/flowchart/parser/flowParser.ts and the
// semantic actions of diagrams/flowchart/parser/flow.jison (see
// internal/grammar/flow/actions.js.txt) (MIT).

package flowchart

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/flow"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// prepareText is the text preparation of flowParser.ts:
//
//	src.replace(/}\s*\n/g, '}\n')
//
// It removes the trailing whitespace after closing curly braces when ending
// a line break. The white space after a "}" is greedy, so the match ends at
// the last line break of the white space.
func prepareText(src string) string {
	if !strings.Contains(src, "}") {
		return src
	}
	var b strings.Builder
	b.Grow(len(src))
	for i := 0; i < len(src); {
		c := src[i]
		if c != '}' {
			b.WriteByte(c)
			i++
			continue
		}
		// The run of white space after the brace, and its last line break.
		lastBreak := -1
		j := i + 1
		for j < len(src) {
			r, size := utf8.DecodeRuneInString(src[j:])
			if !js.IsWhiteSpace(r) {
				break
			}
			if r == '\n' {
				lastBreak = j
			}
			j += size
		}
		b.WriteByte('}')
		if lastBreak >= 0 {
			b.WriteByte('\n')
			i = lastBreak + 1
		} else {
			i++
		}
	}
	return b.String()
}

// parse is parser.parse(text) of flowParser.ts with yy = db.
func parse(d *DB, text string) error {
	_, err := flow.Parse(prepareText(text), d, performAction)
	return err
}

// The semantic values of the grammar are:
//
//	string        the text of a token, an id, a style, the value of an
//	              accessibility statement, the id of a subgraph
//	[]string      a list of node ids, of numbers or of styles
//	[]any         a document: the values of its lines
//	*vertexStmt   a vertex statement: { stmt, nodes }
//	*FlowText     a text: { text, type }
//	*FlowLink     a link: { type, stroke, length, text, id }
//	*dirStmt      a direction statement: { stmt: 'dir', value }

func valueError(i int, want string) error {
	return fmt.Errorf("flowchart parser: $%d is not %s", i, want)
}

func asStrings(s *jison.ParserState, i int) ([]string, error) {
	v, ok := s.V(i).([]string)
	if !ok {
		return nil, valueError(i, "a list")
	}
	return v, nil
}

func asDocument(s *jison.ParserState, i int) ([]any, error) {
	v, ok := s.V(i).([]any)
	if !ok {
		return nil, valueError(i, "a document")
	}
	return v, nil
}

func asVertexStmt(s *jison.ParserState, i int) (*vertexStmt, error) {
	v, ok := s.V(i).(*vertexStmt)
	if !ok || v == nil {
		return nil, valueError(i, "a vertex statement")
	}
	return v, nil
}

func asText(s *jison.ParserState, i int) (*FlowText, error) {
	v, ok := s.V(i).(*FlowText)
	if !ok || v == nil {
		return nil, valueError(i, "a text")
	}
	return v, nil
}

func asLink(s *jison.ParserState, i int) (*FlowLink, error) {
	v, ok := s.V(i).(*FlowLink)
	if !ok || v == nil {
		return nil, valueError(i, "a link")
	}
	return v, nil
}

// vertexShapes maps the productions of vertex with one text to the shape
// and to the position of the text in the production.
var vertexShapes = map[int]struct {
	shape string
	text  int
}{
	56: {"square", 3},
	57: {"doublecircle", 3},
	58: {"circle", 4},
	59: {"ellipse", 3},
	60: {"stadium", 3},
	61: {"subroutine", 3},
	63: {"cylinder", 3},
	64: {"round", 3},
	65: {"diamond", 3},
	66: {"hexagon", 4},
	67: {"odd", 3},
	68: {"trapezoid", 3},
	69: {"inv_trapezoid", 3},
	70: {"lean_right", 3},
	71: {"lean_left", 3},
}

// directions maps the productions of direction to the value.
var directions = map[int]string{185: "TB", 186: "BT", 187: "RL", 188: "LR", 189: "TD"}

// last is a[a.length-1].
func last(a []string) string {
	if len(a) == 0 {
		return ""
	}
	return a[len(a)-1]
}

// concat is a.concat(b...): a new list.
func concat(a []string, b ...string) []string {
	out := make([]string, 0, len(a)+len(b))
	out = append(out, a...)
	return append(out, b...)
}

// linkOf is { type: inf.type, stroke: inf.stroke, length: inf.length }.
func linkOf(inf FlowLink) *FlowLink {
	return &FlowLink{Type: inf.Type, Stroke: inf.Stroke, Length: inf.Length}
}

// performAction is the port of the generated parser.performAction.
func performAction(yy any, s *jison.ParserState, production int) (any, error) {
	d, ok := yy.(*DB)
	if !ok {
		return nil, fmt.Errorf("flowchart parser: yy is not a flowchart db")
	}
	switch production {
	case 2: // $$ = [];
		s.Result = []any{}
	case 3:
		// if(!Array.isArray($2) || $2.length > 0){ $1.push($2); } $$=$1;
		doc, err := asDocument(s, 1)
		if err != nil {
			return nil, err
		}
		item := s.V(2)
		if list, isList := item.([]string); !isList || len(list) > 0 {
			doc = append(doc, item)
		}
		s.Result = doc
	case 4: // $$=$1;
		s.Result = s.V(1)
	case 11: // yy.setDirection('TB');$$ = 'TB';
		d.SetDirection("TB")
		s.Result = "TB"
	case 12: // yy.setDirection($2);$$ = $2;
		d.SetDirection(s.S(2))
		s.Result = s.S(2)
	case 27: // $$=$1.nodes
		stmt, err := asVertexStmt(s, 1)
		if err != nil {
			return nil, err
		}
		s.Result = stmt.nodes
	case 28, 29, 30, 31, 32: // $$=[];
		s.Result = []string{}
	case 33: // $$=yy.addSubGraph($3,$8,$5);
		id, err := asText(s, 3)
		if err != nil {
			return nil, err
		}
		list, err := asDocument(s, 8)
		if err != nil {
			return nil, err
		}
		title, err := asText(s, 5)
		if err != nil {
			return nil, err
		}
		result, err := d.AddSubGraph(id, list, title)
		if err != nil {
			return nil, err
		}
		s.Result = result
	case 34: // $$=yy.addSubGraph($3,$5,$3);
		id, err := asText(s, 3)
		if err != nil {
			return nil, err
		}
		list, err := asDocument(s, 5)
		if err != nil {
			return nil, err
		}
		result, err := d.AddSubGraph(id, list, id)
		if err != nil {
			return nil, err
		}
		s.Result = result
	case 35: // $$=yy.addSubGraph(undefined,$3,undefined);
		list, err := asDocument(s, 3)
		if err != nil {
			return nil, err
		}
		result, err := d.AddSubGraph(nil, list, nil)
		if err != nil {
			return nil, err
		}
		s.Result = result
	case 37: // $$=$2.trim();yy.setAccTitle($$);
		v := js.TrimSpace(s.S(2))
		s.Result = v
		d.SetAccTitle(v)
	case 38: // $$=$2.trim();yy.setAccDescription($$);
		v := js.TrimSpace(s.S(2))
		s.Result = v
		d.SetAccDescription(v)
	case 39: // $$=$1.trim();yy.setAccDescription($$);
		v := js.TrimSpace(s.S(1))
		s.Result = v
		d.SetAccDescription(v)
	case 43: // $$ = $1 + $2;
		s.Result = js.Concat(s.S(1), s.S(2))
	case 44: // $$ = $1;
		s.Result = s.V(1)
	case 45:
		// yy.addVertex($3[$3.length-1],undefined,undefined,undefined, undefined,undefined, undefined,$4);
		// yy.addLink($1.stmt,$3,$2); $$ = { stmt: $3, nodes: $3.concat($1.nodes) }
		stmt, err := asVertexStmt(s, 1)
		if err != nil {
			return nil, err
		}
		link, err := asLink(s, 2)
		if err != nil {
			return nil, err
		}
		node, err := asStrings(s, 3)
		if err != nil {
			return nil, err
		}
		shapeData := s.S(4)
		if err := d.AddVertex(last(node), VertexArgs{Metadata: &shapeData}); err != nil {
			return nil, err
		}
		if err := d.AddLink(stmt.stmt, node, link); err != nil {
			return nil, err
		}
		s.Result = &vertexStmt{stmt: node, nodes: concat(node, stmt.nodes...)}
	case 46, 47: // yy.addLink($1.stmt,$3,$2); $$ = { stmt: $3, nodes: $3.concat($1.nodes) }
		stmt, err := asVertexStmt(s, 1)
		if err != nil {
			return nil, err
		}
		link, err := asLink(s, 2)
		if err != nil {
			return nil, err
		}
		node, err := asStrings(s, 3)
		if err != nil {
			return nil, err
		}
		if err := d.AddLink(stmt.stmt, node, link); err != nil {
			return nil, err
		}
		s.Result = &vertexStmt{stmt: node, nodes: concat(node, stmt.nodes...)}
	case 48, 50: // $$ = {stmt: $1, nodes:$1 }
		node, err := asStrings(s, 1)
		if err != nil {
			return nil, err
		}
		s.Result = &vertexStmt{stmt: node, nodes: node}
	case 49:
		// yy.addVertex($1[$1.length-1],undefined,undefined,undefined, undefined,undefined, undefined,$2);
		// $$ = {stmt: $1, nodes:$1, shapeData: $2}
		node, err := asStrings(s, 1)
		if err != nil {
			return nil, err
		}
		shapeData := s.S(2)
		if err := d.AddVertex(last(node), VertexArgs{Metadata: &shapeData}); err != nil {
			return nil, err
		}
		s.Result = &vertexStmt{stmt: node, nodes: node}
	case 51: // $$ = [$1];
		s.Result = []string{s.S(1)}
	case 52:
		// yy.addVertex($1[$1.length-1],undefined,undefined,undefined, undefined,undefined, undefined,$2); $$ = $1.concat($6);
		node, err := asStrings(s, 1)
		if err != nil {
			return nil, err
		}
		shapeData := s.S(2)
		if err := d.AddVertex(last(node), VertexArgs{Metadata: &shapeData}); err != nil {
			return nil, err
		}
		s.Result = concat(node, s.S(6))
	case 53: // $$ = $1.concat($5);
		node, err := asStrings(s, 1)
		if err != nil {
			return nil, err
		}
		s.Result = concat(node, s.S(5))
	case 54: // $$ = $1;
		s.Result = s.V(1)
	case 55: // $$ = $1;yy.setClass($1,$3)
		s.Result = s.V(1)
		d.SetClass(s.S(1), s.S(3))
	case 56, 57, 58, 59, 60, 61, 63, 64, 65, 66, 67, 68, 69, 70, 71:
		// $$ = $1;yy.addVertex($1,$3,'square'); and the other shapes
		shape := vertexShapes[production]
		text, err := asText(s, shape.text)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		if err := d.AddVertex(s.S(1), VertexArgs{TextObj: text, Type: shape.shape}); err != nil {
			return nil, err
		}
	case 62:
		// $$ = $1;yy.addVertex($1,$7,'rect',undefined,undefined,undefined, Object.fromEntries([[$3, $5]]));
		text, err := asText(s, 7)
		if err != nil {
			return nil, err
		}
		props := js.NewObj[any]()
		// Object.fromEntries defines the property, also for the key __proto__.
		props.Set(s.S(3), s.S(5))
		s.Result = s.V(1)
		if err := d.AddVertex(s.S(1), VertexArgs{TextObj: text, Type: "rect", Props: props}); err != nil {
			return nil, err
		}
	case 72: // $$ = $1;yy.addVertex($1);
		s.Result = s.V(1)
		if err := d.AddVertex(s.S(1), VertexArgs{}); err != nil {
			return nil, err
		}
	case 73, 75: // $1.text = $2;$$ = $1;
		link, err := asLink(s, 1)
		if err != nil {
			return nil, err
		}
		text, err := asText(s, 2)
		if err != nil {
			return nil, err
		}
		link.Text = text
		s.Result = link
	case 74:
		// $1.text = $2;$$ = $1; with $2 the text of a TESTSTR token. No rule
		// of the lexer makes this token, so the production does not occur.
		// Upstream would fail in addSingleLink, which reads the text of an
		// object: linkTextObj.text.trim().
		return nil, typeError("Cannot read properties of undefined (reading 'trim')")
	case 76: // $$ = $1;
		s.Result = s.V(1)
	case 77:
		// var inf = yy.destructLink($3, $1); $$ = {"type":inf.type,"stroke":inf.stroke,"length":inf.length,"text":$2};
		text, err := asText(s, 2)
		if err != nil {
			return nil, err
		}
		link := linkOf(d.DestructLink(s.S(3), s.S(1)))
		link.Text = text
		s.Result = link
	case 78:
		// var inf = yy.destructLink($4, $2); $$ = {"type":inf.type,"stroke":inf.stroke,"length":inf.length,"text":$3, "id": $1};
		text, err := asText(s, 3)
		if err != nil {
			return nil, err
		}
		link := linkOf(d.DestructLink(s.S(4), s.S(2)))
		link.Text = text
		link.ID, link.HasID = s.S(1), true
		s.Result = link
	case 79: // $$={text:$1, type:'text'};
		s.Result = &FlowText{Text: s.S(1), Type: "text"}
	case 80, 87, 102: // $$={text:$1.text+''+$2, type:$1.type};
		text, err := asText(s, 1)
		if err != nil {
			return nil, err
		}
		s.Result = &FlowText{Text: js.Concat(text.Text, s.S(2)), Type: text.Type}
	case 81: // $$={text: $1, type: 'string'};
		s.Result = &FlowText{Text: s.S(1), Type: "string"}
	case 82: // $$={text:$1, type:'markdown'};
		s.Result = &FlowText{Text: s.S(1), Type: "markdown"}
	case 83: // var inf = yy.destructLink($1);$$ = {"type":inf.type,"stroke":inf.stroke,"length":inf.length};
		s.Result = linkOf(d.DestructLink(s.S(1), ""))
	case 84: // var inf = yy.destructLink($2);$$ = {"type":inf.type,"stroke":inf.stroke,"length":inf.length, "id": $1};
		link := linkOf(d.DestructLink(s.S(2), ""))
		link.ID, link.HasID = s.S(1), true
		s.Result = link
	case 85: // $$ = $2;
		s.Result = s.V(2)
	case 86: // $$={text:$1, type: 'text'};
		s.Result = &FlowText{Text: s.S(1), Type: "text"}
	case 88: // $$ = {text: $1, type: 'string'};
		s.Result = &FlowText{Text: s.S(1), Type: "string"}
	case 89: // $$={text: $1, type: 'markdown'};
		s.Result = &FlowText{Text: s.S(1), Type: "markdown"}
	case 101: // $$={text:$1, type: 'text'};
		s.Result = &FlowText{Text: s.S(1), Type: "text"}
	case 103: // $$={text: $1, type: 'text'};
		s.Result = &FlowText{Text: s.S(1), Type: "text"}
	case 104: // $$={text: $1, type: 'markdown'};
		s.Result = &FlowText{Text: s.S(1), Type: "markdown"}
	case 105: // $$ = $1;yy.addClass($3,$5);
		style, err := asStrings(s, 5)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		d.AddClass(s.S(3), style)
	case 106: // $$ = $1;yy.setClass($3, $5);
		s.Result = s.V(1)
		d.SetClass(s.S(3), s.S(5))
	case 107, 115: // $$ = $1;yy.setClickEvent($1, $2);
		s.Result = s.V(1)
		d.SetClickEvent(s.S(1), s.S(2), nil)
	case 108, 116: // $$ = $1;yy.setClickEvent($1, $2);yy.setTooltip($1, $4);
		s.Result = s.V(1)
		d.SetClickEvent(s.S(1), s.S(2), nil)
		if err := d.SetTooltip(s.S(1), s.S(4)); err != nil {
			return nil, err
		}
	case 109: // $$ = $1;yy.setClickEvent($1, $2, $3);
		s.Result = s.V(1)
		args := s.S(3)
		d.SetClickEvent(s.S(1), s.S(2), &args)
	case 110: // $$ = $1;yy.setClickEvent($1, $2, $3);yy.setTooltip($1, $5);
		s.Result = s.V(1)
		args := s.S(3)
		d.SetClickEvent(s.S(1), s.S(2), &args)
		if err := d.SetTooltip(s.S(1), s.S(5)); err != nil {
			return nil, err
		}
	case 111: // $$ = $1;yy.setLink($1, $3);
		s.Result = s.V(1)
		d.SetLink(s.S(1), s.S(3), nil)
	case 112: // $$ = $1;yy.setLink($1, $3);yy.setTooltip($1, $5);
		s.Result = s.V(1)
		d.SetLink(s.S(1), s.S(3), nil)
		if err := d.SetTooltip(s.S(1), s.S(5)); err != nil {
			return nil, err
		}
	case 113: // $$ = $1;yy.setLink($1, $3, $5);
		s.Result = s.V(1)
		target := s.S(5)
		d.SetLink(s.S(1), s.S(3), &target)
	case 114: // $$ = $1;yy.setLink($1, $3, $7);yy.setTooltip($1, $5);
		s.Result = s.V(1)
		target := s.S(7)
		d.SetLink(s.S(1), s.S(3), &target)
		if err := d.SetTooltip(s.S(1), s.S(5)); err != nil {
			return nil, err
		}
	case 117: // $$ = $1;yy.setLink($1, $2);
		s.Result = s.V(1)
		d.SetLink(s.S(1), s.S(2), nil)
	case 118: // $$ = $1;yy.setLink($1, $2);yy.setTooltip($1, $4);
		s.Result = s.V(1)
		d.SetLink(s.S(1), s.S(2), nil)
		if err := d.SetTooltip(s.S(1), s.S(4)); err != nil {
			return nil, err
		}
	case 119: // $$ = $1;yy.setLink($1, $2, $4);
		s.Result = s.V(1)
		target := s.S(4)
		d.SetLink(s.S(1), s.S(2), &target)
	case 120: // $$ = $1;yy.setLink($1, $2, $6);yy.setTooltip($1, $4);
		s.Result = s.V(1)
		target := s.S(6)
		d.SetLink(s.S(1), s.S(2), &target)
		if err := d.SetTooltip(s.S(1), s.S(4)); err != nil {
			return nil, err
		}
	case 121: // $$ = $1;yy.addVertex($3,undefined,undefined,$5);
		style, err := asStrings(s, 5)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		if err := d.AddVertex(s.S(3), VertexArgs{Style: style}); err != nil {
			return nil, err
		}
	case 122: // $$ = $1;yy.updateLink([$3],$5);
		style, err := asStrings(s, 5)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		if err := d.UpdateLink([]string{s.S(3)}, style); err != nil {
			return nil, err
		}
	case 123: // $$ = $1;yy.updateLink($3,$5);
		positions, err := asStrings(s, 3)
		if err != nil {
			return nil, err
		}
		style, err := asStrings(s, 5)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		if err := d.UpdateLink(positions, style); err != nil {
			return nil, err
		}
	case 124: // $$ = $1;yy.updateLinkInterpolate([$3],$7);yy.updateLink([$3],$9);
		style, err := asStrings(s, 9)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		if err := d.UpdateLinkInterpolate([]string{s.S(3)}, s.S(7)); err != nil {
			return nil, err
		}
		if err := d.UpdateLink([]string{s.S(3)}, style); err != nil {
			return nil, err
		}
	case 125: // $$ = $1;yy.updateLinkInterpolate($3,$7);yy.updateLink($3,$9);
		positions, err := asStrings(s, 3)
		if err != nil {
			return nil, err
		}
		style, err := asStrings(s, 9)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		if err := d.UpdateLinkInterpolate(positions, s.S(7)); err != nil {
			return nil, err
		}
		if err := d.UpdateLink(positions, style); err != nil {
			return nil, err
		}
	case 126: // $$ = $1;yy.updateLinkInterpolate([$3],$7);
		s.Result = s.V(1)
		if err := d.UpdateLinkInterpolate([]string{s.S(3)}, s.S(7)); err != nil {
			return nil, err
		}
	case 127: // $$ = $1;yy.updateLinkInterpolate($3,$7);
		positions, err := asStrings(s, 3)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		if err := d.UpdateLinkInterpolate(positions, s.S(7)); err != nil {
			return nil, err
		}
	case 128, 130: // $$ = [$1]
		s.Result = []string{s.S(1)}
	case 129, 131: // $1.push($3);$$ = $1;
		list, err := asStrings(s, 1)
		if err != nil {
			return nil, err
		}
		s.Result = append(list, s.S(3))
	case 133: // $$ = $1 + $2;
		s.Result = js.Concat(s.S(1), s.S(2))
	case 181, 183: // $$=$1
		s.Result = s.V(1)
	case 182, 184: // $$=$1+''+$2
		s.Result = js.Concat(s.S(1), s.S(2))
	case 185, 186, 187, 188, 189: // $$={stmt:'dir', value:'TB'};
		s.Result = &dirStmt{value: directions[production]}
	}
	return nil, nil
}
