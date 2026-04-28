package mermaid

import (
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/dagre"
)

type erNodeMetrics struct {
	attrs     []parsedERAttr
	colWidths []float64
	size      Point
}

func layoutERDiagramFidelity(graph *Graph, theme Theme, config LayoutConfig) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.NodeOrder) == 0 {
		return layoutGeneric(graph, theme)
	}

	metrics := measureERNodeMetrics(graph, theme, config)
	dg := dagre.NewGraph()
	dg.SetGraph(dagre.GraphLabel{
		RankDir: erRankDir(graph.Direction),
		NodeSep: max(100, config.NodeSpacing*2),
		EdgeSep: 100,
		RankSep: max(80, config.RankSpacing*1.6),
		MarginX: 8,
		MarginY: 8,
	})

	for _, id := range graph.NodeOrder {
		size := metrics[id].size
		dg.SetNode(id, &dagre.NodeLabel{Width: size.X, Height: size.Y})
	}

	for i, edge := range graph.Edges {
		if edge.From == "" || edge.To == "" {
			continue
		}
		labelW := 0.0
		labelH := 0.0
		if strings.TrimSpace(edge.Label) != "" {
			labelW = measureTextWidthWithFontSize(edge.Label, max(11, theme.FontSize-2), config.FastTextMetrics) + 10
			labelH = max(21, theme.FontSize*1.35)
		}
		dg.SetEdge(dagre.Edge{V: edge.From, W: edge.To, Name: strconv.Itoa(i)}, &dagre.EdgeLabel{
			MinLen:   1,
			Weight:   1,
			Width:    labelW,
			Height:   labelH,
			LabelPos: "c",
		})
	}

	dagre.Layout(dg)

	minX := 1e9
	minY := 1e9
	maxX := -1e9
	maxY := -1e9

	nodeIndex := map[string]NodeLayout{}
	for _, id := range graph.NodeOrder {
		dn := dg.Node(id)
		if dn == nil {
			continue
		}
		node := NodeLayout{
			ID:    id,
			Label: graph.Nodes[id].Label,
			Shape: ShapeRectangle,
			X:     dn.X - dn.Width/2,
			Y:     dn.Y - dn.Height/2,
			W:     dn.Width,
			H:     dn.Height,
		}
		layout.Nodes = append(layout.Nodes, node)
		nodeIndex[id] = node
		minX = min(minX, node.X)
		minY = min(minY, node.Y)
		maxX = max(maxX, node.X+node.W)
		maxY = max(maxY, node.Y+node.H)
	}

	for i, edge := range graph.Edges {
		if edge.From == "" || edge.To == "" {
			continue
		}
		from, okFrom := nodeIndex[edge.From]
		to, okTo := nodeIndex[edge.To]
		if okFrom && okTo {
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
				MarkerStart: edge.MarkerStart,
				MarkerEnd:   edge.MarkerEnd,
			})
		}

		dl := dg.EdgeByKey(dagre.Edge{V: edge.From, W: edge.To, Name: strconv.Itoa(i)})
		if dl == nil || len(dl.Points) == 0 {
			continue
		}
		layout.Paths = append(layout.Paths, LayoutPath{
			ID:          "L_" + sanitizeID(edge.From, edge.From) + "_" + sanitizeID(edge.To, edge.To) + "_" + intString(i),
			Class:       erRelationshipClass(edge.Style),
			D:           dagreEdgePath(dl.Points),
			Fill:        "none",
			Stroke:      theme.LineColor,
			StrokeWidth: 1.2,
			DashArray:   erRelationshipDash(edge.Style),
			LineCap:     "round",
			LineJoin:    "round",
			MarkerStart: edge.MarkerStart,
			MarkerEnd:   edge.MarkerEnd,
		})
		for _, point := range dl.Points {
			minX = min(minX, point.X)
			minY = min(minY, point.Y)
			maxX = max(maxX, point.X)
			maxY = max(maxY, point.Y)
		}
		if strings.TrimSpace(edge.Label) != "" && dl.HasX && dl.HasY {
			labelW := measureTextWidthWithFontSize(edge.Label, max(11, theme.FontSize-2), config.FastTextMetrics) + 8
			labelH := max(21, theme.FontSize*1.35)
			layout.Texts = append(layout.Texts, LayoutText{
				ID:     "L_" + sanitizeID(edge.From, edge.From) + "_" + sanitizeID(edge.To, edge.To) + "_" + intString(i),
				Class:  "edgeLabel",
				X:      dl.X,
				Y:      dl.Y,
				Value:  edge.Label,
				Anchor: "middle",
				Size:   max(11, theme.FontSize-2),
				Color:  theme.PrimaryTextColor,
			})
			minX = min(minX, dl.X-labelW/2)
			minY = min(minY, dl.Y-labelH/2)
			maxX = max(maxX, dl.X+labelW/2)
			maxY = max(maxY, dl.Y+labelH/2)
		}
	}

	appendERNodePrimitives(&layout, graph, metrics, theme)

	viewBoxPad := 8.0
	layout.ViewBoxX = max(0, minX-viewBoxPad)
	layout.ViewBoxY = max(0, minY-viewBoxPad)
	layout.ViewBoxWidth = (maxX - layout.ViewBoxX) + viewBoxPad
	layout.ViewBoxHeight = (maxY - layout.ViewBoxY) + viewBoxPad
	layout.Width = layout.ViewBoxWidth
	layout.Height = layout.ViewBoxHeight
	applyAspectRatio(&layout, config.PreferredAspectRatio)
	return layout
}

func measureERNodeMetrics(graph *Graph, theme Theme, config LayoutConfig) map[string]erNodeMetrics {
	const (
		entityPadding  = 25.0
		minEntityWidth = 100.0
		minEntityH     = 84.0
		titleH         = 42.75
		rowH           = 42.75
	)

	attrFontSize := max(10.0, theme.FontSize-2)
	out := map[string]erNodeMetrics{}
	for _, id := range graph.NodeOrder {
		label := graph.Nodes[id].Label
		metrics := erNodeMetrics{}
		nameW := max(minEntityWidth, measureTextWidthWithFontSize(label, theme.FontSize, config.FastTextMetrics)+40)
		maxTypeW := 0.0
		maxNameW := 0.0
		maxKeyW := 0.0
		maxCommentW := 0.0

		for _, attr := range graph.ERAttributes[id] {
			parsed := parseERAttribute(attr)
			metrics.attrs = append(metrics.attrs, parsed)
			maxTypeW = max(maxTypeW, measureTextWidthWithFontSize(parsed.t, attrFontSize, config.FastTextMetrics))
			maxNameW = max(maxNameW, measureTextWidthWithFontSize(parsed.n, attrFontSize, config.FastTextMetrics))
			if parsed.k != "" {
				maxKeyW = max(maxKeyW, measureTextWidthWithFontSize(parsed.k, attrFontSize, config.FastTextMetrics))
			}
			if parsed.c != "" {
				maxCommentW = max(maxCommentW, measureTextWidthWithFontSize(parsed.c, attrFontSize, config.FastTextMetrics))
			}
		}

		if len(metrics.attrs) == 0 {
			metrics.size = Point{
				X: nameW,
				Y: max(minEntityH, titleH*2-1.5),
			}
			out[id] = metrics
			continue
		}

		colWidths := []float64{
			maxTypeW + entityPadding,
			maxNameW + entityPadding,
			0,
			0,
		}
		if maxKeyW > 0 {
			colWidths[2] = maxKeyW + entityPadding
		}
		if maxCommentW > 0 {
			colWidths[3] = maxCommentW + entityPadding
		}

		totalW := colWidths[0] + colWidths[1] + colWidths[2] + colWidths[3] + entityPadding
		if totalW < nameW {
			activeCols := 2.0
			if colWidths[2] > 0 {
				activeCols++
			}
			if colWidths[3] > 0 {
				activeCols++
			}
			extra := (nameW - totalW) / activeCols
			for i := range colWidths {
				if colWidths[i] == 0 {
					continue
				}
				colWidths[i] += extra
			}
			totalW = nameW
		}

		metrics.colWidths = colWidths
		metrics.size = Point{
			X: max(minEntityWidth, totalW),
			Y: titleH + float64(len(metrics.attrs))*rowH,
		}
		out[id] = metrics
	}
	return out
}

func appendERNodePrimitives(layout *Layout, graph *Graph, metrics map[string]erNodeMetrics, theme Theme) {
	const (
		entityPadding = 25.0
		titleH        = 42.75
		rowH          = 42.75
		erStroke      = "#9370DB"
		erFill        = "#ececff"
	)

	for _, node := range layout.Nodes {
		layout.Rects = append(layout.Rects, LayoutRect{
			X:           node.X,
			Y:           node.Y,
			W:           node.W,
			H:           node.H,
			Fill:        erFill,
			Stroke:      erStroke,
			StrokeWidth: 1,
			Class:       "outer-path",
		})

		titleY := node.Y + titleH/2 + theme.FontSize*0.35 - 2
		if len(metrics[node.ID].attrs) == 0 {
			titleY = node.Y + node.H/2 + theme.FontSize*0.35
		}
		layout.Texts = append(layout.Texts, LayoutText{
			X:      node.X + node.W/2,
			Y:      titleY,
			Value:  node.Label,
			Anchor: "middle",
			Size:   theme.FontSize,
			Color:  theme.PrimaryTextColor,
			Class:  "label name",
		})

		if len(metrics[node.ID].attrs) == 0 {
			continue
		}

		attrY := node.Y + titleH
		colWidths := metrics[node.ID].colWidths
		for i, attr := range metrics[node.ID].attrs {
			fillClass := "row-rect-odd"
			fillColor := "#ffffff"
			if (i+1)%2 == 0 {
				fillClass = "row-rect-even"
				fillColor = "#f2f2f2"
			}
			layout.Rects = append(layout.Rects, LayoutRect{
				X:           node.X,
				Y:           attrY,
				W:           node.W,
				H:           rowH,
				Fill:        fillColor,
				Stroke:      "none",
				StrokeWidth: 0,
				Class:       fillClass,
			})
			layout.Lines = append(layout.Lines, LayoutLine{
				X1:          node.X,
				Y1:          attrY,
				X2:          node.X + node.W,
				Y2:          attrY,
				Stroke:      erStroke,
				StrokeWidth: 1,
				Class:       "divider",
			})

			textY := attrY + rowH/2 + theme.FontSize*0.35 - 2
			curX := node.X + entityPadding/2
			if colWidths[0] > 0 {
				layout.Texts = append(layout.Texts, LayoutText{X: curX, Y: textY, Value: attr.t, Anchor: "start", Size: max(10, theme.FontSize-2), Color: theme.PrimaryTextColor, Class: "label attribute-type"})
				curX += colWidths[0]
			}
			if colWidths[1] > 0 {
				layout.Texts = append(layout.Texts, LayoutText{X: curX, Y: textY, Value: attr.n, Anchor: "start", Size: max(10, theme.FontSize-2), Color: theme.PrimaryTextColor, Class: "label attribute-name"})
				curX += colWidths[1]
			}
			if colWidths[2] > 0 {
				layout.Texts = append(layout.Texts, LayoutText{X: curX, Y: textY, Value: attr.k, Anchor: "start", Size: max(10, theme.FontSize-2), Color: theme.PrimaryTextColor, Class: "label attribute-keys"})
				curX += colWidths[2]
			}
			if colWidths[3] > 0 {
				layout.Texts = append(layout.Texts, LayoutText{X: curX, Y: textY, Value: attr.c, Anchor: "start", Size: max(10, theme.FontSize-2), Color: theme.PrimaryTextColor, Class: "label attribute-comment"})
			}
			attrY += rowH
		}

		curX := node.X + colWidths[0]
		if colWidths[0] > 0 {
			layout.Lines = append(layout.Lines, LayoutLine{
				X1:          curX,
				Y1:          node.Y + titleH,
				X2:          curX,
				Y2:          node.Y + node.H,
				Stroke:      erStroke,
				StrokeWidth: 1,
				Class:       "divider",
			})
		}
		curX += colWidths[1]
		if colWidths[1] > 0 && curX < node.X+node.W-1 {
			layout.Lines = append(layout.Lines, LayoutLine{
				X1:          curX,
				Y1:          node.Y + titleH,
				X2:          curX,
				Y2:          node.Y + node.H,
				Stroke:      erStroke,
				StrokeWidth: 1,
				Class:       "divider",
			})
		}
		curX += colWidths[2]
		if colWidths[2] > 0 && curX < node.X+node.W-1 {
			layout.Lines = append(layout.Lines, LayoutLine{
				X1:          curX,
				Y1:          node.Y + titleH,
				X2:          curX,
				Y2:          node.Y + node.H,
				Stroke:      erStroke,
				StrokeWidth: 1,
				Class:       "divider",
			})
		}
	}
}

func dagreEdgePath(points []dagre.Point) string {
	if len(points) == 0 {
		return ""
	}
	if len(points) == 1 {
		return "M " + formatFloat(points[0].X) + " " + formatFloat(points[0].Y)
	}
	if len(points) == 2 {
		return "M " + formatFloat(points[0].X) + " " + formatFloat(points[0].Y) +
			" L " + formatFloat(points[1].X) + " " + formatFloat(points[1].Y)
	}
	d := "M " + formatFloat(points[0].X) + " " + formatFloat(points[0].Y)
	for i := 0; i < len(points)-1; i++ {
		p0 := points[max(0, i-1)]
		p1 := points[i]
		p2 := points[i+1]
		p3 := points[min(len(points)-1, i+2)]
		cp1x := p1.X + (p2.X-p0.X)/6
		cp1y := p1.Y + (p2.Y-p0.Y)/6
		cp2x := p2.X - (p3.X-p1.X)/6
		cp2y := p2.Y - (p3.Y-p1.Y)/6
		d += " C " + formatFloat(cp1x) + " " + formatFloat(cp1y) +
			" " + formatFloat(cp2x) + " " + formatFloat(cp2y) +
			" " + formatFloat(p2.X) + " " + formatFloat(p2.Y)
	}
	return d
}

func erRankDir(direction Direction) string {
	switch direction {
	case DirectionBottomTop:
		return "BT"
	case DirectionLeftRight:
		return "LR"
	case DirectionRightLeft:
		return "RL"
	default:
		return "TB"
	}
}

func erRelationshipClass(style EdgeStyle) string {
	if style == EdgeDotted {
		return "edge-thickness-normal edge-pattern-dashed relationshipLine"
	}
	return "edge-thickness-normal edge-pattern-solid relationshipLine"
}

func erRelationshipDash(style EdgeStyle) string {
	if style == EdgeDotted {
		return "8,8"
	}
	return ""
}
