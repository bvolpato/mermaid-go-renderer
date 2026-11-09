package font

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/css"
)

// The tests in this file compare the embedded library with Chromium on
// Linux that has only the Liberation fonts. They do not read the fonts of
// the machine, so they give the same result on every platform.
//
// The data comes from tools/reference/gen-font-linux-probes.mjs, and
// linux-text.json.gz from .parity/probe/text.js:
//
//	cd tools/reference && node oracle-linux.mjs --fonts liberation eval \
//	  ../../.parity/probe/text.js --args ../../.parity/probe/text-cases.json --out out.json

// readGzipJSON reads a gzip-compressed JSON file of testdata.
func readGzipJSON(t *testing.T, name string, v any) {
	t.Helper()
	file, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	zr, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(zr).Decode(v); err != nil {
		t.Fatal(err)
	}
}

// cssWeight is the computed font-weight of a probe case. bolder and lighter
// are relative to the weight 400 of the parent.
func cssWeight(s string) int {
	switch s {
	case "", "normal":
		return 400
	case "bold", "bolder":
		return 700
	case "lighter":
		return 100
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return 400
}

// exactWidth is the width of a text as Blink has it: the advances added in
// fixed point, and the sum snapped up to 1/64 px. An advance is a multiple
// of 1/65536 px, so the float64 sum is exact.
func exactWidth(runs []Run) float64 {
	var sum float64
	for _, run := range runs {
		for _, g := range run.Glyphs {
			sum += g.Advance
		}
	}
	return SnapCeil(sum)
}

// linuxFingerprint names the embedded faces that give the measured width
// of a case when one face shapes the whole text. It helps to read a
// failure.
func linuxFingerprint(c chromeTextCase) string {
	var names []string
	for _, fam := range []string{"Liberation Sans", "Liberation Serif", "Liberation Mono"} {
		for _, weight := range []int{400, 700} {
			for _, italic := range []bool{false, true} {
				f := Embedded().Match(fam, weight, italic)
				if f == nil {
					continue
				}
				g := f.Shape([]rune(c.Text), c.Size)
				if exactWidth([]Run{{Glyphs: g}}) == c.HTMLW {
					name := strings.TrimPrefix(fam, "Liberation ")
					if weight == 700 {
						name += " Bold"
					}
					if italic {
						name += " Italic"
					}
					names = append(names, name)
				}
			}
		}
	}
	if len(names) == 0 {
		return "no face"
	}
	return strings.Join(names, " or ")
}

// checkLinuxText compares the width, the line height and the logical box of
// the cases of a file with Chromium.
func checkLinuxText(t *testing.T, file string) {
	t.Helper()
	exact, total, fails := measureLinuxText(t, file, Embedded())
	t.Logf("%s: %d cases, %d exact", file, total, exact)
	if exact != total {
		t.Errorf("%d cases differ from Chromium; the first:\n%s", total-exact, strings.Join(fails, "\n"))
	}
}

// measureLinuxText compares the cases of a file with a library. It returns
// the number of exact cases, the number of cases and a description of the
// first differences.
func measureLinuxText(t *testing.T, file string, lib *Library) (exact, total int, fails []string) {
	t.Helper()
	var cases []chromeTextCase
	readGzipJSON(t, file, &cases)
	type stat struct{ total, ok int }
	stats := map[string]*stat{}
	bad := 0
	for _, c := range cases {
		// An oblique style with a negative angle gives an upright face.
		italic := strings.HasPrefix(c.Style, "italic") || (strings.HasPrefix(c.Style, "oblique") && !strings.Contains(c.Style, "-"))
		if strings.Contains(c.Text, "\t") {
			// The layout replaces a tab before it shapes the text. The
			// test of the browser package has these cases.
			continue
		}
		chain := lib.Resolve(css.SplitFontFamily(c.Family), cssWeight(c.Weight), italic)
		runs := chain.ShapeText([]rune(c.Text), c.Size)
		w := exactWidth(runs)
		var m Metrics
		if p := chain.Primary(); p != nil {
			m = p.Metrics(c.Size)
		}
		st := stats[c.Family]
		if st == nil {
			st = &stat{}
			stats[c.Family] = st
		}
		st.total++
		// The box of the text has the ink of the glyphs too, so it can be
		// larger than the logical box. The test of the browser package
		// compares the box itself.
		// A line with glyphs of a fallback font has the metrics of that
		// font too. The layout of the browser package computes that.
		fallback := false
		for _, run := range runs {
			fallback = fallback || run.Face != chain.Primary()
		}
		ok := w == c.HTMLW && w == c.SVGLen && (m.LineSpacing == c.HTMLH || fallback)
		if w > 0 && !fallback {
			// A text without a width has only the box of its ink.
			ok = ok && -m.Ascent >= c.BY && m.Descent <= c.BY+c.BH
		}
		if ok {
			st.ok++
			continue
		}
		bad++
		if len(fails) < 60 {
			face := "none"
			if p := chain.Primary(); p != nil {
				face = p.Family
			}
			fails = append(fails, fmt.Sprintf("%q %s %vpx %s %s: width go=%v html=%v svg=%v | lineH go=%v html=%v | box go=(%v,%v) chrome=(%v,%v) | go face %s, chrome %s",
				c.Text, c.Family, c.Size, c.Weight, c.Style, w, c.HTMLW, c.SVGLen, m.LineSpacing, c.HTMLH, -m.Ascent, m.Ascent+m.Descent, c.BY, c.BH, face, linuxFingerprint(c)))
		}
	}
	if testing.Verbose() {
		keys := make([]string, 0, len(stats))
		for k := range stats {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if s := stats[k]; s.ok != s.total {
				t.Logf("%-48s n=%4d exact=%4d", k, s.total, s.ok)
			}
		}
	}
	return len(cases) - bad, len(cases), fails
}

// TestLinuxTextAgainstChromium checks words of the corpus and stress
// strings in the font families that diagrams use.
func TestLinuxTextAgainstChromium(t *testing.T) {
	checkLinuxText(t, "linux-text.json.gz")
}

// TestLinuxFamilyAgainstChromium checks which font a font-family value
// gives: a family that is installed, a family with a metric-compatible
// replacement, a family that is not installed, the generic families and
// lists.
func TestLinuxFamilyAgainstChromium(t *testing.T) {
	checkLinuxText(t, "linux-family.json.gz")
}

// TestLinuxWeightAgainstChromium checks which face of a family a
// font-weight and a font-style give.
func TestLinuxWeightAgainstChromium(t *testing.T) {
	checkLinuxText(t, "linux-weight.json.gz")
}

// TestLinuxMissingAgainstChromium checks characters that no Liberation font
// has, and characters that only some of the Liberation fonts have.
func TestLinuxMissingAgainstChromium(t *testing.T) {
	checkLinuxText(t, "linux-missing.json.gz")
}

// imageFontDir is the directory with the fonts of the Playwright image that
// the oracle runs (its /usr/share/fonts). The files are not in the
// repository.
const imageFontDir = "../../.parity/linux/image/fonts"

// imageFontconfigDir is a copy of /etc/fonts of that image.
const imageFontconfigDir = "../../.parity/linux/image/etc-fonts"

// TestLinuxImageAgainstChromium compares a Linux library that has the fonts
// and the fontconfig configuration of the Playwright image with Chromium
// in that image: Liberation, WenQuanYi Zen Hei, IPAGothic, Noto Color
// Emoji, FreeFont and others. The data comes from
// tools/reference/gen-font-linux-probes.mjs --image. The test is skipped
// when the fonts are absent.
//
// The library follows fontconfig for the families and for the font of a
// character that the families do not have. It does not agree with Chromium
// in every case yet, so the test requires the counts that it reaches:
//
//   - family: the generic family emoji as the only family (Chromium takes
//     the letters from another font than this library).
//   - missing: Arabic text (the shaping is left to right here), a few
//     characters whose font Chromium takes from a Type 1 font file that the
//     library does not read, and advances of fallback fonts where the
//     hinter of Chromium moves the advance by one pixel.
func TestLinuxImageAgainstChromium(t *testing.T) {
	if _, err := os.Stat(imageFontDir); err != nil {
		t.Skip("no fonts of the Playwright image:", err)
	}
	lib := NewLinuxLibrary(imageFontconfigDir, imageFontDir)
	for _, set := range []struct {
		file string
		want int // the least number of exact cases
	}{
		{"linux-image-family.json.gz", 496},
		{"linux-image-weight.json.gz", 1176},
		{"linux-image-missing.json.gz", 1020},
	} {
		t.Run(set.file, func(t *testing.T) {
			exact, total, fails := measureLinuxText(t, set.file, lib)
			t.Logf("%s: %d cases, %d exact", set.file, total, exact)
			if exact < set.want {
				t.Errorf("%d exact cases, want at least %d; the first differences:\n%s", exact, set.want, strings.Join(fails, "\n"))
			}
		})
	}
}
