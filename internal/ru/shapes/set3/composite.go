// Ported from mermaid 12.2.0, rendering-elements/shapes/composite.ts (MIT).

package set3

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("composite", composite)
}

// The composite node of a block diagram has the extra properties class,
// positioned and style.
func composite(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	// ['node', cssClasses, class].filter(Boolean).join(' ')
	classList := []string{"node"}
	if node.CSSClasses != "" {
		classList = append(classList, node.CSSClasses)
	}
	if class := node.Get("class"); js.Truthy(class) {
		classList = append(classList, js.ToString(class))
	}
	classes := strings.Join(classList, " ")
	res, err := labelHelper(ctx, parent, node, classes)
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, halfPadding := res.ShapeSvg, res.BBox, res.HalfPadding
	rect := shapeSvg.Insert("rect", ":first-child")
	nodePadding := ru.NN(node.Padding, 0)
	positioned := js.Truthy(node.Get("positioned"))
	var totalWidth, totalHeight, x, y float64
	if positioned {
		totalWidth = ru.NN(node.Width, 0)
		totalHeight = ru.NN(node.Height, 0)
		x = -totalWidth / 2
		y = -totalHeight / 2
	} else {
		totalWidth = bbox.Width + nodePadding
		totalHeight = bbox.Height + nodePadding
		x = float64(-bbox.Width/2) - halfPadding
		y = float64(-bbox.Height/2) - halfPadding
	}

	rect.
		Attr("class", "basic cluster composite label-container").
		Attr("style", attrValue(node.Get("style"))).
		Attr("rx", ru.NumAttr(node.Rx)).
		Attr("ry", ru.NumAttr(node.Ry)).
		Attr("x", x).
		Attr("y", y).
		Attr("width", totalWidth).
		Attr("height", totalHeight)

	ru.UpdateNodeBounds(node, rect)

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectRect(ru.NodeBounds(node), point)
	}

	return shapeSvg, nil
}
