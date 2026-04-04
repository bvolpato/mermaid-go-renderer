package mermaid

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// markerDef holds a parsed SVG <marker> definition.
type markerDef struct {
	ID          string
	ViewBox     [4]float64 // minX, minY, width, height
	RefX, RefY  float64
	MarkerW, MarkerH float64
	PathD       string // the d= attribute of the <path> child
	FillColor   string // fill of the arrowhead path
}

var (
	reMarkerElement  = regexp.MustCompile(`(?s)<marker\b([^>]*)>(.*?)</marker>`)
	reMarkerEnd      = regexp.MustCompile(`\bmarker-end\s*=\s*"url\(#([^)]+)\)"`)
	reMarkerStart    = regexp.MustCompile(`\bmarker-start\s*=\s*"url\(#([^)]+)\)"`)
	reMarkerPathTag  = regexp.MustCompile(`(?s)<(?:path|circle)\b([^>]*)`)
	rePathWithMarker = regexp.MustCompile(`<path\b[^>]*marker-(?:end|start)\s*=[^>]*>`)
	reLineWithMarker = regexp.MustCompile(`<line\b[^>]*marker-(?:end|start)\s*=[^>]*/>`)
)

func getAttr(attrs, name string) string {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*=\s*"([^"]*)"`)
	m := re.FindStringSubmatch(attrs)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

func parseMarkerDefs(svg string) map[string]markerDef {
	markers := make(map[string]markerDef)
	for _, m := range reMarkerElement.FindAllStringSubmatch(svg, -1) {
		attrs := m[1]
		body := m[2]

		id := getAttr(attrs, "id")
		if id == "" {
			continue
		}

		var def markerDef
		def.ID = id

		if vb := getAttr(attrs, "viewBox"); vb != "" {
			parts := strings.Fields(vb)
			if len(parts) == 4 {
				def.ViewBox[0], _ = strconv.ParseFloat(parts[0], 64)
				def.ViewBox[1], _ = strconv.ParseFloat(parts[1], 64)
				def.ViewBox[2], _ = strconv.ParseFloat(parts[2], 64)
				def.ViewBox[3], _ = strconv.ParseFloat(parts[3], 64)
			}
		}
		def.RefX, _ = strconv.ParseFloat(getAttr(attrs, "refX"), 64)
		def.RefY, _ = strconv.ParseFloat(getAttr(attrs, "refY"), 64)
		def.MarkerW, _ = strconv.ParseFloat(getAttr(attrs, "markerWidth"), 64)
		def.MarkerH, _ = strconv.ParseFloat(getAttr(attrs, "markerHeight"), 64)
		if def.MarkerW <= 0 {
			def.MarkerW = 8
		}
		if def.MarkerH <= 0 {
			def.MarkerH = 8
		}

		// Extract path d or circle from body
		if pm := reMarkerPathTag.FindStringSubmatch(body); len(pm) >= 2 {
			def.PathD = getAttr(pm[1], "d")
			// For circles, synthesize a circle path
			if def.PathD == "" {
				cx, _ := strconv.ParseFloat(getAttr(pm[1], "cx"), 64)
				cy, _ := strconv.ParseFloat(getAttr(pm[1], "cy"), 64)
				r, _ := strconv.ParseFloat(getAttr(pm[1], "r"), 64)
				if r > 0 {
					def.PathD = fmt.Sprintf("M %f %f m -%f 0 a %f %f 0 1 0 %f 0 a %f %f 0 1 0 -%f 0",
						cx, cy, r, r, r, 2*r, r, r, 2*r)
				}
			}
		}

		// Get fill color from arrowMarkerPath style or default
		def.FillColor = "#333333"
		if styleAttr := getAttr(body, "style"); styleAttr != "" {
			if fill := styleValue(styleAttr, "fill"); fill != "" && fill != "none" {
				def.FillColor = fill
			}
		}
		if fillAttr := getAttr(body, "fill"); fillAttr != "" && fillAttr != "none" {
			def.FillColor = fillAttr
		}

		markers[id] = def
	}
	return markers
}

// pathEndpoint extracts the last point and direction angle from an SVG path d string.
func pathEndpoint(d string, atStart bool) (x, y, angle float64, ok bool) {
	// Parse path into coordinate pairs
	points := extractPathPoints(d)
	if len(points) < 2 {
		if len(points) == 1 {
			return points[0][0], points[0][1], 0, true
		}
		return 0, 0, 0, false
	}

	if atStart {
		dx := points[1][0] - points[0][0]
		dy := points[1][1] - points[0][1]
		angle = math.Atan2(dy, dx)
		return points[0][0], points[0][1], angle, true
	}

	n := len(points)
	dx := points[n-1][0] - points[n-2][0]
	dy := points[n-1][1] - points[n-2][1]
	angle = math.Atan2(dy, dx)
	return points[n-1][0], points[n-1][1], angle, true
}

// extractPathPoints parses absolute coordinate pairs from an SVG path d string.
func extractPathPoints(d string) [][2]float64 {
	var points [][2]float64
	tokens := strings.Fields(normalizePathData(d))
	var curX, curY float64
	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		switch tok {
		case "M", "L", "T":
			if i+2 < len(tokens) {
				x, err1 := strconv.ParseFloat(tokens[i+1], 64)
				y, err2 := strconv.ParseFloat(tokens[i+2], 64)
				if err1 == nil && err2 == nil {
					curX, curY = x, y
					points = append(points, [2]float64{x, y})
				}
				i += 3
			} else {
				i++
			}
		case "m", "l", "t":
			if i+2 < len(tokens) {
				dx, err1 := strconv.ParseFloat(tokens[i+1], 64)
				dy, err2 := strconv.ParseFloat(tokens[i+2], 64)
				if err1 == nil && err2 == nil {
					curX += dx
					curY += dy
					points = append(points, [2]float64{curX, curY})
				}
				i += 3
			} else {
				i++
			}
		case "C":
			// Cubic bezier: C x1 y1 x2 y2 x y
			if i+6 < len(tokens) {
				x, err1 := strconv.ParseFloat(tokens[i+5], 64)
				y, err2 := strconv.ParseFloat(tokens[i+6], 64)
				// Also record control point for angle calculation
				cx, err3 := strconv.ParseFloat(tokens[i+3], 64)
				cy, err4 := strconv.ParseFloat(tokens[i+4], 64)
				if err1 == nil && err2 == nil {
					curX, curY = x, y
					if err3 == nil && err4 == nil {
						points = append(points, [2]float64{cx, cy})
					}
					points = append(points, [2]float64{x, y})
				}
				i += 7
			} else {
				i++
			}
		case "c":
			if i+6 < len(tokens) {
				dx, err1 := strconv.ParseFloat(tokens[i+5], 64)
				dy, err2 := strconv.ParseFloat(tokens[i+6], 64)
				cx2, err3 := strconv.ParseFloat(tokens[i+3], 64)
				cy2, err4 := strconv.ParseFloat(tokens[i+4], 64)
				if err1 == nil && err2 == nil {
					if err3 == nil && err4 == nil {
						points = append(points, [2]float64{curX + cx2, curY + cy2})
					}
					curX += dx
					curY += dy
					points = append(points, [2]float64{curX, curY})
				}
				i += 7
			} else {
				i++
			}
		case "Q":
			if i+4 < len(tokens) {
				x, err1 := strconv.ParseFloat(tokens[i+3], 64)
				y, err2 := strconv.ParseFloat(tokens[i+4], 64)
				if err1 == nil && err2 == nil {
					curX, curY = x, y
					points = append(points, [2]float64{x, y})
				}
				i += 5
			} else {
				i++
			}
		case "q":
			if i+4 < len(tokens) {
				dx, err1 := strconv.ParseFloat(tokens[i+3], 64)
				dy, err2 := strconv.ParseFloat(tokens[i+4], 64)
				if err1 == nil && err2 == nil {
					curX += dx
					curY += dy
					points = append(points, [2]float64{curX, curY})
				}
				i += 5
			} else {
				i++
			}
		case "H":
			if i+1 < len(tokens) {
				x, err := strconv.ParseFloat(tokens[i+1], 64)
				if err == nil {
					curX = x
					points = append(points, [2]float64{curX, curY})
				}
				i += 2
			} else {
				i++
			}
		case "h":
			if i+1 < len(tokens) {
				dx, err := strconv.ParseFloat(tokens[i+1], 64)
				if err == nil {
					curX += dx
					points = append(points, [2]float64{curX, curY})
				}
				i += 2
			} else {
				i++
			}
		case "V":
			if i+1 < len(tokens) {
				y, err := strconv.ParseFloat(tokens[i+1], 64)
				if err == nil {
					curY = y
					points = append(points, [2]float64{curX, curY})
				}
				i += 2
			} else {
				i++
			}
		case "v":
			if i+1 < len(tokens) {
				dy, err := strconv.ParseFloat(tokens[i+1], 64)
				if err == nil {
					curY += dy
					points = append(points, [2]float64{curX, curY})
				}
				i += 2
			} else {
				i++
			}
		case "Z", "z":
			i++
		case "A", "a":
			// Arc: skip the 7 parameters
			if i+7 < len(tokens) {
				if tok == "A" {
					x, err1 := strconv.ParseFloat(tokens[i+6], 64)
					y, err2 := strconv.ParseFloat(tokens[i+7], 64)
					if err1 == nil && err2 == nil {
						curX, curY = x, y
						points = append(points, [2]float64{x, y})
					}
				} else {
					dx, err1 := strconv.ParseFloat(tokens[i+6], 64)
					dy, err2 := strconv.ParseFloat(tokens[i+7], 64)
					if err1 == nil && err2 == nil {
						curX += dx
						curY += dy
						points = append(points, [2]float64{curX, curY})
					}
				}
				i += 8
			} else {
				i++
			}
		default:
			i++
		}
	}
	return points
}

// buildArrowheadPath creates an SVG path element for an arrowhead at position (x,y)
// rotated by angle (radians), using a fixed reasonable size.
func buildArrowheadPath(x, y, angle float64, marker markerDef) string {
	// Use a fixed arrowhead size that looks correct across all diagram types.
	// The raw markerWidth/markerHeight from SVG markers can be enormous (190x240
	// for class diagrams) — those are meant for the marker coordinate system, not
	// the user-space coordinate system. A typical arrow tip is ~8px long, ~5px wide.
	size := 8.0
	halfH := 4.0

	// For markers with small userSpaceOnUse values, use them directly
	if marker.MarkerW > 0 && marker.MarkerW <= 12 {
		size = marker.MarkerW
	}
	if marker.MarkerH > 0 && marker.MarkerH <= 12 {
		halfH = marker.MarkerH / 2
	}

	// Triangle points in local coords (tip at origin, pointing right):
	// tip = (0, 0), left = (-size, -halfH), right = (-size, halfH)
	cos := math.Cos(angle)
	sin := math.Sin(angle)

	// Transform points
	tipX, tipY := x, y
	lx := x + (-size)*cos - (-halfH)*sin
	ly := y + (-size)*sin + (-halfH)*cos
	rx := x + (-size)*cos - halfH*sin
	ry := y + (-size)*sin + halfH*cos

	fill := marker.FillColor
	if fill == "" {
		fill = "#333333"
	}

	return fmt.Sprintf(`<path d="M %.2f %.2f L %.2f %.2f L %.2f %.2f Z" fill="%s" stroke="none"/>`,
		tipX, tipY, lx, ly, rx, ry, fill)
}

// inlineMarkers replaces marker-end/marker-start references with inline arrowhead paths.
// This is necessary because oksvg doesn't support SVG <marker> elements.
func inlineMarkers(svg string) string {
	markers := parseMarkerDefs(svg)
	if len(markers) == 0 {
		return svg
	}

	var arrowheads strings.Builder

	// Helper to extract stroke color from a tag
	extractStroke := func(tag string) string {
		strokeColor := getAttr(tag, "stroke")
		if strokeColor == "" || strokeColor == "none" {
			if style := getAttr(tag, "style"); style != "" {
				strokeColor = styleValue(style, "stroke")
			}
		}
		return strokeColor
	}

	// Process <path> elements with marker-end or marker-start
	svg = rePathWithMarker.ReplaceAllStringFunc(svg, func(pathTag string) string {
		d := getAttr(pathTag, "d")
		if d == "" {
			return pathTag
		}

		strokeColor := extractStroke(pathTag)

		// Handle marker-end
		if endMatch := reMarkerEnd.FindStringSubmatch(pathTag); len(endMatch) >= 2 {
			markerID := endMatch[1]
			if marker, ok := markers[markerID]; ok {
				x, y, angle, found := pathEndpoint(d, false)
				if found {
					m := marker
					if strokeColor != "" && strokeColor != "none" {
						m.FillColor = strokeColor
					}
					arrowheads.WriteString(buildArrowheadPath(x, y, angle, m))
				}
			}
		}

		// Handle marker-start
		if startMatch := reMarkerStart.FindStringSubmatch(pathTag); len(startMatch) >= 2 {
			markerID := startMatch[1]
			if marker, ok := markers[markerID]; ok {
				x, y, angle, found := pathEndpoint(d, true)
				if found {
					m := marker
					if strokeColor != "" && strokeColor != "none" {
						m.FillColor = strokeColor
					}
					arrowheads.WriteString(buildArrowheadPath(x, y, angle+math.Pi, m))
				}
			}
		}

		// Remove marker-end/marker-start attributes from the path
		result := reMarkerEnd.ReplaceAllString(pathTag, "")
		result = reMarkerStart.ReplaceAllString(result, "")
		return result
	})

	// Process <line> elements with marker-end or marker-start
	svg = reLineWithMarker.ReplaceAllStringFunc(svg, func(lineTag string) string {
		x1, _ := strconv.ParseFloat(getAttr(lineTag, "x1"), 64)
		y1, _ := strconv.ParseFloat(getAttr(lineTag, "y1"), 64)
		x2, _ := strconv.ParseFloat(getAttr(lineTag, "x2"), 64)
		y2, _ := strconv.ParseFloat(getAttr(lineTag, "y2"), 64)

		strokeColor := extractStroke(lineTag)

		dx := x2 - x1
		dy := y2 - y1
		angle := math.Atan2(dy, dx)

		// Handle marker-end
		if endMatch := reMarkerEnd.FindStringSubmatch(lineTag); len(endMatch) >= 2 {
			markerID := endMatch[1]
			if marker, ok := markers[markerID]; ok {
				m := marker
				if strokeColor != "" && strokeColor != "none" {
					m.FillColor = strokeColor
				}
				arrowheads.WriteString(buildArrowheadPath(x2, y2, angle, m))
			}
		}

		// Handle marker-start
		if startMatch := reMarkerStart.FindStringSubmatch(lineTag); len(startMatch) >= 2 {
			markerID := startMatch[1]
			if marker, ok := markers[markerID]; ok {
				m := marker
				if strokeColor != "" && strokeColor != "none" {
					m.FillColor = strokeColor
				}
				arrowheads.WriteString(buildArrowheadPath(x1, y1, angle+math.Pi, m))
			}
		}

		// Remove marker-end/marker-start attributes from the line
		result := reMarkerEnd.ReplaceAllString(lineTag, "")
		result = reMarkerStart.ReplaceAllString(result, "")
		return result
	})

	// Insert arrowhead paths before </svg>
	if arrowheads.Len() > 0 {
		closingIdx := strings.LastIndex(svg, "</svg>")
		if closingIdx >= 0 {
			svg = svg[:closingIdx] + arrowheads.String() + svg[closingIdx:]
		}
	}

	return svg
}
