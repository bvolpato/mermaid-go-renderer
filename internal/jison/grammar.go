// Package jison runs parsers that jison 0.4.18 generates, with the behavior of
// the generated JavaScript.
//
// Ported from jison 0.4.18, jison/lib/jison.js (parser.parse, the LR driver),
// and from jison-lex 0.3.4, jison-lex/regexp-lexer.js (RegExpLexer.prototype,
// the lexer) (MIT).
//
// mermaid builds each *.jison grammar into a JavaScript module that holds the
// LALR(1) tables, the lexer rules, and two functions with the actions:
// parser.performAction (the semantic actions) and lexer.performAction (the
// lexer actions). tools/reference/gen-jison.mjs writes the tables and the
// rules of every grammar to a package under internal/grammar, with the lexer
// actions in Go. This package is the driver for those tables.
//
// # How to port the parser of a diagram
//
// Read internal/grammar/<name>/actions.js.txt. Its first part lists every
// production with the JavaScript of its semantic action. Write one function
// with the signature of PerformActionFunc that has a case for each production
// with an action:
//
//	func performAction(yy any, s *jison.ParserState, production int) (any, error) {
//		db := yy.(*DB)
//		switch production {
//		case 7: // $$ = $1.concat($2)
//			s.Result = append(s.V(1).([]string), s.S(2))
//		case 9: // yy.addLink($1, $3); return $1
//			if err := db.AddLink(s.S(1), s.S(3)); err != nil {
//				return nil, err
//			}
//			return s.V(1), nil
//		}
//		return nil, nil
//	}
//
// Then parse with the Parse function of the grammar package:
//
//	result, err := flow.Parse(text, db, performAction)
//
// text is the text that mermaid gives to the generated parser, not the text
// of the user: mermaid first removes the front matter, the directives and the
// comments, encodes the entities, adds a line break, and for some diagrams
// (flow, sankey, agentflow) changes the text in a wrapper of the parser.
//
// Some grammars read yy in a lexer action. The grammar package then has an
// interface LexYY, and the yy value must implement it.
//
// The test TestTrace in internal/grammar compares the tokens and the
// reductions of the Go driver with the generated JavaScript parser for every
// corpus fixture of a grammar:
//
//	go test ./internal/grammar -run 'TestTrace/flow' -v
//
// # Strings
//
// A JavaScript regular expression without the "u" flag works on UTF-16 code
// units, and so does this lexer. Lengths and columns count code units. A rule
// can match one half of a surrogate pair, for example a rule for one
// character on an emoji. The token text then holds a lone surrogate, in the
// form that the functions js.UTF16Units and js.FromUTF16Units of internal/js
// document. A semantic action that joins token texts must use js.Concat for
// such grammars, so that the two halves become one character again.
package jison

import (
	"fmt"
	"sort"
	"sync"
)

// Symbol numbers that every jison grammar has.
const (
	// EOF is the number of the "$end" symbol. The lexer returns it at the end of the input.
	EOF = 1
	// TError is the number of the "error" symbol.
	TError = 2
)

// unknownBase is the first number of the tokens that are not symbols of the
// grammar. The JavaScript lexer returns the name of such a token as a string.
const unknownBase = 1 << 24

// Rule is one lexer rule.
type Rule struct {
	// Pattern is the regular expression of the rule for regexp2 in ECMAScript
	// mode, without the "^(?:" and ")" that jison puts around it. Every
	// character class is a set of explicit UTF-16 code units, with the
	// case-insensitive option of the grammar already applied.
	Pattern string
	// First holds the code units that can start a match, as sorted ranges
	// lo, hi, lo, hi. The set can be larger than necessary.
	First []uint16
	// Nullable is true when the rule can match without consuming a code unit.
	Nullable bool
	// Before and Literal describe a rule that starts with an unbounded
	// repetition, such as .*direction\s+TB[^\n]*. Every match consists of
	// code units of the set Before (sorted ranges, as First), then the
	// literal, then the rest. Literal has two code units for each position of
	// the literal: the unit and its other case, or the unit two times. Both
	// fields are nil for other rules. The lexer uses them to skip the regular
	// expression when the literal is not in reach.
	Before  []uint16
	Literal []uint16
}

// Condition is one start condition of the lexer.
type Condition struct {
	Name      string
	Rules     []uint16 // rule numbers, in the order in which the lexer tests them
	Inclusive bool
}

// LexActionFunc runs the action of one lexer rule: it is the port of the
// generated lexer.performAction(yy, yy_, $avoiding_name_collisions, YY_START).
// rule is the number of the matched rule and startCondition is the start
// condition on top of the stack when the rule matched. The function returns
// the symbol number of the token and true, or false when the action returns
// no token. Use Lexer.TokenNamed for a token name that is not a constant.
type LexActionFunc func(yy any, lexer *Lexer, rule int, startCondition string) (token int, ok bool)

// PerformActionFunc runs the semantic action of one production: it is the
// port of the generated parser.performAction. production is the number of the
// reduced production. The function reads $1..$n with ParserState.V and sets
// $$ in ParserState.Result. A non-nil result is a JavaScript "return"
// statement in the action: the parse stops and Parse returns that value. A
// non-nil error is a JavaScript "throw": the parse stops with that error.
type PerformActionFunc func(yy any, state *ParserState, production int) (result any, err error)

// Grammar holds the tables of one generated parser. The generator writes the
// exported fields. A Grammar is safe for concurrent use.
type Grammar struct {
	// Name is the name of the grammar file without the extension.
	Name string
	// Symbols maps a symbol number to its name (the inverse of symbols_).
	Symbols []string
	// Terminals lists the symbol numbers of the terminals (the keys of terminals_).
	Terminals []uint16
	// Productions holds productions_ as pairs: the symbol number of the left
	// side and the length of the right side. Production 0 is "$accept".
	Productions []uint16
	// Table holds the LR table. For each state in order: the number of
	// entries, then for each entry the symbol number and the action. The low
	// two bits of an action are the kind (0 goto, 1 shift, 2 reduce, 3 accept)
	// and the other bits are the state or the production.
	Table []uint16
	// DefaultActions holds defaultActions as pairs: the state and the
	// production that the state reduces without a lookahead token.
	DefaultActions []uint16
	// Rules holds the lexer rules.
	Rules []Rule
	// Conditions holds the start conditions of the lexer.
	Conditions []Condition
	// CaseInsensitive, Flex, BacktrackLexer and Ranges are the lexer options
	// "case-insensitive", "flex", "backtrack_lexer" and "ranges".
	// CaseInsensitive is informative: the patterns already contain the folding.
	CaseInsensitive bool
	Flex            bool
	BacktrackLexer  bool
	Ranges          bool
	// ErrorRecovery is true when a production of the grammar uses the "error"
	// symbol. jison then generates the parse function with error recovery.
	ErrorRecovery bool
	// LexAction runs the lexer actions. The generated package sets it.
	LexAction LexActionFunc

	once       sync.Once
	initErr    error
	numSymbols int
	terminal   []bool
	symbolIDs  map[string]int
	action     []int32 // numStates * numSymbols actions, 0 when there is none
	defaults   []int32 // the default action of each state, 0 when there is none
	rules      []*ruleMatcher
	filters    []*literalFilter // the distinct Before and Literal pairs
	ruleFilter []int            // the index in filters for each rule, or -1
	conds      []*condition
	condIndex  map[string]int
	initial    int
}

// Kinds of an action in Grammar.Table.
const (
	actGoto   = 0
	actShift  = 1
	actReduce = 2
	actAccept = 3
)

// condition is a start condition with the lists of rules that can match for
// each first code unit.
type condition struct {
	name     string
	rules    []uint16
	ascii    [128][]uint16 // candidate rules when the next code unit is ASCII
	other    []uint16      // rules that can start with a code unit above U+007F, or match nothing
	nullable []uint16      // rules that can match at the end of the input
}

// init builds the tables that the driver uses. It runs one time.
func (g *Grammar) init() error {
	g.once.Do(func() { g.initErr = g.build() })
	return g.initErr
}

func (g *Grammar) build() error {
	g.numSymbols = len(g.Symbols)
	g.symbolIDs = make(map[string]int, g.numSymbols)
	for i, name := range g.Symbols {
		g.symbolIDs[name] = i
	}
	g.terminal = make([]bool, g.numSymbols)
	for _, t := range g.Terminals {
		if int(t) >= g.numSymbols {
			return fmt.Errorf("jison: grammar %s: terminal %d is not a symbol", g.Name, t)
		}
		g.terminal[t] = true
	}
	if len(g.Productions)%2 != 0 || len(g.DefaultActions)%2 != 0 {
		return fmt.Errorf("jison: grammar %s: malformed tables", g.Name)
	}

	// Count the states, then fill the dense table.
	states := 0
	for i := 0; i < len(g.Table); states++ {
		n := int(g.Table[i])
		i += 1 + 2*n
		if i > len(g.Table) {
			return fmt.Errorf("jison: grammar %s: malformed table", g.Name)
		}
	}
	g.action = make([]int32, states*g.numSymbols)
	state := 0
	for i := 0; i < len(g.Table); state++ {
		n := int(g.Table[i])
		i++
		for ; n > 0; n-- {
			sym, act := int(g.Table[i]), int32(g.Table[i+1])
			i += 2
			if sym >= g.numSymbols {
				return fmt.Errorf("jison: grammar %s: symbol %d in the table is not a symbol", g.Name, sym)
			}
			g.action[state*g.numSymbols+sym] = act
		}
	}
	g.defaults = make([]int32, states)
	for i := 0; i < len(g.DefaultActions); i += 2 {
		st := int(g.DefaultActions[i])
		if st >= states {
			return fmt.Errorf("jison: grammar %s: default action for state %d", g.Name, st)
		}
		g.defaults[st] = int32(g.DefaultActions[i+1])<<2 | actReduce
	}

	g.rules = make([]*ruleMatcher, len(g.Rules))
	g.ruleFilter = make([]int, len(g.Rules))
	filterIndex := map[string]int{}
	for i := range g.Rules {
		r := &g.Rules[i]
		m, err := newRuleMatcher(r)
		if err != nil {
			return fmt.Errorf("jison: grammar %s: lexer rule %d: %w", g.Name, i, err)
		}
		g.rules[i] = m
		g.ruleFilter[i] = -1
		if len(r.Literal) == 0 {
			continue
		}
		if len(r.Literal)%2 != 0 || len(r.Before)%2 != 0 {
			return fmt.Errorf("jison: grammar %s: lexer rule %d: malformed literal", g.Name, i)
		}
		// Rules with the same literal share one filter and one cache entry.
		key := string(appendKey(appendKey(nil, r.Before), []uint16{0xffff, 0})) + string(appendKey(nil, r.Literal))
		fi, ok := filterIndex[key]
		if !ok {
			fi = len(g.filters)
			filterIndex[key] = fi
			g.filters = append(g.filters, newLiteralFilter(r.Before, r.Literal))
		}
		g.ruleFilter[i] = fi
	}

	g.condIndex = make(map[string]int, len(g.Conditions))
	g.conds = make([]*condition, len(g.Conditions))
	g.initial = -1
	shared := map[string][]uint16{}
	for ci := range g.Conditions {
		c := &g.Conditions[ci]
		for _, r := range c.Rules {
			if int(r) >= len(g.Rules) {
				return fmt.Errorf("jison: grammar %s: condition %s has no rule %d", g.Name, c.Name, r)
			}
		}
		cond := &condition{name: c.Name, rules: c.Rules}
		for unit := 0; unit < 128; unit++ {
			var list []uint16
			for _, r := range c.Rules {
				if g.Rules[r].Nullable || g.rules[r].first.has(rune(unit)) {
					list = append(list, r)
				}
			}
			// Many code units have the same list. Keep one copy.
			key := string(appendKey(nil, list))
			if prev, ok := shared[key]; ok {
				list = prev
			} else {
				shared[key] = list
			}
			cond.ascii[unit] = list
		}
		for _, r := range c.Rules {
			if g.Rules[r].Nullable {
				cond.nullable = append(cond.nullable, r)
			}
			if g.Rules[r].Nullable || g.rules[r].first.aboveASCII() {
				cond.other = append(cond.other, r)
			}
		}
		g.conds[ci] = cond
		g.condIndex[c.Name] = ci
		if c.Name == "INITIAL" {
			g.initial = ci
		}
	}
	if g.initial < 0 {
		return fmt.Errorf("jison: grammar %s: no INITIAL condition", g.Name)
	}
	return nil
}

func appendKey(b []byte, list []uint16) []byte {
	for _, r := range list {
		b = append(b, byte(r), byte(r>>8))
	}
	return b
}

// NumStates returns the number of states of the LR table.
func (g *Grammar) NumStates() int {
	if g.init() != nil {
		return 0
	}
	return len(g.defaults)
}

// SymbolID returns the number of a symbol, as symbols_[name] does.
func (g *Grammar) SymbolID(name string) (int, bool) {
	if g.init() != nil {
		return 0, false
	}
	id, ok := g.symbolIDs[name]
	return id, ok
}

// SymbolName returns the name of a symbol number, or "" when there is none.
func (g *Grammar) SymbolName(id int) string {
	if id < 0 || id >= len(g.Symbols) {
		return ""
	}
	return g.Symbols[id]
}

// IsTerminal reports whether a symbol number is a key of terminals_.
func (g *Grammar) IsTerminal(id int) bool {
	if g.init() != nil {
		return false
	}
	return id >= 0 && id < len(g.terminal) && g.terminal[id]
}

// ProductionLHS returns the symbol number of the left side of a production.
func (g *Grammar) ProductionLHS(production int) int {
	return int(g.Productions[2*production])
}

// ProductionLen returns the length of the right side of a production.
func (g *Grammar) ProductionLen(production int) int {
	return int(g.Productions[2*production+1])
}

// NumProductions returns the number of productions, with "$accept".
func (g *Grammar) NumProductions() int {
	return len(g.Productions) / 2
}

// unitSet is a set of UTF-16 code units, as sorted ranges lo, hi, lo, hi.
type unitSet []uint16

func (s unitSet) has(unit rune) bool {
	if unit < 0 || unit > 0xffff {
		return false
	}
	u := uint16(unit)
	// Find the first range whose high end is not below u.
	i := sort.Search(len(s)/2, func(i int) bool { return s[2*i+1] >= u })
	return i < len(s)/2 && s[2*i] <= u
}

func (s unitSet) aboveASCII() bool {
	return len(s) > 0 && s[len(s)-1] >= 128
}

// literalFilter is the test for the Before and Literal fields of a rule.
type literalFilter struct {
	before  unitSet
	ascii   [128]bool // before, for the code units below 128
	literal []uint16
}

func newLiteralFilter(before, literal []uint16) *literalFilter {
	f := &literalFilter{before: unitSet(before), literal: literal}
	for unit := range f.ascii {
		f.ascii[unit] = f.before.has(rune(unit))
	}
	return f
}

// reach returns the first index at or after from whose code unit is not in
// the set Before, or len(buf).
func (f *literalFilter) reach(buf []rune, from int) int {
	i := from
	for ; i < len(buf); i++ {
		if u := buf[i]; u < 128 {
			if !f.ascii[u] {
				break
			}
		} else if !f.before.has(u) {
			break
		}
	}
	return i
}

// find returns the first index in [from, to] at which the literal starts, or -1.
func (f *literalFilter) find(buf []rune, from, to int) int {
	n := len(f.literal) / 2
	if last := len(buf) - n; to > last {
		to = last
	}
	first, firstAlt := rune(f.literal[0]), rune(f.literal[1])
	for q := from; q <= to; q++ {
		if u := buf[q]; u != first && u != firstAlt {
			continue
		}
		ok := true
		for i := 1; i < n; i++ {
			if u := buf[q+i]; u != rune(f.literal[2*i]) && u != rune(f.literal[2*i+1]) {
				ok = false
				break
			}
		}
		if ok {
			return q
		}
	}
	return -1
}
