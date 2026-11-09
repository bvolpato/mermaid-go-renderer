// Ported from packages/mermaid/src/diagram-api/regexes.ts and
// diagram-api/comments.ts (mermaid 12.2.0).

package config

import (
	"strings"
	"unicode/utf8"

	"github.com/dlclark/regexp2"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The regular expressions of this package run on regexp2, a backtracking
// engine like the one in V8, because upstream uses lookahead. The JavaScript
// classes are spelled out, because regexp2 reads them differently:
//
//	\s   the JavaScript class has the Unicode spaces, U+FEFF and the line terminators
//	\w   ASCII only in JavaScript
//	.    does not match \n, \r, U+2028 and U+2029 in JavaScript
//
// regexp2 counts in runes and JavaScript in UTF-16 code units. No pattern
// here can tell the two apart.
const (
	reS   = `[\t\n\v\f\r \u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000\ufeff]`
	reW   = `[A-Za-z0-9_]`
	reDot = `[^\n\r\u2028\u2029]`
)

func mustCompile(pattern string) *regexp2.Regexp {
	pattern = strings.NewReplacer(`\s`, reS, `\w`, reW, `<dot>`, reDot).Replace(pattern)
	return regexp2.MustCompile(pattern, regexp2.None)
}

// directiveRegex is
// /%{2}{\s*(?:(\w+)\s*:|(\w+))\s*(?:(\w+)|((?:(?!}%{2}).|\r?\n)*))?\s*(?:}%{2})?/gi.
// The i flag has no effect: the pattern has no letters.
var directiveRegex = mustCompile(`%{2}\{\s*(?:(\w+)\s*:|(\w+))\s*(?:(\w+)|((?:(?!\}%{2})<dot>|\r?\n)*))?\s*(?:\}%{2})?`)

// commentRegex is the pattern of cleanupComments: /^\s*%%(?!{)[^\n]+\n?/gm.
// With the m flag, ^ matches after every JavaScript line terminator.
var commentRegex = mustCompile(`(?:\A|(?<=[\n\r\u2028\u2029]))\s*%%(?!\{)[^\n]+\n?`)

// The text that each pattern above needs: a text without it has no match.
const (
	directiveMarker = "%%{"
	commentMarker   = "%%"
)

// replaceAll is text.replace(re, "") for a global regular expression whose
// matches are never empty. marker is a text that every match of re has: a
// text without it stays as it is, and is not searched.
func replaceAll(re *regexp2.Regexp, marker, text string) string {
	// The search below also replaces bytes that are not UTF-8, so only a
	// text of valid UTF-8 can take the short way.
	if !strings.Contains(text, marker) && utf8.ValidString(text) {
		return text
	}
	return replaceAllSearch(re, text)
}

// replaceAllSearch is replaceAll by a search of the text.
func replaceAllSearch(re *regexp2.Regexp, text string) string {
	runes := []rune(text)
	var sb strings.Builder
	pos := 0
	for pos <= len(runes) {
		m, err := re.FindRunesMatchStartingAt(runes, pos)
		if err != nil || m == nil {
			break
		}
		sb.WriteString(string(runes[pos:m.Index]))
		pos = m.Index + m.Length
		if m.Length == 0 {
			if pos < len(runes) {
				sb.WriteRune(runes[pos])
			}
			pos++
		}
	}
	if pos < len(runes) {
		sb.WriteString(string(runes[pos:]))
	}
	return sb.String()
}

// CleanupComments removes all lines starting with `%%` from the text that
// don't contain a `%%{` (cleanupComments).
func CleanupComments(text string) string {
	return js.TrimStart(replaceAll(commentRegex, commentMarker, text))
}

// isWhitespace is the test of one character against /\s/.
func isWhitespace(r rune) bool { return js.IsWhiteSpace(r) }

// isLineBreak is true for the characters `[\n\r]` means in frontMatterRegex.
func isLineBreak(r rune) bool { return r == '\n' || r == '\r' }

// stripAnyComments strips `%%` comment runs exactly like
// text.replace(anyCommentRegex, '\n'), where anyCommentRegex is /\s*%%.*\n/gm.
//
// The scan walks forward once. For each `%%` it takes the line's terminating
// newline as the end of the match and extends left over the preceding
// whitespace run, never past the previous match.
//
// A `%%` with no newline after it is left alone, because `%%.*\n` cannot
// match without one.
func stripAnyComments(text string) string {
	if !strings.Contains(text, commentMarker) && utf8.ValidString(text) {
		// No `%%`: the scan would copy the text.
		return text
	}
	return stripAnyCommentsScan(text)
}

// stripAnyCommentsScan is stripAnyComments by a scan of the text.
func stripAnyCommentsScan(text string) string {
	runes := []rune(text)
	var out strings.Builder
	// Everything before this index has been emitted or dropped; also the
	// floor for the leftward whitespace scan.
	consumed := 0
	searchFrom := 0

	for searchFrom < len(runes) {
		marker := indexRunes(runes, "%%", searchFrom)
		if marker == -1 {
			break
		}
		lineEnd := indexRunes(runes, "\n", marker)
		if lineEnd == -1 {
			// No newline left in the string, so this `%%` and every later
			// one cannot match.
			break
		}
		start := marker
		for start > consumed && isWhitespace(runes[start-1]) {
			start--
		}
		out.WriteString(string(runes[consumed:start]))
		out.WriteByte('\n')
		consumed = lineEnd + 1
		searchFrom = consumed
	}

	out.WriteString(string(runes[consumed:]))
	return out.String()
}

// indexRunes is text.indexOf(needle, from) for an ASCII needle.
func indexRunes(runes []rune, needle string, from int) int {
	n := []rune(needle)
	for i := from; i+len(n) <= len(runes); i++ {
		match := true
		for j := range n {
			if runes[i+j] != n[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// startsWithAt is text.startsWith(prefix, at).
func startsWithAt(runes []rune, prefix []rune, at int) bool {
	if at < 0 || at+len(prefix) > len(runes) {
		return false
	}
	for i, r := range prefix {
		if runes[at+i] != r {
			return false
		}
	}
	return true
}

// lastLineBreakInWhitespaceRun returns the index of the last `[\n\r]` in the
// whitespace run starting at from, or -1 if the run holds none. This is where
// a greedy `\s*[\n\r]+` stops: `\s*` takes the whole run, then gives
// characters back until the next one is a line break.
func lastLineBreakInWhitespaceRun(runes []rune, from int) int {
	index := -1
	for i := from; i < len(runes) && isWhitespace(runes[i]); i++ {
		if isLineBreak(runes[i]) {
			index = i
		}
	}
	return index
}

// frontMatterMatch is the result of matchFrontMatter.
type frontMatterMatch struct {
	// indent is the horizontal indent of the opening fence: capture group 1
	// of frontMatterRegex.
	indent string
	// body is the YAML body between the fences: capture group 2.
	body string
	// length is the length of the whole match in runes, so callers can slice
	// the front matter off.
	length int
}

// matchFrontMatter locates a front matter block exactly like
// text.match(frontMatterRegex), where frontMatterRegex is
// /^([^\S\n\r]*)-{3}\s*[\n\r](.*?)[\n\r]\1-{3}\s*[\n\r]+/s.
//
// The engine tries the longest opening first and shortens it one line break
// at a time until the rest matches, which is equivalent to:
//
//  1. Take the closing fences that could terminate a block: a line break,
//     `indent---`, then at least one more line break in the following
//     whitespace run.
//  2. The opening ends at the last line break of the run after `---` that
//     still leaves a closing fence after it, since a longer opening is
//     preferred but must leave one behind.
//  3. The body ends at the first closing fence after that, because the body
//     is lazy.
func matchFrontMatter(runes []rune) (frontMatterMatch, bool) {
	cursor := 0
	for cursor < len(runes) && isWhitespace(runes[cursor]) && !isLineBreak(runes[cursor]) {
		cursor++
	}
	indent := runes[:cursor]

	if !startsWithAt(runes, []rune("---"), cursor) {
		return frontMatterMatch{}, false
	}

	// Candidate opening ends: the line breaks of the whitespace run after the
	// opening `---`.
	var openingEnds []int
	for i := cursor + 3; i < len(runes) && isWhitespace(runes[i]); i++ {
		if isLineBreak(runes[i]) {
			openingEnds = append(openingEnds, i)
		}
	}
	if len(openingEnds) == 0 {
		return frontMatterMatch{}, false
	}

	closingFence := append(append([]rune{}, indent...), []rune("---")...)
	// A closing fence is only usable if the trailing `\s*[\n\r]+` matches too.
	closingEnd := func(i int) int {
		if i < len(runes) && isLineBreak(runes[i]) && startsWithAt(runes, closingFence, i+1) {
			return lastLineBreakInWhitespaceRun(runes, i+1+len(closingFence))
		}
		return -1
	}

	earliestBodyStart := openingEnds[0] + 1

	lastClosingFence := -1
	for i := earliestBodyStart; i < len(runes); i++ {
		if closingEnd(i) != -1 {
			lastClosingFence = i
		}
	}
	if lastClosingFence == -1 {
		return frontMatterMatch{}, false
	}

	// Longest opening that still leaves a closing fence behind. One always
	// exists: every closing fence sits at or after earliestBodyStart, which
	// is past openingEnds[0].
	openingEnd := openingEnds[0]
	for _, candidate := range openingEnds {
		if candidate < lastClosingFence {
			openingEnd = candidate
		}
	}

	bodyStart := openingEnd + 1
	for i := bodyStart; i <= lastClosingFence; i++ {
		if matchEnd := closingEnd(i); matchEnd != -1 {
			return frontMatterMatch{indent: string(indent), body: string(runes[bodyStart:i]), length: matchEnd + 1}, true
		}
	}

	return frontMatterMatch{}, false
}
