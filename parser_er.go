package mermaid

import (
	"regexp"
	"strings"
)

var erRelationshipRe = regexp.MustCompile(`^\s*("[^"]+"|\S+)\s+([|}o]{1,2}(?:--|\.\.)[|{o]{1,2})\s+("[^"]+"|\S+)\s*$`)

func parseERDiagram(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramER)
	graph.Source = input
	inEntityBlock := false
	currentEntityID := ""

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		low := lower(line)

		if strings.HasPrefix(low, "erdiagram") {
			continue
		}
		if dir, ok := parseDirectionLine(line); ok {
			graph.Direction = dir
			continue
		}

		if inEntityBlock {
			if line == "}" {
				inEntityBlock = false
				currentEntityID = ""
				continue
			}
			if currentEntityID != "" {
				attr := stripQuotes(strings.TrimSpace(line))
				if attr != "" {
					graph.ERAttributes[currentEntityID] = append(graph.ERAttributes[currentEntityID], attr)
				}
			}
			continue
		}

		if strings.HasSuffix(line, "{") {
			entityRaw := strings.TrimSpace(strings.TrimSuffix(line, "{"))
			id, label := parseEREntity(entityRaw)
			if id != "" {
				graph.ensureNode(id, label, ShapeRectangle)
				currentEntityID = id
			}
			inEntityBlock = true
			continue
		}

		leftRaw, rightRaw, relationLabel, edgeStyle, markerStart, markerEnd, ok := parseERRelationship(line)
		if !ok {
			continue
		}

		leftID, leftLabel := parseEREntity(leftRaw)
		rightID, rightLabel := parseEREntity(rightRaw)
		if leftID == "" || rightID == "" {
			continue
		}

		graph.ensureNode(leftID, leftLabel, ShapeRectangle)
		graph.ensureNode(rightID, rightLabel, ShapeRectangle)
		graph.addEdge(Edge{
			From:        leftID,
			To:          rightID,
			Label:       relationLabel,
			Directed:    false,
			ArrowEnd:    false,
			MarkerStart: markerStart,
			MarkerEnd:   markerEnd,
			Style:       edgeStyle,
		})
	}

	return ParseOutput{Graph: graph}, nil
}

func parseEREntity(raw string) (id string, label string) {
	token := strings.TrimSpace(raw)
	if token == "" {
		return "", ""
	}

	// Supports alias notation: entityName[Alias Name]
	if i := strings.Index(token, "["); i > 0 && strings.HasSuffix(token, "]") {
		id = sanitizeID(strings.TrimSpace(token[:i]), "")
		label = stripQuotes(strings.TrimSpace(token[i+1 : len(token)-1]))
		if id == "" {
			return "", ""
		}
		if label == "" {
			label = id
		}
		return id, label
	}

	id = sanitizeID(stripQuotes(token), "")
	if id == "" {
		return "", ""
	}
	label = stripQuotes(token)
	if label == "" {
		label = id
	}
	return id, label
}

func parseERRelationship(line string) (
	left string,
	right string,
	relationLabel string,
	style EdgeStyle,
	markerStart string,
	markerEnd string,
	ok bool,
) {
	parts := strings.SplitN(line, ":", 2)
	head := strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		relationLabel = stripQuotes(strings.TrimSpace(parts[1]))
	}

	m := erRelationshipRe.FindStringSubmatch(head)
	if len(m) != 4 {
		return "", "", "", "", "", "", false
	}
	left = strings.TrimSpace(m[1])
	relation := strings.TrimSpace(m[2])
	right = strings.TrimSpace(m[3])
	markerStart, markerEnd = parseERCardinalityMarkers(relation)
	style = EdgeSolid
	if strings.Contains(relation, "..") {
		style = EdgeDotted
	}
	return left, right, relationLabel, style, markerStart, markerEnd, true
}

func parseERCardinalityMarkers(relation string) (start string, end string) {
	connector := "--"
	if strings.Contains(relation, "..") {
		connector = ".."
	}
	parts := strings.SplitN(relation, connector, 2)
	if len(parts) != 2 {
		return "", ""
	}
	startKind := erMarkerKind(parts[0])
	endKind := erMarkerKind(parts[1])
	if startKind != "" {
		start = "my-svg_er-" + startKind + "Start"
	}
	if endKind != "" {
		end = "my-svg_er-" + endKind + "End"
	}
	return start, end
}

func erMarkerKind(token string) string {
	card := strings.ReplaceAll(strings.TrimSpace(token), "}", "{")
	switch {
	case strings.Contains(card, "o") && strings.Contains(card, "{"):
		return "zeroOrMore"
	case strings.Contains(card, "|") && strings.Contains(card, "{"):
		return "oneOrMore"
	case strings.Count(card, "|") >= 2:
		return "onlyOne"
	case strings.Contains(card, "o") && strings.Contains(card, "|"):
		return "zeroOrOne"
	default:
		return ""
	}
}
