package mermaid

import "strings"

func parseFlowchart(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramFlowchart)
	graph.Source = input

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
			if trimmed == "end" ||
				strings.HasPrefix(low, "subgraph ") ||
				strings.HasPrefix(low, "classdef ") ||
				strings.HasPrefix(low, "class ") ||
				strings.HasPrefix(low, "style ") ||
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
						addedAny = true
					}
				}
				if addedAny {
					continue
				}
			}

			if addEdgeFromLine(&graph, trimmed) {
				continue
			}

			if id, label, shape, ok := parseNodeOnly(trimmed); ok {
				graph.ensureNode(id, label, shape)
			}
		}
	}

	return ParseOutput{Graph: graph}, nil
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
