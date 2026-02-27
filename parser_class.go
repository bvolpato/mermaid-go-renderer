package mermaid

import "strings"

func parseClassDiagram(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramClass)
	graph.Source = input
	currentClass := ""

	for idx, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if idx == 0 && isHeaderLineForKind(line, DiagramClass) {
			continue
		}
		if line == "" {
			continue
		}
		if dir, ok := parseDirectionLine(line); ok {
			graph.Direction = dir
			continue
		}
		if currentClass != "" {
			if line == "}" {
				currentClass = ""
				continue
			}
			appendClassMemberLine(&graph, currentClass, line)
			continue
		}

		low := lower(line)
		if strings.HasPrefix(low, "class ") {
			classID, classLabel, inBlock := parseClassDeclarationLine(line)
			if classID != "" {
				graph.ensureNode(classID, classLabel, ShapeRectangle)
				if inBlock {
					currentClass = classID
				}
			}
			continue
		}

		if classID, member, ok := parseClassMemberAssignmentLine(line); ok {
			graph.ensureNode(classID, classID, ShapeRectangle)
			appendClassMemberLine(&graph, classID, member)
			continue
		}

		if shouldSkipClassLikeLine(DiagramClass, line) {
			continue
		}

		if statements := splitEdgeChain(line); len(statements) > 0 {
			addedAny := false
			for _, stmt := range statements {
				if addEdgeFromLine(&graph, stmt) {
					addedAny = true
				}
			}
			if addedAny {
				continue
			}
		}

		if addEdgeFromLine(&graph, line) {
			continue
		}

		if id, label, shape, ok := parseNodeOnly(line); ok {
			graph.ensureNode(id, label, shape)
		}
	}

	return ParseOutput{Graph: graph}, nil
}

func parseClassDeclarationLine(line string) (id string, label string, inBlock bool) {
	raw := strings.TrimSpace(line[len("class "):])
	if raw == "" {
		return "", "", false
	}
	inBlock = strings.HasSuffix(raw, "{")
	if inBlock {
		raw = strings.TrimSpace(strings.TrimSuffix(raw, "{"))
	}
	if raw == "" {
		return "", "", inBlock
	}

	id, label, _, _ = parseNodeToken(raw)
	if id == "" {
		id = sanitizeID(stripQuotes(raw), "")
		label = stripQuotes(raw)
	}
	if id == "" {
		return "", "", inBlock
	}
	if label == "" {
		label = id
	}
	return id, label, inBlock
}

func parseClassMemberAssignmentLine(line string) (classID string, member string, ok bool) {
	if arrowTokenRe.MatchString(line) {
		return "", "", false
	}
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return "", "", false
	}
	classToken := strings.TrimSpace(line[:idx])
	member = strings.TrimSpace(line[idx+1:])
	if classToken == "" || member == "" {
		return "", "", false
	}
	classID = sanitizeID(stripQuotes(classToken), "")
	if classID == "" {
		return "", "", false
	}
	return classID, member, true
}

func appendClassMemberLine(graph *Graph, classID string, line string) {
	member := stripQuotes(strings.TrimSpace(line))
	if member == "" || member == "{" || member == "}" {
		return
	}
	if strings.HasPrefix(member, "<<") && strings.HasSuffix(member, ">>") {
		return
	}
	if strings.Contains(member, "()") {
		graph.ClassMethods[classID] = append(graph.ClassMethods[classID], member)
		return
	}
	graph.ClassMembers[classID] = append(graph.ClassMembers[classID], member)
}
