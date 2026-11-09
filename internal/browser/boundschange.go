// Ported from Chromium 156.0.8078.4 (Blink), in part: the first SVG layout
// of Blink for the boxes of containers. The functions of this file follow
// these functions of Blink:
//
//   - LayoutSVGContainer::UpdateSVGLayout:
//     core/layout/svg/layout_svg_container.cc (LGPL-2.0-or-later).
//   - LayoutSVGShape::UpdateSVGLayout: core/layout/svg/layout_svg_shape.cc
//     (LGPL-2.0-or-later).
//   - SVGContentContainer::UpdateBoundingBoxes:
//     core/layout/svg/svg_content_container.cc (BSD-3-Clause).
//   - SVGLayoutResult: core/layout/svg/svg_layout_info.h (BSD-3-Clause).
//
// The paths are in third_party/blink/renderer. The license of each file is
// the license in its header at that version. See NOTICE in this directory.

package browser

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

// This file follows the first SVG layout of Blink for the boxes of
// containers (LayoutSVGContainer::UpdateSVGLayout,
// SVGContentContainer::UpdateBoundingBoxes, LayoutSVGShape::UpdateSVGLayout).
//
// Blink computes the boxes of a container only when the layout of one of
// its children reports that bounds changed (SVGLayoutResult::bounds_changed):
//
//   - A shape and a text report it on their first layout.
//   - A container reports it when the computation changed its object
//     bounding box or its decorated bounding box (the box with the strokes).
//     Both start as the empty rectangle at the origin. It also reports it
//     when it has a transform.
//
// A container that never computes its boxes has no valid bounding box for
// its own container. So a group whose content is an empty box at the
// origin is valid, and the group around it is not:
//
//	<g><g><text><tspan></tspan></text></g></g>
//
// The label of an edge without a text has this form in mermaid, and the
// point (0, 0) must not become a part of the box of the diagram.
// testdata/boundschange-*.json has the values of Chromium.

// svgLayoutResult is what the first layout of an SVG element gives to its
// container.
type svgLayoutResult struct {
	// bbox and valid are the object bounding box and whether it counts for
	// the container (HasValidBoundingBoxForContainer).
	bbox  rectF
	valid bool
	// changed is SVGLayoutResult::bounds_changed.
	changed bool
	// decorated says that the decorated bounding box is not empty.
	decorated bool
}

// svgLeaves are the SVG elements with a layout object that is not a
// container.
var svgLeaves = map[string]bool{
	"rect": true, "circle": true, "ellipse": true, "line": true, "polyline": true, "polygon": true,
	"path": true, "image": true, "foreignObject": true, "text": true, "use": true,
}

// svgLayout is the first layout of a child of an SVG container.
func (e *Engine) svgLayout(n *dom.Node) svgLayoutResult {
	if n == nil || n.Type != dom.ElementNode || n.NS != dom.NSSVG {
		return svgLayoutResult{}
	}
	if notRendered[n.Tag] || e.display(n) == "none" || e.svgBoxless(n) {
		// No layout object, or a hidden container. See norender.go.
		return svgLayoutResult{}
	}
	switch n.Tag {
	case "svg", "g", "a", "switch":
		res := e.containerLayout(n)
		// A change of the local transform also reports that bounds
		// changed (UpdateAfterSVGLayout).
		if !e.localTransform(n).isIdentity() {
			res.changed = true
		}
		return res
	}
	if !svgLeaves[n.Tag] {
		return svgLayoutResult{}
	}
	if n.Tag == "foreignObject" {
		// A foreignObject in a hidden container has no layout object.
		if has, _ := e.svgLayoutState(n); !has {
			return svgLayoutResult{}
		}
	}
	bbox, valid := e.objectBBox(n)
	return svgLayoutResult{
		bbox:      bbox,
		valid:     valid,
		changed:   true,
		decorated: valid && (!bbox.isEmpty() || (n.Tag != "text" && e.hasStroke(n))),
	}
}

// containerLayout is the layout of an SVG container: the union of the
// valid children, each mapped by its own transform, when a child reports
// that bounds changed.
func (e *Engine) containerLayout(n *dom.Node) svgLayoutResult {
	var res svgLayoutResult
	childChanged := false
	for c := n.FirstChild; c != nil; c = c.Next {
		if c.Type != dom.ElementNode {
			continue
		}
		cr := e.svgLayout(c)
		childChanged = childChanged || cr.changed
		if !cr.valid {
			continue
		}
		mapped := e.localTransform(c).mapRect(cr.bbox)
		if !res.valid {
			res.bbox, res.valid = mapped, true
		} else {
			res.bbox = res.bbox.unionEvenIfEmpty(mapped)
		}
		res.decorated = res.decorated || cr.decorated
	}
	if !childChanged {
		// The container never computes its boxes.
		return svgLayoutResult{}
	}
	res.changed = res.bbox != (rectF{}) || res.decorated
	return res
}

// hasStroke reports whether an element has a visible stroke: the decorated
// bounding box is then larger than the object bounding box.
func (e *Engine) hasStroke(n *dom.Node) bool {
	stroke := strings.TrimSpace(e.lookup(n, "stroke"))
	if stroke == "" || strings.EqualFold(stroke, "none") {
		return false
	}
	width := strings.TrimSpace(e.lookup(n, "stroke-width"))
	if width == "" {
		return true
	}
	px, ok := e.cssLength(width, e.textStyleOf(n), true)
	return !ok || px > 0
}
