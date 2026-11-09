// Ported from mermaid 12.2.0, diagrams/sequence/svgDraw.js (MIT).

package sequence

import (
	"errors"
	"math"
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
	"github.com/bvolpato/mermaid-go-renderer/internal/svgdraw"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

var nan = math.NaN()

// actorTypeWidth is ACTOR_TYPE_WIDTH.
const actorTypeWidth = 18 * 2

// The stick figure's geometry, in unscaled units measured down from the top
// of its classic box: the head circle reaches actorGlyphTop and the feet
// reach actorGlyphBottom. Under `neo` the figure is scaled to fit the shared
// glyph band; under `classic` it draws at these coordinates unchanged.
const (
	actorGlyphTop    = -5
	actorGlyphBottom = 60
	actorGlyphHeight = actorGlyphBottom - actorGlyphTop
)

const (
	topActorClass       = "actor-top"
	bottomActorClass    = "actor-bottom"
	actorBoxClass       = "actor-box"
	actorManFigureClass = "actor-man"
)

// rectData is the rect object that svgDraw.js stores in actor.rectData.
type rectData = svgdraw.RectData

// neoBands is the band geometry for one participant under `neo`; nil under
// every other look, which keeps each shape's legacy geometry.
func (r *renderer) neoBands(a *actor, isFooter bool, actorY float64) (*bands, error) {
	if !r.conf.isNeo() {
		return nil, nil
	}
	textHeight, err := r.actorLabelHeight(a)
	if err != nil {
		return nil, err
	}
	if isFooter {
		return footerBands(actorY, textHeight), nil
	}
	return headerBands(actorY, a.height, textHeight), nil
}

// isColorTheme is COLOR_THEMES.has(theme): the exact set of themes that use
// color arrays for actor styling.
func isColorTheme(theme any) bool {
	return theme == "redux-color" || theme == "redux-dark-color"
}

// paletteColor picks a colour for the nth actor from a palette, cycling
// within that palette's own length. The result is nil (undefined) for an
// absent or empty palette: selection.style(name, undefined) removes the
// inline value, which lets the stylesheet decide.
func paletteColor(palette any, index int) any {
	switch p := palette.(type) {
	case []any:
		if len(p) == 0 {
			return nil
		}
		v := p[index%len(p)]
		if js.IsNullish(v) {
			return nil
		}
		return v
	case string:
		units := js.UTF16Units(p)
		if len(units) == 0 {
			return nil
		}
		return js.FromUTF16Units(units[index%len(units) : index%len(units)+1])
	}
	return nil
}

// drawRect is drawRect(elem, rectData).
func (r *renderer) drawRect(elem *dom.Node, rect rectData) *dom.Node {
	rectElement := svgdraw.DrawRect(elem, rect)
	// Upstream reads getConfig() here, not the module variable.
	if r.look == "neo" {
		rectElement.Attr("data-look", "neo")
	}
	return rectElement
}

// drawPopup is drawPopup(elem, actor, minMenuWidth, textAttrs, forceMenus).
// It returns the height and the width of the menu.
func (r *renderer) drawPopup(elem *dom.Node, a *actor, minMenuWidth float64) (height, width float64, err error) {
	conf := r.conf
	if a.links == nil || a.links.Len() == 0 {
		return 0, 0, nil
	}

	links := a.links
	rectData := a.rectData
	if rectData == nil {
		return 0, 0, errors.New("TypeError: Cannot read properties of null (reading 'class')")
	}

	displayValue := "none"
	if conf.forceMenus {
		displayValue = "block !important"
	}

	g := elem.Append("g")
	g.Attr("id", "actor"+actorCntText(a)+"_popup")
	g.Attr("class", "actorPopupMenu")
	g.Attr("display", displayValue)
	actorClass := ""
	if rectData.Class != "" {
		actorClass = " " + rectData.Class
	}

	menuWidth := minMenuWidth
	if rectData.Width > minMenuWidth {
		menuWidth = rectData.Width
	}

	rectElem := g.Append("rect")
	rectElem.Attr("class", "actorPopupMenuPanel"+actorClass)
	rectElem.Attr("x", rectData.X)
	rectElem.Attr("y", rectData.Height)
	rectElem.Attr("fill", undef(rectData.Fill))
	rectElem.Attr("stroke", undef(rectData.Stroke))
	rectElem.Attr("width", menuWidth)
	rectElem.Attr("height", rectData.Height)
	rectElem.Attr("rx", rectData.Rx)
	rectElem.Attr("ry", rectData.Ry)
	linkY := 20.0
	for _, key := range links.Keys() {
		linkElem := g.Append("a")
		sanitizedLink := sanitize.SanitizeUrl(js.ToString(links.Get(key)))
		linkElem.Attr("xlink:href", sanitizedLink)
		linkElem.Attr("target", "_blank")

		r.drawMenuItemTextCandidate(
			key,
			linkElem,
			rectData.X+10,
			rectData.Height+linkY,
			menuWidth,
			20,
			[]textAttr{{"class", "actor"}},
		)

		linkY += 30
	}

	rectElem.Attr("height", linkY)

	return rectData.Height + linkY, menuWidth, nil
}

// actorCntText is the actorCnt of an actor inside a string: "null" when the
// actor was not drawn.
func actorCntText(a *actor) string {
	if !a.hasActorCnt {
		return "null"
	}
	return strconv.Itoa(a.actorCnt)
}

func popupMenuToggle(popID string) string {
	return "var pu = document.getElementById('" +
		popID +
		"'); if (pu != null) { pu.style.display = pu.style.display == 'block' ? 'none' : 'block'; }"
}

// textObj is the text object of svgDraw.js: the result of getTextObj of
// svgDrawCommon.ts or of svgDraw.js, with the properties that the renderer
// adds. A dynamic field is nil for undefined.
type textObj struct {
	x, y float64
	// width is undefined when hasWidth is false.
	width             float64
	hasWidth          bool
	height            float64
	fill              any
	anchor            any
	textMargin        float64
	tspan             bool
	valign            any
	text              string
	class             any
	dy                any
	fontFamily        any
	fontSize          any
	fontWeight        any
	dominantBaseline  any
	alignmentBaseline any
	wrap              bool
}

// getTextObjCommon is getTextObj() of svgDrawCommon.ts.
func getTextObjCommon() *textObj {
	return &textObj{
		x:          0,
		y:          0,
		width:      100,
		hasWidth:   true,
		height:     100,
		textMargin: 0,
		tspan:      true,
	}
}

// getTextObj is getTextObj() of svgDraw.js. Its width and height are
// undefined.
func getTextObj() *textObj {
	return &textObj{
		x:          0,
		y:          0,
		fill:       nil,
		anchor:     nil,
		textMargin: 0,
		tspan:      true,
		valign:     nil,
	}
}

// drawText is drawText(elem, textData). It changes x, anchor,
// dominantBaseline and alignmentBaseline of textData, as upstream.
func drawText(elem *dom.Node, textData *textObj) []*dom.Node {
	prevTextHeight := 0.0
	textHeight := 0.0
	lines := sanitize.SplitBreaks(textData.text)

	textFontSize, textFontSizePx, hasFontSize := utils.ParseFontSize(textData.fontSize)

	var textElems []*dom.Node
	dy := 0.0
	yfunc := func() float64 { return textData.y }
	if textData.valign != nil && textData.textMargin > 0 {
		switch textData.valign {
		case "top", "start":
			yfunc = func() float64 { return js.Round(textData.y + textData.textMargin) }
		case "middle", "center":
			yfunc = func() float64 {
				return js.Round(textData.y + float64((prevTextHeight+textHeight+textData.textMargin)/2))
			}
		case "bottom", "end":
			yfunc = func() float64 {
				return js.Round(
					textData.y +
						(prevTextHeight + textHeight + float64(2*textData.textMargin)) -
						textData.textMargin,
				)
			}
		}
	}

	if textData.anchor != nil && textData.hasWidth {
		switch textData.anchor {
		case "left", "start":
			textData.x = js.Round(textData.x + textData.textMargin)
			textData.anchor = "start"
			textData.dominantBaseline = "middle"
			textData.alignmentBaseline = "middle"
		case "middle", "center":
			textData.x = js.Round(textData.x + float64(textData.width/2))
			textData.anchor = "middle"
			textData.dominantBaseline = "middle"
			textData.alignmentBaseline = "middle"
		case "right", "end":
			textData.x = js.Round(textData.x + textData.width - textData.textMargin)
			textData.anchor = "end"
			textData.dominantBaseline = "middle"
			textData.alignmentBaseline = "middle"
		}
	}

	for i, line := range lines {
		if textData.textMargin == 0 && hasFontSize {
			dy = float64(float64(i) * textFontSize)
		}

		textElem := elem.Append("text")
		textElem.Attr("x", textData.x)
		textElem.Attr("y", yfunc())
		if textData.anchor != nil {
			textElem.
				Attr("text-anchor", textData.anchor).
				Attr("dominant-baseline", textData.dominantBaseline).
				Attr("alignment-baseline", textData.alignmentBaseline)
		}
		if textData.fontFamily != nil {
			textElem.Style("font-family", undef(textData.fontFamily))
		}
		if hasFontSize {
			textElem.Style("font-size", textFontSizePx)
		}
		if textData.fontWeight != nil {
			textElem.Style("font-weight", undef(textData.fontWeight))
		}
		if textData.fill != nil {
			textElem.Attr("fill", undef(textData.fill))
		}
		if textData.class != nil {
			textElem.Attr("class", undef(textData.class))
		}
		if textData.dy != nil {
			textElem.Attr("dy", undef(textData.dy))
		} else if dy != 0 {
			textElem.Attr("dy", dy)
		}

		text := line
		if text == "" {
			text = utils.ZeroWidthSpace
		}
		if textData.tspan {
			span := textElem.Append("tspan")
			span.Attr("x", textData.x)
			if textData.fill != nil {
				span.Attr("fill", undef(textData.fill))
			}
			span.Text(text)
		} else {
			textElem.Text(text)
		}
		if textData.valign != nil && textData.textMargin > 0 {
			textHeight += textElem.BBox().Height
			prevTextHeight = textHeight
		}

		textElems = append(textElems, textElem)
	}

	return textElems
}

// drawLabel is drawLabel(elem, txtObject).
func drawLabel(elem *dom.Node, txtObject *textObj) *dom.Node {
	genPoints := func(x, y, width, height, cut float64) string {
		return js.Str(x) +
			"," +
			js.Str(y) +
			" " +
			js.Str(x+width) +
			"," +
			js.Str(y) +
			" " +
			js.Str(x+width) +
			"," +
			js.Str(y+height-cut) +
			" " +
			js.Str(x+width-float64(cut*1.2)) +
			"," +
			js.Str(y+height) +
			" " +
			js.Str(x) +
			"," +
			js.Str(y+height)
	}
	polygon := elem.Append("polygon")
	polygon.Attr("points", genPoints(txtObject.x, txtObject.y, txtObject.width, txtObject.height, 7))
	polygon.Attr("class", "labelBox")

	txtObject.y = txtObject.y + float64(txtObject.height/2)

	drawText(elem, txtObject)
	return polygon
}

// fixLifeLineHeights is fixLifeLineHeights(diagram, actors, actorKeys, conf).
func fixLifeLineHeights(diagram *dom.Node, actors *js.Map[string, *actor], actorKeys []string, conf *conf) {
	for _, actorKey := range actorKeys {
		a := actors.Get(actorKey)
		if a == nil {
			continue
		}
		actorDOM := diagram.Select("#actor" + actorCntText(a))
		if !conf.mirrorActors && truthyNum(a.stopy) {
			actorDOM.Attr("y2", a.stopy+float64(a.height/2))
		} else if conf.mirrorActors {
			actorDOM.Attr("y2", a.stopy)
		}
	}
}

// lifeLine appends the lifeline of an actor to g.
func (r *renderer) lifeLine(g *dom.Node, a *actor, center, centerY float64) {
	g.Append("line").
		Attr("id", "actor"+strconv.Itoa(r.actorCnt)).
		Attr("x1", center).
		Attr("y1", centerY).
		Attr("x2", center).
		Attr("y2", 2000).
		Attr("class", "actor-line 200").
		Attr("stroke-width", "0.5px").
		Attr("stroke", "#999").
		Attr("name", a.name).
		Attr("data-et", "life-line").
		Attr("data-id", a.name)
}

// boxGroup is the start of the box shapes (participant, collections, queue,
// database): the group with the lifeline, and the group of the shape.
func (r *renderer) boxGroup(elem *dom.Node, a *actor, isFooter bool, center, centerY float64) *dom.Node {
	conf := r.conf
	boxplusLineGroup := elem.Append("g").Lower()
	g := boxplusLineGroup

	if !isFooter {
		r.actorCnt++
		if a.links != nil && a.links.Len() > 0 && !conf.forceMenus {
			g.Attr("onclick", popupMenuToggle("actor"+strconv.Itoa(r.actorCnt)+"_popup")).Attr("cursor", "pointer")
		}
		r.lifeLine(g, a, center, centerY)

		g = boxplusLineGroup.Append("g")
		a.actorCnt, a.hasActorCnt = r.actorCnt, true

		if a.links != nil {
			g.Attr("id", "root-"+strconv.Itoa(r.actorCnt))
		}
		if conf.isNeo() {
			g.Attr("data-look", "neo")
		}
	}
	return g
}

// actorClass is the class of a box shape: the class property of the actor,
// or "actor". fromProperties is true when the property gave the class.
func actorClass(a *actor, isFooter bool) (cssclass string, fromProperties bool) {
	cssclass = "actor"
	if v := a.properties.Get("class"); js.Truthy(v) {
		cssclass = js.ToString(v)
		fromProperties = true
	}
	if isFooter {
		cssclass += " " + bottomActorClass
	} else {
		cssclass += " " + topActorClass
	}
	return cssclass, fromProperties
}

// drawIcon draws the icon property of an actor.
func drawIcon(g *dom.Node, a *actor, x, y float64) error {
	icon := a.properties.Get("icon")
	if !js.Truthy(icon) {
		return nil
	}
	s, ok := icon.(string)
	if !ok {
		return errors.New("TypeError: actor.properties.icon.trim is not a function")
	}
	iconSrc := js.TrimSpace(s)
	if js.CharAt(iconSrc, 0) == "@" {
		svgdraw.DrawEmbeddedImage(g, x, y, js.StringSliceFrom(iconSrc, 1))
	} else {
		svgdraw.DrawImage(g, x, y, iconSrc)
	}
	return nil
}

// actorY is `isFooter ? actor.stopy : actor.starty`.
func actorY(a *actor, isFooter bool) float64 {
	if isFooter {
		return a.stopy
	}
	return a.starty
}

func (r *renderer) dropShadowURL(diagramID string) string {
	return "url(#" + dropShadowID(diagramID) + ")"
}

// drawActorTypeParticipant draws an actor in the diagram with the attached
// line.
func (r *renderer) drawActorTypeParticipant(elem *dom.Node, a *actor, isFooter bool, diagramID string, actorIndexMap *js.Map[string, int]) (float64, error) {
	conf := r.conf
	actorY := actorY(a, isFooter)
	center := a.x + float64(a.width/2)
	centerY := actorY + a.height
	bkgColorArray, borderColorArray := conf.themeVar("bkgColorArray"), conf.themeVar("borderColorArray")

	g := r.boxGroup(elem, a, isFooter, center, centerY)

	rect := svgdraw.GetNoteRect()
	cssclass, fromProperties := actorClass(a, isFooter)
	if !fromProperties {
		rect.Fill = "#eaeaea"
	}
	rect.X = a.x
	rect.Y = actorY
	rect.Width = a.width
	rect.Height = a.height
	rect.Class = cssclass
	rect.Rx = 3
	rect.Ry = 3
	rect.Name = a.name

	if conf.isNeo() {
		rect.Rx = 6
		rect.Ry = 6
	}
	rectElem := r.drawRect(g, rect)

	actorCount := actorIndexMap.Get(a.name)
	if isColorTheme(conf.theme) {
		rectElem.Style("stroke", paletteColor(borderColorArray, actorCount))
		rectElem.Style("fill", paletteColor(bkgColorArray, actorCount))
	}
	if conf.isNeo() {
		rectElem.Attr("filter", r.dropShadowURL(diagramID))
	}

	a.rectData = &rect

	if err := drawIcon(g, a, rect.X+rect.Width-20, rect.Y+10); err != nil {
		return 0, err
	}

	if !isFooter {
		g.Attr("data-et", "participant")
		g.Attr("data-type", "participant")
		g.Attr("data-id", a.name)
	}

	if err := r.drawTextCandidate(
		a.description,
		g,
		rect.X,
		rect.Y,
		rect.Width,
		rect.Height,
		[]textAttr{{"class", "actor " + actorBoxClass}},
	); err != nil {
		return 0, err
	}

	height := a.height
	bounds := rectElem.BBox()
	if !conf.isNeo() {
		a.height = bounds.Height
		height = bounds.Height
	}

	return height, nil
}

// drawActorTypeCollections draws an actor in the diagram with the attached
// line.
func (r *renderer) drawActorTypeCollections(elem *dom.Node, a *actor, isFooter bool, diagramID string, actorIndexMap *js.Map[string, int]) (float64, error) {
	conf := r.conf
	actorY := actorY(a, isFooter)
	center := a.x + float64(a.width/2)
	centerY := actorY + a.height
	bkgColorArray, borderColorArray := conf.themeVar("bkgColorArray"), conf.themeVar("borderColorArray")

	g := r.boxGroup(elem, a, isFooter, center, centerY)

	rect := svgdraw.GetNoteRect()
	cssclass, fromProperties := actorClass(a, isFooter)
	if !fromProperties {
		rect.Fill = "#eaeaea"
	}
	rect.X = a.x
	rect.Y = actorY
	rect.Width = a.width
	rect.Height = a.height
	rect.Class = cssclass
	rect.Name = a.name

	// DRAW STACKED RECTANGLES
	const offset = 6
	if conf.isNeo() {
		// The stack must not cross the datum. Both copies are drawn `offset` shorter, so the shadow
		// copy's bottom edge lands exactly on the lifeline start instead of poking below it.
		rect.Height = a.height - offset
	}
	shadowRect := rect
	shadowRect.X = rect.X + -offset
	shadowRect.Y = rect.Y + +offset
	shadowRect.Class = "actor"
	rectElem := r.drawRect(g, rect) // draw main rectangle on top
	stackedRect := r.drawRect(g, shadowRect)
	a.rectData = &rect

	if conf.isNeo() {
		g.Attr("filter", r.dropShadowURL(diagramID))
	}

	actorCount := actorIndexMap.Get(a.name)
	if isColorTheme(conf.theme) {
		rectElem.Style("stroke", paletteColor(borderColorArray, actorCount))
		rectElem.Style("fill", paletteColor(bkgColorArray, actorCount))
		stackedRect.Style("stroke", paletteColor(borderColorArray, actorCount))
		stackedRect.Style("fill", paletteColor(bkgColorArray, actorCount))
	}

	if err := drawIcon(g, a, rect.X+rect.Width-20, rect.Y+10); err != nil {
		return 0, err
	}

	if err := r.drawTextCandidate(
		a.description,
		g,
		rect.X-offset,
		rect.Y+offset,
		rect.Width,
		rect.Height,
		[]textAttr{{"class", "actor " + actorBoxClass}},
	); err != nil {
		return 0, err
	}

	height := a.height
	bounds := rectElem.BBox()
	if !conf.isNeo() {
		a.height = bounds.Height
		height = bounds.Height
	}

	if !isFooter {
		g.Attr("data-et", "participant")
		g.Attr("data-type", "collections")
		g.Attr("data-id", a.name)
	}

	return height, nil
}

func (r *renderer) drawActorTypeQueue(elem *dom.Node, a *actor, isFooter bool, diagramID string, actorIndexMap *js.Map[string, int]) (float64, error) {
	conf := r.conf
	actorY := actorY(a, isFooter)
	center := a.x + float64(a.width/2)
	centerY := actorY + a.height
	bkgColorArray, borderColorArray := conf.themeVar("bkgColorArray"), conf.themeVar("borderColorArray")

	g := r.boxGroup(elem, a, isFooter, center, centerY)

	rect := svgdraw.GetNoteRect()
	cssclass, fromProperties := actorClass(a, isFooter)
	if !fromProperties {
		rect.Fill = "#eaeaea"
	}
	g.Attr("class", cssclass)

	rect.X = a.x
	rect.Y = actorY
	rect.Width = a.width
	rect.Height = a.height
	rect.Name = a.name

	// Cylinder dimensions
	ry := float64(rect.Height / 2)
	rx := ry / (2.5 + rect.Height/50)

	// Cylinder base group
	cylinderGroup := g.Append("g")
	cylinderArc := g.Append("g")

	// Main cylinder body
	cylinderPath := "M " + js.Str(rect.X) + "," + js.Str(rect.Y+ry) + "\n" +
		"    a " + js.Str(rx) + "," + js.Str(ry) + " 0 0 0 0," + js.Str(rect.Height) + "\n" +
		"    h " + js.Str(rect.Width-float64(2*rx)) + "\n" +
		"    a " + js.Str(rx) + "," + js.Str(ry) + " 0 0 0 0,-" + js.Str(rect.Height) + "\n" +
		"    Z\n" +
		"  "
	cylinderGroup.Append("path").Attr("d", cylinderPath)
	cylinderArc.Append("path").Attr(
		"d",
		"M "+js.Str(rect.X)+","+js.Str(rect.Y+ry)+"\n"+
			"      a "+js.Str(rx)+","+js.Str(ry)+" 0 0 0 0,"+js.Str(rect.Height),
	)

	cylinderGroup.Attr("transform", "translate("+js.Str(rx)+", "+js.Str(-(rect.Height/2))+")")
	cylinderArc.Attr("transform", "translate("+js.Str(rect.Width-rx)+", "+js.Str(-rect.Height/2)+")")

	a.rectData = &rect

	if conf.isNeo() {
		cylinderGroup.Attr("filter", r.dropShadowURL(diagramID))
	}

	actorCount := actorIndexMap.Get(a.name)
	if isColorTheme(conf.theme) {
		cylinderGroup.Style("stroke", paletteColor(borderColorArray, actorCount))
		cylinderGroup.Style("fill", paletteColor(bkgColorArray, actorCount))
		cylinderArc.Style("stroke", paletteColor(borderColorArray, actorCount))
		cylinderArc.Style("fill", paletteColor(bkgColorArray, actorCount))
	}

	iconX := rect.X + rect.Width - 20
	iconY := rect.Y + 10
	if err := drawIcon(g, a, iconX, iconY); err != nil {
		return 0, err
	}

	if err := r.drawTextCandidate(
		a.description,
		g,
		rect.X,
		rect.Y,
		rect.Width,
		rect.Height,
		[]textAttr{{"class", "actor " + actorBoxClass}},
	); err != nil {
		return 0, err
	}

	height := a.height
	if lastPath := cylinderGroup.Select("path:last-child"); lastPath != nil {
		bounds := lastPath.BBox()
		if !conf.isNeo() {
			a.height = bounds.Height
			height = bounds.Height
		}
	}

	if !isFooter {
		g.Attr("data-et", "participant")
		g.Attr("data-type", "queue")
		g.Attr("data-id", a.name)
	}

	return height, nil
}

func (r *renderer) drawActorTypeControl(elem *dom.Node, a *actor, isFooter bool, diagramID string, actorIndexMap *js.Map[string, int]) (float64, error) {
	conf := r.conf
	actorY := actorY(a, isFooter)
	center := a.x + float64(a.width/2)
	bands, err := r.neoBands(a, isFooter, actorY)
	if err != nil {
		return 0, err
	}
	centerY := actorY + 75
	if bands != nil {
		centerY = bands.lifelineStartY
	}
	bkgColorArray, borderColorArray := conf.themeVar("bkgColorArray"), conf.themeVar("borderColorArray")
	actorBorder, actorBkg := conf.themeVar("actorBorder"), conf.themeVar("actorBkg")

	line := elem.Append("g").Lower()

	if !isFooter {
		r.actorCnt++
		r.lifeLine(line, a, center, centerY)

		a.actorCnt, a.hasActorCnt = r.actorCnt, true
	}
	actElem := elem.Append("g")
	cssClass := actorManFigureClass
	if isFooter {
		cssClass += " " + bottomActorClass
	} else {
		cssClass += " " + topActorClass
	}
	actElem.Attr("class", cssClass)
	actElem.Attr("name", a.name)

	rect := svgdraw.GetNoteRect()
	rect.X = a.x
	rect.Y = actorY
	rect.Fill = "#eaeaea"
	rect.Width = a.width
	rect.Height = a.height
	rect.Class = "actor"

	cx := a.x + float64(a.width/2)
	cy := actorY + 32
	if bands != nil {
		cy = bands.glyphBottomY - 22
	}
	const radius = 22.0

	actElem.
		Append("defs").
		Append("marker").
		Attr("id", diagramID+"-filled-head-control").
		Attr("refX", 11).
		Attr("refY", 5.8).
		Attr("markerWidth", 20).
		Attr("markerHeight", 28).
		Attr("orient", "172.5").
		Attr("stroke-width", 1.2).
		Append("path").
		Attr("d", "M 14.4 5.6 L 7.2 10.4 L 8.8 5.6 L 7.2 0.8 Z")

	// Draw the base circle
	filter := ""
	if conf.isNeo() {
		filter = r.dropShadowURL(diagramID)
	}
	actElem.
		Append("circle").
		Attr("cx", cx).
		Attr("cy", cy).
		Attr("r", radius).
		Attr("filter", filter)

	// Draw looping arrow as arc path
	actElem.
		Append("line").
		Attr("marker-end", "url(#"+diagramID+"-filled-head-control)").
		Attr("transform", "translate("+js.Str(cx)+", "+js.Str(cy-radius)+")")

	actorCount := actorIndexMap.Get(a.name)
	if isColorTheme(conf.theme) {
		actElem.Style("stroke", paletteColor(borderColorArray, actorCount))
		actElem.Style("fill", paletteColor(bkgColorArray, actorCount))
	} else {
		actElem.Style("stroke", undef(actorBorder))
		actElem.Style("fill", undef(actorBkg))
	}
	if bands == nil {
		bounds := actElem.BBox()
		a.height = bounds.Height + float64(2*conf.sequenceLabelBoxHeight)
	}

	y := rect.Y + radius
	if !isFooter {
		y += 12
	} else {
		y += 5
	}
	if bands != nil {
		y = bands.labelCenterY - float64(rect.Height/2)
	}
	if err := r.drawTextCandidate(
		a.description,
		actElem,
		rect.X,
		y,
		rect.Width,
		rect.Height,
		[]textAttr{{"class", "actor " + actorManFigureClass}},
	); err != nil {
		return 0, err
	}

	if !isFooter {
		actElem.Attr("data-et", "participant")
		actElem.Attr("data-type", "control")
		actElem.Attr("data-id", a.name)
	}

	return a.height, nil
}

func (r *renderer) drawActorTypeEntity(elem *dom.Node, a *actor, isFooter bool, diagramID string, actorIndexMap *js.Map[string, int]) (float64, error) {
	conf := r.conf
	actorY := actorY(a, isFooter)
	center := a.x + float64(a.width/2)
	bands, err := r.neoBands(a, isFooter, actorY)
	if err != nil {
		return 0, err
	}
	centerY := actorY + 75
	if bands != nil {
		centerY = bands.lifelineStartY
	}
	bkgColorArray, borderColorArray := conf.themeVar("bkgColorArray"), conf.themeVar("borderColorArray")

	line := elem.Append("g").Lower()

	actElem := elem.Append("g")
	cssClass := "actor"
	if isFooter {
		cssClass += " " + bottomActorClass
	} else {
		cssClass += " " + topActorClass
	}
	actElem.Attr("class", cssClass)
	actElem.Attr("name", a.name)

	rect := svgdraw.GetNoteRect()
	rect.X = a.x
	rect.Y = actorY
	rect.Fill = "#eaeaea"
	rect.Width = a.width
	rect.Height = a.height
	rect.Class = "actor"

	cx := a.x + float64(a.width/2)
	var cy float64
	switch {
	case bands != nil:
		cy = bands.glyphBottomY - 22
	case !isFooter:
		cy = actorY + 25
	default:
		cy = actorY + 10
	}
	const radius = 22.0

	actElem.
		Append("circle").
		Attr("cx", cx).
		Attr("cy", cy).
		Attr("r", radius).
		Attr("width", a.width).
		Attr("height", a.height)

	actElem.
		Append("line").
		Attr("x1", cx-radius).
		Attr("x2", cx+radius).
		Attr("y1", cy+radius).
		Attr("y2", cy+radius).
		Attr("stroke-width", 2)

	if conf.isNeo() {
		actElem.Attr("filter", r.dropShadowURL(diagramID))
	}

	actorCount := actorIndexMap.Get(a.name)
	if isColorTheme(conf.theme) {
		actElem.Style("stroke", paletteColor(borderColorArray, actorCount))
		actElem.Style("fill", paletteColor(bkgColorArray, actorCount))
	}

	if bands == nil {
		bounds := actElem.BBox()
		a.height = bounds.Height + conf.sequenceLabelBoxHeight
	}

	if !isFooter {
		r.actorCnt++
		r.lifeLine(line, a, center, centerY)

		a.actorCnt, a.hasActorCnt = r.actorCnt, true
	}

	y := rect.Y + 15
	if !isFooter {
		y = rect.Y + 30
	}
	if bands != nil {
		y = bands.labelCenterY - float64(rect.Height/2)
	}
	if err := r.drawTextCandidate(
		a.description,
		actElem,
		rect.X,
		y,
		rect.Width,
		rect.Height,
		[]textAttr{{"class", "actor " + actorManFigureClass}},
	); err != nil {
		return 0, err
	}

	if bands == nil {
		// Legacy nudges. Under the band model the circle is placed exactly, so any leftover translate
		// would reintroduce the stagger the model exists to remove -- getBBox does not see transforms,
		// which is how these hid from every measurement while being plainly visible on screen.
		ty := radius/2 - 5
		if isFooter {
			ty = radius
		}
		actElem.Attr("transform", "translate(0, "+js.Str(ty)+")")
	}
	if !isFooter {
		actElem.Attr("data-et", "participant")
		actElem.Attr("data-type", "entity")
		actElem.Attr("data-id", a.name)
	}

	return a.height, nil
}

func (r *renderer) drawActorTypeDatabase(elem *dom.Node, a *actor, isFooter bool, diagramID string, actorIndexMap *js.Map[string, int]) (float64, error) {
	conf := r.conf
	actorY := actorY(a, isFooter)
	center := a.x + float64(a.width/2)
	bands, err := r.neoBands(a, isFooter, actorY)
	if err != nil {
		return 0, err
	}
	centerY := actorY + a.height + float64(2*conf.boxTextMargin)
	if bands != nil {
		centerY = bands.lifelineStartY
	}
	bkgColorArray, borderColorArray := conf.themeVar("bkgColorArray"), conf.themeVar("borderColorArray")
	actorBorder := conf.themeVar("actorBorder")

	g := r.boxGroup(elem, a, isFooter, center, centerY)

	rect := svgdraw.GetNoteRect()

	cssclass, fromProperties := actorClass(a, isFooter)
	if !fromProperties {
		rect.Fill = "#eaeaea"
	}

	rect.X = a.x
	rect.Y = actorY
	rect.Width = a.width
	rect.Height = a.height
	rect.Class = cssclass
	rect.Name = a.name

	// Cylinder dimensions. The width stays proportional, but under the band model the height is
	// fixed: `width / 3` let a long participant name make the icon taller, and the icon's height
	// must not be able to move anything below it.
	rect.X = a.x
	rect.Y = actorY
	w := rect.Width / 3
	rx := w / 2
	ry := rx / (2.5 + w/50)
	h := rect.Width / 3
	if bands != nil {
		h = glyphBandHeight
	}
	// Vertical anchor: the drawn cylinder spans cylinderY + ry .. cylinderY + h + ry after the
	// translate below, so this puts its bottom on the glyph band's bottom edge.
	cylinderY := rect.Y
	if bands != nil {
		cylinderY = bands.glyphBottomY - h - ry
	}

	// Cylinder base group
	cylinderGroup := g.Append("g")
	cylinderGroup.Attr("class", cssclass)

	d := "\n" +
		"  M " + js.Str(rect.X) + "," + js.Str(cylinderY+ry) + "\n" +
		"  a " + js.Str(rx) + "," + js.Str(ry) + " 0 0 0 " + js.Str(w) + ",0\n" +
		"  a " + js.Str(rx) + "," + js.Str(ry) + " 0 0 0 -" + js.Str(w) + ",0\n" +
		"  l 0," + js.Str(h-float64(2*ry)) + "\n" +
		"  a " + js.Str(rx) + "," + js.Str(ry) + " 0 0 0 " + js.Str(w) + ",0\n" +
		"  l 0,-" + js.Str(h-float64(2*ry)) + "\n"
	// Draw the main cylinder body
	cylinderGroup.Append("path").Attr("d", d)
	if conf.isNeo() {
		cylinderGroup.Attr("filter", r.dropShadowURL(diagramID))
	}
	actorCount := actorIndexMap.Get(a.name)
	if isColorTheme(conf.theme) {
		cylinderGroup.Style("stroke", paletteColor(borderColorArray, actorCount))
		cylinderGroup.Style("fill", paletteColor(bkgColorArray, actorCount))
	} else {
		cylinderGroup.Style("stroke", undef(actorBorder))
	}

	// Both branches were identical upstream: a single unconditional statement.
	cylinderGroup.Attr("transform", "translate("+js.Str(w)+", "+js.Str(ry)+")")
	a.rectData = &rect
	y := rect.Y + 35
	if bands != nil {
		y = bands.labelCenterY - float64(rect.Height/2)
	}
	if err := r.drawTextCandidate(
		a.description,
		g,
		rect.X,
		y,
		rect.Width,
		rect.Height,
		[]textAttr{{"class", "actor " + actorBoxClass}},
	); err != nil {
		return 0, err
	}

	if bands == nil {
		if lastPath := cylinderGroup.Select("path:last-child"); lastPath != nil {
			bounds := lastPath.BBox()
			a.height = bounds.Height + conf.sequenceLabelBoxHeight
		}
	}

	if !isFooter {
		g.Attr("data-et", "participant")
		g.Attr("data-type", "database")
		g.Attr("data-id", a.name)
	}

	return a.height, nil
}

func (r *renderer) drawActorTypeBoundary(elem *dom.Node, a *actor, isFooter bool, diagramID string, actorIndexMap *js.Map[string, int]) (float64, error) {
	conf := r.conf
	actorY := actorY(a, isFooter)
	center := a.x + float64(a.width/2)
	bands, err := r.neoBands(a, isFooter, actorY)
	if err != nil {
		return 0, err
	}
	centerY := actorY + 80
	if bands != nil {
		centerY = bands.lifelineStartY
	}
	const radius = 22.0
	iconCenterY := actorY + 12
	if bands != nil {
		iconCenterY = bands.glyphBottomY - radius
	}
	line := elem.Append("g").Lower()
	bkgColorArray, borderColorArray := conf.themeVar("bkgColorArray"), conf.themeVar("borderColorArray")
	actorBorder := conf.themeVar("actorBorder")

	if !isFooter {
		r.actorCnt++
		r.lifeLine(line, a, center, centerY)

		a.actorCnt, a.hasActorCnt = r.actorCnt, true
	}
	actElem := elem.Append("g")
	cssClass := actorManFigureClass
	if isFooter {
		cssClass += " " + bottomActorClass
	} else {
		cssClass += " " + topActorClass
	}
	actElem.Attr("class", cssClass)
	actElem.Attr("name", a.name)

	rect := svgdraw.GetNoteRect()
	rect.X = a.x
	rect.Y = actorY
	rect.Fill = "#eaeaea"
	rect.Width = a.width
	rect.Height = a.height
	rect.Class = "actor"

	actElem.
		Append("line").
		Attr("id", "actor-man-torso"+strconv.Itoa(r.actorCnt)).
		Attr("x1", a.x+float64(a.width/2)-float64(radius*2.5)).
		Attr("y1", iconCenterY).
		Attr("x2", a.x+float64(a.width/2)-15).
		Attr("y2", iconCenterY)

	actElem.
		Append("line").
		Attr("id", "actor-man-arms"+strconv.Itoa(r.actorCnt)).
		Attr("x1", a.x+float64(a.width/2)-float64(radius*2.5)).
		Attr("y1", iconCenterY-10). // starting Y
		Attr("x2", a.x+float64(a.width/2)-float64(radius*2.5)).
		Attr("y2", iconCenterY+10) // ending Y

	actElem.
		Append("circle").
		Attr("cx", a.x+float64(a.width/2)).
		Attr("cy", iconCenterY).
		Attr("r", radius)

	if conf.isNeo() {
		actElem.Attr("filter", r.dropShadowURL(diagramID))
	}

	actorCount := actorIndexMap.Get(a.name)
	if isColorTheme(conf.theme) {
		actElem.Style("stroke", paletteColor(borderColorArray, actorCount))
		actElem.Style("fill", paletteColor(bkgColorArray, actorCount))
	} else {
		actElem.Style("stroke", undef(actorBorder))
	}
	if bands == nil {
		bounds := actElem.BBox()
		a.height = bounds.Height + conf.sequenceLabelBoxHeight
	}

	y := rect.Y + 15
	if bands != nil {
		y = bands.labelCenterY - float64(rect.Height/2)
	}
	if err := r.drawTextCandidate(
		a.description,
		actElem,
		rect.X,
		y,
		rect.Width,
		rect.Height,
		[]textAttr{{"class", "actor " + actorManFigureClass}},
	); err != nil {
		return 0, err
	}

	if bands == nil {
		// Legacy nudge; see the note in drawActorTypeEntity.
		actElem.Attr("transform", "translate(0,"+js.Str(radius/2+10)+")")
	}

	if !isFooter {
		actElem.Attr("data-et", "participant")
		actElem.Attr("data-type", "boundary")
		actElem.Attr("data-id", a.name)
	}

	return a.height, nil
}

func (r *renderer) drawActorTypeActor(elem *dom.Node, a *actor, isFooter bool, actorIndexMap *js.Map[string, int]) (float64, error) {
	conf := r.conf
	actorY := actorY(a, isFooter)
	center := a.x + float64(a.width/2)
	bands, err := r.neoBands(a, isFooter, actorY)
	if err != nil {
		return 0, err
	}
	centerY := actorY + 80
	if bands != nil {
		centerY = bands.lifelineStartY
	}
	bkgColorArray, borderColorArray := conf.themeVar("bkgColorArray"), conf.themeVar("borderColorArray")
	actorBorder := conf.themeVar("actorBorder")

	line := elem.Append("g").Lower()

	if !isFooter {
		r.actorCnt++
		r.lifeLine(line, a, center, centerY)

		a.actorCnt, a.hasActorCnt = r.actorCnt, true
	}
	actElem := elem.Append("g")
	cssClass := actorManFigureClass
	if isFooter {
		cssClass += " " + bottomActorClass
	} else {
		cssClass += " " + topActorClass
	}
	actElem.Attr("class", cssClass)
	actElem.Attr("name", a.name)

	if !isFooter {
		actElem.Attr("data-et", "participant").Attr("data-type", "actor").Attr("data-id", a.name)
	}

	// Under the band model the figure is scaled to fill the shared glyph band, feet on its bottom
	// edge, the same size as the round icons beside it. `classic` draws the legacy coordinates.
	glyphScale := 1.0
	if bands != nil {
		glyphScale = glyphBandHeight / actorGlyphHeight
	}
	gy := func(offset float64) float64 {
		if bands != nil {
			return bands.glyphBottomY - float64((actorGlyphBottom-offset)*glyphScale)
		}
		return actorY + offset
	}
	gx := func(offset float64) float64 { return center + float64(offset*glyphScale) }

	actElem.
		Append("line").
		Attr("id", "actor-man-torso"+strconv.Itoa(r.actorCnt)).
		Attr("x1", center).
		Attr("y1", gy(25)).
		Attr("x2", center).
		Attr("y2", gy(45))

	actElem.
		Append("line").
		Attr("id", "actor-man-arms"+strconv.Itoa(r.actorCnt)).
		Attr("x1", gx(-actorTypeWidth/2)).
		Attr("y1", gy(33)).
		Attr("x2", gx(actorTypeWidth/2)).
		Attr("y2", gy(33))
	actElem.
		Append("line").
		Attr("x1", gx(-actorTypeWidth/2)).
		Attr("y1", gy(60)).
		Attr("x2", center).
		Attr("y2", gy(45))
	actElem.
		Append("line").
		Attr("x1", center).
		Attr("y1", gy(45)).
		Attr("x2", gx(actorTypeWidth/2-2)).
		Attr("y2", gy(60))

	circle := actElem.Append("circle")
	circle.Attr("cx", a.x+float64(a.width/2))
	circle.Attr("cy", gy(10))
	circle.Attr("r", float64(15*glyphScale))
	circle.Attr("width", a.width)
	circle.Attr("height", a.height)

	if bands == nil {
		// Classic reports the glyph's fixed extent, as it always measured out to.
		a.height = actorGlyphHeight
	}

	rect := svgdraw.GetNoteRect()
	rect.X = a.x
	rect.Y = actorY
	rect.Fill = "#eaeaea"
	rect.Width = a.width
	rect.Height = a.height
	rect.Class = "actor"
	rect.Rx = 3
	rect.Ry = 3

	actorCount := actorIndexMap.Get(a.name)
	if isColorTheme(conf.theme) {
		actElem.Style("stroke", paletteColor(borderColorArray, actorCount))
		actElem.Style("fill", paletteColor(bkgColorArray, actorCount))
	} else {
		actElem.Style("stroke", undef(actorBorder))
	}

	y := actorY + 35
	if bands != nil {
		y = bands.labelCenterY - float64(rect.Height/2)
	}
	if err := r.drawTextCandidate(
		a.description,
		actElem,
		rect.X,
		y,
		rect.Width,
		rect.Height,
		[]textAttr{{"class", "actor " + actorManFigureClass}},
	); err != nil {
		return 0, err
	}

	return a.height, nil
}

// drawActor is drawActor(elem, actor, conf, isFooter, diagramId, diagObj,
// actorIndexMap). The result is NaN (undefined) for an actor type that has
// no shape.
func (r *renderer) drawActor(elem *dom.Node, a *actor, isFooter bool, diagramID string, actorIndexMap *js.Map[string, int]) (float64, error) {
	if a == nil {
		return 0, errors.New("TypeError: Cannot read properties of undefined (reading 'type')")
	}
	if a.typRaw != nil {
		return nan, nil
	}
	switch a.typ {
	case participantActor:
		return r.drawActorTypeActor(elem, a, isFooter, actorIndexMap)
	case participantParticipant:
		return r.drawActorTypeParticipant(elem, a, isFooter, diagramID, actorIndexMap)
	case participantBoundary:
		return r.drawActorTypeBoundary(elem, a, isFooter, diagramID, actorIndexMap)
	case participantControl:
		return r.drawActorTypeControl(elem, a, isFooter, diagramID, actorIndexMap)
	case participantEntity:
		return r.drawActorTypeEntity(elem, a, isFooter, diagramID, actorIndexMap)
	case participantDatabase:
		return r.drawActorTypeDatabase(elem, a, isFooter, diagramID, actorIndexMap)
	case participantCollections:
		return r.drawActorTypeCollections(elem, a, isFooter, diagramID, actorIndexMap)
	case participantQueue:
		return r.drawActorTypeQueue(elem, a, isFooter, diagramID, actorIndexMap)
	}
	return nan, nil
}

// drawBox is drawBox(elem, box, conf).
func (r *renderer) drawBox(elem *dom.Node, b *box) {
	conf := r.conf
	boxplusTextGroup := elem.Append("g")
	g := boxplusTextGroup
	drawBackgroundRect(g, svgdraw.Bound{
		Startx: b.startx,
		Stopx:  b.stopx,
		Starty: b.starty,
		Stopy:  b.stopy,
		Fill:   b.fill,
		Stroke: b.stroke,
	})
	if b.name != "" {
		// A text with KaTeX delimiters is drawn as plain text here: drawBox
		// does not pass hasKatex.
		r.byTextPlacement(
			b.name,
			g,
			b.x,
			b.y+conf.boxTextMargin+float64(orNum(b.textMaxHeight, 0)/2),
			b.width,
			0,
			[]textAttr{{"class", "text"}},
		)
	}
	g.Lower()
}

// anchorElement is anchorElement(elem).
func anchorElement(elem *dom.Node) *dom.Node {
	return elem.Append("g")
}

// drawActivation draws an activation in the diagram.
func (r *renderer) drawActivation(bounds *activation, verticalPos float64, actorActivations int, actorIndexMap *js.Map[string, int]) {
	conf := r.conf
	bkgColorArray, borderColorArray := conf.themeVar("bkgColorArray"), conf.themeVar("borderColorArray")
	mainBkg := conf.themeVar("mainBkg")
	rect := svgdraw.GetNoteRect()
	g := bounds.anchored
	actor := bounds.actor
	rect.X = bounds.startx.nan()
	rect.Y = bounds.starty.nan()
	rect.Class = "activation" + strconv.Itoa(actorActivations%3) // Will evaluate to 0, 1 or 2
	rect.Width = bounds.stopx.nan() - bounds.startx.nan()
	rect.Height = verticalPos - bounds.starty.nan()

	rectElem := r.drawRect(g, rect)
	actorCount := actorIndexMap.Get(actor)
	if isColorTheme(conf.theme) {
		rectElem.Style("stroke", paletteColor(borderColorArray, actorCount))
		fill := paletteColor(bkgColorArray, actorCount)
		if fill == nil {
			fill = undef(mainBkg)
		}
		rectElem.Style("fill", fill)
	}
}

// drawLoop draws a loop in the diagram.
func (r *renderer) drawLoop(elem *dom.Node, loopModel *loopModel, labelText string, msg *message) error {
	conf := r.conf
	boxMargin, boxTextMargin := conf.boxMargin, conf.boxTextMargin
	labelBoxHeight, labelBoxWidth := conf.labelBoxHeight, conf.labelBoxWidth
	fontFamily, fontSize, fontWeight := conf.messageFontFamily, conf.messageFontSize, conf.messageFontWeight
	g := elem.
		Append("g").
		Attr("data-et", "control-structure").
		Attr("data-id", "i"+msg.id)
	drawLoopLine := func(startx, starty, stopx, stopy float64) *dom.Node {
		return g.
			Append("line").
			Attr("x1", startx).
			Attr("y1", starty).
			Attr("x2", stopx).
			Attr("y2", stopy).
			Attr("class", "loopLine")
	}
	startx, starty, stopx, stopy := loopModel.startx.nan(), loopModel.starty.nan(), loopModel.stopx.nan(), loopModel.stopy.nan()
	drawLoopLine(startx, starty, stopx, starty)
	drawLoopLine(stopx, starty, stopx, stopy)
	drawLoopLine(startx, stopy, stopx, stopy)
	drawLoopLine(startx, starty, startx, stopy)
	for _, item := range loopModel.sections {
		drawLoopLine(startx, item.y, stopx, item.y).Style(
			"stroke-dasharray",
			"3, 3",
		)
	}

	txt := getTextObjCommon()
	txt.text = labelText
	txt.x = startx
	txt.y = starty
	txt.fontFamily = fontFamily
	txt.fontSize = fontSize
	txt.fontWeight = fontWeight
	txt.anchor = "middle"
	txt.valign = "middle"
	txt.tspan = false
	labelWidth := 0.0
	if conf.hasLabelBoxWidth {
		labelWidth = labelBoxWidth
	}
	txt.width, txt.hasWidth = js.Max(labelWidth, 50), true
	neoExtra := 0.0
	if conf.isNeo() {
		neoExtra = 15
	}
	txt.height = orNum(labelBoxHeight+neoExtra, 20)
	txt.textMargin = boxTextMargin
	txt.class = "labelText"

	drawLabel(g, txt)
	txt = getTextObj()
	if !loopModel.hasTitle {
		return errors.New("TypeError: Cannot read properties of undefined (reading 'match')")
	}
	txt.text = loopModel.title
	txt.x = startx + float64(labelBoxWidth/2) + float64((stopx-startx)/2)
	txt.y = starty + boxMargin + boxTextMargin
	txt.anchor = "middle"
	txt.valign = "middle"
	txt.textMargin = boxTextMargin
	txt.class = "loopText"
	txt.fontFamily = fontFamily
	txt.fontSize = fontSize
	txt.fontWeight = fontWeight
	txt.wrap = true

	if sanitize.HasKatex(txt.text) {
		return errKatex
	}
	textElem := drawText(g, txt)

	for idx, item := range loopModel.sectionTitles {
		if item.message != "" {
			txt.text = item.message
			txt.x = startx + float64((stopx-startx)/2)
			txt.y = loopModel.sections[idx].y + boxMargin + boxTextMargin
			txt.class = "sectionTitle"
			txt.anchor = "middle"
			txt.valign = "middle"
			txt.tspan = false
			txt.fontFamily = fontFamily
			txt.fontSize = fontSize
			txt.fontWeight = fontWeight
			txt.wrap = loopModel.wrap

			if sanitize.HasKatex(txt.text) {
				return errKatex
			}
			drawText(g, txt)
			sectionHeight := sumHeights(textElem)
			loopModel.sections[idx].height += sectionHeight - (boxMargin + boxTextMargin)
		}
	}

	loopModel.height = js.Round(stopy - starty)
	return nil
}

// drawBackgroundRect draws a background rectangle.
func drawBackgroundRect(elem *dom.Node, bounds svgdraw.Bound) {
	svgdraw.DrawBackgroundRect(elem, bounds)
}

func insertDatabaseIcon(elem *dom.Node, id string) {
	elem.
		Append("defs").
		Append("symbol").
		Attr("id", id+"-database").
		Attr("fill-rule", "evenodd").
		Attr("clip-rule", "evenodd").
		Append("path").
		Attr("transform", "scale(.5)").
		Attr("d", databaseIconPath)
}

func insertComputerIcon(elem *dom.Node, id string) {
	elem.
		Append("defs").
		Append("symbol").
		Attr("id", id+"-computer").
		Attr("width", "24").
		Attr("height", "24").
		Append("path").
		Attr("transform", "scale(.5)").
		Attr(
			"d",
			"M2 2v13h20v-13h-20zm18 11h-16v-9h16v9zm-10.228 6l.466-1h3.524l.467 1h-4.457zm14.228 3h-24l2-6h2.104l-1.33 4h18.45l-1.297-4h2.073l2 6zm-5-10h-14v-7h14v7z",
		)
}

func insertClockIcon(elem *dom.Node, id string) {
	elem.
		Append("defs").
		Append("symbol").
		Attr("id", id+"-clock").
		Attr("width", "24").
		Attr("height", "24").
		Append("path").
		Attr("transform", "scale(.5)").
		Attr(
			"d",
			"M12 2c5.514 0 10 4.486 10 10s-4.486 10-10 10-10-4.486-10-10 4.486-10 10-10zm0-2c-6.627 0-12 5.373-12 12s5.373 12 12 12 12-5.373 12-12-5.373-12-12-12zm5.848 12.459c.202.038.202.333.001.372-1.907.361-6.045 1.111-6.547 1.111-.719 0-1.301-.582-1.301-1.301 0-.512.77-5.447 1.125-7.445.034-.192.312-.181.343.014l.985 6.238 5.394 1.011z",
		)
}

// insertArrowHead sets up the arrow head and defines the marker. The result
// is appended to the svg.
func insertArrowHead(elem *dom.Node, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"-arrowhead").
		Attr("refX", 7.9).
		Attr("refY", 5).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 12).
		Attr("markerHeight", 12).
		Attr("orient", "auto-start-reverse").
		Append("path").
		Attr("d", "M -1 0 L 10 5 L 0 10 z") // this is actual shape for arrowhead
}

func insertArrowFilledHead(elem *dom.Node, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"-filled-head").
		Attr("refX", 15.5).
		Attr("refY", 7).
		Attr("markerWidth", 20).
		Attr("markerHeight", 28).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 18,7 L9,13 L14,7 L9,1 Z")
}

// insertSequenceNumber sets up the node number. The result is appended to
// the svg.
func insertSequenceNumber(elem *dom.Node, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"-sequencenumber").
		Attr("refX", 15).
		Attr("refY", 15).
		Attr("markerWidth", 60).
		Attr("markerHeight", 40).
		Attr("orient", "auto").
		Append("circle").
		Attr("cx", 15).
		Attr("cy", 15).
		Attr("r", 6)
}

// insertArrowCrossHead sets up the cross head and defines the marker.
func insertArrowCrossHead(elem *dom.Node, id string) {
	defs := elem.Append("defs")
	marker := defs.
		Append("marker").
		Attr("id", id+"-crosshead").
		Attr("markerWidth", 15).
		Attr("markerHeight", 8).
		Attr("orient", "auto").
		Attr("refX", 4).
		Attr("refY", 4.5)
	// The cross
	marker.
		Append("path").
		Attr("fill", "none").
		Attr("stroke", "#000000").
		Style("stroke-dasharray", "0, 0").
		Attr("stroke-width", "1pt").
		Attr("d", "M 1,2 L 6,7 M 6,2 L 1,7")
}

// dropShadowID is dropShadowId(diagramId).
func dropShadowID(diagramID string) string {
	if diagramID != "" {
		return diagramID + "-drop-shadow"
	}
	return "drop-shadow"
}

func insertDropShadow(elem *dom.Node, conf *conf, diagramID string) {
	floodColor := "#FFFFFF"
	if conf.theme == "redux" || conf.theme == "redux-color" {
		floodColor = "#000000"
	}
	elem.
		Append("defs").
		Append("filter").
		Attr("id", dropShadowID(diagramID)).
		Attr("height", "130%").
		Attr("width", "130%").
		Append("feDropShadow").
		Attr("dx", "4").
		Attr("dy", "4").
		Attr("stdDeviation", 0).
		Attr("flood-opacity", "0.06").
		Attr("flood-color", floodColor)
}

// textAttr is one entry of a textAttrs object.
type textAttr struct {
	key   string
	value string
}

// setTextAttrs is _setTextAttrs(toText, fromTextAttrsDict).
func setTextAttrs(toText *dom.Node, fromTextAttrsDict []textAttr) {
	for _, attr := range fromTextAttrsDict {
		toText.Attr(attr.key, attr.value)
	}
}

// drawTextCandidate is _drawTextCandidateFunc(conf, hasKatex(content))(...).
func (r *renderer) drawTextCandidate(content string, g *dom.Node, x, y, width, height float64, textAttrs []textAttr) error {
	if sanitize.HasKatex(content) {
		return errKatex
	}
	r.byTextPlacement(content, g, x, y, width, height, textAttrs)
	return nil
}

// byTextPlacement is the function that _drawTextCandidateFunc(conf) returns
// for a text without KaTeX.
func (r *renderer) byTextPlacement(content string, g *dom.Node, x, y, width, height float64, textAttrs []textAttr) {
	switch r.conf.textPlacement {
	case "fo":
		r.byFo(content, g, x, y, width, height, textAttrs)
	case "old":
		byText(content, g, x, y, width, height, textAttrs)
	default:
		r.byTspan(content, g, x, y, width, height, textAttrs)
	}
}

func byText(content string, g *dom.Node, x, y, width, height float64, textAttrs []textAttr) {
	text := g.
		Append("text").
		Attr("x", x+float64(width/2)).
		Attr("y", y+float64(height/2)+5).
		Style("text-anchor", "middle").
		Text(content)
	setTextAttrs(text, textAttrs)
}

func (r *renderer) byTspan(content string, g *dom.Node, x, y, width, height float64, textAttrs []textAttr) {
	conf := r.conf
	actorFontSize, actorFontFamily, actorFontWeight := conf.actorFontSize, conf.actorFontFamily, conf.actorFontWeight

	_actorFontSize, _actorFontSizePx, ok := utils.ParseFontSize(actorFontSize)
	var fontSizePx any
	if ok {
		fontSizePx = _actorFontSizePx
	} else {
		_actorFontSize = nan
	}

	lines := sanitize.SplitBreaks(content)
	for i := range lines {
		dy := float64(float64(i)*_actorFontSize) - float64((_actorFontSize*float64(len(lines)-1))/2)
		text := g.
			Append("text").
			Attr("x", x+float64(width/2)).
			Attr("y", y).
			Style("text-anchor", "middle").
			Style("font-size", fontSizePx).
			Style("font-weight", undef(actorFontWeight)).
			Style("font-family", undef(actorFontFamily))
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

func (r *renderer) byFo(content string, g *dom.Node, x, y, width, height float64, textAttrs []textAttr) {
	s := g.Append("switch")
	f := s.
		Append("foreignObject").
		Attr("x", x).
		Attr("y", y).
		Attr("width", width).
		Attr("height", height)

	text := f.
		Append("xhtml:div").
		Style("display", "table").
		Style("height", "100%").
		Style("width", "100%")

	text.
		Append("div").
		Style("display", "table-cell").
		Style("text-align", "center").
		Style("vertical-align", "middle").
		Text(content)

	r.byTspan(content, s, x, y, width, height, textAttrs)
	setTextAttrs(text, textAttrs)
}

// drawMenuItemTextCandidate is _drawMenuItemTextCandidateFunc(conf)(...).
func (r *renderer) drawMenuItemTextCandidate(content string, g *dom.Node, x, y, width, height float64, textAttrs []textAttr) {
	switch r.conf.textPlacement {
	case "fo":
		r.menuByFo(content, g, x, y, width, height, textAttrs)
	case "old":
		text := g.
			Append("text").
			Attr("x", x).
			Attr("y", y).
			Style("text-anchor", "start").
			Text(content)
		setTextAttrs(text, textAttrs)
	default:
		r.menuByTspan(content, g, x, y, height, textAttrs)
	}
}

func (r *renderer) menuByTspan(content string, g *dom.Node, x, y, height float64, textAttrs []textAttr) {
	conf := r.conf
	actorFontSize, actorFontFamily, actorFontWeight := conf.actorFontSize, conf.actorFontFamily, conf.actorFontWeight
	// The font size is used as a number here, without parseFontSize.
	size := nan
	if actorFontSize != nil {
		size = js.ToNumber(actorFontSize)
	}

	lines := sanitize.SplitBreaks(content)
	for i := range lines {
		dy := float64(float64(i)*size) - float64((size*float64(len(lines)-1))/2)
		text := g.
			Append("text").
			Attr("x", x).
			Attr("y", y).
			Style("text-anchor", "start").
			Style("font-size", undef(actorFontSize)).
			Style("font-weight", undef(actorFontWeight)).
			Style("font-family", undef(actorFontFamily))
		text.Append("tspan").Attr("x", x).Attr("dy", dy).Text(lines[i])

		text.
			Attr("y", y+float64(height/2.0)).
			Attr("dominant-baseline", "central").
			Attr("alignment-baseline", "central")

		setTextAttrs(text, textAttrs)
	}
}

func (r *renderer) menuByFo(content string, g *dom.Node, x, y, width, height float64, textAttrs []textAttr) {
	s := g.Append("switch")
	f := s.
		Append("foreignObject").
		Attr("x", x).
		Attr("y", y).
		Attr("width", width).
		Attr("height", height)

	text := f.
		Append("xhtml:div").
		Style("display", "table").
		Style("height", "100%").
		Style("width", "100%")

	text.
		Append("div").
		Style("display", "table-cell").
		Style("text-align", "center").
		Style("vertical-align", "middle").
		Text(content)

	r.menuByTspan(content, s, x, y, height, textAttrs)
	setTextAttrs(text, textAttrs)
}

// insertSolidTopArrowHead sets up the arrow head and defines the marker.
func insertSolidTopArrowHead(elem *dom.Node, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"-solidTopArrowHead").
		Attr("refX", 7.9).
		Attr("refY", 7.25).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 12).
		Attr("markerHeight", 12).
		Attr("orient", "auto-start-reverse").
		Append("path").
		Attr("d", "M 0 0 L 10 8 L 0 8 z") // this is actual shape for arrowhead
}

func insertSolidBottomArrowHead(elem *dom.Node, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"-solidBottomArrowHead").
		Attr("refX", 7.9).
		Attr("refY", 0.75).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 12).
		Attr("markerHeight", 12).
		Attr("orient", "auto-start-reverse").
		Append("path").
		Attr("d", "M 0 0 L 10 0 L 0 8 z")
}

func insertStickTopArrowHead(elem *dom.Node, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"-stickTopArrowHead").
		Attr("refX", 7.5).
		Attr("refY", 7).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 12).
		Attr("markerHeight", 12).
		Attr("orient", "auto-start-reverse").
		Append("path").
		Attr("d", "M 0 0 L 7 7").
		Attr("stroke", "black").
		Attr("stroke-width", 1.5).
		Attr("fill", "none")
}

func insertStickBottomArrowHead(elem *dom.Node, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"-stickBottomArrowHead").
		Attr("refX", 7.5).
		Attr("refY", 0).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 12).
		Attr("markerHeight", 12).
		Attr("orient", "auto-start-reverse").
		Append("path").
		Attr("d", "M 0 7 L 7 0").
		Attr("stroke", "black").
		Attr("stroke-width", 1.5).
		Attr("fill", "none")
}
