// Ported from mermaid 12.2.0, diagrams/treeView/renderer.ts (MIT).

package treeview

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

const (
	iconSize = 14.0 // ICON_SIZE
	iconGap  = 4.0  // ICON_GAP
	descGap  = 16.0 // DESC_GAP
)

// renderInfo is RenderInfo. labelRightEdge is a number, or a text when a
// padding of the config is a text.
type renderInfo struct {
	node           *Node
	nodeGroup      *dom.Node
	labelRightEdge any
	centerY        float64
}

// add is the + operator of JavaScript for numbers and config values: a
// text, an array or an object on one side gives a text.
func add(a, b any) any {
	isText := func(v any) bool {
		switch v.(type) {
		case string, []any, *js.Obj[any]:
			return true
		}
		return false
	}
	if isText(a) || isText(b) {
		return js.ToString(a) + js.ToString(b)
	}
	return js.ToNumber(a) + js.ToNumber(b)
}

// attrValue is a value for selection.attr: null removes the attribute, as
// undefined does.
func attrValue(v any) any {
	if js.IsNullish(v) {
		return nil
	}
	return v
}

// hasHighlight is cssClass?.split(/\s+/).includes('highlight').
func hasHighlight(cssClass string) bool {
	for _, class := range strings.FieldsFunc(cssClass, js.IsWhiteSpace) {
		if class == "highlight" {
			return true
		}
	}
	return false
}

// resolveNodeIcons is resolveNodeIcons(root, config): the svg of the icon
// of each node that has an icon, by the id of the node.
func resolveNodeIcons(ctx *render.Context, root *Node, config *js.Obj[any]) (map[int]string, error) {
	type nodeIcon struct {
		icon string
		node *Node
	}
	nodeIcons := []nodeIcon{}
	var collect func(node *Node) error
	collect = func(node *Node) error {
		icon, err := getNodeIcon(node, config)
		if err != nil {
			return err
		}
		if js.Truthy(icon) {
			// An icon reference that is not a string names no icon:
			// getIconSVG then gives the unknown icon, as for an empty name.
			name, _ := icon.(string)
			nodeIcons = append(nodeIcons, nodeIcon{icon: name, node: node})
		}
		for _, child := range node.Children {
			if err := collect(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := collect(root); err != nil {
		return nil, err
	}

	svgs := make([]string, len(nodeIcons))
	err := ctx.All(len(nodeIcons), func(i int) error {
		svg, err := ru.GetIconSVG(ctx, nodeIcons[i].icon, map[string]any{
			"height": iconSize,
			"width":  iconSize,
		}, nil)
		if err != nil {
			return err
		}
		ctx.Await()
		svgs[i] = svg
		return nil
	})
	if err != nil {
		return nil, err
	}

	iconSVGs := make(map[int]string, len(nodeIcons))
	for i, ni := range nodeIcons {
		iconSVGs[ni.node.ID] = svgs[i]
	}
	return iconSVGs, nil
}

// treeConfig holds the options of the config that drawTree reads. The raw
// values keep the type that the config has, for the places where upstream
// adds them with + or prints them.
type treeConfig struct {
	obj              *js.Obj[any]
	rawPaddingX      any
	rawPaddingY      any
	rawRowIndent     any
	rawLineThickness any
	paddingX         float64
	paddingY         float64
	rowIndent        float64
	lineThickness    float64
}

func newTreeConfig(config *js.Obj[any]) *treeConfig {
	c := &treeConfig{
		obj:              config,
		rawPaddingX:      config.Get("paddingX"),
		rawPaddingY:      config.Get("paddingY"),
		rawRowIndent:     config.Get("rowIndent"),
		rawLineThickness: config.Get("lineThickness"),
	}
	c.paddingX = js.ToNumber(c.rawPaddingX)
	c.paddingY = js.ToNumber(c.rawPaddingY)
	c.rowIndent = js.ToNumber(c.rawRowIndent)
	c.lineThickness = js.ToNumber(c.rawLineThickness)
	return c
}

// positionLabel is positionLabel(x, y, node, domElem, config, iconSVGs).
func positionLabel(x, y float64, node *Node, domElem *dom.Node, config *treeConfig, iconSVGs map[int]string) (renderInfo, error) {
	nodeGroup := domElem.Append("g")
	cssClasses := "treeView-node-label"
	if node.NodeType == directoryNode {
		cssClasses += " treeView-node-dir"
	}
	if node.CSSClass != "" {
		cssClasses += " " + node.CSSClass
	}

	// Explicit icon() annotations always render; defaults only when showIcons is on
	iconOffset := iconSize + iconGap
	icon, err := getNodeIcon(node, config.obj)
	if err != nil {
		return renderInfo{}, err
	}
	showIcon := icon != nil
	if js.Truthy(icon) {
		nodeGroup.
			Append("g").
			Attr("class", "treeView-node-icon").
			Attr("transform", "translate("+js.ToString(add(x, config.rawPaddingX))+", "+js.ToString(add(y, config.rawPaddingY))+")").
			HTML(iconSVGs[node.ID])
	}

	// Label text
	label := nodeGroup.
		Append("text").
		Text(node.Name).
		Attr("dominant-baseline", "middle").
		Attr("class", cssClasses)
	bbox := label.BBox()
	labelHeight, labelWidth := bbox.Height, bbox.Width
	height := labelHeight + float64(config.paddingY*2)
	offset := 0.0
	if showIcon {
		offset = iconOffset
	}
	labelX := add(add(x, config.rawPaddingX), offset)
	label.Attr("x", labelX)
	label.Attr("y", y+float64(height/2))

	labelRightEdge := add(labelX, labelWidth)
	width := labelWidth + float64(config.paddingX*2) + offset
	node.BBox = &BBox{X: x, Y: y, Width: width, Height: height}

	// Highlight background rect (sized later in drawTree)
	if hasHighlight(node.CSSClass) {
		nodeGroup.
			Insert("rect", ":first-child").
			Attr("x", x).
			Attr("y", y+1).
			Attr("width", 0).
			Attr("height", height-2).
			Attr("rx", 3).
			Attr("class", "treeView-highlight-bg")
	}

	return renderInfo{node: node, nodeGroup: nodeGroup, labelRightEdge: labelRightEdge, centerY: y + float64(height/2)}, nil
}

// positionLine is positionLine(domElem, x1, y1, x2, y2, lineThickness).
func positionLine(domElem *dom.Node, x1, y1, x2, y2, lineThickness any) *dom.Node {
	return domElem.
		Append("line").
		Attr("x1", attrValue(x1)).
		Attr("y1", attrValue(y1)).
		Attr("x2", attrValue(x2)).
		Attr("y2", attrValue(y2)).
		Attr("stroke-width", attrValue(lineThickness)).
		Attr("class", "treeView-node-line")
}

// drawTree is drawTree(elem, root, config, iconSVGs).
func drawTree(elem *dom.Node, root *Node, config *treeConfig, iconSVGs map[int]string) (totalHeight, totalWidth float64, err error) {
	renderInfos := []renderInfo{}

	drawNode := func(node *Node, depth int) error {
		// config.rowIndent + config.paddingX joins two texts.
		indent := float64(float64(depth) * js.ToNumber(add(config.rawRowIndent, config.rawPaddingX)))
		info, err := positionLabel(indent, totalHeight, node, elem, config, iconSVGs)
		if err != nil {
			return err
		}
		renderInfos = append(renderInfos, info)
		height, width := node.BBox.Height, node.BBox.Width
		positionLine(
			elem,
			indent-config.rowIndent,
			totalHeight+float64(height/2),
			indent,
			totalHeight+float64(height/2),
			config.rawLineThickness,
		)

		totalWidth = js.Max(totalWidth, indent+width)
		totalHeight += height
		return nil
	}

	var processNode func(node *Node, depth int) error
	processNode = func(node *Node, depth int) error {
		if err := drawNode(node, depth); err != nil {
			return err
		}
		for _, child := range node.Children {
			if err := processNode(child, depth+1); err != nil {
				return err
			}
		}
		x, y, height := node.BBox.X, node.BBox.Y, node.BBox.Height
		if len(node.Children) > 0 {
			last := node.Children[len(node.Children)-1].BBox
			endY, endHeight := last.Y, last.Height
			positionLine(
				elem,
				add(x, config.rawPaddingX),
				y+height,
				add(x, config.rawPaddingX),
				endY+float64(endHeight/2)+float64(config.lineThickness/2),
				config.rawLineThickness,
			)
		}
		return nil
	}

	if err := processNode(root, 0); err != nil {
		return 0, 0, err
	}

	// Phase 2: Add descriptions, aligned to a common column
	nodesWithDesc := []renderInfo{}
	for _, ri := range renderInfos {
		if ri.node.Description != "" {
			nodesWithDesc = append(nodesWithDesc, ri)
		}
	}
	if len(nodesWithDesc) > 0 {
		edges := make([]float64, len(renderInfos))
		for i, ri := range renderInfos {
			edges[i] = js.ToNumber(ri.labelRightEdge)
		}
		maxLabelRight := js.Max(edges...)
		descX := maxLabelRight + descGap
		for _, ri := range nodesWithDesc {
			desc := ri.nodeGroup.
				Append("text").
				Text(ri.node.Description).
				Attr("dominant-baseline", "middle").
				Attr("class", "treeView-node-description").
				Attr("x", descX).
				Attr("y", ri.centerY)
			descBBox := desc.BBox()
			totalWidth = js.Max(totalWidth, js.ToNumber(add(descX+descBBox.Width, config.rawPaddingX)))
		}
	}

	// Phase 3: Size highlight background rects to full tree width
	for _, ri := range renderInfos {
		if hasHighlight(ri.node.CSSClass) {
			rect := ri.nodeGroup.Select(".treeView-highlight-bg")
			if rect != nil {
				rectWidth := totalWidth - ri.node.BBox.X + 8
				rect.Attr("width", rectWidth)
				// Expand totalWidth to ensure the viewBox includes the highlight rect + stroke
				totalWidth = js.Max(totalWidth, ri.node.BBox.X+rectWidth+2)
			}
		}
	}

	return totalHeight, totalWidth, nil
}

// draw is draw(text, id, _ver, diagObj).
func draw(ctx *render.Context, rdb render.DB, _ string) error {
	// The module registers its icon pack when it loads. Here the packs
	// belong to the render.
	if err := ru.RegisterIconPacks(ctx, ru.IconLoader{
		Name:  treeViewIconsPrefix,
		Icons: treeViewIcons(),
	}); err != nil {
		return err
	}

	db := rdb.(*DB)
	root := db.GetRoot()
	config := db.GetConfig()

	svg := ctx.SelectSvgElement()

	treeElem := svg.Append("g")
	treeElem.Attr("class", "tree-view")

	iconSVGs, err := resolveNodeIcons(ctx, root, config)
	if err != nil {
		return err
	}
	treeConf := newTreeConfig(config)
	totalHeight, totalWidth, err := drawTree(treeElem, root, treeConf, iconSVGs)
	if err != nil {
		return err
	}
	// -${config.lineThickness/2} is required for a line with x coordinate = 0
	// as there is overflow to the left due to the line being centered
	svg.Attr("viewBox", "-"+js.Str(treeConf.lineThickness/2)+" 0 "+js.Str(totalWidth)+" "+js.Str(totalHeight))
	render.ConfigureSvgSize(svg, totalHeight, totalWidth, js.Truthy(config.Get("useMaxWidth")))
	return nil
}
