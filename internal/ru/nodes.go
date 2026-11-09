// Ported from mermaid 12.2.0, rendering-elements/nodes.ts (MIT).

package ru

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// nodeElems is the module-level Map of nodes.ts, kept per render.
func nodeElems(ctx *render.Context) map[string]*dom.Node {
	return ctx.Value("ru.nodeElems", func() any { return map[string]*dom.Node{} }).(map[string]*dom.Node)
}

// InsertNode is insertNode(elem, node, renderOptions).
func InsertNode(ctx *render.Context, elem *dom.Node, node *Node, renderOptions *ShapeRenderOptions) (*dom.Node, error) {
	// The rect shape has two variants: with and without rounded corners.
	if node.Shape == "rect" {
		if js.Truthy(node.Rx) && js.Truthy(node.Ry) {
			node.Shape = "roundedRect"
		} else {
			node.Shape = "squareRect"
		}
	}
	if node.Shape == "" {
		return nil, noSuchShape(node.Shape)
	}
	shapeHandler, err := LookupShape(node.Shape)
	if err != nil {
		return nil, err
	}

	var newEl, el *dom.Node
	if node.Link != "" {
		var target any
		if js.ToString(js.Path(renderOptions.Config, "securityLevel")) == "sandbox" {
			target = "_top"
		} else if node.LinkTarget != "" {
			target = node.LinkTarget
		}
		newEl = elem.Insert("svg:a").Attr("xlink:href", node.Link).Attr("target", target)
		if el, err = shapeHandler(ctx, newEl, node, renderOptions); err != nil {
			return nil, err
		}
	} else {
		if el, err = shapeHandler(ctx, elem, node, renderOptions); err != nil {
			return nil, err
		}
		newEl = el
	}
	// await shapeHandler(...). A handler that is not async also takes one
	// turn of the microtask queue here.
	ctx.Await()
	// Data attributes for the neo look.
	if node.Look != "" {
		newEl.Attr("data-look", node.Look)
	}

	if node.Tooltip != "" {
		el.Attr("title", node.Tooltip)
	}

	nodeElems(ctx)[node.ID] = newEl

	if node.HaveCallback {
		newEl.Attr("class", newEl.AttrOr("class", "null")+" clickable")
	}
	// The await of the caller. See render/async.go for the convention.
	ctx.Await()
	return newEl, nil
}

func noSuchShape(shape string) error {
	if shape == "" {
		shape = "undefined"
	}
	return &render.UnsupportedError{Type: "shape " + shape}
}

// SetNodeElem is setNodeElem(elem, node).
func SetNodeElem(ctx *render.Context, elem *dom.Node, nodeID string) {
	nodeElems(ctx)[nodeID] = elem
}

// ClearNodes is clear() of nodes.ts.
func ClearNodes(ctx *render.Context) {
	m := nodeElems(ctx)
	for k := range m {
		delete(m, k)
	}
}

// PositionedNode is the data that positionNode reads from a laid-out node.
type PositionedNode struct {
	ID            string
	X, Y          float64
	Width, Height float64
	// Diff is node.diff, or undefined.
	Diff        float64
	ClusterNode bool
}

// PositionNode is positionNode(node): it moves the element of a node to
// its position and returns node.diff || 0.
func PositionNode(ctx *render.Context, node PositionedNode) float64 {
	el := nodeElems(ctx)[node.ID]
	const padding = 8
	diff := Or(node.Diff, 0)
	if node.ClusterNode {
		el.Attr("transform", "translate("+js.Str(node.X+diff-float64(node.Width/2))+", "+js.Str(node.Y-float64(node.Height/2)-padding)+")")
	} else {
		el.Attr("transform", "translate("+js.Str(node.X)+", "+js.Str(node.Y)+")")
	}
	return diff
}
