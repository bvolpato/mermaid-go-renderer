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

func adjustColor(color string, deltaH, deltaS, deltaL float64) string {
	h, s, l, ok := parseColorToHSL(color)
	if !ok {
		return color
	}
	h += deltaH
	if h < 0 {
		h = math.Mod(h, 360.0) + 360.0
	} else if h >= 360.0 {
		h = math.Mod(h, 360.0)
	}
	s = clamp(s+deltaS, 0.0, 100.0)
	l = clamp(l+deltaL, 0.0, 100.0)
	return hslColorString(h, s, l)
}

func parseColorToHSL(color string) (h, s, l float64, ok bool) {
	if h, s, l, ok = parseHSL(color); ok {
		return h, s, l, true
	}
	r, g, b, ok := parseHexColor(color)
	if !ok {
		return 0, 0, 0, false
	}
	h, s, l = rgbToHSL(r, g, b)
	return h, s, l, true
}

func hslColorString(h, s, l float64) string {
	h = math.Round(h)
	s = math.Round(s)
	l = math.Round(l)
	return "hsl(" + strconv.FormatFloat(h, 'f', 0, 64) +
		", " + strconv.FormatFloat(s, 'f', 0, 64) +
		"%, " + strconv.FormatFloat(l, 'f', 0, 64) + "%)"
}

func parseHSL(value string) (h, s, l float64, ok bool) {
	trimmed := strings.TrimSpace(value)
	open := strings.Index(trimmed, "(")
	close := strings.LastIndex(trimmed, ")")
	if open < 0 || close <= open {
		return 0, 0, 0, false
	}
	prefix := lower(trimmed[:open])
	if prefix != "hsl" && prefix != "hsla" {
		return 0, 0, 0, false
	}
	inner := trimmed[open+1 : close]
	parts := strings.Split(inner, ",")
	if len(parts) < 3 {
		return 0, 0, 0, false
	}
	h, okH := parseFloat(parts[0])
	sv, okS := parseFloat(strings.TrimSuffix(strings.TrimSpace(parts[1]), "%"))
	lv, okL := parseFloat(strings.TrimSuffix(strings.TrimSpace(parts[2]), "%"))
	if !okH || !okS || !okL {
		return 0, 0, 0, false
	}
	return h, sv, lv, true
}

func parseHexColor(value string) (r, g, b float64, ok bool) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "#") {
		return 0, 0, 0, false
	}
	hex := strings.TrimPrefix(trimmed, "#")
	switch len(hex) {
	case 3:
		hex = strings.Repeat(string(hex[0]), 2) +
			strings.Repeat(string(hex[1]), 2) +
			strings.Repeat(string(hex[2]), 2)
	case 6:
		// keep
	case 8:
		hex = hex[:6]
	default:
		return 0, 0, 0, false
	}
	rv, errR := strconv.ParseUint(hex[0:2], 16, 8)
	gv, errG := strconv.ParseUint(hex[2:4], 16, 8)
	bv, errB := strconv.ParseUint(hex[4:6], 16, 8)
	if errR != nil || errG != nil || errB != nil {
		return 0, 0, 0, false
	}
	return float64(rv) / 255.0, float64(gv) / 255.0, float64(bv) / 255.0, true
}

func rgbToHSL(r, g, b float64) (h, s, l float64) {
	maxRGB := math.Max(r, math.Max(g, b))
	minRGB := math.Min(r, math.Min(g, b))
	l = (maxRGB + minRGB) / 2.0
	d := maxRGB - minRGB
	if d == 0 {
		return 0, 0, l * 100.0
	}
	s = d / (1.0 - math.Abs(2*l-1.0))
	switch maxRGB {
	case r:
		h = math.Mod((g-b)/d, 6.0)
	case g:
		h = (b-r)/d + 2.0
	default:
		h = (r-g)/d + 4.0
	}
	h *= 60.0
	if h < 0 {
		h += 360.0
	}
	return h, s * 100.0, l * 100.0
}
