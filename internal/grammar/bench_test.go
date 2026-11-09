package grammar

import (
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
)

// BenchmarkParse parses the corpus fixtures of each grammar with a semantic
// action that does nothing. One iteration parses every fixture of the grammar
// one time. The extra metrics are the time and the allocations for one
// fixture and the number of tokens in a second.
func BenchmarkParse(b *testing.B) {
	for _, entry := range grammars {
		b.Run(entry.name, func(b *testing.B) {
			tf := loadTrace(b, entry.name)
			var inputs []string
			bytes := 0
			for _, in := range tf.inputs {
				if strings.HasPrefix(in.name, "mutant ") || strings.HasPrefix(in.name, "extra ") {
					continue
				}
				inputs = append(inputs, in.input)
				bytes += len(in.input)
			}
			effect := lexerEffects[entry.name]
			tokens := 0
			run := func(input string) {
				state := &traceYY{}
				p := jison.Parser{
					Grammar: entry.g,
					YY:      state,
					OnToken: func(int, *jison.Lexer) { tokens++ },
					PerformAction: func(yy any, s *jison.ParserState, production int) (any, error) {
						if tf.halts[production] {
							return haltValue{}, nil
						}
						if effect != nil {
							effect(state, production)
						}
						return nil, nil
					},
				}
				// A fixture that mermaid rejects ends with an error here too.
				_, _ = p.Parse(input)
			}
			for _, input := range inputs {
				run(input) // Build the tables and compile the rules before the timer starts.
			}
			tokens = 0
			b.SetBytes(int64(bytes))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, input := range inputs {
					run(input)
				}
			}
			b.StopTimer()
			perInput := float64(b.N * len(inputs))
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/perInput/1000, "µs/diagram")
			b.ReportMetric(float64(tokens)/b.Elapsed().Seconds(), "tokens/s")
		})
	}
}

// BenchmarkFirstParse measures the first parse of a grammar in a process: it
// builds the dense LR table and compiles every lexer rule.
func BenchmarkFirstParse(b *testing.B) {
	for _, name := range []string{"flow", "sequencediagram", "sankey"} {
		var template *jison.Grammar
		for _, entry := range grammars {
			if entry.name == name {
				template = entry.g
			}
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				// A copy of the exported fields is a grammar that is not built yet.
				g := &jison.Grammar{
					Name: template.Name, Symbols: template.Symbols, Terminals: template.Terminals,
					Productions: template.Productions, Table: template.Table, DefaultActions: template.DefaultActions,
					Rules: template.Rules, Conditions: template.Conditions, LexAction: template.LexAction,
				}
				if g.NumStates() == 0 {
					b.Fatal("grammar did not build")
				}
			}
		})
	}
}
