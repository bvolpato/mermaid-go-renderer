// This file ports no upstream file. It has the measurements of the glyphs of
// Apple Color Emoji, from measurements in Chromium.

package font

import (
	"math"

	gtfont "github.com/go-text/typesetting/font"
)

// This file has the measurements of the glyphs of Apple Color Emoji, the
// color bitmap font that macOS uses for emoji: the advance, the bounds and
// the line metrics. It ports no upstream file. The rules come from
// measurements in Chromium for the font sizes below 256px
// (tools/reference/gen-browser-emoji-probe.mjs writes them to
// internal/browser/testdata/emoji-probe.json.gz).
//
// The advance of every emoji glyph in the font is one em. Chromium does not
// get that value: Skia asks CoreText (CTFontGetAdvancesForGlyphs), and
// CoreText draws the glyphs of this font larger than the font size at small
// sizes. Blink then rounds the advance to a whole pixel, because it does not
// position the glyphs of a color bitmap font at subpixels
// (SkiaTextMetrics::GetGlyphWidthForHarfBuzz rounds when the font is not
// subpixel).
//
// The choice of the emoji font for a character is in emoji_segment.go and
// in Chain.ShapeText.

// appleColorEmoji is the family name of the font.
const appleColorEmoji = "Apple Color Emoji"

// appleColorEmojiSize returns the size at which CoreText draws the glyphs of
// Apple Color Emoji for a font size: 1.25 times the size up to 16px, a size
// that grows from 20px to 24px between 16px and 24px, and the font size
// from 24px.
func appleColorEmojiSize(size float64) float64 {
	switch {
	case size <= 16:
		return float64(size * 1.25)
	case size <= 24:
		return 20 + float64((size-16)/2)
	}
	return size
}

// colorEmojiAdvance returns the advance in px of a glyph of Apple Color
// Emoji with a nominal advance in font units, at a font size.
func (f *Face) colorEmojiAdvance(nominal int32, size float64) float64 {
	// Skia has the font size as a float32.
	drawn := appleColorEmojiSize(float64(float32(size)))
	// SkScalarRoundToInt: floor(x + 0.5).
	return math.Floor(float64(float32(float64(nominal)*drawn/f.upem)) + 0.5)
}

// colorEmojiExcess is the amount in px by which the rectangle that CoreText
// reports for a glyph of Apple Color Emoji is larger than the font size,
// for a font size of 24px or more. Its effects show where an edge of the
// rectangle is at a whole pixel: the edge is then one pixel further out.
// The measurements of Chromium put the amount between 0.0000077 and
// 0.0000095: it is more than one half of a float32 step at 128, it moves
// the bottom edge of a glyph of 128px, and it does not move the bottom edge
// of a glyph of 32px. The value is a fit to these measurements. It is not
// the number that CoreText computes.
const colorEmojiExcess = 8e-6

// ColorGlyphBounds returns the bounds that Skia has for a glyph of Apple
// Color Emoji at a font size: a rectangle in px, with y down, relative to
// the origin of the glyph on the baseline. ok is false for another font.
// The rectangle is empty for a glyph without an image and without an
// outline, such as a variation selector, and for a glyph that is too small
// to have bounds. syntheticItalic says that the text is italic: the font
// has no italic face, so Skia slants the glyph, and the rectangle with it.
//
// The image of a glyph is a square with the side of the drawn size (see
// appleColorEmojiSize) on the baseline. A few glyphs have an outline and no
// image: the digits and the letters of the regional indicators. CoreText
// moves each glyph down, so that the middle of the square is at 0.375
// times the font size above the baseline: the square of a font size of 24px
// or more sits one eighth of its side below the baseline. Skia takes the
// rectangle from CTFontGetBoundingRectsForGlyphs, applies the slant,
// converts the result to float32, rounds it out to whole pixels and adds
// one pixel on each side (SkScalerContext_Mac::generateMetrics). It does
// this for every glyph of the font, because the font is not scalable.
//
// The function has what Chromium measures for the font sizes below 256px
// (internal/browser/testdata/emoji-probe.json.gz). Skia draws a larger
// glyph in another way, and the bounds are then not whole pixels.
func (f *Face) ColorGlyphBounds(gid uint32, size float64, syntheticItalic bool) (left, top, right, bottom float32, ok bool) {
	if !f.isAppleColorEmoji() {
		return 0, 0, 0, 0, false
	}
	s := float64(float32(size))
	drawn := appleColorEmojiSize(s)
	if drawn < 0.5 {
		// The advance rounds to zero, and the glyph has no bounds.
		return 0, 0, 0, 0, true
	}
	// shift is the distance by which CoreText moves the glyph down.
	shift := float64(drawn/2) - float64(0.375*s)

	// The rectangle of CoreText, with y up: the origin (x, y) and the size
	// (width, height).
	var x, y, width, height float64
	if _, isBitmap := f.gt.GlyphData(gtfont.GID(gid)).(gtfont.GlyphBitmap); isBitmap {
		side := drawn
		if s >= 24 {
			side = s + colorEmojiExcess
		}
		y = -shift
		width, height = side, side
	} else if minX, minY, maxX, maxY, hasOutline := f.OutlineBounds(gid); hasOutline {
		k := drawn / f.upem
		x = float64(float64(minX) * k)
		y = float64(float64(minY)*k) - shift
		width = float64(float64(maxX-minX) * k)
		height = float64(float64(maxY-minY) * k)
	} else {
		return 0, 0, 0, 0, true
	}
	if syntheticItalic {
		// CGRectApplyAffineTransform with the slant x' = x + y / 4: the
		// bounds of the slanted rectangle.
		x += float64(y / 4)
		width += float64(height / 4)
	}
	// SkRect::MakeXYWH(x, -y - height, width, height) with float32 values.
	fLeft := float32(x)
	fTop := float32(-y - height)
	fRight := fLeft + float32(width)
	fBottom := fTop + float32(height)
	// SkRect::roundOut and SkIRect::outset(1, 1).
	return float32(math.Floor(float64(fLeft))) - 1,
		float32(math.Floor(float64(fTop))) - 1,
		float32(math.Ceil(float64(fRight))) + 1,
		float32(math.Ceil(float64(fBottom))) + 1,
		true
}

// colorEmojiMetricsSize returns the font size that gives the line metrics of
// a face: the drawn size for Apple Color Emoji, and the font size for every
// other font. CoreText reports the ascent and the descent of Apple Color
// Emoji for the size at which it draws the glyphs.
func (f *Face) colorEmojiMetricsSize(size float64) float64 {
	if !f.isAppleColorEmoji() {
		return size
	}
	return appleColorEmojiSize(float64(float32(size)))
}

// emojiFace returns the emoji font of the system, or nil. Blink asks for it
// by name (FontCache::PlatformFallbackFontForCharacter on macOS).
func (l *Library) emojiFace() *Face {
	if l.platform == PlatformLinux {
		// fontconfig gives the emoji font. See linuxEmojiFace.
		return l.linuxEmojiFace()
	}
	if f := l.Match(appleColorEmoji, 400, false); f != nil && f.Family == appleColorEmoji {
		return f
	}
	return nil
}

// emojiFaceFor returns the face for a character of a run with emoji
// presentation: the emoji font when it has the character, and the face of
// a text run in all other cases. A tag character of a flag sequence stays
// with the emoji font of its base. cur is the face of the character before.
func (c *Chain) emojiFaceFor(r rune, cur *Face) *Face {
	if ef := c.lib.emojiFace(); ef != nil {
		if ef.HasGlyph(r) {
			return ef
		}
		if cur == ef && r >= 0xe0020 && r <= 0xe007f {
			return ef
		}
	}
	return c.FaceFor(r)
}

// isDingbat reports whether r is in the Unicode block Dingbats.
func isDingbat(r rune) bool {
	return r >= 0x2700 && r <= 0x27bf
}
