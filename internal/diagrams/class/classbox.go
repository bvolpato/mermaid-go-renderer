// Ported from mermaid 12.2.0,
// rendering-util/rendering-elements/shapes/classBox.ts (MIT).

package class

import (
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

var (
	// translateRegex is /translate\(([^,]+),([^)]+)\)/.
	translateRegex = regexp.MustCompile(`translate\(([^,]+),([^)]+)\)`)
	// colorRegex is /color\s*:\s*([^;]*)/.
	colorRegex = regexp.MustCompile(`color[` + regexpSpace + `]*:[` + regexpSpace + `]*([^;]*)`)
)

// regexpSpace is the content of the class \s of a JavaScript regular
// expression, for the regexp package.
var regexpSpace = `\t\n\v\f\r \x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}`

// classBox is classBox(parent, node).
func classBox(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	config := ctx.Config
	theme := config.Get("theme")
	themeVariables := js.AsObj(config.Get("themeVariables"))
	var useGradient, borderColorArray any
	if themeVariables != nil {
		useGradient = themeVariables.Get("useGradient")
		borderColorArray = themeVariables.Get("borderColorArray")
	}
	PADDING := classPadding(config, 12)
	GAP := PADDING
	// node.useHtmlLabels ?? evaluate(config.htmlLabels) ?? true
	useHtmlLabels := sanitize.Evaluate(config.Get("htmlLabels"))
	if node.UseHtmlLabelsSet {
		useHtmlLabels = node.UseHtmlLabels
	}
	// Treat node as classNode
	annotations, _ := nodeStrings(node, "annotations")
	members, _ := nodeMembers(node, "members")
	methods, _ := nodeMembers(node, "methods")
	classStyles, hasClassStyles := nodeStrings(node, "styles")

	res, err := textHelper(ctx, parent, node, annotations, members, methods, useHtmlLabels, GAP)
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.shapeSvg, res.bbox

	ru.StampColorSlot(shapeSvg, node.ColorIndex, theme, borderColorArray)

	nodeStyleStrings := ru.Styles2String(node)
	labelStyles, nodeStyles := nodeStyleStrings.LabelStyles, nodeStyleStrings.NodeStyles
	node.LabelStyle = labelStyles

	// node.cssStyles = classNode.styles || '';
	// const styles = classNode.styles?.join(';') || nodeStyles || '';
	styles := ""
	if hasClassStyles {
		node.CSSStyles, node.CSSStylesSet = classStyles, true
		styles = strings.Join(classStyles, ";")
	}
	if styles == "" {
		styles = nodeStyles
	}

	if !hasClassStyles {
		node.CSSStyles, node.CSSStylesSet = strings.Split(strings.ReplaceAll(styles, "!important", ""), ";"), true
	}

	renderExtraBox := len(members) == 0 &&
		len(methods) == 0 &&
		!js.Truthy(js.Path(config, "class", "hideEmptyMembersBox"))

	// Setup roughjs
	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := ru.UserNodeOverrides(ctx, node, js.NewObj[any]())

	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
		options.Set("fillStyle", "solid")
	}

	w := js.Max(ru.NN(node.Width, 0), bbox.Width)
	h := js.Max(ru.NN(node.Height, 0), bbox.Height)
	nodeHeightGreater := ru.NN(node.Height, 0) > bbox.Height
	if len(members) == 0 && len(methods) == 0 {
		h += GAP
	} else if len(members) > 0 && len(methods) == 0 {
		h += float64(GAP * 2)
	}

	x := float64(-w / 2)
	y := float64(-h / 2)
	extraHeight := 0.0
	if renderExtraBox {
		extraHeight = float64(PADDING * 2)
	} else if len(members) == 0 && len(methods) == 0 {
		extraHeight = -PADDING
	}
	if nodeHeightGreater {
		extraHeight = float64(PADDING * 2)
	}

	// boxOffset is the expression that upstream repeats:
	// renderExtraBox ? PADDING : members.length === 0 && methods.length === 0 ? -PADDING / 2 : 0
	boxOffset := 0.0
	if renderExtraBox {
		boxOffset = PADDING
	} else if len(members) == 0 && len(methods) == 0 {
		boxOffset = float64(-PADDING / 2)
	}

	// Create and center rectangle
	roughRect, err := rc.Rectangle(
		x-PADDING,
		y-PADDING-boxOffset,
		w+float64(2*PADDING),
		h+float64(2*PADDING)+extraHeight,
		options,
	)
	if err != nil {
		return nil, err
	}

	rect := shapeSvg.InsertNode(roughRect, ":first-child")
	rect.Attr("class", "basic label-container outer-path")
	rectBBox := rect.BBox()

	// Rect is centered so now adjust labels.
	halfPadding := 0.0
	if renderExtraBox {
		halfPadding = float64(PADDING / 2)
	}
	annotationGroupHeight := ru.Or(shapeSvg.Select(".annotation-group").BBox().Height-halfPadding, 0)
	labelGroupHeight := ru.Or(shapeSvg.Select(".label-group").BBox().Height-halfPadding, 0)
	membersGroupHeight := ru.Or(shapeSvg.Select(".members-group").BBox().Height-halfPadding, 0)

	// Y value in the middle of the first line and remaining space.
	methodsAreaPlacement := (annotationGroupHeight +
		labelGroupHeight +
		y +
		PADDING -
		(y -
			PADDING -
			boxOffset)) /
		2
	// Rect is centered so now adjust labels.
	for _, text := range shapeSvg.SelectAll(".text") {
		// Get the current transform attribute
		transform := text.AttrOr("transform", "")
		// Initialize variables for the translation values
		translateY := 0.0
		// Check if the transform attribute exists
		if transform != "" {
			if translate := translateRegex.FindStringSubmatch(transform); translate != nil {
				translateY = js.ParseFloat(translate[2])
			}
		}
		// Add to the y value
		newTranslateY := translateY +
			y +
			PADDING -
			boxOffset
		class := text.AttrOr("class", "")
		if strings.Contains(class, "methods-group") {
			membersGroupHeightForMethods := js.Max(membersGroupHeight, GAP/2)
			if nodeHeightGreater {
				newTranslateY = js.Max(
					methodsAreaPlacement,
					annotationGroupHeight+
						labelGroupHeight+
						membersGroupHeightForMethods+
						y+
						float64(GAP*2)+
						PADDING,
				) +
					float64(GAP*2)
			} else {
				newTranslateY = annotationGroupHeight +
					labelGroupHeight +
					membersGroupHeightForMethods +
					y +
					float64(GAP*4) +
					PADDING
			}
		}
		if len(members) == 0 &&
			len(methods) == 0 &&
			js.Truthy(js.Path(config, "class", "hideEmptyMembersBox")) {
			if len(annotations) > 0 {
				newTranslateY = translateY - GAP
			} else {
				newTranslateY = translateY
			}
		}
		if !useHtmlLabels {
			// Fix so non html labels are better centered.
			// BBox of text seems to be slightly different when calculated so we offset
			newTranslateY -= 4
		}
		newTranslateX := x
		if strings.Contains(class, "label-group") || strings.Contains(class, "annotation-group") {
			newTranslateX = ru.Or(-text.BBox().Width/2, 0)
			for _, t := range shapeSvg.SelectAll("text") {
				if t.ComputedStyle("text-anchor") == "middle" {
					newTranslateX = 0
				}
			}
		}
		// Set the updated transform attribute
		text.Attr("transform", "translate("+js.Str(newTranslateX)+", "+js.Str(newTranslateY)+")")
	}

	dividerClass := "divider"
	if node.Look == "neo" && !js.Truthy(useGradient) {
		dividerClass = "divider neo-line"
	}

	// Render divider lines.
	// Line y-values are offset by 0.001 so gradient stroke can apply.
	// If y-values are the same then the height of the bounding box is zero and it doesn't work.
	// First line (under label)
	if len(members) > 0 || len(methods) > 0 || renderExtraBox {
		firstLineY := annotationGroupHeight + labelGroupHeight + y + PADDING
		roughLine := rc.Line(
			rectBBox.X,
			firstLineY,
			rectBBox.X+rectBBox.Width,
			firstLineY+0.001,
			options,
		)
		line := shapeSvg.InsertNode(roughLine)
		line.
			Attr("class", dividerClass).
			Attr("style", styles)
	}

	// Second line (under members)
	if renderExtraBox || len(members) > 0 || len(methods) > 0 {
		secondLineY := annotationGroupHeight + labelGroupHeight + membersGroupHeight + y + float64(GAP*2) + PADDING
		lineY := secondLineY
		if nodeHeightGreater {
			lineY = js.Max(methodsAreaPlacement, secondLineY)
		}
		roughLine := rc.Line(
			rectBBox.X,
			lineY,
			rectBBox.X+rectBBox.Width,
			lineY+0.001,
			options,
		)
		line := shapeSvg.InsertNode(roughLine)
		line.
			Attr("class", dividerClass).
			Attr("style", styles)
	}

	/// Apply styles ///
	if node.Look != "handDrawn" {
		shapeSvg.SelectAll("path").Attr("style", styles)
	}
	// Apply other styles like stroke-width and stroke-dasharray to border (not background of shape)
	rect.Select(":nth-child(2)").Attr("style", styles)
	// Divider lines
	for _, divider := range shapeSvg.SelectAll(".divider") {
		divider.Select("path").Attr("style", styles)
	}
	// Text elements
	if node.LabelStyle != "" {
		shapeSvg.SelectAll("span").Attr("style", node.LabelStyle)
	} else {
		shapeSvg.SelectAll("span").Attr("style", styles)
	}
	// SVG text uses fill not color
	if !useHtmlLabels {
		// We just want to apply color to the text
		if match := colorRegex.FindString(styles); match != "" {
			colorStyle := strings.Replace(match, "color", "fill", 1)
			shapeSvg.SelectAll("tspan").Attr("style", colorStyle)
		} else if labelStyles != "" {
			if match := colorRegex.FindString(labelStyles); match != "" {
				colorStyle := strings.Replace(match, "color", "fill", 1)
				shapeSvg.SelectAll("tspan").Attr("style", colorStyle)
			}
		}
	}

	ru.UpdateNodeBounds(node, rect)
	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectRect(ru.NodeBounds(node), point)
	}

	return shapeSvg, nil
}
