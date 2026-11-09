// Ported from mermaid 12.2.0, diagrams/railroad/parser/railroadParser.ts,
// ebnfParser.ts, abnfParser.ts and pegParser.ts: each parser reads the text
// with the grammar of its syntax and fills the db with the rules (MIT). Also
// ported from diagrams/common/populateCommonDb.ts.

package railroad

import (
	"math"
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/langium"
)

// populateDb is the part of populateDb that the four parsers share:
//
//	populateCommonDb(ast, db);
//	if (ast.title) {
//	  db.setTitle(ast.title);
//	}
//
// populateCommonDb gives each property that is not empty to the db.
func populateDb(db *DB, common *langium.Common) error {
	if common.AccDescr != nil && *common.AccDescr != "" {
		if err := db.SetAccDescription(*common.AccDescr); err != nil {
			return err
		}
	}
	if common.AccTitle != nil && *common.AccTitle != "" {
		if err := db.SetAccTitle(*common.AccTitle); err != nil {
			return err
		}
	}
	if common.Title != nil && *common.Title != "" {
		// db.setDiagramTitle, which is setTitle.
		if err := db.SetTitle(*common.Title); err != nil {
			return err
		}
	}

	// The parser sets the title a second time, with the same text.
	if common.Title != nil && *common.Title != "" {
		if err := db.SetTitle(*common.Title); err != nil {
			return err
		}
	}
	return nil
}

// start is the first statements of parser.parse of each parser: db.clear().
// The title of the front matter is set before the parse, so an error of
// that call stops the diagram here.
func start(db *DB) error {
	if db.titleErr != nil {
		return db.titleErr
	}
	db.Clear()
	return nil
}

// ---------------------------------------------------------------------------
// railroadParser.ts

// parseRailroadText is parser.parse(input) of railroadParser.ts.
func parseRailroadText(db *DB, input string) error {
	if err := start(db); err != nil {
		return err
	}

	ast, err := parseRailroad(input)
	if err != nil {
		return err
	}

	// populateDb(ast)
	if err := populateDb(db, &ast.Common); err != nil {
		return err
	}
	for _, rule := range ast.rules {
		// transformRule(rule)
		if err := db.AddRule(&Rule{Name: rule.name, Definition: transformExpression(rule.definition)}); err != nil {
			return err
		}
	}
	return nil
}

// transformExpression is transformExpression(expr) of railroadParser.ts.
func transformExpression(expr *railroadExpr) *Node {
	switch expr.typ {
	case "RailroadTerminalExpr":
		return &Node{Type: typeTerminal, Value: expr.value}
	case "RailroadNonTerminalExpr":
		return &Node{Type: typeNonTerminal, Name: expr.name}
	case "RailroadSpecialExpr":
		return &Node{Type: typeSpecial, Text: expr.text}
	case "RailroadSequenceExpr":
		elements := make([]*Node, len(expr.elements))
		for i, e := range expr.elements {
			elements[i] = transformExpression(e)
		}
		if len(elements) == 1 {
			return elements[0]
		}
		return &Node{Type: typeSequence, Elements: elements}
	case "RailroadChoiceExpr":
		alternatives := make([]*Node, len(expr.alternatives))
		for i, e := range expr.alternatives {
			alternatives[i] = transformExpression(e)
		}
		if len(alternatives) == 1 {
			return alternatives[0]
		}
		return &Node{Type: typeChoice, Alternatives: alternatives}
	case "RailroadOptionalExpr":
		return &Node{Type: typeOptional, Element: transformExpression(expr.element)}
	case "RailroadOneOrMoreExpr":
		return &Node{Type: typeRepetition, Element: transformExpression(expr.element), Min: 1, Max: math.Inf(1)}
	default: // RailroadZeroOrMoreExpr
		return &Node{Type: typeRepetition, Element: transformExpression(expr.element), Min: 0, Max: math.Inf(1)}
	}
}

// ---------------------------------------------------------------------------
// ebnfParser.ts

// parseEbnfText is parser.parse(input) of ebnfParser.ts.
func parseEbnfText(db *DB, input string) error {
	if err := start(db); err != nil {
		return err
	}

	ast, err := parseEbnf(input)
	if err != nil {
		return err
	}

	// populateDb(ast)
	if err := populateDb(db, &ast.Common); err != nil {
		return err
	}
	for _, rule := range ast.rules {
		// transformRule(rule)
		if err := db.AddRule(&Rule{Name: rule.name, Definition: ebnfTransformChoice(rule.definition)}); err != nil {
			return err
		}
	}
	return nil
}

// ebnfTransformChoice is transformChoice(choice) of ebnfParser.ts.
func ebnfTransformChoice(choice *ebnfChoice) *Node {
	alternatives := make([]*Node, len(choice.alternatives))
	for i, sequence := range choice.alternatives {
		alternatives[i] = ebnfTransformSequence(sequence)
	}

	if len(alternatives) == 1 {
		return alternatives[0]
	}

	return &Node{Type: typeChoice, Alternatives: alternatives}
}

// ebnfTransformSequence is transformSequence(sequence) of ebnfParser.ts.
func ebnfTransformSequence(sequence *ebnfSequence) *Node {
	elements := make([]*Node, len(sequence.elements))
	for i, term := range sequence.elements {
		elements[i] = ebnfTransformTerm(term)
	}

	if len(elements) == 1 {
		return elements[0]
	}

	return &Node{Type: typeSequence, Elements: elements}
}

// ebnfTransformPrimary is transformPrimary(primary) of ebnfParser.ts.
func ebnfTransformPrimary(primary *ebnfPrimary) *Node {
	switch primary.typ {
	case "EbnfTerminal":
		return &Node{Type: typeTerminal, Value: primary.value}
	case "EbnfNonTerminal":
		return &Node{Type: typeNonTerminal, Name: primary.name}
	case "EbnfSpecial":
		return &Node{Type: typeSpecial, Text: primary.text}
	case "EbnfGroup":
		return ebnfTransformChoice(primary.element)
	case "EbnfOptional":
		return &Node{Type: typeOptional, Element: ebnfTransformChoice(primary.element)}
	default: // EbnfRepetition
		return &Node{Type: typeRepetition, Element: ebnfTransformChoice(primary.element), Min: 0, Max: math.Inf(1)}
	}
}

// ebnfTransformPostfix is transformPostfix(node, postfix) of ebnfParser.ts.
func ebnfTransformPostfix(node *Node, postfix *ebnfPostfix) *Node {
	switch postfix.typ {
	case "EbnfOptionalPostfix":
		return &Node{Type: typeOptional, Element: node}
	case "EbnfZeroOrMorePostfix":
		return &Node{Type: typeRepetition, Element: node, Min: 0, Max: math.Inf(1)}
	case "EbnfOneOrMorePostfix":
		return &Node{Type: typeRepetition, Element: node, Min: 1, Max: math.Inf(1)}
	default: // EbnfExceptionPostfix
		return &Node{Type: typeSequence, Elements: []*Node{
			node,
			{Type: typeTerminal, Value: "-"},
			ebnfTransformPrimary(postfix.except),
		}}
	}
}

// ebnfTransformTerm is transformTerm(term) of ebnfParser.ts.
func ebnfTransformTerm(term *ebnfTerm) *Node {
	currentNode := ebnfTransformPrimary(term.base)
	for _, postfix := range term.postfixes {
		currentNode = ebnfTransformPostfix(currentNode, postfix)
	}
	return currentNode
}

// ---------------------------------------------------------------------------
// abnfParser.ts

// parseAbnfText is parser.parse(input) of abnfParser.ts.
func parseAbnfText(db *DB, input string) error {
	if err := start(db); err != nil {
		return err
	}

	ast, err := parseAbnf(input)
	if err != nil {
		return err
	}

	// populateDb(ast)
	if err := populateDb(db, &ast.Common); err != nil {
		return err
	}
	for _, rule := range ast.rules {
		// transformRule(rule)
		if err := db.AddRule(&Rule{Name: rule.name, Definition: abnfTransformAlternation(rule.definition)}); err != nil {
			return err
		}
	}
	return nil
}

// abnfTransformAlternation is transformAlternation(alt) of abnfParser.ts.
func abnfTransformAlternation(alt *abnfAlternation) *Node {
	alternatives := make([]*Node, len(alt.alternatives))
	for i, concat := range alt.alternatives {
		alternatives[i] = abnfTransformConcatenation(concat)
	}

	if len(alternatives) == 1 {
		return alternatives[0]
	}

	return &Node{Type: typeChoice, Alternatives: alternatives}
}

// abnfTransformConcatenation is transformConcatenation(concat) of
// abnfParser.ts.
func abnfTransformConcatenation(concat *abnfConcatenation) *Node {
	elements := make([]*Node, len(concat.elements))
	for i, element := range concat.elements {
		elements[i] = abnfTransformElement(element)
	}

	if len(elements) == 1 {
		return elements[0]
	}

	return &Node{Type: typeSequence, Elements: elements}
}

// parseInt10 is parseInt(s, 10) for the digits of a repeat token. V8 rounds
// a decimal integer of any length correctly.
func parseInt10(s string) float64 {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return js.ParseInt(s, 10)
		}
	}
	if s == "" {
		return math.NaN()
	}
	// The error is a range error for a number that is too large: the result
	// is then +Inf, as in JavaScript.
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// parseRepeat is parseRepeat(repeat) of abnfParser.ts.
func parseRepeat(repeat string) (min, max float64) {
	if strings.Contains(repeat, "*") {
		// const [minStr, maxStr] = repeat.split('*');
		minStr, maxStr, _ := strings.Cut(repeat, "*")
		if i := strings.Index(maxStr, "*"); i >= 0 {
			maxStr = maxStr[:i]
		}
		min = 0
		if minStr != "" {
			min = parseInt10(minStr)
		}
		max = math.Inf(1)
		if maxStr != "" {
			max = parseInt10(maxStr)
		}
		return min, max
	}
	// Exact repeat: "3" means exactly 3
	exact := parseInt10(repeat)
	return exact, exact
}

// abnfTransformElement is transformElement(element) of abnfParser.ts.
func abnfTransformElement(element *abnfElement) *Node {
	inner := abnfTransformPrimary(element.primary)

	// if (!element.repeat)
	if !element.hasRepeat || element.repeat == "" {
		return inner
	}

	min, max := parseRepeat(element.repeat)

	if min == 0 && max == 1 {
		return &Node{Type: typeOptional, Element: inner}
	}

	return &Node{Type: typeRepetition, Element: inner, Min: min, Max: max}
}

// abnfTransformPrimary is transformPrimary(primary) of abnfParser.ts.
func abnfTransformPrimary(primary *abnfPrimary) *Node {
	switch primary.typ {
	case "AbnfStringLiteral":
		return &Node{Type: typeTerminal, Value: primary.value}
	case "AbnfNumVal":
		return &Node{Type: typeTerminal, Value: primary.value}
	case "AbnfRuleName":
		return &Node{Type: typeNonTerminal, Name: primary.name}
	case "AbnfGroup":
		return abnfTransformAlternation(primary.element)
	default: // AbnfOptionalGroup
		return &Node{Type: typeOptional, Element: abnfTransformAlternation(primary.element)}
	}
}

// ---------------------------------------------------------------------------
// pegParser.ts

// parsePegText is parser.parse(input) of pegParser.ts.
func parsePegText(db *DB, input string) error {
	if err := start(db); err != nil {
		return err
	}

	ast, err := parsePeg(input)
	if err != nil {
		return err
	}

	// populateDb(ast)
	if err := populateDb(db, &ast.Common); err != nil {
		return err
	}
	for _, rule := range ast.rules {
		// transformRule(rule)
		if err := db.AddRule(&Rule{Name: rule.name, Definition: pegTransformOrderedChoice(rule.definition)}); err != nil {
			return err
		}
	}
	return nil
}

// pegTransformOrderedChoice is transformOrderedChoice(choice) of
// pegParser.ts.
func pegTransformOrderedChoice(choice *pegOrderedChoice) *Node {
	alternatives := make([]*Node, len(choice.alternatives))
	for i, sequence := range choice.alternatives {
		alternatives[i] = pegTransformSequence(sequence)
	}

	if len(alternatives) == 1 {
		return alternatives[0]
	}

	return &Node{Type: typeChoice, Alternatives: alternatives}
}

// pegTransformSequence is transformSequence(sequence) of pegParser.ts.
func pegTransformSequence(sequence *pegSequence) *Node {
	elements := make([]*Node, len(sequence.elements))
	for i, prefix := range sequence.elements {
		elements[i] = pegTransformPrefix(prefix)
	}

	if len(elements) == 1 {
		return elements[0]
	}

	return &Node{Type: typeSequence, Elements: elements}
}

// pegTransformPrefix is transformPrefix(prefix) of pegParser.ts.
func pegTransformPrefix(prefix *pegPrefix) *Node {
	inner := pegTransformSuffix(prefix.suffix)

	if prefix.operator == "" {
		return inner
	}

	// & and ! are lookahead operators - render as special nodes
	label := "!" + nodeToLabel(inner)
	if prefix.operator == "&" {
		label = "&" + nodeToLabel(inner)
	}
	return &Node{Type: typeSpecial, Text: label}
}

// nodeToLabel is nodeToLabel(node) of pegParser.ts.
func nodeToLabel(node *Node) string {
	switch node.Type {
	case typeTerminal:
		return `"` + node.Value + `"`
	case typeNonTerminal:
		return node.Name
	case typeSpecial:
		return node.Text
	default:
		return "(...)"
	}
}

// pegTransformSuffix is transformSuffix(suffix) of pegParser.ts.
func pegTransformSuffix(suffix *pegSuffix) *Node {
	inner := pegTransformPrimary(suffix.primary)

	switch suffix.operator {
	case "?":
		return &Node{Type: typeOptional, Element: inner}
	case "*":
		return &Node{Type: typeRepetition, Element: inner, Min: 0, Max: math.Inf(1)}
	case "+":
		return &Node{Type: typeRepetition, Element: inner, Min: 1, Max: math.Inf(1)}
	default:
		// The suffix has no operator.
		return inner
	}
}

// pegTransformPrimary is transformPrimary(primary) of pegParser.ts.
func pegTransformPrimary(primary *pegPrimary) *Node {
	switch primary.typ {
	case "PegLiteral":
		return &Node{Type: typeTerminal, Value: primary.value}
	case "PegIdentifier":
		return &Node{Type: typeNonTerminal, Name: primary.name}
	case "PegGroup":
		return pegTransformOrderedChoice(primary.element)
	default: // PegAny
		return &Node{Type: typeSpecial, Text: primary.dot}
	}
}
