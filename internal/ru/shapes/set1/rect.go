// Ported from mermaid 12.2.0, rendering-elements/shapes/squareRect.ts and
// roundedRect.ts (MIT).

// Package set1 holds the first group of node shapes of the unified
// renderer. Each handler registers itself under the name that it has in
// rendering-elements/shapes.ts.
package set1

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("squareRect", squareRect)
	ru.RegisterShape("roundedRect", roundedRect)
}

func squareRect(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	nodePadding := ru.NN(node.Padding, 0)
	labelPaddingX := float64(nodePadding * 2)
	labelPaddingY := nodePadding
	if node.Look == "neo" {
		labelPaddingX = 16
		labelPaddingY = 12
	}

	options := ru.RectOptions{
		Rx:            0,
		Ry:            0,
		Classes:       "",
		LabelPaddingX: ru.NN(node.LabelPaddingX, labelPaddingX),
		LabelPaddingY: labelPaddingY,
	}
	shapeSvg, err := ru.DrawRect(ctx, parent, node, options)
	if err != nil {
		return nil, err
	}

	// The color slot of the item, where a diagram assigned one.
	if !ru.IsUndef(node.ColorIndex) {
		ru.StampColorSlot(shapeSvg, node.ColorIndex, ctx.Cfg("theme"), ctx.Cfg("themeVariables", "borderColorArray"))
	}
	return shapeSvg, nil
}

func roundedRect(ctx *render.Context, parent *dom.Node, node *ru.Node, options *ru.ShapeRenderOptions) (*dom.Node, error) {
	radius := 5.0
	if v := js.Path(options.Config, "themeVariables", "radius"); !js.IsNullish(v) {
		radius = js.ToNumber(v)
	}
	rectOptions := ru.RectOptions{
		Rx:            radius,
		Ry:            radius,
		Classes:       "",
		LabelPaddingX: float64(ru.NN(node.Padding, 0) * 1),
		LabelPaddingY: float64(ru.NN(node.Padding, 0) * 1),
	}
	return ru.DrawRect(ctx, parent, node, rectOptions)
}
