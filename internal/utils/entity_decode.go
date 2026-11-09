// Ported from mermaid 12.2.0, mermaid/src/utils.ts: entityDecode (MIT). The
// function sets the innerHTML of a detached div, so this file also has the
// part of the innerHTML setter of Chromium that decides between its two HTML
// parsers. A comment names html_document_parser_fastpath.cc of Blink for it
// (third_party/blink/renderer/core/html/parser, BSD-3-Clause at Chromium
// 156.0.8078.4).

package utils

import (
	"strings"

	"golang.org/x/net/html"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// EntityDecode decodes HTML entities. Upstream sets the innerHTML of a
// detached div and reads its textContent.
func EntityDecode(text string) string {
	decoder := dom.NewDocument().CreateElement("div")
	// Escape HTML before decoding for HTML Entities
	text = js.Escape(text)
	text = strings.ReplaceAll(text, "%26", "&")
	text = strings.ReplaceAll(text, "%23", "#")
	text = strings.ReplaceAll(text, "%3B", ";")
	// The two HTML parsers of Chromium do not agree on "&#x;", a
	// hexadecimal reference without digits. The fast path parser of
	// innerHTML reads it as U+FFFD, and so does golang.org/x/net/html. The
	// full parser keeps the four characters. Chromium uses the full parser
	// when the fast path parser gives up on the text.
	if (strings.Contains(text, "&#x;") || strings.Contains(text, "&#X;")) && !fastPathParsesText(text) {
		text = strings.ReplaceAll(text, "&#x;", "&amp;#x;")
		text = strings.ReplaceAll(text, "&#X;", "&amp;#X;")
	}
	decoder.HTML(text)

	return js.Unescape(decoder.TextContent())
}

// fastPathParsesText reports whether the fast path HTML parser of Chromium
// (html_document_parser_fastpath.cc, ScanHTMLCharacterReference) accepts a
// text that has no markup. The text is the output of escape with "&", "#"
// and ";" put back, so it has no "<", no carriage return and no U+0000. The
// parser gives up when a "&" is not the start of a character reference
// that ends with ";" within 21 characters.
func fastPathParsesText(text string) bool {
	// kMaxLength: the number of characters before the ";".
	const maxLength = 20
	for pos := 0; pos < len(text); {
		if text[pos] != '&' {
			pos++
			continue
		}
		start := pos + 1
		end := start
		for {
			if end == len(text) || end-start > maxLength {
				return false
			}
			end++
			if text[end-1] == ';' {
				break
			}
		}
		reference := text[start:end]
		// There are no valid character references shorter than that.
		if len(reference) < 3 {
			return false
		}
		switch {
		case reference[0] == '#':
			res := 0
			if reference[1] == 'x' || reference[1] == 'X' {
				for i := 2; i < len(reference)-1; i++ {
					c := reference[i]
					res *= 16
					switch {
					case c >= '0' && c <= '9':
						res += int(c - '0')
					case c >= 'a' && c <= 'f':
						res += int(c-'a') + 10
					case c >= 'A' && c <= 'F':
						res += int(c-'A') + 10
					default:
						return false
					}
					if res > 0x10ffff {
						return false
					}
				}
			} else {
				for i := 1; i < len(reference)-1; i++ {
					c := reference[i]
					res *= 10
					if c < '0' || c > '9' {
						return false
					}
					res += int(c - '0')
					if res > 0x10ffff {
						return false
					}
				}
			}
			pos = end
		case reference == "amp;" || reference == "lt;" || reference == "gt;" || reference == "nbsp;":
			pos = end
		default:
			// ConsumeHTMLEntity: the longest prefix of the reference that
			// is the name of an entity. The parser goes on after the name.
			n := namedEntityLength(reference)
			if n == 0 {
				return false
			}
			pos = start + n
		}
	}
	return true
}

// namedEntityLength returns the length of the longest prefix of reference
// that is the name of an HTML entity, with or without the ";" at its end,
// or 0. The table of the names is in golang.org/x/net/html, which does not
// export it: a prefix is a name when its last character changes what
// UnescapeString decodes.
func namedEntityLength(reference string) int {
	// A name has letters, digits and the ";" at the end.
	if i := strings.IndexByte(reference, '&'); i >= 0 {
		reference = reference[:i]
	}
	for n := len(reference); n > 0; n-- {
		if html.UnescapeString("&"+reference[:n]) != html.UnescapeString("&"+reference[:n-1])+reference[n-1:n] {
			return n
		}
	}
	return 0
}
