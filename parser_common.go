package mermaid

import (
	"regexp"
	"strings"
)

var (
	arrowPattern  = `<[-.=ox]*[-=]+[-.=ox]*>|<[-.=ox]*[-=]+|[-.=ox]*[-=]+>|[-.=ox]*[-=]+`
	arrowTokenRe  = regexp.MustCompile(arrowPattern)
	pipeLabelRe   = regexp.MustCompile(`^(.+?)\s*(` + arrowPattern + `)\|(.+?)\|\s*(.+)$`)
	labelArrowRe  = regexp.MustCompile(`^(.+?)\s*(<)?([-.=ox]*[-=]+[-.=ox]*)\s+([^<>=|]+?)\s+([-.=ox]*[-=]+[-.=ox]*)(>)?\s*(.+)$`)
	simpleArrowRe = regexp.MustCompile(`^(.+?)\s*(` + arrowPattern + `)\s*(.+)$`)
	nodeMetaShape = regexp.MustCompile(`(?i)\bshape\s*:\s*([a-z0-9_-]+)`)
	nodeMetaLabel = regexp.MustCompile(`(?i)\blabel\s*:\s*("([^"\\]|\\.)*"|'([^'\\]|\\.)*'|[^,}]+)`)
)

func parseDirectionLine(line string) (Direction, bool) {
	parts := strings.Fields(strings.TrimSpace(line))
	if len(parts) != 2 || lower(parts[0]) != "direction" {
		return "", false
	}
	return directionFromToken(parts[1]), true
}

func splitStatements(line string) []string {
	parts := make([]string, 0, 4)
	var current strings.Builder
	depthSquare := 0
	depthParen := 0
	depthCurly := 0
	var quote byte
	escaped := false

	for i := 0; i < len(line); i++ {
		ch := line[i]

		if escaped {
			current.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			current.WriteByte(ch)
			escaped = true
			continue
		}
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			current.WriteByte(ch)
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			current.WriteByte(ch)
			continue
		}

		switch ch {
		case '[':
			depthSquare++
		case ']':
			if depthSquare > 0 {
				depthSquare--
			}
		case '(':
			depthParen++
		case ')':
			if depthParen > 0 {
				depthParen--
			}
		case '{':
			depthCurly++
		case '}':
			if depthCurly > 0 {
				depthCurly--
			}
		case ';':
			if depthSquare == 0 && depthParen == 0 && depthCurly == 0 {
				stmt := strings.TrimSpace(current.String())
				if stmt != "" {
					parts = append(parts, stmt)
				}
				current.Reset()
				continue
			}
		}
		current.WriteByte(ch)
	}

	if stmt := strings.TrimSpace(current.String()); stmt != "" {
		parts = append(parts, stmt)
	}
	return parts
}

func stripTrailingComment(line string) string {
	var out strings.Builder
	var quote byte
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			out.WriteByte(ch)
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			out.WriteByte(ch)
			continue
		}
		if ch == '%' && i+1 < len(line) && line[i+1] == '%' {
			break
		}
		out.WriteByte(ch)
	}
	return strings.TrimSpace(out.String())
}

func stripTrailingCommentKeepIndent(line string) string {
	var out strings.Builder
	var quote byte
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			out.WriteByte(ch)
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			out.WriteByte(ch)
			continue
		}
		if ch == '%' && i+1 < len(line) && line[i+1] == '%' {
			break
		}
		out.WriteByte(ch)
	}
	return strings.TrimRight(out.String(), " \t")
}

func countIndent(line string) int {
	total := 0
	for _, ch := range line {
		if ch == ' ' {
			total++
			continue
		}
		if ch == '\t' {
			total += 2
			continue
		}
		break
	}
	return total
}

func maskBracketContent(line string) string {
	mask := []byte(line)
	depthSquare := 0
	depthParen := 0
	depthCurly := 0
	var quote byte
	escaped := false

	for i := 0; i < len(mask); i++ {
		ch := mask[i]
		if escaped {
			escaped = false
			if depthSquare > 0 || depthParen > 0 || depthCurly > 0 || quote != 0 {
				mask[i] = ' '
			}
			continue
		}
		if ch == '\\' {
			escaped = true
			if depthSquare > 0 || depthParen > 0 || depthCurly > 0 || quote != 0 {
				mask[i] = ' '
			}
			continue
		}

		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			if ch != '"' && ch != '\'' {
				mask[i] = ' '
			}
			continue
		}

		switch ch {
		case '"', '\'':
			quote = ch
		case '[':
			depthSquare++
		case ']':
			if depthSquare > 0 {
				depthSquare--
			}
		case '(':
			if depthSquare == 0 && depthCurly == 0 {
				depthParen++
			}
		case ')':
			if depthParen > 0 {
				depthParen--
			}
		case '{':
			if depthSquare == 0 && depthParen == 0 {
				depthCurly++
			}
		case '}':
			if depthCurly > 0 {
				depthCurly--
			}
		default:
			if depthSquare > 0 || depthParen > 0 || depthCurly > 0 {
				mask[i] = ' '
			}
		}
	}
	return string(mask)
}

func splitEdgeChain(line string) []string {
	masked := maskBracketContent(line)
	matches := arrowTokenRe.FindAllStringIndex(masked, -1)
	if len(matches) < 2 {
		return nil
	}
	if len(matches) == 2 {
		firstArrow := strings.TrimSpace(line[matches[0][0]:matches[0][1]])
		secondArrow := strings.TrimSpace(line[matches[1][0]:matches[1][1]])
		between := strings.TrimSpace(line[matches[0][1]:matches[1][0]])
		// Mermaid supports inline edge labels like `A -. label .-> B`.
		// Treat this as a single edge and let parseEdgeLine handle it.
		if between != "" &&
			!strings.ContainsAny(firstArrow, "<>") &&
			strings.Contains(secondArrow, ">") {
			return nil
		}
	}

	nodes := make([]string, 0, len(matches)+1)
	arrows := make([]string, 0, len(matches))
	last := 0
	for _, m := range matches {
		nodes = append(nodes, strings.TrimSpace(line[last:m[0]]))
		arrows = append(arrows, strings.TrimSpace(line[m[0]:m[1]]))
		last = m[1]
	}
	nodes = append(nodes, strings.TrimSpace(line[last:]))
	if len(nodes) != len(arrows)+1 {
		return nil
	}
	for i := 1; i < len(nodes); i++ {
		trimmed := strings.TrimSpace(nodes[i])
		if strings.HasPrefix(trimmed, "|") {
			if end := strings.Index(trimmed[1:], "|"); end >= 0 {
				label := trimmed[:end+2]
				arrows[i-1] += label
				nodes[i] = strings.TrimSpace(trimmed[end+2:])
			}
		}
	}
	for _, n := range nodes {
		if n == "" {
			return nil
		}
	}

	out := make([]string, 0, len(arrows))
	for i := range arrows {
		out = append(out, nodes[i]+" "+arrows[i]+" "+nodes[i+1])
	}
	return out
}

type edgeMeta struct {
	directed    bool
	arrowStart  bool
	arrowEnd    bool
	style       EdgeStyle
	startDeco   byte
	endDeco     byte
	raw         string
	startMarker string
	endMarker   string
}

func parseEdgeMeta(arrow string) edgeMeta {
	trimmed := strings.TrimSpace(arrow)
	raw := trimmed
	var startDecoration byte
	var endDecoration byte

	startMarker := ""
	endMarker := ""
	if strings.HasPrefix(trimmed, "<|") {
		startMarker = "extension"
		trimmed = strings.TrimPrefix(trimmed, "<|")
	}
	if strings.HasSuffix(trimmed, "|>") {
		endMarker = "extension"
		trimmed = strings.TrimSuffix(trimmed, "|>")
	}
	if strings.HasPrefix(trimmed, "*") {
		startMarker = "composition"
		trimmed = trimmed[1:]
	}
	if strings.HasSuffix(trimmed, "*") {
		endMarker = "composition"
		trimmed = trimmed[:len(trimmed)-1]
	}
	if len(trimmed) > 0 && (trimmed[0] == 'o' || trimmed[0] == 'x') {
		startDecoration = trimmed[0]
		if startDecoration == 'o' {
			startMarker = "aggregation"
		} else if startMarker == "" {
			startMarker = "dependency"
		}
		trimmed = trimmed[1:]
	}
	if len(trimmed) > 0 && (trimmed[len(trimmed)-1] == 'o' || trimmed[len(trimmed)-1] == 'x') {
		endDecoration = trimmed[len(trimmed)-1]
		if endDecoration == 'o' {
			endMarker = "aggregation"
		} else if endMarker == "" {
			endMarker = "dependency"
		}
		trimmed = trimmed[:len(trimmed)-1]
	}

	meta := edgeMeta{
		arrowStart:  strings.HasPrefix(trimmed, "<"),
		arrowEnd:    strings.HasSuffix(trimmed, ">"),
		style:       EdgeSolid,
		startDeco:   startDecoration,
		endDeco:     endDecoration,
		raw:         raw,
		startMarker: startMarker,
		endMarker:   endMarker,
	}
	if meta.endMarker == "" && strings.Contains(trimmed, "..") && strings.HasSuffix(trimmed, ">") {
		meta.endMarker = "dependency"
		meta.arrowEnd = false
	}
	if meta.startMarker == "" && strings.Contains(trimmed, "..") && strings.HasPrefix(trimmed, "<") {
		meta.startMarker = "dependency"
		meta.arrowStart = false
	}
	meta.directed = meta.arrowStart || meta.arrowEnd ||
		startDecoration != 0 || endDecoration != 0 ||
		meta.startMarker != "" || meta.endMarker != ""
	if strings.Contains(trimmed, ".") {
		meta.style = EdgeDotted
	}
	if strings.Contains(trimmed, "=") {
		meta.style = EdgeThick
	}
	return meta
}

func parseEdgeLine(line string) (left, label, right string, meta edgeMeta, ok bool) {
	masked := maskBracketContent(line)

	if idx := pipeLabelRe.FindStringSubmatchIndex(masked); idx != nil && len(idx) >= 10 {
		left = strings.TrimSpace(line[idx[2]:idx[3]])
		arrow := strings.TrimSpace(line[idx[4]:idx[5]])
		label = strings.TrimSpace(line[idx[6]:idx[7]])
		right = strings.TrimSpace(line[idx[8]:idx[9]])
		if left != "" && right != "" {
			meta = parseEdgeMeta(arrow)
			return left, label, right, meta, true
		}
	}

	arrowMatches := arrowTokenRe.FindAllStringIndex(masked, -1)
	if len(arrowMatches) == 2 {
		leftRaw := strings.TrimSpace(line[:arrowMatches[0][0]])
		arrow1 := strings.TrimSpace(line[arrowMatches[0][0]:arrowMatches[0][1]])
		inlineLabel := strings.TrimSpace(line[arrowMatches[0][1]:arrowMatches[1][0]])
		arrow2 := strings.TrimSpace(line[arrowMatches[1][0]:arrowMatches[1][1]])
		rightRaw := strings.TrimSpace(line[arrowMatches[1][1]:])
		if strings.HasPrefix(inlineLabel, ".") && (strings.Contains(arrow1, ".") || strings.HasPrefix(arrow2, ".")) {
			inlineLabel = strings.TrimSpace(strings.TrimPrefix(inlineLabel, "."))
		}
		if strings.HasSuffix(inlineLabel, ".") && (strings.Contains(arrow2, ".") || strings.HasSuffix(arrow1, ".")) {
			inlineLabel = strings.TrimSpace(strings.TrimSuffix(inlineLabel, "."))
		}
		// Support Mermaid's compact inline label form: `A -. label .-> B`
		if leftRaw != "" &&
			rightRaw != "" &&
			inlineLabel != "" &&
			!strings.ContainsAny(arrow1, "<>") &&
			strings.Contains(arrow2, ">") {
			meta = parseEdgeMeta(arrow1 + arrow2)
			return leftRaw, inlineLabel, rightRaw, meta, true
		}
	}

	if idx := labelArrowRe.FindStringSubmatchIndex(masked); idx != nil && len(idx) >= 16 {
		left = strings.TrimSpace(line[idx[2]:idx[3]])
		start := strings.TrimSpace(submatch(line, idx, 4))
		dash1 := strings.TrimSpace(submatch(line, idx, 6))
		label = strings.TrimSpace(submatch(line, idx, 8))
		dash2 := strings.TrimSpace(submatch(line, idx, 10))
		end := strings.TrimSpace(submatch(line, idx, 12))
		right = strings.TrimSpace(submatch(line, idx, 14))
		if left != "" && right != "" && label != "" {
			meta = parseEdgeMeta(start + dash1 + dash2 + end)
			return left, label, right, meta, true
		}
	}

	idx := simpleArrowRe.FindStringSubmatchIndex(masked)
	if idx == nil || len(idx) < 8 {
		return "", "", "", edgeMeta{}, false
	}
	left = strings.TrimSpace(line[idx[2]:idx[3]])
	arrow := strings.TrimSpace(line[idx[4]:idx[5]])
	right = strings.TrimSpace(line[idx[6]:idx[7]])
	if left == "" || right == "" {
		return "", "", "", edgeMeta{}, false
	}

	if strings.HasPrefix(right, "|") {
		if end := strings.Index(right[1:], "|"); end >= 0 {
			label = strings.TrimSpace(right[1 : end+1])
			right = strings.TrimSpace(right[end+2:])
		}
	}
	if right == "" {
		return "", "", "", edgeMeta{}, false
	}

	meta = parseEdgeMeta(arrow)
	return left, label, right, meta, true
}

func submatch(line string, idx []int, group int) string {
	if group*2+1 >= len(idx) {
		return ""
	}
	start := idx[group*2]
	end := idx[group*2+1]
	if start < 0 || end < 0 || start > end || end > len(line) {
		return ""
	}
	return line[start:end]
}

func addEdgeFromLine(graph *Graph, line string) bool {
	left, label, right, meta, ok := parseEdgeLine(line)
	if !ok {
		return false
	}
	sources := splitNodeList(left)
	targets := splitNodeList(right)
	if len(sources) == 0 || len(targets) == 0 {
		return false
	}

	sourceIDs := make([]string, 0, len(sources))
	for _, source := range sources {
		id, nodeLabel, shape, _ := parseNodeToken(source)
		if id == "" {
			continue
		}
		graph.ensureNode(id, nodeLabel, shape)
		sourceIDs = append(sourceIDs, id)
	}

	targetIDs := make([]string, 0, len(targets))
	for _, target := range targets {
		id, nodeLabel, shape, _ := parseNodeToken(target)
		if id == "" {
			continue
		}
		graph.ensureNode(id, nodeLabel, shape)
		targetIDs = append(targetIDs, id)
	}

	for _, from := range sourceIDs {
		for _, to := range targetIDs {
			markerStart := ""
			markerEnd := ""
			if graph.Kind == DiagramFlowchart {
				switch meta.startDeco {
				case 'o':
					markerStart = "my-svg_flowchart-v2-circleStart"
				case 'x':
					markerStart = "my-svg_flowchart-v2-crossStart"
				}
				switch meta.endDeco {
				case 'o':
					markerEnd = "my-svg_flowchart-v2-circleEnd"
				case 'x':
					markerEnd = "my-svg_flowchart-v2-crossEnd"
				}
			} else if graph.Kind == DiagramClass {
				switch meta.startMarker {
				case "aggregation":
					markerStart = "my-svg_class-aggregationStart"
				case "extension":
					markerStart = "my-svg_class-extensionStart"
				case "composition":
					markerStart = "my-svg_class-compositionStart"
				case "dependency":
					markerStart = "my-svg_class-dependencyStart"
				}
				switch meta.endMarker {
				case "aggregation":
					markerEnd = "my-svg_class-aggregationEnd"
				case "extension":
					markerEnd = "my-svg_class-extensionEnd"
				case "composition":
					markerEnd = "my-svg_class-compositionEnd"
				case "dependency":
					markerEnd = "my-svg_class-dependencyEnd"
				}
			}
			arrowStart := meta.arrowStart
			arrowEnd := meta.arrowEnd
			if graph.Kind == DiagramClass {
				if markerStart != "" {
					arrowStart = false
				}
				if markerEnd != "" {
					arrowEnd = false
				}
			}
			graph.addEdge(Edge{
				From:        from,
				To:          to,
				Label:       label,
				Directed:    meta.directed,
				ArrowStart:  arrowStart,
				ArrowEnd:    arrowEnd,
				Style:       meta.style,
				MarkerStart: markerStart,
				MarkerEnd:   markerEnd,
			})
		}
	}
	return true
}

func splitNodeList(raw string) []string {
	parts := strings.Split(raw, "&")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseNodeOnly(line string) (id, label string, shape NodeShape, ok bool) {
	if arrowTokenRe.MatchString(line) || strings.Contains(line, "--") {
		return "", "", "", false
	}
	id, label, shape, _ = parseNodeToken(line)
	return id, label, shape, id != ""
}

func parseNodeToken(token string) (id, label string, shape NodeShape, classes []string) {
	base, classes := splitInlineClasses(token)
	trimmed := strings.TrimSpace(base)
	if trimmed == "" {
		return "", "", "", classes
	}
	if metadataID, metadataLabel, metadataShape, ok := parseNodeMetadataToken(trimmed); ok {
		return metadataID, metadataLabel, metadataShape, classes
	}
	if asymmetricID, asymmetricLabel, ok := splitAsymmetricLabel(trimmed); ok {
		return asymmetricID, asymmetricLabel, ShapeAsymmetric, classes
	}
	if parsedID, parsedLabel, parsedShape, ok := splitIDLabel(trimmed); ok {
		return parsedID, parsedLabel, parsedShape, classes
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "", "", "", classes
	}
	id = stripQuotes(fields[0])
	return id, id, ShapeRectangle, classes
}

func splitInlineClasses(token string) (string, []string) {
	parts := strings.Split(token, ":::")
	base := strings.TrimSpace(parts[0])
	classes := make([]string, 0, len(parts)-1)
	for _, class := range parts[1:] {
		c := strings.TrimSpace(class)
		if c != "" {
			classes = append(classes, c)
		}
	}
	return base, classes
}

func parseNodeMetadataToken(token string) (id, label string, shape NodeShape, ok bool) {
	idx := strings.Index(token, "@{")
	if idx <= 0 || !strings.HasSuffix(strings.TrimSpace(token), "}") {
		return "", "", "", false
	}
	id = stripQuotes(strings.TrimSpace(token[:idx]))
	if id == "" {
		return "", "", "", false
	}
	body := strings.TrimSpace(token[idx+2:])
	if !strings.HasSuffix(body, "}") {
		return "", "", "", false
	}
	body = strings.TrimSpace(body[:len(body)-1])
	if body == "" {
		return id, id, ShapeRectangle, true
	}

	label = id
	shape = ShapeRectangle

	if matches := nodeMetaLabel.FindStringSubmatch(body); len(matches) >= 2 {
		raw := strings.TrimSpace(matches[1])
		label = stripQuotes(raw)
		if label == "" {
			label = id
		}
	}

	if matches := nodeMetaShape.FindStringSubmatch(body); len(matches) >= 2 {
		shape = shapeFromMermaidToken(matches[1])
	}

	return id, label, shape, true
}

func shapeFromMermaidToken(raw string) NodeShape {
	token := lower(strings.TrimSpace(raw))
	switch token {
	case "round-rect", "rounded", "event":
		return ShapeRoundRect
	case "stadium", "pill", "terminal":
		return ShapeStadium
	case "subproc", "subprocess", "subroutine", "fr-rect":
		return ShapeSubroutine
	case "cyl", "cylinder", "db", "database", "h-cyl", "lin-cyl":
		return ShapeCylinder
	case "circle", "circ", "sm-circ", "start", "f-circ", "junction":
		return ShapeCircle
	case "dbl-circ", "double-circle", "fr-circ", "stop", "cross-circ":
		return ShapeDoubleCircle
	case "diamond", "decision", "question", "diam":
		return ShapeDiamond
	case "hex", "hexagon", "prepare":
		return ShapeHexagon
	case "parallelogram", "lean-r", "lean-l", "in-out", "out-in":
		return ShapeParallelogram
	case "trapezoid", "trap-t", "trap-b", "priority", "manual":
		return ShapeTrapezoid
	case "asymmetric", "odd":
		return ShapeAsymmetric
	default:
		return ShapeRectangle
	}
}

func splitAsymmetricLabel(token string) (id, label string, ok bool) {
	trimmed := strings.TrimSpace(token)
	pos := strings.Index(trimmed, ">")
	if pos <= 0 || !strings.HasSuffix(trimmed, "]") || strings.Contains(trimmed, "[") {
		return "", "", false
	}
	id = strings.TrimSpace(trimmed[:pos])
	label = strings.TrimSpace(trimmed[pos+1 : len(trimmed)-1])
	if id == "" || label == "" {
		return "", "", false
	}
	return stripQuotes(id), stripQuotes(label), true
}

func splitIDLabel(token string) (id, label string, shape NodeShape, ok bool) {
	if start := strings.Index(token, "["); start > 0 && strings.HasSuffix(token, "]") {
		id = strings.TrimSpace(token[:start])
		label, shape = parseShapeFromBrackets(strings.TrimSpace(token[start:]))
		return stripQuotes(id), label, shape, id != ""
	}
	if start := strings.Index(token, "("); start > 0 && strings.HasSuffix(token, ")") {
		id = strings.TrimSpace(token[:start])
		label, shape = parseShapeFromParens(strings.TrimSpace(token[start:]))
		return stripQuotes(id), label, shape, id != ""
	}
	if start := strings.Index(token, "{"); start > 0 && strings.HasSuffix(token, "}") {
		id = strings.TrimSpace(token[:start])
		label, shape = parseShapeFromBraces(strings.TrimSpace(token[start:]))
		return stripQuotes(id), label, shape, id != ""
	}
	return "", "", "", false
}

func parseShapeFromBrackets(raw string) (string, NodeShape) {
	trimmed := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(trimmed, "[[") && strings.HasSuffix(trimmed, "]]"):
		return stripQuotes(trimmed[2 : len(trimmed)-2]), ShapeSubroutine
	case strings.HasPrefix(trimmed, "[(") && strings.HasSuffix(trimmed, ")]"):
		return stripQuotes(trimmed[2 : len(trimmed)-2]), ShapeCylinder
	case strings.HasPrefix(trimmed, "[/") && strings.HasSuffix(trimmed, "/]"):
		return stripQuotes(trimmed[2 : len(trimmed)-2]), ShapeParallelogram
	case strings.HasPrefix(trimmed, "[/") && strings.HasSuffix(trimmed, "\\]"):
		return stripQuotes(trimmed[2 : len(trimmed)-2]), ShapeTrapezoid
	case strings.HasPrefix(trimmed, "[(") && strings.HasSuffix(trimmed, ")]"):
		return stripQuotes(trimmed[2 : len(trimmed)-2]), ShapeStadium
	case strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"):
		inner := trimmed[1 : len(trimmed)-1]
		if strings.HasPrefix(inner, "(") && strings.HasSuffix(inner, ")") {
			return stripQuotes(inner[1 : len(inner)-1]), ShapeStadium
		}
		return stripQuotes(inner), ShapeRectangle
	default:
		return stripQuotes(trimmed), ShapeRectangle
	}
}

func parseShapeFromParens(raw string) (string, NodeShape) {
	trimmed := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(trimmed, "([") && strings.HasSuffix(trimmed, "])"):
		return stripQuotes(trimmed[2 : len(trimmed)-2]), ShapeStadium
	case strings.HasPrefix(trimmed, "(((") && strings.HasSuffix(trimmed, ")))"):
		return stripQuotes(trimmed[3 : len(trimmed)-3]), ShapeDoubleCircle
	case strings.HasPrefix(trimmed, "((") && strings.HasSuffix(trimmed, "))"):
		return stripQuotes(trimmed[2 : len(trimmed)-2]), ShapeCircle
	case strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")"):
		return stripQuotes(trimmed[1 : len(trimmed)-1]), ShapeRoundRect
	default:
		return stripQuotes(trimmed), ShapeRoundRect
	}
}

func parseShapeFromBraces(raw string) (string, NodeShape) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") {
		return stripQuotes(trimmed[2 : len(trimmed)-2]), ShapeHexagon
	}
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		return stripQuotes(trimmed[1 : len(trimmed)-1]), ShapeDiamond
	}
	return stripQuotes(trimmed), ShapeDiamond
}
