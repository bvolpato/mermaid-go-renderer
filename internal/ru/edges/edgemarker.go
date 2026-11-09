// Ported from mermaid 12.2.0, rendering-util/rendering-elements/edgeMarker.ts
// (MIT).

package edges

import (
	"regexp"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// AddEdgeMarkers is addEdgeMarkers(svgPath, edge, url, id, diagramType,
// useMargin, strokeColor): it sets the marker-start and marker-end
// attributes of the path. With a stroke color it points them at a colored
// copy of the marker, which it creates beside the original when needed.
// The arguments arrowTypeStart and arrowTypeEnd are the edge properties;
// an empty strokeColor stands for undefined.
func AddEdgeMarkers(ctx *render.Context, svgPath *dom.Node, arrowTypeStart, arrowTypeEnd, url, id, diagramType string, useMargin bool, strokeColor string) {
	if arrowTypeStart != "" {
		addEdgeMarker(ctx, svgPath, "start", arrowTypeStart, url, id, diagramType, useMargin, strokeColor)
	}
	if arrowTypeEnd != "" {
		addEdgeMarker(ctx, svgPath, "end", arrowTypeEnd, url, id, diagramType, useMargin, strokeColor)
	}
}

type arrowTypeInfo struct {
	typ  string
	fill bool
}

var arrowTypesMap = map[string]arrowTypeInfo{
	"arrow_cross":          {typ: "cross", fill: false},
	"arrow_point":          {typ: "point", fill: true},
	"arrow_barb":           {typ: "barb", fill: true},
	"arrow_barb_neo":       {typ: "barb", fill: true},
	"arrow_circle":         {typ: "circle", fill: false},
	"arrow_hierarchy":      {typ: "hierarchy", fill: false},
	"aggregation":          {typ: "aggregation", fill: false},
	"extension":            {typ: "extension", fill: false},
	"composition":          {typ: "composition", fill: true},
	"dependency":           {typ: "dependency", fill: true},
	"lollipop":             {typ: "lollipop", fill: false},
	"only_one":             {typ: "onlyOne", fill: false},
	"zero_or_one":          {typ: "zeroOrOne", fill: false},
	"one_or_more":          {typ: "oneOrMore", fill: false},
	"zero_or_more":         {typ: "zeroOrMore", fill: false},
	"requirement_arrow":    {typ: "requirement_arrow", fill: false},
	"requirement_contains": {typ: "requirement_contains", fill: false},
}

var arrowTypesWithMarginSupport = map[string]bool{
	"cross":       true,
	"point":       true,
	"circle":      true,
	"lollipop":    true,
	"aggregation": true,
	"extension":   true,
	"composition": true,
	"dependency":  true,
	"barb":        true,
}

var reNotAlphanumeric = regexp.MustCompile(`[^\dA-Za-z]`)

// getElementByID is document.getElementById(id): the first element with
// the id in the document, in tree order.
func getElementByID(doc *dom.Document, id string) *dom.Node {
	var walk func(n *dom.Node) *dom.Node
	walk = func(n *dom.Node) *dom.Node {
		for c := n.FirstChild; c != nil; c = c.Next {
			if c.Type != dom.ElementNode {
				continue
			}
			if v, ok := c.GetAttr("id"); ok && v == id {
				return c
			}
			if found := walk(c); found != nil {
				return found
			}
		}
		return nil
	}
	if doc == nil || doc.Body == nil {
		return nil
	}
	return walk(doc.Body)
}

func addEdgeMarker(ctx *render.Context, svgPath *dom.Node, position, arrowType, url, id, diagramType string, useMargin bool, strokeColor string) {
	// 'none' (and empty) are valid "no arrowhead" values, not unknown types.
	if arrowType == "" || arrowType == "none" {
		return
	}

	arrowTypeInfo, ok := arrowTypesMap[arrowType]
	marginSupport := ok && arrowTypesWithMarginSupport[arrowTypeInfo.typ]

	if !ok {
		return // unknown arrow type, ignore
	}

	endMarkerType := arrowTypeInfo.typ
	suffix := "End"
	if position == "start" {
		suffix = "Start"
	}

	offset := ""
	if useMargin && marginSupport {
		offset = "-margin"
	}
	originalMarkerID := id + "_" + diagramType + "-" + endMarkerType + suffix + offset

	// If stroke color is specified and non-empty, create or use a colored variant of the marker
	if strokeColor != "" && js.TrimSpace(strokeColor) != "" {
		// Create a sanitized color value for use in IDs
		colorID := reNotAlphanumeric.ReplaceAllString(strokeColor, "_")
		coloredMarkerID := originalMarkerID + "_" + colorID

		// Check if the colored marker already exists
		if getElementByID(ctx.Doc, coloredMarkerID) == nil {
			// Get the original marker
			originalMarker := getElementByID(ctx.Doc, originalMarkerID)
			if originalMarker != nil {
				// Clone the marker and create colored version
				coloredMarker := originalMarker.CloneNode(true)
				coloredMarker.Attr("id", coloredMarkerID)

				// Apply colors to the paths inside the marker
				for _, path := range coloredMarker.SelectAll("path, circle, line") {
					path.Attr("stroke", strokeColor)

					// Apply fill only to markers that should be filled
					if arrowTypeInfo.fill {
						path.Attr("fill", strokeColor)
					}
				}

				// Add the new colored marker to the defs section
				if originalMarker.Parent != nil {
					originalMarker.Parent.AppendChild(coloredMarker)
				}
			}
		}

		// Use the colored marker
		svgPath.Attr("marker-"+position, "url("+url+"#"+coloredMarkerID+")")
	} else {
		// Always use the original marker for unstyled edges
		svgPath.Attr("marker-"+position, "url("+url+"#"+originalMarkerID+")")
	}
}
