// Ported from cytoscape 3.34.0 (MIT license):
// src/extensions/renderer/base/coord-ele-math/rendered-style.mjs, labels.mjs
// and edge-endpoints.mjs (the getters).

package cy

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// recalculateRenderedStyle is BRp.recalculateRenderedStyle: the renderer
// computes the label position of each node that changed, and the points of
// each edge.
func (cy *Instance) recalculateRenderedStyle(eles []*Element, useCache bool) {
	if len(eles) == 0 {
		return
	}
	var edges []*Element
	var nodes []*Element

	// the renderer can't be used for calcs when destroyed, e.g. ele.boundingBox()
	if cy.destroyed {
		return
	}

	for _, ele := range eles {
		rstyle := &ele.rstyle

		// an edge may be implicitly dirty b/c of one of its connected nodes
		// (and a request for recalc may come in between frames). The flag
		// cleanConnected of a node is set when a frame is drawn, and no frame
		// is drawn before mermaid reads its results: each edge is dirty.
		if !ele.isNode {
			rstyle.clean = false
		}

		// only update if dirty and in graph
		if useCache && rstyle.clean {
			continue
		}

		if ele.isNode {
			nodes = append(nodes, ele)
		} else {
			// edges
			edges = append(edges, ele)
		}
		rstyle.clean = true
	}

	// update node data from projections
	for _, ele := range nodes {
		rstyle := &ele.rstyle
		pos := ele.Position()
		cy.recalculateNodeLabelProjection(ele)
		rstyle.nodeX = pos.X
		rstyle.nodeY = pos.Y
		rstyle.nodeW = ele.NodeStyle.Width
		rstyle.nodeH = ele.NodeStyle.Height
	}
	cy.findEdgeControlPoints(edges)

	// update edge data from projections
	for _, ele := range edges {
		rstyle := &ele.rstyle
		rs := &ele.rs

		// update rstyle positions
		rstyle.srcX = rs.ArrowStartX
		rstyle.srcY = rs.ArrowStartY
		rstyle.tgtX = rs.ArrowEndX
		rstyle.tgtY = rs.ArrowEndY
		rstyle.midX = rs.MidX
		rstyle.midY = rs.MidY
	}
}

// emptyString is the test emptyString of cytoscape: undefined, null, the
// empty string, and a string of white space only.
func emptyString(s string) bool {
	return s == "" || js.TrimSpace(s) == ""
}

// recalculateNodeLabelProjection is BRp.recalculateNodeLabelProjection for
// the horizontal alignment center.
func (cy *Instance) recalculateNodeLabelProjection(node *Element) {
	content := node.label()
	if emptyString(content) {
		return
	}
	var textX, textY float64
	nodeWidth := node.width()
	nodeHeight := node.height()
	padding := node.padding()
	nodePos := node.Position()
	_ = nodeWidth
	rstyle := &node.rstyle
	// text-halign is center.
	textX = nodePos.X
	switch node.NodeStyle.TextValign {
	case "top":
		textY = nodePos.Y - float64(nodeHeight/2) - padding
	case "bottom":
		textY = nodePos.Y + float64(nodeHeight/2) + padding
	default:
		bail.Out("cytoscape: the vertical alignment %q of a label", node.NodeStyle.TextValign)
	}
	rstyle.labelX = textX
	rstyle.labelY = textY
	cy.applyLabelDimensions(node)
}

// applyLabelDimensions is BRp.applyLabelDimensions. An edge also has a
// source label and a target label, which are empty here: the renderer
// measures the empty text for each.
func (cy *Instance) applyLabelDimensions(ele *Element) {
	// save recalc if the label is the same as before: the text and the font
	// of an element do not change.
	if ele.labelMeasured {
		return
	}
	ele.labelMeasured = true
	text := ele.label()
	width, height := cy.calculateLabelDimensions(ele, text)
	// text-wrap is none: one line. line-height is 1.
	ele.rstyle.labelWidth = width
	ele.rstyle.labelHeight = height
	if !ele.isNode {
		cy.calculateLabelDimensions(ele, "")
		cy.calculateLabelDimensions(ele, "")
	}
}

// calculateLabelDimensions is BRp.calculateLabelDimensions with the text
// metrics of the font.
func (cy *Instance) calculateLabelDimensions(ele *Element, text string) (width, height float64) {
	size := ele.fontSize()
	if cy.MeasureText == nil {
		bail.Out("cytoscape: the layout measures a label, and MeasureText is nil")
	}
	font := fontStyle + " " + fontWeight + " " + js.Str(size) + "px " + fontFamily
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		w := math.Ceil(cy.MeasureText(font, line))
		h := size
		width = max(w, width)
		height += h
	}
	return width, height
}

// SourceEndpoint is edge.sourceEndpoint(): the renderer computes the points
// of the edge again, when it is not destroyed.
func (e *Element) SourceEndpoint() Point {
	e.cy.recalculateRenderedStyle([]*Element{e}, true)
	return Point{e.rs.ArrowStartX, e.rs.ArrowStartY}
}

// TargetEndpoint is edge.targetEndpoint().
func (e *Element) TargetEndpoint() Point {
	e.cy.recalculateRenderedStyle([]*Element{e}, true)
	return Point{e.rs.ArrowEndX, e.rs.ArrowEndY}
}

// Midpoint is edge.midpoint().
func (e *Element) Midpoint() Point {
	e.cy.recalculateRenderedStyle([]*Element{e}, true)
	return Point{e.rs.MidX, e.rs.MidY}
}
