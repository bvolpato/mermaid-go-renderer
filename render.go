package mermaid

import (
	"fmt"
	"html"
	"strings"
)

func RenderSVG(layout Layout, theme Theme, _ LayoutConfig) string {
	width := max(1.0, layout.Width)
	height := max(1.0, layout.Height)
	fontFamily := theme.FontFamily
	if fontFamily == "" {
		fontFamily = "sans-serif"
	}
	background := theme.Background
	if background == "" {
		background = "#ffffff"
	}

	var b strings.Builder
	b.Grow(4096)
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString("\n")
	b.WriteString(
		fmt.Sprintf(
			`<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s">`,
			formatFloat(width), formatFloat(height), formatFloat(width), formatFloat(height),
		),
	)
	b.WriteString("\n")
	b.WriteString("<defs>\n")
	b.WriteString(`<marker id="arrow-end" markerWidth="10" markerHeight="7" refX="8" refY="3.5" orient="auto" markerUnits="strokeWidth">`)
	b.WriteString(`<path d="M0,0 L10,3.5 L0,7 z" fill="`)
	b.WriteString(theme.LineColor)
	b.WriteString(`"/></marker>`)
	b.WriteString("\n")
	b.WriteString(`<marker id="arrow-start" markerWidth="10" markerHeight="7" refX="2" refY="3.5" orient="auto" markerUnits="strokeWidth">`)
	b.WriteString(`<path d="M10,0 L0,3.5 L10,7 z" fill="`)
	b.WriteString(theme.LineColor)
	b.WriteString(`"/></marker>`)
	b.WriteString("\n")
	b.WriteString("</defs>\n")

	b.WriteString(fmt.Sprintf(`<rect x="0" y="0" width="%s" height="%s" fill="%s"/>`,
		formatFloat(width), formatFloat(height), html.EscapeString(background)))
	b.WriteString("\n")

	for _, rect := range layout.Rects {
		b.WriteString(`<rect`)
		b.WriteString(fmt.Sprintf(` x="%s" y="%s" width="%s" height="%s"`,
			formatFloat(rect.X), formatFloat(rect.Y), formatFloat(rect.W), formatFloat(rect.H)))
		if rect.RX > 0 {
			b.WriteString(fmt.Sprintf(` rx="%s"`, formatFloat(rect.RX)))
		}
		if rect.RY > 0 {
			b.WriteString(fmt.Sprintf(` ry="%s"`, formatFloat(rect.RY)))
		}
		b.WriteString(` fill="` + html.EscapeString(defaultColor(rect.Fill, "none")) + `"`)
		b.WriteString(` stroke="` + html.EscapeString(defaultColor(rect.Stroke, "none")) + `"`)
		b.WriteString(fmt.Sprintf(` stroke-width="%s"`, formatFloat(defaultFloat(rect.StrokeWidth, 1))))
		if rect.Dashed {
			b.WriteString(` stroke-dasharray="5,4"`)
		}
		b.WriteString("/>\n")
	}

	for _, path := range layout.Paths {
		b.WriteString(`<path d="` + html.EscapeString(path.D) + `"`)
		b.WriteString(` fill="` + html.EscapeString(defaultColor(path.Fill, "none")) + `"`)
		b.WriteString(` stroke="` + html.EscapeString(defaultColor(path.Stroke, "none")) + `"`)
		b.WriteString(fmt.Sprintf(` stroke-width="%s"`, formatFloat(defaultFloat(path.StrokeWidth, 1))))
		b.WriteString("/>\n")
	}

	for _, poly := range layout.Polygons {
		parts := make([]string, 0, len(poly.Points))
		for _, point := range poly.Points {
			parts = append(parts, formatFloat(point.X)+","+formatFloat(point.Y))
		}
		b.WriteString(`<polygon points="` + strings.Join(parts, " ") + `"`)
		b.WriteString(` fill="` + html.EscapeString(defaultColor(poly.Fill, "none")) + `"`)
		b.WriteString(` stroke="` + html.EscapeString(defaultColor(poly.Stroke, "none")) + `"`)
		b.WriteString(fmt.Sprintf(` stroke-width="%s"`, formatFloat(defaultFloat(poly.StrokeWidth, 1))))
		b.WriteString("/>\n")
	}

	for _, line := range layout.Lines {
		b.WriteString(`<line`)
		b.WriteString(fmt.Sprintf(` x1="%s" y1="%s" x2="%s" y2="%s"`,
			formatFloat(line.X1), formatFloat(line.Y1), formatFloat(line.X2), formatFloat(line.Y2)))
		b.WriteString(` stroke="` + html.EscapeString(defaultColor(line.Stroke, "#333333")) + `"`)
		b.WriteString(fmt.Sprintf(` stroke-width="%s"`, formatFloat(defaultFloat(line.StrokeWidth, 1))))
		if line.Dashed {
			b.WriteString(` stroke-dasharray="5,4"`)
		}
		if line.ArrowStart {
			b.WriteString(` marker-start="url(#arrow-start)"`)
		}
		if line.ArrowEnd {
			b.WriteString(` marker-end="url(#arrow-end)"`)
		}
		b.WriteString("/>\n")
	}

	for _, circle := range layout.Circles {
		b.WriteString(`<circle`)
		b.WriteString(fmt.Sprintf(` cx="%s" cy="%s" r="%s"`,
			formatFloat(circle.CX), formatFloat(circle.CY), formatFloat(circle.R)))
		b.WriteString(` fill="` + html.EscapeString(defaultColor(circle.Fill, "none")) + `"`)
		b.WriteString(` stroke="` + html.EscapeString(defaultColor(circle.Stroke, "none")) + `"`)
		b.WriteString(fmt.Sprintf(` stroke-width="%s"`, formatFloat(defaultFloat(circle.StrokeWidth, 1))))
		b.WriteString("/>\n")
	}

	for _, text := range layout.Texts {
		anchor := text.Anchor
		if anchor == "" {
			anchor = "start"
		}
		size := text.Size
		if size <= 0 {
			size = 13
		}
		weight := text.Weight
		if weight == "" {
			weight = "400"
		}
		color := text.Color
		if color == "" {
			color = theme.PrimaryTextColor
		}
		b.WriteString(`<text`)
		b.WriteString(fmt.Sprintf(` x="%s" y="%s"`, formatFloat(text.X), formatFloat(text.Y)))
		b.WriteString(` text-anchor="` + html.EscapeString(anchor) + `"`)
		b.WriteString(` fill="` + html.EscapeString(defaultColor(color, "#1b263b")) + `"`)
		b.WriteString(` font-family="` + html.EscapeString(fontFamily) + `"`)
		b.WriteString(fmt.Sprintf(` font-size="%s"`, formatFloat(size)))
		b.WriteString(` font-weight="` + html.EscapeString(weight) + `"`)
		b.WriteString(`>`)
		b.WriteString(html.EscapeString(text.Value))
		b.WriteString("</text>\n")
	}

	b.WriteString("</svg>\n")
	return b.String()
}

func defaultColor(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func defaultFloat(value, fallback float64) float64 {
	if value == 0 {
		return fallback
	}
	return value
}
