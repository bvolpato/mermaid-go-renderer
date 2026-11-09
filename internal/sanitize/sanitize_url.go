// Ported from @braintree/sanitize-url 7.1.2, dist/index.js and
// dist/constants.js.

package sanitize

import (
	"math"
	"math/big"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// BlankURL is BLANK_URL.
const BlankURL = "about:blank"

var (
	// htmlEntitiesRegex = /&#(\w+)(^\w|;)?/g. The alternative ^\w can never
	// match after the first group, so the optional group is an optional ";".
	htmlEntitiesRegex = regexp.MustCompile(`&#(\w+);?`)
	// htmlCtrlEntityRegex = /&(newline|tab);/gi
	htmlCtrlEntityRegex = regexp.MustCompile(`&([nN][eE][wW][lL][iI][nN][eE]|[tT][aA][bB]);`)
	// whitespaceEscapeCharsRegex = /(\\|%5[cC])((%(6[eE]|72|74))|[nrt])/g
	whitespaceEscapeCharsRegex = regexp.MustCompile(`(\\|%5[cC])((%(6[eE]|72|74))|[nrt])`)
	// urlSchemeRegex = /^.+(:|&colon;)/gim. Only the first match is used. With
	// the m flag, ^ matches after every line terminator of JavaScript. The ^
	// of Go knows the line feed only, so the terminator is matched here, and
	// the first group is the match of the upstream expression.
	urlSchemeRegex = regexp.MustCompile(`(?:^|[` + jsLineTerminator + `])([^` + jsLineTerminator + `]+(?::|&[cC][oO][lL][oO][nN];))`)
)

// isCtrlCharacter reports whether r is in the class of ctrlCharactersRegex:
// the code points U+0000 to U+001F, U+007F to U+009F, U+2000 to U+200D and
// U+FEFF.
func isCtrlCharacter(r rune) bool {
	return r <= 0x1F || (r >= 0x7F && r <= 0x9F) || (r >= 0x2000 && r <= 0x200D) || r == 0xFEFF
}

func hasCtrlCharacters(s string) bool {
	return strings.IndexFunc(s, isCtrlCharacter) >= 0
}

// removeCtrlCharacters is str.replace(ctrlCharactersRegex, "").
func removeCtrlCharacters(s string) string {
	if !hasCtrlCharacters(s) {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		// An invalid byte decodes as U+FFFD with size 1 and is kept.
		if size == 1 && r == utf8.RuneError || !isCtrlCharacter(r) {
			sb.WriteString(s[i : i+size])
		}
		i += size
	}
	return sb.String()
}

// isRelativeUrlWithoutProtocol is isRelativeUrlWithoutProtocol.
func isRelativeUrlWithoutProtocol(url string) bool {
	return url != "" && (url[0] == '.' || url[0] == '/')
}

// stringToNumber is Number(s) for a string of word characters.
func stringToNumber(s string) float64 {
	if len(s) > 2 && s[0] == '0' {
		radix := 0
		switch s[1] {
		case 'x', 'X':
			radix = 16
		case 'o', 'O':
			radix = 8
		case 'b', 'B':
			radix = 2
		}
		if radix != 0 {
			v, ok := new(big.Int).SetString(s[2:], radix)
			if !ok || strings.ContainsAny(s[2:], "_+-") {
				return math.NaN()
			}
			f, _ := new(big.Float).SetInt(v).Float64()
			return f
		}
	}
	return js.Number(s)
}

// fromCharCode is String.fromCharCode(dec) for a string argument: the string
// converts to a number, and the number to a UTF-16 code unit.
func fromCharCode(dec string) rune {
	return rune(js.ToUint32(stringToNumber(dec)) & 0xFFFF)
}

// decodeHtmlCharacters is decodeHtmlCharacters.
func decodeHtmlCharacters(str string) string {
	removedNullByte := removeCtrlCharacters(str)
	if !strings.Contains(removedNullByte, "&#") {
		return removedNullByte
	}
	surrogate := false
	out := htmlEntitiesRegex.ReplaceAllStringFunc(removedNullByte, func(match string) string {
		dec := strings.TrimSuffix(match[2:], ";")
		unit := fromCharCode(dec)
		if unit >= 0xD800 && unit < 0xE000 {
			surrogate = true
		}
		return js.FromUTF16Units([]rune{unit})
	})
	if surrogate {
		// Two halves of a surrogate pair that now are adjacent are one
		// character in a JavaScript string.
		out = js.FromUTF16Units(js.UTF16Units(out))
	}
	return out
}

func hexValue(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// percentByte reads the escape %XX at s[i:].
func percentByte(s string, i int) (byte, bool) {
	if i+2 >= len(s) || s[i] != '%' {
		return 0, false
	}
	hi, ok1 := hexValue(s[i+1])
	lo, ok2 := hexValue(s[i+2])
	return hi<<4 | lo, ok1 && ok2
}

// decodeURIComponent is the global function decodeURIComponent. The second
// result is false when it throws a URIError: for an escape that is not two
// hexadecimal digits, and for escapes that are not the UTF-8 encoding of a
// code point.
func decodeURIComponent(s string) (string, bool) {
	if strings.IndexByte(s, '%') < 0 {
		return s, true
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != '%' {
			sb.WriteByte(s[i])
			i++
			continue
		}
		b, ok := percentByte(s, i)
		if !ok {
			return "", false
		}
		i += 3
		if b < 0x80 {
			sb.WriteByte(b)
			continue
		}
		n := 0
		switch {
		case b&0xE0 == 0xC0:
			n = 2
		case b&0xF0 == 0xE0:
			n = 3
		case b&0xF8 == 0xF0:
			n = 4
		default:
			return "", false
		}
		octets := [4]byte{b}
		for j := 1; j < n; j++ {
			b2, ok := percentByte(s, i)
			if !ok || b2&0xC0 != 0x80 {
				return "", false
			}
			octets[j] = b2
			i += 3
		}
		if r, size := utf8.DecodeRune(octets[:n]); r == utf8.RuneError || size != n {
			return "", false
		}
		sb.Write(octets[:n])
	}
	return sb.String(), true
}

// decodeURI is decodeURI of sanitize-url: decodeURIComponent, or the input
// when that throws.
func decodeURI(uri string) string {
	if decoded, ok := decodeURIComponent(uri); ok {
		return decoded
	}
	return uri
}

// invalidProtocol is invalidProtocolRegex.test(urlScheme) with
// invalidProtocolRegex = /^([^\w]*)(javascript|data|vbscript)/im. The scheme
// is one line in lowercase, so the expression can match only at the start.
func invalidProtocol(urlScheme string) bool {
	rest := strings.TrimLeftFunc(urlScheme, func(r rune) bool {
		return !(r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'))
	})
	for _, protocol := range []string{"javascript", "data", "vbscript"} {
		if len(rest) >= len(protocol) && strings.EqualFold(rest[:len(protocol)], protocol) && isASCII(rest[:len(protocol)]) {
			return true
		}
	}
	return false
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// SanitizeUrl is sanitizeUrl.
func SanitizeUrl(url string) string {
	if url == "" {
		return BlankURL
	}
	decodedUrl := decodeURI(js.TrimSpace(url))
	for {
		decodedUrl = decodeHtmlCharacters(decodedUrl)
		decodedUrl = htmlCtrlEntityRegex.ReplaceAllLiteralString(decodedUrl, "")
		decodedUrl = removeCtrlCharacters(decodedUrl)
		decodedUrl = whitespaceEscapeCharsRegex.ReplaceAllLiteralString(decodedUrl, "")
		decodedUrl = js.TrimSpace(decodedUrl)
		decodedUrl = decodeURI(decodedUrl)
		charsToDecode := hasCtrlCharacters(decodedUrl) ||
			htmlEntitiesRegex.MatchString(decodedUrl) ||
			htmlCtrlEntityRegex.MatchString(decodedUrl) ||
			whitespaceEscapeCharsRegex.MatchString(decodedUrl)
		if !charsToDecode {
			break
		}
	}
	sanitizedUrl := decodedUrl
	if sanitizedUrl == "" {
		return BlankURL
	}
	if isRelativeUrlWithoutProtocol(sanitizedUrl) {
		return sanitizedUrl
	}
	// Remove any leading whitespace before checking the URL scheme.
	trimmedUrl := js.TrimStart(sanitizedUrl)
	urlSchemeParseResults := urlSchemeRegex.FindStringSubmatch(trimmedUrl)
	if urlSchemeParseResults == nil {
		return sanitizedUrl
	}
	urlScheme := js.TrimSpace(js.ToLowerCase(urlSchemeParseResults[1]))
	if invalidProtocol(urlScheme) {
		return BlankURL
	}
	backSanitized := strings.ReplaceAll(trimmedUrl, `\`, "/")
	// Handle special cases for mailto: and custom deep-link protocols.
	if urlScheme == "mailto:" || strings.Contains(urlScheme, "://") {
		return backSanitized
	}
	// For http and https URLs, perform additional validation.
	if urlScheme == "http:" || urlScheme == "https:" {
		// new URL(backSanitized), with the protocol and the hostname set to
		// their lowercase forms. The URL parser already gives both in
		// lowercase, so the two assignments change nothing.
		href, ok := parseHTTPURL(backSanitized)
		if !ok {
			return BlankURL
		}
		return href
	}
	return backSanitized
}
