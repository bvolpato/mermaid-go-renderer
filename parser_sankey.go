package mermaid

import "strings"

func parseSankey(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramSankey)
	graph.Source = input

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if i == 0 && (strings.HasPrefix(lower(line), "sankey-beta") || strings.HasPrefix(lower(line), "sankey")) {
			continue
		}
		fields, ok := parseSankeyCSVRecord(line)
		if !ok {
			continue
		}
		source := stripQuotes(fields[0])
		target := stripQuotes(fields[1])
		value, okValue := parseFloat(fields[2])
		if !okValue || source == "" || target == "" {
			continue
		}
		graph.SankeyLinks = append(graph.SankeyLinks, SankeyLink{
			Source: source,
			Target: target,
			Value:  value,
		})
	}

	if len(graph.SankeyLinks) == 0 {
		return parseClassLike(input, DiagramSankey)
	}
	return ParseOutput{Graph: graph}, nil
}

func parseSankeyCSVRecord(line string) ([]string, bool) {
	fields := make([]string, 0, 3)
	var current strings.Builder
	inQuotes := false

	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch == '"' {
			if inQuotes && i+1 < len(line) && line[i+1] == '"' {
				current.WriteByte('"')
				i++
				continue
			}
			inQuotes = !inQuotes
			continue
		}
		if ch == ',' && !inQuotes {
			fields = append(fields, strings.TrimSpace(current.String()))
			current.Reset()
			continue
		}
		current.WriteByte(ch)
	}
	if inQuotes {
		return nil, false
	}
	fields = append(fields, strings.TrimSpace(current.String()))
	if len(fields) != 3 {
		return nil, false
	}
	return fields, true
}
