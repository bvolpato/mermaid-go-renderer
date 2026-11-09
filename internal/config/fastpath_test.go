package config

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// textParts are pieces of diagram texts: the words of the detectors in both
// cases, the kinds of white space, the markers of directives and comments,
// fences, and bytes that are not UTF-8.
func textParts() []string {
	parts := []string{
		" ", "\t", "\n", "\r", "\r\n", "\u00a0", "\ufeff", "\u2028", "\u2003", "\v", "\f", "\xff", "é",
		"%%", "%%{", "%%{init: {\"theme\": \"dark\"}}%%", "%%{wrap}%%", "}%%", "%% a comment\n", "%%{init: 'x'}%%\n",
		"---", "---\n", "---\ntitle: T\n---\n", "  ---\nconfig:\n  theme: forest\n  ---\n", "-", "<br>", "<b class=\"x\">", ">",
		"A --> B", "error", "ERROR", "Error", "x", ":", "-beta", "-BETA", " TD\n", "\n  a\n", "ſ", "K", "İ",
		"C4Context", "C4Container", "c4context", "C4Dynamic", "wrap", "init", "'", "\"",
	}
	for _, d := range detectors {
		parts = append(parts, d.id)
	}
	for _, word := range []string{
		"flowchart-elk", "mindmap", "architecture", "agentflow-beta", "kanban", "classDiagram", "erDiagram", "gantt",
		"info", "pie", "requirement", "requirementDiagram", "sequenceDiagram", "swimlane-beta", "graph", "flowchart",
		"timeline", "gitGraph", "stateDiagram", "journey", "quadrantChart", "sankey", "sankey-beta", "packet",
		"xychart", "block", "eventmodeling", "treeView-beta", "radar-beta", "ishikawa", "treemap", "railroad-beta",
		"railroad-ebnf-beta", "railroad-abnf-beta", "railroad-peg-beta", "venn-beta", "wardley-beta", "cynefin-beta",
		"usecase-beta",
	} {
		parts = append(parts, word, strings.ToUpper(word), strings.ToLower(word))
	}
	return parts
}

func randomText(rng *rand.Rand, parts []string) string {
	var sb strings.Builder
	for n := rng.Intn(7); n > 0; n-- {
		sb.WriteString(parts[rng.Intn(len(parts))])
	}
	return sb.String()
}

// detectTypeByPatterns is DetectType as it was before the detectors tested
// the start of the text: every step by a search of the whole text.
func detectTypeByPatterns(text string) (string, error) {
	runes := []rune(text)
	if frontMatter, ok := matchFrontMatter(runes); ok {
		text = string(runes[frontMatter.length:])
	}
	text = stripAnyCommentsScan(replaceAllSearch(directiveRegex, text))
	for _, d := range detectors {
		var match bool
		switch {
		case d.id == "error":
			match = js.TrimSpace(strings.ToLower(text)) == "error"
		case d.id == "---":
			match = strings.HasPrefix(js.TrimStart(strings.ToLower(text)), "---")
		case d.pattern != nil:
			match = d.pattern(text)
		default:
			match = d.detect(text)
		}
		if match {
			return d.id, nil
		}
	}
	return "", unknownDiagramError("No diagram type detected matching given configuration for text: " + text)
}

// TestFastPathsAgainstSearch compares the steps that return early for a
// text without a marker with the same steps by a search of the text.
func TestFastPathsAgainstSearch(t *testing.T) {
	parts := textParts()
	rng := rand.New(rand.NewSource(1))
	types := map[string]int{}
	for i := 0; i < 100000; i++ {
		text := randomText(rng, parts)
		if got, want := replaceAll(directiveRegex, directiveMarker, text), replaceAllSearch(directiveRegex, text); got != want {
			t.Fatalf("replaceAll(directiveRegex, %q) = %q, want %q", text, got, want)
		}
		if got, want := replaceAll(commentRegex, commentMarker, text), replaceAllSearch(commentRegex, text); got != want {
			t.Fatalf("replaceAll(commentRegex, %q) = %q, want %q", text, got, want)
		}
		if got, want := replaceAll(commentWithoutDirectives, commentMarker, text), replaceAllSearch(commentWithoutDirectives, text); got != want {
			t.Fatalf("replaceAll(commentWithoutDirectives, %q) = %q, want %q", text, got, want)
		}
		if got, want := stripAnyComments(text), stripAnyCommentsScan(text); got != want {
			t.Fatalf("stripAnyComments(%q) = %q, want %q", text, got, want)
		}
		gotType, gotErr := DetectType(text, nil)
		wantType, wantErr := detectTypeByPatterns(text)
		if gotType != wantType || (gotErr == nil) != (wantErr == nil) || (gotErr != nil && gotErr.Error() != wantErr.Error()) {
			t.Fatalf("DetectType(%q) = %q, %v, want %q, %v", text, gotType, gotErr, wantType, wantErr)
		}
		types[gotType]++
		// cleanupText: the two replacements, each without its test of the
		// text.
		wantClean := htmlTagRe.ReplaceAllStringFunc(crRe.ReplaceAllString(text, "\n"), func(match string) string {
			m := htmlTagRe.FindStringSubmatch(match)
			return "<" + m[1] + attributeRe.ReplaceAllString(m[2], "='${1}'") + ">"
		})
		if got := cleanupText(text); got != wantClean {
			t.Fatalf("cleanupText(%q) = %q, want %q", text, got, wantClean)
		}
	}
	// Every detector must have matched some text, or the comparison says
	// nothing about it.
	for _, d := range detectors {
		if types[d.id] == 0 {
			t.Errorf("no text was detected as %s", d.id)
		}
	}
	if types[""] == 0 {
		t.Error("every text had a type")
	}
}

// TestDirectiveScanAgainstSearch compares detectDirective and the functions
// on top of it for texts with and without directives. A scan keeps the
// position of the last search, so each text runs on a scan that has
// searched another text before.
func TestDirectiveScanAgainstSearch(t *testing.T) {
	parts := textParts()
	rng := rand.New(rand.NewSource(2))
	// search is detectDirective before it tested for the marker.
	search := func(scan *directiveScan, text string, typeMatches func(string) bool) ([]directive, bool) {
		text = strings.ReplaceAll(replaceAllSearch(commentWithoutDirectives, js.TrimSpace(text)), "'", `"`)
		runes := []rune(text)
		var result []directive
		for {
			if scan.lastIndex > len(runes) {
				scan.lastIndex = 0
				break
			}
			m, err := directiveRegex.FindRunesMatchStartingAt(runes, scan.lastIndex)
			if err != nil || m == nil {
				scan.lastIndex = 0
				break
			}
			scan.lastIndex = m.Index + m.Length
			if m.Length == 0 {
				scan.lastIndex++
			}
			g1, _ := group(m, 1)
			g2, _ := group(m, 2)
			if (g1 != "" && typeMatches(g1)) || (g2 != "" && typeMatches(g2)) {
				typ := g2
				if g1 != "" {
					typ = g1
				}
				var args any = js.Null
				if g3, _ := group(m, 3); g3 != "" {
					args = js.TrimSpace(g3)
				} else if g4, _ := group(m, 4); g4 != "" {
					parsed, err := parseJSON(js.TrimSpace(g4))
					if err != nil {
						return []directive{{args: js.Null}}, false
					}
					args = parsed
				}
				result = append(result, directive{typ: typ, hasType: true, args: args})
			}
		}
		if len(result) == 0 {
			return []directive{{typ: text, hasType: true, args: js.Null}}, false
		}
		return result, len(result) > 1
	}
	show := func(ds []directive, isArray bool, lastIndex int) string {
		var sb strings.Builder
		for _, d := range ds {
			sb.WriteString(d.typ + "|")
			if d.hasType {
				sb.WriteString("typed|")
			}
			sb.WriteString(graphOf(d.args) + ";")
		}
		if isArray {
			sb.WriteString("array")
		}
		return sb.String() + "@" + strings.Repeat("i", lastIndex)
	}
	for i := 0; i < 30000; i++ {
		first, second := randomText(rng, parts), randomText(rng, parts)
		got, want := &directiveScan{}, &directiveScan{}
		for _, typeMatches := range []func(string) bool{initDirectiveType, wrapDirectiveType} {
			for _, text := range []string{first, second} {
				gd, ga := got.detectDirective(text, typeMatches)
				wd, wa := search(want, text, typeMatches)
				if g, w := show(gd, ga, got.lastIndex), show(wd, wa, want.lastIndex); g != w {
					t.Fatalf("detectDirective(%q) after %q:\n got  %s\n want %s", text, first, g, w)
				}
			}
		}
	}
}
