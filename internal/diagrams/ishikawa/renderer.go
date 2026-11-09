// Ported from mermaid 12.2.0, diagrams/ishikawa/ishikawaRenderer.ts (MIT).

package ishikawa

import (
	"math"
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// roughContext is RoughContext. The seed and the colors are dynamic values
// of the config: nil is undefined.
type roughContext struct {
	roughSvg  *ru.RoughSVG
	seed      any
	lineColor any
	fillColor any
}

const (
	fontSizeDefault = 14
	spineBaseLength = 250
	boneStub        = 30
	boneBase        = 60
	bonePerChild    = 5
)

// angle, cosA and sinA are ANGLE, COS_A and SIN_A. The operands are
// variables, so that the arithmetic is the float64 arithmetic of JavaScript
// and not the exact arithmetic of Go constants.
var (
	angle = jsAngle(82, math.Pi)
	cosA  = js.Cos(angle)
	sinA  = js.Sin(angle)
)

// jsAngle is (degrees * Math.PI) / 180.
func jsAngle(degrees, pi float64) float64 {
	return degrees * pi / 180
}

// orDefault is value ?? fallback.
func orDefault(value, fallback any) any {
	if js.IsNullish(value) {
		return fallback
	}
	return value
}

// applyPaddedViewBox is applyPaddedViewBox(svgEl, pad, maxW). pad and maxW
// are dynamic values of the config.
func applyPaddedViewBox(svgEl *dom.Node, pad any, maxW any) {
	padNum := js.ToNumber(pad)
	bbox := svgEl.BBox()
	w := bbox.Width + float64(padNum*2)
	h := bbox.Height + float64(padNum*2)
	render.ConfigureSvgSize(svgEl, h, w, js.Truthy(maxW))
	svgEl.Attr("viewBox", js.Str(bbox.X-padNum)+" "+js.Str(bbox.Y-padNum)+" "+js.Str(w)+" "+js.Str(h))
}

// drawIshikawa is draw.
func drawIshikawa(ctx *render.Context, db *DB) error {
	root := db.GetRoot()
	if root == nil {
		return nil
	}

	drawConfig := ctx.Config
	look := drawConfig.Get("look")
	handDrawnSeed := drawConfig.Get("handDrawnSeed")
	themeVariables := drawConfig.Get("themeVariables")
	fontSize := float64(fontSizeDefault)
	if n, _, ok := utils.ParseFontSize(drawConfig.Get("fontSize")); ok {
		fontSize = n
	}
	isHandDrawn := js.StrictEquals(look, "handDrawn")

	causes := root.Children
	padding := orDefault(js.Path(drawConfig, "ishikawa", "diagramPadding"), 20.0)
	useMaxWidth := orDefault(js.Path(drawConfig, "ishikawa", "useMaxWidth"), false)
	svg := ctx.SelectSvgElement()
	g := svg.Append("g").Attr("class", "ishikawa")

	var rc *roughContext
	if isHandDrawn {
		rc = &roughContext{
			roughSvg:  ru.NewRoughSVG(ctx, svg),
			seed:      orDefault(handDrawnSeed, 0.0),
			lineColor: orDefault(js.Path(themeVariables, "lineColor"), "#333"),
			fillColor: orDefault(js.Path(themeVariables, "mainBkg"), "#fff"),
		}
	}

	markerID := "ishikawa-arrow-" + ctx.ID
	if !isHandDrawn {
		g.Append("defs").
			Append("marker").
			Attr("id", markerID).
			Attr("viewBox", "0 0 10 10").
			Attr("refX", 0.0).
			Attr("refY", 5.0).
			Attr("markerWidth", 6.0).
			Attr("markerHeight", 6.0).
			Attr("orient", "auto").
			Append("path").
			Attr("d", "M 10 0 L 0 5 L 10 10 Z").
			Attr("class", "ishikawa-arrow")
	}

	spineX := 0.0
	spineY := float64(spineBaseLength)

	// For handDrawn, defer spine drawing until coordinates are final
	var spineLine *dom.Node
	if !isHandDrawn {
		var err error
		spineLine, err = drawLine(g, spineX, spineY, spineX, spineY, "ishikawa-spine", nil)
		if err != nil {
			return err
		}
	}
	if err := drawHead(g, spineX, spineY, root.Text, fontSize, rc); err != nil {
		return err
	}

	if len(causes) == 0 {
		if isHandDrawn {
			if _, err := drawLine(g, spineX, spineY, spineX, spineY, "ishikawa-spine", rc); err != nil {
				return err
			}
		}
		applyPaddedViewBox(svg, padding, useMaxWidth)
		return nil
	}

	spineX -= 20

	var upperCauses, lowerCauses []*Node
	for i, cause := range causes {
		if i%2 == 0 {
			upperCauses = append(upperCauses, cause)
		} else {
			lowerCauses = append(lowerCauses, cause)
		}
	}

	upperStats := sideStats(upperCauses)
	lowerStats := sideStats(lowerCauses)
	descendantTotal := upperStats.total + lowerStats.total

	upperLen := float64(spineBaseLength)
	lowerLen := float64(spineBaseLength)
	if descendantTotal > 0 {
		base := float64(spineBaseLength)
		pool := base * 2
		minLen := base * 0.3
		upperLen = js.Max(minLen, pool*(upperStats.total/descendantTotal))
		lowerLen = js.Max(minLen, pool*(lowerStats.total/descendantTotal))
	}

	minSpacing := fontSize * 2
	upperLen = js.Max(upperLen, upperStats.max*minSpacing)
	lowerLen = js.Max(lowerLen, lowerStats.max*minSpacing)

	spineY = js.Max(upperLen, spineBaseLength)
	if spineLine != nil {
		spineLine.Attr("y1", spineY).Attr("y2", spineY)
	}
	g.Select(".ishikawa-head-group").Attr("transform", "translate(0,"+js.Str(spineY)+")")

	pairCount := (len(causes) + 1) / 2
	for p := 0; p < pairCount; p++ {
		pg := g.Append("g").Attr("class", "ishikawa-pair")
		if cause := causes[p*2]; cause != nil {
			if err := drawBranch(pg, cause, spineX, spineY, -1, upperLen, fontSize, rc); err != nil {
				return err
			}
		}
		if p*2+1 < len(causes) {
			if cause := causes[p*2+1]; cause != nil {
				if err := drawBranch(pg, cause, spineX, spineY, 1, lowerLen, fontSize, rc); err != nil {
					return err
				}
			}
		}
		left := math.Inf(1)
		for _, n := range pg.SelectAll("text") {
			left = js.Min(left, n.BBox().X)
		}
		spineX = left
	}

	if isHandDrawn {
		if _, err := drawLine(g, spineX, spineY, 0, spineY, "ishikawa-spine", rc); err != nil {
			return err
		}
	} else {
		spineLine.Attr("x1", spineX)
		markerURL := "url(#" + markerID + ")"
		g.SelectAll("line.ishikawa-branch, line.ishikawa-sub-branch").Attr("marker-start", markerURL)
	}
	applyPaddedViewBox(svg, padding, useMaxWidth)
	return nil
}

// stats is the result of sideStats.
type stats struct {
	total float64
	max   float64
}

// sideStats is sideStats(nodes).
func sideStats(nodes []*Node) stats {
	var countDescendants func(node *Node) float64
	countDescendants = func(node *Node) float64 {
		sum := 0.0
		for _, child := range node.Children {
			sum = sum + 1 + countDescendants(child)
		}
		return sum
	}

	s := stats{}
	for _, node := range nodes {
		descendants := countDescendants(node)
		s.total += descendants
		s.max = js.Max(s.max, descendants)
	}
	return s
}

// drawHead is drawHead(svg, x, y, label, fontSize, roughContext).
func drawHead(svg *dom.Node, x, y float64, label string, fontSize float64, rc *roughContext) error {
	maxChars := js.Max(6, math.Floor(110/(fontSize*0.6)))
	headGroup := svg.
		Append("g").
		Attr("class", "ishikawa-head-group").
		Attr("transform", "translate("+js.Str(x)+","+js.Str(y)+")")

	textEl := drawMultilineText(
		headGroup,
		wrapText(label, maxChars),
		0,
		0,
		"ishikawa-head-label",
		"start",
		fontSize,
	)
	tb := textEl.BBox()
	w := js.Max(60, tb.Width+6)
	h := js.Max(40, float64(tb.Height*2)+40)

	headPath := "M 0 " + js.Str(-h/2) + " L 0 " + js.Str(h/2) + " Q " + js.Str(w*2.4) + " 0 0 " + js.Str(-h/2) + " Z"
	if rc != nil {
		o := js.NewObj[any]()
		o.Set("roughness", 1.5)
		o.Set("seed", rc.seed)
		o.Set("fill", rc.fillColor)
		o.Set("fillStyle", "hachure")
		o.Set("fillWeight", 2.5)
		o.Set("hachureGap", 5.0)
		o.Set("stroke", rc.lineColor)
		o.Set("strokeWidth", 2.0)
		roughNode, err := rc.roughSvg.Path(headPath, o)
		if err != nil {
			return err
		}
		headGroup.InsertNode(roughNode, ":first-child").Attr("class", "ishikawa-head")
	} else {
		headGroup.Insert("path", ":first-child").Attr("class", "ishikawa-head").Attr("d", headPath)
	}
	textEl.Attr("transform", "translate("+js.Str(float64((w-tb.Width)/2)-tb.X+3)+","+js.Str(-tb.Y-float64(tb.Height/2))+")")
	return nil
}

// labelEntry is LabelEntry.
type labelEntry struct {
	text        string
	depth       int
	parentIndex int
	childCount  int
}

// boneInfo is BoneInfo.
type boneInfo struct {
	x0, y0, x1, y1 float64
	childCount     int
	childrenDrawn  int
}

// flattenTree is flattenTree(children, direction).
//
// Flatten children so we can assign Y positions without recursion when drawing.
// Even depths are placed in pre-order (close to the spine), odd depths in post-order
// to keep diagonal bones within their parent wedge.
func flattenTree(children []*Node, direction float64) (entries []labelEntry, yOrder []int) {
	var walk func(nodes []*Node, pid, depth int)
	walk = func(nodes []*Node, pid, depth int) {
		ordered := nodes
		if direction == -1 {
			ordered = make([]*Node, len(nodes))
			for i, n := range nodes {
				ordered[len(nodes)-1-i] = n
			}
		}
		for _, child := range ordered {
			idx := len(entries)
			gc := child.Children
			entries = append(entries, labelEntry{
				depth:       depth,
				text:        wrapText(child.Text, 15),
				parentIndex: pid,
				childCount:  len(gc),
			})
			if depth%2 == 0 {
				// Even-depth: pre-order (closer to spine)
				yOrder = append(yOrder, idx)
				if len(gc) > 0 {
					walk(gc, idx, depth+1)
				}
			} else {
				// odd-depth: post-order (within parent diagonal)
				if len(gc) > 0 {
					walk(gc, idx, depth+1)
				}
				yOrder = append(yOrder, idx)
			}
		}
	}
	walk(children, -1, 2)
	return entries, yOrder
}

// drawCauseLabel is drawCauseLabel(svg, text, x, y, direction, fontSize, roughContext).
func drawCauseLabel(svg *dom.Node, text string, x, y, direction, fontSize float64, rc *roughContext) error {
	lg := svg.Append("g").Attr("class", "ishikawa-label-group")
	lt := drawMultilineText(
		lg,
		text,
		x,
		y+float64(11*direction),
		"ishikawa-label cause",
		"middle",
		fontSize,
	)
	tb := lt.BBox()
	if rc != nil {
		o := js.NewObj[any]()
		o.Set("roughness", 1.5)
		o.Set("seed", rc.seed)
		o.Set("fill", rc.fillColor)
		o.Set("fillStyle", "hachure")
		o.Set("fillWeight", 2.5)
		o.Set("hachureGap", 5.0)
		o.Set("stroke", rc.lineColor)
		o.Set("strokeWidth", 2.0)
		roughNode, err := rc.roughSvg.Rectangle(tb.X-20, tb.Y-2, tb.Width+40, tb.Height+4, o)
		if err != nil {
			return err
		}
		lg.InsertNode(roughNode, ":first-child").Attr("class", "ishikawa-label-box")
	} else {
		lg.Insert("rect", ":first-child").
			Attr("class", "ishikawa-label-box").
			Attr("x", tb.X-20).
			Attr("y", tb.Y-2).
			Attr("width", tb.Width+40).
			Attr("height", tb.Height+4)
	}
	return nil
}

// drawArrowMarker is drawArrowMarker(g, x, y, dx, dy, roughContext).
//
// Emulate arrow marker since rough.js does not support marker on line
func drawArrowMarker(g *dom.Node, x, y, dx, dy float64, rc *roughContext) error {
	length := math.Sqrt(float64(dx*dx) + float64(dy*dy))
	if length == 0 {
		return nil
	}
	ux := dx / length
	uy := dy / length
	s := 6.0
	px := float64(-uy * s)
	py := float64(ux * s)
	tipX := x
	tipY := y
	d := "M " + js.Str(tipX) + " " + js.Str(tipY) +
		" L " + js.Str(tipX-float64(float64(ux*s)*2)+px) + " " + js.Str(tipY-float64(float64(uy*s)*2)+py) +
		" L " + js.Str(tipX-float64(float64(ux*s)*2)-px) + " " + js.Str(tipY-float64(float64(uy*s)*2)-py) + " Z"
	o := js.NewObj[any]()
	o.Set("roughness", 1.0)
	o.Set("seed", rc.seed)
	o.Set("fill", rc.lineColor)
	o.Set("fillStyle", "solid")
	o.Set("stroke", rc.lineColor)
	o.Set("strokeWidth", 1.0)
	roughNode, err := rc.roughSvg.Path(d, o)
	if err != nil {
		return err
	}
	g.AppendNode(roughNode)
	return nil
}

// drawBranch is drawBranch(svg, node, startX, startY, direction, length, fontSize, roughContext).
func drawBranch(svg *dom.Node, node *Node, startX, startY, direction, length, fontSize float64, rc *roughContext) error {
	children := node.Children
	factor := 0.2
	if len(children) > 0 {
		factor = 1
	}
	lineLen := length * factor
	dx := float64(-cosA * lineLen)
	dy := float64(sinA * lineLen * direction)
	endX := startX + dx
	endY := startY + dy

	if _, err := drawLine(svg, startX, startY, endX, endY, "ishikawa-branch", rc); err != nil {
		return err
	}
	if rc != nil {
		if err := drawArrowMarker(svg, startX, startY, startX-endX, startY-endY, rc); err != nil {
			return err
		}
	}
	if err := drawCauseLabel(svg, node.Text, endX, endY, direction, fontSize, rc); err != nil {
		return err
	}

	if len(children) == 0 {
		return nil
	}

	entries, yOrder := flattenTree(children, direction)
	entryCount := len(entries)
	ys := make([]float64, entryCount)
	for slot, entryIdx := range yOrder {
		ys[entryIdx] = startY + float64(dy*(float64(slot+1)/float64(entryCount+1)))
	}

	bones := map[int]*boneInfo{}
	bones[-1] = &boneInfo{
		x0:            startX,
		y0:            startY,
		x1:            endX,
		y1:            endY,
		childCount:    len(children),
		childrenDrawn: 0,
	}

	diagonalX := -cosA
	diagonalY := sinA * direction
	oddLabel := "ishikawa-label down"
	if direction < 0 {
		oddLabel = "ishikawa-label up"
	}

	for i, e := range entries {
		y := ys[i]
		par := bones[e.parentIndex]
		grp := svg.Append("g").Attr("class", "ishikawa-sub-group")

		bx0 := 0.0
		by0 := 0.0
		bx1 := 0.0

		if e.depth%2 == 0 {
			// Horizontal bone: attach to parent's diagonal at the target Y, extend left
			dyP := par.y1 - par.y0
			t := 0.5
			if dyP != 0 && !math.IsNaN(dyP) {
				t = (y - par.y0) / dyP
			}
			bx0 = lerp(par.x0, par.x1, t)
			by0 = y
			stub := float64(boneStub)
			if e.childCount > 0 {
				stub = float64(boneBase + e.childCount*bonePerChild)
			}
			bx1 = bx0 - stub
			if _, err := drawLine(grp, bx0, y, bx1, y, "ishikawa-sub-branch", rc); err != nil {
				return err
			}
			if rc != nil {
				if err := drawArrowMarker(grp, bx0, y, 1, 0, rc); err != nil {
					return err
				}
			}
			drawMultilineText(grp, e.text, bx1, y, "ishikawa-label align", "end", fontSize)
		} else {
			// Diagonal bone: start from evenly-spaced point on parent's horizontal, angle toward target Y
			k := par.childrenDrawn
			par.childrenDrawn++
			bx0 = lerp(par.x0, par.x1, float64(par.childCount-k)/float64(par.childCount+1))
			by0 = par.y0
			bx1 = bx0 + float64(diagonalX*((y-by0)/diagonalY))
			if _, err := drawLine(grp, bx0, by0, bx1, y, "ishikawa-sub-branch", rc); err != nil {
				return err
			}
			if rc != nil {
				if err := drawArrowMarker(grp, bx0, by0, bx0-bx1, by0-y, rc); err != nil {
					return err
				}
			}
			drawMultilineText(grp, e.text, bx1, y, oddLabel, "end", fontSize)
		}

		if e.childCount > 0 {
			bones[i] = &boneInfo{
				x0:            bx0,
				y0:            by0,
				x1:            bx1,
				y1:            y,
				childCount:    e.childCount,
				childrenDrawn: 0,
			}
		}
	}
	return nil
}

// jsSpace is the character class \s of a JavaScript regular expression.
const jsSpace = `\t\n\v\f\r \x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}`

var (
	// reLineBreak is /<br\s*\/?>|\n/.
	reLineBreak = regexp.MustCompile(`<br[` + jsSpace + `]*/?>|\n`)
	// reSpaces is /\s+/.
	reSpaces = regexp.MustCompile(`[` + jsSpace + `]+`)
)

// splitLines is splitLines(text).
func splitLines(text string) []string { return reLineBreak.Split(text, -1) }

// wrapText is wrapText(text, maxChars).
func wrapText(text string, maxChars float64) string {
	if float64(js.UTF16Len(text)) <= maxChars {
		return text
	}
	lines := []string{}
	for _, word := range reSpaces.Split(text, -1) {
		last := len(lines) - 1
		if last >= 0 && float64(js.UTF16Len(lines[last])+1+js.UTF16Len(word)) <= maxChars {
			lines[last] += " " + word
		} else {
			lines = append(lines, word)
		}
	}
	return strings.Join(lines, "\n")
}

// drawMultilineText is drawMultilineText(g, text, x, y, cls, anchor, fontSize).
func drawMultilineText(g *dom.Node, text string, x, y float64, cls, anchor string, fontSize float64) *dom.Node {
	lines := splitLines(text)
	lh := fontSize * 1.05
	el := g.
		Append("text").
		Attr("class", cls).
		Attr("text-anchor", anchor).
		Attr("x", x).
		Attr("y", y-float64(float64(float64(len(lines)-1)*lh)/2))
	for i, line := range lines {
		dy := lh
		if i == 0 {
			dy = 0
		}
		el.Append("tspan").
			Attr("x", x).
			Attr("dy", dy).
			Text(line)
	}
	return el
}

// lerp is lerp(a, b, t).
func lerp(a, b, t float64) float64 { return a + float64((b-a)*t) }

// drawLine is drawLine(g, x1, y1, x2, y2, cls, roughContext). It returns nil
// for the hand-drawn look, as upstream returns undefined.
func drawLine(g *dom.Node, x1, y1, x2, y2 float64, cls string, rc *roughContext) (*dom.Node, error) {
	if rc != nil {
		o := js.NewObj[any]()
		o.Set("roughness", 1.5)
		o.Set("seed", rc.seed)
		o.Set("stroke", rc.lineColor)
		o.Set("strokeWidth", 2.0)
		roughNode := rc.roughSvg.Line(x1, y1, x2, y2, o)
		g.AppendNode(roughNode).Attr("class", cls)
		return nil, nil
	}
	return g.
		Append("line").
		Attr("class", cls).
		Attr("x1", x1).
		Attr("y1", y1).
		Attr("x2", x2).
		Attr("y2", y2), nil
}
