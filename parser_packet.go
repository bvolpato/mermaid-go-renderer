package mermaid

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var packetFieldLineRe = regexp.MustCompile(`^\s*(\d+)\s*-\s*(\d+)\s*:\s*(.+?)\s*$`)

func parsePacket(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramPacket)
	graph.Source = input

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		low := lower(line)
		if strings.HasPrefix(low, "packet") {
			continue
		}
		if strings.HasPrefix(low, "title") {
			graph.PacketTitle = stripQuotes(strings.TrimSpace(line[len("title"):]))
			continue
		}

		m := packetFieldLineRe.FindStringSubmatch(line)
		if len(m) != 4 {
			continue
		}
		start, errStart := strconv.Atoi(m[1])
		end, errEnd := strconv.Atoi(m[2])
		if errStart != nil || errEnd != nil {
			continue
		}
		if end < start {
			start, end = end, start
		}
		label := stripQuotes(strings.TrimSpace(m[3]))
		if label == "" {
			continue
		}
		graph.PacketFields = append(graph.PacketFields, PacketField{
			Start: start,
			End:   end,
			Label: label,
		})
	}

	if len(graph.PacketFields) == 0 {
		return parseClassLike(input, DiagramPacket)
	}

	sort.Slice(graph.PacketFields, func(i, j int) bool {
		left := graph.PacketFields[i]
		right := graph.PacketFields[j]
		if left.Start != right.Start {
			return left.Start < right.Start
		}
		if left.End != right.End {
			return left.End < right.End
		}
		return left.Label < right.Label
	})

	return ParseOutput{Graph: graph}, nil
}
