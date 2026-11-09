// Ported from mermaid 12.2.0, rendering-elements/shapes/linedCylinder.ts
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
	ru.RegisterShape("linedCylinder", linedCylinder)
}

// linedCylinderPathD is createCylinderPathD of linedCylinder.ts.
func linedCylinderPathD(x, y, width, height, rx, ry, outerOffset float64) string {
	n := js.Str
	return strings.Join([]string{
		"M" + n(x) + "," + n(y+ry),
		"a" + n(rx) + "," + n(ry) + " 0,0,0 " + n(width) + ",0",
		"a" + n(rx) + "," + n(ry) + " 0,0,0 " + n(-width) + ",0",
		"l0," + n(height),
		"a" + n(rx) + "," + n(ry) + " 0,0,0 " + n(width) + ",0",
		"l0," + n(-height),
		"M" + n(x) + "," + n(y+ry+outerOffset),
		"a" + n(rx) + "," + n(ry) + " 0,0,0 " + n(width) + ",0",
	}, " ")
}

// linedCylinderOuterPathD is createOuterCylinderPathD of linedCylinder.ts.
func linedCylinderOuterPathD(x, y, width, height, rx, ry, outerOffset float64) string {
	n := js.Str
	return strings.Join([]string{
		"M" + n(x) + "," + n(y+ry),
		"M" + n(x+width) + "," + n(y+ry),
		"a" + n(rx) + "," + n(ry) + " 0,0,0 " + n(-width) + ",0",
		"l0," + n(height),
		"a" + n(rx) + "," + n(ry) + " 0,0,0 " + n(width) + ",0",
		"l0," + n(-height),
		"M" + n(x) + "," + n(y+ry+outerOffset),
		"a" + n(rx) + "," + n(ry) + " 0,0,0 " + n(width) + ",0",
	}, " ")
}

// linedCylinderInnerPathD is createInnerCylinderPathD of linedCylinder.ts.
func linedCylinderInnerPathD(x, _, width, height, rx, ry float64) string {
	n := js.Str
	return strings.Join([]string{
		"M" + n(x-float64(width/2)) + "," + n(-height/2),
		"a" + n(rx) + "," + n(ry) + " 0,0,0 " + n(width) + ",0",
	}, " ")
}

const (
	linedCylinderMinHeight = 10
	linedCylinderMinWidth  = 10
)

func linedCylinder(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	nodePadding := ru.NN(node.Padding, 0)
	labelPaddingX := tern(node.Look == "neo", 16, nodePadding)
	labelPaddingY := tern(node.Look == "neo", 24, nodePadding)

	if truthy(node.Width) || truthy(node.Height) {
		originalWidth := ru.NN(node.Width, 0)
		node.Width = ru.NN(node.Width, 0) - labelPaddingX
		if node.Width < linedCylinderMinWidth {
			node.Width = linedCylinderMinWidth
		}

		rx := originalWidth / 2

		// The height follows from this width.
		ry := rx / (2.5 + originalWidth/50)
		node.Height = ru.NN(node.Height, 0) - labelPaddingY - float64(ry*3)
		if node.Height < linedCylinderMinHeight {
			node.Height = linedCylinderMinHeight
		}
	}
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label

	w := ru.Or(node.Width, bbox.Width) + float64(labelPaddingX*2)
	rx := w / 2
	ry := rx / (2.5 + w/50)
	h := ru.Or(node.Height, bbox.Height) + ry + float64(labelPaddingY*2)
	outerOffset := float64(h * 0.1) // 10% of the height

	var cylinder *dom.Node

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		outerPathData := linedCylinderOuterPathD(0, 0, w, h, rx, ry, outerOffset)
		innerPathData := linedCylinderInnerPathD(0, ry, w, h, rx, ry)
		options := ru.UserNodeOverrides(ctx, node, roughOptions())

		outerNode, err := rc.Path(outerPathData, options)
		if err != nil {
			return nil, err
		}
		innerLine, err := rc.Path(innerPathData, options)
		if err != nil {
			return nil, err
		}

		innerLineEl := shapeSvg.InsertNode(innerLine, ":first-child")
		innerLineEl.Attr("class", "line")
		cylinder = shapeSvg.InsertNode(outerNode, ":first-child")
		cylinder.Attr("class", "basic label-container")
		if hasCSSStyles(node) {
			cylinder.Attr("style", ru.CSSStylesAttr(node))
		}
	} else {
		pathData := linedCylinderPathD(0, 0, w, h, rx, ry, outerOffset)
		cylinder = shapeSvg.
			Insert("path", ":first-child").
			Attr("d", pathData).
			Attr("class", "basic label-container outer-path").
			Attr("style", ru.CSSStylesAttr(node)).
			Attr("style", nodeStyles)
	}

	// Find the label and move it down.
	cylinder.Attr("label-offset-y", ry)
	cylinder.Attr("transform", "translate("+js.Str(-w/2)+", "+js.Str(-(float64(h/2)+ry))+")")

	ru.UpdateNodeBounds(node, cylinder)

	label.Attr(
		"transform",
		"translate("+js.Str(-float64(bbox.Width/2)-(bbox.X-bbox.LeftOr0()))+", "+
			js.Str(-float64(bbox.Height/2)+ry-(bbox.Y-bbox.TopOr0()))+")",
	)

	node.Intersect = func(point ru.Point) ru.Point {
		pos := ru.IntersectRect(ru.NodeBounds(node), point)
		x := pos.X - ru.NN(node.X, 0)

		if rx != 0 &&
			(math.Abs(x) < ru.NN(node.Width, 0)/2 ||
				(math.Abs(x) == ru.NN(node.Width, 0)/2 &&
					math.Abs(pos.Y-ru.NN(node.Y, 0)) > float64(ru.NN(node.Height, 0)/2)-ry)) {
			y := float64(ry * ry * (1 - float64(x*x)/float64(rx*rx)))
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
