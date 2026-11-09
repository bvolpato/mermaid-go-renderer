// Ported from mermaid 12.2.0, rendering-elements/shapes/iconRounded.ts (MIT).

package set3

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("iconRounded", iconRounded)
}

// iconRounded.ts is iconSquare.ts with a corner radius of 5 and the class
// icon-shape2 on the icon box.
func iconRounded(ctx *render.Context, parent *dom.Node, node *ru.Node, renderOptions *ru.ShapeRenderOptions) (*dom.Node, error) {
	return iconBox(ctx, parent, node, renderOptions, 5, "icon-shape2")
}
