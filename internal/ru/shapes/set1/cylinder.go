// Ported from mermaid 12.2.0, rendering-elements/shapes/cylinder.ts (MIT).

package set1

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("cylinder", cylinder)
}

func createCylinderPathD(x, y, width, height, rx, ry float64) string {
	n := js.Str
	return strings.Join([]string{
		"M" + n(x) + "," + n(y+ry),
		"a" + n(rx) + "," + n(ry) + " 0,0,0 " + n(width) + ",0",
		"a" + n(rx) + "," + n(ry) + " 0,0,0 " + n(-width) + ",0",
		"l0," + n(height),
		"a" + n(rx) + "," + n(ry) + " 0,0,0 " + n(width) + ",0",
		"l0," + n(-height),
	}, " ")
}

func createOuterCylinderPathD(x, y, width, height, rx, ry float64) string {
	n := js.Str
	return strings.Join([]string{
		"M" + n(x) + "," + n(y+ry),
		"M" + n(x+width) + "," + n(y+ry),
		"a" + n(rx) + "," + n(ry) + " 0,0,0 " + n(-width) + ",0",
		"l0," + n(height),
		"a" + n(rx) + "," + n(ry) + " 0,0,0 " + n(width) + ",0",
		"l0," + n(-height),
	}, " ")
}

func createInnerCylinderPathD(x, _, width, height, rx, ry float64) string {
	n := js.Str
	return strings.Join([]string{
		"M" + n(x-float64(width/2)) + "," + n(-height/2),
		"a" + n(rx) + "," + n(ry) + " 0,0,0 " + n(width) + ",0",
	}, " ")
}

const (
	cylinderMinHeight = 8
	cylinderMinWidth  = 8
)

func cylinder(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles

	nodePadding := ru.NN(node.Padding, 0)
	labelPaddingX, labelPaddingY := nodePadding, nodePadding
	if node.Look == "neo" {
		labelPaddingX, labelPaddingY = 24, 24
	}

	originalWidth := ru.NN(node.Width, 0)
	if js.Truthy(node.Width) {
		node.Width = node.Width - labelPaddingY
		if node.Width < cylinderMinWidth {
			node.Width = cylinderMinWidth
		}
	}

	if js.Truthy(node.Height) {
		rx := originalWidth / 2
		ry := rx / (2.5 + float64(originalWidth/50))
		node.Height = node.Height - labelPaddingX - float64(ry*3)

		if node.Height < cylinderMinHeight {
			node.Height = cylinderMinHeight
		}
	}

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label

	w := js.Max(ru.NN(node.Width, 0), bbox.Width) + labelPaddingY
	rx := w / 2
	ry := rx / (2.5 + float64(w/50))
	h := js.Max(ru.NN(node.Height, 0), bbox.Height) + labelPaddingX + ry

	var cylinder *dom.Node

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		outerPathData := createOuterCylinderPathD(0, 0, w, h, rx, ry)
		innerPathData := createInnerCylinderPathD(0, ry, w, h, rx, ry)
		options := ru.UserNodeOverrides(ctx, node, nil)
		outerNode, err := rc.Path(outerPathData, options)
		if err != nil {
			return nil, err
		}
		noFill := js.NewObj[any]()
		noFill.Set("fill", "none")
		innerLine, err := rc.Path(innerPathData, ru.UserNodeOverrides(ctx, node, noFill))
		if err != nil {
			return nil, err
		}

		shapeSvg.InsertNode(innerLine, ":first-child")
		cylinder = shapeSvg.InsertNode(outerNode, ":first-child")
		cylinder.Attr("class", "basic label-container")
		if hasCSSStyles(node) {
			cylinder.Attr("style", cssStylesString(node))
		}
	} else {
		pathData := createCylinderPathD(0, 0, w, h, rx, ry)
		cylinder = shapeSvg.
			Insert("path", ":first-child").
			Attr("d", pathData).
			Attr("class", "basic label-container outer-path").
			Attr("style", ru.CSSStylesAttr(node)).
			Attr("style", styles.NodeStyles)
	}

	cylinder.Attr("label-offset-y", ry)
	cylinder.Attr("transform", "translate("+js.Str(-w/2)+", "+js.Str(-(float64(h/2)+ry))+")")

	ru.UpdateNodeBounds(node, cylinder)

	label.Attr("transform", "translate("+js.Str(labelX(bbox))+", "+
		js.Str(-float64(bbox.Height/2)+ru.NN(node.Padding, 0)/1.5-(bbox.Y-bbox.TopOr0()))+")")

	node.Intersect = func(point ru.Point) ru.Point {
		pos := ru.IntersectRect(ru.NodeBounds(node), point)
		x := pos.X - ru.NN(node.X, 0)

		if rx != 0 &&
			(math.Abs(x) < ru.NN(node.Width, 0)/2 ||
				(math.Abs(x) == ru.NN(node.Width, 0)/2 &&
					math.Abs(pos.Y-ru.NN(node.Y, 0)) > float64(ru.NN(node.Height, 0)/2)-ry)) {
			y := float64(ry * ry * (1 - (x*x)/(rx*rx)))
			if y > 0 {
				y = math.Sqrt(y)
			}
			y = ry - y
			if point.Y-ru.NN(node.Y, 0) > 0 {
				y = -y
			}

			pos.Y += y
		}

		return pos
	}

	return shapeSvg, nil
}
