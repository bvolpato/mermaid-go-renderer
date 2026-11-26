// Ported from jison 0.4.18, jison/lib/jison.js (parser.parse and parseError)
// (MIT).

package jison

import (
	"errors"
	"strconv"
	"strings"
	"sync"
)

var (
	emptyStrAny  any = ""
	asciiStrAny  [128]any
	commonTokens = map[string]any{
		"  ": "  ", "   ": "   ", "    ": "    ", "\r\n": "\r\n",
		"-->": "-->", "---": "---", "==>": "==>", "-.->": "-.->", "-.-": "-.-",
		"-->|": "-->|", "--": "--", "..": "..", "->": "->", "->>": "->>",
		"-->>": "-->>", "-x": "-x", "--x": "--x", "-)": "-)", "--)": "--)",
		"<--": "<--", "<-->": "<-->", "o--o": "o--o", "x--x": "x--x",
		"::": "::", ":::": ":::", ": ": ": ", "((": "((", "))": "))",
		"[[": "[[", "]]": "]]", "{{": "{{", "}}": "}}", "[(": "[(", ")]": ")]",
		"([": "([", "])": "])", "[/": "[/", "/]": "/]", "[\\": "[\\", "\\]": "\\]",
		"||": "||", "|o": "|o", "o|": "o|", "}|": "}|", "|{": "|{", "}o": "}o", "o{": "o{",
		"<|": "<|", "|>": "|>", "..>": "..>", "<..": "<..", "*--": "*--", "--*": "--*",
		"o--": "o--", "--o": "--o",
		"end": "end", "class": "class", "style": "style", "subgraph": "subgraph",
		"direction": "direction", "click": "click", "classDef": "classDef",
		"linkStyle": "linkStyle", "state": "state", "note": "note", "loop": "loop",
		"alt": "alt", "else": "else", "opt": "opt", "par": "par", "and": "and",
		"rect": "rect", "participant": "participant", "actor": "actor",
		"activate": "activate", "deactivate": "deactivate", "title": "title",
		"section": "section", "TB": "TB", "TD": "TD", "LR": "LR", "RL": "RL", "BT": "BT",
		"graph": "graph", "flowchart": "flowchart", "string": "string", "int": "int",
		"void": "void", "bool": "bool", "float": "float", "PK": "PK", "FK": "FK", "UK": "UK",
	}
)

func init() {
	for i := range asciiStrAny {
		asciiStrAny[i] = string(byte(i))
	}
}

func boxString(s string) any {
	if len(s) == 0 {
		return emptyStrAny
	}
	if len(s) == 1 && s[0] < 128 {
		return asciiStrAny[s[0]]
	}
	if len(s) <= 12 {
		if v, ok := commonTokens[s]; ok {
			return v
		}
	}
	return s
}

// vslot holds a semantic value on the parser value stack without boxing shifted
// token strings into any unless an action reads V(i) or uses $1 as Result.
type vslot struct {
	val   any
	str   string
	isStr bool
}

type parseScratch struct {
	lexer  Lexer
	state  ParserState
	stack  []int
	vstack []vslot
	lstack []Location
}

var parseScratchPool = sync.Pool{
	New: func() any {
		return &parseScratch{
			lexer:  Lexer{sink: newMatchSink()},
			stack:  make([]int, 0, 64),
			vstack: make([]vslot, 0, 32),
			lstack: make([]Location, 0, 32),
		}
	},
}

const maxPooledStackCap = 4096

func releaseParseScratch(sc *parseScratch) {
	clear(sc.vstack[:cap(sc.vstack)])
	sc.state = ParserState{}
	lx := &sc.lexer
	lx.g = nil
	lx.yy = nil
	lx.action = nil
	lx.parseError = nil
	lx.Yytext = ""
	lx.err = nil
	clear(lx.condNames[:cap(lx.condNames)])
	clear(lx.unknown[:cap(lx.unknown)])
	if cap(sc.stack) > maxPooledStackCap || cap(sc.vstack) > maxPooledStackCap || cap(sc.lstack) > maxPooledStackCap || cap(lx.buf) > 1<<18 {
		return
	}
	parseScratchPool.Put(sc)
}

// Error is a JavaScript exception from the generated parser or lexer.
// Message is error.message and Hash is error.hash.
type Error struct {
	// Name is the name of the JavaScript error class: "Error", "TypeError" or "RangeError".
	Name string
	// Message is the message. For a syntax error it starts with "Parse error
	// on line N:" or "Lexical error on line N.".
	Message string
	// Hash is the second argument of parseError, or nil for other errors.
	Hash *ErrorHash
}

func (e *Error) Error() string { return e.Message }

// ErrorHash is the "hash" object that the generated code gives to parseError.
type ErrorHash struct {
	// Text is lexer.match for a parse error and "" for a lexical error.
	Text string
	// Token is the name of the unexpected token. For a lexical error the
	// JavaScript value is null: Token is "" and TokenIsNull is true.
	Token       string
	TokenIsNull bool
	// Line is lexer.yylineno, which starts at 0.
	Line int
	// Loc is the location of the last shifted token. It is nil for a lexical error.
	Loc *Location
	// Expected lists the expected tokens, each in single quotes. It is nil
	// for a lexical error.
	Expected []string
	// Recoverable is set by a parser with error recovery when an error rule can take over.
	Recoverable bool
}

// ParserState gives a semantic action access to the parser: it holds the
// arguments and the "this" object of the generated performAction.
type ParserState struct {
	// Yytext, Yyleng and Yylineno are the values of the last shifted token.
	Yytext   string
	Yyleng   int
	Yylineno int
	// Result is $$ (this.$). Before the action it is $1, or nil for an empty production.
	Result any
	// Loc is @$ (this._$): the location from the first to the last symbol of the production.
	Loc Location
	// Lexer is the lexer of the parse (yy.lexer).
	Lexer *Lexer

	vstack []vslot
	lstack []Location
	n      int // the length of the right side
}

// V returns $i, the semantic value of symbol i of the right side. The first
// symbol is 1. An index below 1 reads the stack below the production, as in
// jison. The value of a token is its yytext, a string.
func (s *ParserState) V(i int) any {
	k := len(s.vstack) - 1 - (s.n - i)
	if k < 0 || k >= len(s.vstack) {
		return nil
	}
	slot := s.vstack[k]
	if slot.isStr {
		return boxString(slot.str)
	}
	return slot.val
}

// S returns $i when it is a string, and "" in all other cases.
func (s *ParserState) S(i int) string {
	k := len(s.vstack) - 1 - (s.n - i)
	if k < 0 || k >= len(s.vstack) {
		return ""
	}
	slot := s.vstack[k]
	if slot.isStr {
		return slot.str
	}
	v, _ := slot.val.(string)
	return v
}

// L returns @i, the location of symbol i of the right side.
func (s *ParserState) L(i int) Location {
	k := len(s.lstack) - 1 - (s.n - i)
	if k < 0 || k >= len(s.lstack) {
		return Location{}
	}
	return s.lstack[k]
}

// Len returns the length of the right side of the production.
func (s *ParserState) Len() int { return s.n }

// Parser runs a parse. Set the fields, then call Parse. A Parser with the
// same fields can run many parses, one at a time.
type Parser struct {
	Grammar *Grammar
	// YY is the object that the actions receive (parser.yy).
	YY any
	// PerformAction runs the semantic actions. When it is nil, no action runs.
	PerformAction PerformActionFunc
	// LexAction runs the lexer actions. When it is nil, Grammar.LexAction runs.
	LexAction LexActionFunc
	// OnToken, when not nil, sees each token that the lexer gives to the parser.
	OnToken func(token int, lexer *Lexer)
}

// Parse parses input and returns the result of the generated parse function:
// true when the parser accepts the input, or the value that a semantic action
// returns. The error is an *Error for an exception of the generated code, or
// the error of an action.
func (g *Grammar) Parse(input string, yy any, performAction PerformActionFunc) (any, error) {
	p := Parser{Grammar: g, YY: yy, PerformAction: performAction}
	return p.Parse(input)
}

// Parse parses input. See Grammar.Parse.
func (p *Parser) Parse(input string) (any, error) {
	g := p.Grammar
	if err := g.init(); err != nil {
		return nil, err
	}
	sc := parseScratchPool.Get().(*parseScratch)
	defer releaseParseScratch(sc)

	lexer := &sc.lexer
	lexer.g = g
	lexer.action = p.LexAction
	if lexer.action == nil {
		lexer.action = g.LexAction
	}
	lexer.SetInput(input, p.YY)

	// parseError only traces an error that an error rule can take over. It
	// throws all other errors.
	parseError := func(message string, hash *ErrorHash) error {
		if hash.Recoverable {
			return nil
		}
		return &Error{Name: "Error", Message: message, Hash: hash}
	}
	lexer.parseError = parseError

	stack := append(sc.stack[:0], 0)
	vstack := append(sc.vstack[:0], vslot{})
	lstack := sc.lstack[:0]
	state := &sc.state
	*state = ParserState{Lexer: lexer}
	defer func() {
		sc.stack = stack
		sc.vstack = vstack
		sc.lstack = lstack
	}()
	var (
		yytext      string
		yylineno    int
		yyleng      int
		recovering  int
		symbol      = -1 // -1 is "null or undefined"
		preError    = -1
		havePreErr  bool
		expected    []string
		numSymbols  = g.numSymbols
		productions = g.Productions
	)
	yyloc := lexer.Yylloc
	lstack = append(lstack, yyloc)

	lex := func() (int, error) {
		token, err := lexer.Lex()
		if err != nil {
			return 0, err
		}
		if p.OnToken != nil {
			p.OnToken(token, lexer)
		}
		return token, nil
	}

	// popStack removes n symbols from the stacks.
	popStack := func(n int) {
		stack = stack[:len(stack)-2*n]
		vstack = vstack[:len(vstack)-n]
		lstack = lstack[:len(lstack)-n]
	}

	for {
		st := stack[len(stack)-1]
		var action int32
		if action = g.defaults[st]; action == 0 {
			if symbol < 0 {
				var err error
				if symbol, err = lex(); err != nil {
					return nil, err
				}
			}
			if symbol < numSymbols {
				action = g.action[st*numSymbols+symbol]
			}
			// A goto entry is a number in the JavaScript table, and a number
			// has no length: the parser treats it as no action.
			if action&3 == actGoto {
				action = 0
			}
		}

		if action == 0 {
			// Handle the parse error.
			errorRuleDepth := -1 // -1 is "false", -2 is "undefined"
			errStr := ""
			locate := func(st int) int {
				probe := len(stack) - 1
				depth := 0
				for {
					if g.action[st*numSymbols+TError] != 0 {
						return depth
					}
					if st == 0 || probe < 2 {
						return -1
					}
					probe -= 2
					st = stack[probe]
					depth++
				}
			}
			if !g.ErrorRecovery || recovering == 0 {
				if g.ErrorRecovery {
					errorRuleDepth = locate(st)
				}
				expected = expected[:0]
				for sym := TError + 1; sym < numSymbols; sym++ {
					if g.terminal[sym] && g.action[st*numSymbols+sym] != 0 {
						expected = append(expected, "'"+g.Symbols[sym]+"'")
					}
				}
				errStr = "Parse error on line " + strconv.Itoa(yylineno+1) + ":\n" + lexer.ShowPosition() +
					"\nExpecting " + strings.Join(expected, ", ") + ", got '" + lexer.TokenName(symbol) + "'"
				loc := yyloc
				err := parseError(errStr, &ErrorHash{
					Text:        lexer.Match(),
					Token:       lexer.TokenName(symbol),
					Line:        lexer.Yylineno,
					Loc:         &loc,
					Expected:    append([]string(nil), expected...),
					Recoverable: g.ErrorRecovery && errorRuleDepth >= 0,
				})
				if err != nil {
					return nil, err
				}
			} else if !(havePreErr && preError == EOF) {
				errorRuleDepth = locate(st)
			} else {
				errorRuleDepth = -2
			}

			// The parser just recovered from another error.
			if recovering == 3 {
				if symbol == EOF || (havePreErr && preError == EOF) {
					if errStr == "" {
						errStr = "Parsing halted while starting to recover from another error."
					}
					return nil, &Error{Name: "Error", Message: errStr}
				}
				// Discard the current lookahead and take another.
				yyleng = lexer.Yyleng
				yytext = lexer.Yytext
				yylineno = lexer.Yylineno
				yyloc = lexer.Yylloc
				var err error
				if symbol, err = lex(); err != nil {
					return nil, err
				}
			}

			// Try to recover from the error.
			if errorRuleDepth == -1 {
				if errStr == "" {
					errStr = "Parsing halted. No suitable error recovery rule available."
				}
				return nil, &Error{Name: "Error", Message: errStr}
			}
			if errorRuleDepth == -2 {
				// popStack(undefined) sets stack.length to NaN.
				return nil, &Error{Name: "RangeError", Message: "Invalid array length"}
			}
			popStack(errorRuleDepth)

			// Save the lookahead token and insert the error symbol as the new lookahead.
			if symbol == TError {
				havePreErr = false
			} else {
				preError, havePreErr = symbol, true
			}
			symbol = TError
			st = stack[len(stack)-1]
			action = g.action[st*numSymbols+TError]
			recovering = 3 // Three tokens must shift before the parser reports a new error.
		}

		switch action & 3 {
		case actShift:
			stack = append(stack, symbol)
			vstack = append(vstack, vslot{str: lexer.Yytext, isStr: true})
			lstack = append(lstack, lexer.Yylloc)
			stack = append(stack, int(action>>2))
			symbol = -1
			if !havePreErr {
				yyleng = lexer.Yyleng
				yytext = lexer.Yytext
				yylineno = lexer.Yylineno
				yyloc = lexer.Yylloc
				if recovering > 0 {
					recovering--
				}
			} else {
				// An error just occurred. Take the lookahead from before the error again.
				symbol = preError
				havePreErr = false
			}

		case actReduce:
			production := int(action >> 2)
			n := int(productions[2*production+1])
			state.n = n
			state.vstack = vstack
			state.lstack = lstack
			state.Yytext, state.Yyleng, state.Yylineno = yytext, yyleng, yylineno
			if n > 0 {
				slot := vstack[len(vstack)-n]
				if slot.isStr {
					state.Result = boxString(slot.str)
				} else {
					state.Result = slot.val
				}
			} else {
				state.Result = nil
			}
			first := n
			if first == 0 {
				first = 1
			}
			last := lstack[len(lstack)-1]
			from := lstack[len(lstack)-first]
			state.Loc = Location{
				FirstLine:   from.FirstLine,
				LastLine:    last.LastLine,
				FirstColumn: from.FirstColumn,
				LastColumn:  last.LastColumn,
			}
			if g.Ranges {
				state.Loc.Range = [2]int{from.Range[0], last.Range[1]}
			}
			if p.PerformAction != nil {
				result, err := p.PerformAction(p.YY, state, production)
				if err != nil {
					return nil, err
				}
				if lexer.err != nil {
					return nil, lexer.err
				}
				if result != nil {
					return result, nil
				}
			}
			if n > 0 {
				stack = stack[:len(stack)-2*n]
				vstack = vstack[:len(vstack)-n]
				lstack = lstack[:len(lstack)-n]
			}
			lhs := int(productions[2*production])
			vstack = append(vstack, vslot{val: state.Result})
			lstack = append(lstack, state.Loc)
			newState := g.action[stack[len(stack)-1]*numSymbols+lhs]
			stack = append(stack, lhs, int(newState>>2))

		case actAccept:
			return true, nil

		default:
			// The JavaScript reads action[0] of an action that is undefined.
			return nil, &Error{Name: "TypeError", Message: "Cannot read properties of undefined (reading '0')"}
		}
	}
}

// ErrNotPorted is the error of a lexer action that still is a generated stub.
var ErrNotPorted = errors.New("jison: lexer action is not ported")

// NotPorted returns the error for the stub of a lexer rule.
func NotPorted(rule int) error {
	return &notPortedError{rule: rule}
}

type notPortedError struct{ rule int }

func (e *notPortedError) Error() string {
	return ErrNotPorted.Error() + ": rule " + strconv.Itoa(e.rule)
}

func (e *notPortedError) Unwrap() error { return ErrNotPorted }
