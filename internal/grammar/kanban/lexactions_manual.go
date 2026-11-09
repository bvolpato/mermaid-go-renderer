// Ported from mermaid 12.2.0, diagrams/kanban/parser/kanban.jison (MIT): the
// lexer actions that tools/reference/gen-jison.mjs does not translate. The
// generator writes a stub for each such action one time. This file is then
// maintained by hand, and the generator does not change it.

package kanban

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The grammar has this code in front of the lexer actions:
//
//	// Pre-lexer code can go here

// replaceLineBreaks is s.replace(/\n\s*/g, "<br/>").
func replaceLineBreaks(s string) string {
	if !strings.Contains(s, "\n") {
		return s
	}
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		b.WriteString("<br/>")
		s = strings.TrimLeftFunc(s[i+1:], js.IsWhiteSpace)
	}
}

// lexRule3 is the action of lexer rule 3: /[^\"]+/i
//
//	// console.log('shapeData', yytext);
//	const re = /\n\s*/g;
//	yytext = yytext.replace(re,"<br/>");
//	return 'SHAPE_DATA'
func lexRule3(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	lx.Yytext = replaceLineBreaks(lx.Yytext)
	return TokSHAPE_DATA, true
}
