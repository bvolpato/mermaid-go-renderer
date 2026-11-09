package marked

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode"
)

type goldenRegex struct {
	Source string `json:"source"`
	Flags  string `json:"flags"`
}

type goldenRules struct {
	Versions map[string]string      `json:"versions"`
	Other    map[string]goldenRegex `json:"other"`
	Block    map[string]goldenRegex `json:"block"`
	Inline   map[string]goldenRegex `json:"inline"`
	Dynamic  map[string]goldenRegex `json:"dynamic"`
	Sets     map[string][][2]rune   `json:"sets"`
}

func loadGoldenRules(t *testing.T) goldenRules {
	t.Helper()
	data, err := os.ReadFile("testdata/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var g goldenRules
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

// normalizeSource removes the backslash that V8 puts before a slash in
// RegExp.prototype.source. An escaped slash and a slash are the same atom.
func normalizeSource(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			if s[i+1] != '/' {
				sb.WriteByte('\\')
			}
			sb.WriteByte(s[i+1])
			i++
			continue
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

func compareRules(t *testing.T, set string, got map[string]jsRegex, want map[string]goldenRegex) {
	t.Helper()
	for name, g := range got {
		w, ok := want[name]
		if !ok {
			t.Errorf("%s.%s: marked has no such rule", set, name)
			continue
		}
		if normalizeSource(g.source) != normalizeSource(w.Source) || g.flags != w.Flags {
			t.Errorf("%s.%s:\n got /%s/%s\nwant /%s/%s", set, name, g.source, g.flags, w.Source, w.Flags)
		}
	}
}

// TestRuleSources compares the JavaScript source of each rule with the
// source that the marked of the mermaid bundle has.
func TestRuleSources(t *testing.T) {
	g := loadGoldenRules(t)
	if g.Versions["marked"] != "16.3.0" {
		t.Fatalf("the golden file is from marked %s", g.Versions["marked"])
	}
	compareRules(t, "other", otherSources, g.Other)
	compareRules(t, "block", blockSources(), g.Block)
	compareRules(t, "inline", inlineSources(), g.Inline)
	for name := range g.Block {
		if _, ok := blockSources()[name]; !ok {
			t.Errorf("block.%s is not ported", name)
		}
	}
	for name := range g.Inline {
		if _, ok := inlineSources()[name]; !ok {
			t.Errorf("inline.%s is not ported", name)
		}
	}

	dynamic := map[string]jsRegex{}
	for _, bull := range listBullets {
		dynamic["listItemRegex "+bull] = listItemRegex(bull)
	}
	for _, indent := range []int{1, 2, 3, 4, 5, 9} {
		suffix := " " + string(rune('0'+indent))
		dynamic["nextBulletRegex"+suffix] = nextBulletRegex(indent)
		dynamic["hrRegex"+suffix] = hrRegex(indent)
		dynamic["fencesBeginRegex"+suffix] = fencesBeginRegex(indent)
		dynamic["headingBeginRegex"+suffix] = headingBeginRegex(indent)
		dynamic["htmlBeginRegex"+suffix] = htmlBeginRegex(indent)
	}
	compareRules(t, "dynamic", dynamic, g.Dynamic)
	if len(dynamic) != len(g.Dynamic) {
		t.Errorf("dynamic: %d rules, the golden file has %d", len(dynamic), len(g.Dynamic))
	}
}

// TestCharacterSets compares the character sets that the rules name with
// the sets of V8: \s, the dot, and the Unicode categories P, S, L and N.
// The categories come from the unicode package, so this test fails when the
// Unicode version of Go differs from the one of the golden file.
func TestCharacterSets(t *testing.T) {
	g := loadGoldenRules(t)
	inRanges := func(ranges [][2]rune, r rune) bool {
		for _, rg := range ranges {
			if r >= rg[0] && r <= rg[1] {
				return true
			}
		}
		return false
	}
	sets := map[string]func(r rune) bool{
		"space":       func(r rune) bool { return inRanges(spaceRanges, r) },
		"dot":         func(r rune) bool { return !inRanges(lineTerminatorRanges, r) },
		"punctuation": func(r rune) bool { return unicode.Is(unicode.P, r) },
		"symbol":      func(r rune) bool { return unicode.Is(unicode.S, r) },
		"letter":      func(r rune) bool { return unicode.Is(unicode.L, r) },
		"number":      func(r rune) bool { return unicode.Is(unicode.N, r) },
	}
	for name, in := range sets {
		want := g.Sets[name]
		if len(want) == 0 {
			t.Fatalf("the golden file has no set %q", name)
		}
		bad := 0
		i := 0
		for r := rune(0); r <= unicode.MaxRune; r++ {
			for i < len(want) && r > want[i][1] {
				i++
			}
			w := i < len(want) && r >= want[i][0]
			if in(r) != w {
				if bad < 5 {
					t.Errorf("%s: U+%04X: got %v, want %v (Go has Unicode %s, the golden file has %s)",
						name, r, in(r), w, unicode.Version, g.Versions["unicode"])
				}
				bad++
			}
		}
		if bad > 0 {
			t.Errorf("%s: %d code points differ", name, bad)
		}
	}

	// The compiled classes agree with the tables at the ends of each range.
	classes := map[string]*regex{
		"space":       mustRegex(`^\s$`, "u"),
		"dot":         mustRegex(`^.$`, "u"),
		"punctuation": mustRegex(`^\p{P}$`, "u"),
		"symbol":      mustRegex(`^\p{S}$`, "u"),
		"letter":      mustRegex(`^\p{L}$`, "u"),
		"number":      mustRegex(`^\p{N}$`, "u"),
	}
	for name, re := range classes {
		for _, rg := range g.Sets[name] {
			for _, r := range []rune{rg[0] - 1, rg[0], rg[1], rg[1] + 1} {
				if r < 0 || r > unicode.MaxRune {
					continue
				}
				if got, want := re.test([]rune{r}), sets[name](r); got != want {
					t.Errorf("%s: regex on U+%04X: got %v, want %v", name, r, got, want)
				}
			}
		}
	}
}
