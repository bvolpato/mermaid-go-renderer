// This file ports no upstream file. It has the parser of CSS declarations
// and style sheets.

package css

import (
	"strings"
)

// Decl is one declaration of a style rule or of a style attribute.
type Decl struct {
	Name      string // lowercase, except for a custom property
	Value     string
	Important bool
	// Raw is the value as written, for a declaration of ParseInlineStyle,
	// whose Value is the canonical text. SetProperty reads it to tell
	// values apart that have the same canonical text.
	Raw string
}

// ParseDeclarations parses the content of a declaration block, such as the
// value of a style attribute. Invalid pieces are skipped, as a browser does.
// Values are kept as written, trimmed. Names are lowercased.
func ParseDeclarations(text string) []Decl {
	var out []Decl
	for _, part := range splitTopLevel(stripComments(text), ';') {
		colon := indexTopLevel(part, ':')
		if colon < 0 {
			continue
		}
		name := strings.TrimSpace(part[:colon])
		value := strings.TrimSpace(part[colon+1:])
		if name == "" || value == "" || !isIdent(name) {
			continue
		}
		d := Decl{Name: name, Value: value}
		if !strings.HasPrefix(name, "--") {
			d.Name = strings.ToLower(name)
		}
		if i := strings.LastIndexByte(value, '!'); i >= 0 && strings.EqualFold(strings.TrimSpace(value[i+1:]), "important") {
			d.Important = true
			d.Value = strings.TrimSpace(value[:i])
			if d.Value == "" {
				continue
			}
		}
		out = append(out, d)
	}
	return out
}

func isIdent(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c == '-' || c == '_' || c >= 0x80 || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return s != ""
}

func stripComments(s string) string {
	if !strings.Contains(s, "/*") {
		return s
	}
	var sb strings.Builder
	for {
		i := strings.Index(s, "/*")
		if i < 0 {
			sb.WriteString(s)
			break
		}
		sb.WriteString(s[:i])
		j := strings.Index(s[i+2:], "*/")
		if j < 0 {
			break
		}
		s = s[i+2+j+2:]
	}
	return sb.String()
}

// splitTopLevel splits on sep outside of strings, parentheses and brackets.
func splitTopLevel(s string, sep byte) []string {
	var parts []string
	depth := 0
	var quote byte
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			if depth > 0 {
				depth--
			}
		case c == sep && depth == 0:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

func indexTopLevel(s string, sep byte) int {
	depth := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '(' || c == '[':
			depth++
		case c == ')' || c == ']':
			if depth > 0 {
				depth--
			}
		case c == sep && depth == 0:
			return i
		}
	}
	return -1
}

// Rule is a style rule: a selector list and its declarations.
type Rule struct {
	Selector string
	Decls    []Decl
}

// ParseStylesheet returns the style rules of a sheet in source order.
// At-rules are skipped, except that the rules inside "@media all" and
// "@media screen" blocks are included.
func ParseStylesheet(text string) []Rule {
	var rules []Rule
	parseRules(stripComments(text), &rules)
	return rules
}

func parseRules(s string, rules *[]Rule) {
	for {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		open := indexTopLevelBrace(s)
		if open < 0 {
			return
		}
		closeIdx := matchBrace(s, open)
		prelude := strings.TrimSpace(s[:open])
		body := s[open+1 : closeIdx]
		if closeIdx+1 <= len(s) {
			s = s[min(closeIdx+1, len(s)):]
		} else {
			s = ""
		}
		// A statement at-rule before this rule ("@import ...;") ends at ';'.
		if i := strings.LastIndexByte(prelude, ';'); i >= 0 {
			prelude = strings.TrimSpace(prelude[i+1:])
		}
		if strings.HasPrefix(prelude, "@") {
			lower := strings.ToLower(prelude)
			if strings.HasPrefix(lower, "@media") {
				q := strings.TrimSpace(lower[len("@media"):])
				if q == "" || q == "all" || q == "screen" {
					parseRules(body, rules)
				}
			}
			continue
		}
		if prelude == "" {
			continue
		}
		*rules = append(*rules, Rule{Selector: prelude, Decls: ParseDeclarations(body)})
	}
}

func indexTopLevelBrace(s string) int {
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '{':
			return i
		}
	}
	return -1
}

func matchBrace(s string, open int) int {
	depth := 0
	var quote byte
	for i := open; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(s)
}
