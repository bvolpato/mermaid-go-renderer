// Ported from mermaid 12.2.0, diagrams/railroad/railroadDb.ts and
// railroadTypes.ts (MIT).

package railroad

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// The values of ASTNode.type.
const (
	typeTerminal    = "terminal"
	typeNonTerminal = "nonterminal"
	typeSequence    = "sequence"
	typeChoice      = "choice"
	typeOptional    = "optional"
	typeRepetition  = "repetition"
	typeSpecial     = "special"
)

// Node is ASTNode of railroadTypes.ts: one node of the expression of a
// rule. Type says which of the other fields the node has.
type Node struct {
	// Type is "terminal", "nonterminal", "sequence", "choice", "optional",
	// "repetition" or "special".
	Type string
	// Value is the text of a terminal.
	Value string
	// Name is the name of a nonterminal.
	Name string
	// Elements are the parts of a sequence.
	Elements []*Node
	// Alternatives are the parts of a choice.
	Alternatives []*Node
	// Element is the content of an optional or of a repetition.
	Element *Node
	// Min and Max are the limits of a repetition: Min is 0 for "*" and 1
	// for "+", and Max is +Inf for no limit.
	Min, Max float64
	// Separator is the optional separator of a repetition. It is nil when
	// the repetition has none.
	Separator *Node
	// Text is the text of a special sequence.
	Text string
}

// Rule is RailroadRule: one rule of the grammar.
type Rule struct {
	Name       string
	Definition *Node
	// Comment is the optional comment. HasComment is false while the
	// property is undefined.
	Comment    string
	HasComment bool
}

// DB is the db of the railroad diagrams: the module state of railroadDb.ts.
// The four diagram types share it.
type DB struct {
	ctx            *render.Context
	diagramTitle   string
	accTitle       string
	accDescription string
	rules          []*Rule
	ruleMap        *js.Map[string, *Rule]
	// titleErr is the error that setDiagramTitle throws for the title of
	// the front matter. Upstream stops there, before it parses the text.
	titleErr error
}

// NewDB returns an empty db.
func NewDB(ctx *render.Context) *DB {
	return &DB{ctx: ctx, rules: []*Rule{}, ruleMap: js.NewMap[string, *Rule]()}
}

// sanitizeText is sanitizeText(text): commonSanitizeText(text, getConfig()).
// The error is the error that upstream throws for a config that DOMPurify
// rejects.
func (d *DB) sanitizeText(text string) (string, error) {
	return d.ctx.Sanitizer.SanitizeTextErr(text, d.ctx.Config)
}

// sanitizeAstNode is sanitizeAstNode(node): a copy of the node with every
// text sanitized.
func (d *DB) sanitizeAstNode(node *Node) (*Node, error) {
	out := *node
	var err error
	switch node.Type {
	case typeTerminal:
		out.Value, err = d.sanitizeText(node.Value)
	case typeNonTerminal:
		out.Name, err = d.sanitizeText(node.Name)
	case typeSequence:
		out.Elements, err = d.sanitizeAstNodes(node.Elements)
	case typeChoice:
		out.Alternatives, err = d.sanitizeAstNodes(node.Alternatives)
	case typeOptional:
		out.Element, err = d.sanitizeAstNode(node.Element)
	case typeRepetition:
		if out.Element, err = d.sanitizeAstNode(node.Element); err != nil {
			return nil, err
		}
		if node.Separator != nil {
			out.Separator, err = d.sanitizeAstNode(node.Separator)
		}
	case typeSpecial:
		out.Text, err = d.sanitizeText(node.Text)
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// sanitizeAstNodes is nodes.map(sanitizeAstNode).
func (d *DB) sanitizeAstNodes(nodes []*Node) ([]*Node, error) {
	out := make([]*Node, len(nodes))
	for i, node := range nodes {
		sanitized, err := d.sanitizeAstNode(node)
		if err != nil {
			return nil, err
		}
		out[i] = sanitized
	}
	return out, nil
}

// Clear is clear(): it clears all diagram state.
func (d *DB) Clear() {
	d.diagramTitle = ""
	d.accTitle = ""
	d.accDescription = ""
	d.rules = d.rules[:0]
	d.ruleMap.Clear()
}

// SetTitle is setTitle(text).
func (d *DB) SetTitle(text string) error {
	title, err := d.sanitizeText(text)
	if err != nil {
		return err
	}
	d.diagramTitle = title
	return nil
}

// GetTitle is getTitle().
func (d *DB) GetTitle() string {
	return d.diagramTitle
}

// AddRule is addRule(rule): it adds a new rule to the diagram.
func (d *DB) AddRule(rule *Rule) error {
	name, err := d.sanitizeText(rule.Name)
	if err != nil {
		return err
	}
	definition, err := d.sanitizeAstNode(rule.Definition)
	if err != nil {
		return err
	}
	sanitizedRule := &Rule{Name: name, Definition: definition}
	// comment: rule.comment ? sanitizeText(rule.comment) : undefined
	if rule.HasComment && rule.Comment != "" {
		comment, err := d.sanitizeText(rule.Comment)
		if err != nil {
			return err
		}
		sanitizedRule.Comment = comment
		sanitizedRule.HasComment = true
	}

	// A rule with a name that exists stays in the list. The map keeps the
	// last rule of a name.
	d.rules = append(d.rules, sanitizedRule)
	d.ruleMap.Set(sanitizedRule.Name, sanitizedRule)
	return nil
}

// GetRules is getRules(): all rules, in the order of the text.
func (d *DB) GetRules() []*Rule {
	return d.rules
}

// GetRule is getRule(name): the last rule with the name, or nil.
func (d *DB) GetRule(name string) *Rule {
	return d.ruleMap.Get(name)
}

// SetAccTitle is setAccTitle(text): the accessibility title.
func (d *DB) SetAccTitle(text string) error {
	title, err := d.sanitizeText(text)
	if err != nil {
		return err
	}
	// .replace(/^\s+/g, '')
	d.accTitle = strings.TrimLeftFunc(title, js.IsWhiteSpace)
	return nil
}

// GetAccTitle is getAccTitle().
func (d *DB) GetAccTitle() string {
	return d.accTitle
}

// SetAccDescription is setAccDescription(text): the accessibility
// description.
func (d *DB) SetAccDescription(text string) error {
	description, err := d.sanitizeText(text)
	if err != nil {
		return err
	}
	d.accDescription = replaceLineBreakSpace(description)
	return nil
}

// GetAccDescription is getAccDescription().
func (d *DB) GetAccDescription() string {
	return d.accDescription
}

// SetDiagramTitle is setDiagramTitle, which is setTitle. The render pipeline
// calls it with the title of the front matter. The parser of upstream clears
// the db before it reads the text, so this title never stays. An error of
// the sanitizer is kept: the parse returns it.
func (d *DB) SetDiagramTitle(text string) {
	if err := d.SetTitle(text); err != nil && d.titleErr == nil {
		d.titleErr = err
	}
}

// GetDiagramTitle is getDiagramTitle, which is getTitle.
func (d *DB) GetDiagramTitle() string {
	return d.GetTitle()
}

// replaceLineBreakSpace is s.replace(/\n\s+/g, '\n'): each line break with
// the white space that follows it becomes one line break.
func replaceLineBreakSpace(s string) string {
	if !strings.Contains(s, "\n") {
		return s
	}
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		rest := strings.TrimLeftFunc(s[i+1:], js.IsWhiteSpace)
		if len(rest) == len(s)-i-1 {
			// No white space follows: the line break stays as it is.
			b.WriteString(s[:i+1])
		} else {
			b.WriteString(s[:i])
			b.WriteByte('\n')
		}
		s = rest
	}
}
