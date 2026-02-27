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
		if addClassRelationLine(&graph, line) {
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

var classRelationTokens = []string{
	"<|==", "==|>",
	"<|--", "--|>",
	"<|..", "..|>",
	"()==", "==()",
	"()--", "--()",
	"()..", "..()",
	"*==", "==*",
	"*--", "--*",
	"*..", "..*",
	"o==", "==o",
	"o--", "--o",
	"o..", "..o",
	"<==", "==>",
	"<--", "-->",
	"<..", "..>",
	"==", "--", "..",
}

func addClassRelationLine(graph *Graph, line string) bool {
	body, label := splitClassRelationBodyAndLabel(line)
	start, end, token, ok := findClassRelationToken(body)
	if !ok {
		return false
	}
	leftRaw := strings.TrimSpace(body[:start])
	rightRaw := strings.TrimSpace(body[end:])
	if leftRaw == "" || rightRaw == "" {
		return false
	}

	fromID, fromLabel, fromShape, _ := parseNodeToken(leftRaw)
	toID, toLabel, toShape, _ := parseNodeToken(rightRaw)
	if fromID == "" || toID == "" {
		return false
	}

	graph.ensureNode(fromID, fromLabel, fromShape)
	graph.ensureNode(toID, toLabel, toShape)

	markerStart, markerEnd := classRelationMarkers(token)
	edge := Edge{
		From:        fromID,
		To:          toID,
		Label:       label,
		Style:       EdgeSolid,
		ArrowStart:  strings.HasPrefix(token, "<"),
		ArrowEnd:    strings.HasSuffix(token, ">"),
		MarkerStart: markerStart,
		MarkerEnd:   markerEnd,
	}
	if strings.Contains(token, "..") {
		edge.Style = EdgeDotted
	} else if strings.Contains(token, "==") {
		edge.Style = EdgeThick
	}
	if edge.MarkerStart == "" && edge.ArrowStart {
		edge.MarkerStart = "my-svg_class-dependencyStart"
	}
	if edge.MarkerEnd == "" && edge.ArrowEnd {
		edge.MarkerEnd = "my-svg_class-dependencyEnd"
	}
	edge.Directed = edge.ArrowStart || edge.ArrowEnd || edge.MarkerStart != "" || edge.MarkerEnd != ""

	graph.addEdge(edge)
	return true
}

func splitClassRelationBodyAndLabel(line string) (body string, label string) {
	masked := maskBracketContent(line)
	colon := strings.Index(masked, ":")
	if colon < 0 {
		return strings.TrimSpace(line), ""
	}
	return strings.TrimSpace(line[:colon]), strings.TrimSpace(line[colon+1:])
}

func findClassRelationToken(body string) (start, end int, token string, ok bool) {
	masked := maskBracketContent(body)
	bestStart := len(masked) + 1
	bestLen := -1
	bestToken := ""
	for _, candidate := range classRelationTokens {
		idx := strings.Index(masked, candidate)
		if idx < 0 {
			continue
		}
		if idx < bestStart || (idx == bestStart && len(candidate) > bestLen) {
			bestStart = idx
			bestLen = len(candidate)
			bestToken = candidate
		}
	}
	if bestLen <= 0 {
		return 0, 0, "", false
	}
	return bestStart, bestStart + bestLen, bestToken, true
}

func classRelationMarkers(token string) (start string, end string) {
	switch {
	case strings.HasPrefix(token, "<|"):
		start = "my-svg_class-extensionStart"
	case strings.HasPrefix(token, "*"):
		start = "my-svg_class-compositionStart"
	case strings.HasPrefix(token, "o"):
		start = "my-svg_class-aggregationStart"
	case strings.HasPrefix(token, "()"):
		start = "my-svg_class-lollipopStart"
	}
	switch {
	case strings.HasSuffix(token, "|>"):
		end = "my-svg_class-extensionEnd"
	case strings.HasSuffix(token, "*"):
		end = "my-svg_class-compositionEnd"
	case strings.HasSuffix(token, "o"):
		end = "my-svg_class-aggregationEnd"
	case strings.HasSuffix(token, "()"):
		end = "my-svg_class-lollipopEnd"
	}
	return start, end
}
