// Text painting: SVG text elements and the HTML content of foreignObject
// elements. The positions come from the layout engine (internal/browser).
// The glyph placement follows SkGlyphRunListPainterCPU::drawForBitmapDevice
// and prepare_for_direct_mask_drawing of skia/src/core/SkGlyphRunPainter.cpp
// (Skia branch chrome/m156).
//
// The comments of this file name parts of Chromium. The license of each
// file is the license in its header at Chromium 156.0.8078.4. The Blink
// paths are in third_party/blink/renderer:
//
//   - FontMetrics::FixedAscent: platform/fonts/font_metrics.h
//     (LGPL-2.0-or-later).
//   - ComputedStyle::GetFontBaseline: core/style/computed_style.h
//     (LGPL-2.0-or-later).
//   - FontMetrics::AscentDescentWithHacks: platform/fonts/font_metrics.cc
//     (BSD-3-Clause, with the copyright of Apple and Alexey Proskuryakov in
//     the file).
//   - TextFragmentPainter, InlineBoxFragmentPainter, TextDecorationInfo,
//     DecorationLinePainter and FragmentPaintPropertyTreeBuilder: core/paint
//     (BSD-3-Clause).
//   - FragmentItem and InlineBoxState: core/layout/inline (BSD-3-Clause).
//   - gfx::GetFontRenderParams: ui/gfx/font_render_params_linux.cc
//     (BSD-3-Clause).
//
// See NOTICE in this directory and finding F1 of docs/LICENSE-AUDIT.md.

package raster

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/browser"
	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// glyphRun is a run of glyphs with one font size and one origin, in the
// user space of the text.
type glyphRun struct {
	glyphs   []browser.RasterGlyph
	x, y     float32 // origin of the run: the baseline start
	fontSize float32
}

// drawGlyphRun draws a run with a fill paint. It is the direct mask path
// of Skia: each glyph is a mask at an integer device position, chosen
// among four horizontal subpixel offsets.
func (p *painter) drawGlyphRun(run *glyphRun, paint *Paint, ctm Affine) {
	m := deviceMatrix(ctm)
	dev := p.cv.dev()
	if shouldDrawAsPath(&m, run.fontSize) {
		p.drawGlyphRunAsPaths(run, paint, ctm)
		return
	}
	sm, ok := strikeMatricesOf(&m, run.fontSize)
	if !ok {
		return
	}
	size, rot := sm.size, sm.rot
	// A paint with a shader has the luminance of middle gray
	// (SkPaintPriv::ComputeLuminanceColor).
	lr, lg, lb := uint8(128), uint8(128), uint8(128)
	if paint.shader == nil {
		lr, lg, lb = paint.R, paint.G, paint.B
	}
	// The table of the text luminance depends on the platform of the font.
	lumDarwin, lumSkia := lumBucket(lr, lg, lb), lumBucketSkia(lr, lg, lb)
	// Chromium on Linux draws text with one coverage per color channel
	// where Skia knows the pixel geometry: on the opaque base bitmap and in
	// an opacity layer that keeps it (opacityLayer), not in another layer.
	// The blitters draw such a mask only for a color with source-over
	// (GlyphRunListPainter::drawForBitmapDevice), and the scaler gives
	// large text one coverage per pixel.
	lcd := dev.lcd && paint.shader == nil && paint.blend == blendSrcOver && !tooBigForLCD(&m, run.fontSize)
	lumLCD := [3]uint8{canonicalLum(lr) >> (8 - lumBits), canonicalLum(lg) >> (8 - lumBits), canonicalLum(lb) >> (8 - lumBits)}
	// Chromium on Linux turns the hinting off when the device scale factor
	// is above 1: it then asks for subpixel positioning, and "to enable
	// subpixel positioning, we need to disable hinting"
	// (gfx::GetFontRenderParams of ui/gfx/font_render_params_linux.cc).
	unhinted := p.scale > 1
	b := p.cv.dr.wrapClip(dev, p.cv.dr.chooseBlitter(dev, paint))
	var colorBlit *spriteBlitter
	var lcdScratch []uint16

	// The position of a glyph is rounded to a quarter of a pixel along the
	// baseline and to a whole pixel across it, when the baseline is along
	// an axis (SkGlyphPositionRoundingSpec with
	// SkScalerContextRec::computeAxisAlignmentForHText). Other text gets
	// quarters in both directions.
	const quarter, half = float32(0.125), float32(0.5)
	roundX, roundY := quarter, half
	subX, subY := true, false
	switch {
	case sm.post[2] == 0:
	case sm.post[0] == 0:
		roundX, roundY = half, quarter
		subX, subY = false, true
	default:
		roundX, roundY = quarter, quarter
		subX, subY = true, true
	}
	ox := float32(m.sx*run.x) + float32(m.kx*run.y) + m.tx + roundX
	oy := float32(m.ky*run.x) + float32(m.sy*run.y) + m.ty + roundY
	var advance float32
	for i := range run.glyphs {
		g := &run.glyphs[i]
		px := advance + float32(g.XOffset)
		py := -float32(g.YOffset)
		advance += float32(g.Advance)
		if g.Face == nil {
			continue
		}
		mx := float32(px*m.sx) + float32(py*m.kx) + ox
		my := float32(px*m.ky) + float32(py*m.sy) + oy
		if !isFinite32(mx) || !isFinite32(my) {
			continue
		}
		ix := int(math.Floor(float64(mx)))
		iy := int(math.Floor(float64(my)))
		var qx, qy uint8
		if subX {
			qx = uint8(int(float32(mx-float32(ix))*4) & 3)
		}
		if subY {
			qy = uint8(int(float32(my-float32(iy))*4) & 3)
		}
		lum := lumDarwin
		if g.Face.Platform() != font.PlatformDarwin {
			lum = lumSkia
		}
		var gm *glyphMask
		if lcd && g.Face.Platform() != font.PlatformDarwin {
			gm = p.glyphs.getLCD(g.Face, g.ID, size, rot, qx, qy, lumLCD, unhinted)
			if gm.empty {
				continue
			}
			bounds := IRect{gm.mask.bounds.L + ix, gm.mask.bounds.T + iy, gm.mask.bounds.R + ix, gm.mask.bounds.B + iy}
			clip := bounds
			if !clip.intersect(dev.clip) {
				continue
			}
			dev.extend(clip)
			if dev.aa != nil && !dev.aa.quickContains(clip) {
				// SkAAClipBlitter::blitMask: the coverage of the clip
				// scales the mask.
				if !clip.intersect(dev.aa.bounds) {
					continue
				}
				lcdScratch = mergeLCD16(lcdScratch, gm.lcd, bounds, clip, dev.aa)
				blitLCD16(dev.pm, lcdScratch, clip, clip, paint)
				continue
			}
			blitLCD16(dev.pm, gm.lcd, bounds, clip, paint)
			continue
		}
		gm = p.glyphs.get(g.Face, g.ID, size, rot, qx, qy, lum, unhinted && g.Face.Platform() != font.PlatformDarwin)
		if gm.empty {
			continue
		}
		bounds := IRect{gm.mask.bounds.L + ix, gm.mask.bounds.T + iy, gm.mask.bounds.R + ix, gm.mask.bounds.B + iy}
		clip := bounds
		if !clip.intersect(dev.clip) {
			continue
		}
		dev.extend(clip)
		mask := maskA8{image: gm.mask.image, bounds: bounds, rowB: gm.mask.rowB}
		if gm.color != nil {
			if colorBlit == nil {
				colorBlit = &spriteBlitter{dst: dev.pm, alpha: paint.A}
			}
			colorBlit.blit(gm.color, bounds, clip, dev.aa)
			continue
		}
		b.blitMask(&mask, clip)
	}
}

// shouldDrawAsPath is SkStrikeSpec::ShouldDrawAsPath: Skia draws the
// outlines of text whose em square is larger than 256 pixels on the
// device, and keeps no masks for it.
func shouldDrawAsPath(m *Matrix, textSize float32) bool {
	const maxSizeSquared = 256 * 256
	// SkFontPriv::MakeTextMatrix and the matrix of the canvas.
	sx, ky := float32(m.sx*textSize), float32(m.ky*textSize)
	kx, sy := float32(m.kx*textSize), float32(m.sy*textSize)
	return float32(sx*sx)+float32(ky*ky) > maxSizeSquared || float32(kx*kx)+float32(sy*sy) > maxSizeSquared
}

// skRelax is sk_relax of skia/src/core/SkScalerContext.cpp: the matrix of
// a strike is rounded to 1/1024, so that tiny changes of the matrix of the
// canvas give the same glyphs.
func skRelax(x float32) float32 {
	return float32(math.Floor(float64(float32(x*1024))+0.5)) / 1024
}

// strikeMatrices describes the strike that Skia makes for a run: the font
// size that the scaler gets, and the matrix that is left for the outline.
type strikeMatrices struct {
	// post is fPost2x2 of SkScalerContextRec: the matrix of the canvas
	// after skRelax, as scaleX, skewX, skewY, scaleY.
	post [4]float32
	// size is the font size of the scaler in device pixels.
	size float32
	// rot is the remaining matrix as a, b, c, d (scaleX, skewY, skewX,
	// scaleY). It is zero when the matrix is the identity.
	rot [4]float32
}

// strikeMatricesOf ports SkScalerContext::MakeRecAndEffects for the matrix
// and SkScalerContextRec::computeMatrices with PreMatrixScale::kVertical,
// which the scalers of FreeType, Fontations and CoreText use: the font
// size is the vertical scale that is left when a rotation takes the
// baseline back to the x axis. ok is false for a singular matrix.
func strikeMatricesOf(m *Matrix, textSize float32) (sm strikeMatrices, ok bool) {
	p00, p01, p10, p11 := float32(1), float32(0), float32(0), float32(1)
	if m.sx != 1 || m.sy != 1 {
		p00, p11 = skRelax(m.sx), skRelax(m.sy)
	}
	if m.kx != 0 || m.ky != 0 {
		p01, p10 = skRelax(m.kx), skRelax(m.ky)
	}
	sm.post = [4]float32{p00, p01, p10, p11}
	// getSingleMatrix: A is the matrix of the strike times the text size.
	asx, akx := float32(p00*textSize), float32(p01*textSize)
	aky, asy := float32(p10*textSize), float32(p11*textSize)

	// GA is A with the rotation removed.
	gsx, gsy := asx, asy
	skewedOrFlipped := akx != 0 || aky != 0 || asx < 0 || asy < 0
	if skewedOrFlipped {
		// SkComputeGivensRotation for the image of the baseline, and
		// SkMatrix::setConcat of the rotation and A.
		c, s := givensRotation(asx, aky)
		switch {
		case c == 1 && s == 0:
		case s == 0 && akx == 0 && aky == 0:
			gsx, gsy = float32(c*asx), float32(c*asy)
		default:
			gsx = mulAddMul(c, asx, -s, aky)
			gsy = mulAddMul(s, akx, c, asy)
		}
	}
	const nearlyZero = float32(1.0 / (1 << 12))
	if fabs32(gsx) <= nearlyZero || fabs32(gsy) <= nearlyZero || !isFinite32(gsx) || !isFinite32(gsy) {
		return sm, false
	}
	sm.size = fabs32(gsy)
	switch {
	case !skewedOrFlipped && asx == asy:
		// No remaining matrix.
	case !skewedOrFlipped:
		sm.rot = [4]float32{asx / sm.size, 0, 0, 1}
	default:
		inv := 1 / sm.size
		sm.rot = [4]float32{float32(asx * inv), float32(aky * inv), float32(akx * inv), float32(asy * inv)}
	}
	return sm, true
}

// givensRotation is SkComputeGivensRotation of skia/src/utils/SkMatrix22.cpp:
// the cosine and the sine of the rotation that takes the vector (a, b) to
// the positive x axis.
func givensRotation(a, b float32) (c, s float32) {
	sign := func(v, of float32) float32 {
		return float32(math.Copysign(float64(v), float64(of)))
	}
	switch {
	case b == 0:
		return sign(1, a), 0
	case a == 0:
		return 0, -sign(1, b)
	case fabs32(b) > fabs32(a):
		t := a / b
		u := sign(sqrt32(1+float32(t*t)), b)
		s = -1 / u
		return float32(-s * t), s
	default:
		t := b / a
		u := sign(sqrt32(1+float32(t*t)), a)
		c = 1 / u
		return c, float32(-c * t)
	}
}

// mulAddMul is muladdmul of skia/src/core/SkMatrix.cpp: a*b + c*d in
// double precision.
func mulAddMul(a, b, c, d float32) float32 {
	return float32(float64(float64(a)*float64(b)) + float64(float64(c)*float64(d)))
}

// tooBigForLCD is too_big_for_lcd of skia/src/core/SkScalerContext.cpp with
// the matrix that SkScalerContext::MakeRecAndEffects takes from the canvas:
// text above 48 px on the device has no LCD masks. The area of the matrix
// has a sign, so text under a flip is never too big.
func tooBigForLCD(m *Matrix, textSize float32) bool {
	const maxSize = 48
	scale := m.sx != 1 || m.sy != 1
	affine := m.kx != 0 || m.ky != 0
	if !scale && !affine {
		return textSize > maxSize
	}
	p00, p11 := float32(1), float32(1)
	if scale {
		p00, p11 = skRelax(m.sx), skRelax(m.sy)
	}
	var p01, p10 float32
	if affine {
		p01, p10 = skRelax(m.kx), skRelax(m.ky)
	}
	area := float32(p00*p11) - float32(p10*p01)
	area *= float32(textSize * textSize)
	return area > maxSize*maxSize
}

// drawGlyphRunAsPaths draws the outlines of a run as paths: the stroke of
// text, and text that is too large for masks.
//
// For a Linux font the outline is the one of the strike that Skia makes
// for paths (SkStrikeSpec::MakePath and SkFont::setupForAsPaths): the
// outline at the canonical size of 64 px without hinting, under a scale of
// the text size divided by 64.
func (p *painter) drawGlyphRunAsPaths(run *glyphRun, paint *Paint, ctm Affine) {
	const canonicalSize = 64
	dev := p.cv.dev()
	var advance float32
	var path, scaled Path
	path.lastMoveIdx = -1
	scaled.lastMoveIdx = -1
	// A shader and a stroke need the matrix of the canvas: Skia then
	// scales the outline itself (needsExactCTM in
	// GlyphRunListPainter::drawForBitmapDevice).
	exactCTM := paint.shader != nil || (paint.stroke && paint.width > 0)
	for i := range run.glyphs {
		g := &run.glyphs[i]
		px := run.x + advance + float32(g.XOffset)
		py := run.y - float32(g.YOffset)
		advance += float32(g.Advance)
		if g.Face == nil {
			continue
		}
		if g.Face.Platform() == font.PlatformDarwin {
			p.glyphs.mu.Lock()
			ok := p.glyphs.devicePath(&path, g.Face, g.ID, run.fontSize, 0)
			p.glyphs.mu.Unlock()
			if !ok {
				continue
			}
			m := deviceMatrix(ctm.Translate(float64(px), float64(py)))
			p.cv.dr.drawPath(dev, &path, paint, &m)
			continue
		}
		o, ok := g.Face.ScalerOutlineUnhinted(g.ID, canonicalSize)
		if !ok || len(o.Segments) == 0 {
			continue
		}
		appendOutline(&path, o.Segments)
		scale := run.fontSize / canonicalSize
		if exactCTM {
			sm := Matrix{sx: scale, sy: scale, tx: px, ty: py}
			path.transformInto(&scaled, &sm)
			m := deviceMatrix(ctm)
			p.cv.dr.drawPath(dev, &scaled, paint, &m)
			continue
		}
		m := deviceMatrix(ctm.Translate(float64(px), float64(py)).Scale(float64(scale), float64(scale)))
		p.cv.dr.drawPath(dev, &path, paint, &m)
	}
}

// spriteBlitter draws premultiplied color glyph images (emoji) with
// source-over.
type spriteBlitter struct {
	dst   *pixmap
	alpha uint8
}

func (s *spriteBlitter) blit(src []uint8, bounds, clip IRect, aa *aaClip) {
	w := bounds.width()
	for y := clip.T; y < clip.B; y++ {
		so := ((y-bounds.T)*w + (clip.L - bounds.L)) * 4
		row := src[so : so+clip.width()*4]
		do := y*s.dst.stride + clip.L*4
		d := s.dst.pix[do : do+clip.width()*4]
		if aa != nil {
			for x := clip.L; x < clip.R; x++ {
				cv := aa.at(x, y)
				if cv == 0 {
					continue
				}
				i := (x - clip.L) * 4
				a := uint32(mulDiv255Round(uint32(s.alpha), uint32(cv)))
				if a == 255 {
					srcOverRow(d[i:i+4], row[i:i+4])
				} else {
					srcOverRowAlpha(d[i:i+4], row[i:i+4], a)
				}
			}
			continue
		}
		if s.alpha == 255 {
			srcOverRow(d, row)
		} else {
			srcOverRowAlpha(d, row, uint32(s.alpha))
		}
	}
}

// paintText paints an SVG text element (the text painters of Blink with
// SVG paints).
func (p *painter) paintText(n *dom.Node, st *svgStyle, ctm Affine) {
	frags := p.eng.RasterSVGText(n)
	if len(frags) == 0 {
		return
	}
	styles := map[*dom.Node]*svgStyle{n: st}
	for i := range frags {
		f := &frags[i]
		fst := p.styleOfTextOwner(f.Owner, n, st, styles)
		if !fst.visible {
			continue
		}
		x, y := p.svgTextOrigin(n, f)
		run := glyphRun{glyphs: f.Glyphs, x: x, y: y, fontSize: float32(f.FontSize)}
		fctm := ctm
		if f.Scale != 1 && f.Scale > 0 {
			// The fragment is in the scaled space of the screen font.
			inv := 1 / float64(f.Scale)
			fctm = ctm.Scale(inv, inv)
			// The stroke is made in that space too: PrepareStrokeGeometry
			// of core/paint/text_painter.cc multiplies its width and its
			// dashes by the scale.
			if fst.strokeWidth > 0 {
				scaled := *fst
				scaled.strokeWidth = float32(fst.strokeWidth * f.Scale)
				scaled.dashOffset = float32(fst.dashOffset * f.Scale)
				if len(fst.dashArray) > 0 {
					scaled.dashArray = make([]float32, len(fst.dashArray))
					for k, d := range fst.dashArray {
						scaled.dashArray[k] = float32(d * f.Scale)
					}
				}
				fst = &scaled
			}
		}
		p.paintTextRun(f.Owner, &run, fst, fctm, f.Width, f.Ascent, f.Descent)
	}
}

// svgTextOrigin returns the origin of the glyphs of a fragment of an SVG
// text element: the start of the alphabetic baseline, as
// ComputePhysicalBoxGeometry and TextFragmentPainter::Paint of
// core/paint/text_fragment_painter.cc compute it from the layout.
//
// The box of the fragment is the rectangle of the layout in LayoutUnits
// (1/64 px), which truncates its left and its top. The baseline is the top
// plus the whole ascent of the font.
//
// The painter also moves the box by the amount that rounds the top of the
// parent of the fragment to a whole pixel. For text directly in the text
// element the parent is the line, whose top is a whole number
// (FragmentItem::SetSvgLineLocalRect). For text in a tspan the parent is
// the inline box of the tspan (InlineBoxFragmentPainter). The inline layout
// puts that box where its dominant baseline is at 0, so its top is minus
// the ascent of the baseline type, which has a fraction for every type but
// the alphabetic one. So a line in a tspan with dominant-baseline: central
// is half a pixel lower than the same line without the tspan when the font
// height is odd, and one with dominant-baseline: middle is higher by the
// fraction of half the x-height.
//
// All values are in the space of the fragment, which is the space of the
// scaled font.
func (p *painter) svgTextOrigin(text *dom.Node, f *browser.RasterTextFragment) (x, y float32) {
	m := font.Metrics{Ascent: f.Ascent, Descent: f.Descent}
	var face *font.Face
	if f.Owner != nil {
		if _, pf, ok := p.eng.RasterPrimaryMetrics(f.Owner); ok && pf != nil {
			face = pf
			m.XHeight = face.Metrics(f.FontSize).XHeight
		}
	}
	left := luTrunc(f.X)
	// SvgFragmentData::rect: the top is the baseline minus the ascent.
	rectY := f.Baseline - float32(m.Ascent)
	if p.borrowsAscent(face, f.FontSize) {
		// The layout has the metrics of device scale factor 1 (see
		// borrowsAscent). The top of the box is the position of the
		// dominant baseline minus the ascent of its type, which changes
		// with the metrics.
		dominant := p.dominantBaseline(f.Owner)
		typ := p.alignmentBaseline(f.Owner, dominant)
		before := fixedAscent(typ, m)
		m.Ascent, m.Descent = m.Ascent-1, m.Descent+1
		rectY = rectY + float32(float32(before)/layoutUnit) - float32(float32(fixedAscent(typ, m))/layoutUnit)
	}
	top := luTrunc(rectY)
	if f.Owner != text && f.Owner != nil {
		// AddBoxFragmentPlaceholder: the top of the inline box, with the
		// baseline type of the text element.
		boxTop := -fixedAscent(p.dominantBaseline(text), m)
		// ComputeAlignmentBaselineShift: an alignment-baseline moves the
		// box.
		dominant := p.dominantBaseline(f.Owner)
		boxTop += fixedAscent(dominant, m) - fixedAscent(p.alignmentBaseline(f.Owner, dominant), m)
		// LayoutUnit::Round is floor(v + 1/2).
		rounded := (boxTop + layoutUnit/2) >> 6 << 6
		top += rounded - boxTop
	}
	return float32(left) / layoutUnit, float32(float32(top)/layoutUnit) + float32(m.Ascent)
}

// borrowsAscent reports whether Blink paints the text of a face at a font
// size with an ascent that is one pixel smaller, and a descent that is one
// pixel larger, than the layout engine has.
//
// FontMetrics::AscentDescentWithHacks of Blink rounds the ascent and the
// descent of a font to whole pixels. On Linux, with subpixel positioning,
// a descent that was rounded down takes one pixel from the ascent, so that
// the bottom of the glyphs stays inside the box. Chromium on Linux turns
// subpixel positioning on when the device scale factor is above 1
// (gfx::GetFontRenderParams), and the emulation of the scale factor in the
// screenshots keeps the layout in CSS pixels. So at a scale above 1 the
// same document has other font metrics.
//
// The layout engine (internal/browser with internal/font) has the metrics
// of scale factor 1. The painter corrects the positions that it can derive:
// the baseline of SVG text (svgTextOrigin) and of HTML lines
// (htmlLineShift). The heights of line boxes with mixed fonts are not
// corrected.
func (p *painter) borrowsAscent(face *font.Face, fontSize float64) bool {
	if !(p.scale > 1) || face == nil || face.Platform() == font.PlatformDarwin {
		return false
	}
	m := face.Metrics(fontSize)
	return m.Descent < m.FloatDescent && m.Ascent >= 1
}

// htmlLineShift returns the number of pixels by which the baseline of an
// HTML line moves up at a device scale factor above 1 (borrowsAscent). The
// baseline of a line is the largest ascent of its inline boxes with their
// half-leading, and the font of the block counts as a box. When all of them
// borrow from the ascent, or none does, the result is exact. Otherwise the
// largest font decides.
func (p *painter) htmlLineShift(block *dom.Node, line *browser.RasterHTMLLine) float64 {
	if !(p.scale > 1) {
		return 0
	}
	shift := 0.0
	var size float64
	if _, face, ok := p.eng.RasterPrimaryMetrics(block); ok {
		size = p.eng.RasterTextStyleOf(block).FontSize
		if p.borrowsAscent(face, size) {
			shift = 1
		}
	}
	for i := range line.Fragments {
		f := &line.Fragments[i]
		if f.FontSize <= size {
			continue
		}
		if _, face, ok := p.eng.RasterPrimaryMetrics(f.Owner); ok {
			size, shift = f.FontSize, 0
			if p.borrowsAscent(face, f.FontSize) {
				shift = 1
			}
		}
	}
	return shift
}

// dominantBaseline returns the computed dominant-baseline of an element of
// an SVG text as a baseline type of Blink (ComputedStyle::GetFontBaseline).
// The property is inherited. auto and the keywords that Blink does not
// implement are the alphabetic baseline.
func (p *painter) dominantBaseline(n *dom.Node) string {
	for cur := n; cur != nil && cur.Type == dom.ElementNode; cur = cur.Parent {
		v, ok := p.eng.RasterDeclared(cur, "dominant-baseline")
		if !ok {
			continue
		}
		switch v = strings.ToLower(strings.TrimSpace(v)); v {
		case "central", "middle", "mathematical", "hanging", "text-before-edge", "text-after-edge", "ideographic":
			return v
		case "auto", "alphabetic", "use-script", "no-change", "reset-size":
			return "alphabetic"
		}
	}
	return "alphabetic"
}

// alignmentBaseline returns the baseline type of the alignment-baseline of
// an element (InlineBoxState::ResetStyle). The property is not inherited,
// and auto and baseline are the dominant baseline.
func (p *painter) alignmentBaseline(n *dom.Node, dominant string) string {
	v, ok := p.eng.RasterDeclared(n, "alignment-baseline")
	if !ok {
		return dominant
	}
	switch v = strings.ToLower(strings.TrimSpace(v)); v {
	case "alphabetic", "central", "middle", "mathematical", "hanging", "text-before-edge", "text-after-edge", "ideographic":
		return v
	case "before-edge":
		return "text-before-edge"
	case "after-edge":
		return "text-after-edge"
	}
	return dominant
}

// fixedAscent is FontMetrics::FixedAscent for a baseline type: the distance
// from the top of the text box to the baseline, as a LayoutUnit raw value.
// The ascent and the descent of the metrics are the whole numbers of Blink.
func fixedAscent(baseline string, m font.Metrics) int64 {
	asc, desc := float32(m.Ascent), float32(m.Descent)
	var a float32
	switch baseline {
	case "central":
		a = (asc + desc) / 2
	case "middle":
		a = asc - float32(float32(m.XHeight)/2)
	case "mathematical":
		a = float32(asc * 0.5)
	case "hanging":
		a = float32(asc * 0.2)
	case "text-before-edge":
		a = 0
	case "text-after-edge", "ideographic":
		a = asc + desc
	default:
		a = asc
	}
	// LayoutUnit::FromFloatRound.
	return int64(math.Round(float64(float32(a * layoutUnit))))
}

// styleOfTextOwner resolves the inherited style of a tspan below a text
// element.
func (p *painter) styleOfTextOwner(owner, text *dom.Node, textStyle *svgStyle, cache map[*dom.Node]*svgStyle) *svgStyle {
	if st, ok := cache[owner]; ok {
		return st
	}
	var chain []*dom.Node
	for a := owner; a != nil && a != text; a = a.Parent {
		chain = append(chain, a)
	}
	st := *textStyle
	for i := len(chain) - 1; i >= 0; i-- {
		p.resolveStyle(chain[i], &st)
		cp := st
		cache[chain[i]] = &cp
	}
	return cache[owner]
}

// paintTextRun paints the fill, the stroke and the decorations of a run
// of SVG text.
func (p *painter) paintTextRun(owner *dom.Node, run *glyphRun, st *svgStyle, ctm Affine, width float32, ascent, descent float64) {
	for _, order := range st.paintOrder {
		switch order {
		case orderFill:
			var paint Paint
			if !p.preparePaint(owner, st, false, &paint, ctm) {
				continue
			}
			paint.antiAlias = true
			p.paintDecorations(owner, run, &paint, ctm, width, ascent, false)
			p.drawGlyphRun(run, &paint, ctm)
		case orderStroke:
			if !(st.strokeWidth > 0) {
				continue
			}
			var paint Paint
			if !p.preparePaint(owner, st, true, &paint, ctm) {
				continue
			}
			paint.antiAlias = true
			p.setStroke(&paint, st)
			p.drawGlyphRunAsPaths(run, &paint, ctm)
		}
	}
}

// textDecoration is the text-decoration of one element.
type textDecoration struct {
	// node is the element that declares the decoration.
	node                         *dom.Node
	underline, overline, through bool
}

// textDecorations returns the text decorations that apply to an element,
// from the outermost one: text-decoration propagates to descendants.
func (p *painter) textDecorations(n *dom.Node) []textDecoration {
	var out []textDecoration
	for a := n; a != nil && a.Type == dom.ElementNode; a = a.Parent {
		d := textDecoration{node: a}
		v, ok := p.eng.RasterDeclared(a, "text-decoration")
		if !ok {
			v, ok = p.eng.RasterDeclared(a, "text-decoration-line")
		}
		if ok {
			lower := strings.ToLower(v)
			d.underline = strings.Contains(lower, "underline")
			d.overline = strings.Contains(lower, "overline")
			d.through = strings.Contains(lower, "line-through")
		}
		if a.NS == dom.NSHTML {
			switch a.Tag {
			case "u", "ins":
				d.underline = true
			case "s", "strike", "del":
				d.through = true
			}
		}
		if d.underline || d.overline || d.through {
			out = append(out, d)
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// paintDecorations draws the underline, overline and line-through of a
// run as filled rectangles (TextDecorationInfo and DecorationLinePainter
// of Blink).
//
// The thickness is one tenth of the font size. The decorations of SVG text
// have the paint of the text and are not snapped. Those of HTML text are
// at least one pixel thick, their top is rounded to a whole pixel and
// their thickness is rounded down (SnapYAxis), both in the space of the
// content, and their color is the color of the element that declares the
// decoration.
func (p *painter) paintDecorations(owner *dom.Node, run *glyphRun, paint *Paint, ctm Affine, width float32, ascent float64, html bool) {
	decorations := p.textDecorations(owner)
	if len(decorations) == 0 {
		return
	}
	thickness := run.fontSize / 10
	if html {
		thickness = max(1, thickness)
	}
	m := deviceMatrix(ctm)
	dev := p.cv.dev()
	top := run.y - float32(ascent)
	for _, d := range decorations {
		linePaint := paint
		var own Paint
		if html {
			c := p.decorationColor(d.node)
			own = Paint{R: c.R, G: c.G, B: c.B, A: cssAlphaByte(c, 1), antiAlias: true}
			if own.A == 0 {
				continue
			}
			linePaint = &own
		}
		line := func(y float32) {
			h := thickness
			if html {
				y = floor32(y + 0.5)
				h = max(floor32(thickness), 1)
			}
			p.cv.dr.drawRect(dev, Rect{run.x, y, run.x + width, y + h}, linePaint, &m)
		}
		if d.underline {
			// ComputeUnderlineOffsetAuto: a gap of max(1, ceil(thickness
			// / 2)) below the baseline.
			gap := max(1, float32(math.Ceil(float64(thickness/2))))
			line(run.y + gap)
		}
		if d.overline {
			// ComputeUnderlineOffsetForUnder for the top of the text: the
			// line grows upwards.
			line(top - floor32(thickness))
		}
		if d.through {
			line(top + float32(2*float32(ascent)/3) - float32(thickness/2))
		}
	}
}

// decorationColor returns the color of the decorations that an element
// declares: text-decoration-color, which is the color of the element by
// default.
func (p *painter) decorationColor(n *dom.Node) css.Color {
	if v, ok := p.eng.RasterDeclared(n, "text-decoration-color"); ok && !strings.EqualFold(strings.TrimSpace(v), "currentcolor") {
		if c, ok := parseColor(v); ok {
			return c
		}
	}
	return p.htmlColor(n)
}

// ---- foreignObject ----

// paintForeignObject paints the HTML content of a foreignObject element.
func (p *painter) paintForeignObject(fo *dom.Node, st *svgStyle, ctm Affine) {
	w := p.attrLength(fo, "width", lengthWidth)
	h := p.attrLength(fo, "height", lengthHeight)
	if !(w > 0 && h > 0) {
		return
	}
	x := p.attrLength(fo, "x", lengthWidth)
	y := p.attrLength(fo, "y", lengthHeight)
	boxes := p.eng.RasterForeignObject(fo)
	if len(boxes) == 0 {
		return
	}
	// The paint offset of the content is the location of the element.
	off := ctm.Translate(float64(x), float64(y))
	dev := p.cv.dev()
	savedClip, savedAA := dev.clip, dev.aa
	defer func() { dev.clip, dev.aa = savedClip, savedAA }()
	// The foreignObject clips its content unless overflow is visible. The
	// clip is the box of the element, snapped to whole pixels in its own
	// space (ToSnappedClipRect in
	// FragmentPaintPropertyTreeBuilder::UpdateOverflowClip), with
	// anti-aliasing: under a translation by a fraction of a pixel the edge
	// pixels of the content are partly covered.
	if p.foreignObjectClips(fo) {
		lu := func(v float32) float64 { return float64(float64(luTrunc(v)) / layoutUnit) }
		if !p.clipRectAA(dev, snappedRect(lu(x), lu(y), lu(w), lu(h)), ctm) {
			return
		}
	}
	for _, b := range boxes {
		p.foY = float64(y)
		p.paintHTMLBox(b, off, 1)
	}
}

// foreignObjectClips reports whether a foreignObject clips its content:
// the user agent style sheet sets overflow: hidden.
func (p *painter) foreignObjectClips(fo *dom.Node) bool {
	v, ok := p.eng.RasterDeclared(fo, "overflow")
	if !ok {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "visible", "auto":
		return false
	}
	return true
}

func luRound(v float64) float64 { return math.Floor(v + 0.5) }

// snappedRect is ToPixelSnappedRect for a box in the coordinate system of
// the foreignObject content.
func snappedRect(x, y, w, h float64) Rect {
	l := luRound(x)
	t := luRound(y)
	return Rect{float32(l), float32(t), float32(luRound(x + w)), float32(luRound(y + h))}
}

// htmlColor returns the value of the inherited color property of an HTML
// element.
func (p *painter) htmlColor(n *dom.Node) css.Color {
	if v := p.eng.RasterLookup(n, "color"); v != "" {
		if c, ok := parseColor(v); ok {
			return c
		}
	}
	return css.Color{A: 1}
}

func (p *painter) htmlVisible(n *dom.Node) bool {
	v := p.eng.RasterLookup(n, "visibility")
	return v == "" || strings.EqualFold(strings.TrimSpace(v), "visible")
}

func (p *painter) htmlOpacity(n *dom.Node) float32 {
	if v, ok := p.eng.RasterDeclared(n, "opacity"); ok {
		if f, ok := parseOpacity(v); ok {
			return f
		}
	}
	return 1
}

// backgroundColor returns the background color of an element.
func (p *painter) backgroundColor(n *dom.Node) (css.Color, bool) {
	v, ok := p.eng.RasterDeclared(n, "background-color")
	if !ok {
		return css.Color{}, false
	}
	if strings.EqualFold(strings.TrimSpace(v), "currentcolor") {
		return p.htmlColor(n), true
	}
	c, ok := parseColor(v)
	if !ok || c.A <= 0 {
		return css.Color{}, false
	}
	return c, true
}

// paintHTMLBox paints a block box: its background, its lines and its
// children.
func (p *painter) paintHTMLBox(b *browser.RasterHTMLBox, ctm Affine, opacity float32) {
	n := b.Node
	if p.eng.RasterDisplay(n) == "none" {
		return
	}
	if o := p.htmlOpacity(n); o < 1 {
		p.opacityLayer()
		defer p.cv.popLayer(o, blendSrcOver)
	}
	m := deviceMatrix(ctm)
	dev := p.cv.dev()
	if p.htmlVisible(n) {
		if c, ok := p.backgroundColor(n); ok {
			paint := Paint{R: c.R, G: c.G, B: c.B, A: cssAlphaByte(c, 1), antiAlias: true}
			p.cv.dr.drawRect(dev, snappedRect(b.X, b.Y, b.W, b.H), &paint, &m)
		}
		p.paintBorders(n, b, ctm)
	}
	// Inline backgrounds, then text.
	for li := range b.Lines {
		p.paintInlineBackgrounds(n, &b.Lines[li], ctm)
	}
	for li := range b.Lines {
		line := &b.Lines[li]
		// ComputePhysicalBoxGeometry of TextFragmentPainter moves the text
		// by the amount that rounds the top of its line to a whole pixel:
		// "Round the physical y-axis to ensure the actual text glyphs are
		// drawn at a whole-pixel y-offset". Each line rounds its own top.
		top := p.foY + line.Y
		snap := math.Floor(top+0.5) - top
		shift := p.htmlLineShift(n, line)
		for fi := range line.Fragments {
			f := &line.Fragments[fi]
			if !p.htmlVisible(f.Owner) {
				continue
			}
			c := p.htmlColor(f.Owner)
			paint := Paint{R: c.R, G: c.G, B: c.B, A: cssAlphaByte(c, 1), antiAlias: true}
			if paint.A == 0 {
				continue
			}
			run := glyphRun{glyphs: f.Glyphs, x: float32(f.X), y: float32(line.Y + snap + line.Baseline - shift), fontSize: float32(f.FontSize)}
			p.paintDecorations(f.Owner, &run, &paint, ctm, float32(f.Width), f.Ascent-shift, true)
			p.drawGlyphRun(&run, &paint, ctm)
		}
	}
	for _, c := range b.Children {
		p.paintHTMLBox(c, ctm, opacity)
	}
}

// paintInlineBackgrounds paints the backgrounds of the inline elements of
// a line: one rectangle per element, over the fragments of its
// descendants, with the height of its font.
func (p *painter) paintInlineBackgrounds(block *dom.Node, line *browser.RasterHTMLLine, ctm Affine) {
	type span struct {
		x0, x1 float64
		set    bool
	}
	var order []*dom.Node
	spans := map[*dom.Node]*span{}
	for fi := range line.Fragments {
		f := &line.Fragments[fi]
		for a := f.Owner; a != nil && a != block; a = a.Parent {
			if _, ok := p.backgroundColor(a); !ok {
				continue
			}
			s := spans[a]
			if s == nil {
				s = &span{}
				spans[a] = s
				order = append(order, a)
			}
			if !s.set {
				s.x0, s.x1, s.set = f.X, f.X+f.Width, true
			} else {
				s.x0, s.x1 = min(s.x0, f.X), max(s.x1, f.X+f.Width)
			}
		}
	}
	if len(order) == 0 {
		return
	}
	m := deviceMatrix(ctm)
	dev := p.cv.dev()
	// Outer elements paint first.
	for i := len(order) - 1; i >= 0; i-- {
		el := order[i]
		s := spans[el]
		c, _ := p.backgroundColor(el)
		metrics, _, ok := p.eng.RasterPrimaryMetrics(el)
		if !ok || !p.htmlVisible(el) {
			continue
		}
		top := line.Y + line.Baseline - metrics.Ascent
		paint := Paint{R: c.R, G: c.G, B: c.B, A: cssAlphaByte(c, 1), antiAlias: true}
		p.cv.dr.drawRect(dev, snappedRect(s.x0, top, s.x1-s.x0, metrics.Ascent+metrics.Descent), &paint, &m)
	}
}

// paintBorders paints the borders of a block box as filled rectangles.
func (p *painter) paintBorders(n *dom.Node, b *browser.RasterHTMLBox, ctm Affine) {
	type side struct {
		width float64
		color css.Color
	}
	parse := func(v string) (side, bool) {
		var s side
		style := ""
		s.color = p.htmlColor(n)
		s.width = 3
		for _, part := range splitCSSFields(v) {
			lower := strings.ToLower(part)
			switch lower {
			case "none", "hidden":
				return side{}, false
			case "solid", "dashed", "dotted", "double", "groove", "ridge", "inset", "outset":
				style = lower
				continue
			case "thin":
				s.width = 1
				continue
			case "medium":
				s.width = 3
				continue
			case "thick":
				s.width = 5
				continue
			}
			if px, ok := p.eng.RasterLength(n, part, false); ok {
				s.width = math.Floor(px)
				continue
			}
			if c, ok := parseColor(part); ok {
				s.color = c
			}
		}
		if style == "" || s.width <= 0 {
			return side{}, false
		}
		return s, true
	}
	var sides [4]side
	var has [4]bool
	if v, ok := p.eng.RasterDeclared(n, "border"); ok {
		if s, ok := parse(v); ok {
			for i := range sides {
				sides[i], has[i] = s, true
			}
		}
	}
	for i, name := range []string{"border-top", "border-right", "border-bottom", "border-left"} {
		if v, ok := p.eng.RasterDeclared(n, name); ok {
			sides[i], has[i] = parse(v)
		}
	}
	if !has[0] && !has[1] && !has[2] && !has[3] {
		return
	}
	m := deviceMatrix(ctm)
	dev := p.cv.dev()
	r := snappedRect(b.X, b.Y, b.W, b.H)
	draw := func(rc Rect, s side) {
		paint := Paint{R: s.color.R, G: s.color.G, B: s.color.B, A: cssAlphaByte(s.color, 1), antiAlias: true}
		p.cv.dr.drawRect(dev, rc, &paint, &m)
	}
	if has[0] {
		draw(Rect{r.L, r.T, r.R, r.T + float32(sides[0].width)}, sides[0])
	}
	if has[2] {
		draw(Rect{r.L, r.B - float32(sides[2].width), r.R, r.B}, sides[2])
	}
	if has[3] {
		draw(Rect{r.L, r.T, r.L + float32(sides[3].width), r.B}, sides[3])
	}
	if has[1] {
		draw(Rect{r.R - float32(sides[1].width), r.T, r.R, r.B}, sides[1])
	}
}

// splitCSSFields splits a value at white space outside of parentheses.
func splitCSSFields(v string) []string {
	var out []string
	depth, start := 0, -1
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
		}
		if (c == ' ' || c == '\t' || c == '\n') && depth == 0 {
			if start >= 0 {
				out = append(out, v[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, v[start:])
	}
	return out
}

// mergeLCD16 is mergeT<uint16_t> of skia/src/core/SkAAClip.cpp for the
// part of an LCD mask inside clip: each channel of the mask times the
// coverage of the anti-aliased clip. The result has the bounds of clip. dst
// is reused when it is large enough.
func mergeLCD16(dst, lcd []uint16, bounds, clip IRect, aa *aaClip) []uint16 {
	w, h := clip.width(), clip.height()
	if cap(dst) < w*h {
		dst = make([]uint16, w*h)
	}
	dst = dst[:w*h]
	sw := bounds.width()
	for y := clip.T; y < clip.B; y++ {
		src := lcd[(y-bounds.T)*sw+(clip.L-bounds.L):]
		crow := aa.rowAt(y)[clip.L-aa.bounds.L:]
		out := dst[(y-clip.T)*w : (y-clip.T+1)*w]
		for i := range out {
			switch cv := uint32(crow[i]); cv {
			case 0xFF:
				out[i] = src[i]
			case 0:
				out[i] = 0
			default:
				v := uint32(src[i])
				r := mulDiv255Round(v>>11, cv)
				g := mulDiv255Round(v>>5&63, cv)
				b := mulDiv255Round(v&31, cv)
				out[i] = uint16(r<<11 | g<<5 | b)
			}
		}
	}
	return dst
}

// blitLCD16 blends a color through an LCD mask: blitmask_lcd16 with
// blend_lcd16 and blend_lcd16_opaque of skia/src/core/SkBlitter_ARGB32.cpp.
// The color is not premultiplied: its alpha scales the mask.
func blitLCD16(dst *pixmap, lcd []uint16, bounds, clip IRect, paint *Paint) {
	w := bounds.width()
	srcR, srcG, srcB := int(paint.R), int(paint.G), int(paint.B)
	opaque := paint.A == 255
	// SkAlpha255To256.
	srcA := int(paint.A) + (int(paint.A) >> 7)
	blend := func(src, d, scale int) uint8 { return uint8(d + ((src-d)*scale)>>5) }
	for y := clip.T; y < clip.B; y++ {
		row := lcd[(y-bounds.T)*w+(clip.L-bounds.L):]
		d := dst.pix[y*dst.stride+clip.L*4:]
		for x := 0; x < clip.width(); x++ {
			m := row[x]
			if m == 0 {
				continue
			}
			px := d[x*4 : x*4+4 : x*4+4]
			if opaque && m == 0xFFFF {
				px[0], px[1], px[2], px[3] = paint.R, paint.G, paint.B, 255
				continue
			}
			// Five bits per channel, then 0 to 32 (upscale_31_to_32).
			mr, mg, mb := int(m>>11), int(m>>6)&31, int(m)&31
			mr, mg, mb = mr+mr>>4, mg+mg>>4, mb+mb>>4
			dstA := int(px[3])
			var ma int
			if opaque {
				ma = max(mr, mg, mb)
			} else {
				mr, mg, mb = mr*srcA>>8, mg*srcA>>8, mb*srcA>>8
				if srcA-1 < dstA {
					ma = min(mr, mg, mb)
				} else {
					ma = max(mr, mg, mb)
				}
			}
			px[0] = blend(srcR, int(px[0]), mr)
			px[1] = blend(srcG, int(px[1]), mg)
			px[2] = blend(srcB, int(px[2]), mb)
			px[3] = blend(0xFF, dstA, ma)
		}
	}
}
