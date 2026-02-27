package mermaid

import (
	"bytes"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func WriteOutputSVG(svg string, outputPath string) error {
	if outputPath == "" {
		_, err := os.Stdout.WriteString(svg)
		return err
	}
	return os.WriteFile(outputPath, []byte(svg), 0o644)
}

func WriteOutputPNG(svg string, outputPath string) error {
	width, height := detectSVGSize(svg)
	img, err := rasterizeSVGToImage(svg, width, height)
	if err != nil {
		return err
	}
	if outputPath == "" {
		return png.Encode(os.Stdout, img)
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}

type svgViewBox struct {
	X float64
	Y float64
	W float64
	H float64
}

func rasterizeSVGToImage(svg string, width int, height int) (*image.NRGBA, error) {
	icon, err := oksvg.ReadIconStream(bytes.NewReader([]byte(svg)))
	if err != nil {
		return nil, fmt.Errorf("parse svg: %w", err)
	}
	viewBox, hasViewBox := parseSVGViewBox(svg)
	icon.SetTarget(0, 0, float64(width), float64(height))

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	scanner := rasterx.NewScannerGV(width, height, img, img.Bounds())
	dasher := rasterx.NewDasher(width, height, scanner)
	icon.Draw(dasher, 1.0)
	overlaySVGText(img, svg, width, height, viewBox, hasViewBox)
	return img, nil
}

func detectSVGSize(svg string) (int, int) {
	const (
		defaultWidth  = 1200
		defaultHeight = 800
	)
	viewBox, hasViewBox := parseSVGViewBox(svg)
	width := parseSVGDimensionAttr(svg, "width")
	height := parseSVGDimensionAttr(svg, "height")

	if width <= 0 && hasViewBox && viewBox.W > 0 {
		width = int(viewBox.W + 0.5)
	}
	if height <= 0 && hasViewBox && viewBox.H > 0 {
		height = int(viewBox.H + 0.5)
	}
	if hasViewBox && (viewBox.X < 0 || viewBox.Y < 0) {
		width += 16
		height += 16
	}
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}
	return width, height
}

func parseSVGViewBox(svg string) (svgViewBox, bool) {
	re := regexp.MustCompile(`viewBox\s*=\s*"([^"]+)"`)
	match := re.FindStringSubmatch(svg)
	if len(match) < 2 {
		return svgViewBox{}, false
	}
	parts := strings.Fields(match[1])
	if len(parts) != 4 {
		return svgViewBox{}, false
	}
	x, okX := parseAnyFloat(parts[0])
	y, okY := parseAnyFloat(parts[1])
	w, okW := parseAnyFloat(parts[2])
	h, okH := parseAnyFloat(parts[3])
	if !okX || !okY || !okW || !okH || w <= 0 || h <= 0 {
		return svgViewBox{}, false
	}
	return svgViewBox{X: x, Y: y, W: w, H: h}, true
}

func parseSVGViewBoxSize(svg string) (int, int) {
	viewBox, ok := parseSVGViewBox(svg)
	if !ok {
		return 0, 0
	}
	return int(viewBox.W + 0.5), int(viewBox.H + 0.5)
}

func parseSVGDimensionAttr(svg string, name string) int {
	re := regexp.MustCompile(name + `\s*=\s*"([^"]+)"`)
	match := re.FindStringSubmatch(svg)
	if len(match) < 2 {
		return 0
	}
	value, ok := parseDimensionValue(match[1])
	if !ok {
		return 0
	}
	return int(value + 0.5)
}

func parseDimensionValue(raw string) (float64, bool) {
	value := strings.TrimSpace(strings.TrimSuffix(raw, "px"))
	if value == "" || strings.HasSuffix(value, "%") {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}

func parseAnyFloat(raw string) (float64, bool) {
	value := strings.TrimSpace(strings.TrimSuffix(raw, "px"))
	if value == "" || strings.HasSuffix(value, "%") {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

var (
	svgTextElementPattern   = regexp.MustCompile(`(?s)<text\b([^>]*)>(.*?)</text>`)
	svgForeignObjectPattern = regexp.MustCompile(`(?s)<foreignObject\b([^>]*)>(.*?)</foreignObject>`)
	svgTagPattern           = regexp.MustCompile(`(?s)<[^>]+>`)
	svgFontFaceCache        sync.Map
)

func overlaySVGText(img *image.NRGBA, svg string, width int, height int, viewBox svgViewBox, hasViewBox bool) {
	if !hasViewBox || viewBox.W <= 0 || viewBox.H <= 0 {
		viewBox = svgViewBox{X: 0, Y: 0, W: float64(width), H: float64(height)}
	}
	scaleX := float64(width) / viewBox.W
	scaleY := float64(height) / viewBox.H

	matches := svgTextElementPattern.FindAllStringSubmatch(svg, -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		attrs := match[1]
		content := extractTextContent(match[2])
		if strings.TrimSpace(content) == "" {
			continue
		}

		rawX := firstNumericToken(parseAttr(attrs, "x"))
		rawY := firstNumericToken(parseAttr(attrs, "y"))
		x, okX := parseAnyFloat(rawX)
		y, okY := parseAnyFloat(rawY)
		if !okX || !okY {
			continue
		}

		fontSize := 16.0
		if rawSize := parseAttr(attrs, "font-size"); rawSize != "" {
			if size, ok := parseDimensionValue(rawSize); ok {
				fontSize = size
			}
		}
		fontFamily := parseAttr(attrs, "font-family")
		face := resolveRasterFontFace(fontFamily, max(8.0, fontSize*scaleY))
		textColor := parseTextColor(parseAttr(attrs, "fill"))
		if textColor == nil {
			textColor = color.NRGBA{R: 0, G: 0, B: 0, A: 255}
		}

		drawer := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(textColor),
			Face: face,
		}
		advance := drawer.MeasureString(content)
		px := (x - viewBox.X) * scaleX
		py := (y - viewBox.Y) * scaleY
		anchor := strings.TrimSpace(parseAttr(attrs, "text-anchor"))
		switch anchor {
		case "middle":
			px -= float64(advance) / 128.0
		case "end":
			px -= float64(advance) / 64.0
		}
		if strings.TrimSpace(parseAttr(attrs, "dominant-baseline")) == "middle" {
			metrics := face.Metrics()
			px = math.Round(px)
			py += float64(metrics.Ascent+metrics.Descent) / 128.0
		}
		// Rotated labels are rare and tiny in our fixtures; skip them for now.
		if strings.Contains(strings.ToLower(parseAttr(attrs, "transform")), "rotate(") {
			continue
		}

		drawer.Dot = fixed.P(int(math.Round(px)), int(math.Round(py)))
		drawer.DrawString(content)
	}

	foMatches := svgForeignObjectPattern.FindAllStringSubmatch(svg, -1)
	for _, match := range foMatches {
		if len(match) < 3 {
			continue
		}
		attrs := match[1]
		content := extractTextContent(match[2])
		if strings.TrimSpace(content) == "" {
			continue
		}
		x, okX := parseAnyFloat(firstNumericToken(parseAttr(attrs, "x")))
		y, okY := parseAnyFloat(firstNumericToken(parseAttr(attrs, "y")))
		w, okW := parseAnyFloat(firstNumericToken(parseAttr(attrs, "width")))
		h, okH := parseAnyFloat(firstNumericToken(parseAttr(attrs, "height")))
		if !okX || !okY || !okW || !okH || w <= 0 || h <= 0 {
			continue
		}

		fontSize := 16.0
		fontFamily := ""
		colorAttr := ""
		inlineStyle := firstStyleAttr(match[2])
		if inlineStyle != "" {
			if v := styleValue(inlineStyle, "font-size"); v != "" {
				if parsed, ok := parseDimensionValue(v); ok {
					fontSize = parsed
				}
			}
			if v := styleValue(inlineStyle, "font-family"); v != "" {
				fontFamily = v
			}
			if v := styleValue(inlineStyle, "color"); v != "" {
				colorAttr = v
			}
		}
		face := resolveRasterFontFace(fontFamily, max(8.0, fontSize*scaleY))
		textColor := parseTextColor(colorAttr)
		if textColor == nil {
			textColor = color.NRGBA{R: 0, G: 0, B: 0, A: 255}
		}
		drawer := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(textColor),
			Face: face,
		}
		px := (x - viewBox.X) * scaleX
		py := (y + h*0.8 - viewBox.Y) * scaleY
		drawer.Dot = fixed.P(int(math.Round(px)), int(math.Round(py)))
		drawer.DrawString(content)
	}
}

func extractTextContent(input string) string {
	value := html.UnescapeString(input)
	value = svgTagPattern.ReplaceAllString(value, "")
	value = strings.Join(strings.Fields(value), " ")
	return strings.TrimSpace(value)
}

func parseAttr(attrs string, name string) string {
	pattern := regexp.MustCompile(name + `\s*=\s*"([^"]*)"`)
	match := pattern.FindStringSubmatch(attrs)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(match[1]))
}

func firstNumericToken(raw string) string {
	parts := strings.Fields(strings.ReplaceAll(raw, ",", " "))
	if len(parts) == 0 {
		return raw
	}
	return parts[0]
}

func firstStyleAttr(raw string) string {
	pattern := regexp.MustCompile(`style\s*=\s*"([^"]*)"`)
	match := pattern.FindStringSubmatch(raw)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(match[1]))
}

func styleValue(style string, key string) string {
	for _, chunk := range strings.Split(style, ";") {
		parts := strings.SplitN(chunk, ":", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(strings.ToLower(parts[0]))
		if k != strings.ToLower(strings.TrimSpace(key)) {
			continue
		}
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func parseTextColor(raw string) color.Color {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" || value == "none" {
		return nil
	}
	if strings.HasPrefix(value, "#") {
		hex := strings.TrimPrefix(value, "#")
		if len(hex) == 3 {
			r, errR := strconv.ParseUint(strings.Repeat(string(hex[0]), 2), 16, 8)
			g, errG := strconv.ParseUint(strings.Repeat(string(hex[1]), 2), 16, 8)
			b, errB := strconv.ParseUint(strings.Repeat(string(hex[2]), 2), 16, 8)
			if errR == nil && errG == nil && errB == nil {
				return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
			}
		}
		if len(hex) == 6 {
			r, errR := strconv.ParseUint(hex[0:2], 16, 8)
			g, errG := strconv.ParseUint(hex[2:4], 16, 8)
			b, errB := strconv.ParseUint(hex[4:6], 16, 8)
			if errR == nil && errG == nil && errB == nil {
				return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
			}
		}
	}
	if strings.HasPrefix(value, "rgb(") && strings.HasSuffix(value, ")") {
		chunks := strings.Split(strings.TrimSuffix(strings.TrimPrefix(value, "rgb("), ")"), ",")
		if len(chunks) == 3 {
			r, errR := strconv.Atoi(strings.TrimSpace(chunks[0]))
			g, errG := strconv.Atoi(strings.TrimSpace(chunks[1]))
			b, errB := strconv.Atoi(strings.TrimSpace(chunks[2]))
			if errR == nil && errG == nil && errB == nil {
				return color.NRGBA{
					R: uint8(clampInt(r, 0, 255)),
					G: uint8(clampInt(g, 0, 255)),
					B: uint8(clampInt(b, 0, 255)),
					A: 255,
				}
			}
		}
	}
	return color.NRGBA{R: 0, G: 0, B: 0, A: 255}
}

func resolveRasterFontFace(fontFamily string, fontSize float64) font.Face {
	path := resolveFontPath(fontFamily)
	if path == "" {
		path = resolveFontPath(defaultMetricFontFamily)
	}
	key := path + "|" + formatFloat(fontSize)
	if cached, ok := svgFontFaceCache.Load(key); ok {
		if face, okFace := cached.(font.Face); okFace {
			return face
		}
	}
	if path != "" {
		if faceData := loadFontFace(path); faceData != nil {
			face, err := opentype.NewFace(faceData, &opentype.FaceOptions{
				Size:    fontSize,
				DPI:     72,
				Hinting: font.HintingNone,
			})
			if err == nil {
				svgFontFaceCache.Store(key, face)
				return face
			}
		}
	}
	return basicfont.Face7x13
}

func clampInt(v int, lo int, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
