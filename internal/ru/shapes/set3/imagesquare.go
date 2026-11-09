// Ported from mermaid 12.2.0, rendering-elements/shapes/imageSquare.ts (MIT).

package set3

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("imageSquare", imageSquare)
}

func imageSquare(ctx *render.Context, parent *dom.Node, node *ru.Node, renderOptions *ru.ShapeRenderOptions) (*dom.Node, error) {
	opts, err := readIconOptions(renderOptions)
	if err != nil {
		return nil, err
	}
	// const img = new Image(); img.src = node?.img ?? ''; await img.decode();
	// An image that the browser cannot load or decode fails the render.
	img, err := naturalImageSize(ctx, node.Img)
	if err != nil {
		return nil, err
	}

	imageNaturalWidth := img.Width
	imageNaturalHeight := img.Height
	node.ImageAspectRatio = imageNaturalWidth / imageNaturalHeight

	node.LabelStyle = ru.Styles2String(node).LabelStyles

	defaultWidth := opts.defaultWidth
	node.DefaultWidth = opts.defaultWidth

	labelWidth := 0.0
	if node.Label != "" {
		labelWidth = ru.NN(defaultWidth, 0)
	}
	imageRawWidth := js.Max(labelWidth, ru.NN(node.AssetWidth, imageNaturalWidth))

	imageWidth := imageRawWidth
	if node.Constraint == "on" && js.Truthy(node.AssetHeight) {
		imageWidth = node.AssetHeight * node.ImageAspectRatio
	}

	var imageHeight float64
	if node.Constraint == "on" {
		imageHeight = imageWidth / node.ImageAspectRatio
	} else {
		imageHeight = ru.NN(node.AssetHeight, imageNaturalHeight)
	}
	node.Width = js.Max(imageWidth, ru.NN(defaultWidth, 0))
	res, err := ru.LabelHelper(ctx, parent, node, "image-shape default")
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label

	topLabel := node.Pos == "t"

	x := -imageWidth / 2
	y := -imageHeight / 2

	labelPadding := labelPaddingOf(node)

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := ru.UserNodeOverrides(ctx, node, roughOptions())

	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
		options.Set("fillStyle", "solid")
	}

	imageNode, err := rc.Rectangle(x, y, imageWidth, imageHeight, options)
	if err != nil {
		return nil, err
	}

	outerWidth := js.Max(imageWidth, bbox.Width)
	outerHeight := imageHeight + bbox.Height + labelPadding

	outerNode, err := rc.Rectangle(-outerWidth/2, -outerHeight/2, outerWidth, outerHeight, spread(options, "fill", "none", "stroke", "none"))
	if err != nil {
		return nil, err
	}

	iconShape := shapeSvg.InsertNode(imageNode, ":first-child")
	outerShape := shapeSvg.InsertNode(outerNode)

	if node.Img != "" {
		image := shapeSvg.Append("image")

		// Set the image attributes.
		image.Attr("href", node.Img)
		image.Attr("width", imageWidth)
		image.Attr("height", imageHeight)
		image.Attr("preserveAspectRatio", "none")

		var imageY float64
		if topLabel {
			imageY = float64(outerHeight/2) - imageHeight
		} else {
			imageY = -outerHeight / 2
		}
		image.Attr("transform", "translate("+js.Str(-imageWidth/2)+","+js.Str(imageY)+")")
	}

	var labelY float64
	if topLabel {
		labelY = float64(-imageHeight/2) - float64(bbox.Height/2) - float64(labelPadding/2)
	} else {
		labelY = float64(imageHeight/2) - float64(bbox.Height/2) + float64(labelPadding/2)
	}
	label.Attr("transform", "translate("+js.Str(labelOffsetX(bbox))+","+js.Str(labelY)+")")

	iconShape.Attr("transform", "translate("+js.Str(0)+","+js.Str(iconShapeOffsetY(bbox, labelPadding, topLabel))+")")

	ru.UpdateNodeBounds(node, outerShape)

	node.Intersect = iconIntersect(node, bbox, imageWidth, imageHeight, labelPadding, topLabel)

	return shapeSvg, nil
}
