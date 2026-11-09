// Ported from mermaid 12.2.0, rendering-elements/shapes/forkJoin.ts (MIT).

package set1

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("forkJoin", forkJoin)
}

func forkJoin(ctx *render.Context, parent *dom.Node, node *ru.Node, options *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.Label, node.HasLabel = "", true
	shapeSvg := parent.
		Insert("g").
		Attr("class", ru.GetNodeClasses(node)).
		Attr("id", nodeID(node))

	width := js.Max(70, ru.NN(node.Width, 0))
	height := js.Max(10, ru.NN(node.Height, 0))

	// The bar must run perpendicular to the flow: with a horizontal flow
	// the long side faces the edges, so the bar is drawn vertical.
	dir := ""
	if options != nil {
		dir = options.Dir
	}
	if dir == "LR" || dir == "RL" {
		width = js.Max(10, ru.NN(node.Width, 0))
		height = js.Max(70, ru.NN(node.Height, 0))
	}

	x := float64(-1*width) / 2
	y := float64(-1*height) / 2

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	lineColor := themeVariable(options, "lineColor")
	overrides := js.NewObj[any]()
	overrides.Set("stroke", lineColor)
	overrides.Set("fill", lineColor)
	roughOptions := ru.UserNodeOverrides(ctx, node, overrides)

	if node.Look != "handDrawn" {
		roughOptions.Set("roughness", 0.0)
		roughOptions.Set("fillStyle", "solid")
	}

	roughNode, err := rc.Rectangle(x, y, width, height, roughOptions)
	if err != nil {
		return nil, err
	}

	shape := shapeSvg.InsertNode(roughNode, ":first-child")

	if hasCSSStyles(node) && node.Look != "handDrawn" {
		shape.SelectAll("path").Attr("style", cssStylesString(node))
	}

	if styles.NodeStyles != "" && node.Look != "handDrawn" {
		shape.SelectAll("path").Attr("style", styles.NodeStyles)
	}

	ru.UpdateNodeBounds(node, shape)
	// const padding = state?.padding ?? 0.
	padding := 0.0
	if options != nil {
		if v := js.Path(options.Config, "state", "padding"); !js.IsNullish(v) {
			padding = js.ToNumber(v)
		}
	}
	if js.Truthy(node.Width) && js.Truthy(node.Height) {
		node.Width += ru.Or(padding/2, 0)
		node.Height += ru.Or(padding/2, 0)
	}
	rectIntersect(node)
	return shapeSvg, nil
}
