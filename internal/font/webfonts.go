// This file ports no upstream file.

package font

// WithWebFonts returns a library that has the @font-face rules of the page
// of mermaid-cli (internal/webfonts) in front of the fonts of l: the
// families Recursive Variable, Open Sans and Font Awesome. The platform
// rules are those of l.
//
// TODO: the library has no web fonts yet, and the result is l.
func (l *Library) WithWebFonts() *Library {
	return l
}
