// Package sanitize ports the text sanitizing helpers of mermaid 12.2.0.
//
// Ported from mermaid 12.2.0, packages/mermaid/src/diagrams/common/common.ts,
// with evaluate and getEffectiveHtmlLabels from packages/mermaid/src/config.ts
// (MIT). The KaTeX rendering functions of common.ts are in internal/ru.
//
// common.ts keeps one DOMPurify instance and one flag at module level: the
// hooks of setupDompurifyHooks are installed by the first removeScript call
// and then stay for every later sanitize call of the page, including the one
// for the final SVG in mermaidAPI.ts. A Sanitizer holds that state. Each
// render must use its own Sanitizer, and must sanitize the final SVG with
// Sanitizer.Purifier.
package sanitize

import (
	"math"
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/purify"
)

// jsSpace is the character class \s of a JavaScript regular expression, for
// use inside a Go character class. The \s of Go matches ASCII only.
const jsSpace = `\t\n\x0B\f\r \x{A0}\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}\x{FEFF}`

// jsLineTerminator is the set of characters that the dot of a JavaScript
// regular expression does not match, for use inside a Go character class.
const jsLineTerminator = `\n\r\x{2028}\x{2029}`

// LineBreakRegex is lineBreakRegex: /<\/?br\s*\/?>/gi. The optional leading
// slash accepts the malformed `</br>`.
var LineBreakRegex = regexp.MustCompile(`</?[bB][rR][` + jsSpace + `]*/?>`)

// KatexRegex is katexRegex: /\$\$(.*?)\$\$/g.
var KatexRegex = regexp.MustCompile(`\$\$([^` + jsLineTerminator + `]*?)\$\$`)

// GetRows gets the rows of lines in a string (getRows).
func GetRows(s string) []string {
	if s == "" {
		return []string{""}
	}
	str := strings.ReplaceAll(BreakToPlaceholder(s), `\n`, "#br#")
	return strings.Split(str, "#br#")
}

// Sanitizer is the module state of common.ts: the DOMPurify instance and the
// flag that says whether the hooks are set up. It is not safe for concurrent
// use.
type Sanitizer struct {
	purifier *purify.Purifier
	setup    bool
}

// New returns the state of a page that has made no sanitize call yet.
func New() *Sanitizer {
	return &Sanitizer{purifier: purify.New()}
}

// Purifier is the DOMPurify instance. mermaidAPI.ts sanitizes the final SVG
// with the same instance, so the hooks apply there too.
func (s *Sanitizer) Purifier() *purify.Purifier { return s.purifier }

// HooksSetUp reports whether setupDompurifyHooks has run.
func (s *Sanitizer) HooksSetUp() bool { return s.setup }

// setupDompurifyHooksIfNotSetup is setupDompurifyHooksIfNotSetup.
func (s *Sanitizer) setupDompurifyHooksIfNotSetup() {
	if !s.setup {
		SetupDompurifyHooks(s.purifier)
		s.setup = true
	}
}

// SetupDompurifyHooks is setupDompurifyHooks: it installs the two hooks that
// keep the target attribute of a link and add rel="noopener" for _blank.
func SetupDompurifyHooks(p *purify.Purifier) {
	const temporaryAttribute = "data-temp-href-target"
	// node.tagName === 'A' is true for an HTML a element only. The tagName of
	// an SVG a element is in lowercase.
	isAnchor := func(node *dom.Node) bool {
		return node.Type == dom.ElementNode && node.NS == dom.NSHTML && node.Tag == "a"
	}
	p.AddHook(purify.BeforeSanitizeAttributes, func(node *dom.Node, _ *purify.HookEvent, _ *purify.Config) {
		if isAnchor(node) && node.HasAttribute("target") {
			target, _ := node.GetAttribute("target")
			_ = node.SetAttribute(temporaryAttribute, target)
		}
	})
	p.AddHook(purify.AfterSanitizeAttributes, func(node *dom.Node, _ *purify.HookEvent, _ *purify.Config) {
		if isAnchor(node) && node.HasAttribute(temporaryAttribute) {
			target, _ := node.GetAttribute(temporaryAttribute)
			_ = node.SetAttribute("target", target)
			node.RemoveAttribute(temporaryAttribute)
			if target, _ := node.GetAttribute("target"); target == "_blank" {
				_ = node.SetAttribute("rel", "noopener")
			}
		}
	})
}

// get reads a property of the config. A nil config has no properties.
func get(config *js.Obj[any], key string) any {
	if config == nil {
		return nil
	}
	return config.Get(key)
}

// RemoveScript removes script tags from a text (removeScript).
func (s *Sanitizer) RemoveScript(txt string) string {
	s.setupDompurifyHooksIfNotSetup()
	return s.purifier.Sanitize(txt, purify.Config{})
}

// SanitizeMore is sanitizeMore.
func (s *Sanitizer) SanitizeMore(text string, config *js.Obj[any]) string {
	if GetEffectiveHtmlLabels(config) {
		level := get(config, "securityLevel")
		if level == "antiscript" || level == "strict" || level == "sandbox" {
			text = s.RemoveScript(text)
		} else if level != "loose" {
			text = BreakToPlaceholder(text)
			text = strings.ReplaceAll(strings.ReplaceAll(text, "<", "&lt;"), ">", "&gt;")
			text = strings.ReplaceAll(text, "=", "&equals;")
			text = PlaceholderToBreak(text)
		}
	}
	return text
}

// SanitizeText is sanitizeText. When upstream throws (an invalid
// dompurifyConfig), it returns the empty string. SanitizeTextErr reports the
// error.
func (s *Sanitizer) SanitizeText(text string, config *js.Obj[any]) string {
	out, err := s.SanitizeTextErr(text, config)
	if err != nil {
		return ""
	}
	return out
}

// SanitizeTextErr is sanitizeText with the error that upstream throws.
func (s *Sanitizer) SanitizeTextErr(text string, config *js.Obj[any]) (string, error) {
	if text == "" {
		return text, nil
	}
	if dompurifyConfig := get(config, "dompurifyConfig"); js.Truthy(dompurifyConfig) {
		cfg, err := purify.ConfigFromJS(dompurifyConfig)
		if err != nil {
			return "", err
		}
		text = s.SanitizeMore(text, config)
		if cfg.ReturnDOM || cfg.ReturnDOMFragment {
			// DOMPurify returns a node, and toString() of a node is its
			// class string.
			node, err := s.purifier.SanitizeDOM(text, cfg)
			switch {
			case err != nil:
				return "", err
			case node == nil:
				return "", &purify.TypeError{Message: "Cannot read properties of null (reading 'toString')"}
			case cfg.ReturnDOMFragment:
				return "[object DocumentFragment]", nil
			case cfg.WholeDocument:
				return "[object HTMLHtmlElement]", nil
			}
			return "[object HTMLBodyElement]", nil
		}
		return s.purifier.SanitizeErr(text, cfg)
	}
	return s.purifier.SanitizeErr(s.SanitizeMore(text, config), purify.Config{ForbidTags: []string{"style"}})
}

// SanitizeTextOrArray is sanitizeTextOrArray for a string. It returns the
// sanitized string.
func (s *Sanitizer) SanitizeTextOrArray(a string, config *js.Obj[any]) string {
	return s.SanitizeText(a, config)
}

// SanitizeTextArray is sanitizeTextOrArray for string[] or string[][]: the
// argument is flattened by one level, as Array.prototype.flat does, and each
// string is sanitized. An element is a string or a []string.
func (s *Sanitizer) SanitizeTextArray(a []any, config *js.Obj[any]) []string {
	out := []string{}
	for _, e := range a {
		switch x := e.(type) {
		case string:
			out = append(out, s.SanitizeText(x, config))
		case []string:
			for _, v := range x {
				out = append(out, s.SanitizeText(v, config))
			}
		case []any:
			for _, v := range x {
				if str, ok := v.(string); ok {
					out = append(out, s.SanitizeText(str, config))
				}
			}
		}
	}
	return out
}

// SanitizeText is sanitizeText on a new Sanitizer: the state of a page whose
// first sanitize call this is. A render must use one Sanitizer for all of its
// calls, because the hooks that an earlier call installed change the result
// for a link with a target attribute.
func SanitizeText(text string, config *js.Obj[any]) string {
	return New().SanitizeText(text, config)
}

// RemoveScript is removeScript on a new Sanitizer.
func RemoveScript(txt string) string {
	return New().RemoveScript(txt)
}

// HasBreaks reports whether a text has any line breaks (hasBreaks).
func HasBreaks(text string) bool {
	return LineBreakRegex.MatchString(text)
}

// SplitBreaks splits on <br> tags (splitBreaks).
func SplitBreaks(text string) []string {
	return LineBreakRegex.Split(text, -1)
}

// PlaceholderToBreak converts placeholders to line breaks in HTML
// (placeholderToBreak).
func PlaceholderToBreak(s string) string {
	return strings.ReplaceAll(s, "#br#", "<br/>")
}

// BreakToPlaceholder converts breaks to placeholders (breakToPlaceholder).
func BreakToPlaceholder(s string) string {
	return LineBreakRegex.ReplaceAllLiteralString(s, "#br#")
}

// Location is the part of window.location that getUrl reads.
type Location struct {
	Protocol string // "https:"
	Host     string // "example.com:8080"
	Pathname string // "/a/b"
	Search   string // "?q=1"
}

// GetUrl gets the current URL (getUrl). Upstream reads window.location. The
// caller passes the location of the page that it emulates.
func GetUrl(useAbsolute bool, location Location) string {
	url := ""
	if useAbsolute {
		url = location.Protocol + "//" + location.Host + location.Pathname + location.Search
		url = CSSEscape(url)
	}
	return url
}

// CSSEscape is CSS.escape: the "serialize an identifier" algorithm of CSSOM.
func CSSEscape(value string) string {
	const hex = "0123456789abcdef"
	var sb strings.Builder
	writeHex := func(c byte) {
		sb.WriteByte('\\')
		if c >= 0x10 {
			sb.WriteByte(hex[c>>4])
		}
		sb.WriteByte(hex[c&0xf])
		sb.WriteByte(' ')
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c == 0:
			sb.WriteRune(0xFFFD)
		case c <= 0x1f || c == 0x7f:
			writeHex(c)
		case c >= '0' && c <= '9' && (i == 0 || (i == 1 && value[0] == '-')):
			writeHex(c)
		case c == '-' && i == 0 && len(value) == 1:
			sb.WriteString(`\-`)
		case c >= 0x80 || c == '-' || c == '_' || (c >= '0' && c <= '9') ||
			(c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z'):
			sb.WriteByte(c)
		default:
			sb.WriteByte('\\')
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

// Evaluate converts a string or a boolean into a boolean (evaluate of
// config.ts, which common.ts exports again).
func Evaluate(val any) bool {
	if b, ok := val.(bool); ok && !b {
		return false
	}
	switch js.ToLowerCase(js.TrimSpace(js.ToString(val))) {
	case "false", "null", "0":
		return false
	}
	return true
}

// GetEffectiveHtmlLabels is getEffectiveHtmlLabels of config.ts: the root
// htmlLabels, then the deprecated flowchart.htmlLabels, then true.
func GetEffectiveHtmlLabels(config *js.Obj[any]) bool {
	var value any
	if config != nil {
		value = config.Get("htmlLabels")
		if js.IsNullish(value) {
			value, _ = js.GetProperty(config.Get("flowchart"), "htmlLabels")
		}
	}
	if js.IsNullish(value) {
		value = true
	}
	return Evaluate(value)
}

// GetMax is a wrapper around Math.max which removes non-numeric values
// (getMax).
func GetMax(values ...float64) float64 {
	newValues := make([]float64, 0, len(values))
	for _, value := range values {
		if !math.IsNaN(value) {
			newValues = append(newValues, value)
		}
	}
	return js.Max(newValues...)
}

// GetMin is a wrapper around Math.min which removes non-numeric values
// (getMin).
func GetMin(values ...float64) float64 {
	newValues := make([]float64, 0, len(values))
	for _, value := range values {
		if !math.IsNaN(value) {
			newValues = append(newValues, value)
		}
	}
	return js.Min(newValues...)
}

// ParseGenericTypes makes generics in typescript syntax (parseGenericTypes):
// "Array~Array~string~~" becomes "Array<Array<string>>".
func ParseGenericTypes(input string) string {
	// input.split(/(,)/): the separators are part of the result.
	parts := strings.Split(input, ",")
	inputSets := make([]string, 0, 2*len(parts)-1)
	for i, part := range parts {
		if i > 0 {
			inputSets = append(inputSets, ",")
		}
		inputSets = append(inputSets, part)
	}
	output := []string{}
	for i := 0; i < len(inputSets); i++ {
		thisSet := inputSets[i]
		// If the original input included a value such as "~K, V~", these will
		// be split into an array of ["~K", ",", " V~"]. This means that on
		// each call of processSet, there will only be 1 ~ present. To account
		// for this, if we encounter a ",", we are checking the previous and
		// next sets in the array to see if they contain matching ~'s, in
		// which case we are assuming that they should be rejoined and sent to
		// be processed.
		if thisSet == "," && i > 0 && i+1 < len(inputSets) {
			previousSet := inputSets[i-1]
			nextSet := inputSets[i+1]
			if ShouldCombineSets(previousSet, nextSet) {
				thisSet = previousSet + "," + nextSet
				i++ // Move the index forward to skip the next iteration since we're combining sets.
				output = output[:len(output)-1]
			}
		}
		output = append(output, ProcessSet(thisSet))
	}
	return strings.Join(output, "")
}

// CountOccurrence is countOccurrence: string.split(substring).length - 1,
// and not less than 0.
func CountOccurrence(str, substring string) int {
	if substring == "" {
		// split("") gives one item for each UTF-16 code unit.
		return max(0, js.UTF16Len(str)-1)
	}
	return strings.Count(str, substring)
}

// ShouldCombineSets is shouldCombineSets.
func ShouldCombineSets(previousSet, nextSet string) bool {
	prevCount := CountOccurrence(previousSet, "~")
	nextCount := CountOccurrence(nextSet, "~")
	return prevCount == 1 && nextCount == 1
}

// ProcessSet is processSet.
func ProcessSet(input string) string {
	tildeCount := CountOccurrence(input, "~")
	hasStartingTilde := false
	if tildeCount <= 1 {
		return input
	}
	// If there is an odd number of tildes, and the input starts with a tilde,
	// we need to remove it and add it back in later.
	if tildeCount%2 != 0 && strings.HasPrefix(input, "~") {
		input = input[1:]
		hasStartingTilde = true
	}
	// Upstream works on the code points of the input. The tilde is ASCII, so
	// the bytes give the same result.
	chars := []byte(input)
	first := strings.IndexByte(input, '~')
	last := strings.LastIndexByte(input, '~')
	for first != -1 && last != -1 && first != last {
		chars[first] = '<'
		chars[last] = '>'
		first = indexByte(chars, '~')
		last = lastIndexByte(chars, '~')
	}
	// Add the starting tilde back in if we removed it.
	if hasStartingTilde {
		return "~" + string(chars)
	}
	return string(chars)
}

func indexByte(b []byte, c byte) int {
	for i, x := range b {
		if x == c {
			return i
		}
	}
	return -1
}

func lastIndexByte(b []byte, c byte) int {
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] == c {
			return i
		}
	}
	return -1
}

// HasKatex reports whether a text has KaTeX delimiters (hasKatex).
func HasKatex(text string) bool {
	return KatexRegex.MatchString(text)
}
