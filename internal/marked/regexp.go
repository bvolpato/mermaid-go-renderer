// JavaScript regular expressions for the rules of marked 16.3.0
// (marked/src/rules.ts). The rules stay as JavaScript sources. This file
// translates a source to the syntax of regexp2 and runs it with the
// semantics of V8.

package marked

import (
	"fmt"
	"strings"

	"github.com/dlclark/regexp2"
)

// regex is a compiled JavaScript regular expression.
//
// A regex without the u flag matches UTF-16 code units, as JavaScript does:
// the input is a slice with one rune for each code unit. A regex with the u
// flag matches code points: see ustring.
type regex struct {
	re *regexp2.Regexp

	// first and needs let exec and test skip the engine when a match is not
	// possible. They do not change a result. rules.go sets them and gives
	// the reason for each rule.
	//
	// When first is not nil, the regex does not match an input that is not
	// empty and that starts with a code unit outside of first.
	first *[128]bool
	// When needs is not 0, the regex does not match an input without this
	// code unit.
	needs rune
}

// spaceRanges is the \s class of ECMAScript: WhiteSpace and LineTerminator.
var spaceRanges = [][2]rune{
	{0x09, 0x0d}, {0x20, 0x20}, {0xa0, 0xa0}, {0x1680, 0x1680}, {0x2000, 0x200a},
	{0x2028, 0x2029}, {0x202f, 0x202f}, {0x205f, 0x205f}, {0x3000, 0x3000}, {0xfeff, 0xfeff},
}

// wordRanges is the \w class of ECMAScript without the u and i flags.
var wordRanges = [][2]rune{{'0', '9'}, {'A', 'Z'}, {'_', '_'}, {'a', 'z'}}

var digitRanges = [][2]rune{{'0', '9'}}

// lineTerminatorRanges is the LineTerminator production of ECMAScript.
var lineTerminatorRanges = [][2]rune{{'\n', '\n'}, {'\r', '\r'}, {0x2028, 0x2029}}

// maxRune is the last value that a class can hold.
const maxRune = 0x10ffff

// complement returns the ranges that are not in the sorted ranges.
func complement(ranges [][2]rune) [][2]rune {
	var out [][2]rune
	next := rune(0)
	for _, r := range ranges {
		if r[0] > next {
			out = append(out, [2]rune{next, r[0] - 1})
		}
		next = r[1] + 1
	}
	if next <= maxRune {
		out = append(out, [2]rune{next, maxRune})
	}
	return out
}

// translator turns one JavaScript regular expression source into a regexp2
// pattern. It reads the syntax that the rules of marked use, and it
// returns an error for any other syntax.
//
// The pattern does not depend on the ECMAScript mode of regexp2 for the
// classes, the dot or the word boundary, because that mode differs from
// JavaScript: its dot matches U+2028 and U+2029, and its word boundary
// counts each Unicode letter as a word character. Each literal character
// is written as an escape, so the syntax of regexp2 cannot give it another
// meaning.
type translator struct {
	src        []rune
	pos        int
	out        strings.Builder
	ignoreCase bool
	multiline  bool
}

// translate returns the regexp2 pattern for a JavaScript source and flags.
// The flags g and u do not change the pattern. With the i flag each ASCII
// letter matches both cases and no other character, which is the result of
// the Canonicalize operation of ECMAScript without the u flag.
func translate(source, flags string) (string, error) {
	t := &translator{src: []rune(source)}
	for _, f := range flags {
		switch f {
		case 'i':
			t.ignoreCase = true
		case 'm':
			t.multiline = true
		case 'g', 'u':
		default:
			return "", fmt.Errorf("marked: unsupported regex flag %q", f)
		}
	}
	if err := t.run(); err != nil {
		return "", fmt.Errorf("marked: regex /%s/%s: %w", source, flags, err)
	}
	return t.out.String(), nil
}

func isASCIILetter(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

func isASCIIDigit(r rune) bool { return r >= '0' && r <= '9' }

func isWordChar(r rune) bool {
	return isASCIILetter(r) || isASCIIDigit(r) || r == '_'
}

// writeRune writes one character so that regexp2 reads it as a literal, in
// a class and out of a class. regexp2 has no escape for a character above
// U+FFFF, so such a character is written as it is. Only the end of a
// complement range is such a character.
func (t *translator) writeRune(r rune) {
	switch {
	case isASCIILetter(r) || isASCIIDigit(r) || r >= 0x10000:
		t.out.WriteRune(r)
	default:
		fmt.Fprintf(&t.out, "\\u%04X", r)
	}
}

func (t *translator) writeRanges(ranges [][2]rune) {
	for _, r := range ranges {
		t.writeRune(r[0])
		if r[1] != r[0] {
			t.out.WriteByte('-')
			t.writeRune(r[1])
		}
	}
}

func (t *translator) writeClass(negate bool, ranges [][2]rune) {
	t.out.WriteByte('[')
	if negate {
		t.out.WriteByte('^')
	}
	t.writeRanges(ranges)
	t.out.WriteByte(']')
}

func (t *translator) run() error {
	for t.pos < len(t.src) {
		c := t.src[t.pos]
		t.pos++
		switch c {
		case '\\':
			if err := t.escape(); err != nil {
				return err
			}
		case '[':
			if err := t.class(); err != nil {
				return err
			}
		case '.':
			t.writeClass(true, lineTerminatorRanges)
		case '^':
			if t.multiline {
				// The position is the start, or a line terminator comes before it.
				t.out.WriteString("(?<!")
				t.writeClass(true, lineTerminatorRanges)
				t.out.WriteByte(')')
			} else {
				t.out.WriteByte('^')
			}
		case '$':
			if t.multiline {
				return fmt.Errorf("$ with the m flag is not supported")
			}
			// regexp2 in ECMAScript mode matches $ at the end of the input only.
			t.out.WriteByte('$')
		case '(':
			t.out.WriteByte('(')
			if t.pos < len(t.src) && t.src[t.pos] == '?' {
				rest := string(t.src[t.pos:])
				if !strings.HasPrefix(rest, "?:") && !strings.HasPrefix(rest, "?=") && !strings.HasPrefix(rest, "?!") {
					return fmt.Errorf("unsupported group at offset %d", t.pos-1)
				}
				t.out.WriteString(rest[:2])
				t.pos += 2
			}
		case ')', '|', '*', '+', '?':
			t.out.WriteRune(c)
		case '{':
			// The rules have a brace out of a class only in a quantifier.
			n := t.quantifierLen()
			if n == 0 {
				return fmt.Errorf("a brace that is not a quantifier at offset %d", t.pos-1)
			}
			t.out.WriteString(string(t.src[t.pos-1 : t.pos-1+n]))
			t.pos += n - 1
		default:
			if t.ignoreCase && isASCIILetter(c) {
				t.writeClass(false, caseRanges(c, c))
			} else {
				t.writeRune(c)
			}
		}
	}
	return nil
}

// quantifierLen returns the length of the quantifier {n}, {n,} or {n,m}
// that starts at the brace before pos, or 0.
func (t *translator) quantifierLen() int {
	i := t.pos
	digits := func() bool {
		start := i
		for i < len(t.src) && isASCIIDigit(t.src[i]) {
			i++
		}
		return i > start
	}
	if !digits() {
		return 0
	}
	if i < len(t.src) && t.src[i] == ',' {
		i++
		digits()
	}
	if i < len(t.src) && t.src[i] == '}' {
		return i + 1 - (t.pos - 1)
	}
	return 0
}

// classItem is one member of a character class: a character, a range, the
// ranges of an escape such as \s, or a Unicode property escape.
type classItem struct {
	lo, hi   rune
	ranges   [][2]rune
	property string
	set      bool // ranges or property is in use
}

// escapeItem reads the escape after a backslash, in a class and out of a
// class: a set of characters, or one character. pos is after the backslash.
func (t *translator) escapeItem() (classItem, error) {
	if t.pos >= len(t.src) {
		return classItem{}, fmt.Errorf("pattern ends with a backslash")
	}
	c := t.src[t.pos]
	t.pos++
	switch c {
	case 'd':
		return classItem{ranges: digitRanges, set: true}, nil
	case 'w':
		return classItem{ranges: wordRanges, set: true}, nil
	case 's':
		return classItem{ranges: spaceRanges, set: true}, nil
	case 'S':
		return classItem{ranges: complement(spaceRanges), set: true}, nil
	case 'p':
		end := t.pos
		for end < len(t.src) && t.src[end] != '}' {
			end++
		}
		if t.pos >= len(t.src) || t.src[t.pos] != '{' || end == len(t.src) {
			return classItem{}, fmt.Errorf("bad property escape at offset %d", t.pos)
		}
		property := "\\p" + string(t.src[t.pos:end+1])
		t.pos = end + 1
		return classItem{property: property, set: true}, nil
	case 'n':
		return classItem{lo: '\n', hi: '\n'}, nil
	case 't':
		return classItem{lo: '\t', hi: '\t'}, nil
	case 'x':
		if t.pos+2 <= len(t.src) {
			var v rune
			if _, err := fmt.Sscanf(string(t.src[t.pos:t.pos+2]), "%02x", &v); err == nil {
				t.pos += 2
				return classItem{lo: v, hi: v}, nil
			}
		}
		return classItem{}, fmt.Errorf("bad hex escape at offset %d", t.pos)
	}
	if isWordChar(c) {
		return classItem{}, fmt.Errorf("unsupported escape \\%c at offset %d", c, t.pos)
	}
	// An identity escape.
	return classItem{lo: c, hi: c}, nil
}

// escape translates an escape out of a class. pos is after the backslash.
func (t *translator) escape() error {
	if t.pos < len(t.src) {
		switch c := t.src[t.pos]; {
		case c == 'b':
			// A word boundary. The rules have it only before a literal word
			// character. Then the right side is a word character, so the
			// left side is not: the start, or a character outside of \w.
			if t.pos+1 >= len(t.src) || !isWordChar(t.src[t.pos+1]) {
				return fmt.Errorf("\\b is supported only before a word character")
			}
			t.pos++
			t.out.WriteString("(?<!")
			t.writeClass(false, wordRanges)
			t.out.WriteByte(')')
			return nil
		case c >= '1' && c <= '9':
			if t.ignoreCase {
				return fmt.Errorf("a back reference with the i flag is not supported")
			}
			if t.pos+1 < len(t.src) && isASCIIDigit(t.src[t.pos+1]) {
				return fmt.Errorf("a back reference with two digits is not supported")
			}
			t.pos++
			t.out.WriteByte('\\')
			t.out.WriteRune(c)
			return nil
		}
	}
	item, err := t.escapeItem()
	if err != nil {
		return err
	}
	switch {
	case item.property != "":
		t.out.WriteString(item.property)
	case item.set:
		t.writeClass(false, item.ranges)
	default:
		t.writeRune(item.lo)
	}
	return nil
}

// caseRanges adds the other case of each ASCII letter in the range.
func caseRanges(lo, hi rune) [][2]rune {
	out := [][2]rune{{lo, hi}}
	if a, b := max(lo, 'a'), min(hi, 'z'); a <= b {
		out = append(out, [2]rune{a &^ 0x20, b &^ 0x20})
	}
	if a, b := max(lo, 'A'), min(hi, 'Z'); a <= b {
		out = append(out, [2]rune{a | 0x20, b | 0x20})
	}
	return out
}

// class translates a character class. pos is after the opening bracket.
// The grammar is the one of ECMAScript Annex B: a hyphen next to a class
// escape such as \w is a literal.
func (t *translator) class() error {
	negate := false
	if t.pos < len(t.src) && t.src[t.pos] == '^' {
		negate = true
		t.pos++
	}
	// atom reads one ClassAtom.
	atom := func() (classItem, error) {
		c := t.src[t.pos]
		t.pos++
		if c != '\\' {
			return classItem{lo: c, hi: c}, nil
		}
		return t.escapeItem()
	}
	var items []classItem
	for {
		if t.pos >= len(t.src) {
			return fmt.Errorf("unterminated character class")
		}
		if t.src[t.pos] == ']' {
			t.pos++
			break
		}
		item, err := atom()
		if err != nil {
			return err
		}
		// A range needs a hyphen and an end that is not the closing bracket.
		if t.pos+1 < len(t.src) && t.src[t.pos] == '-' && t.src[t.pos+1] != ']' {
			t.pos++
			end, err := atom()
			if err != nil {
				return err
			}
			if item.set || end.set {
				return fmt.Errorf("a class escape next to a hyphen is not supported")
			}
			if end.lo < item.lo {
				return fmt.Errorf("range out of order in a character class")
			}
			item.hi = end.lo
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return fmt.Errorf("an empty character class is not supported")
	}
	t.out.WriteByte('[')
	if negate {
		t.out.WriteByte('^')
	}
	for _, item := range items {
		switch {
		case item.property != "":
			t.out.WriteString(item.property)
		case item.set:
			t.writeRanges(item.ranges)
		case t.ignoreCase:
			t.writeRanges(caseRanges(item.lo, item.hi))
		default:
			t.writeRanges([][2]rune{{item.lo, item.hi}})
		}
	}
	t.out.WriteByte(']')
	return nil
}

// mustRegex compiles a JavaScript regular expression. The sources are
// constants of this package, so a failure is a programming error.
func mustRegex(source, flags string) *regex {
	pattern, err := translate(source, flags)
	if err != nil {
		panic(err)
	}
	re, err := regexp2.Compile(pattern, regexp2.ECMAScript)
	if err != nil {
		panic(fmt.Errorf("marked: regex /%s/%s: %w", source, flags, err))
	}
	return &regex{re: re}
}

// startsWith sets the characters that can start a match: see regex.first.
func (r *regex) startsWith(chars string) *regex {
	r.first = new([128]bool)
	for i := 0; i < len(chars); i++ {
		r.first[chars[i]] = true
	}
	return r
}

// skip reports whether the fields first and needs exclude a match on s.
func (r *regex) skip(s []rune) bool {
	if r.first != nil && len(s) > 0 && (s[0] >= 128 || !r.first[s[0]]) {
		return true
	}
	return r.needs != 0 && indexOf(s, r.needs) < 0
}

// exec is RegExp.prototype.exec for a regex without the g and u flags. s
// holds UTF-16 code units. The result is nil when there is no match.
func (r *regex) exec(s []rune) *regexp2.Match {
	if r.skip(s) {
		return nil
	}
	// regexp2 returns an error only when a match timeout is set.
	m, _ := r.re.FindRunesMatch(s)
	return m
}

// execAt is exec for a regex with the g flag and without the u flag:
// lastIndex is the position where the search starts.
func (r *regex) execAt(s []rune, lastIndex int) *regexp2.Match {
	m, _ := r.re.FindRunesMatchStartingAt(s, lastIndex)
	return m
}

// test is RegExp.prototype.test for a regex without the g and u flags.
func (r *regex) test(s []rune) bool {
	if r.skip(s) {
		return false
	}
	ok, _ := r.re.MatchRunes(s)
	return ok
}

// group returns capture group i of a match. ok is false when the group did
// not take part in the match, which is undefined in JavaScript.
func group(m *regexp2.Match, i int) (s []rune, ok bool) {
	g := m.GroupByNumber(i)
	if g == nil || len(g.Captures) == 0 {
		return nil, false
	}
	return g.Runes(), true
}

// replaceAll is String.prototype.replace for a regex with the g flag and
// without the u flag. repl returns the replacement of one match.
func replaceAll(r *regex, s []rune, repl func(m *regexp2.Match) []rune) []rune {
	var out []rune
	last := 0
	for pos := 0; pos <= len(s); {
		m := r.execAt(s, pos)
		if m == nil {
			break
		}
		if out == nil {
			out = make([]rune, 0, len(s))
		}
		out = append(out, s[last:m.Index]...)
		out = append(out, repl(m)...)
		last = m.Index + m.Length
		pos = last
		if m.Length == 0 {
			// AdvanceStringIndex after an empty match.
			pos++
		}
	}
	if out == nil {
		return s
	}
	return append(out, s[last:]...)
}

// replaceFirst is String.prototype.replace for a regex without the g flag.
func replaceFirst(r *regex, s []rune, repl func(m *regexp2.Match) []rune) []rune {
	m := r.exec(s)
	if m == nil {
		return s
	}
	out := make([]rune, 0, len(s))
	out = append(out, s[:m.Index]...)
	out = append(out, repl(m)...)
	return append(out, s[m.Index+m.Length:]...)
}

// ustring is a string as a regex with the u flag sees it.
type ustring struct {
	units []rune // UTF-16 code units
	cps   []rune // code points: a surrogate pair is one element
	// off[i] is the index in units of code point i, and off[len(cps)] is
	// len(units). off is nil when cps is units.
	off []int
}

func isHighSurrogate(r rune) bool { return r >= 0xd800 && r < 0xdc00 }
func isLowSurrogate(r rune) bool  { return r >= 0xdc00 && r < 0xe000 }

// newUString makes the code point view of a string of code units. A
// surrogate without its partner stays one code point, as it does in V8.
func newUString(units []rune) ustring {
	first := -1
	for i := 0; i+1 < len(units); i++ {
		if isHighSurrogate(units[i]) && isLowSurrogate(units[i+1]) {
			first = i
			break
		}
	}
	if first < 0 {
		return ustring{units: units, cps: units}
	}
	cps := make([]rune, 0, len(units))
	off := make([]int, 0, len(units)+1)
	for i := 0; i < len(units); i++ {
		off = append(off, i)
		if i+1 < len(units) && isHighSurrogate(units[i]) && isLowSurrogate(units[i+1]) {
			cps = append(cps, 0x10000+(units[i]-0xd800)<<10+(units[i+1]-0xdc00))
			i++
			continue
		}
		cps = append(cps, units[i])
	}
	off = append(off, len(units))
	return ustring{units: units, cps: cps, off: off}
}

// unitIndex returns the index in units of code point index i.
func (u *ustring) unitIndex(i int) int {
	if u.off == nil {
		return i
	}
	return u.off[i]
}

// cpIndex returns the index of the code point that holds code unit i. An
// index in the middle of a surrogate pair gives the pair: V8 steps back to
// the lead surrogate before it runs a regex with the u flag.
func (u *ustring) cpIndex(i int) int {
	if u.off == nil {
		return i
	}
	lo, hi := 0, len(u.off)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if u.off[mid] <= i {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// umatch is the result of exec for a regex with the u flag. The indexes
// count UTF-16 code units, as they do in JavaScript.
type umatch struct {
	u *ustring
	m *regexp2.Match
}

// execU is RegExp.prototype.exec for a regex with the u flag. lastIndex
// counts code units, and it is 0 for a regex without the g flag.
func (r *regex) execU(u *ustring, lastIndex int) (umatch, bool) {
	if lastIndex > len(u.units) {
		return umatch{}, false
	}
	m, _ := r.re.FindRunesMatchStartingAt(u.cps, u.cpIndex(lastIndex))
	if m == nil {
		return umatch{}, false
	}
	return umatch{u: u, m: m}, true
}

// index is the index property of the match.
func (m umatch) index() int { return m.u.unitIndex(m.m.Index) }

// end is the lastIndex of a regex with the g flag after the match.
func (m umatch) end() int { return m.u.unitIndex(m.m.Index + m.m.Length) }

// group returns capture group i as code units.
func (m umatch) group(i int) (s []rune, ok bool) {
	g := m.m.GroupByNumber(i)
	if g == nil || len(g.Captures) == 0 {
		return nil, false
	}
	return m.u.units[m.u.unitIndex(g.Index):m.u.unitIndex(g.Index+g.Length)], true
}

// firstLen is [...match[0]][0].length: the number of code units of the
// first code point of the match.
func (m umatch) firstLen() int {
	if m.m.Length == 0 {
		return 0
	}
	return m.u.unitIndex(m.m.Index+1) - m.u.unitIndex(m.m.Index)
}
