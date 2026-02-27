package mermaid

import "strings"

func parseGantt(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramGantt)
	graph.Source = input
	currentSection := ""

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		low := lower(line)

		if i == 0 && strings.HasPrefix(low, "gantt") {
			continue
		}
		if strings.HasPrefix(low, "title ") {
			graph.GanttTitle = stripQuotes(strings.TrimSpace(line[len("title "):]))
			continue
		}
		if strings.HasPrefix(low, "section ") {
			currentSection = stripQuotes(strings.TrimSpace(line[len("section "):]))
			if currentSection != "" {
				graph.GanttSections = append(graph.GanttSections, currentSection)
			}
			continue
		}
		if strings.HasPrefix(low, "dateformat ") ||
			strings.HasPrefix(low, "axisformat ") ||
			strings.HasPrefix(low, "excludes ") ||
			strings.HasPrefix(low, "tickinterval ") {
			continue
		}

		task, ok := parseGanttTaskLine(line, currentSection, len(graph.GanttTasks)+1)
		if !ok {
			continue
		}
		graph.GanttTasks = append(graph.GanttTasks, task)
		graph.ensureNode(task.ID, task.Label, ShapeRectangle)
	}

	for i := 1; i < len(graph.GanttTasks); i++ {
		graph.addEdge(Edge{
			From:     graph.GanttTasks[i-1].ID,
			To:       graph.GanttTasks[i].ID,
			Directed: true,
			ArrowEnd: true,
			Style:    EdgeDotted,
		})
	}

	return ParseOutput{Graph: graph}, nil
}

func parseGanttTaskLine(line, section string, seq int) (GanttTask, bool) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return GanttTask{}, false
	}

	label := strings.TrimSpace(parts[0])
	if label == "" {
		return GanttTask{}, false
	}

	metaParts := strings.Split(parts[1], ",")
	for i := range metaParts {
		metaParts[i] = strings.TrimSpace(metaParts[i])
	}

	task := GanttTask{
		ID:      "gantt_" + intString(seq),
		Label:   stripQuotes(label),
		Section: section,
	}

	if len(metaParts) > 0 {
		task.Status = parseGanttStatus(metaParts[0])
	}
	if len(metaParts) > 1 {
		task.Start = metaParts[len(metaParts)-2]
		task.Duration = metaParts[len(metaParts)-1]
	}
	if len(metaParts) >= 3 && !looksLikeDateOrDuration(metaParts[0]) {
		task.ID = sanitizeID(metaParts[0], task.ID)
	}
	return task, true
}

func parseGanttStatus(token string) string {
	switch lower(strings.Trim(token, " ")) {
	case "done":
		return "done"
	case "active":
		return "active"
	case "crit":
		return "crit"
	case "milestone":
		return "milestone"
	default:
		return ""
	}
}
