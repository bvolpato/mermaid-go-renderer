// Ported from mermaid 12.2.0, rendering-elements/shapes/cloud.ts (MIT).

package set1

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("cloud", cloud)
}

func cloud(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, halfPadding, label := res.ShapeSvg, res.BBox, res.HalfPadding, res.Label

	w := bbox.Width + float64(2*halfPadding)
	h := bbox.Height + float64(2*halfPadding)

	// The radii of the cloud.
	r1 := float64(0.15 * w)
	r2 := float64(0.25 * w)
	r3 := float64(0.35 * w)
	r4 := float64(0.2 * w)

	var cloudElem *dom.Node

	// The path of the cloud.
	n := js.Str
	arc := func(rx, ry float64, flag string, dx, dy string) string {
		return "    a" + n(rx) + "," + n(ry) + " " + flag + " 0,1 " + dx + "," + dy + "\n"
	}
	path := "M0 0 \n" +
		arc(r1, r1, "0", n(w*0.25), n(-1*w*0.1)) +
		arc(r3, r3, "1", n(w*0.4), n(-1*w*0.1)) +
		arc(r2, r2, "1", n(w*0.35), n(w*0.2)) +
		"\n" +
		arc(r1, r1, "1", n(w*0.15), n(h*0.35)) +
		arc(r4, r4, "1", n(-1*w*0.15), n(h*0.65)) +
		"\n" +
		arc(r2, r1, "1", n(-1*w*0.25), n(w*0.15)) +
		arc(r3, r3, "1", n(-1*w*0.5), "0") +
		arc(r1, r1, "1", n(-1*w*0.25), n(-1*w*0.15)) +
		"\n" +
		arc(r1, r1, "1", n(-1*w*0.1), n(-1*h*0.35)) +
		arc(r4, r4, "1", n(w*0.1), n(-1*h*0.65)) +
		"  H0 V0 Z"

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		options := ru.UserNodeOverrides(ctx, node, nil)
		roughNode, err := rc.Path(path, options)
		if err != nil {
			return nil, err
		}
		cloudElem = shapeSvg.InsertNode(roughNode, ":first-child")
		cloudElem.Attr("class", "basic label-container").Attr("style", ru.CSSStylesAttr(node))
	} else {
		cloudElem = shapeSvg.
			Insert("path", ":first-child").
			Attr("class", "basic label-container").
			Attr("style", styles.NodeStyles).
			Attr("d", path)
	}

	label.Attr("transform", "translate("+js.Str(-bbox.Width/2)+", "+js.Str(-bbox.Height/2)+")")

	// Center the shape.
	cloudElem.Attr("transform", "translate("+js.Str(-w/2)+", "+js.Str(-h/2)+")")

	ru.UpdateNodeBounds(node, cloudElem)

	node.CalcIntersect = func(bounds ru.Bounds, point ru.Point) ru.Point {
		return ru.IntersectRect(bounds, point)
	}
	rectIntersect(node)

	return shapeSvg, nil
}
