// Package jsyaml is a port of the loader of js-yaml 4.3.0 with its JSON
// schema: yaml.load(text, { schema: yaml.JSON_SCHEMA }), as mermaid 12.2.0
// bundles it. The agentflow db reads the `@{ ... }` metadata of an element
// with it, and translates the reason and the mark of a YAMLException to a
// position in the diagram source. So the errors of this package have the
// reason, the line and the column of js-yaml.
//
// Ported from js-yaml 4.3.0, js-yaml/lib/exception.js, snippet.js, loader.js
// and the types of the JSON schema (type/null.js, bool.js, int.js, float.js,
// str.js, seq.js, map.js) (MIT).
//
// A JavaScript string is a sequence of UTF-16 code units, and the loader
// reads the input by code unit. The positions and the columns of a mark
// count code units.
package jsyaml

import (
	"errors"
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Mark is the mark of a YAMLException: the position of the error in the
// input.
type Mark struct {
	// Buffer is the input, with the line break that the loader adds.
	Buffer string
	// Position is the index of the code unit.
	Position int
	// Line and Column count from 0.
	Line   int
	Column int
	// Snippet is the excerpt of the input around the position, or "".
	Snippet string
}

// Exception is YAMLException.
type Exception struct {
	// Reason is the message without the position.
	Reason string
	// Mark is nil for an error with no position.
	Mark *Mark
	// Message is error.message: the reason, the position and the snippet.
	Message string
}

func (e *Exception) Error() string { return "YAMLException: " + e.Message }

// ErrCyclic is the error of Load for a document that contains itself
// through an alias, for example `&a [*a]`. js-yaml returns such a document.
// The dynamic values of this port have no cycles, and code that walks the
// document (stripPrototypeKeys of the agentflow db) does not end upstream.
var ErrCyclic = errors.New("jsyaml: the document contains itself")

// formatError is formatError(exception, compact) with compact false.
func formatError(reason string, mark *Mark) string {
	message := reason
	if message == "" {
		message = "(unknown reason)"
	}
	if mark == nil {
		return message
	}
	where := "(" + strconv.Itoa(mark.Line+1) + ":" + strconv.Itoa(mark.Column+1) + ")"
	if mark.Snippet != "" {
		where += "\n\n" + mark.Snippet
	}
	return message + " " + where
}

// newException is new YAMLException(reason, mark).
func newException(reason string, mark *Mark) *Exception {
	return &Exception{Reason: reason, Mark: mark, Message: formatError(reason, mark)}
}

// snippetLine is the result of getLine.
type snippetLine struct {
	str string
	pos int
}

// getLine is getLine(buffer, lineStart, lineEnd, position, maxLineLength).
func getLine(buffer []rune, lineStart, lineEnd, position, maxLineLength int) snippetLine {
	head := ""
	tail := ""
	maxHalfLength := floorDiv(maxLineLength, 2) - 1

	if position-lineStart > maxHalfLength {
		head = " ... "
		lineStart = position - maxHalfLength + len(head)
	}

	if lineEnd-position > maxHalfLength {
		tail = " ..."
		lineEnd = position + maxHalfLength - len(tail)
	}

	return snippetLine{
		str: head + strings.ReplaceAll(sliceUnits(buffer, lineStart, lineEnd), "\t", "→") + tail,
		pos: position - lineStart + len(head), // relative position
	}
}

// floorDiv is Math.floor(a / b) for a positive b.
func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && a < 0 {
		q--
	}
	return q
}

// sliceUnits is buffer.slice(start, end) of a JavaScript string. A negative
// index counts from the end.
func sliceUnits(buffer []rune, start, end int) string {
	n := len(buffer)
	if start < 0 {
		start = max(n+start, 0)
	} else if start > n {
		start = n
	}
	if end < 0 {
		end = max(n+end, 0)
	} else if end > n {
		end = n
	}
	if start >= end {
		return ""
	}
	return js.FromUTF16Units(buffer[start:end])
}

// repeat is common.repeat(string, count). A negative count gives the empty
// string.
func repeat(s string, count int) string {
	if count <= 0 {
		return ""
	}
	return strings.Repeat(s, count)
}

// padStart is padStart(string, max) of snippet.js.
func padStart(s string, width int) string {
	return repeat(" ", width-len(s)) + s
}

// makeSnippet is makeSnippet(mark) with the default options: a maximum
// length of 79, an indent of 1, three lines before and two lines after.
func makeSnippet(buffer []rune, position, markLine int) string {
	if len(buffer) == 0 {
		return ""
	}
	const (
		maxLength   = 79
		indent      = 1
		linesBefore = 3
		linesAfter  = 2
	)

	// re = /\r?\n|\r|\0/g
	lineStarts := []int{0}
	lineEnds := []int{}
	foundLineNo := -1

	for i := 0; i < len(buffer); i++ {
		u := buffer[i]
		matchLen := 0
		switch {
		case u == '\r' && i+1 < len(buffer) && buffer[i+1] == '\n':
			matchLen = 2
		case u == '\n' || u == '\r' || u == 0:
			matchLen = 1
		}
		if matchLen == 0 {
			continue
		}
		lineEnds = append(lineEnds, i)
		lineStarts = append(lineStarts, i+matchLen)

		if position <= i && foundLineNo < 0 {
			foundLineNo = len(lineStarts) - 2
		}
		i += matchLen - 1
	}

	if foundLineNo < 0 {
		foundLineNo = len(lineStarts) - 1
	}

	// lineEnds[i] is undefined for the last line when the buffer does not
	// end with a line break. The slice of getLine then goes to the end.
	lineEnd := func(i int) int {
		if i < len(lineEnds) {
			return lineEnds[i]
		}
		return len(buffer)
	}

	result := ""
	lineNoLength := len(strconv.Itoa(min(markLine+linesAfter, len(lineEnds))))
	maxLineLength := maxLength - (indent + lineNoLength + 3)

	for i := 1; i <= linesBefore; i++ {
		if foundLineNo-i < 0 {
			break
		}
		line := getLine(
			buffer,
			lineStarts[foundLineNo-i],
			lineEnd(foundLineNo-i),
			position-(lineStarts[foundLineNo]-lineStarts[foundLineNo-i]),
			maxLineLength,
		)
		result = repeat(" ", indent) + padStart(strconv.Itoa(markLine-i+1), lineNoLength) +
			" | " + line.str + "\n" + result
	}

	line := getLine(buffer, lineStarts[foundLineNo], lineEnd(foundLineNo), position, maxLineLength)
	result += repeat(" ", indent) + padStart(strconv.Itoa(markLine+1), lineNoLength) +
		" | " + line.str + "\n"
	result += repeat("-", indent+lineNoLength+3+line.pos) + "^" + "\n"

	for i := 1; i <= linesAfter; i++ {
		if foundLineNo+i >= len(lineEnds) {
			break
		}
		line := getLine(
			buffer,
			lineStarts[foundLineNo+i],
			lineEnd(foundLineNo+i),
			position-(lineStarts[foundLineNo]-lineStarts[foundLineNo+i]),
			maxLineLength,
		)
		result += repeat(" ", indent) + padStart(strconv.Itoa(markLine+i+1), lineNoLength) +
			" | " + line.str + "\n"
	}

	return strings.TrimSuffix(result, "\n")
}

// Load is yaml.load(input, { schema: yaml.JSON_SCHEMA }). The result is a
// dynamic JavaScript value: nil (undefined) for an input with no document,
// js.Null for an empty document, a bool, a float64, a string, a []any or a
// *js.Obj[any]. The error is an *Exception, or ErrCyclic.
func Load(input string) (doc any, err error) {
	defer func() {
		if p := recover(); p != nil {
			e, ok := p.(*Exception)
			if !ok {
				panic(p)
			}
			doc, err = nil, e
		}
	}()
	documents := loadDocuments(input)
	switch len(documents) {
	case 0:
		return nil, nil
	case 1:
		return export(documents[0])
	}
	return nil, newException("expected a single document in the stream, but found more", nil)
}
