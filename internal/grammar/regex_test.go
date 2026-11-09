package grammar

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/dlclark/regexp2/v2"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// regexGolden is a regex golden file: the match length that V8 gives for
// every lexer rule on every probe string.
type regexGolden struct {
	Format      string
	Grammar     string
	ProbeLength int
	Rules       []struct{ Source, Flags string }
	body        []byte
}

func loadRegexGolden(t testing.TB, name string) *regexGolden {
	t.Helper()
	data := readGzip(t, filepath.Join(goldenDir(), "regex", name+".bin.gz"))
	nl := bytes.IndexByte(data, '\n')
	if nl < 0 {
		t.Fatal("no header")
	}
	g := &regexGolden{body: data[nl+1:]}
	if err := json.Unmarshal(data[:nl], g); err != nil {
		t.Fatal(err)
	}
	if g.Format != "jison-regex 1" || g.Grammar != name {
		t.Fatalf("unexpected header %s", data[:nl])
	}
	return g
}

// eachProbe calls fn for every probe of the golden file with the expected
// match length of each rule, or -1.
func (g *regexGolden) eachProbe(t testing.TB, tf *traceFile, fn func(probe []rune, want []int)) {
	t.Helper()
	body := g.body
	next := func() int {
		v, n := binary.Uvarint(body)
		if n <= 0 {
			t.Fatal("regex golden file is truncated")
		}
		body = body[n:]
		return int(v)
	}
	want := make([]int, len(g.Rules))
	// matches reads the expectations of one probe. It returns false for a
	// probe that occurred before.
	matches := func() bool {
		n := next()
		if n == 0 {
			return false
		}
		for i := range want {
			want[i] = -1
		}
		rule := -1
		for ; n > 1; n-- {
			rule += next() + 1
			want[rule] = next()
		}
		return true
	}
	if n := next(); n != len(tf.inputs) {
		t.Fatalf("the regex golden file has %d inputs and the trace file has %d", n, len(tf.inputs))
	}
	for _, in := range tf.inputs {
		units := js.UTF16Units(in.input)
		offset := 0
		for n := next(); n > 0; n-- {
			offset += next()
			if !matches() {
				continue
			}
			end := offset + g.ProbeLength
			if end > len(units) {
				end = len(units)
			}
			fn(units[offset:end], want)
		}
	}
	for n := next(); n > 0; n-- {
		probe := make([]rune, next())
		for i := range probe {
			probe[i] = rune(next())
		}
		if matches() {
			fn(probe, want)
		}
	}
	if len(body) != 0 {
		t.Fatalf("%d bytes left in the regex golden file", len(body))
	}
}

// TestRules runs every lexer rule of every grammar on the probe strings of
// the golden files. The Go pattern must match the same number of UTF-16 code
// units as the JavaScript regular expression of the rule in V8. The first-unit
// filter of the lexer must admit every probe that matches.
func TestRules(t *testing.T) {
	for _, entry := range grammars {
		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()
			golden := loadRegexGolden(t, entry.name)
			tf := loadTrace(t, entry.name)
			if len(golden.Rules) != len(entry.g.Rules) {
				t.Fatalf("golden file has %d rules, grammar has %d", len(golden.Rules), len(entry.g.Rules))
			}
			tester, err := entry.g.NewRuleTester()
			if err != nil {
				t.Fatal(err)
			}
			probes, tests, positive, skipped := 0, 0, 0, 0
			bad := map[int]int{}
			golden.eachProbe(t, tf, func(probe []rune, want []int) {
				probes++
				for rule, w := range want {
					tests++
					got, err := tester.Match(rule, probe)
					if err != nil {
						t.Fatalf("rule %d: %v", rule, err)
					}
					if w >= 0 {
						positive++
					}
					canStart := tester.CanStart(rule, probe)
					if !canStart {
						skipped++
					}
					if got == w && (w < 0 || canStart) {
						continue
					}
					bad[rule]++
					if bad[rule] == 1 {
						t.Errorf("rule %d /%s/%s on %s: V8 matches %d code units, Go matches %d (the first-unit filter admits the probe: %v)",
							rule, golden.Rules[rule].Source, golden.Rules[rule].Flags, js.QuoteJSON(js.FromUTF16Units(probe)), w, got, canStart)
					}
				}
			})
			t.Logf("%d rules, %d probes, %d rule tests equal to V8 (%d matches; the first-unit filter skips %d%% of the tests)",
				len(golden.Rules), probes, tests, positive, 100*skipped/max(tests, 1))
			if len(bad) > 0 {
				t.Errorf("%d rules differ from V8", len(bad))
			}
		})
	}
}

// TestRulesVerbatim reports what happens when regexp2 gets the JavaScript
// source of each rule unchanged, with the ECMAScript option (and IgnoreCase
// for a case-insensitive grammar), instead of the pattern that the generator
// prints. It lists the rules that then differ from V8. These rules are the
// reason for the explicit code unit sets in the generated patterns. The test
// does not fail for a difference.
func TestRulesVerbatim(t *testing.T) {
	if testing.Short() {
		t.Skip("report only")
	}
	total, differ := 0, 0
	var report []string
	for _, entry := range grammars {
		golden := loadRegexGolden(t, entry.name)
		tf := loadTrace(t, entry.name)
		res := make([]*regexp2.Regexp, len(golden.Rules))
		compileErrors := map[int]string{}
		for i, r := range golden.Rules {
			options := []regexp2.CompileOption{regexp2.ECMAScript, regexp2.OptionMaxBacktrackingStackSize(-1)}
			if strings.Contains(r.Flags, "i") {
				options = append(options, regexp2.IgnoreCase)
			}
			re, err := regexp2.Compile("^(?:"+r.Source+")", options...)
			if err != nil {
				compileErrors[i] = err.Error()
				continue
			}
			res[i] = re
		}
		bad := map[int]int{}
		example := map[int]string{}
		length := -1
		yield := func(v regexp2.MatchView) bool {
			_, length, _ = v.GroupRange(0)
			return false
		}
		golden.eachProbe(t, tf, func(probe []rune, want []int) {
			for rule, w := range want {
				if res[rule] == nil {
					continue
				}
				length = -1
				if err := res[rule].FindRunesMatches(probe, 0, yield); err != nil {
					t.Fatal(err)
				}
				if length != w {
					bad[rule]++
					if bad[rule] == 1 {
						example[rule] = fmt.Sprintf("on %s V8 matches %d and regexp2 matches %d", js.QuoteJSON(js.FromUTF16Units(probe)), w, length)
					}
				}
			}
		})
		total += len(golden.Rules)
		differ += len(bad) + len(compileErrors)
		var rules []int
		for rule := range bad {
			rules = append(rules, rule)
		}
		sort.Ints(rules)
		for _, rule := range rules {
			report = append(report, fmt.Sprintf("%s rule %d /%s/%s: %d probes differ, for example %s",
				entry.name, rule, golden.Rules[rule].Source, golden.Rules[rule].Flags, bad[rule], example[rule]))
		}
		for rule, msg := range compileErrors {
			report = append(report, fmt.Sprintf("%s rule %d /%s/%s: does not compile: %s", entry.name, rule, golden.Rules[rule].Source, golden.Rules[rule].Flags, msg))
		}
	}
	t.Logf("%d of %d rules differ from V8 when regexp2 gets the JavaScript source unchanged", differ, total)
	for _, line := range report {
		t.Log(line)
	}
}
