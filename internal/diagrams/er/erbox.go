// Ported from mermaid 12.2.0,
// rendering-util/rendering-elements/shapes/erBox.ts (MIT).

package er

import (
	"math"
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/rough"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

var (
	colorThemes = map[string]bool{"redux-color": true, "redux-dark-color": true}
	reduxThemes = map[string]bool{"redux": true, "redux-dark": true, "redux-color": true, "redux-dark-color": true}
)

var reTranslate = regexp.MustCompile(`translate\(([^,]+),([^)]+)\)`)

// boxAttribute is one element of entityNode.attributes as the shape reads it.
type boxAttribute struct {
	typ, name, keys, comment string
}

// stringProperty reads a property that the shape uses as a string. The
// second result is false when the property is undefined or null: upstream
// then fails with a TypeError.
func stringProperty(o *js.Obj[any], key string) (string, bool) {
	v := o.Get(key)
	if js.IsNullish(v) {
		return "", false
	}
	return js.ToString(v), true
}

// boxAttributes reads entityNode.attributes: the property `attributes` of
// the node, a list of objects with type, name, keys and comment.
func boxAttributes(node *ru.Node) ([]boxAttribute, error) {
	list, ok := node.Get("attributes").([]any)
	if !ok {
		return nil, typeError("Cannot read properties of undefined (reading 'length')")
	}
	out := make([]boxAttribute, len(list))
	for i, item := range list {
		o := js.AsObj(item)
		if o == nil {
			return nil, typeError("Cannot read properties of undefined (reading 'type')")
		}
		a := &out[i]
		var ok bool
		if a.typ, ok = stringProperty(o, "type"); !ok {
			return nil, typeError("Cannot read properties of undefined (reading 'split')")
		}
		if a.name, ok = stringProperty(o, "name"); !ok {
			return nil, typeError("Cannot read properties of undefined (reading 'split')")
		}
		// attribute.keys.join()
		keys, ok := o.Get("keys").([]any)
		if !ok {
			return nil, typeError("Cannot read properties of undefined (reading 'join')")
		}
		a.keys = js.ToString(keys)
		if a.comment, ok = stringProperty(o, "comment"); !ok {
			return nil, typeError("Cannot read properties of undefined (reading 'split')")
		}
	}
	return out, nil
}

// colorSlot is `color-${colorIndex % borderColorArray.length}`.
func colorSlot(node *ru.Node, borderColorArray any) (string, error) {
	colorIndex := ru.NN(node.ColorIndex, 0)
	var length float64
	switch v := borderColorArray.(type) {
	case []any:
		length = float64(len(v))
	case string:
		length = float64(js.UTF16Len(v))
	default:
		if js.IsNullish(borderColorArray) {
			return "", typeError("Cannot read properties of undefined (reading 'length')")
		}
		// The length of another value is undefined.
		length = math.NaN()
	}
	return "color-" + js.Str(math.Mod(colorIndex, length)), nil
}

// erBox is erBox(parent, node): the shape of an entity. It draws a plain
// box for an entity without attributes, else a table with one row for each
// attribute and the columns type, name, keys and comment.
func erBox(ctx *render.Context, parent *dom.Node, node *ru.Node, renderOptions *ru.ShapeRenderOptions) (*dom.Node, error) {
	// Treat node as entityNode for certain entityNode checks
	if alias := node.Get("alias"); js.Truthy(alias) {
		node.Label, node.HasLabel = js.ToString(alias), true
	}
	cfg := ctx.Config
	theme := cfg.Get("theme")
	themeVariables := js.AsObj(cfg.Get("themeVariables"))
	if themeVariables == nil {
		return nil, typeError("Cannot destructure property 'rowEven' of 'themeVariables' as it is undefined.")
	}
	rowEven, rowOdd := themeVariables.Get("rowEven"), themeVariables.Get("rowOdd")
	nodeBorder, borderColorArray := themeVariables.Get("nodeBorder"), themeVariables.Get("borderColorArray")
	themeName, themeIsString := theme.(string)

	// Background shapes are drawn to fill in the background color and cover up the ER diagram edge markers.
	// Draw background shape once.
	if node.Look == "handDrawn" {
		background := themeVariables.Get("background")
		backgroundNode := *node
		if node.Extra != nil {
			backgroundNode.Extra = make(map[string]any, len(node.Extra))
			for k, v := range node.Extra {
				backgroundNode.Extra[k] = v
			}
		}
		backgroundNode.ID = node.ID + "-background"
		if node.DomID != "" {
			backgroundNode.DomID = node.DomID + "-background"
		} else {
			backgroundNode.DomID = node.ID + "-background"
		}
		backgroundNode.Look = "default"
		backgroundNode.CSSStyles = []string{"stroke: none", "fill: " + js.ToString(background)}
		backgroundNode.CSSStylesSet = true
		if _, err := erBox(ctx, parent, &backgroundNode, renderOptions); err != nil {
			return nil, err
		}
	}

	htmlLabels := cfg.Get("htmlLabels")
	node.UseHtmlLabels, node.UseHtmlLabelsSet = js.Truthy(htmlLabels), htmlLabels != nil
	erConfig := js.AsObj(cfg.Get("er"))
	// config.er?.diagramPadding ?? 10
	padding := 10.0
	if v := js.Path(erConfig, "diagramPadding"); !js.IsNullish(v) {
		padding = js.ToNumber(v)
	}
	// config.er?.entityPadding ?? 6
	textPadding := 6.0
	if v := js.Path(erConfig, "entityPadding"); !js.IsNullish(v) {
		textPadding = js.ToNumber(v)
	}

	cssStyles := node.CSSStyles
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles

	attributes, err := boxAttributes(node)
	if err != nil {
		return nil, err
	}

	// Draw rect if no attributes are found
	if len(attributes) == 0 && node.Label != "" {
		options := ru.RectOptions{
			Rx:            0,
			Ry:            0,
			LabelPaddingX: padding,
			LabelPaddingY: float64(padding * 1.5),
			Classes:       "",
		}
		// Set minimum width
		if erConfig == nil {
			return nil, typeError("Cannot read properties of undefined (reading 'minEntityWidth')")
		}
		labelWidth, err := utils.CalculateTextWidth(ctx, node.Label, utils.TextDimensionConfigOf(cfg))
		if err != nil {
			return nil, err
		}
		if minEntityWidth := erConfig.Get("minEntityWidth"); labelWidth+float64(options.LabelPaddingX*2) < js.ToNumber(minEntityWidth) {
			node.Width = js.ToNumber(minEntityWidth)
		}
		shapeSvg, err := ru.DrawRect(ctx, parent, node, options)
		if err != nil {
			return nil, err
		}
		if themeIsString && colorThemes[themeName] {
			slot, err := colorSlot(node, borderColorArray)
			if err != nil {
				return nil, err
			}
			shapeSvg.Attr("data-color-id", slot)
		}

		// drawRect doesn't center non-htmlLabels correctly as of now, so translate label
		if !sanitize.Evaluate(htmlLabels) {
			textElement := shapeSvg.Select("text")
			if textElement == nil {
				return nil, typeError("Cannot read properties of undefined (reading 'width')")
			}
			bbox := textElement.BBox()
			textElement.Attr("transform", "translate("+js.Str(-bbox.Width/2)+", 0)")
		}
		return shapeSvg, nil
	}

	if !js.Truthy(htmlLabels) {
		padding *= 1.25
		textPadding *= 1.25
	}

	cssClasses := ru.GetNodeClasses(node)
	if cssClasses == "" {
		cssClasses = "node default"
	}

	domID := node.DomID
	if domID == "" {
		domID = node.ID
	}
	shapeSvg := parent.
		Insert("g").
		Attr("class", cssClasses).
		Attr("id", domID)

	nameBBox, err := addText(ctx, shapeSvg, node.Label, cfg, 0, 0, []string{"name"}, labelStyles)
	if err != nil {
		return nil, err
	}
	nameBBox.Height += textPadding
	if !sanitize.Evaluate(htmlLabels) {
		// The box of an SVG label is an SVGRect, which holds its numbers as
		// 32-bit floats: the assignment rounds the sum. The box of an HTML
		// label is a DOMRect, which holds doubles.
		nameBBox.Height = float64(float32(nameBBox.Height))
	}
	yOffset := 0.0
	yOffsets := []float64{}
	type row struct{ yOffset, rowHeight float64 }
	rows := []row{}
	maxTypeWidth := 0.0
	maxNameWidth := 0.0
	maxKeysWidth := 0.0
	maxCommentWidth := 0.0
	keysPresent := true
	commentPresent := true
	for _, attribute := range attributes {
		typeBBox, err := addText(ctx, shapeSvg, attribute.typ, cfg, 0, yOffset, []string{"attribute-type"}, labelStyles)
		if err != nil {
			return nil, err
		}
		maxTypeWidth = js.Max(maxTypeWidth, typeBBox.Width+padding)
		nameBBox, err := addText(ctx, shapeSvg, attribute.name, cfg, 0, yOffset, []string{"attribute-name"}, labelStyles)
		if err != nil {
			return nil, err
		}
		maxNameWidth = js.Max(maxNameWidth, nameBBox.Width+padding)
		keysBBox, err := addText(ctx, shapeSvg, attribute.keys, cfg, 0, yOffset, []string{"attribute-keys"}, labelStyles)
		if err != nil {
			return nil, err
		}
		maxKeysWidth = js.Max(maxKeysWidth, keysBBox.Width+padding)
		commentBBox, err := addText(ctx, shapeSvg, attribute.comment, cfg, 0, yOffset, []string{"attribute-comment"}, labelStyles)
		if err != nil {
			return nil, err
		}
		maxCommentWidth = js.Max(maxCommentWidth, commentBBox.Width+padding)

		rowHeight := js.Max(typeBBox.Height, nameBBox.Height, keysBBox.Height, commentBBox.Height) +
			textPadding
		rows = append(rows, row{yOffset: yOffset, rowHeight: rowHeight})
		yOffset += rowHeight
	}
	totalWidthSections := 4.0

	if maxKeysWidth <= padding {
		keysPresent = false
		maxKeysWidth = 0
		totalWidthSections--
	}
	if maxCommentWidth <= padding {
		commentPresent = false
		maxCommentWidth = 0
		totalWidthSections--
	}

	shapeBBox := shapeSvg.BBox()
	// Add extra padding to attribute components to accommodate for difference in width
	if nameBBox.Width+float64(padding*2)-(maxTypeWidth+maxNameWidth+maxKeysWidth+maxCommentWidth) > 0 {
		difference := nameBBox.Width + float64(padding*2) - (maxTypeWidth + maxNameWidth + maxKeysWidth + maxCommentWidth)
		maxTypeWidth += float64(difference / totalWidthSections)
		maxNameWidth += float64(difference / totalWidthSections)
		if maxKeysWidth > 0 {
			maxKeysWidth += float64(difference / totalWidthSections)
		}
		if maxCommentWidth > 0 {
			maxCommentWidth += float64(difference / totalWidthSections)
		}
	}

	maxWidth := maxTypeWidth + maxNameWidth + maxKeysWidth + maxCommentWidth

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := ru.UserNodeOverrides(ctx, node, js.NewObj[any]())

	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
		options.Set("fillStyle", "solid")
	}

	totalShapeBBoxHeight := 0.0
	if len(rows) > 0 {
		sum := 0.0
		for _, row := range rows {
			sum = sum + row.rowHeight
		}
		totalShapeBBoxHeight = sum
	}
	w := js.Max(shapeBBox.Width+float64(padding*2), ru.Or(node.Width, 0), maxWidth)
	h := js.Max(totalShapeBBoxHeight+nameBBox.Height, ru.Or(node.Height, 0))
	x := float64(-w / 2)
	y := float64(-h / 2)

	// Translate attribute text labels
	for _, text := range shapeSvg.SelectAll("g:not(:first-child)") {
		transform, _ := text.GetAttr("transform")
		translateX := 0.0
		translateY := 0.0

		if transform != "" {
			if translate := reTranslate.FindStringSubmatch(transform); translate != nil {
				translateX = js.ParseFloat(translate[1])
				translateY = js.ParseFloat(translate[2])
				class, ok := text.GetAttr("class")
				if !ok {
					return nil, typeError("Cannot read properties of null (reading 'includes')")
				}
				if strings.Contains(class, "attribute-name") {
					translateX += maxTypeWidth
				} else if strings.Contains(class, "attribute-keys") {
					translateX += maxTypeWidth + maxNameWidth
				} else if strings.Contains(class, "attribute-comment") {
					translateX += maxTypeWidth + maxNameWidth + maxKeysWidth
				}
			}
		}

		text.Attr(
			"transform",
			"translate("+js.Str(x+float64(padding/2)+translateX)+", "+js.Str(translateY+y+nameBBox.Height+float64(textPadding/2))+")",
		)
	}
	// Center the name
	shapeSvg.
		Select(".name").
		Attr("transform", "translate("+js.Str(-nameBBox.Width/2)+", "+js.Str(y+float64(textPadding/2))+")")

	if themeIsString && colorThemes[themeName] {
		slot, err := colorSlot(node, borderColorArray)
		if err != nil {
			return nil, err
		}
		shapeSvg.Attr("data-color-id", slot)
	}
	// Draw shape
	roughRect, err := rc.Rectangle(x, y, w, h, options)
	if err != nil {
		return nil, err
	}
	if !node.CSSStylesSet && cssStyles == nil {
		return nil, typeError("Cannot read properties of undefined (reading 'join')")
	}
	rect := shapeSvg.
		InsertNode(roughRect, ":first-child").
		Attr("class", "outer-path").
		Attr("style", strings.Join(cssStyles, ""))

	yOffsets = append(yOffsets, 0)
	// Draw row rects
	for i, row := range rows {
		contentRowIndex := i + 1 // Adjusted index to skip the header (name) row
		isEven := contentRowIndex%2 == 0 && row.yOffset != 0
		rowOptions := js.NewObj[any]()
		options.Each(func(k string, v any) { rowOptions.Set(k, v) })
		parity := "odd"
		if isEven {
			rowOptions.Set("fill", rowEven)
			parity = "even"
		} else {
			rowOptions.Set("fill", rowOdd)
		}
		rowOptions.Set("stroke", nodeBorder)
		roughRect, err := rc.Rectangle(x, nameBBox.Height+y+row.yOffset, w, row.rowHeight, rowOptions)
		if err != nil {
			return nil, err
		}
		shapeSvg.
			InsertNode(roughRect, "g.label").
			Attr("style", strings.Join(cssStyles, "")).
			Attr("class", "row-rect-"+parity)
	}

	// Draw divider lines
	// Name line
	const thickness = 0.0001

	divider := func(x1, y1, x2, y2 float64) error {
		points := lineToPolygon(x1, y1, x2, y2, thickness)
		roughLine, err := rc.Polygon(points, options)
		if err != nil {
			return err
		}
		shapeSvg.AppendNode(roughLine).Attr("class", "divider")
		return nil
	}

	// 1. Top horizontal line
	if err := divider(x, nameBBox.Height+y, w+x, nameBBox.Height+y); err != nil {
		return nil, err
	}
	// First line
	if err := divider(maxTypeWidth+x, nameBBox.Height+y, maxTypeWidth+x, h+y); err != nil {
		return nil, err
	}
	// Second line
	if keysPresent {
		xCoord := maxTypeWidth + maxNameWidth + x
		if err := divider(xCoord, nameBBox.Height+y, xCoord, h+y); err != nil {
			return nil, err
		}
	}
	// Third line
	if commentPresent {
		xCoord := maxTypeWidth + maxNameWidth + maxKeysWidth + x
		if err := divider(xCoord, nameBBox.Height+y, xCoord, h+y); err != nil {
			return nil, err
		}
	}

	// Attribute divider lines
	for _, yOffset := range yOffsets {
		yCoord := nameBBox.Height + y + yOffset
		if err := divider(x, yCoord, w+x, yCoord); err != nil {
			return nil, err
		}
	}

	ru.UpdateNodeBounds(node, rect)

	if nodeStyles != "" && node.Look != "handDrawn" {
		if themeIsString && reduxThemes[themeName] {
			for _, path := range shapeSvg.SelectAll("path") {
				path.Attr("style", nodeStyles)
			}
		} else {
			allStyle := strings.Split(nodeStyles, ";")
			var strokes []string
			for _, e := range allStyle {
				if strings.Contains(e, "stroke") {
					strokes = append(strokes, e)
				}
			}
			strokeStyles := strings.Join(strokes, "; ")
			for _, path := range shapeSvg.SelectAll("path") {
				path.Attr("style", strokeStyles)
			}
			for _, path := range shapeSvg.SelectAll(".row-rect-even path") {
				path.Attr("style", nodeStyles)
			}
		}
	}

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectRect(ru.NodeBounds(node), point)
	}
	return shapeSvg, nil
}

// replaceAngleEntities is text.replaceAll('&lt;', '<').replaceAll('&gt;', '>').
func replaceAngleEntities(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "&lt;", "<"), "&gt;", ">")
}

// addText is the helper function addText of erBox.ts: it adds a label
// group with a translate position and a style, and returns the measured box
// of the text.
func addText(ctx *render.Context, shapeSvg *dom.Node, labelText string, cfg *js.Obj[any], translateX, translateY float64, classes []string, style string) (dom.Rect, error) {
	htmlLabels := cfg.Get("htmlLabels")
	label := shapeSvg.
		Insert("g").
		Attr("class", "label "+strings.Join(classes, " ")).
		Attr("transform", "translate("+js.Str(translateX)+", "+js.Str(translateY)+")").
		Attr("style", style)

	// Return types need to be parsed
	if parsed := sanitize.ParseGenericTypes(labelText); labelText != parsed {
		labelText = parsed
		// Work around
		labelText = strings.ReplaceAll(strings.ReplaceAll(labelText, "<", "&lt;"), ">", "&gt;")
	}

	textWidth, err := utils.CalculateTextWidth(ctx, labelText, utils.TextDimensionConfigOf(cfg))
	if err != nil {
		return dom.Rect{}, err
	}
	options := ru.DefaultTextOptions()
	options.Width = textWidth + 100
	options.Style = style
	// useHtmlLabels: config.htmlLabels. An undefined value takes the
	// default of createText, which is true.
	options.UseHtmlLabels = htmlLabels == nil || js.Truthy(htmlLabels)
	created, err := ru.CreateText(ctx, label, labelText, options)
	if err != nil {
		return dom.Rect{}, err
	}
	// createText returns the text element of an SVG label, which is in a
	// group with the background rect: appendChild moves it out of the group.
	text := label.AppendChild(created)
	// Undo work around now that text passed through correctly
	if strings.Contains(labelText, "&lt;") || strings.Contains(labelText, "&gt;") {
		child := text.FirstElementChild()
		if child == nil {
			return dom.Rect{}, typeError("Cannot read properties of undefined (reading 'textContent')")
		}
		child.Text(replaceAngleEntities(child.TextContent()))
		// The loop of upstream goes on to the first child node, which is
		// the text node that the assignment above made. Its content has no
		// entity left, so the second replacement changes nothing.
	}

	if sanitize.Evaluate(htmlLabels) {
		div := text.FirstElementChild()
		if div == nil {
			return dom.Rect{}, typeError("Cannot read properties of undefined (reading 'style')")
		}
		div.Style("text-align", "start")
		bbox := div.BoundingClientRect()
		text.Attr("width", bbox.Width)
		text.Attr("height", bbox.Height)
		return bbox, nil
	}
	return text.BBox(), nil
}

// lineToPolygon is lineToPolygon(x1, y1, x2, y2, thickness): a thin polygon
// for a line that is parallel to an axis.
func lineToPolygon(x1, y1, x2, y2, thickness float64) []rough.Point {
	if x1 == x2 {
		// Vertical line
		return []rough.Point{
			{x1 - float64(thickness/2), y1},
			{x1 + float64(thickness/2), y1},
			{x2 + float64(thickness/2), y2},
			{x2 - float64(thickness/2), y2},
		}
	}
	// Horizontal line (ER dividers are always axis-aligned)
	return []rough.Point{
		{x1, y1 - float64(thickness/2)},
		{x1, y1 + float64(thickness/2)},
		{x2, y2 + float64(thickness/2)},
		{x2, y2 - float64(thickness/2)},
	}
}
