// This file ports no upstream file. It has the part of the CSSOM that
// mermaid uses for config.themeCSS: it parses a style sheet and prints each
// rule as a browser prints it.

package render

import (
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/css"
)

// This file has the part of the CSSOM that createCssStyles of mermaidAPI.ts
// uses for config.themeCSS: CSSStyleSheet.replaceSync(text), then the cssText
// of every rule. The browser parses the sheet and prints each rule in
// canonical form, so "fill: #ffffff" becomes "fill: rgb(255, 255, 255)".
//
// Style rules, conditional group rules (@media, @supports, @layer with a
// block, @container), @keyframes, @font-face and @page are handled. Nested
// style rules and namespace prefixes are not: a rule that uses them is
// printed without them.

// styleSheetRules returns the cssText of the rules of a sheet after
// replaceSync(text).
func styleSheetRules(text string) []string {
	return parseSheetRules(stripCSSComments(text), true)
}

// parseSheetRules reads a list of rules. At the top level a stray "}" ends
// the sheet content that the browser can recover.
func parseSheetRules(s string, top bool) []string {
	var out []string
	for {
		s = strings.TrimLeft(s, " \t\n\r\f")
		if s == "" {
			return out
		}
		if s[0] == ';' {
			// A stray ";" starts a qualified rule that takes the next block as
			// its (invalid) content.
			open := indexCSS(s, '{')
			if open < 0 {
				return out
			}
			s = s[cssBlockEnd(s, open):]
			continue
		}
		if s[0] == '}' {
			if top {
				// The "}" starts an invalid qualified rule that runs to the end
				// of the next block.
				open := indexCSS(s, '{')
				if open < 0 {
					return out
				}
				s = s[cssBlockEnd(s, open):]
				continue
			}
			return out
		}
		open := indexCSS(s, '{')
		semi := indexCSS(s, ';')
		if s[0] == '@' && semi >= 0 && (open < 0 || semi < open) {
			// A statement at-rule. replaceSync drops @import, and @charset is
			// not a rule. @namespace stays.
			stmt := strings.TrimSpace(s[:semi])
			s = s[semi+1:]
			if rule, ok := namespaceRuleText(stmt); ok {
				out = append(out, rule)
			}
			continue
		}
		if open < 0 {
			return out
		}
		end := cssBlockEnd(s, open)
		prelude := strings.TrimSpace(s[:open])
		bodyEnd := end
		if bodyEnd > open && bodyEnd <= len(s) && s[bodyEnd-1] == '}' {
			bodyEnd--
		}
		body := s[open+1 : bodyEnd]
		s = s[end:]
		if prelude == "" {
			continue
		}
		if prelude[0] == '@' {
			if rule, ok := atRuleText(prelude, body); ok {
				out = append(out, rule)
			}
			continue
		}
		selector, ok := serializeSelectorList(prelude)
		if !ok {
			continue
		}
		out = append(out, selector+" "+declarationBlockText(body))
	}
}

// declarationBlockText prints "{ a: b; c: d; }" for the declarations of a
// rule body.
func declarationBlockText(body string) string {
	text := css.SerializeInlineStyle(css.ParseInlineStyle(body, false))
	if text == "" {
		return "{ }"
	}
	return "{ " + text + " }"
}

func atRuleText(prelude, body string) (string, bool) {
	name := prelude
	rest := ""
	if i := strings.IndexAny(prelude, " \t\n\r\f("); i >= 0 {
		name, rest = prelude[:i], strings.TrimSpace(prelude[i:])
	}
	rest = strings.Join(strings.Fields(rest), " ")
	switch strings.ToLower(name) {
	case "@media", "@supports", "@layer", "@container":
		var b strings.Builder
		b.WriteString(strings.ToLower(name))
		if rest != "" {
			b.WriteString(" " + rest)
		}
		b.WriteString(" {\n")
		for _, rule := range parseSheetRules(body, false) {
			b.WriteString("  " + rule + "\n")
		}
		b.WriteString("}")
		return b.String(), true
	case "@keyframes":
		var b strings.Builder
		b.WriteString("@keyframes " + rest + " { \n")
		for _, frame := range keyframeRules(body) {
			b.WriteString("  " + frame + "\n")
		}
		b.WriteString("}")
		return b.String(), true
	case "@font-face", "@page":
		head := strings.ToLower(name)
		if rest != "" {
			head += " " + rest
		}
		return head + " " + declarationBlockText(body), true
	}
	return "", false
}

// keyframeRules prints the keyframes of an @keyframes body. The keywords
// from and to print as percentages.
func keyframeRules(body string) []string {
	var out []string
	s := body
	for {
		s = strings.TrimLeft(s, " \t\n\r\f")
		open := indexCSS(s, '{')
		if open < 0 {
			return out
		}
		end := cssBlockEnd(s, open)
		bodyEnd := end
		if s[bodyEnd-1] == '}' {
			bodyEnd--
		}
		var keys []string
		for _, k := range strings.Split(s[:open], ",") {
			k = strings.ToLower(strings.TrimSpace(k))
			switch k {
			case "from":
				k = "0%"
			case "to":
				k = "100%"
			}
			keys = append(keys, k)
		}
		out = append(out, strings.Join(keys, ", ")+" "+declarationBlockText(s[open+1:bodyEnd]))
		s = s[end:]
	}
}

func namespaceRuleText(stmt string) (string, bool) {
	fields := strings.Fields(stmt)
	if len(fields) < 2 || !strings.EqualFold(fields[0], "@namespace") {
		return "", false
	}
	uri := fields[len(fields)-1]
	if strings.HasPrefix(strings.ToLower(uri), "url(") && strings.HasSuffix(uri, ")") {
		uri = strings.Trim(uri[4:len(uri)-1], `"' `)
	} else {
		uri = strings.Trim(uri, `"'`)
	}
	prefix := ""
	if len(fields) == 3 {
		prefix = fields[1] + " "
	}
	return "@namespace " + prefix + `url("` + uri + `");`, true
}

// serializeSelectorList prints a selector list as the CSSOM does: one space
// around a combinator, ", " between selectors, and type selectors in lower
// case. It reports false for a selector that the sheet drops.
func serializeSelectorList(s string) (string, bool) {
	var b strings.Builder
	depth := 0
	var quote byte
	pendingSpace := false
	compoundStart := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				i++
				b.WriteByte(s[i])
			} else if c == quote {
				quote = 0
			}
			continue
		}
		if depth > 0 {
			switch c {
			case '"', '\'':
				quote = c
			case '(', '[':
				depth++
			case ')', ']':
				depth--
			}
			b.WriteByte(c)
			continue
		}
		switch c {
		case ' ', '\t', '\n', '\r', '\f':
			pendingSpace = true
			continue
		case ',':
			b.WriteString(", ")
			pendingSpace = false
			compoundStart = true
			continue
		case '>', '~', '+':
			b.WriteString(" " + string(c) + " ")
			pendingSpace = false
			compoundStart = true
			continue
		case '|':
			// A namespace prefix needs an @namespace rule, which this code
			// does not track.
			return "", false
		}
		if pendingSpace {
			if b.Len() > 0 && !compoundStart {
				b.WriteByte(' ')
			}
			pendingSpace = false
			compoundStart = true
		}
		if c == '[' {
			// An attribute selector prints its value in double quotes.
			if end := strings.IndexByte(s[i:], ']'); end > 0 {
				if attr, ok := serializeAttributeSelector(s[i+1 : i+end]); ok {
					b.WriteString("[" + attr + "]")
					i += end
					compoundStart = false
					continue
				}
			}
		}
		switch c {
		case '"', '\'':
			quote = c
		case '(', '[':
			depth++
		}
		if compoundStart && isCSSIdentStart(c) {
			j := i
			for j < len(s) && isCSSIdentChar(s[j]) {
				j++
			}
			b.WriteString(strings.ToLower(s[i:j]))
			i = j - 1
			compoundStart = false
			continue
		}
		compoundStart = false
		b.WriteByte(c)
	}
	out := strings.TrimSpace(b.String())
	return out, out != ""
}

func isCSSIdentStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c >= 0x80
}

func isCSSIdentChar(c byte) bool {
	return isCSSIdentStart(c) || c >= '0' && c <= '9' || c == '-'
}

// indexCSS returns the index of the first c outside strings and
// parentheses, or -1.
func indexCSS(s string, c byte) int {
	var quote byte
	depth := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case quote != 0:
			if ch == '\\' {
				i++
			} else if ch == quote {
				quote = 0
			}
		case ch == '"' || ch == '\'':
			quote = ch
		case ch == c && depth == 0:
			return i
		case ch == '(' || ch == '[':
			depth++
		case ch == ')' || ch == ']':
			if depth > 0 {
				depth--
			}
		}
	}
	return -1
}

// cssBlockEnd returns the index after the "}" that closes the block that
// opens at s[open]. An open block at the end of the text closes there.
func cssBlockEnd(s string, open int) int {
	var quote byte
	depth := 0
	for i := open; i < len(s); i++ {
		ch := s[i]
		switch {
		case quote != 0:
			if ch == '\\' {
				i++
			} else if ch == quote {
				quote = 0
			}
		case ch == '"' || ch == '\'':
			quote = ch
		case ch == '{':
			depth++
		case ch == '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(s)
}

// stripCSSComments removes /* */ comments outside strings.
func stripCSSComments(s string) string {
	if !strings.Contains(s, "/*") {
		return s
	}
	var b strings.Builder
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				i++
				b.WriteByte(s[i])
			} else if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			b.WriteByte(c)
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '*' {
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				break
			}
			i += end + 3
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

var attributeSelector = regexp.MustCompile(`^\s*([^\s~|^$*=\]]+)\s*(?:([~|^$*]?=)\s*("[^"]*"|'[^']*'|[^\s"'\]]+)\s*([iIsS])?)?\s*$`)

// serializeAttributeSelector prints the inside of [name op value flag] as
// the CSSOM does: no white space, and the value in double quotes.
func serializeAttributeSelector(inner string) (string, bool) {
	m := attributeSelector.FindStringSubmatch(inner)
	if m == nil {
		return "", false
	}
	if m[2] == "" {
		return m[1], true
	}
	value := m[3]
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') {
		value = value[1 : len(value)-1]
	}
	value = strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`)
	out := m[1] + m[2] + `"` + value + `"`
	if m[4] != "" {
		out += " " + strings.ToLower(m[4])
	}
	return out, true
}
