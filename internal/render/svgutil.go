// Ported from mermaid 12.2.0, setupGraphViewbox.js,
// rendering-util/setupViewPortForSVG.ts, rendering-util/selectSvgElement.ts,
// rendering-util/insertElementsForSize.js and utils.ts (insertTitle) (MIT).

package render

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// SelectSvgElement is selectSvgElement(id): root.select(`#${id}`).
func (c *Context) SelectSvgElement() *dom.Node {
	return c.Body.Select("#" + c.ID)
}

// GetDiagramElement is getDiagramElement(id, securityLevel):
// root.select(`[id="${id}"]`).
func (c *Context) GetDiagramElement() *dom.Node {
	return c.Body.Select(`[id="` + c.ID + `"]`)
}

// ConfigureSvgSize is configureSvgSize(svgElem, height, width, useMaxWidth).
func ConfigureSvgSize(svg *dom.Node, height, width float64, useMaxWidth bool) {
	if useMaxWidth {
		svg.Attr("width", "100%")
		svg.Attr("style", "max-width: "+js.Str(width)+"px;")
	} else {
		svg.Attr("height", height)
		svg.Attr("width", width)
	}
}

// SetupGraphViewbox is setupGraphViewbox(graph, svgElem, padding, useMaxWidth).
func SetupGraphViewbox(svg *dom.Node, padding float64, useMaxWidth bool) {
	b := svg.BBox()
	width := b.Width + padding*2
	height := b.Height + padding*2
	ConfigureSvgSize(svg, height, width, useMaxWidth)
	vBox := js.Str(b.X-padding) + " " + js.Str(b.Y-padding) + " " + js.Str(b.Width+2*padding) + " " + js.Str(b.Height+2*padding)
	svg.Attr("viewBox", vBox)
}

// SetupViewPortForSVG is setupViewPortForSVG(svg, padding, cssDiagram, useMaxWidth).
func SetupViewPortForSVG(svg *dom.Node, padding float64, cssDiagram string, useMaxWidth bool) {
	svg.Attr("class", cssDiagram)
	b := svg.BBox()
	width := b.Width + padding*2
	height := b.Height + padding*2
	ConfigureSvgSize(svg, height, width, useMaxWidth)
	viewBox := js.Str(b.X-padding) + " " + js.Str(b.Y-padding) + " " + js.Str(width) + " " + js.Str(height)
	svg.Attr("viewBox", viewBox)
}

// InsertTitle is utils.insertTitle(parent, cssClass, titleTopMargin, title).
func InsertTitle(parent *dom.Node, cssClass string, titleTopMargin float64, title string) {
	if title == "" {
		return
	}
	bounds := parent.BBox()
	parent.Append("text").
		Text(title).
		Attr("text-anchor", "middle").
		Attr("x", bounds.X+float64(bounds.Width/2)).
		Attr("y", -titleTopMargin).
		Attr("class", cssClass)
}
