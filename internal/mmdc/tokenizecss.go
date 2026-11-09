// Ported from mermaid-cli 12.0.0, src/browser/tokenizeCSS.js.

package mmdc

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// cssSourceToken is CSSSourceToken: one source of the src descriptor of a
// @font-face rule. A has field is true when the source has the key, also
// with an empty value.
type cssSourceToken struct {
	url, format, tech, local             string
	hasURL, hasFormat, hasTech, hasLocal bool
}

// stringifyCSSSrc is stringifyCSSSrc: it turns source tokens into the text
// of a src descriptor. It is the reverse of tokenizeCSSSrc.
func stringifyCSSSrc(tokens []cssSourceToken) string {
	sources := make([]string, 0, len(tokens))
	for _, token := range tokens {
		var parts []string
		if token.url != "" {
			parts = append(parts, "url("+js.QuoteJSON(token.url)+")")
		}
		if token.format != "" {
			parts = append(parts, "format("+js.QuoteJSON(token.format)+")")
		}
		if token.tech != "" {
			parts = append(parts, "tech("+token.tech+")")
		}
		if token.local != "" {
			parts = append(parts, "local("+token.local+")")
		}
		sources = append(sources, strings.Join(parts, " "))
	}
	return strings.Join(sources, ", ")
}

// A scanner reads a text as the functions of tokenizeCSS.js do. They index
// a JavaScript string by UTF-16 code units. Every character that they test
// is ASCII, and they copy all others, so that reading by bytes gives the
// same result.
type cssScanner struct {
	s string
	i int
}

func (sc *cssScanner) skipWhitespace() {
	for sc.i < len(sc.s) && isCSSWhitespace(sc.s[sc.i]) {
		sc.i++
	}
}

// parseFontFamily is parseFontFamily: the family names of the text of a
// font-family value, without quotes.
//
//	parseFontFamily(`"Hello, World", Hello, World, Hello World, \22 toto\22`)
//	// ["Hello, World", "Hello", "World", "Hello World", `"toto"`]
func parseFontFamily(s string) ([]string, error) {
	var fontFamilies []string
	sc := &cssScanner{s: s}
	for sc.i < len(s) {
		sc.skipWhitespace()
		parsed, tokens, ok, err := readQuotedString(s[sc.i:])
		if err != nil {
			return nil, err
		}
		if ok {
			fontFamilies = append(fontFamilies, parsed)
			sc.i += tokens
		} else {
			var fontFamilyParts []string
			originalI := sc.i
			for sc.i < len(s) {
				parsed, tokens, ok, err := readIdentToken(s[sc.i:])
				if err != nil {
					return nil, err
				}
				if !ok {
					break
				}
				fontFamilyParts = append(fontFamilyParts, parsed)
				sc.i += tokens
				sc.skipWhitespace()
			}
			if len(fontFamilyParts) == 0 {
				return nil, errors.New("Failed to parse font family from string: " + s[originalI:])
			}
			fontFamilies = append(fontFamilies, strings.Join(fontFamilyParts, " "))
		}
		sc.skipWhitespace()
		if sc.i < len(s) && s[sc.i] == ',' {
			sc.i++ // consume ','
		}
	}
	return fontFamilies, nil
}

// tokenizeCSSSrc is tokenizeCSSSrc: it parses the value of a src
// descriptor.
func tokenizeCSSSrc(s string) ([]cssSourceToken, error) {
	var sources []cssSourceToken
	sc := &cssScanner{s: s}
	sc.skipWhitespace()
	for sc.i < len(s) {
		var source cssSourceToken
		for sc.i < len(s) && s[sc.i] != ',' {
			sc.skipWhitespace()
			if sc.i >= len(s) || s[sc.i] == ',' {
				break
			}
			if parsed, tokens, ok, err := readURLToken(s[sc.i:]); err != nil {
				return nil, err
			} else if ok {
				source.url, source.hasURL = parsed, true
				sc.i += tokens
				sc.skipWhitespace()
				continue
			}

			ident, tokens, ok, err := readIdentToken(s[sc.i:])
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, errors.New("Failed to read identifier token from: " + s[sc.i:])
			}
			sc.i += tokens
			sc.skipWhitespace()
			if sc.i < len(s) && s[sc.i] == '(' {
				sc.i++ // consume '('
				sc.skipWhitespace()
				value := ""
				if parsed, tokens, ok, err := readQuotedString(s[sc.i:]); err != nil {
					return nil, err
				} else if ok {
					value = parsed
					sc.i += tokens
				} else if parsed, tokens, ok, err := readIdentToken(s[sc.i:]); err != nil {
					return nil, err
				} else if ok {
					value = parsed
					sc.i += tokens
				}
				sc.skipWhitespace()
				if sc.i < len(s) && s[sc.i] == ')' {
					sc.i++ // consume ')'
				}
				switch ident {
				case "url":
					source.url, source.hasURL = value, true
				case "format":
					source.format, source.hasFormat = value, true
				case "tech":
					source.tech, source.hasTech = value, true
				case "local":
					source.local, source.hasLocal = value, true
				}
			}
			sc.skipWhitespace()
		}
		if source.hasURL || source.hasFormat || source.hasTech || source.hasLocal {
			sources = append(sources, source)
		}
		if sc.i < len(s) && s[sc.i] == ',' {
			sc.i++ // consume ','
		}
		sc.skipWhitespace()
	}
	return sources, nil
}

// isCSSWhitespace is isWhitespace.
func isCSSWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n'
}

func isHexDigit(ch byte) bool {
	return ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f' || ch >= 'A' && ch <= 'F'
}

// readEscape is readEscape: it parses a CSS escape at the start of s. ok
// is false when s does not start with a backslash.
func readEscape(s string) (parsed string, tokens int, ok bool, err error) {
	if s == "" || s[0] != '\\' {
		return "", 0, false, nil
	}
	hex := ""
	i := 1 // start after the backslash
	for i < len(s) && len(hex) < 6 && isHexDigit(s[i]) {
		hex += string(s[i])
		i++
	}
	if len(hex) > 0 {
		// A single trailing whitespace is part of the hex escape.
		if i < len(s) && isCSSWhitespace(s[i]) {
			i++
		}
		code, _ := strconv.ParseInt(hex, 16, 32)
		if code > utf8.MaxRune {
			// String.fromCodePoint throws a RangeError.
			return "", 0, false, errors.New("Invalid code point " + strconv.FormatInt(code, 10))
		}
		return js.FromUTF16Units(utf16Units(rune(code))), i, true, nil
	}
	if i < len(s) {
		_, size := utf8.DecodeRuneInString(s[i:])
		return s[i : i+size], i + size, true, nil
	}
	return "", 0, false, errors.New("Invalid escape sequence: " + s)
}

// utf16Units is the UTF-16 form of a code point, as String.fromCodePoint
// makes it. A surrogate code point stays one unit.
func utf16Units(r rune) []rune {
	if r >= 0x10000 {
		r -= 0x10000
		return []rune{0xd800 + (r>>10)&0x3ff, 0xdc00 + r&0x3ff}
	}
	return []rune{r}
}

// readQuotedString is readQuotedString: it parses a CSS string at the
// start of s. ok is false when s does not start with a quote.
func readQuotedString(s string) (parsed string, tokens int, ok bool, err error) {
	if s == "" || (s[0] != '"' && s[0] != '\'') {
		return "", 0, false, nil
	}
	quote := s[0]
	i := 1 // consume opening quote
	var result strings.Builder
	for i < len(s) {
		ch := s[i]
		if escaped, n, isEscape, err := readEscape(s[i:]); err != nil {
			return "", 0, false, err
		} else if isEscape {
			result.WriteString(escaped)
			i += n
		} else if ch == quote {
			return result.String(), i + 1, true, nil
		} else {
			result.WriteByte(ch)
			i++
		}
	}
	return "", 0, false, errors.New("Unterminated quoted string: " + s)
}

// isIdentStart is /[a-zA-Z_]|[^\x00-\x7F]/ and isIdentPart is
// /[a-zA-Z0-9_-]|[^\x00-\x7F]/ for one byte.
func isIdentStart(ch byte) bool {
	return ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch == '_' || ch >= 0x80
}

func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || ch >= '0' && ch <= '9' || ch == '-'
}

// readIdentToken is readIdentToken: it reads an ident-token at the start
// of s. ok is false when s does not start with one.
func readIdentToken(s string) (parsed string, tokens int, ok bool, err error) {
	i := 0
	var ident strings.Builder
	if strings.HasPrefix(s, "--") {
		i += 2
		ident.WriteString("--")
	} else {
		if strings.HasPrefix(s, "-") {
			i++
			ident.WriteByte('-')
		}
		if escaped, n, isEscape, err := readEscape(s[i:]); err != nil {
			return "", 0, false, err
		} else if isEscape {
			ident.WriteString(escaped)
			i += n
		} else if i < len(s) && isIdentStart(s[i]) {
			ident.WriteByte(s[i])
			i++
		} else {
			// No valid starting sequence for an ident found
			return "", 0, false, nil
		}
	}
	for i < len(s) {
		if escaped, n, isEscape, err := readEscape(s[i:]); err != nil {
			return "", 0, false, err
		} else if isEscape {
			ident.WriteString(escaped)
			i += n
		} else if isIdentPart(s[i]) {
			ident.WriteByte(s[i])
			i++
		} else {
			break
		}
	}
	return ident.String(), i, true, nil
}

// isURLChar is /[^"'()\\\x00-\x08\x0B\x0E-\x1F\x7F]/ for one byte.
func isURLChar(ch byte) bool {
	switch {
	case ch == '"', ch == '\'', ch == '(', ch == ')', ch == '\\':
		return false
	case ch <= 0x08, ch == 0x0b, ch >= 0x0e && ch <= 0x1f, ch == 0x7f:
		return false
	}
	return true
}

// readURLToken is readUrlToken: it parses a CSS url-token at the start of
// s, which is a url() without quotes. ok is false when s does not start
// with one.
func readURLToken(s string) (parsed string, tokens int, ok bool, err error) {
	ident, i, isIdent, err := readIdentToken(s)
	if err != nil {
		return "", 0, false, err
	}
	if !isIdent || ident != "url" {
		return "", 0, false, nil // Not URL token
	}
	if i >= len(s) || s[i] != '(' {
		return "", 0, false, nil // Not a valid URL token
	}
	i++ // consume '('
	for i < len(s) && isCSSWhitespace(s[i]) {
		i++
	}
	var url strings.Builder
	for i < len(s) {
		if escaped, n, isEscape, err := readEscape(s[i:]); err != nil {
			return "", 0, false, err
		} else if isEscape {
			url.WriteString(escaped)
			i += n
		} else if isURLChar(s[i]) {
			url.WriteByte(s[i])
			i++
		} else {
			break
		}
	}
	for i < len(s) && isCSSWhitespace(s[i]) {
		i++
	}
	if i >= len(s) || s[i] != ')' {
		return "", 0, false, nil // Not a valid URL token
	}
	i++ // consume ')'
	return url.String(), i, true, nil
}
