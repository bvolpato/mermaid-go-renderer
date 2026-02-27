package mermaid

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

var nonIDCharRe = regexp.MustCompile(`[^a-zA-Z0-9_\-]+`)

func upper(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func lower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func stripQuotes(s string) string {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) >= 2 {
		if (trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"') ||
			(trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'') {
			return trimmed[1 : len(trimmed)-1]
		}
	}
	return trimmed
}

func parseFloat(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

func clamp(v, minValue, maxValue float64) float64 {
	if v < minValue {
		return minValue
	}
	if v > maxValue {
		return maxValue
	}
	return v
}

func intString(v int) string {
	return strconv.Itoa(v)
}

func sanitizeID(value, fallback string) string {
	value = strings.TrimSpace(stripQuotes(value))
	if value == "" {
		return fallback
	}
	value = strings.ReplaceAll(value, " ", "_")
	value = nonIDCharRe.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return fallback
	}
	return value
}

func parseStringList(raw string) []string {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start < 0 || end <= start {
		return nil
	}
	content := raw[start+1 : end]
	parts := strings.Split(content, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := stripQuotes(strings.TrimSpace(part))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func parseFloatList(raw string) []float64 {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start < 0 || end <= start {
		return nil
	}
	content := raw[start+1 : end]
	parts := strings.Split(content, ",")
	out := make([]float64, 0, len(parts))
	for _, part := range parts {
		value, ok := parseFloat(part)
		if ok {
			out = append(out, value)
		}
	}
	return out
}

func looksLikeDateOrDuration(token string) bool {
	t := lower(strings.TrimSpace(token))
	if t == "" {
		return false
	}
	if strings.Contains(t, "-") || strings.Contains(t, "/") {
		return true
	}
	return strings.HasSuffix(t, "d") || strings.HasSuffix(t, "w") || strings.HasSuffix(t, "m")
}
