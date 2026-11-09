// Ported from mermaid 12.2.0, rendering-elements/shapes/tiltedCylinder.ts
// (MIT).

package set2

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("tiltedCylinder", tiltedCylinder)
}

// tiltedCylinderPathD is createCylinderPathD of tiltedCylinder.ts. The line
// breaks and the indentation are part of the template literal.
func tiltedCylinderPathD(x, y, width, height, rx, ry float64) string {
	n := js.Str
	return "M" + n(x) + "," + n(y) + "\n" +
		"    a" + n(rx) + "," + n(ry) + " 0,0,1 " + n(0) + "," + n(-height) + "\n" +
		"    l" + n(width) + "," + n(0) + "\n" +
		"    a" + n(rx) + "," + n(ry) + " 0,0,1 " + n(0) + "," + n(height) + "\n" +
		"    M" + n(width) + "," + n(-height) + "\n" +
		"    a" + n(rx) + "," + n(ry) + " 0,0,0 " + n(0) + "," + n(height) + "\n" +
		"    l" + n(-width) + "," + n(0)
}

// tiltedCylinderOuterPathD is createOuterCylinderPathD of tiltedCylinder.ts.
func tiltedCylinderOuterPathD(x, y, width, height, rx, ry float64) string {
	n := js.Str
	return strings.Join([]string{
		"M" + n(x) + "," + n(y),
		"M" + n(x+width) + "," + n(y),
		"a" + n(rx) + "," + n(ry) + " 0,0,0 " + n(0) + "," + n(-height),
		"l" + n(-width) + ",0",
		"a" + n(rx) + "," + n(ry) + " 0,0,0 " + n(0) + "," + n(height),
		"l" + n(width) + ",0",
	}, " ")
}

// tiltedCylinderInnerPathD is createInnerCylinderPathD of tiltedCylinder.ts.
func tiltedCylinderInnerPathD(x, _, width, height, rx, ry float64) string {
	n := js.Str
	return strings.Join([]string{
		"M" + n(x+float64(width/2)) + "," + n(-height/2),
		"a" + n(rx) + "," + n(ry) + " 0,0,0 0," + n(height),
	}, " ")
}

const (
	tiltedCylinderMinHeight = 5
	tiltedCylinderMinWidth  = 10
)

func tiltedCylinder(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	nodePadding := ru.NN(node.Padding, 0)
	labelPadding := tern(node.Look == "neo", 12, float64(nodePadding/2))
	originalHeight := ru.NN(node.Height, 0)
	if truthy(node.Height) {
		node.Height = node.Height - labelPadding
		if node.Height < tiltedCylinderMinHeight {
			node.Height = tiltedCylinderMinHeight
		}
	}

	if truthy(node.Width) {
		ry := originalHeight / 2
		// The width follows from this height.
		rx := ry / (2.5 + originalHeight/50)
		node.Width = node.Width - labelPadding - float64(rx*3)
		if node.Width < tiltedCylinderMinWidth {
			node.Width = tiltedCylinderMinWidth
		}
	}
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label

	h := js.Max(ru.NN(node.Height, 0), bbox.Height) + labelPadding
	ry := h / 2
	rx := ry / (2.5 + h/50)
	w := js.Max(ru.NN(node.Width, 0), bbox.Width) + rx + labelPadding

	var cylinder *dom.Node

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		outerPathData := tiltedCylinderOuterPathD(0, 0, w, h, rx, ry)
		innerPathData := tiltedCylinderInnerPathD(0, 0, w, h, rx, ry)
		outerNode, err := rc.Path(outerPathData, ru.UserNodeOverrides(ctx, node, roughOptions()))
		if err != nil {
			return nil, err
		}
		innerLine, err := rc.Path(innerPathData, ru.UserNodeOverrides(ctx, node, roughOptions("fill", "none")))
		if err != nil {
			return nil, err
		}
		shapeSvg.InsertNode(innerLine, ":first-child")
		cylinder = shapeSvg.InsertNode(outerNode, ":first-child")
		cylinder.Attr("class", "basic label-container")
		if hasCSSStyles(node) {
			cylinder.Attr("style", ru.CSSStylesAttr(node))
		}
	} else {
		pathData := tiltedCylinderPathD(0, 0, w, h, rx, ry)
		cylinder = shapeSvg.
			Insert("path", ":first-child").
			Attr("d", pathData).
			Attr("class", "basic label-container").
			Attr("style", ru.CSSStylesAttr(node)).
			Attr("style", nodeStyles)
		cylinder.Attr("class", "basic label-container outer-path")

		if hasCSSStyles(node) {
			cylinder.SelectAll("path").Attr("style", ru.CSSStylesAttr(node))
		}

		if nodeStyles != "" {
			cylinder.SelectAll("path").Attr("style", nodeStyles)
		}
	}

	cylinder.Attr("label-offset-x", rx)
	cylinder.Attr("transform", "translate("+js.Str(-w/2)+", "+js.Str(h/2)+" )")

	label.Attr(
		"transform",
		"translate("+js.Str(-float64(bbox.Width/2)-rx-(bbox.X-bbox.LeftOr0()))+", "+
			js.Str(-float64(bbox.Height/2)-(bbox.Y-bbox.TopOr0()))+")",
	)

	ru.UpdateNodeBounds(node, cylinder)

	node.Intersect = func(point ru.Point) ru.Point {
		pos := ru.IntersectRect(ru.NodeBounds(node), point)
		y := pos.Y - ru.NN(node.Y, 0)

		if ry != 0 &&
			(math.Abs(y) < ru.NN(node.Height, 0)/2 ||
				(math.Abs(y) == ru.NN(node.Height, 0)/2 &&
					math.Abs(pos.X-ru.NN(node.X, 0)) > float64(ru.NN(node.Width, 0)/2)-rx)) {
			x := float64(rx * rx * (1 - float64(y*y)/float64(ry*ry)))
			if x != 0 {
				x = math.Sqrt(math.Abs(x))
			}
			x = rx - x
			if point.X-ru.NN(node.X, 0) > 0 {
				x = -x
			}

			pos.X += x
		}

		return pos
	}

	return shapeSvg, nil
}
