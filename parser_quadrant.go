package mermaid

import "strings"

func parseQuadrant(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramQuadrant)
	graph.Source = input

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		low := lower(line)
		if i == 0 && strings.HasPrefix(low, "quadrantchart") {
			continue
		}
		switch {
		case strings.HasPrefix(low, "title "):
			graph.QuadrantTitle = stripQuotes(strings.TrimSpace(line[len("title "):]))
		case strings.HasPrefix(low, "x-axis "):
			left, right := parseAxisPair(line[len("x-axis "):])
			graph.QuadrantXAxisLeft = left
			graph.QuadrantXAxisRight = right
		case strings.HasPrefix(low, "y-axis "):
			bottom, top := parseAxisPair(line[len("y-axis "):])
			graph.QuadrantYAxisBottom = bottom
			graph.QuadrantYAxisTop = top
		case strings.HasPrefix(low, "quadrant-1 "):
			graph.QuadrantLabels[0] = stripQuotes(strings.TrimSpace(line[len("quadrant-1 "):]))
		case strings.HasPrefix(low, "quadrant-2 "):
			graph.QuadrantLabels[1] = stripQuotes(strings.TrimSpace(line[len("quadrant-2 "):]))
		case strings.HasPrefix(low, "quadrant-3 "):
			graph.QuadrantLabels[2] = stripQuotes(strings.TrimSpace(line[len("quadrant-3 "):]))
		case strings.HasPrefix(low, "quadrant-4 "):
			graph.QuadrantLabels[3] = stripQuotes(strings.TrimSpace(line[len("quadrant-4 "):]))
		default:
			point, ok := parseQuadrantPoint(line)
			if ok {
				graph.QuadrantPoints = append(graph.QuadrantPoints, point)
				id := sanitizeID(point.Label, "point_"+intString(len(graph.QuadrantPoints)))
				graph.ensureNode(id, point.Label, ShapeCircle)
			}
		}
	}

	return ParseOutput{Graph: graph}, nil
}

func parseAxisPair(input string) (string, string) {
	parts := strings.SplitN(input, "-->", 2)
	if len(parts) != 2 {
		label := stripQuotes(strings.TrimSpace(input))
		return label, ""
	}
	left := stripQuotes(strings.TrimSpace(parts[0]))
	right := stripQuotes(strings.TrimSpace(parts[1]))
	return left, right
}

func parseQuadrantPoint(line string) (QuadrantPoint, bool) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return QuadrantPoint{}, false
	}
	label := stripQuotes(strings.TrimSpace(parts[0]))
	coords := strings.TrimSpace(parts[1])
	coords = strings.TrimPrefix(coords, "[")
	coords = strings.TrimSuffix(coords, "]")
	xy := strings.Split(coords, ",")
	if len(xy) != 2 {
		return QuadrantPoint{}, false
	}
	x, okX := parseFloat(xy[0])
	y, okY := parseFloat(xy[1])
	if !okX || !okY || label == "" {
		return QuadrantPoint{}, false
	}
	return QuadrantPoint{
		Label: label,
		X:     x,
		Y:     y,
	}, true
}
