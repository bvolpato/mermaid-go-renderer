package mermaid

import "strings"

const (
	stateStartNodeID = "__state_start"
	stateEndNodeID   = "__state_end"
)

func parseStateDiagram(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramState)
	graph.Source = input

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		low := lower(line)

		if strings.HasPrefix(low, "statediagram") {
			continue
		}
		if dir, ok := parseDirectionLine(line); ok {
			graph.Direction = dir
			continue
		}
		if line == "{" || line == "}" || low == "end" ||
			strings.HasPrefix(low, "note ") ||
			strings.HasPrefix(low, "classdef ") ||
			strings.HasPrefix(low, "class ") {
			continue
		}

		if strings.HasPrefix(low, "state ") {
			parseStateDeclarationLine(&graph, line)
			continue
		}

		if parseStateLabelAssignment(&graph, line) {
			continue
		}

		if parseStateTransitionLine(&graph, line) {
			continue
		}
	}

	if node, ok := graph.Nodes[stateStartNodeID]; ok {
		node.Label = ""
		graph.Nodes[stateStartNodeID] = node
	}
	if node, ok := graph.Nodes[stateEndNodeID]; ok {
		node.Label = ""
		graph.Nodes[stateEndNodeID] = node
	}
	for id, node := range graph.Nodes {
		if node.Shape == ShapeDiamond {
			node.Label = ""
			graph.Nodes[id] = node
		}
	}

	return ParseOutput{Graph: graph}, nil
}

func parseStateDeclarationLine(graph *Graph, line string) {
	content := strings.TrimSpace(line[len("state "):])
	content = strings.TrimSuffix(strings.TrimSpace(content), "{")
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}

	shape := ShapeRectangle
	isChoice := false
	switch {
	case strings.Contains(lower(content), "<<choice>>"):
		shape = ShapeDiamond
		isChoice = true
		content = strings.ReplaceAll(content, "<<choice>>", "")
	case strings.Contains(lower(content), "<<fork>>"), strings.Contains(lower(content), "<<join>>"):
		content = strings.ReplaceAll(content, "<<fork>>", "")
		content = strings.ReplaceAll(content, "<<join>>", "")
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}

	low := lower(content)
	if idx := strings.Index(low, " as "); idx >= 0 {
		label := stripQuotes(strings.TrimSpace(content[:idx]))
		id := sanitizeID(strings.TrimSpace(content[idx+4:]), "")
		if id == "" {
			return
		}
		if label == "" {
			label = id
		}
		graph.ensureNode(id, label, shape)
		return
	}

	id, label, parsedShape, _ := parseNodeToken(content)
	if id == "" {
		fields := strings.Fields(content)
		if len(fields) == 0 {
			return
		}
		id = sanitizeID(fields[0], "")
		label = stripQuotes(fields[0])
	} else if parsedShape != "" && parsedShape != ShapeRectangle {
		shape = parsedShape
	}
	if id == "" {
		return
	}
	if label == "" {
		label = id
	}
	if isChoice {
		label = ""
	}
	graph.ensureNode(id, label, shape)
}

func parseStateLabelAssignment(graph *Graph, line string) bool {
	if strings.Contains(line, "-->") {
		return false
	}
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return false
	}
	left := strings.TrimSpace(line[:idx])
	right := stripQuotes(strings.TrimSpace(line[idx+1:]))
	if left == "" || right == "" {
		return false
	}
	id := sanitizeID(left, "")
	if id == "" {
		return false
	}
	node, exists := graph.Nodes[id]
	if !exists {
		graph.ensureNode(id, right, ShapeRectangle)
		return true
	}
	node.Label = right
	graph.Nodes[id] = node
	return true
}

func parseStateTransitionLine(graph *Graph, line string) bool {
	trimmed := line
	label := ""
	if idx := strings.Index(trimmed, ":"); idx >= 0 {
		label = strings.TrimSpace(trimmed[idx+1:])
		trimmed = strings.TrimSpace(trimmed[:idx])
	}

	leftRaw, _, rightRaw, meta, ok := parseEdgeLine(trimmed)
	if !ok {
		return false
	}
	leftID, leftLabel, leftShape := parseStateToken(leftRaw, true)
	rightID, rightLabel, rightShape := parseStateToken(rightRaw, false)
	if leftID == "" || rightID == "" {
		return false
	}

	if existing, ok := graph.Nodes[leftID]; ok {
		if leftShape == ShapeRectangle && existing.Shape != ShapeRectangle {
			leftShape = existing.Shape
		}
		if existing.Shape == ShapeDiamond && strings.TrimSpace(existing.Label) == "" {
			leftLabel = ""
		} else if leftLabel == leftID && strings.TrimSpace(existing.Label) != "" {
			leftLabel = existing.Label
		}
	}
	if existing, ok := graph.Nodes[rightID]; ok {
		if rightShape == ShapeRectangle && existing.Shape != ShapeRectangle {
			rightShape = existing.Shape
		}
		if existing.Shape == ShapeDiamond && strings.TrimSpace(existing.Label) == "" {
			rightLabel = ""
		} else if rightLabel == rightID && strings.TrimSpace(existing.Label) != "" {
			rightLabel = existing.Label
		}
	}

	graph.ensureNode(leftID, leftLabel, leftShape)
	graph.ensureNode(rightID, rightLabel, rightShape)
	graph.addEdge(Edge{
		From:       leftID,
		To:         rightID,
		Label:      stripQuotes(label),
		Directed:   true,
		ArrowStart: meta.arrowStart,
		ArrowEnd:   true,
		Style:      meta.style,
	})
	return true
}

func parseStateToken(raw string, isSource bool) (id, label string, shape NodeShape) {
	token := strings.TrimSpace(raw)
	if token == "" {
		return "", "", ""
	}
	if token == "[*]" {
		if isSource {
			return stateStartNodeID, "", ShapeCircle
		}
		return stateEndNodeID, "", ShapeDoubleCircle
	}

	id, label, shape, _ = parseNodeToken(token)
	if id == "" {
		id = sanitizeID(stripQuotes(token), "")
		label = stripQuotes(token)
		shape = ShapeRectangle
	}
	if label == "" {
		label = id
	}
	return id, label, shape
}
