// Ported from Chromium 156.0.8078.4 (Blink), in part: the rules that say
// which SVG elements have a layout object. The functions of this file follow
// the functions of Blink that their comments name:
//
//   - SVGGElement::LayoutObjectIsNeeded: core/svg/svg_g_element.cc
//     (LGPL-2.0-or-later).
//   - SVGTests::IsValid: core/svg/svg_tests.cc (LGPL-2.0-or-later).
//   - SVGSwitchElement::ChildShouldCreateLayoutObject. Chromium 156.0.8078.4
//     has no function with this name. Its rule for the children of a switch
//     element is LayoutSVGTransformableContainer::IsChildAllowed in
//     core/layout/svg/layout_svg_transformable_container.cc
//     (LGPL-2.0-or-later). core/svg/svg_switch_element.cc has the same
//     license.
//
// The paths are in third_party/blink/renderer. The license of each file is
// the license in its header at that version. See NOTICE in this directory.

package browser

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

// This file says which SVG elements have a layout object in Blink. An
// element with no layout object has no box: getBBox and
// getBoundingClientRect return 0, 0, 0, 0, getComputedTextLength returns
// 0, and its container does not count it. The rules:
//
//   - display: none gives no layout object, and none to the descendants.
//     A g element is the exception (SVGGElement::LayoutObjectIsNeeded): it
//     becomes a hidden container, as defs is. Its descendants have layout
//     objects, so getBBox of a shape or a text in it returns the geometry.
//   - The conditional processing attributes must pass (SVGTests::IsValid).
//   - Of the children of a switch element, only the first valid SVG
//     element has a layout object
//     (SVGSwitchElement::ChildShouldCreateLayoutObject). That child can
//     have no box of its own, as a title or an element with display: none
//     does: the switch then shows nothing.
//   - A foreignObject in a hidden container has no layout object.
//
// An element in a hidden container has a layout object and no place on the
// page: getBoundingClientRect returns 0, 0, 0, 0.
//
// testdata/norender-cases.json has the values of Chromium.

// svgExtensions are the values of requiredExtensions that Blink supports.
var svgExtensions = map[string]bool{
	"http://www.w3.org/1999/xhtml":       true,
	"http://www.w3.org/1998/Math/MathML": true,
}

// svgNoTests lists the SVG elements that do not have the conditional
// processing attributes.
var svgNoTests = map[string]bool{"title": true, "desc": true, "metadata": true, "style": true, "script": true}

// svgElementValid is SVGTests::IsValid: the conditional processing
// attributes of the element pass. requiredExtensions passes when it names
// only supported extensions, and at least one. systemLanguage does not pass
// in the browser of the reference: none of "en", "en-US" and the empty
// value does. requiredFeatures has no effect.
func svgElementValid(n *dom.Node) bool {
	if svgNoTests[n.Tag] {
		return true
	}
	if n.HasAttr("systemLanguage") {
		return false
	}
	if v, ok := n.GetAttr("requiredExtensions"); ok {
		values := strings.Fields(v)
		if len(values) == 0 {
			return false
		}
		for _, value := range values {
			if !svgExtensions[value] {
				return false
			}
		}
	}
	return true
}

// switchChild returns the child of a switch element that can have a layout
// object: the first SVG element that is valid.
func switchChild(sw *dom.Node) *dom.Node {
	for c := sw.FirstChild; c != nil; c = c.Next {
		if c.Type == dom.ElementNode && c.NS == dom.NSSVG && svgElementValid(c) {
			return c
		}
	}
	return nil
}

// isHiddenContainer reports whether an SVG element is a hidden container:
// its descendants have layout objects and are never part of the page.
func (e *Engine) isHiddenContainer(n *dom.Node) bool {
	return notRendered[n.Tag] || (n.Tag == "g" && e.display(n) == "none")
}

// svgBoxless reports whether an SVG element has no layout object for a
// reason of its own: its display, its conditional processing attributes, or
// its place in a switch element. It does not look at the ancestors.
func (e *Engine) svgBoxless(n *dom.Node) bool {
	if n.Tag != "g" && e.display(n) == "none" {
		return true
	}
	if !svgElementValid(n) {
		return true
	}
	if p := n.Parent; p != nil && p.Type == dom.ElementNode && p.NS == dom.NSSVG && p.Tag == "switch" && switchChild(p) != n {
		return true
	}
	return false
}

// svgLayoutState reports whether an SVG element has a layout object, and
// whether it is in a hidden container (or is one).
func (e *Engine) svgLayoutState(n *dom.Node) (hasObject, hidden bool) {
	if n == nil || n.Type != dom.ElementNode || n.NS != dom.NSSVG {
		return false, false
	}
	var chain []*dom.Node
	for p := n; p != nil && p.Type == dom.ElementNode && p.NS == dom.NSSVG; p = p.Parent {
		chain = append(chain, p)
	}
	for i := len(chain) - 1; i >= 0; i-- {
		el := chain[i]
		if i < len(chain)-1 && e.svgBoxless(el) {
			return false, hidden
		}
		if el.Tag == "foreignObject" && hidden {
			return false, hidden
		}
		if i < len(chain)-1 && e.isHiddenContainer(el) {
			hidden = true
		}
	}
	return true, hidden
}
