package mermaid

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

func ComputeLayout(graph *Graph, theme Theme, config LayoutConfig) Layout {
	switch graph.Kind {
	case DiagramFlowchart, DiagramClass, DiagramState, DiagramER, DiagramRequirement,
		DiagramC4, DiagramSankey, DiagramZenUML, DiagramBlock, DiagramPacket,
		DiagramKanban, DiagramArchitecture, DiagramRadar, DiagramTreemap:
		return layoutGraphLike(graph, theme, config)
	case DiagramSequence:
		return layoutSequence(graph, theme, config)
	case DiagramPie:
		return layoutPie(graph, theme)
	case DiagramGantt:
		return layoutGantt(graph, theme)
	case DiagramTimeline:
		return layoutTimeline(graph, theme)
	case DiagramJourney:
		return layoutJourney(graph, theme)
	case DiagramMindmap:
		return layoutMindmap(graph, theme)
	case DiagramGitGraph:
		return layoutGitGraph(graph, theme)
	case DiagramXYChart:
		return layoutXYChart(graph, theme)
	case DiagramQuadrant:
		return layoutQuadrant(graph, theme)
	default:
		return layoutGeneric(graph, theme)
	}
}

func layoutGraphLike(graph *Graph, theme Theme, config LayoutConfig) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.NodeOrder) == 0 {
		return layoutGeneric(graph, theme)
	}

	ranks := map[string]int{}
	for _, id := range graph.NodeOrder {
		ranks[id] = 0
	}
	for i := 0; i < len(graph.NodeOrder)+1; i++ {
		updated := false
		for _, edge := range graph.Edges {
			fromRank, okFrom := ranks[edge.From]
			toRank, okTo := ranks[edge.To]
			if !okFrom {
				ranks[edge.From] = 0
				fromRank = 0
			}
			if !okTo {
				ranks[edge.To] = 0
				toRank = 0
			}
			if toRank <= fromRank {
				ranks[edge.To] = fromRank + 1
				updated = true
			}
		}
		if !updated {
			break
		}
	}

	maxRank := 0
	for _, rank := range ranks {
		if rank > maxRank {
			maxRank = rank
		}
	}

	orderedRanks := make(map[int][]string)
	for _, id := range graph.NodeOrder {
		rank := ranks[id]
		if graph.Direction == DirectionBottomTop || graph.Direction == DirectionRightLeft {
			rank = maxRank - rank
		}
		orderedRanks[rank] = append(orderedRanks[rank], id)
	}

	padding := 40.0
	nodeSpacing := max(20, config.NodeSpacing)
	rankSpacing := max(40, config.RankSpacing)
	baseHeight := 56.0
	maxNodeWidth := 100.0
	nodeSizes := map[string]Point{}

	for _, id := range graph.NodeOrder {
		node := graph.Nodes[id]
		w := clamp(measureTextWidth(node.Label, config.FastTextMetrics)+28, 80, 320)
		nodeSizes[id] = Point{X: w, Y: baseHeight}
		if w > maxNodeWidth {
			maxNodeWidth = w
		}
	}

	for rank := 0; rank <= maxRank; rank++ {
		nodes := orderedRanks[rank]
		for index, id := range nodes {
			size := nodeSizes[id]
			x := padding + float64(index)*(maxNodeWidth+nodeSpacing)
			y := padding + float64(rank)*(baseHeight+rankSpacing)
			if graph.Direction == DirectionLeftRight || graph.Direction == DirectionRightLeft {
				x, y = y, x
			}
			layout.Nodes = append(layout.Nodes, NodeLayout{
				ID:    id,
				Label: graph.Nodes[id].Label,
				Shape: graph.Nodes[id].Shape,
				X:     x,
				Y:     y,
				W:     size.X,
				H:     size.Y,
			})
		}
	}

	nodeIndex := map[string]NodeLayout{}
	for _, node := range layout.Nodes {
		nodeIndex[node.ID] = node
	}

	maxX := 0.0
	maxY := 0.0
	for _, node := range layout.Nodes {
		if node.X+node.W > maxX {
			maxX = node.X + node.W
		}
		if node.Y+node.H > maxY {
			maxY = node.Y + node.H
		}
	}

	for _, edge := range graph.Edges {
		from, okFrom := nodeIndex[edge.From]
		to, okTo := nodeIndex[edge.To]
		if !okFrom || !okTo {
			continue
		}
		x1, y1, x2, y2 := edgeEndpoints(from, to, graph.Direction)
		layout.Edges = append(layout.Edges, EdgeLayout{
			From:       edge.From,
			To:         edge.To,
			Label:      edge.Label,
			X1:         x1,
			Y1:         y1,
			X2:         x2,
			Y2:         y2,
			Style:      edge.Style,
			ArrowStart: edge.ArrowStart,
			ArrowEnd:   edge.ArrowEnd || edge.Directed,
		})
	}

	layout.Width = maxX + padding
	layout.Height = maxY + padding
	applyAspectRatio(&layout, config.PreferredAspectRatio)
	addGraphPrimitives(&layout, theme)
	return layout
}

func edgeEndpoints(from, to NodeLayout, direction Direction) (x1, y1, x2, y2 float64) {
	switch direction {
	case DirectionLeftRight:
		return from.X + from.W, from.Y + from.H/2, to.X, to.Y + to.H/2
	case DirectionRightLeft:
		return from.X, from.Y + from.H/2, to.X + to.W, to.Y + to.H/2
	case DirectionBottomTop:
		return from.X + from.W/2, from.Y, to.X + to.W/2, to.Y + to.H
	default:
		return from.X + from.W/2, from.Y + from.H, to.X + to.W/2, to.Y
	}
}

func addGraphPrimitives(layout *Layout, theme Theme) {
	for _, edge := range layout.Edges {
		strokeWidth := 2.0
		dashed := false
		if edge.Style == EdgeDotted {
			dashed = true
		}
		if edge.Style == EdgeThick {
			strokeWidth = 3
		}
		layout.Lines = append(layout.Lines, LayoutLine{
			X1:          edge.X1,
			Y1:          edge.Y1,
			X2:          edge.X2,
			Y2:          edge.Y2,
			Stroke:      theme.LineColor,
			StrokeWidth: strokeWidth,
			Dashed:      dashed,
			ArrowStart:  edge.ArrowStart,
			ArrowEnd:    edge.ArrowEnd,
		})
		if edge.Label != "" {
			layout.Texts = append(layout.Texts, LayoutText{
				X:      (edge.X1 + edge.X2) / 2,
				Y:      (edge.Y1+edge.Y2)/2 - 6,
				Value:  edge.Label,
				Anchor: "middle",
				Size:   max(11, theme.FontSize-1),
				Color:  theme.PrimaryTextColor,
			})
		}
	}

	for _, node := range layout.Nodes {
		addNodePrimitive(layout, theme, node)
		layout.Texts = append(layout.Texts, LayoutText{
			X:      node.X + node.W/2,
			Y:      node.Y + node.H/2 + theme.FontSize*0.35,
			Value:  node.Label,
			Anchor: "middle",
			Size:   theme.FontSize,
			Color:  theme.PrimaryTextColor,
		})
	}
}

func addNodePrimitive(layout *Layout, theme Theme, node NodeLayout) {
	fill := theme.PrimaryColor
	stroke := theme.PrimaryBorderColor
	switch node.Shape {
	case ShapeRoundRect, ShapeStadium:
		layout.Rects = append(layout.Rects, LayoutRect{
			X:           node.X,
			Y:           node.Y,
			W:           node.W,
			H:           node.H,
			RX:          14,
			RY:          14,
			Fill:        fill,
			Stroke:      stroke,
			StrokeWidth: 1.8,
		})
	case ShapeCircle:
		layout.Circles = append(layout.Circles, LayoutCircle{
			CX:          node.X + node.W/2,
			CY:          node.Y + node.H/2,
			R:           min(node.W, node.H) / 2,
			Fill:        fill,
			Stroke:      stroke,
			StrokeWidth: 1.8,
		})
	case ShapeDoubleCircle:
		r := min(node.W, node.H) / 2
		layout.Circles = append(layout.Circles,
			LayoutCircle{
				CX:          node.X + node.W/2,
				CY:          node.Y + node.H/2,
				R:           r,
				Fill:        fill,
				Stroke:      stroke,
				StrokeWidth: 1.8,
			},
			LayoutCircle{
				CX:          node.X + node.W/2,
				CY:          node.Y + node.H/2,
				R:           max(1, r-6),
				Fill:        "none",
				Stroke:      stroke,
				StrokeWidth: 1.5,
			},
		)
	case ShapeDiamond:
		layout.Polygons = append(layout.Polygons, LayoutPolygon{
			Points: []Point{
				{X: node.X + node.W/2, Y: node.Y},
				{X: node.X + node.W, Y: node.Y + node.H/2},
				{X: node.X + node.W/2, Y: node.Y + node.H},
				{X: node.X, Y: node.Y + node.H/2},
			},
			Fill:        fill,
			Stroke:      stroke,
			StrokeWidth: 1.8,
		})
	case ShapeHexagon:
		layout.Polygons = append(layout.Polygons, LayoutPolygon{
			Points: []Point{
				{X: node.X + node.W*0.2, Y: node.Y},
				{X: node.X + node.W*0.8, Y: node.Y},
				{X: node.X + node.W, Y: node.Y + node.H/2},
				{X: node.X + node.W*0.8, Y: node.Y + node.H},
				{X: node.X + node.W*0.2, Y: node.Y + node.H},
				{X: node.X, Y: node.Y + node.H/2},
			},
			Fill:        fill,
			Stroke:      stroke,
			StrokeWidth: 1.8,
		})
	case ShapeParallelogram:
		layout.Polygons = append(layout.Polygons, LayoutPolygon{
			Points: []Point{
				{X: node.X + 14, Y: node.Y},
				{X: node.X + node.W, Y: node.Y},
				{X: node.X + node.W - 14, Y: node.Y + node.H},
				{X: node.X, Y: node.Y + node.H},
			},
			Fill:        fill,
			Stroke:      stroke,
			StrokeWidth: 1.8,
		})
	case ShapeTrapezoid:
		layout.Polygons = append(layout.Polygons, LayoutPolygon{
			Points: []Point{
				{X: node.X + 16, Y: node.Y},
				{X: node.X + node.W - 16, Y: node.Y},
				{X: node.X + node.W, Y: node.Y + node.H},
				{X: node.X, Y: node.Y + node.H},
			},
			Fill:        fill,
			Stroke:      stroke,
			StrokeWidth: 1.8,
		})
	case ShapeAsymmetric:
		layout.Polygons = append(layout.Polygons, LayoutPolygon{
			Points: []Point{
				{X: node.X, Y: node.Y},
				{X: node.X + node.W, Y: node.Y},
				{X: node.X + node.W*0.85, Y: node.Y + node.H},
				{X: node.X, Y: node.Y + node.H},
			},
			Fill:        fill,
			Stroke:      stroke,
			StrokeWidth: 1.8,
		})
	default:
		layout.Rects = append(layout.Rects, LayoutRect{
			X:           node.X,
			Y:           node.Y,
			W:           node.W,
			H:           node.H,
			RX:          6,
			RY:          6,
			Fill:        fill,
			Stroke:      stroke,
			StrokeWidth: 1.8,
		})
	}
}

func layoutSequence(graph *Graph, theme Theme, _ LayoutConfig) Layout {
	layout := Layout{Kind: graph.Kind}
	participants := graph.SequenceParticipants
	if len(participants) == 0 {
		for _, id := range graph.NodeOrder {
			participants = append(participants, graph.Nodes[id].Label)
		}
	}
	if len(participants) == 0 {
		return layoutGeneric(graph, theme)
	}

	padding := 60.0
	boxW := 130.0
	boxH := 36.0
	participantSpacing := 170.0
	topY := 40.0
	msgStart := 120.0
	msgStep := 56.0

	xPos := map[string]float64{}
	for i, participant := range participants {
		xPos[participant] = padding + float64(i)*participantSpacing
		x := xPos[participant]
		layout.Rects = append(layout.Rects, LayoutRect{
			X:           x - boxW/2,
			Y:           topY,
			W:           boxW,
			H:           boxH,
			RX:          6,
			RY:          6,
			Fill:        theme.SecondaryColor,
			Stroke:      theme.PrimaryBorderColor,
			StrokeWidth: 1.8,
		})
		layout.Texts = append(layout.Texts, LayoutText{
			X:      x,
			Y:      topY + boxH/2 + theme.FontSize*0.35,
			Value:  participant,
			Anchor: "middle",
			Size:   theme.FontSize,
			Color:  theme.PrimaryTextColor,
		})
	}

	contentHeight := msgStart + float64(max(1, len(graph.SequenceMessages)))*msgStep
	for _, participant := range participants {
		x := xPos[participant]
		layout.Lines = append(layout.Lines, LayoutLine{
			X1:          x,
			Y1:          topY + boxH,
			X2:          x,
			Y2:          contentHeight,
			Stroke:      theme.LineColor,
			StrokeWidth: 1.3,
			Dashed:      true,
		})
	}

	for i, msg := range graph.SequenceMessages {
		y := msgStart + float64(i)*msgStep
		fromX, okFrom := xPos[msg.From]
		toX, okTo := xPos[msg.To]
		if !okFrom || !okTo {
			continue
		}
		style := edgeStyleFromArrow(msg.Arrow)
		line := LayoutLine{
			X1:          fromX,
			Y1:          y,
			X2:          toX,
			Y2:          y,
			Stroke:      theme.LineColor,
			StrokeWidth: 2,
			ArrowEnd:    strings.Contains(msg.Arrow, ">"),
			Dashed:      style == EdgeDotted,
		}
		if style == EdgeThick {
			line.StrokeWidth = 3
		}
		layout.Lines = append(layout.Lines, line)
		layout.Texts = append(layout.Texts, LayoutText{
			X:      (fromX + toX) / 2,
			Y:      y - 8,
			Value:  msg.Label,
			Anchor: "middle",
			Size:   max(11, theme.FontSize-1),
			Color:  theme.PrimaryTextColor,
		})
	}

	layout.Width = padding*2 + float64(len(participants)-1)*participantSpacing
	layout.Height = contentHeight + 50
	return layout
}

func layoutPie(graph *Graph, theme Theme) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.PieSlices) == 0 {
		return layoutGeneric(graph, theme)
	}

	layout.Width = 860
	layout.Height = 560
	cx := 300.0
	cy := 290.0
	r := 170.0
	total := 0.0
	for _, slice := range graph.PieSlices {
		total += math.Max(slice.Value, 0)
	}
	if total <= 0 {
		total = 1
	}

	palette := []string{
		"#4e79a7", "#f28e2c", "#e15759", "#76b7b2", "#59a14f",
		"#edc948", "#b07aa1", "#ff9da7", "#9c755f", "#bab0ab",
	}
	angle := -math.Pi / 2
	for i, slice := range graph.PieSlices {
		fraction := math.Max(slice.Value, 0) / total
		next := angle + fraction*2*math.Pi
		path := arcPath(cx, cy, r, angle, next)
		layout.Paths = append(layout.Paths, LayoutPath{
			D:           path,
			Fill:        palette[i%len(palette)],
			Stroke:      "#ffffff",
			StrokeWidth: 1.5,
		})

		mid := (angle + next) / 2
		lx := cx + math.Cos(mid)*(r+20)
		ly := cy + math.Sin(mid)*(r+20)
		label := slice.Label
		if graph.PieShowData {
			label = label + " (" + formatFloat(slice.Value) + ")"
		}
		layout.Texts = append(layout.Texts, LayoutText{
			X:      lx,
			Y:      ly,
			Value:  label,
			Anchor: "middle",
			Size:   max(11, theme.FontSize-1),
			Color:  theme.PrimaryTextColor,
		})

		angle = next
	}

	title := graph.PieTitle
	if title == "" {
		title = "Pie Chart"
	}
	layout.Texts = append(layout.Texts, LayoutText{
		X:      cx,
		Y:      48,
		Value:  title,
		Anchor: "middle",
		Size:   theme.FontSize + 4,
		Weight: "600",
		Color:  theme.PrimaryTextColor,
	})
	return layout
}

func arcPath(cx, cy, r, start, end float64) string {
	x1 := cx + r*math.Cos(start)
	y1 := cy + r*math.Sin(start)
	x2 := cx + r*math.Cos(end)
	y2 := cy + r*math.Sin(end)
	largeArc := 0
	if end-start > math.Pi {
		largeArc = 1
	}
	return "M " + formatFloat(cx) + " " + formatFloat(cy) +
		" L " + formatFloat(x1) + " " + formatFloat(y1) +
		" A " + formatFloat(r) + " " + formatFloat(r) + " 0 " + intString(largeArc) + " 1 " +
		formatFloat(x2) + " " + formatFloat(y2) + " Z"
}

func layoutGantt(graph *Graph, theme Theme) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.GanttTasks) == 0 {
		return layoutGeneric(graph, theme)
	}
	left := 220.0
	top := 90.0
	rowH := 36.0
	layout.Width = 980
	layout.Height = top + float64(len(graph.GanttTasks))*rowH + 80

	title := graph.GanttTitle
	if title == "" {
		title = "Gantt"
	}
	layout.Texts = append(layout.Texts, LayoutText{
		X:      24,
		Y:      42,
		Value:  title,
		Anchor: "start",
		Size:   theme.FontSize + 4,
		Weight: "600",
		Color:  theme.PrimaryTextColor,
	})

	for i, task := range graph.GanttTasks {
		y := top + float64(i)*rowH
		w := ganttDurationWidth(task.Duration)
		fill := theme.SecondaryColor
		switch task.Status {
		case "done":
			fill = "#b8e1c6"
		case "active":
			fill = "#9fd3ff"
		case "crit":
			fill = "#ffb3b3"
		case "milestone":
			fill = "#ffd8a8"
		}
		layout.Rects = append(layout.Rects, LayoutRect{
			X:           left,
			Y:           y,
			W:           w,
			H:           rowH - 8,
			RX:          4,
			RY:          4,
			Fill:        fill,
			Stroke:      theme.PrimaryBorderColor,
			StrokeWidth: 1.3,
		})
		layout.Texts = append(layout.Texts,
			LayoutText{
				X:      24,
				Y:      y + rowH*0.65,
				Value:  task.Label,
				Anchor: "start",
				Size:   theme.FontSize,
				Color:  theme.PrimaryTextColor,
			},
			LayoutText{
				X:      left + 8,
				Y:      y + rowH*0.6,
				Value:  task.ID,
				Anchor: "start",
				Size:   max(10, theme.FontSize-2),
				Color:  theme.PrimaryTextColor,
			},
		)
	}
	return layout
}

func ganttDurationWidth(raw string) float64 {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 120
	}
	if strings.HasSuffix(trimmed, "d") || strings.HasSuffix(trimmed, "w") || strings.HasSuffix(trimmed, "m") {
		value, ok := parseFloat(trimmed[:len(trimmed)-1])
		if ok {
			return clamp(value*26, 60, 460)
		}
	}
	return clamp(measureTextWidth(trimmed, true)*4, 80, 300)
}

func layoutTimeline(graph *Graph, theme Theme) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.TimelineEvents) == 0 {
		return layoutGeneric(graph, theme)
	}
	padding := 80.0
	step := 170.0
	baselineY := 250.0
	layout.Width = padding*2 + float64(len(graph.TimelineEvents)-1)*step
	layout.Height = 460

	layout.Lines = append(layout.Lines, LayoutLine{
		X1:          padding,
		Y1:          baselineY,
		X2:          layout.Width - padding,
		Y2:          baselineY,
		Stroke:      theme.LineColor,
		StrokeWidth: 2,
	})

	title := graph.TimelineTitle
	if title == "" {
		title = "Timeline"
	}
	layout.Texts = append(layout.Texts, LayoutText{
		X:      padding,
		Y:      46,
		Value:  title,
		Anchor: "start",
		Size:   theme.FontSize + 4,
		Weight: "600",
		Color:  theme.PrimaryTextColor,
	})

	for i, event := range graph.TimelineEvents {
		x := padding + float64(i)*step
		layout.Circles = append(layout.Circles, LayoutCircle{
			CX:          x,
			CY:          baselineY,
			R:           8,
			Fill:        theme.PrimaryBorderColor,
			Stroke:      theme.PrimaryBorderColor,
			StrokeWidth: 1,
		})
		layout.Texts = append(layout.Texts,
			LayoutText{
				X:      x,
				Y:      baselineY - 18,
				Value:  event.Time,
				Anchor: "middle",
				Size:   theme.FontSize,
				Weight: "600",
				Color:  theme.PrimaryTextColor,
			},
			LayoutText{
				X:      x,
				Y:      baselineY + 28,
				Value:  strings.Join(event.Events, "; "),
				Anchor: "middle",
				Size:   max(11, theme.FontSize-1),
				Color:  theme.PrimaryTextColor,
			},
		)
	}
	return layout
}

func layoutJourney(graph *Graph, theme Theme) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.JourneySteps) == 0 {
		return layoutGeneric(graph, theme)
	}
	padding := 80.0
	stepX := 160.0
	baseY := 220.0
	layout.Width = padding*2 + float64(len(graph.JourneySteps)-1)*stepX
	layout.Height = 420

	title := graph.JourneyTitle
	if title == "" {
		title = "Journey"
	}
	layout.Texts = append(layout.Texts, LayoutText{
		X:      padding,
		Y:      44,
		Value:  title,
		Anchor: "start",
		Size:   theme.FontSize + 4,
		Weight: "600",
		Color:  theme.PrimaryTextColor,
	})

	prevX := 0.0
	prevY := 0.0
	for i, step := range graph.JourneySteps {
		x := padding + float64(i)*stepX
		score := clamp(step.Score, 0, 5)
		y := baseY - score*30
		if i > 0 {
			layout.Lines = append(layout.Lines, LayoutLine{
				X1:          prevX,
				Y1:          prevY,
				X2:          x,
				Y2:          y,
				Stroke:      theme.LineColor,
				StrokeWidth: 2.2,
				ArrowEnd:    true,
			})
		}
		layout.Circles = append(layout.Circles, LayoutCircle{
			CX:          x,
			CY:          y,
			R:           10,
			Fill:        theme.TertiaryColor,
			Stroke:      theme.PrimaryBorderColor,
			StrokeWidth: 1.5,
		})
		layout.Texts = append(layout.Texts,
			LayoutText{
				X:      x,
				Y:      y - 14,
				Value:  step.Label,
				Anchor: "middle",
				Size:   max(11, theme.FontSize-1),
				Color:  theme.PrimaryTextColor,
			},
			LayoutText{
				X:      x,
				Y:      y + 28,
				Value:  "score " + formatFloat(step.Score),
				Anchor: "middle",
				Size:   max(10, theme.FontSize-2),
				Color:  theme.PrimaryTextColor,
			},
		)
		prevX, prevY = x, y
	}
	return layout
}

func layoutMindmap(graph *Graph, theme Theme) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.MindmapNodes) == 0 {
		return layoutGeneric(graph, theme)
	}
	padding := 40.0
	levelSpacing := 220.0
	rowSpacing := 90.0

	nodesByLevel := map[int][]MindmapNode{}
	maxLevel := 0
	for _, node := range graph.MindmapNodes {
		nodesByLevel[node.Level] = append(nodesByLevel[node.Level], node)
		if node.Level > maxLevel {
			maxLevel = node.Level
		}
	}

	nodePos := map[string]Point{}
	nodeSize := map[string]Point{}
	for level := 0; level <= maxLevel; level++ {
		nodes := nodesByLevel[level]
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
		for i, node := range nodes {
			x := padding + float64(level)*levelSpacing
			y := padding + float64(i)*rowSpacing + 60
			w := clamp(measureTextWidth(node.Label, true)+26, 90, 260)
			h := 46.0
			nodePos[node.ID] = Point{X: x, Y: y}
			nodeSize[node.ID] = Point{X: w, Y: h}
			layout.Nodes = append(layout.Nodes, NodeLayout{
				ID:    node.ID,
				Label: node.Label,
				Shape: ShapeRoundRect,
				X:     x,
				Y:     y,
				W:     w,
				H:     h,
			})
		}
	}

	for _, node := range graph.MindmapNodes {
		if node.Parent == "" {
			continue
		}
		from := nodePos[node.Parent]
		fromSize := nodeSize[node.Parent]
		to := nodePos[node.ID]
		layout.Lines = append(layout.Lines, LayoutLine{
			X1:          from.X + fromSize.X,
			Y1:          from.Y + fromSize.Y/2,
			X2:          to.X,
			Y2:          to.Y + nodeSize[node.ID].Y/2,
			Stroke:      theme.LineColor,
			StrokeWidth: 2,
			ArrowEnd:    false,
		})
	}

	for _, node := range layout.Nodes {
		addNodePrimitive(&layout, theme, node)
		layout.Texts = append(layout.Texts, LayoutText{
			X:      node.X + node.W/2,
			Y:      node.Y + node.H/2 + theme.FontSize*0.35,
			Value:  node.Label,
			Anchor: "middle",
			Size:   theme.FontSize,
			Color:  theme.PrimaryTextColor,
		})
	}

	maxX := 0.0
	maxY := 0.0
	for _, node := range layout.Nodes {
		maxX = max(maxX, node.X+node.W)
		maxY = max(maxY, node.Y+node.H)
	}
	layout.Width = maxX + padding
	layout.Height = maxY + padding
	return layout
}

func layoutGitGraph(graph *Graph, theme Theme) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.GitCommits) == 0 {
		return layoutGeneric(graph, theme)
	}

	branches := append([]string(nil), graph.GitBranches...)
	if len(branches) == 0 {
		branches = []string{graph.GitMainBranch}
	}
	sort.Strings(branches)
	branchLane := map[string]int{}
	for i, branch := range branches {
		branchLane[branch] = i
	}

	padding := 60.0
	stepX := 120.0
	laneH := 80.0

	for i, commit := range graph.GitCommits {
		x := padding + float64(i)*stepX
		y := padding + float64(branchLane[commit.Branch])*laneH
		layout.Circles = append(layout.Circles, LayoutCircle{
			CX:          x,
			CY:          y,
			R:           10,
			Fill:        theme.PrimaryBorderColor,
			Stroke:      theme.PrimaryBorderColor,
			StrokeWidth: 1.5,
		})
		layout.Texts = append(layout.Texts, LayoutText{
			X:      x + 14,
			Y:      y - 10,
			Value:  commit.Label,
			Anchor: "start",
			Size:   max(10, theme.FontSize-2),
			Color:  theme.PrimaryTextColor,
		})
		if i > 0 {
			prevX := padding + float64(i-1)*stepX
			prevY := padding + float64(branchLane[graph.GitCommits[i-1].Branch])*laneH
			layout.Lines = append(layout.Lines, LayoutLine{
				X1:          prevX,
				Y1:          prevY,
				X2:          x,
				Y2:          y,
				Stroke:      theme.LineColor,
				StrokeWidth: 2,
				ArrowEnd:    true,
			})
		}
	}

	layout.Width = padding*2 + float64(len(graph.GitCommits))*stepX
	layout.Height = padding*2 + float64(max(1, len(branches)-1))*laneH + 80
	return layout
}

func layoutXYChart(graph *Graph, theme Theme) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.XYSeries) == 0 {
		return layoutGeneric(graph, theme)
	}

	width := 920.0
	height := 560.0
	left := 80.0
	right := width - 60
	top := 80.0
	bottom := height - 80
	layout.Width = width
	layout.Height = height

	layout.Lines = append(layout.Lines,
		LayoutLine{X1: left, Y1: bottom, X2: right, Y2: bottom, Stroke: theme.LineColor, StrokeWidth: 2},
		LayoutLine{X1: left, Y1: top, X2: left, Y2: bottom, Stroke: theme.LineColor, StrokeWidth: 2},
	)

	title := graph.XYTitle
	if title == "" {
		title = "XY Chart"
	}
	layout.Texts = append(layout.Texts, LayoutText{
		X:      left,
		Y:      42,
		Value:  title,
		Anchor: "start",
		Size:   theme.FontSize + 4,
		Weight: "600",
		Color:  theme.PrimaryTextColor,
	})

	maxLen := 0
	maxValue := 1.0
	minValue := 0.0
	if graph.XYYMin != nil {
		minValue = *graph.XYYMin
	}
	if graph.XYYMax != nil {
		maxValue = *graph.XYYMax
	}
	for _, series := range graph.XYSeries {
		if len(series.Values) > maxLen {
			maxLen = len(series.Values)
		}
		for _, v := range series.Values {
			if graph.XYYMax == nil && v > maxValue {
				maxValue = v
			}
			if graph.XYYMin == nil && v < minValue {
				minValue = v
			}
		}
	}
	if maxLen == 0 {
		maxLen = 1
	}
	span := max(1, maxValue-minValue)
	slot := (right - left) / float64(maxLen)

	for i, series := range graph.XYSeries {
		color := seriesColor(i)
		switch series.Kind {
		case XYSeriesLine:
			points := make([]Point, 0, len(series.Values))
			for idx, value := range series.Values {
				x := left + float64(idx)*slot + slot/2
				y := bottom - ((value-minValue)/span)*(bottom-top)
				points = append(points, Point{X: x, Y: y})
			}
			for j := 1; j < len(points); j++ {
				layout.Lines = append(layout.Lines, LayoutLine{
					X1:          points[j-1].X,
					Y1:          points[j-1].Y,
					X2:          points[j].X,
					Y2:          points[j].Y,
					Stroke:      color,
					StrokeWidth: 2.2,
				})
			}
			for _, point := range points {
				layout.Circles = append(layout.Circles, LayoutCircle{
					CX:          point.X,
					CY:          point.Y,
					R:           4,
					Fill:        color,
					Stroke:      color,
					StrokeWidth: 1,
				})
			}
		default:
			barGroup := max(1, len(graph.XYSeries))
			barW := slot / float64(barGroup)
			for idx, value := range series.Values {
				x := left + float64(idx)*slot + float64(i)*barW
				y := bottom - ((value-minValue)/span)*(bottom-top)
				layout.Rects = append(layout.Rects, LayoutRect{
					X:           x + 2,
					Y:           y,
					W:           max(4, barW-4),
					H:           bottom - y,
					Fill:        color,
					Stroke:      color,
					StrokeWidth: 1,
				})
			}
		}
	}

	if len(graph.XYXCategories) > 0 {
		for i, label := range graph.XYXCategories {
			x := left + float64(i)*slot + slot/2
			layout.Texts = append(layout.Texts, LayoutText{
				X:      x,
				Y:      bottom + 20,
				Value:  label,
				Anchor: "middle",
				Size:   max(10, theme.FontSize-2),
				Color:  theme.PrimaryTextColor,
			})
		}
	}
	return layout
}

func layoutQuadrant(graph *Graph, theme Theme) Layout {
	layout := Layout{Kind: graph.Kind}
	layout.Width = 780
	layout.Height = 600

	left := 90.0
	top := 90.0
	size := 440.0
	cx := left + size/2
	cy := top + size/2

	layout.Rects = append(layout.Rects, LayoutRect{
		X:           left,
		Y:           top,
		W:           size,
		H:           size,
		Fill:        "#fdfdfd",
		Stroke:      theme.PrimaryBorderColor,
		StrokeWidth: 1.5,
	})
	layout.Lines = append(layout.Lines,
		LayoutLine{X1: cx, Y1: top, X2: cx, Y2: top + size, Stroke: theme.LineColor, StrokeWidth: 1.5},
		LayoutLine{X1: left, Y1: cy, X2: left + size, Y2: cy, Stroke: theme.LineColor, StrokeWidth: 1.5},
	)

	title := graph.QuadrantTitle
	if title == "" {
		title = "Quadrant Chart"
	}
	layout.Texts = append(layout.Texts, LayoutText{
		X:      left,
		Y:      44,
		Value:  title,
		Anchor: "start",
		Size:   theme.FontSize + 4,
		Weight: "600",
		Color:  theme.PrimaryTextColor,
	})

	if graph.QuadrantXAxisLeft != "" || graph.QuadrantXAxisRight != "" {
		layout.Texts = append(layout.Texts,
			LayoutText{X: left, Y: top + size + 28, Value: graph.QuadrantXAxisLeft, Anchor: "start", Size: max(10, theme.FontSize-2), Color: theme.PrimaryTextColor},
			LayoutText{X: left + size, Y: top + size + 28, Value: graph.QuadrantXAxisRight, Anchor: "end", Size: max(10, theme.FontSize-2), Color: theme.PrimaryTextColor},
		)
	}
	if graph.QuadrantYAxisBottom != "" || graph.QuadrantYAxisTop != "" {
		layout.Texts = append(layout.Texts,
			LayoutText{X: left - 10, Y: top + size, Value: graph.QuadrantYAxisBottom, Anchor: "end", Size: max(10, theme.FontSize-2), Color: theme.PrimaryTextColor},
			LayoutText{X: left - 10, Y: top + 8, Value: graph.QuadrantYAxisTop, Anchor: "end", Size: max(10, theme.FontSize-2), Color: theme.PrimaryTextColor},
		)
	}

	for i, label := range graph.QuadrantLabels {
		if label == "" {
			continue
		}
		var x, y float64
		switch i {
		case 0:
			x, y = cx+size*0.22, cy-size*0.18
		case 1:
			x, y = cx-size*0.22, cy-size*0.18
		case 2:
			x, y = cx-size*0.22, cy+size*0.2
		case 3:
			x, y = cx+size*0.22, cy+size*0.2
		}
		layout.Texts = append(layout.Texts, LayoutText{
			X:      x,
			Y:      y,
			Value:  label,
			Anchor: "middle",
			Size:   max(10, theme.FontSize-2),
			Color:  theme.PrimaryTextColor,
		})
	}

	for i, point := range graph.QuadrantPoints {
		x := left + clamp(point.X, 0, 1)*size
		y := top + (1-clamp(point.Y, 0, 1))*size
		color := seriesColor(i)
		layout.Circles = append(layout.Circles, LayoutCircle{
			CX:          x,
			CY:          y,
			R:           5,
			Fill:        color,
			Stroke:      color,
			StrokeWidth: 1,
		})
		layout.Texts = append(layout.Texts, LayoutText{
			X:      x + 8,
			Y:      y - 6,
			Value:  point.Label,
			Anchor: "start",
			Size:   max(10, theme.FontSize-2),
			Color:  theme.PrimaryTextColor,
		})
	}

	return layout
}

func layoutGeneric(graph *Graph, theme Theme) Layout {
	layout := Layout{Kind: graph.Kind}
	lines := graph.GenericLines
	if len(lines) == 0 {
		lines = []string{mustKindLabel(graph.Kind)}
	}
	padding := 24.0
	lineH := max(18, theme.FontSize+4)
	width := 760.0
	height := padding*2 + float64(len(lines)+2)*lineH
	layout.Width = width
	layout.Height = height

	layout.Rects = append(layout.Rects, LayoutRect{
		X:           1,
		Y:           1,
		W:           width - 2,
		H:           height - 2,
		RX:          8,
		RY:          8,
		Fill:        "#ffffff",
		Stroke:      theme.PrimaryBorderColor,
		StrokeWidth: 1.5,
	})
	layout.Texts = append(layout.Texts, LayoutText{
		X:      padding,
		Y:      padding + lineH,
		Value:  mustKindLabel(graph.Kind),
		Anchor: "start",
		Size:   theme.FontSize + 3,
		Weight: "600",
		Color:  theme.PrimaryTextColor,
	})
	for i, line := range lines {
		layout.Texts = append(layout.Texts, LayoutText{
			X:      padding,
			Y:      padding + lineH*float64(i+3),
			Value:  line,
			Anchor: "start",
			Size:   max(11, theme.FontSize-1),
			Color:  theme.PrimaryTextColor,
		})
	}
	return layout
}

func applyAspectRatio(layout *Layout, ratio *float64) {
	if ratio == nil || *ratio <= 0 || layout.Width <= 0 || layout.Height <= 0 {
		return
	}
	current := layout.Width / layout.Height
	target := *ratio
	if current < target {
		layout.Width = layout.Height * target
	} else {
		layout.Height = layout.Width / target
	}
}

func measureTextWidth(label string, fast bool) float64 {
	perChar := 7.2
	if fast {
		perChar = 6.4
	}
	return float64(len([]rune(label))) * perChar
}

func seriesColor(index int) string {
	colors := []string{
		"#4e79a7", "#f28e2c", "#e15759", "#76b7b2", "#59a14f",
		"#edc948", "#b07aa1", "#ff9da7", "#9c755f", "#bab0ab",
	}
	return colors[index%len(colors)]
}

func formatFloat(v float64) string {
	if math.Abs(v-math.Round(v)) < 0.0001 {
		return intString(int(math.Round(v)))
	}
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(v, 'f', 2, 64), "0"), ".")
}
