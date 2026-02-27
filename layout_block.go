package mermaid

import "math"

func layoutBlockFidelity(graph *Graph, theme Theme, config LayoutConfig) Layout {
	if len(graph.BlockRows) == 0 {
		return layoutGraphLike(graph, theme, config)
	}

	layout := Layout{Kind: graph.Kind}

	columns := graph.BlockColumns
	if columns <= 0 {
		for _, row := range graph.BlockRows {
			columns = max(columns, len(row))
		}
	}
	if columns <= 0 {
		columns = 1
	}

	const (
		cellW       = 96.4140625
		cellH       = 92.71875
		cellGapX    = 8.0
		cellGapY    = 8.0
		cylW        = 26.8671875
		cylH        = 39.76842944595916
		viewPadding = 5.0
	)

	colStep := cellW + cellGapX
	rowStep := cellH + cellGapY
	firstCenterX := cellW / 2
	firstCenterY := -((float64(len(graph.BlockRows)) - 1) * rowStep) / 2

	type nodeGeom struct {
		CenterX float64
		CenterY float64
		W       float64
		H       float64
	}
	nodeByID := map[string]nodeGeom{}

	for rowIdx, row := range graph.BlockRows {
		for colIdx, id := range row {
			node, ok := graph.Nodes[id]
			if !ok {
				continue
			}
			cx := firstCenterX + float64(colIdx)*colStep
			cy := firstCenterY + float64(rowIdx)*rowStep
			w := cellW
			h := cellH
			if node.Shape == ShapeCylinder {
				w = cylW
				h = cylH
			}
			layout.Nodes = append(layout.Nodes, NodeLayout{
				ID:    id,
				Label: node.Label,
				Shape: node.Shape,
				X:     cx - w/2,
				Y:     cy - h/2,
				W:     w,
				H:     h,
			})
			nodeByID[id] = nodeGeom{
				CenterX: cx,
				CenterY: cy,
				W:       w,
				H:       h,
			}
		}
	}

	minX := math.MaxFloat64
	minY := math.MaxFloat64
	maxX := -math.MaxFloat64
	maxY := -math.MaxFloat64
	for _, node := range layout.Nodes {
		minX = min(minX, node.X)
		minY = min(minY, node.Y)
		maxX = max(maxX, node.X+node.W)
		maxY = max(maxY, node.Y+node.H)
	}
	if len(layout.Nodes) == 0 {
		minX = 0
		minY = 0
		maxX = cellW
		maxY = cellH
	}

	for _, edge := range graph.Edges {
		fromNode, okFrom := nodeByID[edge.From]
		toNode, okTo := nodeByID[edge.To]
		if !okFrom || !okTo {
			continue
		}
		dx := toNode.CenterX - fromNode.CenterX
		dy := toNode.CenterY - fromNode.CenterY
		sx := fromNode.CenterX
		sy := fromNode.CenterY
		ex := toNode.CenterX
		ey := toNode.CenterY
		if math.Abs(dx) >= math.Abs(dy) {
			sign := 1.0
			if dx < 0 {
				sign = -1
			}
			sx = fromNode.CenterX + sign*fromNode.W/2
			ex = toNode.CenterX - sign*toNode.W/2
		} else {
			sign := 1.0
			if dy < 0 {
				sign = -1
			}
			sy = fromNode.CenterY + sign*fromNode.H/2
			ey = toNode.CenterY - sign*toNode.H/2
		}
		layout.Edges = append(layout.Edges, EdgeLayout{
			From:     edge.From,
			To:       edge.To,
			Label:    edge.Label,
			X1:       sx,
			Y1:       sy,
			X2:       ex,
			Y2:       ey,
			Style:    edge.Style,
			ArrowEnd: true,
		})
	}

	layout.ViewBoxX = minX - viewPadding
	layout.ViewBoxY = minY - viewPadding
	layout.ViewBoxWidth = (maxX - minX) + viewPadding*2
	layout.ViewBoxHeight = (maxY - minY) + viewPadding*2
	layout.Width = layout.ViewBoxWidth
	layout.Height = layout.ViewBoxHeight
	layout.SVGStyle = "max-width: " + formatFloat(layout.ViewBoxWidth) + "px; background-color: white;"
	return layout
}
