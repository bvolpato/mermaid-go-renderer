package mermaid

import "strings"

func parseTimeline(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramTimeline)
	graph.Source = input
	currentSection := ""

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		low := lower(line)

		if i == 0 && strings.HasPrefix(low, "timeline") {
			continue
		}
		if strings.HasPrefix(low, "title ") {
			graph.TimelineTitle = stripQuotes(strings.TrimSpace(line[len("title "):]))
			continue
		}
		if strings.HasPrefix(low, "section ") {
			currentSection = stripQuotes(strings.TrimSpace(line[len("section "):]))
			if currentSection != "" {
				graph.TimelineSections = append(graph.TimelineSections, currentSection)
			}
			continue
		}

		event, ok := parseTimelineEventLine(line, currentSection)
		if !ok {
			continue
		}
		graph.TimelineEvents = append(graph.TimelineEvents, event)
		id := "timeline_" + intString(len(graph.TimelineEvents))
		graph.ensureNode(id, event.Time, ShapeRoundRect)
	}

	for i := 1; i < len(graph.TimelineEvents); i++ {
		graph.addEdge(Edge{
			From:     "timeline_" + intString(i),
			To:       "timeline_" + intString(i+1),
			Directed: true,
			ArrowEnd: true,
			Style:    EdgeSolid,
		})
	}

	return ParseOutput{Graph: graph}, nil
}

func parseTimelineEventLine(line, section string) (TimelineEvent, bool) {
	parts := strings.Split(line, ":")
	if len(parts) < 2 {
		return TimelineEvent{}, false
	}
	timeLabel := stripQuotes(strings.TrimSpace(parts[0]))
	if timeLabel == "" {
		return TimelineEvent{}, false
	}
	events := make([]string, 0, len(parts)-1)
	for _, part := range parts[1:] {
		event := stripQuotes(strings.TrimSpace(part))
		if event != "" {
			events = append(events, event)
		}
	}
	if len(events) == 0 {
		return TimelineEvent{}, false
	}
	return TimelineEvent{
		Time:    timeLabel,
		Events:  events,
		Section: section,
	}, true
}
