// Ported from dompurify 3.4.12, src/regexp.ts (MPL-2.0 OR Apache-2.0).

package purify

import (
	"regexp"
	"strings"
	"sync"
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
	mustacheExpr = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`(?s)\{\{.*|^.*\}\}`) })
	// ERB_EXPR = /<%[\w\W]*|^[\w\W]*%>/g
	erbExpr = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`(?s)<%.*|^.*%>`) })
	// TMPLIT_EXPR = /\${[\w\W]*/g
	tmplitExpr = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`(?s)\$\{.*`) })
	// DATA_ATTR = /^data-[\-\w.\x{B7}-\x{FFFF}]+$/, with the escapes of Go. A character above U+FFFF is
	// two UTF-16 code units in JavaScript, and both are in the class.
	dataAttr = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`^data-[\-\w.\x{B7}-\x{10FFFF}]+$`) })
	// ARIA_ATTR = /^aria-[\-\w]+$/
	ariaAttr = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`^aria-[\-\w]+$`) })
	// IS_ALLOWED_URI =
	// /^(?:(?:(?:f|ht)tps?|mailto|tel|callto|sms|cid|xmpp|matrix):|[^a-z]|[a-z+.\-]+(?:[^a-z+.\-:]|$))/i
	isAllowedURI = sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(`^(?:(?:(?:[fF]|[hH][tT])[tT][pP][sS]?|` +
			ciAlt("mailto", "tel", "callto", "sms", "cid", "xmpp", "matrix") +
			`):|[^a-zA-Z]|[a-zA-Z+.\-]+(?:[^a-zA-Z+.\-:]|$))`)
	})
	// IS_SCRIPT_OR_DATA = /^(?:\w+script|data):/i
	isScriptOrData = sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(`^(?:\w+` + ci("script") + `|` + ci("data") + `):`)
	})
	// DOCTYPE_NAME = /^html$/i
	doctypeName = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`^` + ci("html") + `$`) })
	// CUSTOM_ELEMENT = /^[a-z][.\w]*(-[.\w]+)+$/i
	customElement = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`^[a-zA-Z][.\w]*(-[.\w]+)+$`) })
	// ELEMENT_MARKUP_PROBE = /<[/\w!]/g
	elementMarkupProbe = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`<[/\w!]`) })
	// COMMENT_MARKUP_PROBE = /<[/\w]/g
	commentMarkupProbe = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`<[/\w]`) })
	// FALLBACK_TAG_CLOSE = /<\/no(script|embed|frames)/i
	fallbackTagClose = sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(`</` + ci("no") + `(` + ciAlt("script", "embed", "frames") + `)`)
	})
	// The expression that _sanitizeAttributes applies to a value:
	// /((--!?|])>)|<\/(style|script|title|xmp|textarea|noscript|iframe|noembed|noframes)/i
	attrValueProbe = sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(`((--!?|\])>)|</(` +
			ciAlt("style", "script", "title", "xmp", "textarea", "noscript", "iframe", "noembed", "noframes") + `)`)
	})
)

func isAttrWhitespaceRune(r rune) bool {
	switch {
	case r <= 0x20, r == 0xA0, r == 0x1680, r == 0x180E, r == 0x205F, r == 0x3000:
		return true
	case r >= 0x2000 && r <= 0x2029:
		return true
	}
	return false
}

// stripAttrWhitespace is value.replace(ATTR_WHITESPACE, "") with
// ATTR_WHITESPACE: the code points U+0000 to U+0020, U+00A0, U+1680, U+180E,
// U+2000 to U+2029, U+205F and U+3000.
func stripAttrWhitespace(value string) string {
	hasWS := false
	for i := 0; i < len(value); {
		c := value[i]
		if c < 0x80 {
			if c <= 0x20 {
				hasWS = true
				break
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(value[i:])
		if size > 1 && isAttrWhitespaceRune(r) {
			hasWS = true
			break
		}
		i += size
	}
	if !hasWS {
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
		if size > 1 && isAttrWhitespaceRune(r) {
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
	if strings.IndexByte(value, '>') < 0 && strings.IndexByte(value, '<') < 0 {
		return false
	}
	return attrValueProbe().MatchString(value)
}

// matchDataAttr is dataAttr.MatchString(name).
func matchDataAttr(name string) bool {
	if !strings.HasPrefix(name, "data-") || len(name) == 5 {
		return false
	}
	for i := 5; i < len(name); {
		c := name[i]
		if c < 0x80 {
			if c == '-' || c == '_' || c == '.' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				i++
				continue
			}
			return false
		}
		r, size := utf8.DecodeRuneInString(name[i:])
		if r < 0xB7 {
			return false
		}
		i += size
	}
	return true
}

// matchAriaAttr is ariaAttr.MatchString(name).
func matchAriaAttr(name string) bool {
	if !strings.HasPrefix(name, "aria-") || len(name) == 5 {
		return false
	}
	for i := 5; i < len(name); i++ {
		c := name[i]
		if !(c == '-' || c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}

// matchElementMarkupProbe is elementMarkupProbe.MatchString(text).
func matchElementMarkupProbe(text string) bool {
	return strings.IndexByte(text, '<') >= 0 && elementMarkupProbe().MatchString(text)
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

// matchAllowedURISkipWS is matchAllowedURI(stripAttrWhitespace(value))
// without allocating a stripped copy of value. It scans only the leading
// scheme prefix (at most 7 non-whitespace bytes).
func matchAllowedURISkipWS(value string) bool {
	isAlpha := func(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
	// nextNonWS returns the next byte/rune that stripAttrWhitespace keeps,
	// starting at index i, and the next index after it. ok is false at EOF.
	nextByte := func(i int) (b byte, nonASCII bool, next int, ok bool) {
		for i < len(value) {
			c := value[i]
			if c < 0x80 {
				if c <= 0x20 {
					i++
					continue
				}
				return c, false, i + 1, true
			}
			r, size := utf8.DecodeRuneInString(value[i:])
			if size > 1 && isAttrWhitespaceRune(r) {
				i += size
				continue
			}
			return 0, true, i + size, true
		}
		return 0, false, i, false
	}

	b, nonASCII, pos, ok := nextByte(0)
	if !ok {
		return false
	}
	if nonASCII || !isAlpha(b) {
		return true
	}
	var lower [7]byte
	if b >= 'A' && b <= 'Z' {
		b += 'a' - 'A'
	}
	lower[0] = b
	n := 1
	for {
		b, nonASCII, pos, ok = nextByte(pos)
		if !ok {
			return true
		}
		if nonASCII || (!isAlpha(b) && b != '+' && b != '.' && b != '-') {
			break
		}
		if n < len(lower) {
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			lower[n] = b
		}
		n++
	}
	if nonASCII || b != ':' {
		return true
	}
	if n > 6 {
		return false
	}
	return allowedSchemes[string(lower[:n])]
}
