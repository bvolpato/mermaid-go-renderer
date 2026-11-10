// This file ports no upstream file. It exports the MathML layout produced
// from Chromium 156.0.8078.4 math layout algorithms for native painting.
package browser

import "github.com/bvolpato/mermaid-go-renderer/internal/dom"

// RasterMathRule is a filled rule in foreignObject content coordinates.
type RasterMathRule struct {
	Owner               *dom.Node
	X, Y, Width, Height float64
}

// RasterMathBox has the glyph runs and rules of one MathML math root.
// Coordinates exclude the x and y of the foreignObject itself.
type RasterMathBox struct {
	Owner               *dom.Node
	X, Y, Width, Height float64
	Runs                []RasterTextFragment
	Rules               []RasterMathRule
	// InlineAncestors have no box in the HTML tree. A painter applies
	// their opacity when it paints this root.
	InlineAncestors []*dom.Node
}

// RasterForeignObjectMath returns math roots in document order. Math text
// does not also appear in the normal HTML text fragments.
func (e *Engine) RasterForeignObjectMath(fo *dom.Node) []RasterMathBox {
	var out []RasterMathBox
	var gather func(*RasterHTMLBox)
	gather = func(b *RasterHTMLBox) {
		if b.Math != nil {
			out = append(out, *b.Math)
		}
		for _, c := range b.Children {
			gather(c)
		}
	}
	for _, b := range e.RasterForeignObject(fo) {
		gather(b)
	}
	return out
}

func appendRasterMath(dst *RasterMathBox, b *htmlBox, x, y float64) {
	for _, run := range b.math.runs {
		run.X += float32(x)
		run.Baseline += float32(y)
		run.Glyphs = append([]RasterGlyph(nil), run.Glyphs...)
		dst.Runs = append(dst.Runs, run)
	}
	for _, rule := range b.math.rules {
		rule.X += x
		rule.Y += y
		dst.Rules = append(dst.Rules, rule)
	}
}
