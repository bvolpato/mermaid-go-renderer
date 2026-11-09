// Ported from Skia at revision aa789d9b, the revision of Chromium
// 156.0.8078.80 (src/ports/SkTypeface_fontations.cpp,
// src/ports/fontations/src/base.rs, src/core/SkStrikeSpec.cpp,
// src/core/SkFont.cpp, src/core/SkScalerContext.cpp; BSD 3-clause license,
// LICENSE-Skia.txt), from Blink of that Chromium
// (third_party/blink/renderer/platform/fonts/font_metrics.cc,
// simple_font_data.cc, skia/skia_text_metrics.cc, web_font_render_style.cc)
// and from the Rust crates skrifa 0.46.2 (src/metrics.rs, src/instance.rs)
// and font-types 0.12.5 (src/fixed.rs), which have the license MIT OR
// Apache-2.0 (LICENSE-skrifa.txt).
//
// The license of each Blink file is the license in its header. The headers
// are the same at Chromium 156.0.8078.80 and 156.0.8078.4:
//
//   - skia/skia_text_metrics.cc and web_font_render_style.cc: BSD-3-Clause,
//     Copyright 2016 The Chromium Authors (LICENSE-Chromium.txt).
//   - font_metrics.cc and simple_font_data.cc: BSD-3-Clause, with the terms
//     in the file.
//     Copyright (C) 2005, 2008, 2010 Apple Inc. All rights reserved.
//     Copyright (C) 2006 Alexey Proskuryakov
//
// NOTICE-Blink.txt in this directory has the headers of the two files that
// have the terms in the file, with the conditions and the disclaimer.
//
// The fixed-point functions repeat FT_MulFix, FT_DivFix and FT_MulDiv of
// FreeType 2.13.2 (src/base/ftcalc.c), which has the FreeType License
// (LICENSE-FreeType.txt).

package font

import (
	"encoding/binary"
	"math"
	"sync"

	gtfont "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
)

// Chromium on Linux gets the metrics of a font from Skia, and Skia reads
// the font with the Fontations backend: the Rust crate skrifa. skrifa
// repeats the fixed-point arithmetic of FreeType, so the function names of
// FreeType are kept here.
//
// The font settings of headless Chromium come from fontconfig: slight
// hinting, antialiasing, no subpixel positioning. Blink still positions
// glyphs at subpixels (WebFontRenderStyle::ApplyToSkFont), and for slight
// hinting the Fontations backend returns linear advances and runs the
// automatic hinter of skrifa in its light mode, which moves outline points
// in the vertical direction only (see autohint.go). So the advance of a
// glyph is the scaled advance of the font, and the bounds of a glyph are
// the bounds of the hinted outline, rounded out to whole pixels.
//
// For a font size above 256px Skia uses a font of 64px without hinting and
// scales its values (SkStrikeSpec::MakeCanonicalized).

// ftMulFix is FT_MulFix: a*b/0x10000, rounded to the nearest integer. A
// half rounds away from zero.
func ftMulFix(a, b int64) int64 {
	ab := a * b
	if ab < 0 {
		return (ab + 0x8000 - 1) >> 16
	}
	return (ab + 0x8000) >> 16
}

// ftDivFix is FT_DivFix: a*0x10000/b, rounded to the nearest integer.
func ftDivFix(a, b int64) int64 {
	s := int64(1)
	if a < 0 {
		a, s = -a, -s
	}
	if b < 0 {
		b, s = -b, -s
	}
	q := int64(0x7FFFFFFF)
	if b > 0 {
		q = ((a << 16) + (b >> 1)) / b
	}
	return s * q
}

// ftMulDiv is FT_MulDiv: a*b/c, rounded to the nearest integer.
func ftMulDiv(a, b, c int64) int64 {
	s := int64(1)
	if a < 0 {
		a, s = -a, -s
	}
	if b < 0 {
		b, s = -b, -s
	}
	if c < 0 {
		c, s = -c, -s
	}
	d := int64(0x7FFFFFFF)
	if c > 0 {
		d = (a*b + (c >> 1)) / c
	}
	return s * d
}

// ftFace has what the rules of Linux read from a font file: the line
// metrics that skrifa selects, and the automatic hinter.
type ftFace struct {
	ld *ot.Loader

	upem int64
	// ascender, descender and lineGap are the line metrics that skrifa
	// selects, in font units. The descender is negative.
	ascender, descender, lineGap int64
	// hasHeights says that the OS/2 table has sxHeight and sCapHeight.
	hasHeights           bool
	sxHeight, sCapHeight int64

	// hinter is the automatic hinter of the face, made on first use. See
	// autohint.go.
	hinterOnce sync.Once
	hinter     *ahInstance
}

func be16(b []byte, off int) int64 { return int64(int16(binary.BigEndian.Uint16(b[off:]))) }
func beU16(b []byte, off int) int  { return int(binary.BigEndian.Uint16(b[off:])) }

// newFTFace reads the tables of a face for the rules of Linux. It returns
// nil for a face of another platform.
func newFTFace(ld *ot.Loader, f *Face) *ftFace {
	if f.platform != PlatformLinux {
		return nil
	}
	t := &ftFace{ld: ld, upem: int64(f.upem)}
	var hheaAscender, hheaDescender, hheaLineGap int64
	hasHhea := false
	if b, err := ld.RawTable(ot.MustNewTag("hhea")); err == nil && len(b) >= 36 {
		hasHhea = true
		hheaAscender, hheaDescender, hheaLineGap = be16(b, 4), be16(b, 6), be16(b, 8)
	}
	// Metrics::new of skrifa selects the line metrics as FreeType does:
	// the typographic values of the OS/2 table when the font asks for
	// them, else the values of the hhea table, and for a font without
	// those the typographic values or the Windows values.
	usedTypo := false
	b, err := ld.RawTable(ot.MustNewTag("OS/2"))
	hasOS2 := err == nil && len(b) >= 78
	if hasOS2 {
		if beU16(b, 62)&128 != 0 {
			// USE_TYPO_METRICS
			t.ascender, t.descender, t.lineGap = be16(b, 68), be16(b, 70), be16(b, 72)
			usedTypo = true
		}
		if binary.BigEndian.Uint16(b) >= 2 && len(b) >= 90 {
			t.hasHeights = true
			t.sxHeight, t.sCapHeight = be16(b, 86), be16(b, 88)
		}
	}
	if !usedTypo {
		if hasHhea {
			t.ascender, t.descender, t.lineGap = hheaAscender, hheaDescender, hheaLineGap
		}
		if t.ascender == 0 && t.descender == 0 && hasOS2 {
			if be16(b, 68) != 0 || be16(b, 70) != 0 {
				t.ascender, t.descender, t.lineGap = be16(b, 68), be16(b, 70), be16(b, 72)
			} else {
				// usWinAscent and usWinDescent are unsigned, and the
				// descent is positive.
				t.ascender, t.descender = int64(beU16(b, 74)), -int64(beU16(b, 76))
			}
		}
	}
	return t
}

// scale is Size::fixed_linear_scale of skrifa, and x_scale of
// FT_Size_Metrics: a 16.16 number that converts font units to 26.6 px. The
// font size becomes a 26.6 number without its fraction, and is divided by
// the em size.
func (t *ftFace) scale(size float64) int64 {
	if t.upem <= 0 {
		return 0x10000 * 64
	}
	return ftDivFix(int64(float32(size)*64), t.upem)
}

// canonicalSize is kCanonicalTextSizeForPaths of Skia, and
// canonicalLimit is the font size above which Skia measures a glyph at the
// canonical size (SkStrikeSpec::ShouldDrawAsPath).
const (
	canonicalSize  = 64
	canonicalLimit = 256
)

// linuxAdvance16 returns the advance of a glyph on Linux in 16.16 fixed
// point px, for a nominal advance in font units.
//
// skrifa computes the linear advance as 16.16 px, as FT_Load_Glyph does
// for linearHoriAdvance (GlyphMetrics::advance_width), and returns it as a
// float32. Blink converts the float32 back to 16.16 for HarfBuzz and cuts
// the fraction.
func (f *Face) linuxAdvance16(nominal int32, size float64) int64 {
	if float32(size) > canonicalLimit {
		// The advance at 64px, times size/64 (SkFont::getWidthsBounds).
		linear := ftMulDiv(int64(nominal), f.ft.scale(canonicalSize), 64)
		advance := float32(linear) / 65536
		advance *= float32(size) / canonicalSize
		return int64(advance * 65536)
	}
	linear := ftMulDiv(int64(nominal), f.ft.scale(size), 64)
	advance := float32(linear) / 65536 // Fixed::to_f32
	if f.defaultStyle {
		// A fallback font has the default font settings of the browser:
		// hinting in both directions and no linear metrics. Skia then
		// rounds the advance of the hinted glyph to a whole pixel
		// (SkFontationsScalerContext::generateMetrics). The hinter can
		// move the advance by a pixel, which this engine does not model:
		// it rounds the scaled advance.
		advance = float32(math.Round(float64(advance)))
	}
	return int64(advance * 65536) // SkiaScalarToHarfBuzzPosition
}

// linuxMetrics returns the vertical metrics of the face on Linux: Metrics
// of skrifa and SkFontationsScalerContext::generateFontMetrics of Skia,
// then FontMetrics::AscentDescentWithHacks and SimpleFontData::PlatformInit
// of Blink.
func (f *Face) linuxMetrics(size float64) Metrics {
	if f.web != nil {
		// A variable web font has the metrics of its instance.
		return f.webLinuxMetrics(size)
	}
	t := f.ft
	// Size::linear_scale: the font size divided by the em size in float32.
	// Every value is the value of the font times this factor.
	scale := float32(1)
	if t.upem != 0 {
		scale = float32(size) / float32(t.upem)
	}
	ascent := float32(float32(t.ascender) * scale)
	descent := float32(float32(-t.descender) * scale)
	leading := float32(float32(t.lineGap) * scale)
	var xHeight, capHeight float32
	if t.hasHeights {
		xHeight = float32(float32(t.sxHeight) * scale)
		capHeight = float32(float32(t.sCapHeight) * scale)
	}
	// A font without the values in the OS/2 table gets them from the
	// height of the outlines of 'H' and 'x', and then from the ascent.
	if capHeight == 0 {
		if h, ok := f.letterHeight('H', size); ok {
			capHeight = h
		} else {
			capHeight = ascent
		}
	}
	if xHeight == 0 {
		if h, ok := f.letterHeight('x', size); ok {
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

// ftCBox is the bounds of the points of the path of a scaled glyph
// outline in 26.6 px, y up. The control points of curves count, as in
// SkPath::getBounds.
type ftCBox struct {
	xMin, yMin, xMax, yMax int64
	ok                     bool // false for a glyph without contours
}

// letterHeight is getContourHeightForLetter of Skia: the height of the
// bounds of the hinted outline of the glyph of a character, in px.
func (f *Face) letterHeight(r rune, size float64) (float32, bool) {
	gid, ok := f.NominalGlyph(r)
	if !ok || gid == 0 {
		return 0, false
	}
	box, ok := f.hintedCBox(gid, size)
	if !ok || !box.ok {
		return 0, false
	}
	return float32(float32(box.yMax)/64) - float32(float32(box.yMin)/64), true
}

// ScalerBounds returns the bounds that the font rasterizer gives Skia for a
// glyph at a font size, when the rasterizer has rules of its own: the
// rectangle in px, with y down, relative to the origin of the glyph on the
// baseline. handled is false when the caller computes the bounds from the
// outline of the glyph, as on macOS. empty is true for a glyph without
// contours.
//
// On Linux the bounds are the bounds of the points of the hinted outline,
// rounded out to whole pixels (SkScalerContext::GenerateMetricsFromPath).
// Above 256px they are the bounds at 64px without hinting, times size/64.
func (f *Face) ScalerBounds(gid uint32, size float64) (left, top, right, bottom float32, empty, handled bool) {
	if f.platform != PlatformLinux {
		if f.web != nil && f.platform == PlatformDarwin && (f.web.syntheticBold || f.web.syntheticItalic || len(f.web.coords) != 0) {
			// A web font with a synthetic style, or an instance of a
			// variable font, on macOS: the bounds of the path that Skia
			// makes.
			left, top, right, bottom, empty = f.webDarwinBounds(gid, size)
			return left, top, right, bottom, empty, true
		}
		return 0, 0, 0, 0, false, false
	}
	if f.web != nil {
		// A web font has normal hinting and can have synthetic styles.
		left, top, right, bottom, empty = f.webScalerBounds(gid, size)
		return left, top, right, bottom, empty, true
	}
	if float32(size) > canonicalLimit {
		l, t, r, b, empty := f.pixelBounds(gid, canonicalSize, false)
		if empty {
			return 0, 0, 0, 0, true, true
		}
		// scale_pos of SkFont.cpp: each edge times the scale in float32.
		s := float32(size) / canonicalSize
		return float32(l * s), float32(t * s), float32(r * s), float32(b * s), false, true
	}
	left, top, right, bottom, empty = f.pixelBounds(gid, size, true)
	return left, top, right, bottom, empty, true
}

// ScalerOutline returns the outline that the font rasterizer gives Skia for
// a glyph at a font size, when the rasterizer has rules of its own: the path
// in px, with y up, relative to the origin of the glyph on the baseline. It
// is the outline whose bounds ScalerBounds returns. ok is false when the
// caller scales the outline of the font, as on macOS, and for a font without
// TrueType outlines. A glyph without a path has no segments.
//
// On Linux the outline is the outline after the automatic hinter
// (generatePathForGlyphId of the Fontations backend of Skia). Skia uses it
// for text along an axis: the hinter works at the font size, and a rotation
// by a quarter turn or a flip applies to the hinted outline. Above 256px
// the outline is the outline at 64px without hinting, times size/64: Skia
// draws such a glyph as a path through a matrix
// (SkStrikeSpec::ShouldDrawAsPath).
//
// The segments are the segments of the path of Skia. A move starts a
// contour, and each contour is closed. There is no segment for the close.
func (f *Face) ScalerOutline(gid uint32, size float64) (gtfont.GlyphOutline, bool) {
	if f.isWebLinux() {
		if float32(size) > canonicalLimit {
			outline, ok := f.ScalerOutlineUnhinted(gid, canonicalSize)
			s := float32(size) / canonicalSize
			for i := range outline.Segments {
				for j := range outline.Segments[i].Args {
					p := &outline.Segments[i].Args[j]
					p.X, p.Y = float32(p.X*s), float32(p.Y*s)
				}
			}
			return outline, ok
		}
		segments, _, _ := f.webFinalPath(gid, float32(size), true)
		return gtfont.GlyphOutline{Segments: segments}, true
	}
	if float32(size) > canonicalLimit {
		outline, ok := f.ScalerOutlineUnhinted(gid, canonicalSize)
		s := float32(size) / canonicalSize
		for i := range outline.Segments {
			for j := range outline.Segments[i].Args {
				p := &outline.Segments[i].Args[j]
				p.X, p.Y = float32(p.X*s), float32(p.Y*s)
			}
		}
		return outline, ok
	}
	inst := f.scalerHinter()
	if inst == nil {
		return gtfont.GlyphOutline{}, false
	}
	return gtfont.GlyphOutline{Segments: inst.path(gid, float32(size), true)}, true
}

// ScalerOutlineUnhinted returns the outline that the font rasterizer gives
// Skia for a glyph at a font size when hinting is off, with the units and
// the result of ScalerOutline. Skia turns hinting off for text that is not
// along an axis (SkTypeface_Fontations::onFilterRec), and for a glyph that
// it draws as a path, which it takes at 64px.
//
// The outline is not the outline of the font times the scale: the scaler
// rounds each point to 1/64 px at the font size.
func (f *Face) ScalerOutlineUnhinted(gid uint32, size float64) (gtfont.GlyphOutline, bool) {
	if f.isWebLinux() {
		// The segments are shared: copy them, because the caller of the
		// size above 256px scales the result.
		segments, _, _ := f.webFinalPath(gid, float32(size), false)
		return gtfont.GlyphOutline{Segments: append([]ot.Segment(nil), segments...)}, true
	}
	inst := f.scalerHinter()
	if inst == nil {
		return gtfont.GlyphOutline{}, false
	}
	return gtfont.GlyphOutline{Segments: inst.path(gid, float32(size), false)}, true
}

// scalerHinter returns the port of the scaler of skrifa for the face, or
// nil when the outlines of the face come from the font without rules of the
// platform: a face of another platform, and a font without TrueType
// outlines.
func (f *Face) scalerHinter() *ahInstance {
	if f.platform != PlatformLinux {
		return nil
	}
	return f.autohinter()
}

// pixelBounds returns the bounds of the outline of a glyph at a font size,
// rounded out to whole pixels, with y down. hinted selects the outline of
// the automatic hinter.
func (f *Face) pixelBounds(gid uint32, size float64, hinted bool) (left, top, right, bottom float32, empty bool) {
	var box ftCBox
	var ok bool
	if hinted {
		box, ok = f.hintedCBox(gid, size)
	} else {
		box, ok = f.unhintedCBox(gid, size)
	}
	if !ok {
		// A font without TrueType outlines: the outline in font units,
		// scaled. This is not the arithmetic of skrifa.
		minX, minY, maxX, maxY, has := f.OutlineBounds(gid)
		if !has {
			return 0, 0, 0, 0, true
		}
		scale := size / f.upem
		return float32(math.Floor(float64(minX) * scale)), float32(-math.Ceil(float64(maxY) * scale)),
			float32(math.Ceil(float64(maxX) * scale)), float32(-math.Floor(float64(minY) * scale)), false
	}
	if !box.ok {
		return 0, 0, 0, 0, true
	}
	// Rounded out to whole pixels: floor of the minimum, ceiling of the
	// maximum.
	floor := func(v int64) float32 { return float32(v >> 6) }
	ceil := func(v int64) float32 { return float32((v + 63) >> 6) }
	return floor(box.xMin), -ceil(box.yMax), ceil(box.xMax), -floor(box.yMin), false
}
