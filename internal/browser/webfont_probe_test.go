package browser

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// The tests in this file compare the engine with Chromium in a page that
// has the web fonts of mermaid-cli 12 (internal/webfonts: Recursive
// Variable, Open Sans and Font Awesome 7), as oracle.mjs --webfonts makes
// it. The engine uses a library with web fonts (Library.WithWebFonts).
//
// tools/reference/gen-browser-webfont-probes.mjs writes the cases and the
// expected values. The set text has one text style for each case with 61
// strings: ASCII, Latin-1, Latin Extended, Vietnamese, Cyrillic, Greek,
// symbols, characters outside every range, digits and punctuation, at 18
// sizes from 8 to 40 px, the weights 100 to 900, normal and italic, in nine
// font-family lists. Each string is measured as SVG text (the length and
// the box) and as an HTML box. The set label has the markup of mermaid
// labels with the words of the corpus.

// webFontProbeSets lists the case sets.
var webFontProbeSets = []string{"text", "label"}

// webFontReplay is the result of the replay of one case set. A value is a
// number, or a rectangle of four numbers.
type webFontReplay struct {
	cases int
	// exact and differ count the values by query kind.
	exact, differ map[string]int
	// gaps counts the values that differ in a known way, by the name of
	// the gap.
	gaps map[string]int
	// fails describes the first other values that differ.
	fails []string
	bad   int
}

// replayWebFontSet replays the cases of a set with a font library and
// compares each value with the value of Chromium. gap names the known gap
// of a value that differs, or returns "". ok is false when a file of the
// set is absent.
func replayWebFontSet(t *testing.T, set, platform string, lib *font.Library, gap func(c probeCase, q probeQuery, got, want any) string) (res webFontReplay, ok bool) {
	t.Helper()
	var cases struct{ Cases []probeCase }
	var results []probeResult
	if !readLinuxJSON(t, "webfont-"+set+"-cases.json.gz", &cases) || !readLinuxJSON(t, platform+"-webfont-"+set+"-results.json.gz", &results) {
		return res, false
	}
	if len(cases.Cases) == 0 || len(cases.Cases) != len(results) {
		t.Fatalf("%d cases, %d results", len(cases.Cases), len(results))
	}
	res.cases = len(cases.Cases)
	res.exact, res.differ, res.gaps = map[string]int{}, map[string]int{}, map[string]int{}
	for i, c := range cases.Cases {
		doc := NewDocumentWithFonts(lib)
		container := doc.Body.Append("div").Attr("id", "container")
		container.HTML(c.HTML)
		for qi, q := range c.Queries {
			var got []any
			if q.All {
				for _, el := range container.SelectAll(q.Sel) {
					got = append(got, runQuery(el, q))
				}
			} else {
				got = []any{runQuery(container.Select(q.Sel), q)}
			}
			var want []any
			if q.All {
				_ = json.Unmarshal(results[i].Results[qi], &want)
			} else {
				var one any
				_ = json.Unmarshal(results[i].Results[qi], &one)
				want = []any{one}
			}
			if len(got) != len(want) {
				res.bad++
				res.fails = append(res.fails, fmt.Sprintf("%s %s(%s): %d values, want %d", c.Name, q.Op, q.Sel, len(got), len(want)))
				continue
			}
			for vi := range got {
				g, w := normalizeProbeValue(got[vi]), normalizeProbeValue(want[vi])
				if probeValuesEqual(g, w) {
					res.exact[q.Op]++
					continue
				}
				res.differ[q.Op]++
				if name := gap(c, q, g, w); name != "" {
					res.gaps[name]++
					continue
				}
				res.bad++
				if len(res.fails) < 20 {
					html := c.HTML
					if len(html) > 260 {
						html = html[:260] + "…"
					}
					res.fails = append(res.fails, fmt.Sprintf("%s %s(%s) value %d: got %v, want %v\n      %s", c.Name, q.Op, q.Sel, vi, g, w, html))
				}
			}
		}
	}
	return res, true
}

// normalizeProbeValue turns a value of a query into a number or a list of
// numbers, as JSON has it.
func normalizeProbeValue(v any) any {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out any
	_ = json.Unmarshal(data, &out)
	return out
}

func probeValuesEqual(a, b any) bool {
	al, aok := a.([]any)
	bl, bok := b.([]any)
	if aok != bok {
		return false
	}
	if !aok {
		return a == b
	}
	if len(al) != len(bl) {
		return false
	}
	for i := range al {
		if al[i] != bl[i] {
			return false
		}
	}
	return true
}

// edgesWithin reports whether each edge of one rectangle is at most tol
// away from the same edge of another rectangle.
func edgesWithin(a, b any, tol float64) bool {
	al, aok := a.([]any)
	bl, bok := b.([]any)
	if !aok || !bok || len(al) != 4 || len(bl) != 4 {
		return false
	}
	var av, bv [4]float64
	for i := range al {
		x, okx := al[i].(float64)
		y, oky := bl[i].(float64)
		if !okx || !oky {
			return false
		}
		av[i], bv[i] = x, y
	}
	near := func(x, y float64) bool { return x-y <= tol && y-x <= tol }
	return near(av[0], bv[0]) && near(av[1], bv[1]) && near(av[0]+av[2], bv[0]+bv[2]) && near(av[1]+av[3], bv[1]+bv[3])
}

// summary prints the counts of a replay.
func (r webFontReplay) summary() string {
	var parts []string
	for _, op := range []string{"textlen", "bbox", "rect"} {
		if r.exact[op]+r.differ[op] > 0 {
			parts = append(parts, fmt.Sprintf("%s exact=%d differ=%d", op, r.exact[op], r.differ[op]))
		}
	}
	return strings.Join(parts, "; ")
}

// TestWebFontProbesAgainstChromiumLinux replays the cases with the
// embedded fonts and the web fonts, and requires the numbers of Chromium
// on Linux with only the Liberation fonts installed.
//
// One gap is known. The glyphs of Open Sans have TrueType bytecode, and
// Chromium runs it (the hinting instance of skrifa). This module does not
// have the interpreter. The advances are exact without it. The ink of a
// glyph is not: its vertical extent can be one pixel off, and in a few
// cases two. The box of an SVG text has the ink, so the box can differ when
// the ink leaves the line box: accents on capital letters, and a text with
// a synthetic bold, whose stroke adds to the ink on every side and follows
// the hinted outline at its corners. The test accepts a box that is at
// most two pixels off on an edge when the font list has Open Sans. It
// limits the number of these boxes to the number that was measured, so the
// gap cannot grow.
func TestWebFontProbesAgainstChromiumLinux(t *testing.T) {
	// The probes ran in Chromium, not in the headless shell of mermaid-cli.
	lib := font.Embedded().WithWebFontsInChromium()
	const bytecode = "ink of Open Sans, which has bytecode"
	gap := func(c probeCase, q probeQuery, got, want any) string {
		if (q.Op == "bbox" || q.Op == "rect") && strings.Contains(strings.ToLower(c.HTML), "open sans") && edgesWithin(got, want, 2) {
			return bytecode
		}
		return ""
	}
	// The values of each set, and the largest number of values in the gap.
	for set, limit := range map[string]struct{ values, gap int }{
		"text":  {108336, 270},
		"label": {4500, 20},
	} {
		t.Run(set, func(t *testing.T) {
			res, ok := replayWebFontSet(t, set, "linux", lib, gap)
			if !ok {
				t.Skip("no data for this set")
			}
			t.Logf("%d cases: %s; known gap: %v", res.cases, res.summary(), res.gaps)
			total := 0
			for _, n := range res.exact {
				total += n
			}
			for _, n := range res.differ {
				total += n
			}
			if total != limit.values {
				t.Errorf("%d values, want %d", total, limit.values)
			}
			if res.gaps[bytecode] > limit.gap {
				t.Errorf("%d values in the known gap, at most %d before", res.gaps[bytecode], limit.gap)
			}
			if res.bad > 0 {
				t.Errorf("%d values differ from Chromium on Linux; the first:\n%s", res.bad, strings.Join(res.fails, "\n"))
			}
		})
	}
}

// TestWebFontProbesAgainstChromiumDarwin replays the cases with the fonts
// of this machine and the web fonts against Chromium on macOS. It runs on
// macOS only: the fallback fonts are the fonts of the reference machine.
//
// The rules of macOS for web fonts are not complete, so the test requires
// the counts that the library reaches and not every value:
//
//   - The length of a text and the box of an HTML label are exact, but for
//     strings whose characters go to a fallback font of the system that
//     this library chooses in another way than CoreText (dingbats, CJK, and
//     arrows at the weight 500). The same strings differ without web fonts.
//   - The box of an SVG text has the ink of the glyphs. CoreText makes the
//     outline of an instance of a variable font with its own arithmetic,
//     and the ink of a synthetic italic and of a synthetic bold follows
//     from it. Many boxes differ by less than 0.01 px.
func TestWebFontProbesAgainstChromiumDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the macOS reference needs the fonts of macOS")
	}
	// The probes ran in Chromium, not in the headless shell of mermaid-cli.
	lib := font.System().WithWebFontsInChromium()
	none := func(probeCase, probeQuery, any, any) string { return "not required" }
	for set, want := range map[string]map[string]int{
		// The least number of exact values by query kind.
		"text":  {"textlen": 34900, "rect": 34500, "bbox": 20000},
		"label": {"textlen": 1100, "rect": 1990, "bbox": 1150},
	} {
		t.Run(set, func(t *testing.T) {
			res, ok := replayWebFontSet(t, set, "darwin", lib, none)
			if !ok {
				t.Skip("no data for this set")
			}
			t.Logf("%d cases: %s", res.cases, res.summary())
			for op, least := range want {
				if res.exact[op] < least {
					t.Errorf("%s: %d exact values, at least %d before", op, res.exact[op], least)
				}
			}
		})
	}
}
