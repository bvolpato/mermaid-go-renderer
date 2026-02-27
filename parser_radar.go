package mermaid

import (
	"regexp"
	"strings"
)

var (
	radarAxisTokenRe  = regexp.MustCompile(`^([A-Za-z0-9_]+)\s*(?:\[\s*"([^"]+)"\s*\])?$`)
	radarCurveTokenRe = regexp.MustCompile(`^([A-Za-z0-9_]+)\s*(?:\[\s*"([^"]+)"\s*\])?\s*\{(.+)\}$`)
)

type radarCurveDraft struct {
	Name    string
	Label   string
	Entries []float64
	ByAxis  map[string]float64
}

func parseRadar(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}

	graph := newGraph(DiagramRadar)
	graph.Source = input
	curveDrafts := make([]radarCurveDraft, 0, 8)

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		low := lower(line)
		if i == 0 && (strings.HasPrefix(low, "radar-beta") || strings.HasPrefix(low, "radar")) {
			continue
		}
		switch {
		case strings.HasPrefix(low, "title "):
			graph.RadarTitle = stripQuotes(strings.TrimSpace(line[len("title"):]))
			continue
		case strings.HasPrefix(low, "axis "):
			for _, token := range splitRadarTopLevelCSV(strings.TrimSpace(line[len("axis"):])) {
				axis, ok := parseRadarAxisToken(token)
				if ok {
					graph.RadarAxes = append(graph.RadarAxes, axis)
				}
			}
			continue
		case strings.HasPrefix(low, "curve "):
			for _, token := range splitRadarTopLevelCSV(strings.TrimSpace(line[len("curve"):])) {
				curve, ok := parseRadarCurveToken(token)
				if ok {
					curveDrafts = append(curveDrafts, curve)
				}
			}
			continue
		case strings.HasPrefix(low, "showlegend "):
			switch lower(strings.TrimSpace(line[len("showLegend"):])) {
			case "false":
				graph.RadarShowLegend = false
			default:
				graph.RadarShowLegend = true
			}
			continue
		case strings.HasPrefix(low, "ticks "):
			if value, ok := parseFloat(strings.TrimSpace(line[len("ticks"):])); ok {
				graph.RadarTicks = max(1, int(value))
			}
			continue
		case strings.HasPrefix(low, "max "):
			if value, ok := parseFloat(strings.TrimSpace(line[len("max"):])); ok {
				graph.RadarMax = &value
			}
			continue
		case strings.HasPrefix(low, "min "):
			if value, ok := parseFloat(strings.TrimSpace(line[len("min"):])); ok {
				graph.RadarMin = &value
			}
			continue
		case strings.HasPrefix(low, "graticule "):
			kind := lower(strings.TrimSpace(line[len("graticule"):]))
			if kind == "polygon" {
				graph.RadarGraticule = "polygon"
			} else {
				graph.RadarGraticule = "circle"
			}
			continue
		}
	}

	for _, draft := range curveDrafts {
		entries := append([]float64(nil), draft.Entries...)
		if len(entries) == 0 && len(draft.ByAxis) > 0 && len(graph.RadarAxes) > 0 {
			entries = make([]float64, 0, len(graph.RadarAxes))
			for _, axis := range graph.RadarAxes {
				entries = append(entries, draft.ByAxis[axis.Name])
			}
		}
		if len(entries) == 0 {
			continue
		}
		graph.RadarCurves = append(graph.RadarCurves, RadarCurve{
			Name:    draft.Name,
			Label:   draft.Label,
			Entries: entries,
		})
	}

	if len(graph.RadarAxes) == 0 || len(graph.RadarCurves) == 0 {
		return parseClassLike(input, DiagramRadar)
	}
	return ParseOutput{Graph: graph}, nil
}

func parseRadarAxisToken(token string) (RadarAxis, bool) {
	m := radarAxisTokenRe.FindStringSubmatch(strings.TrimSpace(token))
	if len(m) != 3 {
		return RadarAxis{}, false
	}
	name := sanitizeID(m[1], "")
	if name == "" {
		return RadarAxis{}, false
	}
	label := stripQuotes(strings.TrimSpace(m[2]))
	if label == "" {
		label = name
	}
	return RadarAxis{Name: name, Label: label}, true
}

func parseRadarCurveToken(token string) (radarCurveDraft, bool) {
	m := radarCurveTokenRe.FindStringSubmatch(strings.TrimSpace(token))
	if len(m) != 4 {
		return radarCurveDraft{}, false
	}
	name := sanitizeID(m[1], "")
	if name == "" {
		return radarCurveDraft{}, false
	}
	label := stripQuotes(strings.TrimSpace(m[2]))
	if label == "" {
		label = name
	}

	rawEntries := strings.TrimSpace(m[3])
	if rawEntries == "" {
		return radarCurveDraft{}, false
	}
	parts := splitRadarTopLevelCSV(rawEntries)
	entries := make([]float64, 0, len(parts))
	byAxis := map[string]float64{}
	hasNamedEntry := false
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Count(part, ":") == 1 {
			pair := strings.SplitN(part, ":", 2)
			key := sanitizeID(strings.TrimSpace(pair[0]), "")
			value, ok := parseFloat(strings.TrimSpace(pair[1]))
			if key != "" && ok {
				hasNamedEntry = true
				byAxis[key] = value
				continue
			}
		}
		if value, ok := parseFloat(part); ok {
			entries = append(entries, value)
		}
	}
	if hasNamedEntry {
		return radarCurveDraft{Name: name, Label: label, ByAxis: byAxis}, true
	}
	if len(entries) == 0 {
		return radarCurveDraft{}, false
	}
	return radarCurveDraft{Name: name, Label: label, Entries: entries}, true
}

func splitRadarTopLevelCSV(raw string) []string {
	parts := make([]string, 0, 8)
	var current strings.Builder
	depthBracket := 0
	depthBrace := 0
	var quote byte
	escaped := false

	for i := 0; i < len(raw); i++ {
		ch := raw[i]
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
			depthBracket++
		case ']':
			if depthBracket > 0 {
				depthBracket--
			}
		case '{':
			depthBrace++
		case '}':
			if depthBrace > 0 {
				depthBrace--
			}
		case ',':
			if depthBracket == 0 && depthBrace == 0 {
				part := strings.TrimSpace(current.String())
				if part != "" {
					parts = append(parts, part)
				}
				current.Reset()
				continue
			}
		}
		current.WriteByte(ch)
	}
	if part := strings.TrimSpace(current.String()); part != "" {
		parts = append(parts, part)
	}
	return parts
}
