// Ported from mermaid 12.2.0, rendering-elements/shapes/drawRect.ts (MIT).
// Several shapes share this function, so it is part of the core package.

package ru

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// RectOptions is RectOptions of types.ts.
type RectOptions struct {
	Rx, Ry        float64
	LabelPaddingX float64
	LabelPaddingY float64
	Classes       string
}

// DrawRect is drawRect(parent, node, options).
func DrawRect(ctx *render.Context, parent *dom.Node, node *Node, options RectOptions) (*dom.Node, error) {
	styles := Styles2String(node)
	node.LabelStyle = styles.LabelStyles
	res, err := LabelHelper(ctx, parent, node, GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.ShapeSvg, res.BBox

	totalWidth := js.Max(bbox.Width+float64(options.LabelPaddingX*2), Or(node.Width, 0))
	totalHeight := js.Max(bbox.Height+float64(options.LabelPaddingY*2), Or(node.Height, 0))
	x := -totalWidth / 2
	y := -totalHeight / 2

	var rect *dom.Node
	rx, ry := node.Rx, node.Ry

	// Use the rx and ry of the options when both are present.
	if js.Truthy(options.Rx) && js.Truthy(options.Ry) {
		rx = options.Rx
		ry = options.Ry
	}

	if node.Look == "handDrawn" {
		rc := NewRoughSVG(ctx, shapeSvg)
		roughOptions := UserNodeOverrides(ctx, node, nil)

		var roughNode *dom.Node
		if js.Truthy(rx) || js.Truthy(ry) {
			roughNode, err = rc.Path(CreateRoundedRectPathD(x, y, totalWidth, totalHeight, Or(rx, 0)), roughOptions)
		} else {
			roughNode, err = rc.Rectangle(x, y, totalWidth, totalHeight, roughOptions)
		}
		if err != nil {
			return nil, err
		}
		rect = shapeSvg.InsertNode(roughNode, ":first-child")
		rect.Attr("class", "basic label-container").Attr("style", cssStylesAttr(node))
	} else {
		rect = shapeSvg.Insert("rect", ":first-child")
		rect.
			Attr("class", "basic label-container").
			Attr("style", styles.NodeStyles).
			Attr("rx", NumAttr(rx)).
			Attr("ry", NumAttr(ry)).
			Attr("x", x).
			Attr("y", y).
			Attr("width", totalWidth).
			Attr("height", totalHeight)
	}

	// The geometry of a plain rect is what was just drawn. A hand-drawn
	// rect overflows its nominal box, so it needs a real measurement.
	if node.Look == "handDrawn" {
		UpdateNodeBounds(node, rect)
	} else {
		UpdateNodeBounds(node, rect, Bounds{Width: totalWidth, Height: totalHeight})
	}

	node.CalcIntersect = func(bounds Bounds, point Point) Point {
		return IntersectRect(bounds, point)
	}
	node.Intersect = func(point Point) Point {
		return IntersectRect(NodeBounds(node), point)
	}
	// The await of the caller. A handler that returns drawRect(...) then
	// needs nothing more: the promise it returns resolves one turn later
	// than its own would, and LabelHelper has already awaited.
	ctx.Await()
	return shapeSvg, nil
}

// cssStylesAttr is handleUndefinedAttr(node.cssStyles) as an attribute
// value: an array prints as its elements joined with commas.
func cssStylesAttr(node *Node) any {
	if !node.CSSStylesSet && len(node.CSSStyles) == 0 {
		return nil
	}
	return strings.Join(node.CSSStyles, ",")
}

// CSSStylesAttr is handleUndefinedAttr(node.cssStyles) for shape handlers.
func CSSStylesAttr(node *Node) any { return cssStylesAttr(node) }
