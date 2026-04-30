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
	"io"
	"math"
	"os"
	"regexp"
	"sort"
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

// WritePNGFromSource renders a Mermaid diagram to PNG using the pure-Go renderer.
// It parses the Mermaid code, generates SVG, and rasterizes it to PNG.
func WritePNGFromSource(mermaidCode string, outputPath string) error {
	if strings.TrimSpace(mermaidCode) == "" {
		return fmt.Errorf("mermaid code is empty")
	}
	if _, parseErr := ParseMermaid(mermaidCode); parseErr != nil {
		return parseErr
	}
	svg, err := RenderWithOptions(mermaidCode, DefaultRenderOptions())
	if err != nil {
		return err
	}
	return writeOutputPNG(svg, outputPath, 0, 0)
}

// WritePNGFromSourceWithFallback is an alias for WritePNGFromSource.
// Deprecated: Use WritePNGFromSource directly.
func WritePNGFromSourceWithFallback(mermaidCode string, outputPath string) error {
	return WritePNGFromSource(mermaidCode, outputPath)
}

// HasBrowser is deprecated and always returns false.
// The library no longer requires or uses a browser for rendering.
func HasBrowser() bool {
	return false
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

type svgRasterTransform struct {
	Scale   float64
	TargetX float64
	TargetY float64
	TargetW float64
	TargetH float64
}

func computeSVGRasterTransform(width int, height int, viewBox svgViewBox) svgRasterTransform {
	if viewBox.W <= 0 || viewBox.H <= 0 {
		return svgRasterTransform{
			Scale:   1,
			TargetX: 0,
			TargetY: 0,
			TargetW: float64(width),
			TargetH: float64(height),
		}
	}
	scale := math.Min(float64(width)/viewBox.W, float64(height)/viewBox.H)
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		scale = 1
	}
	targetW := viewBox.W * scale
	targetH := viewBox.H * scale
	return svgRasterTransform{
		Scale:   scale,
		TargetX: (float64(width) - targetW) / 2.0,
		TargetY: (float64(height) - targetH) / 2.0,
		TargetW: targetW,
		TargetH: targetH,
	}
}

func (t svgRasterTransform) mapX(x float64, viewBox svgViewBox) float64 {
	return t.TargetX + (x-viewBox.X)*t.Scale
}

func (t svgRasterTransform) mapY(y float64, viewBox svgViewBox) float64 {
	return t.TargetY + (y-viewBox.Y)*t.Scale
}

func rasterizeSVGToImage(svg string, width int, height int) (*image.NRGBA, error) {
	prepared := prepareSVGForRasterizer(svg)
	textOverlaySource := prepared
	rasterSVG := prepared
	if shouldStripSVGTextForRasterizer(prepared) {
		rasterSVG = stripSVGTextElements(prepared)
	}
	icon, err := parseIconRobust(rasterSVG)
	if err != nil {
		return nil, fmt.Errorf("parse svg: %w", err)
	}
	viewBox, hasViewBox := parseSVGViewBox(prepared)
	if !hasViewBox || viewBox.W <= 0 || viewBox.H <= 0 {
		viewBox = svgViewBox{X: 0, Y: 0, W: float64(width), H: float64(height)}
	}
	transform := computeSVGRasterTransform(width, height, viewBox)
	icon.SetTarget(transform.TargetX, transform.TargetY, transform.TargetW, transform.TargetH)

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	scanner := rasterx.NewScannerGV(width, height, img, img.Bounds())
	dasher := rasterx.NewDasher(width, height, scanner)
	icon.Draw(dasher, 1.0)
	overlaySVGText(img, textOverlaySource, width, height, viewBox, hasViewBox)
	return img, nil
}

// prepareSVGForRasterizer transforms SVG to be oksvg-compatible:
//   - Replaces percentage/missing width/height with absolute pixel values from viewBox
//   - Expands viewBox to encompass all content (e.g. cluster labels above y=0)
//   - Inlines marker arrowheads as real SVG paths (oksvg doesn't support <marker>)
//   - Strips <foreignObject> blocks (text is overlaid separately)
func prepareSVGForRasterizer(svg string) string {
	if !skipViewBoxExpansion(svg) {
		svg = expandViewBoxToContent(svg)
	}
	svg = fixSVGRootDimensions(svg)
	svg = convertHSLToHex(svg)
	svg = inlineMarkers(svg)
	svg = stripSVGForeignObjectSwitches(svg)
	svg = normalizeFontsForRasterizer(svg)
	svg = stripClipPaths(svg)
	svg = inlineCSS(svg)
	svg = regexp.MustCompile(`(?i)rotate\(\s*([\d\.-]+)\s*deg\s*\)`).ReplaceAllString(svg, "rotate(${1})")
	return svg
}

func skipViewBoxExpansion(svg string) bool {
	return strings.Contains(svg, `aria-roledescription="mindmap"`) ||
		strings.Contains(svg, `class="mindmapDiagram"`) ||
		strings.Contains(svg, `aria-roledescription="kanban"`) ||
		strings.Contains(svg, `aria-roledescription="gitGraph"`) ||
		strings.Contains(svg, `aria-roledescription="radar"`)
}

func normalizeFontsForRasterizer(svg string) string {
	svg = strings.ReplaceAll(svg, `&quot;Open Sans&quot;, sans-serif`, `'trebuchet ms', verdana, arial, sans-serif`)
	svg = strings.ReplaceAll(svg, `"Open Sans", sans-serif`, `'trebuchet ms', verdana, arial, sans-serif`)
	return svg
}

func shouldStripSVGTextForRasterizer(svg string) bool {
	return strings.Contains(svg, `aria-roledescription="gantt"`)
}

func stripSVGTextElements(svg string) string {
	return svgTextElementPattern.ReplaceAllString(svg, "")
}

var hslFloatPattern = regexp.MustCompile(`(?i)hsl\(\s*([\d\.]+)\s*,\s*([\d\.]+)%\s*,\s*([\d\.]+)%\s*\)`)

func convertHSLToHex(svg string) string {
	return hslFloatPattern.ReplaceAllStringFunc(svg, func(match string) string {
		parts := hslFloatPattern.FindStringSubmatch(match)
		if len(parts) == 4 {
			hue, _ := strconv.ParseFloat(parts[1], 64)
			sat, _ := strconv.ParseFloat(parts[2], 64)
			lit, _ := strconv.ParseFloat(parts[3], 64)
			r, g, b := hslToRGB(hue, sat, lit)
			return "#" + hexByte(int(math.Round(r*255))) + hexByte(int(math.Round(g*255))) + hexByte(int(math.Round(b*255)))
		}
		return match
	})
}

var cssRulePattern = regexp.MustCompile(`([^\{\}]+)\{([^}]+)\}`)
var styleTagPattern = regexp.MustCompile(`(?is)<style[^>]*>(.*?)</style>`)
var elementTagPattern = regexp.MustCompile(`(?is)<([a-zA-Z0-9]+)\s+([^>]+)/*>`)

type styleRule struct {
	selector string
	styles   stylesMap
	parts    []cssSelectorPart
}

type cssSelectorPart struct {
	tag     string
	id      string
	classes []string
}

type elementContext struct {
	tag     string
	id      string
	classes []string
}

func parseCSS(svg string) []styleRule {
	var rules []styleRule
	matches := styleTagPattern.FindAllStringSubmatch(svg, -1)
	for _, m := range matches {
		css := m[1]
		ruleMatches := cssRulePattern.FindAllStringSubmatch(css, -1)
		for _, rm := range ruleMatches {
			selectors := strings.Split(rm[1], ",")
			styles := strings.TrimSpace(rm[2])
			if !strings.HasSuffix(styles, ";") {
				styles += ";"
			}
			for _, sel := range selectors {
				s := strings.TrimSpace(sel)
				s = strings.TrimPrefix(s, "#my-svg ")
				parts, ok := parseSelectorParts(s)
				if s != "" && ok {
					rules = append(rules, styleRule{
						selector: s,
						styles:   parseStylesMap(styles),
						parts:    parts,
					})
				}
			}
		}
	}
	return rules
}

func inlineCSS(svg string) string {
	rules := parseCSS(svg)
	if len(rules) == 0 {
		return svg
	}
	decoder := xml.NewDecoder(strings.NewReader(svg))
	var out bytes.Buffer
	encoder := xml.NewEncoder(&out)
	contextStack := make([]elementContext, 0, 32)

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return svg
		}

		switch t := tok.(type) {
		case xml.StartElement:
			ctx := elementContext{
				tag:     t.Name.Local,
				id:      attrValue(t.Attr, "id"),
				classes: strings.Fields(attrValue(t.Attr, "class")),
			}
			t.Attr = inlineElementStyles(t.Attr, ctx, contextStack, rules)
			if err := encoder.EncodeToken(t); err != nil {
				return svg
			}
			contextStack = append(contextStack, ctx)
		case xml.EndElement:
			if err := encoder.EncodeToken(t); err != nil {
				return svg
			}
			if len(contextStack) > 0 {
				contextStack = contextStack[:len(contextStack)-1]
			}
		default:
			if err := encoder.EncodeToken(tok); err != nil {
				return svg
			}
		}
	}
	if err := encoder.Flush(); err != nil {
		return svg
	}
	return cleanupInlinedSVG(out.String())
}

type stylesMap map[string]string

func (s stylesMap) String() string {
	var sb strings.Builder
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := s[k]
		sb.WriteString(k + ":" + v + ";")
	}
	return strings.ReplaceAll(sb.String(), "\"", "'")
}

func parseStylesMap(s string) stylesMap {
	m := make(stylesMap)
	parts := strings.Split(s, ";")
	for _, p := range parts {
		kv := strings.SplitN(p, ":", 2)
		if len(kv) == 2 {
			value := strings.TrimSpace(strings.ReplaceAll(kv[1], "!important", ""))
			m[strings.TrimSpace(kv[0])] = strings.TrimSpace(value)
		}
	}
	return m
}

func cleanupInlinedSVG(svg string) string {
	svg = strings.ReplaceAll(svg, `xmlns:_xmlns="xmlns" `, "")
	svg = strings.ReplaceAll(svg, `_xmlns:xlink=`, `xmlns:xlink=`)
	svg = strings.Replace(svg,
		`xmlns="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg"`,
		`xmlns="http://www.w3.org/2000/svg"`,
		1,
	)
	return svg
}

func inlineElementStyles(attrs []xml.Attr, ctx elementContext, ancestors []elementContext, rules []styleRule) []xml.Attr {
	inline := parseStylesMap(attrValue(attrs, "style"))
	computed := cloneStylesMap(inline)

	for _, rule := range rules {
		if !matchSelector(rule.parts, ctx, ancestors) {
			continue
		}
		for k, v := range rule.styles {
			if _, ok := inline[k]; ok {
				continue
			}
			computed[k] = v
		}
	}

	if len(computed) > 0 {
		attrs = materializePresentationAttrs(attrs, ctx, computed)
		// Remove the 'style' attribute entirely, as oksvg has a bug where it fails
		// to accumulate alpha correctly if fill-opacity is provided via 'style'.
		// Since we materialized all supported styles into presentation attributes,
		// we no longer need the 'style' attribute.
		var newAttrs []xml.Attr
		for _, attr := range attrs {
			if !strings.EqualFold(attr.Name.Local, "style") {
				newAttrs = append(newAttrs, attr)
			}
		}
		attrs = newAttrs
	}
	return attrs
}

func cloneStylesMap(src stylesMap) stylesMap {
	dst := make(stylesMap, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func attrValue(attrs []xml.Attr, name string) string {
	for _, attr := range attrs {
		if strings.EqualFold(attr.Name.Local, name) {
			return attr.Value
		}
	}
	return ""
}

func setAttrValue(attrs []xml.Attr, name string, value string) []xml.Attr {
	for i := range attrs {
		if strings.EqualFold(attrs[i].Name.Local, name) {
			attrs[i].Value = value
			return attrs
		}
	}
	return append(attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
}

var presentationStyleAttrs = map[string]string{
	"fill":               "fill",
	"fill-opacity":       "fill-opacity",
	"opacity":            "opacity",
	"stroke":             "stroke",
	"stroke-dasharray":   "stroke-dasharray",
	"stroke-linecap":     "stroke-linecap",
	"stroke-linejoin":    "stroke-linejoin",
	"stroke-miterlimit":  "stroke-miterlimit",
	"stroke-opacity":     "stroke-opacity",
	"stroke-width":       "stroke-width",
	"display":            "display",
	"visibility":         "visibility",
	"font-family":        "font-family",
	"font-size":          "font-size",
	"font-style":         "font-style",
	"font-weight":        "font-weight",
	"text-anchor":        "text-anchor",
	"dominant-baseline":  "dominant-baseline",
	"alignment-baseline": "alignment-baseline",
}

func materializePresentationAttrs(attrs []xml.Attr, ctx elementContext, styles stylesMap) []xml.Attr {
	for styleName, attrName := range presentationStyleAttrs {
		value, ok := styles[styleName]
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		attrs = setAttrValue(attrs, attrName, value)
	}
	if fill, ok := styles["color"]; ok && strings.EqualFold(ctx.tag, "text") && strings.TrimSpace(fill) != "" {
		if strings.TrimSpace(attrValue(attrs, "fill")) == "" {
			attrs = setAttrValue(attrs, "fill", fill)
		}
	}
	return attrs
}

func parseSelectorParts(selector string) ([]cssSelectorPart, bool) {
	selector = strings.TrimSpace(html.UnescapeString(selector))
	if selector == "" {
		return nil, false
	}
	selector = strings.ReplaceAll(selector, ">", " ")
	parts := strings.Fields(selector)
	if len(parts) == 0 {
		return nil, false
	}
	parsed := make([]cssSelectorPart, 0, len(parts))
	for _, part := range parts {
		parsedPart, ok := parseSelectorPart(part)
		if !ok {
			return nil, false
		}
		parsed = append(parsed, parsedPart)
	}
	return parsed, true
}

func parseSelectorPart(part string) (cssSelectorPart, bool) {
	part = strings.TrimSpace(part)
	if part == "" {
		return cssSelectorPart{}, false
	}
	if strings.ContainsAny(part, ">*+~[:") {
		return cssSelectorPart{}, false
	}
	var parsed cssSelectorPart
	rest := part
	if rest[0] != '.' && rest[0] != '#' {
		next := len(rest)
		if idx := strings.IndexAny(rest, ".#"); idx >= 0 {
			next = idx
		}
		parsed.tag = rest[:next]
		rest = rest[next:]
	}
	for rest != "" {
		switch rest[0] {
		case '.':
			rest = rest[1:]
			next := len(rest)
			if idx := strings.IndexAny(rest, ".#"); idx >= 0 {
				next = idx
			}
			if next == 0 {
				return cssSelectorPart{}, false
			}
			parsed.classes = append(parsed.classes, rest[:next])
			rest = rest[next:]
		case '#':
			rest = rest[1:]
			next := len(rest)
			if idx := strings.IndexAny(rest, ".#"); idx >= 0 {
				next = idx
			}
			if next == 0 || parsed.id != "" {
				return cssSelectorPart{}, false
			}
			parsed.id = rest[:next]
			rest = rest[next:]
		default:
			return cssSelectorPart{}, false
		}
	}
	if parsed.tag == "" && parsed.id == "" && len(parsed.classes) == 0 {
		return cssSelectorPart{}, false
	}
	return parsed, true
}

func matchSelector(selector []cssSelectorPart, current elementContext, ancestors []elementContext) bool {
	if len(selector) == 0 || !selectorPartMatches(selector[len(selector)-1], current) {
		return false
	}
	ancestorIdx := len(ancestors) - 1
	for partIdx := len(selector) - 2; partIdx >= 0; partIdx-- {
		found := false
		for ancestorIdx >= 0 {
			if selectorPartMatches(selector[partIdx], ancestors[ancestorIdx]) {
				found = true
				ancestorIdx--
				break
			}
			ancestorIdx--
		}
		if !found {
			return false
		}
	}
	return true
}

func selectorPartMatches(part cssSelectorPart, elem elementContext) bool {
	if part.tag != "" && !strings.EqualFold(part.tag, elem.tag) {
		return false
	}
	if part.id != "" && part.id != elem.id {
		return false
	}
	for _, className := range part.classes {
		if !containsClass(elem.classes, className) {
			return false
		}
	}
	return true
}

func containsClass(classes []string, target string) bool {
	for _, className := range classes {
		if className == target {
			return true
		}
	}
	return false
}

var svgRootTagPattern = regexp.MustCompile(`(?i)(<svg\b)([^>]*)(>)`)
var svgWidthAttrPattern = regexp.MustCompile(`(?i)\bwidth\s*=\s*"([^"]*)"`)
var svgHeightAttrPattern = regexp.MustCompile(`(?i)\bheight\s*=\s*"([^"]*)"`)

func fixSVGRootDimensions(svg string) string {
	viewBox, ok := parseSVGViewBox(svg)
	if !ok || viewBox.W <= 0 || viewBox.H <= 0 {
		return svg
	}
	rootMatch := svgRootTagPattern.FindStringSubmatchIndex(svg)
	if rootMatch == nil {
		return svg
	}
	rootTag := svg[rootMatch[0]:rootMatch[6]]
	wStr := formatFloat(viewBox.W)
	hStr := formatFloat(viewBox.H)

	newTag := rootTag
	if m := svgWidthAttrPattern.FindStringSubmatchIndex(newTag); m != nil {
		valStart := m[2]
		valEnd := m[3]
		val := newTag[valStart:valEnd]
		if strings.Contains(val, "%") || val == "0" {
			newTag = newTag[:valStart] + wStr + newTag[valEnd:]
		}
	}
	if m := svgHeightAttrPattern.FindStringSubmatchIndex(newTag); m != nil {
		valStart := m[2]
		valEnd := m[3]
		val := newTag[valStart:valEnd]
		if strings.Contains(val, "%") || val == "0" {
			newTag = newTag[:valStart] + hStr + newTag[valEnd:]
		}
	} else {
		closing := strings.LastIndex(newTag, ">")
		if closing > 0 {
			newTag = newTag[:closing] + ` height="` + hStr + `"` + newTag[closing:]
		}
	}
	return svg[:rootMatch[0]] + newTag + svg[rootMatch[6]:]
}

// expandViewBoxToContent scans for translate transforms in the SVG and
// expands the viewBox so that all content (including elements positioned
// above y=0, like subgraph cluster labels) fits within the raster canvas.
func expandViewBoxToContent(svg string) string {
	viewBox, ok := parseSVGViewBox(svg)
	if !ok || viewBox.W <= 0 || viewBox.H <= 0 {
		return svg
	}
	minX := viewBox.X
	minY := viewBox.Y
	maxX := viewBox.X + viewBox.W
	maxY := viewBox.Y + viewBox.H

	for _, m := range svgTranslatePattern.FindAllStringSubmatch(svg, -1) {
		if len(m) < 2 {
			continue
		}
		if tx, ok := parseAnyFloat(m[1]); ok {
			if tx < minX {
				minX = tx - 10
			}
			if tx > maxX {
				maxX = tx + 10
			}
		}
		if len(m) >= 3 && strings.TrimSpace(m[2]) != "" {
			if ty, ok := parseAnyFloat(m[2]); ok {
				if ty < minY {
					minY = ty - 10
				}
				if ty > maxY {
					maxY = ty + 10
				}
			}
		}
	}

	if minX >= viewBox.X && minY >= viewBox.Y && maxX <= viewBox.X+viewBox.W && maxY <= viewBox.Y+viewBox.H {
		return svg
	}
	newW := maxX - minX
	newH := maxY - minY
	oldVB := fmt.Sprintf(`viewBox="%s %s %s %s"`,
		formatFloat(viewBox.X), formatFloat(viewBox.Y),
		formatFloat(viewBox.W), formatFloat(viewBox.H))
	newVB := fmt.Sprintf(`viewBox="%s %s %s %s"`,
		formatFloat(minX), formatFloat(minY),
		formatFloat(newW), formatFloat(newH))
	return strings.Replace(svg, oldVB, newVB, 1)
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
var svgForeignObjectSwitchPattern = regexp.MustCompile(`(?s)<switch\b[^>]*>.*?<foreignObject\b[^>]*>.*?</foreignObject>.*?</switch>`)
var svgRGBDecimalPattern = regexp.MustCompile(`rgb\(\s*([0-9]*\.?[0-9]+)\s*,\s*([0-9]*\.?[0-9]+)\s*,\s*([0-9]*\.?[0-9]+)\s*\)`)
var svgRGBAPattern = regexp.MustCompile(`rgba\(\s*([0-9]*\.?[0-9]+)\s*,\s*([0-9]*\.?[0-9]+)\s*,\s*([0-9]*\.?[0-9]+)\s*,\s*[0-9]*\.?[0-9]+\s*\)`)

func normalizeSVGForRasterizer(svg string) string {
	normalized := normalizeSVGPathData(svg)
	normalized = normalizeSVGLineAttrs(normalized)
	normalized = normalizeSVGCurrentColor(normalized)
	normalized = normalizeSVGTransparentColor(normalized)
	normalized = normalizeSVGRGBAColors(normalized)
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

func stripSVGForeignObjectSwitches(svg string) string {
	// Extract <text> fallbacks from <switch> blocks, removing the foreignObject.
	// The switch structure is: <switch><foreignObject>...</foreignObject><text>...</text></switch>
	// We keep the <text>...</text> and remove the rest.
	re := regexp.MustCompile(`(?s)<switch\b[^>]*>\s*<foreignObject\b[^>]*>.*?</foreignObject>\s*(<text\b[^>]*>.*?</text>)\s*</switch>`)
	return re.ReplaceAllString(svg, "$1")
}

var svgClipPathElementPattern = regexp.MustCompile(`(?s)<clipPath\b[^>]*>.*?</clipPath>`)
var svgClipPathAttrPattern = regexp.MustCompile(`\s*clip-path\s*=\s*"[^"]*"`)

func stripClipPaths(svg string) string {
	svg = svgClipPathElementPattern.ReplaceAllString(svg, "")
	svg = svgClipPathAttrPattern.ReplaceAllString(svg, "")
	return svg
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

func normalizeSVGRGBAColors(svg string) string {
	return svgRGBAPattern.ReplaceAllStringFunc(svg, func(raw string) string {
		match := svgRGBAPattern.FindStringSubmatch(raw)
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
	svgTSpanOpenPattern   = regexp.MustCompile(`(?is)<tspan\b([^>]*)>`)
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
	svg = stripSVGForeignObjectSwitches(svg)
	if !hasViewBox || viewBox.W <= 0 || viewBox.H <= 0 {
		viewBox = svgViewBox{X: 0, Y: 0, W: float64(width), H: float64(height)}
	}
	transform := computeSVGRasterTransform(width, height, viewBox)

	matches := svgTextElementPattern.FindAllStringSubmatchIndex(svg, -1)
	for _, loc := range matches {
		if len(loc) < 6 {
			continue
		}
		attrs := svg[loc[2]:loc[3]]
		inner := svg[loc[4]:loc[5]]
		content := extractTextContent(inner)
		if strings.TrimSpace(content) == "" {
			continue
		}
		tspanAttrs, hasTSpan := firstTSpanAttrs(inner)

		rawX := firstNumericToken(parseAttr(attrs, "x"))
		rawY := firstNumericToken(parseAttr(attrs, "y"))
		if rawX == "" && hasTSpan {
			rawX = firstNumericToken(parseAttr(tspanAttrs, "x"))
		}
		if rawY == "" && hasTSpan {
			rawY = firstNumericToken(parseAttr(tspanAttrs, "y"))
		}
		x, okX := parseAnyFloat(rawX)
		y, okY := parseAnyFloat(rawY)
		if rawX == "" && hasTSpan {
			x = 0
			okX = true
		}
		if rawY == "" && hasTSpan {
			y = 0
			okY = true
		}
		if !okX || !okY {
			continue
		}

		ancestorTransform := accumulateGroupTransform(svg[:loc[0]])

		fontSize := 16.0
		if rawSize := parseAttr(attrs, "font-size"); rawSize != "" {
			if size, ok := parseDimensionValue(rawSize); ok {
				fontSize = size
			}
		}
		if rawSize := styleValue(parseAttr(attrs, "style"), "font-size"); rawSize != "" {
			if size, ok := parseDimensionValue(rawSize); ok {
				fontSize = size
			}
		}
		fontFamily := parseAttr(attrs, "font-family")
		if fontFamily == "" {
			fontFamily = styleValue(parseAttr(attrs, "style"), "font-family")
		}
		face := resolveRasterFontFace(fontFamily, max(8.0, fontSize*transform.Scale))
		textColor := color.Color(nil)
		if hasTSpan {
			textColor = parseTextColor(parseAttr(tspanAttrs, "fill"))
			if textColor == nil {
				if styleFill := styleValue(parseAttr(tspanAttrs, "style"), "fill"); styleFill != "" {
					textColor = parseTextColor(styleFill)
				}
			}
		}
		if textColor == nil {
			textColor = parseTextColor(parseAttr(attrs, "fill"))
			if textColor == nil {
				if styleFill := styleValue(parseAttr(attrs, "style"), "fill"); styleFill != "" {
					textColor = parseTextColor(styleFill)
				}
			}
		}
		rawDX := parseAttr(attrs, "dx")
		rawDY := parseAttr(attrs, "dy")
		if hasTSpan {
			if rawDX == "" {
				rawDX = parseAttr(tspanAttrs, "dx")
			}
			if rawDY == "" {
				rawDY = parseAttr(tspanAttrs, "dy")
			}
		}
		localX := x + parseSVGTextLength(rawDX, fontSize)
		localY := y + parseSVGTextLength(rawDY, fontSize)

		// Apply the text element's own transform (e.g. translate) to local coords.
		// XY chart text elements use x=0 y=0 with transform="translate(px,py) rotate(r)".
		if transformAttr := strings.TrimSpace(parseAttr(attrs, "transform")); transformAttr != "" {
			elemTransform := parseSVGTransform(transformAttr)
			localX, localY = elemTransform.apply(localX, localY)
		}

		pointX, pointY := ancestorTransform.apply(localX, localY)
		px := transform.mapX(pointX, viewBox)
		py := transform.mapY(pointY, viewBox)
		advance := font.MeasureString(face, content)
		anchor := strings.TrimSpace(parseAttr(attrs, "text-anchor"))
		if anchor == "" {
			anchor = styleValue(parseAttr(attrs, "style"), "text-anchor")
		}
		var anchorOffsetX, baselineOffsetY float64
		switch anchor {
		case "middle":
			anchorOffsetX = -float64(advance) / 128.0
		case "end":
			anchorOffsetX = -float64(advance) / 64.0
		}
		if textColor == nil {
			textWidth := float64(advance) / 64.0
			sampleX := px + anchorOffsetX + textWidth/2.0
			sampleY := py - fontSize*transform.Scale*0.35
			textColor = autoContrastTextColor(img, int(math.Round(sampleX)), int(math.Round(sampleY)))
		}

		drawer := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(textColor),
			Face: face,
		}
		dominantBaseline := strings.TrimSpace(parseAttr(attrs, "dominant-baseline"))
		if dominantBaseline == "" {
			dominantBaseline = styleValue(parseAttr(attrs, "style"), "dominant-baseline")
		}
		if dominantBaseline == "" {
			dominantBaseline = strings.TrimSpace(parseAttr(attrs, "alignment-baseline"))
		}
		if dominantBaseline == "" {
			dominantBaseline = styleValue(parseAttr(attrs, "style"), "alignment-baseline")
		}
		baselineOffsetY = svgTextBaselineOffset(face, dominantBaseline)

		rotateAngle := ancestorTransform.rotationDegrees()
		if transformAttr := strings.TrimSpace(parseAttr(attrs, "transform")); transformAttr != "" {
			rotateAngle += parseSVGTransform(transformAttr).rotationDegrees()
		}

		if rotateAngle != 0 {
			overlayRotatedText(img, content, face, textColor, px, py, rotateAngle, anchorOffsetX, baselineOffsetY)
		} else {
			drawer.Dot = fixed.P(int(math.Round(px+anchorOffsetX)), int(math.Round(py+baselineOffsetY)))
			drawer.DrawString(content)
		}
	}

	overlaySVGForeignObjectText(img, svg, width, height, viewBox, hasViewBox)
}

func firstTSpanAttrs(inner string) (string, bool) {
	match := svgTSpanOpenPattern.FindStringSubmatch(inner)
	if len(match) < 2 {
		return "", false
	}
	return match[1], true
}

func parseSVGTextLength(raw string, fontSize float64) float64 {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0
	}
	value = firstNumericToken(strings.ReplaceAll(value, ",", " "))
	switch {
	case strings.HasSuffix(value, "em"):
		scale, ok := parseAnyFloat(strings.TrimSuffix(value, "em"))
		if !ok {
			return 0
		}
		return scale * fontSize
	case strings.HasSuffix(value, "px"):
		fallthrough
	default:
		parsed, ok := parseAnyFloat(value)
		if !ok {
			return 0
		}
		return parsed
	}
}

func svgTextBaselineOffset(face font.Face, dominantBaseline string) float64 {
	mode := strings.ToLower(strings.TrimSpace(dominantBaseline))
	if mode == "" {
		return 0
	}
	metrics := face.Metrics()
	ascent := float64(metrics.Ascent) / 64.0
	descent := float64(metrics.Descent) / 64.0
	switch mode {
	case "middle", "central":
		return (ascent - descent) / 2.0
	case "hanging", "text-before-edge", "before-edge":
		return ascent
	case "ideographic", "text-after-edge", "after-edge":
		return -descent
	default:
		return 0
	}
}

func overlaySVGForeignObjectText(img *image.NRGBA, svg string, width int, height int, viewBox svgViewBox, hasViewBox bool) {
	if !hasViewBox || viewBox.W <= 0 || viewBox.H <= 0 {
		viewBox = svgViewBox{X: 0, Y: 0, W: float64(width), H: float64(height)}
	}
	transform := computeSVGRasterTransform(width, height, viewBox)
	mindmapCenteredText := strings.Contains(svg, "mindmapDiagram")

	for _, label := range extractForeignObjectLabels(svg, viewBox) {
		face := resolveRasterFontFace(label.FontFamily, max(8.0, label.FontSize*transform.Scale))
		px := transform.mapX(label.X, viewBox)
		py := transform.mapY(label.Y, viewBox)
		textColor := parseTextColor(label.Color)
		if textColor == nil {
			textColor = autoContrastTextColor(img, int(math.Round(px)), int(math.Round(py)))
		}
		drawer := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(textColor),
			Face: face,
		}
		metrics := face.Metrics()
		ascent := float64(metrics.Ascent) / 64.0
		descent := float64(metrics.Descent) / 64.0
		lineBoxHeight := label.H * transform.Scale
		if lineBoxHeight < ascent+descent {
			lineBoxHeight = ascent + descent
		}
		py += (lineBoxHeight-(ascent+descent))/2.0 + ascent
		textWidth := float64(drawer.MeasureString(label.Text)) / 64.0
		if mindmapCenteredText && label.W > 0 {
			boxWidth := label.W * transform.Scale
			px += (boxWidth - textWidth) / 2.0
			px += 11.0
		} else if label.W > 0 {
			switch strings.ToLower(strings.TrimSpace(label.TextAlign)) {
			case "center", "middle":
				px += (label.W*transform.Scale - textWidth) / 2.0
			case "right", "end":
				px += label.W*transform.Scale - textWidth
			}
		}
		drawer.Dot = fixed.P(int(math.Round(px)), int(math.Round(py)))
		drawer.DrawString(label.Text)
	}
}

type foreignObjectLabel struct {
	X          float64
	Y          float64
	W          float64
	H          float64
	Text       string
	FontSize   float64
	FontFamily string
	Color      string
	TextAlign  string
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
	TextAlign  string
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
						W:          current.W,
						H:          current.H,
						Text:       text,
						FontSize:   current.FontSize,
						FontFamily: current.FontFamily,
						Color:      current.Color,
						TextAlign:  current.TextAlign,
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
		if v := styleValue(style, "font-family"); v != "" {
			capture.FontFamily = v
		}
		if v := styleValue(style, "color"); v != "" {
			capture.Color = v
		}
		if v := styleValue(style, "text-align"); v != "" {
			capture.TextAlign = v
		}
	}
	if size := xmlAttr(attrs, "font-size"); size != "" {
		if parsed, ok := parseDimensionValue(size); ok {
			capture.FontSize = parsed
		}
	}
	if family := xmlAttr(attrs, "font-family"); family != "" {
		capture.FontFamily = family
	}
	if col := xmlAttr(attrs, "color"); col != "" {
		capture.Color = col
	}
	if fill := xmlAttr(attrs, "fill"); fill != "" {
		capture.Color = fill
	}
	if align := xmlAttr(attrs, "text-align"); align != "" {
		capture.TextAlign = align
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

type svgAffineTransform struct {
	A float64
	B float64
	C float64
	D float64
	E float64
	F float64
}

func identitySVGTransform() svgAffineTransform {
	return svgAffineTransform{A: 1, D: 1}
}

func (t svgAffineTransform) multiply(other svgAffineTransform) svgAffineTransform {
	return svgAffineTransform{
		A: t.A*other.A + t.C*other.B,
		B: t.B*other.A + t.D*other.B,
		C: t.A*other.C + t.C*other.D,
		D: t.B*other.C + t.D*other.D,
		E: t.A*other.E + t.C*other.F + t.E,
		F: t.B*other.E + t.D*other.F + t.F,
	}
}

func (t svgAffineTransform) apply(x, y float64) (float64, float64) {
	return t.A*x + t.C*y + t.E, t.B*x + t.D*y + t.F
}

func (t svgAffineTransform) rotationDegrees() float64 {
	return math.Atan2(t.B, t.A) * 180.0 / math.Pi
}

var svgTransformOpPattern = regexp.MustCompile(`([a-zA-Z]+)\(([^)]*)\)`)

func parseSVGTransform(transform string) svgAffineTransform {
	current := identitySVGTransform()
	for _, match := range svgTransformOpPattern.FindAllStringSubmatch(transform, -1) {
		if len(match) < 3 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(match[1]))
		fields := strings.Fields(strings.ReplaceAll(match[2], ",", " "))
		values := make([]float64, 0, len(fields))
		for _, field := range fields {
			if value, ok := parseAnyFloat(field); ok {
				values = append(values, value)
			}
		}
		switch name {
		case "translate":
			if len(values) == 0 {
				continue
			}
			tx := values[0]
			ty := 0.0
			if len(values) > 1 {
				ty = values[1]
			}
			current = current.multiply(svgAffineTransform{A: 1, D: 1, E: tx, F: ty})
		case "scale":
			if len(values) == 0 {
				continue
			}
			sx := values[0]
			sy := sx
			if len(values) > 1 {
				sy = values[1]
			}
			current = current.multiply(svgAffineTransform{A: sx, D: sy})
		case "rotate":
			if len(values) == 0 {
				continue
			}
			angle := values[0] * math.Pi / 180.0
			cosA := math.Cos(angle)
			sinA := math.Sin(angle)
			rotation := svgAffineTransform{A: cosA, B: sinA, C: -sinA, D: cosA}
			if len(values) >= 3 {
				cx := values[1]
				cy := values[2]
				current = current.
					multiply(svgAffineTransform{A: 1, D: 1, E: cx, F: cy}).
					multiply(rotation).
					multiply(svgAffineTransform{A: 1, D: 1, E: -cx, F: -cy})
			} else {
				current = current.multiply(rotation)
			}
		case "matrix":
			if len(values) != 6 {
				continue
			}
			current = current.multiply(svgAffineTransform{
				A: values[0],
				B: values[1],
				C: values[2],
				D: values[3],
				E: values[4],
				F: values[5],
			})
		}
	}
	return current
}

func parseTransformOffset(transform string) (float64, float64) {
	matrix := parseSVGTransform(transform)
	return matrix.E, matrix.F
}

var svgGOpenPattern = regexp.MustCompile(`<g\b([^>]*)>`)
var svgGClosePattern = regexp.MustCompile(`</g\s*>`)

func accumulateGroupTransform(svgPrefix string) svgAffineTransform {
	type gEntry struct {
		transform svgAffineTransform
	}
	var stack []gEntry

	opens := svgGOpenPattern.FindAllStringSubmatchIndex(svgPrefix, -1)
	closes := svgGClosePattern.FindAllStringIndex(svgPrefix, -1)

	type event struct {
		pos    int
		isOpen bool
		attrs  string
	}
	events := make([]event, 0, len(opens)+len(closes))
	for _, o := range opens {
		attrs := ""
		if len(o) >= 4 {
			attrs = svgPrefix[o[2]:o[3]]
		}
		events = append(events, event{pos: o[0], isOpen: true, attrs: attrs})
	}
	for _, c := range closes {
		events = append(events, event{pos: c[0], isOpen: false})
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].pos < events[j].pos
	})

	for _, ev := range events {
		if ev.isOpen {
			matrix := identitySVGTransform()
			if transformAttr := parseAttr(ev.attrs, "transform"); transformAttr != "" {
				matrix = parseSVGTransform(transformAttr)
			}
			stack = append(stack, gEntry{transform: matrix})
		} else {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}

	total := identitySVGTransform()
	for _, entry := range stack {
		total = total.multiply(entry.transform)
	}
	return total
}

func accumulateTransforms(svgPrefix string) (float64, float64) {
	total := accumulateGroupTransform(svgPrefix)
	return total.E, total.F
}

func extractTextContent(input string) string {
	value := html.UnescapeString(input)
	value = svgTagPattern.ReplaceAllString(value, "")
	value = strings.Join(strings.Fields(value), " ")
	return strings.TrimSpace(value)
}

func parseAttr(attrs string, name string) string {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*=\s*"([^"]*)"`)
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
		value := strings.ReplaceAll(parts[1], "!important", "")
		return strings.TrimSpace(value)
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

func overlayRotatedText(img *image.NRGBA, content string, face font.Face, textColor color.Color, px, py, angleDeg, anchorOffsetX, baselineOffsetY float64) {
	// 1. Measure the text
	advance := font.MeasureString(face, content)
	metrics := face.Metrics()
	width := int(math.Ceil(float64(advance) / 64.0))
	ascent := int(math.Ceil(float64(metrics.Ascent) / 64.0))
	descent := int(math.Ceil(float64(metrics.Descent) / 64.0))

	pad := 2
	w := width + pad*2
	h := ascent + descent + pad*2

	// Render into offscreen buffer
	off := image.NewNRGBA(image.Rect(0, 0, w, h))
	drawer := &font.Drawer{
		Dst:  off,
		Src:  image.NewUniform(textColor),
		Face: face,
		Dot:  fixed.P(pad, pad+ascent),
	}
	drawer.DrawString(content)

	// Origin point inside the offscreen image corresponds to dot
	ox := float64(pad) - anchorOffsetX
	oy := float64(pad+ascent) - baselineOffsetY

	// Rotation (SVG rotations act on origin, we want to rotate around px, py)
	// Actually SVG rotate(angle) is clockwise for positive angles, but math.Sincos is CCW in standard math plane.
	// SVG coordinate system: +Y is down. So clockwise rotation means standard positive angle in this metric space.
	rad := angleDeg * math.Pi / 180.0
	sin, cos := math.Sincos(rad)

	bounds := img.Bounds()
	maxX, maxY := bounds.Dx(), bounds.Dy()

	// 2. Map and blend pixels
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := off.At(x, y)
			_, _, _, a := c.RGBA()
			if a > 0 {
				// offset from origin
				tx := float64(x) - ox
				ty := float64(y) - oy

				// Rotate
				rx := tx*cos - ty*sin
				ry := tx*sin + ty*cos

				// Transform back to absolute
				dx := int(math.Round(px + rx))
				dy := int(math.Round(py + ry))

				if dx >= 0 && dy >= 0 && dx < maxX && dy < maxY {
					cr, cg, cb, ca := c.RGBA()
					bg := img.At(dx, dy)
					br, bg_, bb, ba := bg.RGBA()

					alpha := ca
					if alpha == 0xffff {
						img.Set(dx, dy, c)
					} else {
						// Blend
						r := (cr*alpha + br*(0xffff-alpha)) / 0xffff
						g := (cg*alpha + bg_*(0xffff-alpha)) / 0xffff
						b := (cb*alpha + bb*(0xffff-alpha)) / 0xffff
						aOut := alpha + ba*(0xffff-alpha)/0xffff
						img.Set(dx, dy, color.NRGBA64{R: uint16(r), G: uint16(g), B: uint16(b), A: uint16(aOut)})
					}
				}
			}
		}
	}
}
