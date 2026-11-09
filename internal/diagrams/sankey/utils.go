// Ported from mermaid 12.2.0, diagrams/sankey/sankeyUtils.ts (MIT).

package sankey

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// isSpaceInLine reports whether r matches [^\S\n\r] in a JavaScript regular
// expression: white space that is not a line feed or a carriage return.
func isSpaceInLine(r rune) bool {
	return r != '\n' && r != '\r' && js.IsWhiteSpace(r)
}

// PrepareTextForParsing is prepareTextForParsing(text). The first regular
// expression of upstream has no multiline flag, so it removes the spaces at
// the start and at the end of the text only, not of each row.
func PrepareTextForParsing(text string) string {
	// .replaceAll(/^[^\S\n\r]+|[^\S\n\r]+$/g, '')
	textToParse := strings.TrimLeftFunc(text, isSpaceInLine)
	textToParse = strings.TrimRightFunc(textToParse, isSpaceInLine)
	// .replaceAll(/([\n\r])+/g, '\n')
	var sb strings.Builder
	sb.Grow(len(textToParse))
	inBreak := false
	for i := 0; i < len(textToParse); i++ {
		c := textToParse[i]
		if c == '\n' || c == '\r' {
			if !inBreak {
				sb.WriteByte('\n')
				inBreak = true
			}
			continue
		}
		inBreak = false
		sb.WriteByte(c)
	}
	return js.TrimSpace(sb.String())
}
