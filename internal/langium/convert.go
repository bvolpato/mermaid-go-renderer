// Ported from langium 4.4.0, langium/src/parser/value-converter.ts
// (DefaultValueConverter, ValueConverter) (MIT), and from mermaid 12.2.0,
// packages/parser/src/language/common/valueConverter.ts and matcher.ts
// (AbstractMermaidValueConverter.runCommonConverter) (MIT).

package langium

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// ConvertString is ValueConverter.convertString, the converter of the
// terminal STRING: it removes the first and the last character and replaces
// each backslash escape.
func ConvertString(input string) string {
	units := js.UTF16Units(input)
	result := make([]rune, 0, len(units))
	for i := 1; i < len(units)-1; i++ {
		c := units[i]
		if c == '\\' {
			// input.charAt(++i) is the empty string after the last unit.
			i++
			if i < len(units) {
				result = append(result, convertEscapeCharacter(units[i]))
			}
		} else {
			result = append(result, c)
		}
	}
	return js.FromUTF16Units(result)
}

func convertEscapeCharacter(char rune) rune {
	switch char {
	case 'b':
		return '\b'
	case 'f':
		return '\f'
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 't':
		return '\t'
	case 'v':
		return '\v'
	case '0':
		return 0
	}
	return char
}

// ConvertID is ValueConverter.convertID, the converter of the terminal ID:
// it removes a leading "^".
func ConvertID(input string) string {
	return strings.TrimPrefix(input, "^")
}

// ConvertInt is ValueConverter.convertInt, the converter of the terminal
// INT: parseInt(input).
func ConvertInt(input string) float64 {
	digits := 0
	for digits < len(input) && input[digits] >= '0' && input[digits] <= '9' {
		digits++
	}
	if digits == len(input) && digits > 0 {
		// V8 rounds a decimal integer of any length correctly.
		if f, err := strconv.ParseFloat(input, 64); err == nil {
			return f
		}
	}
	return js.ParseInt(input, 0)
}

// ConvertNumber is ValueConverter.convertNumber, the converter of a terminal
// that returns number: Number(input).
func ConvertNumber(input string) float64 {
	return js.Number(input)
}

// ConvertBoolean is ValueConverter.convertBoolean, the converter of a
// terminal that returns boolean.
func ConvertBoolean(input string) bool {
	return js.ToLowerCase(input) == "true"
}

// jsSpace is the class \s of a JavaScript regular expression.
const jsSpace = `[\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`

var (
	// accessibilityDescrRegex, accessibilityTitleRegex and titleRegex of
	// matcher.ts.
	accessibilityDescrRegex = regexp.MustCompile(`accDescr(?:[\t ]*:([^\n\r]*)|` + jsSpace + `*\{([^}]*)\})`)
	accessibilityTitleRegex = regexp.MustCompile(`accTitle[\t ]*:([^\n\r]*)`)
	titleRegex              = regexp.MustCompile(`title([\t ][^\n\r]*|)`)

	reBlanks     = regexp.MustCompile(`[\t ]{2,}`)
	reLineBreaks = regexp.MustCompile(`[\n\r]{2,}`)
)

// ConvertAccDescr is the converter of the terminal ACC_DESCR: the text
// after "accDescr:", or the text between the braces of "accDescr { ... }".
// The second result is false when the converter returns undefined, which
// does not happen for a token of the terminal.
func ConvertAccDescr(input string) (string, bool) {
	return runCommonConverter(accessibilityDescrRegex, input)
}

// ConvertAccTitle is the converter of the terminal ACC_TITLE: the text
// after "accTitle:".
func ConvertAccTitle(input string) (string, bool) {
	return runCommonConverter(accessibilityTitleRegex, input)
}

// ConvertTitle is the converter of the terminal TITLE: the text after
// "title".
func ConvertTitle(input string) (string, bool) {
	return runCommonConverter(titleRegex, input)
}

// runCommonConverter is AbstractMermaidValueConverter.runCommonConverter for
// the rule that regex belongs to.
func runCommonConverter(regex *regexp.Regexp, input string) (string, bool) {
	match := regex.FindStringSubmatchIndex(input)
	if match == nil {
		return "", false
	}
	// single line title, accTitle, accDescr
	if match[2] >= 0 {
		return reBlanks.ReplaceAllString(js.TrimSpace(input[match[2]:match[3]]), " "), true
	}
	// multi line accDescr
	if len(match) > 4 && match[4] >= 0 {
		text := []rune(input[match[4]:match[5]])
		text = removeLineStartSpace(text)
		text = removeLineEndSpace(text)
		s := reBlanks.ReplaceAllString(string(text), " ")
		return reLineBreaks.ReplaceAllString(s, "\n"), true
	}
	return "", false
}

// isLineTerminator reports whether "^" and "$" of a JavaScript regular
// expression with the m flag match beside c.
func isLineTerminator(c rune) bool {
	return c == '\n' || c == '\r' || c == 0x2028 || c == 0x2029
}

// removeLineStartSpace is text.replace(/^\s*/gm, ”): it removes each run of
// white space that starts at the start of a line. The run can hold line
// breaks, so it also removes empty lines.
func removeLineStartSpace(text []rune) []rune {
	out := make([]rune, 0, len(text))
	for i := 0; i < len(text); {
		if i == 0 || isLineTerminator(text[i-1]) {
			for i < len(text) && js.IsWhiteSpace(text[i]) {
				i++
			}
			if i >= len(text) {
				break
			}
		}
		out = append(out, text[i])
		i++
	}
	return out
}

// removeLineEndSpace is text.replace(/\s+$/gm, ”): it removes each run of
// white space that ends at the end of a line or of the text.
func removeLineEndSpace(text []rune) []rune {
	n := len(text)
	out := make([]rune, 0, n)
	for i := 0; i < n; {
		if !js.IsWhiteSpace(text[i]) {
			out = append(out, text[i])
			i++
			continue
		}
		// \s+ takes the whole run and then gives units back until "$"
		// matches: at the end of the text or in front of a line terminator.
		k := i
		for k < n && js.IsWhiteSpace(text[k]) {
			k++
		}
		end := -1
		if k == n {
			end = k
		} else {
			for m := k - 1; m > i; m-- {
				if isLineTerminator(text[m]) {
					end = m
					break
				}
			}
		}
		if end > i {
			i = end
			continue
		}
		out = append(out, text[i])
		i++
	}
	return out
}
