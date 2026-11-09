// Ported from Skia at revision aa789d9b, the revision of Chromium
// 156.0.8078.80 (src/ports/SkTypeface_fontations.cpp:
// SkFontationsScalerContext::generateMetrics, generatePathImpl and
// generateFontMetrics; src/core/SkScalerContext.cpp:
// SkScalerContextRec::computeMatrices, SkScalerContextRec::useStrokeForFakeBold,
// SkScalerContext::internalGetPath and GenerateMetricsFromPath;
// src/core/SkMatrix.cpp: SkMatrix::setConcat, preScale and Affine_vpts;
// src/core/SkFont.cpp: SkFont::setupForAsPaths and getWidthsBounds; BSD
// 3-clause license, LICENSE-Skia.txt) and from skrifa 0.46.2 with read-fonts
// 0.43.2 (src/outline/cff/mod.rs, src/ps/transform.rs:
// Transform::transform_h_metric; src/metrics.rs: Metrics::new; MIT OR
// Apache-2.0, LICENSE-skrifa.txt).

package font

import (
	"math"

	gtfont "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
)

// This file gives the faces of web fonts what the scaler context of Skia
// computes for a glyph on Linux: the advance, the path and the bounds.
//
// A web font has normal hinting (see webfonts_face.go). For a font with
// TrueType outlines Skia makes a hinting instance of skrifa that runs the
// bytecode of the font when the font has a control value program or a font
// program, and the automatic hinter in all other cases. The fonts of the
// page all have a control value program.
//
// The glyphs of Recursive have no bytecode, so the hinted outline is the
// scaled outline, and the scaler here (glyf_var.go) is exact: the tests of
// glyf_var_test.go compare it with skrifa.
//
// The glyphs of Open Sans have bytecode, which this package does not run.
// The scaler gives the outline before the bytecode, at the size of the
// hinted mode. Against skrifa (tools/reference/skrifa-oracle), the advance,
// the left side bearing and the horizontal bounds in whole pixels of every
// glyph of the two Latin files of Open Sans are exact at 13 font sizes, and
// the vertical bounds are not: they are one pixel off for many glyphs. The
// control value program of Open Sans also turns hinting off below 9 pixels
// per em, which this package does not know: a font size below 8.5px that is
// not a whole number has the advance of the size rounded to a whole number
// here, and of the size itself in Chromium.

// webDrawKey names the draw of a glyph at a font size.
type webDrawKey struct {
	gid    uint32
	size   uint32 // the bits of the font size as a float32
	hinted bool
}

// webGlyf returns the reader of the TrueType outlines of a web face, or
// nil for a font with PostScript outlines.
func (f *Face) webGlyf() *ahFont {
	w := f.web
	w.glyfOnce.Do(func() {
		if f.ft != nil && w.font.glyf {
			w.glyf = newAHFont(f.ft.ld)
		}
	})
	return w.glyf
}

// webSkrifaCoords returns the normalized coordinates that skrifa computes
// for the values of the axes, or nil at the default instance.
func webSkrifaCoords(ld *ot.Loader, user map[uint32]float32) []int16 {
	return ahEffectiveCoords(ahNormalizedCoords(ld, user))
}

// webDraw is OutlineGlyph::draw of skrifa with the pen of Skia for a glyph
// of a web font: the path in px with y up, and the advance. hinted selects
// the hinting instance of normal hinting. ok is false for a glyph that
// skrifa does not draw.
func (f *Face) webDraw(gid uint32, size float32, hinted bool) (ahDraw, bool) {
	key := webDrawKey{gid, math.Float32bits(size), hinted}
	if v, ok := f.web.draws.Load(key); ok {
		d := v.(*webDrawn)
		return d.draw, d.ok
	}
	d := &webDrawn{}
	d.draw, d.ok = f.webDrawUncached(gid, size, hinted)
	f.web.draws.Store(key, d)
	return d.draw, d.ok
}

type webDrawn struct {
	draw ahDraw
	ok   bool
}

func (f *Face) webDrawUncached(gid uint32, size float32, hinted bool) (ahDraw, bool) {
	if g := f.webGlyf(); g != nil {
		o, ok := g.outline(gid)
		if !ok {
			return ahDraw{}, false
		}
		// The hinting instance is taken as enabled and in the backward
		// compatibility mode. That is the state after the control value
		// program of Recursive. Open Sans leaves that mode, which changes
		// the scaler only where its bytecode runs.
		return g.drawAt(o, size, f.web.skCoords, hinted, true, true)
	}
	// PostScript outlines. The path is the outline of the font times the
	// scale: the hinter of the PostScript interpreter is not ported, and
	// it moves points along the y axis only.
	var d ahDraw
	upem := int64(f.upem)
	if upem <= 0 {
		return d, false
	}
	if outline, ok := f.gt.GlyphDataOutline(gtfont.GID(gid)); ok {
		scale := size / float32(f.upem)
		d.segments = make([]ot.Segment, len(outline.Segments))
		for i, seg := range outline.Segments {
			d.segments[i].Op = seg.Op
			for j, p := range seg.Args {
				d.segments[i].Args[j] = ot.SegmentPoint{X: float32(p.X * scale), Y: float32(p.Y * scale)}
			}
		}
	}
	// Transform::transform_h_metric: the advance times the scale, in 26.6
	// px. The hinted draw rounds it to a whole pixel. The advance is the
	// advance of the hmtx table: the charstrings of the fonts of the page
	// have the same widths.
	scale := ftDivFix(int64(float32(size)*64), upem)
	adv26 := ftMulFix(int64(f.webNominal(gid)), scale)
	if hinted {
		adv26 = (adv26 + 32) &^ 63
	}
	if adv26 < 0 {
		return d, false
	}
	d.advance = float32(adv26) / 64
	return d, true
}

// webLinearUnits is the advance of a glyph in font units that
// GlyphMetrics::advance_width of skrifa scales: the advance of the hmtx
// table with the delta of the variation.
func (f *Face) webLinearUnits(gid uint32) int32 {
	if g := f.webGlyf(); g != nil {
		return g.advanceWidthAt(gid, f.web.skCoords)
	}
	return f.webNominal(gid)
}

// webHintedAdvance is the advance of the hinted glyph in px:
// AdjustedMetrics.advance_width of the hinted draw of skrifa, which is a
// whole number. ok is false for a glyph that skrifa does not draw.
func (f *Face) webHintedAdvance(gid uint32, size float64) (float32, bool) {
	d, ok := f.webDraw(gid, float32(size), true)
	if !ok {
		return 0, false
	}
	return d.advance, true
}

// webRemaining is the matrix that Skia applies to the outline of a glyph
// after the scaler made it at the font size: the remaining matrix of
// SkScalerContextRec::computeMatrices with the vertical scale taken out. A
// face without a synthetic italic has the identity. A synthetic italic is
// a skew of one quarter (FontPlatformData::CreateSkFont), and the matrix is
// then the text matrix times the inverse of the size, in float32: its
// entries are not exactly 1 and -1/4 for every size.
type webRemaining struct {
	scaleX, skewX, scaleY float32
	identity              bool
}

func (f *Face) webRemainingMatrix(size float32) webRemaining {
	if !f.web.syntheticItalic {
		return webRemaining{scaleX: 1, scaleY: 1, identity: true}
	}
	// SkFontPriv::MakeTextMatrix: a scale by the size, then a skew. The
	// product goes through float64 (muladdmul of SkMatrix::setConcat).
	const skew = -0.25
	aScaleX := float32(float64(size))
	aSkewX := float32(float64(skew) * float64(size))
	aScaleY := float32(float64(size))
	// computeMatrices: the Givens rotation of a matrix without rotation is
	// the identity, the scale is the vertical scale, and the remaining
	// matrix is the total matrix times the inverse scale (preScale).
	inv := float32(1) / aScaleY
	return webRemaining{
		scaleX: float32(aScaleX * inv),
		skewX:  float32(aSkewX * inv),
		scaleY: float32(aScaleY * inv),
	}
}

// apply is SkMatrix::Affine_vpts for a point of a path of Skia, which has
// y down. x and y are a point with y up, and so is the result.
func (m webRemaining) apply(x, y float32) (float32, float32) {
	if m.identity {
		return x, y
	}
	yDown := -y
	outX := float32(x*m.scaleX) + float32(yDown*m.skewX)
	outY := float32(yDown*m.scaleY) + float32(x*0)
	return outX, -outY
}

// webPath is the path of a glyph of a web font as the scaler context of
// Skia has it (generatePathImpl): the outline at the font size, with the
// hinting of the face or without hinting, through the remaining matrix. The
// path is in px with y up. ok is false for a glyph without a path.
//
// A face with a synthetic italic is not hinted (onFilterRec of the
// Fontations typeface). A synthetic bold is not in this path: see
// webStrokedPath.
func (f *Face) webPath(gid uint32, size float32, hinted bool) ([]ot.Segment, bool) {
	d, ok := f.webDraw(gid, size, hinted && !f.web.syntheticItalic)
	if !ok || len(d.segments) == 0 {
		return nil, false
	}
	m := f.webRemainingMatrix(size)
	if m.identity {
		return d.segments, true
	}
	out := make([]ot.Segment, len(d.segments))
	for i, seg := range d.segments {
		out[i].Op = seg.Op
		for j, p := range seg.Args {
			x, y := m.apply(p.X, p.Y)
			out[i].Args[j] = ot.SegmentPoint{X: x, Y: y}
		}
	}
	return out, true
}

// segmentArgs is the number of points of a path segment.
func segmentArgs(op ot.SegmentOp) int {
	switch op {
	case ot.SegmentOpQuadTo:
		return 2
	case ot.SegmentOpCubeTo:
		return 3
	}
	return 1
}

// fakeBoldWidth is the width of the stroke that Skia adds to the outline
// of a glyph for a synthetic bold (SkScalerContextRec::useStrokeForFakeBold
// with SkFloatInterpFunc): 1/24 of the font size up to 9px, 1/32 from 36px,
// and a value between them in between.
func fakeBoldWidth(size float32) float32 {
	const (
		key0, key1     = float32(9), float32(36)
		value0, value1 = float32(1) / 24, float32(1) / 32
	)
	scale := value1
	switch {
	case !(size > key0):
		scale = value0
	case size < key1:
		// SkFloatInterpFunc: the position between the keys, then the
		// value on the line.
		t := (size - key0) / (key1 - key0)
		scale = value0 + float32(t*(value1-value0))
	}
	return float32(size * scale)
}

// webFinalPath is the path that Skia fills for a glyph of a web font
// (SkScalerContext::internalGetPath): the path of the scaler, with the
// stroke of a synthetic bold. stroked is false when the face has a
// synthetic bold and the program has no stroker (see RegisterGlyphStroker):
// the path is then the path without the stroke.
func (f *Face) webFinalPath(gid uint32, size float32, hinted bool) (segments []ot.Segment, stroked, ok bool) {
	segments, ok = f.webPath(gid, size, hinted)
	if !ok {
		return nil, true, false
	}
	if !f.web.syntheticBold {
		return segments, true, true
	}
	stroker := glyphStroker.Load()
	if stroker == nil {
		return segments, false, true
	}
	return (*stroker)(segments, fakeBoldWidth(size)), true, true
}

// webScalerBounds is the rectangle of a glyph of a web font that Skia
// gives Blink (SkFont::getWidthsBounds): the bounds of the points of the
// path of the glyph, rounded out to whole pixels
// (GenerateMetricsFromPath), with y down. Above 256px the glyph is
// measured at 64px without hinting and the rectangle is scaled.
func (f *Face) webScalerBounds(gid uint32, size float64) (left, top, right, bottom float32, empty bool) {
	size32 := float32(size)
	if size32 > canonicalLimit {
		l, t, r, b, empty := f.webPixelBounds(gid, canonicalSize, false)
		if empty {
			return 0, 0, 0, 0, true
		}
		s := size32 / canonicalSize
		return float32(l * s), float32(t * s), float32(r * s), float32(b * s), false
	}
	return f.webPixelBounds(gid, size32, true)
}

func (f *Face) webPixelBounds(gid uint32, size float32, hinted bool) (left, top, right, bottom float32, empty bool) {
	key := webDrawKey{gid, math.Float32bits(size), hinted}
	if v, ok := f.web.bounds.Load(key); ok {
		b := v.([5]float32)
		return b[0], b[1], b[2], b[3], b[4] != 0
	}
	l, t, r, b, empty := f.webPixelBoundsUncached(gid, size, hinted)
	rec := [5]float32{l, t, r, b, 0}
	if empty {
		rec[4] = 1
	}
	f.web.bounds.Store(key, rec)
	return l, t, r, b, empty
}

func (f *Face) webPixelBoundsUncached(gid uint32, size float32, hinted bool) (left, top, right, bottom float32, empty bool) {
	segments, stroked, ok := f.webFinalPath(gid, size, hinted)
	if !ok {
		return 0, 0, 0, 0, true
	}
	// SkPath::getBounds: the bounds of all points, with the control
	// points.
	first := true
	var minX, minY, maxX, maxY float32
	for _, seg := range segments {
		for _, p := range seg.Args[:segmentArgs(seg.Op)] {
			if first {
				minX, maxX, minY, maxY, first = p.X, p.X, p.Y, p.Y, false
				continue
			}
			minX, maxX = min(minX, p.X), max(maxX, p.X)
			minY, maxY = min(minY, p.Y), max(maxY, p.Y)
		}
	}
	if first {
		return 0, 0, 0, 0, true
	}
	if !stroked {
		// No stroker: the bounds of the outline, larger by half of the
		// width of the stroke on each side. This is not the path of
		// Skia.
		half := float32(fakeBoldWidth(size) / 2)
		minX, maxX, minY, maxY = minX-half, maxX+half, minY-half, maxY+half
	}
	// SkRect::roundOut, with y down.
	left = float32(math.Floor(float64(minX)))
	right = float32(math.Ceil(float64(maxX)))
	top = float32(math.Floor(float64(-maxY)))
	bottom = float32(math.Ceil(float64(-minY)))
	if left == right || top == bottom {
		// SkScalerContext::internalMakeGlyph: a glyph without a width or
		// without a height has no bounds.
		return 0, 0, 0, 0, true
	}
	return left, top, right, bottom, false
}

// The tags of the metrics of the MVAR table that skrifa reads.
const (
	tagHasc = 'h'<<24 | 'a'<<16 | 's'<<8 | 'c'
	tagHdsc = 'h'<<24 | 'd'<<16 | 's'<<8 | 'c'
	tagHlgp = 'h'<<24 | 'l'<<16 | 'g'<<8 | 'p'
	tagCpht = 'c'<<24 | 'p'<<16 | 'h'<<8 | 't'
	tagXhgt = 'x'<<24 | 'h'<<16 | 'g'<<8 | 't'
)

// webLinuxMetrics is linuxMetrics for a face of a web font: Metrics::new
// of skrifa adds the deltas of the MVAR table at the instance of a
// variable font.
func (f *Face) webLinuxMetrics(size float64) Metrics {
	t := f.ft
	scale := float32(1)
	if t.upem != 0 {
		scale = float32(size) / float32(t.upem)
	}
	ascent := float32(float32(t.ascender) * scale)
	descent := float32(float32(t.descender) * scale)
	leading := float32(float32(t.lineGap) * scale)
	var xHeight, capHeight float32
	if t.hasHeights {
		xHeight = float32(float32(t.sxHeight) * scale)
		capHeight = float32(float32(t.sCapHeight) * scale)
	}
	if g := f.webGlyf(); g != nil && len(f.web.skCoords) != 0 {
		// mvar.metric_delta(tag, coords).to_f64() as f32 * scale.
		delta := func(tag uint32) float32 {
			return float32(float32(float64(g.mvarDelta(tag, f.web.skCoords))/65536) * scale)
		}
		ascent += delta(tagHasc)
		descent += delta(tagHdsc)
		leading += delta(tagHlgp)
		if t.hasHeights {
			capHeight += delta(tagCpht)
			xHeight += delta(tagXhgt)
		}
	}
	// The descent of skrifa is negative, and Skia changes its sign.
	descent = -descent
	// generateFontMetrics: a font without the heights gets them from the
	// outlines of H and x, and then from the ascent.
	if capHeight == 0 {
		if h, ok := f.webLetterHeight('H', size); ok {
			capHeight = h
		} else {
			capHeight = ascent
		}
	}
	if xHeight == 0 {
		if h, ok := f.webLetterHeight('x', size); ok {
			xHeight = h
		} else {
			xHeight = ascent
		}
	}
	m := Metrics{FloatAscent: float64(ascent), FloatDescent: float64(descent), LineGap: float64(leading)}
	m.Ascent = roundScalar(float64(ascent))
	m.Descent = roundScalar(float64(descent))
	m.LineSpacing = math.Round(m.Ascent) + math.Round(m.Descent) + math.Round(float64(leading))
	m.XHeight = float64(xHeight)
	m.CapHeight = float64(capHeight)
	return m
}

// webLetterHeight is getContourHeightForLetter of Skia: the height of the
// bounds of the path of the glyph of a character, in px.
func (f *Face) webLetterHeight(r rune, size float64) (float32, bool) {
	gid, ok := f.NominalGlyph(r)
	if !ok || gid == 0 {
		return 0, false
	}
	segments, ok := f.webPath(gid, float32(size), true)
	if !ok {
		return 0, false
	}
	first := true
	var minY, maxY float32
	for _, seg := range segments {
		for _, p := range seg.Args[:segmentArgs(seg.Op)] {
			if first {
				minY, maxY, first = p.Y, p.Y, false
				continue
			}
			minY, maxY = min(minY, p.Y), max(maxY, p.Y)
		}
	}
	if first {
		return 0, false
	}
	return maxY - minY, true
}

// webDarwinBounds is the rectangle of a glyph of a web font on macOS, with
// y down: the bounds of the points of the path of the glyph. Blink takes
// the path of the glyph there (GetPathBoundsForGlyph in
// skia_text_metrics.cc), and does not round the bounds of a font with
// subpixel positioning.
//
// SkFont::getPath makes the path at 64px and scales it to the font size
// (SkFont::setupForAsPaths). At 64px the path is the outline of CoreText
// (SkScalerContext_Mac::generatePath) through the remaining matrix, which
// has the skew of a synthetic italic, with the stroke of a synthetic bold
// (SkScalerContext::internalGetPath). The outline of CoreText is the
// outline of the font in font units, at the instance of a variable font,
// times the scale.
func (f *Face) webDarwinBounds(gid uint32, size float64) (left, top, right, bottom float32, empty bool) {
	key := webDrawKey{gid, math.Float32bits(float32(size)), false}
	if v, ok := f.web.bounds.Load(key); ok {
		b := v.([5]float32)
		return b[0], b[1], b[2], b[3], b[4] != 0
	}
	left, top, right, bottom, empty = f.webDarwinBoundsUncached(gid, size)
	rec := [5]float32{left, top, right, bottom, 0}
	if empty {
		rec[4] = 1
	}
	f.web.bounds.Store(key, rec)
	return left, top, right, bottom, empty
}

func (f *Face) webDarwinBoundsUncached(gid uint32, size float64) (left, top, right, bottom float32, empty bool) {
	outline, ok := f.gt.GlyphDataOutline(gtfont.GID(gid))
	if !ok || len(outline.Segments) == 0 {
		return 0, 0, 0, 0, true
	}
	scale := canonicalSize / f.upem
	m := f.webRemainingMatrix(canonicalSize)
	variable := len(f.web.coords) != 0
	segments := make([]ot.Segment, len(outline.Segments))
	for i, seg := range outline.Segments {
		segments[i].Op = seg.Op
		for j, p := range seg.Args[:segmentArgs(seg.Op)] {
			ux, uy := float64(p.X), float64(p.Y)
			if variable {
				// CoreText has the points of an instance of a variable
				// font in whole font units.
				ux, uy = math.Floor(ux+0.5), math.Floor(uy+0.5)
			}
			px, py := float64(ux*scale), float64(uy*scale)
			if m.identity {
				segments[i].Args[j] = ot.SegmentPoint{X: float32(px), Y: float32(py)}
				continue
			}
			// CoreText applies the matrix in float64, with y up: the skew
			// has the other sign there (MatrixToCGAffineTransform).
			x := float64(float64(m.scaleX)*px) + float64(float64(-m.skewX)*py)
			y := float64(float64(m.scaleY) * py)
			segments[i].Args[j] = ot.SegmentPoint{X: float32(x), Y: float32(y)}
		}
	}
	half := float32(0)
	if f.web.syntheticBold {
		width := fakeBoldWidth(canonicalSize)
		if stroker := glyphStroker.Load(); stroker != nil {
			segments = (*stroker)(segments, width)
		} else {
			half = float32(width / 2)
		}
	}
	first := true
	var minX, minY, maxX, maxY float32
	for _, seg := range segments {
		for _, p := range seg.Args[:segmentArgs(seg.Op)] {
			if first {
				minX, maxX, minY, maxY, first = p.X, p.X, p.Y, p.Y, false
				continue
			}
			minX, maxX = min(minX, p.X), max(maxX, p.X)
			minY, maxY = min(minY, p.Y), max(maxY, p.Y)
		}
	}
	if first {
		return 0, 0, 0, 0, true
	}
	minX, maxX, minY, maxY = minX-half, maxX+half, minY-half, maxY+half
	// The path at 64px times the scale to the font size (SkMatrix::Scale
	// and Scale_pts): the bounds of the scaled points are the scaled
	// bounds, because the scale is positive.
	toSize := float32(size) / canonicalSize
	return float32(minX * toSize), float32(-maxY * toSize), float32(maxX * toSize), float32(-minY * toSize), false
}
