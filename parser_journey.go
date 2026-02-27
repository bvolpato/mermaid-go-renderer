package mermaid

import "strings"

func parseJourney(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}
	graph := newGraph(DiagramJourney)
	graph.Source = input
	currentSection := ""

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		low := lower(line)
		if i == 0 && strings.HasPrefix(low, "journey") {
			continue
		}
		if strings.HasPrefix(low, "title ") {
			graph.JourneyTitle = stripQuotes(strings.TrimSpace(line[len("title "):]))
			continue
		}
		if strings.HasPrefix(low, "section ") {
			currentSection = stripQuotes(strings.TrimSpace(line[len("section "):]))
			continue
		}
		step, ok := parseJourneyTaskLine(line, currentSection)
		if !ok {
			continue
		}
		graph.JourneySteps = append(graph.JourneySteps, step)
		id := "journey_" + intString(len(graph.JourneySteps))
		graph.ensureNode(id, step.Label, ShapeRoundRect)
		if len(graph.JourneySteps) > 1 {
			graph.addEdge(Edge{
				From:     "journey_" + intString(len(graph.JourneySteps)-1),
				To:       id,
				Directed: true,
				ArrowEnd: true,
				Style:    EdgeSolid,
			})
		}
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
		Score:   0,
		Section: section,
	}
	score, ok := parseFloat(strings.TrimSpace(parts[1]))
	if ok {
		step.Score = score
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
