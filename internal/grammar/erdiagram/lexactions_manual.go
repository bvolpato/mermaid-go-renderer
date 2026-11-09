// Ported from mermaid 12.2.0, diagrams/er/parser/erDiagram.jison (MIT): the
// lexer actions that tools/reference/gen-jison.mjs does not translate. The
// generator writes a stub for each such action one time. This file is then
// maintained by hand, and the generator does not change it.

package erdiagram

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// lexRule32 is the action of lexer rule 32: /./i
//
//	return yytext[0];
//
// The token is the matched character itself. It is a symbol of the grammar
// only by accident, so the parser normally reports it as unexpected.
func lexRule32(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	return lx.TokenNamed(js.CharAt(lx.Yytext, 0))
}

// lexRule81 is the action of lexer rule 81: /./i
//
//	return yytext[0];
func lexRule81(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	return lx.TokenNamed(js.CharAt(lx.Yytext, 0))
}
