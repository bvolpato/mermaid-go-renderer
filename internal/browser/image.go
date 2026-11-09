// Ported from Chromium 156.0.8078.4 (Blink), in part: the size of the img
// element. The size computation follows ComputeReplacedSize of LayoutNG
// (core/layout/length_utils.cc) and LayoutUnit::MulDiv
// (platform/geometry/layout_unit.h). The comments also name
// HTMLImageFallbackHelper (core/html/html_image_fallback_helper.cc). The
// paths are in third_party/blink/renderer. The three files have the
// BSD-3-Clause license at that version. layout_unit.h has the BSD terms in
// the file, with the copyright of Google. See NOTICE in this directory. The
// states of an image and their sizes are the values of Chromium that
// testdata/image-cases.json has.

package browser

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/imagesize"
)

// This file has the img element of HTML: a replaced element whose size
// comes from its style and from the natural size of its image.
//
// An image has one of four states in Chromium, and each has a natural size
// (testdata/image-cases.json has the values):
//
//   - Pending: the image is not loaded yet. This is the state right after
//     the markup is set, also for a data URL. The natural size is 0 x 0.
//   - Loaded: the natural size of the image, and its aspect ratio.
//   - Failed: the load ended with an error event. The element is the
//     broken image icon, 16 x 16, with no aspect ratio.
//   - Empty: the element has no src, or an empty one. It is complete at
//     once, fires no event, and is 0 x 0.
//
// A failed image and an empty image are not replaced elements in Blink
// (HTMLImageFallbackHelper): the element is a box that holds the icon, or
// nothing. So an inline one keeps a width and a height only when it has
// both, and a block-level one is as wide as its containing block.
//
// The engine never loads an image. CompleteImages moves the images of a
// subtree from pending to the state that the load would give, at the point
// where the code of mermaid waits for the load or error events. The natural
// size comes from the data of a data URL, or from Engine.ImageSize. The
// state belongs to the source: Chromium keeps an image that it loaded, so
// every img element with the same source is complete from then on.
//
// An image of a data URL is never pending here. Its load ends in the page
// before mermaid measures the label for the first time: mermaid reads the
// size in the next animation frame (fastdom.measure), and the data of the
// image is at hand. An image from the network is still pending then. The
// label cases of internal/ru/labelprobe show both.

type imageState uint8

const (
	imagePending imageState = iota
	imageLoaded
	imageFailed
	imageEmpty
)

// brokenImageSize is the size of the broken image icon of Chromium.
const brokenImageSize = 16

// naturalImage is the natural size of an image in a state.
type naturalImage struct {
	state imageState
	w, h  lu
	// ratio is true when the image keeps its aspect ratio: a loaded image.
	ratio bool
}

// CompleteImages ends the load of every img element in the subtree of
// container: each image is then loaded or failed. Call it where the code of
// mermaid waits for the load event or the error event of the images. Before
// the call an image is pending and has the size 0 x 0.
func (e *Engine) CompleteImages(container *dom.Node) {
	if container == nil {
		return
	}
	var walk func(n *dom.Node)
	walk = func(n *dom.Node) {
		if n.Type != dom.ElementNode {
			return
		}
		if isImage(n) {
			if e.loadedImages == nil {
				e.loadedImages = map[string]bool{}
			}
			e.loadedImages[imagesize.CleanURL(n.AttrOr("src", ""))] = true
		}
		for c := n.FirstChild; c != nil; c = c.Next {
			walk(c)
		}
	}
	walk(container)
	// The layouts with an image are not valid any more.
	e.imageEpoch++
}

// isImage reports whether n is an img element of HTML.
func isImage(n *dom.Node) bool {
	return n != nil && n.Type == dom.ElementNode && n.NS == dom.NSHTML && strings.EqualFold(n.Tag, "img")
}

// imageOf returns the natural size of the image of an img element in its
// state.
func (e *Engine) imageOf(img *dom.Node) naturalImage {
	src := imagesize.CleanURL(img.AttrOr("src", ""))
	if src == "" {
		return naturalImage{state: imageEmpty}
	}
	if !imagesize.IsDataURL(src) && !e.loadedImages[src] {
		return naturalImage{state: imagePending}
	}
	failed := naturalImage{state: imageFailed, w: brokenImageSize * luPerPx, h: brokenImageSize * luPerPx}
	var w, h float64
	if imagesize.IsDataURL(src) {
		size, err := imagesize.FromDataURL(src)
		if err != nil {
			return failed
		}
		w, h = size.Width, size.Height
	} else {
		if e.ImageSize == nil {
			return failed
		}
		var ok bool
		if w, h, ok = e.ImageSize(src); !ok {
			return failed
		}
	}
	return naturalImage{state: imageLoaded, w: luFromFloat(float32(w)), h: luFromFloat(float32(h)), ratio: w > 0 && h > 0}
}

// replacedBox is the size of a replaced element and its edges.
type replacedBox struct {
	// w and h are the content box.
	w, h                lu
	pad, border, margin boxEdges
}

// borderW and borderH are the border box.
func (r replacedBox) borderW() lu { return r.w + r.pad.horizontal() + r.border.horizontal() }
func (r replacedBox) borderH() lu { return r.h + r.pad.vertical() + r.border.vertical() }

// imageLength reads a size property of an img element: the declaration of
// the style, or for width and height the attribute, which is a
// presentational hint. The second result is false for auto and for a
// percentage that has no base.
func (e *Engine) imageLength(img *dom.Node, prop string, ts *textStyle, base lu) (lu, bool) {
	if v, ok := e.lengthProp(img, prop, ts, base); ok {
		return v, true
	}
	if _, declared := e.declared(img, prop); declared || (prop != "width" && prop != "height") {
		return 0, false
	}
	v, ok := img.GetAttr(prop)
	if !ok {
		return 0, false
	}
	v = strings.TrimSpace(v)
	if strings.HasSuffix(v, "%") {
		f, ok := parseCSSNumber(v[:len(v)-1])
		if !ok || base >= luInfinite {
			return 0, false
		}
		return lu(float64(base) * f / 100), true
	}
	// A dimension attribute is a number of pixels.
	f, ok := parseCSSNumber(strings.TrimSuffix(v, "px"))
	if !ok || f < 0 {
		return 0, false
	}
	return luFromPx(f), true
}

// replacedSize is the used size of an img element (CSS 2.1, sections 10.3.2,
// 10.4 and 10.6.2, as ComputeReplacedSize of LayoutNG does them). avail is
// the width of the containing block, or luInfinite when it is not known:
// a percentage is then auto. inline is true for an img on a line and false
// for a block-level one.
func (e *Engine) replacedSize(img *dom.Node, avail lu, inline bool) replacedBox {
	ts := e.textStyleOf(img)
	nat := e.imageOf(img)
	box := replacedBox{
		pad:    e.edges(img, "padding", "", ts, avail),
		border: e.borderEdges(img, ts),
		margin: e.edges(img, "margin", "", ts, avail),
	}
	borderBox := strings.EqualFold(strings.TrimSpace(e.lookupDeclared(img, "box-sizing")), "border-box")
	extraW := box.pad.horizontal() + box.border.horizontal()
	extraH := box.pad.vertical() + box.border.vertical()
	// length reads a size property as the size of the content box.
	length := func(prop string, base, extra lu) (lu, bool) {
		v, has := e.imageLength(img, prop, ts, base)
		if has && borderBox {
			v = max(0, v-extra)
		}
		return v, has
	}
	w, hasW := length("width", avail, extraW)
	h, hasH := length("height", luInfinite, extraH)
	minW, hasMinW := length("min-width", avail, extraW)
	maxW, hasMaxW := length("max-width", avail, extraW)
	minH, hasMinH := length("min-height", luInfinite, extraH)
	maxH, hasMaxH := length("max-height", luInfinite, extraH)
	clampW := func(v lu) lu {
		if hasMaxW {
			v = min(v, maxW)
		}
		if hasMinW {
			v = max(v, minW)
		}
		return v
	}
	clampH := func(v lu) lu {
		if hasMaxH {
			v = min(v, maxH)
		}
		if hasMinH {
			v = max(v, minH)
		}
		return v
	}
	// heightFor and widthFor keep the aspect ratio (LayoutUnit::MulDiv).
	heightFor := func(w lu) lu { return w.mulDiv(nat.h, nat.w) }
	widthFor := func(h lu) lu { return h.mulDiv(nat.w, nat.h) }
	if nat.state == imageFailed || nat.state == imageEmpty {
		// Not a replaced element: a box with the icon, or with nothing.
		switch {
		case inline && !(hasW && hasH):
			w, h = nat.w, nat.h
		case !inline:
			if !hasW {
				w = nat.w
				if avail < luInfinite {
					w = max(0, avail-extraW-box.margin.horizontal())
				}
			}
			if !hasH {
				h = nat.h
			}
		}
		box.w, box.h = clampW(w), clampH(h)
		return box
	}
	switch {
	case hasW && hasH:
		w, h = clampW(w), clampH(h)
	case hasW:
		w = clampW(w)
		if nat.ratio {
			h = heightFor(w)
		} else {
			h = nat.h
		}
		h = clampH(h)
	case hasH:
		h = clampH(h)
		if nat.ratio {
			w = widthFor(h)
		} else {
			w = nat.w
		}
		w = clampW(w)
	default:
		w, h = nat.w, nat.h
		if !nat.ratio {
			w, h = clampW(w), clampH(h)
			break
		}
		// Both are auto: the width takes its limits first, the height
		// follows by the ratio, then the height takes its limits and the
		// width follows.
		if cw := clampW(w); cw != w {
			w = cw
			h = heightFor(w)
		}
		if ch := clampH(h); ch != h {
			h = ch
			w = clampW(widthFor(h))
		}
	}
	box.w, box.h = w, h
	return box
}

// replacedIntrinsic returns the min-content and max-content widths of the
// margin box of an img element. An image whose width is a percentage adds
// nothing to the min-content width, and its natural width to the
// max-content width.
func (e *Engine) replacedIntrinsic(img *dom.Node) (minW, maxW lu) {
	box := e.replacedSize(img, luInfinite, false)
	outer := box.borderW() + box.margin.horizontal()
	if v, ok := e.declared(img, "width"); ok && strings.HasSuffix(strings.TrimSpace(v), "%") {
		return outer - box.w, outer
	}
	return outer, outer
}

// newImageItem makes the item of an img element that is inline: an atomic
// inline box on a line. Its baseline is the bottom of its margin box.
func (e *Engine) newImageItem(img *dom.Node, avail lu) *inlineItem {
	box := e.replacedSize(img, avail, true)
	it := e.newMarkItem(markAtomic, img, img.Parent, inlineEdge{})
	it.mark.atomicW = box.borderW() + box.margin.horizontal()
	it.mark.atomicH = box.borderH() + box.margin.vertical()
	it.mark.atomicBaseline = it.mark.atomicH
	it.mark.atomicMargin = box.margin
	return it
}

// layoutReplacedBlock is the box of an img element that is block-level,
// such as the img of a label after mermaid sets display: flex on it.
func (e *Engine) layoutReplacedBlock(img *dom.Node, avail lu) *htmlBox {
	box := e.replacedSize(img, avail, false)
	return &htmlBox{node: img, style: e.textStyleOf(img), w: box.borderW(), h: box.borderH()}
}
