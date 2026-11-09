// Ported from Chromium 156.0.8078.4 (Blink), in part: the value of the CSS
// property content, and the string token of CSS. The functions of this file
// follow the functions of Blink that their comments name:
//
//   - CSSTokenizer::ConsumeStringTokenUntil:
//     core/css/parser/css_tokenizer.cc (BSD-3-Clause).
//   - ConsumeEscape: core/css/parser/css_parser_idioms.cc (BSD-3-Clause).
//   - ParseContentValue and Content::ApplyValue:
//     core/css/properties/longhands/longhands_custom.cc (BSD-3-Clause).
//
// SerializeString is the algorithm "serialize a string" of the CSS Object
// Model specification. Blink has it in core/css/css_markup.cc
// (LGPL-2.0-or-later). The paths are in third_party/blink/renderer. See
// NOTICE in this directory.

package css

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// ContentKind is the kind of the value of the property content.
type ContentKind uint8

const (
	// ContentNormal is the keyword normal, the initial value. A ::before
	// or ::after pseudo-element with it has no box.
	ContentNormal ContentKind = iota
	// ContentNone is the keyword none: no box.
	ContentNone
	// ContentItems is a list of items.
	ContentItems
)

// ContentItem is one item of the value of the property content.
type ContentItem struct {
	// Text is the text of a string item. Strings that follow each other
	// are one item, as in Blink.
	Text string
	// Other is the source text of an item that is not a string: a quote
	// keyword, a counter, attr() or an image. Such an item makes no text
	// in this package.
	Other string
}

// Content is the parsed value of the property content.
type Content struct {
	Kind  ContentKind
	Items []ContentItem
	// Alt has the items of the alternative text, after the slash. HasAlt
	// is true when the value has the slash.
	Alt    []ContentItem
	HasAlt bool
}

// ParseContent parses a value of the property content. The second result is
// false for a value that is not valid.
func ParseContent(value string) (Content, bool) {
	s := strings.TrimSpace(value)
	switch strings.ToLower(s) {
	case "normal":
		return Content{Kind: ContentNormal}, true
	case "none":
		return Content{Kind: ContentNone}, true
	}
	out := Content{Kind: ContentItems}
	i := 0
	items := &out.Items
	for {
		i = skipCSSSpace(s, i)
		if i >= len(s) {
			break
		}
		switch c := s[i]; {
		case c == '"' || c == '\'':
			text, next, ok := ConsumeString(s, i)
			if !ok {
				return Content{}, false
			}
			i = next
			// Content::ApplyValue joins the strings of the content that
			// follow each other. The strings of the alternative text stay
			// as they are.
			if n := len(*items); n > 0 && (*items)[n-1].Other == "" && !out.HasAlt {
				(*items)[n-1].Text += text
			} else {
				*items = append(*items, ContentItem{Text: text})
			}
		case c == '/':
			// The alternative text follows one slash, and at least one
			// item comes before it.
			if out.HasAlt || len(out.Items) == 0 {
				return Content{}, false
			}
			out.HasAlt = true
			items = &out.Alt
			i++
		default:
			// A keyword or a function: its text up to the next white space
			// or slash that is not in parentheses.
			start := i
			depth := 0
		scan:
			for i < len(s) {
				switch s[i] {
				case '"', '\'':
					_, next, ok := ConsumeString(s, i)
					if !ok {
						return Content{}, false
					}
					i = next
					continue
				case '(':
					depth++
				case ')':
					depth--
				case ' ', '\t', '\n', '\r', '\f', '/':
					if depth <= 0 {
						break scan
					}
				}
				i++
			}
			word := s[start:i]
			if !validContentWord(word, out.HasAlt) {
				return Content{}, false
			}
			*items = append(*items, ContentItem{Other: word})
		}
	}
	if len(out.Items) == 0 || (out.HasAlt && len(out.Alt) == 0) {
		return Content{}, false
	}
	return out, true
}

// validContentWord reports whether a keyword or a function is an item of
// the property content. The alternative text takes strings, attr() and
// counters only.
func validContentWord(word string, alt bool) bool {
	lower := strings.ToLower(word)
	if open := strings.IndexByte(lower, '('); open > 0 && strings.HasSuffix(lower, ")") {
		switch lower[:open] {
		case "attr", "counter", "counters":
			return true
		case "url", "image-set", "-webkit-image-set", "linear-gradient", "radial-gradient", "conic-gradient",
			"repeating-linear-gradient", "repeating-radial-gradient", "repeating-conic-gradient", "light-dark":
			return !alt
		}
		return false
	}
	switch lower {
	case "open-quote", "close-quote", "no-open-quote", "no-close-quote":
		return !alt
	}
	return false
}

// WithAttrs returns the content with each attr() item replaced by a string:
// the value that get returns for the name of the attribute. A browser does
// this at computed-value time. An attr() with a type or a fallback stays as
// it is. Strings that then follow each other are one item.
func (c Content) WithAttrs(get func(name string) string) Content {
	has := false
	for _, it := range c.Items {
		if _, ok := attrName(it.Other); ok {
			has = true
		}
	}
	if !has {
		return c
	}
	items := make([]ContentItem, 0, len(c.Items))
	for _, it := range c.Items {
		if name, ok := attrName(it.Other); ok {
			it = ContentItem{Text: get(name)}
			// An attr() item is a string also when it is empty.
			if n := len(items); n > 0 && items[n-1].Other == "" {
				items[n-1].Text += it.Text
				continue
			}
		} else if n := len(items); it.Other == "" && n > 0 && items[n-1].Other == "" {
			items[n-1].Text += it.Text
			continue
		}
		items = append(items, it)
	}
	c.Items = items
	return c
}

// attrName returns the name of the attribute of an item attr(name).
func attrName(item string) (string, bool) {
	if len(item) < 7 || !strings.EqualFold(item[:5], "attr(") || item[len(item)-1] != ')' {
		return "", false
	}
	name := strings.TrimSpace(item[5 : len(item)-1])
	if name == "" || !isIdent(name) {
		return "", false
	}
	return name, true
}

// Text returns the text that the items make: the strings. The second result
// is false when the content has an item that is not a string.
func (c Content) Text() (string, bool) {
	var sb strings.Builder
	onlyText := true
	for _, it := range c.Items {
		if it.Other != "" {
			onlyText = false
			continue
		}
		sb.WriteString(it.Text)
	}
	return sb.String(), onlyText
}

// CSSText returns the computed value as getComputedStyle prints it.
func (c Content) CSSText() string {
	switch c.Kind {
	case ContentNormal:
		return "normal"
	case ContentNone:
		return "none"
	}
	var sb strings.Builder
	write := func(items []ContentItem) {
		for i, it := range items {
			if i > 0 {
				sb.WriteByte(' ')
			}
			if it.Other != "" {
				sb.WriteString(it.Other)
			} else {
				sb.WriteString(SerializeString(it.Text))
			}
		}
	}
	write(c.Items)
	if c.HasAlt {
		sb.WriteString(" / ")
		write(c.Alt)
	}
	return sb.String()
}

func skipCSSSpace(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r' || s[i] == '\f') {
		i++
	}
	return i
}

func isCSSNewline(c byte) bool { return c == '\n' || c == '\r' || c == '\f' }

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// ConsumeString is CSSTokenizer::ConsumeStringTokenUntil: it reads the
// string token whose quote is at s[i]. It returns the value of the string
// and the index after the token. The last result is false for a string
// that a line break ends, which is not a valid token. A string that the end
// of the text ends is valid.
func ConsumeString(s string, i int) (value string, next int, ok bool) {
	quote := s[i]
	i++
	// A string without an escape is a part of the text.
	for j := i; j < len(s); j++ {
		c := s[j]
		if c == quote {
			return s[i:j], j + 1, true
		}
		if isCSSNewline(c) {
			return "", j, false
		}
		if c == 0 || c == '\\' {
			break
		}
		if j == len(s)-1 {
			return s[i:], len(s), true
		}
	}
	var sb strings.Builder
	for i < len(s) {
		c := s[i]
		i++
		switch {
		case c == quote:
			return sb.String(), i, true
		case isCSSNewline(c):
			return "", i - 1, false
		case c == '\\':
			if i >= len(s) {
				// A backslash at the end of the text is dropped.
				continue
			}
			if isCSSNewline(s[i]) {
				// An escaped line break continues the string.
				if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
					i++
				}
				i++
				continue
			}
			var r rune
			r, i = consumeEscape(s, i)
			sb.WriteRune(r)
		case c == 0:
			sb.WriteRune(utf8.RuneError)
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String(), i, true
}

// consumeEscape is ConsumeEscape: it reads an escape whose backslash is
// before s[i], which is not a line break. An escape is one to six
// hexadecimal digits with one optional white space after them, or one
// character.
func consumeEscape(s string, i int) (rune, int) {
	if isHexDigit(s[i]) {
		start := i
		for i < len(s) && i-start < 6 && isHexDigit(s[i]) {
			i++
		}
		v, _ := strconv.ParseUint(s[start:i], 16, 32)
		// One white space after the digits is a part of the escape.
		if i < len(s) {
			switch {
			case s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n':
				i += 2
			case s[i] == ' ' || s[i] == '\t' || isCSSNewline(s[i]):
				i++
			}
		}
		if v == 0 || (v >= 0xD800 && v <= 0xDFFF) || v > 0x10FFFF {
			return utf8.RuneError, i
		}
		return rune(v), i
	}
	r, size := utf8.DecodeRuneInString(s[i:])
	return r, i + size
}

// SerializeString is "serialize a string" of the CSS Object Model: the
// string in double quotes, with an escape for a quote, a backslash and a
// control character.
func SerializeString(s string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range s {
		switch {
		case r <= 0x1f || r == 0x7f:
			sb.WriteByte('\\')
			sb.WriteString(strconv.FormatInt(int64(r), 16))
			sb.WriteByte(' ')
		case r == '"' || r == '\\':
			sb.WriteByte('\\')
			sb.WriteRune(r)
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}
