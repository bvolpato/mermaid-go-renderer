// Ported from mermaid 12.2.0, rendering-elements/shapes/filledCircle.ts (MIT).

package set2

import (
	"errors"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("filledCircle", filledCircle)
}

func filledCircle(ctx *render.Context, parent *dom.Node, node *ru.Node, renderOptions *ru.ShapeRenderOptions) (*dom.Node, error) {
	// The parameter is destructured: { config: { themeVariables } }.
	if renderOptions == nil || renderOptions.Config == nil {
		return nil, errors.New("Cannot destructure property 'themeVariables' of 'config' as it is undefined.")
	}
	themeVariables := js.AsObj(renderOptions.Config.Get("themeVariables"))
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.Label, node.HasLabel = "", true
	node.LabelStyle = labelStyles
	shapeSvg := insertShapeGroup(parent, node)
	const radius = 7.0

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	if themeVariables == nil {
		return nil, errors.New("Cannot destructure property 'nodeBorder' of 'themeVariables' as it is undefined.")
	}
	nodeBorder := themeVariables.Get("nodeBorder")
	options := ru.UserNodeOverrides(ctx, node, roughOptions("fillStyle", "solid"))

	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
	}

	circleNode, err := rc.Circle(0, 0, radius*2, options)
	if err != nil {
		return nil, err
	}

	filledCircle := shapeSvg.InsertNode(circleNode, ":first-child")

	filledCircle.SelectAll("path").Attr("style", "fill: "+js.ToString(nodeBorder)+" !important;")

	if hasCSSStyles(node) && len(node.CSSStyles) > 0 && node.Look != "handDrawn" {
		filledCircle.SelectAll("path").Attr("style", ru.CSSStylesAttr(node))
	}

	if nodeStyles != "" && node.Look != "handDrawn" {
		filledCircle.SelectAll("path").Attr("style", nodeStyles)
	}

	ru.UpdateNodeBounds(node, filledCircle)

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectCircle(ru.NodeBounds(node), radius, point)
	}

	return shapeSvg, nil
}
