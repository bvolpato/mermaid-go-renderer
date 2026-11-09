package markdown

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// graphemeGolden is graphemes.json.gz. The generator is
// tools/reference/gen-markdown-graphemes.mjs.
type graphemeGolden struct {
	Unicode  string `json:"unicode"`
	Contexts []struct {
		Context []rune               `json:"context"`
		Results map[string][][2]rune `json:"results"`
	} `json:"contexts"`
}

// TestGraphemeContexts segments code points in the contexts that the
// grapheme cluster rules name, and compares the segments with the ones of
// Intl.Segmenter in V8. It finds a code point whose properties differ from
// the Unicode version of V8.
//
// The golden file has the result of each code point as ranges. The test
// reads each code point of a range of up to 1024 code points. A longer
// range is an unassigned area, a private use area or a block of one script,
// and the test reads its ends and one code point in 16. With
// GRAPHEME_FULL=1 in the environment, the test reads each code point.
func TestGraphemeContexts(t *testing.T) {
	var golden graphemeGolden
	loadGolden(t, "graphemes.json.gz", &golden)
	if len(golden.Contexts) < 20 {
		t.Fatalf("the golden file has %d contexts", len(golden.Contexts))
	}
	full := os.Getenv("GRAPHEME_FULL") == "1"
	// One goroutine for each context.
	type result struct {
		total, failed int
		messages      []string
	}
	results := make([]result, len(golden.Contexts))
	var wg sync.WaitGroup
	for k, c := range golden.Contexts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := &results[k]
			// want[cp] is the index of the result of the code point, plus
			// one. skip[cp] is true for a code point that the test leaves out.
			want := make([]uint8, 0x110000)
			skip := make([]bool, 0x110000)
			var sigs []string
			for sig, ranges := range c.Results {
				sigs = append(sigs, sig)
				for _, r := range ranges {
					long := r[1]-r[0] >= 1024 && !full
					for cp := r[0]; cp <= r[1]; cp++ {
						want[cp] = uint8(len(sigs))
						skip[cp] = long && cp != r[0] && cp != r[1] && cp%16 != 0
					}
				}
			}
			var sb strings.Builder
			for cp := rune(0); cp <= 0x10ffff; cp++ {
				if cp >= 0xd800 && cp <= 0xdfff || skip[cp] {
					continue
				}
				if want[cp] == 0 {
					res.messages = append(res.messages, fmt.Sprintf("context %U: the golden file has no result for U+%04X", c.Context, cp))
					res.failed++
					return
				}
				sb.Reset()
				for _, p := range c.Context {
					if p < 0 {
						p = cp
					}
					sb.WriteRune(p)
				}
				sig := ""
				for i, g := range SplitTextToChars(sb.String()) {
					if i > 0 {
						sig += ","
					}
					sig += strconv.Itoa(len([]rune(g)))
				}
				res.total++
				if sig != sigs[want[cp]-1] {
					res.failed++
					if res.failed <= 4 {
						res.messages = append(res.messages, fmt.Sprintf("context %U with U+%04X: got segments %s, want %s", c.Context, cp, sig, sigs[want[cp]-1]))
					}
				}
			}
		}()
	}
	wg.Wait()
	total, failed := 0, 0
	for k, res := range results {
		total += res.total
		failed += res.failed
		for _, m := range res.messages {
			t.Error(m)
		}
		if res.failed > 0 {
			t.Errorf("context %U: %d code points failed", golden.Contexts[k].Context, res.failed)
		}
	}
	t.Logf("%d strings, %d failed (the golden file has Unicode %s)", total, failed, golden.Unicode)
	if failed > 0 {
		t.Errorf("%d of %d strings failed", failed, total)
	}
}
