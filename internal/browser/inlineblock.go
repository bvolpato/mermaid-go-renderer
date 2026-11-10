// Ported from Chromium 156.0.8078.4 (Blink), in part: an element with
// display: inline-block as an atomic inline box on a line. The functions of
// this file follow the functions of Blink that their comments name:
//
//   - LogicalBoxFragment::BaselineMetrics:
//     core/layout/logical_box_fragment.cc (BSD-3-Clause).
//   - BlockLayoutAlgorithm::PropagateBaselineFromLineBox and
//     PropagateBaselineFromBlockChild:
//     core/layout/block_layout_algorithm.cc (BSD-3-Clause).
//   - PhysicalBoxFragment::ForceInlineBaselineSynthesis:
//     core/layout/physical_box_fragment.h (BSD-3-Clause).
//   - LogicalLineBuilder::PlaceAtomicInline and PlaceLayoutResult:
//     core/layout/inline/logical_line_builder.cc (BSD-3-Clause).
//   - InlineLayoutStateStack::ApplyBaselineShift:
//     core/layout/inline/inline_box_state.cc (BSD-3-Clause).
//
// The paths are in third_party/blink/renderer. See NOTICE in this directory.

package browser

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

// This file has the atomic inline box of an element with display:
// inline-block, inline-table, inline-flex or inline-grid. The icon of Font
// Awesome is one: an i element with a width of 1.25em, a line height of 1
// and the character of the icon as generated content.
//
// The box is laid out as a block that shrinks to fit (layoutBlock). On the
// line it is one item, as an img is (image.go): it has the width and the
// height of its margin box, and it stands on the baseline of its parent
// with its own baseline. LogicalBoxFragment::BaselineMetrics: that
// baseline is the baseline of its last line. A box with no line, or one
// that scrolls its content, has it at the bottom edge of its margin box.
//
// The engine does not lay out a line of a foreignObject itself: it takes
// each child of a foreignObject as a block. So an element with display:
// inline-block whose line would be a line of the foreignObject stays a
// block that shrinks to fit, as it was before this file. Its width and its
// height are the same. Only its place on that line is not known.
//
// testdata/pagesheets-cases.json has the values of Chromium.

// isAtomicInlineDisplay reports whether a value of display makes an atomic
// inline box.
func isAtomicInlineDisplay(d string) bool {
	switch d {
	case "inline-block", "inline-table", "inline-flex", "inline-grid":
		return true
	}
	return false
}

// isAtomicInline reports whether an element with the display d is an atomic
// inline box on a line that the engine lays out: a line of an HTML block,
// and not a line of the foreignObject.
func (e *Engine) isAtomicInline(n *dom.Node, d string) bool {
	if isMathRoot(n) && (d == "inline-math" || d == "inline math") {
		return true
	}
	if !isAtomicInlineDisplay(d) {
		return false
	}
	for p := n.Parent; p != nil && p.Type == dom.ElementNode; p = p.Parent {
		if p.NS != dom.NSHTML {
			// The line is a line of the foreignObject.
			return false
		}
		if e.display(p) != "inline" {
			// The block that has the line.
			return true
		}
	}
	return false
}

// isOutOfFlow reports whether an element has a box that is not in the
// flow: position is absolute or fixed. Such a box takes no space in its
// container, so the size of a label does not depend on it. The engine does
// not place the box itself.
func (e *Engine) isOutOfFlow(n *dom.Node) bool {
	v, ok := e.declared(n, "position")
	if !ok {
		return false
	}
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "absolute" || v == "fixed"
}

// newInlineBlockItem makes the item of an element that is an atomic inline
// box with content of its own. avail is the width of the containing block,
// or luInfinite.
func (e *Engine) newInlineBlockItem(n *dom.Node, avail lu) *inlineItem {
	box := e.layoutBlock(n, avail)
	ts := e.textStyleOf(n)
	margin := e.edges(n, "margin", "", ts, avail)
	it := e.newMarkItem(markAtomic, n, n.Parent, inlineEdge{})
	mark := it.mark
	mark.atomicBox = box
	mark.atomicMargin = margin
	mark.atomicW = box.w + margin.horizontal()
	mark.atomicH = box.h + margin.vertical()
	// LogicalBoxFragment::BaselineMetrics: the baseline of the last line,
	// or the bottom edge of the margin box.
	mark.atomicBaseline = mark.atomicH
	if !e.forcesBaselineSynthesis(n) {
		if baseline, ok := e.lastBaseline(box); ok {
			mark.atomicBaseline = margin.top + baseline
		}
	}
	// InlineLayoutStateStack::ApplyBaselineShift for the box of the atomic
	// inline: its metrics are the part above its baseline and the part
	// below it.
	mark.atomicShift = e.baselineShift(n, ts, &inlineBoxMetrics{lineAscent: mark.atomicBaseline, lineDescent: mark.atomicH - mark.atomicBaseline})
	return it
}

// forcesBaselineSynthesis is PhysicalBoxFragment::ForceInlineBaselineSynthesis:
// a box that scrolls its content in the block direction has no baseline of
// its own for the line that it is on.
func (e *Engine) forcesBaselineSynthesis(n *dom.Node) bool {
	for _, prop := range []string{"overflow-y", "overflow"} {
		v, ok := e.declared(n, prop)
		if !ok {
			continue
		}
		// The second value of the overflow shorthand is overflow-y.
		fields := strings.Fields(strings.ToLower(v))
		if len(fields) == 0 {
			continue
		}
		switch fields[len(fields)-1] {
		case "hidden", "scroll", "auto", "overlay":
			return true
		}
		return false
	}
	return false
}

// lastBaseline returns the last baseline of a block, from the top of its
// border box: the baseline of its last line that has a line box, or the
// last baseline of its last child block that has one
// (PropagateBaselineFromLineBox and PropagateBaselineFromBlockChild with
// the inline-block baseline algorithm). The second result is false for a
// block with no baseline.
func (e *Engine) lastBaseline(b *htmlBox) (lu, bool) {
	if b.math != nil {
		return b.math.ascent, true
	}
	var baseline, top lu
	found := false
	// The lines and the child blocks are in the flow from top to bottom:
	// the last one that has a baseline is the lowest one.
	for _, line := range b.lines {
		if line.isEmpty() {
			// An empty line box has no baseline.
			continue
		}
		if !found || line.y >= top {
			baseline, top, found = line.y+line.baseline, line.y, true
		}
	}
	for _, child := range b.children {
		d := e.display(child.node)
		if e.isAtomicInline(child.node, d) {
			// The box is on a line, and the line has the baseline.
			continue
		}
		var cb lu
		ok := false
		switch {
		case d == "table" || d == "inline-table":
			// A table gives no baseline to an inline-block.
		case e.forcesBaselineSynthesis(child.node):
			// The bottom edge of the margin box of the child.
			margin := e.edges(child.node, "margin", "", e.textStyleOf(child.node), 0)
			cb, ok = child.h+margin.bottom, true
		default:
			cb, ok = e.lastBaseline(child)
		}
		if ok && (!found || child.y >= top) {
			baseline, top, found = child.y+cb, child.y, true
		}
	}
	return baseline, found
}

// placeAtomicBoxes gives the boxes of the atomic inline boxes of finished
// lines their place in the block that has the lines, and appends them to
// the children of that block. The lines have their final place in the
// block. Such a child is not a block of the flow: isAtomicInline tells the
// two apart.
func placeAtomicBoxes(children []*htmlBox, lines []*htmlLine) []*htmlBox {
	for _, line := range lines {
		for i := range line.marks {
			m := &line.marks[i]
			mark := m.item.mark
			if mark.kind != markAtomic || mark.atomicBox == nil {
				continue
			}
			// The mark has the margin box. The box is the border box.
			mark.atomicBox.x = line.x + m.x + mark.atomicMargin.left
			mark.atomicBox.y = line.y + m.top + mark.atomicMargin.top
			children = append(children, mark.atomicBox)
		}
	}
	return children
}
