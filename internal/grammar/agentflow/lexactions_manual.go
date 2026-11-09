// Ported from mermaid 12.2.0, diagrams/agentflow/parser/agentflow.jison (MIT):
// the lexer actions that tools/reference/gen-jison.mjs does not translate. The
// generator writes a stub for each such action one time. This file is then
// maintained by hand, and the generator does not change it.

package agentflow

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// LexYY is the part of yy that the lexer actions call. The yy value of a
// parse must implement it.
type LexYY interface {
	// FirstGraph is yy.lex.firstGraph(): true for the first call after the
	// database is cleared, then false.
	FirstGraph() bool
}

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

// lexRule10 is the action of lexer rule 10: /[^\"]+/
//
//	// console.log('shapeData', yytext);
//	const re = /\n\s*/g;
//	yytext = yytext.replace(re,"<br/>");
//	return 'SHAPE_DATA'
func lexRule10(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	lx.Yytext = replaceLineBreaks(lx.Yytext)
	return TokSHAPE_DATA, true
}

// lexRule35 is the action of lexer rule 35: /agentflow-beta\b/
//
//	if(yy.lex.firstGraph()){this.begin("dir");}  return 'GRAPH';
func lexRule35(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	y, ok := yy.(LexYY)
	if !ok {
		lx.Throw(&jison.Error{Name: "TypeError", Message: "Cannot read properties of undefined (reading 'firstGraph')"})
		return 0, false
	}
	if y.FirstGraph() {
		lx.Begin("dir")
	}
	return TokGRAPH, true
}
