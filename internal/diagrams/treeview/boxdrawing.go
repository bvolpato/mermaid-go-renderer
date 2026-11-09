// Ported from mermaid 12.2.0, diagrams/treeView/boxDrawingPreprocessor.ts
// (MIT).
//
// The box-drawing pre-processor converts box-drawing character input
// (├──, └──, │) to indent-based input before the parser runs. It supports
// both the standard and the heavy Unicode variants.

package treeview

import (
	"errors"
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// A line is a list of UTF-16 code units here, because upstream counts
// columns in code units.

// isAllBoxChar is the class of ALL_BOX_CHARS: /[─━│┃└┗├┣]/.
func isAllBoxChar(u rune) bool {
	switch u {
	case '─', '━', '│', '┃', '└', '┗', '├', '┣':
		return true
	}
	return false
}

// isBranchChar is the class of BRANCH_CHAR: /[└┗├┣]/.
func isBranchChar(u rune) bool {
	switch u {
	case '└', '┗', '├', '┣':
		return true
	}
	return false
}

// isDashChar is the class of DASH_CHAR: /[─━]/.
func isDashChar(u rune) bool {
	return u == '─' || u == '━'
}

// hasBoxChar is ALL_BOX_CHARS.test(line).
func hasBoxChar(line []rune) bool {
	for _, u := range line {
		if isAllBoxChar(u) {
			return true
		}
	}
	return false
}

// branchIndex is BRANCH_CHAR.exec(line)?.index, or -1 without a match.
func branchIndex(line []rune) int {
	for i, u := range line {
		if isBranchChar(u) {
			return i
		}
	}
	return -1
}

// allOf is /^[...]+$/.test(line) for the class that in tests.
func allOf(line []rune, in func(rune) bool) bool {
	if len(line) == 0 {
		return false
	}
	for _, u := range line {
		if !in(u) {
			return false
		}
	}
	return true
}

// isDecorationOnly is DECORATION_ONLY.test(line): /^[\s│┃]+$/.
func isDecorationOnly(line []rune) bool {
	return allOf(line, func(u rune) bool { return js.IsWhiteSpace(u) || u == '│' || u == '┃' })
}

// skipSpace returns the offset of the first code unit that is not white
// space: the end of the match of /^\s*/.
func skipSpace(line []rune) int {
	i := 0
	for i < len(line) && js.IsWhiteSpace(line[i]) {
		i++
	}
	return i
}

// hasPrefixAt reports whether line has the ASCII text prefix at offset i.
func hasPrefixAt(line []rune, i int, prefix string) bool {
	if i+len(prefix) > len(line) {
		return false
	}
	for k := 0; k < len(prefix); k++ {
		if line[i+k] != rune(prefix[k]) {
			return false
		}
	}
	return true
}

// skipBlanks returns the offset after the tabs and spaces at offset i.
func skipBlanks(line []rune, i int) int {
	for i < len(line) && (line[i] == '\t' || line[i] == ' ') {
		i++
	}
	return i
}

// isMetadataLine is METADATA_LINE.test(line):
// /^\s*(title[\t ]|accTitle[\t ]*:|accDescr[\t ]*[:{])/.
func isMetadataLine(line []rune) bool {
	i := skipSpace(line)
	if hasPrefixAt(line, i, "title") {
		if k := i + len("title"); k < len(line) && (line[k] == '\t' || line[k] == ' ') {
			return true
		}
	}
	if hasPrefixAt(line, i, "accTitle") {
		if k := skipBlanks(line, i+len("accTitle")); k < len(line) && line[k] == ':' {
			return true
		}
	}
	if hasPrefixAt(line, i, "accDescr") {
		if k := skipBlanks(line, i+len("accDescr")); k < len(line) && (line[k] == ':' || line[k] == '{') {
			return true
		}
	}
	return false
}

// isCommentLine is COMMENT_LINE.test(line): /^\s*%%/.
func isCommentLine(line []rune) bool {
	return hasPrefixAt(line, skipSpace(line), "%%")
}

// indentUnit is INDENT_UNIT: 4 spaces per depth level in the output.
const indentUnit = "    "

// expandTabs is line.replace(/\t/g, '    ').
func expandTabs(line []rune) []rune {
	out := make([]rune, 0, len(line))
	for _, u := range line {
		if u == '\t' {
			out = append(out, ' ', ' ', ' ', ' ')
		} else {
			out = append(out, u)
		}
	}
	return out
}

// isBoxDrawingFormat is isBoxDrawingFormat(lines): it detects whether any
// of the given lines contain box-drawing characters.
func isBoxDrawingFormat(lines [][]rune) bool {
	for _, line := range lines {
		if hasBoxChar(line) {
			return true
		}
	}
	return false
}

// inferSegmentWidth is inferSegmentWidth(contentLines): it infers the
// segment width (code units per depth level) by finding the first branch
// character (├/└/┣/┗) at a column position > 0. It falls back to 4 if all
// branches are at column 0 or none exist.
func inferSegmentWidth(contentLines [][]rune) int {
	for _, line := range contentLines {
		if index := branchIndex(line); index > 0 {
			return index
		}
	}
	return 4
}

// reLineNumber is /\bline\s+(\d+)\b/gi.
var reLineNumber = regexp.MustCompile(`(?i)\bline[\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]+([0-9]+)\b`)

// remapErrorLines is remapErrorLines(message, lineMap): it remaps the line
// numbers in an error message from output line numbers to original line
// numbers.
func remapErrorLines(message string, lineMap *js.Map[int, int]) string {
	return reLineNumber.ReplaceAllStringFunc(message, func(match string) string {
		lineStr := reLineNumber.FindStringSubmatch(match)[1]
		line := js.ParseInt(lineStr, 10)
		if !(line < 1<<31) {
			// No output has that many lines.
			return match
		}
		original := lineMap.Get(int(line))
		if original != 0 {
			return "line " + js.Str(float64(original))
		}
		return match
	})
}

// preprocessBoxDrawing is preprocessBoxDrawing(input): it pre-processes
// box-drawing formatted treeView input into indent-based format.
//
// If the input uses box-drawing characters (├── └── │ or heavy variants
// ┣━━ ┗━━ ┃), it is converted to the indentation-based format that the
// parser reads. If the input is already indent-based, it is returned
// unchanged.
//
// It returns the transformed text and a line mapping for error remapping:
// output line numbers (1-based) to original line numbers (1-based). The
// mapping is empty if there was no transformation.
func preprocessBoxDrawing(input string) (string, *js.Map[int, int], error) {
	textLines := strings.Split(input, "\n")
	lines := make([][]rune, len(textLines))
	for i, line := range textLines {
		lines[i] = js.UTF16Units(line)
	}
	lineMap := js.NewMap[int, int]()

	// Find keyword line
	keywordIdx := -1
	for i, line := range textLines {
		if js.TrimSpace(line) == "treeView-beta" {
			keywordIdx = i
			break
		}
	}

	if keywordIdx == -1 {
		// No keyword found: return as-is (let the parser handle the error)
		return input, lineMap, nil
	}

	// Collect content line texts for format detection (skip blanks, comments, metadata, decoration)
	contentLineTexts := [][]rune{}
	for i := keywordIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := js.TrimSpace(textLines[i])
		if trimmed == "" || isCommentLine(line) || isMetadataLine(line) {
			continue
		}
		if isDecorationOnly(line) {
			continue
		}
		// Normalize tabs early so segment-width inference uses consistent column positions
		contentLineTexts = append(contentLineTexts, expandTabs(line))
	}

	// If no box-drawing characters found → return unchanged
	if !isBoxDrawingFormat(contentLineTexts) {
		return input, lineMap, nil
	}

	// Infer segment width
	segmentWidth := inferSegmentWidth(contentLineTexts)

	// Build output
	outputLines := []string{}
	outLineNo := 0

	// Pass through all lines up to and including keyword
	for i := 0; i <= keywordIdx; i++ {
		outputLines = append(outputLines, textLines[i])
		outLineNo++
		lineMap.Set(outLineNo, i+1)
	}

	// Process lines after keyword
	for i := keywordIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := js.TrimSpace(textLines[i])
		origLineNo := i + 1

		// Blank lines → pass through
		if trimmed == "" {
			outputLines = append(outputLines, textLines[i])
			outLineNo++
			lineMap.Set(outLineNo, origLineNo)
			continue
		}

		// Comments → pass through
		if isCommentLine(line) {
			outputLines = append(outputLines, textLines[i])
			outLineNo++
			lineMap.Set(outLineNo, origLineNo)
			continue
		}

		// Metadata (title, accTitle, accDescr) → pass through
		if isMetadataLine(line) {
			outputLines = append(outputLines, textLines[i])
			outLineNo++
			lineMap.Set(outLineNo, origLineNo)
			continue
		}

		// Decoration-only lines (│ + whitespace, no actual content) → skip
		if isDecorationOnly(line) {
			continue
		}

		// Normalize tabs to spaces for consistent column-position math
		normalized := expandTabs(line)

		// Find branch character (├, └, ┣, ┗)
		branchCol := branchIndex(normalized)

		if branchCol >= 0 {
			// Has branch char → compute depth from column position
			depth := int(js.Round(float64(branchCol)/float64(segmentWidth))) + 1

			// Extract content: skip branch char, then dashes, then spaces
			pos := branchCol + 1
			for pos < len(normalized) && isDashChar(normalized[pos]) {
				pos++
			}
			for pos < len(normalized) && normalized[pos] == ' ' {
				pos++
			}
			content := js.TrimEnd(js.FromUTF16Units(normalized[pos:]))

			if content == "" {
				return "", nil, errors.New("Line " + js.Str(float64(origLineNo)) +
					": Empty node — expected a filename or directory name after the box-drawing prefix")
			}

			indent := strings.Repeat(indentUnit, depth)
			outputLines = append(outputLines, indent+content)
			outLineNo++
			lineMap.Set(outLineNo, origLineNo)
		} else if allOf(normalized, func(u rune) bool { return js.IsWhiteSpace(u) || isAllBoxChar(u) }) {
			// Entire line is box-drawing decoration and whitespace: skip
			continue
		} else if hasBoxChar(normalized) {
			// Has box chars but no branch char: likely content containing a box char (e.g. "Section ─ A.txt")
			// Treat as root-level item
			outputLines = append(outputLines, textLines[i])
			outLineNo++
			lineMap.Set(outLineNo, origLineNo)
		} else if len(normalized) > 0 && js.IsWhiteSpace(normalized[0]) {
			// Leading whitespace without box chars in box mode → likely mixed format
			return "", nil, errors.New("Line " + js.Str(float64(origLineNo)) + ": Unexpected indentation without box-drawing characters. " +
				"In box-drawing format, use ├── or └── prefixes for indented nodes.")
		} else {
			// No box chars, no leading whitespace → root-level item (depth 0)
			outputLines = append(outputLines, textLines[i])
			outLineNo++
			lineMap.Set(outLineNo, origLineNo)
		}
	}

	return strings.Join(outputLines, "\n"), lineMap, nil
}
