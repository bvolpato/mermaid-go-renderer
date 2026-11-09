package raster

// Golden tests against Chromium. Each SVG in testdata/golden has a
// screenshot at scale 1 (NAME.png) and at scale 2 (NAME@2x.png), made with
// tools/reference/oracle.mjs shoot. The tests render the SVG and compare
// the pixels. The comparison is not exact: goldenLimits holds the largest
// share of pixels that may differ by more than 16 of 255 in a channel.

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
)

// goldenLimits is the allowed share of mismatching pixels per file, in
// percent. A file without an entry gets defaultGoldenLimit: shapes, strokes,
// dashes, gradients, markers, layers, filters and the root size match
// Chromium, apart from single pixels on edges. Text has the residual error
// of the glyph model.
var goldenLimits = map[string]float64{
	"clip-nested":   0.3,
	"foreignobject": 0.6,
	"text-sizes":    0.8,
	"transform":     0.3,
	// Stroked text: Chromium draws the stroke as a glyph mask with the
	// coverage table of the text color. This package strokes the outline.
	"text-basic": 4.5,
	// Color emoji: the bitmap strikes are scaled with another filter.
	"emoji": 12,
}

const defaultGoldenLimit = 0.05

// pixelDiff compares two images of the same size. It returns the share of
// pixels where a channel differs by more than 16, and the mean absolute
// difference per channel.
func pixelDiff(got *image.RGBA, want *image.NRGBA) (mismatch, mad float64) {
	b := got.Bounds()
	var bad, sum int
	for y := b.Min.Y; y < b.Max.Y; y++ {
		g := got.Pix[got.PixOffset(b.Min.X, y):]
		w := want.Pix[want.PixOffset(b.Min.X, y):]
		for x := 0; x < b.Dx(); x++ {
			over := false
			for c := 0; c < 4; c++ {
				d := int(g[x*4+c]) - int(w[x*4+c])
				if d < 0 {
					d = -d
				}
				sum += d
				if d > 16 {
					over = true
				}
			}
			if over {
				bad++
			}
		}
	}
	n := b.Dx() * b.Dy()
	if n == 0 {
		return 0, 0
	}
	return float64(bad) / float64(n), float64(sum) / float64(4*n)
}

func TestGolden(t *testing.T) {
	files, err := filepath.Glob("testdata/golden/*.svg")
	if err != nil || len(files) == 0 {
		t.Fatalf("no golden files: %v", err)
	}
	sort.Strings(files)
	out := os.Getenv("RASTER_GOLDEN_OUT")
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".svg")
		svg, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		// A screenshot of a file with text has the glyphs of the system
		// fonts of the macOS reference machine. A machine with other fonts
		// renders such a file and checks its size, but not its pixels.
		hasText := bytes.Contains(svg, []byte("<text")) || bytes.Contains(svg, []byte("<foreignObject"))
		for _, scale := range []float64{1, 2} {
			suffix := ""
			if scale == 2 {
				suffix = "@2x"
			}
			t.Run(name+suffix, func(t *testing.T) {
				want, err := loadNRGBA(filepath.Join("testdata/golden", name+suffix+".png"))
				if err != nil {
					t.Fatal(err)
				}
				got, err := Render(string(svg), Options{Scale: scale})
				if err != nil {
					t.Fatal(err)
				}
				if got.Bounds() != want.Bounds() {
					t.Fatalf("size %v, want %v", got.Bounds().Size(), want.Bounds().Size())
				}
				if out != "" {
					writeComparison(filepath.Join(out, name+suffix+".cmp.png"), got, want)
				}
				if hasText {
					fonttest.RequireMacOSReference(t)
				}
				mismatch, mad := pixelDiff(got, want)
				limit, ok := goldenLimits[name]
				if !ok {
					limit = defaultGoldenLimit
				}
				msg := fmt.Sprintf("mismatch %.3f%% (limit %.2f%%), mean difference %.3f", 100*mismatch, limit, mad)
				if 100*mismatch > limit {
					t.Error(msg)
				} else {
					t.Log(msg)
				}
			})
		}
	}
}

// writeComparison writes the reference, the render and their difference
// side by side, for a person to look at.
func writeComparison(path string, got *image.RGBA, want *image.NRGBA) {
	b := got.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewNRGBA(image.Rect(0, 0, 3*w+8, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			g := got.Pix[got.PixOffset(x, y):]
			r := want.Pix[want.PixOffset(x, y):]
			copy(out.Pix[out.PixOffset(x, y):], r[:4])
			copy(out.Pix[out.PixOffset(x+w+4, y):], g[:4])
			worst := 0
			for c := 0; c < 4; c++ {
				d := int(g[c]) - int(r[c])
				if d < 0 {
					d = -d
				}
				worst = max(worst, d)
			}
			o := out.Pix[out.PixOffset(x+2*w+8, y):]
			switch {
			case worst > 16:
				o[0], o[1], o[2], o[3] = 255, 0, 0, 255
			case worst > 0:
				v := uint8(255 - 8*worst)
				o[0], o[1], o[2], o[3] = v, v, 255, 255
			default:
				o[0], o[1], o[2], o[3] = 255, 255, 255, 255
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	png.Encode(f, out)
}
