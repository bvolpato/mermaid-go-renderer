// Ported from mermaid 12.2.0, rendering-elements/shapes/stadium.ts (MIT).

package set1

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("stadium", stadium)
}

const arcPointCount = 50

func stadiumDimensions(bbox ru.LabelBBox, paddingX, paddingY float64) (w, h float64) {
	h = bbox.Height + paddingY
	// The sampled arcs sit just inside the circles. Use their inscribed
	// circle for label clearance, and account for the missing horizontal
	// extrema when enforcing the outer aspect ratio.
	inscribedDiameter := float64(h * js.Cos(math.Pi/(2*(arcPointCount-1))))
	capWidthAtLabel := math.Sqrt(js.Max(0, js.Pow(inscribedDiameter, 2)-js.Pow(bbox.Height, 2)))
	w = js.Max(
		bbox.Width+float64(h/4)+paddingX,
		float64(1.5*h)+(h-inscribedDiameter),
		bbox.Width+paddingX+h-capWidthAtLabel,
	)
	return w, h
}

func stadium(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles
	nodePadding := ru.NN(node.Padding, 0)
	paddingX, paddingY := nodePadding, nodePadding
	if node.Look == "neo" {
		paddingX, paddingY = 40, 24
	}
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.ShapeSvg, res.BBox
	w, h := stadiumDimensions(bbox, paddingX, paddingY)

	radius := float64(h / 2)
	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := ru.UserNodeOverrides(ctx, node, nil)

	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
		options.Set("fillStyle", "solid")
	}

	points := []ru.Point{
		{X: float64(-w/2) + radius, Y: -h / 2},
		{X: float64(w/2) - radius, Y: -h / 2},
	}
	points = append(points, ru.GenerateCirclePoints(float64(-w/2)+radius, 0, radius, arcPointCount, 90, 270)...)
	points = append(points, ru.Point{X: float64(w/2) - radius, Y: h / 2})
	points = append(points, ru.GenerateCirclePoints(float64(w/2)-radius, 0, radius, arcPointCount, 270, 450)...)

	pathData := ru.CreatePathFromPoints(points)
	shapeNode, err := rc.Path(pathData, options)
	if err != nil {
		return nil, err
	}

	polygon := shapeSvg.InsertNode(shapeNode, ":first-child")
	polygon.Attr("class", "basic label-container outer-path")

	if hasCSSStyles(node) && node.Look != "handDrawn" {
		selectChildren(polygon, "path").Attr("style", cssStylesString(node))
	}

	if styles.NodeStyles != "" && node.Look != "handDrawn" {
		selectChildren(polygon, "path").Attr("style", styles.NodeStyles)
	}

	ru.UpdateNodeBounds(node, polygon)

	polygonIntersect(node, points)

	return shapeSvg, nil
}
