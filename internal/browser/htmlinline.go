// Ported from Chromium 156.0.8078.4 (Blink), in part: the boxes of the
// inline elements of HTML. The functions of this file follow the functions
// of Blink that their comments name:
//
//   - LayoutInline::QuadsForSelf: core/layout/layout_inline.cc
//     (LGPL-2.0-or-later).
//   - LayoutObject::AbsoluteQuads: core/layout/layout_object.cc
//     (LGPL-2.0-or-later).
//   - Element::GetBoundingClientRect: core/dom/element.cc
//     (LGPL-2.0-or-later).
//   - ComputedStyle::ComputedFontSizeAsFixed and ComputedLineHeightAsFixed:
//     core/style/computed_style.h and computed_style.cc
//     (LGPL-2.0-or-later).
//   - FontMetrics::FixedAscent and FixedDescent:
//     platform/fonts/font_metrics.h (LGPL-2.0-or-later).
//   - MinimumValueForLength: platform/geometry/length_functions.cc
//     (LGPL-2.0-or-later).
//   - InlineBoxState::ComputeTextMetrics and
//     InlineLayoutStateStack::ApplyBaselineShift:
//     core/layout/inline/inline_box_state.cc (BSD-3-Clause).
//   - BoxInfo::ShouldCreateBoxFragmentForChild:
//     core/layout/inline/inline_items_builder.cc (BSD-3-Clause).
//   - LineInfo::IsEmptyLine: core/layout/inline/line_info.h (BSD-3-Clause).
//   - ShapeResultView::SnappedWidth and ShapingLineBreaker::ShapeLine:
//     platform/fonts/shaping (BSD-3-Clause).
//   - gfx::RectF::Union: ui/gfx/geometry/rect_f.cc of Chromium
//     (BSD-3-Clause).
//
// The Blink paths are in third_party/blink/renderer. The license of each
// file is the license in its header at that version. See NOTICE in this
// directory.

package browser

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// This file has the boxes of the inline elements of HTML: what
// getBoundingClientRect returns for a span, a strong, a br and the like.
// It follows LayoutNG of Blink:
//
//   - InlineBoxState::ComputeTextMetrics: the box of an inline element, and
//     the box of a text, has the height of the ascent plus the descent of
//     the primary font of the element. It does not have the height of the
//     line. The half leading only moves it in the line.
//   - LayoutInline::QuadsForSelf: an inline element has one rectangle for
//     each of its boxes. An inline element with no box of its own (a culled
//     inline box) has the rectangles of its children: its texts, its inline
//     elements and its blocks.
//   - InlineItemsBuilder: an inline element has a box of its own when it
//     has padding, a border, a margin or a background, when it has no
//     content, or when a child inline element has a margin or other font
//     metrics (BoxInfo::ShouldCreateBoxFragmentForChild).
//   - Element::GetBoundingClientRect unites the rectangles with
//     gfx::RectF::Union, which drops an empty rectangle.
//
// testdata/htmlbox-cases.json has the values of Chromium.

// markKind is the kind of an item with no characters.
type markKind uint8

const (
	// markBreak is a br element. Its box is at the end of its line.
	markBreak markKind = iota + 1
	// markEmpty is an inline element with no content.
	markEmpty
	// markOpen is the start edge of an inline box: margin, border, padding.
	markOpen
	// markClose is the end edge of an inline box.
	markClose
	// markAtomic is an atomic inline box: a replaced element such as an
	// img. It is one box on the line, with its own width and height.
	markAtomic
)

// inlineEdge is the space that an inline box takes in a line beside its
// content. start and end are the border plus the padding.
type inlineEdge struct {
	startMargin, start, end, endMargin lu
}

func (g inlineEdge) width() lu { return g.startMargin + g.start + g.end + g.endMargin }

// inlineMark describes an item with no characters.
type inlineMark struct {
	kind markKind
	el   *dom.Node
	edge inlineEdge
	// atomicW and atomicH are the margin box of an atomic inline box.
	// atomicBaseline is the distance from the top of the margin box to
	// the baseline of the box, and atomicMargin has its margins.
	atomicW, atomicH, atomicBaseline lu
	atomicMargin                     boxEdges
	// atomicBox is the layout of an atomic inline box that has content of
	// its own, and atomicShift is the baseline shift that its own
	// vertical-align gives. See inlineblock.go.
	atomicBox   *htmlBox
	atomicShift lu
}

// advance is the width that the mark takes on its line.
func (m *inlineMark) advance() lu { return m.edge.width() + m.atomicW }

// lineMark is the place of a mark on a line. at is the index of the
// fragment that follows the mark. x is relative to the content box of the
// block, top is relative to the top of the line, as for an htmlFrag.
type lineMark struct {
	item        *inlineItem
	at          int
	x           lu
	top, height lu
}

func (e *Engine) newMarkItem(kind markKind, el, owner *dom.Node, edge inlineEdge) *inlineItem {
	return &inlineItem{node: el, owner: owner, style: e.textStyleOf(owner), mark: &inlineMark{kind: kind, el: el, edge: edge}}
}

func marksWidth(marks []lineMark) lu {
	var w lu
	for i := range marks {
		w += marks[i].item.mark.advance()
	}
	return w
}

// takeTrailingMarks removes the marks after the last fragment of a line
// that go with the content that follows them: a start edge and an empty
// box. It returns them for the next line.
func takeTrailingMarks(line *htmlLine) []lineMark {
	n := len(line.marks)
	for n > 0 {
		m := line.marks[n-1]
		if m.at < len(line.frags) || (m.item.mark.kind != markOpen && m.item.mark.kind != markEmpty) {
			break
		}
		n--
	}
	moved := append([]lineMark(nil), line.marks[n:]...)
	line.marks = line.marks[:n]
	for i := range moved {
		moved[i].at = 0
	}
	return moved
}

// isEmpty is LineInfo::IsEmptyLine: the line has no text, no forced break
// and no edge with a width. Such a line has no line box.
func (l *htmlLine) isEmpty() bool {
	if len(l.frags) > 0 || l.forced {
		return false
	}
	for i := range l.marks {
		if m := l.marks[i].item.mark; m.kind == markBreak || m.kind == markAtomic || m.advance() != 0 {
			return false
		}
	}
	return true
}

// hasLayoutChild reports whether an element has a child that layout sees: a
// text or an element.
func hasLayoutChild(n *dom.Node) bool {
	for c := n.FirstChild; c != nil; c = c.Next {
		if c.Type == dom.TextNode || c.Type == dom.ElementNode {
			return true
		}
	}
	return false
}

// hasLayoutChild is hasLayoutChild with the boxes of the pseudo-elements
// ::before and ::after, which are children for layout.
func (e *Engine) hasLayoutChild(n *dom.Node) bool {
	if hasLayoutChild(n) {
		return true
	}
	before, after := e.pseudoBoxes(n)
	return before != nil || after != nil
}

// inlineEdges returns the margins, the borders and the padding of an inline
// element in the direction of the line.
func (e *Engine) inlineEdges(n *dom.Node) inlineEdge {
	ts := e.textStyleOf(n)
	pad := e.edges(n, "padding", "", ts, 0)
	border := e.borderEdges(n, ts)
	margin := e.edges(n, "margin", "", ts, 0)
	return inlineEdge{startMargin: margin.left, start: border.left + pad.left, end: pad.right + border.right, endMargin: margin.right}
}

// inlineBoxMetrics is the part of InlineBoxState that the position of an
// inline box needs.
type inlineBoxMetrics struct {
	// ascent and descent are FontMetrics::FixedAscent and FixedDescent of
	// the primary font: the box of the text.
	ascent, descent lu
	// lineAscent and lineDescent have the half leading of the line height.
	lineAscent, lineDescent lu
	// shift moves the baseline of the box from the baseline of the line.
	// A positive shift is down.
	shift lu
}

func (e *Engine) primaryMetrics(ts *textStyle) font.Metrics {
	if p := e.chainOf(ts).Primary(); p != nil {
		return p.Metrics(ts.fontSize())
	}
	return font.Metrics{}
}

// fixedFontSize is ComputedStyle::ComputedFontSizeAsFixed.
func fixedFontSize(ts *textStyle) lu {
	return luFromFloatRound(ts.size)
}

// inlineBoxOf returns the metrics of the inline box of el in a line of
// container. For the container itself they are the metrics of the root
// inline box, which is the strut of the line.
func (e *Engine) inlineBoxOf(el, container *dom.Node) inlineBoxMetrics {
	ts := e.textStyleOf(el)
	m := e.primaryMetrics(ts)
	out := inlineBoxMetrics{ascent: luFromFloatRound(m.Ascent), descent: luFromFloatRound(m.Descent)}
	out.lineAscent, out.lineDescent = lineExtent(ts, m)
	if el == container || el.Parent == nil || el.Parent.Type != dom.ElementNode {
		return out
	}
	parent := e.inlineBoxOf(el.Parent, container)
	out.shift = parent.shift + e.baselineShift(el, ts, &out)
	return out
}

// baselineShift is the switch of InlineLayoutStateStack::ApplyBaselineShift
// for the values of vertical-align that do not need the size of the line:
// sub, super, middle, a length and a percentage.
func (e *Engine) baselineShift(el *dom.Node, ts *textStyle, box *inlineBoxMetrics) lu {
	v, ok := e.declared(el, "vertical-align")
	v = strings.ToLower(strings.TrimSpace(v))
	if !ok && el.NS == dom.NSHTML {
		// The style sheet of the user agent.
		switch el.Tag {
		case "sub":
			v = "sub"
		case "sup":
			v = "super"
		}
	}
	parentStyle := e.textStyleOf(el.Parent)
	switch v {
	case "", "baseline", "top", "bottom", "text-top", "text-bottom":
		return 0
	case "sub":
		return fixedFontSize(parentStyle)/5 + luPerPx
	case "super":
		return -(fixedFontSize(parentStyle)/3 + luPerPx)
	case "middle":
		shift := (box.lineAscent - box.lineDescent) / 2
		return shift - luFromFloatRound(e.primaryMetrics(parentStyle).XHeight/2)
	}
	if strings.HasSuffix(v, "%") {
		f, ok := parseCSSNumber(v[:len(v)-1])
		if !ok {
			return 0
		}
		lineHeight := computedLineHeight(ts, e.primaryMetrics(ts))
		return -luFromFloat(float32(lineHeight.f32() * float32(f) / 100))
	}
	if css.IsMathFunction(v) {
		// A math function can have a percentage of the line height
		// (ValueForLength of a calculated length).
		c, ok := e.evalCalc(v, ts)
		if !ok || c.IsNumber {
			return 0
		}
		px := float32(c.Px)
		if c.HasPercent {
			lineHeight := computedLineHeight(ts, e.primaryMetrics(ts))
			px += float32(lineHeight.f32() * float32(c.Percent) / 100)
		}
		return -luFromFloat(px)
	}
	if px, ok := e.cssLength(v, ts, false); ok {
		return -luFromFloat(float32(px))
	}
	return 0
}

// computedLineHeight is ComputedStyle::ComputedLineHeightAsFixed. A number
// is a percentage of the font size: MinimumValueForLength multiplies the
// font size, as a LayoutUnit, in float.
func computedLineHeight(ts *textStyle, m font.Metrics) lu {
	switch ts.lhKind {
	case lhNumber:
		percent := float32(ts.lhValue * 100)
		return luFromFloat(float32(fixedFontSize(ts).f32()*percent) / 100)
	case lhLength:
		// A length is rounded to a LayoutUnit (LayoutUnit::FromFloatRound).
		return luFromFloatRound(ts.lhValue)
	}
	return luFromFloatRound(m.LineSpacing)
}

// luRect is a rectangle in LayoutUnits.
type luRect struct{ x, y, w, h lu }

func (r luRect) isEmpty() bool { return r.w <= 0 || r.h <= 0 }

// unionQuads is the loop of Element::GetBoundingClientRect over the
// rectangles of an element, with gfx::RectF::Union: an empty rectangle does
// not add to the result, and it is the result only when no rectangle with
// an area comes after it.
func unionQuads(quads []luRect) luRect {
	var out luRect
	for _, q := range quads {
		switch {
		case out.isEmpty():
			out = q
		case q.isEmpty():
		default:
			x0, y0 := min(out.x, q.x), min(out.y, q.y)
			x1, y1 := max(out.x+out.w, q.x+q.w), max(out.y+out.h, q.y+q.h)
			out = luRect{x0, y0, x1 - x0, y1 - y0}
		}
	}
	return out
}

// quadCollector finds the rectangles of the inline elements of one block.
type quadCollector struct {
	e         *Engine
	root      *htmlRootLayout
	box       *htmlBox
	container *dom.Node
	ox, oy    lu // origin of the box in the foreignObject
	// contentX and contentW are the content box of the block.
	contentX, contentW lu
}

// blockContainerOf returns the nearest ancestor of n that has a block box.
func (root *htmlRootLayout) blockContainerOf(n *dom.Node) *dom.Node {
	for p := n.Parent; p != nil && p.Type == dom.ElementNode; p = p.Parent {
		if _, ok := root.index[p]; ok {
			return p
		}
	}
	return nil
}

// htmlQuads returns the rectangles of an HTML element in a foreignObject,
// in the coordinate system of the foreignObject: what
// LayoutObject::AbsoluteQuads gives before the mapping to the page. The
// second result is false for an element that is not rendered.
func (e *Engine) htmlQuads(n *dom.Node) ([]luRect, bool) {
	fo := foreignObjectOf(n)
	if fo == nil {
		return nil, false
	}
	if has, hidden := e.svgLayoutState(fo); !has || hidden {
		// The foreignObject has no layout object: see norender.go.
		return nil, false
	}
	for p := n; p != nil && p != fo; p = p.Parent {
		if e.display(p) == "none" || e.isOutOfFlow(p) {
			// An element with no box, or with a box that is not in the
			// flow, which the engine does not place.
			return nil, false
		}
	}
	root := e.layoutForeignObject(fo)
	if b, found := root.index[n]; found {
		o := root.origin[b]
		return []luRect{{o[0], o[1], b.w, b.h}}, true
	}
	container := root.blockContainerOf(n)
	if container == nil {
		return nil, false
	}
	box := root.index[container]
	o := root.origin[box]
	ts := e.textStyleOf(container)
	pad := e.edges(container, "padding", "", ts, 0)
	border := e.borderEdges(container, ts)
	c := &quadCollector{e: e, root: root, box: box, container: container, ox: o[0], oy: o[1]}
	c.contentX = o[0] + border.left + pad.left
	c.contentW = box.w - border.horizontal() - pad.horizontal()
	var quads []luRect
	if n.NS == dom.NSHTML && n.Tag == "br" {
		quads = c.markQuads(n)
	} else {
		quads = c.quads(n)
	}
	if len(quads) == 0 {
		// LayoutInline::QuadsForSelf: an inline element with no box has an
		// empty rectangle where its block starts a line with no width.
		x := c.contentX + alignOffset(ts.textAlign, c.contentW, 0)
		quads = []luRect{{x, o[1] + border.top + pad.top, 0, 0}}
	}
	return quads, true
}

// isBlockChild reports whether a child of an inline element is a block in
// the flow of the container.
func (c *quadCollector) isBlockChild(n *dom.Node) bool {
	if n.Type != dom.ElementNode {
		return false
	}
	_, ok := c.root.index[n]
	return ok && !c.e.isAtomicInline(n, c.e.display(n))
}

// isAtomicChild reports whether a child of an inline element is an atomic
// inline box with a layout of its own: an element with display:
// inline-block and the like.
func (c *quadCollector) isAtomicChild(n *dom.Node) bool {
	if n.Type != dom.ElementNode {
		return false
	}
	_, ok := c.root.index[n]
	return ok && c.e.isAtomicInline(n, c.e.display(n))
}

// skipsBetweenBlocks reports whether a node does not end a run of blocks:
// white space, a comment or an element that is not displayed.
func (c *quadCollector) skipsBetweenBlocks(n *dom.Node) bool {
	switch n.Type {
	case dom.TextNode:
		return strings.TrimSpace(n.Data) == ""
	case dom.ElementNode:
		return c.e.display(n) == "none"
	}
	return true
}

// blockRun returns the last block of the run of blocks that starts at
// first.
func (c *quadCollector) blockRun(first *dom.Node) *dom.Node {
	last := first
	for nx := first.Next; nx != nil; nx = nx.Next {
		if c.isBlockChild(nx) {
			last = nx
			continue
		}
		if !c.skipsBetweenBlocks(nx) {
			break
		}
	}
	return last
}

// anonymousBlockQuad is the rectangle of the anonymous block that holds a
// run of blocks in an inline element (a block-in-inline). The margins of
// the blocks collapse through it, so it starts at the top border edge of
// the first block and ends at the bottom border edge of the last block. It
// is as wide as the content box of the container.
func (c *quadCollector) anonymousBlockQuad(first, last *dom.Node) luRect {
	fb, lb := c.root.index[first], c.root.index[last]
	top := c.root.origin[fb][1]
	bottom := c.root.origin[lb][1] + lb.h
	return luRect{c.contentX, top, c.contentW, max(bottom-top, 0)}
}

// hasInlineContent reports whether a node makes content on a line: a text
// with a character that is not white space, a br, an atomic inline box, or
// an inline element with such content.
func (c *quadCollector) hasInlineContent(n *dom.Node) bool {
	switch n.Type {
	case dom.TextNode:
		return strings.TrimSpace(n.Data) != ""
	case dom.ElementNode:
		if c.e.display(n) == "none" || c.isBlockChild(n) {
			return false
		}
		if isImage(n) || c.isAtomicChild(n) || (n.NS == dom.NSHTML && n.Tag == "br") {
			return true
		}
		before, after := c.e.pseudoBoxes(n)
		if (before != nil && c.hasInlineContent(before)) || (after != nil && c.hasInlineContent(after)) {
			return true
		}
		for ch := n.FirstChild; ch != nil; ch = ch.Next {
			if c.hasInlineContent(ch) {
				return true
			}
		}
	}
	return false
}

// afterBlockQuads returns the rectangle of the inline element on the line
// after a run of blocks that ends at last, when the element has no content
// after the run: the element ends on an empty line below the blocks. An
// empty line has no size, and text-align gives its place. This rectangle is
// the rectangle of the element when the blocks have no width or no height
// (gfx::RectF::Union).
func (c *quadCollector) afterBlockQuads(last *dom.Node) []luRect {
	for nx := last.Next; nx != nil; nx = nx.Next {
		if c.isBlockChild(nx) || c.hasInlineContent(nx) {
			return nil
		}
	}
	lb := c.root.index[last]
	x := c.contentX
	switch c.e.textStyleOf(c.container).textAlign {
	case "center", "-webkit-center":
		x += c.contentW / 2
	case "right", "end", "-webkit-right":
		x += c.contentW
	}
	return []luRect{{x, c.root.origin[lb][1] + lb.h, 0, 0}}
}

// hasOwnBox reports whether an inline element has a box fragment of its
// own. Without one its rectangles are the rectangles of its children.
func (c *quadCollector) hasOwnBox(el *dom.Node) bool {
	e := c.e
	if !e.hasLayoutChild(el) {
		return true
	}
	ts := e.textStyleOf(el)
	if e.inlineEdges(el).width() != 0 {
		return true
	}
	if pad := e.edges(el, "padding", "", ts, 0); pad.vertical() != 0 {
		return true
	}
	if border := e.borderEdges(el, ts); border.vertical() != 0 {
		return true
	}
	for _, prop := range []string{"background-color", "background", "background-image"} {
		if v, ok := e.declared(el, prop); ok {
			if v = strings.ToLower(strings.TrimSpace(v)); v != "" && v != "none" && v != "transparent" {
				return true
			}
		}
	}
	// BoxInfo::ShouldCreateBoxFragmentForChild.
	own := e.primaryMetrics(ts)
	forChild := func(ch *dom.Node) bool {
		if ch.Type != dom.ElementNode || c.isBlockChild(ch) {
			return false
		}
		if c.isAtomicChild(ch) {
			// InlineItemsBuilder::AppendAtomicInline: an inline box with an
			// atomic inline box in it has a box fragment, because the two
			// can have different heights.
			return true
		}
		if e.display(ch) != "inline" || (ch.NS == dom.NSHTML && ch.Tag == "br") {
			return false
		}
		cts := e.textStyleOf(ch)
		margin := e.edges(ch, "margin", "", cts, 0)
		if margin.left != 0 || margin.right != 0 || margin.top != 0 || margin.bottom != 0 {
			return true
		}
		cm := e.primaryMetrics(cts)
		return luFromFloatRound(cm.Ascent) != luFromFloatRound(own.Ascent) || luFromFloatRound(cm.Descent) != luFromFloatRound(own.Descent)
	}
	// The boxes of the pseudo-elements are children too.
	before, after := e.pseudoBoxes(el)
	if (before != nil && forChild(before)) || (after != nil && forChild(after)) {
		return true
	}
	for ch := el.FirstChild; ch != nil; ch = ch.Next {
		if forChild(ch) {
			return true
		}
	}
	return false
}

// quads returns the rectangles of an inline element.
func (c *quadCollector) quads(el *dom.Node) []luRect {
	if c.hasOwnBox(el) {
		return c.boxQuads(el)
	}
	var out []luRect
	// pseudoQuads returns the rectangles of the box of a pseudo-element.
	pseudoQuads := func(p *dom.Node) []luRect {
		switch {
		case c.isBlockChild(p):
			return []luRect{c.anonymousBlockQuad(p, p)}
		case c.isAtomicChild(p):
			return c.markQuads(p)
		}
		return c.quads(p)
	}
	// The boxes of the pseudo-elements are the first and the last child.
	before, after := c.e.pseudoBoxes(el)
	if before != nil {
		out = append(out, pseudoQuads(before)...)
	}
	for ch := el.FirstChild; ch != nil; ch = ch.Next {
		switch ch.Type {
		case dom.TextNode:
			out = append(out, c.textQuads(ch)...)
		case dom.ElementNode:
			switch {
			case c.e.display(ch) == "none":
			case c.isBlockChild(ch):
				last := c.blockRun(ch)
				out = append(out, c.anonymousBlockQuad(ch, last))
				out = append(out, c.afterBlockQuads(last)...)
				ch = last
			case c.isAtomicChild(ch), ch.NS == dom.NSHTML && ch.Tag == "br":
				out = append(out, c.markQuads(ch)...)
			default:
				out = append(out, c.quads(ch)...)
			}
		}
	}
	if after != nil {
		out = append(out, pseudoQuads(after)...)
	}
	return out
}

// textQuads returns the rectangles of the fragments of a text node.
func (c *quadCollector) textQuads(text *dom.Node) []luRect {
	var out []luRect
	for _, line := range c.box.lines {
		for i := range line.frags {
			if f := &line.frags[i]; f.item.node == text {
				out = append(out, luRect{c.ox + line.x + f.x, c.oy + line.y + f.top, f.width, f.height})
			}
		}
	}
	return out
}

// verticalEdges returns the padding plus the border above and below the
// content of an inline box.
func (c *quadCollector) verticalEdges(el *dom.Node) (top, bottom lu) {
	ts := c.e.textStyleOf(el)
	pad := c.e.edges(el, "padding", "", ts, 0)
	border := c.e.borderEdges(el, ts)
	return pad.top + border.top, pad.bottom + border.bottom
}

// markQuads returns the rectangle of a br element or of an inline element
// with no content.
func (c *quadCollector) markQuads(el *dom.Node) []luRect {
	var out []luRect
	for _, line := range c.box.lines {
		for i := range line.marks {
			m := &line.marks[i]
			mark := m.item.mark
			if mark.el != el || (mark.kind != markBreak && mark.kind != markEmpty && mark.kind != markAtomic) {
				continue
			}
			if mark.kind == markAtomic {
				// The border box of the atomic inline box.
				mg := mark.atomicMargin
				out = append(out, luRect{c.ox + line.x + m.x + mg.left, c.oy + line.y + m.top + mg.top, mark.atomicW - mg.horizontal(), mark.atomicH - mg.vertical()})
				continue
			}
			r := luRect{c.ox + line.x + m.x + mark.edge.startMargin, c.oy + line.y + m.top, mark.edge.start + mark.edge.end, m.height}
			if mark.kind == markEmpty && !line.isEmpty() {
				top, bottom := c.verticalEdges(el)
				r.y -= top
				r.h += top + bottom
			}
			out = append(out, r)
		}
	}
	return out
}

// boxQuads returns the rectangles of an inline element that has a box of
// its own: one box for each line that has a part of the element, and the
// anonymous block of each run of blocks in the element.
func (c *quadCollector) boxQuads(el *dom.Node) []luRect {
	if !c.e.hasLayoutChild(el) {
		return c.markQuads(el)
	}
	var out []luRect
	var done map[*htmlLine]bool
	visitLine := func(line *htmlLine) {
		if done[line] {
			return
		}
		if done == nil {
			done = map[*htmlLine]bool{}
		}
		done[line] = true
		out = append(out, c.lineBoxQuad(el, line))
	}
	var walk func(n *dom.Node)
	// visitElement visits the lines that have a part of an inline-level
	// element in el.
	visitElement := func(ch *dom.Node) {
		for _, line := range c.box.lines {
			for i := range line.marks {
				if line.marks[i].item.mark.el == ch {
					visitLine(line)
					break
				}
			}
		}
		if !c.isAtomicChild(ch) {
			walk(ch)
		}
	}
	walk = func(n *dom.Node) {
		// The boxes of the pseudo-elements are the first and the last
		// child.
		before, after := c.e.pseudoBoxes(n)
		if before != nil {
			visitElement(before)
		}
		for ch := n.FirstChild; ch != nil; ch = ch.Next {
			switch ch.Type {
			case dom.TextNode:
				for _, line := range c.box.lines {
					for i := range line.frags {
						if line.frags[i].item.node == ch {
							visitLine(line)
							break
						}
					}
				}
			case dom.ElementNode:
				switch {
				case c.e.display(ch) == "none":
				case c.isBlockChild(ch):
					last := c.blockRun(ch)
					out = append(out, c.anonymousBlockQuad(ch, last))
					out = append(out, c.afterBlockQuads(last)...)
					ch = last
				default:
					visitElement(ch)
				}
			}
		}
		if after != nil {
			visitElement(after)
		}
	}
	// The edges of the element itself.
	for _, line := range c.box.lines {
		for i := range line.marks {
			if m := line.marks[i].item.mark; m.el == el && m.kind == markOpen {
				visitLine(line)
			}
		}
	}
	walk(el)
	for _, line := range c.box.lines {
		for i := range line.marks {
			if m := line.marks[i].item.mark; m.el == el && m.kind == markClose {
				visitLine(line)
			}
		}
	}
	return out
}

// lineBoxQuad is the box of an inline element on one line: from the start
// of its first content to the end of its last content on the line, and as
// high as the ascent plus the descent of its font, with its padding and its
// border.
func (c *quadCollector) lineBoxQuad(el *dom.Node, line *htmlLine) luRect {
	have := false
	var x0, x1 lu
	add := func(a, b lu) {
		if !have {
			x0, x1, have = a, b, true
			return
		}
		x0, x1 = min(x0, a), max(x1, b)
	}
	for i := range line.frags {
		if f := &line.frags[i]; isDescendantOf(f.item.owner, el) {
			add(f.x, f.x+f.width)
		}
	}
	for i := range line.marks {
		m := &line.marks[i]
		mark := m.item.mark
		switch {
		case mark.el == el && mark.kind == markOpen:
			add(m.x+mark.edge.startMargin, m.x+mark.edge.width())
		case mark.el == el && mark.kind == markClose:
			add(m.x, m.x+mark.edge.end)
		case isDescendantOf(m.item.owner, el):
			add(m.x, m.x+mark.advance())
		}
	}
	r := luRect{c.ox + line.x + x0, c.oy + line.y, x1 - x0, 0}
	if line.isEmpty() {
		return r
	}
	m := c.e.inlineBoxOf(el, c.container)
	top, bottom := c.verticalEdges(el)
	r.y += line.baseline + m.shift - m.ascent - top
	r.h = m.ascent + m.descent + top + bottom
	return r
}

// fragmentWidth is the width of the part [lo, hi) of a text item on a line.
//
// For a line of a block it is the sum of the advances of its glyphs,
// rounded up to a LayoutUnit (ShapeResultView::SnappedWidth).
//
// For the min-content width Blink does not make lines. It measures each
// word of the text as the difference of the positions of its two ends in
// the shape result of the text, and each position is rounded up to a
// LayoutUnit. So a word that does not start the text can be one LayoutUnit
// narrower than the same word on a line of its own. A table with a width
// is as wide as its widest word when the word is wider than the width, so
// the width of a label that wraps depends on this.
// testdata/mincontent-cases.json has the values of Chromium.
func (lb *lineBreaker) fragmentWidth(it *inlineItem, lo, hi int) lu {
	if lb.minContent {
		return it.snappedPosition(hi) - it.snappedPosition(lo)
	}
	if lo == 0 && hi == len(it.runes) {
		return it.width
	}
	return luFromFloatCeil(float64(it.glyphRangeWidth(lo, hi)))
}

// snappedPosition is the position of the character k of a text item from
// the start of the item, rounded up to a LayoutUnit.
func (it *inlineItem) snappedPosition(k int) lu {
	return luFromFloatCeil(float64(it.glyphRangeWidth(0, k)))
}

// lineStartKey names a fragment that starts a line.
type lineStartKey struct {
	item *inlineItem
	lo   int
}

// lineStartWidth is fragmentWidth for a fragment that starts a line in the
// middle of a text item. The first glyph of the line can have a part of
// its advance from the glyph before it: the kern machine of HarfBuzz gives
// half of the kerning of a pair to the second glyph, as for a space and a
// W in Times. That glyph is on the line before, so Blink shapes the start
// of the line again (ShapingLineBreaker::ShapeLine, when the start is not
// safe to break), and the first glyph has the advance of a glyph with
// nothing before it. testdata/linewrap-cases.json has the values of
// Chromium.
func (lb *lineBreaker) lineStartWidth(it *inlineItem, lo, hi int) lu {
	if lb.minContent || lo == 0 || lo >= hi {
		return lb.fragmentWidth(it, lo, hi)
	}
	key := lineStartKey{it, lo}
	delta, ok := lb.startDelta[key]
	if !ok {
		delta = lb.e.lineStartDelta(it, lo)
		if lb.startDelta == nil {
			lb.startDelta = map[lineStartKey]float64{}
		}
		lb.startDelta[key] = delta
	}
	if delta == 0 {
		return lb.fragmentWidth(it, lo, hi)
	}
	return max(luFromFloatCeil(float64(float32(float64(it.glyphRangeWidth(lo, hi))+delta))), 0)
}

// lineStartDelta is the change of the advance of the glyphs of the rune lo
// of an item when a line starts at that rune: their advance when the text
// is shaped from that rune, minus their advance in the whole text.
func (e *Engine) lineStartDelta(it *inlineItem, lo int) float64 {
	var context float64
	glyphs := 0
	for i := range it.glyphs {
		if it.glyphs[i].runeIdx == lo {
			context += it.glyphs[i].glyph.Advance
			glyphs++
		}
	}
	if glyphs == 0 {
		return 0
	}
	// The rune and the one after it: the kerning with the next glyph stays.
	window := it.runes[lo:min(lo+2, len(it.runes))]
	var alone float64
	for _, run := range e.chainOf(it.style).ShapeText(window, it.style.fontSize()) {
		for _, g := range run.Glyphs {
			if run.Start+g.Cluster == 0 {
				alone += g.Advance + it.style.letterSpacing
			}
		}
	}
	return alone - context
}
