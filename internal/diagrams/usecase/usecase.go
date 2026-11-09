// Package usecase is the usecase diagram (the keyword usecase-beta): actors,
// use cases, system boundaries, notes, JSON tables and their relationships.
//
// Ported from diagrams/usecase of mermaid 12.2.0: usecaseDiagram.ts,
// usecaseDetector.ts, usecaseDb.ts, usecaseTypes.ts, usecaseRenderer.ts and
// the parser directory (a Chevrotain lexer and parser, a CST visitor and a
// model builder), and from the node shapes of rendering-util that only this
// diagram uses (shapes/usecase*.ts).
//
// The diagram is drawn by the unified renderer (internal/ru). The standard
// shapes, the edges, the cluster shapes and the layouts register themselves
// there: a program must link those packages too.
package usecase

import (
	"errors"
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:  "usecase",
		New: func(ctx *render.Context) render.DB { return NewDB(ctx) },
		Parse: func(_ *render.Context, db render.DB, text string) error {
			return parse(db.(*DB), text)
		},
		Draw: func(ctx *render.Context, db render.DB, _ string) error {
			return draw(ctx, db.(*DB))
		},
	})
}

// Ported from diagrams/usecase/parser/usecase.chevrotain.ts and
// diagrams/common/parser/runChevrotainParse.ts.

// isKnownLocation is isKnownLocation(value). Chevrotain marks missing
// locations (EOF, inserted tokens) with -1.
func isKnownLocation(value int) bool {
	return value >= 0
}

// parse is parser.parse(input): it fills the db from the diagram text.
func parse(db *DB, input string) error {
	db.clear()
	text := js.UTF16Units(input)
	if err := runChevrotainParse(db, text); err != nil {
		db.clear()
		return err
	}
	return nil
}

// runChevrotainParse is runChevrotainParse(config, input), with the error
// handling of parser.parse for a parser error: the position of the token
// at which the parser stopped is added to the message.
func runChevrotainParse(db *DB, input []rune) error {
	tokens, lexError := tokenize(input)
	if lexError != nil {
		// Lexer failures never reach `parser.errors`, so callers cannot enrich them the way
		// they enrich parser errors. Attach the position here so both failure modes report
		// the same fields.
		start := lexError.offset
		end := start + lexError.length
		return errors.New(
			"Error lexing usecase diagram: " + lexError.message + " at line " +
				strconv.Itoa(lexError.line) + ", column " + strconv.Itoa(lexError.column) +
				" [" + strconv.Itoa(start) + "," + strconv.Itoa(end) + ")",
		)
	}

	cst, parseError := parseTokens(tokens, (*usecaseParser).start)
	if parseError != nil {
		message := "Error parsing usecase diagram: " + parseError.message
		tok := parseError.token
		start := len(input)
		if isKnownLocation(tok.startOffset) {
			start = tok.startOffset
		}
		end := start
		if isKnownLocation(tok.endOffset) {
			end = tok.endOffset + 1
		}
		line := tok.startLine
		if !isKnownLocation(tok.startLine) {
			// input.slice(0, start).split(/\r\n|\r|\n/).length
			line = 1
			for i := 0; i < start; i++ {
				switch input[i] {
				case '\r':
					if i+1 < start && input[i+1] == '\n' {
						i++
					}
					line++
				case '\n':
					line++
				}
			}
		}
		// Math.max(input.lastIndexOf('\n', start - 1), input.lastIndexOf('\r', start - 1))
		lineStart := -1
		for i := min(max(start-1, 0), len(input)-1); i >= 0; i-- {
			if input[i] == '\n' || input[i] == '\r' {
				lineStart = i
				break
			}
		}
		column := tok.startColumn
		if !isKnownLocation(tok.startColumn) {
			column = start - lineStart
		}
		return errors.New(
			message + " at line " + strconv.Itoa(line) + ", column " + strconv.Itoa(column) +
				" [" + strconv.Itoa(start) + "," + strconv.Itoa(end) + ")",
		)
	}

	visitor := &usecaseVisitor{builder: newModelBuilder(db), source: input}
	return visitor.build(cst)
}
