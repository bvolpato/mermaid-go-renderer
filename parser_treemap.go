package mermaid

import "strings"

func parseTreemap(input string) (ParseOutput, error) {
	lines, err := preprocessInputKeepIndent(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramTreemap)
	graph.Source = input

	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(lower(trimmed), "treemap") {
			continue
		}

		depth := treemapDepth(raw)
		labelPart := trimmed
		value := 0.0
		hasValue := false
		if idx := strings.LastIndex(trimmed, ":"); idx > 0 {
			left := strings.TrimSpace(trimmed[:idx])
			right := strings.TrimSpace(trimmed[idx+1:])
			if v, ok := parseFloat(right); ok {
				labelPart = left
				value = v
				hasValue = true
			}
		}

		label := stripQuotes(strings.TrimSpace(labelPart))
		if label == "" {
			continue
		}
		graph.TreemapItems = append(graph.TreemapItems, TreemapItem{
			Depth:    depth,
			Label:    label,
			Value:    value,
			HasValue: hasValue,
		})
	}

	if len(graph.TreemapItems) == 0 {
		return parseClassLike(input, DiagramTreemap)
	}

	return ParseOutput{Graph: graph}, nil
}

func treemapDepth(raw string) int {
	indent := 0
	for _, ch := range raw {
		if ch == ' ' {
			indent++
			continue
		}
		if ch == '\t' {
			indent += 2
			continue
		}
		break
	}
	return indent / 2
}
