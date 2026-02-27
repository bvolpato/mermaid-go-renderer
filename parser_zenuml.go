package mermaid

import "strings"

func parseZenUML(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramZenUML)
	graph.Source = input
	participantSet := map[string]struct{}{}
	mainCounter := 0
	inAlt := false
	altBase := 0
	altCounter := 0
	nextReturn := false
	currentAlt := ZenUMLAltBlock{ElseStart: -1}

	addParticipant := func(id string, label string) {
		id = sanitizeID(id, "")
		if id == "" {
			return
		}
		if _, ok := participantSet[id]; !ok {
			participantSet[id] = struct{}{}
			graph.SequenceParticipants = append(graph.SequenceParticipants, id)
		}
		if label == "" {
			label = id
		}
		if existing, ok := graph.SequenceParticipantLabels[id]; ok && strings.TrimSpace(existing) != "" {
			if label == id || label == "" {
				return
			}
		}
		graph.SequenceParticipantLabels[id] = label
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		low := lower(line)
		if strings.HasPrefix(low, "zenuml") {
			continue
		}
		if strings.HasPrefix(low, "title ") {
			graph.ZenUMLTitle = stripQuotes(strings.TrimSpace(line[len("title "):]))
			continue
		}

		if condition, ok := parseZenUMLIfCondition(line, low); ok {
			// Mermaid assigns the parent branch index ("4") and nested statements as "4.1", "4.2", etc.
			mainCounter++
			altBase = mainCounter
			altCounter = 0
			inAlt = true
			currentAlt = ZenUMLAltBlock{
				Condition: condition,
				Start:     len(graph.SequenceMessages),
				ElseStart: -1,
				End:       len(graph.SequenceMessages) - 1,
			}
			continue
		}
		if isZenUMLElseLine(low) {
			if inAlt && currentAlt.ElseStart < 0 {
				currentAlt.ElseStart = len(graph.SequenceMessages)
			}
			continue
		}
		if isZenUMLBlockEnd(line, low) {
			if inAlt {
				currentAlt.End = len(graph.SequenceMessages) - 1
				if currentAlt.End >= currentAlt.Start {
					graph.ZenUMLAltBlocks = append(graph.ZenUMLAltBlocks, currentAlt)
				}
			}
			inAlt = false
			currentAlt = ZenUMLAltBlock{ElseStart: -1}
			continue
		}
		if isZenUMLReturnLine(low) {
			nextReturn = true
			continue
		}
		if shouldSkipZenUMLControlLine(low) {
			continue
		}

		if id, label, ok := parseZenUMLParticipantDeclaration(line); ok {
			addParticipant(id, label)
			continue
		}

		if msg, ok := parseSequenceMessage(line); ok {
			if inAlt {
				altCounter++
				msg.Index = intString(altBase) + "." + intString(altCounter)
				currentAlt.End = len(graph.SequenceMessages)
			} else {
				mainCounter++
				msg.Index = intString(mainCounter)
			}
			if nextReturn {
				msg.IsReturn = true
				nextReturn = false
			}
			graph.SequenceMessages = append(graph.SequenceMessages, msg)
			addParticipant(msg.From, msg.From)
			addParticipant(msg.To, msg.To)
			continue
		}
	}
	if inAlt {
		currentAlt.End = len(graph.SequenceMessages) - 1
		if currentAlt.End >= currentAlt.Start {
			graph.ZenUMLAltBlocks = append(graph.ZenUMLAltBlocks, currentAlt)
		}
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

func shouldSkipZenUMLControlLine(low string) bool {
	if strings.HasPrefix(low, "//") || low == "{" || low == "}" {
		return true
	}
	switch {
	case strings.HasPrefix(low, "while("),
		strings.HasPrefix(low, "for("),
		strings.HasPrefix(low, "foreach("),
		strings.HasPrefix(low, "loop"),
		strings.HasPrefix(low, "par"),
		strings.HasPrefix(low, "opt"),
		strings.HasPrefix(low, "try"),
		strings.HasPrefix(low, "catch"),
		strings.HasPrefix(low, "finally"):
		return true
	default:
		return false
	}
}

func parseZenUMLIfCondition(line string, low string) (string, bool) {
	if !strings.HasPrefix(low, "if(") {
		return "", false
	}
	open := strings.Index(line, "(")
	close := strings.LastIndex(line, ")")
	if open < 0 || close <= open {
		return "", true
	}
	condition := strings.TrimSpace(line[open+1 : close])
	condition = stripQuotes(condition)
	return condition, true
}

func isZenUMLElseLine(low string) bool {
	return strings.HasPrefix(low, "else") || strings.Contains(low, "} else")
}

func isZenUMLReturnLine(low string) bool {
	return strings.HasPrefix(low, "@return") || strings.HasPrefix(low, "return ")
}

func isZenUMLBlockEnd(line string, low string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.Contains(low, "} else") {
		return false
	}
	return trimmed == "}" || trimmed == "};"
}

func parseZenUMLParticipantDeclaration(line string) (id, label string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.Contains(trimmed, "->") {
		return "", "", false
	}

	// Annotated participant declaration, e.g. "@Actor Alice"
	if strings.HasPrefix(trimmed, "@") {
		fields := strings.Fields(trimmed)
		if len(fields) >= 2 {
			id = stripQuotes(fields[1])
			label = id
			return id, label, id != ""
		}
		return "", "", false
	}

	low := lower(trimmed)
	if idx := strings.Index(low, " as "); idx > 0 {
		id = stripQuotes(strings.TrimSpace(trimmed[:idx]))
		label = stripQuotes(strings.TrimSpace(trimmed[idx+4:]))
		if label == "" {
			label = id
		}
		return id, label, id != ""
	}

	// Bare participant line ("Bob")
	if !strings.Contains(trimmed, " ") {
		id = stripQuotes(trimmed)
		return id, id, id != ""
	}
	return "", "", false
}
