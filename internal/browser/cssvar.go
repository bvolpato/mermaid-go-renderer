// This file ports no upstream file. It has the CSS function var() and the
// custom properties, as the CSS specification defines them.

package browser

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

// This file is the CSS function var() and the custom properties that it
// reads (CSS Custom Properties for Cascading Variables, level 1).
//
// A custom property inherits. A declaration whose var() reference has no
// value and no fallback is invalid at computed-value time: the property then
// acts as if it had the value `unset`.

// maxVarDepth bounds the chain of custom properties that refer to each
// other. A longer chain is a cycle.
const maxVarDepth = 32

// unsetValue is the value of a property whose declaration is invalid at
// computed-value time.
const unsetValue = "unset"

// isIdentChar reports whether c can be part of a CSS identifier.
func isIdentChar(c byte) bool {
	return c == '-' || c == '_' || c >= 0x80 ||
		(c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// skipCSSString returns the index after the quoted string that starts at i.
func skipCSSString(s string, i int) int {
	quote := s[i]
	for i++; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case quote:
			return i + 1
		}
	}
	return len(s)
}

// findVar returns the index of the next var( function at or after from, or
// -1. Text in a quoted string is skipped.
func findVar(s string, from int) int {
	for i := from; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"' || c == '\'':
			i = skipCSSString(s, i) - 1
		case (c == 'v' || c == 'V') && i+4 <= len(s) && strings.EqualFold(s[i:i+4], "var(") &&
			(i == 0 || !isIdentChar(s[i-1])):
			return i
		}
	}
	return -1
}

// hasVar reports whether a value has a var() reference.
func hasVar(value string) bool {
	return findVar(value, 0) >= 0
}

// closeParen returns the index of the parenthesis that closes the one at
// open, or -1.
func closeParen(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '"', '\'':
			i = skipCSSString(s, i) - 1
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// splitVarArguments splits the arguments of var() into the property name
// and the fallback, at the first comma that is not nested.
func splitVarArguments(s string) (name, fallback string, hasFallback bool) {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"', '\'':
			i = skipCSSString(s, i) - 1
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				return s[:i], s[i+1:], true
			}
		}
	}
	return s, "", false
}

// substituteVars replaces every var() reference of a value, as seen from the
// element n. The second result is false when a reference has no value.
func (e *Engine) substituteVars(n *dom.Node, value string, depth int) (string, bool) {
	if depth > maxVarDepth {
		return "", false
	}
	var out strings.Builder
	for i := 0; i < len(value); {
		start := findVar(value, i)
		if start < 0 {
			out.WriteString(value[i:])
			break
		}
		out.WriteString(value[i:start])
		end := closeParen(value, start+3)
		if end < 0 {
			return "", false
		}
		name, fallback, hasFallback := splitVarArguments(value[start+4 : end])
		if v, ok := e.customProperty(n, strings.TrimSpace(name), depth); ok {
			out.WriteString(v)
		} else if hasFallback {
			v, ok := e.substituteVars(n, fallback, depth+1)
			if !ok {
				return "", false
			}
			out.WriteString(strings.TrimSpace(v))
		} else {
			return "", false
		}
		i = end + 1
	}
	return strings.TrimSpace(out.String()), true
}

// customProperty returns the computed value of a custom property on the
// element n: the cascaded value of the nearest element that declares it,
// with its own var() references replaced. The second result is false when
// the property has no value.
func (e *Engine) customProperty(n *dom.Node, name string, depth int) (string, bool) {
	if !strings.HasPrefix(name, "--") {
		return "", false
	}
	for cur := n; cur != nil && cur.Type == dom.ElementNode; cur = cur.Parent {
		raw, ok := e.styleOf(cur).declared[name]
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "inherit", "unset", "revert", "revert-layer":
			continue
		case "initial":
			return "", false
		}
		if !hasVar(raw) {
			return strings.TrimSpace(raw), true
		}
		return e.substituteVars(cur, raw, depth+1)
	}
	return "", false
}

// longhandsOf returns the properties that a declaration of name sets in the
// cascade of this engine: the longhands that expandDecl writes for a
// shorthand, or the property itself.
func longhandsOf(name string) []string {
	switch name {
	case "font":
		return []string{"font-style", "font-weight", "font-size", "line-height", "font-family"}
	case "margin", "padding":
		return []string{name + "-top", name + "-right", name + "-bottom", name + "-left"}
	case "margin-block", "padding-block":
		prefix := name[:strings.IndexByte(name, '-')]
		return []string{prefix + "-top", prefix + "-bottom"}
	case "margin-inline", "padding-inline":
		prefix := name[:strings.IndexByte(name, '-')]
		return []string{prefix + "-left", prefix + "-right"}
	case "margin-block-start", "padding-block-start":
		return []string{name[:strings.IndexByte(name, '-')] + "-top"}
	case "margin-block-end", "padding-block-end":
		return []string{name[:strings.IndexByte(name, '-')] + "-bottom"}
	case "margin-inline-start", "padding-inline-start":
		return []string{name[:strings.IndexByte(name, '-')] + "-left"}
	case "margin-inline-end", "padding-inline-end":
		return []string{name[:strings.IndexByte(name, '-')] + "-right"}
	case "background":
		return []string{"background", "background-color"}
	case "border-width", "border-style":
		suffix := name[len("border"):]
		return []string{"border-top" + suffix, "border-right" + suffix, "border-bottom" + suffix, "border-left" + suffix}
	}
	return []string{name}
}

// resolveVars is the value of a declaration of a property that is not a
// custom property, with every var() reference replaced. The second result
// is false when the declaration is invalid at computed-value time: a
// reference has no value, or nothing is left of the value.
func (e *Engine) resolveVars(n *dom.Node, value string) (string, bool) {
	resolved, ok := e.substituteVars(n, value, 0)
	return resolved, ok && resolved != ""
}
