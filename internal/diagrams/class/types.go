// Ported from mermaid 12.2.0, diagrams/class/classTypes.ts (MIT).

package class

import (
	"strconv"
	"strings"

	"github.com/dlclark/regexp2"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

// TypeError is a JavaScript TypeError that upstream throws in the db or in
// the member parser, for example for an annotation of a class that does not
// exist.
type TypeError struct {
	Message string
}

func (e *TypeError) Error() string { return e.Message }

func undefinedRead(property string) *TypeError {
	return &TypeError{Message: "Cannot read properties of undefined (reading '" + property + "')"}
}

func undefinedSet(property string) *TypeError {
	return &TypeError{Message: "Cannot set properties of undefined (setting '" + property + "')"}
}

// ClassNode is ClassNode: one class of the diagram.
type ClassNode struct {
	ID          string
	Type        string
	Label       string
	Shape       string
	Text        string
	CSSClasses  string
	Methods     []*ClassMember
	Members     []*ClassMember
	Annotations []string
	DomID       string
	Styles      []string

	// The optional properties. A Has field is true when the property is
	// set. Link can be set and undefined: formatUrl returns undefined for
	// an empty URL. LinkDefined tells the two apart.
	Parent        string
	HasParent     bool
	Link          string
	LinkDefined   bool
	LinkTarget    string
	HasLinkTarget bool
	HaveCallback  bool
	Tooltip       string
	HasTooltip    bool
}

// visibilityValues is visibilityValues.
var visibilityValues = []string{"#", "+", "~", "-", ""}

func isVisibility(s string) bool {
	for _, v := range visibilityValues {
		if v == s {
			return true
		}
	}
	return false
}

// ClassMember is ClassMember: a member variable or a method of a class.
type ClassMember struct {
	// ID is the name. A method that the pattern does not match has no id
	// upstream, and the constructor throws.
	ID string
	// MemberType is "method" or "attribute".
	MemberType string
	// Visibility is "#", "+", "~", "-" or "".
	Visibility string
	Text       string
	// Classifier is "*" (abstract), "$" (static) or "".
	Classifier string
	// Parameters and ReturnType are set for a method.
	Parameters string
	ReturnType string
}

// jsEscape returns the escapes of UTF-16 code units for a regular
// expression.
func jsEscape(units ...rune) string {
	var sb strings.Builder
	for _, u := range units {
		sb.WriteString(`\` + "u" + strconv.FormatInt(0x10000+int64(u), 16)[1:])
	}
	return sb.String()
}

// jsSpace is the content of the class \s of a JavaScript regular
// expression, and jsDot is the class of the dot.
var (
	jsSpace = `\t\n\v\f\r ` + jsEscape(0xa0, 0x1680, 0x2000) + "-" + jsEscape(0x200a, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff)
	jsDot   = `[^\n\r` + jsEscape(0x2028, 0x2029) + `]`
)

// methodRegEx is /([#+~-])?(.+)\((.*)\)([\s$*])?(.*)([$*])?/.
var methodRegEx = regexp2.MustCompile(`([#+~-])?(`+jsDot+`+)\((`+jsDot+`*)\)([`+jsSpace+`$*])?(`+jsDot+`*)([$*])?`, regexp2.None)

// isClassifier is /[$*]/.exec(s) as a boolean.
func isClassifier(s string) bool {
	return strings.ContainsAny(s, "$*")
}

// NewClassMember is the constructor of ClassMember. sanitizeText is
// sanitizeText(text, getConfig()).
func NewClassMember(input, memberType string, sanitizeText func(string) string) (*ClassMember, error) {
	m := &ClassMember{MemberType: memberType}
	sanitizedInput := sanitizeText(input)
	if err := m.parseMember(sanitizedInput); err != nil {
		return nil, err
	}
	return m, nil
}

// DisplayDetails is the result of getDisplayDetails.
type DisplayDetails struct {
	DisplayText string
	CSSStyle    string
}

// GetDisplayDetails is getDisplayDetails().
func (m *ClassMember) GetDisplayDetails() DisplayDetails {
	displayText := m.Visibility + sanitize.ParseGenericTypes(m.ID)
	if m.MemberType == "method" {
		displayText += "(" + sanitize.ParseGenericTypes(js.TrimSpace(m.Parameters)) + ")"
		if m.ReturnType != "" {
			displayText += " : " + sanitize.ParseGenericTypes(m.ReturnType)
		}
	}

	displayText = js.TrimSpace(displayText)
	cssStyle := m.ParseClassifier()

	return DisplayDetails{DisplayText: displayText, CSSStyle: cssStyle}
}

// lastUnit is s.substring(s.length - 1): the last UTF-16 code unit, or the
// whole string when it is empty.
func lastUnit(s string) string {
	n := js.UTF16Len(s)
	if n == 0 {
		return ""
	}
	return js.StringSliceFrom(s, n-1)
}

// parseMember is parseMember(input).
func (m *ClassMember) parseMember(input string) error {
	potentialClassifier := ""
	hasID := false

	if m.MemberType == "method" {
		match, err := methodRegEx.FindStringMatch(input)
		if err != nil {
			return err
		}
		if match != nil {
			group := func(i int) string {
				g := match.GroupByNumber(i)
				if g == nil || len(g.Captures) == 0 {
					return ""
				}
				return g.String()
			}
			detectedVisibility := js.TrimSpace(group(1))

			if isVisibility(detectedVisibility) {
				m.Visibility = detectedVisibility
			}

			m.ID, hasID = group(2), true
			m.Parameters = js.TrimSpace(group(3))
			potentialClassifier = js.TrimSpace(group(4))
			m.ReturnType = js.TrimSpace(group(5))

			if potentialClassifier == "" {
				lastChar := lastUnit(m.ReturnType)
				if isClassifier(lastChar) {
					potentialClassifier = lastChar
					m.ReturnType = m.ReturnType[:len(m.ReturnType)-1]
				}
			}
		}
	} else {
		length := js.UTF16Len(input)
		firstChar := js.StringSlice(input, 0, 1)
		lastChar := lastUnit(input)

		if isVisibility(firstChar) {
			m.Visibility = firstChar
		}

		if isClassifier(lastChar) {
			potentialClassifier = lastChar
		}

		start, end := 0, length
		if m.Visibility != "" {
			start = 1
		}
		if potentialClassifier != "" {
			end = length - 1
		}
		// String.prototype.substring swaps its arguments when the first is
		// larger, and clamps them.
		if end < 0 {
			end = 0
		}
		if start > end {
			start, end = end, start
		}
		m.ID, hasID = js.StringSlice(input, start, end), true
	}

	m.Classifier = potentialClassifier
	if !hasID {
		// this.id is undefined.
		return undefinedRead("startsWith")
	}
	// Preserve one space only
	if strings.HasPrefix(m.ID, " ") {
		m.ID = " " + js.TrimSpace(m.ID)
	} else {
		m.ID = js.TrimSpace(m.ID)
	}

	combinedText := ""
	if m.Visibility != "" {
		combinedText = `\` + m.Visibility
	}
	combinedText += sanitize.ParseGenericTypes(m.ID)
	if m.MemberType == "method" {
		combinedText += "(" + sanitize.ParseGenericTypes(m.Parameters) + ")"
		if m.ReturnType != "" {
			combinedText += " : " + sanitize.ParseGenericTypes(m.ReturnType)
		}
	}
	m.Text = strings.ReplaceAll(strings.ReplaceAll(combinedText, "<", "&lt;"), ">", "&gt;")
	if strings.HasPrefix(m.Text, `\&lt;`) {
		m.Text = strings.Replace(m.Text, `\&lt;`, "~", 1)
	}
	return nil
}

// ParseClassifier is parseClassifier(): the style of the classifier.
func (m *ClassMember) ParseClassifier() string {
	return classifierStyle(m.Classifier)
}

func classifierStyle(classifier string) string {
	switch classifier {
	case "*":
		return "font-style:italic;"
	case "$":
		return "text-decoration:underline;"
	default:
		return ""
	}
}

// ClassNote is ClassNote. HasClass is false for a note without a class.
type ClassNote struct {
	ID        string
	Class     string
	HasClass  bool
	Text      string
	Index     int
	Parent    string
	HasParent bool
}

// Relation types and line types. relNone stands for the string 'none' that
// the grammar gives for an end without a marker.
const (
	relNone        = -1
	relAggregation = 0
	relExtension   = 1
	relComposition = 2
	relDependency  = 3
	relLollipop    = 4

	lineLine       = 0
	lineDottedLine = 1
)

// RelationSpec is the relation property of a ClassRelation.
type RelationSpec struct {
	// Type1 and Type2 are relation types, or relNone.
	Type1    int
	Type2    int
	LineType int
}

// ClassRelation is ClassRelation. HasTitle is false when the relation has
// no label.
type ClassRelation struct {
	ID1            string
	ID2            string
	RelationTitle1 string
	RelationTitle2 string
	Title          string
	HasTitle       bool
	Relation       RelationSpec
}

// Interface is Interface: the end of a lollipop relation.
type Interface struct {
	ID      string
	Label   string
	ClassID string
}

// NamespaceNode is NamespaceNode.
type NamespaceNode struct {
	ID       string
	Label    string
	DomID    string
	Classes  *js.Map[string, *ClassNode]
	Notes    *js.Map[string, *ClassNote]
	Children *js.Map[string, *NamespaceNode]
	// Parent is the id of the parent namespace. HasParent is false for a
	// namespace of the top level.
	Parent    string
	HasParent bool
	// Explicit is true for a namespace that the user declared, and false
	// for an ancestor that a dotted name created. The compact
	// (non-hierarchical) rendering mode emits only declared namespaces.
	Explicit bool
}

// StyleClass is StyleClass: a classDef.
type StyleClass struct {
	ID         string
	Styles     []string
	TextStyles []string
}
