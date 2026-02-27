package mermaid

import "strings"

func parsePie(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}
	graph := newGraph(DiagramPie)
	graph.Source = input

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		low := lower(line)
		if strings.HasPrefix(low, "pie") {
			if strings.Contains(low, "showdata") {
				graph.PieShowData = true
			}
			if idx := strings.Index(low, "title"); idx >= 0 {
				title := strings.TrimSpace(line[idx+len("title"):])
				graph.PieTitle = stripQuotes(title)
			}
			continue
		}
		if strings.HasPrefix(low, "showdata") {
			graph.PieShowData = true
			continue
		}
		if strings.HasPrefix(low, "title ") {
			graph.PieTitle = stripQuotes(strings.TrimSpace(line[len("title "):]))
			continue
		}
		label, value, ok := parsePieSliceLine(line)
		if !ok {
			continue
		}
		graph.PieSlices = append(graph.PieSlices, PieSlice{Label: label, Value: value})
	}

	return ParseOutput{Graph: graph}, nil
}

func parsePieSliceLine(line string) (string, float64, bool) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", 0, false
	}
	label := stripQuotes(strings.TrimSpace(parts[0]))
	valueRaw := strings.TrimSpace(parts[1])
	if valueRaw == "" || label == "" {
		return "", 0, false
	}
	value, ok := parseFloat(valueRaw)
	if !ok || label == "" {
		return "", 0, false
	}
	return label, value, true
}
