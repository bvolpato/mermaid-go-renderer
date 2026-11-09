package jison_test

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison/jisontest"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The grammars of this test are in tools/reference/gen-jison-synthetic.mjs.
// They use the parts of jison that no mermaid grammar uses. The functions
// below are the lexer actions and the semantic actions of those grammars in
// Go. The golden file has the trace of the generated JavaScript parsers.

// synthetic is one grammar of the test with its actions.
type synthetic struct {
	lex     func(sym map[string]int, events *[]string) jison.LexActionFunc
	perform jison.PerformActionFunc
}

func itoa(n int) string { return strconv.Itoa(n) }

// rangeText prints a range as Array.prototype.toString does.
func rangeText(r [2]int) string { return itoa(r[0]) + "," + itoa(r[1]) }

// tokens returns a lexer action for rules that only return a token. An empty
// name is a rule without a token.
func tokens(sym map[string]int, names ...string) func(rule int) (int, bool) {
	return func(rule int) (int, bool) {
		if rule >= len(names) || names[rule] == "" {
			return 0, false
		}
		return sym[names[rule]], true
	}
}

var synthetics = map[string]synthetic{
	"calc": {
		lex: func(sym map[string]int, events *[]string) jison.LexActionFunc {
			simple := tokens(sym, "", "NUMBER", "*", "/", "-", "+", "(", ")", ";", "EOF")
			return func(yy any, lx *jison.Lexer, rule int, start string) (int, bool) {
				if rule == 10 {
					// INVALID is not a symbol of the grammar.
					return lx.TokenNamed("INVALID")
				}
				return simple(rule)
			}
		},
		perform: func(yy any, s *jison.ParserState, production int) (any, error) {
			num := func(i int) float64 { return s.V(i).(float64) }
			switch production {
			case 1: // return $1.join(' ');
				return strings.Join(s.V(1).([]string), " "), nil
			case 2: // $$ = [];
				s.Result = []string{}
			case 3: // $1.push($2); $$ = $1;
				s.Result = append(s.V(1).([]string), s.V(2).(string))
			case 4:
				l := s.L(1)
				s.Result = js.Str(num(1)) + "@" + jisontest.LocText(l) + "[" + rangeText(l.Range) + "]"
			case 5:
				s.Result = "error(" + s.Yytext + "," + itoa(s.Yyleng) + "," + itoa(s.Yylineno) + ")@" +
					itoa(s.L(1).FirstColumn) + "[" + rangeText(s.Loc.Range) + "]"
			case 6:
				s.Result = num(1) + num(3)
			case 7:
				s.Result = num(1) - num(3)
			case 8:
				s.Result = num(1) * num(3)
			case 9:
				s.Result = num(1) / num(3)
			case 10:
				s.Result = -num(2)
			case 11:
				s.Result = s.V(2)
			case 12:
				s.Result = js.Number(s.Yytext)
			}
			return nil, nil
		},
	},
	"flexlex": {
		lex: func(sym map[string]int, events *[]string) jison.LexActionFunc {
			simple := tokens(sym, "IF", "IFFY", "ID", "NUM", "EQ", "ASSIGN", "", "EOF")
			return func(yy any, lx *jison.Lexer, rule int, start string) (int, bool) {
				if rule == 8 {
					// The rule that the "flex" option adds: console.log(yytext).
					*events = append(*events, "LOG "+js.QuoteJSON(lx.Yytext))
					return 0, false
				}
				return simple(rule)
			}
		},
		perform: func(yy any, s *jison.ParserState, production int) (any, error) {
			switch production {
			case 1:
				return s.V(1), nil
			case 2:
				s.Result = ""
			case 3:
				s.Result = s.S(1) + "(" + s.S(2) + ")"
			}
			return nil, nil
		},
	},
	"backtrack": {
		lex: func(sym map[string]int, events *[]string) jison.LexActionFunc {
			simple := tokens(sym, "", "END", "TRI", "CHAR", "", "DIGIT", "", "NEVER", "EOF")
			return func(yy any, lx *jison.Lexer, rule int, start string) (int, bool) {
				switch rule {
				case 0:
					if lx.Yytext == "end" || len(lx.Yytext) > 5 {
						lx.Begin("special")
						lx.Reject()
						return 0, false
					}
					return sym["WORD"], true
				case 4:
					if len(lx.Yytext) > 2 {
						lx.Reject()
						return 0, false
					}
					return sym["NUM"], true
				}
				return simple(rule)
			}
		},
		perform: func(yy any, s *jison.ParserState, production int) (any, error) {
			switch production {
			case 1:
				return s.V(1), nil
			case 2:
				s.Result = ""
			case 3:
				s.Result = s.S(1) + "(" + s.S(2) + "@" + jisontest.LocText(s.L(2)) + ")"
			}
			return nil, nil
		},
	},
	"moreless": {
		lex: func(sym map[string]int, events *[]string) jison.LexActionFunc {
			return func(yy any, lx *jison.Lexer, rule int, start string) (int, bool) {
				switch rule {
				case 0:
					lx.Begin("str")
					lx.More()
				case 1, 2, 3:
					lx.More()
				case 4:
					lx.PopState()
					return sym["STRING"], true
				case 5:
					lx.Less(1)
					return sym["A"], true
				case 6:
					lx.Unput(">>x")
					return sym["OPEN"], true
				case 7:
					return sym["CLOSE"], true
				case 8:
					c := lx.Input()
					lx.Yytext = lx.Yytext + "|" + c + "|" + lx.TopState(0) + "|" + lx.TopState(1) + "|" + itoa(lx.StateStackSize())
					return sym["HASH"], true
				case 9:
					lx.Begin("incl")
					return sym["AT"], true
				case 10:
					lx.PopState()
					return sym["BANG"], true
				case 11:
					lx.Yytext = lx.PastInput() + "^" + lx.UpcomingInput() + "^" + lx.ShowPosition()
					return sym["Q"], true
				case 12:
					lx.Begin("nosuch")
					return sym["DOLLAR"], true
				case 13:
					lx.Begin("")
					return sym["PCT"], true
				case 14:
					return lx.TokenNamed("TILDE_UNKNOWN")
				case 15:
					lx.Unput("\n\n")
					return sym["AMP"], true
				case 16:
					lx.Less(4)
					return sym["LINE"], true
				case 17:
					return sym["WORD"], true
				case 18:
					return sym["NUM"], true
				case 20:
					return sym["EOF"], true
				}
				return 0, false
			}
		},
		perform: func(yy any, s *jison.ParserState, production int) (any, error) {
			switch production {
			case 1:
				return s.V(1), nil
			case 2:
				s.Result = ""
			case 3:
				s.Result = js.Concat(s.S(1), "<", s.S(2), ">")
			}
			return nil, nil
		},
	},
	"stall": {
		lex: func(sym map[string]int, events *[]string) jison.LexActionFunc {
			simple := tokens(sym, "", "", "NUM", "", "EOF")
			return func(yy any, lx *jison.Lexer, rule int, start string) (int, bool) {
				if rule == 1 {
					lx.Unput("1")
					return 0, false
				}
				return simple(rule)
			}
		},
		perform: func(yy any, s *jison.ParserState, production int) (any, error) {
			switch production {
			case 1:
				return s.V(1), nil
			case 2:
				s.Result = ""
			case 3:
				s.Result = s.S(1) + "(" + s.S(2) + ")"
			}
			return nil, nil
		},
	},
}

// TestSynthetic compares the Go driver with the generated JavaScript parsers
// of the synthetic grammars: tokens, locations, reductions, the value that
// the parse returns, and the errors.
func TestSynthetic(t *testing.T) {
	f, err := os.Open("testdata/synthetic.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	type testCase struct {
		grammar string
		input   string
		events  []string
	}
	var cases []testCase
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<22)
	for line := 0; sc.Scan(); line++ {
		text := sc.Text()
		switch {
		case line == 0:
			if text != "jison-synthetic 1" {
				t.Fatalf("unexpected header %q", text)
			}
		case strings.HasPrefix(text, "G "), text == "LOC":
		case strings.HasPrefix(text, "F "):
			cases = append(cases, testCase{grammar: text[2:]})
		case strings.HasPrefix(text, "I "):
			input, err := jisontest.UnquoteJSON(text[2:])
			if err != nil {
				t.Fatal(err)
			}
			cases[len(cases)-1].input = input
		default:
			cases[len(cases)-1].events = append(cases[len(cases)-1].events, text)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, c := range cases {
		g := syntheticGrammars[c.grammar]
		def, ok := synthetics[c.grammar]
		if g == nil || !ok {
			t.Fatalf("no grammar %q", c.grammar)
		}
		counts[c.grammar]++
		rec := &jisontest.Recorder{WithLoc: true}
		halt := -1
		p := jison.Parser{
			Grammar:   g,
			LexAction: def.lex(syntheticSymbols[c.grammar], &rec.Events),
			OnToken:   rec.Token,
			PerformAction: func(yy any, s *jison.ParserState, production int) (any, error) {
				rec.Reduce(s, production)
				result, err := def.perform(yy, s, production)
				if result != nil {
					halt = production
				}
				return result, err
			},
		}
		result, err := p.Parse(c.input)
		rec.Finish(result, err, halt)
		if err == nil && halt >= 0 {
			rec.Events = append(rec.Events, "V "+js.QuoteJSON(result.(string)))
		}
		if d := jisontest.Diff(c.events, rec.Events); d >= 0 {
			t.Errorf("%s %q: event %d differs\n  after: %s\n  want:  %s\n  got:   %s", c.grammar, c.input, d,
				jisontest.Line(c.events, d-1), jisontest.Line(c.events, d), jisontest.Line(rec.Events, d))
		}
	}
	t.Logf("%d inputs: %v", len(cases), counts)
}
