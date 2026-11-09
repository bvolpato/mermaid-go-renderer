// Ported from mermaid 12.2.0, the semantic actions of
// diagrams/requirement/parser/requirementDiagram.jison (see
// internal/grammar/requirementdiagram/actions.js.txt) (MIT).

package requirement

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/requirementdiagram"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// parse is parser.parse(text) with yy = db.
func parse(d *DB, text string) error {
	_, err := requirementdiagram.Parse(text, d, performAction)
	return err
}

// The semantic values of the grammar are:
//
//	string    the text of a token, a name, an id, a style, the value of an
//	          accessibility statement, a requirement type, a risk level, a
//	          verification method, a relationship type
//	[]string  a list of ids (idList) or of styles (stylesOpt)
//	[]any     the value of an empty diagram

func valueError(i int, want string) error {
	return fmt.Errorf("requirement parser: $%d is not %s", i, want)
}

func stringsAt(s *jison.ParserState, i int) ([]string, error) {
	v, ok := s.V(i).([]string)
	if !ok {
		return nil, valueError(i, "a list of strings")
	}
	return v, nil
}

// concat is $1.concat([$3]): a new list.
func concat(list []string, v string) []string {
	out := make([]string, len(list), len(list)+1)
	copy(out, list)
	return append(out, v)
}

// performAction is the generated performAction of the parser: the semantic
// action of one production.
func performAction(yy any, s *jison.ParserState, production int) (any, error) {
	d, ok := yy.(*DB)
	if !ok {
		return nil, fmt.Errorf("requirement parser: yy is %T", yy)
	}
	switch production {
	case 4: // directive -> acc_title acc_title_value
		value := js.TrimSpace(s.S(2))
		s.Result = value
		d.SetAccTitle(value)
	case 5: // directive -> acc_descr acc_descr_value
		value := js.TrimSpace(s.S(2))
		s.Result = value
		d.SetAccDescription(value)
	case 6: // directive -> acc_descr_multiline_value
		value := js.TrimSpace(s.S(1))
		s.Result = value
		d.SetAccDescription(value)
	case 7: // diagram -> (empty)
		s.Result = []any{}

	case 17: // direction -> direction_tb
		d.SetDirection("TB")
	case 18: // direction -> direction_bt
		d.SetDirection("BT")
	case 19: // direction -> direction_rl
		d.SetDirection("RL")
	case 20: // direction -> direction_lr
		d.SetDirection("LR")

	case 21: // requirementDef -> requirementType requirementName STRUCT_START NEWLINE requirementBody
		d.AddRequirement(s.S(2), s.S(1))
	case 22: // requirementDef -> requirementType requirementName STYLE_SEPARATOR idList STRUCT_START NEWLINE requirementBody
		ids, err := stringsAt(s, 4)
		if err != nil {
			return nil, err
		}
		d.AddRequirement(s.S(2), s.S(1))
		d.SetClass([]string{s.S(2)}, ids)
	case 23: // requirementBody -> ID COLONSEP id NEWLINE requirementBody
		d.SetNewReqId(s.S(3))
	case 24: // requirementBody -> TEXT COLONSEP text NEWLINE requirementBody
		d.SetNewReqText(s.S(3))
	case 25: // requirementBody -> RISK COLONSEP riskLevel NEWLINE requirementBody
		d.SetNewReqRisk(s.S(3))
	case 26: // requirementBody -> VERIFYMTHD COLONSEP verifyType NEWLINE requirementBody
		d.SetNewReqVerifyMethod(s.S(3))

	case 29: // requirementType -> REQUIREMENT
		s.Result = TypeRequirement
	case 30: // requirementType -> FUNCTIONAL_REQUIREMENT
		s.Result = TypeFunctionalRequirement
	case 31: // requirementType -> INTERFACE_REQUIREMENT
		s.Result = TypeInterfaceRequirement
	case 32: // requirementType -> PERFORMANCE_REQUIREMENT
		s.Result = TypePerformanceRequirement
	case 33: // requirementType -> PHYSICAL_REQUIREMENT
		s.Result = TypePhysicalRequirement
	case 34: // requirementType -> DESIGN_CONSTRAINT
		s.Result = TypeDesignConstraint

	case 35: // riskLevel -> LOW_RISK
		s.Result = RiskLow
	case 36: // riskLevel -> MED_RISK
		s.Result = RiskMedium
	case 37: // riskLevel -> HIGH_RISK
		s.Result = RiskHigh

	case 38: // verifyType -> VERIFY_ANALYSIS
		s.Result = VerifyAnalysis
	case 39: // verifyType -> VERIFY_DEMONSTRATION
		s.Result = VerifyDemonstration
	case 40: // verifyType -> VERIFY_INSPECTION
		s.Result = VerifyInspection
	case 41: // verifyType -> VERIFY_TEST
		s.Result = VerifyTest

	case 42: // elementDef -> ELEMENT elementName STRUCT_START NEWLINE elementBody
		d.AddElement(s.S(2))
	case 43: // elementDef -> ELEMENT elementName STYLE_SEPARATOR idList STRUCT_START NEWLINE elementBody
		ids, err := stringsAt(s, 4)
		if err != nil {
			return nil, err
		}
		d.AddElement(s.S(2))
		d.SetClass([]string{s.S(2)}, ids)
	case 44: // elementBody -> TYPE COLONSEP type NEWLINE elementBody
		d.SetNewElementType(s.S(3))
	case 45: // elementBody -> DOCREF COLONSEP ref NEWLINE elementBody
		d.SetNewElementDocRef(s.S(3))

	case 48: // relationshipDef -> id END_ARROW_L relationship LINE id
		d.AddRelationship(s.S(3), s.S(5), s.S(1))
	case 49: // relationshipDef -> id LINE relationship END_ARROW_R id
		d.AddRelationship(s.S(3), s.S(1), s.S(5))

	case 50: // relationship -> CONTAINS
		s.Result = RelContains
	case 51: // relationship -> COPIES
		s.Result = RelCopies
	case 52: // relationship -> DERIVES
		s.Result = RelDerives
	case 53: // relationship -> SATISFIES
		s.Result = RelSatisfies
	case 54: // relationship -> VERIFIES
		s.Result = RelVerifies
	case 55: // relationship -> REFINES
		s.Result = RelRefines
	case 56: // relationship -> TRACES
		s.Result = RelTraces

	case 57: // classDefStatement -> CLASSDEF idList stylesOpt
		ids, err := stringsAt(s, 2)
		if err != nil {
			return nil, err
		}
		styles, err := stringsAt(s, 3)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		d.DefineClass(ids, styles)
	case 58: // classStatement -> CLASS idList idList
		ids, err := stringsAt(s, 2)
		if err != nil {
			return nil, err
		}
		classNames, err := stringsAt(s, 3)
		if err != nil {
			return nil, err
		}
		d.SetClass(ids, classNames)
	case 59: // classStatement -> id STYLE_SEPARATOR idList
		classNames, err := stringsAt(s, 3)
		if err != nil {
			return nil, err
		}
		d.SetClass([]string{s.S(1)}, classNames)

	case 60, 62: // idList -> ALPHA | id
		s.Result = []string{s.S(1)}
	case 61, 63: // idList -> idList COMMA ALPHA | idList COMMA id
		list, err := stringsAt(s, 1)
		if err != nil {
			return nil, err
		}
		s.Result = concat(list, s.S(3))

	case 64: // styleStatement -> STYLE idList stylesOpt
		ids, err := stringsAt(s, 2)
		if err != nil {
			return nil, err
		}
		styles, err := stringsAt(s, 3)
		if err != nil {
			return nil, err
		}
		s.Result = s.V(1)
		d.SetCssStyle(ids, styles)
	case 65: // stylesOpt -> style
		s.Result = []string{s.S(1)}
	case 66: // stylesOpt -> stylesOpt COMMA style
		list, err := stringsAt(s, 1)
		if err != nil {
			return nil, err
		}
		s.Result = append(list, s.S(3))
	case 68: // style -> style styleComponent
		s.Result = js.Concat(s.S(1), s.S(2))
	}
	return nil, nil
}
