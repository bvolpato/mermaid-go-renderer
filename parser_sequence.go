package mermaid

import (
	"regexp"
	"strings"
)

var sequenceMessageRe = regexp.MustCompile(`^\s*([^\s:]+)\s*([-.<>=x]+)\s*([^\s:]+)\s*:\s*(.+)\s*$`)

func parseSequence(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramSequence)
	graph.Source = input
	participantSet := map[string]struct{}{}

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if i == 0 && strings.HasPrefix(lower(line), "sequencediagram") {
			continue
		}
		if line == "" {
			continue
		}

		if name, ok := parseSequenceParticipant(line); ok {
			if _, exists := participantSet[name]; !exists {
				participantSet[name] = struct{}{}
				graph.SequenceParticipants = append(graph.SequenceParticipants, name)
			}
			continue
		}

		if msg, ok := parseSequenceMessage(line); ok {
			graph.SequenceMessages = append(graph.SequenceMessages, msg)
			for _, participant := range []string{msg.From, msg.To} {
				if _, exists := participantSet[participant]; !exists {
					participantSet[participant] = struct{}{}
					graph.SequenceParticipants = append(graph.SequenceParticipants, participant)
				}
			}
			continue
		}
		graph.GenericLines = append(graph.GenericLines, line)
	}

	for _, participant := range graph.SequenceParticipants {
		graph.ensureNode(participant, participant, ShapeRectangle)
	}
	for _, msg := range graph.SequenceMessages {
		graph.addEdge(Edge{
			From:     msg.From,
			To:       msg.To,
			Label:    msg.Label,
			Directed: strings.Contains(msg.Arrow, ">"),
			ArrowEnd: strings.Contains(msg.Arrow, ">"),
			Style:    edgeStyleFromArrow(msg.Arrow),
		})
	}

	return ParseOutput{Graph: graph}, nil
}

func parseSequenceParticipant(line string) (string, bool) {
	low := lower(line)
	if !strings.HasPrefix(low, "participant ") && !strings.HasPrefix(low, "actor ") {
		return "", false
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", false
	}
	if len(fields) >= 4 && lower(fields[2]) == "as" {
		return stripQuotes(fields[1]), true
	}
	return stripQuotes(fields[1]), true
}

func parseSequenceMessage(line string) (SequenceMessage, bool) {
	idx := sequenceMessageRe.FindStringSubmatchIndex(line)
	if idx == nil || len(idx) < 10 {
		return SequenceMessage{}, false
	}
	from := stripQuotes(strings.TrimSpace(submatch(line, idx, 1)))
	arrow := strings.TrimSpace(submatch(line, idx, 2))
	to := stripQuotes(strings.TrimSpace(submatch(line, idx, 3)))
	label := strings.TrimSpace(submatch(line, idx, 4))
	if from == "" || to == "" || label == "" {
		return SequenceMessage{}, false
	}
	return SequenceMessage{
		From:  from,
		To:    to,
		Label: label,
		Arrow: arrow,
	}, true
}

func edgeStyleFromArrow(arrow string) EdgeStyle {
	switch {
	case strings.Contains(arrow, "."):
		return EdgeDotted
	case strings.Contains(arrow, "="):
		return EdgeThick
	default:
		return EdgeSolid
	}
}
