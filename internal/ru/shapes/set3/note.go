// Ported from mermaid 12.2.0, rendering-elements/shapes/note.ts (MIT).

package set3

import (
	"errors"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

func init() {
	ru.RegisterShape("note", note)
}

func note(ctx *render.Context, parent *dom.Node, node *ru.Node, renderOptions *ru.ShapeRenderOptions) (*dom.Node, error) {
	// The parameter is destructured: { config: { themeVariables } }.
	if renderOptions == nil || renderOptions.Config == nil {
		return nil, errors.New("Cannot destructure property 'themeVariables' of 'config' as it is undefined.")
	}
	themeVariables := js.AsObj(renderOptions.Config.Get("themeVariables"))
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	useHtmlLabels := node.UseHtmlLabels || sanitize.GetEffectiveHtmlLabels(ctx.Config)
	if !useHtmlLabels {
		node.CenterLabel = true
	}
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label
	totalWidth := js.Max(bbox.Width+float64(ru.NN(node.Padding, 0)*2), ru.NN(node.Width, 0))
	totalHeight := js.Max(bbox.Height+float64(ru.NN(node.Padding, 0)*2), ru.NN(node.Height, 0))
	x := -totalWidth / 2
	y := -totalHeight / 2

	// Add the rect.
	rc := ru.NewRoughSVG(ctx, shapeSvg)
	if themeVariables == nil {
		return nil, errors.New("Cannot read properties of undefined (reading 'noteBkgColor')")
	}
	options := ru.UserNodeOverrides(ctx, node, roughOptions(
		"fill", themeVariables.Get("noteBkgColor"),
		"stroke", themeVariables.Get("noteBorderColor"),
	))

	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
		options.Set("fillStyle", "solid")
	}

	noteShapeNode, err := rc.Rectangle(x, y, totalWidth, totalHeight, options)
	if err != nil {
		return nil, err
	}

	rect := shapeSvg.InsertNode(noteShapeNode, ":first-child")
	rect.Attr("class", "basic label-container outer-path")
	label.Attr("class", "label noteLabel")

	applyPathStyles(rect, node, nodeStyles)

	label.Attr(
		"transform",
		"translate("+js.Str(labelOffsetX(bbox))+", "+js.Str(-float64(bbox.Height/2)-(bbox.Y-bbox.TopOr0()))+")",
	)

	ru.UpdateNodeBounds(node, rect)

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectRect(ru.NodeBounds(node), point)
	}

	return shapeSvg, nil
}
