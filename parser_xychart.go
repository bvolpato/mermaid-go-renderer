package mermaid

import "strings"

func parseXYChart(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}
	graph := newGraph(DiagramXYChart)
	graph.Source = input
	graph.Direction = DirectionLeftRight

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		low := lower(line)
		if strings.HasPrefix(low, "xychart") {
			continue
		}
		if strings.HasPrefix(low, "title") {
			graph.XYTitle = stripQuotes(strings.TrimSpace(line[len("title"):]))
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

	return ParseOutput{Graph: graph}, nil
}

func parseXYXAxis(line string) (label string, categories []string) {
	content := strings.TrimSpace(line)
	content = strings.TrimPrefix(strings.TrimPrefix(content, "x-axis"), "xaxis")
	content = strings.TrimSpace(content)

	if idx := strings.Index(content, "["); idx >= 0 {
		labelPart := strings.TrimSpace(content[:idx])
		if labelPart != "" {
			label = stripQuotes(labelPart)
		}
		categories = parseXYAxisCategories(content[idx:])
		return label, categories
	}
	return "", parseXYAxisCategories(content)
}

func parseXYYAxis(line string) (label string, minValue, maxValue *float64) {
	content := strings.TrimSpace(line)
	content = strings.TrimPrefix(strings.TrimPrefix(content, "y-axis"), "yaxis")
	content = strings.TrimSpace(content)

	if idx := strings.Index(content, "-->"); idx >= 0 {
		left := strings.TrimSpace(content[:idx])
		right := strings.TrimSpace(content[idx+3:])
		fields := strings.Fields(left)
		minRaw := "0"
		if len(fields) > 0 {
			minRaw = fields[len(fields)-1]
		}
		if v, ok := parseFloat(minRaw); ok {
			minValue = &v
		}
		if v, ok := parseFloat(right); ok {
			maxValue = &v
		}
		labelPart := strings.TrimSpace(strings.TrimSuffix(left, minRaw))
		if labelPart != "" {
			label = stripQuotes(labelPart)
		}
		return label, minValue, maxValue
	}

	label = stripQuotes(content)
	return label, nil, nil
}

func parseXYSeriesLine(line string) (XYSeries, bool) {
	low := lower(line)
	kind := XYSeriesBar
	rest := ""
	switch {
	case strings.HasPrefix(low, "bar "):
		kind = XYSeriesBar
		rest = strings.TrimSpace(line[len("bar"):])
	case strings.HasPrefix(low, "line "):
		kind = XYSeriesLine
		rest = strings.TrimSpace(line[len("line"):])
	default:
		return XYSeries{}, false
	}

	label := ""
	valuesRaw := rest
	if idx := strings.Index(rest, "["); idx >= 0 {
		labelPart := strings.TrimSpace(rest[:idx])
		if labelPart != "" {
			label = stripQuotes(labelPart)
		}
		valuesRaw = rest[idx:]
	}
	values := parseFloatList(valuesRaw)
	if len(values) == 0 {
		return XYSeries{}, false
	}
	return XYSeries{
		Kind:   kind,
		Label:  label,
		Values: values,
	}, true
}

func parseXYAxisCategories(rest string) []string {
	trimmed := strings.TrimSpace(rest)
	if trimmed == "" {
		return nil
	}
	content := trimmed
	if open := strings.Index(trimmed, "["); open >= 0 {
		if close := strings.LastIndex(trimmed, "]"); close > open {
			content = trimmed[open+1 : close]
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
	return out
}
