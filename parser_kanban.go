package mermaid

import (
	"regexp"
	"strings"
)

var kanbanCardRe = regexp.MustCompile(`^([A-Za-z0-9_-]+)\[(.+?)\](?:@\{(.+)\})?$`)

func parseKanban(input string) (ParseOutput, error) {
	lines, err := preprocessInputKeepIndent(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramKanban)
	graph.Source = input
	currentColumn := -1

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(lower(line), "kanban") {
			continue
		}

		depth := kanbanIndentDepth(raw)
		if depth <= 1 {
			title := stripQuotes(line)
			if title == "" {
				continue
			}
			graph.KanbanBoard = append(graph.KanbanBoard, KanbanColumn{Title: title})
			currentColumn = len(graph.KanbanBoard) - 1
			continue
		}

		if currentColumn < 0 {
			continue
		}
		card, ok := parseKanbanCard(line)
		if !ok {
			continue
		}
		graph.KanbanBoard[currentColumn].Cards = append(graph.KanbanBoard[currentColumn].Cards, card)
	}

	if len(graph.KanbanBoard) == 0 {
		return parseClassLike(input, DiagramKanban)
	}
	return ParseOutput{Graph: graph}, nil
}

func parseKanbanCard(line string) (KanbanCard, bool) {
	m := kanbanCardRe.FindStringSubmatch(strings.TrimSpace(line))
	if len(m) < 3 {
		return KanbanCard{}, false
	}
	id := sanitizeID(strings.TrimSpace(m[1]), "")
	title := stripQuotes(strings.TrimSpace(m[2]))
	if id == "" || title == "" {
		return KanbanCard{}, false
	}
	card := KanbanCard{
		ID:    id,
		Title: title,
	}
	if len(m) < 4 || strings.TrimSpace(m[3]) == "" {
		return card, true
	}

	for _, token := range splitKanbanMetaTokens(m[3]) {
		pair := strings.SplitN(token, ":", 2)
		if len(pair) != 2 {
			continue
		}
		key := lower(strings.TrimSpace(pair[0]))
		value := stripQuotes(strings.TrimSpace(pair[1]))
		switch key {
		case "ticket":
			card.Ticket = value
		case "assigned":
			card.Assigned = value
		case "priority":
			card.Priority = value
		}
	}
	return card, true
}

func splitKanbanMetaTokens(raw string) []string {
	parts := make([]string, 0, 6)
	var cur strings.Builder
	inSingle := false
	inDouble := false
	for _, ch := range raw {
		switch ch {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
			cur.WriteRune(ch)
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
			cur.WriteRune(ch)
		case ',':
			if inSingle || inDouble {
				cur.WriteRune(ch)
				continue
			}
			token := strings.TrimSpace(cur.String())
			if token != "" {
				parts = append(parts, token)
			}
			cur.Reset()
		default:
			cur.WriteRune(ch)
		}
	}
	token := strings.TrimSpace(cur.String())
	if token != "" {
		parts = append(parts, token)
	}
	return parts
}

func kanbanIndentDepth(raw string) int {
	indent := 0
	for _, ch := range raw {
		if ch == ' ' {
			indent++
			continue
		}
		if ch == '\t' {
			indent += 2
			continue
		}
		break
	}
	return indent / 2
}
