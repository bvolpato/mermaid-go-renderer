// Ported from jison-lex 0.3.4, jison-lex/regexp-lexer.js
// (RegExpLexer.prototype) (MIT).

package jison

import (
	"math"
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Undefined is the value of a Location field that is undefined in JavaScript.
// It occurs only after Lexer.Reject: the generated test_match saves
// this.last_line, which does not exist, as the last line of the location.
const Undefined = math.MinInt32

// maxStalls is the number of lexer rules that can match in sequence without a
// token and without progress in the input. The JavaScript lexer calls itself
// for each such rule, and V8 stops an endless sequence with a RangeError.
const maxStalls = 10000

// Location is a token location (yylloc). Lines start at 1 and columns at 0.
// A column counts UTF-16 code units. Range is set only with the "ranges" option.
type Location struct {
	FirstLine   int
	LastLine    int
	FirstColumn int
	LastColumn  int
	Range       [2]int
}

// Lexer is the lexer of a generated parser. The fields Yytext, Yyleng,
// Yylineno and Yylloc are the JavaScript properties with the same names, and
// a lexer action can change them.
//
// Yytext is a Go string. If a rule matches one half of a surrogate pair,
// Yytext holds that half in the three-byte form that internal/js documents.
// Use js.Concat to join such texts.
type Lexer struct {
	Yytext   string
	Yyleng   int
	Yylineno int
	Yylloc   Location

	g      *Grammar
	yy     any
	action LexActionFunc

	buf   []rune // the input, one rune for each UTF-16 code unit
	pos   int    // this._input is buf[pos:]
	match []rune // this.match
	// this.matched is all the input that the lexer consumed. Until the first
	// Unput it is buf[:pos], and the lexer keeps no copy (ownMatched is false).
	matched    []rune
	ownMatched bool
	offset     int

	more      bool
	backtrack bool
	done      bool

	condNames []string // this.conditionStack
	condIdx   []int    // the index of each name in Grammar.conds, or condUnknown, or condEmpty

	err        error
	parseError func(message string, hash *ErrorHash) error
	sink       *matchSink
	unknown    []string
	scratch    []uint16
	bytes      []byte
	reach      []literalReach // one entry for each literal filter of the grammar
}

// literalReach caches the result of a literal filter. The lexer moves forward
// in small steps, so one scan of a line serves many tokens.
type literalReach struct {
	valid bool
	from  int // the position of the scan
	end   int // the first index at or after from whose unit is not in Before
	at    int // the first index in [atFor, end] at which the literal starts, or -1
	atFor int // the position for which at was computed
}

const (
	condUnknown = -1 // a name that is not a start condition
	condEmpty   = -2 // the empty name, which selects the INITIAL rules
)

// NewLexer returns a lexer for input. yy is the value that the lexer actions
// receive. The lexer actions are Grammar.LexAction.
func (g *Grammar) NewLexer(input string, yy any) (*Lexer, error) {
	if err := g.init(); err != nil {
		return nil, err
	}
	lx := &Lexer{g: g, action: g.LexAction, sink: newMatchSink()}
	lx.SetInput(input, yy)
	return lx, nil
}

// SetInput resets the lexer and sets a new input.
func (lx *Lexer) SetInput(input string, yy any) {
	lx.yy = yy
	// One rune for each code unit. The number of code units is at most the
	// number of bytes, so the buffer does not grow.
	if cap(lx.buf) < len(input) {
		lx.buf = make([]rune, 0, len(input))
	}
	lx.buf = js.AppendUTF16Units(lx.buf[:0], input)
	lx.pos = 0
	lx.more, lx.backtrack, lx.done = false, false, false
	lx.Yylineno, lx.Yyleng = 0, 0
	lx.Yytext = ""
	lx.matched = lx.matched[:0]
	lx.ownMatched = false
	lx.match = lx.match[:0]
	lx.condNames = append(lx.condNames[:0], "INITIAL")
	lx.condIdx = append(lx.condIdx[:0], lx.g.initial)
	lx.Yylloc = Location{FirstLine: 1, FirstColumn: 0, LastLine: 1, LastColumn: 0}
	lx.offset = 0
	lx.err = nil
	lx.unknown = lx.unknown[:0]
	lx.resetReach()
}

// resetReach drops the cached results of the literal filters. Call it when
// the input buffer changes.
func (lx *Lexer) resetReach() {
	if n := len(lx.g.filters); len(lx.reach) != n {
		lx.reach = make([]literalReach, n)
		return
	}
	for i := range lx.reach {
		lx.reach[i].valid = false
	}
}

// inReach reports whether the literal of filter fi can be part of a match at
// the current position.
func (lx *Lexer) inReach(fi int) bool {
	f := lx.g.filters[fi]
	r := &lx.reach[fi]
	if !r.valid || lx.pos < r.from || lx.pos > r.end {
		r.valid = true
		r.from = lx.pos
		r.end = f.reach(lx.buf, lx.pos)
		r.atFor = -1
	}
	if r.atFor < 0 || lx.pos < r.atFor || (r.at >= 0 && lx.pos > r.at) {
		r.atFor = lx.pos
		r.at = f.find(lx.buf, lx.pos, r.end)
	}
	return r.at >= 0
}

// YY returns the value that SetInput received.
func (lx *Lexer) YY() any { return lx.yy }

// Throw stops the lexer with an error, as a JavaScript "throw" in a lexer
// action does. The first error wins. The action must return after the call.
func (lx *Lexer) Throw(err error) {
	if lx.err == nil && err != nil {
		lx.err = err
	}
}

// Err returns the error that stopped the lexer, or nil.
func (lx *Lexer) Err() error { return lx.err }

// TokenNamed converts a token name to a token number, as the parser does for
// a string that a lexer action returns: self.symbols_[token] || token. A name
// that is not a symbol gets a number of its own, and the parser reports it by
// name. The empty name is no token.
//
// A name that is a decimal number is a special case. The parser uses the
// string as the key of table[state], and the keys of that object are the
// symbol numbers. So the name "9" is the symbol with the number 9.
func (lx *Lexer) TokenNamed(name string) (int, bool) {
	if name == "" {
		return 0, false
	}
	if id, ok := lx.g.symbolIDs[name]; ok && id != 0 {
		return id, true
	}
	if id, ok := canonicalIndex(name); ok && id < lx.g.numSymbols {
		return id, true
	}
	for i, u := range lx.unknown {
		if u == name {
			return unknownBase + i, true
		}
	}
	lx.unknown = append(lx.unknown, name)
	return unknownBase + len(lx.unknown) - 1, true
}

// canonicalIndex reads a name that is the decimal form of a small integer
// without a sign and without leading zeros.
func canonicalIndex(name string) (int, bool) {
	if len(name) > 6 || (len(name) > 1 && name[0] == '0') {
		return 0, false
	}
	n := 0
	for i := 0; i < len(name); i++ {
		if name[i] < '0' || name[i] > '9' {
			return 0, false
		}
		n = n*10 + int(name[i]-'0')
	}
	return n, true
}

// TokenName returns the text that the parser prints for a token in an error
// message: this.terminals_[symbol] || symbol.
func (lx *Lexer) TokenName(token int) string {
	if token >= unknownBase {
		if i := token - unknownBase; i < len(lx.unknown) {
			return lx.unknown[i]
		}
	}
	if token >= 0 && token < len(lx.g.terminal) && lx.g.terminal[token] {
		return lx.g.Symbols[token]
	}
	return strconv.Itoa(token)
}

// Input consumes one code unit of the input and returns it.
func (lx *Lexer) Input() string {
	if lx.pos >= len(lx.buf) {
		// this._input[0] is undefined, and ch.match fails.
		lx.Throw(&Error{Name: "TypeError", Message: "Cannot read properties of undefined (reading 'match')"})
		return ""
	}
	unit := lx.buf[lx.pos : lx.pos+1]
	ch := js.FromUTF16Units(unit)
	lx.Yytext = js.Concat(lx.Yytext, ch)
	lx.Yyleng++
	lx.offset++
	lx.match = append(lx.match, unit...)
	if lx.ownMatched {
		lx.matched = append(lx.matched, unit...)
	}
	if unit[0] == '\r' || unit[0] == '\n' {
		lx.Yylineno++
		lx.Yylloc.LastLine++
	} else {
		lx.Yylloc.LastColumn++
	}
	if lx.g.Ranges {
		lx.Yylloc.Range[1]++
	}
	lx.pos++
	return ch
}

// lineLengths splits code units at each line break (\r\n, \r or \n), as
// String.prototype.split(/(?:\r\n?|\n)/g) does, and returns the length of
// each part.
func lineLengths(units []rune) []int {
	parts := make([]int, 0, 4)
	start := 0
	for i := 0; i < len(units); i++ {
		switch units[i] {
		case '\r':
			parts = append(parts, i-start)
			if i+1 < len(units) && units[i+1] == '\n' {
				i++
			}
			start = i + 1
		case '\n':
			parts = append(parts, i-start)
			start = i + 1
		}
	}
	return append(parts, len(units)-start)
}

// Unput puts text back at the start of the input.
func (lx *Lexer) Unput(ch string) {
	lx.unput(js.UTF16Units(ch))
}

func (lx *Lexer) unput(units []rune) {
	n := len(units)
	lines := lineLengths(units)

	// From here on this.matched differs from the consumed input: keep a copy.
	if !lx.ownMatched {
		lx.matched = append(lx.matched[:0], lx.buf[:lx.pos]...)
		lx.ownMatched = true
	}
	// this._input = ch + this._input
	if n <= lx.pos && equalUnits(lx.buf[lx.pos-n:lx.pos], units) {
		lx.pos -= n
	} else {
		buf := make([]rune, 0, n+len(lx.buf)-lx.pos)
		buf = append(buf, units...)
		lx.buf = append(buf, lx.buf[lx.pos:]...)
		lx.pos = 0
		lx.resetReach()
	}
	yytext := js.UTF16Units(lx.Yytext)
	keep := len(yytext) - n
	if keep < 0 {
		keep = 0
	}
	lx.Yytext = js.FromUTF16Units(yytext[:keep])
	lx.offset -= n
	oldLines := lineLengths(lx.match)
	// The generated code removes one code unit here, whatever the length of ch is.
	if len(lx.match) > 0 {
		lx.match = lx.match[:len(lx.match)-1]
	}
	if len(lx.matched) > 0 {
		lx.matched = lx.matched[:len(lx.matched)-1]
	}
	if len(lines)-1 != 0 {
		lx.Yylineno -= len(lines) - 1
	}
	r := lx.Yylloc.Range
	if len(lines) > len(oldLines) {
		// oldLines[oldLines.length - lines.length] is undefined.
		lx.Throw(&Error{Name: "TypeError", Message: "Cannot read properties of undefined (reading 'length')"})
		return
	}
	lastColumn := oldLines[len(oldLines)-len(lines)] - lines[0]
	if len(lines) == len(oldLines) {
		lastColumn += lx.Yylloc.FirstColumn
	}
	lx.Yylloc = Location{
		FirstLine:   lx.Yylloc.FirstLine,
		LastLine:    lx.Yylineno + 1,
		FirstColumn: lx.Yylloc.FirstColumn,
		LastColumn:  lastColumn,
	}
	if lx.g.Ranges {
		lx.Yylloc.Range = [2]int{r[0], r[0] + lx.Yyleng - n}
	}
	lx.Yyleng = keep
}

func equalUnits(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// More makes the next match append to Yytext instead of replacing it.
func (lx *Lexer) More() { lx.more = true }

// Reject tells the lexer that the rule does not match, so that the lexer
// tests the next rule. It needs the "backtrack_lexer" option. Without the
// option it is a lexical error, as in the generated code.
func (lx *Lexer) Reject() {
	if lx.g.BacktrackLexer {
		lx.backtrack = true
		return
	}
	lx.fail("Lexical error on line "+strconv.Itoa(lx.Yylineno+1)+
		". You can only invoke reject() in the lexer when the lexer is of the backtracking persuasion (options.backtrack_lexer = true).\n"+
		lx.ShowPosition(), &ErrorHash{Text: "", Token: "", TokenIsNull: true, Line: lx.Yylineno})
}

// Less keeps the first n code units of the match and puts the rest back.
func (lx *Lexer) Less(n int) {
	start := n
	if start < 0 {
		start += len(lx.match)
		if start < 0 {
			start = 0
		}
	}
	if start > len(lx.match) {
		start = len(lx.match)
	}
	lx.unput(append([]rune(nil), lx.match[start:]...))
}

// Match returns the text that the current rule matched (this.match).
func (lx *Lexer) Match() string { return js.FromUTF16Units(lx.match) }

// Matched returns all the input that the lexer consumed (this.matched).
func (lx *Lexer) Matched() string { return js.FromUTF16Units(lx.matchedUnits()) }

func (lx *Lexer) matchedUnits() []rune {
	if lx.ownMatched {
		return lx.matched
	}
	return lx.buf[:lx.pos]
}

// Remaining returns the input that the lexer did not consume (this._input).
func (lx *Lexer) Remaining() string { return js.FromUTF16Units(lx.buf[lx.pos:]) }

func withoutNewlines(units []rune) []rune {
	out := make([]rune, 0, len(units))
	for _, u := range units {
		if u != '\n' {
			out = append(out, u)
		}
	}
	return out
}

func (lx *Lexer) pastInput() []rune {
	matched := lx.matchedUnits()
	n := len(matched) - len(lx.match)
	if n < 0 {
		n = 0
	}
	past := matched[:n]
	tail := past
	if len(tail) > 20 {
		tail = tail[len(tail)-20:]
	}
	out := make([]rune, 0, 23)
	if len(past) > 20 {
		out = append(out, '.', '.', '.')
	}
	return append(out, withoutNewlines(tail)...)
}

func (lx *Lexer) upcomingInput() []rune {
	next := lx.match
	if len(next) < 20 {
		rest := lx.buf[lx.pos:]
		if len(rest) > 20-len(next) {
			rest = rest[:20-len(next)]
		}
		next = append(append(make([]rune, 0, 20), next...), rest...)
	}
	out := next
	if len(out) > 20 {
		out = out[:20]
	}
	out = append([]rune(nil), out...)
	if len(next) > 20 {
		out = append(out, '.', '.', '.')
	}
	return withoutNewlines(out)
}

// PastInput returns the input before the current match, for error messages.
func (lx *Lexer) PastInput() string { return js.FromUTF16Units(lx.pastInput()) }

// UpcomingInput returns the current match and the input after it, for error messages.
func (lx *Lexer) UpcomingInput() string { return js.FromUTF16Units(lx.upcomingInput()) }

// ShowPosition returns two lines that show where the lexer is, for error messages.
func (lx *Lexer) ShowPosition() string {
	pre := lx.pastInput()
	out := append(append([]rune(nil), pre...), lx.upcomingInput()...)
	out = append(out, '\n')
	for range pre {
		out = append(out, '-')
	}
	out = append(out, '^')
	return js.FromUTF16Units(out)
}

// Begin pushes a start condition on the condition stack.
func (lx *Lexer) Begin(condition string) {
	lx.condNames = append(lx.condNames, condition)
	idx := condEmpty
	if condition != "" {
		var ok bool
		if idx, ok = lx.g.condIndex[condition]; !ok {
			idx = condUnknown
		}
	}
	lx.condIdx = append(lx.condIdx, idx)
}

// PushState is an alias of Begin.
func (lx *Lexer) PushState(condition string) { lx.Begin(condition) }

// PopState removes the top of the condition stack and returns it. The last
// condition stays on the stack.
func (lx *Lexer) PopState() string {
	n := len(lx.condNames) - 1
	if n > 0 {
		top := lx.condNames[n]
		lx.condNames = lx.condNames[:n]
		lx.condIdx = lx.condIdx[:n]
		return top
	}
	return lx.condNames[0]
}

// TopState returns the condition that is n places below the top of the stack.
func (lx *Lexer) TopState(n int) string {
	if n < 0 {
		n = -n
	}
	n = len(lx.condNames) - 1 - n
	if n >= 0 {
		return lx.condNames[n]
	}
	return "INITIAL"
}

// StateStackSize returns the number of conditions on the stack.
func (lx *Lexer) StateStackSize() int { return len(lx.condNames) }

// ConditionStack returns a copy of the condition stack, the top last.
func (lx *Lexer) ConditionStack() []string {
	return append([]string(nil), lx.condNames...)
}

// currentRules is _currentRules: the rules of the condition on top of the stack.
func (lx *Lexer) currentRules() *condition {
	switch idx := lx.condIdx[len(lx.condIdx)-1]; {
	case idx >= 0:
		return lx.g.conds[idx]
	case idx == condEmpty:
		return lx.g.conds[lx.g.initial]
	}
	// this.conditions[name] is undefined.
	lx.Throw(&Error{Name: "TypeError", Message: "Cannot read properties of undefined (reading 'rules')"})
	return nil
}

// fail is lexer.parseError.
func (lx *Lexer) fail(message string, hash *ErrorHash) {
	if lx.parseError != nil {
		lx.Throw(lx.parseError(message, hash))
		return
	}
	lx.Throw(&Error{Name: "Error", Message: message})
}

// lexerBackup is the state that test_match saves for Reject.
type lexerBackup struct {
	yylineno  int
	yylloc    Location
	yytext    string
	match     []rune
	matched   []rune
	ownMatch  bool
	yyleng    int
	offset    int
	more      bool
	buf       []rune
	pos       int
	condNames []string
	condIdx   []int
	done      bool
}

func (lx *Lexer) save() *lexerBackup {
	b := &lexerBackup{
		yylineno: lx.Yylineno,
		yylloc: Location{
			FirstLine:   lx.Yylloc.FirstLine,
			LastLine:    Undefined, // this.last_line does not exist
			FirstColumn: lx.Yylloc.FirstColumn,
			LastColumn:  lx.Yylloc.LastColumn,
		},
		yytext:    lx.Yytext,
		match:     append([]rune(nil), lx.match...),
		matched:   append([]rune(nil), lx.matched...),
		ownMatch:  lx.ownMatched,
		yyleng:    lx.Yyleng,
		offset:    lx.offset,
		more:      lx.more,
		buf:       lx.buf,
		pos:       lx.pos,
		condNames: append([]string(nil), lx.condNames...),
		condIdx:   append([]int(nil), lx.condIdx...),
		done:      lx.done,
	}
	if lx.g.Ranges {
		b.yylloc.Range = lx.Yylloc.Range
	}
	return b
}

func (lx *Lexer) restore(b *lexerBackup) {
	lx.Yylineno = b.yylineno
	lx.Yylloc = b.yylloc
	lx.Yytext = b.yytext
	lx.match = b.match
	lx.matched = b.matched
	lx.ownMatched = b.ownMatch
	lx.Yyleng = b.yyleng
	lx.offset = b.offset
	lx.more = b.more
	lx.buf = b.buf
	lx.pos = b.pos
	lx.condNames = b.condNames
	lx.condIdx = b.condIdx
	lx.done = b.done
	lx.resetReach()
}

// testMatch is test_match: it consumes a match of n code units of a rule and
// runs the action of the rule.
func (lx *Lexer) testMatch(n int, rule uint16) (int, bool) {
	var backup *lexerBackup
	if lx.g.BacktrackLexer {
		backup = lx.save()
	}
	text := lx.buf[lx.pos : lx.pos+n]

	// lines = match[0].match(/(?:\r\n?|\n).*/g): one entry for each line
	// break. The last entry is the line break and the text after it up to the
	// next line terminator, which U+2028 and U+2029 are as well.
	lines, tail := 0, 0
	for i := 0; i < n; i++ {
		switch text[i] {
		case '\r':
			if i+1 < n && text[i+1] == '\n' {
				i++
			}
			lines++
			tail = 0
			for j := i + 1; j < n && text[j] != '\r' && text[j] != '\n' && text[j] != 0x2028 && text[j] != 0x2029; j++ {
				tail++
			}
		case '\n':
			lines++
			tail = 0
			for j := i + 1; j < n && text[j] != '\r' && text[j] != '\n' && text[j] != 0x2028 && text[j] != 0x2029; j++ {
				tail++
			}
		}
	}
	if lines > 0 {
		lx.Yylineno += lines
	}
	loc := Location{
		FirstLine:   lx.Yylloc.LastLine,
		LastLine:    lx.Yylineno + 1,
		FirstColumn: lx.Yylloc.LastColumn,
	}
	if lines > 0 {
		loc.LastColumn = tail
	} else {
		loc.LastColumn = lx.Yylloc.LastColumn + n
	}
	lx.Yylloc = loc
	if lx.Yytext == "" {
		lx.Yytext = lx.text(text)
		lx.Yyleng = n
	} else {
		lx.Yytext = js.Concat(lx.Yytext, js.FromUTF16Units(text))
		lx.Yyleng = len(js.UTF16Units(lx.Yytext))
	}
	lx.match = append(lx.match, text...)
	if lx.g.Ranges {
		lx.Yylloc.Range = [2]int{lx.offset, lx.offset + lx.Yyleng}
		lx.offset += lx.Yyleng
	}
	lx.more = false
	lx.backtrack = false
	lx.pos += n
	if lx.ownMatched {
		lx.matched = append(lx.matched, text...)
	}

	var token int
	var ok bool
	if lx.action != nil {
		token, ok = lx.action(lx.yy, lx, int(rule), lx.condNames[len(lx.condNames)-1])
	}
	if lx.done && lx.pos < len(lx.buf) {
		lx.done = false
	}
	if ok {
		return token, true
	}
	if lx.backtrack && backup != nil {
		lx.restore(backup)
	}
	return 0, false
}

// text converts code units to a string with one allocation.
func (lx *Lexer) text(units []rune) string {
	lx.bytes = js.AppendFromUTF16Units(lx.bytes[:0], units)
	return string(lx.bytes)
}

// candidates returns the rules of a condition that can match the remaining
// input, in the order of the condition.
func (lx *Lexer) candidates(cond *condition, input []rune) []uint16 {
	if len(input) == 0 {
		return cond.nullable
	}
	unit := input[0]
	if unit < 128 {
		return cond.ascii[unit]
	}
	lx.scratch = lx.scratch[:0]
	for _, r := range cond.other {
		if lx.g.Rules[r].Nullable || lx.g.rules[r].first.has(unit) {
			lx.scratch = append(lx.scratch, r)
		}
	}
	return lx.scratch
}

// next returns the next match in the input. The second result is false when
// the matched rule has no token.
func (lx *Lexer) next() (int, bool) {
	if lx.done {
		return EOF, true
	}
	if lx.pos >= len(lx.buf) {
		lx.done = true
	}
	if !lx.more {
		lx.Yytext = ""
		lx.match = lx.match[:0]
	}
	cond := lx.currentRules()
	if cond == nil {
		return 0, false
	}
	input := lx.buf[lx.pos:]
	matched := false
	var best uint16
	bestLen := -1
	for _, r := range lx.candidates(cond, input) {
		if fi := lx.g.ruleFilter[r]; fi >= 0 && !lx.inReach(fi) {
			continue
		}
		n, err := lx.g.rules[r].match(input, lx.sink)
		if err != nil {
			lx.Throw(err)
			return 0, false
		}
		if n < 0 || (matched && n <= bestLen) {
			continue
		}
		matched, best, bestLen = true, r, n
		if lx.g.BacktrackLexer {
			token, ok := lx.testMatch(n, r)
			if lx.err != nil {
				return 0, false
			}
			if ok {
				return token, true
			}
			if lx.backtrack {
				matched = false
				continue // The action of the rule called Reject.
			}
			return 0, false
		}
		if !lx.g.Flex {
			break
		}
	}
	if matched {
		return lx.testMatch(bestLen, best)
	}
	if lx.pos >= len(lx.buf) {
		return EOF, true
	}
	lx.fail("Lexical error on line "+strconv.Itoa(lx.Yylineno+1)+". Unrecognized text.\n"+lx.ShowPosition(),
		&ErrorHash{Text: "", Token: "", TokenIsNull: true, Line: lx.Yylineno})
	return 0, false
}

// Lex returns the next token: the symbol number that a lexer action returns,
// or EOF at the end of the input.
func (lx *Lexer) Lex() (int, error) {
	if lx.err != nil {
		return 0, lx.err
	}
	least := len(lx.buf) - lx.pos
	stalls := 0
	for {
		token, ok := lx.next()
		if lx.err != nil {
			return 0, lx.err
		}
		if ok {
			return token, nil
		}
		// The JavaScript lex calls itself here. A sequence of rules that
		// never gives a token and never shortens the input ends in V8 with a
		// stack overflow.
		if rest := len(lx.buf) - lx.pos; rest < least {
			least = rest
			stalls = 0
		} else if stalls++; stalls > maxStalls {
			lx.Throw(&Error{Name: "RangeError", Message: "Maximum call stack size exceeded"})
			return 0, lx.err
		}
	}
}
