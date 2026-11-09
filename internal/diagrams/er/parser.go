// Ported from mermaid 12.2.0, the semantic actions of
// diagrams/er/parser/erDiagram.jison (see
// internal/grammar/erdiagram/actions.js.txt) (MIT).

package er

import (
	"fmt"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/erdiagram"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// parse is parser.parse(text) with yy = db.
func parse(d *DB, text string) error {
	_, err := erdiagram.Parse(text, d, performAction)
	return err
}

// The semantic values of the grammar are:
//
//	string           the text of a token, a name, a style, a role, the
//	                 value of an accessibility statement, the id of a
//	                 subgraph
//	[]string         a list of ids, of styles or of attribute keys
//	[]any            a document or the value of a statement: names
//	                 (string) and direction statements (*DirStmt)
//	*DirStmt         a direction statement: { stmt: 'dir', value }
//	*subgraphHeader  the header of a subgraph: { id, text }
//	*Attribute       an attribute
//	[]*Attribute     a list of attributes, the last attribute first
//	*RelSpec         the specification of a relationship

// subgraphHeader is the value of the rule subgraphHeader: { id, text }.
type subgraphHeader struct {
	id   string
	text string
}

func valueError(i int, want string) error {
	return fmt.Errorf("er parser: $%d is not %s", i, want)
}

func stringsAt(s *jison.ParserState, i int) ([]string, error) {
	v, ok := s.V(i).([]string)
	if !ok {
		return nil, valueError(i, "a list of strings")
	}
	return v, nil
}

func attributesAt(s *jison.ParserState, i int) ([]*Attribute, error) {
	v, ok := s.V(i).([]*Attribute)
	if !ok {
		return nil, valueError(i, "a list of attributes")
	}
	return v, nil
}

func relSpecAt(s *jison.ParserState, i int) (*RelSpec, error) {
	v, ok := s.V(i).(*RelSpec)
	if !ok || v == nil {
		return nil, valueError(i, "a relationship specification")
	}
	return v, nil
}

// removeQuotes is s.replace(/"/g, ...) with an empty string.
func removeQuotes(s string) string {
	return strings.ReplaceAll(s, `"`, "")
}

// concat is $1.concat($2) for a document: the elements of a list value, or
// the value itself.
func concat(doc []any, v any) []any {
	out := make([]any, len(doc), len(doc)+2)
	copy(out, doc)
	if list, ok := v.([]any); ok {
		return append(out, list...)
	}
	return append(out, v)
}

// performAction is the generated performAction of the parser: the semantic
// action of one production.
func performAction(yy any, s *jison.ParserState, production int) (any, error) {
	d, ok := yy.(*DB)
	if !ok {
		return nil, fmt.Errorf("er parser: yy is %T", yy)
	}
	switch production {
	case 1: // start -> ER_DIAGRAM document EOF
	case 2: // document -> (empty)
		s.Result = []any{}
	case 3: // document -> document line
		doc, ok := s.V(1).([]any)
		if !ok {
			return nil, valueError(1, "a document")
		}
		s.Result = concat(doc, s.V(2))
	case 4: // line -> statement
		s.Result = s.V(1)
	case 5, 6: // line -> NEWLINE | EOF
		s.Result = []any{}

	case 7: // statement -> entityName relSpec entityName COLON role
		rel, err := relSpecAt(s, 2)
		if err != nil {
			return nil, err
		}
		d.AddEntity(s.S(1))
		d.AddEntity(s.S(3))
		d.AddRelationship(s.S(1), s.S(5), s.S(3), rel)
		s.Result = []any{s.S(1), s.S(3)}
	case 8: // statement -> entityName STYLE_SEPARATOR idList relSpec entityName STYLE_SEPARATOR idList COLON role
		ids3, err := stringsAt(s, 3)
		if err != nil {
			return nil, err
		}
		rel, err := relSpecAt(s, 4)
		if err != nil {
			return nil, err
		}
		ids7, err := stringsAt(s, 7)
		if err != nil {
			return nil, err
		}
		d.AddEntity(s.S(1))
		d.AddEntity(s.S(5))
		d.AddRelationship(s.S(1), s.S(9), s.S(5), rel)
		d.SetClass([]string{s.S(1)}, ids3)
		d.SetClass([]string{s.S(5)}, ids7)
		s.Result = []any{s.S(1), s.S(5)}
	case 9: // statement -> entityName STYLE_SEPARATOR idList relSpec entityName COLON role
		ids3, err := stringsAt(s, 3)
		if err != nil {
			return nil, err
		}
		rel, err := relSpecAt(s, 4)
		if err != nil {
			return nil, err
		}
		d.AddEntity(s.S(1))
		d.AddEntity(s.S(5))
		d.AddRelationship(s.S(1), s.S(7), s.S(5), rel)
		d.SetClass([]string{s.S(1)}, ids3)
		s.Result = []any{s.S(1), s.S(5)}
	case 10: // statement -> entityName relSpec entityName STYLE_SEPARATOR idList COLON role
		rel, err := relSpecAt(s, 2)
		if err != nil {
			return nil, err
		}
		ids5, err := stringsAt(s, 5)
		if err != nil {
			return nil, err
		}
		d.AddEntity(s.S(1))
		d.AddEntity(s.S(3))
		d.AddRelationship(s.S(1), s.S(7), s.S(3), rel)
		d.SetClass([]string{s.S(3)}, ids5)
		s.Result = []any{s.S(1), s.S(3)}
	case 11: // statement -> entityName BLOCK_START attributes BLOCK_STOP
		attribs, err := attributesAt(s, 3)
		if err != nil {
			return nil, err
		}
		d.AddEntity(s.S(1))
		d.AddAttributes(s.S(1), attribs)
		s.Result = []any{s.S(1)}
	case 12: // statement -> entityName STYLE_SEPARATOR idList BLOCK_START attributes BLOCK_STOP
		ids3, err := stringsAt(s, 3)
		if err != nil {
			return nil, err
		}
		attribs, err := attributesAt(s, 5)
		if err != nil {
			return nil, err
		}
		d.AddEntity(s.S(1))
		d.AddAttributes(s.S(1), attribs)
		d.SetClass([]string{s.S(1)}, ids3)
		s.Result = []any{s.S(1)}
	case 13: // statement -> entityName BLOCK_START BLOCK_STOP
		d.AddEntity(s.S(1))
		s.Result = []any{s.S(1)}
	case 14: // statement -> entityName STYLE_SEPARATOR idList BLOCK_START BLOCK_STOP
		ids3, err := stringsAt(s, 3)
		if err != nil {
			return nil, err
		}
		d.AddEntity(s.S(1))
		d.SetClass([]string{s.S(1)}, ids3)
		s.Result = []any{s.S(1)}
	case 15: // statement -> entityName
		d.AddEntity(s.S(1))
		s.Result = []any{s.S(1)}
	case 16: // statement -> entityName STYLE_SEPARATOR idList
		ids3, err := stringsAt(s, 3)
		if err != nil {
			return nil, err
		}
		d.AddEntity(s.S(1))
		d.SetClass([]string{s.S(1)}, ids3)
		s.Result = []any{s.S(1)}
	case 17: // statement -> entityName SQS entityName SQE BLOCK_START attributes BLOCK_STOP
		attribs, err := attributesAt(s, 6)
		if err != nil {
			return nil, err
		}
		d.AddEntity(s.S(1), s.S(3))
		d.AddAttributes(s.S(1), attribs)
		s.Result = []any{s.S(1)}
	case 18: // statement -> entityName SQS entityName SQE STYLE_SEPARATOR idList BLOCK_START attributes BLOCK_STOP
		ids6, err := stringsAt(s, 6)
		if err != nil {
			return nil, err
		}
		attribs, err := attributesAt(s, 8)
		if err != nil {
			return nil, err
		}
		d.AddEntity(s.S(1), s.S(3))
		d.AddAttributes(s.S(1), attribs)
		d.SetClass([]string{s.S(1)}, ids6)
		s.Result = []any{s.S(1)}
	case 19: // statement -> entityName SQS entityName SQE BLOCK_START BLOCK_STOP
		d.AddEntity(s.S(1), s.S(3))
		s.Result = []any{s.S(1)}
	case 20: // statement -> entityName SQS entityName SQE STYLE_SEPARATOR idList BLOCK_START BLOCK_STOP
		ids6, err := stringsAt(s, 6)
		if err != nil {
			return nil, err
		}
		d.AddEntity(s.S(1), s.S(3))
		d.SetClass([]string{s.S(1)}, ids6)
		s.Result = []any{s.S(1)}
	case 21: // statement -> entityName SQS entityName SQE
		// The action sets no value: $$ stays $1, the name.
		d.AddEntity(s.S(1), s.S(3))
	case 22: // statement -> entityName SQS entityName SQE STYLE_SEPARATOR idList
		ids6, err := stringsAt(s, 6)
		if err != nil {
			return nil, err
		}
		d.AddEntity(s.S(1), s.S(3))
		d.SetClass([]string{s.S(1)}, ids6)
	case 23, 24: // statement -> title title_value | acc_title acc_title_value
		value := js.TrimSpace(s.S(2))
		s.Result = value
		d.SetAccTitle(value)
	case 25: // statement -> acc_descr acc_descr_value
		value := js.TrimSpace(s.S(2))
		s.Result = value
		d.SetAccDescription(value)
	case 26: // statement -> acc_descr_multiline_value
		value := js.TrimSpace(s.S(1))
		s.Result = value
		d.SetAccDescription(value)
	case 27: // statement -> direction
		dir, ok := s.V(1).(*DirStmt)
		if !ok || dir == nil {
			return nil, valueError(1, "a direction statement")
		}
		if d.SubgraphDepth == 0 {
			d.SetDirection(dir.Value)
			s.Result = []any{}
		} else {
			s.Result = dir
		}
	case 28, 29, 30: // statement -> classDefStatement | classStatement | styleStatement
	case 31: // statement -> subgraphHeader document END
		header, ok := s.V(1).(*subgraphHeader)
		if !ok || header == nil {
			return nil, valueError(1, "a subgraph header")
		}
		doc, ok := s.V(2).([]any)
		if !ok {
			return nil, valueError(2, "a document")
		}
		// yy.subgraphDepth = (yy.subgraphDepth || 1) - 1
		if d.SubgraphDepth == 0 {
			d.SubgraphDepth = 1
		}
		d.SubgraphDepth--
		s.Result = d.AddSubGraph(header.id, doc, header.text)

	case 32: // subgraphHeader -> SUBGRAPH entityName separator
		d.SubgraphDepth++
		s.Result = &subgraphHeader{id: s.S(2), text: s.S(2)}
	case 33: // subgraphHeader -> SUBGRAPH entityName SQS subgraphTitle SQE separator
		d.SubgraphDepth++
		s.Result = &subgraphHeader{id: s.S(2), text: s.S(4)}
	case 34: // subgraphTitle -> entityName
		s.Result = s.V(1)
	case 35: // subgraphTitle -> subgraphTitle entityName
		s.Result = js.Concat(s.S(1), " ", s.S(2))

	case 36: // direction -> direction_tb
		s.Result = &DirStmt{Stmt: "dir", Value: "TB"}
	case 37: // direction -> direction_bt
		s.Result = &DirStmt{Stmt: "dir", Value: "BT"}
	case 38: // direction -> direction_rl
		s.Result = &DirStmt{Stmt: "dir", Value: "RL"}
	case 39: // direction -> direction_lr
		s.Result = &DirStmt{Stmt: "dir", Value: "LR"}

	case 40: // classDefStatement -> CLASSDEF idList stylesOpt separator
		ids, err := stringsAt(s, 2)
		if err != nil {
			return nil, err
		}
		styles, err := stringsAt(s, 3)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		d.AddClass(ids, styles)
	case 41, 42: // idList -> UNICODE_TEXT | STYLE_TEXT
		s.Result = []string{s.S(1)}
	case 43, 44: // idList -> idList COMMA UNICODE_TEXT | idList COMMA STYLE_TEXT
		ids, err := stringsAt(s, 1)
		if err != nil {
			return nil, err
		}
		out := make([]string, len(ids), len(ids)+1)
		copy(out, ids)
		s.Result = append(out, s.S(3))
	case 45: // classStatement -> CLASS idList idList
		ids, err := stringsAt(s, 2)
		if err != nil {
			return nil, err
		}
		classNames, err := stringsAt(s, 3)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		d.SetClass(ids, classNames)
	case 46: // styleStatement -> STYLE idList stylesOpt separator
		ids, err := stringsAt(s, 2)
		if err != nil {
			return nil, err
		}
		styles, err := stringsAt(s, 3)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		d.AddCssStyles(ids, styles)
	case 47: // stylesOpt -> style
		s.Result = []string{s.S(1)}
	case 48: // stylesOpt -> stylesOpt COMMA style
		styles, err := stringsAt(s, 1)
		if err != nil {
			return nil, err
		}
		s.Result = append(styles, s.S(3))
	case 49: // style -> styleComponent
	case 50: // style -> style styleComponent
		s.Result = js.Concat(s.S(1), s.S(2))
	case 51, 52, 53: // separator -> SEMI | NEWLINE | EOF
	case 54, 55, 56, 57: // styleComponent -> STYLE_TEXT | NUM | COLON | BRKT

	case 58: // entityName -> ENTITY_NAME
		s.Result = removeQuotes(s.S(1))
	case 59, 60, 61, 62: // entityName -> UNICODE_TEXT | NUM | DECIMAL_NUM | ENTITY_ONE
		s.Result = s.V(1)

	case 63: // attributes -> attribute
		attribute, ok := s.V(1).(*Attribute)
		if !ok || attribute == nil {
			return nil, valueError(1, "an attribute")
		}
		s.Result = []*Attribute{attribute}
	case 64: // attributes -> attribute attributes
		attribute, ok := s.V(1).(*Attribute)
		if !ok || attribute == nil {
			return nil, valueError(1, "an attribute")
		}
		attribs, err := attributesAt(s, 2)
		if err != nil {
			return nil, err
		}
		s.Result = append(attribs, attribute)
	case 65: // attribute -> attributeType attributeName
		s.Result = &Attribute{Type: s.S(1), Name: s.S(2)}
	case 66: // attribute -> attributeType attributeName attributeKeyTypeList
		keys, err := stringsAt(s, 3)
		if err != nil {
			return nil, err
		}
		s.Result = &Attribute{Type: s.S(1), Name: s.S(2), Keys: keys}
	case 67: // attribute -> attributeType attributeName attributeComment
		s.Result = &Attribute{Type: s.S(1), Name: s.S(2), Comment: s.S(3)}
	case 68: // attribute -> attributeType attributeName attributeKeyTypeList attributeComment
		keys, err := stringsAt(s, 3)
		if err != nil {
			return nil, err
		}
		s.Result = &Attribute{Type: s.S(1), Name: s.S(2), Keys: keys, Comment: s.S(4)}
	case 69: // attributeType -> ATTRIBUTE_WORD
		s.Result = s.V(1)
	case 70: // attributeType -> ATTRIBUTE_WORD ?
		s.Result = js.Concat(s.S(1), s.S(2))
	case 71: // attributeName -> ATTRIBUTE_WORD
		s.Result = s.V(1)
	case 72: // attributeKeyTypeList -> attributeKeyType
		s.Result = []string{s.S(1)}
	case 73: // attributeKeyTypeList -> attributeKeyTypeList , attributeKeyType
		keys, err := stringsAt(s, 1)
		if err != nil {
			return nil, err
		}
		s.Result = append(keys, s.S(3))
	case 74: // attributeKeyType -> ATTRIBUTE_KEY
		s.Result = s.V(1)
	case 75: // attributeComment -> COMMENT
		s.Result = removeQuotes(s.S(1))

	case 76: // relSpec -> cardinality relType cardinality
		s.Result = &RelSpec{CardA: s.S(3), RelType: s.S(2), CardB: s.S(1)}
	case 77: // cardinality -> ZERO_OR_ONE
		s.Result = CardinalityZeroOrOne
	case 78: // cardinality -> ZERO_OR_MORE
		s.Result = CardinalityZeroOrMore
	case 79: // cardinality -> ONE_OR_MORE
		s.Result = CardinalityOneOrMore
	case 80: // cardinality -> ONLY_ONE
		s.Result = CardinalityOnlyOne
	case 81: // cardinality -> MD_PARENT
		s.Result = CardinalityMdParent
	case 82: // relType -> NON_IDENTIFYING
		s.Result = IdentificationNonIdentifying
	case 83: // relType -> IDENTIFYING
		s.Result = IdentificationIdentifying

	case 84, 85: // role -> WORD | ENTITY_NAME
		s.Result = removeQuotes(s.S(1))
	case 86: // role -> UNICODE_TEXT
		s.Result = s.V(1)
	}
	return nil, nil
}
