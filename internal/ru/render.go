// Ported from mermaid 12.2.0, rendering-util/render.ts and
// rendering-util/insertLookDefs.ts (MIT).

package ru

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// Render is render(data4Layout, svg): it runs the layout algorithm of the
// layout data, which also draws the diagram.
func Render(ctx *render.Context, data *LayoutData, svg *dom.Node) error {
	var layoutFn LayoutAlgorithm
	var algorithm string
	if name, l, ok := lookupPageLayout(ctx, data); ok {
		// The page registered the layout that the config asks for, so
		// getRegisteredLayoutAlgorithm returned its name and no fallback.
		data.LayoutAlgorithm = name
		layoutFn, algorithm = l.fn, l.algorithm
	} else {
		var err error
		layoutFn, algorithm, err = lookupLayout(data.LayoutAlgorithm)
		if err != nil {
			return err
		}
	}

	// Prefix the DOM ids of the nodes with the id of the svg, so that they
	// are unique across the diagrams of a page.
	if data.DiagramID != "" {
		for _, node := range data.Nodes {
			originalDomID := node.DomID
			if originalDomID == "" {
				originalDomID = node.ID
			}
			node.DomID = data.DiagramID + "-" + originalDomID
		}
	}

	InsertLookDefs(svg, data.Config)

	return layoutFn(ctx, data, svg, algorithm)
}

// InsertLookDefs is insertLookDefs(svg, config): it appends the <defs>
// that the style rules of the neo look point at.
func InsertLookDefs(svg *dom.Node, cfg *js.Obj[any]) {
	theme, _ := cfg.Get("theme").(string)
	themeVariables := js.AsObj(cfg.Get("themeVariables"))
	var useGradient, gradientStart, gradientStop any
	if themeVariables != nil {
		useGradient = themeVariables.Get("useGradient")
		gradientStart = themeVariables.Get("gradientStart")
		gradientStop = themeVariables.Get("gradientStop")
	}
	svgID := svg.AttrOr("id", "")
	// On a dark theme a black shadow is invisible.
	floodColor := "#000000"
	if strings.Contains(theme, "dark") {
		floodColor = "#FFFFFF"
	}

	dropShadow := func(id, size, offset string) {
		svg.Append("defs").
			Append("filter").
			Attr("id", id).
			Attr("height", size).
			Attr("width", size).
			Append("feDropShadow").
			Attr("dx", offset).
			Attr("dy", offset).
			Attr("stdDeviation", 0).
			Attr("flood-opacity", "0.06").
			Attr("flood-color", floodColor)
	}
	dropShadow(svgID+"-drop-shadow", "130%", "4")
	dropShadow(svgID+"-drop-shadow-small", "150%", "2")

	if !js.Truthy(useGradient) {
		return
	}

	gradient := svg.Append("linearGradient").
		Attr("id", svgID+"-gradient").
		Attr("gradientUnits", "objectBoundingBox").
		Attr("x1", "0%").
		Attr("y1", "0%").
		Attr("x2", "100%").
		Attr("y2", "0%")

	gradient.Append("svg:stop").
		Attr("offset", "0%").
		Attr("stop-color", attrValue(gradientStart)).
		Attr("stop-opacity", 1)

	gradient.Append("svg:stop").
		Attr("offset", "100%").
		Attr("stop-color", attrValue(gradientStop)).
		Attr("stop-opacity", 1)
}

// attrValue converts a dynamic value for selection.attr: undefined and
// null remove the attribute, and other values are converted to a string.
func attrValue(v any) any {
	if js.IsNullish(v) {
		return nil
	}
	return js.ToString(v)
}
