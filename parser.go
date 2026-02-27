package mermaid

import (
	"errors"
	"strings"
)

type ParseOutput struct {
	Graph Graph
}

func ParseMermaid(input string) (ParseOutput, error) {
	kind := detectDiagramKind(input)

	switch kind {
	case DiagramFlowchart:
		return parseFlowchart(input)
	case DiagramSequence:
		return parseSequence(input)
	case DiagramClass:
		return parseClassLike(input, DiagramClass)
	case DiagramState:
		return parseClassLike(input, DiagramState)
	case DiagramER:
		return parseClassLike(input, DiagramER)
	case DiagramPie:
		return parsePie(input)
	case DiagramMindmap:
		return parseMindmap(input)
	case DiagramJourney:
		return parseJourney(input)
	case DiagramTimeline:
		return parseTimeline(input)
	case DiagramGantt:
		return parseGantt(input)
	case DiagramRequirement:
		return parseClassLike(input, DiagramRequirement)
	case DiagramGitGraph:
		return parseGitGraph(input)
	case DiagramC4:
		return parseClassLike(input, DiagramC4)
	case DiagramSankey:
		return parseClassLike(input, DiagramSankey)
	case DiagramQuadrant:
		return parseQuadrant(input)
	case DiagramZenUML:
		return parseClassLike(input, DiagramZenUML)
	case DiagramBlock:
		return parseClassLike(input, DiagramBlock)
	case DiagramPacket:
		return parseClassLike(input, DiagramPacket)
	case DiagramKanban:
		return parseClassLike(input, DiagramKanban)
	case DiagramArchitecture:
		return parseClassLike(input, DiagramArchitecture)
	case DiagramRadar:
		return parseClassLike(input, DiagramRadar)
	case DiagramTreemap:
		return parseClassLike(input, DiagramTreemap)
	case DiagramXYChart:
		return parseXYChart(input)
	default:
		return parseFlowchart(input)
	}
}

func detectDiagramKind(input string) DiagramKind {
	for _, rawLine := range strings.Split(input, "\n") {
		line := stripTrailingComment(strings.TrimSpace(rawLine))
		if line == "" {
			continue
		}
		l := lower(line)
		switch {
		case strings.HasPrefix(l, "sequencediagram"):
			return DiagramSequence
		case strings.HasPrefix(l, "classdiagram"):
			return DiagramClass
		case strings.HasPrefix(l, "statediagram"):
			return DiagramState
		case strings.HasPrefix(l, "erdiagram"):
			return DiagramER
		case strings.HasPrefix(l, "pie"):
			return DiagramPie
		case strings.HasPrefix(l, "mindmap"):
			return DiagramMindmap
		case strings.HasPrefix(l, "journey"):
			return DiagramJourney
		case strings.HasPrefix(l, "timeline"):
			return DiagramTimeline
		case strings.HasPrefix(l, "gantt"):
			return DiagramGantt
		case strings.HasPrefix(l, "requirementdiagram"):
			return DiagramRequirement
		case strings.HasPrefix(l, "gitgraph"):
			return DiagramGitGraph
		case strings.HasPrefix(l, "c4"):
			return DiagramC4
		case strings.HasPrefix(l, "sankey"):
			return DiagramSankey
		case strings.HasPrefix(l, "quadrantchart"):
			return DiagramQuadrant
		case strings.HasPrefix(l, "zenuml"):
			return DiagramZenUML
		case strings.HasPrefix(l, "block"):
			return DiagramBlock
		case strings.HasPrefix(l, "packet"):
			return DiagramPacket
		case strings.HasPrefix(l, "kanban"):
			return DiagramKanban
		case strings.HasPrefix(l, "architecture"):
			return DiagramArchitecture
		case strings.HasPrefix(l, "radar"):
			return DiagramRadar
		case strings.HasPrefix(l, "treemap"):
			return DiagramTreemap
		case strings.HasPrefix(l, "xychart"):
			return DiagramXYChart
		case strings.HasPrefix(l, "flowchart"), strings.HasPrefix(l, "graph"):
			return DiagramFlowchart
		}
	}
	return DiagramFlowchart
}

func preprocessInput(input string) ([]string, error) {
	lines := make([]string, 0, 64)
	for _, raw := range strings.Split(input, "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "%%{") || strings.HasPrefix(trimmed, "%%") {
			continue
		}
		trimmed = stripTrailingComment(trimmed)
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
	}
	if len(lines) == 0 {
		return nil, errors.New("no mermaid content found")
	}
	return lines, nil
}

func preprocessInputKeepIndent(input string) ([]string, error) {
	lines := make([]string, 0, 64)
	for _, raw := range strings.Split(input, "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "%%{") || strings.HasPrefix(trimmed, "%%") {
			continue
		}
		withoutComment := stripTrailingCommentKeepIndent(raw)
		if strings.TrimSpace(withoutComment) == "" {
			continue
		}
		lines = append(lines, withoutComment)
	}
	if len(lines) == 0 {
		return nil, errors.New("no mermaid content found")
	}
	return lines, nil
}

func parseClassLike(input string, kind DiagramKind) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}
	graph := newGraph(kind)
	graph.Source = input

	for idx, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if idx == 0 && isHeaderLineForKind(line, kind) {
			continue
		}
		if line == "" {
			continue
		}

		if statements := splitEdgeChain(line); len(statements) > 0 {
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

		if addEdgeFromLine(&graph, line) {
			continue
		}

		if id, label, shape, ok := parseNodeOnly(line); ok {
			graph.ensureNode(id, label, shape)
			continue
		}
		graph.GenericLines = append(graph.GenericLines, line)
	}

	if len(graph.NodeOrder) == 0 && len(graph.GenericLines) > 0 {
		for i, line := range graph.GenericLines {
			id := "line_" + intString(i+1)
			graph.ensureNode(id, line, ShapeRectangle)
			if i > 0 {
				graph.addEdge(Edge{
					From:     "line_" + intString(i),
					To:       id,
					Directed: true,
					ArrowEnd: true,
					Style:    EdgeSolid,
				})
			}
		}
	}

	return ParseOutput{Graph: graph}, nil
}

func isHeaderLineForKind(line string, kind DiagramKind) bool {
	l := lower(line)
	switch kind {
	case DiagramClass:
		return strings.HasPrefix(l, "classdiagram")
	case DiagramState:
		return strings.HasPrefix(l, "statediagram")
	case DiagramER:
		return strings.HasPrefix(l, "erdiagram")
	case DiagramRequirement:
		return strings.HasPrefix(l, "requirementdiagram")
	case DiagramC4:
		return strings.HasPrefix(l, "c4")
	case DiagramSankey:
		return strings.HasPrefix(l, "sankey")
	case DiagramZenUML:
		return strings.HasPrefix(l, "zenuml")
	case DiagramBlock:
		return strings.HasPrefix(l, "block")
	case DiagramPacket:
		return strings.HasPrefix(l, "packet")
	case DiagramKanban:
		return strings.HasPrefix(l, "kanban")
	case DiagramArchitecture:
		return strings.HasPrefix(l, "architecture")
	case DiagramRadar:
		return strings.HasPrefix(l, "radar")
	case DiagramTreemap:
		return strings.HasPrefix(l, "treemap")
	default:
		return false
	}
}
