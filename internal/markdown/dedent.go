// Ported from ts-dedent 2.2.0 (ts-dedent/src/index.ts), the version that
// mermaid 12.2.0 bundles.

package markdown

import (
	"strings"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

func isTabOrSpace(c byte) bool { return c == '\t' || c == ' ' }

// dedent is dedent(templ) for one string argument, which is how mermaid
// calls it. It removes the indentation that all lines share.
func dedent(templ string) string {
	s := templ

	// strings[strings.length - 1].replace(/\r?\n([\t ]*)$/, '')
	//
	// Only the last line feed can match: the characters after it must be
	// tabs and spaces up to the end.
	if i := strings.LastIndexByte(s, '\n'); i >= 0 && strings.Trim(s[i+1:], "\t ") == "" {
		if i > 0 && s[i-1] == '\r' {
			i--
		}
		s = s[:i]
	}

	// str.match(/\n([\t ]+|(?!\s).)/g), and for each match the number of
	// tabs and spaces in it. A line feed matches when tabs and spaces
	// follow it, or a character that is not white space. In the second
	// case the number is 0.
	minIndent := -1
	for i := 0; i < len(s); i++ {
		if s[i] != '\n' {
			continue
		}
		n := 0
		for i+1+n < len(s) && isTabOrSpace(s[i+1+n]) {
			n++
		}
		if n == 0 {
			if i+1 == len(s) {
				continue
			}
			if r, _ := utf8.DecodeRuneInString(s[i+1:]); js.IsWhiteSpace(r) {
				continue
			}
		}
		if minIndent < 0 || n < minIndent {
			minIndent = n
		}
	}

	// if (indentLengths.length) {
	//   var pattern = new RegExp("\n[\t ]{" + Math.min.apply(Math, indentLengths) + "}", "g");
	//   strings = strings.map(str => str.replace(pattern, "\n"));
	// }
	if minIndent > 0 {
		var sb strings.Builder
		sb.Grow(len(s))
		for i := 0; i < len(s); i++ {
			sb.WriteByte(s[i])
			if s[i] != '\n' {
				continue
			}
			n := 0
			for n < minIndent && i+1+n < len(s) && isTabOrSpace(s[i+1+n]) {
				n++
			}
			if n == minIndent {
				i += n
			}
		}
		s = sb.String()
	}

	// strings[0].replace(/^\r?\n/, '')
	if strings.HasPrefix(s, "\r\n") {
		s = s[2:]
	} else if strings.HasPrefix(s, "\n") {
		s = s[1:]
	}
	return s
}
