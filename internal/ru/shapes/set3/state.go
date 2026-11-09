// Ported from mermaid 12.2.0, rendering-elements/shapes/state.ts (MIT).

package set3

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("state", state)
}

func state(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	radius := 5.0
	if node.Look == "neo" {
		radius = 3
	}
	// The options have no label padding: it is undefined upstream, so the
	// size of the rect is NaN.
	options := ru.RectOptions{
		Rx:            radius,
		Ry:            radius,
		Classes:       "flowchart-node",
		LabelPaddingX: ru.Undef,
		LabelPaddingY: ru.Undef,
	}
	return ru.DrawRect(ctx, parent, node, options)
}
