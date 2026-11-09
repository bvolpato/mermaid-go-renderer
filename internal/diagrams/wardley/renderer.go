// Ported from mermaid 12.2.0, diagrams/wardley/wardleyRenderer.ts (MIT).
//
// The options of the config and the colors of the theme are dynamic values
// here, as they are upstream: a text where the schema has a number changes
// the meaning of +, and upstream prints some options as they are. The
// functions add, num and str are the operators of JavaScript for such
// values.

package wardley

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// defaultStages is DEFAULT_STAGES.
var defaultStages = []string{"Genesis", "Custom Built", "Product", "Commodity"}

// add is a + b of JavaScript: a text, an array or an object on one side
// gives a text.
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

// num is Number(v): the value of an operand of -, *, / and of a comparison.
func num(v any) float64 { return js.ToNumber(v) }

// str is `${v}`: a value in a template literal.
func str(v any) string { return js.ToString(v) }

// attrValue is a value for selection.attr: null removes the attribute, as
// undefined does.
func attrValue(v any) any {
	if js.IsNullish(v) {
		return nil
	}
	return v
}

// or is a ?? b ?? ...: the first value that is not undefined or null.
func or(values ...any) any {
	for _, v := range values {
		if !js.IsNullish(v) {
			return v
		}
	}
	return values[len(values)-1]
}

// wardleyTheme is WardleyTheme.
type wardleyTheme struct {
	backgroundColor     any
	axisColor           any
	axisTextColor       any
	gridColor           any
	componentFill       any
	componentStroke     any
	componentLabelColor any
	linkStroke          any
	evolutionStroke     any
	annotationStroke    any
	annotationTextColor any
	annotationFill      any
}

// getTheme is getTheme().
func getTheme(ctx *render.Context) wardleyTheme {
	themeVariables := ctx.Config.Get("themeVariables")
	// wardley is themeVariables.wardley?.[key].
	wardley := func(key string) any {
		return js.GetMember(js.GetMember(themeVariables, "wardley"), key)
	}
	variable := func(key string) any { return js.GetMember(themeVariables, key) }
	return wardleyTheme{
		backgroundColor:     or(wardley("backgroundColor"), variable("background"), "#fff"),
		axisColor:           or(wardley("axisColor"), "#000"),
		axisTextColor:       or(wardley("axisTextColor"), variable("primaryTextColor"), "#222"),
		gridColor:           or(wardley("gridColor"), "rgba(100, 100, 100, 0.2)"),
		componentFill:       or(wardley("componentFill"), "#fff"),
		componentStroke:     or(wardley("componentStroke"), "#000"),
		componentLabelColor: or(wardley("componentLabelColor"), variable("primaryTextColor"), "#222"),
		linkStroke:          or(wardley("linkStroke"), "#000"),
		evolutionStroke:     or(wardley("evolutionStroke"), "#dc3545"),
		annotationStroke:    or(wardley("annotationStroke"), "#000"),
		annotationTextColor: or(wardley("annotationTextColor"), variable("primaryTextColor"), "#222"),
		annotationFill:      or(wardley("annotationFill"), variable("background"), "#fff"),
	}
}

// wardleyConfigValues is the result of getConfigValues().
type wardleyConfigValues struct {
	width           any
	height          any
	padding         any
	nodeRadius      any
	nodeLabelOffset any
	axisFontSize    any
	labelFontSize   any
	showGrid        any
	useMaxWidth     any
}

// getConfigValues is getConfigValues().
func getConfigValues(ctx *render.Context) wardleyConfigValues {
	wardleyConfig := ctx.Config.Get("wardley-beta")
	option := func(key string, fallback any) any {
		return or(js.GetMember(wardleyConfig, key), fallback)
	}
	return wardleyConfigValues{
		width:           option("width", 900.0),
		height:          option("height", 600.0),
		padding:         option("padding", 48.0),
		nodeRadius:      option("nodeRadius", 6.0),
		nodeLabelOffset: option("nodeLabelOffset", 8.0),
		axisFontSize:    option("axisFontSize", 12.0),
		labelFontSize:   option("labelFontSize", 10.0),
		showGrid:        option("showGrid", false),
		useMaxWidth:     option("useMaxWidth", true),
	}
}

// configureSvgSize is configureSvgSize(svgElem, height, width, useMaxWidth)
// for sizes that can be texts.
func configureSvgSize(svg *dom.Node, height, width any, useMaxWidth bool) {
	if useMaxWidth {
		svg.Attr("width", "100%")
		svg.Attr("style", "max-width: "+str(width)+"px;")
	} else {
		svg.Attr("height", attrValue(height))
		svg.Attr("width", attrValue(width))
	}
}

// position is an entry of the map positions: the place of a node on the
// canvas. x is a number, or a text when the padding of the config is a text.
type position struct {
	x    any
	y    float64
	node *Node
}

// draw is draw(text, id, _version, diagObj).
func draw(ctx *render.Context, rdb render.DB, _ string) error {
	id := ctx.ID
	configValues := getConfigValues(ctx)
	theme := getTheme(ctx)
	nodeRadius := num(configValues.nodeRadius)
	squareSize := float64(nodeRadius * 1.6) // Size of pipeline parent square nodes
	db := rdb.(*DB)
	data := db.GetWardleyData()
	title := db.GetDiagramTitle()

	// Override size if specified in diagram
	rawWidth, rawHeight := configValues.width, configValues.height
	if data.Size != nil {
		rawWidth, rawHeight = data.Size.Width, data.Size.Height
	}
	width, height := num(rawWidth), num(rawHeight)
	padding := num(configValues.padding)

	svg := ctx.SelectSvgElement()
	svg.SelectAll("*").Remove()

	configureSvgSize(svg, rawHeight, rawWidth, js.Truthy(configValues.useMaxWidth))
	svg.Attr("viewBox", "0 0 "+str(rawWidth)+" "+str(rawHeight))

	root := svg.Append("g").Attr("class", "wardley-map")

	// Define arrow markers
	defs := svg.Append("defs")

	// Evolution trend arrow (red)
	defs.
		Append("marker").
		Attr("id", "arrow-"+id).
		Attr("viewBox", "0 0 10 10").
		Attr("refX", 9).
		Attr("refY", 5).
		Attr("markerWidth", 6).
		Attr("markerHeight", 6).
		Attr("orient", "auto-start-reverse").
		Append("path").
		Attr("d", "M 0 0 L 10 5 L 0 10 z").
		Attr("fill", attrValue(theme.evolutionStroke)).
		Attr("stroke", "none")

	// Link flow arrow (theme color)
	defs.
		Append("marker").
		Attr("id", "link-arrow-end-"+id).
		Attr("viewBox", "0 0 10 10").
		Attr("refX", 9).
		Attr("refY", 5).
		Attr("markerWidth", 5).
		Attr("markerHeight", 5).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 0 0 L 10 5 L 0 10 z").
		Attr("fill", attrValue(theme.linkStroke)).
		Attr("stroke", "none")

	// Link flow arrow start (for backward/bidirectional)
	defs.
		Append("marker").
		Attr("id", "link-arrow-start-"+id).
		Attr("viewBox", "0 0 10 10").
		Attr("refX", 1).
		Attr("refY", 5).
		Attr("markerWidth", 5).
		Attr("markerHeight", 5).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 10 0 L 0 5 L 10 10 z").
		Attr("fill", attrValue(theme.linkStroke)).
		Attr("stroke", "none")

	root.
		Append("rect").
		Attr("class", "wardley-background").
		Attr("width", attrValue(rawWidth)).
		Attr("height", attrValue(rawHeight)).
		Attr("fill", attrValue(theme.backgroundColor))

	chartWidth := width - float64(padding*2)
	chartHeight := height - float64(padding*2)

	// Render title if present
	if title != "" {
		root.
			Append("text").
			Attr("class", "wardley-title").
			Attr("x", width/2).
			Attr("y", padding/2).
			Attr("fill", attrValue(theme.axisTextColor)).
			Attr("font-size", num(configValues.axisFontSize)*1.05).
			Attr("font-weight", "bold").
			Attr("text-anchor", "middle").
			Attr("dominant-baseline", "middle").
			Text(title)
	}

	// projectX is configValues.padding + (value / 100) * chartWidth: the
	// padding is on the left of a +.
	projectX := func(value float64) any {
		return add(configValues.padding, float64(float64(value/100)*chartWidth))
	}
	projectY := func(value float64) float64 {
		return height - padding - float64(float64(value/100)*chartHeight)
	}

	axisGroup := root.Append("g").Attr("class", "wardley-axes")
	axisGroup.
		Append("line").
		Attr("x1", attrValue(configValues.padding)).
		Attr("x2", width-padding).
		Attr("y1", height-padding).
		Attr("y2", height-padding).
		Attr("stroke", attrValue(theme.axisColor)).
		Attr("stroke-width", 1)
	axisGroup.
		Append("line").
		Attr("x1", attrValue(configValues.padding)).
		Attr("x2", attrValue(configValues.padding)).
		Attr("y1", attrValue(configValues.padding)).
		Attr("y2", height-padding).
		Attr("stroke", attrValue(theme.axisColor)).
		Attr("stroke-width", 1)

	xLabel := "Evolution"
	if data.Axes.XLabel != nil {
		xLabel = *data.Axes.XLabel
	}
	yLabel := "Visibility"
	if data.Axes.YLabel != nil {
		yLabel = *data.Axes.YLabel
	}

	axisGroup.
		Append("text").
		Attr("class", "wardley-axis-label wardley-axis-label-x").
		Attr("x", add(configValues.padding, float64(chartWidth/2))).
		Attr("y", height-float64(padding/4)).
		Attr("fill", attrValue(theme.axisTextColor)).
		Attr("font-size", attrValue(configValues.axisFontSize)).
		Attr("font-weight", "bold").
		Attr("text-anchor", "middle").
		Text(xLabel)
	axisGroup.
		Append("text").
		Attr("class", "wardley-axis-label wardley-axis-label-y").
		Attr("x", padding/3).
		Attr("y", add(configValues.padding, float64(chartHeight/2))).
		Attr("fill", attrValue(theme.axisTextColor)).
		Attr("font-size", attrValue(configValues.axisFontSize)).
		Attr("font-weight", "bold").
		Attr("text-anchor", "middle").
		Attr(
			"transform",
			"rotate(-90 "+js.Str(padding/3)+" "+str(add(configValues.padding, float64(chartHeight/2)))+")",
		).
		Text(yLabel)

	stages := defaultStages
	if len(data.Axes.Stages) > 0 {
		stages = data.Axes.Stages
	}
	if len(stages) > 0 {
		stageGroup := root.Append("g").Attr("class", "wardley-stages")
		boundaries := data.Axes.StageBoundaries

		// Calculate stage positions
		type stagePosition struct{ start, end float64 }
		stagePositions := []stagePosition{}
		if boundaries != nil && len(boundaries) == len(stages) {
			// Use custom boundaries
			prevBoundary := 0.0
			for _, boundary := range boundaries {
				stagePositions = append(stagePositions, stagePosition{start: prevBoundary, end: boundary})
				prevBoundary = boundary
			}
		} else {
			// Use equal distribution
			stageWidth := 1.0 / float64(len(stages))
			for index := range stages {
				stagePositions = append(stagePositions, stagePosition{
					start: float64(index) * stageWidth,
					end:   float64(index+1) * stageWidth,
				})
			}
		}

		for index, stage := range stages {
			pos := stagePositions[index]
			startX := add(configValues.padding, float64(pos.start*chartWidth))
			endX := add(configValues.padding, float64(pos.end*chartWidth))
			centerX := num(add(startX, endX)) / 2

			// Draw dividing line (except before first stage)
			if index > 0 {
				stageGroup.
					Append("line").
					Attr("x1", startX).
					Attr("x2", startX).
					Attr("y1", attrValue(configValues.padding)).
					Attr("y2", height-padding).
					Attr("stroke", "#000").
					Attr("stroke-width", 1).
					Attr("stroke-dasharray", "5 5").
					Attr("opacity", 0.8)
			}

			// Draw stage label
			stageGroup.
				Append("text").
				Attr("class", "wardley-stage-label").
				Attr("x", centerX).
				Attr("y", height-float64(padding/1.5)).
				Attr("fill", attrValue(theme.axisTextColor)).
				Attr("font-size", num(configValues.axisFontSize)-2).
				Attr("text-anchor", "middle").
				Text(stage)
		}
	}

	if js.Truthy(configValues.showGrid) {
		gridGroup := root.Append("g").Attr("class", "wardley-grid")
		for i := 1; i < 4; i++ {
			ratio := float64(i) / 4
			x := add(configValues.padding, float64(chartWidth*ratio))
			gridGroup.
				Append("line").
				Attr("x1", x).
				Attr("x2", x).
				Attr("y1", attrValue(configValues.padding)).
				Attr("y2", height-padding).
				Attr("stroke", attrValue(theme.gridColor)).
				Attr("stroke-dasharray", "2 6")
			gridGroup.
				Append("line").
				Attr("x1", attrValue(configValues.padding)).
				Attr("x2", width-padding).
				Attr("y1", height-padding-float64(chartHeight*ratio)).
				Attr("y2", height-padding-float64(chartHeight*ratio)).
				Attr("stroke", attrValue(theme.gridColor)).
				Attr("stroke-dasharray", "2 6")
		}
	}

	positions := map[string]*position{}
	nodesByID := map[string]*Node{}
	for _, node := range data.Nodes {
		positions[node.ID] = &position{
			x:    projectX(node.X),
			y:    projectY(node.Y),
			node: node,
		}
		nodesByID[node.ID] = node
	}

	// Render pipeline boxes and evolution links
	if len(data.Pipelines) > 0 {
		pipelineGroup := root.Append("g").Attr("class", "wardley-pipelines")
		pipelineLinksGroup := root.Append("g").Attr("class", "wardley-pipeline-links")

		for _, pipeline := range data.Pipelines {
			if len(pipeline.ComponentIDs) == 0 {
				continue
			}

			// Sort components by X coordinate (evolution) to draw links in order
			type component struct {
				id   string
				pos  *position
				node *Node
			}
			sortedComponents := []component{}
			for _, id := range pipeline.ComponentIDs {
				c := component{id: id, pos: positions[id], node: nodesByID[id]}
				if c.pos != nil && c.node != nil {
					sortedComponents = append(sortedComponents, c)
				}
			}
			js.SortFunc(sortedComponents, func(a, b component) float64 { return a.node.X - b.node.X })

			// Draw dotted links between consecutive pipeline components
			for i := 0; i < len(sortedComponents)-1; i++ {
				current := sortedComponents[i]
				next := sortedComponents[i+1]

				pipelineLinksGroup.
					Append("line").
					Attr("class", "wardley-pipeline-evolution-link").
					Attr("x1", current.pos.x).
					Attr("y1", current.pos.y).
					Attr("x2", next.pos.x).
					Attr("y2", next.pos.y).
					Attr("stroke", attrValue(theme.linkStroke)).
					Attr("stroke-width", 1).
					Attr("stroke-dasharray", "4 4")
			}

			// Find min and max X coordinates of pipeline components
			minX := math.Inf(1)
			maxX := math.Inf(-1)
			y := 0.0

			for _, componentID := range pipeline.ComponentIDs {
				if pos := positions[componentID]; pos != nil {
					minX = js.Min(minX, num(pos.x))
					maxX = js.Max(maxX, num(pos.x))
					y = pos.y
				}
			}

			if minX != math.Inf(1) && maxX != math.Inf(-1) {
				const padding = 15.0              // Padding around the box
				height := float64(nodeRadius * 4) // Height of the pipeline box
				boxTop := y - float64(height/2)

				// Position the parent node at the top of the pipeline box, 2/3 outside, 1/3 inside
				if parentPos := positions[pipeline.NodeID]; parentPos != nil {
					centerX := (minX + maxX) / 2
					parentPos.x = centerX
					parentPos.y = boxTop - float64(squareSize/6) // Position so 2/3 is outside, 1/3 inside
				}

				pipelineGroup.
					Append("rect").
					Attr("class", "wardley-pipeline-box").
					Attr("x", minX-padding).
					Attr("y", boxTop).
					Attr("width", maxX-minX+padding*2).
					Attr("height", height).
					Attr("fill", "none").
					Attr("stroke", attrValue(theme.axisColor)).
					Attr("stroke-width", 1.5).
					Attr("rx", 4).
					Attr("ry", 4)
			}
		}
	}

	linksGroup := root.Append("g").Attr("class", "wardley-links")

	// Build a map of pipeline parent -> component IDs for filtering
	pipelineMap := map[string]map[string]bool{}
	for _, pipeline := range data.Pipelines {
		components := map[string]bool{}
		for _, componentID := range pipeline.ComponentIDs {
			components[componentID] = true
		}
		pipelineMap[pipeline.NodeID] = components
	}

	validLinks := []LinkData{}
	for _, link := range data.Links {
		// Check if link has valid positions
		if positions[link.Source] == nil || positions[link.Target] == nil {
			continue
		}

		// Filter out links from pipeline components to their parent
		if pipelineMap[link.Target][link.Source] {
			continue
		}

		validLinks = append(validLinks, link)
	}
	// radiusOf is the radius of the shape of a node: a number, or the
	// option nodeRadius as it is in the config.
	radiusOf := func(node *Node) float64 {
		if node.IsPipelineParent {
			return squareSize / math.Sqrt(2)
		}
		return nodeRadius
	}
	for _, link := range validLinks {
		sourcePos := positions[link.Source]
		targetPos := positions[link.Target]
		sourceNode := nodesByID[link.Source]
		targetNode := nodesByID[link.Target]
		class := "wardley-link"
		if link.Dashed {
			class += " wardley-link--dashed"
		}
		line := linksGroup.Append("line").Attr("class", class)
		{
			radius := radiusOf(sourceNode)
			dx := num(targetPos.x) - num(sourcePos.x)
			dy := targetPos.y - sourcePos.y
			distance := math.Sqrt(float64(dx*dx) + float64(dy*dy))
			line.Attr("x1", add(sourcePos.x, float64(float64(dx/distance)*radius)))
			line.Attr("y1", sourcePos.y+float64(float64(dy/distance)*radius))
		}
		{
			radius := radiusOf(targetNode)
			dx := num(sourcePos.x) - num(targetPos.x)
			dy := sourcePos.y - targetPos.y
			distance := math.Sqrt(float64(dx*dx) + float64(dy*dy))
			line.Attr("x2", add(targetPos.x, float64(float64(dx/distance)*radius)))
			line.Attr("y2", targetPos.y+float64(float64(dy/distance)*radius))
		}
		line.
			Attr("stroke", attrValue(theme.linkStroke)).
			Attr("stroke-width", 1)
		if link.Dashed {
			line.Attr("stroke-dasharray", "6 6")
		}
		if link.Flow == "forward" || link.Flow == "bidirectional" {
			line.Attr("marker-end", "url(#link-arrow-end-"+id+")")
		}
		if link.Flow == "backward" || link.Flow == "bidirectional" {
			line.Attr("marker-start", "url(#link-arrow-start-"+id+")")
		}
	}

	// Add link labels
	for _, link := range validLinks {
		if link.Label == "" {
			continue
		}
		sourcePos := positions[link.Source]
		targetPos := positions[link.Target]
		midX := num(add(sourcePos.x, targetPos.x)) / 2
		midY := (sourcePos.y + targetPos.y) / 2
		dx := num(targetPos.x) - num(sourcePos.x)
		dy := targetPos.y - sourcePos.y
		distance := math.Sqrt(float64(dx*dx) + float64(dy*dy))
		const offset = 8.0 // Distance above the line
		// Perpendicular vector pointing "up" relative to line direction
		perpX := dy / distance
		perpY := -dx / distance
		labelX := float64(midX) + float64(perpX*offset)
		labelY := float64(midY) + float64(perpY*offset)
		angle := float64(js.Atan2(dy, dx)*180) / math.Pi
		// Flip text if it would be upside down
		if angle > 90 || angle < -90 {
			angle += 180
		}
		linksGroup.
			Append("text").
			Attr("class", "wardley-link-label").
			Attr("x", labelX).
			Attr("y", labelY).
			Attr("fill", attrValue(theme.axisTextColor)).
			Attr("font-size", attrValue(configValues.labelFontSize)).
			Attr("text-anchor", "middle").
			Attr("dominant-baseline", "middle").
			Attr("transform", "rotate("+js.Str(angle)+" "+js.Str(labelX)+" "+js.Str(labelY)+")").
			Text(link.Label)
	}

	trendGroup := root.Append("g").Attr("class", "wardley-trends")
	type trendWithPositions struct {
		origin     *position
		targetX    any
		targetY    float64
		adjustedX2 any
		adjustedY2 float64
	}
	trendsWithPositions := []trendWithPositions{}
	for _, trend := range data.Trends {
		origin := positions[trend.NodeID]
		if origin == nil {
			continue
		}
		targetX := projectX(trend.TargetX)
		targetY := projectY(trend.TargetY)

		// Calculate the vector from origin to target
		dx := num(targetX) - num(origin.x)
		dy := targetY - origin.y
		distance := math.Sqrt(float64(dx*dx) + float64(dy*dy))

		// Shorten the line by nodeRadius so arrow stops at circle edge
		shortenBy := num(add(configValues.nodeRadius, 2.0)) // +2 for small gap
		adjustedX2 := targetX
		adjustedY2 := targetY
		if distance > shortenBy {
			adjustedX2 = num(targetX) - float64(float64(dx/distance)*shortenBy)
			adjustedY2 = targetY - float64(float64(dy/distance)*shortenBy)
		}

		trendsWithPositions = append(trendsWithPositions, trendWithPositions{
			origin:     origin,
			targetX:    targetX,
			targetY:    targetY,
			adjustedX2: adjustedX2,
			adjustedY2: adjustedY2,
		})
	}

	for _, trend := range trendsWithPositions {
		trendGroup.
			Append("line").
			Attr("class", "wardley-trend").
			Attr("x1", trend.origin.x).
			Attr("y1", trend.origin.y).
			Attr("x2", trend.adjustedX2).
			Attr("y2", trend.adjustedY2).
			Attr("stroke", attrValue(theme.evolutionStroke)).
			Attr("stroke-width", 1).
			Attr("stroke-dasharray", "4 4").
			Attr("marker-end", "url(#arrow-"+id+")")
	}

	nodesGroup := root.Append("g").Attr("class", "wardley-nodes")
	// nodeEnter holds the g element of each node. Upstream adds each kind
	// of shape to all nodes before the next kind.
	nodeEnter := make([]*dom.Node, len(data.Nodes))
	for i, node := range data.Nodes {
		class := "wardley-node"
		if node.ClassName != "" {
			class += " wardley-node--" + node.ClassName
		}
		nodeEnter[i] = nodesGroup.Append("g").Attr("class", class)
	}
	// each calls fn for the nodes that the filter keeps, with the g element
	// and the position of the node.
	each := func(filter func(node *Node) bool, fn func(g *dom.Node, node *Node, pos *position)) {
		for i, node := range data.Nodes {
			if filter(node) {
				fn(nodeEnter[i], node, positions[node.ID])
			}
		}
	}
	overlay := func(strategy, class string, fill, stroke any) {
		each(func(node *Node) bool { return node.SourceStrategy == strategy }, func(g *dom.Node, _ *Node, pos *position) {
			g.Append("circle").
				Attr("class", class).
				Attr("cx", pos.x).
				Attr("cy", pos.y).
				Attr("r", nodeRadius*2).
				Attr("fill", fill).
				Attr("stroke", stroke).
				Attr("stroke-width", 1)
		})
	}

	// Render outsource overlay circles first (larger dark circle for outsourced components - behind main circle)
	overlay("outsource", "wardley-outsource-overlay", "#666", attrValue(theme.componentStroke))

	// Render buy overlay circles (larger light grey circle for bought components - behind main circle)
	overlay("buy", "wardley-buy-overlay", "#ccc", attrValue(theme.componentStroke))

	// Render build overlay circles (larger very light grey circle with black border for built components - behind main circle)
	overlay("build", "wardley-build-overlay", "#eee", "#000")

	// Render market overlay (larger circle with three small circles in triangle pattern)
	isMarket := func(node *Node) bool { return node.SourceStrategy == "market" }

	// Outer circle for market
	overlay("market", "wardley-market-overlay", "white", attrValue(theme.componentStroke))

	// Render circles for normal nodes and pipeline child components (exclude market components and anchors)
	each(func(node *Node) bool {
		return !node.IsPipelineParent && node.SourceStrategy != "market" && node.ClassName != "anchor"
	}, func(g *dom.Node, _ *Node, pos *position) {
		g.Append("circle").
			Attr("cx", pos.x).
			Attr("cy", pos.y).
			Attr("r", attrValue(configValues.nodeRadius)).
			Attr("fill", attrValue(theme.componentFill)).
			Attr("stroke", attrValue(theme.componentStroke)).
			Attr("stroke-width", 1)
	})

	// Render three small circles in triangle pattern for market components (on top of main circle)
	smallCircleRadius := nodeRadius * 0.7
	triangleRadius := float64(nodeRadius * 1.2) // Position so inner circle edges are near outer circle
	// Math.PI / 6 is a division of two float64 values.
	pi := math.Pi
	cos := js.Cos(pi / 6)
	sin := js.Sin(pi / 6)
	// left and right are the x of the two bottom corners of the triangle,
	// and bottom is their y.
	left := func(pos *position) float64 { return num(pos.x) - float64(triangleRadius*cos) }
	right := func(pos *position) any { return add(pos.x, float64(triangleRadius*cos)) }
	bottom := func(pos *position) float64 { return pos.y + float64(triangleRadius*sin) }

	// Draw lines connecting the three circles to form a triangle (render first so circles appear on top)
	// Top to bottom-left
	each(isMarket, func(g *dom.Node, _ *Node, pos *position) {
		g.Append("line").
			Attr("class", "wardley-market-line").
			Attr("x1", pos.x).
			Attr("y1", pos.y-triangleRadius).
			Attr("x2", left(pos)).
			Attr("y2", bottom(pos)).
			Attr("stroke", attrValue(theme.componentStroke)).
			Attr("stroke-width", 1)
	})

	// Line from bottom-left to bottom-right
	each(isMarket, func(g *dom.Node, _ *Node, pos *position) {
		g.Append("line").
			Attr("class", "wardley-market-line").
			Attr("x1", left(pos)).
			Attr("y1", bottom(pos)).
			Attr("x2", right(pos)).
			Attr("y2", bottom(pos)).
			Attr("stroke", attrValue(theme.componentStroke)).
			Attr("stroke-width", 1)
	})

	// Line from bottom-right to top
	each(isMarket, func(g *dom.Node, _ *Node, pos *position) {
		g.Append("line").
			Attr("class", "wardley-market-line").
			Attr("x1", right(pos)).
			Attr("y1", bottom(pos)).
			Attr("x2", pos.x).
			Attr("y2", pos.y-triangleRadius).
			Attr("stroke", attrValue(theme.componentStroke)).
			Attr("stroke-width", 1)
	})

	marketDot := func(cx func(pos *position) any, cy func(pos *position) float64) {
		each(isMarket, func(g *dom.Node, _ *Node, pos *position) {
			g.Append("circle").
				Attr("class", "wardley-market-dot").
				Attr("cx", cx(pos)).
				Attr("cy", cy(pos)).
				Attr("r", smallCircleRadius).
				Attr("fill", "white").
				Attr("stroke", attrValue(theme.componentStroke)).
				Attr("stroke-width", 2)
		})
	}

	// Top circle (white fill so it covers the lines)
	marketDot(func(pos *position) any { return pos.x }, func(pos *position) float64 { return pos.y - triangleRadius })

	// Bottom-left circle (white fill so it covers the lines)
	marketDot(func(pos *position) any { return left(pos) }, bottom)

	// Bottom-right circle (white fill so it covers the lines)
	marketDot(right, bottom)

	// Render squares for pipeline parent nodes
	each(func(node *Node) bool { return node.IsPipelineParent }, func(g *dom.Node, _ *Node, pos *position) {
		g.Append("rect").
			Attr("x", num(pos.x)-float64(squareSize/2)).
			Attr("y", pos.y-float64(squareSize/2)).
			Attr("width", squareSize).
			Attr("height", squareSize).
			Attr("fill", attrValue(theme.componentFill)).
			Attr("stroke", attrValue(theme.componentStroke)).
			Attr("stroke-width", 1)
	})

	// Render inertia indicators (vertical lines to the right of components)
	inertiaX := func(node *Node, pos *position) any {
		var offset any
		if node.IsPipelineParent {
			offset = float64(squareSize/2) + 15
		} else {
			offset = add(configValues.nodeRadius, 15.0)
		}
		// Add extra offset if component has source strategy overlay
		if node.SourceStrategy != "" {
			offset = add(offset, add(configValues.nodeRadius, 10.0))
		}
		return add(pos.x, offset)
	}
	inertiaLineHeight := func(node *Node) float64 {
		if node.IsPipelineParent {
			return squareSize
		}
		return nodeRadius * 2
	}
	each(func(node *Node) bool { return node.HasInertia && node.Inertia }, func(g *dom.Node, node *Node, pos *position) {
		g.Append("line").
			Attr("class", "wardley-inertia").
			Attr("x1", inertiaX(node, pos)).
			Attr("y1", pos.y-float64(inertiaLineHeight(node)/2)).
			Attr("x2", inertiaX(node, pos)).
			Attr("y2", pos.y+float64(inertiaLineHeight(node)/2)).
			Attr("stroke", attrValue(theme.componentStroke)).
			Attr("stroke-width", 6)
	})

	each(func(*Node) bool { return true }, func(g *dom.Node, node *Node, pos *position) {
		isAnchor := node.ClassName == "anchor"

		var x any
		if isAnchor {
			// Anchors have no offset, centered on position
			x = pos.x
			if node.HasLabelOffsetX {
				x = add(pos.x, node.LabelOffsetX)
			}
		} else {
			defaultOffset := configValues.nodeLabelOffset
			// Apply automatic spacing for components with source strategy
			if node.SourceStrategy != "" && !node.HasLabelOffsetX {
				defaultOffset = add(defaultOffset, 10.0)
			}
			customOffset := defaultOffset
			if node.HasLabelOffsetX {
				customOffset = node.LabelOffsetX
			}
			x = add(pos.x, customOffset)
		}

		var y float64
		if isAnchor {
			// Anchors have small upward offset, centered on position
			y = pos.y - 3
			if node.HasLabelOffsetY {
				y = pos.y + node.LabelOffsetY
			}
		} else {
			defaultOffset := -num(configValues.nodeLabelOffset)
			// Apply automatic spacing for components with source strategy
			if node.SourceStrategy != "" && !node.HasLabelOffsetY {
				defaultOffset -= 10
			}
			customOffset := defaultOffset
			if node.HasLabelOffsetY {
				customOffset = node.LabelOffsetY
			}
			y = pos.y + customOffset
		}

		var fill any
		switch node.ClassName {
		case "evolved":
			fill = attrValue(theme.evolutionStroke)
		case "anchor":
			fill = "#000"
		default:
			fill = attrValue(theme.componentLabelColor)
		}
		fontWeight, textAnchor, dominantBaseline := "normal", "start", "auto"
		if isAnchor {
			fontWeight, textAnchor, dominantBaseline = "bold", "middle", "middle"
		}

		g.Append("text").
			Attr("x", x).
			Attr("y", y).
			Attr("class", "wardley-node-label").
			Attr("fill", fill).
			Attr("font-size", attrValue(configValues.labelFontSize)).
			Attr("font-weight", fontWeight).
			Attr("text-anchor", textAnchor).
			Attr("dominant-baseline", dominantBaseline).
			Text(node.Label)
	})

	// Render annotations
	if len(data.Annotations) > 0 {
		annotationsGroup := root.Append("g").Attr("class", "wardley-annotations")

		for _, annotation := range data.Annotations {
			// Project all coordinates
			type projected struct {
				x any
				y float64
			}
			projectedCoords := make([]projected, len(annotation.Coordinates))
			for i, coord := range annotation.Coordinates {
				projectedCoords[i] = projected{x: projectX(coord.X), y: projectY(coord.Y)}
			}

			// If there are multiple coordinates, draw lines connecting them
			if len(projectedCoords) > 1 {
				for i := 0; i < len(projectedCoords)-1; i++ {
					annotationsGroup.
						Append("line").
						Attr("class", "wardley-annotation-line").
						Attr("x1", projectedCoords[i].x).
						Attr("y1", projectedCoords[i].y).
						Attr("x2", projectedCoords[i+1].x).
						Attr("y2", projectedCoords[i+1].y).
						Attr("stroke", attrValue(theme.axisColor)).
						Attr("stroke-width", 1.5).
						Attr("stroke-dasharray", "4 4")
				}
			}

			// Draw circles and numbers at each coordinate
			for _, coord := range projectedCoords {
				annotationNode := annotationsGroup.Append("g").Attr("class", "wardley-annotation")

				// Draw circle
				annotationNode.
					Append("circle").
					Attr("cx", coord.x).
					Attr("cy", coord.y).
					Attr("r", 10).
					Attr("fill", "white").
					Attr("stroke", attrValue(theme.axisColor)).
					Attr("stroke-width", 1.5)

				// Draw number
				annotationNode.
					Append("text").
					Attr("x", coord.x).
					Attr("y", coord.y).
					Attr("text-anchor", "middle").
					Attr("dominant-baseline", "central").
					Attr("font-size", 10).
					Attr("fill", attrValue(theme.axisTextColor)).
					Attr("font-weight", "bold").
					Text(js.Str(annotation.Number))
			}
		}

		// Draw annotations text box if position is defined
		if data.AnnotationsBox != nil {
			rawBoxX := projectX(data.AnnotationsBox.X)
			boxY := projectY(data.AnnotationsBox.Y)
			const padding = 10.0
			const lineHeight = 16.0
			const fontSize = 11.0

			// Create text box group
			textBoxGroup := annotationsGroup.Append("g").Attr("class", "wardley-annotations-box")

			// Sort annotations by number
			sortedAnnotations := []AnnotationData{}
			for _, a := range data.Annotations {
				if a.Text != "" {
					sortedAnnotations = append(sortedAnnotations, a)
				}
			}
			js.SortFunc(sortedAnnotations, func(a, b AnnotationData) float64 { return a.Number - b.Number })

			// Draw text lines (temporarily to measure)
			textElements := []*dom.Node{}
			for idx, annotation := range sortedAnnotations {
				text := textBoxGroup.
					Append("text").
					Attr("x", add(rawBoxX, padding)).
					Attr("y", boxY+padding+float64(float64(idx+1)*lineHeight)).
					Attr("font-size", fontSize).
					Attr("fill", attrValue(theme.axisTextColor)).
					Attr("text-anchor", "start").
					Attr("dominant-baseline", "middle").
					Text(js.Str(annotation.Number) + ". " + annotation.Text)
				textElements = append(textElements, text)
			}

			// Calculate box dimensions based on text
			if len(textElements) > 0 {
				maxWidth := 0.0
				maxHeight := 0.0
				for _, text := range textElements {
					// Use getComputedTextLength for more accurate width measurement
					textWidth := text.ComputedTextLength()
					maxWidth = js.Max(maxWidth, textWidth)
					bbox := text.BBox()
					maxHeight = js.Max(maxHeight, bbox.Height)
				}

				// Add extra buffer to padding to ensure text fits comfortably
				boxWidth := maxWidth + padding*2 + 105 // Extra 105px buffer for safety
				boxHeight := float64(float64(len(sortedAnnotations))*lineHeight) + padding*2 + float64(maxHeight/2)

				// Constrain box to stay within map boundaries
				minX := num(configValues.padding)
				maxX := width - num(configValues.padding) - boxWidth
				minY := num(configValues.padding)
				maxY := height - num(configValues.padding) - boxHeight

				boxX := js.Max(minX, js.Min(num(rawBoxX), maxX))
				boxY = js.Max(minY, js.Min(boxY, maxY))

				// Update text positions after clamping
				for idx, text := range textElements {
					text.Attr("x", boxX+padding).Attr("y", boxY+padding+float64(float64(idx+1)*lineHeight))
				}

				// Draw box background (insert before text)
				textBoxGroup.
					Insert("rect", "text").
					Attr("x", boxX).
					Attr("y", boxY).
					Attr("width", boxWidth).
					Attr("height", boxHeight).
					Attr("fill", "white").
					Attr("stroke", attrValue(theme.axisColor)).
					Attr("stroke-width", 1.5).
					Attr("rx", 4).
					Attr("ry", 4)
			}
		}
	}

	// Render notes
	if len(data.Notes) > 0 {
		notesGroup := root.Append("g").Attr("class", "wardley-notes")

		for _, note := range data.Notes {
			noteX := projectX(note.X)
			noteY := projectY(note.Y)

			notesGroup.
				Append("text").
				Attr("x", noteX).
				Attr("y", noteY).
				Attr("text-anchor", "start").
				Attr("font-size", 11).
				Attr("fill", attrValue(theme.axisTextColor)).
				Attr("font-weight", "bold").
				Text(note.Text)
		}
	}

	// Arrow dimensions
	const (
		arrowWidth     = 60.0
		arrowHeight    = 30.0
		arrowHeadWidth = 20.0
	)

	// Render accelerators (large right-pointing arrows)
	if len(data.Accelerators) > 0 {
		acceleratorsGroup := root.Append("g").Attr("class", "wardley-accelerators")

		for _, accelerator := range data.Accelerators {
			accX := projectX(accelerator.X)
			accY := projectY(accelerator.Y)

			// shaft is accX + arrowWidth - arrowHeadWidth, and tip is
			// accX + arrowWidth.
			tip := add(accX, arrowWidth)
			shaft := js.Str(num(tip) - arrowHeadWidth)

			// Draw arrow shape: shaft + arrowhead pointing right
			arrowPath := strings.Join([]string{
				"",
				"        M " + str(accX) + " " + js.Str(accY-arrowHeight/2),
				"        L " + shaft + " " + js.Str(accY-arrowHeight/2),
				"        L " + shaft + " " + js.Str(accY-arrowHeight/2-8),
				"        L " + str(tip) + " " + js.Str(accY),
				"        L " + shaft + " " + js.Str(accY+arrowHeight/2+8),
				"        L " + shaft + " " + js.Str(accY+arrowHeight/2),
				"        L " + str(accX) + " " + js.Str(accY+arrowHeight/2),
				"        Z",
				"      ",
			}, "\n")

			acceleratorsGroup.
				Append("path").
				Attr("d", arrowPath).
				Attr("fill", "white").
				Attr("stroke", attrValue(theme.componentStroke)).
				Attr("stroke-width", 1)

			// Add label below the arrow
			acceleratorsGroup.
				Append("text").
				Attr("x", add(accX, arrowWidth/2)).
				Attr("y", accY+arrowHeight/2+15).
				Attr("text-anchor", "middle").
				Attr("font-size", 10).
				Attr("fill", attrValue(theme.axisTextColor)).
				Attr("font-weight", "bold").
				Text(accelerator.Name)
		}
	}

	// Render deaccelerators (large left-pointing arrows)
	if len(data.Deaccelerators) > 0 {
		deacceleratorsGroup := root.Append("g").Attr("class", "wardley-deaccelerators")

		for _, deaccelerator := range data.Deaccelerators {
			decX := projectX(deaccelerator.X)
			decY := projectY(deaccelerator.Y)

			// tail is decX + arrowWidth, and head is decX + arrowHeadWidth.
			tail := str(add(decX, arrowWidth))
			head := str(add(decX, arrowHeadWidth))

			// Draw arrow shape pointing left: shaft + arrowhead pointing left
			arrowPath := strings.Join([]string{
				"",
				"        M " + tail + " " + js.Str(decY-arrowHeight/2),
				"        L " + head + " " + js.Str(decY-arrowHeight/2),
				"        L " + head + " " + js.Str(decY-arrowHeight/2-8),
				"        L " + str(decX) + " " + js.Str(decY),
				"        L " + head + " " + js.Str(decY+arrowHeight/2+8),
				"        L " + head + " " + js.Str(decY+arrowHeight/2),
				"        L " + tail + " " + js.Str(decY+arrowHeight/2),
				"        Z",
				"      ",
			}, "\n")

			deacceleratorsGroup.
				Append("path").
				Attr("d", arrowPath).
				Attr("fill", "white").
				Attr("stroke", attrValue(theme.componentStroke)).
				Attr("stroke-width", 1)

			// Add label below the arrow
			deacceleratorsGroup.
				Append("text").
				Attr("x", add(decX, arrowWidth/2)).
				Attr("y", decY+arrowHeight/2+15).
				Attr("text-anchor", "middle").
				Attr("font-size", 10).
				Attr("fill", attrValue(theme.axisTextColor)).
				Attr("font-weight", "bold").
				Text(deaccelerator.Name)
		}
	}
	return nil
}
