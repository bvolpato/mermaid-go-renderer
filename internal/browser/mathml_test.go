// Tests Chromium 156.0.8078.4 MathML measurements recorded by
// tools/reference/gen-browser-mathlayout-probe.mjs (BSD-3-Clause upstream).
package browser

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

func TestMathLayoutAgainstChromium(t *testing.T) {
	var input struct {
		Cases []struct{ Name, HTML, Sel string }
	}
	read := func(path string, dst any) {
		data, err := os.ReadFile("testdata/" + path)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(data, dst); err != nil {
			t.Fatal(err)
		}
	}
	read("mathlayout-cases.json", &input)
	for _, env := range []string{"linux", "macos"} {
		t.Run(env, func(t *testing.T) {
			var lib *font.Library
			if env == "linux" {
				lib = font.Embedded()
			} else {
				macOSProbe(t)
				lib = font.System()
			}
			var results []struct {
				Name  string
				Boxes []struct {
					Tag           string
					Width, Height float64
				}
			}
			read("mathlayout-"+env+"-results.json", &results)
			if len(results) != len(input.Cases) {
				t.Fatal("reference case count differs")
			}
			exact, total := 0, 0
			for i, c := range input.Cases {
				doc := NewDocumentWithFonts(lib)
				container := doc.Body.Append("div").Attr("id", "container")
				container.HTML(c.HTML)
				// The capture container has no box of its own beyond the normal body margin.
				elements := container.SelectAll(c.Sel)
				if len(elements) != len(results[i].Boxes) {
					t.Fatalf("%s: %d elements, want %d", c.Name, len(elements), len(results[i].Boxes))
				}
				for j, n := range elements {
					want := results[i].Boxes[j]
					if results[i].Name != c.Name || want.Tag != n.Tag {
						t.Fatalf("unexpected reference box for %s %s", c.Name, n.Tag)
					}
					got := n.BoundingClientRect()
					w, h := want.Width, want.Height
					total++
					if got.Width == w && got.Height == h {
						exact++
					} else {
						// Reduced table and radical layout can round two layout
						// units differently. Other sizing errors must fail.
						if math.Abs(got.Width-w) > 1.0/32 || math.Abs(got.Height-h) > 1.0/32 {
							t.Errorf("%s %s: got %g x %g, want %g x %g", c.Name, n.Tag, got.Width, got.Height, w, h)
						}
					}
				}
			}
			t.Logf("exact root/wrapper dimensions: %d/%d", exact, total)
		})
	}
}

// This contract protects the raster handoff independently of pixel painting.
func TestMathRasterExport(t *testing.T) {
	doc := NewDocumentWithFonts(font.Embedded())
	svg := doc.Body.Append("svg:svg")
	fo := svg.Append("foreignObject").Attr("width", "200").Attr("height", "100")
	div := fo.Append("xhtml:div").Attr("style", "font-size:16px")
	if fo.NS != dom.NSSVG || div.NS != dom.NSHTML {
		t.Fatal("invalid foreignObject test tree")
	}
	div.HTML(`<span style="opacity:0.5"><math xmlns="http://www.w3.org/1998/Math/MathML"><semantics><mrow><mi>x</mi><mfrac><mspace width="30px" height="10px"/><mspace width="30px" height="10px"/></mfrac></mrow><annotation>NOT PAINTED</annotation></semantics></math></span>`)
	e := doc.Engine.(*Engine)
	var roots []*RasterMathBox
	var normalGlyphs int
	var visit func(*RasterHTMLBox)
	visit = func(b *RasterHTMLBox) {
		if b.Math != nil {
			roots = append(roots, b.Math)
		}
		for _, line := range b.Lines {
			for _, frag := range line.Fragments {
				normalGlyphs += len(frag.Glyphs)
			}
		}
		for _, c := range b.Children {
			visit(c)
		}
	}
	for _, b := range e.RasterForeignObject(fo) {
		visit(b)
	}
	if len(roots) != 1 {
		t.Fatalf("got %d math roots, want one", len(roots))
	}
	root := roots[0]
	if normalGlyphs != 0 || len(root.Rules) != 1 || len(root.Runs) != 1 {
		t.Fatalf("duplicate or absent math paint: normal glyphs %d, rules %d, runs %d", normalGlyphs, len(root.Rules), len(root.Runs))
	}
	if len(root.Runs[0].Glyphs) != 1 || root.Runs[0].Glyphs[0].ID == 0 {
		t.Fatal("missing mathematical alphabet glyph has no readable italic fallback")
	}
	if len(root.InlineAncestors) != 1 || root.InlineAncestors[0].Tag != "span" {
		t.Fatalf("unexpected unboxed ancestors: %v", root.InlineAncestors)
	}
	if root.Rules[0].Width != 30 || root.Rules[0].Height <= 0 {
		t.Fatalf("unexpected fraction rule: %+v", root.Rules[0])
	}
	root.Runs[0].Glyphs[0].ID = 0
	fresh := e.RasterForeignObjectMath(fo)
	if len(fresh) != 1 || fresh[0].Runs[0].Glyphs[0].ID == 0 {
		t.Fatal("a caller changed cached layout through exported glyphs")
	}
}

func TestMathAnnotationOnlyIsNotPainted(t *testing.T) {
	doc := NewDocumentWithFonts(font.Embedded())
	fo := doc.Body.Append("svg:svg").Append("foreignObject").Attr("width", "200").Attr("height", "100")
	fo.Append("xhtml:div").HTML(`<math xmlns="http://www.w3.org/1998/Math/MathML"><semantics><annotation>SECRET</annotation></semantics></math>`)
	boxes := doc.Engine.(*Engine).RasterForeignObjectMath(fo)
	if len(boxes) != 1 {
		t.Fatalf("got %d roots, want one", len(boxes))
	}
	if len(boxes[0].Runs) != 0 || len(boxes[0].Rules) != 0 || boxes[0].Width != 0 || boxes[0].Height != 0 {
		t.Fatalf("hidden annotation produced visible math: %+v", boxes[0])
	}
}
