// This file ports no upstream file. It says whether a face has an x-height
// of its own. The comment names FontMetrics::HasXHeight of Blink
// (third_party/blink/renderer/platform/fonts/font_metrics.h, which has the
// license LGPL-2.0-or-later at Chromium 156.0.8078.4). See
// ../browser/NOTICE.

package font

// HasXHeight reports whether the face has an x-height of its own
// (FontMetrics::HasXHeight in Blink). When it is false, Metrics.XHeight is
// a guess from the ascent, and the CSS unit ex is half of the font size.
func (f *Face) HasXHeight() bool {
	if f.platform == PlatformLinux {
		// Skia always gives an x-height on Linux. See linux_scaler.go.
		return true
	}
	if f.platform == PlatformDarwin {
		if gid, ok := f.NominalGlyph('x'); ok {
			if _, ok := f.tightTop(gid); ok {
				return true
			}
		}
	}
	return f.xHeight > 0
}
