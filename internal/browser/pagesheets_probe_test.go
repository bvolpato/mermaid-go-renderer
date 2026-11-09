package browser

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/webfonts"
)

// The tests in this file replay the probe cases of
// tools/reference/gen-browser-pagesheets-probe.mjs: the labels of mermaid
// with the icons of Font Awesome, in a page that has the style sheets and
// the web fonts of mermaid-cli. The expected values are what Chromium 156
// returned, on macOS and on Linux with only the Liberation fonts. The
// comparison is exact.

// pageQuery is a query of the page sheets probe. Fonts is true when the
// value depends on a face of the web fonts.
type pageQuery struct {
	Sel   string          `json:"sel"`
	Op    string          `json:"op"`
	Arg   json.RawMessage `json:"arg"`
	All   bool            `json:"all"`
	Fonts bool            `json:"fonts"`
}

type pageCase struct {
	Name    string      `json:"name"`
	HTML    string      `json:"html"`
	Queries []pageQuery `json:"queries"`
}

// pageSheetsOfMermaidCLI returns the text of the style sheets of the page
// of mermaid-cli, in the order of the page.
func pageSheetsOfMermaidCLI(t testing.TB) []string {
	t.Helper()
	var sheets []string
	for _, sheet := range webfonts.Sheets {
		data, err := webfonts.FS.ReadFile(sheet.Path)
		if err != nil {
			t.Fatal(err)
		}
		sheets = append(sheets, string(data))
	}
	return sheets
}

func runPageQuery(e *Engine, el *dom.Node, q pageQuery) any {
	if el == nil {
		return nil
	}
	if q.Op == "pstyle" {
		var arg [2]string
		_ = json.Unmarshal(q.Arg, &arg)
		return e.ComputedPseudoStyle(el, arg[0], arg[1])
	}
	return runQuery(el, probeQuery{Sel: q.Sel, Op: q.Op, Arg: q.Arg, All: q.All})
}

// The reasons why a value that differs from Chromium does not fail the
// test. The first three are parts of CSS that the engine does not have, in
// classes of Font Awesome that are not icons. The labels of mermaid do not
// have these classes: an author of a diagram can write them in the HTML of
// a label.
const (
	// gapFloat: fa-pull-left and fa-pull-right float the icon.
	gapFloat = "float"
	// gapPosition: fa-li, fa-stack-1x and fa-stack-2x have position:
	// absolute. The engine takes such a box out of the flow, so the label
	// has the size of Chromium. It does not place the box: the rectangle of
	// the icon itself and its used width are not the values of Chromium.
	gapPosition = "position: absolute"
	// gapTransform: fa-rotate-90 and the like turn the box of the icon.
	// The label does not change. The rectangle of the icon itself does.
	gapTransform = "transform of an HTML box"
	// gapInfinite: a width of calc(10px / 0) is the largest length. The
	// label has the size of Chromium. The box itself is 4 px narrower in
	// Chromium, which adds with saturation.
	gapInfinite = "division by zero in a math function"
)

// pageSheetsGapLimits bounds the number of values of each known gap, so
// that a gap cannot grow without a failure of the test. The limits are the
// numbers of the cases that have the reason.
var pageSheetsGapLimits = map[string]int{gapFloat: 20, gapPosition: 24, gapTransform: 4, gapInfinite: 1}

// pageSheetsKnownGap returns the reason why a value of a case that differs
// from Chromium does not fail the test, or "".
func pageSheetsKnownGap(html string, q pageQuery) string {
	switch {
	case strings.Contains(html, "fa-pull-"):
		return gapFloat
	case strings.Contains(html, "fa-li") || strings.Contains(html, "fa-stack-1x") || strings.Contains(html, "fa-stack-2x"):
		return gapPosition
	case (strings.Contains(html, "fa-rotate-") || strings.Contains(html, "fa-flip-")) && q.Op == "rect" && strings.HasSuffix(q.Sel, " i"):
		return gapTransform
	case strings.Contains(html, "calc(10px / 0)") && q.Op == "rect" && q.Sel == ".b":
		return gapInfinite
	}
	return ""
}

// replayPageSheets replays the cases with a font library and compares with
// the results of a file. webFonts is true when the library has the faces of
// the web fonts: the values that need them are then compared too.
func replayPageSheets(t *testing.T, resultsFile string, lib *font.Library, webFonts bool) {
	t.Helper()
	var cases struct{ Cases []pageCase }
	var results []probeResult
	if !readLinuxJSON(t, "pagesheets-cases.json.gz", &cases) || !readLinuxJSON(t, resultsFile, &results) {
		t.Fatal("no probe data")
	}
	if len(cases.Cases) == 0 || len(cases.Cases) != len(results) {
		t.Fatalf("%d cases, %d results", len(cases.Cases), len(results))
	}
	sheets := pageSheetsOfMermaidCLI(t)
	filter := os.Getenv("MMDG_PROBE_FILTER")
	type stat struct{ exact, waiting, gap, bad int }
	stats := map[string]*stat{}
	gaps := map[string]int{}
	var fails []string
	for i, c := range cases.Cases {
		if filter != "" && !strings.Contains(c.Name, filter) {
			continue
		}
		kind := c.Name[:strings.LastIndexByte(c.Name, '-')]
		st := stats[kind]
		if st == nil {
			st = &stat{}
			stats[kind] = st
		}
		doc := NewDocumentWithFonts(lib)
		doc.PageSheets = sheets
		engine := doc.Engine.(*Engine)
		container := doc.Body.Append("div").Attr("id", "container")
		container.HTML(c.HTML)
		for qi, q := range c.Queries {
			if q.Fonts && !webFonts {
				st.waiting++
				continue
			}
			var got any
			if q.All {
				vals := []any{}
				for _, el := range container.SelectAll(q.Sel) {
					vals = append(vals, runPageQuery(engine, el, q))
				}
				got = vals
			} else {
				got = runPageQuery(engine, container.Select(q.Sel), q)
			}
			want := results[i].Results[qi]
			if compare(got, want, 0) == "exact" {
				st.exact++
				continue
			}
			if reason := pageSheetsKnownGap(c.HTML, q); reason != "" {
				st.gap++
				gaps[reason]++
				if os.Getenv("MMDG_PROBE_VERBOSE") != "" {
					gotJSON, _ := json.Marshal(got)
					t.Logf("known gap (%s): %s %s(%s %s) got %s want %s", reason, c.Name, q.Op, q.Sel, q.Arg, gotJSON, want)
				}
				continue
			}
			st.bad++
			if len(fails) < 40 {
				gotJSON, _ := json.Marshal(got)
				var compact any
				_ = json.Unmarshal(want, &compact)
				wantJSON, _ := json.Marshal(compact)
				html := c.HTML
				if j := strings.Index(html, "<foreignObject"); j >= 0 {
					html = html[j:]
				}
				if css := strings.Index(c.HTML, "--mermaid-font-family"); kind == "pseudo" && css >= 0 {
					rules := c.HTML[css:strings.Index(c.HTML, "</style>")]
					html = rules[strings.IndexByte(rules, '}')+1:] + " " + html
				}
				fails = append(fails, fmt.Sprintf("%s %s(%s %s)\n      got  %s\n      want %s\n      %s", c.Name, q.Op, q.Sel, q.Arg, gotJSON, wantJSON, html))
			}
		}
	}
	kinds := make([]string, 0, len(stats))
	for k := range stats {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	var total stat
	for _, k := range kinds {
		s := stats[k]
		t.Logf("%-14s exact=%5d waits for the web fonts=%4d known gap=%3d differs=%3d", k, s.exact, s.waiting, s.gap, s.bad)
		total.exact, total.waiting, total.gap, total.bad = total.exact+s.exact, total.waiting+s.waiting, total.gap+s.gap, total.bad+s.bad
	}
	t.Logf("%d cases: exact=%d, waits for the web fonts=%d, known gaps=%d %v, differs=%d", len(cases.Cases), total.exact, total.waiting, total.gap, gaps, total.bad)
	if total.bad > 0 {
		t.Errorf("%d values differ from Chromium:\n%s", total.bad, strings.Join(fails, "\n"))
	}
	if filter == "" {
		for reason, n := range gaps {
			if limit := pageSheetsGapLimits[reason]; n > limit {
				t.Errorf("known gap %q: %d values, more than the %d that are known", reason, n, limit)
			}
		}
	}
}

// TestPageSheetsAgainstChromium replays the cases against Chromium on
// macOS, with the fonts of the system.
func TestPageSheetsAgainstChromium(t *testing.T) {
	macOSProbe(t)
	lib := font.System()
	// The probe ran in the Chromium of oracle.mjs. The page of mermaid-cli
	// runs in the headless shell, which has other generic families on
	// macOS (font.Library.WithWebFontsInChromium).
	replayPageSheets(t, "pagesheets-results.json.gz", lib.WithWebFontsInChromium(), os.Getenv("MMDG_PAGESHEETS_NO_WEBFONTS") == "")
}

// TestLinuxPageSheetsAgainstChromium replays the cases against Chromium on
// Linux with only the Liberation fonts. It uses the embedded fonts, so it
// gives the same result on every platform.
func TestLinuxPageSheetsAgainstChromium(t *testing.T) {
	lib := font.Embedded()
	replayPageSheets(t, "linux-pagesheets-results.json.gz", lib.WithWebFonts(), os.Getenv("MMDG_PAGESHEETS_NO_WEBFONTS") == "")
}

// pageSheetsLabel is the markup of a flowchart label with an icon between
// two words, with no space.
const pageSheetsLabel = `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg"><style>#my-svg{font-family:arial,sans-serif;font-size:14px;}#my-svg p{margin:0;}</style><g><foreignObject id="fo" width="300" height="100"><div xmlns="http://www.w3.org/1999/xhtml" id="d" style="display: table-cell; white-space: nowrap; line-height: 1.5; max-width: 200px; text-align: center;"><span id="s" class="nodeLabel"><p id="p">A<i id="i" class="fa fa-car"></i>car</p></span></div></foreignObject></g></svg>`

// TestPageSheetsOnlyInThePage checks that a document without page sheets
// has no icon box: the mode has no effect on a plain page. It also checks
// the box of the icon that the sheets give, which needs no web font: the
// width is 1.25em and the height is the line height 1.
func TestPageSheetsOnlyInThePage(t *testing.T) {
	lib := font.Embedded()
	measure := func(sheets []string) (icon, label dom.Rect, display string) {
		doc := NewDocumentWithFonts(lib)
		doc.PageSheets = sheets
		container := doc.Body.Append("div").Attr("id", "container")
		container.HTML(pageSheetsLabel)
		i := container.Select("#i")
		return i.BoundingClientRect(), container.Select("#d").BoundingClientRect(), i.ComputedStyle("display")
	}
	plainIcon, plainLabel, plainDisplay := measure(nil)
	pageIcon, pageLabel, pageDisplay := measure(pageSheetsOfMermaidCLI(t))
	if plainDisplay != "inline" || plainIcon.Width != 0 {
		t.Errorf("plain page: the icon has display %q and the width %v, want inline and 0", plainDisplay, plainIcon.Width)
	}
	if pageDisplay != "inline-block" || pageIcon.Width != 17.5 || pageIcon.Height != 14 {
		t.Errorf("page of mermaid-cli: the icon has display %q and the size %v x %v, want inline-block and 17.5 x 14", pageDisplay, pageIcon.Width, pageIcon.Height)
	}
	if got := pageLabel.Width - plainLabel.Width; got != 17.5 {
		t.Errorf("the icon makes the label %v wider, want 17.5", got)
	}
	if pageLabel.Height != plainLabel.Height {
		t.Errorf("the icon changes the height of the label from %v to %v", plainLabel.Height, pageLabel.Height)
	}
}

// TestPageRuleSetIsShared checks that documents with the same page sheets
// share one rule set, also when the sheets come from another slice, and
// that engines of several goroutines can use it at the same time. Run it
// with the race detector.
func TestPageRuleSetIsShared(t *testing.T) {
	sheets := pageSheetsOfMermaidCLI(t)
	first := pageRuleSetOf(sheets)
	if first == nil || len(first.pseudo) == 0 || len(first.byClass["fa-car"]) == 0 {
		t.Fatal("the rule set of the page sheets has no rule for an icon")
	}
	if again := pageRuleSetOf(append([]string(nil), sheets...)); again != first {
		t.Error("the same sheets have two rule sets")
	}
	if len(first.universal) != 0 {
		t.Errorf("%d rules of the page sheets are tried for every element", len(first.universal))
	}
	lib := font.Embedded()
	want := ""
	results := make(chan string, 8)
	for g := 0; g < cap(results); g++ {
		go func() {
			doc := NewDocumentWithFonts(lib)
			doc.PageSheets = sheets
			container := doc.Body.Append("div").Attr("id", "container")
			container.HTML(pageSheetsLabel)
			engine := doc.Engine.(*Engine)
			i := container.Select("#i")
			results <- fmt.Sprint(i.BoundingClientRect(), container.Select("#d").BoundingClientRect(),
				i.ComputedStyle("font-family"), engine.ComputedPseudoStyle(i, "::before", "content"))
		}()
	}
	for g := 0; g < cap(results); g++ {
		got := <-results
		if want == "" {
			want = got
		}
		if got != want {
			t.Errorf("goroutines differ:\n %s\n %s", got, want)
		}
	}
	if !strings.Contains(want, `"Font Awesome 7 Free"`) || !strings.Contains(want, "") {
		t.Errorf("the icon has not the style of the page sheets: %s", want)
	}
}
