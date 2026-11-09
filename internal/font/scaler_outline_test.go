package font

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"

	ot "github.com/go-text/typesetting/font/opentype"

	"github.com/bvolpato/mermaid-go-renderer/internal/font/liberation"
)

// outlineBounds returns the bounds of the points of an outline, with the
// control points, as SkPath::getBounds does. ok is false for an outline
// without segments.
func outlineBounds(segments []ot.Segment) (minX, minY, maxX, maxY float32, ok bool) {
	for i := range segments {
		for _, p := range segments[i].ArgsSlice() {
			if !ok {
				minX, minY, maxX, maxY, ok = p.X, p.Y, p.X, p.Y, true
				continue
			}
			minX, maxX = min(minX, p.X), max(maxX, p.X)
			minY, maxY = min(minY, p.Y), max(maxY, p.Y)
		}
	}
	return minX, minY, maxX, maxY, ok
}

// outlineInk returns the bounds of an outline with y up as the canvas of
// Chromium reports the ink of a glyph: left with the sign changed, right,
// ascent and descent, rounded out to whole pixels. An outline without
// segments has zeros.
func outlineInk(segments []ot.Segment) [4]float64 {
	minX, minY, maxX, maxY, ok := outlineBounds(segments)
	if !ok {
		return [4]float64{}
	}
	return [4]float64{
		-math.Floor(float64(minX)), math.Ceil(float64(maxX)),
		math.Ceil(float64(maxY)), -math.Floor(float64(minY)),
	}
}

// TestScalerOutlineAgainstChromium compares the bounds of the outline that
// ScalerOutline returns with the ink bounds that Chromium on Linux reports,
// and with ScalerBounds. The data is the data of
// TestLinuxGlyphAgainstChromium: every embedded face, at font sizes from 7px
// to 400px.
//
// Up to 256px the outline is the hinted outline, and its bounds, rounded out
// to whole pixels, are the ink bounds. Above 256px Skia measures the glyph
// at 64px without hinting and scales the rounded bounds, so the test rounds
// the bounds of the outline at 64px, and it checks that ScalerOutline
// returns that outline times size/64.
func TestScalerOutlineAgainstChromium(t *testing.T) {
	var data struct {
		Chars []string         `json:"chars"`
		Cases []linuxGlyphCase `json:"cases"`
	}
	readGzipJSON(t, "linux-glyph.json.gz", &data)
	lib := Embedded()
	total, hinted, scaled, chromiumOK, boundsOK, shapeOK := 0, 0, 0, 0, 0, 0
	var fails []string
	fail := func(c linuxGlyphCase, ch string, gid uint32, format string, args ...any) {
		if len(fails) < 40 {
			fails = append(fails, fmt.Sprintf("%s %s %s %vpx %q (glyph %d): %s",
				c.Family, c.Weight, c.Style, c.Size, ch, gid, fmt.Sprintf(format, args...)))
		}
	}
	for _, c := range data.Cases {
		face := lib.Match(strings.Trim(c.Family, `"`), cssWeight(c.Weight), c.Style == "italic")
		if face == nil {
			t.Fatalf("no face for %s", c.Family)
		}
		size := effectiveFontSize(c.Size)
		for i, ch := range data.Chars {
			want := [4]float64{c.Rows[i][1], c.Rows[i][2], c.Rows[i][3], c.Rows[i][4]}
			glyphs := face.Shape([]rune(ch), size)
			if len(glyphs) != 1 {
				continue
			}
			total++
			gid := glyphs[0].ID
			outline, ok := face.ScalerOutline(gid, size)
			if !ok {
				t.Fatalf("%s: no outline of the scaler", c.Family)
			}
			segments := outline.Segments
			// The path of Skia starts each contour with a move.
			if len(segments) == 0 || segments[0].Op == ot.SegmentOpMoveTo {
				shapeOK++
			} else {
				fail(c, ch, gid, "the outline does not start with a move")
			}
			left, top, right, bottom, empty, _ := face.ScalerBounds(gid, size)
			bounds := [4]float64{float64(-left), float64(right), float64(-top), float64(bottom)}
			if empty {
				bounds = [4]float64{}
			}
			var got [4]float64
			if float32(size) > canonicalLimit {
				scaled++
				// The outline at 64px without hinting.
				unhinted, ok := face.ScalerOutlineUnhinted(gid, canonicalSize)
				if !ok {
					t.Fatalf("%s: no outline of the scaler without hinting", c.Family)
				}
				canonical := unhinted.Segments
				s := float32(size) / canonicalSize
				got = outlineInk(canonical)
				for k := range got {
					got[k] = float64(float32(float32(got[k]) * s))
				}
				same := len(canonical) == len(segments)
				for k := 0; same && k < len(canonical); k++ {
					a, b := canonical[k], segments[k]
					same = a.Op == b.Op
					for j := range a.Args {
						same = same && float32(a.Args[j].X*s) == b.Args[j].X && float32(a.Args[j].Y*s) == b.Args[j].Y
					}
				}
				if !same {
					fail(c, ch, gid, "the outline is not the outline at %vpx times %v", canonicalSize, s)
					continue
				}
			} else {
				hinted++
				got = outlineInk(segments)
			}
			if got == want {
				chromiumOK++
			} else {
				fail(c, ch, gid, "ink [left right ascent descent] outline=%v chrome=%v", got, want)
			}
			if got == bounds {
				boundsOK++
			} else {
				fail(c, ch, gid, "ink [left right ascent descent] outline=%v ScalerBounds=%v", got, bounds)
			}
		}
	}
	t.Logf("%d glyph outlines (%d hinted, %d above %dpx): bounds equal to Chromium=%d, equal to ScalerBounds=%d",
		total, hinted, scaled, canonicalLimit, chromiumOK, boundsOK)
	if chromiumOK != total || boundsOK != total || shapeOK != total {
		t.Errorf("differences; the first:\n%s", strings.Join(fails, "\n"))
	}
}

// pathText writes the segments of a pen as the text of an SVG path, with
// one digit after the decimal point.
func pathText(segments []ot.Segment) string {
	var b strings.Builder
	for i := range segments {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString([]string{"M", "L", "Q", "C"}[segments[i].Op])
		for j, p := range segments[i].ArgsSlice() {
			if j > 0 {
				b.WriteByte(' ')
			}
			fmt.Fprintf(&b, "%.1f,%.1f", p.X, p.Y)
		}
	}
	return b.String()
}

// TestContourToPath checks the conversion of a contour to a path with the
// test cases of skrifa (src/outline/path.rs: the cases with the path style
// of FreeType), and the rules of the pen of Skia.
func TestContourToPath(t *testing.T) {
	const on, off = ahPointOnCurve, uint8(0)
	type pt = ahPathPoint
	tests := []struct {
		name   string
		points []pt
		want   string
	}{
		{
			// all_off_curve_to_path_scan_backward: the path starts in the
			// middle of the last and the first point.
			name:   "all points off the curve",
			points: []pt{{640, 128, off}, {256, 64, off}, {640, 64, off}, {128, 128, off}},
			want:   "M6.0,2.0 Q10.0,2.0 7.0,1.5 Q4.0,1.0 7.0,1.0 Q10.0,1.0 6.0,1.5 Q2.0,2.0 6.0,2.0",
		},
		{
			// start_off_curve_to_path_scan_backward.
			name:   "the first point off the curve",
			points: []pt{{640, 128, off}, {256, 64, on}, {640, 64, off}, {128, 128, off}},
			want:   "M6.0,2.0 Q10.0,2.0 4.0,1.0 Q10.0,1.0 6.0,1.5 Q2.0,2.0 6.0,2.0",
		},
		{
			// The path starts at the last point, and the loop does not have
			// that point a second time.
			name:   "the first point off the curve, the last point on the curve",
			points: []pt{{640, 128, off}, {256, 64, on}, {128, 128, on}},
			want:   "M2.0,2.0 Q10.0,2.0 4.0,1.0",
		},
		{
			name:   "lines",
			points: []pt{{0, 0, on}, {64, 0, on}, {64, 64, on}},
			want:   "M0.0,0.0 L1.0,0.0 L1.0,1.0",
		},
		{
			// The pen of Skia drops a line to the current point.
			name:   "a point two times",
			points: []pt{{0, 0, on}, {64, 0, on}, {64, 0, on}, {64, 64, on}},
			want:   "M0.0,0.0 L1.0,0.0 L1.0,1.0",
		},
		{
			// The pen of Skia starts a contour at the first segment that
			// moves.
			name:   "all points at one position",
			points: []pt{{64, 64, on}, {64, 64, off}, {64, 64, on}},
			want:   "",
		},
		{
			name:   "a cubic curve",
			points: []pt{{0, 0, on}, {64, 0, ahPointOffCurveCubic}, {64, 64, ahPointOffCurveCubic}, {0, 64, on}},
			want:   "M0.0,0.0 C1.0,0.0 1.0,1.0 0.0,1.0",
		},
	}
	for _, test := range tests {
		var pen ahPathPen
		if !ahContourToPath(len(test.points), func(i int) ahPathPoint { return test.points[i] }, &pen) {
			t.Errorf("%s: the contour is not valid", test.name)
			continue
		}
		if got := pathText(pen.segments); got != test.want {
			t.Errorf("%s:\n got %s\nwant %s", test.name, got, test.want)
		}
	}
	// One off-curve point of a cubic curve is not a valid contour.
	bad := []pt{{0, 0, on}, {64, 0, ahPointOffCurveCubic}, {0, 64, on}}
	var pen ahPathPen
	if ahContourToPath(len(bad), func(i int) ahPathPoint { return bad[i] }, &pen) {
		t.Errorf("a contour with one off-curve point of a cubic curve is valid")
	}
}

// TestScalerOutlineOtherFaces checks the result of ScalerOutline for a face
// that has no outline of the scaler: a face with the rules of macOS.
func TestScalerOutlineOtherFaces(t *testing.T) {
	data, err := liberation.Font("LiberationSans-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	faces, err := LoadFor(PlatformDarwin, data)
	if err != nil || len(faces) == 0 {
		t.Fatalf("no face: %v", err)
	}
	gid, _ := faces[0].NominalGlyph('A')
	if _, ok := faces[0].ScalerOutline(gid, 16); ok {
		t.Errorf("a face of macOS has an outline of the scaler")
	}
	if _, ok := faces[0].ScalerOutline(gid, 300); ok {
		t.Errorf("a face of macOS has an outline of the scaler above 256px")
	}
	if _, ok := faces[0].ScalerOutlineUnhinted(gid, 16); ok {
		t.Errorf("a face of macOS has an outline of the scaler without hinting")
	}
}

// TestScalerOutlineConcurrent gets the outlines of the same glyphs from
// many goroutines and compares them with the outlines of one goroutine. Run
// it with -race.
func TestScalerOutlineConcurrent(t *testing.T) {
	// A face of its own, so that the first use of the hinter is in the
	// goroutines.
	data, err := liberation.Font("LiberationSerif-Italic.ttf")
	if err != nil {
		t.Fatal(err)
	}
	load := func() *Face {
		faces, err := LoadFor(PlatformLinux, data)
		if err != nil || len(faces) == 0 {
			t.Fatalf("no face: %v", err)
		}
		return faces[0]
	}
	sizes := []float64{9, 12.5, 16, 33, 300}
	text := []rune("Hamburgefonstiv 0123456789 éÄ")
	outlines := func(face *Face) string {
		var b strings.Builder
		for _, size := range sizes {
			for _, r := range text {
				gid, _ := face.NominalGlyph(r)
				hinted, _ := face.ScalerOutline(gid, size)
				unhinted, _ := face.ScalerOutlineUnhinted(gid, size)
				fmt.Fprintf(&b, "%v %d: %v | %v\n", size, gid, hinted.Segments, unhinted.Segments)
			}
		}
		return b.String()
	}
	want := outlines(load())
	shared := load()
	const workers = 8
	got := make([]string, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[w] = outlines(shared)
		}()
	}
	wg.Wait()
	for w := range got {
		if got[w] != want {
			t.Fatalf("goroutine %d: the outlines differ from the outlines of one goroutine", w)
		}
	}
}
