package mermaid

import (
	"math"
	"strings"
)

const (
	c4DiagramMarginX   = 50.0
	c4DiagramMarginY   = 10.0
	c4ShapeMargin      = 50.0
	c4ShapePadding     = 20.0
	c4DefaultNodeWidth = 216.0
	c4DefaultNodeH     = 60.0
	c4ShapeInRow       = 4
	c4WidthLimit       = 800.0
	c4TitleExtraHeight = 60.0
	c4TypeFontSize     = 12.0
	c4LabelFontSize    = 16.0
	c4BodyFontSize     = 14.0
	c4TypeLineHeight   = 14.0
	c4LabelLineHeight  = 22.0
	c4BodyLineHeight   = 19.0
	c4NodeStartX       = 100.0
	c4NodeStartY       = 74.0
)

type c4NodeInfo struct {
	RawType      string
	DisplayType  string
	Name         string
	Description  []string
	IsPerson     bool
	IsExternal   bool
	IconDataHref string
}

type c4NodeMetrics struct {
	Width       float64
	Height      float64
	TypeTextW   float64
	TypeY       float64
	ImageY      float64
	LabelY      float64
	LabelHeight float64
	DescrY      float64
	DescrHeight float64
}

type c4Bounds struct {
	startX     float64
	stopX      float64
	startY     float64
	stopY      float64
	widthLimit float64

	nextStartX float64
	nextStopX  float64
	nextStartY float64
	nextStopY  float64
	nextCount  int
}

func newC4Bounds(startX, startY, widthLimit float64) *c4Bounds {
	return &c4Bounds{
		startX:     startX,
		stopX:      startX,
		startY:     startY,
		stopY:      startY,
		widthLimit: widthLimit,
		nextStartX: startX,
		nextStopX:  startX,
		nextStartY: startY,
		nextStopY:  startY,
	}
}

func (b *c4Bounds) insert(width, height float64) (float64, float64) {
	b.nextCount++

	startX := b.nextStopX + c4ShapeMargin
	if b.nextStartX != b.nextStopX {
		startX = b.nextStopX + c4ShapeMargin*2
	}
	stopX := startX + width
	startY := b.nextStartY + c4ShapeMargin*2
	stopY := startY + height

	if startX >= b.widthLimit || stopX >= b.widthLimit || b.nextCount > c4ShapeInRow {
		startX = b.nextStartX + c4ShapeMargin
		startY = b.nextStopY + c4ShapeMargin*2
		stopX = startX + width
		stopY = startY + height
		b.nextStartY = b.nextStopY
		b.nextStopX = stopX
		b.nextStopY = stopY
		b.nextCount = 1
	} else {
		b.nextStopX = math.Max(b.nextStopX, stopX)
		b.nextStopY = math.Max(b.nextStopY, stopY)
	}

	b.startX = math.Min(b.startX, startX)
	b.startY = math.Min(b.startY, startY)
	b.stopX = math.Max(b.stopX, stopX)
	b.stopY = math.Max(b.stopY, stopY)
	b.nextStartX = math.Min(b.nextStartX, startX)

	return startX, startY
}

func (b *c4Bounds) bumpLastMargin() {
	b.stopX += c4ShapeMargin
	b.stopY += c4ShapeMargin
}

func parseC4NodeInfo(label string, fallbackID string, shape NodeShape) c4NodeInfo {
	lines := splitLinesPreserve(label)
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cleaned = append(cleaned, line)
	}

	info := c4NodeInfo{
		Name:        strings.TrimSpace(fallbackID),
		Description: []string{},
	}
	if len(cleaned) > 0 && strings.HasPrefix(cleaned[0], "<<") && strings.HasSuffix(cleaned[0], ">>") {
		info.DisplayType = cleaned[0]
		raw := strings.TrimSuffix(strings.TrimPrefix(cleaned[0], "<<"), ">>")
		info.RawType = lower(strings.TrimSpace(raw))
		cleaned = cleaned[1:]
	}
	if info.RawType == "" && shape == ShapePerson {
		info.RawType = "person"
		info.DisplayType = "<<person>>"
	}
	if len(cleaned) > 0 {
		info.Name = cleaned[0]
		if len(cleaned) > 1 {
			info.Description = append(info.Description, cleaned[1:]...)
		}
	}
	info.IsPerson = strings.Contains(info.RawType, "person")
	info.IsExternal = strings.Contains(info.RawType, "external")
	return info
}

func measureC4Node(info c4NodeInfo, fast bool) c4NodeMetrics {
	typeText := info.DisplayType
	nameLines := splitLinesPreserve(info.Name)
	if len(nameLines) == 0 {
		nameLines = []string{""}
	}
	descLines := make([]string, 0, len(info.Description))
	for _, line := range info.Description {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		descLines = append(descLines, line)
	}

	maxTextWidth := 0.0
	for _, line := range nameLines {
		maxTextWidth = math.Max(maxTextWidth, measureTextWidthWithFontSize(line, c4LabelFontSize, fast))
	}
	for _, line := range descLines {
		maxTextWidth = math.Max(maxTextWidth, measureTextWidthWithFontSize(line, c4BodyFontSize, fast))
	}

	labelHeight := c4LabelLineHeight * float64(max(1, len(nameLines)))
	descrHeight := c4BodyLineHeight * float64(len(descLines))
	typeTextW := 0.0
	if strings.TrimSpace(typeText) != "" {
		typeTextW = measureTextWidthWithFontSize(typeText, c4TypeFontSize, fast)
	}

	y := c4ShapePadding + c4TypeLineHeight - 4
	imageY := 0.0
	if info.IsPerson {
		imageY = y
		y = imageY + 48
	}

	labelY := y + 8
	y = labelY + labelHeight

	descrY := 0.0
	height := math.Max(c4DefaultNodeH, y)
	if len(descLines) > 0 {
		descrY = y + 20
		y = descrY + descrHeight
		height = math.Max(c4DefaultNodeH, y-float64(len(descLines))*5)
	}

	width := math.Max(c4DefaultNodeWidth, maxTextWidth+c4ShapePadding)
	return c4NodeMetrics{
		Width:       width,
		Height:      height,
		TypeTextW:   typeTextW,
		TypeY:       c4ShapePadding,
		ImageY:      imageY,
		LabelY:      labelY,
		LabelHeight: labelHeight,
		DescrY:      descrY,
		DescrHeight: descrHeight,
	}
}

func c4NodeColors(info c4NodeInfo, fill, stroke string) (string, string) {
	switch info.RawType {
	case "external_person":
		return "#686868", "#8A8A8A"
	case "external_system", "external_container", "external_component", "external_system_db", "external_container_db", "external_component_db":
		return "#999999", "#8A8A8A"
	case "person":
		return "#08427B", "#073B6F"
	case "system", "container", "component", "system_db", "container_db", "component_db":
		return "#1168BD", "#3C7FC0"
	}
	if strings.TrimSpace(fill) != "" || strings.TrimSpace(stroke) != "" {
		return defaultColor(fill, "#1168BD"), defaultColor(stroke, "#3C7FC0")
	}
	if info.IsPerson && info.IsExternal {
		return "#686868", "#8A8A8A"
	}
	if info.IsExternal {
		return "#999999", "#8A8A8A"
	}
	if info.IsPerson {
		return "#08427B", "#073B6F"
	}
	return "#1168BD", "#3C7FC0"
}

type c4Point struct {
	X float64
	Y float64
}

func c4IntersectPoint(from NodeLayout, end c4Point) c4Point {
	x1 := from.X
	y1 := from.Y
	x2 := end.X
	y2 := end.Y
	centerX := x1 + from.W/2
	centerY := y1 + from.H/2
	dx := math.Abs(x1 - x2)
	dy := math.Abs(y1 - y2)
	tan := dy / math.Max(dx, 0.000001)
	fromSlope := from.H / math.Max(from.W, 0.000001)
	point := c4Point{X: centerX, Y: centerY}

	switch {
	case y1 == y2 && x1 < x2:
		return c4Point{X: x1 + from.W, Y: centerY}
	case y1 == y2 && x1 > x2:
		return c4Point{X: x1, Y: centerY}
	case x1 == x2 && y1 < y2:
		return c4Point{X: centerX, Y: y1 + from.H}
	case x1 == x2 && y1 > y2:
		return c4Point{X: centerX, Y: y1}
	case x1 > x2 && y1 < y2:
		if fromSlope >= tan {
			point = c4Point{X: x1, Y: centerY + (tan*from.W)/2}
		} else {
			point = c4Point{X: centerX - ((dx/dy)*from.H)/2, Y: y1 + from.H}
		}
	case x1 < x2 && y1 < y2:
		if fromSlope >= tan {
			point = c4Point{X: x1 + from.W, Y: centerY + (tan*from.W)/2}
		} else {
			point = c4Point{X: centerX + ((dx/dy)*from.H)/2, Y: y1 + from.H}
		}
	case x1 < x2 && y1 > y2:
		if fromSlope >= tan {
			point = c4Point{X: x1 + from.W, Y: centerY - (tan*from.W)/2}
		} else {
			point = c4Point{X: centerX + ((from.H / 2) * dx / dy), Y: y1}
		}
	case x1 > x2 && y1 > y2:
		if fromSlope >= tan {
			point = c4Point{X: x1, Y: centerY - (from.W/2)*tan}
		} else {
			point = c4Point{X: centerX - ((from.H / 2) * dx / dy), Y: y1}
		}
	}
	return point
}

func c4IntersectPoints(from, to NodeLayout) (c4Point, c4Point) {
	toCenter := c4Point{X: to.X + to.W/2, Y: to.Y + to.H/2}
	fromPoint := c4IntersectPoint(from, toCenter)
	fromCenter := c4Point{X: from.X + from.W/2, Y: from.Y + from.H/2}
	toPoint := c4IntersectPoint(to, fromCenter)
	return fromPoint, toPoint
}

func layoutC4(graph *Graph, theme Theme, config LayoutConfig) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.NodeOrder) == 0 {
		layout.Width = 400
		layout.Height = 60
		layout.ViewBoxX = 0
		layout.ViewBoxY = -10
		layout.ViewBoxWidth = layout.Width
		layout.ViewBoxHeight = layout.Height + 10
		if strings.TrimSpace(graph.C4Title) != "" {
			layout.Texts = append(layout.Texts, LayoutText{
				Class: "c4-title",
				X:     16,
				Y:     20,
				Value: graph.C4Title,
				Color: theme.PrimaryTextColor,
			})
		}
		return layout
	}

	screenStartX := c4DiagramMarginX
	screenStartY := c4DiagramMarginY
	boxStopX := screenStartX
	boxStopY := screenStartY

	bounds := newC4Bounds(c4NodeStartX, c4NodeStartY, c4WidthLimit)
	for _, id := range graph.NodeOrder {
		node := graph.Nodes[id]
		info := parseC4NodeInfo(node.Label, id, node.Shape)
		metrics := measureC4Node(info, config.FastTextMetrics)
		x, y := bounds.insert(metrics.Width, metrics.Height)
		fill, stroke := c4NodeColors(info, node.Fill, node.Stroke)
		layout.Nodes = append(layout.Nodes, NodeLayout{
			ID:          id,
			Label:       node.Label,
			Shape:       node.Shape,
			X:           x,
			Y:           y,
			W:           metrics.Width,
			H:           metrics.Height,
			Fill:        fill,
			Stroke:      stroke,
			StrokeWidth: 0.5,
		})
	}
	bounds.bumpLastMargin()
	boxStopX = math.Max(boxStopX, bounds.stopX+c4ShapeMargin)
	boxStopY = math.Max(boxStopY, bounds.stopY+c4ShapeMargin)

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
		start, end := c4IntersectPoints(from, to)
		layout.Edges = append(layout.Edges, EdgeLayout{
			From:        edge.From,
			To:          edge.To,
			Label:       edge.Label,
			X1:          start.X,
			Y1:          start.Y,
			X2:          end.X,
			Y2:          end.Y,
			Style:       edge.Style,
			ArrowStart:  edge.ArrowStart,
			ArrowEnd:    edge.ArrowEnd || edge.Directed,
			MarkerStart: edge.MarkerStart,
			MarkerEnd:   edge.MarkerEnd,
		})
	}

	if strings.TrimSpace(graph.C4Title) != "" {
		layout.Texts = append(layout.Texts, LayoutText{
			Class: "c4-title",
			X:     (boxStopX-screenStartX)/2 - 4*c4DiagramMarginX,
			Y:     screenStartY + c4DiagramMarginY,
			Value: graph.C4Title,
			Color: theme.PrimaryTextColor,
		})
	}

	width := (boxStopX - screenStartX) + 2*c4DiagramMarginX
	height := (boxStopY - screenStartY) + 2*c4DiagramMarginY
	extraVertForTitle := 0.0
	if strings.TrimSpace(graph.C4Title) != "" {
		extraVertForTitle = c4TitleExtraHeight
	}
	layout.Width = width
	layout.Height = height
	layout.ViewBoxX = screenStartX - c4DiagramMarginX
	layout.ViewBoxY = -(c4DiagramMarginY + extraVertForTitle)
	layout.ViewBoxWidth = width
	layout.ViewBoxHeight = height + extraVertForTitle
	return layout
}
