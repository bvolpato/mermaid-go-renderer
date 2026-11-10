// This file ports no upstream file. It checks the integration of the
// browser MathML layout with the foreignObject painter.

package raster

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

func renderMathTest(t *testing.T, content, style, placement string, scale float64) *image.RGBA {
	t.Helper()
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="220" height="120"><foreignObject %s width="180" height="100"><div xmlns="http://www.w3.org/1999/xhtml" style="font-size:24px;color:#b02040;%s">%s</div></foreignObject></svg>`, placement, style, content)
	img, err := Render(svg, Options{Fonts: font.Embedded(), Background: color.NRGBA{}, Scale: scale})
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func mathTestInk(img *image.RGBA) (bounds image.Rectangle, maxAlpha uint8) {
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			a := img.Pix[img.PixOffset(x, y)+3]
			if a != 0 {
				bounds = bounds.Union(image.Rect(x, y, x+1, y+1))
				maxAlpha = max(maxAlpha, a)
			}
		}
	}
	return bounds, maxAlpha
}

func TestMathForeignObjectPainting(t *testing.T) {
	cases := []struct{ name, content string }{
		{"identifier-x", `<mi>x</mi>`},
		{"identifier-greek", `<mi>α</mi>`},
		{"power", `<msup><mi>x</mi><mn>2</mn></msup>`},
		{"fraction", `<mfrac><mn>1</mn><mn>2</mn></mfrac>`},
		{"radical", `<msqrt><mi>x</mi><mo>+</mo><mn>3</mn></msqrt>`},
		{"greek", `<mrow><mi>α</mi><mo>+</mo><mi>β</mi></mrow>`},
		{"matrix", `<mtable><mtr><mtd><mn>1</mn></mtd><mtd><mn>2</mn></mtd></mtr><mtr><mtd><mn>3</mn></mtd><mtd><mn>4</mn></mtd></mtr></mtable>`},
	}
	for _, tc := range cases {
		for _, scale := range []float64{1, 2} {
			t.Run(fmt.Sprintf("%s@%g", tc.name, scale), func(t *testing.T) {
				content := `<math xmlns="http://www.w3.org/1998/Math/MathML">` + tc.content + `</math>`
				got := renderMathTest(t, content, "", `x="12" y="8"`, scale)
				bounds, alpha := mathTestInk(got)
				if bounds.Empty() || alpha == 0 {
					t.Fatal("math painted no pixels")
				}
				for i := 0; i < len(got.Pix); i += 4 {
					if got.Pix[i+3] > 32 && got.Pix[i] <= got.Pix[i+1] {
						t.Fatal("math did not inherit the HTML text color")
					}
				}
				moved := renderMathTest(t, content, "", `transform="translate(12 8)"`, scale)
				if !bytes.Equal(got.Pix, moved.Pix) {
					t.Error("foreignObject position and equivalent transform painted different pixels")
				}
				if out := os.Getenv("RASTER_MATH_OUT"); out != "" {
					if err := os.MkdirAll(out, 0755); err != nil {
						t.Fatal(err)
					}
					file, err := os.Create(filepath.Join(out, fmt.Sprintf("%s-%g.png", tc.name, scale)))
					if err != nil {
						t.Fatal(err)
					}
					err = EncodePNG(file, got)
					closeErr := file.Close()
					if err != nil || closeErr != nil {
						t.Fatalf("write PNG: %v, close: %v", err, closeErr)
					}
				}
			})
		}
	}
}

func TestMathForeignObjectStyles(t *testing.T) {
	const expr = `<math xmlns="http://www.w3.org/1998/Math/MathML"><mfrac><mn>1</mn><mn>2</mn></mfrac></math>`
	for _, style := range []string{"visibility:hidden", "display:none", "opacity:0"} {
		t.Run(style, func(t *testing.T) {
			got := renderMathTest(t, expr, style, "", 1)
			if bounds, _ := mathTestInk(got); !bounds.Empty() {
				t.Errorf("hidden math painted pixels at %v", bounds)
			}
		})
	}
	t.Run("nested opacity", func(t *testing.T) {
		content := `<span style="opacity:0.5">` + expr + `</span>`
		got := renderMathTest(t, content, "opacity:0.5", "", 1)
		outer := renderMathTest(t, expr, "opacity:0.5", "", 1)
		_, outerAlpha := mathTestInk(outer)
		bounds, alpha := mathTestInk(got)
		delta := int(alpha)*2 - int(outerAlpha)
		if bounds.Empty() || alpha == 0 || delta < -1 || delta > 1 {
			t.Errorf("nested opacity painted bounds %v with alpha %d, want half of outer alpha %d", bounds, alpha, outerAlpha)
		}
	})
	t.Run("annotation", func(t *testing.T) {
		content := `<math xmlns="http://www.w3.org/1998/Math/MathML"><semantics><mfrac><mn>1</mn><mn>2</mn></mfrac><annotation encoding="application/x-tex">HIDDEN ANNOTATION</annotation></semantics></math>`
		got := renderMathTest(t, content, "", "", 1)
		want := renderMathTest(t, expr, "", "", 1)
		if !bytes.Equal(got.Pix, want.Pix) {
			t.Error("the annotation changed the painted expression")
		}
	})
	t.Run("phantom", func(t *testing.T) {
		content := `<math xmlns="http://www.w3.org/1998/Math/MathML"><mphantom><mfrac><mn>1</mn><mn>2</mn></mfrac></mphantom></math>`
		if bounds, _ := mathTestInk(renderMathTest(t, content, "", "", 1)); !bounds.Empty() {
			t.Errorf("phantom math painted pixels at %v", bounds)
		}
	})
	t.Run("legacy branch", func(t *testing.T) {
		prefix := `<span class="katex"><span class="katex-mathml">` + expr + `</span>`
		want := renderMathTest(t, prefix+`</span>`, "", "", 1)
		got := renderMathTest(t, prefix+`<span class="katex-html" aria-hidden="true">DUPLICATE HTML</span></span>`, "", "", 1)
		if !bytes.Equal(got.Pix, want.Pix) {
			t.Error("the legacy HTML branch painted over native math")
		}
		unrelated := renderMathTest(t, prefix+`</span><span class="katex-html" aria-hidden="true">VISIBLE HTML</span>`, "", "", 1)
		if bytes.Equal(unrelated.Pix, want.Pix) {
			t.Error("an unrelated HTML branch was hidden")
		}
	})
}

func TestMathForeignObjectRuleAndClip(t *testing.T) {
	const expr = `<math xmlns="http://www.w3.org/1998/Math/MathML"><mfrac><mspace width="64px" height="16px"/><mspace width="64px" height="16px"/></mfrac></math>`
	got := renderMathTest(t, expr, "", `x="12" y="8"`, 1)
	bounds, _ := mathTestInk(got)
	if bounds.Dx() < 64 || bounds.Dy() < 1 || bounds.Dy() > 4 {
		t.Fatalf("fraction rule painted bounds %v, want a thin horizontal line at least 64 pixels wide", bounds)
	}
	for _, overflow := range []string{"hidden", "visible"} {
		t.Run(overflow, func(t *testing.T) {
			svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="220" height="120"><foreignObject x="12" y="8" width="16" height="100" style="overflow:%s"><div xmlns="http://www.w3.org/1999/xhtml" style="font-size:24px">%s</div></foreignObject></svg>`, overflow, expr)
			img, err := Render(svg, Options{Fonts: font.Embedded(), Background: color.NRGBA{}})
			if err != nil {
				t.Fatal(err)
			}
			bounds, _ := mathTestInk(img)
			if bounds.Empty() {
				t.Fatal("fraction rule painted no pixels")
			}
			if overflow == "hidden" && bounds.Max.X > 28 {
				t.Errorf("foreignObject clip leaked math pixels to %v", bounds)
			}
			if overflow == "visible" && bounds.Max.X <= 28 {
				t.Errorf("overflow:visible clipped the fraction rule at %v", bounds)
			}
		})
	}
}
