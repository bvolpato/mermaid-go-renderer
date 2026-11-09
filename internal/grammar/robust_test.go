package grammar

import (
	"math/rand"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
)

// TestRandomInputs feeds random text to every grammar. The driver handles
// user input, so it must return a result or an error for every input: no
// panic and no endless loop. The inputs are the start of a corpus input, then
// random pieces of the corpus in a random order with some random bytes (also
// invalid UTF-8) and random characters.
func TestRandomInputs(t *testing.T) {
	for _, entry := range grammars {
		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()
			tf := loadTrace(t, entry.name)
			var corpus strings.Builder
			for _, in := range tf.inputs {
				corpus.WriteString(in.input)
			}
			pool := corpus.String()
			rnd := rand.New(rand.NewSource(int64(len(entry.name)) * 7919))
			var okCount, errCount int
			for i := 0; i < 400; i++ {
				var b strings.Builder
				if i%4 != 0 {
					// Start as a corpus input does, so that the parser gets
					// past the first token.
					start := tf.inputs[rnd.Intn(len(tf.inputs))].input
					b.WriteString(start[:min(len(start), 12+rnd.Intn(60))])
				}
				for limit := b.Len() + 20 + rnd.Intn(300); b.Len() < limit; {
					switch rnd.Intn(12) {
					case 0:
						b.WriteByte(byte(rnd.Intn(256)))
					case 1:
						b.WriteRune(rune(rnd.Intn(0x11000)))
					default:
						from := rnd.Intn(len(pool))
						to := min(len(pool), from+1+rnd.Intn(40))
						b.WriteString(pool[from:to])
					}
				}
				state := &traceYY{}
				p := jison.Parser{
					Grammar: entry.g,
					YY:      state,
					PerformAction: func(yy any, s *jison.ParserState, production int) (any, error) {
						// Read every value and location that the action can read.
						for k := 0; k <= s.Len()+1; k++ {
							_, _ = s.V(k), s.L(k)
						}
						if tf.halts[production] {
							return haltValue{}, nil
						}
						return nil, nil
					},
				}
				if _, err := p.Parse(b.String()); err != nil {
					errCount++
				} else {
					okCount++
				}
			}
			t.Logf("%d inputs parse, %d inputs give an error", okCount, errCount)
		})
	}
}

// TestConcurrentParse runs parses of one grammar in several goroutines. A
// Grammar is shared by all parses, so every goroutine must get the trace of
// the JavaScript parser. Run it with -race.
func TestConcurrentParse(t *testing.T) {
	for _, name := range []string{"flow", "sequencediagram", "statediagram"} {
		var g *jison.Grammar
		for _, entry := range grammars {
			if entry.name == name {
				g = entry.g
			}
		}
		tf := loadTrace(t, name)
		inputs := tf.inputs
		if len(inputs) > 120 {
			inputs = inputs[:120]
		}
		var wg sync.WaitGroup
		failures := make([]int, 8)
		for w := range failures {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range inputs {
					in := inputs[(i+w*17)%len(inputs)]
					got := goTrace(name, g, tf.halts, in.input, in.withLoc)
					if strings.Join(got, "\n") != strings.Join(in.events, "\n") {
						failures[w]++
					}
				}
			}()
		}
		wg.Wait()
		for w, n := range failures {
			if n > 0 {
				t.Errorf("%s: goroutine %d: %d traces differ", name, w, n)
			}
		}
	}
}
