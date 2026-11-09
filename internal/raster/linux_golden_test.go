package raster

// Golden tests against Chromium on Linux. Each SVG in testdata/linux has a
// screenshot at scale 1 (NAME.png), and some have one at scale 2
// (NAME@2x.png), made in a container that has only the Liberation fonts:
//
//	node tools/reference/gen-raster-linux-golden.mjs --shoot
//
// The tests render the SVG with the embedded fonts, which are the same
// fonts, and with the rules of Chromium on Linux. They need no font of the
// system, so they run on every machine.
//
// The files are the probes of tools/reference/gen-raster-linux-probes.mjs,
// one for each rule of the text of Chromium on Linux that this package
// ports, and corpus diagrams with the same cases:
//
//   - baseline-svg-*: where the baseline of SVG text lands for each
//     dominant-baseline, with and without a tspan (svgTextOrigin).
//   - baseline-html: where the lines of HTML labels land (paintHTMLBox).
//   - lcd-opacity*: LCD masks in opacity layers, and what a filter or a
//     blend mode elsewhere in the document does to them (opacityLayer,
//     scanEffects).
//   - lcd-clip: LCD masks under clip paths and viewports (applyClipPath).
//   - lcd-filter: text in filter layers.
//   - rotate*: hinted outlines under quarter turns and flips, outlines
//     without hinting under other matrices (glyphCache.build).
//   - large: text above 48 px without LCD masks (tooBigForLCD), and text
//     above 256 px as paths (drawGlyphRunAsPaths).
//   - *@2x: no hinting and other font metrics at a device scale factor
//     above 1 (borrowsAscent).

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// linuxGoldenLimits is the allowed share of mismatching pixels per file, in
// percent: a value just above the measured one. A file without an entry
// gets linuxGoldenDefault. Most files have no pixel that differs by more
// than 16 of 255.
var linuxGoldenLimits = map[string]float64{
	// Text under a rotation that is not a quarter turn: a few glyphs are
	// a quarter of a pixel away from their place in Chromium.
	"rotate":        0.035,
	"rotate-labels": 0.075,
	"corpus-git":    0.15,
	// The edges of the outlines of text above 256 px.
	"large": 0.025,
	// The underline does not skip the parentheses that cross it
	// (text-decoration-skip-ink).
	"corpus-class": 0.04,
	// The stroke of a circle at scale 2.
	"corpus-sequence@2x": 0.025,
}

const linuxGoldenDefault = 0.005

func TestGoldenLinux(t *testing.T) {
	files, err := filepath.Glob("testdata/linux/*.svg")
	if err != nil || len(files) == 0 {
		t.Fatalf("no golden files: %v", err)
	}
	sort.Strings(files)
	out := os.Getenv("RASTER_GOLDEN_OUT")
	fonts := font.Embedded()
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".svg")
		svg, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, scale := range []float64{1, 2} {
			suffix := ""
			if scale == 2 {
				suffix = "@2x"
			}
			shot := filepath.Join("testdata/linux", name+suffix+".png")
			if _, err := os.Stat(shot); err != nil {
				if scale == 1 {
					t.Errorf("%s: no screenshot", name)
				}
				continue
			}
			t.Run(name+suffix, func(t *testing.T) {
				want, err := loadNRGBA(shot)
				if err != nil {
					t.Fatal(err)
				}
				got, err := Render(string(svg), Options{Scale: scale, Fonts: fonts})
				if err != nil {
					t.Fatal(err)
				}
				if got.Bounds() != want.Bounds() {
					t.Fatalf("size %v, want %v", got.Bounds().Size(), want.Bounds().Size())
				}
				if out != "" {
					writeComparison(filepath.Join(out, "linux-"+name+suffix+".cmp.png"), got, want)
				}
				mismatch, mad := pixelDiff(got, want)
				limit, ok := linuxGoldenLimits[name+suffix]
				if !ok {
					limit = linuxGoldenDefault
				}
				msg := fmt.Sprintf("mismatch %.4f%% (limit %.3f%%), mean difference %.3f", 100*mismatch, limit, mad)
				if 100*mismatch > limit {
					t.Error(msg)
				} else {
					t.Log(msg)
				}
			})
		}
	}
}
