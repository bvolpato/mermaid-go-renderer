// Ported from mermaid 12.2.0, rendering-elements/shapes/util.ts,
// shapes/labelImageUtils.ts, shapes/roundedRectPath.ts,
// shapes/insertPolygonShape.ts and the parts of
// diagrams/common/colorThemeGate.ts that shapes use (MIT).

package ru

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

// LabelBBox is the measured box of a label. An HTML label is measured with
// getBoundingClientRect, which gives a DOMRect with left and top. SVG text
// is measured with getBBox, which gives an SVGRect without them. Shapes
// depend on the difference (`bbox.left ?? 0`), so HasLeft records the kind.
type LabelBBox struct {
	X, Y, Width, Height float64
	Left, Top           float64
	HasLeft             bool
}

// LeftOr0 is `bbox.left ?? 0`.
func (b LabelBBox) LeftOr0() float64 {
	if b.HasLeft {
		return b.Left
	}
	return 0
}

// TopOr0 is `bbox.top ?? 0`.
func (b LabelBBox) TopOr0() float64 {
	if b.HasLeft {
		return b.Top
	}
	return 0
}

func domRectBBox(r dom.Rect) LabelBBox {
	return LabelBBox{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height, Left: r.X, Top: r.Y, HasLeft: true}
}

func svgRectBBox(r dom.Rect) LabelBBox {
	return LabelBBox{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}
}

// WithMinWidth is withMinWidth(bbox, minWidth): it widens a measured label
// box. SVG text stays centered as it grows.
func WithMinWidth(bbox LabelBBox, minWidth float64) LabelBBox {
	if bbox.Width >= minWidth {
		return bbox
	}
	if !bbox.HasLeft {
		return LabelBBox{X: bbox.X - float64((minWidth-bbox.Width)/2), Y: bbox.Y, Width: minWidth, Height: bbox.Height}
	}
	return LabelBBox{X: bbox.X, Y: bbox.Y, Width: minWidth, Height: bbox.Height, Left: bbox.X, Top: bbox.Y, HasLeft: true}
}

// LabelResult is the return value of labelHelper and insertLabel.
type LabelResult struct {
	ShapeSvg    *dom.Node
	BBox        LabelBBox
	HalfPadding float64
	Label       *dom.Node
}

// C4LabelHelper is c4LabelHelper of shapes/c4LabelHelper.ts. The package
// that ports it sets this variable.
var C4LabelHelper func(ctx *render.Context, parent *dom.Node, node *Node, classes string) (*LabelResult, error)

// LabelHelperOptions are the options of labelHelper.
type LabelHelperOptions struct {
	IgnoreMinWidth bool
}

func translate(x, y float64) string {
	return "translate(" + js.Str(x) + ", " + js.Str(y) + ")"
}

// LabelHelper is labelHelper(parent, node, classes, options): it adds the
// outer group of a shape and its label, and measures the label.
func LabelHelper(ctx *render.Context, parent *dom.Node, node *Node, classes string, opts ...LabelHelperOptions) (*LabelResult, error) {
	var o LabelHelperOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	// Nodes with a stereotype render a stacked multi-section SVG label.
	if node.HasStereotype {
		if C4LabelHelper == nil {
			return nil, &NotPortedError{Kind: "shape", Name: "c4LabelHelper"}
		}
		return C4LabelHelper(ctx, parent, node, classes)
	}
	useHtmlLabels := node.UseHtmlLabels || config.Evaluate(ctx.Cfg("htmlLabels"))
	cssClasses := classes
	if classes == "" {
		cssClasses = "node default"
	}

	shapeSvg := parent.Insert("g").Attr("class", cssClasses)
	if node.DomID != "" {
		shapeSvg.Attr("id", node.DomID)
	} else {
		shapeSvg.Attr("id", node.ID)
	}

	// Create the label and insert it after the rect.
	labelEl := shapeSvg.Insert("g").Attr("class", "label").Attr("style", node.LabelStyle)

	label := node.Label

	addBackground := node.Icon != "" || node.Img != ""
	isMarkdown := node.LabelType == "markdown"
	// An explicit node width wins over the diagram-level minimum. An empty
	// label has nothing to widen.
	minLabelWidth := 0.0
	if label != "" && !js.Truthy(node.Width) && !o.IgnoreMinWidth {
		minLabelWidth = NN(node.MinWidth, 0)
	}
	width := node.Width
	if !js.Truthy(width) {
		width = node.WrappingWidth
	}
	if !js.Truthy(width) {
		if v, ok := ctx.CfgNum("flowchart", "wrappingWidth"); ok {
			width = v
		} else {
			width = Undef
		}
	}
	textClasses := ""
	if isMarkdown {
		textClasses = "markdown-node-label"
	}
	text, err := CreateText(ctx, labelEl, ctx.SanitizeText(render.DecodeEntities(label)), TextOptions{
		UseHtmlLabels:    useHtmlLabels,
		Width:            width,
		MinWidth:         minLabelWidth,
		Classes:          textClasses,
		Style:            node.LabelStyle,
		AddSvgBackground: addBackground,
		Markdown:         isMarkdown,
		IsNode:           true,
	})
	if err != nil {
		return nil, err
	}

	halfPadding := NN(node.Padding, 0) / 2
	var bbox LabelBBox
	if useHtmlLabels {
		div := text.FirstElementChild()
		ConfigureLabelImages(ctx, div)
		ctx.AwaitMeasure()
		bbox = WithMinWidth(domRectBBox(div.BoundingClientRect()), minLabelWidth)
		text.Attr("width", bbox.Width)
		text.Attr("height", bbox.Height)
	} else {
		ctx.AwaitMeasure()
		bbox = WithMinWidth(svgRectBBox(text.BBox()), minLabelWidth)
	}

	// Center the label.
	if useHtmlLabels {
		labelEl.Attr("transform", translate(-bbox.Width/2, -bbox.Height/2))
	} else {
		labelEl.Attr("transform", translate(0, -bbox.Height/2))
	}
	if node.CenterLabel {
		labelEl.Attr("transform", translate(-bbox.Width/2, -bbox.Height/2))
	}
	labelEl.Insert("rect", ":first-child")
	// The await of the caller.
	ctx.Await()
	return &LabelResult{ShapeSvg: shapeSvg, BBox: bbox, HalfPadding: halfPadding, Label: labelEl}, nil
}

// InsertLabelOptions are the options of insertLabel.
type InsertLabelOptions struct {
	LabelStyle string
	Icon       bool
	Img        string
	// UseHtmlLabels is used when UseHtmlLabelsSet is true.
	UseHtmlLabels    bool
	UseHtmlLabelsSet bool
	Padding          float64
	Width            float64
	CenterLabel      bool
	AddSvgBackground bool
}

// InsertLabel is insertLabel(parent, label, options).
func InsertLabel(ctx *render.Context, parent *dom.Node, label string, options InsertLabelOptions) (*LabelResult, error) {
	effective := sanitize.GetEffectiveHtmlLabels(ctx.Config)
	useHtmlLabels := effective
	if options.UseHtmlLabelsSet {
		useHtmlLabels = options.UseHtmlLabels
	}

	labelEl := parent.Insert("g").Attr("class", "label").Attr("style", options.LabelStyle)

	width := options.Width
	if !js.Truthy(width) {
		if v, ok := ctx.CfgNum("flowchart", "wrappingWidth"); ok {
			width = v
		} else {
			width = Undef
		}
	}
	text, err := CreateText(ctx, labelEl, ctx.SanitizeText(render.DecodeEntities(label)), TextOptions{
		UseHtmlLabels:    useHtmlLabels,
		Width:            width,
		Style:            options.LabelStyle,
		AddSvgBackground: options.Icon || options.Img != "",
		Markdown:         true,
		IsNode:           true,
	})
	if err != nil {
		return nil, err
	}
	halfPadding := options.Padding / 2
	var bbox LabelBBox
	if effective {
		div := text.FirstElementChild()
		ctx.AwaitMeasure()
		bbox = domRectBBox(div.BoundingClientRect())
		text.Attr("width", bbox.Width)
		text.Attr("height", bbox.Height)
	} else {
		ctx.AwaitMeasure()
		bbox = svgRectBBox(text.BBox())
	}

	if useHtmlLabels {
		labelEl.Attr("transform", translate(-bbox.Width/2, -bbox.Height/2))
	} else {
		labelEl.Attr("transform", translate(0, -bbox.Height/2))
	}
	if options.CenterLabel {
		labelEl.Attr("transform", translate(-bbox.Width/2, -bbox.Height/2))
	}
	labelEl.Insert("rect", ":first-child")
	// The await of the caller.
	ctx.Await()
	return &LabelResult{ShapeSvg: parent, BBox: bbox, HalfPadding: halfPadding, Label: labelEl}, nil
}

// UpdateNodeBounds is updateNodeBounds(node, element, knownBounds). Pass
// known bounds only when they are exactly what getBBox would return.
func UpdateNodeBounds(node *Node, element *dom.Node, knownBounds ...Bounds) {
	if len(knownBounds) > 0 {
		node.Width = knownBounds[0].Width
		node.Height = knownBounds[0].Height
		return
	}
	bbox := element.BBox()
	node.Width = bbox.Width
	node.Height = bbox.Height
}

// InsertPolygonShape is insertPolygonShape(parent, w, h, points).
func InsertPolygonShape(parent *dom.Node, w, h float64, points []Point) *dom.Node {
	parts := make([]string, len(points))
	for i, d := range points {
		parts[i] = js.Str(d.X) + "," + js.Str(d.Y)
	}
	return parent.Insert("polygon", ":first-child").
		Attr("points", strings.Join(parts, " ")).
		Attr("class", "label-container").
		Attr("transform", "translate("+js.Str(-w/2)+","+js.Str(h/2)+")")
}

// GetNodeClasses is getNodeClasses(node, extra).
func GetNodeClasses(node *Node, extra ...string) string {
	base := "node"
	if node.Look == "handDrawn" {
		base = "rough-node"
	}
	cssClasses := node.CSSClasses
	if !node.CSSClassesSet && cssClasses == "" {
		cssClasses = "undefined"
	}
	e := ""
	if len(extra) > 0 {
		e = extra[0]
	}
	return base + " " + cssClasses + " " + e
}

// CreatePathFromPoints is createPathFromPoints(points).
func CreatePathFromPoints(points []Point) string {
	parts := make([]string, 0, len(points)+1)
	for i, p := range points {
		cmd := "L"
		if i == 0 {
			cmd = "M"
		}
		parts = append(parts, cmd+js.Str(p.X)+","+js.Str(p.Y))
	}
	parts = append(parts, "Z")
	return strings.Join(parts, " ")
}

// GenerateFullSineWavePoints is generateFullSineWavePoints(...).
func GenerateFullSineWavePoints(x1, y1, x2, y2, amplitude, numCycles float64) []Point {
	var points []Point
	const steps = 50
	deltaX := x2 - x1
	deltaY := y2 - y1
	cycleLength := deltaX / numCycles

	frequency := float64(2*math.Pi) / cycleLength
	midY := y1 + float64(deltaY/2)

	for i := 0; i <= steps; i++ {
		t := float64(i) / steps
		x := x1 + float64(t*deltaX)
		y := midY + float64(amplitude*js.Sin(float64(frequency*(x-x1))))
		points = append(points, Point{X: x, Y: y})
	}
	return points
}

// GenerateCirclePoints is generateCirclePoints(...). The angles are in
// degrees.
func GenerateCirclePoints(centerX, centerY, radius float64, numPoints int, startAngle, endAngle float64) []Point {
	var points []Point
	startAngleRad := float64(startAngle*math.Pi) / 180
	endAngleRad := float64(endAngle*math.Pi) / 180
	angleRange := endAngleRad - startAngleRad
	angleStep := angleRange / float64(numPoints-1)

	for i := 0; i < numPoints; i++ {
		angle := startAngleRad + float64(float64(i)*angleStep)
		x := centerX + float64(radius*js.Cos(angle))
		y := centerY + float64(radius*js.Sin(angle))
		points = append(points, Point{X: -x, Y: -y})
	}
	return points
}

// MergePaths is mergePaths(roughElement): one path for the paths that
// rough drew, in a new group.
func MergePaths(doc *dom.Document, roughElement *dom.Node) *dom.Node {
	var paths []*dom.Node
	for c := roughElement.FirstChild; c != nil; c = c.Next {
		if c.Type == dom.ElementNode && c.Tag == "path" {
			paths = append(paths, c)
		}
	}
	mergedPath := doc.CreateElementNS(dom.NSSVG, "path")
	var ds []string
	for _, p := range paths {
		if d, ok := p.GetAttr("d"); ok {
			ds = append(ds, d)
		}
	}
	mergedPath.Attr("d", strings.Join(ds, " "))

	var fillPath, strokePath *dom.Node
	for _, p := range paths {
		if v, _ := p.GetAttr("fill"); v != "none" && fillPath == nil {
			fillPath = p
		}
		if v, _ := p.GetAttr("stroke"); v != "none" && strokePath == nil {
			strokePath = p
		}
	}
	set := func(attr, value string) {
		if value != "" {
			mergedPath.Attr(attr, value)
		}
	}
	if fillPath != nil {
		set("fill", fillPath.AttrOr("fill", ""))
		set("fill-opacity", fillPath.AttrOr("fill-opacity", "1"))
	}
	if strokePath != nil {
		set("stroke", strokePath.AttrOr("stroke", ""))
		set("stroke-width", strokePath.AttrOr("stroke-width", "1"))
		set("stroke-opacity", strokePath.AttrOr("stroke-opacity", "1"))
	}
	group := doc.CreateElementNS(dom.NSSVG, "g")
	group.AppendChild(mergedPath)
	return group
}

// CreateRoundedRectPathD is createRoundedRectPathD(x, y, w, h, radius).
func CreateRoundedRectPathD(x, y, totalWidth, totalHeight, radius float64) string {
	n := js.Str
	return strings.Join([]string{
		"M", n(x + radius), n(y),
		"H", n(x + totalWidth - radius),
		"A", n(radius), n(radius), "0", "0", "1", n(x + totalWidth), n(y + radius),
		"V", n(y + totalHeight - radius),
		"A", n(radius), n(radius), "0", "0", "1", n(x + totalWidth - radius), n(y + totalHeight),
		"H", n(x + radius),
		"A", n(radius), n(radius), "0", "0", "1", n(x), n(y + totalHeight - radius),
		"V", n(y + radius),
		"A", n(radius), n(radius), "0", "0", "1", n(x + radius), n(y),
		"Z",
	}, " ")
}

// ConfigureLabelImages is configureLabelImages(container): it sizes the
// images of an HTML label. The browser does this when each image loads or
// fails to load, which is before the label is measured.
func ConfigureLabelImages(ctx *render.Context, container *dom.Node) {
	images := container.SelectAll("img")
	if len(images) == 0 {
		// The await of the caller.
		ctx.Await()
		return
	}
	// Each image resolves from a timer or from its load event. Then
	// Promise.all and its await take two turns, and the await of the caller
	// takes one.
	ctx.AwaitTimeout()
	defer ctx.AwaitN(3)
	// The load of each image has ended here: the image has its natural
	// size, or it is the broken image. Before this point the layout engine
	// gives a pending image the size 0 x 0, as the browser does.
	if ctx.Engine != nil {
		ctx.Engine.CompleteImages(container)
	}
	noImgText := !hasTextBesidesImages(container)
	for _, img := range images {
		img.Style("display", "flex")
		img.Style("flex-direction", "column")
		if noImgText {
			const enlargingFactor = 5
			parsed := 16.0
			if v := ctx.Cfg("fontSize"); js.Truthy(v) {
				if n, ok := parseFontSizeNumber(v); ok {
					parsed = n
				}
			}
			width := js.Str(float64(parsed*enlargingFactor)) + "px"
			img.Style("min-width", width)
			img.Style("max-width", width)
		} else {
			img.Style("width", "100%")
		}
	}
}

// parseFontSizeNumber is the first element of parseFontSize(fontSize).
func parseFontSizeNumber(fontSize any) (float64, bool) {
	if n, ok := fontSize.(float64); ok {
		return n, true
	}
	n := js.ParseInt(js.ToString(fontSize), 10)
	if n != n {
		return 0, false
	}
	return n, true
}

func hasTextBesidesImages(node *dom.Node) bool {
	switch node.Type {
	case dom.TextNode:
		return js.TrimSpace(node.Data) != ""
	case dom.ElementNode:
		if strings.EqualFold(node.Tag, "img") {
			return false
		}
		for c := node.FirstChild; c != nil; c = c.Next {
			if hasTextBesidesImages(c) {
				return true
			}
		}
	}
	return false
}

// colorThemes are the themes that color items by slot.
var colorThemes = map[string]bool{"redux-color": true, "redux-dark-color": true}

func paletteSlotCount(palette any) int {
	if arr, ok := palette.([]any); ok {
		return len(arr)
	}
	return 0
}

// IsColorTheme is isColorTheme(theme, palette).
func IsColorTheme(theme any, palette any) bool {
	t, ok := theme.(string)
	return ok && colorThemes[t] && paletteSlotCount(palette) > 0
}

// StampColorSlot is stampColorSlot(shapeSvg, colorIndex, theme, palette):
// it marks the element with its color slot when the theme has a palette.
func StampColorSlot(shapeSvg *dom.Node, colorIndex float64, theme any, palette any) {
	if IsUndef(colorIndex) || !IsColorTheme(theme, palette) {
		return
	}
	slot := math.Mod(colorIndex, float64(paletteSlotCount(palette)))
	shapeSvg.Attr("data-color-id", "color-"+js.Str(slot))
}
