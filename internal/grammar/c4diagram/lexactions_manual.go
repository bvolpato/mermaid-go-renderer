// Ported from mermaid 12.2.0, diagrams/c4/parser/c4Diagram.jison (MIT): the
// lexer actions that tools/reference/gen-jison.mjs does not translate. The
// generator writes a stub for each such action one time. This file is then
// maintained by hand, and the generator does not change it.

package c4diagram

import "github.com/bvolpato/mermaid-go-renderer/internal/jison"

// lexRule14 is the action of lexer rule 14: /%%[^\n]*(\r?\n)*/
//
//	c            /* skip comments */
//
// The action reads a variable "c" that does not exist, so it would stop the
// parse with a ReferenceError. Rule 13 comes first and matches every text
// that this rule matches, so the lexer never runs this action.
func lexRule14(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	lx.Throw(&jison.Error{Name: "ReferenceError", Message: "c is not defined"})
	return 0, false
}
