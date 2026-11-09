// Ported from mermaid 12.2.0, diagrams/agentflow/agentflowDb.ts: the functions
// that read the `@{ ... }` metadata of an element (stripPrototypeKeys,
// stripLineTrailingCommas, the YAML part of addVertex and
// rethrowMetadataYamlError) (MIT).

package agentflow

import (
	"errors"
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/diagrams/agentflow/jsyaml"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// stripPrototypeKeys is stripPrototypeKeys(value): it removes the keys
// `__proto__`, `constructor` and `prototype` from the parsed metadata, at
// every level.
func stripPrototypeKeys(value any) any {
	switch x := value.(type) {
	case []any:
		out := make([]any, len(x))
		for i, entry := range x {
			out[i] = stripPrototypeKeys(entry)
		}
		return out
	case *js.Obj[any]:
		if x == nil {
			return value
		}
		clean := js.NewObj[any]()
		for _, key := range x.Keys() {
			if key == "__proto__" || key == "constructor" || key == "prototype" {
				continue
			}
			clean.Set(key, stripPrototypeKeys(x.Get(key)))
		}
		return clean
	}
	return value
}

// leadingSpace is line.length - line.trimStart().length: the number of
// UTF-16 code units of white space at the start of a line. Every white
// space character of JavaScript is one code unit.
func leadingSpace(line string) int {
	return js.UTF16Len(line) - js.UTF16Len(js.TrimStart(line))
}

// opensBlockScalar is /:\s*[>|][\d+-]*\s*$/.test(code): `key: |` or
// `key: >` opens a block scalar.
func opensBlockScalar(code string) bool {
	s := js.TrimEnd(code)
	s = strings.TrimRight(s, "0123456789+-")
	if !strings.HasSuffix(s, ">") && !strings.HasSuffix(s, "|") {
		return false
	}
	s = js.TrimEnd(s[:len(s)-1])
	return strings.HasSuffix(s, ":")
}

// stripTrailingComma is code.replace(/,([\t ]*)$/, '$1').
func stripTrailingComma(code string) string {
	rest := strings.TrimRight(code, "\t ")
	if !strings.HasSuffix(rest, ",") {
		return code
	}
	return rest[:len(rest)-1] + code[len(rest):]
}

// stripLineTrailingCommas is stripLineTrailingCommas(body): it removes the
// commas at the end of the lines of a multi-line `@{ ... }` metadata body.
//
// A multi-line body is block YAML, where the comma-separated style of the
// single-line form (`@{ a: 1, b: 2 }`) is a syntax error. Only the commas
// that are trailing in the syntax go: a comma in a quoted scalar, in a flow
// collection (`[ ... ]`, `{ ... }`) or in the content of a block scalar stays,
// and a comma directly before a comment at the end of a line counts as
// trailing.
func stripLineTrailingCommas(body string) string {
	inSingle := false
	inDouble := false
	flowDepth := 0
	blockScalarIndent, inBlockScalar := 0, false
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if inBlockScalar {
			indent := leadingSpace(line)
			if js.TrimSpace(line) == "" || indent > blockScalarIndent {
				// Block scalar content is verbatim text: commas are content.
				out = append(out, line)
				continue
			}
			inBlockScalar = false
		}
		commentStart := -1
	chars:
		for i := 0; i < len(line); i++ {
			ch := line[i]
			switch {
			case inDouble:
				if ch == '\\' {
					i++
				} else if ch == '"' {
					inDouble = false
				}
			case inSingle:
				if ch == '\'' {
					inSingle = false
				}
			case ch == '"':
				inDouble = true
			case ch == '\'':
				inSingle = true
			case ch == '#' && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t'):
				commentStart = i
				break chars
			case ch == '[' || ch == '{':
				flowDepth++
			case ch == ']' || ch == '}':
				flowDepth = max(0, flowDepth-1)
			}
		}
		if inSingle || inDouble || flowDepth > 0 {
			// The line ends inside a quoted scalar or flow collection: any
			// trailing comma is content or a separator, not a stray.
			out = append(out, line)
			continue
		}
		code, comment := line, ""
		if commentStart >= 0 {
			code, comment = line[:commentStart], line[commentStart:]
		}
		if opensBlockScalar(code) {
			// `key: |` / `key: >` opens a block scalar; its deeper-indented lines
			// are verbatim content.
			blockScalarIndent, inBlockScalar = leadingSpace(line), true
			out = append(out, line)
			continue
		}
		out = append(out, stripTrailingComma(code)+comment)
	}
	return strings.Join(out, "\n")
}

// errCallStack is the RangeError of JavaScript for a recursion that does
// not end.
func errCallStack() *Error {
	return &Error{Name: "RangeError", Message: "Maximum call stack size exceeded"}
}

// yamlLoad is yaml.load(text, { schema: yaml.JSON_SCHEMA }) ?? {}. The
// second result is the YAMLException of js-yaml. The third result is the
// error for a document that contains itself through an alias: js-yaml
// returns such a document, and stripPrototypeKeys then does not end.
func yamlLoad(text string) (any, *jsyaml.Exception, error) {
	doc, err := jsyaml.Load(text)
	if err != nil {
		var ex *jsyaml.Exception
		if errors.As(err, &ex) {
			return nil, ex, nil
		}
		return nil, nil, errCallStack()
	}
	if js.IsNullish(doc) {
		// A whitespace-only YAML document loads as `null`, not `{}`. Coalesce
		// so an empty multi-line `@{\n}` behaves like single-line `@{}` (whose
		// brace-wrap already yields `{}`) instead of handing `null` to the
		// metadata consumers below (issue #83).
		return js.NewObj[any](), nil, nil
	}
	return doc, nil, nil
}

// loadMetadata is the step of addVertex that reads the shape data: the text
// between "@{" and "}" is YAML. The result is not nullish.
func (d *DB) loadMetadata(metadata string, metadataLoc *Location) (any, error) {
	multiLine := strings.Contains(metadata, "\n")
	var yamlData string
	// detect if shapeData contains a newline character
	if !multiLine {
		yamlData = "{\n" + metadata + "\n}"
	} else {
		yamlData = metadata + "\n"
	}
	doc, yamlErr, err := yamlLoad(yamlData)
	if err != nil {
		return nil, err
	}
	if yamlErr == nil {
		return doc, nil
	}
	// Multi-line bodies are block YAML, where the comma-separated style of
	// the single-line form is a syntax error. Retry once with line-trailing
	// commas stripped so trailing commas work in both forms; valid
	// documents never reach this path and are parsed verbatim above.
	stripped := metadata
	if multiLine {
		stripped = stripLineTrailingCommas(metadata)
	}
	if stripped != metadata {
		doc, retryErr, err := yamlLoad(stripped + "\n")
		if err != nil {
			return nil, err
		}
		if retryErr == nil {
			return doc, nil
		}
		// The commas weren't the problem: surface the original error
		// against the original, unstripped buffer so positions match.
	}
	// js-yaml reports the failure position relative to the `@{ ... }` block
	// buffer, not the source. Translate it into absolute source coordinates
	// before it propagates so error banners and editor markers land on the
	// right line/column (issue #56 part 2). Always throws.
	return nil, d.rethrowMetadataYamlError(yamlErr, metadata, metadataLoc)
}

// sourcePos is a line and a column of the source.
type sourcePos struct {
	line, column int
}

// padStart is String(n).padStart(width).
func padStart(n, width int) string {
	s := strconv.Itoa(n)
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}

// rethrowMetadataYamlError is rethrowMetadataYamlError(err, metadata,
// metadataLoc): it translates the failure of js-yaml for an `<id>@{ ... }`
// block from the coordinates of the block to the coordinates of the source,
// and returns the error to throw.
//
// The line and the column of the mark of js-yaml (and the `(R:C)` reference
// and the `N |` prefixes of the excerpt in the message) count from the start
// of the buffer that the db gives to js-yaml: the `{\n ... \n}` wrapper for a
// single-line `@{ ... }`, or the multi-line body. The location of the shape
// data (the position of `@{`) and the number of lines of the front matter
// give the coordinates of the source. The function writes the message, the
// mark and a hash in the form of jison.
//
// When the block has no location, the error stays as it is.
func (d *DB) rethrowMetadataYamlError(ex *jsyaml.Exception, metadata string, metadataLoc *Location) error {
	out := &Error{Name: "YAMLException", Message: ex.Message}
	if ex.Mark == nil {
		return out
	}
	out.Mark = &ErrorMark{Line: ex.Mark.Line, Column: ex.Mark.Column}
	if metadataLoc == nil {
		return out
	}
	markLine, markColumn := ex.Mark.Line, ex.Mark.Column

	isInline := !strings.Contains(metadata, "\n")
	// Buffer the DB actually handed to yaml.load, kept in lock-step with the
	// `addVertex` wrapping above.
	var buffer string
	if isInline {
		buffer = "{\n" + metadata + "\n}"
	} else {
		buffer = metadata + "\n"
	}
	// `@{`'s source position. Content begins two columns later (after `@{`).
	blockLine := metadataLoc.FirstLine + d.frontmatterLineOffset
	contentColumn := metadataLoc.FirstColumn + 2

	// Map a 0-based (line, column) in the block buffer to 0-based source
	// coordinates. For the single-line wrapper only buffer line 1 holds real
	// content (lines 0 and 2 are the synthetic braces) and it lives on the
	// block's own source line. For a multi-line body, buffer line 0 is the
	// tail of the `@{` line and every later line is a verbatim source line,
	// so its columns already match source.
	toSource := func(line, column int) sourcePos {
		if isInline {
			if line == 1 {
				return sourcePos{blockLine, contentColumn + column}
			}
			return sourcePos{blockLine, contentColumn}
		}
		if line == 0 {
			return sourcePos{blockLine + line, contentColumn + column}
		}
		return sourcePos{blockLine + line, column}
	}

	src := toSource(markLine, markColumn)
	reason := ex.Reason

	// Rebuild the excerpt in source-line space. Show the offending buffer
	// line (skipping the synthetic braces for the inline form) with a caret
	// under the failure column; tabs render as a right arrow exactly as js-yaml does so
	// the caret stays aligned.
	bufferLines := strings.Split(buffer, "\n")
	var display []int
	if isInline {
		display = append(display, 1)
	} else {
		for i := markLine - 1; i <= markLine+1; i++ {
			if i < 0 || i >= len(bufferLines) {
				continue
			}
			// Drop the trailing empty line our `+ '\n'` appended.
			if i == len(bufferLines)-1 && bufferLines[i] == "" && i != markLine {
				continue
			}
			display = append(display, i)
		}
	}
	gutterWidth := 0
	for _, i := range display {
		gutterWidth = max(gutterWidth, len(strconv.Itoa(toSource(i, 0).line)))
	}
	var snippet []string
	for _, i := range display {
		lineNo := toSource(i, 0).line
		prefix := " " + padStart(lineNo, gutterWidth) + " | "
		snippet = append(snippet, prefix+strings.ReplaceAll(bufferLines[i], "\t", "→"))
		if i == markLine {
			snippet = append(snippet, strings.Repeat("-", len(prefix)+markColumn)+"^")
		}
	}

	out.Message = reason + " (" + strconv.Itoa(src.line) + ":" + strconv.Itoa(src.column+1) + ")\n\n" + strings.Join(snippet, "\n")
	out.Mark = &ErrorMark{Line: src.line - 1, Column: src.column}
	// JISON-style hash so editors can consume source coordinates directly.
	out.Hash = &ErrorHash{
		Line: src.line - 1,
		Loc: ErrorLoc{
			FirstLine:   src.line,
			LastLine:    src.line,
			FirstColumn: src.column,
			LastColumn:  src.column + 1,
		},
	}
	return out
}
