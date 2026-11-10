// This file ports no upstream file. It has the Go API of the package: the
// call of katex.renderToString that mermaid 12.2.0 makes in
// packages/mermaid/src/diagrams/common/common.ts (renderKatexUnsanitized,
// MIT), and the errors of KaTeX 0.18.11 (src/ParseError.ts, MIT).

// Package katex gives the string that KaTeX returns for a TeX expression.
//
// mermaid replaces each $$...$$ of a label with
//
//	katex.renderToString(c, {throwOnError: true, displayMode: true, output: 'mathml'})
//
// and with the configuration forceLegacyMathML the output is 'htmlAndMathml'.
// RenderToString is that call.
//
// The package runs the code of KaTeX itself: katexjs/katex.mjs is the file
// dist/katex.mjs of KaTeX 0.18.11, the version in the bundle of mermaid
// 12.2.0, and goja interprets it. So the string is the string of the same
// code, and the message of an error is the same message. The golden test
// compares the results with the same file in node.
//
// The JavaScript does not start before the first call of RenderToString. The
// script is compiled one time for the process. A runtime of goja is not safe
// for concurrent use, so the package keeps a pool of runtimes, and calls from
// several goroutines run in parallel. KaTeX keeps no state between two calls
// with these options: a macro that one expression defines with \gdef does not
// exist in the next expression.
//
// Strings. A JavaScript string is a sequence of UTF-16 code units. The Go
// strings of this package are UTF-8, and one half of a surrogate pair is the
// three bytes that UTF-8 would use for it, as in internal/js (UTF16Units).
package katex

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Version is the version of KaTeX in the mermaid 12.2.0 bundle.
const Version = "0.18.11"

// Output is the option "output" of KaTeX.
type Output string

const (
	// OutputMathML is the output of mermaid by default.
	OutputMathML Output = "mathml"
	// OutputHTMLAndMathML is the output of mermaid with forceLegacyMathML.
	OutputHTMLAndMathML Output = "htmlAndMathml"
)

// ParseError is the ParseError of KaTeX: the expression is not valid.
type ParseError struct {
	// Message is the property "message" of the error. It starts with
	// "KaTeX parse error: ".
	Message string
	// RawMessage is the property "rawMessage": the message without the
	// prefix and without the position.
	RawMessage string
	// HasPosition tells that the error has a place in the expression.
	HasPosition bool
	// Position is the property "position": the index of the first UTF-16
	// code unit of the bad token.
	Position int
	// Length is the property "length": the number of UTF-16 code units of
	// the bad token.
	Length int
}

// Error returns the property "message" of the JavaScript error.
func (e *ParseError) Error() string { return e.Message }

// Error is a JavaScript error of KaTeX that is not a ParseError, for example
// a RangeError for an expression that nests too deeply.
type Error struct {
	// Name is the property "name" of the error, for example "RangeError".
	Name string
	// Message is the property "message" of the error.
	Message string
}

// Error returns the property "message" of the JavaScript error.
func (e *Error) Error() string { return e.Message }

// ErrTimeout is the error for an expression that runs longer than
// MaxDuration, or longer than the context allows. KaTeX in a browser has no
// such limit.
var ErrTimeout = errors.New("katex: the expression took too long")

// MaxDuration is the longest time that one call of RenderToString runs. A
// normal expression takes about a millisecond.
const MaxDuration = 60 * time.Second

// RenderToString is katex.renderToString(tex, {throwOnError: true,
// displayMode: true, output: output}).
//
// When KaTeX throws, the error is a *ParseError, or an *Error for another
// JavaScript error. Its Error() text is the property "message" of the
// JavaScript error, exactly. Each other error is a failure of this package
// and not of KaTeX: ErrTimeout, or an error of the interpreter.
func RenderToString(tex string, output Output) (string, error) {
	return RenderToStringContext(context.Background(), tex, output)
}

// RenderToStringContext is RenderToString with a context. When the context
// ends, the call stops and returns an error that wraps ErrTimeout and the
// error of the context.
func RenderToStringContext(ctx context.Context, tex string, output Output) (string, error) {
	if output != OutputMathML && output != OutputHTMLAndMathML {
		return "", fmt.Errorf("katex: unknown output %q", string(output))
	}
	return render(ctx, tex, output)
}

// Started reports whether the JavaScript of KaTeX was compiled or run in
// this process. It is false until the first call of RenderToString.
func Started() bool {
	return started.Load()
}
