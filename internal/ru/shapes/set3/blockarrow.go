// Ported from mermaid 12.2.0, rendering-elements/shapes/blockArrow.ts (MIT).

package set3

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("block_arrow", blockArrow)
}

// arrowMetrics is ArrowMetrics.
type arrowMetrics struct {
	height   float64
	midpoint float64
	padding  float64
	width    float64
}

var directionOrder = []string{"right", "left", "up", "down"}

const pointKey = "point"

// expandAndDeduplicateDirections is expandAndDeduplicateDirections: the
// set of the directions, where x stands for right and left, and y for up
// and down.
func expandAndDeduplicateDirections(directions []string) *js.Set[string] {
	uniqueDirections := js.NewSet[string]()

	for _, direction := range directions {
		switch direction {
		case "x":
			uniqueDirections.Add("right")
			uniqueDirections.Add("left")
		case "y":
			uniqueDirections.Add("up")
			uniqueDirections.Add("down")
		default:
			uniqueDirections.Add(direction)
		}
	}

	return uniqueDirections
}

// getDirectionKey is getDirectionKey(directions).
func getDirectionKey(directions *js.Set[string]) string {
	var parts []string
	for _, direction := range directionOrder {
		if directions.Has(direction) {
			parts = append(parts, direction)
		}
	}
	if key := strings.Join(parts, "|"); key != "" {
		return key
	}
	return pointKey
}

// arrowPointFactories is arrowPointFactories: the points of the arrow for
// each set of directions.
var arrowPointFactories = map[string]func(m arrowMetrics) []ru.Point{
	"right|left|up|down": func(m arrowMetrics) []ru.Point {
		height, midpoint, padding, width := m.height, m.midpoint, m.padding, m.width
		return []ru.Point{
			{X: 0, Y: 0},
			{X: midpoint, Y: 0},
			{X: width / 2, Y: 2 * padding},
			{X: width - midpoint, Y: 0},
			{X: width, Y: 0},
			{X: width, Y: -height / 3},
			{X: width + float64(2*padding), Y: -height / 2},
			{X: width, Y: (-2 * height) / 3},
			{X: width, Y: -height},
			{X: width - midpoint, Y: -height},
			{X: width / 2, Y: -height - float64(2*padding)},
			{X: midpoint, Y: -height},
			{X: 0, Y: -height},
			{X: 0, Y: (-2 * height) / 3},
			{X: -2 * padding, Y: -height / 2},
			{X: 0, Y: -height / 3},
		}
	},
	"right|left|up": func(m arrowMetrics) []ru.Point {
		height, midpoint, width := m.height, m.midpoint, m.width
		return []ru.Point{
			{X: midpoint, Y: 0},
			{X: width - midpoint, Y: 0},
			{X: width, Y: -height / 2},
			{X: width - midpoint, Y: -height},
			{X: midpoint, Y: -height},
			{X: 0, Y: -height / 2},
		}
	},
	"right|left|down": func(m arrowMetrics) []ru.Point {
		height, midpoint, width := m.height, m.midpoint, m.width
		return []ru.Point{
			{X: 0, Y: 0},
			{X: midpoint, Y: -height},
			{X: width - midpoint, Y: -height},
			{X: width, Y: 0},
		}
	},
	"right|up|down": func(m arrowMetrics) []ru.Point {
		height, midpoint, width := m.height, m.midpoint, m.width
		return []ru.Point{
			{X: 0, Y: 0},
			{X: width, Y: -midpoint},
			{X: width, Y: -height + midpoint},
			{X: 0, Y: -height},
		}
	},
	"left|up|down": func(m arrowMetrics) []ru.Point {
		height, midpoint, width := m.height, m.midpoint, m.width
		return []ru.Point{
			{X: width, Y: 0},
			{X: 0, Y: -midpoint},
			{X: 0, Y: -height + midpoint},
			{X: width, Y: -height},
		}
	},
	"right|left": func(m arrowMetrics) []ru.Point {
		height, midpoint, padding, width := m.height, m.midpoint, m.padding, m.width
		return []ru.Point{
			{X: midpoint, Y: 0},
			{X: midpoint, Y: -padding},
			{X: width - midpoint, Y: -padding},
			{X: width - midpoint, Y: 0},
			{X: width, Y: -height / 2},
			{X: width - midpoint, Y: -height},
			{X: width - midpoint, Y: -height + padding},
			{X: midpoint, Y: -height + padding},
			{X: midpoint, Y: -height},
			{X: 0, Y: -height / 2},
		}
	},
	"up|down": func(m arrowMetrics) []ru.Point {
		height, midpoint, padding, width := m.height, m.midpoint, m.padding, m.width
		return []ru.Point{
			{X: width / 2, Y: 0},
			{X: 0, Y: -padding},
			{X: midpoint, Y: -padding},
			{X: midpoint, Y: -height + padding},
			{X: 0, Y: -height + padding},
			{X: width / 2, Y: -height},
			{X: width, Y: -height + padding},
			{X: width - midpoint, Y: -height + padding},
			{X: width - midpoint, Y: -padding},
			{X: width, Y: -padding},
		}
	},
	"right|up": func(m arrowMetrics) []ru.Point {
		height, midpoint, width := m.height, m.midpoint, m.width
		return []ru.Point{
			{X: 0, Y: 0},
			{X: width, Y: -midpoint},
			{X: 0, Y: -height},
		}
	},
	"right|down": func(m arrowMetrics) []ru.Point {
		height, width := m.height, m.width
		return []ru.Point{
			{X: 0, Y: 0},
			{X: width, Y: 0},
			{X: 0, Y: -height},
		}
	},
	"left|up": func(m arrowMetrics) []ru.Point {
		height, midpoint, width := m.height, m.midpoint, m.width
		return []ru.Point{
			{X: width, Y: 0},
			{X: 0, Y: -midpoint},
			{X: width, Y: -height},
		}
	},
	"left|down": func(m arrowMetrics) []ru.Point {
		height, width := m.height, m.width
		return []ru.Point{
			{X: width, Y: 0},
			{X: 0, Y: 0},
			{X: width, Y: -height},
		}
	},
	"right": func(m arrowMetrics) []ru.Point {
		height, midpoint, padding, width := m.height, m.midpoint, m.padding, m.width
		return []ru.Point{
			{X: midpoint, Y: -padding},
			{X: midpoint, Y: -padding},
			{X: width - midpoint, Y: -padding},
			{X: width - midpoint, Y: 0},
			{X: width, Y: -height / 2},
			{X: width - midpoint, Y: -height},
			{X: width - midpoint, Y: -height + padding},
			{X: midpoint, Y: -height + padding},
			{X: midpoint, Y: -height + padding},
		}
	},
	"left": func(m arrowMetrics) []ru.Point {
		height, midpoint, padding, width := m.height, m.midpoint, m.padding, m.width
		return []ru.Point{
			{X: midpoint, Y: 0},
			{X: midpoint, Y: -padding},
			{X: width - midpoint, Y: -padding},
			{X: width - midpoint, Y: -height + padding},
			{X: midpoint, Y: -height + padding},
			{X: midpoint, Y: -height},
			{X: 0, Y: -height / 2},
		}
	},
	"up": func(m arrowMetrics) []ru.Point {
		height, midpoint, padding, width := m.height, m.midpoint, m.padding, m.width
		return []ru.Point{
			{X: midpoint, Y: -padding},
			{X: midpoint, Y: -height + padding},
			{X: 0, Y: -height + padding},
			{X: width / 2, Y: -height},
			{X: width, Y: -height + padding},
			{X: width - midpoint, Y: -height + padding},
			{X: width - midpoint, Y: -padding},
		}
	},
	"down": func(m arrowMetrics) []ru.Point {
		height, midpoint, padding, width := m.height, m.midpoint, m.padding, m.width
		return []ru.Point{
			{X: width / 2, Y: 0},
			{X: 0, Y: -padding},
			{X: midpoint, Y: -padding},
			{X: midpoint, Y: -height + padding},
			{X: width - midpoint, Y: -height + padding},
			{X: width - midpoint, Y: -padding},
			{X: width, Y: -padding},
		}
	},
	pointKey: func(arrowMetrics) []ru.Point {
		return []ru.Point{{X: 0, Y: 0}}
	},
}

// getArrowPoints is getArrowPoints(duplicatedDirections, bbox, node,
// totalWidth). bboxWidth and bboxHeight are the size of the label, padding
// is node.padding, and totalWidth is undefined (NaN) when the width follows
// from the label.
func getArrowPoints(duplicatedDirections []string, bboxWidth, bboxHeight, nodePadding, totalWidth float64) []ru.Point {
	directions := expandAndDeduplicateDirections(duplicatedDirections)
	padding := float64(ru.NN(nodePadding, 0) / 2)
	height := bboxHeight + float64(4*padding)
	midpoint := float64(height / 2)
	width := totalWidth
	if ru.IsUndef(width) {
		width = bboxWidth + float64(2*midpoint) + float64(2*padding)
	}
	key := getDirectionKey(directions)
	pointFactory, ok := arrowPointFactories[key]
	if !ok {
		pointFactory = arrowPointFactories[pointKey]
	}

	return pointFactory(arrowMetrics{height: height, midpoint: midpoint, padding: padding, width: width})
}

// nodeDirections is `blockNode.directions ?? []` as strings.
func nodeDirections(node *ru.Node) []string {
	arr, _ := node.Get("directions").([]any)
	directions := make([]string, len(arr))
	for i, d := range arr {
		directions[i] = js.ToString(d)
	}
	return directions
}

// The block arrow node of a block diagram has the extra properties
// directions, positioned, style and widthInColumns.
func blockArrow(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	res, err := labelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.ShapeSvg, res.BBox
	nodePadding := ru.NN(node.Padding, 0)
	height := bbox.Height + float64(2*nodePadding)
	midpoint := float64(height / 2)
	naturalWidth := bbox.Width + float64(2*midpoint) + nodePadding
	nodeWidth := ru.NN(node.Width, 0)
	widthInColumns := 1.0
	if v := node.Get("widthInColumns"); !js.IsNullish(v) {
		widthInColumns = js.ToNumber(v)
	}
	isSpanning := js.Truthy(node.Get("positioned")) && widthInColumns > 1 && nodeWidth > naturalWidth
	width := naturalWidth
	if isSpanning {
		width = nodeWidth
	}
	points := getArrowPoints(nodeDirections(node), bbox.Width, bbox.Height, node.Padding, width)
	blockArrow := ru.InsertPolygonShape(shapeSvg, width, height, points)

	blockArrow.Attr("style", attrValue(node.Get("style")))
	ru.UpdateNodeBounds(node, blockArrow)

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectPolygon(ru.NodeBounds(node), points, point)
	}

	return shapeSvg, nil
}
