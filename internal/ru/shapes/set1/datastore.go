// Ported from mermaid 12.2.0, rendering-elements/shapes/datastore.ts (MIT).

package set1

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("datastore", datastore)
}

func datastore(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	// The size of the node before drawRect replaces it.
	width, height := node.Width, node.Height

	rectOptions := ru.RectOptions{
		Rx:            0,
		Ry:            0,
		Classes:       node.CSSClasses,
		LabelPaddingX: ru.NN(node.LabelPaddingX, float64(ru.NN(node.Padding, 0)*2)),
		LabelPaddingY: ru.NN(node.LabelPaddingY, ru.NN(node.Padding, 0)),
	}

	rect, err := ru.DrawRect(ctx, parent, node, rectOptions)
	if err != nil {
		return nil, err
	}

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, rect)
		nodeOverrideOptions := ru.UserNodeOverrides(ctx, node, nil)

		borderPath := rect.Select(".basic.label-container > path:nth-child(2)")
		if borderPath == nil {
			return rect, nil
		}

		bbox := borderPath.BBox()

		rect.InsertNode(
			rc.Line(bbox.X, bbox.Y, bbox.X+bbox.Width, bbox.Y, nodeOverrideOptions),
			".basic.label-container g.label",
		)
		rect.InsertNode(
			rc.Line(bbox.X, bbox.Y+bbox.Height, bbox.X+bbox.Width, bbox.Y+bbox.Height, nodeOverrideOptions),
			".basic.label-container g.label",
		)
		borderPath.Remove()
		return rect, nil
	}

	selection := rect.Select(".basic.label-container")
	// (Number(selection.attr('width')) || width) ?? 0. A missing attribute
	// is null, and Number(null) is 0.
	size := func(attr string, fallback float64) float64 {
		v := 0.0
		if s, ok := selection.GetAttr(attr); ok {
			v = js.Number(s)
		}
		if !js.Truthy(v) {
			v = fallback
		}
		return ru.NN(v, 0)
	}
	datastoreWidth := size("width", width)
	datastoreHeight := size("height", height)
	if datastoreWidth > 0 && datastoreHeight > 0 {
		selection.Attr("stroke-dasharray", js.Str(datastoreWidth)+" "+js.Str(datastoreHeight))
	}

	return rect, nil
}
