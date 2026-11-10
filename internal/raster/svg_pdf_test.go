// This file validates the original PDF target with Poppler and the recorded
// Chromium 156 raster fixtures already used by this package.
package raster

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

func pdfTool(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s is not installed", name)
	}
	return path
}

func pdfImage(t *testing.T, data []byte, page int) image.Image {
	t.Helper()
	tool := pdfTool(t, "pdftoppm")
	dir := t.TempDir()
	path := filepath.Join(dir, "render.pdf")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "page")
	cmd := exec.Command(tool, "-r", "96", "-f", fmt.Sprint(page), "-l", fmt.Sprint(page), "-singlefile", "-png", path, out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pdftoppm: %v: %s", err, b)
	}
	f, err := os.Open(out + ".png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func nearPDFColor(t *testing.T, img image.Image, x, y int, want color.NRGBA) {
	t.Helper()
	got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	for i, p := range [][2]uint8{{got.R, want.R}, {got.G, want.G}, {got.B, want.B}} {
		if math.Abs(float64(p[0])-float64(p[1])) > 3 {
			t.Errorf("pixel %d,%d channel %d: %v, want %v", x, y, i, got, want)
		}
	}
}

func TestPDFPageSlices(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="80"><rect width="40" height="40" fill="red"/><rect y="40" width="40" height="40" fill="blue"/></svg>`
	data, err := RenderPDF(svg, Options{Fonts: font.Embedded(), Background: color.Transparent}, PDFPage{Width: 60, Height: 40, X: 10, Background: color.NRGBA{G: 128, A: 255}, Pages: 2})
	if err != nil {
		t.Fatal(err)
	}
	for page, want := range []color.NRGBA{{R: 255, A: 255}, {B: 255, A: 255}} {
		img := pdfImage(t, data, page+1)
		if img.Bounds().Dx() != 60 || img.Bounds().Dy() != 40 {
			t.Fatalf("page bounds %v", img.Bounds())
		}
		nearPDFColor(t, img, 20, 20, want)
		nearPDFColor(t, img, 5, 20, color.NRGBA{G: 128, A: 255})
	}
}

func TestPDFClipAndOpacity(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><defs><clipPath id="c"><circle cx="25" cy="25" r="20"/></clipPath></defs><g opacity="0.5" transform="translate(10 10)"><g clip-path="url(#c)"><rect width="50" height="50" fill="red"/><rect x="20" width="30" height="50" fill="blue"/></g></g><rect x="65" y="10" width="25" height="25" fill="green"/></svg>`
	data, err := RenderPDF(svg, Options{Fonts: font.Embedded()}, PDFPage{})
	if err != nil {
		t.Fatal(err)
	}
	img := pdfImage(t, data, 1)
	nearPDFColor(t, img, 10, 10, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	nearPDFColor(t, img, 25, 35, color.NRGBA{R: 255, G: 127, B: 127, A: 255})
	nearPDFColor(t, img, 40, 35, color.NRGBA{R: 127, G: 127, B: 255, A: 255})
	nearPDFColor(t, img, 75, 20, color.NRGBA{G: 128, A: 255})
}

func TestPDFEmbeddedText(t *testing.T) {
	tool := pdfTool(t, "pdftotext")
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="240" height="80"><text x="10" y="25" font-family="Arial" font-size="16">Hello café Ω</text><foreignObject x="10" y="35" width="220" height="40"><div xmlns="http://www.w3.org/1999/xhtml" style="font-family:Arial;font-size:16px">HTML label</div></foreignObject></svg>`
	data, err := RenderPDF(svg, Options{Fonts: font.Embedded()}, PDFPage{})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "text.pdf")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(tool, "-layout", path, "-").CombinedOutput()
	if err != nil {
		t.Fatalf("pdftotext: %v: %s", err, out)
	}
	for _, want := range []string{"Hello café Ω", "HTML label"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("extracted %q, missing %q", out, want)
		}
	}
	// Poppler must also paint the embedded fonts without font lookup errors.
	pdfImage(t, data, 1)
}

func TestPDFGolden(t *testing.T) {
	pdfTool(t, "pdftoppm")
	cases := []struct {
		name  string
		limit float64
	}{
		{"shapes-aa", 0.5}, {"stroke-caps", 0.5}, {"dash", 0.5}, {"markers", 0.5},
		{"clip-nested", 0.5}, {"opacity-blend", 0.5}, {"gradient-linear", 0.5}, {"gradient-radial", 0.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svg, err := os.ReadFile(filepath.Join("testdata/golden", tc.name+".svg"))
			if err != nil {
				t.Fatal(err)
			}
			data, err := RenderPDF(string(svg), Options{Fonts: font.Embedded()}, PDFPage{})
			if err != nil {
				t.Fatal(err)
			}
			img := pdfImage(t, data, 1)
			want, err := loadNRGBA(filepath.Join("testdata/golden", tc.name+".png"))
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds() != want.Bounds() {
				t.Fatalf("size %v, want %v", img.Bounds(), want.Bounds())
			}
			var bad, interiorBad, interior int
			for y := 0; y < img.Bounds().Dy(); y++ {
				for x := 0; x < img.Bounds().Dx(); x++ {
					g := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
					w := want.NRGBAAt(x, y)
					different := pdfColorDifference(g, w) > 32
					if different {
						bad++
					}
					// Poppler and Chromium cover path edges differently. Compare
					// color interiors separately to detect lost geometry and paint.
					if pdfInterior(want, x, y) {
						interior++
						if different {
							interiorBad++
						}
					}
				}
			}
			mismatch := 100 * float64(bad) / float64(img.Bounds().Dx()*img.Bounds().Dy())
			interiorMismatch := 100 * float64(interiorBad) / float64(interior)
			t.Logf("pixels differing by more than 32/255: all %.3f%%, interiors %.3f%%", mismatch, interiorMismatch)
			if interiorMismatch > tc.limit {
				t.Errorf("interior mismatch %.3f%% exceeds %.2f%%", interiorMismatch, tc.limit)
			}
		})
	}
}

func pdfInterior(img *image.NRGBA, x, y int) bool {
	want := img.NRGBAAt(x, y)
	for yy := max(0, y-1); yy <= min(img.Bounds().Dy()-1, y+1); yy++ {
		for xx := max(0, x-1); xx <= min(img.Bounds().Dx()-1, x+1); xx++ {
			if pdfColorDifference(want, img.NRGBAAt(xx, yy)) > 32 {
				return false
			}
		}
	}
	return true
}

func pdfColorDifference(a, b color.NRGBA) int {
	return max(absInt(int(a.R)-int(b.R)), absInt(int(a.G)-int(b.G)), absInt(int(a.B)-int(b.B)))
}

func TestPDFInvalidOptions(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40"/>`
	for _, page := range []PDFPage{{Width: -1}, {Height: math.Inf(1)}, {X: math.NaN()}, {Pages: -1}} {
		if _, err := RenderPDF(svg, Options{}, page); err == nil {
			t.Errorf("accepted %#v", page)
		}
	}
	if _, err := RenderPDF(svg, Options{Scale: math.Inf(1)}, PDFPage{}); err == nil {
		t.Error("accepted infinite scale")
	}
	if _, err := RenderPDF("<div/>", Options{}, PDFPage{}); err == nil {
		t.Error("accepted missing SVG")
	}
	data, err := RenderPDF(svg, Options{Fonts: font.Embedded()}, PDFPage{})
	if err != nil || !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatalf("valid PDF: %v", err)
	}
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func TestPDFFlatShadow(t *testing.T) {
	for _, filter := range []string{`url(#shadow)`, `drop-shadow(8px 6px 0 rgba(0,0,0,0.25))`} {
		t.Run(filter, func(t *testing.T) {
			svg := `<svg xmlns="http://www.w3.org/2000/svg" width="80" height="60"><defs><filter id="shadow" x="-50%" y="-50%" width="200%" height="200%"><feDropShadow dx="8" dy="6" stdDeviation="0" flood-color="black" flood-opacity="0.25"/></filter></defs><rect x="10" y="10" width="20" height="20" fill="red" filter="` + filter + `"/></svg>`
			data, err := RenderPDF(svg, Options{Fonts: font.Embedded()}, PDFPage{})
			if err != nil {
				t.Fatal(err)
			}
			img := pdfImage(t, data, 1)
			nearPDFColor(t, img, 20, 20, color.NRGBA{R: 255, A: 255})
			nearPDFColor(t, img, 34, 25, color.NRGBA{R: 191, G: 191, B: 191, A: 255})
			nearPDFColor(t, img, 45, 25, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		})
	}
}

func TestPDFTinyViewBox(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 1e-7 1e-7"><path d="M0 0L1e-7 0L0 1e-7Z" fill="red"/></svg>`
	data, err := RenderPDF(svg, Options{Fonts: font.Embedded()}, PDFPage{})
	if err != nil {
		t.Fatal(err)
	}
	img := pdfImage(t, data, 1)
	nearPDFColor(t, img, 10, 10, color.NRGBA{R: 255, A: 255})
	nearPDFColor(t, img, 90, 90, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
}

func TestPDFExtremeQuadratic(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><path d="M-3e38 0 Q3e38 0 3e38 10"/></svg>`
	data, err := RenderPDF(svg, Options{Fonts: font.Embedded()}, PDFPage{})
	if err != nil {
		t.Fatal(err)
	}
	pdfImage(t, data, 1)
}

func TestPDFContextualWebFontText(t *testing.T) {
	tool := pdfTool(t, "pdftotext")
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="240" height="50"><text x="10" y="30" font-family="Recursive Variable" font-size="14">Hello café World</text></svg>`
	data, err := RenderPDF(svg, Options{Fonts: font.Embedded().WithWebFonts()}, PDFPage{})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "contextual.pdf")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(tool, "-layout", path, "-").CombinedOutput()
	if err != nil {
		t.Fatalf("pdftotext: %v: %s", err, out)
	}
	if !strings.Contains(string(out), "Hello café World") {
		t.Fatalf("contextual web font text extracted %q", out)
	}
}

func TestPDFClipUnion(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><defs><clipPath id="upper"><path d="M0 0H50V40H0Z"/><path d="M30 0V40H80V0Z"/></clipPath><clipPath id="lower"><path clip-rule="evenodd" d="M10 50H50V90H10Z M20 60H40V80H20Z"/><rect x="70" y="50" width="20" height="40"/></clipPath></defs><g clip-path="url(#upper)"><rect width="100" height="40" fill="red"/></g><g clip-path="url(#lower)"><rect width="100" height="100" fill="blue"/></g></svg>`
	data, err := RenderPDF(svg, Options{Fonts: font.Embedded()}, PDFPage{})
	if err != nil {
		t.Fatal(err)
	}
	img := pdfImage(t, data, 1)
	nearPDFColor(t, img, 40, 20, color.NRGBA{R: 255, A: 255})
	nearPDFColor(t, img, 15, 55, color.NRGBA{B: 255, A: 255})
	nearPDFColor(t, img, 30, 70, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	nearPDFColor(t, img, 75, 55, color.NRGBA{B: 255, A: 255})
}
