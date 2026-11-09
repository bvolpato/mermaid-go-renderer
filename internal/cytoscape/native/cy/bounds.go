// Ported from cytoscape 3.34.0 (MIT license):
// src/collection/dimensions/bounds.mjs.

package cy

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// BB is a bounding box.
type BB struct {
	X1, Y1, X2, Y2, W, H float64
}

// bbOptions are the options of boundingBox.
type bbOptions struct {
	includeNodes, includeEdges                                   bool
	includeLabels, includeMainLabels                             bool
	includeSourceLabels, includeTargetLabels                     bool
	includeOverlays, includeUnderlays, includeOutlines, useCache bool
}

// defBbOpts is defBbOpts.
var defBbOpts = bbOptions{true, true, true, true, true, true, true, true, true, true}

// compoundBbOpts are the options of updateCompoundBounds for the children of
// a compound node whose labels take part in its size: { includeLabels: true,
// includeOverlays: false, useCache: false } over the defaults.
var compoundBbOpts = bbOptions{true, true, true, true, true, true, false, true, true, false}

// getKey is getKey of the bounding box options. The JavaScript reads the
// member incudeNodes, which does not exist, so the first bit is 0.
func getKey(opts *bbOptions) int {
	i := 0
	tf := func(val bool) int {
		v := 0
		if val {
			v = 1 << i
		}
		i++
		return v
	}
	key := 0
	key += tf(false)
	key += tf(opts.includeEdges)
	key += tf(opts.includeLabels)
	key += tf(opts.includeMainLabels)
	key += tf(opts.includeSourceLabels)
	key += tf(opts.includeTargetLabels)
	key += tf(opts.includeOverlays)
	key += tf(opts.includeOutlines)
	return key
}

var defBbOptsKey = getKey(&defBbOpts)

// updateCompoundBounds is eles.updateCompoundBounds(force): the size and the
// position of a compound node follow from its children.
func (cy *Instance) updateCompoundBounds(eles []*Element, force bool) {
	// not possible to do on non-compound graphs
	if !cy.hasCompoundNodes {
		return
	}

	// save cycles when batching -- but bounds will be stale (or not exist yet)
	if !force && cy.batching() {
		return
	}

	update := func(parent *Element) {
		if !parent.IsParent() {
			return
		}
		children := parent.children
		// compound-sizing-wrt-labels is include, and the minimum sizes and
		// their biases are 0.
		const minWidth, minHeight = 0.0, 0.0
		bb := cy.boundingBox(children, &compoundBbOpts)
		pos := &parent.position

		// if children take up zero area then keep position and fall back on stylesheet w/h
		if bb.W == 0 || bb.H == 0 {
			bb = BB{W: parent.NodeStyle.Width, H: parent.NodeStyle.Height}
			bb.X1 = pos.X - float64(bb.W/2)
			bb.X2 = pos.X + float64(bb.W/2)
			bb.Y1 = pos.Y - float64(bb.H/2)
			bb.Y2 = pos.Y + float64(bb.H/2)
		}
		// computeBiasValues gives 0 for the biases of 0.
		diffLeft, diffRight, diffTop, diffBottom := 0.0, 0.0, 0.0, 0.0
		// computePaddingValues: the unit of the padding is px.
		parent.autoPadding = parent.stylePadding()
		parent.hasAutoPadding = true
		parent.autoWidth = max(bb.W, minWidth)
		pos.X = (-diffLeft + bb.X1 + bb.X2 + diffRight) / 2
		parent.autoHeight = max(bb.H, minHeight)
		pos.Y = (-diffTop + bb.Y1 + bb.Y2 + diffBottom) / 2
	}
	for _, ele := range eles {
		if !ele.compoundBoundsClean || force {
			update(ele)
			if !cy.batching() {
				ele.compoundBoundsClean = true
			}
		}
	}
}

func noninf(x float64) float64 {
	if math.IsInf(x, 0) {
		return 0
	}
	return x
}

// updateBounds is updateBounds: it grows b by the box (x1, y1, x2, y2).
func updateBounds(b *BB, x1, y1, x2, y2 float64) {
	// don't update with zero area boxes
	if x2-x1 == 0 || y2-y1 == 0 {
		return
	}

	if x1 < b.X1 {
		b.X1 = x1
	}
	if x2 > b.X2 {
		b.X2 = x2
	}
	if y1 < b.Y1 {
		b.Y1 = y1
	}
	if y2 > b.Y2 {
		b.Y2 = y2
	}
	b.W = b.X2 - b.X1
	b.H = b.Y2 - b.Y1
}

func updateBoundsFromBox(b *BB, b2 *BB) {
	if b2 == nil {
		return
	}
	updateBounds(b, b2.X1, b2.Y1, b2.X2, b2.Y2)
}

// updateBoundsFromLabel is updateBoundsFromLabel for the main label of an
// element, without a rotation, margins, an outline or a border of the text.
func updateBoundsFromLabel(bounds *BB, ele *Element) {
	rstyle := &ele.rstyle
	label := ele.label()
	if label == "" {
		return
	}
	labelWidth := rstyle.labelWidth
	labelHeight := rstyle.labelHeight
	labelX := rstyle.labelX
	labelY := rstyle.labelY
	const marginX, marginY = 0.0, 0.0
	const outlineWidth, halfBorderWidth, padding = 0.0, 0.0, 0.0
	const marginOfError = 2.0 // expand to work around browser dimension inaccuracies

	lh := labelHeight
	lw := labelWidth
	lw2 := float64(lw / 2)
	lh2 := float64(lh / 2)
	var lx1, lx2, ly1, ly2 float64
	if !ele.isNode {
		lx1 = labelX - lw2
		lx2 = labelX + lw2
		ly1 = labelY - lh2
		ly2 = labelY + lh2
	} else {
		// text-halign is center.
		lx1 = labelX - lw2
		lx2 = labelX + lw2
		switch ele.NodeStyle.TextValign {
		case "top":
			ly1 = labelY - lh
			ly2 = labelY
		case "bottom":
			ly1 = labelY
			ly2 = labelY + lh
		default:
			ly1 = labelY - lh2
			ly2 = labelY + lh2
		}
	}

	// shift by margin and expand by outline and border
	leftPad := marginX - max(outlineWidth, halfBorderWidth) - padding - marginOfError
	rightPad := marginX + max(outlineWidth, halfBorderWidth) + padding + marginOfError
	topPad := marginY - max(outlineWidth, halfBorderWidth) - padding - marginOfError
	botPad := marginY + max(outlineWidth, halfBorderWidth) + padding + marginOfError
	lx1 += leftPad
	lx2 += rightPad
	ly1 += topPad
	ly2 += botPad

	updateBounds(bounds, lx1, ly1, lx2, ly2)
	updateBounds(ele.labelBounds.all, lx1, ly1, lx2, ly2)
}

// boundingBoxImpl is boundingBoxImpl with the default options.
func boundingBoxImpl(ele *Element) *BB {
	bounds := makeBoundingBox()
	isNode := ele.isNode
	rstyle := &ele.rstyle
	// bounds-expansion is 0, the element is displayed, and the overlay and
	// the underlay have no opacity.
	w := ele.styleWidth()
	wHalf := float64(w / 2)
	if isNode {
		pos := ele.Position()
		x := pos.X
		y := pos.Y
		w := ele.OuterWidth()
		halfW := float64(w / 2)
		h := ele.OuterHeight()
		halfH := float64(h / 2)

		// handle node dimensions
		ex1 := x - halfW
		ex2 := x + halfW
		ey1 := y - halfH
		ey2 := y + halfH
		updateBounds(bounds, ex1, ey1, ex2, ey2)
		// The outline has no width. The border of a compound node is on a
		// rectangle, which has no miter bounds: updateBoundsFromMiter reads
		// the position and the size, and changes nothing.
		if ele.styleBorderWidth() > 0 {
			ele.Position()
			ele.width()
			ele.height()
		}
	} else {
		// handle edge dimensions (rough box estimate)
		ex1 := min(rstyle.srcX, rstyle.midX, rstyle.tgtX)
		ex2 := max(rstyle.srcX, rstyle.midX, rstyle.tgtX)
		ey1 := min(rstyle.srcY, rstyle.midY, rstyle.tgtY)
		ey2 := max(rstyle.srcY, rstyle.midY, rstyle.tgtY)

		// take into account edge width
		ex1 -= wHalf
		ex2 += wHalf
		ey1 -= wHalf
		ey2 += wHalf
		updateBounds(bounds, ex1, ey1, ex2, ey2)

		// precise edges
		if ele.EdgeStyle.Segments {
			if rstyle.hasLinePts {
				for _, pt := range rstyle.linePts {
					ex1 = pt.X - wHalf
					ex2 = pt.X + wHalf
					ey1 = pt.Y - wHalf
					ey2 = pt.Y + wHalf
					updateBounds(bounds, ex1, ey1, ex2, ey2)
				}
			}
		}
		// The arrows have the shape none.
	}

	// always store the body bounds separately from the labels
	bbBody := &BB{}
	ele.bodyBounds = bbBody
	assignBoundingBox(bbBody, bounds)
	expandBoundingBox(bbBody, 1) // expand to work around browser dimension inaccuracies

	// overlay: the padding is 0
	updateBounds(bounds, bounds.X1-0, bounds.Y1-0, bounds.X2+0, bounds.Y2+0)

	// handle label dimensions
	ele.labelBounds.all = makeBoundingBox()
	updateBoundsFromLabel(bounds, ele)

	bounds.X1 = noninf(bounds.X1)
	bounds.Y1 = noninf(bounds.Y1)
	bounds.X2 = noninf(bounds.X2)
	bounds.Y2 = noninf(bounds.Y2)
	bounds.W = noninf(bounds.X2 - bounds.X1)
	bounds.H = noninf(bounds.Y2 - bounds.Y1)
	if bounds.W > 0 && bounds.H > 0 {
		// expand bounds by 1 because antialiasing can increase the visual/effective size by 1 on all sides
		expandBoundingBox(bounds, 1)
	}
	return bounds
}

// getBoundingBoxPosKey is the hash of the rounded positions that the bounds
// of an element depend on.
func getBoundingBoxPosKey(ele *Element) int32 {
	if !ele.isNode {
		p1 := ele.source.Position()
		p2 := ele.target.Position()
		return hashIntsArray([]float64{js.Round(p1.X), js.Round(p1.Y), js.Round(p2.X), js.Round(p2.Y)})
	}
	p := ele.Position()
	return hashIntsArray([]float64{js.Round(p.X), js.Round(p.Y)})
}

// cachedBoundingBoxImpl is cachedBoundingBoxImpl.
func cachedBoundingBoxImpl(ele *Element, opts *bbOptions) *BB {
	var bb *BB
	usingDefOpts := getKey(opts) == defBbOptsKey
	if ele.bbCache == nil {
		bb = boundingBoxImpl(ele)
		ele.bbCache = bb
		ele.bbCachePosKey = getBoundingBoxPosKey(ele)
	} else {
		bb = ele.bbCache
	}

	// not using def opts => need to build up bb from combination of sub bbs
	if !usingDefOpts {
		bb = makeBoundingBox()
		if opts.includeNodes && ele.isNode || opts.includeEdges && !ele.isNode {
			// The overlay bounds are the body bounds: the overlay has no
			// padding.
			updateBoundsFromBox(bb, ele.bodyBounds)
		}
		if opts.includeLabels {
			// includeMainLabels, includeSourceLabels and
			// includeTargetLabels are set.
			updateBoundsFromBox(bb, ele.labelBounds.all)
		}
		bb.W = bb.X2 - bb.X1
		bb.H = bb.Y2 - bb.Y1
	}
	return bb
}

// BoundingBox is eles.boundingBox() with the default options.
func (cy *Instance) BoundingBox(eles []*Element) BB {
	return cy.boundingBox(eles, nil)
}

// boundingBox is eles.boundingBox(options). options is nil for undefined.
func (cy *Instance) boundingBox(eles []*Element, options *bbOptions) BB {
	var bounds *BB
	useCache := options == nil || options.useCache
	// isDirty is memoized by the id of the element.
	type dirtyState struct{ known, dirty bool }
	memo := make(map[*Element]dirtyState, len(eles))
	isDirty := func(ele *Element) bool {
		if s := memo[ele]; s.known {
			return s.dirty
		}
		dirty := ele.bbCache == nil || ele.bbCachePosKey != getBoundingBoxPosKey(ele)
		memo[ele] = dirtyState{true, dirty}
		return dirty
	}

	// the main usecase is ele.boundingBox() for a single element with no/def options
	// specified s.t. the cache is used, so check for this case to make it faster by
	// avoiding the overhead of the rest of the function
	if useCache && len(eles) == 1 && !isDirty(eles[0]) {
		if options == nil {
			options = &defBbOpts
		}
		bounds = cachedBoundingBoxImpl(eles[0], options)
	} else {
		bounds = makeBoundingBox()
		if options == nil {
			options = &defBbOpts
		}
		opts := options

		// cache the isDirty state for all eles, edges first since they depend on node state
		for _, ele := range eles {
			if !ele.isNode {
				isDirty(ele)
			}
		}
		for _, ele := range eles {
			if ele.isNode {
				isDirty(ele)
			}
		}
		cy.recalculateRenderedStyle(eles, useCache)
		cy.updateCompoundBounds(eles, !useCache)
		for _, ele := range eles {
			if isDirty(ele) {
				ele.dirtyBoundingBoxCache()
			}
			updateBoundsFromBox(bounds, cachedBoundingBoxImpl(ele, opts))
		}
	}
	bounds.X1 = noninf(bounds.X1)
	bounds.Y1 = noninf(bounds.Y1)
	bounds.X2 = noninf(bounds.X2)
	bounds.Y2 = noninf(bounds.Y2)
	bounds.W = noninf(bounds.X2 - bounds.X1)
	bounds.H = noninf(bounds.Y2 - bounds.Y1)
	return *bounds
}
