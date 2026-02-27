package mermaid

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type measuredText struct {
	Lines  []string
	Width  float64
	Height float64
}

func measureTextWidthWithFontSize(text string, fontSize float64, fast bool, fontFamily ...string) float64 {
	if text == "" || fontSize <= 0 {
		return 0
	}
	if fast {
		return float64(len([]rune(text))) * fontSize * 0.5
	}
	family := ""
	if len(fontFamily) > 0 {
		family = fontFamily[0]
	}
	if width, ok := measureNativeTextWidth(text, fontSize, family); ok {
		return width
	}
	sum := 0.0
	for _, r := range text {
		if r == '\n' || r == '\r' {
			continue
		}
		sum += runeWidthScale(r)
	}
	return sum * fontSize
}

func runeWidthScale(r rune) float64 {
	switch {
	case r == '\t':
		return 2.0
	case r == ' ':
		return 0.33
	case strings.ContainsRune("ilI|!.,:;`'\"", r):
		return 0.28
	case strings.ContainsRune("fjrt()[]{}", r):
		return 0.36
	case strings.ContainsRune("mwMW@#%&", r):
		return 0.88
	case unicode.IsUpper(r):
		return 0.66
	case unicode.IsDigit(r):
		return 0.56
	case unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r):
		return 1.0
	default:
		return 0.50
	}
}

func splitLinesPreserve(text string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func measureTextBlockWithFontSize(
	text string,
	fontSize float64,
	lineHeight float64,
	fast bool,
	fontFamily ...string,
) measuredText {
	lines := splitLinesPreserve(text)
	width := 0.0
	for _, line := range lines {
		w := measureTextWidthWithFontSize(line, fontSize, fast, fontFamily...)
		if w > width {
			width = w
		}
	}
	height := float64(len(lines)) * fontSize * lineHeight
	return measuredText{
		Lines:  lines,
		Width:  width,
		Height: height,
	}
}

func appendTextBlock(
	layout *Layout,
	x float64,
	y float64,
	block measuredText,
	theme Theme,
	config LayoutConfig,
	fontSize float64,
	anchor string,
	color string,
	baseline bool,
	weight string,
) {
	if len(block.Lines) == 0 {
		return
	}
	totalHeight := float64(len(block.Lines)) * fontSize * config.LabelLineHeight
	startY := y
	if !baseline {
		startY = y - totalHeight/2.0 + fontSize
	}
	for idx, line := range block.Lines {
		lineY := startY + float64(idx)*fontSize*config.LabelLineHeight
		layout.Texts = append(layout.Texts, LayoutText{
			X:      x,
			Y:      lineY,
			Value:  line,
			Anchor: anchor,
			Size:   fontSize,
			Weight: maxTextWeight(weight, "400"),
			Color:  color,
		})
	}
}

func maxTextWeight(weight string, fallback string) string {
	if strings.TrimSpace(weight) == "" {
		return fallback
	}
	return weight
}

func formatPieValue(value float64) string {
	rounded := math.Round(value*100.0) / 100.0
	if math.Abs(rounded-math.Round(rounded)) < 0.001 {
		return strconv.FormatInt(int64(math.Round(rounded)), 10)
	}
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(rounded, 'f', 2, 64), "0"), ".")
}

func pieSlicePath(cx, cy, radius, startAngle, endAngle float64) string {
	sx := cx + radius*math.Cos(startAngle)
	sy := cy + radius*math.Sin(startAngle)
	ex := cx + radius*math.Cos(endAngle)
	ey := cy + radius*math.Sin(endAngle)
	largeArc := 0
	if math.Abs(endAngle-startAngle) > math.Pi {
		largeArc = 1
	}
	return "M " + formatFloat(cx) + " " + formatFloat(cy) +
		" L " + formatFloat(sx) + " " + formatFloat(sy) +
		" A " + formatFloat(radius) + " " + formatFloat(radius) + " 0 " + intString(largeArc) + " 1 " +
		formatFloat(ex) + " " + formatFloat(ey) + " Z"
}

func layoutPieFidelity(graph *Graph, theme Theme, config LayoutConfig) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.PieSlices) == 0 {
		return layoutGeneric(graph, theme)
	}

	pieCfg := config.Pie
	titleBlock := measuredText{}
	hasTitle := strings.TrimSpace(graph.PieTitle) != ""
	if hasTitle {
		titleBlock = measureTextBlockWithFontSize(
			graph.PieTitle,
			theme.PieTitleTextSize,
			config.LabelLineHeight,
			config.FastTextMetrics,
		)
	}

	total := 0.0
	for _, slice := range graph.PieSlices {
		total += math.Max(slice.Value, 0.0)
	}
	fallbackTotal := float64(max(1, len(graph.PieSlices)))
	if total <= 0.0 {
		total = fallbackTotal
	}

	type pieDatum struct {
		Index int
		Label string
		Value float64
	}
	filtered := make([]pieDatum, 0, len(graph.PieSlices))
	for idx, slice := range graph.PieSlices {
		value := math.Max(slice.Value, 0.0)
		percent := 0.0
		if total > 0.0 {
			percent = value / total * 100.0
		}
		if percent >= pieCfg.MinPercent {
			filtered = append(filtered, pieDatum{
				Index: idx,
				Label: slice.Label,
				Value: value,
			})
		}
	}
	palette := theme.PieColors
	if len(palette) == 0 {
		palette = []string{"#ECECFF", "#E3E2C8", "#B9EE52", "#CBD5E1"}
	}
	colorMap := map[string]string{}
	colorIndex := 0
	resolveColor := func(label string) string {
		if color, ok := colorMap[label]; ok {
			return color
		}
		color := palette[colorIndex%len(palette)]
		colorIndex++
		colorMap[label] = color
		return color
	}

	type pieSliceLayout struct {
		Label      string
		LabelBlock measuredText
		Value      float64
		StartAngle float64
		EndAngle   float64
		Color      string
	}
	slices := make([]pieSliceLayout, 0, len(filtered))
	// Mermaid starts slices at 12 o'clock and keeps declaration order.
	angle := -math.Pi / 2.0
	for _, datum := range filtered {
		span := 2.0 * math.Pi / fallbackTotal
		if total > 0.0 {
			span = datum.Value / total * 2.0 * math.Pi
		}
		labelBlock := measureTextBlockWithFontSize(
			datum.Label,
			theme.PieSectionTextSize,
			config.LabelLineHeight,
			config.FastTextMetrics,
		)
		color := resolveColor(datum.Label)
		slices = append(slices, pieSliceLayout{
			Label:      datum.Label,
			LabelBlock: labelBlock,
			Value:      datum.Value,
			StartAngle: angle,
			EndAngle:   angle + span,
			Color:      color,
		})
		angle += span
	}

	type pieLegendItem struct {
		X          float64
		Y          float64
		LabelBlock measuredText
		Color      string
		MarkerSize float64
		Value      float64
	}
	legend := make([]pieLegendItem, 0, len(graph.PieSlices))
	legendWidth := 0.0
	legendItems := make([]pieLegendItem, 0, len(graph.PieSlices))
	for _, slice := range graph.PieSlices {
		valueText := formatPieValue(slice.Value)
		labelText := slice.Label
		if graph.PieShowData {
			labelText = slice.Label + " [" + valueText + "]"
		}
		label := measureTextBlockWithFontSize(
			labelText,
			theme.PieLegendTextSize,
			config.LabelLineHeight,
			config.FastTextMetrics,
		)
		legendWidth = max(legendWidth, label.Width)
		legendItems = append(legendItems, pieLegendItem{
			LabelBlock: label,
			Color:      resolveColor(slice.Label),
			MarkerSize: pieCfg.LegendRectSize,
			Value:      slice.Value,
		})
	}

	legendTextHeight := theme.PieLegendTextSize * 1.25
	legendItemHeight := max(pieCfg.LegendRectSize+pieCfg.LegendSpacing, legendTextHeight)
	legendOffset := legendItemHeight * float64(len(legendItems)) / 2.0

	height := max(1.0, pieCfg.Height)
	pieWidth := height
	radius := max(1.0, min(pieWidth, height)/2.0-pieCfg.Margin)
	centerX := pieWidth / 2.0
	centerY := height / 2.0
	legendX := centerX + radius + pieCfg.Margin*0.6

	for idx, item := range legendItems {
		vertical := float64(idx)*legendItemHeight - legendOffset
		item.X = legendX
		item.Y = centerY + vertical
		legend = append(legend, item)
	}

	width := legendX + pieCfg.LegendRectSize + pieCfg.LegendSpacing + legendWidth + pieCfg.Margin*0.4
	layout.Width = max(200.0, width)
	layout.Height = max(1.0, height)

	sliceStroke := theme.Background
	sliceStrokeWidth := max(theme.PieStrokeWidth, 1.2)
	for _, slice := range slices {
		span := math.Abs(slice.EndAngle - slice.StartAngle)
		if span <= 0.0001 {
			continue
		}
		if span >= 2.0*math.Pi-0.001 {
			layout.Circles = append(layout.Circles, LayoutCircle{
				CX:          centerX,
				CY:          centerY,
				R:           radius,
				Fill:        slice.Color,
				Stroke:      sliceStroke,
				StrokeWidth: sliceStrokeWidth,
				Opacity:     theme.PieOpacity,
			})
			continue
		}
		layout.Paths = append(layout.Paths, LayoutPath{
			D:           pieSlicePath(centerX, centerY, radius, slice.StartAngle, slice.EndAngle),
			Fill:        slice.Color,
			Stroke:      sliceStroke,
			StrokeWidth: sliceStrokeWidth,
			Opacity:     theme.PieOpacity,
		})
	}

	if theme.PieOuterStrokeWidth > 0 {
		layout.Circles = append(layout.Circles, LayoutCircle{
			CX:          centerX,
			CY:          centerY,
			R:           radius + theme.PieOuterStrokeWidth/2.0,
			Fill:        "none",
			Stroke:      theme.PieOuterStrokeColor,
			StrokeWidth: theme.PieOuterStrokeWidth,
		})
	}

	type pieLabel struct {
		Text      string
		FontSize  float64
		Outside   bool
		Side      int
		X         float64
		Y         float64
		EdgeX     float64
		EdgeY     float64
		LineColor string
	}
	labels := make([]pieLabel, 0, len(slices))
	suppressOutsideLabels := len(legend) >= 4
	legendTotal := 0.0
	for _, item := range legend {
		legendTotal += max(item.Value, 0.0)
	}
	if legendTotal <= 0 {
		for _, slice := range slices {
			legendTotal += max(slice.Value, 0.0)
		}
	}
	for _, slice := range slices {
		span := math.Abs(slice.EndAngle - slice.StartAngle)
		if span <= 0.0001 || legendTotal <= 0.0 {
			continue
		}
		percent := slice.Value / legendTotal * 100.0
		if percent < pieCfg.MinPercent {
			continue
		}
		percentText := strconv.FormatInt(int64(math.Round(percent)), 10) + "%"
		midAngle := (slice.StartAngle + slice.EndAngle) / 2.0
		fontSize := theme.PieSectionTextSize
		arcLen := radius * span
		percentWidth := measureTextWidthWithFontSize(percentText, fontSize, config.FastTextMetrics)
		outside := !suppressOutsideLabels && (arcLen < percentWidth*1.35 || span < 0.4)
		labelText := percentText
		if outside {
			labelText = strings.Join(slice.LabelBlock.Lines, " ")
		}
		edgeX := centerX + radius*math.Cos(midAngle)
		edgeY := centerY + radius*math.Sin(midAngle)
		bump := max(fontSize*1.6, radius*0.18)
		labelX := centerX + pieCfg.TextPosition*radius*math.Cos(midAngle)
		labelY := centerY + pieCfg.TextPosition*radius*math.Sin(midAngle)
		if outside {
			labelX = centerX + (radius+bump)*math.Cos(midAngle)
			labelY = centerY + (radius+bump)*math.Sin(midAngle)
		}
		side := -1
		if math.Cos(midAngle) >= 0 {
			side = 1
		}
		labels = append(labels, pieLabel{
			Text:      labelText,
			FontSize:  fontSize,
			Outside:   outside,
			Side:      side,
			X:         labelX,
			Y:         labelY,
			EdgeX:     edgeX,
			EdgeY:     edgeY,
			LineColor: slice.Color,
		})
	}

	minY := centerY - radius*1.1
	maxY := centerY + radius*1.1
	minGap := theme.PieSectionTextSize * 1.2
	left := []int{}
	right := []int{}
	for idx, label := range labels {
		if !label.Outside {
			continue
		}
		if label.Side >= 0 {
			right = append(right, idx)
		} else {
			left = append(left, idx)
		}
	}
	distribute := func(indices []int) {
		sort.Slice(indices, func(i, j int) bool {
			return labels[indices[i]].Y < labels[indices[j]].Y
		})
		prev := minY - minGap
		for _, idx := range indices {
			y := max(labels[idx].Y, prev+minGap)
			labels[idx].Y = y
			prev = y
		}
		if len(indices) > 0 {
			last := indices[len(indices)-1]
			overflow := labels[last].Y - maxY
			if overflow > 0 {
				for _, idx := range indices {
					labels[idx].Y -= overflow
				}
			}
			first := indices[0]
			underflow := minY - labels[first].Y
			if underflow > 0 {
				for _, idx := range indices {
					labels[idx].Y += underflow
				}
			}
		}
	}
	distribute(left)
	distribute(right)

	for _, label := range labels {
		anchor := "middle"
		labelX := label.X
		if label.Outside {
			bump := max(label.FontSize*1.6, radius*0.18)
			if label.Side >= 0 {
				labelX = centerX + radius + bump
				anchor = "start"
			} else {
				labelX = centerX - radius - bump
				anchor = "end"
			}
			elbowX := labelX - 6.0
			if label.Side < 0 {
				elbowX = labelX + 6.0
			}
			layout.Paths = append(layout.Paths, LayoutPath{
				D: "M " + formatFloat(label.EdgeX) + "," + formatFloat(label.EdgeY) +
					" L " + formatFloat(elbowX) + "," + formatFloat(label.Y) +
					" L " + formatFloat(labelX) + "," + formatFloat(label.Y),
				Fill:        "none",
				Stroke:      label.LineColor,
				StrokeWidth: 1.0,
			})
			labelWidth := measureTextWidthWithFontSize(label.Text, label.FontSize, config.FastTextMetrics)
			padX := max(label.FontSize*0.35, 4.0)
			padY := max(label.FontSize*0.25, 2.5)
			rectW := labelWidth + padX*2.0
			rectH := label.FontSize + padY*2.0
			rectX := labelX - padX
			if label.Side < 0 {
				rectX = labelX - rectW + padX
			}
			rectY := label.Y - rectH/2.0
			bg := theme.EdgeLabelBackground
			if bg == "none" {
				bg = theme.Background
			}
			layout.Rects = append(layout.Rects, LayoutRect{
				X:    rectX,
				Y:    rectY,
				W:    rectW,
				H:    rectH,
				RX:   2,
				RY:   2,
				Fill: bg,
			})
		}
		layout.Texts = append(layout.Texts, LayoutText{
			X:                labelX,
			Y:                label.Y,
			Value:            label.Text,
			Anchor:           anchor,
			Size:             label.FontSize,
			Color:            theme.PieSectionTextColor,
			DominantBaseline: "middle",
		})
	}

	for _, item := range legend {
		layout.Rects = append(layout.Rects, LayoutRect{
			X:           item.X,
			Y:           item.Y,
			W:           item.MarkerSize,
			H:           item.MarkerSize,
			Fill:        item.Color,
			Stroke:      item.Color,
			StrokeWidth: theme.PieStrokeWidth,
		})
		labelX := item.X + item.MarkerSize + pieCfg.LegendSpacing
		labelY := item.Y + item.MarkerSize/2.0
		appendTextBlock(
			&layout,
			labelX,
			labelY,
			item.LabelBlock,
			theme,
			config,
			theme.PieLegendTextSize,
			"start",
			theme.PieLegendTextColor,
			true,
			"400",
		)
	}

	if hasTitle {
		appendTextBlock(
			&layout,
			centerX,
			centerY-(height-50.0)/2.0,
			titleBlock,
			theme,
			config,
			theme.PieTitleTextSize,
			"middle",
			theme.PieTitleTextColor,
			true,
			"400",
		)
	}

	return layout
}

func journeyScoreColor(score float64) string {
	clamped := clamp(score, 1.0, 5.0)
	t := (clamped - 1.0) / 4.0
	start := [3]float64{248.0, 113.0, 113.0}
	end := [3]float64{74.0, 222.0, 128.0}
	r := int(math.Round(start[0] + (end[0]-start[0])*t))
	g := int(math.Round(start[1] + (end[1]-start[1])*t))
	b := int(math.Round(start[2] + (end[2]-start[2])*t))
	return "#" + strings.ToUpper(strconv.FormatInt(int64((r<<16)|(g<<8)|b), 16))
}

func normalizeHexColor(color string) string {
	clean := strings.TrimPrefix(strings.ToUpper(color), "#")
	for len(clean) < 6 {
		clean = "0" + clean
	}
	if len(clean) > 6 {
		clean = clean[len(clean)-6:]
	}
	return "#" + clean
}

func appendJourneyFace(layout *Layout, cx, cy, r, score float64, stroke string) {
	layout.Circles = append(layout.Circles, LayoutCircle{
		CX:          cx,
		CY:          cy,
		R:           r,
		Fill:        "#ECE6C5",
		Stroke:      stroke,
		StrokeWidth: 1.0,
	})
	eyeR := max(1.0, r*0.13)
	eyeDX := r * 0.33
	eyeY := cy - r*0.15
	layout.Circles = append(layout.Circles,
		LayoutCircle{
			CX:          cx - eyeDX,
			CY:          eyeY,
			R:           eyeR,
			Fill:        stroke,
			Stroke:      "none",
			StrokeWidth: 0,
		},
		LayoutCircle{
			CX:          cx + eyeDX,
			CY:          eyeY,
			R:           eyeR,
			Fill:        stroke,
			Stroke:      "none",
			StrokeWidth: 0,
		},
	)
	var d string
	switch {
	case score >= 4.3:
		d = "M " + formatFloat(cx-r*0.35) + "," + formatFloat(cy+r*0.18) +
			" Q " + formatFloat(cx) + "," + formatFloat(cy+r*0.45) +
			" " + formatFloat(cx+r*0.35) + "," + formatFloat(cy+r*0.18)
	case score >= 2.7:
		d = "M " + formatFloat(cx-r*0.35) + "," + formatFloat(cy+r*0.28) +
			" L " + formatFloat(cx+r*0.35) + "," + formatFloat(cy+r*0.28)
	default:
		d = "M " + formatFloat(cx-r*0.35) + "," + formatFloat(cy+r*0.38) +
			" Q " + formatFloat(cx) + "," + formatFloat(cy+r*0.08) +
			" " + formatFloat(cx+r*0.35) + "," + formatFloat(cy+r*0.38)
	}
	layout.Paths = append(layout.Paths, LayoutPath{
		D:           d,
		Fill:        "none",
		Stroke:      stroke,
		StrokeWidth: 1.2,
		LineCap:     "round",
		LineJoin:    "round",
	})
}

func layoutJourneyFidelity(graph *Graph, theme Theme, config LayoutConfig) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.JourneySteps) == 0 {
		return layoutGeneric(graph, theme)
	}
	const (
		confDiagramMarginX = 50.0
		confDiagramMarginY = 10.0
		confLeftMargin     = 150.0
		confWidth          = 150.0
		confHeight         = 50.0
		confTaskMargin     = 50.0
		maxHeight          = 450.0
	)
	actorColours := []string{"#8FBC8F", "#7CFC00", "#00FFFF", "#20B2AA", "#B0E0E6", "#FFFFE0"}
	sectionFills := []string{"#191970", "#8B008B", "#4B0082", "#2F4F4F", "#800000", "#8B4513", "#00008B"}
	sectionColours := []string{"#fff"}

	actorPos := map[string]int{}
	actorNames := make([]string, 0)
	for _, step := range graph.JourneySteps {
		for _, actor := range step.Actors {
			name := strings.TrimSpace(actor)
			if name == "" {
				continue
			}
			if _, ok := actorPos[name]; ok {
				continue
			}
			actorPos[name] = len(actorNames)
			actorNames = append(actorNames, name)
		}
	}

	maxLabelWidth := 0.0
	for _, actor := range actorNames {
		w := measureTextWidthWithFontSize(actor, theme.FontSize, config.FastTextMetrics, theme.FontFamily)
		if w > maxLabelWidth && w > confLeftMargin-w {
			maxLabelWidth = w
		}
	}
	leftMargin := confLeftMargin + maxLabelWidth

	yPos := 60.0
	for idx, actor := range actorNames {
		color := actorColours[idx%len(actorColours)]
		layout.Circles = append(layout.Circles, LayoutCircle{
			CX:          20,
			CY:          yPos,
			R:           7,
			Class:       "actor-" + intString(idx),
			Fill:        color,
			Stroke:      "#000",
			StrokeWidth: 1,
		})
		layout.Texts = append(layout.Texts, LayoutText{
			Class:  "legend",
			X:      40,
			Y:      yPos + 7,
			Value:  actor,
			Anchor: "start",
			Size:   theme.FontSize,
			Color:  "#666",
		})
		yPos += 20
	}

	boxStartY := 0.0
	boxStopX := leftMargin
	boxStopY := max(0.0, float64(len(actorNames))*50.0)
	verticalPos := 0.0
	sectionVHeight := confHeight*2 + confDiagramMarginY
	taskPos := verticalPos + sectionVHeight

	lastSection := ""
	sectionNumber := 0
	taskCount := -1
	currentFill := sectionFills[0]
	currentTextColor := sectionColours[0]

	for i, step := range graph.JourneySteps {
		sectionName := strings.TrimSpace(step.Section)
		if sectionName != lastSection {
			currentFill = sectionFills[sectionNumber%len(sectionFills)]
			currentTextColor = sectionColours[sectionNumber%len(sectionColours)]

			taskInSectionCount := 0
			for j := i; j < len(graph.JourneySteps) && strings.TrimSpace(graph.JourneySteps[j].Section) == sectionName; j++ {
				taskInSectionCount++
			}

			sectionType := sectionNumber % len(sectionFills)
			sectionX := float64(i)*confTaskMargin + float64(i)*confWidth + leftMargin
			sectionW := confWidth*float64(taskInSectionCount) + confDiagramMarginX*float64(max(0, taskInSectionCount-1))
			sectionClass := "journey-section section-type-" + intString(sectionType)
			layout.Rects = append(layout.Rects, LayoutRect{
				Class:       sectionClass,
				X:           sectionX,
				Y:           50,
				W:           sectionW,
				H:           confHeight,
				RX:          3,
				RY:          3,
				Fill:        currentFill,
				Stroke:      "#666",
				StrokeWidth: 1,
			})
			layout.Texts = append(layout.Texts, LayoutText{
				Class:  sectionClass,
				X:      sectionX + sectionW/2.0,
				Y:      75,
				Value:  sectionName,
				Anchor: "middle",
				Size:   14,
				Color:  currentTextColor,
				BoxX:   sectionX,
				BoxY:   50,
				BoxW:   sectionW,
				BoxH:   confHeight,
			})

			lastSection = sectionName
			sectionNumber++
		}

		taskType := (sectionNumber - 1) % len(sectionFills)
		taskClass := "task task-type-" + intString(taskType)
		x := float64(i)*confTaskMargin + float64(i)*confWidth + leftMargin
		center := x + confWidth/2.0
		taskCount++

		layout.Lines = append(layout.Lines, LayoutLine{
			ID:          "task" + intString(taskCount),
			Class:       "task-line",
			X1:          center,
			Y1:          taskPos,
			X2:          center,
			Y2:          maxHeight,
			Stroke:      "#666",
			StrokeWidth: 1,
			DashArray:   "4 2",
		})

		score := 3.0
		if step.HasScore {
			score = step.Score
		}
		faceY := 300.0 + (5.0-score)*30.0
		layout.Circles = append(layout.Circles, LayoutCircle{
			Class:       "face",
			CX:          center,
			CY:          faceY,
			R:           15,
			StrokeWidth: 2,
		})
		layout.Circles = append(layout.Circles,
			LayoutCircle{
				CX:          center - 5,
				CY:          faceY - 5,
				R:           1.5,
				Fill:        "#666",
				Stroke:      "#666",
				StrokeWidth: 2,
			},
			LayoutCircle{
				CX:          center + 5,
				CY:          faceY - 5,
				R:           1.5,
				Fill:        "#666",
				Stroke:      "#666",
				StrokeWidth: 2,
			},
		)
		if score > 3 {
			layout.Paths = append(layout.Paths, LayoutPath{
				Class:     "mouth",
				D:         "M7.5,0A7.5,7.5,0,1,1,-7.5,0L-6.818,0A6.818,6.818,0,1,0,6.818,0Z",
				Transform: "translate(" + formatFloat(center) + "," + formatFloat(faceY+2) + ")",
			})
		} else if score < 3 {
			layout.Paths = append(layout.Paths, LayoutPath{
				Class:     "mouth",
				D:         "M-7.5,0A7.5,7.5,0,1,1,7.5,0L6.818,0A6.818,6.818,0,1,0,-6.818,0Z",
				Transform: "translate(" + formatFloat(center) + "," + formatFloat(faceY+7) + ")",
			})
		} else {
			layout.Lines = append(layout.Lines, LayoutLine{
				Class:       "mouth",
				X1:          center - 5,
				Y1:          faceY + 7,
				X2:          center + 5,
				Y2:          faceY + 7,
				Stroke:      "#666",
				StrokeWidth: 1,
			})
		}

		layout.Rects = append(layout.Rects, LayoutRect{
			Class:       taskClass,
			X:           x,
			Y:           taskPos,
			W:           confWidth,
			H:           confHeight,
			RX:          3,
			RY:          3,
			Fill:        currentFill,
			Stroke:      "#666",
			StrokeWidth: 1,
		})

		actorX := x + 14.0
		for _, actor := range step.Actors {
			name := strings.TrimSpace(actor)
			pos, ok := actorPos[name]
			if !ok {
				continue
			}
			layout.Circles = append(layout.Circles, LayoutCircle{
				Class:       "actor-" + intString(pos),
				CX:          actorX,
				CY:          taskPos,
				R:           7,
				Fill:        actorColours[pos%len(actorColours)],
				Stroke:      "#000",
				StrokeWidth: 1,
			})
			actorX += 10
		}

		layout.Texts = append(layout.Texts, LayoutText{
			Class:  "task",
			X:      center,
			Y:      taskPos + confHeight/2.0,
			Value:  step.Label,
			Anchor: "middle",
			Size:   14,
			Color:  currentTextColor,
			BoxX:   x,
			BoxY:   taskPos,
			BoxW:   confWidth,
			BoxH:   confHeight,
		})

		boxStopX = max(boxStopX, x+confDiagramMarginX+confTaskMargin)
		boxStopY = max(boxStopY, maxHeight)
	}

	width := leftMargin + boxStopX + 2*confDiagramMarginX
	height := boxStopY - boxStartY + 2*confDiagramMarginY
	layout.Lines = append(layout.Lines, LayoutLine{
		X1:          leftMargin,
		Y1:          confHeight * 4,
		X2:          width - leftMargin - 4,
		Y2:          confHeight * 4,
		Stroke:      "black",
		StrokeWidth: 4,
		ArrowEnd:    true,
	})

	extraVertForTitle := 0.0
	if strings.TrimSpace(graph.JourneyTitle) != "" {
		layout.Texts = append(layout.Texts, LayoutText{
			Class:      "journey-title",
			X:          leftMargin,
			Y:          25,
			Value:      graph.JourneyTitle,
			Anchor:     "start",
			Weight:     "bold",
			FontFamily: `"trebuchet ms", verdana, arial, sans-serif`,
		})
		extraVertForTitle = 70
	}

	layout.ViewBoxX = 0
	layout.ViewBoxY = -25
	layout.ViewBoxWidth = width
	layout.ViewBoxHeight = height + extraVertForTitle
	layout.Width = width
	layout.Height = height + extraVertForTitle + 25
	return layout
}

func layoutTimelineFidelity(graph *Graph, theme Theme, config LayoutConfig) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.TimelineEvents) == 0 {
		return layoutGeneric(graph, theme)
	}
	paddingX := 46.0
	paddingTop := 48.0
	timeBoxH := 62.0
	eventBoxH := 44.0
	colGap := 18.0
	textPadX := 14.0

	titleY := paddingTop
	if strings.TrimSpace(graph.TimelineTitle) != "" {
		layout.Texts = append(layout.Texts, LayoutText{
			X:      paddingX,
			Y:      titleY + theme.FontSize*1.5,
			Value:  graph.TimelineTitle,
			Anchor: "start",
			Size:   theme.FontSize * 1.9,
			Weight: "700",
			Color:  theme.PrimaryTextColor,
		})
		titleY += theme.FontSize*1.9 + 12.0
	}

	maxBoxW := 186.0
	for _, event := range graph.TimelineEvents {
		w := measureTextWidthWithFontSize(event.Time, theme.FontSize*0.95, config.FastTextMetrics, theme.FontFamily)
		maxBoxW = max(maxBoxW, w+textPadX*2.0)
		for _, item := range event.Events {
			w = measureTextWidthWithFontSize(item, theme.FontSize*0.95, config.FastTextMetrics, theme.FontFamily)
			maxBoxW = max(maxBoxW, w+textPadX*2.0)
		}
	}

	startX := paddingX + 138.0
	topBoxY := titleY + 6.0
	axisY := topBoxY + timeBoxH + 120.0
	eventTopY := axisY + 82.0

	topColors := []string{"#7e81ea", "#eceb66"}
	textColors := []string{"#ffffff", "#222222"}
	sectionClassToken := func(section string) string {
		section = strings.TrimSpace(section)
		if section == "" {
			return "0"
		}
		sectionIndex := -1
		for i, name := range graph.TimelineSections {
			if strings.TrimSpace(name) == section {
				sectionIndex = i
				break
			}
		}
		if sectionIndex <= 0 {
			return "-1"
		}
		return intString(sectionIndex - 1)
	}

	maxEvents := 1
	for _, event := range graph.TimelineEvents {
		maxEvents = max(maxEvents, len(event.Events))
	}
	type sectionSpan struct {
		first int
		last  int
	}
	sectionSpans := map[string]sectionSpan{}
	for idx, event := range graph.TimelineEvents {
		section := strings.TrimSpace(event.Section)
		if section == "" {
			continue
		}
		span, ok := sectionSpans[section]
		if !ok {
			sectionSpans[section] = sectionSpan{first: idx, last: idx}
			continue
		}
		span.last = idx
		sectionSpans[section] = span
	}
	sectionY := topBoxY - 12.0
	for _, section := range graph.TimelineSections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}
		span, ok := sectionSpans[section]
		if !ok {
			continue
		}
		xStart := startX + float64(span.first)*(maxBoxW+colGap)
		xEnd := startX + float64(span.last)*(maxBoxW+colGap) + maxBoxW
		layout.Texts = append(layout.Texts, LayoutText{
			X:      (xStart + xEnd) / 2.0,
			Y:      sectionY,
			Value:  section,
			Anchor: "middle",
			Size:   theme.FontSize * 0.95,
			Weight: "700",
			Color:  theme.PrimaryTextColor,
		})
	}

	for i, event := range graph.TimelineEvents {
		x := startX + float64(i)*(maxBoxW+colGap)
		cx := x + maxBoxW/2.0
		color := topColors[i%len(topColors)]
		textColor := textColors[i%len(textColors)]

		layout.Rects = append(layout.Rects, LayoutRect{
			ID:          "node-undefined",
			Class:       "node-bkg node-undefined",
			X:           x,
			Y:           topBoxY,
			W:           maxBoxW,
			H:           timeBoxH,
			RX:          2,
			RY:          2,
			Fill:        color,
			Stroke:      "#7e81ea",
			StrokeWidth: 1.0,
		})
		layout.Lines = append(layout.Lines, LayoutLine{
			Class:       "node-line-" + sectionClassToken(event.Section),
			X1:          x,
			Y1:          topBoxY + timeBoxH + 5,
			X2:          x + maxBoxW,
			Y2:          topBoxY + timeBoxH + 5,
			Stroke:      "#333333",
			StrokeWidth: 2.2,
		})
		layout.Texts = append(layout.Texts, LayoutText{
			X:      cx,
			Y:      topBoxY + timeBoxH/2.0 + theme.FontSize*0.3,
			Value:  event.Time,
			Anchor: "middle",
			Size:   theme.FontSize * 0.95,
			Weight: "700",
			Color:  textColor,
		})

		for j, item := range event.Events {
			ey := eventTopY + float64(j)*(eventBoxH+8.0)
			layout.Rects = append(layout.Rects, LayoutRect{
				ID:          "node-undefined",
				Class:       "node-bkg node-undefined",
				X:           x,
				Y:           ey,
				W:           maxBoxW,
				H:           eventBoxH,
				RX:          1.5,
				RY:          1.5,
				Fill:        color,
				Stroke:      "none",
				StrokeWidth: 0,
			})
			layout.Lines = append(layout.Lines, LayoutLine{
				Class:       "node-line-" + sectionClassToken(event.Section),
				X1:          x,
				Y1:          ey + eventBoxH + 5,
				X2:          x + maxBoxW,
				Y2:          ey + eventBoxH + 5,
				Stroke:      "#333333",
				StrokeWidth: 2.2,
			})
			layout.Texts = append(layout.Texts, LayoutText{
				X:      cx,
				Y:      ey + eventBoxH/2.0 + theme.FontSize*0.3,
				Value:  strings.ReplaceAll(item, " ", "   "),
				Anchor: "middle",
				Size:   theme.FontSize * 0.95,
				Color:  textColor,
			})
		}

		laneBottom := eventTopY + float64(len(event.Events))*(eventBoxH+8.0) + 18.0
		layout.Lines = append(layout.Lines, LayoutLine{
			Class:       "lineWrapper",
			X1:          cx,
			Y1:          topBoxY + timeBoxH,
			X2:          cx,
			Y2:          laneBottom - 7.0,
			Stroke:      "#333333",
			StrokeWidth: 1.1,
			DashArray:   "3,2",
			ArrowEnd:    true,
		})
	}

	lastRight := startX + float64(len(graph.TimelineEvents)-1)*(maxBoxW+colGap) + maxBoxW
	layout.Lines = append(layout.Lines, LayoutLine{
		Class:       "lineWrapper",
		X1:          startX - 26.0,
		Y1:          axisY,
		X2:          lastRight + 166.0,
		Y2:          axisY,
		Stroke:      "#222222",
		StrokeWidth: 3.0,
		ArrowEnd:    true,
	})

	layout.Width = lastRight + 202.0
	layout.Height = eventTopY + float64(maxEvents)*(eventBoxH+8.0) + 194.0
	layout.ViewBoxX = 100
	layout.ViewBoxY = -61.5
	layout.ViewBoxWidth = max(900, layout.Width+2)
	layout.ViewBoxHeight = max(620, layout.Height-8)
	return layout
}

func layoutXYChartFidelity(graph *Graph, theme Theme, config LayoutConfig) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.XYSeries) == 0 {
		return layoutGeneric(graph, theme)
	}

	allValues := []float64{}
	maxSeriesValues := 0
	barSeries := make([]XYSeries, 0, len(graph.XYSeries))
	lineSeries := make([]XYSeries, 0, len(graph.XYSeries))
	for _, series := range graph.XYSeries {
		allValues = append(allValues, series.Values...)
		maxSeriesValues = max(maxSeriesValues, len(series.Values))
		if series.Kind == XYSeriesBar {
			barSeries = append(barSeries, series)
		} else if series.Kind == XYSeriesLine {
			lineSeries = append(lineSeries, series)
		}
	}

	minVal := 0.0
	maxVal := 0.0
	if graph.XYYMin != nil {
		minVal = *graph.XYYMin
	}
	if graph.XYYMax != nil {
		maxVal = *graph.XYYMax
	}
	if graph.XYYMin == nil || graph.XYYMax == nil {
		for _, value := range allValues {
			if graph.XYYMin == nil {
				minVal = math.Min(minVal, value)
			}
			if graph.XYYMax == nil {
				maxVal = math.Max(maxVal, value)
			}
		}
	}
	if maxVal <= minVal {
		maxVal = minVal + 1
	}

	numCategories := max(1, max(len(graph.XYXCategories), maxSeriesValues))
	categories := make([]string, 0, numCategories)
	for i := 0; i < numCategories; i++ {
		if i < len(graph.XYXCategories) && strings.TrimSpace(graph.XYXCategories[i]) != "" {
			categories = append(categories, graph.XYXCategories[i])
		} else {
			categories = append(categories, "Q"+intString(i+1))
		}
	}

	const (
		totalWidth     = 700.0
		totalHeight    = 500.0
		axisLeft       = 66.53125190734863
		axisBottom     = 468.0
		axisRight      = 700.0
		axisTop        = 43.5
		yLabelTop      = 51.5
		yLabelBottom   = 459.0
		yZeroLine      = 459.0
		barBottom      = 467.0
		centerStart    = 102.53125190734863
		centerEnd      = 665.0
		barSeriesWidth = 66.5
	)

	layout.Rects = append(layout.Rects, LayoutRect{
		Class:  "background",
		X:      0,
		Y:      0,
		W:      totalWidth,
		H:      totalHeight,
		Fill:   "white",
		Stroke: "none",
	})

	centers := make([]float64, numCategories)
	if numCategories == 1 {
		centers[0] = (centerStart + centerEnd) / 2.0
	} else {
		step := (centerEnd - centerStart) / float64(numCategories-1)
		for i := 0; i < numCategories; i++ {
			centers[i] = centerStart + float64(i)*step
		}
	}

	valueRange := maxVal - minVal
	if valueRange <= 0 {
		valueRange = 1
	}
	valueToY := func(v float64) float64 {
		ratio := (v - minVal) / valueRange
		return yZeroLine - ratio*(yLabelBottom-yLabelTop)
	}

	barCount := max(1, len(barSeries))
	barWidth := barSeriesWidth / float64(barCount)
	for idx, series := range barSeries {
		for i, value := range series.Values {
			if i >= len(centers) {
				break
			}
			x := centers[i] - barSeriesWidth/2.0 + float64(idx)*barWidth
			y := valueToY(value)
			layout.Rects = append(layout.Rects, LayoutRect{
				X:           x,
				Y:           y,
				W:           barWidth,
				H:           max(0, barBottom-y),
				Fill:        "#ECECFF",
				Stroke:      "#ECECFF",
				StrokeWidth: 0,
			})
		}
	}

	for _, series := range lineSeries {
		if len(series.Values) == 0 {
			continue
		}
		var d strings.Builder
		for i, value := range series.Values {
			if i >= len(centers) {
				break
			}
			if i == 0 {
				d.WriteString("M")
			} else {
				d.WriteString("L")
			}
			d.WriteString(formatFloat(centers[i]))
			d.WriteString(",")
			d.WriteString(formatFloat(valueToY(value)))
		}
		layout.Paths = append(layout.Paths, LayoutPath{
			D:           d.String(),
			Fill:        "none",
			Stroke:      "#8493A6",
			StrokeWidth: 2,
		})
	}

	layout.Paths = append(layout.Paths,
		LayoutPath{
			D:           "M " + formatFloat(axisLeft+1.0) + "," + formatFloat(axisBottom) + " L " + formatFloat(axisRight) + "," + formatFloat(axisBottom),
			Fill:        "none",
			Stroke:      "#131300",
			StrokeWidth: 2,
		},
		LayoutPath{
			D:           "M " + formatFloat(axisLeft) + "," + formatFloat(axisTop) + " L " + formatFloat(axisLeft) + "," + formatFloat(barBottom),
			Fill:        "none",
			Stroke:      "#131300",
			StrokeWidth: 2,
		},
	)

	for _, center := range centers {
		layout.Paths = append(layout.Paths, LayoutPath{
			D:           "M " + formatFloat(center) + ",469 L " + formatFloat(center) + ",474",
			Fill:        "none",
			Stroke:      "#131300",
			StrokeWidth: 2,
		})
	}

	tickCount := 12
	for i := 0; i <= tickCount; i++ {
		y := yLabelTop + float64(i)*(yLabelBottom-yLabelTop)/float64(tickCount)
		value := maxVal - float64(i)*(maxVal-minVal)/float64(tickCount)
		layout.Paths = append(layout.Paths, LayoutPath{
			D:           "M " + formatFloat(axisLeft-1.0) + "," + formatFloat(y) + " L " + formatFloat(axisLeft-6.0) + "," + formatFloat(y),
			Fill:        "none",
			Stroke:      "#131300",
			StrokeWidth: 2,
		})
		label := strconv.FormatFloat(value, 'f', 0, 64)
		if math.Abs(value-math.Round(value)) > 0.001 {
			label = strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 2, 64), "0"), ".")
		}
		layout.Texts = append(layout.Texts, LayoutText{
			X:                0,
			Y:                0,
			Value:            label,
			Anchor:           "end",
			Size:             14,
			Color:            "#131300",
			DominantBaseline: "middle",
			Transform:        "translate(" + formatFloat(axisLeft-11.0) + ", " + formatFloat(y) + ") rotate(0)",
		})
	}

	for idx, label := range categories {
		center := centers[idx]
		layout.Texts = append(layout.Texts, LayoutText{
			X:                0,
			Y:                0,
			Value:            label,
			Anchor:           "middle",
			Size:             14,
			Color:            "#131300",
			DominantBaseline: "text-before-edge",
			Transform:        "translate(" + formatFloat(center) + ", 479) rotate(0)",
		})
	}

	if strings.TrimSpace(graph.XYTitle) != "" {
		layout.Texts = append(layout.Texts, LayoutText{
			X:                0,
			Y:                0,
			Value:            graph.XYTitle,
			Anchor:           "middle",
			Size:             20,
			Color:            "#131300",
			DominantBaseline: "middle",
			Transform:        "translate(350, 21.75) rotate(0)",
		})
	}
	if strings.TrimSpace(graph.XYYAxisLabel) != "" {
		layout.Texts = append(layout.Texts, LayoutText{
			X:                0,
			Y:                0,
			Value:            graph.XYYAxisLabel,
			Anchor:           "middle",
			Size:             16,
			Color:            "#131300",
			DominantBaseline: "text-before-edge",
			Transform:        "translate(5, 255.25) rotate(270)",
		})
	}

	layout.Width = totalWidth
	layout.Height = totalHeight
	layout.ViewBoxX = 0
	layout.ViewBoxY = 0
	layout.ViewBoxWidth = totalWidth
	layout.ViewBoxHeight = totalHeight
	return layout
}

func ganttPalette(theme Theme) []string {
	return []string{
		theme.PrimaryBorderColor,
		"#0ea5e9",
		"#10b981",
		"#6366f1",
		"#f97316",
	}
}

func shiftColor(color string, targetS, targetL, strength float64) string {
	_, s, l, ok := parseColorToHSL(color)
	if !ok {
		return color
	}
	deltaS := (targetS - s) * strength
	deltaL := (targetL - l) * strength
	return adjustColor(color, 0.0, deltaS, deltaL)
}

func ganttSectionPalette(theme Theme, sections []string) map[string]string {
	result := map[string]string{}
	if len(sections) == 0 {
		return result
	}
	base := theme.PrimaryBorderColor
	step := 360.0 / float64(max(1, len(sections)))
	for idx, name := range sections {
		hueShift := step * float64(idx)
		color := adjustColor(base, hueShift, 0.0, 0.0)
		color = shiftColor(color, 60.0, 55.0, 0.4)
		result[name] = color
	}
	return result
}

func ganttTaskColor(status string, base string, fallback string) string {
	if _, _, _, ok := parseColorToHSL(base); !ok {
		base = fallback
	}
	switch status {
	case "done":
		return shiftColor(base, 30.0, 80.0, 0.7)
	case "active":
		return shiftColor(base, 70.0, 52.0, 0.6)
	case "crit":
		_, s, l, ok := parseColorToHSL(base)
		if !ok {
			return "#ef4444"
		}
		return hslColorString(0.0, max(65.0, s), clamp(l, 45.0, 60.0))
	case "milestone":
		_, s, l, ok := parseColorToHSL(base)
		if !ok {
			return "#f59e0b"
		}
		return hslColorString(45.0, max(65.0, s), clamp(l, 50.0, 65.0))
	default:
		return base
	}
}

func parseGanttDurationValue(value string) (float64, bool) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return 0, false
	}
	digits := strings.Builder{}
	unit := byte(0)
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if (ch >= '0' && ch <= '9') || ch == '.' {
			digits.WriteByte(ch)
		} else if ch != ' ' && ch != '\t' {
			unit = byte(strings.ToLower(string(ch))[0])
		}
	}
	number, ok := parseFloat(digits.String())
	if !ok {
		return 0, false
	}
	mult := 1.0
	switch unit {
	case 'd':
		mult = 1.0
	case 'w':
		mult = 7.0
	case 'h':
		mult = 1.0 / 24.0
	case 'm':
		mult = 30.0
	case 'y':
		mult = 365.0
	}
	return number * mult, true
}

func parseGanttDateValue(value string) (int, bool) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return 0, false
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '-' || r == '/' || r == '.'
	})
	if len(parts) != 3 {
		return 0, false
	}
	year, errY := strconv.Atoi(parts[0])
	month, errM := strconv.Atoi(parts[1])
	day, errD := strconv.Atoi(parts[2])
	if errY != nil || errM != nil || errD != nil {
		return 0, false
	}
	if month <= 0 || month > 12 || day <= 0 || day > 31 {
		return 0, false
	}
	return daysFromCivil(year, month, day), true
}

func daysFromCivil(year, month, day int) int {
	y := year
	if month <= 2 {
		y--
	}
	era := y / 400
	if y < 0 {
		era = (y - 399) / 400
	}
	yoe := y - era*400
	m := month
	doy := (153*(m+func() int {
		if m > 2 {
			return -3
		}
		return 9
	}()) + 2) / 5
	doy += day - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

func civilFromDays(days int) (int, int, int) {
	z := days + 719468
	era := z / 146097
	if z < 0 {
		era = (z - 146096) / 146097
	}
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y := yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d := doy - (153*mp+2)/5 + 1
	m := mp
	if mp < 10 {
		m += 3
	} else {
		m -= 9
	}
	year := y
	if m <= 2 {
		year++
	}
	return year, m, d
}

func formatGanttDate(days int) string {
	year, month, day := civilFromDays(days)
	return strconv.FormatInt(int64(year), 10) + "-" +
		leftPadInt(month, 2) + "-" + leftPadInt(day, 2)
}

func leftPadInt(value int, width int) string {
	raw := strconv.Itoa(value)
	for len(raw) < width {
		raw = "0" + raw
	}
	return raw
}

func layoutGanttFidelity(graph *Graph, theme Theme, config LayoutConfig) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.GanttTasks) == 0 {
		return layoutGeneric(graph, theme)
	}

	padding := theme.FontSize * 1.25
	rowHeight := max(theme.FontSize*1.5, theme.FontSize+8.0)
	labelGap := theme.FontSize * 1.05
	defaultDuration := 3.0

	titleBlock := measuredText{}
	hasTitle := strings.TrimSpace(graph.GanttTitle) != ""
	if hasTitle {
		titleBlock = measureTextBlockWithFontSize(
			graph.GanttTitle,
			theme.FontSize,
			config.LabelLineHeight,
			config.FastTextMetrics,
		)
	}
	titleHeight := 0.0
	if hasTitle {
		titleHeight = titleBlock.Height + padding
	}

	sectionLabelWidth := 0.0
	for _, task := range graph.GanttTasks {
		if strings.TrimSpace(task.Section) != "" {
			sectionLabel := measureTextBlockWithFontSize(
				task.Section,
				theme.FontSize,
				config.LabelLineHeight,
				config.FastTextMetrics,
			)
			sectionLabelWidth = max(sectionLabelWidth, sectionLabel.Width)
		}
	}
	labelX := padding
	sectionTaskGap := 0.0
	labelWidth := sectionLabelWidth
	sectionLabelX := labelX
	taskLabelX := labelX + sectionLabelWidth + sectionTaskGap
	chartX := padding + labelWidth + labelGap
	chartY := titleHeight + padding
	chartWidth := theme.FontSize * 26.0

	parsedStarts := map[string]float64{}
	origin := math.MaxFloat64
	hasOrigin := false
	for _, task := range graph.GanttTasks {
		if start, ok := parseGanttDateValue(task.Start); ok {
			value := float64(start)
			parsedStarts[task.ID] = value
			if !hasOrigin || value < origin {
				origin = value
				hasOrigin = true
			}
		}
	}
	hasDates := hasOrigin

	timing := map[string][2]float64{}
	cursor := 0.0
	timeStart := math.MaxFloat64
	timeEnd := -math.MaxFloat64

	type computedTask struct {
		Label    string
		Start    float64
		Duration float64
		Status   string
		Section  string
	}
	computed := make([]computedTask, 0, len(graph.GanttTasks))
	for _, task := range graph.GanttTasks {
		duration, ok := parseGanttDurationValue(task.Duration)
		if !ok {
			duration = defaultDuration
		}
		duration = max(duration, 0.1)
		start, hasStart := parsedStarts[task.ID]
		if !hasStart && strings.TrimSpace(task.After) != "" {
			if afterTiming, ok := timing[task.After]; ok {
				start = afterTiming[1]
				hasStart = true
			}
		}
		fallbackBase := 0.0
		if hasOrigin {
			fallbackBase = origin
		}
		if !hasStart {
			start = fallbackBase + cursor
		}
		end := start + duration
		timing[task.ID] = [2]float64{start, end}
		cursor = max(cursor, end+0.5)
		timeStart = min(timeStart, start)
		timeEnd = max(timeEnd, end)
		computed = append(computed, computedTask{
			Label:    task.Label,
			Start:    start,
			Duration: duration,
			Status:   task.Status,
			Section:  task.Section,
		})
	}
	if !isFinite(timeStart) || !isFinite(timeEnd) {
		timeStart = 0.0
		timeEnd = 1.0
	}
	if math.Abs(timeEnd-timeStart) < 0.01 {
		timeEnd = timeStart + 1.0
	}
	timeSpan := max(timeEnd-timeStart, 1.0)
	chartWidth = max(chartWidth, timeSpan*theme.FontSize*6.0)
	timeScale := chartWidth / timeSpan

	type ganttTick struct {
		X     float64
		Label string
	}
	ticks := []ganttTick{}
	tickCount := 4
	for i := 0; i <= tickCount; i++ {
		t := timeStart + timeSpan*float64(i)/float64(tickCount)
		x := chartX + (t-timeStart)*timeScale
		label := strconv.FormatFloat(t-timeStart, 'f', 0, 64)
		if hasDates {
			label = formatGanttDate(int(math.Round(t)))
		}
		ticks = append(ticks, ganttTick{X: x, Label: label})
	}

	palette := ganttPalette(theme)
	sectionPalette := ganttSectionPalette(theme, graph.GanttSections)
	currentSection := ""
	currentSectionIdx := -1
	type ganttSectionLayout struct {
		Label     measuredText
		Y         float64
		Height    float64
		Color     string
		BandColor string
	}
	type ganttTaskLayout struct {
		Label    measuredText
		X        float64
		Y        float64
		Width    float64
		Height   float64
		Color    string
		Status   string
		RawLabel string
	}
	sections := []ganttSectionLayout{}
	tasks := []ganttTaskLayout{}
	y := chartY
	for idx, task := range computed {
		if task.Section != currentSection {
			if currentSectionIdx >= 0 {
				height := max(y-sections[currentSectionIdx].Y, rowHeight)
				sections[currentSectionIdx].Height = height
			}
			if strings.TrimSpace(task.Section) != "" {
				baseColor := sectionPalette[task.Section]
				if strings.TrimSpace(baseColor) == "" {
					baseColor = palette[idx%len(palette)]
				}
				bandColor := shiftColor(baseColor, 20.0, 92.0, 0.7)
				label := measureTextBlockWithFontSize(
					task.Section,
					theme.FontSize*0.9,
					config.LabelLineHeight,
					config.FastTextMetrics,
				)
				sections = append(sections, ganttSectionLayout{
					Label:     label,
					Y:         y,
					Height:    0.0,
					Color:     baseColor,
					BandColor: bandColor,
				})
				currentSectionIdx = len(sections) - 1
			} else {
				currentSectionIdx = -1
			}
			currentSection = task.Section
		}
		barX := chartX + (task.Start-timeStart)*timeScale
		barWidth := task.Duration * timeScale
		minWidth := rowHeight * 0.5
		if barWidth < minWidth {
			barWidth = minWidth
		}
		baseColor := palette[idx%len(palette)]
		if strings.TrimSpace(task.Section) != "" {
			if color := sectionPalette[task.Section]; strings.TrimSpace(color) != "" {
				baseColor = color
			}
		}
		color := ganttTaskColor(task.Status, baseColor, palette[0])
		label := measureTextBlockWithFontSize(
			task.Label,
			theme.FontSize*0.85,
			config.LabelLineHeight,
			config.FastTextMetrics,
		)
		tasks = append(tasks, ganttTaskLayout{
			Label:    label,
			X:        barX,
			Y:        y,
			Width:    barWidth,
			Height:   rowHeight,
			Color:    color,
			Status:   task.Status,
			RawLabel: task.Label,
		})
		y += rowHeight
	}
	if currentSectionIdx >= 0 {
		height := max(y-sections[currentSectionIdx].Y, rowHeight)
		sections[currentSectionIdx].Height = height
	}

	tickFont := theme.FontSize * 0.8
	maxTickHalfWidth := 0.0
	for _, tick := range ticks {
		label := measureTextBlockWithFontSize(
			tick.Label,
			tickFont,
			config.LabelLineHeight,
			config.FastTextMetrics,
		)
		maxTickHalfWidth = max(maxTickHalfWidth, label.Width/2.0)
	}
	axisPad := rowHeight*0.9 + theme.FontSize
	height := y + padding + axisPad
	width := max(chartX+chartWidth+padding, chartX+chartWidth+maxTickHalfWidth+padding*0.4)
	layout.Width = width
	layout.Height = height

	if hasTitle {
		appendTextBlock(
			&layout,
			chartX+chartWidth/2.0,
			chartY-rowHeight*0.6,
			titleBlock,
			theme,
			config,
			theme.FontSize,
			"middle",
			theme.PrimaryTextColor,
			false,
			"400",
		)
	}

	chartLeft := chartX
	chartRight := chartX + chartWidth
	fullWidth := chartRight + labelX
	barHeight := max(theme.FontSize*1.1, min(rowHeight*0.82, rowHeight-4.0))

	axisY := chartY + (y - chartY) + rowHeight*0.85
	for _, tick := range ticks {
		layout.Lines = append(layout.Lines, LayoutLine{
			X1:          tick.X,
			Y1:          chartY,
			X2:          tick.X,
			Y2:          y,
			Stroke:      "#E2E8F0",
			StrokeWidth: 1.0,
		})
		if strings.TrimSpace(tick.Label) != "" {
			layout.Texts = append(layout.Texts, LayoutText{
				X:      tick.X,
				Y:      axisY,
				Value:  tick.Label,
				Anchor: "middle",
				Size:   tickFont,
				Color:  theme.TextColor,
			})
		}
	}
	layout.Lines = append(layout.Lines,
		LayoutLine{
			X1:          chartLeft,
			Y1:          y,
			X2:          chartRight,
			Y2:          y,
			Stroke:      theme.LineColor,
			StrokeWidth: 1.0,
		},
		LayoutLine{
			X1:          chartLeft,
			Y1:          chartY,
			X2:          chartLeft,
			Y2:          y,
			Stroke:      "#E2E8F0",
			StrokeWidth: 1.0,
		},
	)

	sectionFont := theme.FontSize * 0.9
	taskFont := theme.FontSize * 0.85
	for _, section := range sections {
		labelBandWidth := chartX
		layout.Rects = append(layout.Rects,
			LayoutRect{
				X:           0.0,
				Y:           section.Y,
				W:           labelBandWidth,
				H:           section.Height,
				Fill:        section.BandColor,
				FillOpacity: 0.22,
				Stroke:      "none",
			},
			LayoutRect{
				X:           chartX,
				Y:           section.Y,
				W:           chartWidth,
				H:           section.Height,
				Fill:        section.BandColor,
				FillOpacity: 0.12,
				Stroke:      "none",
			},
			LayoutRect{
				X:           0.0,
				Y:           section.Y,
				W:           max(theme.FontSize*0.3, 3.0),
				H:           section.Height,
				Fill:        section.Color,
				FillOpacity: 0.9,
				Stroke:      "none",
			},
		)
		labelY := min(section.Y+rowHeight*0.55, section.Y+section.Height-rowHeight*0.45)
		appendTextBlock(
			&layout,
			sectionLabelX,
			labelY,
			section.Label,
			theme,
			config,
			sectionFont,
			"start",
			theme.PrimaryTextColor,
			false,
			"400",
		)
	}

	rowLines := []float64{chartY, y}
	for _, section := range sections {
		rowLines = append(rowLines, section.Y, section.Y+section.Height)
	}
	for _, task := range tasks {
		rowLines = append(rowLines, task.Y)
	}
	sort.Float64s(rowLines)
	dedup := make([]float64, 0, len(rowLines))
	for _, value := range rowLines {
		if len(dedup) == 0 || math.Abs(dedup[len(dedup)-1]-value) >= 0.5 {
			dedup = append(dedup, value)
		}
	}
	for _, lineY := range dedup {
		layout.Lines = append(layout.Lines, LayoutLine{
			X1:          0.0,
			Y1:          lineY,
			X2:          fullWidth,
			Y2:          lineY,
			Stroke:      "#E2E8F0",
			StrokeWidth: 1.0,
		})
	}

	ganttLabelColor := func(fill string) string {
		_, _, l, ok := parseColorToHSL(fill)
		if !ok {
			return theme.PrimaryTextColor
		}
		if l < 55.0 {
			return "#FFFFFF"
		}
		return "#0F172A"
	}

	for _, task := range tasks {
		rowCenter := task.Y + rowHeight/2.0
		barY := rowCenter - barHeight/2.0
		labelRenderedInside := false
		if task.Status == "milestone" {
			size := barHeight * 0.6
			cx := task.X
			cy := rowCenter
			layout.Polygons = append(layout.Polygons, LayoutPolygon{
				Points: []Point{
					{X: cx, Y: cy - size},
					{X: cx + size, Y: cy},
					{X: cx, Y: cy + size},
					{X: cx - size, Y: cy},
				},
				Fill:        task.Color,
				Stroke:      theme.PrimaryBorderColor,
				StrokeWidth: 1.0,
			})
		} else {
			layout.Rects = append(layout.Rects, LayoutRect{
				X:           task.X,
				Y:           barY,
				W:           task.Width,
				H:           barHeight,
				RX:          3.0,
				RY:          3.0,
				Fill:        task.Color,
				Stroke:      theme.PrimaryBorderColor,
				StrokeWidth: 1.0,
			})
			labelText := strings.TrimSpace(task.RawLabel)
			if labelText != "" {
				fontSize := taskFont * 0.95
				textWidth := measureTextWidthWithFontSize(labelText, fontSize, config.FastTextMetrics)
				pad := max(fontSize*0.6, 6.0)
				if task.Width >= textWidth+pad*2.0 && barHeight >= fontSize*1.1 {
					layout.Texts = append(layout.Texts, LayoutText{
						X:                task.X + task.Width/2.0,
						Y:                rowCenter,
						Value:            labelText,
						Anchor:           "middle",
						Size:             fontSize,
						Color:            ganttLabelColor(task.Color),
						DominantBaseline: "middle",
					})
					labelRenderedInside = true
				}
			}
		}
		if !labelRenderedInside {
			appendTextBlock(
				&layout,
				taskLabelX,
				rowCenter,
				task.Label,
				theme,
				config,
				taskFont,
				"start",
				theme.PrimaryTextColor,
				false,
				"400",
			)
		}
	}

	return layout
}

func layoutGanttFidelityV2(graph *Graph, theme Theme, config LayoutConfig) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.GanttTasks) == 0 {
		return layoutGeneric(graph, theme)
	}

	const (
		totalWidth           = 2184.0
		totalHeight          = 244.0
		titleTopMargin       = 25.0
		barHeight            = 20.0
		barGap               = 4.0
		topPadding           = 50.0
		leftPadding          = 75.0
		rightPadding         = 75.0
		gridLineStartPadding = 35.0
		fontSize             = 11.0
		numberSectionStyles  = 4
		defaultDuration      = 1.0
	)
	gap := barHeight + barGap
	plotWidth := totalWidth - leftPadding - rightPadding

	type rawTask struct {
		Index     int
		ID        string
		Label     string
		Section   string
		Status    string
		After     string
		Start     float64
		HasStart  bool
		Duration  float64
		Milestone bool
		Done      bool
		Active    bool
		Crit      bool
	}
	raw := make([]rawTask, 0, len(graph.GanttTasks))
	minStart := math.MaxFloat64
	for idx, task := range graph.GanttTasks {
		duration, ok := parseGanttDurationValue(task.Duration)
		if !ok {
			duration = defaultDuration
		}
		if duration < 0 {
			duration = 0
		}
		startVal := 0.0
		hasStart := false
		if start, ok := parseGanttDateValue(task.Start); ok {
			startVal = float64(start)
			hasStart = true
			minStart = min(minStart, startVal)
		}
		status := lower(strings.TrimSpace(task.Status))
		raw = append(raw, rawTask{
			Index:     idx,
			ID:        task.ID,
			Label:     task.Label,
			Section:   task.Section,
			Status:    status,
			After:     strings.TrimSpace(task.After),
			Start:     startVal,
			HasStart:  hasStart,
			Duration:  duration,
			Milestone: status == "milestone",
			Done:      status == "done",
			Active:    status == "active",
			Crit:      status == "crit",
		})
	}
	if !isFinite(minStart) {
		minStart = 0
	}

	type computedTask struct {
		rawTask
		StartTime float64
		EndTime   float64
		Order     int
	}
	computed := make([]computedTask, 0, len(raw))
	endByID := map[string]float64{}
	cursor := minStart
	for _, task := range raw {
		startTime := cursor
		if task.HasStart {
			startTime = task.Start
		} else if task.After != "" {
			if afterEnd, ok := endByID[task.After]; ok {
				startTime = afterEnd
			}
		}
		endTime := startTime + task.Duration
		endByID[task.ID] = endTime
		cursor = max(cursor, endTime)
		computed = append(computed, computedTask{
			rawTask:   task,
			StartTime: startTime,
			EndTime:   endTime,
		})
	}
	sort.SliceStable(computed, func(i, j int) bool {
		if computed[i].StartTime == computed[j].StartTime {
			return computed[i].Index < computed[j].Index
		}
		return computed[i].StartTime < computed[j].StartTime
	})
	for idx := range computed {
		computed[idx].Order = idx
	}

	minTime := math.MaxFloat64
	maxTime := -math.MaxFloat64
	for _, task := range computed {
		minTime = min(minTime, task.StartTime)
		maxTime = max(maxTime, task.EndTime)
	}
	if !isFinite(minTime) || !isFinite(maxTime) || maxTime <= minTime {
		minTime = 0
		maxTime = 1
	}
	scale := func(v float64) float64 {
		return (v - minTime) / (maxTime - minTime) * plotWidth
	}

	categories := make([]string, 0)
	categoryIndex := map[string]int{}
	for _, task := range graph.GanttTasks {
		section := strings.TrimSpace(task.Section)
		if _, ok := categoryIndex[section]; ok {
			continue
		}
		categoryIndex[section] = len(categories)
		categories = append(categories, section)
	}
	if len(categories) == 0 {
		categories = append(categories, "")
		categoryIndex[""] = 0
	}

	categoryHeights := map[string]int{}
	for _, task := range computed {
		categoryHeights[task.Section]++
	}

	// section rows
	for _, task := range computed {
		secNum := categoryIndex[task.Section] % numberSectionStyles
		layout.Rects = append(layout.Rects, LayoutRect{
			X:     0,
			Y:     float64(task.Order)*gap + topPadding - 2,
			W:     totalWidth - rightPadding/2,
			H:     gap,
			Class: "section section" + intString(secNum),
		})
	}

	// task bars and labels
	for _, task := range computed {
		secNum := categoryIndex[task.Section] % numberSectionStyles
		startX := scale(task.StartTime)
		endX := scale(task.EndTime)
		rectX := startX + leftPadding
		rectW := max(1.0, endX-startX)
		if task.Milestone {
			rectX = startX + leftPadding - barHeight/2.0
			rectW = barHeight
		}
		rectY := float64(task.Order)*gap + topPadding

		taskClass := ""
		if task.Active {
			if task.Crit {
				taskClass += " activeCrit"
			} else {
				taskClass = " active"
			}
		} else if task.Done {
			if task.Crit {
				taskClass = " doneCrit"
			} else {
				taskClass = " done"
			}
		} else if task.Crit {
			taskClass += " crit"
		}
		if taskClass == "" {
			taskClass = " task"
		}
		if task.Milestone {
			taskClass = " milestone " + taskClass
		}
		taskClass += intString(secNum)
		taskClass = "task" + taskClass

		layout.Rects = append(layout.Rects, LayoutRect{
			ID:              task.ID,
			Class:           taskClass,
			X:               rectX,
			Y:               rectY,
			W:               rectW,
			H:               barHeight,
			RX:              3,
			RY:              3,
			TransformOrigin: formatFloat(rectX+rectW/2.0) + "px " + formatFloat(rectY+barHeight/2.0) + "px",
		})

		textWidth := measureTextWidthWithFontSize(task.Label, fontSize, config.FastTextMetrics, theme.FontFamily)
		taskType := ""
		if task.Active {
			if task.Crit {
				taskType = "activeCritText" + intString(secNum)
			} else {
				taskType = "activeText" + intString(secNum)
			}
		}
		if task.Done {
			if task.Crit {
				taskType += " doneCritText" + intString(secNum)
			} else {
				taskType += " doneText" + intString(secNum)
			}
		} else if task.Crit {
			taskType += " critText" + intString(secNum)
		}
		if task.Milestone {
			taskType += " milestoneText"
		}

		textClass := ""
		textX := (endX-startX)/2.0 + startX + leftPadding
		if textWidth > endX-startX {
			if endX+textWidth+1.5*leftPadding > totalWidth {
				textClass = "taskTextOutsideLeft taskTextOutside" + intString(secNum) + " " + strings.TrimSpace(taskType)
				textX = startX + leftPadding - 5
			} else {
				textClass = "taskTextOutsideRight taskTextOutside" + intString(secNum) + " " + strings.TrimSpace(taskType) +
					" width-" + strconv.FormatFloat(textWidth, 'f', -1, 64)
				textX = endX + leftPadding + 5
			}
		} else {
			textClass = "taskText taskText" + intString(secNum) + " " + strings.TrimSpace(taskType) +
				" width-" + strconv.FormatFloat(textWidth, 'f', -1, 64)
		}
		layout.Texts = append(layout.Texts, LayoutText{
			ID:    task.ID + "-text",
			Class: strings.TrimSpace(textClass),
			X:     textX,
			Y:     float64(task.Order)*gap + barHeight/2.0 + (fontSize/2.0 - 2.0) + topPadding,
			Value: task.Label,
			Size:  fontSize,
		})
	}

	// section labels
	prevGap := 0
	for idx, category := range categories {
		count := categoryHeights[category]
		if count <= 0 {
			continue
		}
		y := float64(count)*gap/2.0 + float64(prevGap)*gap + topPadding
		prevGap += count
		layout.Texts = append(layout.Texts, LayoutText{
			Class: "sectionTitle sectionTitle" + intString(idx%numberSectionStyles),
			X:     10,
			Y:     y,
			Value: category,
			Size:  fontSize,
		})
	}

	// bottom grid ticks
	minDay := int(math.Round(minTime))
	maxDay := int(math.Round(maxTime))
	tickEnd := maxDay - 1
	lastTickDay := minDay - 2
	for day := minDay; day <= tickEnd; day += 2 {
		layout.Texts = append(layout.Texts, LayoutText{
			Class: "gantt-tick-label",
			X:     scale(float64(day)),
			Y:     3,
			Value: formatGanttDate(day),
			Size:  10,
		})
		lastTickDay = day
	}
	if tickEnd >= minDay && lastTickDay != tickEnd {
		layout.Texts = append(layout.Texts, LayoutText{
			Class: "gantt-tick-label",
			X:     scale(float64(tickEnd)),
			Y:     3,
			Value: formatGanttDate(tickEnd),
			Size:  10,
		})
	}

	layout.Paths = append(layout.Paths, LayoutPath{
		Class:  "domain",
		Stroke: "currentColor",
		D:      "M0.5,-159V0.5H" + formatFloat(plotWidth+0.5) + "V-159",
	})

	// today marker
	now := time.Now()
	today := float64(daysFromCivil(now.Year(), int(now.Month()), now.Day()))
	todayX := scale(today) + leftPadding
	layout.Lines = append(layout.Lines, LayoutLine{
		Class: "today",
		X1:    todayX,
		X2:    todayX,
		Y1:    titleTopMargin,
		Y2:    totalHeight - titleTopMargin,
	})

	if strings.TrimSpace(graph.GanttTitle) != "" {
		layout.Texts = append(layout.Texts, LayoutText{
			Class: "titleText",
			X:     totalWidth / 2.0,
			Y:     titleTopMargin,
			Value: graph.GanttTitle,
		})
	}

	layout.ViewBoxX = 0
	layout.ViewBoxY = 0
	layout.ViewBoxWidth = totalWidth
	layout.ViewBoxHeight = totalHeight
	layout.Width = totalWidth
	layout.Height = totalHeight
	return layout
}

func layoutPacketFidelity(graph *Graph, theme Theme, config LayoutConfig) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.PacketFields) == 0 {
		return layoutGraphLike(graph, theme, config)
	}

	const (
		bitsPerRow  = 32
		bitCell     = 32.0
		gapPerBlock = 5.0
		rowStartY   = 15.0
		rowStepY    = 47.0
		rowHeight   = 32.0
		baseX       = 1.0
	)

	type packetSegment struct {
		Row   int
		Start int
		End   int
		Label string
	}

	segments := make([]packetSegment, 0, len(graph.PacketFields))
	maxRow := 0
	for _, field := range graph.PacketFields {
		start := max(0, field.Start)
		end := max(start, field.End)
		for cursor := start; cursor <= end; {
			row := cursor / bitsPerRow
			rowEndBit := row*bitsPerRow + (bitsPerRow - 1)
			segEnd := min(end, rowEndBit)
			segments = append(segments, packetSegment{
				Row:   row,
				Start: cursor,
				End:   segEnd,
				Label: field.Label,
			})
			maxRow = max(maxRow, row)
			cursor = segEnd + 1
		}
	}

	sort.Slice(segments, func(i, j int) bool {
		if segments[i].Row != segments[j].Row {
			return segments[i].Row < segments[j].Row
		}
		if segments[i].Start != segments[j].Start {
			return segments[i].Start < segments[j].Start
		}
		return segments[i].End < segments[j].End
	})

	for _, seg := range segments {
		rowY := rowStartY + float64(seg.Row)*rowStepY
		colStart := seg.Start % bitsPerRow
		bits := max(1, seg.End-seg.Start+1)
		x := baseX + float64(colStart)*bitCell
		w := float64(bits)*bitCell - gapPerBlock
		w = max(1.0, w)

		layout.Rects = append(layout.Rects, LayoutRect{
			Class:       "packetBlock",
			X:           x,
			Y:           rowY,
			W:           w,
			H:           rowHeight,
			Fill:        "#efefef",
			Stroke:      "#000000",
			StrokeWidth: 1,
		})
		layout.Texts = append(layout.Texts,
			LayoutText{
				Class:            "packetLabel",
				X:                x + w/2.0,
				Y:                rowY + rowHeight/2.0,
				Value:            seg.Label,
				Anchor:           "middle",
				Size:             12,
				Color:            "#000000",
				DominantBaseline: "middle",
			},
			LayoutText{
				Class:            "packetByte start",
				X:                x,
				Y:                rowY - 2.0,
				Value:            strconv.Itoa(seg.Start),
				Anchor:           "start",
				Size:             10,
				Color:            "#000000",
				DominantBaseline: "auto",
			},
			LayoutText{
				Class:            "packetByte end",
				X:                x + w,
				Y:                rowY - 2.0,
				Value:            strconv.Itoa(seg.End),
				Anchor:           "end",
				Size:             10,
				Color:            "#000000",
				DominantBaseline: "auto",
			},
		)
	}

	fullRowWidth := float64(bitsPerRow)*bitCell - gapPerBlock
	layout.Width = baseX + fullRowWidth + 6.0
	rowCount := maxRow + 1
	contentBottom := rowStartY + float64(max(1, rowCount)-1)*rowStepY + rowHeight

	title := strings.TrimSpace(graph.PacketTitle)
	if title != "" {
		titleY := contentBottom + 23.5
		layout.Texts = append(layout.Texts, LayoutText{
			Class:            "packetTitle",
			X:                layout.Width / 2.0,
			Y:                titleY,
			Value:            title,
			Anchor:           "middle",
			Size:             14,
			Color:            "#000000",
			DominantBaseline: "middle",
		})
		layout.Height = titleY + 23.5
	} else {
		layout.Height = contentBottom + 15.0
	}

	return layout
}

func layoutKanbanFidelity(graph *Graph, theme Theme, config LayoutConfig) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.KanbanBoard) == 0 {
		return layoutGraphLike(graph, theme, config)
	}

	const (
		columnX0    = 100.0
		columnY     = -300.0
		columnWidth = 200.0
		columnGap   = 5.0
		cardWidth   = 185.0
		cardXPad    = 7.5
		cardYTopPad = 25.0
		cardGapY    = 5.0
	)

	type cardLayout struct {
		colX      float64
		y         float64
		h         float64
		title     []string
		ticket    string
		assigned  string
		priority  string
		priorityC string
	}
	cardLayouts := make([]cardLayout, 0, 16)

	maxColumnHeight := 0.0
	for idx, column := range graph.KanbanBoard {
		x := columnX0 + float64(idx)*(columnWidth+columnGap)
		cardY := columnY + cardYTopPad

		for _, card := range column.Cards {
			titleLines := wrapKanbanText(card.Title, cardWidth-20.0, 15.0, config.FastTextMetrics)
			if len(titleLines) == 0 {
				titleLines = []string{card.Title}
			}
			height := 44.0 + float64(max(0, len(titleLines)-1))*24.0
			if strings.TrimSpace(card.Ticket) != "" || strings.TrimSpace(card.Assigned) != "" {
				height = max(height, 56.0)
			}

			cardLayouts = append(cardLayouts, cardLayout{
				colX:      x,
				y:         cardY,
				h:         height,
				title:     []string{card.Title},
				ticket:    card.Ticket,
				assigned:  card.Assigned,
				priority:  card.Priority,
				priorityC: kanbanPriorityColor(card.Priority),
			})
			cardY += height + cardGapY
		}

		columnHeight := (cardY + cardGapY) - columnY
		maxColumnHeight = max(maxColumnHeight, columnHeight)

		layout.Rects = append(layout.Rects, LayoutRect{
			ID:          sanitizeID(column.Title, ""),
			Class:       "kanban-column",
			X:           x,
			Y:           columnY,
			W:           columnWidth,
			H:           columnHeight,
			RX:          5,
			RY:          5,
			Fill:        "hsl(240, 100%, 86.2745098039%)",
			Stroke:      "hsl(240, 100%, 76.2745098039%)",
			StrokeWidth: 2,
		})
		layout.Texts = append(layout.Texts, LayoutText{
			Class:            "kanban-column-title",
			X:                x + columnWidth/2.0,
			Y:                columnY + 12.5,
			Value:            column.Title,
			Anchor:           "middle",
			Size:             16,
			Color:            "#000000",
			DominantBaseline: "middle",
		})
	}

	for _, card := range cardLayouts {
		cardX := card.colX + cardXPad
		layout.Rects = append(layout.Rects, LayoutRect{
			Class:       "kanban-card",
			X:           cardX,
			Y:           card.y,
			W:           cardWidth,
			H:           card.h,
			RX:          5,
			RY:          5,
			Fill:        "#ffffff",
			Stroke:      "#9370DB",
			StrokeWidth: 1,
		})

		titleY := card.y + 14.0
		for _, line := range card.title {
			layout.Texts = append(layout.Texts, LayoutText{
				Class:            "kanban-card-text",
				X:                cardX + 10,
				Y:                titleY,
				Value:            line,
				Anchor:           "start",
				Size:             14,
				Color:            "#000000",
				DominantBaseline: "hanging",
			})
			titleY += 20
		}

		metaY := card.y + card.h - 12
		if strings.TrimSpace(card.ticket) != "" {
			layout.Texts = append(layout.Texts, LayoutText{
				Class:            "kanban-card-meta",
				X:                cardX + 10,
				Y:                metaY,
				Value:            card.ticket,
				Anchor:           "start",
				Size:             12,
				Color:            "#000000",
				DominantBaseline: "middle",
			})
		}
		if strings.TrimSpace(card.assigned) != "" {
			layout.Texts = append(layout.Texts, LayoutText{
				Class:            "kanban-card-meta",
				X:                cardX + cardWidth - 10,
				Y:                metaY,
				Value:            card.assigned,
				Anchor:           "end",
				Size:             12,
				Color:            "#000000",
				DominantBaseline: "middle",
			})
		}
		if strings.TrimSpace(card.priorityC) != "" {
			layout.Lines = append(layout.Lines, LayoutLine{
				X1:          cardX + 2,
				Y1:          card.y + 2,
				X2:          cardX + 2,
				Y2:          card.y + card.h - 2,
				Stroke:      card.priorityC,
				StrokeWidth: 4,
			})
		}
	}

	viewWidth := float64(len(graph.KanbanBoard))*columnWidth + float64(max(0, len(graph.KanbanBoard)-1))*columnGap + 20.0
	viewHeight := maxColumnHeight + 20.0
	layout.Width = viewWidth
	layout.Height = viewHeight
	layout.ViewBoxX = 90
	layout.ViewBoxY = -310
	layout.ViewBoxWidth = viewWidth
	layout.ViewBoxHeight = viewHeight
	return layout
}

func wrapKanbanText(text string, maxWidth float64, fontSize float64, fast bool) []string {
	words := strings.Fields(strings.TrimSpace(text))
	if len(words) == 0 {
		return nil
	}
	lines := make([]string, 0, 3)
	current := words[0]
	for _, word := range words[1:] {
		candidate := current + " " + word
		if measureTextWidthWithFontSize(candidate, fontSize, fast) <= maxWidth {
			current = candidate
			continue
		}
		lines = append(lines, current)
		current = word
	}
	lines = append(lines, current)
	return lines
}

func kanbanPriorityColor(priority string) string {
	switch lower(strings.TrimSpace(priority)) {
	case "very high":
		return "red"
	case "high":
		return "orange"
	case "low":
		return "blue"
	default:
		return ""
	}
}

func layoutTreemapFidelity(graph *Graph, theme Theme, config LayoutConfig) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.TreemapItems) == 0 {
		return layoutGraphLike(graph, theme, config)
	}

	type treeNode struct {
		Label    string
		Value    float64
		Children []*treeNode
	}

	root := &treeNode{Label: ""}
	stack := make([]*treeNode, 0, 8)
	for _, item := range graph.TreemapItems {
		node := &treeNode{
			Label: item.Label,
			Value: item.Value,
		}
		depth := max(0, item.Depth)
		if depth == 0 {
			root.Children = append(root.Children, node)
			stack = stack[:0]
			stack = append(stack, node)
			continue
		}
		if len(stack) == 0 {
			root.Children = append(root.Children, node)
			stack = append(stack, node)
			continue
		}
		if depth > len(stack) {
			depth = len(stack)
		}
		parent := stack[depth-1]
		parent.Children = append(parent.Children, node)
		stack = stack[:depth]
		stack = append(stack, node)
	}

	var finalize func(*treeNode) float64
	finalize = func(node *treeNode) float64 {
		if node == nil {
			return 0
		}
		if len(node.Children) == 0 {
			if node.Value <= 0 {
				node.Value = 1
			}
			return node.Value
		}
		sum := 0.0
		for _, child := range node.Children {
			sum += finalize(child)
		}
		if node.Value <= 0 {
			node.Value = sum
		}
		return max(node.Value, sum)
	}
	finalize(root)

	sectionPalette := []string{
		"hsl(240, 100%, 76.2745098039%)",
		"hsl(60, 100%, 73.5294117647%)",
		"hsl(80, 100%, 76.2745098039%)",
		"hsl(270, 100%, 76.2745098039%)",
		"hsl(210, 100%, 76.2745098039%)",
	}
	sectionStroke := []string{
		"hsl(240, 100%, 61.2745098039%)",
		"hsl(60, 100%, 48.5294117647%)",
		"hsl(80, 100%, 56.2745098039%)",
		"hsl(270, 100%, 61.2745098039%)",
		"hsl(210, 100%, 61.2745098039%)",
	}
	sectionColor := func(depth int) (string, string) {
		idx := depth % len(sectionPalette)
		return sectionPalette[idx], sectionStroke[idx]
	}

	type frame struct {
		x float64
		y float64
		w float64
		h float64
	}

	addLeaf := func(node *treeNode, f frame, classIdx int) {
		fill, _ := sectionColor(max(0, classIdx))
		layout.Rects = append(layout.Rects, LayoutRect{
			Class:         "treemapLeaf",
			X:             f.x,
			Y:             f.y,
			W:             f.w,
			H:             f.h,
			Fill:          fill,
			FillOpacity:   0.3,
			Stroke:        fill,
			StrokeWidth:   3,
			StrokeOpacity: 1,
		})
		labelSize := clamp(min(f.w, f.h)*0.25, 20, 38)
		valueSize := clamp(labelSize*0.6, 14, 23)
		layout.Texts = append(layout.Texts,
			LayoutText{
				Class:            "treemapLabel",
				X:                f.x + f.w/2.0,
				Y:                f.y + f.h/2.0,
				Value:            node.Label,
				Anchor:           "middle",
				Size:             labelSize,
				Color:            "#000000",
				DominantBaseline: "middle",
			},
			LayoutText{
				Class:            "treemapValue",
				X:                f.x + f.w/2.0,
				Y:                f.y + f.h/2.0 + valueSize*0.9,
				Value:            strconv.FormatFloat(node.Value, 'f', 0, 64),
				Anchor:           "middle",
				Size:             valueSize,
				Color:            "#000000",
				DominantBaseline: "hanging",
			},
		)
	}

	var layoutNode func(node *treeNode, depth int, f frame)
	layoutNode = func(node *treeNode, depth int, f frame) {
		if node == nil || f.w <= 1 || f.h <= 1 {
			return
		}
		if len(node.Children) == 0 {
			addLeaf(node, f, depth-1)
			return
		}

		fill, stroke := sectionColor(max(0, depth-1))
		rectClass := "treemapSection section" + intString(max(0, depth))
		if depth == 0 {
			fill = "transparent"
			stroke = "transparent"
		}
		layout.Rects = append(layout.Rects, LayoutRect{
			Class:         rectClass,
			X:             f.x,
			Y:             f.y,
			W:             f.w,
			H:             f.h,
			Fill:          fill,
			FillOpacity:   0.6,
			Stroke:        stroke,
			StrokeWidth:   2,
			StrokeOpacity: 0.4,
		})
		if depth > 0 {
			layout.Texts = append(layout.Texts,
				LayoutText{
					Class:            "treemapSectionLabel",
					X:                f.x + 6,
					Y:                f.y + 12.5,
					Value:            node.Label,
					Anchor:           "start",
					Size:             12,
					Weight:           "700",
					Color:            "#000000",
					DominantBaseline: "middle",
				},
				LayoutText{
					Class:            "treemapSectionValue",
					X:                f.x + f.w - 10,
					Y:                f.y + 12.5,
					Value:            strconv.FormatFloat(node.Value, 'f', 0, 64),
					Anchor:           "end",
					Size:             10,
					Color:            "#000000",
					DominantBaseline: "middle",
				},
			)
		}

		children := append([]*treeNode(nil), node.Children...)
		sort.Slice(children, func(i, j int) bool {
			return children[i].Value > children[j].Value
		})

		content := frame{
			x: f.x + 10,
			y: f.y + 35,
			w: max(1, f.w-20),
			h: max(1, f.h-45),
		}
		if len(children) == 0 {
			return
		}
		total := 0.0
		for _, child := range children {
			total += max(child.Value, 0.0001)
		}
		if total <= 0 {
			total = float64(len(children))
		}

		gap := 10.0
		horizontal := content.w > content.h*1.2
		if horizontal {
			available := content.w - float64(len(children)-1)*gap
			used := 0.0
			x := content.x
			for idx, child := range children {
				ratio := max(child.Value, 0.0001) / total
				childW := available * ratio
				if idx == len(children)-1 {
					childW = max(1, content.x+content.w-x)
				}
				layoutNode(child, depth+1, frame{x: x, y: content.y, w: childW, h: content.h})
				x += childW + gap
				used += childW
			}
			_ = used
			return
		}

		available := content.h - float64(len(children)-1)*gap
		y := content.y
		for idx, child := range children {
			ratio := max(child.Value, 0.0001) / total
			childH := available * ratio
			if idx == len(children)-1 {
				childH = max(1, content.y+content.h-y)
			}
			layoutNode(child, depth+1, frame{x: content.x, y: y, w: content.w, h: childH})
			y += childH + gap
		}
	}

	layoutNode(root, 0, frame{x: 0, y: 0, w: 1000, h: 400})
	layout.Width = 996
	layout.Height = 371
	layout.ViewBoxX = 2
	layout.ViewBoxY = 27
	layout.ViewBoxWidth = 996
	layout.ViewBoxHeight = 371
	return layout
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
