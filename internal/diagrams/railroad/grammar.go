// Ported from mermaid 12.2.0, the parts that the four railroad grammars of
// packages/parser/src/language share: the start of the entry rule in
// railroad/railroad.langium, railroad-ebnf/railroad-ebnf.langium,
// railroad-abnf/railroad-abnf.langium and railroad-peg/railroad-peg.langium,
// and the value converters in the valueConverter.ts of each grammar (MIT).

package railroad

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/langium"
)

// decodeEscapedString is decodeEscapedString of valueConverter.ts: it
// removes the first and the last character and replaces each backslash
// escape. A backslash at the end stays.
func decodeEscapedString(input string) string {
	units := js.UTF16Units(input)
	// const content = input.slice(1, -1);
	var content []rune
	if len(units) >= 2 {
		content = units[1 : len(units)-1]
	}
	value := make([]rune, 0, len(content))

	for index := 0; index < len(content); index++ {
		character := content[index]
		if character == '\\' && index+1 < len(content) {
			index++
			escaped := content[index]
			switch escaped {
			case 'n':
				value = append(value, '\n')
			case 'r':
				value = append(value, '\r')
			case 't':
				value = append(value, '\t')
			default:
				value = append(value, escaped)
			}
			continue
		}

		value = append(value, character)
	}

	return js.FromUTF16Units(value)
}

// sliceInner is input.slice(1, -1): the text without its first and its last
// UTF-16 code unit.
func sliceInner(input string) string {
	n := js.UTF16Len(input)
	if n < 2 {
		return ""
	}
	return js.StringSlice(input, 1, n-1)
}

// convertTitle is runConverter of the value converters for the rule TITLE.
// The common converter gives the text after "title". The converter of the
// grammar then removes the quotes of a quoted title with unquote:
// decodeEscapedString, or slice(1, -1) in the ABNF grammar.
func convertTitle(image string, unquote func(string) string) string {
	value, ok := langium.ConvertTitle(image)
	if !ok {
		// The common converter returns undefined and the default converter
		// of Langium then keeps the text of the token.
		value = image
	}

	trimmedValue := js.TrimSpace(value)
	if (strings.HasPrefix(trimmedValue, `"`) && strings.HasSuffix(trimmedValue, `"`)) ||
		(strings.HasPrefix(trimmedValue, "'") && strings.HasSuffix(trimmedValue, "'")) {
		return unquote(trimmedValue)
	}

	return value
}

// header is the start of the entry rule of each grammar:
//
//	'<keyword>'
//	((title=TITLE | accTitle=ACC_TITLE | accDescr=ACC_DESCR))*
//
// A property that the text sets two times keeps the last value.
func header(p *langium.Parser, keyword string, common *langium.Common, unquote func(string) string) error {
	if _, err := p.Expect(keyword); err != nil {
		return err
	}
	for p.At("TITLE", "ACC_TITLE", "ACC_DESCR") {
		t := p.Next()
		var value string
		switch t.Type.Name {
		case "TITLE":
			value = convertTitle(t.Image, unquote)
			common.Title = &value
		case "ACC_TITLE":
			var ok bool
			if value, ok = langium.ConvertAccTitle(t.Image); !ok {
				value = t.Image
			}
			common.AccTitle = &value
		default:
			var ok bool
			if value, ok = langium.ConvertAccDescr(t.Image); !ok {
				value = t.Image
			}
			common.AccDescr = &value
		}
	}
	return nil
}
