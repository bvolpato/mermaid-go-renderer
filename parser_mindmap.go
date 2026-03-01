package mermaid

import "strings"

func parseMindmap(input string) (ParseOutput, error) {
	lines, err := preprocessInputKeepIndent(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramMindmap)
	graph.Source = input
	type stackItem struct {
		indent int
		id     string
		level  int
	}
	stack := make([]stackItem, 0, 16)

	for i, raw := range lines {
		if i == 0 && strings.HasPrefix(lower(strings.TrimSpace(raw)), "mindmap") {
			continue
		}

		indent := countIndent(raw)
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}

		label, shape := parseMindmapNode(trimmed)
		if shape == "" {
			shape = ShapeRoundRect
		}
		id := sanitizeID(label, "mindmap_"+intString(len(graph.MindmapNodes)+1))
		level := 0
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		parentID := ""
		if len(stack) > 0 {
			parent := stack[len(stack)-1]
			parentID = parent.id
			level = parent.level + 1
		}

		if len(graph.MindmapNodes) == 0 {
			graph.MindmapRootID = id
			level = 0
		}

		graph.MindmapNodes = append(graph.MindmapNodes, MindmapNode{
			ID:     id,
			Label:  label,
			Level:  level,
			Parent: parentID,
			Shape:  shape,
		})
		graph.ensureNode(id, label, shape)
		if parentID != "" {
			graph.addEdge(Edge{
				From:     parentID,
				To:       id,
				Directed: false,
				Style:    EdgeSolid,
			})
		}
		stack = append(stack, stackItem{indent: indent, id: id, level: level})
	}

	return ParseOutput{Graph: graph}, nil
}

func parseMindmapNode(line string) (label string, shape NodeShape) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "::") {
		trimmed = strings.TrimPrefix(trimmed, "::")
	}

	// Plain mindmap text nodes can contain spaces ("Branch A", "Leaf B1").
	// parseNodeToken treats the first token as ID, which truncates labels,
	// so preserve the full text when no explicit shape syntax is present.
	hasExplicitShape := strings.ContainsAny(trimmed, "[](){}")
	if !hasExplicitShape {
		label = strings.TrimSpace(stripQuotes(trimmed))
		shape = ShapeRoundRect
	} else {
		_, parsedLabel, parsedShape, _ := parseNodeToken(trimmed)
		label = strings.TrimSpace(parsedLabel)
		shape = parsedShape
		if label == "" {
			label = stripQuotes(trimmed)
		}
	}
	if label == "" {
		label = "node"
	}
	return label, shape
}
