// Ported from mermaid 12.2.0, rendering-util/rendering-elements/edges.js
// (MIT).

// Package edges is the port of the edge rendering of mermaid's unified
// renderer: rendering-elements/edges.js, edgeMarker.ts, markers.js,
// lineJump.ts and labelTransform.ts. The layout algorithms call it to
// insert edge labels before the layout, and to draw edge paths and move the
// labels after it.
//
// The functions keep the names and the argument order of upstream. Each
// one takes the render context first, where upstream reads module state or
// the global config.
package edges

import (
	"encoding/base64"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/shape"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// The Math functions of internal/js return the bits of Chromium. The golden
// tests of this package check that against the browser.
func sin(x float64) float64      { return js.SinCR(x) }
func cos(x float64) float64      { return js.CosCR(x) }
func acos(x float64) float64     { return js.AcosCR(x) }
func atan2(y, x float64) float64 { return js.Atan2(y, x) }

// typeError is the TypeError that upstream throws when it reads a property
// of undefined.
func typeError(format string, args ...any) error {
	return fmt.Errorf("TypeError: "+format, args...)
}

// TerminalLabelElems is one value of the terminalLabels Map: the groups of
// the terminal labels of one edge. A nil field is an undefined property.
type TerminalLabelElems struct {
	StartLeft, StartRight, EndLeft, EndRight *dom.Node
}

// Get is terminalLabels.get(id)[key] for the keys startLeft, startRight,
// endLeft and endRight. It returns nil for another key.
func (t *TerminalLabelElems) Get(key string) *dom.Node {
	if t == nil {
		return nil
	}
	switch key {
	case "startLeft":
		return t.StartLeft
	case "startRight":
		return t.StartRight
	case "endLeft":
		return t.EndLeft
	case "endRight":
		return t.EndRight
	}
	return nil
}

func (t *TerminalLabelElems) set(key string, el *dom.Node) {
	switch key {
	case "startLeft":
		t.StartLeft = el
	case "startRight":
		t.StartRight = el
	case "endLeft":
		t.EndLeft = el
	case "endRight":
		t.EndRight = el
	}
}

// EdgeLabels is the module-level Map edgeLabels of edges.js, kept per
// render: edge id to the g.edgeLabel element. Upstream only reads it with
// has and get, so a Go map stands for it.
func EdgeLabels(ctx *render.Context) map[string]*dom.Node {
	return ctx.Value("ru.edges.edgeLabels", func() any { return map[string]*dom.Node{} }).(map[string]*dom.Node)
}

// TerminalLabels is the module-level Map terminalLabels of edges.js, kept
// per render: edge id to the groups of its terminal labels.
func TerminalLabels(ctx *render.Context) map[string]*TerminalLabelElems {
	return ctx.Value("ru.edges.terminalLabels", func() any { return map[string]*TerminalLabelElems{} }).(map[string]*TerminalLabelElems)
}

// Clear is clear(): it empties the edgeLabels and terminalLabels maps.
func Clear(ctx *render.Context) {
	clear(EdgeLabels(ctx))
	clear(TerminalLabels(ctx))
}

// ResolveEdgeCurveType is resolveEdgeCurveType(edge.curve): the curve of
// the edge when it is a string, else flowchart.curve of the config. An
// empty edgeCurve stands for a curve that is not a string. The result is ""
// when the config value is not a string either.
func ResolveEdgeCurveType(ctx *render.Context, edgeCurve string) string {
	if edgeCurve != "" {
		return edgeCurve
	}
	curve, _ := ctx.Cfg("flowchart", "curve").(string)
	return curve
}

// HasEdgeLabel is hasEdgeLabel(edge): whether the edge has a label or a
// terminal label.
func HasEdgeLabel(edge *ru.Edge) bool {
	return edge.Label != "" || edge.StartLabelLeft != "" || edge.StartLabelRight != "" ||
		edge.EndLabelLeft != "" || edge.EndLabelRight != ""
}

// GetLabelStyles is getLabelStyles(styleArray) for an array: every style
// with a ";" in front of it. A nil slice stands for undefined and gives "".
// For a string, upstream returns the string itself.
func GetLabelStyles(styleArray []string) string {
	acc := ""
	for _, style := range styleArray {
		acc = acc + ";" + style
	}
	return acc
}

// edgeLabelStyles is styles2String(edge).labelStyles: the label styles of
// cssCompiledStyles, cssStyles and labelStyle of the edge, compiled.
func edgeLabelStyles(edge *ru.Edge) string {
	var all []string
	all = append(all, edge.CSSCompiledStyles...)
	all = append(all, edge.CSSStyles...)
	// normalizeStyleList(edge.labelStyle)
	if edge.LabelStyleIsText {
		if edge.LabelStyleText != "" {
			for _, style := range strings.Split(edge.LabelStyleText, ";") {
				if s := js.TrimSpace(style); s != "" {
					all = append(all, s)
				}
			}
		}
	} else {
		all = append(all, edge.LabelStyle...)
	}
	var labelStyles []string
	for _, style := range ru.Styles2Map(all).Entries() {
		if ru.IsLabelStyle(style.Key) {
			labelStyles = append(labelStyles, style.Join()+" !important")
		}
	}
	return strings.Join(labelStyles, ";")
}

// InsertEdgeLabel is insertEdgeLabel(elem, edge): it creates the label of
// the edge and its terminal labels in elem, records them for
// PositionEdgeLabel, and stores the measured size in edge.Width,
// edge.Height and edge.TerminalLabelSizes. The result is the label element
// (a foreignObject, or a g or text element for SVG labels).
//
// Upstream replaces edge.labelStyle, an array, with the compiled label
// style text. Here edge.LabelStyleText holds that text afterwards,
// edge.LabelStyleIsText is true and edge.LabelStyle is nil. Code that
// copies labelStyle from one edge to another must copy the three fields.
//
// Upstream is an async function, and the layouts call it for several edges
// inside Promise.all. The function marks its awaits for the scheduler of
// the render context (internal/render/async.go): inside ctx.All the calls
// for several edges take turns as the promises of upstream do, so the
// elements come out in the same document order. InsertEdgeLabels is that
// loop.
func InsertEdgeLabel(ctx *render.Context, elem *dom.Node, edge *ru.Edge) (*dom.Node, error) {
	config := ctx.Config
	useHtmlLabels := sanitize.GetEffectiveHtmlLabels(config)
	labelStyles := edgeLabelStyles(edge)
	edge.LabelStyle = nil
	edge.LabelStyleText, edge.LabelStyleIsText = labelStyles, true

	// Create outer g, edgeLabel, this will be positioned after graph layout
	edgeLabel := elem.Insert("g").Attr("class", "edgeLabel")

	// Create inner g, label, this will be positioned now for centering the text
	label := edgeLabel.Insert("g").Attr("class", "label").Attr("data-id", edge.ID)

	isMarkdown := edge.LabelType == "markdown"
	labelElement, err := ru.CreateText(ctx, elem, edge.Label, ru.TextOptions{
		// getLabelStyles(edge.labelStyle): labelStyle is a string here.
		Style:            edge.LabelStyleText,
		UseHtmlLabels:    useHtmlLabels,
		AddSvgBackground: true,
		IsNode:           false,
		Markdown:         isMarkdown,
		// Plain text edge labels auto-wrap at the default width, and so do
		// markdown edge labels.
		Width: ru.Undef,
	})
	if err != nil {
		return nil, err
	}

	if label == nil || labelElement == nil {
		// label.node() is null for an empty selection.
		return nil, typeError("Cannot read properties of null (reading 'appendChild')")
	}
	label.AppendChild(labelElement)

	// Center the label
	var bbox, transformBbox dom.Rect
	if useHtmlLabels {
		div := labelElement.FirstElementChild()
		dv := labelElement
		ctx.AwaitMeasure()
		bbox = div.BoundingClientRect()
		transformBbox = bbox
		dv.Attr("width", bbox.Width)
		dv.Attr("height", bbox.Height)
	} else {
		// For SVG labels, use text element's bbox so the text is centered on the edge
		textEl := labelElement.Select("text")
		ctx.AwaitMeasure()
		bbox = labelElement.BBox()
		if textEl != nil {
			transformBbox = textEl.BBox()
		} else {
			transformBbox = bbox
		}
	}
	label.Attr("transform", ComputeLabelTransform(transformBbox, useHtmlLabels))

	// Make element accessible by id for positioning
	EdgeLabels(ctx)[edge.ID] = edgeLabel

	// Update the abstract data of the edge with the new information about its width and height
	edge.Width = bbox.Width
	edge.Height = bbox.Height

	// insertTerminal is one of the four blocks of upstream that create a
	// terminal label. The blocks differ in the text and in the key only.
	insertTerminal := func(key, text string) error {
		// Create the actual text element
		terminal := elem.Insert("g").Attr("class", "edgeTerminals")
		inner := terminal.Insert("g").Attr("class", "inner")
		// getLabelStyles(edge.labelStyle) || ''
		terminalElement, err := ru.CreateLabel(ctx, inner, text, edge.LabelStyleText, false, false)
		if err != nil {
			return err
		}
		if terminalElement == nil {
			return typeError("Cannot read properties of null (reading 'getBBox')")
		}
		fo := terminalElement
		slBox := terminalElement.BBox()
		if useHtmlLabels {
			div := terminalElement.FirstElementChild()
			dv := terminalElement
			slBox = div.BoundingClientRect()
			dv.Attr("width", slBox.Width)
			dv.Attr("height", slBox.Height)
		}
		inner.Attr("transform", ComputeLabelTransform(slBox, useHtmlLabels))
		terminalLabels := TerminalLabels(ctx)
		if terminalLabels[edge.ID] == nil {
			terminalLabels[edge.ID] = &TerminalLabelElems{}
		}
		terminalLabels[edge.ID].set(key, terminal)
		SetTerminalWidth(ctx, fo, text, slBox)
		recordTerminalLabelSize(edge, key, slBox)
		return nil
	}
	if edge.StartLabelLeft != "" {
		if err := insertTerminal("startLeft", edge.StartLabelLeft); err != nil {
			return nil, err
		}
	}
	if edge.StartLabelRight != "" {
		if err := insertTerminal("startRight", edge.StartLabelRight); err != nil {
			return nil, err
		}
	}
	if edge.EndLabelLeft != "" {
		if err := insertTerminal("endLeft", edge.EndLabelLeft); err != nil {
			return nil, err
		}
	}
	if edge.EndLabelRight != "" {
		if err := insertTerminal("endRight", edge.EndLabelRight); err != nil {
			return nil, err
		}
	}
	// The await of the caller.
	ctx.Await()
	return labelElement, nil
}

// InsertEdgeLabels is
// `await Promise.all(edges.map((edge) => insertEdgeLabel(elem, edge)))`:
// ctx.All with one InsertEdgeLabel call for each edge. The calls take turns
// at their awaits, as the promises of upstream do. A layout whose callback
// does more than this call uses ctx.All itself.
func InsertEdgeLabels(ctx *render.Context, elem *dom.Node, edges []*ru.Edge) error {
	return ctx.All(len(edges), func(i int) error {
		_, err := InsertEdgeLabel(ctx, elem, edges[i])
		return err
	})
}

// recordTerminalLabelSize records the measured size of a terminal label,
// so that a layout can reserve room for it. Every terminal label is
// centered on the origin of its group.
func recordTerminalLabelSize(edge *ru.Edge, key string, box dom.Rect) {
	if edge.TerminalLabelSizes == nil {
		edge.TerminalLabelSizes = map[string]ru.TerminalLabelSize{}
	}
	edge.TerminalLabelSizes[key] = ru.TerminalLabelSize{Width: box.Width, Height: box.Height}
}

// SetTerminalWidth is setTerminalWidth(fo, value, box): with HTML labels
// it sets the CSS width and height of the foreignObject of a terminal
// label. value is the label text and box is its measured size.
func SetTerminalWidth(ctx *render.Context, fo *dom.Node, value string, box dom.Rect) {
	if sanitize.GetEffectiveHtmlLabels(ctx.Config) && fo != nil {
		// Never smaller than the measured text; a fixed 12px height clipped it (#8329).
		fo.Style("width", js.Str(js.Max(float64(js.UTF16Len(value)*9), box.Width))+"px")
		fo.Style("height", js.Str(box.Height)+"px")
	}
}

// Paths is the result of insertEdge: the points that the label position is
// computed from.
type Paths struct {
	// UpdatedPath is paths.updatedPath: the points after clipping, when
	// they differ from the layout points. HasUpdatedPath is false when the
	// property is undefined.
	UpdatedPath    []ru.Point
	HasUpdatedPath bool
	// OriginalPath is paths.originalPath: edge.points. HasOriginalPath is
	// false when the property is undefined.
	OriginalPath    []ru.Point
	HasOriginalPath bool
}

func toUtilsPoints(points []ru.Point) []utils.Point {
	out := make([]utils.Point, len(points))
	for i, p := range points {
		out[i] = utils.Point(p)
	}
	return out
}

// labelPosition is a label position. XRaw and YRaw say that the coordinate
// is edge.x or edge.y as it is, which prints as "undefined" when it is
// undefined. A computed coordinate prints as "NaN" then.
type labelPosition struct {
	x, y       float64
	xRaw, yRaw bool
}

// str prints a coordinate as a template literal does.
func coordStr(v float64, raw bool) string {
	if raw && ru.IsUndef(v) {
		return "undefined"
	}
	return js.Str(v)
}

func resolveEdgeLabelPosition(edge *ru.Edge, paths *Paths) (labelPosition, error) {
	pos := labelPosition{x: edge.X, y: edge.Y, xRaw: true, yRaw: true}
	if paths != nil && paths.HasUpdatedPath {
		updatedMid, err := utils.CalcLabelPosition(toUtilsPoints(paths.UpdatedPath))
		if err != nil {
			return pos, err
		}
		if paths.HasOriginalPath {
			originalMid, err := utils.CalcLabelPosition(toUtilsPoints(paths.OriginalPath))
			if err != nil {
				return pos, err
			}
			pos.x = edge.X + (updatedMid.X - originalMid.X)
			pos.y = edge.Y + (updatedMid.Y - originalMid.Y)
		} else {
			pos.x = updatedMid.X
			pos.y = updatedMid.Y
		}
		pos.xRaw, pos.yRaw = false, false
	}
	return pos, nil
}

// ResolveEdgeLabelPosition is resolveEdgeLabelPosition(edge, paths): where
// to put the label of an edge along its drawn path. paths may be nil
// (undefined). The result is (edge.X, edge.Y) when there is no updated
// path. The error is the one that calcLabelPosition throws for a path
// without a suitable point.
func ResolveEdgeLabelPosition(edge *ru.Edge, paths *Paths) (ru.Point, error) {
	pos, err := resolveEdgeLabelPosition(edge, paths)
	return ru.Point{X: pos.x, Y: pos.y}, err
}

// PositionEdgeLabel is positionEdgeLabel(edge, paths): it moves the label
// and the terminal labels that InsertEdgeLabel created to their place. It
// returns an error where upstream throws: for a nil paths, and for a label
// that InsertEdgeLabel did not create.
func PositionEdgeLabel(ctx *render.Context, edge *ru.Edge, paths *Paths) error {
	siteConfig := ctx.Config
	subGraphTitleTotalMargin := utils.GetSubGraphTitleMargins(siteConfig).SubGraphTitleTotalMargin
	if edge.Label != "" {
		el := EdgeLabels(ctx)[edge.ID]
		pos, err := resolveEdgeLabelPosition(edge, paths)
		if err != nil {
			return err
		}
		if el == nil {
			return typeError("Cannot read properties of undefined (reading 'attr')")
		}
		el.Attr("transform", "translate("+coordStr(pos.x, pos.xRaw)+", "+js.Str(pos.y+float64(subGraphTitleTotalMargin/2))+")")
	}

	// `path` is still needed below for the terminal-label branches, which only read from it
	// (they do not themselves reposition the main label).
	if paths == nil {
		return typeError("Cannot read properties of undefined (reading 'updatedPath')")
	}
	path, hasPath := paths.OriginalPath, paths.HasOriginalPath
	if paths.HasUpdatedPath {
		path, hasPath = paths.UpdatedPath, true
	}

	terminal := func(key, position, text, arrowType string) error {
		if text == "" {
			return nil
		}
		labels := TerminalLabels(ctx)[edge.ID]
		if labels == nil {
			return typeError("Cannot read properties of undefined (reading '%s')", key)
		}
		el := labels.Get(key)
		x, y := edge.X, edge.Y
		raw := true
		if hasPath {
			markerSize := 0.0
			if arrowType != "" {
				markerSize = 10
			}
			pos, err := utils.CalcTerminalLabelPosition(markerSize, position, toUtilsPoints(path))
			if err != nil {
				return err
			}
			x, y = pos.X, pos.Y
			raw = false
		}
		if el == nil {
			return typeError("Cannot read properties of undefined (reading 'attr')")
		}
		el.Attr("transform", "translate("+coordStr(x, raw)+", "+coordStr(y, raw)+")")
		return nil
	}
	if err := terminal("startLeft", "start_left", edge.StartLabelLeft, edge.ArrowTypeStart); err != nil {
		return err
	}
	if err := terminal("startRight", "start_right", edge.StartLabelRight, edge.ArrowTypeStart); err != nil {
		return err
	}
	if err := terminal("endLeft", "end_left", edge.EndLabelLeft, edge.ArrowTypeEnd); err != nil {
		return err
	}
	return terminal("endRight", "end_right", edge.EndLabelRight, edge.ArrowTypeEnd)
}

// orthogonalizeToLabelClippedPoints is a swimlanes-only helper: it
// self-gates to `-to-label` edges (the swimlanes edge-label waypoint
// mechanism).
func orthogonalizeToLabelClippedPoints(edge *ru.Edge, points []ru.Point) []ru.Point {
	if !edge.IsLabelEdge || !strings.HasSuffix(edge.ID, "-to-label") {
		return points
	}

	if len(points) != 2 {
		return points
	}

	start, end := points[0], points[1]
	dx := math.Abs(end.X - start.X)
	dy := math.Abs(end.Y - start.Y)

	if dx < 1e-3 || dy < 1e-3 {
		return points
	}

	if dy >= dx {
		return []ru.Point{start, {X: start.X, Y: end.Y}, end}
	}

	return []ru.Point{start, {X: end.X, Y: start.Y}, end}
}

func outsideNode(node ru.Bounds, point ru.Point) bool {
	x := node.X
	y := node.Y
	dx := math.Abs(point.X - x)
	dy := math.Abs(point.Y - y)
	w := node.Width / 2
	h := node.Height / 2
	return dx >= w || dy >= h
}

// Intersection is intersection(node, outsidePoint, insidePoint): the point
// where the segment between the two points crosses the border of the
// rectangle node (given by its center and size).
func Intersection(node ru.Bounds, outsidePoint, insidePoint ru.Point) ru.Point {
	x := node.X
	y := node.Y

	dx := math.Abs(x - insidePoint.X)
	w := float64(node.Width / 2)
	r := w + dx
	if insidePoint.X < outsidePoint.X {
		r = w - dx
	}
	h := float64(node.Height / 2)

	Q := math.Abs(outsidePoint.Y - insidePoint.Y)
	R := math.Abs(outsidePoint.X - insidePoint.X)

	if math.Abs(y-outsidePoint.Y)*w > math.Abs(x-outsidePoint.X)*h {
		// Intersection is top or bottom of rect.
		q := y - h - outsidePoint.Y
		if insidePoint.Y < outsidePoint.Y {
			q = outsidePoint.Y - h - y
		}
		r = float64(R*q) / Q
		var res ru.Point
		if insidePoint.X < outsidePoint.X {
			res.X = insidePoint.X + r
		} else {
			res.X = insidePoint.X - R + r
		}
		if insidePoint.Y < outsidePoint.Y {
			res.Y = insidePoint.Y + Q - q
		} else {
			res.Y = insidePoint.Y - Q + q
		}

		if r == 0 {
			res.X = outsidePoint.X
			res.Y = outsidePoint.Y
		}
		if R == 0 {
			res.X = outsidePoint.X
		}
		if Q == 0 {
			res.Y = outsidePoint.Y
		}

		return res
	}
	// Intersection on sides of rect
	if insidePoint.X < outsidePoint.X {
		r = outsidePoint.X - w - x
	} else {
		r = x - w - outsidePoint.X
	}
	q := float64(Q*r) / R
	var _x, _y float64
	if insidePoint.X < outsidePoint.X {
		_x = insidePoint.X + R - r
	} else {
		_x = insidePoint.X - R + r
	}
	if insidePoint.Y < outsidePoint.Y {
		_y = insidePoint.Y + q
	} else {
		_y = insidePoint.Y - q
	}
	if r == 0 {
		_x = outsidePoint.X
		_y = outsidePoint.Y
	}
	if R == 0 {
		_x = outsidePoint.X
	}
	if Q == 0 {
		_y = outsidePoint.Y
	}

	return ru.Point{X: _x, Y: _y}
}

func cutPathAtIntersect(_points []ru.Point, boundaryNode ru.Bounds) []ru.Point {
	points := []ru.Point{}
	if len(_points) == 0 {
		// _points[0] is undefined, and nothing reads it.
		return points
	}
	lastPointOutside := _points[0]
	isInside := false
	for _, point := range _points {
		if !outsideNode(boundaryNode, point) && !isInside {
			inter := Intersection(boundaryNode, lastPointOutside, point)

			some := false
			for _, e := range points {
				if e.X == inter.X && e.Y == inter.Y {
					some = true
					break
				}
			}
			if !some {
				points = append(points, inter)
			}
			isInside = true
		} else {
			lastPointOutside = point
			if !isInside {
				points = append(points, point)
			}
		}
	}
	return points
}

func extractCornerPoints(points []ru.Point) (cornerPoints []ru.Point, cornerPointPositions []int) {
	for i := 1; i < len(points)-1; i++ {
		prev := points[i-1]
		curr := points[i]
		next := points[i+1]
		if prev.X == curr.X &&
			curr.Y == next.Y &&
			math.Abs(curr.X-next.X) > 5 &&
			math.Abs(curr.Y-prev.Y) > 5 {
			cornerPoints = append(cornerPoints, curr)
			cornerPointPositions = append(cornerPointPositions, i)
		} else if prev.Y == curr.Y &&
			curr.X == next.X &&
			math.Abs(curr.X-prev.X) > 5 &&
			math.Abs(curr.Y-next.Y) > 5 {
			cornerPoints = append(cornerPoints, curr)
			cornerPointPositions = append(cornerPointPositions, i)
		}
	}
	return cornerPoints, cornerPointPositions
}

func findAdjacentPoint(pointA, pointB ru.Point, distance float64) ru.Point {
	xDiff := pointB.X - pointA.X
	yDiff := pointB.Y - pointA.Y
	length := math.Sqrt(float64(xDiff*xDiff) + float64(yDiff*yDiff))
	ratio := distance / length
	return ru.Point{X: pointB.X - float64(ratio*xDiff), Y: pointB.Y - float64(ratio*yDiff)}
}

func fixCorners(lineData []ru.Point) []ru.Point {
	_, cornerPointPositions := extractCornerPoints(lineData)
	includes := func(i int) bool {
		for _, p := range cornerPointPositions {
			if p == i {
				return true
			}
		}
		return false
	}
	newLineData := []ru.Point{}
	for i := 0; i < len(lineData); i++ {
		if includes(i) {
			prevPoint := lineData[i-1]
			nextPoint := lineData[i+1]
			cornerPoint := lineData[i]

			newPrevPoint := findAdjacentPoint(prevPoint, cornerPoint, 5)
			newNextPoint := findAdjacentPoint(nextPoint, cornerPoint, 5)

			xDiff := newNextPoint.X - newPrevPoint.X
			yDiff := newNextPoint.Y - newPrevPoint.Y
			newLineData = append(newLineData, newPrevPoint)

			a := math.Sqrt(2) * 2
			newCornerPoint := ru.Point{X: cornerPoint.X, Y: cornerPoint.Y}
			if math.Abs(nextPoint.X-prevPoint.X) > 10 && math.Abs(nextPoint.Y-prevPoint.Y) >= 10 {
				const r = 5
				if cornerPoint.X == newPrevPoint.X {
					if xDiff < 0 {
						newCornerPoint.X = newPrevPoint.X - r + a
					} else {
						newCornerPoint.X = newPrevPoint.X + r - a
					}
					if yDiff < 0 {
						newCornerPoint.Y = newPrevPoint.Y - a
					} else {
						newCornerPoint.Y = newPrevPoint.Y + a
					}
				} else {
					if xDiff < 0 {
						newCornerPoint.X = newPrevPoint.X - a
					} else {
						newCornerPoint.X = newPrevPoint.X + a
					}
					if yDiff < 0 {
						newCornerPoint.Y = newPrevPoint.Y - r + a
					} else {
						newCornerPoint.Y = newPrevPoint.Y + r - a
					}
				}
			}
			newLineData = append(newLineData, newCornerPoint, newNextPoint)
		} else {
			newLineData = append(newLineData, lineData[i])
		}
	}
	return newLineData
}

func generateDashArray(length, oValueS, oValueE float64) string {
	middleLength := length - oValueS - oValueE
	const dashLength = 2 // Length of each dash
	const gapLength = 2  // Length of each gap
	const dashGapPairLength = dashLength + gapLength

	// Calculate number of complete dash-gap pairs that can fit. Clamp to a
	// non-negative, finite integer.
	rawPairs := math.Floor(middleLength / dashGapPairLength)
	numberOfPairs := 0
	if !math.IsNaN(rawPairs) && !math.IsInf(rawPairs, 0) {
		// A path cannot be long enough to overflow the int.
		numberOfPairs = int(js.Max(0, rawPairs))
	}

	// Generate the middle pattern array
	pairs := make([]string, numberOfPairs)
	for i := range pairs {
		pairs[i] = "2 2"
	}
	middlePattern := strings.Join(pairs, " ")

	// Combine all parts
	return "0 " + js.Str(oValueS) + " " + middlePattern + " " + js.Str(oValueE)
}

// ClusterDb is the part of the clusterDb Map that insertEdge reads:
// clusterDb.get(id).node, the laid-out cluster node, as its center and
// size. ok is false when the map has no entry for the id or the entry has
// no node; upstream throws a TypeError then, and InsertEdge returns an
// error. A nil ClusterDb is an empty Map.
type ClusterDb func(id string) (node ru.Bounds, ok bool)

func (db ClusterDb) node(id string) (ru.Bounds, error) {
	if db != nil {
		if node, ok := db(id); ok {
			return node, nil
		}
	}
	return ru.Bounds{}, typeError("Cannot read properties of undefined (reading 'node')")
}

// pageLocation is window.location of the page of the reference renderer.
var pageLocation = sanitize.Location{Protocol: "about:", Host: "", Pathname: "blank", Search: ""}

// SetPageLocation sets window.location for the render. InsertEdge reads it
// for the marker urls when arrowMarkerAbsolute is set. The default is the
// about:blank page of the reference renderer.
func SetPageLocation(ctx *render.Context, location sanitize.Location) {
	*ctx.Value("ru.edges.pageLocation", func() any { l := pageLocation; return &l }).(*sanitize.Location) = location
}

func getPageLocation(ctx *render.Context) sanitize.Location {
	return *ctx.Value("ru.edges.pageLocation", func() any { l := pageLocation; return &l }).(*sanitize.Location)
}

// pointsJSON is JSON.stringify(points) for points that have the properties
// x and y. A NaN or an infinite number is null.
func pointsJSON(points []ru.Point) string {
	num := func(v float64) string {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "null"
		}
		return js.Str(v)
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, p := range points {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`{"x":` + num(p.X) + `,"y":` + num(p.Y) + `}`)
	}
	sb.WriteByte(']')
	return sb.String()
}

var reStrokeColor = regexp.MustCompile(`stroke:([^;]+)`)

func reverse(points []ru.Point) {
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}
}

// InsertEdge is insertEdge(elem, edge, clusterDb, diagramType, startNode,
// endNode, diagramId, skipIntersect, layoutAlgorithm): it draws the path of
// the edge into elem with its markers and returns the paths for
// PositionEdgeLabel.
//
// startNode and endNode are the laid-out nodes of the edge; only their
// Intersect function is used. A nil node is the empty object that the
// common layout passes for an unknown node. An empty layoutAlgorithm stands
// for undefined: the layout of the config is used then.
//
// The result shares edge.Points (OriginalPath), as upstream does. In one
// case upstream reverses edge.points in place, and so does InsertEdge: an
// edge with fromCluster and without toCluster whose points were not
// clipped to the nodes.
func InsertEdge(ctx *render.Context, elem *dom.Node, edge *ru.Edge, clusterDb ClusterDb, diagramType string, startNode, endNode *ru.Node, diagramID string, skipIntersect bool, layoutAlgorithm string) (*Paths, error) {
	if diagramID == "" {
		// The text is the error message of upstream.
		return nil, fmt.Errorf("insertEdge: missing diagramId for edge \"%s\" \u2014 edge IDs require a diagram prefix for uniqueness", edge.ID)
	}
	handDrawnSeed := ctx.Cfg("handDrawnSeed")
	layout := layoutAlgorithm
	if layout == "" {
		layout, _ = ctx.Cfg("layout").(string)
	}
	points := edge.Points
	pointsHasChanged := false
	tail := startNode
	head := endNode
	var edgeClassStyles []string
	// `for (const key in edge.cssCompiledStyles)`: the keys are the array
	// indexes, and no index is a label style.
	edgeClassStyles = append(edgeClassStyles, edge.CSSCompiledStyles...)
	hasIntersect := head != nil && head.Intersect != nil && tail != nil && tail.Intersect != nil

	// Edge endpoint clipping. The swimlanes layout produces orthogonal edges whose
	// axis-aligned entry/exit segments must be preserved, so it uses a dedicated
	// boundary-clipping path. Every other layout (dagre, ELK and so on) keeps the
	// original clipping below, so their edge ports are unaffected by swimlanes.
	if layout == "swimlane" {
		if !skipIntersect && hasIntersect && len(points) >= 2 {
			if len(points) == 2 {
				// Simple straight edge: just clip the two endpoints to the node boundaries.
				points = []ru.Point{tail.Intersect(points[0]), head.Intersect(points[1])}
			} else {
				// For multi-segment paths, keep the inner bend points and just adjust the entry/exit
				// segments near the nodes.
				innerPoints := append([]ru.Point{}, points[1:len(points)-1]...)
				firstInner := innerPoints[0]
				lastInner := innerPoints[len(innerPoints)-1]
				const TOLERANCE = 0.5
				lastIsPinned := math.Abs(points[len(points)-1].X-lastInner.X) < TOLERANCE &&
					math.Abs(points[len(points)-1].Y-lastInner.Y) < TOLERANCE

				newFirst := tail.Intersect(firstInner)
				newLast := lastInner
				if !lastIsPinned {
					newLast = head.Intersect(lastInner)
				}

				// When the boundary intersection lands ~on the inner point, skip it to
				// avoid a zero-length final segment (keeps the entry/exit segment orthogonal).
				lastIsDuplicate := math.Abs(newLast.X-lastInner.X) < TOLERANCE &&
					math.Abs(newLast.Y-lastInner.Y) < TOLERANCE
				firstIsDuplicate := math.Abs(newFirst.X-firstInner.X) < TOLERANCE &&
					math.Abs(newFirst.Y-firstInner.Y) < TOLERANCE

				newPoints := []ru.Point{}
				if !firstIsDuplicate {
					newPoints = append(newPoints, newFirst)
				}
				newPoints = append(newPoints, innerPoints...)
				if !lastIsDuplicate {
					newPoints = append(newPoints, newLast)
				}
				points = newPoints
			}
		}
		points = orthogonalizeToLabelClippedPoints(edge, points)
	} else if hasIntersect && !skipIntersect {
		// Original clipping, unchanged for dagre, ELK and every non-swimlanes layout.
		// points.slice(1, edge.points.length - 1)
		end := max(len(edge.Points)-1, 0)
		start := min(1, len(points))
		if end < start {
			end = start
		}
		points = append([]ru.Point{}, points[start:end]...)
		if len(points) == 0 {
			// tail.intersect(undefined)
			return nil, typeError("Cannot read properties of undefined (reading 'x')")
		}
		points = append([]ru.Point{tail.Intersect(points[0])}, points...)
		points = append(points, head.Intersect(points[len(points)-1]))
	}
	pointsStr := base64.StdEncoding.EncodeToString([]byte(pointsJSON(points)))
	if edge.ToCluster != "" {
		node, err := clusterDb.node(edge.ToCluster)
		if err != nil {
			return nil, err
		}
		points = cutPathAtIntersect(edge.Points, node)

		pointsHasChanged = true
	}

	if edge.FromCluster != "" {
		node, err := clusterDb.node(edge.FromCluster)
		if err != nil {
			return nil, err
		}
		// points.reverse() reverses in place. When points is still
		// edge.points, the edge sees it.
		reverse(points)
		points = cutPathAtIntersect(points, node)
		reverse(points)

		pointsHasChanged = true
	}

	lineData := []ru.Point{}
	for _, p := range points {
		if !math.IsNaN(p.Y) {
			lineData = append(lineData, p)
		}
	}
	// Resolve curve type: use edge.curve if it's a string, otherwise fall back to config default
	edgeCurveType := ResolveEdgeCurveType(ctx, edge.Curve)
	// Apply fixCorners for non-rounded curves to pre-round right-angle corners
	// (rounded curve type uses generateRoundedPath instead)
	if edgeCurveType != "rounded" {
		lineData = fixCorners(lineData)
	}
	var curve shape.CurveFactory
	switch edgeCurveType {
	case "linear":
		curve = shape.CurveLinear
	case "basis":
		curve = shape.CurveBasis
	case "cardinal":
		curve = shape.CurveCardinal
	case "bumpX":
		curve = shape.CurveBumpX
	case "bumpY":
		curve = shape.CurveBumpY
	case "catmullRom":
		curve = shape.CurveCatmullRom
	case "monotoneX":
		curve = shape.CurveMonotoneX
	case "monotoneY":
		curve = shape.CurveMonotoneY
	case "natural":
		curve = shape.CurveNatural
	case "step":
		curve = shape.CurveStep
	case "stepAfter":
		curve = shape.CurveStepAfter
	case "stepBefore":
		curve = shape.CurveStepBefore
	case "rounded":
		curve = shape.CurveLinear
	default:
		curve = shape.CurveBasis
	}

	fns := utils.GetLineFunctionsWithOffset(edge.ArrowTypeStart, edge.ArrowTypeEnd)
	lineFunction := shape.NewLine[utils.Point]().X(fns.X).Y(fns.Y).Curve(curve)

	var strokeClasses string
	switch edge.Thickness {
	case "normal":
		strokeClasses = "edge-thickness-normal"
	case "thick":
		strokeClasses = "edge-thickness-thick"
	case "invisible":
		strokeClasses = "edge-thickness-invisible"
	default:
		strokeClasses = "edge-thickness-normal"
	}
	switch edge.Pattern {
	case "solid":
		strokeClasses += " edge-pattern-solid"
	case "dotted":
		strokeClasses += " edge-pattern-dotted"
	case "dashed":
		strokeClasses += " edge-pattern-dashed"
	default:
		strokeClasses += " edge-pattern-solid"
	}
	var svgPath *dom.Node
	// The line function returns null for a line without commands. The
	// rounded path of less than two points is the empty string.
	var linePath string
	linePathIsNull := false
	if edgeCurveType == "rounded" {
		linePath = GenerateRoundedPath(ApplyMarkerOffsetsToPoints(lineData, edge.ArrowTypeStart, edge.ArrowTypeEnd), 5)
	} else {
		linePath = lineFunction.Path(toUtilsPoints(lineData))
		linePathIsNull = linePath == ""
	}
	// An undefined style is the array [undefined], and its element prints
	// as the text "undefined".
	edgeStyles := edge.Style
	if edge.StyleUndefined {
		edgeStyles = []string{"undefined"}
	}
	strokeColor := ""
	if !edge.StyleUndefined {
		for _, style := range edgeStyles {
			if strings.HasPrefix(style, "stroke:") {
				strokeColor = style
				break
			}
		}
	}
	// stylesAfter is edgeStyles.reduce((acc, style) => acc + ';' + style, '').
	stylesAfter := ""
	for _, style := range edgeStyles {
		stylesAfter = stylesAfter + ";" + style
	}

	animationClass := ""
	if edge.Animate {
		animationClass = "edge-animation-fast"
	}
	if edge.Animation != "" {
		animationClass = "edge-animation-" + edge.Animation
	}
	// classAttr is the class attribute of the path.
	classAttr := func() string {
		class := " " + strokeClasses
		if edge.Classes != "" {
			class += " " + edge.Classes
		}
		if animationClass != "" {
			class += " " + animationClass
		}
		return class
	}

	animatedEdge := false
	if edge.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, elem)

		options := js.NewObj[any]()
		options.Set("roughness", 0.3)
		options.Set("seed", handDrawnSeed)
		svgPathNode, err := rc.Path(linePath, options)
		if err != nil {
			return nil, err
		}

		strokeClasses += " transition"

		svgPath = svgPathNode.Select("path").
			Attr("id", diagramID+"-"+edge.ID).
			Attr("class", classAttr()).
			Attr("style", stylesAfter)
		if svgPath == nil {
			// svgPath.attr('d') of an empty selection.
			return nil, typeError("Cannot read properties of null (reading 'getAttribute')")
		}
		if d, ok := svgPath.GetAttr("d"); ok {
			svgPath.Attr("d", d)
		} else {
			svgPath.Attr("d", nil)
		}
		elem.AppendChild(svgPath)
	} else {
		stylesFromClasses := strings.Join(edgeClassStyles, ";")
		styles := ""
		for _, style := range edgeStyles {
			styles = styles + style + ";"
		}

		pathStyle := styles
		if stylesFromClasses != "" {
			pathStyle = stylesFromClasses + ";" + styles + ";"
		}
		pathStyle = pathStyle + ";" + stylesAfter
		svgPath = elem.Append("path")
		if !linePathIsNull {
			svgPath.Attr("d", linePath)
		}
		svgPath.
			Attr("id", diagramID+"-"+edge.ID).
			Attr("class", classAttr()).
			Attr("style", pathStyle)

		strokeColor = ""
		if m := reStrokeColor.FindStringSubmatch(pathStyle); m != nil {
			strokeColor = m[1]
		}

		animatedEdge = edge.Animate || edge.Animation != "" || strings.Contains(stylesFromClasses, "animation")
		length := svgPath.TotalLength()
		oValueS, _ := utils.MarkerOffset2(edge.ArrowTypeStart)
		oValueE, _ := utils.MarkerOffset2(edge.ArrowTypeEnd)

		if edge.Look == "neo" && !animatedEdge {
			var dashArray string
			if edge.Pattern == "dotted" || edge.Pattern == "dashed" {
				dashArray = generateDashArray(length, oValueS, oValueE)
			} else {
				dashArray = "0 " + js.Str(oValueS) + " " + js.Str(length-oValueS-oValueE) + " " + js.Str(oValueE)
			}

			// No offset needed because we already start with a zero-length dash that effectively sets us up for a gap at the start.
			mOffset := "stroke-dasharray: " + dashArray + "; stroke-dashoffset: 0;"
			svgPath.Attr("style", mOffset+svgPath.AttrOr("style", "null"))
		}
	}

	// MC Special
	svgPath.Attr("data-edge", "true")
	svgPath.Attr("data-et", "edge")
	svgPath.Attr("data-id", edge.ID)
	svgPath.Attr("data-points", pointsStr)
	// Add data attributes for neo look support
	if edge.Look != "" {
		svgPath.Attr("data-look", edge.Look)
	} else {
		svgPath.Attr("data-look", nil)
	}
	if edge.ShowPoints {
		for _, point := range lineData {
			elem.
				Append("circle").
				Style("stroke", "red").
				Style("fill", "red").
				Attr("r", 1).
				Attr("cx", point.X).
				Attr("cy", point.Y)
		}
	}

	url := ""
	if ctx.CfgBool("flowchart", "arrowMarkerAbsolute") || ctx.CfgBool("state", "arrowMarkerAbsolute") {
		location := getPageLocation(ctx)
		url = location.Protocol + "//" + location.Host + location.Pathname + location.Search
		url = strings.ReplaceAll(strings.ReplaceAll(url, "(", `\(`), ")", `\)`)
	}

	useMargin := !animatedEdge && edge.Look == "neo"
	AddEdgeMarkers(ctx, svgPath, edge.ArrowTypeStart, edge.ArrowTypeEnd, url, diagramID, diagramType, useMargin, strokeColor)
	midIndex := len(points) / 2
	if midIndex >= len(points) {
		// points[midIndex] is undefined, and isLabelCoordinateInPath reads it.
		return nil, typeError("Cannot read properties of undefined (reading 'x')")
	}
	point := points[midIndex]
	d, ok := svgPath.GetAttr("d")
	if !ok {
		// isLabelCoordinateInPath(point, null)
		return nil, typeError("Cannot read properties of null (reading 'replace')")
	}
	if !utils.IsLabelCoordinateInPath(utils.Point(point), d) {
		pointsHasChanged = true
	}

	paths := &Paths{}
	if pointsHasChanged {
		paths.UpdatedPath, paths.HasUpdatedPath = points, true
	}
	paths.OriginalPath, paths.HasOriginalPath = edge.Points, true
	return paths, nil
}

// GenerateRoundedPath is generateRoundedPath(points, radius): SVG path
// data with rounded corners through the points.
func GenerateRoundedPath(points []ru.Point, radius float64) string {
	if len(points) < 2 {
		return ""
	}

	var path strings.Builder
	size := len(points)
	const epsilon = 1e-5

	for i := 0; i < size; i++ {
		currPoint := points[i]

		if i == 0 {
			// Move to the first point
			path.WriteString("M" + js.Str(currPoint.X) + "," + js.Str(currPoint.Y))
		} else if i == size-1 {
			// Last point, draw a straight line to the final point
			path.WriteString("L" + js.Str(currPoint.X) + "," + js.Str(currPoint.Y))
		} else {
			prevPoint := points[i-1]
			nextPoint := points[i+1]
			// Calculate vectors for incoming and outgoing segments
			dx1 := currPoint.X - prevPoint.X
			dy1 := currPoint.Y - prevPoint.Y
			dx2 := nextPoint.X - currPoint.X
			dy2 := nextPoint.Y - currPoint.Y

			len1 := js.Hypot(dx1, dy1)
			len2 := js.Hypot(dx2, dy2)

			// Prevent division by zero
			if len1 < epsilon || len2 < epsilon {
				path.WriteString("L" + js.Str(currPoint.X) + "," + js.Str(currPoint.Y))
				continue
			}

			// Normalize the vectors
			nx1 := dx1 / len1
			ny1 := dy1 / len1
			nx2 := dx2 / len2
			ny2 := dy2 / len2

			// Calculate the angle between the vectors
			dot := float64(nx1*nx2) + float64(ny1*ny2)
			// Clamp the dot product to avoid numerical issues with acos
			clampedDot := js.Max(-1, js.Min(1, dot))
			angle := acos(clampedDot)

			// Skip rounding if the angle is too small or too close to 180 degrees
			if angle < epsilon || math.Abs(math.Pi-angle) < epsilon {
				path.WriteString("L" + js.Str(currPoint.X) + "," + js.Str(currPoint.Y))
				continue
			}

			// Calculate the distance to offset the control point
			cutLen := js.Min(radius/sin(angle/2), len1/2, len2/2)

			// Calculate the start and end points of the curve
			startX := currPoint.X - float64(nx1*cutLen)
			startY := currPoint.Y - float64(ny1*cutLen)
			endX := currPoint.X + float64(nx2*cutLen)
			endY := currPoint.Y + float64(ny2*cutLen)

			// Draw the line to the start of the curve
			path.WriteString("L" + js.Str(startX) + "," + js.Str(startY))

			// Draw the quadratic Bezier curve
			path.WriteString("Q" + js.Str(currPoint.X) + "," + js.Str(currPoint.Y) + " " + js.Str(endX) + "," + js.Str(endY))
		}
	}

	return path.String()
}

// calculateDeltaAndAngle calculates the delta and the angle between two
// points.
func calculateDeltaAndAngle(point1, point2 ru.Point) (angle, deltaX, deltaY float64) {
	deltaX = point2.X - point1.X
	deltaY = point2.Y - point1.Y
	angle = atan2(deltaY, deltaX)
	return angle, deltaX, deltaY
}

// markerOffset is markerOffsets[arrowType] when it is truthy.
func markerOffset(arrowType string) (float64, bool) {
	v, ok := utils.MarkerOffset(arrowType)
	return v, ok && v != 0
}

// ApplyMarkerOffsetsToPoints is applyMarkerOffsetsToPoints(points, edge):
// a copy of the points with the first and the last point moved along the
// edge by the offset of the start and end markers. The arguments are
// edge.arrowTypeStart and edge.arrowTypeEnd.
func ApplyMarkerOffsetsToPoints(points []ru.Point, arrowTypeStart, arrowTypeEnd string) []ru.Point {
	// Copy the points array to avoid mutating the original data
	newPoints := append([]ru.Point{}, points...)

	// Handle the first point (start of the edge)
	if offsetValue, ok := markerOffset(arrowTypeStart); len(points) >= 2 && ok {
		point1 := points[0]
		point2 := points[1]

		angle, _, _ := calculateDeltaAndAngle(point1, point2)

		offsetX := float64(offsetValue * cos(angle))
		offsetY := float64(offsetValue * sin(angle))

		newPoints[0].X = point1.X + offsetX
		newPoints[0].Y = point1.Y + offsetY
	}

	// Handle the last point (end of the edge)
	n := len(points)
	if offsetValue, ok := markerOffset(arrowTypeEnd); n >= 2 && ok {
		point1 := points[n-1]
		point2 := points[n-2]

		angle, _, _ := calculateDeltaAndAngle(point2, point1)

		offsetX := float64(offsetValue * cos(angle))
		offsetY := float64(offsetValue * sin(angle))

		newPoints[n-1].X = point1.X - offsetX
		newPoints[n-1].Y = point1.Y - offsetY
	}

	return newPoints
}
