package mermaid

import (
	"strconv"
	"strings"
	"unicode"
)

func parseBlock(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramBlock)
	graph.Source = input

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		low := lower(line)
		if low == "block" || strings.HasPrefix(low, "block ") {
			continue
		}
		if strings.HasPrefix(low, "columns ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if n, convErr := strconv.Atoi(fields[1]); convErr == nil && n > 0 {
					graph.BlockColumns = n
				}
			}
			continue
		}
		if addEdgeFromLine(&graph, line) {
			continue
		}

		row := make([]string, 0, 4)
		for _, token := range splitBlockRowTokens(line) {
			id, label, shape, ok := parseNodeOnly(token)
			if !ok {
				continue
			}
			graph.ensureNode(id, label, shape)
			row = append(row, id)
		}
		if len(row) > 0 {
			graph.BlockRows = append(graph.BlockRows, row)
		}
	}

	if graph.BlockColumns <= 0 {
		for _, row := range graph.BlockRows {
			graph.BlockColumns = max(graph.BlockColumns, len(row))
		}
		if graph.BlockColumns == 0 {
			graph.BlockColumns = max(1, len(graph.NodeOrder))
		}
	}

	return ParseOutput{Graph: graph}, nil
}

func splitBlockRowTokens(line string) []string {
	tokens := make([]string, 0, 4)
	var current strings.Builder
	brackets := 0
	parens := 0
	braces := 0
	inQuote := rune(0)
	prev := rune(0)
	flush := func() {
		token := strings.TrimSpace(current.String())
		if token != "" {
			tokens = append(tokens, token)
		}
		current.Reset()
	}
	for _, r := range line {
		if (r == '"' || r == '\'') && prev != '\\' {
			if inQuote == 0 {
				inQuote = r
			} else if inQuote == r {
				inQuote = 0
			}
			current.WriteRune(r)
			prev = r
			continue
		}
		if inQuote != 0 {
			current.WriteRune(r)
			prev = r
			continue
		}
		switch r {
		case '[':
			brackets++
		case ']':
			brackets = max(0, brackets-1)
		case '(':
			parens++
		case ')':
			parens = max(0, parens-1)
		case '{':
			braces++
		case '}':
			braces = max(0, braces-1)
		}
		if unicode.IsSpace(r) && brackets == 0 && parens == 0 && braces == 0 {
			flush()
			prev = r
			continue
		}
		current.WriteRune(r)
		prev = r
	}
	flush()
	return tokens
}
