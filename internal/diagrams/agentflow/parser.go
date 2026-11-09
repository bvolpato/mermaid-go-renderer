// Ported from mermaid 12.2.0, diagrams/agentflow/parser/agentflowParser.ts and
// the semantic actions of diagrams/agentflow/parser/agentflow.jison (see
// internal/grammar/agentflow/actions.js.txt) (MIT).

package agentflow

import (
	"fmt"
	"strings"
	"unicode/utf8"

	grammar "github.com/bvolpato/mermaid-go-renderer/internal/grammar/agentflow"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// prepareText is the text preparation of agentflowParser.ts:
//
//	src.replace(/}[^\S\n]*\n/g, '}\n')
//
// It removes the horizontal white space between a closing `}` and the line
// break of its line, so that the rule `node shapeData separator` of the
// grammar still reduces. The white space is every white space character
// but the line break: a blank line after an `@{ ... }` block stays, and the
// positions of the lines after it do not move.
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
		// The run of white space after the brace, up to a line break.
		j := i + 1
		for j < len(src) {
			r, size := utf8.DecodeRuneInString(src[j:])
			if r == '\n' || !js.IsWhiteSpace(r) {
				break
			}
			j += size
		}
		b.WriteByte('}')
		if j < len(src) && src[j] == '\n' {
			b.WriteByte('\n')
			i = j + 1
		} else {
			i++
		}
	}
	return b.String()
}

// parse is parser.parse(text) of agentflowParser.ts with yy = db.
func parse(d *DB, text string) error {
	_, err := grammar.Parse(prepareText(text), d, performAction)
	return err
}

// The semantic values of the grammar are:
//
//	string        the text of a token, an id, a style, the value of an
//	              accessibility statement, the id of a container or of a
//	              connector
//	[]string      a list of node ids, of numbers or of styles
//	[]any         a document: the values of its lines
//	*vertexStmt   a vertex statement: { stmt, nodes }
//	*FlowText     a text: { text, type }
//	*FlowLink     a link: { type, stroke, length, edgeSemantic, text, id }
//	*dirStmt      a direction statement: { stmt: 'dir', value }

func valueError(i int, want string) error {
	return fmt.Errorf("agentflow parser: $%d is not %s", i, want)
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

// loc is @i: the location of symbol i of the right side.
func loc(s *jison.ParserState, i int) *Location {
	return toLocation(s.L(i))
}

// toLocation converts a location of the driver.
func toLocation(l jison.Location) *Location {
	return &Location{
		FirstLine:   l.FirstLine,
		FirstColumn: l.FirstColumn,
		LastLine:    l.LastLine,
		LastColumn:  l.LastColumn,
	}
}

// vertexShapes maps the productions of vertex with one text to the shape
// and to the position of the text in the production.
var vertexShapes = map[int]struct {
	shape string
	text  int
}{
	69: {"square", 3},
	70: {"doublecircle", 3},
	71: {"circle", 4},
	72: {"ellipse", 3},
	73: {"stadium", 3},
	74: {"subroutine", 3},
	76: {"cylinder", 3},
	77: {"round", 3},
	78: {"diamond", 3},
	79: {"hexagon", 4},
	80: {"odd", 3},
	81: {"trapezoid", 3},
	82: {"inv_trapezoid", 3},
	83: {"lean_right", 3},
	84: {"lean_left", 3},
}

// directions maps the productions of direction to the value.
var directions = map[int]string{200: "TB", 201: "BT", 202: "RL", 203: "LR", 204: "TD"}

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

// positions is the list of the positions of a linkStyle statement: the
// texts of the numbers, or "default".
func positions(list ...string) []any {
	out := make([]any, len(list))
	for i, p := range list {
		out[i] = p
	}
	return out
}

// linkOf is { type: inf.type, stroke: inf.stroke, length: inf.length,
// edgeSemantic: inf.edgeSemantic }.
func linkOf(inf FlowLink) *FlowLink {
	return &FlowLink{Type: inf.Type, Stroke: inf.Stroke, Length: inf.Length, EdgeSemantic: inf.EdgeSemantic}
}

// emptyText is the text object with an empty text and the type 'text'.
func emptyText() *FlowText {
	return &FlowText{Text: "", Type: "text"}
}

// performAction is the port of the generated parser.performAction.
func performAction(yy any, s *jison.ParserState, production int) (any, error) {
	d, ok := yy.(*DB)
	if !ok {
		return nil, fmt.Errorf("agentflow parser: yy is not an agentflow db")
	}
	// addShapeData is yy.addVertex(id, ..., $shapeData, @shapeData).
	addShapeData := func(id string, i int) error {
		shapeData := s.S(i)
		return d.AddVertex(id, VertexArgs{Metadata: &shapeData, MetadataLoc: loc(s, i)})
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
	case 5: // $$=[];
		s.Result = []string{}
	case 13: // yy.setDirection('TB');$$ = 'TB';
		d.SetDirection("TB")
		s.Result = "TB"
	case 14: // yy.setDirection($2);$$ = $2;
		d.SetDirection(s.S(2))
		s.Result = s.S(2)
	case 32: // $$=$1.nodes
		stmt, err := asVertexStmt(s, 1)
		if err != nil {
			return nil, err
		}
		s.Result = stmt.nodes
	case 33, 34, 35, 36, 37: // $$=[];
		s.Result = []string{}
	case 38:
		// $$=yy.addSubGraph($3,$8,$5,'flow');if(yy.addSubgraphMapping){yy.addSubgraphMapping($3,$5,@1,@9);}
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
		result, err := d.AddSubGraph(id, list, title, "flow")
		if err != nil {
			return nil, err
		}
		s.Result = result
		d.AddSubgraphMapping(id, loc(s, 1), loc(s, 9))
	case 39:
		// $$=yy.addSubGraph($3,$9,$5,'flow');yy.addVertex($$,...,$7,@7);if(yy.addSubgraphMapping){yy.addSubgraphMapping($3,$5,@1,@10);}
		id, err := asText(s, 3)
		if err != nil {
			return nil, err
		}
		list, err := asDocument(s, 9)
		if err != nil {
			return nil, err
		}
		title, err := asText(s, 5)
		if err != nil {
			return nil, err
		}
		result, err := d.AddSubGraph(id, list, title, "flow")
		if err != nil {
			return nil, err
		}
		s.Result = result
		if err := addShapeData(result, 7); err != nil {
			return nil, err
		}
		d.AddSubgraphMapping(id, loc(s, 1), loc(s, 10))
	case 40:
		// $$=yy.addSubGraph($3,$5,{text:'', type:'text'},'flow');if(yy.addSubgraphMapping){yy.addSubgraphMapping($3,{text:'', type:'text'},@1,@6);}
		id, err := asText(s, 3)
		if err != nil {
			return nil, err
		}
		list, err := asDocument(s, 5)
		if err != nil {
			return nil, err
		}
		result, err := d.AddSubGraph(id, list, emptyText(), "flow")
		if err != nil {
			return nil, err
		}
		s.Result = result
		d.AddSubgraphMapping(id, loc(s, 1), loc(s, 6))
	case 41:
		// $$=yy.addSubGraph($3,$6,{text:'', type:'text'},'flow');yy.addVertex($$,...,$4,@4);if(yy.addSubgraphMapping){yy.addSubgraphMapping($3,{text:'', type:'text'},@1,@7);}
		id, err := asText(s, 3)
		if err != nil {
			return nil, err
		}
		list, err := asDocument(s, 6)
		if err != nil {
			return nil, err
		}
		result, err := d.AddSubGraph(id, list, emptyText(), "flow")
		if err != nil {
			return nil, err
		}
		s.Result = result
		if err := addShapeData(result, 4); err != nil {
			return nil, err
		}
		d.AddSubgraphMapping(id, loc(s, 1), loc(s, 7))
	case 42:
		// $$=yy.addSubGraph(undefined,$3,undefined,'flow');if(yy.addSubgraphMapping){yy.addSubgraphMapping(undefined,undefined,@1,@4);}
		list, err := asDocument(s, 3)
		if err != nil {
			return nil, err
		}
		result, err := d.AddSubGraph(nil, list, nil, "flow")
		if err != nil {
			return nil, err
		}
		s.Result = result
		d.AddSubgraphMapping(nil, loc(s, 1), loc(s, 4))
	case 43: // yy.addGlobal($3);$$=[];
		list, err := asDocument(s, 3)
		if err != nil {
			return nil, err
		}
		d.AddGlobal(list)
		s.Result = []string{}
	case 44:
		// $$=yy.addConnector($3,$5);if(yy.addConnectorMapping){yy.addConnectorMapping($3,$5,@1,@7);}
		id, err := asText(s, 3)
		if err != nil {
			return nil, err
		}
		title, err := asText(s, 5)
		if err != nil {
			return nil, err
		}
		result, err := d.AddConnector(id, title)
		if err != nil {
			return nil, err
		}
		s.Result = result
		d.AddConnectorMapping(id, loc(s, 1), loc(s, 7))
	case 45:
		// $$=yy.addConnector($3,$5);yy.addVertex($$,...,$7,@7);if(yy.addConnectorMapping){yy.addConnectorMapping($3,$5,@1,@8);}
		id, err := asText(s, 3)
		if err != nil {
			return nil, err
		}
		title, err := asText(s, 5)
		if err != nil {
			return nil, err
		}
		result, err := d.AddConnector(id, title)
		if err != nil {
			return nil, err
		}
		s.Result = result
		if err := addShapeData(result, 7); err != nil {
			return nil, err
		}
		d.AddConnectorMapping(id, loc(s, 1), loc(s, 8))
	case 46:
		// $$=yy.addConnector($3,{text:'', type:'text'});if(yy.addConnectorMapping){yy.addConnectorMapping($3,{text:'', type:'text'},@1,@4);}
		id, err := asText(s, 3)
		if err != nil {
			return nil, err
		}
		result, err := d.AddConnector(id, emptyText())
		if err != nil {
			return nil, err
		}
		s.Result = result
		d.AddConnectorMapping(id, loc(s, 1), loc(s, 4))
	case 47:
		// $$=yy.addConnector($3,{text:'', type:'text'});yy.addVertex($$,...,$4,@4);if(yy.addConnectorMapping){yy.addConnectorMapping($3,{text:'', type:'text'},@1,@5);}
		id, err := asText(s, 3)
		if err != nil {
			return nil, err
		}
		result, err := d.AddConnector(id, emptyText())
		if err != nil {
			return nil, err
		}
		s.Result = result
		if err := addShapeData(result, 4); err != nil {
			return nil, err
		}
		d.AddConnectorMapping(id, loc(s, 1), loc(s, 5))
	case 49: // $$=$2.trim();yy.setAccTitle($$);
		v := js.TrimSpace(s.S(2))
		s.Result = v
		d.SetAccTitle(v)
	case 50: // $$=$2.trim();yy.setAccDescription($$);
		v := js.TrimSpace(s.S(2))
		s.Result = v
		d.SetAccDescription(v)
	case 51: // $$=$1.trim();yy.setAccDescription($$);
		v := js.TrimSpace(s.S(1))
		s.Result = v
		d.SetAccDescription(v)
	case 56: // $$ = $1 + $2;
		s.Result = js.Concat(s.S(1), s.S(2))
	case 57: // $$ = $1;
		s.Result = s.V(1)
	case 58:
		// yy.addVertex($3[$3.length-1],...,$4,@4); if(yy.extendVertexMapping){yy.extendVertexMapping($3[$3.length-1],@4);}
		// yy.addLink($1.stmt,$3,$2); if(yy.addEdgeMapping){yy.addEdgeMapping($1.stmt,$3,$2,@$);}
		// $$ = { stmt: $3, nodes: $3.concat($1.nodes) }
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
		if err := addShapeData(last(node), 4); err != nil {
			return nil, err
		}
		d.ExtendVertexMapping(last(node), loc(s, 4))
		if err := d.AddLink(stmt.stmt, node, link); err != nil {
			return nil, err
		}
		d.AddEdgeMapping(node, toLocation(s.Loc))
		s.Result = &vertexStmt{stmt: node, nodes: concat(node, stmt.nodes...)}
	case 59, 60:
		// yy.addLink($1.stmt,$3,$2); if(yy.addEdgeMapping){yy.addEdgeMapping($1.stmt,$3,$2,@$);}
		// $$ = { stmt: $3, nodes: $3.concat($1.nodes) }
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
		d.AddEdgeMapping(node, toLocation(s.Loc))
		s.Result = &vertexStmt{stmt: node, nodes: concat(node, stmt.nodes...)}
	case 61, 63: // $$ = {stmt: $1, nodes:$1 }
		node, err := asStrings(s, 1)
		if err != nil {
			return nil, err
		}
		s.Result = &vertexStmt{stmt: node, nodes: node}
	case 62:
		// yy.addVertex($1[$1.length-1],...,$2,@2);
		// if(yy.extendVertexMapping){yy.extendVertexMapping($1[$1.length-1],@2);}
		// $$ = {stmt: $1, nodes:$1, shapeData: $2}
		node, err := asStrings(s, 1)
		if err != nil {
			return nil, err
		}
		if err := addShapeData(last(node), 2); err != nil {
			return nil, err
		}
		d.ExtendVertexMapping(last(node), loc(s, 2))
		s.Result = &vertexStmt{stmt: node, nodes: node}
	case 64: // $$ = [$1];
		s.Result = []string{s.S(1)}
	case 65:
		// yy.addVertex($1[$1.length-1],...,$2,@2); if(yy.extendVertexMapping){yy.extendVertexMapping($1[$1.length-1],@2);} $$ = $1.concat($6);
		node, err := asStrings(s, 1)
		if err != nil {
			return nil, err
		}
		if err := addShapeData(last(node), 2); err != nil {
			return nil, err
		}
		d.ExtendVertexMapping(last(node), loc(s, 2))
		s.Result = concat(node, s.S(6))
	case 66: // $$ = $1.concat($5);
		node, err := asStrings(s, 1)
		if err != nil {
			return nil, err
		}
		s.Result = concat(node, s.S(5))
	case 67: // $$ = $1;
		s.Result = s.V(1)
	case 68: // $$ = $1;yy.setClass($1,$3)
		s.Result = s.V(1)
		d.SetClass(s.S(1), s.S(3))
	case 69, 70, 71, 72, 73, 74, 76, 77, 78, 79, 80, 81, 82, 83, 84:
		// $$ = $1;yy.addVertex($1,$3,'square');if(yy.addVertexMapping){yy.addVertexMapping($1,$3,'square',@$);}
		// and the other shapes
		shape := vertexShapes[production]
		text, err := asText(s, shape.text)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		if err := d.AddVertex(s.S(1), VertexArgs{TextObj: text, Type: shape.shape}); err != nil {
			return nil, err
		}
		d.AddVertexMapping(s.S(1), true, toLocation(s.Loc))
	case 75:
		// $$ = $1;yy.addVertex($1,$7,'rect',undefined,undefined,undefined, Object.fromEntries([[$3, $5]]));
		// if(yy.addVertexMapping){yy.addVertexMapping($1,$7,'rect',@$);}
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
		d.AddVertexMapping(s.S(1), true, toLocation(s.Loc))
	case 85:
		// $$ = $1;yy.addVertex($1);if(yy.addVertexMapping){yy.addVertexMapping($1,undefined,undefined,@$);}
		s.Result = s.V(1)
		if err := d.AddVertex(s.S(1), VertexArgs{}); err != nil {
			return nil, err
		}
		d.AddVertexMapping(s.S(1), false, toLocation(s.Loc))
	case 86, 88: // $1.text = $2;$$ = $1;
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
	case 87:
		// $1.text = $2;$$ = $1; with $2 the text of a TESTSTR token. No rule
		// of the lexer makes this token, so the production does not occur.
		// Upstream would fail in addSingleLink, which reads the text of an
		// object: linkTextObj.text.trim().
		return nil, typeError("Cannot read properties of undefined (reading 'trim')")
	case 89: // $$ = $1;
		s.Result = s.V(1)
	case 90:
		// var inf = yy.destructLink($3, $1); $$ = {"type":inf.type,"stroke":inf.stroke,"length":inf.length,"edgeSemantic":inf.edgeSemantic,"text":$2};
		text, err := asText(s, 2)
		if err != nil {
			return nil, err
		}
		link := linkOf(d.DestructLink(s.S(3), s.S(1)))
		link.Text = text
		s.Result = link
	case 91:
		// var inf = yy.destructLink($4, $2); $$ = {"type":inf.type,"stroke":inf.stroke,"length":inf.length,"edgeSemantic":inf.edgeSemantic,"text":$3, "id": $1};
		text, err := asText(s, 3)
		if err != nil {
			return nil, err
		}
		link := linkOf(d.DestructLink(s.S(4), s.S(2)))
		link.Text = text
		link.ID, link.HasID = s.S(1), true
		s.Result = link
	case 92: // $$={text:$1, type:'text'};
		s.Result = &FlowText{Text: s.S(1), Type: "text"}
	case 93, 100, 117: // $$={text:$1.text+''+$2, type:$1.type};
		text, err := asText(s, 1)
		if err != nil {
			return nil, err
		}
		s.Result = &FlowText{Text: js.Concat(text.Text, s.S(2)), Type: text.Type}
	case 94: // $$={text: $1, type: 'string'};
		s.Result = &FlowText{Text: s.S(1), Type: "string"}
	case 95: // $$={text:$1, type:'markdown'};
		s.Result = &FlowText{Text: s.S(1), Type: "markdown"}
	case 96: // var inf = yy.destructLink($1);$$ = {"type":inf.type,"stroke":inf.stroke,"length":inf.length,"edgeSemantic":inf.edgeSemantic};
		s.Result = linkOf(d.DestructLink(s.S(1), ""))
	case 97: // var inf = yy.destructLink($2);$$ = {..., "id": $1};
		link := linkOf(d.DestructLink(s.S(2), ""))
		link.ID, link.HasID = s.S(1), true
		s.Result = link
	case 98: // $$ = $2;
		s.Result = s.V(2)
	case 99: // $$={text:$1, type: 'text'};
		s.Result = &FlowText{Text: s.S(1), Type: "text"}
	case 101: // $$ = {text: $1, type: 'string'};
		s.Result = &FlowText{Text: s.S(1), Type: "string"}
	case 102: // $$={text: $1, type: 'markdown'};
		s.Result = &FlowText{Text: s.S(1), Type: "markdown"}
	case 116: // $$={text:$1, type: 'text'};
		s.Result = &FlowText{Text: s.S(1), Type: "text"}
	case 118: // $$={text: $1, type: 'text'};
		s.Result = &FlowText{Text: s.S(1), Type: "text"}
	case 119: // $$={text: $1, type: 'markdown'};
		s.Result = &FlowText{Text: s.S(1), Type: "markdown"}
	case 120: // $$ = $1;yy.addClass($3,$5);
		style, err := asStrings(s, 5)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		d.AddClass(s.S(3), style)
	case 121: // $$ = $1;yy.setClass($3, $5);
		s.Result = s.V(1)
		d.SetClass(s.S(3), s.S(5))
	case 122, 130: // $$ = $1;yy.setClickEvent($1, $2);
		s.Result = s.V(1)
		d.SetClickEvent(s.S(1), s.S(2), nil)
	case 123, 131: // $$ = $1;yy.setClickEvent($1, $2);yy.setTooltip($1, $4);
		s.Result = s.V(1)
		d.SetClickEvent(s.S(1), s.S(2), nil)
		if err := d.SetTooltip(s.S(1), s.S(4)); err != nil {
			return nil, err
		}
	case 124: // $$ = $1;yy.setClickEvent($1, $2, $3);
		s.Result = s.V(1)
		args := s.S(3)
		d.SetClickEvent(s.S(1), s.S(2), &args)
	case 125: // $$ = $1;yy.setClickEvent($1, $2, $3);yy.setTooltip($1, $5);
		s.Result = s.V(1)
		args := s.S(3)
		d.SetClickEvent(s.S(1), s.S(2), &args)
		if err := d.SetTooltip(s.S(1), s.S(5)); err != nil {
			return nil, err
		}
	case 126: // $$ = $1;yy.setLink($1, $3);
		s.Result = s.V(1)
		d.SetLink(s.S(1), s.S(3), nil)
	case 127: // $$ = $1;yy.setLink($1, $3);yy.setTooltip($1, $5);
		s.Result = s.V(1)
		d.SetLink(s.S(1), s.S(3), nil)
		if err := d.SetTooltip(s.S(1), s.S(5)); err != nil {
			return nil, err
		}
	case 128: // $$ = $1;yy.setLink($1, $3, $5);
		s.Result = s.V(1)
		target := s.S(5)
		d.SetLink(s.S(1), s.S(3), &target)
	case 129: // $$ = $1;yy.setLink($1, $3, $7);yy.setTooltip($1, $5);
		s.Result = s.V(1)
		target := s.S(7)
		d.SetLink(s.S(1), s.S(3), &target)
		if err := d.SetTooltip(s.S(1), s.S(5)); err != nil {
			return nil, err
		}
	case 132: // $$ = $1;yy.setLink($1, $2);
		s.Result = s.V(1)
		d.SetLink(s.S(1), s.S(2), nil)
	case 133: // $$ = $1;yy.setLink($1, $2);yy.setTooltip($1, $4);
		s.Result = s.V(1)
		d.SetLink(s.S(1), s.S(2), nil)
		if err := d.SetTooltip(s.S(1), s.S(4)); err != nil {
			return nil, err
		}
	case 134: // $$ = $1;yy.setLink($1, $2, $4);
		s.Result = s.V(1)
		target := s.S(4)
		d.SetLink(s.S(1), s.S(2), &target)
	case 135: // $$ = $1;yy.setLink($1, $2, $6);yy.setTooltip($1, $4);
		s.Result = s.V(1)
		target := s.S(6)
		d.SetLink(s.S(1), s.S(2), &target)
		if err := d.SetTooltip(s.S(1), s.S(4)); err != nil {
			return nil, err
		}
	case 136: // $$ = $1;yy.addVertex($3,undefined,undefined,$5);
		style, err := asStrings(s, 5)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		if err := d.AddVertex(s.S(3), VertexArgs{Style: style}); err != nil {
			return nil, err
		}
	case 137: // $$ = $1;yy.updateLink([$3],$5);
		style, err := asStrings(s, 5)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		if err := d.UpdateLink(positions(s.S(3)), style); err != nil {
			return nil, err
		}
	case 138: // $$ = $1;yy.updateLink($3,$5);
		list, err := asStrings(s, 3)
		if err != nil {
			return nil, err
		}
		style, err := asStrings(s, 5)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		if err := d.UpdateLink(positions(list...), style); err != nil {
			return nil, err
		}
	case 139: // $$ = $1;yy.updateLinkInterpolate([$3],$7);yy.updateLink([$3],$9);
		style, err := asStrings(s, 9)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		if err := d.UpdateLinkInterpolate(positions(s.S(3)), s.S(7)); err != nil {
			return nil, err
		}
		if err := d.UpdateLink(positions(s.S(3)), style); err != nil {
			return nil, err
		}
	case 140: // $$ = $1;yy.updateLinkInterpolate($3,$7);yy.updateLink($3,$9);
		list, err := asStrings(s, 3)
		if err != nil {
			return nil, err
		}
		style, err := asStrings(s, 9)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		if err := d.UpdateLinkInterpolate(positions(list...), s.S(7)); err != nil {
			return nil, err
		}
		if err := d.UpdateLink(positions(list...), style); err != nil {
			return nil, err
		}
	case 141: // $$ = $1;yy.updateLinkInterpolate([$3],$7);
		s.Result = s.V(1)
		if err := d.UpdateLinkInterpolate(positions(s.S(3)), s.S(7)); err != nil {
			return nil, err
		}
	case 142: // $$ = $1;yy.updateLinkInterpolate($3,$7);
		list, err := asStrings(s, 3)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		if err := d.UpdateLinkInterpolate(positions(list...), s.S(7)); err != nil {
			return nil, err
		}
	case 143, 145: // $$ = [$1]
		s.Result = []string{s.S(1)}
	case 144, 146: // $1.push($3);$$ = $1;
		list, err := asStrings(s, 1)
		if err != nil {
			return nil, err
		}
		s.Result = append(list, s.S(3))
	case 148: // $$ = $1 + $2;
		s.Result = js.Concat(s.S(1), s.S(2))
	case 196, 198: // $$=$1
		s.Result = s.V(1)
	case 197, 199: // $$=$1+''+$2
		s.Result = js.Concat(s.S(1), s.S(2))
	case 200, 201, 202, 203, 204: // $$={stmt:'dir', value:'TB'};
		s.Result = &dirStmt{value: directions[production]}
	}
	return nil, nil
}
