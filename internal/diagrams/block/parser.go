// Ported from mermaid 12.2.0, the semantic actions of
// diagrams/block/parser/block.jison (see
// internal/grammar/block/actions.js.txt) (MIT).

package block

import (
	"fmt"

	blockgrammar "github.com/bvolpato/mermaid-go-renderer/internal/grammar/block"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// parse is parser.parse(text) with yy = db.
func parse(d *DB, text string) error {
	_, err := blockgrammar.Parse(text, d, performAction)
	return err
}

// The semantic values of the grammar are:
//
//	string      the text of a token
//	*nodeValue  node
//	*shapeValue nodeShapeNLabel
//	*linkValue  link
//	[]string    dirList
//	*Block      a statement: a node, a space, a composite block, a columns
//	            statement, a classDef, a class or a style statement
//	[]*Block    a statement with links: the source block, the edge and the
//	            target block of the last link
//	[]any       document: the statements, each a *Block or a []*Block

// nodeValue is the value of the rule node: { id, label, typeStr, directions }.
type nodeValue struct {
	id       string
	label    string
	hasLabel bool
	// typeStr is "" when it is undefined: typeStr2Type gives "na" for both.
	typeStr    string
	directions []string
}

// shapeValue is the value of the rule nodeShapeNLabel:
// { typeStr, label, directions }.
type shapeValue struct {
	typeStr    string
	label      string
	directions []string
}

// linkValue is the value of the rule link: { edgeTypeStr, label }.
type linkValue struct {
	edgeTypeStr string
	label       string
}

func valueError(i int, want string) error {
	return fmt.Errorf("block parser: $%d is not %s", i, want)
}

func nodeAt(s *jison.ParserState, i int) (*nodeValue, error) {
	v, ok := s.V(i).(*nodeValue)
	if !ok {
		return nil, valueError(i, "a node")
	}
	return v, nil
}

func documentAt(s *jison.ParserState, i int) ([]any, error) {
	v, ok := s.V(i).([]any)
	if !ok {
		return nil, valueError(i, "a document")
	}
	return v, nil
}

func dirListAt(s *jison.ParserState, i int) ([]string, error) {
	v, ok := s.V(i).([]string)
	if !ok {
		return nil, valueError(i, "a list of directions")
	}
	return v, nil
}

// nodeBlock is { id: n.id, label: n.label, type: yy.typeStr2Type(n.typeStr),
// directions: n.directions }, the block of a node.
func nodeBlock(n *nodeValue) *Block {
	b := newBlock("id", "label", "type", "directions")
	b.ID = n.id
	b.Label, b.HasLabel = n.label, n.hasLabel
	b.Type = TypeStr2Type(n.typeStr)
	b.Directions = n.directions
	return b
}

// performAction is the generated performAction of the parser: the semantic
// action of one production.
func performAction(yy any, s *jison.ParserState, production int) (any, error) {
	d, ok := yy.(*DB)
	if !ok {
		return nil, fmt.Errorf("block parser: yy is %T", yy)
	}
	switch production {
	case 7: // start -> BLOCK_DIAGRAM_KEY document EOF
		document, err := documentAt(s, 2)
		if err != nil {
			return nil, err
		}
		if err := d.SetHierarchy(document); err != nil {
			return nil, err
		}

	case 12: // document -> statement
		// typeof $1.length === 'number' ? $$ = $1 : $$ = [$1]
		if list, ok := s.V(1).([]*Block); ok {
			document := make([]any, len(list))
			for i, b := range list {
				document[i] = b
			}
			s.Result = document
		} else {
			s.Result = []any{s.V(1)}
		}
	case 13: // document -> statement document
		// $$ = [$1].concat($2)
		document, err := documentAt(s, 2)
		if err != nil {
			return nil, err
		}
		result := make([]any, 0, len(document)+1)
		result = append(result, s.V(1))
		s.Result = append(result, document...)

	case 14: // link -> LINK
		s.Result = &linkValue{edgeTypeStr: s.S(1), label: ""}
	case 15: // link -> START_LINK LINK_LABEL STR LINK
		s.Result = &linkValue{edgeTypeStr: s.S(4), label: s.S(3)}

	case 18: // statement -> SPACE_BLOCK
		num := js.ParseIntAuto(s.S(1))
		spaceID := d.GenerateId()
		b := newBlock("id", "type", "label", "width", "children")
		b.ID = spaceID
		b.Type = typeSpace
		b.Label, b.HasLabel = "", true
		b.Width = num
		b.document, b.hasDocument = []any{}, true
		s.Result = b

	case 23: // nodeStatement -> nodeStatement link node
		link, ok := s.V(2).(*linkValue)
		if !ok {
			return nil, valueError(2, "a link")
		}
		node, err := nodeAt(s, 3)
		if err != nil {
			return nil, err
		}
		edgeData := EdgeStrToEdgeData(link.edgeTypeStr)
		startEdgeData := EdgeStrToEdgeStartData(link.edgeTypeStr)
		lineThickness := EdgeStrToThickness(link.edgeTypeStr)
		linePattern := EdgeStrToPattern(link.edgeTypeStr)

		// {id: $1.id, label: $1.label, type: $1.type, directions: $1.directions}
		// $1 is a block, or the list of blocks of the link before this
		// one. A list has none of these properties.
		source := newBlock("id", "label", "type", "directions")
		source.IDUndefined = true
		if first, ok := s.V(1).(*Block); ok {
			source.ID, source.IDUndefined = first.ID, first.IDUndefined
			source.Label, source.HasLabel = first.Label, first.HasLabel
			source.Type = first.Type
			source.Directions = first.Directions
		}

		edge := newBlock("id", "start", "end", "label", "type", "thickness", "pattern", "directions", "arrowTypeEnd", "arrowTypeStart")
		edge.ID = js.Concat(source.idString(), "-", node.id)
		edge.Start = source.ID
		edge.End = node.id
		edge.Label, edge.HasLabel = link.label, true
		edge.Type = typeEdge
		edge.Thickness = lineThickness
		edge.Pattern = linePattern
		edge.Directions = node.directions
		edge.ArrowTypeEnd = edgeData
		edge.ArrowTypeStart = startEdgeData

		s.Result = []*Block{source, edge, nodeBlock(node)}
	case 24: // nodeStatement -> node SIZE
		node, err := nodeAt(s, 1)
		if err != nil {
			return nil, err
		}
		b := nodeBlock(node)
		// parseInt($2, 10): the text of the token has digits only.
		b.WidthInColumns = js.ParseIntAuto(s.S(2))
		b.define("widthInColumns")
		s.Result = b
	case 25: // nodeStatement -> node
		node, err := nodeAt(s, 1)
		if err != nil {
			return nil, err
		}
		b := nodeBlock(node)
		b.WidthInColumns = 1
		b.define("widthInColumns")
		s.Result = b

	case 26: // columnsStatement -> COLUMNS
		// {type: 'column-setting', columns: $1 === 'auto' ? -1 : parseInt($1)}
		b := newBlock("type", "columns")
		b.IDUndefined = true
		b.Type = typeColumnSetting
		if s.S(1) == "auto" {
			b.Columns = -1
		} else {
			b.Columns = js.ParseIntAuto(s.S(1))
		}
		s.Result = b

	case 27: // blockStatement -> id-block nodeStatement document end
		document, err := documentAt(s, 3)
		if err != nil {
			return nil, err
		}
		// const id2 = yy.generateId(): the id is not used.
		d.GenerateId()
		// { ...$2, type: 'composite', children: $3 }
		var b *Block
		switch head := s.V(2).(type) {
		case *Block:
			b = head.clone()
		case []*Block:
			// The spread of a list gives the properties "0", "1" and "2".
			b = newBlock("0", "1", "2")
			b.IDUndefined = true
			b.spread = head
		default:
			return nil, valueError(2, "a node statement")
		}
		b.Type = typeComposite
		b.Children = nil
		b.document, b.hasDocument = document, true
		b.define("type", "children")
		s.Result = b
	case 28: // blockStatement -> BLOCK_DIAGRAM_KEY document end
		document, err := documentAt(s, 2)
		if err != nil {
			return nil, err
		}
		id := d.GenerateId()
		b := newBlock("id", "type", "label", "children")
		b.ID = id
		b.Type = typeComposite
		b.Label, b.HasLabel = "", true
		b.document, b.hasDocument = document, true
		s.Result = b

	case 29: // node -> NODE_ID
		s.Result = &nodeValue{id: s.S(1)}
	case 30: // node -> NODE_ID nodeShapeNLabel
		shape, ok := s.V(2).(*shapeValue)
		if !ok {
			return nil, valueError(2, "a node shape")
		}
		s.Result = &nodeValue{id: s.S(1), label: shape.label, hasLabel: true, typeStr: shape.typeStr, directions: shape.directions}

	case 31: // dirList -> DIR
		s.Result = []string{s.S(1)}
	case 32: // dirList -> DIR dirList
		// $$ = [$1].concat($2)
		list, err := dirListAt(s, 2)
		if err != nil {
			return nil, err
		}
		result := make([]string, 0, len(list)+1)
		result = append(result, s.S(1))
		s.Result = append(result, list...)

	case 33: // nodeShapeNLabel -> NODE_DSTART STR NODE_DEND
		s.Result = &shapeValue{typeStr: s.S(1) + s.S(3), label: s.S(2)}
	case 34: // nodeShapeNLabel -> BLOCK_ARROW_START STR dirList BLOCK_ARROW_END
		list, err := dirListAt(s, 3)
		if err != nil {
			return nil, err
		}
		s.Result = &shapeValue{typeStr: s.S(1) + s.S(4), label: s.S(2), directions: list}

	case 35, 36: // classDefStatement -> classDef CLASSDEF_ID CLASSDEF_STYLEOPTS | classDef DEFAULT CLASSDEF_STYLEOPTS
		b := newBlock("type", "id", "css")
		b.Type = typeClassDef
		b.ID = js.TrimSpace(s.S(2))
		b.CSS = js.TrimSpace(s.S(3))
		s.Result = b
	case 37: // cssClassStatement -> class CLASSENTITY_IDS STYLECLASS
		b := newBlock("type", "id", "styleClass")
		b.Type = typeApplyClass
		b.ID = js.TrimSpace(s.S(2))
		b.StyleClass = js.TrimSpace(s.S(3))
		s.Result = b
	case 38: // styleStatement -> style STYLE_ENTITY_IDS STYLE_DEFINITION_DATA
		b := newBlock("type", "id", "stylesStr")
		b.Type = typeApplyStyles
		b.ID = js.TrimSpace(s.S(2))
		b.StylesStr = js.TrimSpace(s.S(3))
		s.Result = b
	}
	return nil, nil
}
