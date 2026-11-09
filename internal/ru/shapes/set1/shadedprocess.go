// Ported from mermaid 12.2.0, rendering-elements/shapes/shadedProcess.ts
// (MIT).

package set1

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/rough"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("shadedProcess", shadedProcess)
}

// shadedProcessFrameWidth is the width of the frame on the left of the
// shape.
const shadedProcessFrameWidth = 8

func shadedProcess(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles

	paddingX, paddingY := ru.NN(node.Padding, 0), ru.NN(node.Padding, 0)
	if node.Look == "neo" {
		paddingX, paddingY = 16, 12
	}
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label
	frame := float64(shadedProcessFrameWidth * 2)
	if node.Look == "neo" {
		frame = shadedProcessFrameWidth
	}
	totalWidth := ru.NN(node.Width, bbox.Width) + float64(paddingX*2) + frame
	totalHeight := ru.NN(node.Height, bbox.Height) + float64(paddingY*2)
	w := totalWidth - shadedProcessFrameWidth
	h := totalHeight
	x := shadedProcessFrameWidth - float64(totalWidth/2)
	y := float64(-totalHeight / 2)

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := ru.UserNodeOverrides(ctx, node, nil)

	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
		options.Set("fillStyle", "solid")
	}

	points := []rough.Point{
		{x, y},
		{x + w, y},
		{x + w, y + h},
		{x - shadedProcessFrameWidth, y + h},
		{x - shadedProcessFrameWidth, y},
		{x, y},
		{x, y + h},
	}

	roughNode, err := rc.Polygon(points, options)
	if err != nil {
		return nil, err
	}

	rect := shapeSvg.InsertNode(roughNode, ":first-child")

	rect.
		Attr("class", "basic label-container outer-path").
		Attr("style", ru.CSSStylesAttr(node))

	if styles.NodeStyles != "" && node.Look != "handDrawn" {
		rect.SelectAll("path").Attr("style", styles.NodeStyles)
	}

	if hasCSSStyles(node) && node.Look != "handDrawn" {
		rect.SelectAll("path").Attr("style", styles.NodeStyles)
	}

	// The inner main rect is centered at half the frame width, not at 0.
	// The label moves right by that amount, so that it is centered inside
	// the main rect.
	label.Attr("transform", "translate("+
		js.Str(shadedProcessFrameWidth/2-float64(bbox.Width/2)-(bbox.X-bbox.LeftOr0()))+", "+
		js.Str(-float64(bbox.Height/2)-(bbox.Y-bbox.TopOr0()))+")")

	ru.UpdateNodeBounds(node, rect)

	rectIntersect(node)

	return shapeSvg, nil
}
