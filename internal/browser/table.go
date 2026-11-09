// This file ports no upstream file. It has the heights that a percentage
// refers to, and the cell of a table that fills its table, with the values
// of Chromium that testdata/tablecell-cases.json has.

package browser

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

// This file has the heights that a percentage refers to, and the cell of a
// table that fills its table. The journey and eventmodeling diagrams center
// a text with a div of display: table, width: 100% and height: 100% in a
// foreignObject, and a table-cell with vertical-align: middle in it.
// testdata/tablecell-cases.json has the values of Chromium.

// tableOf returns the element with display: table that a table-cell or a
// table-row belongs to, through its rows and row groups, or nil. A
// table-cell with no table around it is in an anonymous table that is as
// large as the cell.
func (e *Engine) tableOf(n *dom.Node) *dom.Node {
	for p := n.Parent; p != nil && p.Type == dom.ElementNode && p.NS == dom.NSHTML; p = p.Parent {
		switch e.display(p) {
		case "table", "inline-table":
			return p
		case "table-row", "table-row-group", "table-header-group", "table-footer-group":
			continue
		}
		return nil
	}
	return nil
}

// containingHeight is the height that a percentage height of n refers to:
// the height of the content box of the containing block when it is
// definite, or luInfinite. A foreignObject has a definite height. A block
// has one when its own height is a length, or a percentage of a definite
// height.
func (e *Engine) containingHeight(n *dom.Node) lu {
	p := n.Parent
	for p != nil && p.Type == dom.ElementNode && p.NS == dom.NSHTML && !isBlockLevelDisplay(e.display(p)) {
		p = p.Parent
	}
	if p == nil || p.Type != dom.ElementNode {
		return luInfinite
	}
	if p.NS == dom.NSSVG {
		if p.Tag == "foreignObject" {
			return e.foreignObjectFrame(p).h
		}
		return luInfinite
	}
	if h, ok := e.definiteContentHeight(p); ok {
		return h
	}
	return luInfinite
}

// definiteContentHeight is the height of the content box of a block that
// the height property gives, when it is definite.
func (e *Engine) definiteContentHeight(n *dom.Node) (lu, bool) {
	ts := e.textStyleOf(n)
	h, ok := e.lengthProp(n, "height", ts, e.containingHeight(n))
	if !ok {
		return 0, false
	}
	if strings.EqualFold(strings.TrimSpace(e.lookupDeclared(n, "box-sizing")), "border-box") {
		pad := e.edges(n, "padding", "", ts, 0)
		border := e.borderEdges(n, ts)
		h = max(0, h-pad.vertical()-border.vertical())
	}
	return h, true
}

// cellStretch returns how far the content of a table-cell moves down when
// the cell is higher than its content: the row of a table is as high as
// the table. free is the height of the cell that the content does not use.
func (e *Engine) cellStretch(cell *dom.Node, free lu) lu {
	v, _ := e.declared(cell, "vertical-align")
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "middle":
		return free / 2
	case "bottom":
		return free
	}
	return 0
}
