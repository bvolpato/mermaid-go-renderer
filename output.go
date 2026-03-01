package mermaid

import (
	"bytes"
	"encoding/xml"
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
	return writeOutputPNG(svg, outputPath, 0, 0)
}

func WriteOutputPNGWithSize(svg string, outputPath string, width int, height int) error {
	return writeOutputPNG(svg, outputPath, width, height)
}

func writeOutputPNG(svg string, outputPath string, width int, height int) error {
	if width <= 0 || height <= 0 {
		width, height = detectSVGSize(svg)
	}
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
	return rasterizeSVGToImageResvg(svg, width, height)
}

func rasterizeSVGToImageLegacy(svg string, width int, height int) (*image.NRGBA, error) {
	icon, err := parseIconRobust(svg)
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

func parseIconRobust(svg string) (*oksvg.SvgIcon, error) {
	icon, err := oksvg.ReadIconStream(bytes.NewReader([]byte(svg)))
	if err == nil {
		return icon, nil
	}
	normalized := normalizeSVGForRasterizer(svg)
	if normalized != svg {
		icon, normalizedErr := oksvg.ReadIconStream(bytes.NewReader([]byte(normalized)))
		if normalizedErr == nil {
			return icon, nil
		}
	}
	withoutForeignObjects := stripSVGForeignObjects(normalized)
	if withoutForeignObjects == normalized {
		return nil, err
	}
	icon, foreignObjectErr := oksvg.ReadIconStream(bytes.NewReader([]byte(withoutForeignObjects)))
	if foreignObjectErr == nil {
		return icon, nil
	}
	return nil, err
}

var svgPathDataAttrPattern = regexp.MustCompile(`\bd\s*=\s*"([^"]*)"`)
var svgLineTagPattern = regexp.MustCompile(`<line\b[^>]*>`)
var svgMarkerElementPattern = regexp.MustCompile(`(?s)<marker\b[^>]*>.*?</marker>`)
var svgForeignObjectPatternForRaster = regexp.MustCompile(`(?s)<foreignObject\b[^>]*>.*?</foreignObject>`)
var svgRGBDecimalPattern = regexp.MustCompile(`rgb\(\s*([0-9]*\.?[0-9]+)\s*,\s*([0-9]*\.?[0-9]+)\s*,\s*([0-9]*\.?[0-9]+)\s*\)`)

func normalizeSVGForRasterizer(svg string) string {
	normalized := normalizeSVGPathData(svg)
	normalized = normalizeSVGLineAttrs(normalized)
	normalized = normalizeSVGCurrentColor(normalized)
	normalized = normalizeSVGTransparentColor(normalized)
	normalized = normalizeSVGRGBColors(normalized)
	normalized = stripSVGMarkerDefs(normalized)
	return normalized
}

func normalizeSVGPathData(svg string) string {
	return svgPathDataAttrPattern.ReplaceAllStringFunc(svg, func(attr string) string {
		match := svgPathDataAttrPattern.FindStringSubmatch(attr)
		if len(match) < 2 {
			return attr
		}
		normalized := normalizePathData(match[1])
		if normalized == match[1] {
			return attr
		}
		return `d="` + normalized + `"`
	})
}

func normalizeSVGLineAttrs(svg string) string {
	return svgLineTagPattern.ReplaceAllStringFunc(svg, func(tag string) string {
		trimmed := strings.TrimSpace(tag)
		selfClosing := strings.HasSuffix(trimmed, "/>")
		body := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSuffix(trimmed, "/>"), ">"), "<line")
		body = ensureSVGAttr(body, "x1", "0")
		body = ensureSVGAttr(body, "y1", "0")
		body = ensureSVGAttr(body, "x2", "0")
		body = ensureSVGAttr(body, "y2", "0")
		if selfClosing {
			return "<line" + body + "/>"
		}
		return "<line" + body + ">"
	})
}

func stripSVGMarkerDefs(svg string) string {
	return svgMarkerElementPattern.ReplaceAllString(svg, "")
}

func stripSVGForeignObjects(svg string) string {
	return svgForeignObjectPatternForRaster.ReplaceAllString(svg, "")
}

func normalizeSVGCurrentColor(svg string) string {
	normalized := strings.ReplaceAll(svg, `"currentColor"`, `"#000000"`)
	normalized = strings.ReplaceAll(normalized, `"currentcolor"`, `"#000000"`)
	return normalized
}

func normalizeSVGTransparentColor(svg string) string {
	normalized := strings.ReplaceAll(svg, `"transparent"`, `"none"`)
	normalized = strings.ReplaceAll(normalized, `"Transparent"`, `"none"`)
	return normalized
}

func normalizeSVGRGBColors(svg string) string {
	return svgRGBDecimalPattern.ReplaceAllStringFunc(svg, func(raw string) string {
		match := svgRGBDecimalPattern.FindStringSubmatch(raw)
		if len(match) != 4 {
			return raw
		}
		r, okR := parseAnyFloat(match[1])
		g, okG := parseAnyFloat(match[2])
		b, okB := parseAnyFloat(match[3])
		if !okR || !okG || !okB {
			return raw
		}
		return fmt.Sprintf("rgb(%d, %d, %d)", clampInt(int(math.Round(r)), 0, 255), clampInt(int(math.Round(g)), 0, 255), clampInt(int(math.Round(b)), 0, 255))
	})
}

func ensureSVGAttr(attrs string, name string, value string) string {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*=`)
	if pattern.MatchString(attrs) {
		return attrs
	}
	return attrs + ` ` + name + `="` + value + `"`
}

func normalizePathData(path string) string {
	buf := make([]byte, 0, len(path)+16)
	lastByte := func() byte {
		if len(buf) == 0 {
			return 0
		}
		return buf[len(buf)-1]
	}
	for i := 0; i < len(path); i++ {
		ch := path[i]
		switch {
		case ch == ',':
			buf = append(buf, ' ')
		case (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z'):
			if len(buf) > 0 {
				last := lastByte()
				if last != ' ' {
					buf = append(buf, ' ')
				}
			}
			buf = append(buf, ch, ' ')
		case ch == '-':
			if len(buf) > 0 {
				last := lastByte()
				if (last >= '0' && last <= '9' || last == '.') && last != 'e' && last != 'E' {
					buf = append(buf, ' ')
				}
			}
			buf = append(buf, ch)
		case ch == '+':
			if len(buf) > 0 {
				last := lastByte()
				if (last >= '0' && last <= '9' || last == '.') && last != 'e' && last != 'E' {
					buf = append(buf, ' ')
				}
			}
			buf = append(buf, ch)
		default:
			buf = append(buf, ch)
		}
	}
	return strings.Join(strings.Fields(string(buf)), " ")
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
	rootTag := parseRootSVGTag(svg)
	if rootTag == "" {
		return svgViewBox{}, false
	}
	re := regexp.MustCompile(`viewBox\s*=\s*"([^"]+)"`)
	match := re.FindStringSubmatch(rootTag)
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
	rootTag := parseRootSVGTag(svg)
	if rootTag == "" {
		return 0
	}
	re := regexp.MustCompile(name + `\s*=\s*"([^"]+)"`)
	match := re.FindStringSubmatch(rootTag)
	if len(match) < 2 {
		return 0
	}
	value, ok := parseDimensionValue(match[1])
	if !ok {
		return 0
	}
	return int(value + 0.5)
}

func parseRootSVGTag(svg string) string {
	re := regexp.MustCompile(`(?is)<svg\b[^>]*>`)
	return re.FindString(svg)
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
	svgTextElementPattern = regexp.MustCompile(`(?s)<text\b([^>]*)>(.*?)</text>`)
	svgTagPattern         = regexp.MustCompile(`(?s)<[^>]+>`)
	svgFontFaceCache      sync.Map
	svgTranslatePattern   = regexp.MustCompile(`translate\(\s*([+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?)\s*(?:[, ]\s*([+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?))?\s*\)`)
	svgMatrixPattern      = regexp.MustCompile(`matrix\(\s*[+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?[\s,]+[+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?[\s,]+[+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?[\s,]+[+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?[\s,]+([+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?)[\s,]+([+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?)\s*\)`)
)

type zenumlRasterMessage struct {
	From  string
	To    string
	Label string
}

var (
	zenumlTitlePattern       = regexp.MustCompile(`(?is)<div\s+class="title[^"]*"[^>]*>(.*?)</div>`)
	zenumlParticipantPattern = regexp.MustCompile(`data-participant-id="([^"]+)"`)
	zenumlMessagePattern     = regexp.MustCompile(`(?is)<div[^>]*\bdata-source="([^"]+)"[^>]*\bdata-target="([^"]+)"[^>]*\bdata-signature="([^"]*)"[^>]*>`)
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

	overlaySVGForeignObjectText(img, svg, width, height, viewBox, hasViewBox)
}

func overlaySVGForeignObjectText(img *image.NRGBA, svg string, width int, height int, viewBox svgViewBox, hasViewBox bool) {
	if !hasViewBox || viewBox.W <= 0 || viewBox.H <= 0 {
		viewBox = svgViewBox{X: 0, Y: 0, W: float64(width), H: float64(height)}
	}
	scaleX := float64(width) / viewBox.W
	scaleY := float64(height) / viewBox.H

	for _, label := range extractForeignObjectLabels(svg, viewBox) {
		face := resolveRasterFontFace(label.FontFamily, max(8.0, label.FontSize*scaleY))
		px := (label.X - viewBox.X) * scaleX
		py := (label.Y + label.H*0.8 - viewBox.Y) * scaleY
		textColor := parseTextColor(label.Color)
		if textColor == nil {
			textColor = autoContrastTextColor(img, int(math.Round(px)), int(math.Round(py)))
		}
		drawer := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(textColor),
			Face: face,
		}
		drawer.Dot = fixed.P(int(math.Round(px)), int(math.Round(py)))
		drawer.DrawString(label.Text)
	}
}

type foreignObjectLabel struct {
	X          float64
	Y          float64
	H          float64
	Text       string
	FontSize   float64
	FontFamily string
	Color      string
}

type foreignObjectCapture struct {
	BaseX      float64
	BaseY      float64
	X          float64
	Y          float64
	W          float64
	H          float64
	FontSize   float64
	FontFamily string
	Color      string
	Depth      int
	Text       strings.Builder
}

type svgTransformState struct {
	X float64
	Y float64
}

func extractForeignObjectLabels(svg string, viewBox svgViewBox) []foreignObjectLabel {
	decoder := xml.NewDecoder(strings.NewReader(svg))
	states := []svgTransformState{{X: 0, Y: 0}}
	labels := make([]foreignObjectLabel, 0, 32)
	var current *foreignObjectCapture

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := token.(type) {
		case xml.StartElement:
			parent := states[len(states)-1]
			tx, ty := parent.X, parent.Y
			if transform := xmlAttr(t.Attr, "transform"); transform != "" {
				dx, dy := parseTransformOffset(transform)
				tx += dx
				ty += dy
			}
			states = append(states, svgTransformState{X: tx, Y: ty})

			if strings.EqualFold(t.Name.Local, "foreignObject") {
				x, okX := parseDimensionValueWithPercent(xmlAttr(t.Attr, "x"), viewBox.W)
				if !okX {
					x = 0
				}
				y, okY := parseDimensionValueWithPercent(xmlAttr(t.Attr, "y"), viewBox.H)
				if !okY {
					y = 0
				}
				w, okW := parseDimensionValueWithPercent(xmlAttr(t.Attr, "width"), viewBox.W)
				h, okH := parseDimensionValueWithPercent(xmlAttr(t.Attr, "height"), viewBox.H)
				if !okW || !okH || w <= 0 || h <= 0 {
					current = nil
					continue
				}
				current = &foreignObjectCapture{
					BaseX:    tx,
					BaseY:    ty,
					X:        x,
					Y:        y,
					W:        w,
					H:        h,
					FontSize: 16,
					Depth:    1,
				}
				continue
			}

			if current != nil {
				current.Depth++
				updateCaptureStyle(current, t.Attr)
			}

		case xml.EndElement:
			if len(states) > 1 {
				states = states[:len(states)-1]
			}
			if current == nil {
				continue
			}
			current.Depth--
			if strings.EqualFold(t.Name.Local, "foreignObject") || current.Depth <= 0 {
				text := strings.Join(strings.Fields(current.Text.String()), " ")
				text = strings.TrimSpace(html.UnescapeString(text))
				if text != "" {
					labels = append(labels, foreignObjectLabel{
						X:          current.BaseX + current.X,
						Y:          current.BaseY + current.Y,
						H:          current.H,
						Text:       text,
						FontSize:   current.FontSize,
						FontFamily: current.FontFamily,
						Color:      current.Color,
					})
				}
				current = nil
			}

		case xml.CharData:
			if current != nil {
				current.Text.WriteString(" ")
				current.Text.Write([]byte(t))
			}
		}
	}
	return labels
}

func updateCaptureStyle(capture *foreignObjectCapture, attrs []xml.Attr) {
	if capture == nil {
		return
	}
	style := xmlAttr(attrs, "style")
	if style != "" {
		if v := styleValue(style, "font-size"); v != "" {
			if parsed, ok := parseDimensionValue(v); ok {
				capture.FontSize = parsed
			}
		}
		if v := styleValue(style, "font-family"); v != "" && capture.FontFamily == "" {
			capture.FontFamily = v
		}
		if v := styleValue(style, "color"); v != "" && capture.Color == "" {
			capture.Color = v
		}
	}
	if size := xmlAttr(attrs, "font-size"); size != "" {
		if parsed, ok := parseDimensionValue(size); ok {
			capture.FontSize = parsed
		}
	}
	if family := xmlAttr(attrs, "font-family"); family != "" && capture.FontFamily == "" {
		capture.FontFamily = family
	}
	if col := xmlAttr(attrs, "color"); col != "" && capture.Color == "" {
		capture.Color = col
	}
	if fill := xmlAttr(attrs, "fill"); fill != "" && capture.Color == "" {
		capture.Color = fill
	}
}

func xmlAttr(attrs []xml.Attr, key string) string {
	for _, attr := range attrs {
		if strings.EqualFold(attr.Name.Local, key) {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

func parseDimensionValueWithPercent(raw string, reference float64) (float64, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, false
	}
	if strings.HasSuffix(value, "%") {
		if reference <= 0 {
			return 0, false
		}
		percent, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(value, "%")), 64)
		if err != nil {
			return 0, false
		}
		return reference * percent / 100.0, true
	}
	return parseAnyFloat(firstNumericToken(value))
}

func parseTransformOffset(transform string) (float64, float64) {
	tx := 0.0
	ty := 0.0
	for _, match := range svgTranslatePattern.FindAllStringSubmatch(transform, -1) {
		if len(match) < 2 {
			continue
		}
		if x, ok := parseAnyFloat(match[1]); ok {
			tx += x
		}
		if len(match) >= 3 && strings.TrimSpace(match[2]) != "" {
			if y, ok := parseAnyFloat(match[2]); ok {
				ty += y
			}
		}
	}
	for _, match := range svgMatrixPattern.FindAllStringSubmatch(transform, -1) {
		if len(match) < 3 {
			continue
		}
		if x, ok := parseAnyFloat(match[1]); ok {
			tx += x
		}
		if y, ok := parseAnyFloat(match[2]); ok {
			ty += y
		}
	}
	return tx, ty
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

func autoContrastTextColor(img *image.NRGBA, x int, y int) color.Color {
	if img == nil {
		return color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	}
	bounds := img.Bounds()
	if x < bounds.Min.X {
		x = bounds.Min.X
	}
	if x >= bounds.Max.X {
		x = bounds.Max.X - 1
	}
	if y < bounds.Min.Y {
		y = bounds.Min.Y
	}
	if y >= bounds.Max.Y {
		y = bounds.Max.Y - 1
	}
	offset := img.PixOffset(x, y)
	r := float64(img.Pix[offset])
	g := float64(img.Pix[offset+1])
	b := float64(img.Pix[offset+2])
	luma := 0.2126*r + 0.7152*g + 0.0722*b
	if luma < 128 {
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}
	return color.NRGBA{R: 0, G: 0, B: 0, A: 255}
}

func isZenUMLSVG(svg string) bool {
	lowerSVG := strings.ToLower(svg)
	return strings.Contains(lowerSVG, `aria-roledescription="zenuml"`) ||
		(strings.Contains(lowerSVG, "<foreignobject") && strings.Contains(lowerSVG, "zenuml"))
}

func imageNonWhitePixels(img *image.NRGBA) int {
	if img == nil {
		return 0
	}
	count := 0
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			o := img.PixOffset(x, y)
			if !(img.Pix[o] > 245 && img.Pix[o+1] > 245 && img.Pix[o+2] > 245) {
				count++
			}
		}
	}
	return count
}

func buildZenUMLRasterFallbackSVG(sourceSVG string, width int, height int) (string, bool) {
	title := "ZenUML"
	if match := zenumlTitlePattern.FindStringSubmatch(sourceSVG); len(match) >= 2 {
		extracted := extractTextContent(match[1])
		if strings.TrimSpace(extracted) != "" {
			title = extracted
		}
	}

	participants := make([]string, 0, 8)
	seenParticipants := map[string]bool{}
	for _, match := range zenumlParticipantPattern.FindAllStringSubmatch(sourceSVG, -1) {
		if len(match) < 2 {
			continue
		}
		name := strings.TrimSpace(html.UnescapeString(match[1]))
		if name == "" || seenParticipants[name] {
			continue
		}
		seenParticipants[name] = true
		participants = append(participants, name)
	}

	messages := make([]zenumlRasterMessage, 0, 16)
	for _, match := range zenumlMessagePattern.FindAllStringSubmatch(sourceSVG, -1) {
		if len(match) < 4 {
			continue
		}
		from := strings.TrimSpace(html.UnescapeString(match[1]))
		to := strings.TrimSpace(html.UnescapeString(match[2]))
		label := strings.TrimSpace(html.UnescapeString(match[3]))
		if from == "" || to == "" {
			continue
		}
		if !seenParticipants[from] {
			seenParticipants[from] = true
			participants = append(participants, from)
		}
		if !seenParticipants[to] {
			seenParticipants[to] = true
			participants = append(participants, to)
		}
		messages = append(messages, zenumlRasterMessage{
			From:  from,
			To:    to,
			Label: label,
		})
	}
	if len(participants) == 0 {
		return "", false
	}
	if len(messages) == 0 {
		// Keep fallback deterministic: render participants even if no messages.
		messages = append(messages, zenumlRasterMessage{
			From:  participants[0],
			To:    participants[0],
			Label: "",
		})
	}

	leftPad := 70.0
	rightPad := 70.0
	topY := 36.0
	headW := 120.0
	headH := 34.0
	lifelineY := topY + headH
	firstMessageY := lifelineY + 46
	stepY := 44.0

	xByParticipant := map[string]float64{}
	if len(participants) == 1 {
		xByParticipant[participants[0]] = float64(width) / 2
	} else {
		spacing := (float64(width) - leftPad - rightPad) / float64(len(participants)-1)
		spacing = max(80, spacing)
		headW = min(headW, spacing*0.78)
		for i, participant := range participants {
			xByParticipant[participant] = leftPad + float64(i)*spacing
		}
	}

	lastMessageY := firstMessageY + float64(max(1, len(messages)-1))*stepY
	lifelineEndY := min(float64(height)-26, lastMessageY+58)
	if lifelineEndY < lifelineY+20 {
		lifelineEndY = lifelineY + 20
	}

	var b strings.Builder
	b.Grow(8192)
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="`)
	b.WriteString(intString(width))
	b.WriteString(`" height="`)
	b.WriteString(intString(height))
	b.WriteString(`" viewBox="0 0 `)
	b.WriteString(intString(width))
	b.WriteString(` `)
	b.WriteString(intString(height))
	b.WriteString(`">`)
	b.WriteString(`<rect x="0" y="0" width="`)
	b.WriteString(intString(width))
	b.WriteString(`" height="`)
	b.WriteString(intString(height))
	b.WriteString(`" fill="#ffffff"/>`)
	b.WriteString(`<defs><marker id="zenuml-fallback-arrow" refX="9" refY="5" markerWidth="10" markerHeight="10" orient="auto"><path d="M0,0 L10,5 L0,10 z" fill="#333"/></marker></defs>`)
	b.WriteString(`<text x="18" y="24" fill="#1f2937" font-size="18" font-family="Trebuchet MS, Verdana, Arial, sans-serif" font-weight="600">`)
	b.WriteString(html.EscapeString(title))
	b.WriteString(`</text>`)

	for _, participant := range participants {
		x := xByParticipant[participant]
		b.WriteString(`<rect x="`)
		b.WriteString(formatFloat(x - headW/2))
		b.WriteString(`" y="`)
		b.WriteString(formatFloat(topY))
		b.WriteString(`" width="`)
		b.WriteString(formatFloat(headW))
		b.WriteString(`" height="`)
		b.WriteString(formatFloat(headH))
		b.WriteString(`" rx="6" ry="6" fill="#eaeaea" stroke="#666" stroke-width="1.3"/>`)
		b.WriteString(`<text x="`)
		b.WriteString(formatFloat(x))
		b.WriteString(`" y="`)
		b.WriteString(formatFloat(topY + headH/2 + 5))
		b.WriteString(`" fill="#111827" font-size="14" text-anchor="middle" font-family="Trebuchet MS, Verdana, Arial, sans-serif">`)
		b.WriteString(html.EscapeString(participant))
		b.WriteString(`</text>`)
		b.WriteString(`<line x1="`)
		b.WriteString(formatFloat(x))
		b.WriteString(`" y1="`)
		b.WriteString(formatFloat(lifelineY))
		b.WriteString(`" x2="`)
		b.WriteString(formatFloat(x))
		b.WriteString(`" y2="`)
		b.WriteString(formatFloat(lifelineEndY))
		b.WriteString(`" stroke="#999" stroke-width="1" stroke-dasharray="3,3"/>`)
	}

	for i, msg := range messages {
		fromX, okFrom := xByParticipant[msg.From]
		toX, okTo := xByParticipant[msg.To]
		if !okFrom || !okTo {
			continue
		}
		y := firstMessageY + float64(i)*stepY
		if fromX == toX {
			loopX := fromX + 48
			b.WriteString(`<path d="M`)
			b.WriteString(formatFloat(fromX))
			b.WriteString(` `)
			b.WriteString(formatFloat(y))
			b.WriteString(` C `)
			b.WriteString(formatFloat(loopX))
			b.WriteString(` `)
			b.WriteString(formatFloat(y - 10))
			b.WriteString(`, `)
			b.WriteString(formatFloat(loopX))
			b.WriteString(` `)
			b.WriteString(formatFloat(y + 22))
			b.WriteString(`, `)
			b.WriteString(formatFloat(fromX))
			b.WriteString(` `)
			b.WriteString(formatFloat(y + 12))
			b.WriteString(`" fill="none" stroke="#333" stroke-width="1.8" marker-end="url(#zenuml-fallback-arrow)"/>`)
			b.WriteString(`<text x="`)
			b.WriteString(formatFloat(fromX + 26))
			b.WriteString(`" y="`)
			b.WriteString(formatFloat(y - 8))
			b.WriteString(`" fill="#111827" font-size="13" text-anchor="middle" font-family="Trebuchet MS, Verdana, Arial, sans-serif">`)
			b.WriteString(html.EscapeString(msg.Label))
			b.WriteString(`</text>`)
			continue
		}
		b.WriteString(`<line x1="`)
		b.WriteString(formatFloat(fromX))
		b.WriteString(`" y1="`)
		b.WriteString(formatFloat(y))
		b.WriteString(`" x2="`)
		b.WriteString(formatFloat(toX))
		b.WriteString(`" y2="`)
		b.WriteString(formatFloat(y))
		b.WriteString(`" stroke="#333" stroke-width="1.8" marker-end="url(#zenuml-fallback-arrow)"/>`)
		labelX := (fromX + toX) / 2
		b.WriteString(`<text x="`)
		b.WriteString(formatFloat(labelX))
		b.WriteString(`" y="`)
		b.WriteString(formatFloat(y - 8))
		b.WriteString(`" fill="#111827" font-size="13" text-anchor="middle" font-family="Trebuchet MS, Verdana, Arial, sans-serif">`)
		b.WriteString(html.EscapeString(msg.Label))
		b.WriteString(`</text>`)
	}
	b.WriteString(`</svg>`)
	return b.String(), true
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
