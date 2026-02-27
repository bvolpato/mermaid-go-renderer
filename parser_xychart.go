package mermaid

import "strings"

func parseXYChart(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}
	graph := newGraph(DiagramXYChart)
	graph.Source = input

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		low := lower(line)
		if i == 0 && strings.HasPrefix(low, "xychart") {
			if idx := strings.Index(low, "title "); idx >= 0 {
				graph.XYTitle = stripQuotes(strings.TrimSpace(line[idx+len("title "):]))
			}
			continue
		}
		if strings.HasPrefix(low, "title ") {
			graph.XYTitle = stripQuotes(strings.TrimSpace(line[len("title "):]))
			continue
		}
		if strings.HasPrefix(low, "x-axis ") || strings.HasPrefix(low, "xaxis ") {
			graph.XYXAxisLabel, graph.XYXCategories = parseXYXAxis(line)
			continue
		}
		if strings.HasPrefix(low, "y-axis ") || strings.HasPrefix(low, "yaxis ") {
			label, minValue, maxValue := parseXYYAxis(line)
			graph.XYYAxisLabel = label
			graph.XYYMin = minValue
			graph.XYYMax = maxValue
			continue
		}
		if series, ok := parseXYSeriesLine(line); ok {
			graph.XYSeries = append(graph.XYSeries, series)
		}
	}

	for i, series := range graph.XYSeries {
		id := "series_" + intString(i+1)
		label := series.Label
		if label == "" {
			label = string(series.Kind) + " " + intString(i+1)
		}
		graph.ensureNode(id, label, ShapeRectangle)
		if i > 0 {
			graph.addEdge(Edge{
				From:     "series_" + intString(i),
				To:       id,
				Directed: false,
				Style:    EdgeDotted,
			})
		}
	}

	return ParseOutput{Graph: graph}, nil
}

func parseXYXAxis(line string) (label string, categories []string) {
	content := strings.TrimSpace(line)
	content = strings.TrimPrefix(strings.TrimPrefix(content, "x-axis"), "xaxis")
	content = strings.TrimSpace(content)

	if strings.Contains(content, "[") && strings.Contains(content, "]") {
		categories = parseStringList(content)
		if len(categories) > 0 {
			return "", categories
		}
	}

	parts := strings.Split(content, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := stripQuotes(strings.TrimSpace(part))
		if value != "" {
			out = append(out, value)
		}
	}
	return "", out
}

func parseXYYAxis(line string) (label string, minValue, maxValue *float64) {
	content := strings.TrimSpace(line)
	content = strings.TrimPrefix(strings.TrimPrefix(content, "y-axis"), "yaxis")
	content = strings.TrimSpace(content)

	var rangePart string
	if idx := strings.Index(content, "-->"); idx >= 0 {
		rangePart = strings.TrimSpace(content[idx+3:])
		left := strings.TrimSpace(content[:idx])
		fields := strings.Fields(left)
		if len(fields) > 0 {
			if v, ok := parseFloat(fields[len(fields)-1]); ok {
				minValue = &v
				fields = fields[:len(fields)-1]
			}
			label = stripQuotes(strings.Join(fields, " "))
		}
		if v, ok := parseFloat(rangePart); ok {
			maxValue = &v
		}
		return label, minValue, maxValue
	}

	label = stripQuotes(content)
	return label, nil, nil
}

func parseXYSeriesLine(line string) (XYSeries, bool) {
	low := lower(line)
	kind := XYSeriesBar
	switch {
	case strings.HasPrefix(low, "bar "):
		kind = XYSeriesBar
	case strings.HasPrefix(low, "line "):
		kind = XYSeriesLine
	default:
		return XYSeries{}, false
	}

	content := strings.TrimSpace(line[len(strings.Fields(line)[0]):])
	values := parseFloatList(content)
	if len(values) == 0 {
		return XYSeries{}, false
	}
	return XYSeries{
		Kind:   kind,
		Values: values,
	}, true
}
