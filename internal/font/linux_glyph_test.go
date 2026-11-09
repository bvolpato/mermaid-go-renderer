package font

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

type linuxGlyphCase struct {
	Family      string       `json:"family"`
	Weight      string       `json:"weight"`
	Style       string       `json:"style"`
	Size        float64      `json:"size"`
	FontAscent  float64      `json:"fontAscent"`
	FontDescent float64      `json:"fontDescent"`
	Rows        [][5]float64 `json:"rows"`
}

// effectiveFontSize is FontDescription::EffectiveFontSize of Blink: the
// font size cut to 1/100 px in float32 (see textStyle.fontSize in
// internal/browser).
func effectiveFontSize(size float64) float64 {
	scaled := float32(size) * 100
	return float64(float32(math.Floor(float64(scaled))) / 100)
}

// TestLinuxGlyphAgainstChromium compares the advance and the ink bounds of
// single glyphs with what the canvas of Chromium on Linux reports
// (measureText): every embedded face, at font sizes from 7px to 400px. The
// bounds are the control box of the outline that FreeType scaled, rounded
// out to whole pixels.
func TestLinuxGlyphAgainstChromium(t *testing.T) {
	var data struct {
		Chars []string         `json:"chars"`
		Cases []linuxGlyphCase `json:"cases"`
	}
	readGzipJSON(t, "linux-glyph.json.gz", &data)
	lib := Embedded()
	total, advanceOK, boundsOK := 0, 0, 0
	var fails []string
	for _, c := range data.Cases {
		face := lib.Match(strings.Trim(c.Family, `"`), cssWeight(c.Weight), c.Style == "italic")
		if face == nil {
			t.Fatalf("no face for %s", c.Family)
		}
		size := effectiveFontSize(c.Size)
		for i, ch := range data.Chars {
			want := c.Rows[i]
			glyphs := face.Shape([]rune(ch), size)
			if len(glyphs) != 1 {
				continue
			}
			total++
			g := glyphs[0]
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
				fails = append(fails, fmt.Sprintf("%s %s %s %vpx %q (glyph %d): advance go=%v chrome=%v | ink [left right ascent descent] go=%v chrome=%v",
					c.Family, c.Weight, c.Style, c.Size, ch, g.ID, g.Advance, want[0], got, want[1:]))
			}
		}
	}
	t.Logf("%d glyph measurements: advance exact=%d, ink bounds exact=%d", total, advanceOK, boundsOK)
	if advanceOK != total || boundsOK != total {
		t.Errorf("differences from Chromium; the first:\n%s", strings.Join(fails, "\n"))
	}
}
