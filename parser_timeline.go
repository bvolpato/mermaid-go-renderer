package mermaid

import "strings"

func parseTimeline(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramTimeline)
	graph.Source = input
	graph.Direction = DirectionLeftRight
	currentSection := ""
	var pendingTime string
	var pendingEvents []string

	flushPending := func() {
		if strings.TrimSpace(pendingTime) == "" {
			pendingEvents = nil
			return
		}
		graph.TimelineEvents = append(graph.TimelineEvents, TimelineEvent{
			Time:    pendingTime,
			Events:  append([]string(nil), pendingEvents...),
			Section: currentSection,
		})
		pendingTime = ""
		pendingEvents = nil
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		low := lower(line)
		if strings.HasPrefix(low, "timeline") {
			continue
		}
		if strings.HasPrefix(low, "title") {
			graph.TimelineTitle = stripQuotes(strings.TrimSpace(line[len("title"):]))
			continue
		}
		if strings.HasPrefix(low, "section") {
			flushPending()
			currentSection = stripQuotes(strings.TrimSpace(line[len("section"):]))
			if currentSection != "" {
				graph.TimelineSections = append(graph.TimelineSections, currentSection)
			}
			continue
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		timePart := strings.TrimSpace(line[:colonIdx])
		eventsPart := strings.TrimSpace(line[colonIdx+1:])
		if timePart == "" {
			continue
		}
		flushPending()
		pendingTime = stripQuotes(timePart)
		for _, event := range strings.Split(eventsPart, ":") {
			ev := stripQuotes(strings.TrimSpace(event))
			if ev != "" {
				pendingEvents = append(pendingEvents, ev)
			}
		}
	}
	flushPending()

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
