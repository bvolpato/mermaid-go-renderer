package mermaid

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// TestFirstRender reports the cost of the first render of a process: the
// render that loads the fonts, the grammars and the tables. Run it alone, so
// that no other test renders before it:
//
//	go test -run TestFirstRender -count=1 -v .
//
// The first line is the first render of the process. The lines after it are
// the first renders of other diagram types, which pay only for what the
// first one did not load. Each line has the second render of the same
// source next to it, for comparison.
func TestFirstRender(t *testing.T) { firstRender(t) }

// TestFirstRenderSystemFonts is TestFirstRender with the fonts of this
// machine. Run it alone too.
func TestFirstRenderSystemFonts(t *testing.T) { firstRender(t, WithSystemFonts()) }

func firstRender(t *testing.T, opts ...Option) {
	measure := func(src string) (wall, cpu time.Duration, alloc uint64) {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		cpu0 := processCPUTime()
		start := time.Now()
		if _, err := Render(src, opts...); err != nil {
			t.Fatal(err)
		}
		wall = time.Since(start)
		cpu = processCPUTime() - cpu0
		runtime.ReadMemStats(&after)
		return wall, cpu, after.TotalAlloc - before.TotalAlloc
	}
	for _, rel := range []string{"pie/nightly-check/default-theme.mmd", "sequence", "flowchart", "class-diagram", "gantt"} {
		name, src := firstFixture(t, rel)
		family := strings.SplitN(name, string(filepath.Separator), 2)[0]
		w1, c1, a1 := measure(src)
		w2, c2, a2 := measure(src)
		t.Logf("%-14s first: %8.2f ms wall, %8.2f ms cpu, %7d KB   second: %6.2f ms wall, %6.2f ms cpu, %6d KB",
			family, ms(w1), ms(c1), a1/1024, ms(w2), ms(c2), a2/1024)
	}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// TestFontsAreParsedOnce checks that a font library parses a font one time:
// renders and PNG output after the first ones parse no font, with the
// embedded fonts and with the fonts of this machine.
func TestFontsAreParsedOnce(t *testing.T) {
	_, src := firstFixture(t, "sequence")
	for _, tc := range []struct {
		name string
		lib  *font.Library
		opts []Option
	}{
		{"embedded", font.Embedded(), nil},
		{"system", font.System(), []Option{WithSystemFonts()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svg, err := Render(src, tc.opts...)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := SVGToPNG(svg, tc.opts...); err != nil {
				t.Fatal(err)
			}
			parsed := tc.lib.ParsedFonts()
			if parsed == 0 {
				t.Fatal("the first render parsed no font")
			}
			for i := 0; i < 3; i++ {
				if _, err := RenderPNG(src, tc.opts...); err != nil {
					t.Fatal(err)
				}
			}
			if got := tc.lib.ParsedFonts(); got != parsed {
				t.Errorf("later renders parsed %d more fonts", got-parsed)
			}
			t.Logf("%d fonts parsed", parsed)
		})
	}
}

// TestSimpleParserShare reports how many of the documents that the sanitizer
// parsed went to the simple parser of internal/dom, in a build with the tag
// domcheck. In that build every document of the simple parser is compared
// with the document of the full parser, so this run proves the simple parser
// on the corpus:
//
//	go test -tags domcheck -run 'TestCorpusParity|TestSimpleParserShare' -v .
func TestSimpleParserShare(t *testing.T) {
	simple, full := dom.SimpleParserCounts()
	if simple+full == 0 {
		t.Skip("no counts: the build has no tag domcheck, or no test rendered before this one")
	}
	t.Logf("%d documents: the simple parser read %d (%.1f%%), the full parser %d",
		simple+full, simple, 100*float64(simple)/float64(simple+full), full)
}
