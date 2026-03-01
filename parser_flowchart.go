package mermaid

import "strings"

func parseFlowchart(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramFlowchart)
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
		if subgraphNodeSets[subgraphIdx] == nil {
			subgraphNodeSets[subgraphIdx] = map[string]struct{}{}
		}
		if _, exists := subgraphNodeSets[subgraphIdx][nodeID]; exists {
			return
		}
		subgraphNodeSets[subgraphIdx][nodeID] = struct{}{}
		graph.FlowSubgraphs[subgraphIdx].NodeIDs = append(graph.FlowSubgraphs[subgraphIdx].NodeIDs, nodeID)
	}
	addNodesToActiveSubgraphs := func(nodeIDs []string) {
		for _, nodeID := range nodeIDs {
			for _, subgraphIdx := range activeSubgraphs {
				addNodeToSubgraph(subgraphIdx, nodeID)
			}
		}
	}

	for _, rawLine := range lines {
		for _, line := range splitStatements(rawLine) {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}

			if dir, ok := parseFlowchartHeaderDirection(trimmed); ok {
				graph.Direction = dir
				continue
			}

			if dir, ok := parseDirectionLine(trimmed); ok {
				graph.Direction = dir
				continue
			}

			low := lower(trimmed)
			if trimmed == "end" {
				if len(activeSubgraphs) > 0 {
					activeSubgraphs = activeSubgraphs[:len(activeSubgraphs)-1]
				}
				continue
			}
			if strings.HasPrefix(low, "subgraph ") {
				subgraphRaw := strings.TrimSpace(trimmed[len("subgraph "):])
				subgraphID, subgraphLabel, _, _ := parseNodeToken(subgraphRaw)
				if subgraphID == "" {
					subgraphID = sanitizeID(subgraphRaw, "subgraph_"+intString(len(graph.FlowSubgraphs)+1))
				}
				if subgraphLabel == "" {
					subgraphLabel = stripQuotes(subgraphRaw)
				}
				if subgraphLabel == "" {
					subgraphLabel = subgraphID
				}
				graph.FlowSubgraphs = append(graph.FlowSubgraphs, FlowSubgraph{
					ID:      subgraphID,
					Label:   subgraphLabel,
					NodeIDs: []string{},
				})
				subgraphNodeSets = append(subgraphNodeSets, map[string]struct{}{})
				activeSubgraphs = append(activeSubgraphs, len(graph.FlowSubgraphs)-1)
				continue
			}
			if strings.HasPrefix(low, "style ") {
				parseFlowchartStyleDirective(&graph, trimmed)
				continue
			}
			if strings.HasPrefix(low, "classdef ") ||
				strings.HasPrefix(low, "class ") ||
				strings.HasPrefix(low, "linkstyle ") ||
				strings.HasPrefix(low, "click ") ||
				strings.HasPrefix(low, "title ") ||
				strings.HasPrefix(low, "accdescr") ||
				strings.HasPrefix(low, "acctitle") {
				continue
			}

			if statements := splitEdgeChain(trimmed); len(statements) > 0 {
				addedAny := false
				for _, stmt := range statements {
					if addEdgeFromLine(&graph, stmt) {
						addNodesToActiveSubgraphs(flowchartEdgeNodeIDs(stmt))
						addedAny = true
					}
				}
				if addedAny {
					continue
				}
			}

			if addEdgeFromLine(&graph, trimmed) {
				addNodesToActiveSubgraphs(flowchartEdgeNodeIDs(trimmed))
				continue
			}

			if id, label, shape, ok := parseNodeOnly(trimmed); ok {
				graph.ensureNode(id, label, shape)
				addNodesToActiveSubgraphs([]string{id})
			}
		}
	}

	return ParseOutput{Graph: graph}, nil
}

func parseFlowchartStyleDirective(graph *Graph, line string) {
	rest := strings.TrimSpace(line[len("style "):])
	if rest == "" {
		return
	}
	parts := strings.SplitN(rest, " ", 2)
	nodeID := sanitizeID(parts[0], "")
	if nodeID == "" {
		return
	}
	graph.ensureNode(nodeID, nodeID, ShapeRectangle)
	if len(parts) < 2 {
		return
	}
	node := graph.Nodes[nodeID]
	for _, decl := range strings.Split(parts[1], ",") {
		kv := strings.SplitN(strings.TrimSpace(decl), ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := lower(strings.TrimSpace(kv[0]))
		value := strings.TrimSpace(strings.TrimSuffix(kv[1], ";"))
		switch key {
		case "fill":
			node.Fill = value
		case "stroke":
			node.Stroke = value
		case "stroke-width":
			value = strings.TrimSuffix(strings.TrimSpace(value), "px")
			if strokeWidth, ok := parseFloat(value); ok {
				node.StrokeWidth = strokeWidth
			}
		}
	}
	graph.Nodes[nodeID] = node
}

func flowchartEdgeNodeIDs(line string) []string {
	left, _, right, _, ok := parseEdgeLine(line)
	if !ok {
		return nil
	}
	ids := make([]string, 0, 4)
	for _, source := range splitNodeList(left) {
		id, _, _, _ := parseNodeToken(source)
		if id != "" {
			ids = append(ids, id)
		}
	}
	for _, target := range splitNodeList(right) {
		id, _, _, _ := parseNodeToken(target)
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func parseFlowchartHeaderDirection(line string) (Direction, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", false
	}
	kind := lower(fields[0])
	if kind != "flowchart" && kind != "graph" {
		return "", false
	}
	return directionFromToken(fields[1]), true
}
