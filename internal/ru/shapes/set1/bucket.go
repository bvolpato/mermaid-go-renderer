// Ported from mermaid 12.2.0, rendering-elements/shapes/bucket.ts (MIT).

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
	ru.RegisterShape("bucket", bucket)
}

// bucket is the bucket shape: an open-top container that narrows to a
// curved base, with an elliptical rim.
func bucket(ctx *render.Context, parent *dom.Node, node *ru.Node, options *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label

	accent := accentColor(options)
	padding := ru.NN(node.Padding, 12)
	w := js.Max(bbox.Width+float64(padding*2), ru.NN(node.Width, 0), 80)
	rimRy := js.Max(js.Min(float64(w*0.08), 12), 5)
	totalHeight := js.Max(bbox.Height+float64(padding*2)+rimRy, ru.NN(node.Height, 0))
	topY := float64(-totalHeight/2) + rimRy
	bottomY := float64(totalHeight / 2)
	bottomW := float64(w * 0.72)

	n := js.Str
	body := strings.Join([]string{
		"M" + n(-w/2) + "," + n(topY),
		"L" + n(-bottomW/2) + "," + n(bottomY),
		"A" + n(bottomW/2) + "," + n(rimRy) + " 0 0 0 " + n(bottomW/2) + "," + n(bottomY),
		"L" + n(w/2) + "," + n(topY),
		"A" + n(w/2) + "," + n(rimRy) + " 0 0 0 " + n(-w/2) + "," + n(topY),
		"Z",
	}, " ")

	group := shapeSvg.Insert("g", ":first-child").Attr("class", "basic label-container")

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		roughNode, err := rc.Path(body, ru.UserNodeOverrides(ctx, node, nil))
		if err != nil {
			return nil, err
		}
		group.AppendChild(roughNode)
		if hasCSSStyles(node) {
			group.Attr("style", cssStylesString(node))
		}
	} else {
		group.Append("path").Attr("d", body).Attr("style", styles.NodeStyles)
	}

	group.
		Append("ellipse").
		Attr("cx", 0).
		Attr("cy", topY).
		Attr("rx", w/2).
		Attr("ry", rimRy).
		Attr("style", "fill:none;stroke:"+accent+";stroke-width:1px")

	ru.UpdateNodeBounds(node, group)

	bodyCenterY := topY + float64((bottomY-topY)/2)
	label.Attr("transform", "translate("+js.Str(labelX(bbox))+", "+
		js.Str(bodyCenterY-float64(bbox.Height/2)-(bbox.Y-bbox.TopOr0()))+")")

	// The outline for edge intersection: the upper arc of the rim and the
	// lower arc of the base, joined by the tapered sides.
	const arcSegments = 12
	arc := func(rx, cy, ySign float64) []ru.Point {
		out := make([]ru.Point, arcSegments+1)
		for i := range out {
			theta := math.Pi - float64(float64(i)*math.Pi)/arcSegments
			out[i] = ru.Point{X: float64(rx * js.Cos(theta)), Y: cy + float64(ySign*rimRy*js.Sin(theta))}
		}
		return out
	}
	outline := arc(w/2, topY, -1)
	base := arc(bottomW/2, bottomY, 1)
	for i := len(base) - 1; i >= 0; i-- {
		outline = append(outline, base[i])
	}

	polygonIntersect(node, outline)

	return shapeSvg, nil
}
