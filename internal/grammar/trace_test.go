package grammar

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison/jisontest"
)

// traceInput is one input of a trace file with the trace of the JavaScript parser.
type traceInput struct {
	name    string
	input   string
	withLoc bool // the trace has the location of each reduction
	events  []string
}

type traceFile struct {
	halts  map[int]bool // productions whose action has a return statement
	inputs []traceInput
}

func readGzip(t testing.TB, path string) []byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(zr); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// goldenDir returns the directory of the golden files. The environment
// variable JISON_GOLDEN_DIR selects another set, for example a larger set
// that tools/reference/gen-jison-golden.mjs wrote for a one-time run.
func goldenDir() string {
	if dir := os.Getenv("JISON_GOLDEN_DIR"); dir != "" {
		return dir
	}
	return "testdata"
}

func loadTrace(t testing.TB, name string) *traceFile {
	t.Helper()
	data := readGzip(t, filepath.Join(goldenDir(), "trace", name+".txt.gz"))
	tf := &traceFile{halts: map[int]bool{}}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 1<<20), 1<<26)
	var cur *traceInput
	for line := 0; sc.Scan(); line++ {
		text := sc.Text()
		switch {
		case line == 0:
			if text != "jison-trace 1 "+name {
				t.Fatalf("unexpected header %q", text)
			}
		case strings.HasPrefix(text, "halts"):
			for _, f := range strings.Fields(text)[1:] {
				n, err := strconv.Atoi(f)
				if err != nil {
					t.Fatal(err)
				}
				tf.halts[n] = true
			}
		case strings.HasPrefix(text, "F "):
			tf.inputs = append(tf.inputs, traceInput{name: text[2:]})
			cur = &tf.inputs[len(tf.inputs)-1]
		case strings.HasPrefix(text, "I "):
			input, err := jisontest.UnquoteJSON(text[2:])
			if err != nil {
				t.Fatalf("%s: %v", cur.name, err)
			}
			cur.input = input
		case text == "LOC":
			cur.withLoc = true
		default:
			cur.events = append(cur.events, text)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return tf
}

// traceYY is the yy object of a trace run. It is the Go form of recordingYY
// in tools/reference/jison-lib.mjs: the few members that a lexer action reads
// behave as in mermaid, and all other calls do nothing.
type traceYY struct {
	firstGraphDone    bool
	indentMode        bool
	consumeIndentText bool
}

func (y *traceYY) FirstGraph() bool {
	if !y.firstGraphDone {
		y.firstGraphDone = true
		return true
	}
	return false
}
func (y *traceYY) GetIndentMode() bool         { return y.indentMode }
func (y *traceYY) SetIndentMode(v bool)        { y.indentMode = v }
func (y *traceYY) ConsumeIndentText() bool     { return y.consumeIndentText }
func (y *traceYY) SetConsumeIndentText(v bool) { y.consumeIndentText = v }

// lexerEffects lists the semantic actions that change what the lexer does
// next. The semantic action of the trace test only records the production, so
// it repeats these effects. Only venn has such actions: the actions of the
// productions 9 to 16 (the "set" and "union" statements) end with
// yy.setIndentMode(true).
var lexerEffects = map[string]func(yy *traceYY, production int){
	"venn": func(yy *traceYY, production int) {
		if production >= 9 && production <= 16 {
			yy.SetIndentMode(true)
		}
	},
}

// haltValue is what the recording semantic action returns for a production
// whose JavaScript action has a return statement.
type haltValue struct{}

// goTrace runs the Go driver on one input and returns the trace in the format
// of tracer in tools/reference/jison-lib.mjs.
func goTrace(name string, g *jison.Grammar, halts map[int]bool, input string, withLoc bool) []string {
	rec := &jisontest.Recorder{WithLoc: withLoc}
	state := &traceYY{}
	effect := lexerEffects[name]
	halt := -1
	p := jison.Parser{
		Grammar: g,
		YY:      state,
		OnToken: rec.Token,
		PerformAction: func(yy any, s *jison.ParserState, production int) (any, error) {
			rec.Reduce(s, production)
			if halts[production] {
				halt = production
				return haltValue{}, nil
			}
			if effect != nil {
				effect(state, production)
			}
			return nil, nil
		},
	}
	result, err := p.Parse(input)
	rec.Finish(result, err, halt)
	return rec.Events
}

// TestTrace replays every input of the trace files through the Go driver. The
// tokens (name, condition stack, yylineno, yyleng, yylloc, yytext), the
// reductions and the result or the error message must be the trace of the
// JavaScript parser.
func TestTrace(t *testing.T) {
	for _, entry := range grammars {
		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()
			tf := loadTrace(t, entry.name)
			var corpus, mutants, extras, failed, tokens, reductions, errorsSeen int
			for _, in := range tf.inputs {
				switch {
				case strings.HasPrefix(in.name, "mutant "):
					mutants++
				case strings.HasPrefix(in.name, "extra "):
					extras++
				default:
					corpus++
				}
				got := goTrace(entry.name, entry.g, tf.halts, in.input, in.withLoc)
				for _, e := range got {
					switch e[0] {
					case 'T':
						tokens++
					case 'R':
						reductions++
					case 'E':
						errorsSeen++
					}
				}
				diff := jisontest.Diff(in.events, got)
				if diff < 0 {
					continue
				}
				failed++
				if failed > 5 {
					continue
				}
				t.Errorf("%s: event %d differs\n  after: %s\n  want:  %s\n  got:   %s", in.name, diff,
					jisontest.Line(in.events, diff-1), jisontest.Line(in.events, diff), jisontest.Line(got, diff))
			}
			t.Logf("%d of %d inputs have the trace of the JavaScript parser (%d corpus fixtures, %d mutants, %d extra inputs; %d tokens, %d reductions, %d errors)",
				len(tf.inputs)-failed, len(tf.inputs), corpus, mutants, extras, tokens, reductions, errorsSeen)
			if failed > 0 {
				t.Errorf("%d of %d inputs differ", failed, len(tf.inputs))
			}
		})
	}
}
