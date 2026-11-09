package raster

// The calibration of the glyph model (glyph.go, glyph_zones.go) against
// Chromium.
//
// A sheet is an SVG with text at known positions. The oracle shoots it:
//
//	RASTER_WRITE_SHEETS=testdata/glyph go test -run TestWriteGlyphSheets ./internal/raster
//	node tools/reference/oracle.mjs shoot --out internal/raster/testdata/glyph \
//	    --base internal/raster/testdata/glyph internal/raster/testdata/glyph/*.svg
//
// There are two sets:
//
//   - The calibration set: single glyphs. cal-shapes has flat, round and
//     diagonal glyphs at sizes without vertical fitting. The parameters of
//     the growth and of the curve flattening come from it (TestFitGrowth).
//     cal-zones-* has the glyphs E z o l p - H x d at every size from 8 to
//     22 for one family. The table of capital heights comes from it
//     (TestFitZones).
//   - The validation set: val-words-* has lines of text, in other colors,
//     with glyphs and glyph pairs that the calibration set does not have.
//     TestGlyphModel measures the model on it and checks the residual
//     error.

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
)

// calItem is one text element of a sheet.
type calItem struct {
	Family string
	Weight int
	Size   float64
	Text   string
	// X and Y are the origin of the text: the start of the baseline.
	X, Y float64
	Fill string
	// Cell is the region of the sheet that belongs to the item.
	Cell image.Rectangle
}

// calSheet is a calibration or validation sheet.
type calSheet struct {
	Name  string
	W, H  int
	Bg    string
	Items []calItem
}

type calFont struct {
	Family string
	Weight int
}

// gridSheet lays out one row per font and size, and one cell per text and
// subpixel phase.
func gridSheet(name, bg, fill string, fonts []calFont, sizes []float64, texts []string, phases []float64) *calSheet {
	s := &calSheet{Name: name, Bg: bg}
	y := 4.0
	for _, f := range fonts {
		for _, size := range sizes {
			y += math.Ceil(size*1.15) + 3
			x := 6.0
			top := int(y - math.Ceil(size*1.1) - 1)
			bottom := int(y + math.Ceil(size*0.35) + 2)
			for _, text := range texts {
				n := float64(len([]rune(text)))
				cw := math.Ceil(size*1.05*n) + 8
				for _, ph := range phases {
					s.Items = append(s.Items, calItem{
						Family: f.Family, Weight: f.Weight, Size: size, Text: text,
						X: x + ph, Y: y, Fill: fill,
						Cell: image.Rect(int(x)-4, top, int(x+cw)-4, bottom),
					})
					x += cw
				}
			}
			if int(x)+4 > s.W {
				s.W = int(x) + 4
			}
			y += math.Ceil(size*0.35) + 3
		}
	}
	s.H = int(y) + 4
	return s
}

// wordSheet lays out lines of text, one cell per line.
func wordSheet(name, bg, fill string, fonts []calFont, sizes []float64, text string) *calSheet {
	s := &calSheet{Name: name, Bg: bg}
	y := 4.0
	for _, f := range fonts {
		for _, size := range sizes {
			y += math.Ceil(size*1.15) + 3
			w := math.Ceil(size*0.66*float64(len([]rune(text)))) + 12
			s.Items = append(s.Items, calItem{
				Family: f.Family, Weight: f.Weight, Size: size, Text: text,
				X: 6, Y: y, Fill: fill,
				Cell: image.Rect(2, int(y-math.Ceil(size*1.1)-1), int(w), int(y+math.Ceil(size*0.35)+2)),
			})
			if int(w)+4 > s.W {
				s.W = int(w) + 4
			}
			y += math.Ceil(size*0.35) + 3
		}
	}
	s.H = int(y) + 4
	return s
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// svg returns the SVG of the sheet with the items that keep accepts.
func (s *calSheet) svg(keep func(*calItem) bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, s.W, s.H, s.W, s.H)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="%s"/>`, s.W, s.H, s.Bg)
	for i := range s.Items {
		it := &s.Items[i]
		if keep != nil && !keep(it) {
			continue
		}
		fmt.Fprintf(&b, `<text x="%g" y="%g" font-family="%s" font-size="%gpx" font-weight="%d" fill="%s">%s</text>`,
			it.X, it.Y, xmlEscape(it.Family), it.Size, it.Weight, it.Fill, xmlEscape(it.Text))
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// modelFamilies are the font families of the themes of mermaid.
var modelFamilies = []string{"Arial", "Trebuchet MS", "Helvetica", "Courier", "Times"}

func familyFile(family string) string {
	return strings.ReplaceAll(strings.ToLower(family), " ", "")
}

// shapeSheet is the calibration sheet for the growth and the flattening:
// flat, round and diagonal glyphs at sizes above the vertical fitting.
func shapeSheet() *calSheet {
	texts := []string{"I", "l", "-", "_", ".", "H", "o", "e", "s", "8", "v", "x", "A", "/"}
	return gridSheet("cal-shapes", "#ffffff", "#000000", []calFont{{"Arial", 400}}, []float64{20, 26, 34, 44, 58, 76}, texts, []float64{0})
}

// zoneSheet is the calibration sheet of the vertical fitting of a family.
func zoneSheet(family string) *calSheet {
	sizes := []float64{8, 9, 9.5, 10, 10.5, 11, 11.5, 12, 12.5, 13, 13.5, 14, 14.5, 15, 15.5, 16, 16.5, 17, 17.5, 18, 18.5, 19, 19.5, 20, 20.5, 21, 22}
	texts := []string{"E", "z", "o", "l", "p", "-", "H", "x", "d"}
	return gridSheet("cal-zones-"+familyFile(family), "#ffffff", "#000000", []calFont{{family, 400}}, sizes, texts, []float64{0})
}

// validationSheets are lines of text in the fonts, sizes and colors that
// the diagrams use most. The text is not in the calibration set.
func validationSheets() []*calSheet {
	var regular []calFont
	for _, f := range modelFamilies {
		regular = append(regular, calFont{f, 400})
	}
	bold := []calFont{{"Arial", 700}, {"Trebuchet MS", 700}}
	small := []float64{10, 11, 12, 13, 14, 15, 16, 17, 18}
	large := []float64{20, 24, 28, 32, 40}
	const text = "Jackdaws love my big sphinx of quartz, 0123456789"
	return []*calSheet{
		wordSheet("val-words-small", "#ffffff", "#333333", regular, small, text),
		wordSheet("val-words-large", "#ffffff", "#333333", regular[:2], large, text),
		wordSheet("val-words-bold", "#ececff", "#000000", bold, []float64{12, 14, 16, 18, 24}, text),
		wordSheet("val-words-dark", "#1f2020", "#cccccc", regular[:2], []float64{12, 14, 16, 20, 28}, text),
	}
}

func allSheets() []*calSheet {
	sheets := []*calSheet{shapeSheet()}
	for _, f := range modelFamilies {
		sheets = append(sheets, zoneSheet(f))
	}
	return append(sheets, validationSheets()...)
}

const glyphDataDir = "testdata/glyph"

// TestWriteGlyphSheets writes the SVG of every sheet into the directory
// RASTER_WRITE_SHEETS, for the oracle to shoot.
func TestWriteGlyphSheets(t *testing.T) {
	dir := os.Getenv("RASTER_WRITE_SHEETS")
	if dir == "" {
		t.Skip("RASTER_WRITE_SHEETS is not set")
	}
	for _, s := range allSheets() {
		if err := os.WriteFile(filepath.Join(dir, s.Name+".svg"), []byte(s.svg(nil)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func loadNRGBA(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	return out, nil
}

func loadSheet(t testing.TB, s *calSheet) *image.NRGBA {
	t.Helper()
	ref, err := loadNRGBA(filepath.Join(glyphDataDir, s.Name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	if ref.Bounds().Dx() != s.W || ref.Bounds().Dy() != s.H {
		t.Fatalf("%s: the screenshot is %v, the sheet %dx%d: shoot the sheets again", s.Name, ref.Bounds().Size(), s.W, s.H)
	}
	return ref
}

// cellError is the difference between a render and the reference over
// cells.
type cellError struct {
	n      int     // pixels
	sq     float64 // sum of squared channel differences
	signed float64 // sum of differences, render minus reference (green)
	ink    float64 // sum of the reference ink (distance to the background)
	over   int     // pixels with a channel difference above 16
	edge   int     // pixels where the reference is neither background nor full ink
}

func (e *cellError) add(o cellError) {
	e.n += o.n
	e.sq += o.sq
	e.signed += o.signed
	e.ink += o.ink
	e.over += o.over
	e.edge += o.edge
}

// rms is the root mean square difference per channel, over all pixels of
// the cells.
func (e cellError) rms() float64 {
	if e.n == 0 {
		return 0
	}
	return math.Sqrt(e.sq / float64(3*e.n))
}

// bias is the relative ink error: the summed signed difference over the
// summed reference ink.
func (e cellError) bias() float64 {
	if e.ink == 0 {
		return 0
	}
	return e.signed / e.ink
}

// overShare is the share of the edge pixels of the text that differ by
// more than 16 of 255, in percent.
func (e cellError) overShare() float64 {
	return 100 * float64(e.over) / float64(max(e.edge, 1))
}

func compareCell(got *image.RGBA, want *image.NRGBA, cell image.Rectangle, bg [3]uint8) cellError {
	var e cellError
	cell = cell.Intersect(got.Bounds()).Intersect(want.Bounds())
	for y := cell.Min.Y; y < cell.Max.Y; y++ {
		for x := cell.Min.X; x < cell.Max.X; x++ {
			g := got.Pix[got.PixOffset(x, y):]
			w := want.Pix[want.PixOffset(x, y):]
			over := false
			for c := 0; c < 3; c++ {
				d := float64(g[c]) - float64(w[c])
				e.sq += float64(d * d)
				if d > 16 || d < -16 {
					over = true
				}
			}
			e.signed += float64(g[1]) - float64(w[1])
			ink := math.Abs(float64(w[1]) - float64(bg[1]))
			e.ink += ink
			if ink > 8 && ink < 200 {
				e.edge++
			}
			if over {
				e.over++
			}
			e.n++
		}
	}
	return e
}

// sheetError renders the items of a sheet that keep accepts and compares
// their cells with the reference.
func sheetError(t testing.TB, s *calSheet, ref *image.NRGBA, keep func(*calItem) bool) cellError {
	resetGlyphCache()
	img, err := Render(s.svg(keep), Options{})
	if err != nil {
		t.Fatal(err)
	}
	bg := [3]uint8{ref.Pix[0], ref.Pix[1], ref.Pix[2]}
	var total cellError
	for i := range s.Items {
		it := &s.Items[i]
		if keep != nil && !keep(it) {
			continue
		}
		total.add(compareCell(img, ref, it.Cell, bg))
	}
	return total
}

// resetGlyphCache drops the cached masks, after a change of the model.
func resetGlyphCache() {
	c := sharedGlyphCache()
	c.mu.Lock()
	c.masks = map[glyphKey]*glyphMask{}
	c.zoneCache = nil
	c.bytes = 0
	c.mu.Unlock()
}

// The limits of TestGlyphModel on the validation set: the root mean square
// difference per channel over the cells of the text, and the share of edge
// pixels that differ by more than 16 of 255.
const (
	maxValidationRMS  = 4.0
	maxValidationOver = 9.0
)

// TestGlyphModel measures the glyph model on the validation set and logs
// the residual error per sheet and per font size. The sheets are screenshots
// of text in the system fonts of the macOS reference machine.
func TestGlyphModel(t *testing.T) {
	fonttest.RequireMacOSReference(t)
	for _, s := range validationSheets() {
		ref := loadSheet(t, s)
		resetGlyphCache()
		img, err := Render(s.svg(nil), Options{})
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds() != ref.Bounds() {
			t.Fatalf("%s: size %v, want %v", s.Name, img.Bounds().Size(), ref.Bounds().Size())
		}
		bg := [3]uint8{ref.Pix[0], ref.Pix[1], ref.Pix[2]}
		bySize := map[float64]*cellError{}
		var total cellError
		for i := range s.Items {
			it := &s.Items[i]
			e := compareCell(img, ref, it.Cell, bg)
			if bySize[it.Size] == nil {
				bySize[it.Size] = &cellError{}
			}
			bySize[it.Size].add(e)
			total.add(e)
			if e.rms() > 2*maxValidationRMS {
				t.Errorf("%s: %s %d at %g px: rms %.2f", s.Name, it.Family, it.Weight, it.Size, e.rms())
			}
		}
		var sizes []float64
		for k := range bySize {
			sizes = append(sizes, k)
		}
		sort.Float64s(sizes)
		var b strings.Builder
		for _, k := range sizes {
			e := bySize[k]
			fmt.Fprintf(&b, "\n  %4g px: rms %.2f, ink bias %+.3f, %.1f%% of edge pixels over 16", k, e.rms(), e.bias(), e.overShare())
		}
		t.Logf("%s: rms %.2f, ink bias %+.3f, %.1f%% of edge pixels over 16%s", s.Name, total.rms(), total.bias(), total.overShare(), b.String())
		if total.rms() > maxValidationRMS || total.overShare() > maxValidationOver {
			t.Errorf("%s: rms %.2f (limit %.1f), %.1f%% of edge pixels over 16 (limit %.1f%%)", s.Name, total.rms(), maxValidationRMS, total.overShare(), maxValidationOver)
		}
	}
}

// TestFitGrowth prints the error of the model on the shape sheet for a
// range of growth factors and flattening tolerances, per kind of glyph. It
// documents the choice of glyphModel.GrowX, GrowY and flattenTolerance. It
// runs with RASTER_FIT=1.
func TestFitGrowth(t *testing.T) {
	if os.Getenv("RASTER_FIT") == "" {
		t.Skip("RASTER_FIT is not set")
	}
	s := shapeSheet()
	ref := loadSheet(t, s)
	saved, savedTol := model, flattenTolerance
	defer func() { model, flattenTolerance = saved, savedTol; resetGlyphCache() }()
	kinds := []struct {
		name  string
		texts string
	}{{"flat", "Il-_.H"}, {"round", "oes8"}, {"diagonal", "vxA/"}}
	for _, kind := range kinds {
		keep := func(it *calItem) bool { return strings.Contains(kind.texts, it.Text) }
		for _, g := range []float32{0.0054, 0.0058, 0.0062} {
			for _, tol := range []float32{0.005, 0.12, 0.24, 0.36} {
				model.GrowX, model.GrowY, flattenTolerance = g, g, tol
				e := sheetError(t, s, ref, keep)
				fmt.Printf("%-8s growth %.4f flatten %.3f: rms %.3f, ink bias %+.4f, %.1f%% over 16\n", kind.name, g, tol, e.rms(), e.bias(), e.overShare())
			}
		}
	}
}

// TestFitZones searches the table of capital heights of a font family
// (zoneTables): for each size it tries the pixel borders next to the top
// of the capitals, and no move, and keeps the choice with the smallest
// error on the zone sheet of the family. It prints the table as Go code.
// RASTER_FIT_ZONES is the family.
func TestFitZones(t *testing.T) {
	family := os.Getenv("RASTER_FIT_ZONES")
	if family == "" {
		t.Skip("RASTER_FIT_ZONES is not set")
	}
	s := zoneSheet(family)
	ref := loadSheet(t, s)
	key := strings.ToLower(family)
	saved := zoneTables[key]
	defer func() {
		if saved == nil {
			delete(zoneTables, key)
		} else {
			zoneTables[key] = saved
		}
		resetGlyphCache()
	}()
	table := &zoneTable{}
	zoneTables[key] = table
	face := font.System().Resolve([]string{family}, 400, false).Primary()
	if face == nil {
		t.Fatalf("no font %q", family)
	}
	c := sharedGlyphCache()
	c.mu.Lock()
	z := c.zones(face)
	c.mu.Unlock()
	for n := 9; n <= maxFitSize; n++ {
		size := float64(n)
		keep := func(it *calItem) bool { return it.Size == size }
		_, gy := model.growth(float32(n))
		capTop := float32(z.capTop*float32(n)) + float32(2*gy)
		best, bestC := math.Inf(1), int8(0)
		for _, fc := range []int8{0, int8(capTop) - 1, int8(capTop), int8(capTop) + 1} {
			table.caps[n] = fc
			if r := sheetError(t, s, ref, keep).rms(); r < best {
				best, bestC = r, fc
			}
		}
		table.caps[n] = bestC
		fmt.Printf("size %2d: capitals %.2f -> %d: rms %.2f\n", n, capTop, bestC, best)
	}
	fmt.Printf("%q: {caps: %#v},\n", key, table.caps)
}
