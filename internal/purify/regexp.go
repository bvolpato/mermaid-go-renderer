// Ported from dompurify 3.4.12, src/regexp.ts (MPL-2.0 OR Apache-2.0).

package purify

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// The upstream expressions use the i flag without the u flag. In that mode a
// JavaScript engine never lets a non-ASCII character match an ASCII letter.
// The (?i) flag of Go does (U+017F matches s, U+212A matches k). So every
// case-insensitive expression is written here with explicit ASCII classes.

// ci turns a lowercase ASCII literal into a pattern that matches it in any
// ASCII case.
func ci(literal string) string {
	var sb strings.Builder
	for i := 0; i < len(literal); i++ {
		c := literal[i]
		if c >= 'a' && c <= 'z' {
			sb.WriteByte('[')
			sb.WriteByte(c)
			sb.WriteByte(c - 'a' + 'A')
			sb.WriteByte(']')
		} else {
			sb.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return sb.String()
}

// ciAlt is ci for a list of alternatives.
func ciAlt(literals ...string) string {
	parts := make([]string, len(literals))
	for i, l := range literals {
		parts[i] = ci(l)
	}
	return strings.Join(parts, "|")
}

var (
	// MUSTACHE_EXPR = /{{[\w\W]*|^[\w\W]*}}/g
	mustacheExpr = regexp.MustCompile(`(?s)\{\{.*|^.*\}\}`)
	// ERB_EXPR = /<%[\w\W]*|^[\w\W]*%>/g
	erbExpr = regexp.MustCompile(`(?s)<%.*|^.*%>`)
	// TMPLIT_EXPR = /\${[\w\W]*/g
	tmplitExpr = regexp.MustCompile(`(?s)\$\{.*`)
	// DATA_ATTR = /^data-[\-\w.\x{B7}-\x{FFFF}]+$/, with the escapes of Go. A character above U+FFFF is
	// two UTF-16 code units in JavaScript, and both are in the class.
	dataAttr = regexp.MustCompile(`^data-[\-\w.\x{B7}-\x{10FFFF}]+$`)
	// ARIA_ATTR = /^aria-[\-\w]+$/
	ariaAttr = regexp.MustCompile(`^aria-[\-\w]+$`)
	// IS_ALLOWED_URI =
	// /^(?:(?:(?:f|ht)tps?|mailto|tel|callto|sms|cid|xmpp|matrix):|[^a-z]|[a-z+.\-]+(?:[^a-z+.\-:]|$))/i
	isAllowedURI = regexp.MustCompile(`^(?:(?:(?:[fF]|[hH][tT])[tT][pP][sS]?|` +
		ciAlt("mailto", "tel", "callto", "sms", "cid", "xmpp", "matrix") +
		`):|[^a-zA-Z]|[a-zA-Z+.\-]+(?:[^a-zA-Z+.\-:]|$))`)
	// IS_SCRIPT_OR_DATA = /^(?:\w+script|data):/i
	isScriptOrData = regexp.MustCompile(`^(?:\w+` + ci("script") + `|` + ci("data") + `):`)
	// DOCTYPE_NAME = /^html$/i
	doctypeName = regexp.MustCompile(`^` + ci("html") + `$`)
	// CUSTOM_ELEMENT = /^[a-z][.\w]*(-[.\w]+)+$/i
	customElement = regexp.MustCompile(`^[a-zA-Z][.\w]*(-[.\w]+)+$`)
	// ELEMENT_MARKUP_PROBE = /<[/\w!]/g
	elementMarkupProbe = regexp.MustCompile(`<[/\w!]`)
	// COMMENT_MARKUP_PROBE = /<[/\w]/g
	commentMarkupProbe = regexp.MustCompile(`<[/\w]`)
	// FALLBACK_TAG_CLOSE = /<\/no(script|embed|frames)/i
	fallbackTagClose = regexp.MustCompile(`</` + ci("no") + `(` + ciAlt("script", "embed", "frames") + `)`)
	// The expression that _sanitizeAttributes applies to a value:
	// /((--!?|])>)|<\/(style|script|title|xmp|textarea|noscript|iframe|noembed|noframes)/i
	attrValueProbe = regexp.MustCompile(`((--!?|\])>)|</(` +
		ciAlt("style", "script", "title", "xmp", "textarea", "noscript", "iframe", "noembed", "noframes") + `)`)
	// The expression of _initDocument for leading white space: /^[\r\n\t ]+/
	leadingWhitespace = regexp.MustCompile(`^[\r\n\t ]+`)
)

// stripAttrWhitespace is value.replace(ATTR_WHITESPACE, "") with
// ATTR_WHITESPACE: the code points U+0000 to U+0020, U+00A0, U+1680, U+180E,
// U+2000 to U+2029, U+205F and U+3000.
func stripAttrWhitespace(value string) string {
	isWS := func(r rune) bool {
		switch {
		case r <= 0x20, r == 0xA0, r == 0x1680, r == 0x180E, r == 0x205F, r == 0x3000:
			return true
		case r >= 0x2000 && r <= 0x2029:
			return true
		}
		return false
	}
	if strings.IndexFunc(value, isWS) < 0 {
		return value
	}
	var sb strings.Builder
	sb.Grow(len(value))
	for i := 0; i < len(value); {
		c := value[i]
		if c < 0x80 {
			if c > 0x20 {
				sb.WriteByte(c)
			}
			i++
			continue
		}
		// An invalid byte decodes with size 1 and is copied as is.
		r, size := utf8.DecodeRuneInString(value[i:])
		if size > 1 && isWS(r) {
			i += size
			continue
		}
		sb.WriteString(value[i : i+size])
		i += size
	}
	return sb.String()
}

// The functions below give the result of an expression above without the
// regular expression engine. sanitize applies these tests to every attribute
// of a document, and most values have no character that the expression
// needs. TestFastPaths compares each function with its expression.

// matchAttrValueProbe is attrValueProbe.MatchString(value). Every
// alternative of the expression has a ">" or a "</".
func matchAttrValueProbe(value string) bool {
	if strings.IndexByte(value, '>') < 0 && !strings.Contains(value, "</") {
		return false
	}
	return attrValueProbe.MatchString(value)
}

// matchDataAttr is dataAttr.MatchString(name).
func matchDataAttr(name string) bool {
	return strings.HasPrefix(name, "data-") && dataAttr.MatchString(name)
}

// matchAriaAttr is ariaAttr.MatchString(name).
func matchAriaAttr(name string) bool {
	return strings.HasPrefix(name, "aria-") && ariaAttr.MatchString(name)
}

// matchElementMarkupProbe is elementMarkupProbe.MatchString(text).
func matchElementMarkupProbe(text string) bool {
	return strings.IndexByte(text, '<') >= 0 && elementMarkupProbe.MatchString(text)
}

// allowedSchemes are the schemes of IS_ALLOWED_URI.
var allowedSchemes = map[string]bool{
	"ftp": true, "ftps": true, "http": true, "https": true, "mailto": true, "tel": true,
	"callto": true, "sms": true, "cid": true, "xmpp": true, "matrix": true,
}

// matchAllowedURI is isAllowedURI.MatchString(value). The expression accepts
// a value that does not start with a letter, a value whose leading run of
// scheme characters does not end at a colon, and a value with one of the
// allowed schemes.
func matchAllowedURI(value string) bool {
	if value == "" {
		return false
	}
	isAlpha := func(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
	if !isAlpha(value[0]) {
		return true
	}
	i := 1
	for i < len(value) {
		c := value[i]
		if !isAlpha(c) && c != '+' && c != '.' && c != '-' {
			break
		}
		i++
	}
	if i == len(value) || value[i] != ':' {
		return true
	}
	if i > 6 {
		return false
	}
	var lower [6]byte
	for k := 0; k < i; k++ {
		c := value[k]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		lower[k] = c
	}
	return allowedSchemes[string(lower[:i])]
}
