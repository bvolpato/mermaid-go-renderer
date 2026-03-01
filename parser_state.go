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
	subgraphNodeSets := make([]map[string]struct{}, 0, 8)
	activeSubgraphs := make([]int, 0, 4)

	addNodeToSubgraph := func(subgraphIdx int, nodeID string) {
		if subgraphIdx < 0 || subgraphIdx >= len(graph.FlowSubgraphs) {
			return
		}
		nodeID = strings.TrimSpace(nodeID)
		if nodeID == "" {
			return
		}
		if nodeID == stateStartNodeID || nodeID == stateEndNodeID {
			return
		}
		if subgraphNodeSets[subgraphIdx] == nil {
			subgraphNodeSets[subgraphIdx] = map[string]struct{}{}
		}
		if _, exists := subgraphNodeSets[subgraphIdx][nodeID]; exists {
			return
		}
		subgraphNodeSets[subgraphIdx][nodeID] = struct{}{}
		graph.FlowSubgraphs[subgraphIdx].NodeIDs = append(graph.FlowSubgraphs[subgraphIdx].NodeIDs, nodeID)
	}
	addNewNodesToActiveSubgraphs := func(prevNodeCount int) {
		if len(activeSubgraphs) == 0 || prevNodeCount >= len(graph.NodeOrder) {
			return
		}
		for _, nodeID := range graph.NodeOrder[prevNodeCount:] {
			for _, subgraphIdx := range activeSubgraphs {
				addNodeToSubgraph(subgraphIdx, nodeID)
			}
		}
	}

	for _, raw := range lines {
		prevNodeCount := len(graph.NodeOrder)
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
		if line == "{" || low == "end" ||
			strings.HasPrefix(low, "note ") ||
			strings.HasPrefix(low, "classdef ") ||
			strings.HasPrefix(low, "class ") {
			continue
		}
		if line == "}" {
			if len(activeSubgraphs) > 0 {
				activeSubgraphs = activeSubgraphs[:len(activeSubgraphs)-1]
			}
			continue
		}

		if strings.HasPrefix(low, "state ") {
			parseStateDeclarationLine(&graph, line)
			addNewNodesToActiveSubgraphs(prevNodeCount)
			if strings.HasSuffix(strings.TrimSpace(line), "{") {
				compositeID := parseStateCompositeID(line)
				if compositeID != "" {
					label := compositeID
					if node, ok := graph.Nodes[compositeID]; ok && strings.TrimSpace(node.Label) != "" {
						label = node.Label
					}
					graph.FlowSubgraphs = append(graph.FlowSubgraphs, FlowSubgraph{
						ID:      compositeID,
						Label:   label,
						NodeIDs: []string{},
					})
					subgraphNodeSets = append(subgraphNodeSets, map[string]struct{}{})
					activeSubgraphs = append(activeSubgraphs, len(graph.FlowSubgraphs)-1)
				}
			}
			continue
		}

		if parseStateLabelAssignment(&graph, line) {
			addNewNodesToActiveSubgraphs(prevNodeCount)
			continue
		}

		scopeID := ""
		if len(activeSubgraphs) > 0 {
			scopeParts := make([]string, 0, len(activeSubgraphs))
			for _, subgraphIdx := range activeSubgraphs {
				if subgraphIdx < 0 || subgraphIdx >= len(graph.FlowSubgraphs) {
					continue
				}
				scopePart := sanitizeID(graph.FlowSubgraphs[subgraphIdx].ID, "")
				if scopePart == "" {
					continue
				}
				scopeParts = append(scopeParts, scopePart)
			}
			if len(scopeParts) > 0 {
				scopeID = strings.Join(scopeParts, "__")
			}
		}
		if parseStateTransitionLine(&graph, line, scopeID) {
			addNewNodesToActiveSubgraphs(prevNodeCount)
			continue
		}
	}

	for _, subgraph := range graph.FlowSubgraphs {
		subgraphID := sanitizeID(strings.TrimSpace(subgraph.ID), "")
		if subgraphID == "" {
			continue
		}
		startID := stateStartNodeID + "_" + subgraphID
		endID := stateEndNodeID + "_" + subgraphID
		_, hasStart := graph.Nodes[startID]
		_, hasEnd := graph.Nodes[endID]
		for idx := range graph.Edges {
			if hasStart && graph.Edges[idx].To == subgraphID {
				graph.Edges[idx].To = startID
			}
			if hasEnd && graph.Edges[idx].From == subgraphID {
				graph.Edges[idx].From = endID
			}
		}
		if node, ok := graph.Nodes[subgraphID]; ok {
			node.Shape = ShapeHidden
			node.Label = ""
			graph.Nodes[subgraphID] = node
		}
	}

	if _, ok := graph.Nodes[stateStartNodeID]; !ok {
		graph.Nodes[stateStartNodeID] = Node{ID: stateStartNodeID, Shape: ShapeCircle, Label: ""}
	}
	if _, ok := graph.Nodes[stateEndNodeID]; !ok {
		graph.Nodes[stateEndNodeID] = Node{ID: stateEndNodeID, Shape: ShapeDoubleCircle, Label: ""}
	}

	for nodeID, node := range graph.Nodes {
		if strings.HasPrefix(nodeID, stateStartNodeID) || strings.HasPrefix(nodeID, stateEndNodeID) {
			node.Label = ""
			graph.Nodes[nodeID] = node
		}
	}

	return ParseOutput{Graph: graph}, nil
}

func parseStateCompositeID(line string) string {
	content := strings.TrimSpace(line)
	if !strings.HasPrefix(lower(content), "state ") {
		return ""
	}
	content = strings.TrimSpace(content[len("state "):])
	content = strings.TrimSuffix(strings.TrimSpace(content), "{")
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	content = strings.ReplaceAll(content, "<<choice>>", "")
	content = strings.ReplaceAll(content, "<<fork>>", "")
	content = strings.ReplaceAll(content, "<<join>>", "")
	content = strings.TrimSpace(content)
	if idx := strings.Index(lower(content), " as "); idx >= 0 {
		return sanitizeID(strings.TrimSpace(content[idx+4:]), "")
	}
	id, _, _, _ := parseNodeToken(content)
	if id != "" {
		return id
	}
	fields := strings.Fields(content)
	if len(fields) == 0 {
		return ""
	}
	return sanitizeID(fields[0], "")
}

func parseStateDeclarationLine(graph *Graph, line string) {
	content := strings.TrimSpace(line[len("state "):])
	content = strings.TrimSuffix(strings.TrimSpace(content), "{")
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}

	shape := ShapeRectangle
	switch {
	case strings.Contains(lower(content), "<<choice>>"):
		shape = ShapeDiamond
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

func parseStateTransitionLine(graph *Graph, line string, scopeID string) bool {
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
	leftID, leftLabel, leftShape := parseStateToken(leftRaw, true, scopeID)
	rightID, rightLabel, rightShape := parseStateToken(rightRaw, false, scopeID)
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

func parseStateToken(raw string, isSource bool, scopeID string) (id, label string, shape NodeShape) {
	token := strings.TrimSpace(raw)
	if token == "" {
		return "", "", ""
	}
	if token == "[*]" {
		if strings.TrimSpace(scopeID) != "" {
			scopedID := sanitizeID(scopeID, "")
			if scopedID != "" {
				if isSource {
					return stateStartNodeID + "_" + scopedID, "", ShapeCircle
				}
				return stateEndNodeID + "_" + scopedID, "", ShapeDoubleCircle
			}
		}
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
