// Ported from mermaid 12.2.0, diagrams/venn/parser/venn.jison (MIT): the lexer
// actions that tools/reference/gen-jison.mjs does not translate. The generator
// writes a stub for each such action one time. This file is then maintained by
// hand, and the generator does not change it.

package venn

import "github.com/bvolpato/mermaid-go-renderer/internal/jison"

// LexYY is the part of yy that the lexer actions use. The semantic actions of
// the parser also call SetIndentMode, so the tokens of this grammar depend on
// the semantic actions. When yy does not implement LexYY, the lexer works as
// with a yy object that has none of these members.
type LexYY interface {
	// GetIndentMode is yy.getIndentMode().
	GetIndentMode() bool
	// SetIndentMode is yy.setIndentMode(enabled).
	SetIndentMode(enabled bool)
	// ConsumeIndentText reads the property yy.consumeIndentText.
	ConsumeIndentText() bool
	// SetConsumeIndentText writes the property yy.consumeIndentText.
	SetConsumeIndentText(v bool)
}

// lexRule3 is the action of lexer rule 3: /[ \t]+(?=text\b)/i
//
//	if (yy.getIndentMode && yy.getIndentMode()) {
//	  yy.consumeIndentText = true;
//	  this.begin('INITIAL');
//	  return 'INDENT_TEXT';
//	}
func lexRule3(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	if y, ok := yy.(LexYY); ok && y.GetIndentMode() {
		y.SetConsumeIndentText(true)
		lx.Begin("INITIAL")
		return TokINDENT_TEXT, true
	}
	return 0, false
}

// lexRule5 is the action of lexer rule 5: /[^ \t\n\r]/i
//
//	if (yy.setIndentMode) { yy.setIndentMode(false); } this.begin('INITIAL'); this.unput(yytext);
func lexRule5(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	if y, ok := yy.(LexYY); ok {
		y.SetIndentMode(false)
	}
	lx.Begin("INITIAL")
	lx.Unput(lx.Yytext)
	return 0, false
}

// lexRule14 is the action of lexer rule 14: /text\b/i
//
//	if (yy.consumeIndentText) { yy.consumeIndentText = false; } else { return 'TEXT'; }
func lexRule14(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	if y, ok := yy.(LexYY); ok && y.ConsumeIndentText() {
		y.SetConsumeIndentText(false)
		return 0, false
	}
	return TokTEXT, true
}
