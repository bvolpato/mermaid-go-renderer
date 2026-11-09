// Ported from jison-lex 0.3.4, jison-lex/regexp-lexer.js: the rule test in
// RegExpLexer.prototype.next, this._input.match(this.rules[rules[i]]) (MIT).

package jison

import (
	"github.com/dlclark/regexp2/v2"
)

// ruleMatcher tests one lexer rule at the start of the remaining input.
//
// The JavaScript lexer keeps the remaining input in a string and runs
// /^(?:rule)/ on it. A JavaScript regular expression without the "u" flag
// works on UTF-16 code units. So the input here is a slice with one rune for
// each code unit, and the pattern sees a surrogate pair as two characters, as
// V8 does. regexp2 does not copy the slice, so that a test at a new position
// costs no allocation.
type ruleMatcher struct {
	re    *regexp2.Regexp
	first unitSet
}

func newRuleMatcher(r *Rule) (*ruleMatcher, error) {
	// The backtracking stack has no limit, as in V8. The generator prints
	// every pattern in a small subset of the syntax, which regexp2 reads in
	// the same way with and without the ECMAScript option. The option selects
	// the ASCII meaning of \b.
	re, err := regexp2.Compile("^(?:"+r.Pattern+")", regexp2.ECMAScript, regexp2.OptionMaxBacktrackingStackSize(-1))
	if err != nil {
		return nil, err
	}
	return &ruleMatcher{re: re, first: unitSet(r.First)}, nil
}

// matchSink receives the length of a match. A lexer owns one sink and one
// callback, so that a rule test does not allocate a closure.
type matchSink struct {
	length int
	yield  func(regexp2.MatchView) bool
}

func newMatchSink() *matchSink {
	s := &matchSink{}
	s.yield = func(v regexp2.MatchView) bool {
		_, s.length, _ = v.GroupRange(0)
		return false
	}
	return s
}

// match returns the length in code units of the match of the rule at the
// start of input, or -1 when the rule does not match.
func (m *ruleMatcher) match(input []rune, sink *matchSink) (int, error) {
	sink.length = -1
	if err := m.re.FindRunesMatches(input, 0, sink.yield); err != nil {
		return -1, err
	}
	return sink.length, nil
}

// RuleTester tests single lexer rules. It exists for tests of the rule
// patterns. A RuleTester is not safe for concurrent use.
type RuleTester struct {
	g    *Grammar
	sink *matchSink
}

// NewRuleTester returns a RuleTester for the rules of the grammar.
func (g *Grammar) NewRuleTester() (*RuleTester, error) {
	if err := g.init(); err != nil {
		return nil, err
	}
	return &RuleTester{g: g, sink: newMatchSink()}, nil
}

// Match tests one lexer rule at the start of input, which holds one rune for
// each UTF-16 code unit. It returns the length of the match in code units, or
// -1. It is the equivalent of input.match(lexer.rules[rule]).
func (t *RuleTester) Match(rule int, input []rune) (int, error) {
	return t.g.rules[rule].match(input, t.sink)
}

// CanStart reports whether the lexer tests a rule for an input. The lexer
// skips the regular expression of a rule that cannot match: the first code
// unit of input is not in the First set of the rule and the rule is not
// Nullable, or the Literal of the rule is not in reach.
func (t *RuleTester) CanStart(rule int, input []rune) bool {
	if !t.g.Rules[rule].Nullable && (len(input) == 0 || !t.g.rules[rule].first.has(input[0])) {
		return false
	}
	if fi := t.g.ruleFilter[rule]; fi >= 0 {
		f := t.g.filters[fi]
		return f.find(input, 0, f.reach(input, 0)) >= 0
	}
	return true
}
