package marked

import (
	"strconv"
	"strings"
	"testing"

	"github.com/dlclark/regexp2"
)

// rulesExecGolden is rules_exec.json.gz. The generator is
// tools/reference/gen-marked-rules.mjs.
type rulesExecGolden struct {
	// A probe is the index of a case of lexer.json.gz and an offset in
	// UTF-16 code units.
	Probes [][2]int `json:"probes"`
	Rules  map[string]struct {
		Flags   string            `json:"flags"`
		Results map[string]string `json:"results"`
	} `json:"rules"`
}

// span writes ";start,end" for a group, or ";-" for a group that did not
// take part in the match.
func span(sb *strings.Builder, start, end int, ok bool) {
	if !ok {
		sb.WriteString(";-")
		return
	}
	sb.WriteByte(';')
	sb.WriteString(strconv.Itoa(start))
	sb.WriteByte(',')
	sb.WriteString(strconv.Itoa(end))
}

// execAll returns up to three matches of a rule in the format of the
// golden file. htmlGroups is true for the block html rule, whose first
// group is four groups in the Go pattern: see expandHTMLScript.
func execAll(re *regex, unicode, htmlGroups bool, s []rune) string {
	var sb strings.Builder
	u := newUString(s)
	lastIndex := 0
	for k := 0; k < 3; k++ {
		var m *regexp2.Match
		// index maps an index of the match to code units.
		index := func(i int) int { return i }
		if unicode {
			um, ok := re.execU(&u, lastIndex)
			if !ok {
				break
			}
			m = um.m
			index = u.unitIndex
		} else {
			if lastIndex > len(s) {
				break
			}
			m = re.execAt(s, lastIndex)
			if m == nil {
				break
			}
		}
		if k > 0 {
			sb.WriteByte('|')
		}
		start, end := index(m.Index), index(m.Index+m.Length)
		sb.WriteString(strconv.Itoa(start))
		sb.WriteByte(',')
		sb.WriteString(strconv.Itoa(end - start))
		groupSpan := func(i int) (int, int, bool) {
			g := m.GroupByNumber(i)
			if len(g.Captures) == 0 {
				return 0, 0, false
			}
			return index(g.Index), index(g.Index + g.Length), true
		}
		first := 1
		if htmlGroups {
			// Group 1 of marked is the one of the first four groups that
			// took part.
			var gs, ge int
			var ok bool
			for i := 1; i <= htmlScriptGroups && !ok; i++ {
				gs, ge, ok = groupSpan(i)
			}
			span(&sb, gs, ge, ok)
			first = htmlScriptGroups + 1
		}
		for i := first; i < m.GroupCount(); i++ {
			gs, ge, ok := groupSpan(i)
			span(&sb, gs, ge, ok)
		}
		lastIndex = end
		if end == start {
			// AdvanceStringIndex after an empty match.
			lastIndex++
			if unicode && end+1 < len(s) && isHighSurrogate(s[end]) && isLowSurrogate(s[end+1]) {
				lastIndex++
			}
		}
	}
	return sb.String()
}

// wholeMatches removes the capture groups from a result.
//
// The blockquote rule has capture groups in a group that repeats.
// JavaScript clears such groups at the start of each repetition, and
// regexp2 keeps the capture of an earlier repetition. The tokenizer reads
// only the whole match of this rule, so the test compares only that.
func wholeMatches(result string) string {
	matches := strings.Split(result, "|")
	for i, m := range matches {
		matches[i], _, _ = strings.Cut(m, ";")
	}
	return strings.Join(matches, "|")
}

// TestRuleMatches runs each compiled rule on probe strings and compares
// the matches and the capture groups with the ones of V8. This is the test
// of the translation of each JavaScript regex to regexp2.
func TestRuleMatches(t *testing.T) {
	var golden rulesExecGolden
	loadGolden(t, "rules_exec.json.gz", &golden)
	var cases []goldenCase
	loadGolden(t, "lexer.json.gz", &cases)
	probes := make([][]rune, len(golden.Probes))
	for i, p := range golden.Probes {
		if p[0] >= len(cases) {
			t.Fatalf("probe %d names case %d, and lexer.json.gz has %d cases", i, p[0], len(cases))
		}
		probes[i] = units(cases[p[0]].input(t))[p[1]:]
	}
	if len(probes) < 4000 {
		t.Fatalf("the golden file has %d probes", len(probes))
	}

	regexes := namedRegexes()
	total, matches, failed := 0, 0, 0
	for name, re := range regexes {
		name = strings.TrimPrefix(name, "rules.")
		if name == "inline.backpedal" {
			name = "inline._backpedal"
		}
		rule, ok := golden.Rules[name]
		if !ok {
			t.Errorf("the golden file has no rule %s", name)
			continue
		}
		unicode := strings.Contains(rule.Flags, "u")
		for i, s := range probes {
			want := rule.Results[strconv.Itoa(i)]
			got := execAll(re, unicode, name == "block.html", s)
			total++
			if got != "" {
				matches++
			}
			// exec and test use the prefilters of the rule. They must
			// agree with the engine on a match at the start.
			if !unicode {
				engine := re.execAt(s, 0)
				if m := re.exec(s); (m != nil) != (engine != nil) || m != nil && (m.Index != engine.Index || m.Length != engine.Length) {
					got += " and exec differs from the engine"
				}
				if re.test(s) != (engine != nil) {
					got += " and test differs from the engine"
				}
			}
			if name == "block.blockquote" {
				got, want = wholeMatches(got), wholeMatches(want)
			}
			if got != want {
				failed++
				if failed <= 10 {
					t.Errorf("%s on %q:\n got %s\nwant %s", name, string(s), got, want)
				}
			}
		}
	}
	if len(regexes) != len(golden.Rules) {
		t.Errorf("%d compiled rules, and the golden file has %d", len(regexes), len(golden.Rules))
	}
	t.Logf("%d rules, %d probes, %d matches, %d failed", len(regexes), len(probes), matches, failed)
	if failed > 0 {
		t.Errorf("%d of %d rule runs failed", failed, total)
	}
}
