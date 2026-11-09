// Ported from mermaid 12.2.0, diagrams/user-journey/svgDraw.js (MIT). The
// functions drawLabel and drawBackgroundRect of the file are not here: no
// journey renderer calls them.

package journey

import (
	"errors"
	"math"
	"regexp"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/shape"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/svgdraw"
)

// svgDraw holds the module state of svgDraw.js.
type svgDraw struct {
	taskCount int
}

// drawRect is drawRect(elem, rectData).
func drawRect(elem *dom.Node, rectData svgdraw.RectData) *dom.Node {
	return svgdraw.DrawRect(elem, rectData)
}

// faceData is the argument of drawFace.
type faceData struct {
	cx, cy, score float64
}

// drawFace is drawFace(element, faceData).
func drawFace(element *dom.Node, faceData faceData) *dom.Node {
	const radius = 15.0
	circleElement := element.
		Append("circle").
		Attr("cx", faceData.cx).
		Attr("cy", faceData.cy).
		Attr("class", "face").
		Attr("r", radius).
		Attr("stroke-width", 2).
		Attr("overflow", "visible")

	face := element.Append("g")

	// The left eye.
	face.
		Append("circle").
		Attr("cx", faceData.cx-radius/3).
		Attr("cy", faceData.cy-radius/3).
		Attr("r", 1.5).
		Attr("stroke-width", 2).
		Attr("fill", "#666").
		Attr("stroke", "#666")

	// The right eye.
	face.
		Append("circle").
		Attr("cx", faceData.cx+radius/3).
		Attr("cy", faceData.cy-radius/3).
		Attr("r", 1.5).
		Attr("stroke-width", 2).
		Attr("fill", "#666").
		Attr("stroke", "#666")

	smile := func(face *dom.Node) {
		arc := shape.NewArc[struct{}]().
			StartAngle(math.Pi / 2).
			EndAngle(3 * (math.Pi / 2)).
			InnerRadius(radius / 2).
			OuterRadius(radius / 2.2)
		// The mouth.
		face.
			Append("path").
			Attr("class", "mouth").
			Attr("d", arc.Path(struct{}{})).
			Attr("transform", "translate("+js.Str(faceData.cx)+","+js.Str(faceData.cy+2)+")")
	}

	sad := func(face *dom.Node) {
		arc := shape.NewArc[struct{}]().
			StartAngle((3 * math.Pi) / 2).
			EndAngle(5 * (math.Pi / 2)).
			InnerRadius(radius / 2).
			OuterRadius(radius / 2.2)
		// The mouth.
		face.
			Append("path").
			Attr("class", "mouth").
			Attr("d", arc.Path(struct{}{})).
			Attr("transform", "translate("+js.Str(faceData.cx)+","+js.Str(faceData.cy+7)+")")
	}

	ambivalent := func(face *dom.Node) {
		face.
			Append("line").
			Attr("class", "mouth").
			Attr("stroke", 2).
			Attr("x1", faceData.cx-5).
			Attr("y1", faceData.cy+7).
			Attr("x2", faceData.cx+5).
			Attr("y2", faceData.cy+7).
			Attr("class", "mouth").
			Attr("stroke-width", "1px").
			Attr("stroke", "#666")
	}

	if faceData.score > 3 {
		smile(face)
	} else if faceData.score < 3 {
		sad(face)
	} else {
		ambivalent(face)
	}

	return circleElement
}

// circleData is the argument of drawCircle. fill is a config value, and a
// nil title is an undefined title.
type circleData struct {
	cx, cy, r float64
	fill      any
	stroke    string
	pos       float64
	title     *string
}

// drawCircle is drawCircle(element, circleData).
func drawCircle(element *dom.Node, circleData circleData) *dom.Node {
	circleElement := element.Append("circle")
	circleElement.Attr("cx", circleData.cx)
	circleElement.Attr("cy", circleData.cy)
	circleElement.Attr("class", "actor-"+js.Str(circleData.pos))
	circleElement.Attr("fill", attrValue(circleData.fill))
	circleElement.Attr("stroke", circleData.stroke)
	circleElement.Attr("r", circleData.r)

	// Upstream tests circleElement.class here. A d3 selection has no class
	// property, so the test is always false.

	if circleData.title != nil {
		circleElement.Append("title").Text(*circleData.title)
	}

	return circleElement
}

// drawText is drawText(elem, textData).
func drawText(elem *dom.Node, textData svgdraw.TextData) *dom.Node {
	return svgdraw.DrawText(elem, textData)
}

// sectionData is the section object that drawTasks gives to drawSection.
type sectionData struct {
	x, y      float64
	text      string
	fill      any
	num       float64
	colour    any
	taskCount float64
}

// drawSection is drawSection(elem, section, conf).
func drawSection(elem *dom.Node, section sectionData, conf *journeyConf) {
	g := elem.Append("g")

	rect := svgdraw.GetNoteRect()
	rect.X = section.x
	rect.Y = section.y
	rect.Fill = section.fill
	// The section width covers all nested tasks.
	rect.Width = float64(conf.num("width")*section.taskCount) + // The width of the tasks.
		float64(conf.num("diagramMarginX")*(section.taskCount-1)) // The width of the space between the tasks.
	rect.Height = conf.num("height")
	rect.Class = "journey-section section-type-" + js.Str(section.num)
	rect.Rx = 3
	rect.Ry = 3
	drawRect(g, rect)

	drawTextCandidateFunc(conf)(
		section.text,
		g,
		rect.X,
		rect.Y,
		rect.Width,
		rect.Height,
		[]textAttr{{"class", "journey-section section-type-" + js.Str(section.num)}},
		conf,
		section.colour,
	)
}

// drawTask draws a task in the diagram with the attached line.
func (s *svgDraw) drawTask(elem *dom.Node, task *Task, conf *journeyConf, diagramID string) error {
	center := task.x + float64(conf.num("width")/2)
	g := elem.Append("g")
	s.taskCount++
	const maxHeight = 300 + 5*30
	g.Append("line").
		Attr("id", diagramID+"-task"+js.Str(float64(s.taskCount))).
		Attr("x1", center).
		Attr("y1", task.y).
		Attr("x2", center).
		Attr("y2", maxHeight).
		Attr("class", "task-line").
		Attr("stroke-width", "1px").
		Attr("stroke-dasharray", "4 2").
		Attr("stroke", "#666")

	drawFace(g, faceData{
		cx:    center,
		cy:    300 + float64((5-task.Score)*30),
		score: task.Score,
	})

	rect := svgdraw.GetNoteRect()
	rect.X = task.x
	rect.Y = task.y
	rect.Fill = task.fill
	rect.Width = conf.num("width")
	rect.Height = conf.num("height")
	rect.Class = "task task-type-" + js.Str(task.num)
	rect.Rx = 3
	rect.Ry = 3
	drawRect(g, rect)

	xPos := task.x + 14
	for _, person := range task.People {
		// The renderer puts every person of the task into task.actors.
		actor := task.actors.get(person)
		if actor == nil {
			return errors.New("Cannot read properties of undefined (reading 'color')")
		}
		colour := actor.color

		title := person
		circle := circleData{
			cx:     xPos,
			cy:     task.y,
			r:      7,
			fill:   colour,
			stroke: "#000",
			title:  &title,
			pos:    actor.position,
		}

		drawCircle(g, circle)
		xPos += 10
	}

	drawTextCandidateFunc(conf)(
		task.Task,
		g,
		rect.X,
		rect.Y,
		rect.Width,
		rect.Height,
		[]textAttr{{"class", "task"}},
		conf,
		task.colour,
	)
	return nil
}

// textAttr is one property of the textAttrs object of the text functions.
type textAttr struct {
	key, value string
}

// drawTextFunc is the type of the functions that _drawTextCandidateFunc
// returns. Upstream calls each of them with the same nine arguments.
type drawTextFunc func(content string, g *dom.Node, x, y, width, height float64, textAttrs []textAttr, conf *journeyConf, colour any)

// reBr is /<br\s*\/?>/gi, with the white space of a JavaScript regular
// expression.
var reBr = regexp.MustCompile(`(?i)<br[\t\n\x0B\f\r \x{A0}\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}\x{FEFF}]*/?>`)

// byText is byText of _drawTextCandidateFunc. Its eighth parameter is
// colour, and the callers give the conf object in that position. So the
// value of the font-color style is the string of an object. font-color is
// not a CSS property, and the browser ignores it.
func byText(content string, g *dom.Node, x, y, width, height float64, textAttrs []textAttr, _ *journeyConf, _ any) {
	text := g.
		Append("text").
		Attr("x", x+float64(width/2)).
		Attr("y", y+float64(height/2)+5).
		Style("font-color", "[object Object]").
		Style("text-anchor", "middle").
		Text(content)
	setTextAttrs(text, textAttrs)
}

// byTspan is byTspan of _drawTextCandidateFunc.
func byTspan(content string, g *dom.Node, x, y, width, height float64, textAttrs []textAttr, conf *journeyConf, colour any) {
	taskFontSize, taskFontFamily := conf.get("taskFontSize"), conf.get("taskFontFamily")
	taskFontSizeNumber := js.ToNumber(taskFontSize)

	lines := reBr.Split(content, -1)
	for i := 0; i < len(lines); i++ {
		dy := float64(float64(i)*taskFontSizeNumber) - float64(float64(taskFontSizeNumber*float64(len(lines)-1))/2)
		text := g.
			Append("text").
			Attr("x", x+float64(width/2)).
			Attr("y", y).
			Attr("fill", attrValue(colour)).
			Style("text-anchor", "middle").
			Style("font-size", attrValue(taskFontSize)).
			Style("font-family", attrValue(taskFontFamily))
		text.
			Append("tspan").
			Attr("x", x+float64(width/2)).
			Attr("dy", dy).
			Text(lines[i])

		text.
			Attr("y", y+float64(height/2.0)).
			Attr("dominant-baseline", "central").
			Attr("alignment-baseline", "central")

		setTextAttrs(text, textAttrs)
	}
}

// byFo is byFo of _drawTextCandidateFunc. It gives no colour to byTspan.
func byFo(content string, g *dom.Node, x, y, width, height float64, textAttrs []textAttr, conf *journeyConf, _ any) {
	body := g.Append("switch")
	f := body.
		Append("foreignObject").
		Attr("x", x).
		Attr("y", y).
		Attr("width", width).
		Attr("height", height).
		Attr("position", "fixed")

	text := f.
		Append("xhtml:div").
		Style("display", "table").
		Style("height", "100%").
		Style("width", "100%")

	text.
		Append("div").
		Attr("class", "label").
		Style("display", "table-cell").
		Style("text-align", "center").
		Style("vertical-align", "middle").
		Text(content)

	byTspan(content, body, x, y, width, height, textAttrs, conf, nil)
	setTextAttrs(text, textAttrs)
}

// setTextAttrs is _setTextAttrs(toText, fromTextAttrsDict).
func setTextAttrs(toText *dom.Node, fromTextAttrsDict []textAttr) {
	for _, attr := range fromTextAttrsDict {
		toText.Attr(attr.key, attr.value)
	}
}

// drawTextCandidateFunc is _drawTextCandidateFunc(conf).
func drawTextCandidateFunc(conf *journeyConf) drawTextFunc {
	switch conf.get("textPlacement") {
	case "fo":
		return byFo
	case "old":
		return byText
	}
	return byTspan
}

// initGraphics is initGraphics(graphics, id).
func (s *svgDraw) initGraphics(graphics *dom.Node, id string) {
	s.taskCount = -1
	graphics.
		Append("defs").
		Append("marker").
		Attr("id", id+"-arrowhead").
		Attr("refX", 5).
		Attr("refY", 2).
		Attr("markerWidth", 6).
		Attr("markerHeight", 4).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 0,0 V 4 L6,2 Z") // This is the shape of the arrowhead.
}
