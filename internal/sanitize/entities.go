// Ported from mermaid 12.2.0, packages/mermaid/src/utils.ts: encodeEntities,
// decodeEntities and formatUrl (MIT).

package sanitize

import (
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

var (
	// /style.*:\S*#.*;/g
	styleHashRegex = regexp.MustCompile(`style[^` + jsLineTerminator + `]*:[^` + jsSpace + `]*#[^` + jsLineTerminator + `]*;`)
	// /classDef.*:\S*#.*;/g
	classDefHashRegex = regexp.MustCompile(`classDef[^` + jsLineTerminator + `]*:[^` + jsSpace + `]*#[^` + jsLineTerminator + `]*;`)
	// /#\w+;/g
	entityRegex = regexp.MustCompile(`#\w+;`)
)

// isInt is /^\+?\d+$/.test(s).
func isInt(s string) bool {
	s = strings.TrimPrefix(s, "+")
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// EncodeEntities is encodeEntities: it replaces the #name; and #number;
// placeholders of a diagram text with markers that the grammars accept.
func EncodeEntities(text string) string {
	txt := text
	dropLast := func(s string) string { return s[:len(s)-1] }
	txt = styleHashRegex.ReplaceAllStringFunc(txt, dropLast)
	txt = classDefHashRegex.ReplaceAllStringFunc(txt, dropLast)
	txt = entityRegex.ReplaceAllStringFunc(txt, func(s string) string {
		innerTxt := s[1 : len(s)-1]
		if isInt(innerTxt) {
			return "ﬂ°°" + innerTxt + "¶ß"
		}
		return "ﬂ°" + innerTxt + "¶ß"
	})
	return txt
}

// DecodeEntities partially reverts the encoding of EncodeEntities
// (decodeEntities).
func DecodeEntities(text string) string {
	text = strings.ReplaceAll(text, "ﬂ°°", "&#")
	text = strings.ReplaceAll(text, "ﬂ°", "&")
	return strings.ReplaceAll(text, "¶ß", ";")
}

// FormatUrl formats a URL string (formatUrl). The second result is false
// when upstream returns undefined.
func FormatUrl(linkStr string, config *js.Obj[any]) (string, bool) {
	url := js.TrimSpace(linkStr)
	if url == "" {
		return "", false
	}
	if get(config, "securityLevel") != "loose" {
		return SanitizeUrl(url), true
	}
	return url, true
}
