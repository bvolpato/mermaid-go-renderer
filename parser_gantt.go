package mermaid

import "strings"

func parseGantt(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramGantt)
	graph.Source = input
	graph.Direction = DirectionLeftRight
	currentSection := ""
	lastTaskID := ""
	taskSeq := 0

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		low := lower(line)
		if strings.HasPrefix(low, "gantt") {
			continue
		}
		if strings.HasPrefix(low, "title") {
			graph.GanttTitle = stripQuotes(strings.TrimSpace(line[len("title"):]))
			continue
		}
		if strings.HasPrefix(low, "section") {
			currentSection = stripQuotes(strings.TrimSpace(line[len("section"):]))
			if currentSection != "" {
				graph.GanttSections = append(graph.GanttSections, currentSection)
			}
			lastTaskID = ""
			continue
		}
		if strings.HasPrefix(low, "dateformat ") ||
			strings.HasPrefix(low, "axisformat ") ||
			strings.HasPrefix(low, "todaymarker ") ||
			strings.HasPrefix(low, "includes ") ||
			strings.HasPrefix(low, "excludes ") ||
			strings.HasPrefix(low, "tickinterval ") {
			continue
		}

		taskLabel, meta, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		taskLabel = strings.TrimSpace(taskLabel)
		if taskLabel == "" {
			continue
		}
		id, details, after, status := parseGanttTaskMeta(meta)
		taskSeq++
		task := GanttTask{
			ID:      id,
			Label:   stripQuotes(taskLabel),
			Section: currentSection,
			Status:  status,
			After:   after,
		}
		if task.ID == "" {
			task.ID = "gantt_" + intString(taskSeq)
		}
		task.Start, task.Duration = extractGanttTiming(details)
		graph.GanttTasks = append(graph.GanttTasks, task)
		graph.ensureNode(task.ID, task.Label, ShapeRectangle)
		if task.After != "" {
			graph.addEdge(Edge{
				From:     task.After,
				To:       task.ID,
				Directed: true,
				ArrowEnd: true,
				Style:    EdgeSolid,
			})
		} else if lastTaskID != "" {
			graph.addEdge(Edge{
				From:     lastTaskID,
				To:       task.ID,
				Directed: false,
				ArrowEnd: false,
				Style:    EdgeSolid,
			})
		}
		lastTaskID = task.ID
	}

	return ParseOutput{Graph: graph}, nil
}

func parseGanttTaskMeta(meta string) (id string, details []string, after string, status string) {
	for _, rawToken := range strings.Split(meta, ",") {
		token := strings.TrimSpace(rawToken)
		if token == "" {
			continue
		}
		low := lower(token)
		if strings.HasPrefix(low, "after ") {
			after = strings.TrimSpace(token[len("after "):])
			continue
		}
		if parsed := parseGanttStatus(low); parsed != "" {
			status = parsed
			details = append(details, token)
			continue
		}
		if looksLikeDate(token) || looksLikeDuration(token) {
			details = append(details, token)
			continue
		}
		if id == "" {
			id = sanitizeID(token, "")
		} else {
			details = append(details, token)
		}
	}
	return id, details, after, status
}

func extractGanttTiming(details []string) (start string, duration string) {
	for _, detail := range details {
		if start == "" && looksLikeDate(detail) {
			start = detail
		} else if duration == "" && looksLikeDuration(detail) {
			duration = detail
		}
	}
	return start, duration
}

func looksLikeDate(token string) bool {
	t := strings.TrimSpace(token)
	return strings.Contains(t, "-") || strings.Contains(t, "/") || strings.Contains(t, ".")
}

func looksLikeDuration(token string) bool {
	t := lower(strings.TrimSpace(token))
	if t == "" {
		return false
	}
	last := t[len(t)-1]
	switch last {
	case 'd', 'h', 'w', 'm', 'y':
		return true
	default:
		return false
	}
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
