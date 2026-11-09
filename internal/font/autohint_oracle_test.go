package font

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/font/liberation"
)

// TestAutohintAgainstChromium compares the advance and the ink bounds of
// single glyphs with what the canvas of Chromium on Linux reports, for the
// characters that TestLinuxGlyphAgainstChromium does not have: all other
// characters that the twelve embedded faces share. They are more Latin,
// Greek and Cyrillic, Hebrew, combining marks, superscripts, subscripts and
// symbols, so they use the styles of the automatic hinter that the other
// test does not reach. Each face has three font sizes.
//
// The data is the set autohint of tools/reference/gen-font-linux-probes.mjs.
func TestAutohintAgainstChromium(t *testing.T) {
	var data struct {
		Chars []string         `json:"chars"`
		Cases []linuxGlyphCase `json:"cases"`
	}
	readGzipJSON(t, "linux-autohint.json.gz", &data)
	lib := Embedded()
	total, advanceOK, boundsOK := 0, 0, 0
	styles := map[string]int{}
	var fails []string
	for _, c := range data.Cases {
		face := lib.Match(strings.Trim(c.Family, `"`), cssWeight(c.Weight), c.Style == "italic")
		if face == nil {
			t.Fatalf("no face for %s", c.Family)
		}
		size := effectiveFontSize(c.Size)
		inst := face.autohinter()
		for i, ch := range data.Chars {
			want := c.Rows[i]
			glyphs := face.Shape([]rune(ch), size)
			if len(glyphs) != 1 {
				continue
			}
			total++
			g := glyphs[0]
			style, _ := inst.styles.style(g.ID)
			name := style.styleClass().name
			if style.isNonBase() {
				name += ", not a base character"
			}
			styles[name]++
			okAdvance := g.Advance == want[0]
			if okAdvance {
				advanceOK++
			}
			// The canvas reports the ink: the left and the top edge with
			// the sign changed. A glyph without ink has zeros.
			left, top, right, bottom, empty, _ := face.ScalerBounds(g.ID, size)
			got := [4]float64{float64(-left), float64(right), float64(-top), float64(bottom)}
			if empty {
				got = [4]float64{}
			}
			okBounds := got == [4]float64{want[1], want[2], want[3], want[4]}
			if okBounds {
				boundsOK++
			}
			if !(okAdvance && okBounds) && len(fails) < 40 {
				fails = append(fails, fmt.Sprintf("%s %s %s %vpx %q U+%04X (glyph %d, %s): advance go=%v chrome=%v | ink [left right ascent descent] go=%v chrome=%v",
					c.Family, c.Weight, c.Style, c.Size, ch, []rune(ch)[0], g.ID, name, g.Advance, want[0], got, want[1:]))
			}
		}
	}
	t.Logf("%d glyph measurements: advance exact=%d, ink bounds exact=%d", total, advanceOK, boundsOK)
	t.Logf("glyph styles of the measurements: %v", styles)
	if advanceOK != total || boundsOK != total {
		t.Errorf("differences from Chromium; the first:\n%s", strings.Join(fails, "\n"))
	}
}

// autohintPixelBox rounds a box out to whole pixels, as pixelBounds does. The
// result has the form of the ink bounds of the canvas: left, right, ascent,
// descent.
func autohintPixelBox(box ftCBox) [4]float64 {
	if !box.ok {
		return [4]float64{}
	}
	return [4]float64{-float64(box.xMin >> 6), float64((box.xMax + 63) >> 6), float64((box.yMax + 63) >> 6), -float64(box.yMin >> 6)}
}

// TestAutohintImageAgainstChromium compares the ink bounds of single glyphs
// of fonts of the Playwright image with Chromium in that image: the FreeFont
// families, IPAGothic and WenQuanYi Zen Hei. These fonts have scripts that
// the Liberation fonts do not have: Arabic, Armenian, Cherokee, Ethiopic,
// Georgian, Thai, the scripts of India, Chinese, Japanese and others. So
// the test reaches the styles of the automatic hinter that the embedded
// fonts cannot reach: the alignment zones of CJK, the Indic group and the
// hinting from the top to the bottom.
//
// The fontconfig files of the image set the hint style none for WenQuanYi
// Zen Hei, and for IPAGothic up to 18px. Chromium does not hint those
// glyphs, so the test compares them with unhintedCBox.
//
// A text is compared when it shapes to one glyph that has the advance of
// Chromium. The other texts differ in shaping, which is not the subject of
// this test.
//
// The data is the set autohint of tools/reference/gen-font-linux-probes.mjs
// --image. The test is skipped when the fonts are absent.
func TestAutohintImageAgainstChromium(t *testing.T) {
	if _, err := os.Stat(imageFontDir); err != nil {
		t.Skip("no fonts of the Playwright image:", err)
	}
	var cases []struct {
		linuxGlyphCase
		Chars []string `json:"chars"`
	}
	readGzipJSON(t, "linux-image-autohint.json.gz", &cases)
	lib := NewLibraryFor(PlatformLinux, imageFontDir)
	total, skipped, hintedTotal, hintedOK, unhintedTotal, unhintedOK := 0, 0, 0, 0, 0, 0
	styles := map[string]int{}
	var fails []string
	for _, c := range cases {
		family := strings.Trim(c.Family, `"`)
		face := lib.Match(family, cssWeight(c.Weight), c.Style == "italic")
		if face == nil || face.Family != family {
			t.Fatalf("no face for %s", c.Family)
		}
		size := effectiveFontSize(c.Size)
		hinted := !(family == "WenQuanYi Zen Hei" || (family == "IPAGothic" && size <= 18))
		inst := face.autohinter()
		if inst == nil {
			t.Fatalf("%s: no TrueType outlines", c.Family)
		}
		for i, ch := range c.Chars {
			want := c.Rows[i]
			total++
			glyphs := face.Shape([]rune(ch), size)
			if len(glyphs) != 1 || glyphs[0].Advance != want[0] {
				skipped++
				continue
			}
			g := glyphs[0]
			style, _ := inst.styles.style(g.ID)
			name := style.styleClass().name
			var got [4]float64
			if hinted {
				hintedTotal++
				styles[name]++
				left, top, right, bottom, empty, _ := face.ScalerBounds(g.ID, size)
				if !empty {
					got = [4]float64{float64(-left), float64(right), float64(-top), float64(bottom)}
				}
			} else {
				unhintedTotal++
				box, _ := face.unhintedCBox(g.ID, size)
				got = autohintPixelBox(box)
			}
			if got == [4]float64{want[1], want[2], want[3], want[4]} {
				if hinted {
					hintedOK++
				} else {
					unhintedOK++
				}
			} else if len(fails) < 40 {
				fails = append(fails, fmt.Sprintf("%s %vpx %q U+%04X (glyph %d, %s, hinted %v): ink [left right ascent descent] go=%v chrome=%v",
					c.Family, c.Size, ch, []rune(ch)[0], g.ID, name, hinted, got, want[1:]))
			}
		}
	}
	t.Logf("%d glyph measurements, %d not compared: with hinting %d, ink bounds exact=%d; without hinting %d, ink bounds exact=%d",
		total, skipped, hintedTotal, hintedOK, unhintedTotal, unhintedOK)
	t.Logf("glyph styles of the measurements with hinting: %v", styles)
	if hintedOK != hintedTotal || unhintedOK != unhintedTotal {
		t.Errorf("differences from Chromium; the first:\n%s", strings.Join(fails, "\n"))
	}
}

// TestAutohintUnhintedAgainstChromium compares the bounds of unhintedCBox
// with the ink bounds that Chromium reports for a font size above 256px.
// Skia measures such a glyph at 64px without hinting and scales the bounds
// (see ScalerBounds). The data is the data of TestLinuxGlyphAgainstChromium.
func TestAutohintUnhintedAgainstChromium(t *testing.T) {
	var data struct {
		Chars []string         `json:"chars"`
		Cases []linuxGlyphCase `json:"cases"`
	}
	readGzipJSON(t, "linux-glyph.json.gz", &data)
	lib := Embedded()
	total, boundsOK := 0, 0
	var fails []string
	for _, c := range data.Cases {
		size := effectiveFontSize(c.Size)
		if float32(size) <= canonicalLimit {
			continue
		}
		face := lib.Match(strings.Trim(c.Family, `"`), cssWeight(c.Weight), c.Style == "italic")
		if face == nil {
			t.Fatalf("no face for %s", c.Family)
		}
		for i, ch := range data.Chars {
			want := c.Rows[i]
			glyphs := face.Shape([]rune(ch), size)
			if len(glyphs) != 1 {
				continue
			}
			total++
			g := glyphs[0]
			box, ok := face.unhintedCBox(g.ID, canonicalSize)
			if !ok {
				t.Fatalf("%s: no TrueType outlines", c.Family)
			}
			var got [4]float64
			if box.ok {
				// The box is rounded out to whole pixels, and each edge is
				// multiplied by the scale in float32, as in ScalerBounds.
				s := float32(size) / canonicalSize
				left, right := float32(box.xMin>>6), float32((box.xMax+63)>>6)
				top, bottom := -float32((box.yMax+63)>>6), -float32(box.yMin>>6)
				got = [4]float64{float64(-float32(left * s)), float64(float32(right * s)), float64(-float32(top * s)), float64(float32(bottom * s))}
			}
			if got == [4]float64{want[1], want[2], want[3], want[4]} {
				boundsOK++
			} else if len(fails) < 40 {
				fails = append(fails, fmt.Sprintf("%s %s %s %vpx %q (glyph %d): ink [left right ascent descent] go=%v chrome=%v",
					c.Family, c.Weight, c.Style, c.Size, ch, g.ID, got, want[1:]))
			}
		}
	}
	t.Logf("%d glyph measurements above %dpx: ink bounds exact=%d", total, canonicalLimit, boundsOK)
	if boundsOK != total {
		t.Errorf("differences from Chromium; the first:\n%s", strings.Join(fails, "\n"))
	}
}

// TestAutohintConcurrent uses the automatic hinter of one face from many
// goroutines at the same time. Each goroutine must get the bounds that a
// face of its own gives.
func TestAutohintConcurrent(t *testing.T) {
	data, err := liberation.Font("LiberationSerif-Italic.ttf")
	if err != nil {
		t.Fatal(err)
	}
	load := func() *Face {
		faces, err := LoadFor(PlatformLinux, data)
		if err != nil {
			t.Fatal(err)
		}
		return faces[0]
	}
	sizes := []float64{9, 13.333, 16, 31}
	const glyphs = 400
	want := map[ahBoxKey]ftCBox{}
	single := load()
	for gid := uint32(0); gid < glyphs; gid++ {
		for i, size := range sizes {
			box, ok := single.hintedCBox(gid, size)
			if !ok {
				t.Fatal("no TrueType outlines")
			}
			want[ahBoxKey{gid: gid, size: uint32(i)}] = box
		}
	}
	shared := load()
	var wg sync.WaitGroup
	errs := make(chan string, 16)
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for n := 0; n < glyphs; n++ {
				// Each goroutine walks the glyphs from another start, so
				// that the goroutines ask for one glyph at the same time and
				// for different glyphs also.
				gid := uint32((n + 25*worker) % glyphs)
				for i, size := range sizes {
					box, _ := shared.hintedCBox(gid, size)
					if box != want[ahBoxKey{gid: gid, size: uint32(i)}] {
						select {
						case errs <- fmt.Sprintf("glyph %d at %vpx: %v, want %v", gid, size, box, want[ahBoxKey{gid: gid, size: uint32(i)}]):
						default:
						}
						return
					}
				}
			}
		}(worker)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}
