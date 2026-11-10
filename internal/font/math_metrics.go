// This file ports no upstream implementation. It combines nominal glyph
// metrics from go-text/typesetting v0.3.5 (Unlicense OR BSD-3-Clause) with
// glyph information from the OpenType MATH specification.

package font

import (
	"math"

	gtfont "github.com/go-text/typesetting/font"
)

// MathGlyphMetrics has the unhinted metrics of one nominal glyph, in pixels.
// Left, Top, Right, and Bottom describe its ink relative to the baseline.
// Positive x points right. Positive y points down. Ascent and Descent are
// nonnegative distances from the baseline. HasInk is false for blank glyphs.
// ItalicCorrection is zero when MATH provides no correction.
// TopAccentAttachment defaults to half the advance without a MATH value.
// Synthetic styles and MATH device or variation adjustments are not applied.
type MathGlyphMetrics struct {
	ID                       uint32
	Advance                  float64
	Left, Top, Right, Bottom float64
	Ascent, Descent          float64
	HasInk                   bool
	ItalicCorrection         float64
	TopAccentAttachment      float64
	HasTopAccentAttachment   bool
	ExtendedShape            bool
}

// MathGlyph maps r through the nominal cmap and returns its math metrics.
// It performs no shaping, font fallback, or math-alphabet substitution.
// A missing character, invalid size, or nil face returns false.
func (f *Face) MathGlyph(r rune, size float64) (MathGlyphMetrics, bool) {
	if f == nil || f.gt == nil || r < 0 || r > 0x10ffff || !mathValidSize(size) {
		return MathGlyphMetrics{}, false
	}
	id, ok := f.NominalGlyph(r)
	if !ok {
		return MathGlyphMetrics{}, false
	}
	return f.MathGlyphByID(id, size)
}

// MathGlyphByID returns metrics for a glyph, including a size variant.
// It is safe for concurrent use. The underlying go-text extent cache belongs
// to one pooled shaper, rather than to the shared go-text face.
func (f *Face) MathGlyphByID(gid uint32, size float64) (MathGlyphMetrics, bool) {
	if f == nil || f.gt == nil || f.upem <= 0 || !mathValidSize(size) || gid > 0xffff {
		return MathGlyphMetrics{}, false
	}
	source := f.mathSource()
	if source.numGlyphs != 0 && gid >= source.numGlyphs {
		return MathGlyphMetrics{}, false
	}
	s := f.shaper.Get().(*shaper)
	defer f.shaper.Put(s)
	face := s.font.Face()
	ext, hasExtents := face.GlyphExtents(gtfont.GID(gid))
	advance := face.HorizontalAdvance(gtfont.GID(gid))
	if source.numGlyphs == 0 && !hasExtents && advance == 0 {
		return MathGlyphMetrics{}, false
	}
	scale := size / f.upem
	m := MathGlyphMetrics{ID: gid, Advance: float64(advance) * scale}
	if hasExtents && ext.Width != 0 && ext.Height != 0 {
		m.HasInk = true
		m.Left = float64(ext.XBearing) * scale
		m.Top = -float64(ext.YBearing) * scale
		m.Right = (float64(ext.XBearing) + float64(ext.Width)) * scale
		m.Bottom = -(float64(ext.YBearing) + float64(ext.Height)) * scale
		m.Ascent = math.Max(0, -m.Top)
		m.Descent = math.Max(0, m.Bottom)
	}
	info := mathGlyphInfo(source.table)
	if correction, ok := mathGlyphValue(info, 0, gid); ok {
		m.ItalicCorrection = float64(correction) * scale
	}
	m.TopAccentAttachment = m.Advance / 2
	if attachment, ok := mathGlyphValue(info, 2, gid); ok {
		m.TopAccentAttachment = float64(attachment) * scale
		m.HasTopAccentAttachment = true
	}
	if len(info) >= 8 {
		if coverage, ok := info.resolve(int(info.u16(4)), 4); ok {
			_, m.ExtendedShape = mathCoverageIndex(coverage, gid)
		}
	}
	return m, true
}
