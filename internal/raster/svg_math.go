// This file ports no upstream file. It paints the MathML layout exported
// by internal/browser through the existing glyph and rectangle painters.
// The layout follows Chromium 156.0.8078.4, including MathMLPaintInfo in
// third_party/blink/renderer/core/layout/mathml/mathml_paint_info.h
// (BSD-3-Clause).

package raster

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/browser"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

// paintHTMLMathBox paints one math root while its HTML ancestor layers and
// the foreignObject clip are still active. Its children have no text lines.
func (p *painter) paintHTMLMathBox(b *browser.RasterHTMLBox, ctm Affine) {
	box := b.Math
	if box == nil {
		return
	}
	opacity := float32(1)
	for _, ancestor := range box.InlineAncestors {
		opacity *= p.htmlOpacity(ancestor)
	}
	if opacity == 0 {
		return
	}
	if opacity < 1 {
		p.opacityLayer()
		defer p.cv.popLayer(opacity, blendSrcOver)
	}
	for _, rule := range box.Rules {
		p.paintMathRule(rule.Owner, box.Owner, rule.X, rule.Y, rule.Width, rule.Height, ctm)
	}
	for i := range box.Runs {
		p.paintMathRun(&box.Runs[i], box.Owner, ctm)
	}
}

// mathPaintOpacity applies styles below a math root. The HTML box painter
// already applies opacity on the root and its HTML ancestors.
func (p *painter) mathPaintOpacity(owner, root *dom.Node) float32 {
	opacity := float32(1)
	for n := owner; n != nil; n = n.Parent {
		if p.eng.RasterDisplay(n) == "none" {
			return 0
		}
		if n == root {
			break
		}
		opacity *= p.htmlOpacity(n)
	}
	return opacity
}

func (p *painter) paintMathRun(f *browser.RasterTextFragment, root *dom.Node, ctm Affine) {
	owner := f.Owner
	if owner == nil {
		owner = root
	}
	if !p.htmlVisible(owner) || !(f.FontSize > 0) || len(f.Glyphs) == 0 {
		return
	}
	c := p.htmlColor(owner)
	paint := Paint{R: c.R, G: c.G, B: c.B, A: cssAlphaByte(c, p.mathPaintOpacity(owner, root)), antiAlias: true}
	if paint.A == 0 {
		return
	}
	run := glyphRun{glyphs: f.Glyphs, x: f.X, y: f.Baseline, fontSize: float32(f.FontSize)}
	p.drawGlyphRun(&run, &paint, ctm)
}

func (p *painter) paintMathRule(owner, root *dom.Node, x, y, w, h float64, ctm Affine) {
	if owner == nil {
		owner = root
	}
	if !p.htmlVisible(owner) || !(w > 0 && h > 0) {
		return
	}
	c := p.htmlColor(owner)
	paint := Paint{R: c.R, G: c.G, B: c.B, A: cssAlphaByte(c, p.mathPaintOpacity(owner, root)), antiAlias: true}
	if paint.A == 0 {
		return
	}
	m := deviceMatrix(ctm)
	p.cv.dr.drawRect(p.cv.dev(), Rect{float32(x), float32(y), float32(x + w), float32(y + h)}, &paint, &m)
}
