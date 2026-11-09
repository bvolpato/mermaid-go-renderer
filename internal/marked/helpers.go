// Ported from marked/src/helpers.ts (marked 16.3.0): the helpers that the
// tokenizer uses. This file also has the String.prototype methods that the
// port needs on UTF-16 code units.

package marked

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// A JavaScript string is a []rune in this package: one rune for each UTF-16
// code unit. So an index and a length count code units, as they do in
// marked, and a slice of the string is String.prototype.substring.

// units converts a Go string to code units.
func units(s string) []rune { return js.UTF16Units(s) }

// str converts code units to a Go string. A surrogate without its partner
// keeps the three-byte form that internal/js defines.
func str(u []rune) string { return js.FromUTF16Units(u) }

// concat is the + operator on strings.
func concat(parts ...[]rune) []rune {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := make([]rune, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// clampIndex resolves a relative index as String.prototype.slice does.
func clampIndex(i, n int) int {
	if i < 0 {
		return max(i+n, 0)
	}
	return min(i, n)
}

// slice is String.prototype.slice(start, end).
func slice(s []rune, start, end int) []rune {
	start, end = clampIndex(start, len(s)), clampIndex(end, len(s))
	if start >= end {
		return s[:0]
	}
	return s[start:end]
}

// sliceFrom is String.prototype.slice(start).
func sliceFrom(s []rune, start int) []rune {
	return s[clampIndex(start, len(s)):]
}

// substring is String.prototype.substring(start, end).
func substring(s []rune, start, end int) []rune {
	start, end = min(max(start, 0), len(s)), min(max(end, 0), len(s))
	if start > end {
		start, end = end, start
	}
	return s[start:end]
}

// substringFrom is String.prototype.substring(start).
func substringFrom(s []rune, start int) []rune {
	return s[min(max(start, 0), len(s)):]
}

// indexOf is String.prototype.indexOf for one code unit.
func indexOf(s []rune, c rune) int {
	for i, r := range s {
		if r == c {
			return i
		}
	}
	return -1
}

// lastIndexOf is String.prototype.lastIndexOf for one code unit.
func lastIndexOf(s []rune, c rune) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// firstLine is s.split('\n', 1)[0].
func firstLine(s []rune) []rune {
	if i := indexOf(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// split is s.split(sep) for one code unit.
func split(s []rune, sep rune) [][]rune {
	out := make([][]rune, 0, 4)
	start := 0
	for i, r := range s {
		if r == sep {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

// join is Array.prototype.join for one code unit.
func join(parts [][]rune, sep rune) []rune {
	n := len(parts)
	for _, p := range parts {
		n += len(p)
	}
	out := make([]rune, 0, n)
	for i, p := range parts {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, p...)
	}
	return out
}

// trimStart is String.prototype.trimStart.
func trimStart(s []rune) []rune {
	for len(s) > 0 && js.IsWhiteSpace(s[0]) {
		s = s[1:]
	}
	return s
}

// trimEnd is String.prototype.trimEnd.
func trimEnd(s []rune) []rune {
	for len(s) > 0 && js.IsWhiteSpace(s[len(s)-1]) {
		s = s[:len(s)-1]
	}
	return s
}

// trim is String.prototype.trim.
func trim(s []rune) []rune { return trimEnd(trimStart(s)) }

// repeat is String.prototype.repeat for one code unit.
func repeat(c rune, n int) []rune {
	out := make([]rune, max(n, 0))
	for i := range out {
		out[i] = c
	}
	return out
}

// equal reports whether a JavaScript string equals a Go string of ASCII
// characters.
func equal(s []rune, ascii string) bool {
	if len(s) != len(ascii) {
		return false
	}
	for i, r := range s {
		if r != rune(ascii[i]) {
			return false
		}
	}
	return true
}

// searchNonSpace is s.search(other.nonSpaceChar), where nonSpaceChar is
// /[^ ]/: the index of the first code unit that is not a space, or -1.
func searchNonSpace(s []rune) int {
	for i, r := range s {
		if r != ' ' {
			return i
		}
	}
	return -1
}

// replaceTabs is s.replace(other.tabCharGlobal, '    '), where
// tabCharGlobal is /\t/g.
func replaceTabs(s []rune) []rune {
	if indexOf(s, '\t') < 0 {
		return s
	}
	out := make([]rune, 0, len(s)+12)
	for _, r := range s {
		if r == '\t' {
			out = append(out, ' ', ' ', ' ', ' ')
		} else {
			out = append(out, r)
		}
	}
	return out
}

// splitCells is splitCells(tableRow, count). count is 0 when the caller
// gives no count.
func splitCells(tableRow []rune, count int) [][]rune {
	// ensure that every cell-delimiting pipe has a space
	// before it to distinguish it from an escaped pipe
	//
	// This is tableRow.replace(other.findPipe, ...), where findPipe is /\|/g.
	row := make([]rune, 0, len(tableRow)+8)
	for offset, r := range tableRow {
		if r != '|' {
			row = append(row, r)
			continue
		}
		escaped := false
		curr := offset
		for curr--; curr >= 0 && tableRow[curr] == '\\'; curr-- {
			escaped = !escaped
		}
		if escaped {
			// odd number of slashes means | is escaped
			// so we leave it alone
			row = append(row, '|')
		} else {
			// add space before unescaped |
			row = append(row, ' ', '|')
		}
	}
	// This is row.split(other.splitPipe), where splitPipe is / \|/.
	var cells [][]rune
	start := 0
	for i := 0; i+1 < len(row); i++ {
		if row[i] == ' ' && row[i+1] == '|' {
			cells = append(cells, row[start:i])
			start = i + 2
			i++
		}
	}
	cells = append(cells, row[start:])

	// First/last cell in a row cannot be empty if it has no leading/trailing pipe
	if len(trim(cells[0])) == 0 {
		cells = cells[1:]
	}
	if len(cells) > 0 && len(trim(cells[len(cells)-1])) == 0 {
		cells = cells[:len(cells)-1]
	}

	if count != 0 {
		if len(cells) > count {
			cells = cells[:count]
		} else {
			for len(cells) < count {
				cells = append(cells, nil)
			}
		}
	}

	for i := range cells {
		// leading or trailing whitespace is ignored per the gfm spec
		//
		// The replace is .replace(other.slashPipe, '|'), where slashPipe is
		// /\\\|/g.
		cell := trim(cells[i])
		out := make([]rune, 0, len(cell))
		for j := 0; j < len(cell); j++ {
			if cell[j] == '\\' && j+1 < len(cell) && cell[j+1] == '|' {
				out = append(out, '|')
				j++
				continue
			}
			out = append(out, cell[j])
		}
		cells[i] = out
	}
	return cells
}

// rtrim removes trailing c characters. It is rtrim(str, c) with a falsy
// invert argument, which is the only form that the lexer uses.
func rtrim(s []rune, c rune) []rune {
	l := len(s)
	// Length of suffix matching the invert condition.
	suffLen := 0

	// Step left until we fail to match the invert condition.
	for suffLen < l && s[l-suffLen-1] == c {
		suffLen++
	}

	return s[:l-suffLen]
}

// findClosingBracket is findClosingBracket(str, b) for the brackets b[0]
// and b[1].
func findClosingBracket(s []rune, open, close rune) int {
	if indexOf(s, close) == -1 {
		return -1
	}

	level := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++
		} else if s[i] == open {
			level++
		} else if s[i] == close {
			level--
			if level < 0 {
				return i
			}
		}
	}
	if level > 0 {
		return -2
	}

	return -1
}
