// This file ports no upstream file. It connects the faces of web fonts
// with the stroker of the rasterizer.

package font

import (
	"sync/atomic"

	ot "github.com/go-text/typesetting/font/opentype"
)

// GlyphStroker makes the path that Skia fills for a glyph with a synthetic
// bold: the outline of the glyph together with a stroke of the given width
// around it (SkStrokeRec::applyToPath with the style stroke and fill, a
// miter join and the miter limit 4). The segments are in px with y up, as
// Face.ScalerOutline returns them, and so is the result.
type GlyphStroker func(segments []ot.Segment, width float32) []ot.Segment

var glyphStroker atomic.Pointer[GlyphStroker]

// RegisterGlyphStroker sets the stroker for the synthetic bold of web
// fonts. The stroker is the port of the stroker of Skia in internal/raster,
// which registers it when the package is linked: this package cannot import
// the rasterizer. internal/render imports the rasterizer, so every render
// has the stroker.
//
// Without a stroker the bounds of a glyph with a synthetic bold are the
// bounds of its outline, larger by half of the width of the stroke on each
// side, which is not always the rectangle of Skia.
func RegisterGlyphStroker(stroker GlyphStroker) {
	glyphStroker.Store(&stroker)
}
