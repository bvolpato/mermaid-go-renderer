// Ported from mermaid 12.2.0,
// rendering-util/rendering-elements/shapes/requirementBox.ts (MIT).

package requirement

import (
	"math"
	"regexp"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/rough"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

var reTranslate = regexp.MustCompile(`translate\(([^,]+),([^)]+)\)`)

// The values of TextAlignment.
const (
	alignCenter = "center"
	alignStart  = "start"
)

// property is node[key] for a property of a requirement or of an element.
// The db keeps these properties in Extra. The model of a layout node has a
// numeric field for `type`, which a node from another source can hold.
func property(node *ru.Node, key string) any {
	if v, ok := node.Extra[key]; ok {
		return v
	}
	if key == "type" && !ru.IsUndef(node.Type) {
		return node.Type
	}
	return nil
}

// lengthOf is `v?.length`. The second result is false when the value is
// undefined: v is nullish, or it is not a string or an array.
func lengthOf(v any) (float64, bool) {
	switch x := v.(type) {
	case []any:
		return float64(len(x)), true
	case string:
		return float64(js.UTF16Len(x)), true
	}
	return 0, false
}

// bodyLine is the text of a line of the body: the prefix and the value, or
// the empty string when the value is falsy.
func bodyLine(prefix string, value any) string {
	if !js.Truthy(value) {
		return ""
	}
	return prefix + js.ToString(value)
}

// requirementBox is requirementBox(parent, node): the shape of a
// requirement or of an element. It draws a box with the type and the name,
// and below a divider the lines of the body.
func requirementBox(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	const padding = 20.0
	const gap = 20.0
	_, isRequirementNode := node.Extra["verifyMethod"]
	classes := ru.GetNodeClasses(node)
	config := ctx.Config
	themeVariablesValue := config.Get("themeVariables")
	if js.IsNullish(themeVariablesValue) {
		return nil, typeError("Cannot destructure property 'borderColorArray' of 'themeVariables' as it is " + js.ToString(themeVariablesValue) + ".")
	}
	var borderColorArray, requirementEdgeLabelBackground any
	if themeVariables := js.AsObj(themeVariablesValue); themeVariables != nil {
		borderColorArray = themeVariables.Get("borderColorArray")
		requirementEdgeLabelBackground = themeVariables.Get("requirementEdgeLabelBackground")
	}
	borderColorCount, hasBorderColorCount := lengthOf(borderColorArray)
	// borderColorArray?.length as a condition.
	hasBorderColors := hasBorderColorCount && borderColorCount != 0
	bodyTextAlignment := alignCenter
	if js.StrictEquals(config.Get("layout"), "elk") {
		bodyTextAlignment = alignStart
	}

	// Add outer g element
	domID := node.DomID
	if domID == "" {
		domID = node.ID
	}
	shapeSvg := parent.
		Insert("g").
		Attr("class", classes).
		Attr("id", domID)

	var typeHeight float64
	var err error
	if isRequirementNode {
		typeHeight, err = addText(
			ctx,
			shapeSvg,
			"&lt;&lt;"+js.ToString(property(node, "type"))+"&gt;&gt;",
			0,
			node.LabelStyle,
			alignCenter,
		)
	} else {
		typeHeight, err = addText(ctx, shapeSvg, "&lt;&lt;Element&gt;&gt;", 0, node.LabelStyle, alignCenter)
	}
	if err != nil {
		return nil, err
	}

	accumulativeHeight := typeHeight
	name, ok := property(node, "name").(string)
	if !ok {
		// decodeEntities(inputText) calls inputText.replace.
		if v := property(node, "name"); js.IsNullish(v) {
			return nil, typeError("Cannot read properties of " + js.ToString(v) + " (reading 'replace')")
		}
		return nil, typeError("text.replace is not a function")
	}
	nameHeight, err := addText(
		ctx,
		shapeSvg,
		name,
		accumulativeHeight,
		node.LabelStyle+"; font-weight: bold;",
		alignCenter,
	)
	if err != nil {
		return nil, err
	}
	accumulativeHeight += nameHeight + gap

	// Requirement
	if isRequirementNode {
		idHeight, err := addText(
			ctx,
			shapeSvg,
			bodyLine("ID: ", property(node, "requirementId")),
			accumulativeHeight,
			node.LabelStyle,
			bodyTextAlignment,
		)
		if err != nil {
			return nil, err
		}

		accumulativeHeight += idHeight
		textHeight, err := addText(
			ctx,
			shapeSvg,
			bodyLine("Text: ", property(node, "text")),
			accumulativeHeight,
			node.LabelStyle,
			bodyTextAlignment,
		)
		if err != nil {
			return nil, err
		}
		accumulativeHeight += textHeight
		riskHeight, err := addText(
			ctx,
			shapeSvg,
			bodyLine("Risk: ", property(node, "risk")),
			accumulativeHeight,
			node.LabelStyle,
			bodyTextAlignment,
		)
		if err != nil {
			return nil, err
		}
		accumulativeHeight += riskHeight
		if _, err := addText(
			ctx,
			shapeSvg,
			bodyLine("Verification: ", property(node, "verifyMethod")),
			accumulativeHeight,
			node.LabelStyle,
			bodyTextAlignment,
		); err != nil {
			return nil, err
		}
	} else {
		// Element
		typeHeight, err := addText(
			ctx,
			shapeSvg,
			bodyLine("Type: ", property(node, "type")),
			accumulativeHeight,
			node.LabelStyle,
			bodyTextAlignment,
		)
		if err != nil {
			return nil, err
		}
		accumulativeHeight += typeHeight
		if _, err := addText(
			ctx,
			shapeSvg,
			bodyLine("Doc Ref: ", property(node, "docRef")),
			accumulativeHeight,
			node.LabelStyle,
			bodyTextAlignment,
		); err != nil {
			return nil, err
		}
	}

	totalWidth := shapeSvg.BBox().Width + padding
	totalHeight := shapeSvg.BBox().Height + padding
	x := float64(-totalWidth / 2)
	y := float64(-totalHeight / 2)

	// Setup roughjs
	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := ru.UserNodeOverrides(ctx, node, js.NewObj[any]())

	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
		options.Set("fillStyle", "solid")
	}

	// Create and center rectangle
	roughRect, err := rc.Rectangle(x, y, totalWidth, totalHeight, options)
	if err != nil {
		return nil, err
	}

	rect := shapeSvg.InsertNode(roughRect, ":first-child")
	rect.Attr("class", "basic label-container outer-path").Attr("style", nodeStyles)

	if hasBorderColors {
		colorIndex := ru.NN(node.ColorIndex, 0)
		shapeSvg.Attr("data-color-id", "color-"+js.Str(math.Mod(colorIndex, borderColorCount)))
	}

	// Re-translate labels now that rect is centered
	for i, text := range shapeSvg.SelectAll(".label") {
		transform, _ := text.GetAttr("transform")
		translateX := 0.0
		translateY := 0.0
		if transform != "" {
			if translate := reTranslate.FindStringSubmatch(transform); translate != nil {
				translateX = js.ParseFloat(translate[1])
				translateY = js.ParseFloat(translate[2])
			}
		}

		newTranslateY := translateY - float64(totalHeight/2)
		newTranslateX := x + padding/2

		// Keep type and name labels centered.
		if i == 0 || i == 1 {
			newTranslateX = translateX
		}
		// Set the updated transform attribute
		text.Attr("transform", "translate("+js.Str(newTranslateX)+", "+js.Str(newTranslateY+padding)+")")
	}

	// Insert divider line if there is body text
	if accumulativeHeight > typeHeight+nameHeight+gap {
		lineY := y + typeHeight + nameHeight + gap
		var roughLine *dom.Node
		if node.Look == "neo" {
			const thickness = 0.001
			polygonPoints := []rough.Point{
				{x, lineY},
				{x + totalWidth, lineY},
				{x + totalWidth, lineY + thickness},
				{x, lineY + thickness},
			}
			if roughLine, err = rc.Polygon(polygonPoints, options); err != nil {
				return nil, err
			}
		} else {
			roughLine = rc.Line(x, lineY, x+totalWidth, lineY, options)
		}
		dividerLine := shapeSvg.InsertNode(roughLine)
		dividerLine.Attr("class", "divider")
	}

	ru.UpdateNodeBounds(node, rect)

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectRect(ru.NodeBounds(node), point)
	}

	if nodeStyles != "" &&
		node.Look != "handDrawn" &&
		(js.Truthy(requirementEdgeLabelBackground) || hasBorderColors) {
		for _, path := range shapeSvg.SelectAll("path") {
			path.Attr("style", nodeStyles)
		}
	}

	return shapeSvg, nil
}

// addText is the helper function addText of requirementBox.ts: it adds a
// label group for one line of the box and returns the height of the label.
// An empty text adds nothing and has the height 0.
//
// addText is async upstream, and a layout draws its nodes together with
// Promise.all. A box with more lines awaits more often, so it draws its
// rectangle, and takes its numbers of Math.random, after a box with fewer
// lines. The calls of ctx.Await keep that order (render/async.go).
func addText(ctx *render.Context, parentGroup *dom.Node, inputText string, yOffset float64, style string, alignment string) (float64, error) {
	if inputText == "" {
		// The await of the caller.
		ctx.Await()
		return 0, nil
	}
	textEl := parentGroup.Insert("g").Attr("class", "label").Attr("style", style)
	config := ctx.Config
	// config.htmlLabels ?? true
	htmlLabels := config.Get("htmlLabels")
	useHtmlLabels := js.IsNullish(htmlLabels) || js.Truthy(htmlLabels)

	sanitized := ctx.SanitizeText(render.DecodeEntities(inputText))
	textWidth, err := utils.CalculateTextWidth(ctx, inputText, utils.TextDimensionConfigOf(config))
	if err != nil {
		return 0, err
	}
	options := ru.DefaultTextOptions()
	options.Width = textWidth + 50 // Add room for error when splitting text into multiple lines
	options.Classes = "markdown-node-label"
	options.UseHtmlLabels = useHtmlLabels
	options.Style = style
	text, err := ru.CreateText(ctx, textEl, sanitized, options)
	if err != nil {
		return 0, err
	}
	var bbox dom.Rect

	if !useHtmlLabels {
		textChild := text.FirstElementChild()
		if textChild == nil {
			return 0, typeError("Cannot read properties of undefined (reading 'children')")
		}
		for _, child := range textChild.Children() {
			if style != "" {
				child.Attr("style", style)
			}
		}
		if alignment == alignStart {
			textChild.Attr("text-anchor", "start")
			for _, child := range textChild.Children() {
				child.Attr("text-anchor", "start")
			}
		}
		// Get the bounding box after the style update
		bbox = text.BBox()
		// Add extra height so it is similar to the html labels. The box is
		// an SVGRect, whose attributes are float32: the assignment rounds.
		bbox.Height = float64(float32(bbox.Height + 6))
	} else {
		div := text.FirstElementChild()

		if alignment == alignStart {
			div.Style("text-align", "left")
		}
		if div == nil {
			return 0, typeError("Cannot read properties of undefined (reading 'getBoundingClientRect')")
		}
		bbox = div.BoundingClientRect()
		text.Attr("width", bbox.Width)
		text.Attr("height", bbox.Height)
	}

	// Center text and offset by yOffset
	textEl.Attr("transform", "translate("+js.Str(-bbox.Width/2)+","+js.Str(float64(-bbox.Height/2)+yOffset)+")")
	// The await of the caller. See render/async.go for the convention.
	ctx.Await()
	return bbox.Height, nil
}
