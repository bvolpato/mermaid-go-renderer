// Ported from mermaid 12.2.0, rendering-elements/shapes/person.ts (MIT).

package set1

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("person", person)
}

// person is the person shape: a circular head above a rounded-rectangle
// body, as the C4 model notation draws people and actors.
func person(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label

	padding := ru.NN(node.Padding, 20)
	w := js.Max(bbox.Width+float64(padding*2), ru.NN(node.Width, 0), 100)
	// The proportions are those of the c4model.com person: the head radius
	// is 0.23 times the body width, the head overlaps the body by 0.27 times
	// the head radius, and the body corners are 0.177 times the width. The
	// head is clamped, so that a body that a long label widens keeps a
	// person-sized head.
	headRadius := js.Min(js.Max(float64(w*0.23), 16), 56)
	overlap := float64(headRadius * 0.27)
	heightForBody := 0.0
	if js.Truthy(node.Height) {
		heightForBody = node.Height - (float64(2*headRadius) - overlap)
	}
	bodyHeight := js.Max(bbox.Height+float64(padding*2), heightForBody)
	bodyRadius := js.Min(float64(w*0.177), float64(bodyHeight*0.45))
	totalHeight := bodyHeight + float64(2*headRadius) - overlap
	top := float64(-totalHeight / 2)
	bodyTop := top + float64(2*headRadius) - overlap

	group := shapeSvg.Insert("g", ":first-child").Attr("class", "basic label-container")

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		options := ru.UserNodeOverrides(ctx, node, nil)
		body, err := rc.Path(ru.CreateRoundedRectPathD(-w/2, bodyTop, w, bodyHeight, bodyRadius), options)
		if err != nil {
			return nil, err
		}
		head, err := rc.Circle(0, top+headRadius, headRadius*2, options)
		if err != nil {
			return nil, err
		}

		// Body first, then the head on top, so that the full circle stays
		// visible.
		group.InsertNode(head, ":first-child")
		group.InsertNode(body, ":first-child")
		if hasCSSStyles(node) {
			group.Attr("style", cssStylesString(node))
		}
	} else {
		// Body first, then the head on top, so that the full circle stays
		// visible.
		group.
			Append("rect").
			Attr("x", -w/2).
			Attr("y", bodyTop).
			Attr("width", w).
			Attr("height", bodyHeight).
			Attr("rx", bodyRadius).
			Attr("ry", bodyRadius).
			Attr("style", styles.NodeStyles)

		group.
			Append("circle").
			Attr("cx", 0).
			Attr("cy", top+headRadius).
			Attr("r", headRadius).
			Attr("style", styles.NodeStyles)
	}

	ru.UpdateNodeBounds(node, group)

	bodyCenterY := bodyTop + float64(bodyHeight/2)
	label.Attr("transform", "translate("+js.Str(labelX(bbox))+", "+
		js.Str(bodyCenterY-float64(bbox.Height/2)-(bbox.Y-bbox.TopOr0()))+")")

	// The outline for edge intersection: the exposed arc of the head joined
	// to the rounded-rectangle outline of the body, so that arrows meet the
	// silhouette of the person and not its bounding box.
	headCenterY := top + headRadius
	phiRightDeg := float64(js.Asin(js.Min(1, (bodyTop-headCenterY)/headRadius))*180) / math.Pi
	const headSegments = 24
	outline := ru.GenerateCirclePoints(0, -headCenterY, headRadius, headSegments, 180+phiRightDeg, -phiRightDeg)
	outline = append(outline, ru.GenerateCirclePoints(-(float64(-w/2)+bodyRadius), -(bodyTop+bodyRadius), bodyRadius, 12, 90, 0)...)
	outline = append(outline, ru.GenerateCirclePoints(-(float64(-w/2)+bodyRadius), -(float64(totalHeight/2)-bodyRadius), bodyRadius, 12, 360, 270)...)
	outline = append(outline, ru.GenerateCirclePoints(-(float64(w/2)-bodyRadius), -(float64(totalHeight/2)-bodyRadius), bodyRadius, 12, 270, 180)...)
	outline = append(outline, ru.GenerateCirclePoints(-(float64(w/2)-bodyRadius), -(bodyTop+bodyRadius), bodyRadius, 12, 180, 90)...)

	polygonIntersect(node, outline)

	return shapeSvg, nil
}
