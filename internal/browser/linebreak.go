// This file ports no upstream file. It has the break opportunities between
// two printable ASCII characters, from a table that a probe recorded from
// Chromium (linebreak_ascii_gen.go). The comment that follows names the
// place of this data in Blink: platform/text/text_break_iterator.cc, which
// has the license LGPL-2.0-or-later at Chromium 156.0.8078.4. See NOTICE in
// this directory.

package browser

// This file has the break opportunities between two printable ASCII
// characters. Blink does not ask the Unicode line breaking algorithm for
// such a pair: it reads a table that is compatible with other browsers
// (ShouldBreakAfter and kAsciiLineBreakTable in
// platform/text/text_break_iterator.cc). For example:
//
//   - A line can break after a hyphen and after a question mark.
//   - A line can break before "(", "<", "[" and "{" after a character such
//     as "." or "]". So "[a].[b]" can break before the second bracket.
//   - A line cannot break after "." or "/" before a letter. So "a.b.c" and
//     a URL stay in one piece.
//   - A line can break between a hyphen and a digit only when a letter or a
//     digit is before the hyphen: the hyphen can be a minus sign.
//
// linebreak_ascii_gen.go has the table, which
// tools/reference/gen-browser-linebreak-probe.mjs reads from Chromium.

// asciiBreakBetween reports whether a line can break between the
// characters prev and next. before is the character before prev, or 0 when
// there is none. The second result is false when prev or next is not in
// the table: the caller then uses its other rules.
func asciiBreakBetween(before, prev, next rune) (canBreak, ok bool) {
	if prev < asciiBreakFirst || prev > asciiBreakLast || next < asciiBreakFirst || next > asciiBreakLast {
		return false, false
	}
	if prev == '-' && next >= '0' && next <= '9' {
		if before < asciiBreakFirst || before > asciiBreakLast {
			return false, true
		}
		return asciiBreakHyphenDigit[before-asciiBreakFirst] == '1', true
	}
	return asciiBreakTable[prev-asciiBreakFirst][next-asciiBreakFirst] == '1', true
}
