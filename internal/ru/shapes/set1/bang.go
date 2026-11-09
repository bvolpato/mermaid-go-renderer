// Ported from mermaid 12.2.0, rendering-elements/shapes/bang.ts (MIT).

package set1

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("bang", bang)
}

func bang(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, halfPadding, label := res.ShapeSvg, res.BBox, res.HalfPadding, res.Label

	w := bbox.Width + float64(10*halfPadding)
	h := bbox.Height + float64(8*halfPadding)
	r := float64(0.15 * w)

	minWidth := bbox.Width + 20
	minHeight := bbox.Height + 20
	effectiveWidth := js.Max(w, minWidth)
	effectiveHeight := js.Max(h, minHeight)

	label.Attr("transform", "translate("+js.Str(-bbox.Width/2)+", "+js.Str(-bbox.Height/2)+")")

	n := js.Str
	arc := "a" + n(r) + "," + n(r) + " 1 0,0 "
	smallArc := "a" + n(r*0.8) + "," + n(r*0.8) + " 1 0,0 0,"
	path := "M0 0 \n" +
		"    " + arc + n(effectiveWidth*0.25) + "," + n(-1*effectiveHeight*0.1) + "\n" +
		"    " + arc + n(effectiveWidth*0.25) + "," + n(0) + "\n" +
		"    " + arc + n(effectiveWidth*0.25) + "," + n(0) + "\n" +
		"    " + arc + n(effectiveWidth*0.25) + "," + n(effectiveHeight*0.1) + "\n" +
		"\n" +
		"    " + arc + n(effectiveWidth*0.15) + "," + n(effectiveHeight*0.33) + "\n" +
		"    " + smallArc + n(effectiveHeight*0.34) + "\n" +
		"    " + arc + n(-1*effectiveWidth*0.15) + "," + n(effectiveHeight*0.33) + "\n" +
		"\n" +
		"    " + arc + n(-1*effectiveWidth*0.25) + "," + n(effectiveHeight*0.15) + "\n" +
		"    " + arc + n(-1*effectiveWidth*0.25) + ",0\n" +
		"    " + arc + n(-1*effectiveWidth*0.25) + ",0\n" +
		"    " + arc + n(-1*effectiveWidth*0.25) + "," + n(-1*effectiveHeight*0.15) + "\n" +
		"\n" +
		"    " + arc + n(-1*effectiveWidth*0.1) + "," + n(-1*effectiveHeight*0.33) + "\n" +
		"    " + smallArc + n(-1*effectiveHeight*0.34) + "\n" +
		"    " + arc + n(effectiveWidth*0.1) + "," + n(-1*effectiveHeight*0.33) + "\n" +
		"  H0 V0 Z"

	var bangElem *dom.Node
	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		options := ru.UserNodeOverrides(ctx, node, nil)
		roughNode, err := rc.Path(path, options)
		if err != nil {
			return nil, err
		}
		bangElem = shapeSvg.InsertNode(roughNode, ":first-child")
		bangElem.Attr("class", "basic label-container").Attr("style", ru.CSSStylesAttr(node))
	} else {
		bangElem = shapeSvg.
			Insert("path", ":first-child").
			Attr("class", "basic label-container").
			Attr("style", styles.NodeStyles).
			Attr("d", path)
	}

	// Translate the path, to center the shape.
	bangElem.Attr("transform", "translate("+js.Str(-effectiveWidth/2)+", "+js.Str(-effectiveHeight/2)+")")

	ru.UpdateNodeBounds(node, bangElem)
	node.CalcIntersect = func(bounds ru.Bounds, point ru.Point) ru.Point {
		return ru.IntersectRect(bounds, point)
	}
	rectIntersect(node)

	return shapeSvg, nil
}
