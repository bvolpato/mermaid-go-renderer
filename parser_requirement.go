package mermaid

import "strings"

func parseRequirement(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}
	graph := newGraph(DiagramRequirement)
	graph.Source = input

	type requirementBlock struct {
		kind string
		id   string
		attr map[string]string
	}

	var current *requirementBlock
	flushBlock := func(block *requirementBlock) {
		if block == nil || block.id == "" {
			return
		}
		label := buildRequirementNodeLabel(block.kind, block.id, block.attr)
		graph.ensureNode(block.id, label, ShapeRectangle)
	}

	for idx, raw := range lines {
		line := strings.TrimSpace(raw)
		if idx == 0 && strings.HasPrefix(lower(line), "requirementdiagram") {
			continue
		}
		if line == "" {
			continue
		}

		if current != nil {
			if line == "}" {
				flushBlock(current)
				current = nil
				continue
			}
			if sep := strings.Index(line, ":"); sep > 0 {
				key := lower(strings.TrimSpace(line[:sep]))
				value := strings.TrimSpace(stripQuotes(line[sep+1:]))
				if key != "" && value != "" {
					current.attr[key] = value
				}
			}
			continue
		}

		if strings.HasSuffix(line, "{") {
			head := strings.TrimSpace(strings.TrimSuffix(line, "{"))
			parts := strings.Fields(head)
			if len(parts) >= 2 {
				id := sanitizeID(parts[1], parts[1])
				if id != "" {
					current = &requirementBlock{
						kind: lower(parts[0]),
						id:   id,
						attr: map[string]string{},
					}
				}
			}
			continue
		}

		if addRequirementRelationLine(&graph, line) {
			continue
		}
		if addEdgeFromLine(&graph, line) {
			continue
		}
	}

	flushBlock(current)
	return ParseOutput{Graph: graph}, nil
}

func addRequirementRelationLine(graph *Graph, line string) bool {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return false
	}
	if fields[1] != "-" || fields[len(fields)-2] != "->" {
		return false
	}
	fromID := sanitizeID(fields[0], "")
	toID := sanitizeID(fields[len(fields)-1], "")
	if fromID == "" || toID == "" {
		return false
	}
	rel := strings.TrimSpace(strings.Join(fields[2:len(fields)-2], " "))
	if rel == "" {
		return false
	}
	rel = "<<" + lower(rel) + ">>"
	graph.ensureNode(fromID, fromID, ShapeRectangle)
	graph.ensureNode(toID, toID, ShapeRectangle)
	graph.addEdge(Edge{
		From:        fromID,
		To:          toID,
		Label:       rel,
		Directed:    true,
		ArrowEnd:    true,
		Style:       EdgeDotted,
		MarkerEnd:   "",
		MarkerStart: "",
	})
	return true
}

func buildRequirementNodeLabel(kind, id string, attrs map[string]string) string {
	lines := []string{requirementStereotype(kind), id}
	appendIf := func(key, out string) {
		if value := strings.TrimSpace(attrs[key]); value != "" {
			if key == "risk" || key == "verifymethod" {
				value = toTitleWords(value)
			}
			lines = append(lines, out+": "+value)
		}
	}
	appendIf("id", "ID")
	appendIf("text", "Text")
	appendIf("type", "Type")
	appendIf("risk", "Risk")
	appendIf("verifymethod", "Verification")
	return strings.Join(lines, "\n")
}

func toTitleWords(value string) string {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) == 0 {
		return strings.TrimSpace(value)
	}
	for idx, part := range parts {
		runes := []rune(strings.ToLower(part))
		if len(runes) == 0 {
			continue
		}
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		parts[idx] = string(runes)
	}
	return strings.Join(parts, " ")
}

func requirementStereotype(kind string) string {
	switch lower(strings.TrimSpace(kind)) {
	case "requirement":
		return "<<Requirement>>"
	case "functionalrequirement":
		return "<<Functional Requirement>>"
	case "interfacerequirement":
		return "<<Interface Requirement>>"
	case "performancerequirement":
		return "<<Performance Requirement>>"
	case "physicalrequirement":
		return "<<Physical Requirement>>"
	case "designconstraint":
		return "<<Design Constraint>>"
	case "element":
		return "<<Element>>"
	default:
		if kind == "" {
			return "<<Requirement>>"
		}
		return "<<" + strings.Title(kind) + ">>"
	}
}
