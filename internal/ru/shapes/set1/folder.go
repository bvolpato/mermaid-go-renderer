// Ported from mermaid 12.2.0, rendering-elements/shapes/folder.ts (MIT).

package set1

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("folder", folder)
}

// folder is the folder shape: a rectangle with a raised tab on its top-left
// edge.
func folder(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label

	padding := ru.NN(node.Padding, 12)
	w := js.Max(bbox.Width+float64(padding*2), ru.NN(node.Width, 0), 90)
	contentHeight := bbox.Height + float64(padding*2)
	tabHeight := js.Max(js.Min(float64(contentHeight*0.16), 14), 8)
	totalHeight := js.Max(contentHeight+tabHeight, ru.NN(node.Height, 0))
	bodyHeight := totalHeight - tabHeight
	tabWidth := js.Max(float64(w*0.38), 28)
	top := float64(-totalHeight / 2)

	points := []ru.Point{
		{X: -w / 2, Y: top},
		{X: float64(-w/2) + tabWidth, Y: top},
		{X: float64(-w/2) + tabWidth, Y: top + tabHeight},
		{X: w / 2, Y: top + tabHeight},
		{X: w / 2, Y: totalHeight / 2},
		{X: -w / 2, Y: totalHeight / 2},
	}

	parts := []string{"M" + js.Str(points[0].X) + "," + js.Str(points[0].Y)}
	for _, p := range points[1:] {
		parts = append(parts, "L"+js.Str(p.X)+","+js.Str(p.Y))
	}
	parts = append(parts, "Z")
	pathData := strings.Join(parts, " ")

	var folderShape *dom.Node

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		roughNode, err := rc.Path(pathData, ru.UserNodeOverrides(ctx, node, nil))
		if err != nil {
			return nil, err
		}
		folderShape = shapeSvg.
			InsertNode(roughNode, ":first-child").
			Attr("class", "basic label-container")
		if hasCSSStyles(node) {
			folderShape.Attr("style", cssStylesString(node))
		}
	} else {
		folderShape = shapeSvg.
			Insert("path", ":first-child").
			Attr("d", pathData).
			Attr("class", "basic label-container").
			Attr("style", styles.NodeStyles)
	}

	if node.Look == "handDrawn" {
		// The rough path can overflow its nominal box, so measure the DOM.
		ru.UpdateNodeBounds(node, folderShape)
	} else {
		ru.UpdateNodeBounds(node, folderShape, ru.Bounds{Width: w, Height: totalHeight})
	}

	bodyCenterY := top + tabHeight + float64(bodyHeight/2)
	label.Attr("transform", "translate("+js.Str(labelX(bbox))+", "+
		js.Str(bodyCenterY-float64(bbox.Height/2)-(bbox.Y-bbox.TopOr0()))+")")

	polygonIntersect(node, points)

	return shapeSvg, nil
}
