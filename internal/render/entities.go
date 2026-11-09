// Ported from mermaid 12.2.0, utils.ts (encodeEntities, decodeEntities) and
// mermaidAPI.ts (cleanUpSvgCode) (MIT).

package render

import (
	"regexp"
	"strings"
)

var (
	reStyleHash    = regexp.MustCompile(`style.*:\S*#.*;`)
	reClassDefHash = regexp.MustCompile(`classDef.*:\S*#.*;`)
	reEntity       = regexp.MustCompile(`#\w+;`)
	reIsInt        = regexp.MustCompile(`^\+?\d+$`)
	reMarkerEnd    = regexp.MustCompile(`marker-end="url\([\d+./:=?A-Za-z-]*?#`)
)

// EncodeEntities is encodeEntities(text): it protects `#name;` entities of
// the diagram text from the parser.
func EncodeEntities(text string) string {
	if !strings.Contains(text, "#") {
		// Each of the three patterns has a '#'.
		return text
	}
	txt := reStyleHash.ReplaceAllStringFunc(text, func(s string) string { return s[:len(s)-1] })
	txt = reClassDefHash.ReplaceAllStringFunc(txt, func(s string) string { return s[:len(s)-1] })
	txt = reEntity.ReplaceAllStringFunc(txt, func(s string) string {
		inner := s[1 : len(s)-1]
		if reIsInt.MatchString(inner) {
			return "ﬂ°°" + inner + "¶ß"
		}
		return "ﬂ°" + inner + "¶ß"
	})
	return txt
}

// DecodeEntities is decodeEntities(text): it partially reverts EncodeEntities.
func DecodeEntities(text string) string {
	if !strings.Contains(text, "ﬂ°") && !strings.Contains(text, "¶ß") {
		return text
	}
	text = strings.ReplaceAll(text, "ﬂ°°", "&#")
	text = strings.ReplaceAll(text, "ﬂ°", "&")
	return strings.ReplaceAll(text, "¶ß", ";")
}

// cleanUpSvgCode is cleanUpSvgCode(svgCode, inSandboxMode, useArrowMarkerUrls).
func cleanUpSvgCode(svgCode string, inSandboxMode, useArrowMarkerUrls bool) string {
	cleaned := svgCode
	if !useArrowMarkerUrls && !inSandboxMode {
		cleaned = stripMarkerEndURL(cleaned)
	}
	cleaned = DecodeEntities(cleaned)
	return strings.ReplaceAll(cleaned, "<br>", "<br/>")
}

// stripMarkerEndURL is svgCode.replace(/marker-end="url\([\d+./:=?A-Za-z-]*?#/g,
// 'marker-end="url(#'): it removes the address of the page in front of the
// fragment of each marker reference. The regular expression reMarkerEnd is
// the definition, and a test compares the two. A marker reference of this
// renderer has no address, so the usual result is the text itself, and the
// regular expression would copy all of it.
func stripMarkerEndURL(svgCode string) string {
	const attr = `marker-end="url(`
	// sb gets the result from the first reference that has an address. It
	// stays empty for a text that does not change.
	var sb strings.Builder
	changed := false
	// done is the start of the part of svgCode that is not yet in sb.
	done := 0
	for pos := 0; ; {
		i := strings.Index(svgCode[pos:], attr)
		if i < 0 {
			break
		}
		// The address: the characters of the class up to the first other
		// character, which must be the '#'.
		start := pos + i + len(attr)
		end := start
		for end < len(svgCode) && isMarkerURLChar(svgCode[end]) {
			end++
		}
		if end == len(svgCode) || svgCode[end] != '#' {
			// No match here. The attribute name cannot start again inside
			// itself, so the search goes on after it.
			pos = start
			continue
		}
		if end > start {
			if !changed {
				changed = true
				sb.Grow(len(svgCode))
			}
			sb.WriteString(svgCode[done:start])
			done = end
		}
		pos = end + 1
	}
	if !changed {
		return svgCode
	}
	sb.WriteString(svgCode[done:])
	return sb.String()
}

// isMarkerURLChar reports whether c is in [\d+./:=?A-Za-z-].
func isMarkerURLChar(c byte) bool {
	switch {
	case c >= '0' && c <= '9', c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z':
		return true
	}
	switch c {
	case '+', '.', '/', ':', '=', '?', '-':
		return true
	}
	return false
}
