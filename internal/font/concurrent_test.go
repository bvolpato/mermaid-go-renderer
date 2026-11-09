package font

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/css"
)

type concurrentCase struct {
	text     string
	families string
	size     float64
	weight   int
	italic   bool
}

// concurrentCases lists texts in the font families of mermaid: plain words,
// pairs with kerning, accents, characters that need a fallback font, emoji
// and characters that no font has.
func concurrentCases() []concurrentCase {
	texts := []string{
		"Start", "Is it working?", "AVATAR To Te Ty", "office affluent fi fl", "Égalité Ångström ñ ç",
		"jumping (query) [x] {y}", "100% $9.99 #tag", "日本語 中文测试", "한국어", "a😀b 🎉", "→ ← ✓ ★",
		" ­​", " \U0010ffff", "Привет мир", "αβγ Ωμέγα", "x", " ", "",
	}
	families := []string{
		`"trebuchet ms", verdana, arial, sans-serif`, "arial", `"times new roman"`, `"courier new"`,
		"monospace", "serif", "sans-serif", "system-ui", `"no such font"`, `"Recursive Variable", arial, sans-serif`,
	}
	var cases []concurrentCase
	for _, text := range texts {
		for i, fam := range families {
			size := []float64{16, 14, 12, 17.5, 13.333, 24, 300}[(i+len(text))%7]
			cases = append(cases,
				concurrentCase{text, fam, size, 400, false},
				concurrentCase{text, fam, size, 700, i%2 == 0},
			)
		}
	}
	return cases
}

// measureCase shapes and measures one case and returns everything that the
// layout engine reads from the font package as a string.
func measureCase(lib *Library, c concurrentCase) string {
	var sb strings.Builder
	chain := lib.Resolve(css.SplitFontFamily(c.families), c.weight, c.italic)
	primary := chain.Primary()
	if primary == nil {
		return "no font"
	}
	m := primary.Metrics(c.size)
	fmt.Fprintf(&sb, "%s/%d/%v metrics=%v xh=%v|", primary.Family, primary.Weight, primary.Italic, m, primary.HasXHeight())
	runs := chain.ShapeText([]rune(c.text), c.size)
	fmt.Fprintf(&sb, "w=%v|", TextWidth(runs))
	for _, run := range runs {
		fmt.Fprintf(&sb, "run %s/%d/%v@%d:", run.Face.Family, run.Face.Weight, run.Face.Italic, run.Start)
		for _, g := range run.Glyphs {
			fmt.Fprintf(&sb, " %d/%d/%v/%v/%v", g.ID, g.Cluster, g.Advance, g.XOffset, g.YOffset)
			if l, t, r, b, empty, handled := run.Face.ScalerBounds(g.ID, c.size); handled {
				fmt.Fprintf(&sb, "[%v %v %v %v %v]", l, t, r, b, empty)
			} else {
				minX, minY, maxX, maxY, ok := run.Face.OutlineBounds(g.ID)
				fmt.Fprintf(&sb, "[%v %v %v %v %v]", minX, minY, maxX, maxY, ok)
			}
			if l, t, r, b, ok := run.Face.ColorGlyphBounds(g.ID, c.size, false); ok {
				fmt.Fprintf(&sb, "{%v %v %v %v}", l, t, r, b)
			}
		}
		sb.WriteByte('|')
	}
	for _, r := range c.text {
		f := chain.FaceFor(r)
		fmt.Fprintf(&sb, "%s:%v ", f.Family, f.HasGlyph(r))
	}
	return sb.String()
}

// TestConcurrentMeasure shapes and measures the same texts from many
// goroutines and requires the results of a serial run. The goroutines
// start with a library that has loaded nothing, so that they also load the
// fonts and fill the caches at the same time. Run it with -race.
func TestConcurrentMeasure(t *testing.T) {
	cases := concurrentCases()
	if testing.Short() {
		// The race detector makes the loading of fonts slow.
		short := cases[:0:0]
		for i := 0; i < len(cases); i += 4 {
			short = append(short, cases[i])
		}
		cases = short
	}
	libraries := []struct {
		name  string
		fresh func() *Library
	}{
		{"embedded", func() *Library { return newLibrary(PlatformLinux, embedFonts, nil) }},
		{"system", func() *Library {
			platform := DefaultPlatform()
			lib := newLibrary(platform, defaultEmbedMode(platform), systemFontDirs())
			lib.fcSystemConfig = platform == PlatformLinux
			return lib
		}},
	}
	const workers = 16
	for _, entry := range libraries {
		t.Run(entry.name, func(t *testing.T) {
			serialLib := entry.fresh()
			want := make([]string, len(cases))
			for i, c := range cases {
				want[i] = measureCase(serialLib, c)
			}
			lib := entry.fresh()
			got := make([][]string, workers)
			var wg sync.WaitGroup
			for w := 0; w < workers; w++ {
				wg.Add(1)
				go func(w int) {
					defer wg.Done()
					out := make([]string, len(cases))
					// Each worker starts at another case, so that the
					// workers reach a font at different times.
					for k := range cases {
						i := (k + w*len(cases)/workers) % len(cases)
						out[i] = measureCase(lib, cases[i])
					}
					got[w] = out
				}(w)
			}
			wg.Wait()
			bad := 0
			for w := range got {
				for i := range cases {
					if got[w][i] != want[i] {
						bad++
						if bad <= 3 {
							t.Errorf("worker %d, case %+v\n got  %s\n want %s", w, cases[i], got[w][i], want[i])
						}
					}
				}
			}
			if bad > 3 {
				t.Errorf("%d results differ from the serial run", bad)
			}
			t.Logf("%d cases, %d goroutines", len(cases), workers)
		})
	}
}
