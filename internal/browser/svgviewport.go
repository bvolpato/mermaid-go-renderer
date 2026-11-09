// Ported from Chromium 156.0.8078.4 (Blink), in part: the viewport of a
// percentage in an SVG length, and the limits of a length. The functions of
// this file follow the parts of Blink that their comments name:
//
//   - SVGLengthContext::DetermineViewport and ValueForLength. Chromium
//     156.0.8078.4 has this code as SVGViewportResolver::ResolveViewport and
//     ValueForLength in core/svg/svg_length_functions.cc
//     (LGPL-2.0-or-later). core/svg/svg_length_context.cc, which has the
//     class SVGLengthContext, has the same license.
//   - kMinValueForCssLength and kMaxValueForCssLength:
//     core/css/css_primitive_value.cc (LGPL-2.0-or-later).
//
// The paths are in third_party/blink/renderer. The license of each file is
// the license in its header at that version. See NOTICE in this directory.

package browser

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

// This file has the SVG lengths that need more than their own number: a
// percentage, which refers to the viewport of the nearest svg element, and
// the limits of a length. testdata/svgviewport-cases.json has the values
// of Chromium.

// cssLengthMin and cssLengthMax are kMinValueForCssLength and
// kMaxValueForCssLength of Blink as float values. Blink keeps the lengths
// of SVG attributes, and of the translate function of a transform, as CSS
// lengths, and a CSS length has these limits. A longer length is cut: the
// line of a gantt chart at x = 117550174 is at 33554428.
const (
	cssLengthMin = -33554430
	cssLengthMax = 33554428
)

// clampCSSLength cuts a length to the limits of a CSS length.
func clampCSSLength(v float32) float32 {
	return min(max(v, cssLengthMin), cssLengthMax)
}

// svgViewportSize is SVGLengthContext::DetermineViewport: the size that a
// percentage in a length of n refers to. It is the size of the viewBox of
// the nearest svg ancestor, or the size of that svg element when it has no
// viewBox.
func (e *Engine) svgViewportSize(n *dom.Node) (w, h float32, ok bool) {
	for p := n.Parent; p != nil && p.Type == dom.ElementNode && p.NS == dom.NSSVG; p = p.Parent {
		if p.Tag != "svg" {
			continue
		}
		if vb := parseNumberList(p.AttrOr("viewBox", "")); len(vb) == 4 && vb[2] > 0 && vb[3] > 0 {
			return vb[2], vb[3], true
		}
		if p == outermostSVG(p) {
			box := e.svgRootBoxOf(p)
			return box.w.f32(), box.h.f32(), true
		}
		// An inner svg: its width and its height, which are 100% of its
		// own viewport when it does not have them.
		size := e.textStyleOf(p).size
		pw, ph, _ := e.svgViewportSize(p)
		w, h = pw, ph
		if v, ok := e.firstLength(p, "width", size); ok {
			w = v
		}
		if v, ok := e.firstLength(p, "height", size); ok {
			h = v
		}
		return w, h, true
	}
	return 0, 0, false
}

// svgPercentLength resolves a percentage of the length attribute name of
// the element n (SVGLengthContext::ValueForLength). A horizontal length
// refers to the width of the viewport, a vertical length to its height, and
// another length, such as the radius of a circle, to its diagonal divided
// by the square root of 2.
func (e *Engine) svgPercentLength(n *dom.Node, name string, percent float32) float32 {
	w, h, ok := e.svgViewportSize(n)
	if !ok {
		return 0
	}
	var base float32
	switch name {
	case "x", "dx", "width", "x1", "x2", "cx", "rx", "fx", "textLength", "startOffset":
		base = w
	case "y", "dy", "height", "y1", "y2", "cy", "ry", "fy":
		base = h
	default:
		base = float32(math.Sqrt(float64(float32(float32(w*w)+float32(h*h)) / 2)))
	}
	return float32(base*percent) / 100
}

// percentValue parses a percentage and returns its number.
func percentValue(v string) (float32, bool) {
	v = strings.TrimSpace(v)
	if !strings.HasSuffix(v, "%") {
		return 0, false
	}
	f, rest, ok := parseSVGNumber(v[:len(v)-1])
	if !ok || strings.TrimSpace(rest) != "" {
		return 0, false
	}
	return f, true
}
