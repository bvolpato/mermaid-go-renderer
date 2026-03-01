package mermaid

import (
	"math"
	"strings"
)

const (
	sequenceActivationWidth = 10.0
	sequenceActorMargin     = 50.0
	sequenceActorWidth      = 150.0
	sequenceActorHeight     = 65.0
	sequenceBoxMargin       = 10.0
	sequenceBoxTextMargin   = 5.0
	sequenceBottomMarginAdj = 1.0
	sequenceDiagramMarginX  = 50.0
	sequenceDiagramMarginY  = 10.0
	sequenceLabelBoxHeight  = 20.0
	sequenceMessageFontSize = 16.0
	sequenceWrapPadding     = 10.0
)

type sequenceMessageLayout struct {
	Message SequenceMessage
	StartX  float64
	StopX   float64
	LineY   float64
	TextY   float64
	Self    bool
	Dashed  bool
	Note    bool
}

type sequenceLoopSectionLayout struct {
	Y     float64
	Label string
}

type sequenceLoopLayout struct {
	Kind     string
	StartX   float64
	StopX    float64
	StartY   float64
	StopY    float64
	Title    string
	Sections []sequenceLoopSectionLayout
}

type sequenceActivationLayout struct {
	X          float64
	Y          float64
	W          float64
	H          float64
	ClassIndex int
}

type sequenceRenderPlan struct {
	ParticipantLeft   map[string]float64
	ParticipantCenter map[string]float64
	ParticipantWidth  map[string]float64

	MessageLayouts    []sequenceMessageLayout
	LoopLayouts       []sequenceLoopLayout
	ActivationLayouts []sequenceActivationLayout

	LifelineEndY float64
	BottomY      float64

	Width         float64
	Height        float64
	ViewBoxX      float64
	ViewBoxY      float64
	ViewBoxWidth  float64
	ViewBoxHeight float64
}

type sequenceOpenActivation struct {
	StartX float64
	StartY float64
}

type sequenceOpenLoop struct {
	Layout sequenceLoopLayout
}

func buildSequencePlan(
	participants []string,
	participantLabels map[string]string,
	messages []SequenceMessage,
	events []SequenceEvent,
	theme Theme,
) sequenceRenderPlan {
	if len(participants) == 0 {
		seen := map[string]bool{}
		for _, msg := range messages {
			if msg.From != "" && !seen[msg.From] {
				seen[msg.From] = true
				participants = append(participants, msg.From)
			}
			if msg.To != "" && !seen[msg.To] {
				seen[msg.To] = true
				participants = append(participants, msg.To)
			}
		}
	}
	if len(events) == 0 {
		events = defaultSequenceEvents(messages)
	}
	if participantLabels == nil {
		participantLabels = map[string]string{}
	}

	indexOf := map[string]int{}
	for i, id := range participants {
		indexOf[id] = i
	}

	actorWidths := make([]float64, len(participants))
	for i, id := range participants {
		label := strings.TrimSpace(participantLabels[id])
		if label == "" {
			label = id
		}
		actorWidths[i] = max(
			sequenceActorWidth,
			math.Ceil(measureTextWidthWithFontSize(label, sequenceMessageFontSize, true, theme.FontFamily)+2*sequenceWrapPadding),
		)
	}

	margins := make([]float64, len(participants))
	for i := range margins {
		margins[i] = sequenceActorMargin
	}

	for _, msg := range messages {
		fromIdx, okFrom := indexOf[msg.From]
		toIdx, okTo := indexOf[msg.To]
		if !okFrom || !okTo {
			continue
		}

		labelWidth := measureTextWidthWithFontSize(msg.Label, sequenceMessageFontSize, true, theme.FontFamily)
		// Mermaid's browser text metrics are narrower than our server-side defaults.
		messageWidth := (labelWidth + 2*sequenceWrapPadding) * 0.875
		if messageWidth < sequenceActorWidth {
			messageWidth = sequenceActorWidth
		}

		if fromIdx+1 == toIdx {
			required := messageWidth + sequenceActorMargin - actorWidths[fromIdx]/2 - actorWidths[toIdx]/2
			margins[fromIdx] = max(margins[fromIdx], required)
		} else if toIdx+1 == fromIdx {
			required := messageWidth + sequenceActorMargin - actorWidths[toIdx]/2 - actorWidths[fromIdx]/2
			margins[toIdx] = max(margins[toIdx], required)
		} else if fromIdx == toIdx {
			required := messageWidth/2 + sequenceActorMargin - actorWidths[fromIdx]/2
			margins[fromIdx] = max(margins[fromIdx], required)
		}
	}
	for i := range margins {
		margins[i] = max(sequenceActorMargin, math.Ceil(margins[i]))
	}

	participantLeft := map[string]float64{}
	participantCenter := map[string]float64{}
	participantWidth := map[string]float64{}
	prevWidth := 0.0
	prevMargin := 0.0
	for i, id := range participants {
		x := prevWidth + prevMargin
		participantLeft[id] = x
		participantCenter[id] = x + actorWidths[i]/2
		participantWidth[id] = actorWidths[i]
		prevWidth += actorWidths[i] + prevMargin
		prevMargin = margins[i]
	}

	activationBounds := func(actor string, open map[string][]sequenceOpenActivation) (float64, float64, bool) {
		center, ok := participantCenter[actor]
		if !ok {
			return 0, 0, false
		}
		stack := open[actor]
		if len(stack) == 0 {
			return center - 1, center + 1, true
		}
		left := math.MaxFloat64
		right := -math.MaxFloat64
		for _, act := range stack {
			left = min(left, act.StartX)
			right = max(right, act.StartX+sequenceActivationWidth)
		}
		return left, right, true
	}

	updateOpenLoopBounds := func(openLoops []*sequenceOpenLoop, startx, starty, stopx, stopy float64) {
		for idx, openLoop := range openLoops {
			n := float64(len(openLoops) - idx)
			openLoop.Layout.StartY = min(openLoop.Layout.StartY, starty-n*sequenceBoxMargin)
			openLoop.Layout.StopY = max(openLoop.Layout.StopY, stopy+n*sequenceBoxMargin)
			openLoop.Layout.StartX = min(openLoop.Layout.StartX, min(startx, stopx)-n*sequenceBoxMargin)
			openLoop.Layout.StopX = max(openLoop.Layout.StopX, max(startx, stopx)+n*sequenceBoxMargin)
		}
	}

	verticalPos := sequenceActorHeight
	openActivations := map[string][]sequenceOpenActivation{}
	openLoops := make([]*sequenceOpenLoop, 0, 4)
	messageLayouts := make([]sequenceMessageLayout, 0, len(messages))
	loopLayouts := make([]sequenceLoopLayout, 0, 4)
	activationLayouts := make([]sequenceActivationLayout, 0, 4)

	for _, event := range events {
		switch event.Kind {
		case SequenceEventMessage:
			if event.MessageIndex < 0 || event.MessageIndex >= len(messages) {
				continue
			}
			msg := messages[event.MessageIndex]
			if msg.IsNote {
				center, ok := participantCenter[msg.From]
				if !ok {
					continue
				}
				noteWidth := max(
					sequenceActorWidth,
					math.Ceil(measureTextWidthWithFontSize(msg.Label, sequenceMessageFontSize, true, theme.FontFamily)+2*sequenceWrapPadding),
				)
				noteHeight := 39.0
				verticalPos += sequenceBoxMargin
				noteTop := verticalPos
				noteBottom := noteTop + noteHeight
				updateOpenLoopBounds(openLoops, center-noteWidth/2, noteTop, center+noteWidth/2, noteBottom)
				messageLayouts = append(messageLayouts, sequenceMessageLayout{
					Message: msg,
					StartX:  center - noteWidth/2,
					StopX:   center + noteWidth/2,
					LineY:   noteTop,
					TextY:   noteTop + 5,
					Note:    true,
				})
				verticalPos = noteBottom
				continue
			}

			fromLeft, fromRight, okFrom := activationBounds(msg.From, openActivations)
			toLeft, toRight, okTo := activationBounds(msg.To, openActivations)
			if !okFrom || !okTo {
				continue
			}

			isArrowToRight := fromLeft <= toLeft
			startX := fromLeft
			stopX := toRight
			if isArrowToRight {
				startX = fromRight
				stopX = toLeft
			}

			activate := sequenceArrowActivationStart(msg.Arrow)
			if msg.From == msg.To {
				stopX = startX
			} else {
				isArrowToActivation := math.Abs(toLeft-toRight) > 2
				if activate && !isArrowToActivation {
					if isArrowToRight {
						stopX -= sequenceActivationWidth/2 - 1
					} else {
						stopX += sequenceActivationWidth/2 - 1
					}
				}
				if !sequenceOpenArrow(msg.Arrow) {
					if isArrowToRight {
						stopX -= 3
					} else {
						stopX += 3
					}
				}
			}

			textHeight := sequenceMessageTextHeight(msg.Label)
			verticalPos += sequenceBoxMargin
			verticalPos += textHeight
			totalOffset := textHeight - sequenceBoxMargin
			lineY := 0.0

			if msg.From == msg.To {
				totalOffset += sequenceBoxMargin
				lineY = verticalPos + totalOffset
				totalOffset += 30

				textWidth := measureTextWidthWithFontSize(msg.Label, sequenceMessageFontSize, true, theme.FontFamily)
				dx := max(textWidth/2, sequenceActorWidth/2)
				insertStartY := verticalPos - sequenceBoxMargin + totalOffset
				insertStopY := verticalPos + 30 + totalOffset
				updateOpenLoopBounds(openLoops, startX-dx, insertStartY, startX+dx, insertStopY)
				verticalPos += totalOffset
			} else {
				totalOffset += sequenceBoxMargin
				lineY = verticalPos + totalOffset
				updateOpenLoopBounds(openLoops, startX, lineY-sequenceBoxMargin, stopX, lineY)
				verticalPos += totalOffset
			}

			messageLayouts = append(messageLayouts, sequenceMessageLayout{
				Message: msg,
				StartX:  startX,
				StopX:   stopX,
				LineY:   lineY,
				TextY:   lineY - 33,
				Self:    msg.From == msg.To,
				Dashed:  strings.Contains(sequenceArrowBase(msg.Arrow), "--") || msg.IsReturn,
			})

		case SequenceEventAltStart:
			verticalPos += sequenceBoxMargin
			openLoops = append(openLoops, &sequenceOpenLoop{
				Layout: sequenceLoopLayout{
					Kind:   "alt",
					StartX: math.MaxFloat64,
					StopX:  -math.MaxFloat64,
					StartY: verticalPos,
					StopY:  verticalPos,
					Title:  strings.TrimSpace(event.Label),
				},
			})
			post := sequenceBoxMargin + sequenceBoxTextMargin
			if strings.TrimSpace(event.Label) != "" {
				post += sequenceLabelBoxHeight
			}
			verticalPos += post

		case SequenceEventParStart:
			verticalPos += sequenceBoxMargin
			openLoops = append(openLoops, &sequenceOpenLoop{
				Layout: sequenceLoopLayout{
					Kind:   "par",
					StartX: math.MaxFloat64,
					StopX:  -math.MaxFloat64,
					StartY: verticalPos,
					StopY:  verticalPos,
					Title:  strings.TrimSpace(event.Label),
				},
			})
			post := sequenceBoxMargin + sequenceBoxTextMargin
			if strings.TrimSpace(event.Label) != "" {
				post += sequenceLabelBoxHeight
			}
			verticalPos += post

		case SequenceEventAltElse, SequenceEventParAnd:
			if len(openLoops) == 0 {
				continue
			}
			verticalPos += sequenceBoxMargin + sequenceBoxTextMargin
			top := openLoops[len(openLoops)-1]
			top.Layout.Sections = append(top.Layout.Sections, sequenceLoopSectionLayout{
				Y:     verticalPos,
				Label: strings.TrimSpace(event.Label),
			})
			post := sequenceBoxMargin
			if strings.TrimSpace(event.Label) != "" {
				post += sequenceLabelBoxHeight
			}
			verticalPos += post

		case SequenceEventAltEnd, SequenceEventParEnd:
			if len(openLoops) == 0 {
				continue
			}
			last := openLoops[len(openLoops)-1]
			openLoops = openLoops[:len(openLoops)-1]
			if last.Layout.StartX == math.MaxFloat64 {
				if len(participants) > 0 {
					first := participants[0]
					lastID := participants[len(participants)-1]
					last.Layout.StartX = participantLeft[first]
					last.Layout.StopX = participantLeft[lastID] + participantWidth[lastID]
				} else {
					last.Layout.StartX = 0
					last.Layout.StopX = sequenceActorWidth
				}
			}
			last.Layout.StopY = max(last.Layout.StopY, verticalPos)
			loopLayouts = append(loopLayouts, last.Layout)
			verticalPos = max(verticalPos, last.Layout.StopY)

		case SequenceEventActivateStart:
			actor := strings.TrimSpace(event.Actor)
			center, ok := participantCenter[actor]
			if !ok {
				continue
			}
			stackedSize := len(openActivations[actor])
			startX := center + (float64(stackedSize-1)*sequenceActivationWidth)/2
			openActivations[actor] = append(openActivations[actor], sequenceOpenActivation{
				StartX: startX,
				StartY: verticalPos + 2,
			})

		case SequenceEventActivateEnd:
			actor := strings.TrimSpace(event.Actor)
			stack := openActivations[actor]
			if len(stack) == 0 {
				continue
			}
			activation := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			openActivations[actor] = stack
			if activation.StartY+18 > verticalPos {
				activation.StartY = verticalPos - 6
				verticalPos += 12
			}
			activationLayouts = append(activationLayouts, sequenceActivationLayout{
				X:          activation.StartX,
				Y:          activation.StartY,
				W:          sequenceActivationWidth,
				H:          max(0, verticalPos-activation.StartY),
				ClassIndex: len(stack) % 3,
			})
			updateOpenLoopBounds(openLoops, activation.StartX, verticalPos-sequenceBoxMargin, activation.StartX+sequenceActivationWidth, verticalPos)
		}
	}

	for len(openLoops) > 0 {
		last := openLoops[len(openLoops)-1]
		openLoops = openLoops[:len(openLoops)-1]
		if last.Layout.StartX == math.MaxFloat64 {
			if len(participants) > 0 {
				first := participants[0]
				lastID := participants[len(participants)-1]
				last.Layout.StartX = participantLeft[first]
				last.Layout.StopX = participantLeft[lastID] + participantWidth[lastID]
			} else {
				last.Layout.StartX = 0
				last.Layout.StopX = sequenceActorWidth
			}
		}
		last.Layout.StopY = max(last.Layout.StopY, verticalPos)
		loopLayouts = append(loopLayouts, last.Layout)
		verticalPos = max(verticalPos, last.Layout.StopY)
	}

	lifelineEndY := verticalPos + 2*sequenceBoxMargin
	bottomY := lifelineEndY
	finalVertical := bottomY + sequenceActorHeight + sequenceBoxMargin

	boxStartX := 0.0
	boxStopX := 0.0
	if len(participants) > 0 {
		first := participants[0]
		lastID := participants[len(participants)-1]
		boxStartX = participantLeft[first]
		boxStopX = participantLeft[lastID] + participantWidth[lastID]
	}
	for _, loop := range loopLayouts {
		boxStartX = min(boxStartX, loop.StartX)
		boxStopX = max(boxStopX, loop.StopX)
	}

	viewBoxWidth := (boxStopX - boxStartX) + 2*sequenceDiagramMarginX
	viewBoxHeight := finalVertical + 2*sequenceDiagramMarginY - sequenceBoxMargin + sequenceBottomMarginAdj

	return sequenceRenderPlan{
		ParticipantLeft:   participantLeft,
		ParticipantCenter: participantCenter,
		ParticipantWidth:  participantWidth,
		MessageLayouts:    messageLayouts,
		LoopLayouts:       loopLayouts,
		ActivationLayouts: activationLayouts,
		LifelineEndY:      lifelineEndY,
		BottomY:           bottomY,
		Width:             viewBoxWidth,
		Height:            viewBoxHeight,
		ViewBoxX:          boxStartX - sequenceDiagramMarginX,
		ViewBoxY:          -sequenceDiagramMarginY,
		ViewBoxWidth:      viewBoxWidth,
		ViewBoxHeight:     viewBoxHeight,
	}
}

func sequenceMessageTextHeight(label string) float64 {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return 19
	}
	normalized := strings.ReplaceAll(trimmed, "<br/>", "\n")
	normalized = strings.ReplaceAll(normalized, "<br>", "\n")
	lines := strings.Split(normalized, "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		count++
	}
	if count == 0 {
		count = 1
	}
	return float64(count) * 19
}

func defaultSequenceEvents(messages []SequenceMessage) []SequenceEvent {
	events := make([]SequenceEvent, 0, len(messages)*2)
	for idx, msg := range messages {
		events = append(events, SequenceEvent{
			Kind:         SequenceEventMessage,
			MessageIndex: idx,
		})
		if sequenceArrowActivationStart(msg.Arrow) {
			events = append(events, SequenceEvent{
				Kind:  SequenceEventActivateStart,
				Actor: msg.To,
			})
		}
		if sequenceArrowActivationEnd(msg.Arrow) {
			events = append(events, SequenceEvent{
				Kind:  SequenceEventActivateEnd,
				Actor: msg.From,
			})
		}
	}
	return events
}

func sequenceArrowBase(arrow string) string {
	trimmed := strings.TrimSpace(arrow)
	if strings.HasSuffix(trimmed, "+") || strings.HasSuffix(trimmed, "-") {
		return strings.TrimSpace(trimmed[:len(trimmed)-1])
	}
	return trimmed
}

func sequenceArrowActivationStart(arrow string) bool {
	return strings.HasSuffix(strings.TrimSpace(arrow), "+")
}

func sequenceArrowActivationEnd(arrow string) bool {
	return strings.HasSuffix(strings.TrimSpace(arrow), "-")
}

func sequenceOpenArrow(arrow string) bool {
	base := sequenceArrowBase(arrow)
	return strings.Contains(base, "->") && !strings.Contains(base, ">>")
}
