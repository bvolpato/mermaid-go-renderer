package mermaid

import "strings"

func parseC4(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}
	graph := newGraph(DiagramC4)
	graph.Source = input
	graph.Direction = DirectionLeftRight

	for idx, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if idx == 0 && strings.HasPrefix(lower(line), "c4") {
			continue
		}
		if strings.HasPrefix(lower(line), "title ") {
			graph.C4Title = stripQuotes(strings.TrimSpace(line[len("title "):]))
			continue
		}
		if dir, ok := parseDirectionLine(line); ok {
			graph.Direction = dir
			continue
		}

		name, args, ok := parseC4Call(line)
		if !ok {
			continue
		}
		switch lower(name) {
		case "person", "person_ext", "system", "system_ext", "systemdb", "systemdb_ext", "container", "container_ext":
			if len(args) == 0 {
				continue
			}
			id := sanitizeID(args[0], args[0])
			if id == "" {
				continue
			}
			label := id
			if len(args) >= 2 && strings.TrimSpace(args[1]) != "" {
				label = strings.TrimSpace(args[1])
			}
			description := ""
			if len(args) >= 3 {
				description = strings.TrimSpace(args[2])
			}
			stereotype := c4Stereotype(name)
			nodeLabel := label
			if stereotype != "" {
				nodeLabel = stereotype + "\n" + nodeLabel
			}
			if description != "" {
				nodeLabel += "\n" + description
			}
			shape := ShapeRoundRect
			if strings.HasPrefix(lower(name), "person") {
				shape = ShapePerson
			}
			graph.ensureNode(id, nodeLabel, shape)
			node := graph.Nodes[id]
			node.Fill = "#1168BD"
			node.Stroke = "#3C7FC0"
			node.StrokeWidth = 0.5
			lowerName := lower(name)
			if strings.HasPrefix(lowerName, "person") && !strings.Contains(lowerName, "_ext") {
				node.Fill = "#08427B"
				node.Stroke = "#073B6F"
			}
			if strings.Contains(lowerName, "_ext") {
				node.Fill = "#999999"
				node.Stroke = "#8A8A8A"
			}
			graph.Nodes[id] = node
		case "rel", "rel_u", "rel_d", "rel_l", "rel_r":
			if len(args) < 2 {
				continue
			}
			fromID := sanitizeID(args[0], "")
			toID := sanitizeID(args[1], "")
			if fromID == "" || toID == "" {
				continue
			}
			label := ""
			if len(args) >= 3 {
				label = strings.TrimSpace(args[2])
			}
			if len(args) >= 4 {
				tech := strings.TrimSpace(args[3])
				if tech != "" {
					if label != "" {
						label += "\n[" + tech + "]"
					} else {
						label = "[" + tech + "]"
					}
				}
			}
			graph.ensureNode(fromID, fromID, ShapeRectangle)
			graph.ensureNode(toID, toID, ShapeRectangle)
			graph.addEdge(Edge{
				From:     fromID,
				To:       toID,
				Label:    label,
				Directed: true,
				ArrowEnd: true,
				Style:    EdgeSolid,
			})
		}
	}

	return ParseOutput{Graph: graph}, nil
}

func parseC4Call(line string) (name string, args []string, ok bool) {
	open := strings.Index(line, "(")
	close := strings.LastIndex(line, ")")
	if open <= 0 || close <= open {
		return "", nil, false
	}
	name = strings.TrimSpace(line[:open])
	if name == "" {
		return "", nil, false
	}
	rawArgs := strings.TrimSpace(line[open+1 : close])
	if rawArgs == "" {
		return name, nil, true
	}
	args = splitCSVRespectingQuotes(rawArgs)
	for i := range args {
		args[i] = strings.TrimSpace(stripQuotes(args[i]))
	}
	return name, args, true
}

func splitCSVRespectingQuotes(input string) []string {
	parts := make([]string, 0, 4)
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false
	for _, r := range input {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			current.WriteRune(r)
			continue
		}
		if r == '\'' && !inDouble {
			inSingle = !inSingle
			current.WriteRune(r)
			continue
		}
		if r == '"' && !inSingle {
			inDouble = !inDouble
			current.WriteRune(r)
			continue
		}
		if r == ',' && !inSingle && !inDouble {
			parts = append(parts, strings.TrimSpace(current.String()))
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	parts = append(parts, strings.TrimSpace(current.String()))
	return parts
}

func c4Stereotype(name string) string {
	switch lower(strings.TrimSpace(name)) {
	case "person":
		return "<<person>>"
	case "person_ext":
		return "<<external_person>>"
	case "system":
		return "<<system>>"
	case "system_ext":
		return "<<external_system>>"
	case "systemdb":
		return "<<system_db>>"
	case "systemdb_ext":
		return "<<external_system_db>>"
	case "container":
		return "<<container>>"
	case "container_ext":
		return "<<external_container>>"
	default:
		return ""
	}
}
