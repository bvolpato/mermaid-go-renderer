package mermaid

import "strings"

func parseJourney(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}
	graph := newGraph(DiagramJourney)
	graph.Source = input
	graph.Direction = DirectionLeftRight
	currentSection := ""
	lastStepID := ""
	stepSeq := 0

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		low := lower(line)
		if strings.HasPrefix(low, "journey") {
			continue
		}
		if strings.HasPrefix(low, "title") {
			graph.JourneyTitle = stripQuotes(strings.TrimSpace(line[len("title"):]))
			continue
		}
		if strings.HasPrefix(low, "section") {
			currentSection = stripQuotes(strings.TrimSpace(line[len("section"):]))
			lastStepID = ""
			continue
		}
		step, ok := parseJourneyTaskLine(line, currentSection)
		if !ok {
			continue
		}
		stepSeq++
		step.ID = "journey_" + intString(stepSeq)
		graph.JourneySteps = append(graph.JourneySteps, step)
		graph.ensureNode(step.ID, step.Label, ShapeRectangle)
		if lastStepID != "" {
			graph.addEdge(Edge{
				From:     lastStepID,
				To:       step.ID,
				Directed: false,
				ArrowEnd: false,
				Style:    EdgeSolid,
			})
		}
		lastStepID = step.ID
	}

	return ParseOutput{Graph: graph}, nil
}

func parseJourneyTaskLine(line, section string) (JourneyStep, bool) {
	parts := strings.Split(line, ":")
	if len(parts) < 2 {
		return JourneyStep{}, false
	}
	label := stripQuotes(strings.TrimSpace(parts[0]))
	if label == "" {
		return JourneyStep{}, false
	}
	step := JourneyStep{
		Label:   label,
		Section: section,
	}
	score, ok := parseFloat(strings.TrimSpace(parts[1]))
	if ok {
		step.Score = score
		step.HasScore = true
	}
	if len(parts) >= 3 {
		for _, actorRaw := range strings.Split(parts[2], ",") {
			actor := stripQuotes(strings.TrimSpace(actorRaw))
			if actor != "" {
				step.Actors = append(step.Actors, actor)
			}
		}
	}
	return step, true
}
