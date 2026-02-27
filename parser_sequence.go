package mermaid

import (
	"regexp"
	"strings"
)

var sequenceMessageRe = regexp.MustCompile(`^\s*([^\s:]+?)\s*([-.<>=x()/\\|+]+)\s*([^\s:]+?)\s*:\s*(.+)\s*$`)
var sequenceInlineAliasRe = regexp.MustCompile(`(?i)"alias"\s*:\s*"([^"]+)"`)

func parseSequence(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramSequence)
	graph.Source = input
	participantSet := map[string]struct{}{}
	type blockKind string
	const (
		blockAlt blockKind = "alt"
		blockPar blockKind = "par"
	)
	var blockStack []blockKind
	messageIdx := 0

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if i == 0 && strings.HasPrefix(lower(line), "sequencediagram") {
			continue
		}
		if line == "" {
			continue
		}

		if participantID, participantLabel, ok := parseSequenceParticipant(line); ok {
			if _, exists := participantSet[participantID]; !exists {
				participantSet[participantID] = struct{}{}
				graph.SequenceParticipants = append(graph.SequenceParticipants, participantID)
			}
			if participantLabel == "" {
				participantLabel = participantID
			}
			graph.SequenceParticipantLabels[participantID] = participantLabel
			continue
		}

		if msg, ok := parseSequenceMessage(line); ok {
			msg.IsReturn = strings.Contains(sequenceArrowBase(msg.Arrow), "--")
			graph.SequenceMessages = append(graph.SequenceMessages, msg)
			graph.SequenceEvents = append(graph.SequenceEvents, SequenceEvent{
				Kind:         SequenceEventMessage,
				MessageIndex: messageIdx,
			})
			if sequenceArrowActivationStart(msg.Arrow) {
				graph.SequenceEvents = append(graph.SequenceEvents, SequenceEvent{
					Kind:  SequenceEventActivateStart,
					Actor: msg.To,
				})
			}
			if sequenceArrowActivationEnd(msg.Arrow) {
				graph.SequenceEvents = append(graph.SequenceEvents, SequenceEvent{
					Kind:  SequenceEventActivateEnd,
					Actor: msg.From,
				})
			}
			messageIdx++
			for _, participant := range []string{msg.From, msg.To} {
				if _, exists := participantSet[participant]; !exists {
					participantSet[participant] = struct{}{}
					graph.SequenceParticipants = append(graph.SequenceParticipants, participant)
				}
			}
			continue
		}

		if actor, ok := parseSequenceActivationLine(line, true); ok {
			graph.SequenceEvents = append(graph.SequenceEvents, SequenceEvent{
				Kind:  SequenceEventActivateStart,
				Actor: actor,
			})
			continue
		}
		if actor, ok := parseSequenceActivationLine(line, false); ok {
			graph.SequenceEvents = append(graph.SequenceEvents, SequenceEvent{
				Kind:  SequenceEventActivateEnd,
				Actor: actor,
			})
			continue
		}

		if kind, label, ok := parseSequenceControlLine(line); ok {
			switch kind {
			case SequenceEventAltStart:
				blockStack = append(blockStack, blockAlt)
				graph.SequenceEvents = append(graph.SequenceEvents, SequenceEvent{Kind: kind, Label: label})
			case SequenceEventParStart:
				blockStack = append(blockStack, blockPar)
				graph.SequenceEvents = append(graph.SequenceEvents, SequenceEvent{Kind: kind, Label: label})
			case SequenceEventAltElse:
				graph.SequenceEvents = append(graph.SequenceEvents, SequenceEvent{Kind: kind, Label: label})
			case SequenceEventParAnd:
				graph.SequenceEvents = append(graph.SequenceEvents, SequenceEvent{Kind: kind, Label: label})
			case SequenceEventAltEnd, SequenceEventParEnd:
				if len(blockStack) == 0 {
					continue
				}
				last := blockStack[len(blockStack)-1]
				blockStack = blockStack[:len(blockStack)-1]
				if last == blockAlt {
					graph.SequenceEvents = append(graph.SequenceEvents, SequenceEvent{Kind: SequenceEventAltEnd})
				} else if last == blockPar {
					graph.SequenceEvents = append(graph.SequenceEvents, SequenceEvent{Kind: SequenceEventParEnd})
				}
			}
			continue
		}
		graph.GenericLines = append(graph.GenericLines, line)
	}

	for _, participant := range graph.SequenceParticipants {
		label := participant
		if named, ok := graph.SequenceParticipantLabels[participant]; ok && strings.TrimSpace(named) != "" {
			label = named
		}
		graph.ensureNode(participant, label, ShapeRectangle)
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

func parseSequenceParticipant(line string) (id string, label string, ok bool) {
	low := lower(line)
	if !strings.HasPrefix(low, "participant ") && !strings.HasPrefix(low, "actor ") {
		return "", "", false
	}
	rest := line
	if strings.HasPrefix(low, "participant ") {
		rest = strings.TrimSpace(line[len("participant "):])
	} else {
		rest = strings.TrimSpace(line[len("actor "):])
	}
	if rest == "" {
		return "", "", false
	}

	alias := ""
	if idx := strings.Index(lower(rest), " as "); idx >= 0 {
		alias = stripQuotes(strings.TrimSpace(rest[idx+4:]))
		rest = strings.TrimSpace(rest[:idx])
	}

	id, inlineAlias := parseSequenceParticipantIDSpec(rest)
	if id == "" {
		return "", "", false
	}

	label = alias
	if label == "" {
		label = inlineAlias
	}
	if label == "" {
		label = id
	}

	return id, label, true
}

func parseSequenceParticipantIDSpec(spec string) (id string, inlineAlias string) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return "", ""
	}

	if idx := strings.Index(trimmed, "@{"); idx > 0 && strings.HasSuffix(strings.TrimSpace(trimmed), "}") {
		id = stripQuotes(strings.TrimSpace(trimmed[:idx]))
		metaBody := strings.TrimSpace(trimmed[idx+2:])
		if strings.HasSuffix(metaBody, "}") {
			metaBody = strings.TrimSpace(metaBody[:len(metaBody)-1])
			if match := sequenceInlineAliasRe.FindStringSubmatch(metaBody); len(match) == 2 {
				inlineAlias = stripQuotes(strings.TrimSpace(match[1]))
			}
		}
		return id, inlineAlias
	}

	return stripQuotes(trimmed), ""
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
	base := sequenceArrowBase(arrow)
	switch {
	case strings.Contains(base, "."):
		return EdgeDotted
	case strings.Contains(base, "="):
		return EdgeThick
	default:
		return EdgeSolid
	}
}

func parseSequenceActivationLine(line string, activate bool) (string, bool) {
	low := lower(line)
	prefix := "activate "
	if !activate {
		prefix = "deactivate "
	}
	if !strings.HasPrefix(low, prefix) {
		return "", false
	}
	actor := strings.TrimSpace(line[len(prefix):])
	actor = stripQuotes(actor)
	if actor == "" {
		return "", false
	}
	return actor, true
}

func parseSequenceControlLine(line string) (SequenceEventKind, string, bool) {
	trimmed := strings.TrimSpace(line)
	low := lower(trimmed)
	switch {
	case strings.HasPrefix(low, "alt"):
		return SequenceEventAltStart, strings.TrimSpace(stripQuotes(trimmed[len("alt"):])), true
	case strings.HasPrefix(low, "par"):
		return SequenceEventParStart, strings.TrimSpace(stripQuotes(trimmed[len("par"):])), true
	case strings.HasPrefix(low, "else"):
		return SequenceEventAltElse, strings.TrimSpace(stripQuotes(trimmed[len("else"):])), true
	case strings.HasPrefix(low, "and"):
		return SequenceEventParAnd, strings.TrimSpace(stripQuotes(trimmed[len("and"):])), true
	case low == "end":
		return SequenceEventAltEnd, "", true
	default:
		return "", "", false
	}
}
