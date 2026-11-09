// Ported from mermaid 12.2.0, diagrams/sequence/sequenceRenderer.ts (MIT).

package sequence

import (
	"errors"
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
	"github.com/bvolpato/mermaid-go-renderer/internal/svgdraw"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// renderer holds the module state of sequenceRenderer.ts and of svgDraw.js
// for one render.
type renderer struct {
	ctx    *render.Context
	db     *db
	conf   *conf
	bounds boundsState
	// look and themeVariables are the properties of getConfig().
	look           any
	themeVariables *js.Obj[any]
	// actorCnt is the module variable of svgDraw.js. It starts at -1.
	actorCnt int
}

// errKatex reports a text with KaTeX delimiters. The KaTeX rendering of
// common.ts is not ported.
var errKatex = errors.New("sequence: a text with $$ math needs KaTeX, which is not supported")

// pageLocation is window.location of the page of the reference renderer.
var pageLocation = sanitize.Location{Protocol: "about:", Host: "", Pathname: "blank", Search: ""}

func (r *renderer) calculateTextDimensions(text string, font utils.TextDimensionConfig) (utils.TextDimensions, error) {
	return utils.CalculateTextDimensions(r.ctx, text, font)
}

func (r *renderer) wrapLabel(label string, maxWidth float64, font utils.TextDimensionConfig) (string, error) {
	return utils.WrapLabel(r.ctx, label, maxWidth, utils.WrapLabelConfig{
		FontSize:   font.FontSize,
		FontWeight: font.FontWeight,
		FontFamily: font.FontFamily,
	})
}

// sumHeights is Math.round(textElem.map((te) => te.getBBox().height)
// .reduce((acc, curr) => acc + curr)).
func sumHeights(elems []*dom.Node) float64 {
	sum := 0.0
	for i, te := range elems {
		h := te.BBox().Height
		if i == 0 {
			sum = h
		} else {
			sum = sum + h
		}
	}
	return js.Round(sum)
}

// drawNote draws a note in the diagram with the attached line.
func (r *renderer) drawNote(elem *dom.Node, noteModel *noteModel, id string) error {
	conf := r.conf
	bounds := &r.bounds
	bounds.bumpVerticalPos(conf.boxMargin)
	noteModel.height = conf.boxMargin
	noteModel.starty = bounds.getVerticalPos()
	rect := svgdraw.GetNoteRect()
	rect.X = noteModel.startx
	rect.Y = noteModel.starty
	rect.Width = noteModel.width
	if !(noteModel.width != 0 && !math.IsNaN(noteModel.width)) {
		rect.Width = conf.width
	}
	rect.Class = "note"

	g := elem.Append("g")
	g.Attr("data-et", "note")
	g.Attr("data-id", "i"+id)
	rectElem := r.drawRect(g, rect)
	textObj := getTextObjCommon()
	textObj.x = noteModel.startx
	textObj.y = noteModel.starty
	textObj.width, textObj.hasWidth = rect.Width, true
	textObj.dy = "1em"
	textObj.text = noteModel.message
	textObj.class = "noteText"
	textObj.fontFamily = conf.noteFontFamily
	textObj.fontSize = conf.noteFontSize
	textObj.fontWeight = conf.noteFontWeight
	textObj.anchor = conf.noteAlign
	textObj.textMargin = conf.noteMargin
	textObj.valign = "center"

	if sanitize.HasKatex(textObj.text) {
		return errKatex
	}
	textElem := drawText(g, textObj)

	textHeight := sumHeights(textElem)

	rectElem.Attr("height", textHeight+float64(2*conf.noteMargin))
	noteModel.height += textHeight + float64(2*conf.noteMargin)
	bounds.bumpVerticalPos(textHeight + float64(2*conf.noteMargin))
	noteModel.stopy = noteModel.starty + textHeight + float64(2*conf.noteMargin)
	noteModel.stopx = noteModel.startx + rect.Width
	bounds.insert(noteModel.startx, noteModel.starty, noteModel.stopx, noteModel.stopy)
	bounds.models.notes = append(bounds.models.notes, noteModel)
	return nil
}

func (r *renderer) drawCentralConnection(elem *dom.Node, msg *message, msgModel *msgModel, lineStartY float64) error {
	actors := r.db.actors
	fromActor := actors.Get(msg.from)
	toActor := actors.Get(msg.to)
	if fromActor == nil || toActor == nil {
		return errors.New("TypeError: Cannot read properties of undefined (reading 'x')")
	}
	isAutoNumberOn := msgModel.sequenceVisible
	fromCenter := fromActor.x + float64(fromActor.width/2)
	toCenter := toActor.x + float64(toActor.width/2)

	// Determine arrow direction: left-to-right or right-to-left
	isLeftToRight := fromCenter <= toCenter
	isReverse := isReverseArrowType(msg)

	g := elem.Append("g")

	const centralConnectionCircleOffset = 16.5

	getCircleOffset := func(isLeftToRight, isReverse bool) float64 {
		baseOffset := -centralConnectionCircleOffset
		if isLeftToRight {
			baseOffset = centralConnectionCircleOffset
		}
		if isReverse {
			return -baseOffset
		}
		return baseOffset
	}

	drawCircle := func(cx float64) {
		g.Append("circle").
			Attr("cx", cx).
			Attr("cy", lineStartY).
			Attr("r", 5).
			Attr("width", 10).
			Attr("height", 10)
	}

	// Calculate circle position adjustments when autonumber is enabled
	if isAutoNumberOn {
		switch msg.centralConnection {
		case lineCentralConnection:
			// Pattern: actor ->>() actor - circle at destination
			if isReverse {
				toCenter += getCircleOffset(isLeftToRight, true)
			}
			// No adjustment for normal arrows
		case lineCentralConnectionReverse:
			// Pattern: actor ()->> actor - circle at source
			if !isReverse {
				fromCenter += getCircleOffset(isLeftToRight, false)
			}
			// No adjustment for reverse arrows
		case lineCentralConnectionDual:
			// Pattern: actor ()->>() actor - circles at both ends
			if isReverse {
				toCenter += getCircleOffset(isLeftToRight, true)
			} else {
				fromCenter += getCircleOffset(isLeftToRight, false)
			}
		}
	}

	// Draw circles based on central connection type
	switch msg.centralConnection {
	case lineCentralConnection:
		drawCircle(toCenter)
	case lineCentralConnectionReverse:
		drawCircle(fromCenter)
	case lineCentralConnectionDual:
		drawCircle(fromCenter)
		drawCircle(toCenter)
	}
	return nil
}

// boundMessage processes a message by adding its dimensions to the bound. It
// returns the Y coordinate of the message so it can be drawn later. The
// message is not drawn at this point so the arrowhead can be on top of the
// activation box.
func (r *renderer) boundMessage(msgModel *msgModel) (float64, error) {
	conf := r.conf
	bounds := &r.bounds
	bounds.bumpVerticalPos(10)
	startx, stopx, message := msgModel.startx, msgModel.stopx, msgModel.message
	lines := float64(len(sanitize.SplitBreaks(message)))
	if sanitize.HasKatex(message) {
		return 0, errKatex
	}
	textDims, err := r.calculateTextDimensions(message, conf.messageFont())
	if err != nil {
		return 0, err
	}

	lineHeight := textDims.Height / lines
	msgModel.height += lineHeight
	bounds.bumpVerticalPos(lineHeight)

	var lineStartY float64
	totalOffset := textDims.Height - 10
	textWidth := textDims.Width

	if startx == stopx {
		lineStartY = bounds.getVerticalPos() + totalOffset
		if !conf.rightAngles {
			totalOffset += conf.boxMargin
			lineStartY = bounds.getVerticalPos() + totalOffset
		}
		totalOffset += 30
		dx := sanitize.GetMax(textWidth/2, conf.width/2)
		bounds.insert(
			startx-dx,
			bounds.getVerticalPos()-10+totalOffset,
			stopx+dx,
			bounds.getVerticalPos()+30+totalOffset,
		)
	} else {
		totalOffset += conf.boxMargin
		lineStartY = bounds.getVerticalPos() + totalOffset
		bounds.insert(startx, lineStartY-10, stopx, lineStartY)
	}
	bounds.bumpVerticalPos(totalOffset)
	msgModel.height += totalOffset
	msgModel.stopy = msgModel.starty + msgModel.height
	bounds.insert(msgModel.fromBounds, msgModel.starty, msgModel.toBounds, msgModel.stopy)

	return lineStartY, nil
}

// drawMessage draws a message. The bounds have previously been updated by
// boundMessage.
func (r *renderer) drawMessage(diagram *dom.Node, msgModel *msgModel, lineStartY float64, msg *message, diagramID string) error {
	conf := r.conf
	startx, stopx, starty, message, typ, sequenceIndex, sequenceVisible :=
		msgModel.startx, msgModel.stopx, msgModel.starty, msgModel.message, msgModel.typ, msgModel.sequenceIndex, msgModel.sequenceVisible
	textDims, err := r.calculateTextDimensions(message, conf.messageFont())
	if err != nil {
		return err
	}
	textObj := getTextObjCommon()
	textObj.x = min(startx, stopx)
	textObj.y = starty + 10
	textObj.width, textObj.hasWidth = math.Abs(stopx-startx), true
	textObj.class = "messageText"
	textObj.dy = "1em"
	textObj.text = message
	textObj.fontFamily = conf.messageFontFamily
	textObj.fontSize = conf.messageFontSize
	textObj.fontWeight = conf.messageFontWeight
	textObj.anchor = conf.messageAlign
	textObj.valign = "center"
	textObj.textMargin = conf.wrapPadding
	textObj.tspan = false

	if sanitize.HasKatex(textObj.text) {
		return errKatex
	}
	drawText(diagram, textObj)

	textWidth := textDims.Width

	var line *dom.Node
	if startx == stopx {
		isAutoNumberOn := sequenceVisible || conf.showSequenceNumbers
		isReverse := isReverseArrowType(msg)
		isBidirectional := isBidirectionalArrowType(msg)
		lineStartX := startx
		if isAutoNumberOn && (isReverse || isBidirectional) {
			lineStartX = startx + 10
		}

		if conf.rightAngles {
			line = diagram.
				Append("path").
				Attr("d",
					"M  "+js.Str(lineStartX)+","+js.Str(lineStartY)+" H "+
						js.Str(startx+sanitize.GetMax(conf.width/2, textWidth/2))+
						" V "+js.Str(lineStartY+25)+" H "+js.Str(startx))
		} else {
			line = diagram.
				Append("path").
				Attr("d",
					"M "+
						js.Str(lineStartX)+
						","+
						js.Str(lineStartY)+
						" C "+
						js.Str(lineStartX+60)+
						","+
						js.Str(lineStartY-10)+
						" "+
						js.Str(startx+60)+
						","+
						js.Str(lineStartY+30)+
						" "+
						js.Str(startx)+
						","+
						js.Str(lineStartY+20))
		}
		// Draw central connection circles for self-connection arrows
		if hasCentralConnection(msg) {
			if err := r.drawCentralConnection(diagram, msg, msgModel, lineStartY); err != nil {
				return err
			}
		}
	} else {
		line = diagram.Append("line")
		line.Attr("x1", startx)
		line.Attr("y1", lineStartY)
		line.Attr("x2", stopx)
		line.Attr("y2", lineStartY)
		if hasCentralConnection(msg) {
			if err := r.drawCentralConnection(diagram, msg, msgModel, lineStartY); err != nil {
				return err
			}
		}
	}
	// Make an SVG Container
	// Draw the line
	if typ == lineDotted ||
		typ == lineDottedCross ||
		typ == lineDottedPoint ||
		typ == lineDottedOpen ||
		typ == lineBidirectionalDotted ||
		typ == lineSolidTopDotted ||
		typ == lineSolidBottomDotted ||
		typ == lineStickTopDotted ||
		typ == lineStickBottomDotted ||
		typ == lineSolidArrowTopReverseDotted ||
		typ == lineSolidArrowBottomReverseDotted ||
		typ == lineStickArrowTopReverseDotted ||
		typ == lineStickArrowBottomReverseDotted {
		line.Style("stroke-dasharray", "3, 3")
		line.Attr("class", "messageLine1")
	} else {
		line.Attr("class", "messageLine0")
	}

	line.Attr("data-et", "message")
	line.Attr("data-id", "i"+msgModel.id)
	line.Attr("data-from", msgModel.from)
	line.Attr("data-to", msgModel.to)

	url := ""
	if conf.arrowMarkerAbsolute {
		url = sanitize.GetUrl(true, pageLocation)
	}

	line.Attr("stroke-width", 2)
	line.Attr("stroke", "none") // handled by theme/css anyway
	line.Style("fill", "none")  // remove any fill colour

	marker := func(name string) string { return "url(" + url + "#" + diagramID + name + ")" }

	if typ == lineSolidTop || typ == lineSolidTopDotted {
		line.Attr("marker-end", marker("-solidTopArrowHead"))
	}
	if typ == lineSolidBottom || typ == lineSolidBottomDotted {
		line.Attr("marker-end", marker("-solidBottomArrowHead"))
	}
	if typ == lineStickTop || typ == lineStickTopDotted {
		line.Attr("marker-end", marker("-stickTopArrowHead"))
	}
	if typ == lineStickBottom || typ == lineStickBottomDotted {
		line.Attr("marker-end", marker("-stickBottomArrowHead"))
	}

	if typ == lineSolidArrowTopReverse || typ == lineSolidArrowTopReverseDotted {
		line.Attr("marker-start", marker("-solidBottomArrowHead"))
	}
	if typ == lineSolidArrowBottomReverse || typ == lineSolidArrowBottomReverseDotted {
		line.Attr("marker-start", marker("-solidTopArrowHead"))
	}
	if typ == lineStickArrowTopReverse || typ == lineStickArrowTopReverseDotted {
		line.Attr("marker-start", marker("-stickBottomArrowHead"))
	}
	if typ == lineStickArrowBottomReverse || typ == lineStickArrowBottomReverseDotted {
		line.Attr("marker-start", marker("-stickTopArrowHead"))
	}

	if typ == lineSolid || typ == lineDotted {
		line.Attr("marker-end", marker("-arrowhead"))
	}
	if typ == lineBidirectionalSolid || typ == lineBidirectionalDotted {
		line.Attr("marker-start", marker("-arrowhead"))
		line.Attr("marker-end", marker("-arrowhead"))
	}
	if typ == lineSolidPoint || typ == lineDottedPoint {
		line.Attr("marker-end", marker("-filled-head"))
	}

	if typ == lineSolidCross || typ == lineDottedCross {
		line.Attr("marker-end", marker("-crosshead"))
	}

	// add node number
	if sequenceVisible || conf.showSequenceNumbers {
		isBidirectional := typ == lineBidirectionalSolid || typ == lineBidirectionalDotted

		isReverseArrowType := typ == lineSolidArrowTopReverse ||
			typ == lineSolidArrowTopReverseDotted ||
			typ == lineSolidArrowBottomReverse ||
			typ == lineSolidArrowBottomReverseDotted ||
			typ == lineStickArrowTopReverse ||
			typ == lineStickArrowTopReverseDotted ||
			typ == lineStickArrowBottomReverse ||
			typ == lineStickArrowBottomReverseDotted

		const sequenceNumberRadius = 6.0
		hasCentralConn := hasCentralConnection(msg)
		lineStartX := startx
		lineStopX := stopx
		reverseOrDual := msg.centralConnection == lineCentralConnectionDual ||
			msg.centralConnection == lineCentralConnectionReverse

		if isBidirectional {
			// For bidirectional arrows, adjust the start position
			if startx < stopx {
				lineStartX = startx + sequenceNumberRadius*2
			} else {
				lineStartX = startx - sequenceNumberRadius
				if hasCentralConn {
					lineStartX += -5
				}
				if reverseOrDual {
					lineStartX += -7.5
				}
			}
			line.Attr("x1", lineStartX)
		} else if isReverseArrowType {
			// For reverse arrows, adjust the stop position (where the arrowhead is)
			if stopx > startx {
				lineStopX = stopx - 2*sequenceNumberRadius
			} else {
				lineStopX = stopx - sequenceNumberRadius
				if reverseOrDual {
					lineStartX += -7.5
				}
			}
			if hasCentralConn {
				lineStopX += 15
			}

			line.Attr("x2", lineStopX)
			line.Attr("x1", lineStartX)
		} else {
			line.Attr("x1", startx+sequenceNumberRadius)
		}

		// Calculate autonumber X position
		var autonumberX float64
		isSelfMessage := startx == stopx
		isLeftToRight := startx <= stopx

		if isSelfMessage {
			autonumberX = msgModel.fromBounds + 1
		} else if isReverseArrowType {
			if isLeftToRight {
				autonumberX = msgModel.toBounds - 1
			} else {
				autonumberX = msgModel.fromBounds + 1
			}
		} else {
			if isLeftToRight {
				autonumberX = msgModel.fromBounds + 1
			} else {
				autonumberX = msgModel.toBounds - 1
			}
		}

		fontSize := "12px"
		sequenceIndexText := js.Str(sequenceIndex)
		sequenceIndexLength := js.UTF16Len(sequenceIndexText)

		if sequenceIndexLength > 5 {
			fontSize = "7px"
		} else if sequenceIndexLength > 3 {
			fontSize = "9px"
		}

		diagram.
			Append("line").
			Attr("x1", autonumberX).
			Attr("y1", lineStartY).
			Attr("x2", autonumberX).
			Attr("y2", lineStartY).
			Attr("stroke-width", 0).
			Attr("marker-start", marker("-sequencenumber"))

		diagram.
			Append("text").
			Attr("x", autonumberX).
			Attr("y", lineStartY+4).
			Attr("font-family", "sans-serif").
			Attr("font-size", fontSize).
			Attr("text-anchor", "middle").
			Attr("class", "sequenceNumber").
			Text(sequenceIndexText)
	}
	return nil
}

func (r *renderer) addActorRenderingData(actors *js.Map[string, *actor], createdActors *js.Map[string, int], actorKeys []string, verticalPos float64, isFooter bool) error {
	conf := r.conf
	bounds := &r.bounds
	prevWidth := 0.0
	prevMargin := 0.0
	var prevBox *box
	maxHeight := 0.0

	for _, actorKey := range actorKeys {
		actor := actors.Get(actorKey)
		if actor == nil {
			return errors.New("TypeError: Cannot read properties of undefined (reading 'box')")
		}
		box := actor.box

		// end of box
		if prevBox != nil && prevBox != box {
			if !isFooter {
				bounds.models.boxes = append(bounds.models.boxes, prevBox)
			}
			prevMargin += conf.boxMargin + prevBox.margin
		}

		// new box
		if box != nil && box != prevBox {
			if !isFooter {
				box.x = prevWidth + prevMargin
				box.y = verticalPos
			}
			prevMargin += box.margin
		}

		// Add some rendering data to the object
		actor.width = sanitize.GetMax(orNum(actor.width, conf.width), conf.width)
		actor.height = sanitize.GetMax(orNum(actor.height, conf.height), conf.height)
		actor.margin = orNum(actor.margin, conf.actorMargin)

		maxHeight = sanitize.GetMax(maxHeight, actor.height)

		// if the actor is created by a message, widen margin
		if createdActors.Get(actor.name) != 0 {
			prevMargin += float64(actor.width / 2)
		}

		actor.x = prevWidth + prevMargin
		actor.starty = bounds.getVerticalPos()

		bounds.insert(actor.x, verticalPos, actor.x+actor.width, actor.height)

		prevWidth += actor.width + prevMargin
		if actor.box != nil {
			actor.box.width = prevWidth + box.margin - actor.box.x
		}
		prevMargin = actor.margin
		prevBox = actor.box
		bounds.models.actors = append(bounds.models.actors, actor)
	}

	// end of box
	if prevBox != nil && !isFooter {
		bounds.models.boxes = append(bounds.models.boxes, prevBox)
	}

	// Add a margin between the actor boxes and the first arrow
	bounds.bumpVerticalPos(maxHeight)
	return nil
}

// orNum is `a || b` for numbers.
func orNum(a, b float64) float64 {
	if a != 0 && !math.IsNaN(a) {
		return a
	}
	return b
}

func (r *renderer) drawActors(diagram *dom.Node, actors *js.Map[string, *actor], actorKeys []string, isFooter bool, diagramID string, actorIndexMap *js.Map[string, int]) error {
	conf := r.conf
	bounds := &r.bounds
	if !isFooter {
		for _, actorKey := range actorKeys {
			actor := actors.Get(actorKey)
			// Draw the box with the attached line
			if _, err := r.drawActor(diagram, actor, false, diagramID, actorIndexMap); err != nil {
				return err
			}
		}
	} else {
		maxHeight := 0.0
		bounds.bumpVerticalPos(float64(conf.boxMargin * 2))
		for _, actorKey := range actorKeys {
			actor := actors.Get(actorKey)
			if actor == nil {
				return errors.New("TypeError: Cannot read properties of undefined (reading 'stopy')")
			}
			if !(actor.stopy != 0 && !math.IsNaN(actor.stopy)) {
				actor.stopy = bounds.getVerticalPos()
			}
			height, err := r.drawActor(diagram, actor, true, diagramID, actorIndexMap)
			if err != nil {
				return err
			}
			maxHeight = sanitize.GetMax(maxHeight, height)
		}
		bounds.bumpVerticalPos(maxHeight + conf.boxMargin)
	}
	return nil
}

func (r *renderer) drawActorsPopup(diagram *dom.Node, actors *js.Map[string, *actor], actorKeys []string) (maxHeight, maxWidth float64, err error) {
	for _, actorKey := range actorKeys {
		actor := actors.Get(actorKey)
		if actor == nil {
			return 0, 0, errors.New("TypeError: Cannot read properties of undefined (reading 'links')")
		}
		minMenuWidth, err := r.getRequiredPopupWidth(actor)
		if err != nil {
			return 0, 0, err
		}
		height, width, err := r.drawPopup(diagram, actor, minMenuWidth)
		if err != nil {
			return 0, 0, err
		}
		if height > maxHeight {
			maxHeight = height
		}
		if width+actor.x > maxWidth {
			maxWidth = width + actor.x
		}
	}
	return maxHeight, maxWidth, nil
}

// activationBounds is activationBounds(actor, actors): it handles multiple
// stacked activations for the same actor.
func (r *renderer) activationBounds(actorKey string, actors *js.Map[string, *actor]) (left, right float64, err error) {
	actorObj := actors.Get(actorKey)
	if actorObj == nil {
		return 0, 0, errors.New("TypeError: Cannot read properties of undefined (reading 'x')")
	}
	activations := r.bounds.actorActivations(actorKey)

	left = actorObj.x + float64(actorObj.width/2) - 1
	for _, activation := range activations {
		left = sanitize.GetMin(left, activation.startx.nan())
	}
	right = actorObj.x + float64(actorObj.width/2) + 1
	for _, activation := range activations {
		right = sanitize.GetMax(right, activation.stopx.nan())
	}
	return left, right, nil
}

// loopWidth is one entry of the result of calculateLoopBounds.
type loopWidth struct {
	id       string
	from, to float64
	width    float64
}

func (r *renderer) adjustLoopHeightForWrap(loopWidths map[string]*loopWidth, msg *message, preMargin, postMargin float64, addLoopFn func(*message) error) error {
	conf := r.conf
	bounds := &r.bounds
	bounds.bumpVerticalPos(preMargin)
	heightAdjust := postMargin
	if msg.id != "" && msg.message != "" && loopWidths[msg.id] != nil {
		loopWidth := loopWidths[msg.id].width
		textConf := conf.messageFont()
		wrapped, err := r.wrapLabel("["+msg.message+"]", loopWidth-float64(2*conf.wrapPadding), textConf)
		if err != nil {
			return err
		}
		msg.message = wrapped
		msg.width, msg.hasWidth = loopWidth, true
		msg.wrap = true

		// const lines = common.splitBreaks(msg.message).length;
		textDims, err := r.calculateTextDimensions(msg.message, textConf)
		if err != nil {
			return err
		}
		totalOffset := sanitize.GetMax(textDims.Height, conf.labelBoxHeight)
		heightAdjust = postMargin + totalOffset
	}
	if err := addLoopFn(msg); err != nil {
		return err
	}
	bounds.bumpVerticalPos(heightAdjust)
	return nil
}

// adjustCreatedDestroyedData adjusts the msgModel and the actor for the
// rendering in case the latter is created or destroyed by the msg.
func (r *renderer) adjustCreatedDestroyedData(msg *message, msgModel *msgModel, lineStartY float64, index int, actors *js.Map[string, *actor], createdActors, destroyedActors *js.Map[string, int]) error {
	conf := r.conf
	bounds := &r.bounds
	errUndefined := errors.New("TypeError: Cannot read properties of undefined (reading 'x')")
	receiverAdjustment := func(actor *actor, adjustment float64) error {
		from := actors.Get(msg.from)
		if from == nil {
			return errUndefined
		}
		if actor.x < from.x {
			bounds.insert(
				msgModel.stopx-adjustment,
				msgModel.starty,
				msgModel.startx,
				msgModel.stopy+float64(actor.height/2)+conf.noteMargin,
			)
			msgModel.stopx = msgModel.stopx + adjustment
		} else {
			bounds.insert(
				msgModel.startx,
				msgModel.starty,
				msgModel.stopx+adjustment,
				msgModel.stopy+float64(actor.height/2)+conf.noteMargin,
			)
			msgModel.stopx = msgModel.stopx - adjustment
		}
		return nil
	}

	senderAdjustment := func(actor *actor, adjustment float64) error {
		to := actors.Get(msg.to)
		if to == nil {
			return errUndefined
		}
		if actor.x < to.x {
			bounds.insert(
				msgModel.startx-adjustment,
				msgModel.starty,
				msgModel.stopx,
				msgModel.stopy+float64(actor.height/2)+conf.noteMargin,
			)
			msgModel.startx = msgModel.startx + adjustment
		} else {
			bounds.insert(
				msgModel.stopx,
				msgModel.starty,
				msgModel.startx+adjustment,
				msgModel.stopy+float64(actor.height/2)+conf.noteMargin,
			)
			msgModel.startx = msgModel.startx - adjustment
		}
		return nil
	}
	isActorType := func(typ string) bool {
		return typ == participantActor || typ == participantControl || typ == participantEntity || typ == participantDatabase
	}
	at := func(m *js.Map[string, int], key string) (int, bool) {
		return m.Lookup(key)
	}

	// if it is a create message
	if v, ok := at(createdActors, msg.to); ok && v == index {
		actor := actors.Get(msg.to)
		if actor == nil {
			return errUndefined
		}
		adjustment := float64(actor.width/2) + 3
		if isActorType(actor.typ) {
			adjustment = actorTypeWidth/2 + 3
		}
		if err := receiverAdjustment(actor, adjustment); err != nil {
			return err
		}
		actor.starty = lineStartY - float64(actor.height/2)
		bounds.bumpVerticalPos(actor.height / 2)
	} else if v, ok := at(destroyedActors, msg.from); ok && v == index {
		// if it is a destroy sender message
		actor := actors.Get(msg.from)
		if actor == nil {
			return errUndefined
		}
		if conf.mirrorActors {
			adjustment := actor.width / 2
			if isActorType(actor.typ) {
				adjustment = actorTypeWidth / 2
			}
			if err := senderAdjustment(actor, adjustment); err != nil {
				return err
			}
		}
		actor.stopy = lineStartY - float64(actor.height/2)
		bounds.bumpVerticalPos(actor.height / 2)
	} else if v, ok := at(destroyedActors, msg.to); ok && v == index {
		// if it is a destroy receiver message
		actor := actors.Get(msg.to)
		if actor == nil {
			return errUndefined
		}
		if conf.mirrorActors {
			adjustment := float64(actor.width/2) + 3
			if isActorType(actor.typ) {
				adjustment = actorTypeWidth/2 + 3
			}
			if err := receiverAdjustment(actor, adjustment); err != nil {
				return err
			}
		}
		actor.stopy = lineStartY - float64(actor.height/2)
		bounds.bumpVerticalPos(actor.height / 2)
	}
	return nil
}

// isLineType reports whether a message type is a line: the list that
// upstream has in draw and in buildMessageModel.
func isLineType(typ int) bool {
	switch typ {
	case lineSolidOpen, lineDottedOpen, lineSolid,
		lineSolidTop, lineSolidBottom, lineStickTop, lineStickBottom,
		lineSolidTopDotted, lineSolidBottomDotted, lineStickTopDotted, lineStickBottomDotted,
		lineSolidArrowTopReverse, lineSolidArrowBottomReverse, lineStickArrowTopReverse, lineStickArrowBottomReverse,
		lineSolidArrowTopReverseDotted, lineSolidArrowBottomReverseDotted, lineStickArrowTopReverseDotted, lineStickArrowBottomReverseDotted,
		lineDotted, lineSolidCross, lineDottedCross, lineSolidPoint, lineDottedPoint,
		lineBidirectionalSolid, lineBidirectionalDotted:
		return true
	}
	return false
}

// draw draws a sequenceDiagram in the svg of the render.
func draw(ctx *render.Context, d *db) error {
	r := &renderer{ctx: ctx, db: d, actorCnt: -1}
	cfg := ctx.ConfigState.GetConfig()
	r.look = cfg.Get("look")
	r.themeVariables = js.AsObj(cfg.Get("themeVariables"))
	r.conf = &conf{obj: js.AsObj(cfg.Get("sequence"))}
	conf := r.conf
	bounds := &r.bounds
	bounds.conf = conf
	id := ctx.ID
	// bounds.init() calls setConf(getConfig()).
	bounds.init()
	if err := setConf(conf, ctx.ConfigState.GetConfig()); err != nil {
		return err
	}

	diagram := ctx.GetDiagramElement()

	// Fetch data from the parsing
	actors := d.actors
	createdActors := d.createdActors
	destroyedActors := d.destroyedActors
	boxes := d.boxes
	actorKeys := d.getActorKeys()
	messages := d.messages
	title := d.GetDiagramTitle()
	hasBoxes := d.hasAtLeastOneBox()
	hasBoxTitles := d.hasAtLeastOneBoxWithTitle()
	for _, key := range actorKeys {
		if a := actors.Get(key); a.descriptionRaw != nil {
			return errors.New("TypeError: text.match is not a function")
		}
	}
	maxMessageWidthPerActor, err := r.getMaxMessageWidthPerActor(actors, messages)
	if err != nil {
		return err
	}
	height, err := r.calculateActorMargins(actors, maxMessageWidthPerActor, boxes)
	if err != nil {
		return err
	}
	conf.height = height
	conf.obj.Set("height", height)

	insertComputerIcon(diagram, id)
	insertDatabaseIcon(diagram, id)
	insertClockIcon(diagram, id)

	if hasBoxes {
		bounds.bumpVerticalPos(conf.boxMargin)
		if hasBoxTitles {
			bounds.bumpVerticalPos(boxes[0].textMaxHeight)
		}
	}

	if conf.hideUnusedParticipants {
		newActors := map[string]bool{}
		for _, message := range messages {
			if message.hasFrom {
				newActors[message.from] = true
			}
			if message.hasTo {
				newActors[message.to] = true
			}
		}
		filtered := actorKeys[:0:0]
		for _, actorKey := range actorKeys {
			if newActors[actorKey] {
				filtered = append(filtered, actorKey)
			}
		}
		actorKeys = filtered
	}

	actorIndexMap := js.NewMap[string, int]()
	for index, actorKey := range actorKeys {
		name := actorKey
		if a := actors.Get(actorKey); a != nil {
			name = a.name
		}
		actorIndexMap.Set(name, index)
	}

	if err := r.addActorRenderingData(actors, createdActors, actorKeys, 0, false); err != nil {
		return err
	}
	loopWidths, err := r.calculateLoopBounds(messages, actors)
	if err != nil {
		return err
	}

	// The arrow head definition is attached to the svg once
	insertArrowHead(diagram, id)
	insertArrowCrossHead(diagram, id)
	insertArrowFilledHead(diagram, id)
	insertSequenceNumber(diagram, id)
	insertSolidTopArrowHead(diagram, id)
	insertSolidBottomArrowHead(diagram, id)
	insertStickTopArrowHead(diagram, id)
	insertStickBottomArrowHead(diagram, id)

	if r.look == "neo" {
		insertDropShadow(diagram, conf, id)
	}

	activeEnd := func(msg *message, verticalPos float64) error {
		activationData := bounds.endActivation(msg)
		if activationData == nil {
			return errors.New("TypeError: Cannot read properties of undefined (reading 'starty')")
		}
		if activationData.starty.nan()+18 > verticalPos {
			activationData.starty.set(verticalPos - 6)
			verticalPos += 12
		}
		r.drawActivation(activationData, verticalPos, len(bounds.actorActivations(msg.from)), actorIndexMap)

		bounds.insert(activationData.startx.nan(), verticalPos-10, activationData.stopx.nan(), verticalPos)
		return nil
	}

	newLoop := func(message *message) error {
		bounds.newLoop(message, nil)
		return nil
	}
	addSection := func(message *message) error {
		return bounds.addSectionToLoop(message)
	}
	// endLoop is the code of the end of a loop, alt, opt, par, critical and
	// break.
	endLoop := func(labelText string, msg *message) error {
		loopModel := bounds.endLoop()
		if loopModel == nil {
			return errors.New("TypeError: Cannot read properties of undefined (reading 'startx')")
		}
		if err := r.drawLoop(diagram, loopModel, labelText, msg); err != nil {
			return err
		}
		bounds.bumpVerticalPos(loopModel.stopy.nan() - bounds.getVerticalPos())
		bounds.models.loops = append(bounds.models.loops, loopModel)
		return nil
	}

	// Draw the messages/signals
	sequenceIndex := 1.0
	sequenceIndexStep := 1.0
	type messageToDraw struct {
		messageModel *msgModel
		lineStartY   float64
		msg          *message
	}
	var messagesToDraw []messageToDraw
	var backgrounds []*loopModel
	index := 0
	for _, msg := range messages {
		var err error
		switch {
		case msg.kind == kindNote:
			bounds.resetVerticalPos()
			if msg.noteModel == nil {
				return errors.New("TypeError: Cannot read properties of undefined (reading 'height')")
			}
			err = r.drawNote(diagram, msg.noteModel, msg.id)
		case msg.kind == kindSequenceIndex:
			if v := msg.auto.start; msg.auto.hasStart && v != 0 && !math.IsNaN(v) {
				sequenceIndex = v
			}
			if v := msg.auto.step; msg.auto.hasStep && v != 0 && !math.IsNaN(v) {
				sequenceIndexStep = v
			}
			if msg.auto.visible {
				d.enableSequenceNumbers()
			} else {
				d.disableSequenceNumbers()
			}
		case msg.typ == lineActiveStart, msg.typ == lineCentralConnection, msg.typ == lineCentralConnectionReverse:
			err = bounds.newActivation(msg, diagram, actors)
		case msg.typ == lineActiveEnd:
			err = activeEnd(msg, bounds.getVerticalPos())
		case msg.typ == lineLoopStart:
			err = r.adjustLoopHeightForWrap(loopWidths, msg, conf.boxMargin, conf.boxMargin+conf.boxTextMargin, newLoop)
		case msg.typ == lineLoopEnd:
			err = endLoop("loop", msg)
		case msg.typ == lineRectStart:
			err = r.adjustLoopHeightForWrap(loopWidths, msg, conf.boxMargin, conf.boxMargin, func(message *message) error {
				var fill any = message.message
				if message.message == "" {
					// Apply a theme-aware default fill.
					// Because the rect is rendered behind actors and text, it does not obscure them.
					// Use theme background when no explicit color is given; the rect renders
					// behind all content so any semi-transparent theme color is safe.
					fill = "rgba(128, 128, 128, 0.5)"
					if r.themeVariables != nil {
						if v := r.themeVariables.Get("rectBkgColor"); js.Truthy(v) {
							fill = v
						} else if v := r.themeVariables.Get("actorBkg"); js.Truthy(v) {
							fill = v
						}
					}
				}
				bounds.newLoop(nil, fill)
				return nil
			})
		case msg.typ == lineRectEnd:
			loopModel := bounds.endLoop()
			if loopModel == nil {
				return errors.New("TypeError: Cannot read properties of undefined (reading 'stopy')")
			}
			backgrounds = append(backgrounds, loopModel)
			bounds.models.loops = append(bounds.models.loops, loopModel)
			bounds.bumpVerticalPos(loopModel.stopy.nan() - bounds.getVerticalPos())
		case msg.typ == lineOptStart:
			err = r.adjustLoopHeightForWrap(loopWidths, msg, conf.boxMargin, conf.boxMargin+conf.boxTextMargin, newLoop)
		case msg.typ == lineOptEnd:
			err = endLoop("opt", msg)
		case msg.typ == lineAltStart:
			err = r.adjustLoopHeightForWrap(loopWidths, msg, conf.boxMargin, conf.boxMargin+conf.boxTextMargin, newLoop)
		case msg.typ == lineAltElse:
			err = r.adjustLoopHeightForWrap(loopWidths, msg, conf.boxMargin+conf.boxTextMargin, conf.boxMargin, addSection)
		case msg.typ == lineAltEnd:
			err = endLoop("alt", msg)
		case msg.typ == lineParStart, msg.typ == lineParOverStart:
			err = r.adjustLoopHeightForWrap(loopWidths, msg, conf.boxMargin, conf.boxMargin+conf.boxTextMargin, newLoop)
			bounds.saveVerticalPos()
		case msg.typ == lineParAnd:
			err = r.adjustLoopHeightForWrap(loopWidths, msg, conf.boxMargin+conf.boxTextMargin, conf.boxMargin, addSection)
		case msg.typ == lineParEnd:
			err = endLoop("par", msg)
		case msg.typ == lineCriticalStart:
			err = r.adjustLoopHeightForWrap(loopWidths, msg, conf.boxMargin, conf.boxMargin+conf.boxTextMargin, newLoop)
		case msg.typ == lineCriticalOption:
			err = r.adjustLoopHeightForWrap(loopWidths, msg, conf.boxMargin+conf.boxTextMargin, conf.boxMargin, addSection)
		case msg.typ == lineCriticalEnd:
			err = endLoop("critical", msg)
		case msg.typ == lineBreakStart:
			err = r.adjustLoopHeightForWrap(loopWidths, msg, conf.boxMargin, conf.boxMargin+conf.boxTextMargin, newLoop)
		case msg.typ == lineBreakEnd:
			err = endLoop("break", msg)
		default:
			// Upstream catches an error here, logs it and goes on.
			func() {
				msgModel := msg.msgModel
				if msgModel == nil || msgModel.empty {
					return
				}
				msgModel.starty = bounds.getVerticalPos()
				msgModel.sequenceIndex = sequenceIndex
				msgModel.sequenceVisible = d.showSequenceNumbers()
				msgModel.id = msg.id
				msgModel.from = msg.from
				msgModel.to = msg.to
				lineStartY, err := r.boundMessage(msgModel)
				if err != nil {
					return
				}
				if err := r.adjustCreatedDestroyedData(msg, msgModel, lineStartY, index, actors, createdActors, destroyedActors); err != nil {
					return
				}
				messagesToDraw = append(messagesToDraw, messageToDraw{messageModel: msgModel, lineStartY: lineStartY, msg: msg})
				bounds.models.messages = append(bounds.models.messages, msgModel)
			}()
		}
		if err != nil {
			return err
		}

		// Increment sequence counter if msg.type is a line (and not another event like activation or note, etc)
		if msg.kind == kindSignal && isLineType(msg.typ) {
			// hitting a floating point number error, so round to 2 decimal places
			sequenceIndex = js.Round(float64((sequenceIndex+sequenceIndexStep)*100)) / 100
		}
		index++
	}

	if err := r.drawActors(diagram, actors, actorKeys, false, id, actorIndexMap); err != nil {
		return err
	}

	for _, e := range messagesToDraw {
		if err := r.drawMessage(diagram, e.messageModel, e.lineStartY, e.msg, id); err != nil {
			return err
		}
	}
	if conf.mirrorActors {
		if err := r.drawActors(diagram, actors, actorKeys, true, id, actorIndexMap); err != nil {
			return err
		}
	}
	for _, e := range backgrounds {
		drawBackgroundRect(diagram, svgdraw.Bound{
			Startx: e.startx.nan(),
			Stopx:  e.stopx.nan(),
			Starty: e.starty.nan(),
			Stopy:  e.stopy.nan(),
			Fill:   e.fill,
		})
	}
	fixLifeLineHeights(diagram, actors, actorKeys, conf)

	for _, box := range bounds.models.boxes {
		box.height = bounds.getVerticalPos() - box.y
		bounds.insert(box.x, box.y, box.x+box.width, box.height)
		boxPadding := float64(conf.boxMargin * 2)
		box.startx = box.x - boxPadding
		box.starty = box.y - float64(boxPadding*0.25)
		box.stopx = box.startx + box.width + float64(2*boxPadding)
		box.stopy = box.starty + box.height + float64(boxPadding*0.75)
		box.stroke = "rgb(0,0,0, 0.5)"
		r.drawBox(diagram, box)
	}

	if hasBoxes {
		bounds.bumpVerticalPos(conf.boxMargin)
	}

	// only draw popups for the top row of actors.
	requiredMaxHeight, requiredMaxWidth, err := r.drawActorsPopup(diagram, actors, actorKeys)
	if err != nil {
		return err
	}

	box := &bounds.data

	if !box.startx.ok {
		box.startx.set(0)
	}
	if !box.starty.ok {
		box.starty.set(0)
	}
	if !box.stopx.ok {
		box.stopx.set(0)
	}
	if !box.stopy.ok {
		box.stopy.set(0)
	}

	// Make sure the height of the diagram supports long menus.
	boxHeight := box.stopy.v - box.starty.v
	if boxHeight < requiredMaxHeight {
		boxHeight = requiredMaxHeight
	}

	svgHeight := boxHeight + float64(2*conf.diagramMarginY)
	if conf.mirrorActors {
		svgHeight = svgHeight - conf.boxMargin + conf.bottomMarginAdj
	}

	// Make sure the width of the diagram supports wide menus.
	boxWidth := box.stopx.v - box.startx.v
	if boxWidth < requiredMaxWidth {
		boxWidth = requiredMaxWidth
	}
	width := boxWidth + float64(2*conf.diagramMarginX)

	if title != "" {
		diagram.
			Append("text").
			Text(title).
			Attr("x", float64((box.stopx.v-box.startx.v)/2)-float64(2*conf.diagramMarginX)).
			Attr("y", -25)
	}

	render.ConfigureSvgSize(diagram, svgHeight, width, conf.useMaxWidth)

	extraVertForTitle := 0.0
	if title != "" {
		extraVertForTitle = 40
	}
	extraHeightForNeoActors := 0.0
	if actors.Len() != 0 && r.look == "neo" {
		extraHeightForNeoActors = 30
	}
	diagram.Attr(
		"viewBox",
		js.Str(box.startx.v-conf.diagramMarginX)+
			" -"+
			js.Str(conf.diagramMarginY+extraVertForTitle)+
			" "+
			js.Str(width)+
			" "+
			js.Str(svgHeight+extraVertForTitle+extraHeightForNeoActors),
	)
	return nil
}

// getMaxMessageWidthPerActor retrieves the max message width of each actor.
// It supports signals (messages, loops) and notes.
//
// It enumerates each given message, and determines its text width, in
// relation to the actor it originates from, and destined to.
func (r *renderer) getMaxMessageWidthPerActor(actors *js.Map[string, *actor], messages []*message) (*js.Obj[float64], error) {
	conf := r.conf
	maxMessageWidthPerActor := js.NewObj[float64]()
	// widthOr is `maxMessageWidthPerActor[key] || 0`.
	widthOr := func(key string) float64 {
		return orNum(maxMessageWidthPerActor.Get(key), 0)
	}

	for _, msg := range messages {
		if !msg.hasTo || !msg.hasFrom || actors.Get(msg.to) == nil || actors.Get(msg.from) == nil {
			continue
		}
		actor := actors.Get(msg.to)

		// If this is the first actor, and the message is left of it, no need to calculate the margin
		if msg.isNote && msg.placement == placementLeftOf && actor.prevActor == "" {
			continue
		}

		// If this is the last actor, and the message is right of it, no need to calculate the margin
		if msg.isNote && msg.placement == placementRightOf && actor.nextActor == "" {
			continue
		}

		isNote := msg.isNote
		isMessage := !isNote

		textFont := conf.messageFont()
		if isNote {
			textFont = conf.noteFont()
		}
		wrappedMessage := msg.message
		if msg.wrap {
			var err error
			wrappedMessage, err = r.wrapLabel(msg.message, conf.width-float64(2*conf.wrapPadding), textFont)
			if err != nil {
				return nil, err
			}
		}
		if sanitize.HasKatex(wrappedMessage) {
			return nil, errKatex
		}
		messageDimensions, err := r.calculateTextDimensions(wrappedMessage, textFont)
		if err != nil {
			return nil, err
		}
		messageWidth := messageDimensions.Width + float64(2*conf.wrapPadding)

		// The following scenarios should be supported:
		//
		// - There's a message (non-note) between fromActor and toActor
		//   - If fromActor is on the right and toActor is on the left, we should
		//     define the toActor's margin
		//   - If fromActor is on the left and toActor is on the right, we should
		//     define the fromActor's margin
		// - There's a note, in which case fromActor == toActor
		//   - If the note is to the left of the actor, we should define the previous actor
		//     margin
		//   - If the note is on the actor, we should define both the previous and next actor
		//     margins, each being the half of the note size
		//   - If the note is on the right of the actor, we should define the current actor
		//     margin
		if isMessage && actor.nextActor != "" && msg.from == actor.nextActor {
			maxMessageWidthPerActor.Set(msg.to, sanitize.GetMax(widthOr(msg.to), messageWidth))
		} else if isMessage && actor.prevActor != "" && msg.from == actor.prevActor {
			maxMessageWidthPerActor.Set(msg.from, sanitize.GetMax(widthOr(msg.from), messageWidth))
		} else if isMessage && msg.from == msg.to {
			maxMessageWidthPerActor.Set(msg.from, sanitize.GetMax(widthOr(msg.from), messageWidth/2))

			maxMessageWidthPerActor.Set(msg.to, sanitize.GetMax(widthOr(msg.to), messageWidth/2))
		} else if isNote && msg.placement == placementRightOf {
			maxMessageWidthPerActor.Set(msg.from, sanitize.GetMax(widthOr(msg.from), messageWidth))
		} else if isNote && msg.placement == placementLeftOf {
			// The key is actor.prevActor. It is not undefined here.
			maxMessageWidthPerActor.Set(actor.prevActor, sanitize.GetMax(widthOr(actor.prevActor), messageWidth))
		} else if isNote && msg.placement == placementOver {
			if actor.prevActor != "" {
				maxMessageWidthPerActor.Set(actor.prevActor, sanitize.GetMax(widthOr(actor.prevActor), messageWidth/2))
			}

			if actor.nextActor != "" {
				maxMessageWidthPerActor.Set(msg.from, sanitize.GetMax(widthOr(msg.from), messageWidth/2))
			}
		}
	}

	return maxMessageWidthPerActor, nil
}

func (r *renderer) getRequiredPopupWidth(actor *actor) (float64, error) {
	conf := r.conf
	requiredPopupWidth := 0.0
	textFont := conf.actorFont()
	for _, key := range actor.links.Keys() {
		labelDimensions, err := r.calculateTextDimensions(key, textFont)
		if err != nil {
			return 0, err
		}
		labelWidth := labelDimensions.Width + float64(2*conf.wrapPadding) + float64(2*conf.boxMargin)
		if requiredPopupWidth < labelWidth {
			requiredPopupWidth = labelWidth
		}
	}

	return requiredPopupWidth, nil
}

// calculateActorMargins calculates the optimal margin for each given actor,
// for a given actor to messageWidth map.
//
// An actor's margin is determined by the width of the actor, the width of
// the largest message that originates from it, and the configured
// conf.actorMargin.
func (r *renderer) calculateActorMargins(actors *js.Map[string, *actor], actorToMessageWidth *js.Obj[float64], boxes []*box) (float64, error) {
	conf := r.conf
	maxHeight := 0.0
	for _, prop := range actors.Keys() {
		actor := actors.Get(prop)
		if actor.wrap {
			wrapped, err := r.wrapLabel(actor.description, conf.width-float64(2*conf.wrapPadding), conf.actorFont())
			if err != nil {
				return 0, err
			}
			actor.description = wrapped
		}
		if sanitize.HasKatex(actor.description) {
			return 0, errKatex
		}
		actDims, err := r.calculateTextDimensions(actor.description, conf.actorFont())
		if err != nil {
			return 0, err
		}

		if actor.wrap {
			actor.width = conf.width
		} else {
			actor.width = sanitize.GetMax(conf.width, actDims.Width+float64(2*conf.wrapPadding))
		}

		if r.look == "neo" {
			// The band model: the row is as tall as its tallest glyph-gap-label-gap stack, and every
			// shape anchors to the row's shared datum rather than measuring itself. The measured text
			// height travels with the actor so the shapes place the label from the same number.
			actor.actorTextHeight, actor.hasActorTextHeight = actDims.Height, true
			actor.height = sanitize.GetMax(actorStackHeight(actDims.Height), conf.height)
		} else if actor.wrap {
			actor.height = sanitize.GetMax(actDims.Height, conf.height)
		} else {
			actor.height = conf.height
		}
		maxHeight = sanitize.GetMax(maxHeight, actor.height)
	}

	for _, actorKey := range actorToMessageWidth.Keys() {
		actor := actors.Get(actorKey)

		if actor == nil {
			continue
		}

		// actors.get(undefined) is undefined.
		nextActor := actors.Get(actor.nextActor)
		if actor.nextActor == "" {
			nextActor = nil
		}

		// No need to space out an actor that doesn't have a next link
		if nextActor == nil {
			messageWidth := actorToMessageWidth.Get(actorKey)
			actorWidth := messageWidth + conf.actorMargin - float64(actor.width/2)
			actor.margin = sanitize.GetMax(actorWidth, conf.actorMargin)
			continue
		}

		messageWidth := actorToMessageWidth.Get(actorKey)
		actorWidth := messageWidth + conf.actorMargin - float64(actor.width/2) - float64(nextActor.width/2)

		actor.margin = sanitize.GetMax(actorWidth, conf.actorMargin)
	}

	maxBoxHeight := 0.0
	for _, box := range boxes {
		textFont := conf.messageFont()
		totalWidth := 0.0
		for _, aKey := range box.actorKeys {
			a := actors.Get(aKey)
			if a == nil {
				return 0, errors.New("TypeError: Cannot read properties of undefined (reading 'width')")
			}
			totalWidth += a.width + orNum(a.margin, 0)
		}

		standardBoxPadding := float64(conf.boxMargin * 8)
		totalWidth += standardBoxPadding

		totalWidth -= float64(2 * conf.boxTextMargin)
		if box.wrap {
			wrapped, err := r.wrapLabel(box.name, totalWidth-float64(2*conf.wrapPadding), textFont)
			if err != nil {
				return 0, err
			}
			box.name = wrapped
		}

		boxMsgDimensions, err := r.calculateTextDimensions(box.name, textFont)
		if err != nil {
			return 0, err
		}
		maxBoxHeight = sanitize.GetMax(boxMsgDimensions.Height, maxBoxHeight)
		minWidth := sanitize.GetMax(totalWidth, boxMsgDimensions.Width+float64(2*conf.wrapPadding))
		box.margin = conf.boxTextMargin
		if totalWidth < minWidth {
			missing := float64((minWidth - totalWidth) / 2)
			box.margin += missing
		}
	}
	for _, box := range boxes {
		box.textMaxHeight = maxBoxHeight
	}

	return sanitize.GetMax(maxHeight, conf.height), nil
}

func (r *renderer) buildNoteModel(msg *message, actors *js.Map[string, *actor]) (*noteModel, error) {
	conf := r.conf
	fromActor := actors.Get(msg.from)
	toActor := actors.Get(msg.to)
	if fromActor == nil || toActor == nil {
		return nil, errors.New("TypeError: Cannot read properties of undefined (reading 'x')")
	}
	startx := fromActor.x
	stopx := toActor.x
	shouldWrap := msg.wrap && msg.message != ""

	if sanitize.HasKatex(msg.message) {
		return nil, errKatex
	}
	text := msg.message
	if shouldWrap {
		var err error
		text, err = r.wrapLabel(msg.message, conf.width, conf.noteFont())
		if err != nil {
			return nil, err
		}
	}
	textDimensions, err := r.calculateTextDimensions(text, conf.noteFont())
	if err != nil {
		return nil, err
	}
	noteModel := &noteModel{
		height:  0,
		startx:  fromActor.x,
		stopx:   0,
		starty:  0,
		stopy:   0,
		message: msg.message,
	}
	if shouldWrap {
		noteModel.width = conf.width
	} else {
		noteModel.width = sanitize.GetMax(conf.width, textDimensions.Width+float64(2*conf.noteMargin))
	}
	if msg.placement == placementRightOf {
		if shouldWrap {
			noteModel.width = sanitize.GetMax(conf.width, textDimensions.Width)
		} else {
			noteModel.width = sanitize.GetMax(
				float64(fromActor.width/2)+float64(toActor.width/2),
				textDimensions.Width+float64(2*conf.noteMargin),
			)
		}
		noteModel.startx = startx + float64((fromActor.width+conf.actorMargin)/2)
	} else if msg.placement == placementLeftOf {
		if shouldWrap {
			noteModel.width = sanitize.GetMax(conf.width, textDimensions.Width+float64(2*conf.noteMargin))
		} else {
			noteModel.width = sanitize.GetMax(
				float64(fromActor.width/2)+float64(toActor.width/2),
				textDimensions.Width+float64(2*conf.noteMargin),
			)
		}
		noteModel.startx = startx - noteModel.width + float64((fromActor.width-conf.actorMargin)/2)
	} else if msg.to == msg.from {
		text := msg.message
		if shouldWrap {
			text, err = r.wrapLabel(msg.message, sanitize.GetMax(conf.width, fromActor.width), conf.noteFont())
			if err != nil {
				return nil, err
			}
		}
		textDimensions, err = r.calculateTextDimensions(text, conf.noteFont())
		if err != nil {
			return nil, err
		}
		if shouldWrap {
			noteModel.width = sanitize.GetMax(conf.width, fromActor.width)
		} else {
			noteModel.width = sanitize.GetMax(fromActor.width, conf.width, textDimensions.Width+float64(2*conf.noteMargin))
		}
		noteModel.startx = startx + float64((fromActor.width-noteModel.width)/2)
	} else {
		noteModel.width = math.Abs(startx+float64(fromActor.width/2)-(stopx+float64(toActor.width/2))) + conf.actorMargin
		if startx < stopx {
			noteModel.startx = startx + float64(fromActor.width/2) - float64(conf.actorMargin/2)
		} else {
			noteModel.startx = stopx + float64(toActor.width/2) - float64(conf.actorMargin/2)
		}
	}
	if shouldWrap {
		noteModel.message, err = r.wrapLabel(msg.message, noteModel.width-float64(2*conf.wrapPadding), conf.noteFont())
		if err != nil {
			return nil, err
		}
	}
	return noteModel, nil
}

// Central connection positioning constants
const (
	centralConnectionBaseOffset          = 4
	centralConnectionBidirectionalOffset = 6
)

// hasCentralConnection checks if a message has central connection.
func hasCentralConnection(msg *message) bool {
	return msg.kind == kindSignal && (msg.centralConnection == lineCentralConnection ||
		msg.centralConnection == lineCentralConnectionReverse ||
		msg.centralConnection == lineCentralConnectionDual)
}

// calculateCentralConnectionOffset calculates the positioning offset for
// central connection arrows.
func calculateCentralConnectionOffset(msg *message, isArrowToRight bool) float64 {
	offset := 0.0

	if msg.centralConnection == lineCentralConnectionReverse ||
		msg.centralConnection == lineCentralConnectionDual {
		offset += centralConnectionBaseOffset
	}

	if (msg.centralConnection == lineCentralConnectionReverse ||
		msg.centralConnection == lineCentralConnectionDual) &&
		(msg.typ == lineBidirectionalSolid || msg.typ == lineBidirectionalDotted) {
		if !isArrowToRight {
			offset += -centralConnectionBidirectionalOffset
		}
	}

	return offset
}

// isReverseArrowType checks if a message is a reverse arrow type.
func isReverseArrowType(msg *message) bool {
	switch msg.typ {
	case lineSolidArrowTopReverse, lineSolidArrowTopReverseDotted,
		lineSolidArrowBottomReverse, lineSolidArrowBottomReverseDotted,
		lineStickArrowTopReverse, lineStickArrowTopReverseDotted,
		lineStickArrowBottomReverse, lineStickArrowBottomReverseDotted:
		return true
	}
	return false
}

// isBidirectionalArrowType checks if a message is a bidirectional arrow type.
func isBidirectionalArrowType(msg *message) bool {
	return msg.typ == lineBidirectionalSolid || msg.typ == lineBidirectionalDotted
}

func (r *renderer) buildMessageModel(msg *message, actors *js.Map[string, *actor]) (*msgModel, error) {
	conf := r.conf
	look := r.look
	if msg.kind != kindSignal || !isLineType(msg.typ) {
		return &msgModel{empty: true}, nil
	}
	fromLeft, fromRight, err := r.activationBounds(msg.from, actors)
	if err != nil {
		return nil, err
	}
	toLeft, toRight, err := r.activationBounds(msg.to, actors)
	if err != nil {
		return nil, err
	}
	isArrowToRight := fromLeft <= toLeft
	startx := fromLeft
	if isArrowToRight {
		startx = fromRight
	}
	stopx := toRight
	if isArrowToRight {
		stopx = toLeft
	}

	if look == "neo" {
		const offset = 3
		if msg.typ != lineSolidOpen {
			if isArrowToRight {
				stopx += -offset
			} else {
				stopx += offset
			}
		}
		if msg.typ == lineBidirectionalSolid || msg.typ == lineBidirectionalDotted {
			if isArrowToRight {
				startx += offset
			} else {
				startx += -offset
			}
		}
	}

	// Apply central connection positioning adjustments
	startx += calculateCentralConnectionOffset(msg, isArrowToRight)
	// As the line width is considered, the left and right values will be off by 2.
	isArrowToActivation := math.Abs(toLeft-toRight) > 2

	// adjustValue adjusts the value based on the arrow direction. It returns
	// the adjustment with correct sign to be added to the actual value.
	adjustValue := func(value float64) float64 {
		if isArrowToRight {
			return -value
		}
		return value
	}

	if msg.from == msg.to {
		// This is a self reference, so we need to make sure the arrow is drawn correctly
		// There are many checks in the downstream rendering that checks for equality.
		// The lines on loops will be off by few pixels, but that's fine for now.
		stopx = startx
	} else {
		// This is an edge case for the first activation.
		// Proper fix would require significant changes.
		// So, we set an activate flag in the message, and cross check that with isToActivation
		// In cases where the message is to an activation that was properly detected, we don't want to move the arrow head
		// The activation will not be detected on the first message, so we need to move the arrow head
		if msg.activate && !isArrowToActivation {
			stopx += adjustValue(float64(conf.activationWidth/2) - 1)
		}

		// Shorten the length of arrow at the end and move the marker forward (using refX) to have a clean arrowhead
		// This is not required for open arrows that don't have arrowheads
		switch msg.typ {
		case lineSolidOpen, lineDottedOpen,
			lineStickTop, lineStickBottom,
			lineStickTopDotted, lineStickBottomDotted,
			lineSolidArrowTopReverseDotted, lineSolidArrowBottomReverseDotted,
			lineStickArrowTopReverse, lineStickArrowBottomReverse,
			lineStickArrowTopReverseDotted, lineStickArrowBottomReverseDotted,
			lineSolidArrowTopReverse, lineSolidArrowBottomReverse:
		default:
			stopx += adjustValue(3)
		}

		// Shorten start position of bidirectional arrow to accommodate for second arrowhead
		switch msg.typ {
		case lineBidirectionalSolid, lineBidirectionalDotted,
			lineSolidArrowTopReverseDotted, lineSolidArrowBottomReverseDotted,
			lineSolidArrowTopReverse, lineSolidArrowBottomReverse:
			startx -= adjustValue(3)
		}
	}

	boundedWidth := math.Abs(startx - stopx)
	if msg.wrap && msg.message != "" {
		wrapped, err := r.wrapLabel(
			msg.message,
			sanitize.GetMax(boundedWidth+float64(2*conf.wrapPadding), conf.width),
			conf.messageFont(),
		)
		if err != nil {
			return nil, err
		}
		msg.message = wrapped
	}
	msgDims, err := r.calculateTextDimensions(msg.message, conf.messageFont())
	if err != nil {
		return nil, err
	}

	textWidth := msgDims.Width + float64(2*conf.wrapPadding)
	if msg.wrap {
		textWidth = 0
	}
	return &msgModel{
		width: sanitize.GetMax(
			textWidth,
			boundedWidth+float64(2*conf.wrapPadding),
			conf.width,
		),
		height:     0,
		startx:     startx,
		stopx:      stopx,
		starty:     0,
		stopy:      0,
		message:    msg.message,
		typ:        msg.typ,
		wrap:       msg.wrap,
		fromBounds: js.Min(fromLeft, fromRight, toLeft, toRight),
		toBounds:   js.Max(fromLeft, fromRight, toLeft, toRight),
	}, nil
}

func (r *renderer) calculateLoopBounds(messages []*message, actors *js.Map[string, *actor]) (map[string]*loopWidth, error) {
	conf := r.conf
	bounds := &r.bounds
	loops := map[string]*loopWidth{}
	var stack []*loopWidth
	errUndefined := errors.New("TypeError: Cannot read properties of undefined (reading 'id')")
	pop := func() *loopWidth {
		n := len(stack)
		if n == 0 {
			return nil
		}
		current := stack[n-1]
		stack = stack[:n-1]
		return current
	}

	for _, msg := range messages {
		if msg.kind == kindSignal {
			switch msg.typ {
			case lineLoopStart, lineAltStart, lineOptStart, lineParStart, lineParOverStart, lineCriticalStart, lineBreakStart:
				stack = append(stack, &loopWidth{
					id:    msg.id,
					from:  9007199254740991,  // Number.MAX_SAFE_INTEGER
					to:    -9007199254740991, // Number.MIN_SAFE_INTEGER
					width: 0,
				})
			case lineAltElse, lineParAnd, lineCriticalOption:
				if msg.message != "" {
					current := pop()
					if current == nil {
						return nil, errUndefined
					}
					loops[current.id] = current
					loops[msg.id] = current
					stack = append(stack, current)
				}
			case lineLoopEnd, lineAltEnd, lineOptEnd, lineParEnd, lineCriticalEnd, lineBreakEnd:
				current := pop()
				if current == nil {
					return nil, errUndefined
				}
				loops[current.id] = current
			case lineActiveStart:
				actorRect := actors.Get(msg.from)
				if !msg.hasFrom || msg.from == "" || actorRect == nil {
					return nil, errors.New("TypeError: Cannot read properties of undefined (reading 'x')")
				}
				stackedSize := float64(len(bounds.actorActivations(msg.from)))
				x := actorRect.x + float64(actorRect.width/2) + float64(((stackedSize-1)*conf.activationWidth)/2)
				toAdd := &activation{actor: msg.from}
				toAdd.startx.set(x)
				toAdd.stopx.set(x + conf.activationWidth)
				bounds.activations = append(bounds.activations, toAdd)
			case lineActiveEnd:
				bounds.spliceActivation(bounds.lastActivationIndex(msg.from))
			}
		}
		if msg.isNote {
			noteModel, err := r.buildNoteModel(msg, actors)
			if err != nil {
				return nil, err
			}
			msg.noteModel = noteModel
			for _, current := range stack {
				current.from = sanitize.GetMin(current.from, noteModel.startx)
				current.to = sanitize.GetMax(current.to, noteModel.startx+noteModel.width)
				current.width = sanitize.GetMax(current.width, math.Abs(current.from-current.to)) - conf.labelBoxWidth
			}
		} else {
			msgModel, err := r.buildMessageModel(msg, actors)
			if err != nil {
				return nil, err
			}
			msg.msgModel = msgModel
			if !msgModel.empty && truthyNum(msgModel.startx) && truthyNum(msgModel.stopx) && len(stack) > 0 {
				for _, current := range stack {
					if msgModel.startx == msgModel.stopx {
						from := actors.Get(msg.from)
						to := actors.Get(msg.to)
						current.from = sanitize.GetMin(
							from.x-float64(msgModel.width/2),
							from.x-float64(from.width/2),
							current.from,
						)
						current.to = sanitize.GetMax(
							to.x+float64(msgModel.width/2),
							to.x+float64(from.width/2),
							current.to,
						)
						current.width = sanitize.GetMax(current.width, math.Abs(current.to-current.from)) -
							conf.labelBoxWidth
					} else {
						current.from = sanitize.GetMin(msgModel.startx, current.from)
						current.to = sanitize.GetMax(msgModel.stopx, current.to)
						current.width = sanitize.GetMax(current.width, msgModel.width) - conf.labelBoxWidth
					}
				}
			}
		}
	}
	bounds.activations = nil
	return loops, nil
}

// truthyNum is the JavaScript conversion of a number to a boolean.
func truthyNum(x float64) bool {
	return x != 0 && !math.IsNaN(x)
}
