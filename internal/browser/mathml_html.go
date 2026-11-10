// Ported in part from Chromium 156.0.8078.4, core/layout/flex/
// flex_layout_algorithm.cc and core/layout/mathml/math_row_layout_algorithm.cc
// (BSD-3-Clause). Only the nowrap math label row is implemented here.
package browser

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

func containsMath(n *dom.Node) bool {
	if isMathRoot(n) {
		return true
	}
	for c := n.FirstChild; c != nil; c = c.Next {
		if c.Type == dom.ElementNode && containsMath(c) {
			return true
		}
	}
	return false
}

func (e *Engine) subtreeHasMath(n *dom.Node) bool {
	if n == nil || n.Type != dom.ElementNode {
		return false
	}
	st := e.styleOf(n)
	gen := n.Document().Generation()
	if st.mathGen == gen {
		return st.hasMath
	}
	st.mathGen, st.hasMath = gen, isMathRoot(n)
	if !st.hasMath {
		for c := n.FirstChild; c != nil; c = c.Next {
			if e.subtreeHasMath(c) {
				st.hasMath = true
				break
			}
		}
	}
	return st.hasMath
}

func (e *Engine) isMathFlex(n *dom.Node) bool {
	d := e.display(n)
	return (d == "flex" || d == "inline-flex") && e.subtreeHasMath(n)
}

func (e *Engine) layoutMathFlex(n *dom.Node, avail lu) *htmlBox {
	ts := e.textStyleOf(n)
	b := &htmlBox{node: n, style: ts}
	pad, border := e.edges(n, "padding", "", ts, avail), e.borderEdges(n, ts)
	var items []*htmlBox
	var text []*dom.Node
	flush := func() {
		if len(text) == 0 {
			return
		}
		lb := e.prepareInline(text, n, luInfinite)
		lines := lb.breakLines(luInfinite)
		w := maxLineWidth(lines)
		h := lb.finishLines(lines, w, "start")
		anon := &htmlBox{style: ts, w: w, h: h, lines: lines}
		anon.children = placeAtomicBoxes(anon.children, lines)
		items = append(items, anon)
		text = nil
	}
	for c := n.FirstChild; c != nil; c = c.Next {
		if c.Type == dom.TextNode {
			text = append(text, c)
			continue
		}
		if c.Type != dom.ElementNode || e.display(c) == "none" || e.isOutOfFlow(c) {
			continue
		}
		flush()
		items = append(items, e.layoutBlock(c, luInfinite))
	}
	flush()
	var w, h lu
	for _, c := range items {
		w += c.w
		h = max(h, c.h)
	}
	if value, ok := e.lengthProp(n, "width", ts, avail); ok {
		w = value
	} else if avail < luInfinite && e.display(n) == "flex" {
		w = avail - pad.horizontal() - border.horizontal()
	}
	if value, ok := e.lengthProp(n, "max-width", ts, avail); ok {
		w = min(w, value)
	}
	var used lu
	for _, c := range items {
		used += c.w
	}
	var x lu
	switch e.lookupDeclared(n, "justify-content") {
	case "center":
		x = max(0, (w-used)/2)
	case "flex-end", "end":
		x = max(0, w-used)
	}
	align := e.lookupDeclared(n, "align-items")
	for _, c := range items {
		c.x = border.left + pad.left + x
		c.y = border.top + pad.top
		switch align {
		case "center":
			c.y += (h - c.h) / 2
		case "flex-end", "end":
			c.y += h - c.h
		}
		x += c.w
		b.children = append(b.children, c)
	}
	b.w = w + pad.horizontal() + border.horizontal()
	b.h = h + pad.vertical() + border.vertical()
	return b
}

// mathBodyClientRect supplies the temporary, absolutely positioned math
// label used by mermaid's calculateMathMLDimensions helper.
func (e *Engine) mathBodyClientRect(n *dom.Node) (dom.Rect, bool) {
	top := n
	for top.Parent != nil && top.Parent.Tag != "body" && top.Parent.NS == dom.NSHTML {
		if e.isOutOfFlow(top) {
			break
		}
		top = top.Parent
	}
	if !e.subtreeHasMath(top) || top.Parent == nil || (top.Parent.Tag != "body" && !e.isOutOfFlow(top)) {
		return dom.Rect{}, false
	}
	for p := n; p != nil && p != top.Parent; p = p.Parent {
		if e.display(p) == "none" {
			return dom.Rect{}, true
		}
	}
	b := e.layoutBlock(top, luInfinite)
	var found *htmlBox
	var ox, oy lu
	var find func(*htmlBox, lu, lu)
	find = func(b *htmlBox, x, y lu) {
		x += b.x
		y += b.y
		if b.node == n {
			found = b
			ox, oy = x, y
			return
		}
		for _, c := range b.children {
			find(c, x, y)
		}
	}
	find(b, 0, 0)
	if found == nil {
		return dom.Rect{}, false
	}
	x, y := e.BodyMargin+float64(ox.px()), e.BodyMargin+float64(oy.px())
	if strings.EqualFold(e.lookupDeclared(top, "position"), "absolute") {
		if v, ok := e.lengthProp(top, "top", e.textStyleOf(top), 0); ok {
			y = float64(v.px()) + float64(oy.px())
		}
	}
	return dom.Rect{X: x, Y: y, Width: found.w.px(), Height: found.h.px()}, true
}
