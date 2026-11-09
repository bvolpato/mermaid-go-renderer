// Ported from mermaid 12.2.0, diagrams/block/parser/block.jison (MIT): the
// lexer actions that tools/reference/gen-jison.mjs does not translate. The
// generator writes a stub for each such action one time. This file is then
// maintained by hand, and the generator does not change it.

package block

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// trimLeadingComma is s.replace(/^,\s*/, "").
func trimLeadingComma(s string) string {
	if !strings.HasPrefix(s, ",") {
		return s
	}
	return strings.TrimLeftFunc(s[1:], js.IsWhiteSpace)
}

// lexRule6 is the action of lexer rule 6: /columns\s+auto\b/
//
//	yytext=-1; return 'COLUMNS';
//
// The JavaScript sets yytext to the number -1. Yytext is a string here, so
// the semantic value of the token is "-1". The semantic action reads it with
// parseInt, which gives -1 for both.
func lexRule6(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	lx.Yytext = "-1"
	return TokCOLUMNS, true
}

// lexRule7 is the action of lexer rule 7: /columns\s+[\d]+/
//
//	yytext = yytext.replace(/columns\s+/,''); yy.getLogger().debug('COLUMNS (LEX)', yytext); return 'COLUMNS';
func lexRule7(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	s := lx.Yytext
	// Remove the first "columns" that white space follows, with the white space.
	for from := 0; ; {
		i := strings.Index(s[from:], "columns")
		if i < 0 {
			break
		}
		i += from
		rest := strings.TrimLeftFunc(s[i+len("columns"):], js.IsWhiteSpace)
		if len(rest) < len(s)-i-len("columns") {
			s = s[:i] + rest
			break
		}
		from = i + 1
	}
	lx.Yytext = s
	return TokCOLUMNS, true
}

// lexRule14 is the action of lexer rule 14: /space[:]\d+/
//
//	yytext = yytext.replace(/space\:/,'');yy.getLogger().debug('SPACE NUM (LEX)', yytext); return 'SPACE_BLOCK';
func lexRule14(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	lx.Yytext = strings.Replace(lx.Yytext, "space:", "", 1)
	return TokSPACE_BLOCK, true
}

// lexRule85 is the action of lexer rule 85: /,?\s*right\s*/
//
//	yytext = yytext.replace(/^,\s*/, ''); yy.getLogger().debug('Lex (right): dir:',yytext);return "DIR";
func lexRule85(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	lx.Yytext = trimLeadingComma(lx.Yytext)
	return TokDIR, true
}

// lexRule86 is the action of lexer rule 86: /,?\s*left\s*/
//
//	yytext = yytext.replace(/^,\s*/, ''); yy.getLogger().debug('Lex (left):',yytext);return "DIR";
func lexRule86(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	lx.Yytext = trimLeadingComma(lx.Yytext)
	return TokDIR, true
}

// lexRule87 is the action of lexer rule 87: /,?\s*x\s*/
//
//	yytext = yytext.replace(/^,\s*/, ''); yy.getLogger().debug('Lex (x):',yytext); return "DIR";
func lexRule87(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	lx.Yytext = trimLeadingComma(lx.Yytext)
	return TokDIR, true
}

// lexRule88 is the action of lexer rule 88: /,?\s*y\s*/
//
//	yytext = yytext.replace(/^,\s*/, ''); yy.getLogger().debug('Lex (y):',yytext); return "DIR";
func lexRule88(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	lx.Yytext = trimLeadingComma(lx.Yytext)
	return TokDIR, true
}

// lexRule89 is the action of lexer rule 89: /,?\s*up\s*/
//
//	yytext = yytext.replace(/^,\s*/, ''); yy.getLogger().debug('Lex (up):',yytext); return "DIR";
func lexRule89(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	lx.Yytext = trimLeadingComma(lx.Yytext)
	return TokDIR, true
}

// lexRule90 is the action of lexer rule 90: /,?\s*down\s*/
//
//	yytext = yytext.replace(/^,\s*/, ''); yy.getLogger().debug('Lex (down):',yytext); return "DIR";
func lexRule90(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	lx.Yytext = trimLeadingComma(lx.Yytext)
	return TokDIR, true
}
