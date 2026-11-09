// Ported from mermaid 12.2.0, diagrams/state/parser/stateDiagram.jison (MIT):
// the lexer actions that tools/reference/gen-jison.mjs does not translate. The
// generator writes a stub for each such action one time. This file is then
// maintained by hand, and the generator does not change it.

package statediagram

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The grammar has this code in front of the lexer actions:
//
//	function processId() {
//	    const idx = yytext.indexOf('%%');
//	    if (idx === 0) {
//	        return false;
//	    }
//	    if (idx > 0) {
//	        const before = yytext.slice(0, idx);
//	        const after = yytext.slice(idx);
//
//	        if (after) {
//	            yy.lexer.unput(after);
//	        }
//	        yytext = before;
//	    }
//	    return true;
//	}

// processID is processId. It cuts a "%%" comment off an identifier and puts
// the comment back into the input. yy.lexer is the lexer itself.
func processID(lx *jison.Lexer) bool {
	// "%%" is ASCII, so the byte index splits the text between two characters.
	idx := strings.Index(lx.Yytext, "%%")
	if idx == 0 {
		return false
	}
	if idx > 0 {
		before, after := lx.Yytext[:idx], lx.Yytext[idx:]
		lx.Unput(after)
		lx.Yytext = before
	}
	return true
}

// lexRule49 is the action of lexer rule 49: /[^\n\{]*/i
//
//	if (!processId()) return; this.popState(); /* console.log('STATE_ID', yytext); */ return "ID";
func lexRule49(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	if !processID(lx) {
		return 0, false
	}
	lx.PopState()
	return TokID, true
}

// lexRule52 is the action of lexer rule 52: /\w+\s+\w+.*?\{/i
//
//	throw new Error('Error: State name must be a single word. Found: "' + yytext.trim() + '"');
func lexRule52(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	lx.Throw(&jison.Error{
		Name:    "Error",
		Message: `Error: State name must be a single word. Found: "` + js.TrimSpace(lx.Yytext) + `"`,
	})
	return 0, false
}

// lexRule65 is the action of lexer rule 65: /[^\n]*/i
//
//	if (!processId()) return; this.popState(); /* console.log('Floating note ID', yytext);*/ return "ID";
func lexRule65(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	if !processID(lx) {
		return 0, false
	}
	lx.PopState()
	return TokID, true
}

// lexRule66 is the action of lexer rule 66: /\s*[^:\n\s\-]+/i
//
//	if (!processId()) return; this.popState(); this.pushState('NOTE_TEXT'); /*console.log('Got ID for note', yytext);*/ return 'ID';
func lexRule66(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	if !processID(lx) {
		return 0, false
	}
	lx.PopState()
	lx.PushState("NOTE_TEXT")
	return TokID, true
}

// lexRule73 is the action of lexer rule 73: /[^:\n\s\-\{]+/i
//
//	if (!processId()) return; /* console.log('=>ID=',yytext); */ return 'ID';
func lexRule73(yy any, lx *jison.Lexer, startCondition string) (int, bool) {
	if !processID(lx) {
		return 0, false
	}
	return TokID, true
}
