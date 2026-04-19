package mermaid

import (
	"regexp"
	"strings"
)

var erRelationshipHeadRe = regexp.MustCompile(`^\s*("[^"]+"|\S+)\s+(.+)\s+("[^"]+"|\S+)\s*$`)

type erCardinalityAlias struct {
	alias string
	kind  string
}

var erCardinalityAliases = []erCardinalityAlias{
	{alias: "one or zero", kind: "zeroOrOne"},
	{alias: "zero or one", kind: "zeroOrOne"},
	{alias: "zero or more", kind: "zeroOrMore"},
	{alias: "zero or many", kind: "zeroOrMore"},
	{alias: "one or more", kind: "oneOrMore"},
	{alias: "one or many", kind: "oneOrMore"},
	{alias: "only one", kind: "onlyOne"},
	{alias: "many(0)", kind: "zeroOrMore"},
	{alias: "many(1)", kind: "oneOrMore"},
	{alias: "1+", kind: "oneOrMore"},
	{alias: "0+", kind: "zeroOrMore"},
	{alias: "many", kind: "zeroOrMore"},
	{alias: "one", kind: "onlyOne"},
	{alias: "1", kind: "onlyOne"},
	{alias: "||", kind: "onlyOne"},
	{alias: "|o", kind: "zeroOrOne"},
	{alias: "o|", kind: "zeroOrOne"},
	{alias: "}o", kind: "zeroOrMore"},
	{alias: "o{", kind: "zeroOrMore"},
	{alias: "}|", kind: "oneOrMore"},
	{alias: "|{", kind: "oneOrMore"},
}

type erRelationAlias struct {
	alias string
	style EdgeStyle
}

var erRelationAliases = []erRelationAlias{
	{alias: "optionally to", style: EdgeDotted},
	{alias: "to", style: EdgeSolid},
	{alias: "--", style: EdgeSolid},
	{alias: "..", style: EdgeDotted},
	{alias: ".-", style: EdgeDotted},
	{alias: "-.", style: EdgeDotted},
}

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

	m := erRelationshipHeadRe.FindStringSubmatch(head)
	if len(m) != 4 {
		return "", "", "", "", "", "", false
	}
	left = strings.TrimSpace(m[1])
	right = strings.TrimSpace(m[3])
	relation := strings.TrimSpace(m[2])

	startKind, parsedStyle, endKind, ok := parseERRelationshipSpec(relation)
	if !ok {
		return "", "", "", "", "", "", false
	}
	markerStart = erCardinalityMarkerID(startKind, true)
	markerEnd = erCardinalityMarkerID(endKind, false)
	style = parsedStyle
	return left, right, relationLabel, style, markerStart, markerEnd, true
}

func parseERRelationshipSpec(spec string) (startKind string, style EdgeStyle, endKind string, ok bool) {
	firstKind, rest, ok := consumeERCardinality(spec)
	if !ok {
		return "", "", "", false
	}
	style, rest, ok = consumeERRelationshipConnector(rest)
	if !ok {
		return "", "", "", false
	}
	secondKind, rest, ok := consumeERCardinality(rest)
	if !ok || strings.TrimSpace(rest) != "" {
		return "", "", "", false
	}
	return firstKind, style, secondKind, true
}

func consumeERCardinality(raw string) (kind string, rest string, ok bool) {
	trimmed := strings.TrimSpace(raw)
	lowered := lower(trimmed)
	for _, alias := range erCardinalityAliases {
		if !strings.HasPrefix(lowered, alias.alias) {
			continue
		}
		if !erTokenBoundary(trimmed, len(alias.alias)) {
			continue
		}
		return alias.kind, strings.TrimSpace(trimmed[len(alias.alias):]), true
	}
	return "", raw, false
}

func consumeERRelationshipConnector(raw string) (style EdgeStyle, rest string, ok bool) {
	trimmed := strings.TrimSpace(raw)
	lowered := lower(trimmed)
	for _, alias := range erRelationAliases {
		if !strings.HasPrefix(lowered, alias.alias) {
			continue
		}
		if !erTokenBoundary(trimmed, len(alias.alias)) {
			continue
		}
		return alias.style, strings.TrimSpace(trimmed[len(alias.alias):]), true
	}
	return "", raw, false
}

func erTokenBoundary(raw string, offset int) bool {
	if offset >= len(raw) {
		return true
	}
	switch raw[offset] {
	case ' ', '\t', '\r', '\n', '-', '.', '|', '{', '}', 'o':
		return true
	default:
		return false
	}
}

func erCardinalityMarkerID(kind string, start bool) string {
	if kind == "" {
		return ""
	}
	if start {
		return "my-svg_er-" + kind + "Start"
	}
	return "my-svg_er-" + kind + "End"
}
