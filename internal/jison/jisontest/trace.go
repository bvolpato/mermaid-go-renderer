// Package jisontest records the trace of a parse of the Go driver in the
// format of the traces that tools/reference/jison-lib.mjs records from the
// generated JavaScript parser. Tests compare the two.
//
// A trace has one line for each event:
//
//	T name <tab> condition stack <tab> yylineno <tab> yyleng <tab> yylloc <tab> yytext
//	R production [<tab> location]
//	OK accept | OK return production
//	ERR name <tab> message
//	H token <tab> line <tab> text <tab> location <tab> expected
//
// T is a token that the lexer gives to the parser, R is a reduction, and H is
// the hash of a parse error. Texts are JSON strings.
package jisontest

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Num prints a field of a location. A field that is undefined in JavaScript
// prints as "undefined".
func Num(n int) string {
	if n == jison.Undefined {
		return "undefined"
	}
	return strconv.Itoa(n)
}

// LocText prints a location as first_line:first_column-last_line:last_column.
func LocText(l jison.Location) string {
	return Num(l.FirstLine) + ":" + Num(l.FirstColumn) + "-" + Num(l.LastLine) + ":" + Num(l.LastColumn)
}

// Recorder collects the events of one parse.
type Recorder struct {
	// Events holds the trace lines.
	Events []string
	// WithLoc adds the location (@$) to each reduction.
	WithLoc bool
}

// Token records a token. Use it as Parser.OnToken.
func (r *Recorder) Token(token int, lx *jison.Lexer) {
	r.Events = append(r.Events, "T "+lx.TokenName(token)+"\t"+strings.Join(lx.ConditionStack(), " ")+"\t"+
		strconv.Itoa(lx.Yylineno)+"\t"+strconv.Itoa(lx.Yyleng)+"\t"+LocText(lx.Yylloc)+"\t"+js.QuoteJSON(lx.Yytext))
}

// Reduce records a reduction. Call it first in the semantic action.
func (r *Recorder) Reduce(s *jison.ParserState, production int) {
	if r.WithLoc {
		r.Events = append(r.Events, "R "+strconv.Itoa(production)+"\t"+LocText(s.Loc))
	} else {
		r.Events = append(r.Events, "R "+strconv.Itoa(production))
	}
}

// Finish records the end of the parse. result and err are the results of
// Parser.Parse. halt is the production whose action returned the result, or a
// negative number when no action did.
func (r *Recorder) Finish(result any, err error, halt int) {
	var jerr *jison.Error
	switch {
	case errors.As(err, &jerr):
		r.Events = append(r.Events, "ERR "+jerr.Name+"\t"+js.QuoteJSON(jerr.Message))
		if h := jerr.Hash; h != nil {
			loc := "-"
			if h.Loc != nil {
				loc = LocText(*h.Loc)
			}
			r.Events = append(r.Events, "H "+js.QuoteJSON(h.Token)+"\t"+strconv.Itoa(h.Line)+"\t"+js.QuoteJSON(h.Text)+"\t"+
				loc+"\t"+js.QuoteJSON(strings.Join(h.Expected, ",")))
		}
	case err != nil:
		r.Events = append(r.Events, "GOERR "+err.Error())
	case halt >= 0:
		r.Events = append(r.Events, "OK return "+strconv.Itoa(halt))
	case result == true:
		r.Events = append(r.Events, "OK accept")
	default:
		r.Events = append(r.Events, fmt.Sprintf("GOERR unexpected result %v", result))
	}
}

// Diff returns the index of the first event that differs between two traces,
// or -1 when the traces are equal.
func Diff(want, got []string) int {
	for i := 0; i < len(want) || i < len(got); i++ {
		if i >= len(want) || i >= len(got) || want[i] != got[i] {
			return i
		}
	}
	return -1
}

// Line returns event i of a trace, or a marker for the end of the trace.
func Line(events []string, i int) string {
	if i >= 0 && i < len(events) {
		return events[i]
	}
	return "(end of trace)"
}

// UnquoteJSON reads a JSON string. Unlike encoding/json it keeps a lone
// surrogate escape, in the three-byte form that internal/js documents.
func UnquoteJSON(s string) (string, error) {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return "", fmt.Errorf("not a JSON string: %.40q", s)
	}
	s = s[1 : len(s)-1]
	if !strings.Contains(s, `\`) {
		return s, nil
	}
	units := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		c := s[i]
		if c != '\\' {
			r, size := utf8.DecodeRuneInString(s[i:])
			i += size
			if r >= 0x10000 {
				r -= 0x10000
				units = append(units, 0xd800+(r>>10), 0xdc00+(r&0x3ff))
			} else {
				units = append(units, r)
			}
			continue
		}
		if i+1 >= len(s) {
			return "", errors.New("bad escape")
		}
		i += 2
		switch s[i-1] {
		case '"', '\\', '/':
			units = append(units, rune(s[i-1]))
		case 'b':
			units = append(units, '\b')
		case 'f':
			units = append(units, '\f')
		case 'n':
			units = append(units, '\n')
		case 'r':
			units = append(units, '\r')
		case 't':
			units = append(units, '\t')
		case 'u':
			if i+4 > len(s) {
				return "", errors.New("bad \\u escape")
			}
			v, err := strconv.ParseUint(s[i:i+4], 16, 16)
			if err != nil {
				return "", err
			}
			units = append(units, rune(v))
			i += 4
		default:
			return "", errors.New("bad escape")
		}
	}
	return js.FromUTF16Units(units), nil
}
