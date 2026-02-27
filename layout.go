package mermaid

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

func ComputeLayout(graph *Graph, theme Theme, config LayoutConfig) Layout {
	switch graph.Kind {
	case DiagramFlowchart, DiagramState, DiagramRequirement,
		DiagramC4:
		return layoutGraphLike(graph, theme, config)
	case DiagramSankey:
		return layoutSankeyFidelity(graph, theme, config)
	case DiagramRadar:
		return layoutRadarFidelity(graph, theme, config)
	case DiagramER:
		return layoutERDiagram(graph, theme, config)
	case DiagramClass:
		return layoutClassDiagram(graph, theme, config)
	case DiagramArchitecture:
		return layoutArchitecture(graph, theme, config)
	case DiagramBlock:
		return layoutBlockFidelity(graph, theme, config)
	case DiagramSequence:
		return layoutSequence(graph, theme, config)
	case DiagramZenUML:
		return layoutSequence(graph, theme, config)
	case DiagramPie:
		return layoutPieFidelity(graph, theme, config)
	case DiagramGantt:
		return layoutGanttFidelityV2(graph, theme, config)
	case DiagramTimeline:
		return layoutTimelineFidelity(graph, theme, config)
	case DiagramJourney:
		return layoutJourneyFidelity(graph, theme, config)
	case DiagramPacket:
		return layoutPacketFidelity(graph, theme, config)
	case DiagramMindmap:
		return layoutMindmap(graph, theme)
	case DiagramGitGraph:
		return layoutGitGraphFidelity(graph, theme, config)
	case DiagramTreemap:
		return layoutTreemapFidelity(graph, theme, config)
	case DiagramKanban:
		return layoutKanbanFidelity(graph, theme, config)
	case DiagramXYChart:
		return layoutXYChartFidelity(graph, theme, config)
	case DiagramQuadrant:
		return layoutQuadrant(graph, theme)
	default:
		return layoutGeneric(graph, theme)
	}
}

func layoutClassDiagram(graph *Graph, theme Theme, config LayoutConfig) Layout {
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
	nodeSpacing := max(24, config.NodeSpacing)
	rankSpacing := max(56, config.RankSpacing)
	lineH := max(14, theme.FontSize+2)
	titleH := 34.0
	maxNodeW := 140.0
	maxNodeH := 56.0
	nodeSizes := map[string]Point{}

	for _, id := range graph.NodeOrder {
		label := graph.Nodes[id].Label
		members := graph.ClassMembers[id]
		methods := graph.ClassMethods[id]

		longest := measureTextWidth(label, config.FastTextMetrics)
		for _, m := range members {
			longest = max(longest, measureTextWidth(m, config.FastTextMetrics))
		}
		for _, m := range methods {
			longest = max(longest, measureTextWidth(m, config.FastTextMetrics))
		}

		memberH := 0.0
		if len(members) > 0 {
			memberH = float64(len(members))*lineH + 10
		}
		methodH := 0.0
		if len(methods) > 0 {
			methodH = float64(len(methods))*lineH + 10
		}

		w := clamp(longest+26, 120, 420)
		h := max(56, titleH+memberH+methodH)
		nodeSizes[id] = Point{X: w, Y: h}
		maxNodeW = max(maxNodeW, w)
		maxNodeH = max(maxNodeH, h)
	}

	for rank := 0; rank <= maxRank; rank++ {
		nodes := orderedRanks[rank]
		for index, id := range nodes {
			size := nodeSizes[id]
			x := padding + float64(index)*(maxNodeW+nodeSpacing)
			y := padding + float64(rank)*(maxNodeH+rankSpacing)
			if graph.Direction == DirectionLeftRight || graph.Direction == DirectionRightLeft {
				x, y = y, x
			}
			layout.Nodes = append(layout.Nodes, NodeLayout{
				ID:    id,
				Label: graph.Nodes[id].Label,
				Shape: ShapeRectangle,
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
		maxX = max(maxX, node.X+node.W)
		maxY = max(maxY, node.Y+node.H)
	}

	for _, edge := range graph.Edges {
		from, okFrom := nodeIndex[edge.From]
		to, okTo := nodeIndex[edge.To]
		if !okFrom || !okTo {
			continue
		}
		x1, y1, x2, y2 := edgeEndpoints(from, to, graph.Direction)
		layout.Edges = append(layout.Edges, EdgeLayout{
			From:        edge.From,
			To:          edge.To,
			Label:       edge.Label,
			X1:          x1,
			Y1:          y1,
			X2:          x2,
			Y2:          y2,
			Style:       edge.Style,
			ArrowStart:  edge.ArrowStart,
			ArrowEnd:    edge.ArrowEnd || edge.Directed,
			MarkerStart: edge.MarkerStart,
			MarkerEnd:   edge.MarkerEnd,
		})
	}

	for edgeIdx, edge := range layout.Edges {
		lineClass := "relation"
		if edge.Style == EdgeDotted {
			lineClass += " dotted-line"
		}
		line := LayoutLine{
			ID:          "id_" + edge.From + "_" + edge.To + "_" + intString(edgeIdx+1),
			Class:       lineClass,
			X1:          edge.X1,
			Y1:          edge.Y1,
			X2:          edge.X2,
			Y2:          edge.Y2,
			Stroke:      theme.LineColor,
			StrokeWidth: 1.6,
			Dashed:      edge.Style == EdgeDotted,
			ArrowStart:  edge.ArrowStart,
			ArrowEnd:    edge.ArrowEnd,
			MarkerStart: edge.MarkerStart,
			MarkerEnd:   edge.MarkerEnd,
		}
		if strings.TrimSpace(line.MarkerStart) != "" || strings.TrimSpace(line.MarkerEnd) != "" {
			line.ArrowStart = false
			line.ArrowEnd = false
		}
		layout.Lines = append(layout.Lines, line)
		layout.Texts = append(layout.Texts, LayoutText{
			ID:     line.ID,
			Class:  "class-edge-label",
			X:      (edge.X1 + edge.X2) / 2,
			Y:      (edge.Y1+edge.Y2)/2 - 6,
			Value:  edge.Label,
			Anchor: "middle",
			Size:   max(11, theme.FontSize-1),
			Color:  theme.PrimaryTextColor,
		})
	}

	for _, node := range layout.Nodes {
		members := graph.ClassMembers[node.ID]
		methods := graph.ClassMethods[node.ID]
		memberH := 0.0
		if len(members) > 0 {
			memberH = float64(len(members))*lineH + 10
		}
		methodH := 0.0
		if len(methods) > 0 {
			methodH = float64(len(methods))*lineH + 10
		}

		layout.Rects = append(layout.Rects, LayoutRect{
			X:           node.X,
			Y:           node.Y,
			W:           node.W,
			H:           node.H,
			RX:          4,
			RY:          4,
			Fill:        "#ffffff",
			Stroke:      theme.PrimaryBorderColor,
			StrokeWidth: 1.6,
		})
		layout.Texts = append(layout.Texts, LayoutText{
			X:      node.X + node.W/2,
			Y:      node.Y + titleH*0.67,
			Value:  node.Label,
			Anchor: "middle",
			Size:   theme.FontSize,
			Weight: "600",
			Color:  theme.PrimaryTextColor,
		})

		sepY := node.Y + titleH
		if len(members) > 0 || len(methods) > 0 {
			layout.Lines = append(layout.Lines, LayoutLine{
				X1:          node.X,
				Y1:          sepY,
				X2:          node.X + node.W,
				Y2:          sepY,
				Stroke:      theme.PrimaryBorderColor,
				StrokeWidth: 1.1,
			})
		}

		y := sepY + lineH*0.85
		for _, member := range members {
			layout.Texts = append(layout.Texts, LayoutText{
				X:      node.X + node.W/2,
				Y:      y,
				Value:  member,
				Anchor: "middle",
				Size:   max(10, theme.FontSize-1),
				Color:  theme.PrimaryTextColor,
			})
			y += lineH
		}

		if len(methods) > 0 {
			methodSepY := node.Y + titleH + memberH
			layout.Lines = append(layout.Lines, LayoutLine{
				X1:          node.X,
				Y1:          methodSepY,
				X2:          node.X + node.W,
				Y2:          methodSepY,
				Stroke:      theme.PrimaryBorderColor,
				StrokeWidth: 1.1,
			})
			y = methodSepY + lineH*0.85
			for _, method := range methods {
				layout.Texts = append(layout.Texts, LayoutText{
					X:      node.X + node.W/2,
					Y:      y,
					Value:  method,
					Anchor: "middle",
					Size:   max(10, theme.FontSize-1),
					Color:  theme.PrimaryTextColor,
				})
				y += lineH
			}
		}

		_ = methodH
	}

	layout.Width = maxX + padding
	layout.Height = maxY + padding
	return layout
}

func layoutERDiagram(graph *Graph, theme Theme, config LayoutConfig) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.NodeOrder) == 0 {
		return layoutGeneric(graph, theme)
	}

	paddingX := 24.0
	paddingY := 32.0
	// Mermaid ER defaults (ranksep/nodesep) produce larger vertical lanes.
	rowGap := max(164, config.RankSpacing*2.5)
	lineH := max(12, theme.FontSize+1)
	titleH := 34.0

	maxNodeW := 140.0
	nodeSizes := map[string]Point{}
	for _, id := range graph.NodeOrder {
		label := graph.Nodes[id].Label
		longest := measureTextWidth(label, config.FastTextMetrics)
		for _, attr := range graph.ERAttributes[id] {
			longest = max(longest, measureTextWidth(attr, config.FastTextMetrics))
		}
		w := clamp(longest+20, 110, 380)
		attrH := 0.0
		if len(graph.ERAttributes[id]) > 0 {
			attrH = float64(len(graph.ERAttributes[id]))*lineH + 10
		}
		h := max(56, titleH+attrH)
		nodeSizes[id] = Point{X: w, Y: h}
		maxNodeW = max(maxNodeW, w)
	}

	y := paddingY
	maxX := 0.0
	for _, id := range graph.NodeOrder {
		size := nodeSizes[id]
		x := paddingX
		if graph.Direction == DirectionLeftRight || graph.Direction == DirectionRightLeft {
			x = paddingX + (maxNodeW-size.X)/2
		}
		layout.Nodes = append(layout.Nodes, NodeLayout{
			ID:    id,
			Label: graph.Nodes[id].Label,
			Shape: ShapeRectangle,
			X:     x,
			Y:     y,
			W:     size.X,
			H:     size.Y,
		})
		y += size.Y + rowGap
		maxX = max(maxX, x+size.X)
	}

	nodeIndex := map[string]NodeLayout{}
	for _, node := range layout.Nodes {
		nodeIndex[node.ID] = node
	}

	for _, edge := range graph.Edges {
		from, okFrom := nodeIndex[edge.From]
		to, okTo := nodeIndex[edge.To]
		if !okFrom || !okTo {
			continue
		}
		x1, y1 := from.X+from.W/2, from.Y+from.H
		x2, y2 := to.X+to.W/2, to.Y
		layout.Edges = append(layout.Edges, EdgeLayout{
			From:        edge.From,
			To:          edge.To,
			Label:       edge.Label,
			X1:          x1,
			Y1:          y1,
			X2:          x2,
			Y2:          y2,
			Style:       edge.Style,
			ArrowStart:  false,
			ArrowEnd:    false,
			MarkerStart: edge.MarkerStart,
			MarkerEnd:   edge.MarkerEnd,
		})
	}

	for _, edge := range layout.Edges {
		layout.Lines = append(layout.Lines, LayoutLine{
			X1:          edge.X1,
			Y1:          edge.Y1,
			X2:          edge.X2,
			Y2:          edge.Y2,
			Stroke:      theme.LineColor,
			StrokeWidth: 1.4,
			Dashed:      edge.Style == EdgeDotted,
			ArrowStart:  false,
			ArrowEnd:    false,
			MarkerStart: edge.MarkerStart,
			MarkerEnd:   edge.MarkerEnd,
		})
		if edge.Label != "" {
			layout.Texts = append(layout.Texts, LayoutText{
				X:      (edge.X1 + edge.X2) / 2,
				Y:      (edge.Y1+edge.Y2)/2 - 6,
				Value:  edge.Label,
				Anchor: "middle",
				Size:   max(10, theme.FontSize-1),
				Color:  theme.PrimaryTextColor,
			})
		}
	}

	maxY := 0.0
	for _, node := range layout.Nodes {
		layout.Rects = append(layout.Rects, LayoutRect{
			X:           node.X,
			Y:           node.Y,
			W:           node.W,
			H:           node.H,
			RX:          2,
			RY:          2,
			Fill:        "#ffffff",
			Stroke:      theme.PrimaryBorderColor,
			StrokeWidth: 1.4,
		})
		layout.Texts = append(layout.Texts, LayoutText{
			X:      node.X + node.W/2,
			Y:      node.Y + titleH*0.67,
			Value:  node.Label,
			Anchor: "middle",
			Size:   theme.FontSize,
			Weight: "600",
			Color:  theme.PrimaryTextColor,
		})

		attrs := graph.ERAttributes[node.ID]
		if len(attrs) > 0 {
			sepY := node.Y + titleH
			layout.Lines = append(layout.Lines, LayoutLine{
				X1:          node.X,
				Y1:          sepY,
				X2:          node.X + node.W,
				Y2:          sepY,
				Stroke:      theme.PrimaryBorderColor,
				StrokeWidth: 1.0,
			})
			yAttr := sepY + lineH*0.85
			for _, attr := range attrs {
				fields := strings.Fields(attr)
				if len(fields) >= 2 {
					attrType := fields[0]
					attrName := strings.Join(fields[1:], " ")
					layout.Texts = append(layout.Texts,
						LayoutText{
							X:      node.X + 8,
							Y:      yAttr,
							Value:  attrType,
							Anchor: "start",
							Size:   max(10, theme.FontSize-1),
							Color:  theme.PrimaryTextColor,
						},
						LayoutText{
							X:      node.X + node.W*0.42,
							Y:      yAttr,
							Value:  attrName,
							Anchor: "start",
							Size:   max(10, theme.FontSize-1),
							Color:  theme.PrimaryTextColor,
						},
						LayoutText{
							X:      node.X + node.W + 12,
							Y:      yAttr,
							Value:  "",
							Anchor: "start",
							Size:   max(10, theme.FontSize-1),
							Color:  theme.PrimaryTextColor,
						},
						LayoutText{
							X:      node.X + node.W + 12,
							Y:      yAttr,
							Value:  "",
							Anchor: "start",
							Size:   max(10, theme.FontSize-1),
							Color:  theme.PrimaryTextColor,
						},
					)
				} else {
					layout.Texts = append(layout.Texts,
						LayoutText{
							X:      node.X + 8,
							Y:      yAttr,
							Value:  attr,
							Anchor: "start",
							Size:   max(10, theme.FontSize-1),
							Color:  theme.PrimaryTextColor,
						},
						LayoutText{
							X:      node.X + node.W + 12,
							Y:      yAttr,
							Value:  "",
							Anchor: "start",
							Size:   max(10, theme.FontSize-1),
							Color:  theme.PrimaryTextColor,
						},
						LayoutText{
							X:      node.X + node.W + 12,
							Y:      yAttr,
							Value:  "",
							Anchor: "start",
							Size:   max(10, theme.FontSize-1),
							Color:  theme.PrimaryTextColor,
						},
					)
				}
				yAttr += lineH
			}
		}

		maxY = max(maxY, node.Y+node.H)
	}

	layout.Width = maxX + paddingX
	layout.Height = maxY + paddingY
	return layout
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
	if graph.Kind == DiagramState {
		padding = 24.0
		// State diagrams need larger rank spacing to avoid over-compression.
		nodeSpacing = max(14, config.NodeSpacing*0.55)
		rankSpacing = max(96, config.RankSpacing*1.6)
		baseHeight = 40.0
	}
	maxNodeWidth := 100.0
	nodeSizes := map[string]Point{}

	for _, id := range graph.NodeOrder {
		node := graph.Nodes[id]
		if graph.Kind == DiagramState &&
			(node.Shape == ShapeCircle || node.Shape == ShapeDoubleCircle) &&
			strings.TrimSpace(node.Label) == "" {
			nodeSizes[id] = Point{X: 28, Y: 28}
			maxNodeWidth = max(maxNodeWidth, 28)
			continue
		}
		minW := 80.0
		maxW := 320.0
		paddingW := 28.0
		if graph.Kind == DiagramState {
			minW = 44
			maxW = 170
			paddingW = 14
		}
		w := clamp(measureTextWidth(node.Label, config.FastTextMetrics)+paddingW, minW, maxW)
		h := baseHeight
		if graph.Kind == DiagramState {
			h = 40
		}
		nodeSizes[id] = Point{X: w, Y: h}
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
	for edgeIdx, edge := range layout.Edges {
		strokeWidth := 2.0
		dashed := false
		if edge.Style == EdgeDotted {
			dashed = true
		}
		if edge.Style == EdgeThick {
			strokeWidth = 3
		}
		line := LayoutLine{
			X1:          edge.X1,
			Y1:          edge.Y1,
			X2:          edge.X2,
			Y2:          edge.Y2,
			Stroke:      theme.LineColor,
			StrokeWidth: strokeWidth,
			Dashed:      dashed,
			ArrowStart:  edge.ArrowStart,
			ArrowEnd:    edge.ArrowEnd,
			MarkerStart: edge.MarkerStart,
			MarkerEnd:   edge.MarkerEnd,
		}
		if layout.Kind == DiagramState {
			line.ID = "edge" + intString(edgeIdx)
			line.Class = "edge-thickness-normal edge-pattern-solid transition"
		}
		layout.Lines = append(layout.Lines, line)
		if layout.Kind == DiagramState {
			layout.Texts = append(layout.Texts, LayoutText{
				ID:     "edge" + intString(edgeIdx),
				Class:  "state-edge-label",
				X:      (edge.X1 + edge.X2) / 2,
				Y:      (edge.Y1+edge.Y2)/2 - 6,
				Value:  edge.Label,
				Anchor: "middle",
				Size:   max(11, theme.FontSize-1),
				Color:  theme.PrimaryTextColor,
			})
		} else if edge.Label != "" {
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
		addNodePrimitive(layout, theme, layout.Kind, node)
		if layout.Kind == DiagramState && strings.TrimSpace(node.Label) == "" {
			continue
		}
		textClass := ""
		if layout.Kind == DiagramState {
			textClass = "state-node-label"
		}
		layout.Texts = append(layout.Texts, LayoutText{
			Class:  textClass,
			X:      node.X + node.W/2,
			Y:      node.Y + node.H/2 + theme.FontSize*0.35,
			Value:  node.Label,
			Anchor: "middle",
			Size:   theme.FontSize,
			Color:  theme.PrimaryTextColor,
		})
	}
}

func addNodePrimitive(layout *Layout, theme Theme, kind DiagramKind, node NodeLayout) {
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
		if kind == DiagramState {
			layout.Circles = append(layout.Circles, LayoutCircle{
				CX:          node.X + node.W/2,
				CY:          node.Y + node.H/2,
				R:           r,
				Fill:        fill,
				Stroke:      stroke,
				StrokeWidth: 2.2,
			})
			break
		}
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
	zenuml := graph.Kind == DiagramZenUML
	if zenuml {
		layout.ZenUMLTitle = graph.ZenUMLTitle
		layout.ZenUMLMessages = append([]SequenceMessage(nil), graph.SequenceMessages...)
		layout.ZenUMLAltBlocks = append([]ZenUMLAltBlock(nil), graph.ZenUMLAltBlocks...)
	}
	participants := graph.SequenceParticipants
	if len(participants) == 0 {
		for _, id := range graph.NodeOrder {
			participants = append(participants, graph.Nodes[id].Label)
		}
	}
	if len(participants) == 0 {
		return layoutGeneric(graph, theme)
	}
	if !zenuml {
		participantLabels := make(map[string]string, len(graph.SequenceParticipantLabels))
		for key, value := range graph.SequenceParticipantLabels {
			participantLabels[key] = value
		}
		events := append([]SequenceEvent(nil), graph.SequenceEvents...)
		if len(events) == 0 {
			events = defaultSequenceEvents(graph.SequenceMessages)
		}
		layout.SequenceParticipants = append([]string(nil), participants...)
		layout.SequenceParticipantLabels = participantLabels
		layout.SequenceMessages = append([]SequenceMessage(nil), graph.SequenceMessages...)
		layout.SequenceEvents = events
		plan := buildSequencePlan(layout.SequenceParticipants, layout.SequenceParticipantLabels, layout.SequenceMessages, layout.SequenceEvents, theme)
		layout.Width = plan.Width
		layout.Height = plan.Height
		layout.ViewBoxX = plan.ViewBoxX
		layout.ViewBoxY = plan.ViewBoxY
		layout.ViewBoxWidth = plan.ViewBoxWidth
		layout.ViewBoxHeight = plan.ViewBoxHeight
		layout.SVGStyle = "max-width: " + formatFloat(plan.ViewBoxWidth) + "px; background-color: white;"
		return layout
	}
	if zenuml {
		layout.ZenUMLParticipants = append([]string(nil), participants...)
	}

	padding := 60.0
	boxW := 130.0
	boxH := 36.0
	participantSpacing := 170.0
	topY := 40.0
	msgStart := 120.0
	msgStep := 56.0
	if zenuml {
		boxW = 130
		boxH = 36
		participantSpacing = 170
		topY = 40
		msgStart = 120
		msgStep = 56
		if strings.TrimSpace(graph.ZenUMLTitle) != "" {
			layout.Texts = append(layout.Texts, LayoutText{
				X:      24,
				Y:      26,
				Value:  graph.ZenUMLTitle,
				Anchor: "start",
				Size:   theme.FontSize + 3,
				Weight: "600",
				Color:  theme.PrimaryTextColor,
			})
		}
	}

	xPos := map[string]float64{}
	for i, participant := range participants {
		xPos[participant] = padding + float64(i)*participantSpacing
		x := xPos[participant]
		label := participant
		if named, ok := graph.SequenceParticipantLabels[participant]; ok && strings.TrimSpace(named) != "" {
			label = named
		}
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
			Value:  label,
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

	if zenuml {
		leftX := xPos[participants[0]] - boxW/2 - 56
		rightX := xPos[participants[len(participants)-1]] + boxW/2 + 10
		for _, block := range graph.ZenUMLAltBlocks {
			if block.Start < 0 || block.End < block.Start {
				continue
			}
			startY := msgStart + float64(block.Start)*msgStep - 24
			endY := msgStart + float64(block.End)*msgStep + 16
			layout.Rects = append(layout.Rects, LayoutRect{
				X:               leftX,
				Y:               startY,
				W:               rightX - leftX,
				H:               endY - startY + 24,
				RX:              6,
				RY:              6,
				Fill:            "none",
				Stroke:          theme.PrimaryBorderColor,
				StrokeWidth:     1.4,
				StrokeDasharray: "5,4",
			})
			layout.Texts = append(layout.Texts, LayoutText{
				X:      leftX + 12,
				Y:      startY + 16,
				Value:  "Alt",
				Anchor: "start",
				Size:   max(12, theme.FontSize),
				Weight: "600",
				Color:  theme.PrimaryTextColor,
			})
			if strings.TrimSpace(block.Condition) != "" {
				layout.Texts = append(layout.Texts, LayoutText{
					X:      leftX + 12,
					Y:      startY + 34,
					Value:  "[" + block.Condition + "]",
					Anchor: "start",
					Size:   max(11, theme.FontSize-1),
					Color:  theme.PrimaryTextColor,
				})
			}
			if block.ElseStart >= 0 && block.ElseStart <= block.End {
				elseY := msgStart + float64(block.ElseStart)*msgStep - 18
				layout.Lines = append(layout.Lines, LayoutLine{
					X1:          leftX,
					Y1:          elseY,
					X2:          rightX,
					Y2:          elseY,
					Stroke:      theme.PrimaryBorderColor,
					StrokeWidth: 1.1,
				})
				layout.Texts = append(layout.Texts, LayoutText{
					X:      leftX + 12,
					Y:      elseY - 4,
					Value:  "[else]",
					Anchor: "start",
					Size:   max(11, theme.FontSize-1),
					Color:  theme.PrimaryTextColor,
				})
			}
		}
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
		if msg.IsReturn {
			line.Dashed = true
		}
		layout.Lines = append(layout.Lines, line)
		if zenuml && strings.TrimSpace(msg.Index) != "" {
			layout.Texts = append(layout.Texts, LayoutText{
				X:      min(fromX, toX) - 8,
				Y:      y - 2,
				Value:  msg.Index,
				Anchor: "end",
				Size:   max(10, theme.FontSize-3),
				Color:  "#6b7280",
			})
		}
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
	if zenuml {
		layout.Height += 12
	}
	return layout
}

func layoutArchitecture(graph *Graph, theme Theme, config LayoutConfig) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.ArchitectureServices) == 0 {
		return layoutGraphLike(graph, theme, config)
	}

	iconW := 80.0
	iconH := 80.0
	serviceW := iconW
	serviceH := iconH + 28.0
	groupPadX := 34.0
	groupPadY := 24.0
	groupHeaderH := 32.0
	cellGapX := 120.0
	cellGapY := 120.0
	groupGapX := 86.0
	baseX := -278.0
	baseY := -126.0

	type groupPlacement struct {
		Group  ArchitectureGroup
		X      float64
		Y      float64
		W      float64
		H      float64
		IDs    []string
		Active bool
	}

	servicesByID := map[string]ArchitectureService{}
	for _, service := range graph.ArchitectureServices {
		servicesByID[service.ID] = service
	}

	groupOrder := make([]ArchitectureGroup, 0, len(graph.ArchitectureGroups))
	groupSeen := map[string]bool{}
	for _, group := range graph.ArchitectureGroups {
		if group.ID == "" || groupSeen[group.ID] {
			continue
		}
		groupSeen[group.ID] = true
		groupOrder = append(groupOrder, group)
	}
	for _, service := range graph.ArchitectureServices {
		if strings.TrimSpace(service.GroupID) == "" {
			continue
		}
		if groupSeen[service.GroupID] {
			continue
		}
		groupSeen[service.GroupID] = true
		groupOrder = append(groupOrder, ArchitectureGroup{
			ID:    service.GroupID,
			Label: service.GroupID,
			Icon:  "cloud",
		})
	}
	if len(groupOrder) == 0 {
		groupOrder = append(groupOrder, ArchitectureGroup{ID: "_default", Label: "Services", Icon: "cloud"})
	}

	groupServices := map[string][]string{}
	for _, service := range graph.ArchitectureServices {
		groupID := service.GroupID
		if groupID == "" {
			groupID = groupOrder[0].ID
		}
		groupServices[groupID] = append(groupServices[groupID], service.ID)
	}

	servicePos := map[string]Point{}
	groupPlacements := make([]groupPlacement, 0, len(groupOrder))
	currentX := baseX

	for _, group := range groupOrder {
		ids := groupServices[group.ID]
		if len(ids) == 0 {
			continue
		}
		type slot struct {
			Col int
			Row int
		}
		slots := make([]slot, 0, len(ids))
		switch len(ids) {
		case 1:
			slots = append(slots, slot{Col: 0, Row: 0})
		case 2:
			slots = append(slots, slot{Col: 0, Row: 0}, slot{Col: 1, Row: 0})
		case 3:
			// Matches Mermaid's canonical architecture sample placement.
			slots = append(slots, slot{Col: 0, Row: 0}, slot{Col: 0, Row: 1}, slot{Col: 1, Row: 0})
		default:
			for i := range ids {
				slots = append(slots, slot{Col: i % 2, Row: i / 2})
			}
		}

		maxCol := 0
		maxRow := 0
		for _, s := range slots {
			maxCol = max(maxCol, s.Col)
			maxRow = max(maxRow, s.Row)
		}
		groupW := groupPadX*2 + serviceW + float64(maxCol)*cellGapX
		groupH := groupPadY*2 + groupHeaderH + serviceH + float64(maxRow)*cellGapY
		groupX := currentX
		groupY := baseY
		currentX += groupW + groupGapX

		for i, id := range ids {
			slot := slots[i]
			sx := groupX + groupPadX + float64(slot.Col)*cellGapX
			sy := groupY + groupPadY + groupHeaderH + float64(slot.Row)*cellGapY
			servicePos[id] = Point{X: sx, Y: sy}
		}

		groupPlacements = append(groupPlacements, groupPlacement{
			Group:  group,
			X:      groupX,
			Y:      groupY,
			W:      groupW,
			H:      groupH,
			IDs:    append([]string(nil), ids...),
			Active: true,
		})
	}

	minX := math.MaxFloat64
	minY := math.MaxFloat64
	maxX := -math.MaxFloat64
	maxY := -math.MaxFloat64
	trackBounds := func(x1, y1, x2, y2 float64) {
		minX = min(minX, x1)
		minY = min(minY, y1)
		maxX = max(maxX, x2)
		maxY = max(maxY, y2)
	}

	cloudPath := func(x, y, w, h float64) string {
		return "M" + formatFloat(x+w*0.22) + "," + formatFloat(y+h*0.62) +
			" C" + formatFloat(x+w*0.18) + "," + formatFloat(y+h*0.48) + " " + formatFloat(x+w*0.26) + "," + formatFloat(y+h*0.33) + " " + formatFloat(x+w*0.4) + "," + formatFloat(y+h*0.3) +
			" C" + formatFloat(x+w*0.5) + "," + formatFloat(y+h*0.16) + " " + formatFloat(x+w*0.7) + "," + formatFloat(y+h*0.14) + " " + formatFloat(x+w*0.8) + "," + formatFloat(y+h*0.3) +
			" C" + formatFloat(x+w*0.92) + "," + formatFloat(y+h*0.34) + " " + formatFloat(x+w*0.96) + "," + formatFloat(y+h*0.52) + " " + formatFloat(x+w*0.86) + "," + formatFloat(y+h*0.62) +
			" L" + formatFloat(x+w*0.22) + "," + formatFloat(y+h*0.62) + " Z"
	}

	for _, gp := range groupPlacements {
		layout.ArchitectureGroups = append(layout.ArchitectureGroups, ArchitectureGroupLayout{
			ID:    gp.Group.ID,
			Label: gp.Group.Label,
			Icon:  gp.Group.Icon,
			X:     gp.X,
			Y:     gp.Y,
			W:     gp.W,
			H:     gp.H,
		})
		layout.Rects = append(layout.Rects, LayoutRect{
			ID:              "group-" + gp.Group.ID,
			Class:           "node-bkg",
			X:               gp.X,
			Y:               gp.Y,
			W:               gp.W,
			H:               gp.H,
			Fill:            "none",
			Stroke:          "#B7BDEB",
			StrokeWidth:     2.0,
			StrokeDasharray: "8",
		})
		iconX := gp.X + 1
		iconY := gp.Y + 1
		layout.Rects = append(layout.Rects, LayoutRect{
			X:           iconX,
			Y:           iconY,
			W:           30,
			H:           30,
			Fill:        "#087ebf",
			Stroke:      "none",
			StrokeWidth: 0,
		})
		layout.Paths = append(layout.Paths, LayoutPath{
			D:           cloudPath(iconX+4, iconY+4, 22, 22),
			Fill:        "none",
			Stroke:      "#ffffff",
			StrokeWidth: 2.0,
			LineJoin:    "round",
			LineCap:     "round",
		})
		layout.Texts = append(layout.Texts, LayoutText{
			X:      gp.X + 33,
			Y:      gp.Y + 13,
			Value:  gp.Group.Label,
			Anchor: "start",
			Size:   max(12, theme.FontSize),
			Color:  theme.PrimaryTextColor,
		})
		trackBounds(gp.X, gp.Y, gp.X+gp.W, gp.Y+gp.H)
	}

	for _, service := range graph.ArchitectureServices {
		pos, ok := servicePos[service.ID]
		if !ok {
			continue
		}
		x := pos.X
		y := pos.Y

		iconType := lower(strings.TrimSpace(service.Icon))
		if iconType == "" {
			iconType = "server"
		}
		layout.ArchitectureServices = append(layout.ArchitectureServices, ArchitectureServiceLayout{
			ID:      service.ID,
			Label:   service.Label,
			Icon:    iconType,
			GroupID: service.GroupID,
			X:       x,
			Y:       y,
			W:       iconW,
			H:       iconH,
		})

		layout.Rects = append(layout.Rects, LayoutRect{
			ID:          "service-" + service.ID,
			Class:       "architecture-service",
			X:           x,
			Y:           y,
			W:           iconW,
			H:           iconH,
			Fill:        "#087ebf",
			Stroke:      "none",
			StrokeWidth: 0,
		})

		switch iconType {
		case "database":
			layout.Ellipses = append(layout.Ellipses,
				LayoutEllipse{CX: x + 40, CY: y + 22.14, RX: 20, RY: 7.14, Fill: "none", Stroke: "#ffffff", StrokeWidth: 2},
			)
			layout.Paths = append(layout.Paths,
				LayoutPath{D: "M" + formatFloat(x+20) + "," + formatFloat(y+34.05) + " C" + formatFloat(x+24) + "," + formatFloat(y+40) + " " + formatFloat(x+56) + "," + formatFloat(y+40) + " " + formatFloat(x+60) + "," + formatFloat(y+34.05), Fill: "none", Stroke: "#ffffff", StrokeWidth: 2},
				LayoutPath{D: "M" + formatFloat(x+20) + "," + formatFloat(y+45.95) + " C" + formatFloat(x+24) + "," + formatFloat(y+51) + " " + formatFloat(x+56) + "," + formatFloat(y+51) + " " + formatFloat(x+60) + "," + formatFloat(y+45.95), Fill: "none", Stroke: "#ffffff", StrokeWidth: 2},
				LayoutPath{D: "M" + formatFloat(x+20) + "," + formatFloat(y+57.86) + " C" + formatFloat(x+24) + "," + formatFloat(y+63) + " " + formatFloat(x+56) + "," + formatFloat(y+63) + " " + formatFloat(x+60) + "," + formatFloat(y+57.86), Fill: "none", Stroke: "#ffffff", StrokeWidth: 2},
			)
			layout.Lines = append(layout.Lines,
				LayoutLine{X1: x + 20, Y1: y + 22.14, X2: x + 20, Y2: y + 57.86, Stroke: "#ffffff", StrokeWidth: 2},
				LayoutLine{X1: x + 60, Y1: y + 22.14, X2: x + 60, Y2: y + 57.86, Stroke: "#ffffff", StrokeWidth: 2},
			)
		case "disk":
			layout.Rects = append(layout.Rects, LayoutRect{
				X:           x + 20,
				Y:           y + 15,
				W:           40,
				H:           50,
				RX:          1,
				RY:          1,
				Fill:        "none",
				Stroke:      "#ffffff",
				StrokeWidth: 2,
			})
			layout.Ellipses = append(layout.Ellipses,
				LayoutEllipse{CX: x + 24, CY: y + 19.17, RX: 0.8, RY: 0.83, Fill: "none", Stroke: "#ffffff", StrokeWidth: 2},
				LayoutEllipse{CX: x + 56, CY: y + 19.17, RX: 0.8, RY: 0.83, Fill: "none", Stroke: "#ffffff", StrokeWidth: 2},
				LayoutEllipse{CX: x + 24, CY: y + 60.83, RX: 0.8, RY: 0.83, Fill: "none", Stroke: "#ffffff", StrokeWidth: 2},
				LayoutEllipse{CX: x + 56, CY: y + 60.83, RX: 0.8, RY: 0.83, Fill: "none", Stroke: "#ffffff", StrokeWidth: 2},
				LayoutEllipse{CX: x + 40, CY: y + 33.75, RX: 14, RY: 14.58, Fill: "none", Stroke: "#ffffff", StrokeWidth: 2},
				LayoutEllipse{CX: x + 40, CY: y + 33.75, RX: 4, RY: 4.17, Fill: "#ffffff", Stroke: "#ffffff", StrokeWidth: 2},
			)
			layout.Paths = append(layout.Paths, LayoutPath{
				D:           "M" + formatFloat(x+37.5) + "," + formatFloat(y+42.52) + " L" + formatFloat(x+32.68) + "," + formatFloat(y+55.74) + " L" + formatFloat(x+30.73) + "," + formatFloat(y+54.58) + " L" + formatFloat(x+35.42) + "," + formatFloat(y+41.32) + " Z",
				Fill:        "#ffffff",
				Stroke:      "none",
				StrokeWidth: 0,
			})
		default:
			// "server" plus fallback.
			layout.Rects = append(layout.Rects, LayoutRect{
				X:           x + 17.5,
				Y:           y + 17.5,
				W:           45,
				H:           45,
				RX:          2,
				RY:          2,
				Fill:        "none",
				Stroke:      "#ffffff",
				StrokeWidth: 2,
			})
			layout.Lines = append(layout.Lines,
				LayoutLine{X1: x + 17.5, Y1: y + 32.5, X2: x + 62.5, Y2: y + 32.5, Stroke: "#ffffff", StrokeWidth: 2},
				LayoutLine{X1: x + 17.5, Y1: y + 47.5, X2: x + 62.5, Y2: y + 47.5, Stroke: "#ffffff", StrokeWidth: 2},
			)
			for _, row := range []float64{25, 40, 55} {
				layout.Paths = append(layout.Paths, LayoutPath{
					D:           "M" + formatFloat(x+44.75) + "," + formatFloat(y+row) + " L" + formatFloat(x+55.25) + "," + formatFloat(y+row),
					Fill:        "none",
					Stroke:      "#ffffff",
					StrokeWidth: 2,
					LineCap:     "round",
				})
				for _, col := range []float64{22.5, 27.5, 32.5} {
					layout.Circles = append(layout.Circles, LayoutCircle{
						CX:          x + col,
						CY:          y + row,
						R:           0.75,
						Fill:        "#ffffff",
						Stroke:      "#ffffff",
						StrokeWidth: 1,
					})
				}
			}
		}

		layout.Texts = append(layout.Texts, LayoutText{
			X:      x + iconW/2,
			Y:      y + iconH + 16,
			Value:  service.Label,
			Anchor: "middle",
			Size:   max(12, theme.FontSize),
			Color:  theme.PrimaryTextColor,
		})
		trackBounds(x, y, x+serviceW, y+serviceH)
	}

	serviceAnchor := func(id, side string) (float64, float64, bool) {
		pos, ok := servicePos[id]
		if !ok {
			return 0, 0, false
		}
		x := pos.X
		y := pos.Y
		switch upper(side) {
		case "L":
			return x, y + iconH/2, true
		case "R":
			return x + iconW, y + iconH/2, true
		case "T":
			return x + iconW/2, y, true
		case "B":
			return x + iconW/2, y + iconH, true
		default:
			return x + iconW, y + iconH/2, true
		}
	}

	for i, edge := range graph.ArchitectureLinks {
		x1, y1, okFrom := serviceAnchor(edge.From.ID, edge.From.Side)
		x2, y2, okTo := serviceAnchor(edge.To.ID, edge.To.Side)
		if !okFrom || !okTo {
			continue
		}
		pathD := ""
		if math.Abs(x2-x1) >= math.Abs(y2-y1) {
			midX := (x1 + x2) / 2
			pathD = "M " + formatFloat(x1) + "," + formatFloat(y1) +
				" L " + formatFloat(midX) + "," + formatFloat(y1) +
				" L" + formatFloat(x2) + "," + formatFloat(y2)
		} else {
			midY := (y1 + y2) / 2
			pathD = "M " + formatFloat(x1) + "," + formatFloat(y1) +
				" L " + formatFloat(x1) + "," + formatFloat(midY) +
				" L" + formatFloat(x2) + "," + formatFloat(y2)
		}
		layout.Paths = append(layout.Paths, LayoutPath{
			ID:          "L_" + edge.From.ID + "_" + edge.To.ID + "_" + intString(i),
			Class:       "edge",
			D:           pathD,
			Fill:        "none",
			Stroke:      "#333333",
			StrokeWidth: 2.5,
			LineCap:     "round",
			LineJoin:    "round",
		})
		trackBounds(min(x1, x2), min(y1, y2), max(x1, x2), max(y1, y2))
	}

	if minX == math.MaxFloat64 || minY == math.MaxFloat64 || maxX == -math.MaxFloat64 || maxY == -math.MaxFloat64 {
		return layoutGraphLike(graph, theme, config)
	}

	layout.ViewBoxX = minX - 40
	layout.ViewBoxY = minY - 40
	layout.ViewBoxWidth = (maxX - minX) + 214
	layout.ViewBoxHeight = (maxY - minY) + 160
	layout.Width = layout.ViewBoxWidth
	layout.Height = layout.ViewBoxHeight
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

	paddingX := 36.0
	paddingY := 24.0
	levelSpacing := 120.0
	rowSpacing := 52.0
	siblingGap := 8.0

	rootID := graph.MindmapRootID
	if rootID == "" {
		rootID = graph.MindmapNodes[0].ID
	}
	layout.MindmapRootID = rootID
	layout.MindmapNodes = append(layout.MindmapNodes, graph.MindmapNodes...)

	nodeByID := map[string]MindmapNode{}
	children := map[string][]string{}
	for _, node := range graph.MindmapNodes {
		nodeByID[node.ID] = node
		if node.Parent != "" {
			children[node.Parent] = append(children[node.Parent], node.ID)
		}
	}

	side := map[string]int{rootID: 0}
	var assignSide func(string, int)
	assignSide = func(id string, value int) {
		side[id] = value
		for _, childID := range children[id] {
			assignSide(childID, value)
		}
	}
	rootChildren := children[rootID]
	for i, childID := range rootChildren {
		assign := 1
		if i%2 == 1 {
			assign = -1
		}
		assignSide(childID, assign)
	}
	for _, node := range graph.MindmapNodes {
		if _, ok := side[node.ID]; !ok {
			side[node.ID] = 1
		}
	}

	depth := map[string]int{}
	for _, node := range graph.MindmapNodes {
		d := node.Level
		if node.ID == rootID {
			d = 0
		}
		depth[node.ID] = d
	}

	nodeSize := map[string]Point{}
	for _, node := range graph.MindmapNodes {
		w := clamp(measureTextWidth(node.Label, true)+26, 86, 280)
		h := 46.0
		shape := node.Shape
		if shape == "" {
			shape = ShapeRoundRect
		}
		if shape == ShapeCircle || shape == ShapeDoubleCircle {
			d := clamp(max(w, h), 70, 180)
			w = d
			h = d
		}
		nodeSize[node.ID] = Point{X: w, Y: h}
	}

	subtreeHeight := map[string]float64{}
	var calcSubtreeHeight func(string) float64
	calcSubtreeHeight = func(id string) float64 {
		if cached, ok := subtreeHeight[id]; ok {
			return cached
		}
		kids := children[id]
		if len(kids) == 0 {
			h := max(rowSpacing, nodeSize[id].Y+10)
			subtreeHeight[id] = h
			return h
		}
		total := 0.0
		for i, childID := range kids {
			total += calcSubtreeHeight(childID)
			if i < len(kids)-1 {
				total += siblingGap
			}
		}
		total = max(total, nodeSize[id].Y+12)
		subtreeHeight[id] = total
		return total
	}
	sideChildren := map[int][]string{
		-1: {},
		1:  {},
	}
	for _, childID := range rootChildren {
		sideChildren[side[childID]] = append(sideChildren[side[childID]], childID)
	}
	calcSideHeight := func(ids []string) float64 {
		if len(ids) == 0 {
			return rowSpacing
		}
		total := 0.0
		for i, id := range ids {
			total += calcSubtreeHeight(id)
			if i < len(ids)-1 {
				total += siblingGap
			}
		}
		return total
	}
	leftHeight := calcSideHeight(sideChildren[-1])
	rightHeight := calcSideHeight(sideChildren[1])
	centerY := paddingY + max(leftHeight, rightHeight)/2 + 36

	yCenter := map[string]float64{rootID: centerY}
	var placeSubtree func(string, float64)
	placeSubtree = func(id string, topY float64) {
		kids := children[id]
		if len(kids) == 0 {
			yCenter[id] = topY + max(rowSpacing, nodeSize[id].Y+10)/2
			return
		}
		current := topY
		for i, childID := range kids {
			placeSubtree(childID, current)
			current += calcSubtreeHeight(childID)
			if i < len(kids)-1 {
				current += siblingGap
			}
		}
		first := kids[0]
		last := kids[len(kids)-1]
		yCenter[id] = (yCenter[first] + yCenter[last]) / 2
	}
	placeSide := func(ids []string, sideHeight float64) {
		if len(ids) == 0 {
			return
		}
		y := centerY - sideHeight/2
		for i, id := range ids {
			placeSubtree(id, y)
			y += calcSubtreeHeight(id)
			if i < len(ids)-1 {
				y += siblingGap
			}
		}
	}
	placeSide(sideChildren[-1], leftHeight)
	placeSide(sideChildren[1], rightHeight)

	leftExtent := 0.0
	rightExtent := 0.0
	for _, node := range graph.MindmapNodes {
		if node.ID == rootID {
			continue
		}
		extent := float64(depth[node.ID])*levelSpacing + nodeSize[node.ID].X/2.0
		if side[node.ID] < 0 {
			leftExtent = max(leftExtent, extent)
		} else {
			rightExtent = max(rightExtent, extent)
		}
	}
	rootHalfW := nodeSize[rootID].X / 2.0
	centerX := paddingX + leftExtent + rootHalfW + 12.0
	nodePos := map[string]Point{}
	maxX := 0.0
	maxY := 0.0
	for _, node := range graph.MindmapNodes {
		shape := node.Shape
		if shape == "" {
			shape = ShapeRoundRect
		}
		d := depth[node.ID]
		s := side[node.ID]
		cx := centerX + float64(s)*float64(d)*levelSpacing
		if node.ID == rootID || s == 0 {
			cx = centerX
		}
		w := nodeSize[node.ID].X
		h := nodeSize[node.ID].Y
		x := cx - w/2
		y := yCenter[node.ID] - h/2
		nodePos[node.ID] = Point{X: x, Y: y}
		layout.Nodes = append(layout.Nodes, NodeLayout{
			ID:    node.ID,
			Label: node.Label,
			Shape: shape,
			X:     x,
			Y:     y,
			W:     w,
			H:     h,
		})
		maxX = max(maxX, x+w)
		maxY = max(maxY, y+h)
	}

	nodeLayoutByID := map[string]NodeLayout{}
	for _, node := range layout.Nodes {
		nodeLayoutByID[node.ID] = node
	}
	for _, node := range graph.MindmapNodes {
		if node.Parent == "" {
			continue
		}
		parent, okParent := nodeLayoutByID[node.Parent]
		child, okChild := nodeLayoutByID[node.ID]
		if !okParent || !okChild {
			continue
		}
		x1 := parent.X + parent.W
		x2 := child.X
		if child.X+child.W/2 < parent.X+parent.W/2 {
			x1 = parent.X
			x2 = child.X + child.W
		}
		layout.Lines = append(layout.Lines, LayoutLine{
			X1:          x1,
			Y1:          parent.Y + parent.H/2,
			X2:          x2,
			Y2:          child.Y + child.H/2,
			Stroke:      theme.LineColor,
			StrokeWidth: 2,
			ArrowEnd:    false,
		})
	}

	for _, node := range layout.Nodes {
		addNodePrimitive(&layout, theme, graph.Kind, node)
		layout.Texts = append(layout.Texts, LayoutText{
			X:      node.X + node.W/2,
			Y:      node.Y + node.H/2 + theme.FontSize*0.35,
			Value:  node.Label,
			Anchor: "middle",
			Size:   theme.FontSize,
			Color:  theme.PrimaryTextColor,
		})
	}

	layout.Width = maxX + paddingX
	layout.Height = maxY + paddingY
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
