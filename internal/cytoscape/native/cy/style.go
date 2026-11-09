// The values of the style that the geometry reads, by element. Ported from
// cytoscape 3.34.0 (MIT license): src/style/properties.mjs (the default
// values and the default style sheet) and src/collection/dimensions/
// width-height.mjs.

package cy

import "github.com/bvolpato/mermaid-go-renderer/internal/js"

// The default style sheet of cytoscape has the rule
// `:parent { shape: rectangle, padding: 10, border-width: 1 }`.
const parentBorderWidth = 1.0

// stylePadding is pstyle('padding').pfValue.
func (e *Element) stylePadding() float64 {
	if e.IsParent() {
		return e.NodeStyle.ParentPadding
	}
	return e.NodeStyle.Padding
}

// styleBorderWidth is pstyle('border-width').pfValue.
func (e *Element) styleBorderWidth() float64 {
	if e.IsParent() {
		return parentBorderWidth
	}
	return 0
}

// CSSPadding is parseInt(ele.css('padding')): the layouts read the padding
// of a node through the text of the style value, for example "40px".
func (e *Element) CSSPadding() float64 {
	return js.ParseInt(js.Str(e.stylePadding())+"px", 10)
}

// styleWidth is pstyle('width').pfValue of an element.
func (e *Element) styleWidth() float64 {
	if e.isNode {
		return e.NodeStyle.Width
	}
	return e.EdgeStyle.Width
}

// label is pstyle('label').strValue.
func (e *Element) label() string {
	if e.isNode {
		return e.NodeStyle.Label
	}
	return e.EdgeStyle.Label
}

// fontSize is pstyle('font-size').pfValue.
func (e *Element) fontSize() float64 {
	if e.isNode {
		return e.NodeStyle.FontSize
	}
	return e.EdgeStyle.FontSize
}

// The font of a label in the default style of cytoscape.
const (
	fontStyle  = "normal"
	fontWeight = "normal"
	fontFamily = "Helvetica Neue, Helvetica, sans-serif"
)

// width is ele.width() of a node (dimImpl): the size of a compound node is
// the size of its content.
func (e *Element) width() float64 {
	if e.IsParent() {
		e.cy.updateCompoundBounds([]*Element{e}, false)
		// _p.autoWidth || 0
		if e.autoWidth != e.autoWidth || e.autoWidth == 0 {
			return 0
		}
		return e.autoWidth
	}
	return e.NodeStyle.Width
}

func (e *Element) height() float64 {
	if e.IsParent() {
		e.cy.updateCompoundBounds([]*Element{e}, false)
		if e.autoHeight != e.autoHeight || e.autoHeight == 0 {
			return 0
		}
		return e.autoHeight
	}
	return e.NodeStyle.Height
}

// padding is ele.padding().
func (e *Element) padding() float64 {
	if e.IsParent() {
		e.cy.updateCompoundBounds([]*Element{e}, false)
		if e.hasAutoPadding {
			return e.autoPadding
		}
		return e.stylePadding()
	}
	return e.stylePadding()
}

// OuterWidth is ele.outerWidth(): the width with the border and the padding.
// The border position is center.
func (e *Element) OuterWidth() float64 {
	dim := e.width()
	border := e.styleBorderWidth() // n.b. 1/2 each side
	padding := float64(2 * e.padding())
	return dim + border + padding
}

// OuterHeight is ele.outerHeight().
func (e *Element) OuterHeight() float64 {
	dim := e.height()
	border := e.styleBorderWidth()
	padding := float64(2 * e.padding())
	return dim + border + padding
}

// takesUpSpace is eleTakesUpSpace for a node: the display is element, and
// the width and the height are not 0.
func (e *Element) takesUpSpace() bool {
	if !e.isNode {
		return e.EdgeStyle.Width != 0
	}
	return e.width() != 0 && e.height() != 0
}

// LayoutDimensions is ele.layoutDimensions() without the labels: the outer
// size, or 1 by 1 for a node with a size of 0.
func (e *Element) LayoutDimensions() (w, h float64) {
	if !e.takesUpSpace() {
		w, h = 0, 0
	} else {
		w, h = e.OuterWidth(), e.OuterHeight()
	}

	// sanitise the dimensions for external layouts (avoid division by zero)
	if w == 0 || h == 0 {
		w, h = 1, 1
	}
	return w, h
}
