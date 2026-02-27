package mermaid

import (
	"regexp"
	"strings"
)

var architectureGroupRe = regexp.MustCompile(`^group\s+([A-Za-z0-9_]+)\s*(?:\(([^)]+)\))?\s*\[([^\]]+)\]\s*$`)
var architectureServiceRe = regexp.MustCompile(`^service\s+([A-Za-z0-9_]+)\s*(?:\(([^)]+)\))?\s*\[([^\]]+)\](?:\s+in\s+([A-Za-z0-9_]+))?\s*$`)

func parseArchitecture(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramArchitecture)
	graph.Source = input

	groupIndex := map[string]int{}
	serviceSeen := map[string]struct{}{}

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if i == 0 && strings.HasPrefix(lower(line), "architecture") {
			continue
		}

		if group, ok := parseArchitectureGroup(line); ok {
			if _, exists := groupIndex[group.ID]; !exists {
				groupIndex[group.ID] = len(graph.ArchitectureGroups)
				graph.ArchitectureGroups = append(graph.ArchitectureGroups, group)
			} else {
				graph.ArchitectureGroups[groupIndex[group.ID]] = group
			}
			continue
		}

		if service, ok := parseArchitectureService(line); ok {
			graph.ArchitectureServices = append(graph.ArchitectureServices, service)
			if _, exists := groupIndex[service.GroupID]; !exists && service.GroupID != "" {
				groupIndex[service.GroupID] = len(graph.ArchitectureGroups)
				graph.ArchitectureGroups = append(graph.ArchitectureGroups, ArchitectureGroup{
					ID:    service.GroupID,
					Label: service.GroupID,
					Icon:  "cloud",
				})
			}
			if _, okService := serviceSeen[service.ID]; !okService {
				graph.ensureNode(service.ID, service.Label, ShapeRectangle)
				serviceSeen[service.ID] = struct{}{}
			}
			continue
		}

		if link, ok := parseArchitectureLink(line); ok {
			graph.ArchitectureLinks = append(graph.ArchitectureLinks, link)
			if link.From.ID != "" && link.To.ID != "" {
				graph.addEdge(Edge{
					From:     link.From.ID,
					To:       link.To.ID,
					Directed: false,
					Style:    EdgeSolid,
				})
			}
			continue
		}

		graph.GenericLines = append(graph.GenericLines, line)
	}

	// Keep behavior deterministic for downstream layout logic.
	if len(graph.ArchitectureServices) == 0 {
		return parseClassLike(input, DiagramArchitecture)
	}

	return ParseOutput{Graph: graph}, nil
}

func parseArchitectureGroup(line string) (ArchitectureGroup, bool) {
	m := architectureGroupRe.FindStringSubmatch(strings.TrimSpace(line))
	if len(m) != 4 {
		return ArchitectureGroup{}, false
	}
	id := sanitizeID(stripQuotes(strings.TrimSpace(m[1])), "")
	if id == "" {
		return ArchitectureGroup{}, false
	}
	icon := strings.TrimSpace(m[2])
	if icon == "" {
		icon = "cloud"
	}
	label := stripQuotes(strings.TrimSpace(m[3]))
	if label == "" {
		label = id
	}
	return ArchitectureGroup{
		ID:    id,
		Label: label,
		Icon:  icon,
	}, true
}

func parseArchitectureService(line string) (ArchitectureService, bool) {
	m := architectureServiceRe.FindStringSubmatch(strings.TrimSpace(line))
	if len(m) != 5 {
		return ArchitectureService{}, false
	}
	id := sanitizeID(stripQuotes(strings.TrimSpace(m[1])), "")
	if id == "" {
		return ArchitectureService{}, false
	}
	icon := strings.TrimSpace(m[2])
	if icon == "" {
		icon = "server"
	}
	label := stripQuotes(strings.TrimSpace(m[3]))
	if label == "" {
		label = id
	}
	groupID := sanitizeID(stripQuotes(strings.TrimSpace(m[4])), "")
	return ArchitectureService{
		ID:      id,
		Label:   label,
		Icon:    icon,
		GroupID: groupID,
	}, true
}

func parseArchitectureLink(line string) (ArchitectureLink, bool) {
	parts := strings.Split(line, "--")
	if len(parts) != 2 {
		return ArchitectureLink{}, false
	}
	from, okFrom := parseArchitectureEndpoint(parts[0])
	to, okTo := parseArchitectureEndpoint(parts[1])
	if !okFrom || !okTo || from.ID == "" || to.ID == "" {
		return ArchitectureLink{}, false
	}
	if from.Side == "" {
		from.Side = "R"
	}
	if to.Side == "" {
		to.Side = "L"
	}
	return ArchitectureLink{From: from, To: to}, true
}

func parseArchitectureEndpoint(raw string) (ArchitectureEndpoint, bool) {
	token := strings.TrimSpace(raw)
	token = stripQuotes(token)
	if token == "" {
		return ArchitectureEndpoint{}, false
	}
	id := token
	side := ""
	if strings.Count(token, ":") == 1 {
		parts := strings.SplitN(token, ":", 2)
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		switch {
		case isArchitectureSide(left):
			side = upper(left)
			id = right
		case isArchitectureSide(right):
			side = upper(right)
			id = left
		}
	}
	id = sanitizeID(id, "")
	if id == "" {
		return ArchitectureEndpoint{}, false
	}
	return ArchitectureEndpoint{ID: id, Side: side}, true
}

func isArchitectureSide(token string) bool {
	switch upper(strings.TrimSpace(token)) {
	case "L", "R", "T", "B":
		return true
	default:
		return false
	}
}

