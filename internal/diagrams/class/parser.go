// Ported from mermaid 12.2.0, the semantic actions of
// diagrams/class/parser/classDiagram.jison (see
// internal/grammar/classdiagram/actions.js.txt) (MIT).

package class

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/classdiagram"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// parse is parser.parse(text) with yy = db.
func parse(d *DB, text string) error {
	_, err := classdiagram.Parse(text, d, performAction)
	if err == nil {
		err = d.err
	}
	return err
}

// The semantic values of the grammar are:
//
//	string            the text of a token, a name, a note id
//	int               a relation type or a line type
//	*RelationSpec     the relation object { type1, type2, lineType }
//	*ClassRelation    the relation statement object
//	[]string          the members of a class, a class list, the styles
//	*classStatements  the pair [classNames, noteNames] of a namespace body

// classStatements is the value of the classStatements rule: [[...], [...]].
type classStatements struct {
	classes []string
	notes   []string
}

func asStrings(s *jison.ParserState, i int) ([]string, error) {
	a, ok := s.V(i).([]string)
	if !ok {
		return nil, fmt.Errorf("class parser: $%d is not an array", i)
	}
	return a, nil
}

func asStatements(s *jison.ParserState, i int) (*classStatements, error) {
	a, ok := s.V(i).(*classStatements)
	if !ok || a == nil {
		return nil, fmt.Errorf("class parser: $%d is not a pair of arrays", i)
	}
	return a, nil
}

func asRelation(s *jison.ParserState, i int) (*RelationSpec, error) {
	r, ok := s.V(i).(*RelationSpec)
	if !ok || r == nil {
		return nil, fmt.Errorf("class parser: $%d is not a relation", i)
	}
	return r, nil
}

func asInt(s *jison.ParserState, i int) (int, error) {
	n, ok := s.V(i).(int)
	if !ok {
		return 0, fmt.Errorf("class parser: $%d is not a number", i)
	}
	return n, nil
}

// performAction is the generated performAction of the parser.
func performAction(yy any, s *jison.ParserState, production int) (any, error) {
	d, ok := yy.(*DB)
	if !ok {
		return nil, fmt.Errorf("class parser: yy is %T", yy)
	}
	if err := reduce(d, s, production); err != nil {
		return nil, err
	}
	// An exception of a db function stops the parse.
	return nil, d.err
}

func reduce(d *DB, s *jison.ParserState, production int) error {
	switch production {
	case 8: // classLabel -> SQS STR SQE
		s.Result = s.V(2)
	case 9, 10, 13, 15:
		s.Result = s.V(1)
	case 11, 14: // $$=$1+'.'+$3
		s.Result = s.S(1) + "." + s.S(3)
	case 12, 16: // $$=$1+$2
		s.Result = s.S(1) + s.S(2)
	case 17, 18: // $$=$1+'~'+$2+'~'
		s.Result = s.S(1) + "~" + s.S(2) + "~"
	case 19: // statement -> relationStatement
		rel, ok := s.V(1).(*ClassRelation)
		if !ok || rel == nil {
			return fmt.Errorf("class parser: $1 is not a relation statement")
		}
		d.AddRelation(rel)
	case 20: // statement -> relationStatement LABEL
		rel, ok := s.V(1).(*ClassRelation)
		if !ok || rel == nil {
			return fmt.Errorf("class parser: $1 is not a relation statement")
		}
		rel.Title, rel.HasTitle = d.CleanupLabel(s.S(2)), true
		d.AddRelation(rel)
	case 31: // statement -> acc_title acc_title_value
		v := js.TrimSpace(s.S(2))
		s.Result = v
		d.SetAccTitle(v)
	case 32: // statement -> acc_descr acc_descr_value
		v := js.TrimSpace(s.S(2))
		s.Result = v
		d.SetAccDescription(v)
	case 33: // statement -> acc_descr_multiline_value
		v := js.TrimSpace(s.S(1))
		s.Result = v
		d.SetAccDescription(v)
	case 34: // namespaceStatement -> namespaceIdentifier STRUCT_START classStatements STRUCT_STOP
		st, err := asStatements(s, 3)
		if err != nil {
			return err
		}
		d.AddClassesToNamespace(s.S(1), st.classes, st.notes)
		d.PopNamespace()
	case 35: // namespaceStatement -> namespaceIdentifier STRUCT_START NEWLINE classStatements STRUCT_STOP
		st, err := asStatements(s, 4)
		if err != nil {
			return err
		}
		d.AddClassesToNamespace(s.S(1), st.classes, st.notes)
		d.PopNamespace()
	case 36: // namespaceIdentifier -> NAMESPACE namespaceName
		s.Result = d.AddNamespace(s.S(2), "")
	case 37: // namespaceIdentifier -> NAMESPACE namespaceName classLabel
		s.Result = d.AddNamespace(s.S(2), s.S(3))
	case 38, 39: // $$=[[$1], []]
		s.Result = &classStatements{classes: []string{s.S(1)}, notes: []string{}}
	case 40: // $3[0].unshift($1); $$=$3
		st, err := asStatements(s, 3)
		if err != nil {
			return err
		}
		st.classes = append([]string{s.S(1)}, st.classes...)
		s.Result = st
	case 41, 42: // $$=[[], [$1]]
		s.Result = &classStatements{classes: []string{}, notes: []string{s.S(1)}}
	case 43: // $3[1].unshift($1); $$=$3
		st, err := asStatements(s, 3)
		if err != nil {
			return err
		}
		st.notes = append([]string{s.S(1)}, st.notes...)
		s.Result = st
	case 44, 45: // $$=[[], []]
		s.Result = &classStatements{classes: []string{}, notes: []string{}}
	case 46: // $$=$3
		s.Result = s.V(3)
	case 48: // classStatement -> classIdentifier STYLE_SEPARATOR alphaNumToken
		d.SetCssClass(s.S(1), s.S(3))
	case 49: // classStatement -> classIdentifier STRUCT_START members STRUCT_STOP
		members, err := asStrings(s, 3)
		if err != nil {
			return err
		}
		d.AddMembers(s.S(1), members)
	case 51: // classStatement -> classIdentifier STYLE_SEPARATOR alphaNumToken STRUCT_START members STRUCT_STOP
		members, err := asStrings(s, 5)
		if err != nil {
			return err
		}
		d.SetCssClass(s.S(1), s.S(3))
		d.AddMembers(s.S(1), members)
	case 52, 54: // yy.addAnnotation($1, $3)
		d.AddAnnotation(s.S(1), s.S(3))
	case 53: // classStatement -> classIdentifier ANNOTATION_START alphaNumToken ANNOTATION_END STRUCT_START members STRUCT_STOP
		members, err := asStrings(s, 6)
		if err != nil {
			return err
		}
		d.AddAnnotation(s.S(1), s.S(3))
		if d.err != nil {
			return nil
		}
		d.AddMembers(s.S(1), members)
	case 55: // classIdentifier -> CLASS className
		s.Result = s.V(2)
		d.AddClass(s.S(2))
	case 56: // classIdentifier -> CLASS className classLabel
		s.Result = s.V(2)
		d.AddClass(s.S(2))
		d.SetClassLabel(s.S(2), s.S(3))
	case 60: // annotationStatement -> ANNOTATION_START alphaNumToken ANNOTATION_END className
		d.AddAnnotation(s.S(4), s.S(2))
	case 61: // members -> MEMBER
		s.Result = []string{s.S(1)}
	case 62: // members -> MEMBER members: $2.push($1); $$=$2
		members, err := asStrings(s, 2)
		if err != nil {
			return err
		}
		s.Result = append(members, s.S(1))
	case 64: // memberStatement -> className LABEL
		d.AddMember(s.S(1), d.CleanupLabel(s.S(2)))
	case 67: // relationStatement -> className relation className
		rel, err := asRelation(s, 2)
		if err != nil {
			return err
		}
		s.Result = &ClassRelation{ID1: s.S(1), ID2: s.S(3), Relation: *rel, RelationTitle1: "none", RelationTitle2: "none"}
	case 68: // relationStatement -> className STR relation className
		rel, err := asRelation(s, 3)
		if err != nil {
			return err
		}
		s.Result = &ClassRelation{ID1: s.S(1), ID2: s.S(4), Relation: *rel, RelationTitle1: s.S(2), RelationTitle2: "none"}
	case 69: // relationStatement -> className relation STR className
		rel, err := asRelation(s, 2)
		if err != nil {
			return err
		}
		s.Result = &ClassRelation{ID1: s.S(1), ID2: s.S(4), Relation: *rel, RelationTitle1: "none", RelationTitle2: s.S(3)}
	case 70: // relationStatement -> className STR relation STR className
		rel, err := asRelation(s, 3)
		if err != nil {
			return err
		}
		s.Result = &ClassRelation{ID1: s.S(1), ID2: s.S(5), Relation: *rel, RelationTitle1: s.S(2), RelationTitle2: s.S(4)}
	case 71: // noteStatement -> NOTE_FOR className noteText
		s.Result = d.AddNote(s.S(3), s.S(2), true)
	case 72: // noteStatement -> NOTE noteText
		s.Result = d.AddNote(s.S(2), "", false)
	case 73: // classDefStatement -> CLASSDEF classList stylesOpt
		ids, err := asStrings(s, 2)
		if err != nil {
			return err
		}
		style, err := asStrings(s, 3)
		if err != nil {
			return err
		}
		s.Result = s.V(1)
		d.DefineClass(ids, style)
	case 74: // classList -> ALPHA
		s.Result = []string{s.S(1)}
	case 75: // classList -> classList COMMA ALPHA: $$ = $1.concat([$3])
		ids, err := asStrings(s, 1)
		if err != nil {
			return err
		}
		s.Result = append(append([]string{}, ids...), s.S(3))
	case 76:
		d.SetDirection("TB")
	case 77:
		d.SetDirection("BT")
	case 78:
		d.SetDirection("RL")
	case 79:
		d.SetDirection("LR")
	case 80: // relation -> relationType lineType relationType
		t1, err := asInt(s, 1)
		if err != nil {
			return err
		}
		lt, err := asInt(s, 2)
		if err != nil {
			return err
		}
		t2, err := asInt(s, 3)
		if err != nil {
			return err
		}
		s.Result = &RelationSpec{Type1: t1, Type2: t2, LineType: lt}
	case 81: // relation -> lineType relationType
		lt, err := asInt(s, 1)
		if err != nil {
			return err
		}
		t2, err := asInt(s, 2)
		if err != nil {
			return err
		}
		s.Result = &RelationSpec{Type1: relNone, Type2: t2, LineType: lt}
	case 82: // relation -> relationType lineType
		t1, err := asInt(s, 1)
		if err != nil {
			return err
		}
		lt, err := asInt(s, 2)
		if err != nil {
			return err
		}
		s.Result = &RelationSpec{Type1: t1, Type2: relNone, LineType: lt}
	case 83: // relation -> lineType
		lt, err := asInt(s, 1)
		if err != nil {
			return err
		}
		s.Result = &RelationSpec{Type1: relNone, Type2: relNone, LineType: lt}
	case 84:
		s.Result = relAggregation
	case 85:
		s.Result = relExtension
	case 86:
		s.Result = relComposition
	case 87:
		s.Result = relDependency
	case 88:
		s.Result = relLollipop
	case 89:
		s.Result = lineLine
	case 90:
		s.Result = lineDottedLine
	case 91: // clickStatement -> CALLBACK className STR
		s.Result = s.V(1)
		d.SetClickEvent(s.S(2), s.S(3), nil)
	case 92: // clickStatement -> CALLBACK className STR STR
		s.Result = s.V(1)
		d.SetClickEvent(s.S(2), s.S(3), nil)
		d.SetTooltip(s.S(2), s.S(4))
	case 93: // clickStatement -> LINK className STR
		s.Result = s.V(1)
		d.SetLink(s.S(2), s.S(3), nil)
	case 94: // clickStatement -> LINK className STR LINK_TARGET
		s.Result = s.V(1)
		target := s.S(4)
		d.SetLink(s.S(2), s.S(3), &target)
	case 95: // clickStatement -> LINK className STR STR
		s.Result = s.V(1)
		d.SetLink(s.S(2), s.S(3), nil)
		d.SetTooltip(s.S(2), s.S(4))
	case 96: // clickStatement -> LINK className STR STR LINK_TARGET
		s.Result = s.V(1)
		target := s.S(5)
		d.SetLink(s.S(2), s.S(3), &target)
		d.SetTooltip(s.S(2), s.S(4))
	case 97: // clickStatement -> CLICK className CALLBACK_NAME
		s.Result = s.V(1)
		d.SetClickEvent(s.S(2), s.S(3), nil)
	case 98: // clickStatement -> CLICK className CALLBACK_NAME STR
		s.Result = s.V(1)
		d.SetClickEvent(s.S(2), s.S(3), nil)
		d.SetTooltip(s.S(2), s.S(4))
	case 99: // clickStatement -> CLICK className CALLBACK_NAME CALLBACK_ARGS
		s.Result = s.V(1)
		args := s.S(4)
		d.SetClickEvent(s.S(2), s.S(3), &args)
	case 100: // clickStatement -> CLICK className CALLBACK_NAME CALLBACK_ARGS STR
		s.Result = s.V(1)
		args := s.S(4)
		d.SetClickEvent(s.S(2), s.S(3), &args)
		d.SetTooltip(s.S(2), s.S(5))
	case 101: // clickStatement -> CLICK className HREF STR
		s.Result = s.V(1)
		d.SetLink(s.S(2), s.S(4), nil)
	case 102: // clickStatement -> CLICK className HREF STR LINK_TARGET
		s.Result = s.V(1)
		target := s.S(5)
		d.SetLink(s.S(2), s.S(4), &target)
	case 103: // clickStatement -> CLICK className HREF STR STR
		s.Result = s.V(1)
		d.SetLink(s.S(2), s.S(4), nil)
		d.SetTooltip(s.S(2), s.S(5))
	case 104: // clickStatement -> CLICK className HREF STR STR LINK_TARGET
		s.Result = s.V(1)
		target := s.S(6)
		d.SetLink(s.S(2), s.S(4), &target)
		d.SetTooltip(s.S(2), s.S(5))
	case 105: // styleStatement -> STYLE ALPHA stylesOpt
		styles, err := asStrings(s, 3)
		if err != nil {
			return err
		}
		s.Result = s.V(1)
		d.SetCssStyle(s.S(2), styles)
	case 106: // cssClassStatement -> CSSCLASS STR ALPHA
		d.SetCssClass(s.S(2), s.S(3))
	case 107: // stylesOpt -> style
		s.Result = []string{s.S(1)}
	case 108: // stylesOpt -> stylesOpt COMMA style: $1.push($3); $$ = $1
		styles, err := asStrings(s, 1)
		if err != nil {
			return err
		}
		s.Result = append(styles, s.S(3))
	case 110: // style -> style styleComponent
		s.Result = s.S(1) + s.S(2)
	}
	return nil
}
