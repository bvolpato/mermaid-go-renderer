// Ported from Chromium 156.0.8078.4 (Blink), in part: block and inline
// layout of HTML. The file is a small layout engine of its own. Some of its
// steps follow the functions of LayoutNG that their comments name:
//
//   - InlineItemsBuilder: core/layout/inline/inline_items_builder.cc.
//   - LineBreaker: core/layout/inline/line_breaker.cc.
//   - InlineBoxState: core/layout/inline/inline_box_state.cc.
//   - InlineNode::ShapeText: core/layout/inline/inline_node.cc.
//   - LineInfo: core/layout/inline/line_info.h.
//   - LineOffsetForTextAlign: core/layout/length_utils.cc.
//   - LayoutUnit: platform/geometry/layout_unit.h.
//   - StyleBuilderConverter::ClampLineWidth:
//     core/css/resolver/style_builder_converter.cc.
//
// The paths are in third_party/blink/renderer. These files have the
// BSD-3-Clause license in their headers at that version. layout_unit.h and
// style_builder_converter.cc have the BSD terms in the file, with the
// copyright of Google. See NOTICE in this directory.

package browser

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// This file lays out the HTML that mermaid puts inside <foreignObject>:
// labels made of div, span, p, br and simple inline formatting. It is a
// small subset of CSS block and inline layout, in LayoutUnits like Blink.

const luInfinite = lu(math.MaxInt64 / 4)

// htmlBox is the border box of an element, relative to the content origin
// of the foreignObject.
type htmlBox struct {
	node       *dom.Node
	x, y, w, h lu
	lines      []*htmlLine
	children   []*htmlBox
	style      *textStyle
}

type htmlLine struct {
	x, y, width, height lu
	baseline            lu // from the top of the line
	frags               []htmlFrag
	// marks are the places of the items with no characters. See
	// htmlinline.go.
	marks []lineMark
	// forced is true for a line that a forced break ends. Such a line has
	// a line box also when it has no text.
	forced bool
}

// htmlFrag is the part of an item that is on one line.
type htmlFrag struct {
	item     *inlineItem
	lo, hi   int // rune range in the item
	x, width lu  // x is relative to the content box of the block
	// top and height are the box of the text: the ascent and the descent
	// of the font of its element. top is relative to the top of the line.
	top, height lu
}

type boxEdges struct{ top, right, bottom, left lu }

func (b boxEdges) horizontal() lu { return b.left + b.right }
func (b boxEdges) vertical() lu   { return b.top + b.bottom }

func (e *Engine) lengthProp(n *dom.Node, prop string, ts *textStyle, percentBase lu) (lu, bool) {
	v, ok := e.declared(n, prop)
	if !ok {
		return 0, false
	}
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "auto" || v == "none" || v == "" {
		return 0, false
	}
	if strings.HasSuffix(v, "%") {
		f, ok := parseCSSNumber(v[:len(v)-1])
		if !ok || percentBase >= luInfinite {
			return 0, false
		}
		return lu(float64(percentBase) * f / 100), true
	}
	if v[len(v)-1] == ')' && css.IsMathFunction(v) {
		// A math function can have a percentage of the base.
		c, ok := e.evalCalc(v, ts)
		if !ok || c.IsNumber || (c.HasPercent && percentBase >= luInfinite) {
			return 0, false
		}
		px := c.Px
		if c.HasPercent {
			px += float64(percentBase.px() * c.Percent / 100)
		}
		px = clampCalcPx(px)
		if px < 0 && !strings.HasPrefix(prop, "margin") {
			// A math function is clamped to the range of the property:
			// only a margin can be negative.
			px = 0
		}
		return luFromPx(px), true
	}
	px, ok := e.cssLength(v, ts, false)
	if !ok {
		return 0, false
	}
	return luFromPx(px), true
}

func (e *Engine) edges(n *dom.Node, prefix, suffix string, ts *textStyle, base lu) boxEdges {
	var ua boxEdges
	if prefix == "margin" && n.NS == dom.NSHTML {
		ua = uaMargins(n.Tag, ts)
	}
	get := func(side string, def lu) lu {
		if _, declared := e.declared(n, prefix+side+suffix); !declared {
			return def
		}
		v, _ := e.lengthProp(n, prefix+side+suffix, ts, base)
		return v
	}
	return boxEdges{get("-top", ua.top), get("-right", ua.right), get("-bottom", ua.bottom), get("-left", ua.left)}
}

// uaMargins returns the margins of the user agent style sheet.
func uaMargins(tag string, ts *textStyle) boxEdges {
	em := func(f float64) lu { return luFromPx(float64(f * ts.size)) }
	switch tag {
	case "p", "ul", "ol", "dl", "pre", "h4":
		if tag == "h4" {
			return boxEdges{top: em(1.33), bottom: em(1.33)}
		}
		return boxEdges{top: em(1), bottom: em(1)}
	case "h1":
		return boxEdges{top: em(0.67), bottom: em(0.67)}
	case "h2":
		return boxEdges{top: em(0.83), bottom: em(0.83)}
	case "h3":
		return boxEdges{top: em(1), bottom: em(1)}
	case "h5":
		return boxEdges{top: em(1.67), bottom: em(1.67)}
	case "h6":
		return boxEdges{top: em(2.33), bottom: em(2.33)}
	case "blockquote", "figure":
		return boxEdges{top: em(1), bottom: em(1), left: luFromPx(40), right: luFromPx(40)}
	case "hr":
		return boxEdges{top: luFromPx(8), bottom: luFromPx(8)}
	}
	return boxEdges{}
}

func (e *Engine) borderEdges(n *dom.Node, ts *textStyle) boxEdges {
	var out boxEdges
	parse := func(v string) lu {
		for _, part := range strings.Fields(strings.ToLower(v)) {
			switch part {
			case "none", "hidden":
				return 0
			case "thin":
				return luFromPx(1)
			case "medium":
				return luFromPx(3)
			case "thick":
				return luFromPx(5)
			}
			if px, ok := e.cssLength(part, ts, false); ok {
				return clampLineWidth(px)
			}
		}
		return 0
	}
	if v, ok := e.declared(n, "border"); ok && strings.ContainsAny(v, "sdg") {
		w := parse(v)
		if !strings.Contains(strings.ToLower(v), "none") && w == 0 && strings.ContainsAny(strings.ToLower(v), "solid dashed dotted double") {
			w = luFromPx(3)
		}
		out = boxEdges{w, w, w, w}
	}
	for i, side := range []string{"top", "right", "bottom", "left"} {
		if v, ok := e.declared(n, "border-"+side); ok {
			w := parse(v)
			switch i {
			case 0:
				out.top = w
			case 1:
				out.right = w
			case 2:
				out.bottom = w
			case 3:
				out.left = w
			}
		}
	}
	// The longhands border-width and border-style, which expandDecl turns
	// into the properties of the four sides. A declaration of them that
	// wins the cascade is there after the border shorthand reset them.
	if _, ok := e.styleOf(n).declared[borderLonghandsMark]; !ok {
		return out
	}
	for i, side := range borderSides {
		wv, hasW := e.declared(n, "border-"+side+"-width")
		sv, hasS := e.declared(n, "border-"+side+"-style")
		hasW, hasS = hasW && wv != "", hasS && sv != ""
		if !hasW && !hasS {
			continue
		}
		edge := &out.top
		switch i {
		case 1:
			edge = &out.right
		case 2:
			edge = &out.bottom
		case 3:
			edge = &out.left
		}
		// The style and the width that the shorthands gave: a side with a
		// width has a style.
		drawn, width := *edge > 0, *edge
		if hasS {
			sv = strings.ToLower(strings.TrimSpace(sv))
			drawn = sv != "none" && sv != "hidden"
			if !hasW && width == 0 {
				// The initial width is medium.
				width = luFromPx(3)
			}
		}
		if hasW {
			width = parse(wv)
		}
		if !drawn {
			width = 0
		}
		*edge = width
	}
	return out
}

// clampLineWidth is StyleBuilderConverter::ClampLineWidth: the width of a
// border is a whole number of pixels, and a width between 0 and 1 is 1.
func clampLineWidth(px float64) lu {
	if px > 0 && px < 1 {
		return luFromPx(1)
	}
	return luFromPx(math.Floor(px))
}

func isBlockLevelDisplay(d string) bool {
	switch d {
	case "block", "table", "list-item", "flex", "grid", "table-cell", "table-row", "table-row-group",
		"table-header-group", "table-footer-group", "table-caption", "flow-root":
		return true
	case "inline-block", "inline-table", "inline-flex", "inline-grid":
		// An atomic inline box. Labels use it as the only child of its
		// container, where it lays out like a shrink-to-fit block.
		return true
	}
	return false
}

// containsBlock reports whether an inline element has a block-level
// descendant, which splits the inline formatting context.
func (e *Engine) containsBlock(n *dom.Node) bool {
	check := func(c *dom.Node) bool {
		d := e.display(c)
		if d == "none" || e.isOutOfFlow(c) || e.isAtomicInline(c, d) {
			// An atomic inline box is on a line. Its content is in a
			// formatting context of its own.
			return false
		}
		return isBlockLevelDisplay(d) || e.containsBlock(c)
	}
	before, after := e.pseudoBoxes(n)
	if before != nil && check(before) {
		return true
	}
	for c := n.FirstChild; c != nil; c = c.Next {
		if c.Type == dom.ElementNode && check(c) {
			return true
		}
	}
	return after != nil && check(after)
}

// flowChild is a child of a block container: either a block-level element
// or a run of inline-level nodes.
type flowChild struct {
	block  *dom.Node
	inline []*dom.Node
}

func (e *Engine) flowChildren(n *dom.Node) []flowChild {
	var out []flowChild
	var inline []*dom.Node
	flush := func() {
		if len(inline) > 0 {
			out = append(out, flowChild{inline: inline})
			inline = nil
		}
	}
	add := func(c *dom.Node) {
		switch c.Type {
		case dom.TextNode:
			inline = append(inline, c)
		case dom.ElementNode:
			d := e.display(c)
			if d == "none" || e.isOutOfFlow(c) {
				return
			}
			if e.isAtomicInline(c, d) {
				// An atomic inline box is on a line of this container.
				inline = append(inline, c)
			} else if isBlockLevelDisplay(d) {
				flush()
				out = append(out, flowChild{block: c})
			} else if e.containsBlock(c) {
				// An inline element with block children: its children take
				// part in the block flow of this container. The inline
				// content in front of its first block is on the line of
				// the content in front of the element, and the inline
				// content behind its last block starts the line of the
				// content behind the element.
				sub := e.flowChildren(c)
				if len(sub) > 0 && sub[0].block == nil {
					inline = append(inline, sub[0].inline...)
					sub = sub[1:]
				}
				flush()
				if n := len(sub); n > 0 && sub[n-1].block == nil {
					inline = sub[n-1].inline
					sub = sub[:n-1]
				}
				out = append(out, sub...)
			} else {
				inline = append(inline, c)
			}
		}
	}
	// The boxes of the pseudo-elements are the first and the last child.
	before, after := e.pseudoBoxes(n)
	if before != nil {
		add(before)
	}
	for c := n.FirstChild; c != nil; c = c.Next {
		add(c)
	}
	if after != nil {
		add(after)
	}
	flush()
	return out
}

// collectInline turns inline-level nodes into items. A nil entry in the
// result is a forced line break.
func (e *Engine) collectInline(nodes []*dom.Node, cs *collapseState, avail lu) []*inlineItem {
	var items []*inlineItem
	var walk func(n *dom.Node)
	walk = func(n *dom.Node) {
		switch n.Type {
		case dom.TextNode:
			owner := n.Parent
			ts := e.textStyleOf(owner)
			lineStart := cs.atStart
			runes := processWhiteSpace(n.Data, ts.whiteSpace, cs, false)
			runes = applyTextTransform(runes, ts.textTransform)
			// A preserved newline is a forced break.
			start := 0
			// InlineItemsBuilder::AppendPreserveWhitespace: the spaces at
			// the start of a line, in a text that keeps its spaces and
			// wraps, are an item of their own, and a break opportunity
			// follows them. The rest of the text is a new item: it is not
			// shaped with the spaces, and its positions start again at 0.
			boundary := false
			if lineStart && (ts.whiteSpace == "pre-wrap" || ts.whiteSpace == "break-spaces") {
				lead := 0
				for lead < len(runes) && runes[lead] == ' ' {
					lead++
				}
				if lead > 0 {
					items = append(items, &inlineItem{node: n, owner: owner, style: ts, runes: runes[:lead]})
					runes = runes[lead:]
					boundary = true
				}
			}
			for i, r := range runes {
				if r == '\n' {
					if i > start {
						items = append(items, &inlineItem{node: n, owner: owner, style: ts, runes: runes[start:i], shapeBoundary: boundary})
						boundary = false
					}
					items = append(items, nil)
					start = i + 1
				}
			}
			if start < len(runes) {
				it := &inlineItem{node: n, owner: owner, style: ts, runes: runes[start:], shapeBoundary: boundary}
				if cs.prevSpace {
					cs.lastSpace = it
				}
				items = append(items, it)
			}
		case dom.ElementNode:
			if e.display(n) == "none" || e.isOutOfFlow(n) {
				return
			}
			if n.NS == dom.NSHTML && n.Tag == "br" {
				// A collapsible space at the end of a line is removed, and
				// a br element ends the line.
				if cs.prevSpace && cs.lastSpace != nil {
					it := cs.lastSpace
					if n := len(it.runes); n > 0 && it.runes[n-1] == ' ' {
						it.runes = it.runes[:n-1]
					}
				}
				// The br has a box of its own at the end of its line.
				items = append(items, e.newMarkItem(markBreak, n, n.Parent, inlineEdge{}), nil)
				cs.atStart, cs.prevSpace = true, false
				return
			}
			if isImage(n) {
				// A replaced element is an atomic inline box.
				items = append(items, e.newImageItem(n, avail))
				cs.atStart, cs.prevSpace = false, false
				return
			}
			if e.isAtomicInline(n, e.display(n)) {
				// An element with display: inline-block and the like is an
				// atomic inline box. See inlineblock.go.
				items = append(items, e.newInlineBlockItem(n, avail))
				cs.atStart, cs.prevSpace = false, false
				return
			}
			edge := e.inlineEdges(n)
			before, after := e.pseudoBoxes(n)
			if before == nil && after == nil && !hasLayoutChild(n) {
				// An inline element with no content has an empty box.
				items = append(items, e.newMarkItem(markEmpty, n, n, edge))
				return
			}
			if edge.startMargin != 0 || edge.start != 0 {
				items = append(items, e.newMarkItem(markOpen, n, n, inlineEdge{startMargin: edge.startMargin, start: edge.start}))
			}
			if before != nil {
				walk(before)
			}
			for c := n.FirstChild; c != nil; c = c.Next {
				walk(c)
			}
			if after != nil {
				walk(after)
			}
			if edge.end != 0 || edge.endMargin != 0 {
				items = append(items, e.newMarkItem(markClose, n, n, inlineEdge{end: edge.end, endMargin: edge.endMargin}))
			}
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return items
}

// isBreakAfter reports whether a line can break after the rune at index i
// of the item. It covers the common cases of UAX #14.
func isBreakAfter(runes []rune, i int, ws string) bool {
	r := runes[i]
	var next rune
	if i+1 < len(runes) {
		next = runes[i+1]
	}
	switch {
	case r == ' ':
		if ws == "break-spaces" {
			return true
		}
		return next != ' '
	case r == 0x200b:
		return true
	}
	// A pair of printable ASCII characters has its answer in a table.
	var before rune
	if i > 0 {
		before = runes[i-1]
	}
	if can, ok := asciiBreakBetween(before, r, next); ok {
		return can
	}
	switch {
	case r == '-':
		// A hyphen allows a break when a letter follows and a letter
		// precedes it.
		if i > 0 && next != 0 && isLetter(runes[i-1]) && isLetter(next) {
			return true
		}
		return false
	case isCJK(r):
		return next != 0 && !isClosingPunct(next)
	}
	if next != 0 && isCJK(next) && r != ' ' && !isOpeningPunct(r) {
		return true
	}
	return false
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r > 0x7f
}

func isCJK(r rune) bool {
	return (r >= 0x2e80 && r <= 0x9fff) || (r >= 0xac00 && r <= 0xd7af) || (r >= 0xf900 && r <= 0xfaff) || (r >= 0xff00 && r <= 0xffef && r != 0xff0c && r != 0xff0e) || (r >= 0x20000 && r <= 0x3ffff)
}

func isClosingPunct(r rune) bool {
	return strings.ContainsRune("、。，．：；！？）］｝〉》」』】〕)]}.,:;!?", r)
}

func isOpeningPunct(r rune) bool {
	return strings.ContainsRune("（［｛〈《「『【〔([{", r)
}

// lineBreaker holds the state of one inline formatting context.
type lineBreaker struct {
	e     *Engine
	items []*inlineItem
	strut *textStyle
	// container is the block that holds the lines.
	container *dom.Node
	// minContent is true while the lines are broken for the min-content
	// width. See fragmentWidth.
	minContent bool
	// startDelta caches lineStartDelta. See lineStartWidth.
	startDelta map[lineStartKey]float64
}

// breakLines places the items on lines of at most avail width.
func (lb *lineBreaker) breakLines(avail lu) []*htmlLine {
	var lines []*htmlLine
	cur := &htmlLine{}
	// last is the last break opportunity of the current line that has
	// content before it: the item and the rune where the next line would
	// start. LineBreaker::HandleOverflow goes back to it when the content
	// after it does not fit and has no break opportunity of its own.
	var last struct {
		idx, lo int
		ok      bool
	}
	newLine := func() {
		lines = append(lines, cur)
		cur = &htmlLine{}
		last.ok = false
	}
	index := make(map[*inlineItem]int, len(lb.items))
	for i, it := range lb.items {
		if it != nil {
			index[it] = i
		}
	}
	// rewind cuts the current line at the last break opportunity.
	rewind := func() {
		frags := cur.frags[:0]
		for _, f := range cur.frags {
			switch i := index[f.item]; {
			case i < last.idx, i == last.idx && f.hi <= last.lo:
				frags = append(frags, f)
			case i == last.idx && f.lo < last.lo:
				f.hi = last.lo
				if len(frags) == 0 && !lineHasContent(&htmlLine{marks: cur.marks}) {
					f.width = lb.lineStartWidth(f.item, f.lo, f.hi)
				} else {
					f.width = lb.fragmentWidth(f.item, f.lo, f.hi)
				}
				frags = append(frags, f)
			}
		}
		cur.frags = frags
		marks := cur.marks[:0]
		for _, m := range cur.marks {
			if index[m.item] < last.idx {
				marks = append(marks, m)
			}
		}
		cur.marks = marks
	}
	resume := 0
	// lineWidth is the width of the fragments on the current line.
	lineWidth := func() lu {
		var w lu
		for _, f := range cur.frags {
			w += f.width
		}
		return w + marksWidth(cur.marks)
	}
	// LineBreaker::AvailableWidthToFit: a line can be wider than the
	// available width by one LayoutUnit. The width of a text is rounded up
	// to a LayoutUnit, and this tolerance hides the rounding.
	fit := avail
	if fit < luInfinite {
		fit++
	}
	// The first text of a line can start in the middle of an item.
	fragWidth := func(it *inlineItem, lo, hi int) lu {
		if !lineHasContent(cur) {
			return lb.lineStartWidth(it, lo, hi)
		}
		return lb.fragmentWidth(it, lo, hi)
	}
items:
	for idx := 0; idx < len(lb.items); idx++ {
		it := lb.items[idx]
		if it == nil {
			cur.forced = true
			newLine()
			continue
		}
		if it.mark != nil {
			if it.mark.kind == markAtomic && lineHasContent(cur) && it.style.whiteSpace != "nowrap" && it.style.whiteSpace != "pre" &&
				lineWidth()+it.mark.advance() > fit {
				// The box does not fit on the line: it starts a new one.
				lb.trimTrailingSpace(cur)
				moved := takeTrailingMarks(cur)
				newLine()
				cur.marks = moved
			}
			cur.marks = append(cur.marks, lineMark{item: it, at: len(cur.frags)})
			continue
		}
		ws := it.style.whiteSpace
		wrap := ws != "nowrap" && ws != "pre"
		breakAnywhere := false
		if wrap {
			if v := strings.ToLower(lb.e.lookup(it.owner, "overflow-wrap")); v == "anywhere" || v == "break-word" {
				breakAnywhere = true
			}
			if v := strings.ToLower(lb.e.lookup(it.owner, "word-break")); v == "break-all" || v == "break-word" {
				breakAnywhere = true
			}
		}
		if !wrap {
			cur.frags = append(cur.frags, htmlFrag{item: it, lo: 0, hi: len(it.runes), width: it.width})
			continue
		}
		lo := resume
		resume = 0
		if lo == 0 && lineHasContent(cur) && lb.canBreakBefore(idx, 0) {
			last.idx, last.lo, last.ok = idx, 0, true
		}
		for lo < len(it.runes) {
			used := lineWidth()
			// Find the longest prefix [lo, hi) that fits, ending at a break
			// opportunity or at the end of the item.
			best := -1
			var bestWidth lu
			for i := lo; i < len(it.runes); i++ {
				end := i + 1
				atEnd := end == len(it.runes)
				if !atEnd && !isBreakAfter(it.runes, i, ws) {
					continue
				}
				// Trailing collapsible spaces hang: they do not count.
				measureEnd := end
				if ws != "break-spaces" && ws != "pre-wrap" {
					for measureEnd > lo && it.runes[measureEnd-1] == ' ' {
						measureEnd--
					}
				}
				w := fragWidth(it, lo, measureEnd)
				if used+w <= fit {
					best, bestWidth = end, fragWidth(it, lo, end)
					if atEnd {
						break
					}
					continue
				}
				break
			}
			if best < 0 {
				// Nothing fits. If the line has content, wrap first.
				if lineHasContent(cur) && lb.canBreakBefore(idx, lo) {
					lb.trimTrailingSpace(cur)
					// The boxes that open after the last text of the line
					// go to the next line with their content.
					moved := takeTrailingMarks(cur)
					newLine()
					cur.marks = moved
					continue
				}
				if last.ok {
					// Go back to the last break opportunity of the line:
					// the content after it starts the next line.
					rewind()
					lb.trimTrailingSpace(cur)
					idx, resume = last.idx-1, last.lo
					newLine()
					continue items
				}
				// Take the first unbreakable unit, even if it overflows.
				end := len(it.runes)
				for i := lo; i < len(it.runes); i++ {
					if i+1 < len(it.runes) && isBreakAfter(it.runes, i, ws) {
						end = i + 1
						break
					}
				}
				if breakAnywhere {
					// Break inside the word: as many characters as fit.
					end = lo + 1
					for end < len(it.runes) && used+fragWidth(it, lo, end+1) <= fit {
						end++
					}
				}
				best, bestWidth = end, fragWidth(it, lo, end)
			}
			cur.frags = append(cur.frags, htmlFrag{item: it, lo: lo, hi: best, width: bestWidth})
			// The last break opportunity in the part that is on the line.
			for i := best - 1; i >= lo; i-- {
				if isBreakAfter(it.runes, i, ws) {
					if i+1 < best || lineHasContent(cur) {
						last.idx, last.lo, last.ok = idx, i+1, true
					}
					break
				}
			}
			lo = best
			if lo < len(it.runes) {
				lb.trimTrailingSpace(cur)
				newLine()
			}
		}
	}
	if len(cur.frags) > 0 || len(cur.marks) > 0 || len(lines) == 0 {
		lines = append(lines, cur)
	}
	return lines
}

// lineHasContent reports whether a line has a text or an atomic inline box.
func lineHasContent(line *htmlLine) bool {
	if len(line.frags) > 0 {
		return true
	}
	for i := range line.marks {
		if line.marks[i].item.mark.kind == markAtomic {
			return true
		}
	}
	return false
}

// canBreakBefore reports whether a line break is allowed before rune lo of
// item idx: the previous content must end at a break opportunity.
func (lb *lineBreaker) canBreakBefore(idx, lo int) bool {
	it := lb.items[idx]
	if lo > 0 {
		return isBreakAfter(it.runes, lo-1, it.style.whiteSpace)
	}
	for j := idx - 1; j >= 0; j-- {
		prev := lb.items[j]
		if prev == nil {
			return true
		}
		if prev.mark != nil && prev.mark.kind == markAtomic {
			// A line can break after an atomic inline box.
			return true
		}
		if len(prev.runes) == 0 {
			continue
		}
		last := prev.runes[len(prev.runes)-1]
		var before rune
		if len(prev.runes) > 1 {
			before = prev.runes[len(prev.runes)-2]
		}
		if can, ok := asciiBreakBetween(before, last, it.runes[0]); ok {
			return can
		}
		return last == ' ' || last == 0x200b || isCJK(last) || isCJK(it.runes[0])
	}
	return false
}

// trimTrailingSpace removes the collapsible space at the end of a wrapped
// line from the width of the line.
func (lb *lineBreaker) trimTrailingSpace(line *htmlLine) {
	if len(line.frags) == 0 {
		return
	}
	f := &line.frags[len(line.frags)-1]
	ws := f.item.style.whiteSpace
	if ws == "break-spaces" || ws == "pre-wrap" || ws == "pre" {
		return
	}
	hi := f.hi
	for hi > f.lo && f.item.runes[hi-1] == ' ' {
		hi--
	}
	if hi != f.hi {
		f.hi = hi
		if len(line.frags) == 1 && !lineHasContent(&htmlLine{marks: line.marks}) {
			f.width = lb.lineStartWidth(f.item, f.lo, hi)
		} else {
			f.width = lb.fragmentWidth(f.item, f.lo, hi)
		}
	}
}

// finishLines computes the size of each line and the x of each fragment.
func (lb *lineBreaker) finishLines(lines []*htmlLine, contentW lu, align string) lu {
	strutChain := lb.e.chainOf(lb.strut)
	var strutMetrics font.Metrics
	if p := strutChain.Primary(); p != nil {
		strutMetrics = p.Metrics(lb.strut.fontSize())
	}
	var y lu
	for _, line := range lines {
		ascent, descent := lineExtent(lb.strut, strutMetrics)
		// InlineLayoutAlgorithm::HandleOpenTag: each inline box that is open
		// on the line adds the metrics of its own font and line height,
		// also when it has no text of its own.
		open := func(el *dom.Node) {
			for p := el; p != nil && p != lb.container && p.Type == dom.ElementNode; p = p.Parent {
				m := lb.e.inlineBoxOf(p, lb.container)
				ascent, descent = max(ascent, m.lineAscent-m.shift), max(descent, m.lineDescent+m.shift)
			}
		}
		var w lu
		next := 0
		placeMarks := func(before int) {
			for ; next < len(line.marks) && line.marks[next].at <= before; next++ {
				m := &line.marks[next]
				m.x = w
				w += m.item.mark.advance()
				open(m.item.owner)
				if mark := m.item.mark; mark.kind == markAtomic {
					// The box stands on the baseline of its parent, moved
					// by its own vertical-align.
					shift := lb.e.inlineBoxOf(m.item.owner, lb.container).shift + mark.atomicShift
					ascent = max(ascent, mark.atomicBaseline-shift)
					descent = max(descent, mark.atomicH-mark.atomicBaseline+shift)
				}
			}
		}
		for i := range line.frags {
			placeMarks(i)
			f := &line.frags[i]
			open(f.item.owner)
			if f.item.style.lhKind == lhNormal {
				// InlineBoxState::AccumulateUsedFonts: with line-height
				// normal, each font that draws a glyph of the text adds
				// its own metrics to the line. This counts for a font
				// with larger metrics than the font of the text, such as
				// the emoji font.
				a, d := lb.usedFontsExtent(f)
				ascent, descent = max(ascent, a), max(descent, d)
			}
			f.x = w
			w += f.width
		}
		placeMarks(len(line.frags))
		line.width = w
		line.y = y
		if line.isEmpty() {
			// LineInfo::IsEmptyLine: a line with no text and no forced
			// break has no line box. Its boxes are empty, at the place
			// that text-align gives to a line with no width.
			line.x = alignOffset(align, contentW, w)
			continue
		}
		line.height = ascent + descent
		line.baseline = ascent
		y += line.height
		for i := range line.frags {
			f := &line.frags[i]
			m := lb.e.inlineBoxOf(f.item.owner, lb.container)
			f.top, f.height = line.baseline+m.shift-m.ascent, m.ascent+m.descent
		}
		for i := range line.marks {
			mk := &line.marks[i]
			m := lb.e.inlineBoxOf(mk.item.owner, lb.container)
			mk.top, mk.height = line.baseline+m.shift-m.ascent, m.ascent+m.descent
			if mark := mk.item.mark; mark.kind == markAtomic {
				mk.top, mk.height = line.baseline+m.shift+mark.atomicShift-mark.atomicBaseline, mark.atomicH
			}
		}
		line.x = alignOffset(align, contentW, w)
	}
	return y
}

// alignOffset is LineOffsetForTextAlign for a block with the direction ltr:
// where a line of the width w starts in a block of the width contentW. A
// line that is wider than the block starts at the left edge.
func alignOffset(align string, contentW, w lu) lu {
	if contentW >= luInfinite {
		return 0
	}
	switch align {
	case "center", "-webkit-center":
		return max((contentW-w)/2, 0)
	case "right", "end", "-webkit-right":
		return max(contentW-w, 0)
	}
	return 0
}

// usedFontsExtent returns the largest ascent and descent of the fallback
// fonts that draw the glyphs of a fragment: the fonts of the shape result
// other than the font of the item.
func (lb *lineBreaker) usedFontsExtent(f *htmlFrag) (ascent, descent lu) {
	it := f.item
	primary := lb.e.chainOf(it.style).Primary()
	var seen *font.Face
	for i := range it.glyphs {
		g := &it.glyphs[i]
		if g.runeIdx < f.lo || g.runeIdx >= f.hi || g.face == nil || g.face == primary || g.face == seen {
			continue
		}
		seen = g.face
		a, d := lineExtent(it.style, g.face.Metrics(it.style.fontSize()))
		ascent, descent = max(ascent, a), max(descent, d)
	}
	return ascent, descent
}

// lineExtent returns the ascent and descent of an inline box including half
// leading, as LayoutNG computes them (the ascent part of the leading is
// floored).
func lineExtent(ts *textStyle, m font.Metrics) (ascent, descent lu) {
	a := luFromFloatRound(m.Ascent)
	d := luFromFloatRound(m.Descent)
	// CalculateLeadingSpace: the half of the leading above the text is a
	// LayoutUnit division, then LayoutUnit::Floor to a whole pixel. The
	// rest of the leading is below the text.
	leading := computedLineHeight(ts, m) - (a + d)
	half := ((leading / 2) >> 6) << 6
	return a + half, d + (leading - half)
}

func maxLineWidth(lines []*htmlLine) lu {
	var w lu
	for _, l := range lines {
		lw := marksWidth(l.marks)
		for _, f := range l.frags {
			lw += f.width
		}
		w = max(w, lw)
	}
	return w
}

// prepareInline collects and shapes the items of an inline group.
// avail is the width of the content box of the container, or luInfinite
// when the widths are intrinsic.
func (e *Engine) prepareInline(nodes []*dom.Node, container *dom.Node, avail lu) *lineBreaker {
	cs := &collapseState{atStart: true}
	items := e.collectInline(nodes, cs, avail)
	// Trailing collapsible space at the end of the block is removed.
	if cs.prevSpace && cs.lastSpace != nil {
		it := cs.lastSpace
		if n := len(it.runes); n > 0 && it.runes[n-1] == ' ' {
			it.runes = it.runes[:n-1]
		}
	}
	// An item with no characters is not shaped. The texts on its two sides
	// are shaped together when they have the same font: Blink treats an
	// open tag and a close tag as opaque to shaping (InlineNode::ShapeText).
	var real []*inlineItem
	for _, it := range items {
		if it != nil && it.mark == nil {
			real = append(real, it)
		}
	}
	// A forced break is a shaping boundary.
	boundaries := map[*inlineItem]bool{}
	prevBreak := true
	for _, it := range items {
		if it == nil {
			prevBreak = true
			continue
		}
		if it.mark != nil {
			continue
		}
		if prevBreak || it.shapeBoundary {
			boundaries[it] = true
		}
		prevBreak = false
	}
	e.shapeItems(real, func(i int) bool { return boundaries[real[i]] })
	// Drop items that became empty, but keep forced breaks and marks.
	kept := items[:0]
	for _, it := range items {
		if it == nil || it.mark != nil || len(it.runes) > 0 {
			kept = append(kept, it)
		}
	}
	return &lineBreaker{e: e, items: kept, strut: e.textStyleOf(container), container: container}
}

// intrinsicWidths returns the min-content and max-content widths of the
// content box of a block container.
func (e *Engine) intrinsicWidths(n *dom.Node) (minW, maxW lu) {
	for _, fc := range e.flowChildren(n) {
		if fc.block != nil {
			cmin, cmax := e.outerIntrinsic(fc.block)
			minW, maxW = max(minW, cmin), max(maxW, cmax)
			continue
		}
		lb := e.prepareInline(fc.inline, n, luInfinite)
		if !hasContent(lb.items) {
			continue
		}
		maxW = max(maxW, maxLineWidth(lb.breakLines(luInfinite)))
		lb.minContent = true
		minW = max(minW, maxLineWidth(lb.breakLines(0)))
		lb.minContent = false
	}
	return minW, maxW
}

func hasContent(items []*inlineItem) bool {
	return len(items) > 0
}

// outerIntrinsic is the intrinsic width of a block-level child including
// its padding, border and margins.
func (e *Engine) outerIntrinsic(n *dom.Node) (minW, maxW lu) {
	if isImage(n) {
		return e.replacedIntrinsic(n)
	}
	ts := e.textStyleOf(n)
	pad := e.edges(n, "padding", "", ts, 0)
	border := e.borderEdges(n, ts)
	margin := e.edges(n, "margin", "", ts, 0)
	extra := pad.horizontal() + border.horizontal() + margin.horizontal()
	if w, ok := e.lengthProp(n, "width", ts, luInfinite); ok {
		if strings.EqualFold(strings.TrimSpace(e.lookupDeclared(n, "box-sizing")), "border-box") {
			return w + margin.horizontal(), w + margin.horizontal()
		}
		return w + extra, w + extra
	}
	minW, maxW = e.intrinsicWidths(n)
	if mw, ok := e.lengthProp(n, "max-width", ts, luInfinite); ok {
		minW, maxW = min(minW, mw), min(maxW, mw)
	}
	if mw, ok := e.lengthProp(n, "min-width", ts, luInfinite); ok {
		minW, maxW = max(minW, mw), max(maxW, mw)
	}
	return minW + extra, maxW + extra
}

func (e *Engine) lookupDeclared(n *dom.Node, prop string) string {
	v, _ := e.declared(n, prop)
	return v
}

// layoutBlock lays out a block-level element whose containing block has the
// given content width. The box is positioned at (0, 0); the caller moves it.
func (e *Engine) layoutBlock(n *dom.Node, avail lu) *htmlBox {
	if isImage(n) {
		return e.layoutReplacedBlock(n, avail)
	}
	ts := e.textStyleOf(n)
	disp := e.display(n)
	box := &htmlBox{node: n, style: ts}
	pad := e.edges(n, "padding", "", ts, avail)
	border := e.borderEdges(n, ts)
	margin := e.edges(n, "margin", "", ts, avail)
	borderBox := strings.EqualFold(strings.TrimSpace(e.lookupDeclared(n, "box-sizing")), "border-box")
	inner := pad.horizontal() + border.horizontal()

	specW, hasW := e.lengthProp(n, "width", ts, avail)
	if hasW && borderBox {
		specW = max(0, specW-inner)
	}
	maxW, hasMax := e.lengthProp(n, "max-width", ts, avail)
	minW, hasMin := e.lengthProp(n, "min-width", ts, avail)
	if borderBox {
		if hasMax {
			maxW = max(0, maxW-inner)
		}
		if hasMin {
			minW = max(0, minW-inner)
		}
	}

	var contentW lu
	shrink := false
	switch disp {
	case "table", "inline-table":
		minC, maxC := e.intrinsicWidths(n)
		if hasW {
			// max-width limits the width of a table, and a table is never
			// narrower than its min-content width.
			contentW = specW
			if hasMax {
				contentW = min(contentW, maxW)
			}
			contentW = max(contentW, minC)
		} else {
			contentW = maxC
			if avail < luInfinite {
				contentW = min(max(minC, avail-inner-margin.horizontal()), maxC)
			}
			if hasMax {
				contentW = max(min(contentW, maxW), minC)
			}
		}
		if hasMin {
			contentW = max(contentW, minW)
		}
	case "table-cell", "inline-block", "inline-flex", "inline-grid":
		// A cell in a table is as wide as the table. See table.go.
		shrink = disp != "table-cell" || e.tableOf(n) == nil
		fallthrough
	default:
		if hasW {
			contentW = specW
		} else if shrink || avail >= luInfinite {
			minC, maxC := e.intrinsicWidths(n)
			space := luInfinite
			if avail < luInfinite {
				space = avail - inner - margin.horizontal()
			}
			if hasMax && disp != "table-cell" {
				// max-width limits the space that shrink-to-fit uses.
				space = min(space, maxW)
			}
			contentW = min(max(minC, space), maxC)
			// max-width also limits the result: a box whose narrowest
			// content is wider than max-width has the width max-width, and
			// the content overflows it.
		} else {
			contentW = max(0, avail-inner-margin.horizontal())
		}
		if hasMax {
			contentW = min(contentW, maxW)
		}
		if hasMin {
			contentW = max(contentW, minW)
		}
	}

	align := ts.textAlign
	var y lu
	var prevMargin lu
	first := true
	for _, fc := range e.flowChildren(n) {
		if fc.block != nil {
			child := e.layoutBlock(fc.block, contentW)
			cts := e.textStyleOf(fc.block)
			cm := e.edges(fc.block, "margin", "", cts, contentW)
			// Adjacent vertical margins collapse to the larger one.
			gap := cm.top
			if !first {
				gap = max(prevMargin, cm.top) - prevMargin
			}
			y += gap
			child.x = border.left + pad.left + cm.left
			child.y = border.top + pad.top + y
			if child.h == 0 && len(child.lines) == 0 && len(child.children) == 0 {
				// A block with no height collapses its own margins: the
				// top margin and the bottom margin are one margin.
				collapsed := cm.top
				if !first {
					collapsed = max(prevMargin, cm.top)
				}
				if cm.bottom > collapsed {
					y += cm.bottom - collapsed
					collapsed = cm.bottom
				}
				prevMargin = collapsed
			} else {
				y += child.h + cm.bottom
				prevMargin = cm.bottom
			}
			first = false
			box.children = append(box.children, child)
			continue
		}
		lb := e.prepareInline(fc.inline, n, contentW)
		if !hasContent(lb.items) {
			continue
		}
		lines := lb.breakLines(contentW)
		h := lb.finishLines(lines, contentW, align)
		for _, line := range lines {
			line.y += border.top + pad.top + y
			line.x += border.left + pad.left
		}
		box.lines = append(box.lines, lines...)
		box.children = placeAtomicBoxes(box.children, lines)
		y += h
		if h > 0 {
			prevMargin = 0
			first = false
		}
	}
	contentH := y
	flowH := y
	// A percentage height refers to the height of the containing block
	// when that height is definite. See table.go.
	heightBase := e.containingHeight(n)
	if h, ok := e.lengthProp(n, "height", ts, heightBase); ok {
		if borderBox {
			h = max(0, h-pad.vertical()-border.vertical())
		}
		if disp == "table" || disp == "inline-table" {
			// The height of a table is a minimum.
			contentH = max(contentH, h)
		} else {
			contentH = h
		}
	}
	if h, ok := e.lengthProp(n, "min-height", ts, heightBase); ok {
		contentH = max(contentH, h)
	}
	if disp == "table-cell" || disp == "table-row" {
		if table := e.tableOf(n); table != nil {
			// The row of a table is as high as the table, and the cell is
			// as high as the row. vertical-align places the content.
			if th, ok := e.definiteContentHeight(table); ok {
				if free := th - pad.vertical() - border.vertical() - contentH; free > 0 {
					contentH += free
				}
			}
		}
	}
	if disp == "table-cell" && contentH > flowH {
		// vertical-align places the content of a cell that is higher than
		// its content.
		if shift := e.cellStretch(n, contentH-flowH); shift != 0 {
			for _, line := range box.lines {
				line.y += shift
			}
			for _, child := range box.children {
				child.y += shift
			}
		}
	}
	box.w = contentW + inner
	box.h = contentH + pad.vertical() + border.vertical()
	return box
}

// htmlRootLayout is the layout of the content of one foreignObject.
type htmlRootLayout struct {
	boxes []*htmlBox
	index map[*dom.Node]*htmlBox
	// origin of each box in the coordinate system of the foreignObject.
	origin map[*htmlBox][2]lu
	// imageEpoch is Engine.imageEpoch at the time of the layout.
	imageEpoch uint64
}

func (e *Engine) layoutForeignObject(fo *dom.Node) *htmlRootLayout {
	st := e.styleOf(fo)
	if st.layoutOK {
		if l, ok := st.layout.(*htmlRootLayout); ok && l.imageEpoch == e.imageEpoch {
			return l
		}
	}
	avail := luInfinite
	if _, ok := e.geometryLength(fo, "width"); ok {
		avail = e.foreignObjectFrame(fo).w
	}
	root := &htmlRootLayout{index: map[*dom.Node]*htmlBox{}, origin: map[*htmlBox][2]lu{}, imageEpoch: e.imageEpoch}
	var y lu
	for _, fc := range e.flowChildren(fo) {
		if fc.block == nil {
			continue
		}
		box := e.layoutBlock(fc.block, avail)
		cts := e.textStyleOf(fc.block)
		cm := e.edges(fc.block, "margin", "", cts, avail)
		box.x = cm.left
		box.y = y + cm.top
		y = box.y + box.h + cm.bottom
		root.boxes = append(root.boxes, box)
	}
	var register func(b *htmlBox, ox, oy lu)
	register = func(b *htmlBox, ox, oy lu) {
		root.index[b.node] = b
		root.origin[b] = [2]lu{ox + b.x, oy + b.y}
		for _, c := range b.children {
			register(c, ox+b.x, oy+b.y)
		}
	}
	for _, b := range root.boxes {
		register(b, 0, 0)
	}
	st.layout, st.layoutOK = root, true
	return root
}

// foreignObjectOf returns the <foreignObject> that contains an HTML node.
func foreignObjectOf(n *dom.Node) *dom.Node {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == dom.ElementNode && p.NS == dom.NSSVG && p.Tag == "foreignObject" {
			return p
		}
	}
	return nil
}

// htmlBoxRect returns the border box of an HTML element inside a
// foreignObject, in the coordinate system of the foreignObject. An inline
// element gets the union of its rectangles (see htmlQuads).
func (e *Engine) htmlBoxRect(n *dom.Node) (x, y, w, h lu, ok bool) {
	quads, ok := e.htmlQuads(n)
	if !ok {
		return 0, 0, 0, 0, false
	}
	r := unionQuads(quads)
	return r.x, r.y, r.w, r.h, true
}
