// Ported from mermaid 12.2.0, diagrams/sequence/sequenceRenderer.ts: the
// `bounds` object (MIT).

package sequence

import (
	"errors"
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

// optNum is a number property that can be undefined.
type optNum struct {
	v  float64
	ok bool
}

// set assigns a value.
func (o *optNum) set(v float64) { o.v, o.ok = v, true }

// nan is the value in arithmetic: undefined is NaN.
func (o optNum) nan() float64 {
	if !o.ok {
		return math.NaN()
	}
	return o.v
}

// updateVal is bounds.updateVal(obj, key, val, fun).
func updateVal(o *optNum, val float64, fun func(...float64) float64) {
	if !o.ok {
		o.set(val)
	} else {
		o.set(fun(val, o.v))
	}
}

// boundsItem holds the four properties that updateBounds writes on a loop
// and on an activation.
type boundsItem struct {
	startx, starty, stopx, stopy optNum
}

// loopSection is one section of a loop: { y, height }.
type loopSection struct {
	y, height float64
}

// loopModel is the object that createLoop returns.
type loopModel struct {
	boundsItem
	// title is undefined when hasTitle is false.
	title    string
	hasTitle bool
	wrap     bool
	width    float64
	hasWidth bool
	height   float64
	// fill is a string, or nil for undefined.
	fill          any
	sections      []*loopSection
	sectionTitles []*message
	// overlap is never set upstream. isLoopOverlap reads it.
	overlap bool
}

// activation is one entry of bounds.activations.
type activation struct {
	boundsItem
	actor    string
	anchored *dom.Node
}

// noteModel is the NoteModel interface.
type noteModel struct {
	width, height                float64
	startx, stopx, starty, stopy float64
	message                      string
}

// msgModel is the object that buildMessageModel returns. empty is true for
// the object {} of a message that is not a line.
type msgModel struct {
	empty                        bool
	width, height                float64
	startx, stopx, starty, stopy float64
	message                      string
	typ                          int
	wrap                         bool
	fromBounds, toBounds         float64
	sequenceIndex                float64
	sequenceVisible              bool
	id, from, to                 string
}

// models is bounds.models.
type models struct {
	actors   []*actor
	boxes    []*box
	loops    []*loopModel
	messages []*msgModel
	notes    []*noteModel
}

// boundsState is the `bounds` object.
type boundsState struct {
	conf *conf

	data          boundsItem
	verticalPos   float64
	sequenceItems []*loopModel
	activations   []*activation
	models        models
	// savedVerticalPos is undefined until saveVerticalPos sets it.
	savedVerticalPos optNum
}

// init is bounds.init(), without the setConf call.
func (b *boundsState) init() {
	b.sequenceItems = nil
	b.activations = nil
	b.models = models{}
	b.data = boundsItem{}
	b.verticalPos = 0
}

// updateBounds is bounds.updateBounds(startx, starty, stopx, stopy).
func (b *boundsState) updateBounds(startx, starty, stopx, stopy float64) {
	cnt := 0
	boxMargin := b.conf.boxMargin
	updateItemBounds := func(item *boundsItem, isActivation bool) {
		cnt++
		// The loop sequenceItems is a stack so the biggest margins in the beginning of the sequenceItems
		n := float64(len(b.sequenceItems) - cnt + 1)

		updateVal(&item.starty, starty-float64(n*boxMargin), js.Min)
		updateVal(&item.stopy, stopy+float64(n*boxMargin), js.Max)

		updateVal(&b.data.startx, startx-float64(n*boxMargin), js.Min)
		updateVal(&b.data.stopx, stopx+float64(n*boxMargin), js.Max)

		if !isActivation {
			updateVal(&item.startx, startx-float64(n*boxMargin), js.Min)
			updateVal(&item.stopx, stopx+float64(n*boxMargin), js.Max)

			updateVal(&b.data.starty, starty-float64(n*boxMargin), js.Min)
			updateVal(&b.data.stopy, stopy+float64(n*boxMargin), js.Max)
		}
	}
	for _, item := range b.sequenceItems {
		updateItemBounds(&item.boundsItem, false)
	}
	for _, item := range b.activations {
		updateItemBounds(&item.boundsItem, true)
	}
}

// insert is bounds.insert(startx, starty, stopx, stopy).
func (b *boundsState) insert(startx, starty, stopx, stopy float64) {
	_startx := sanitize.GetMin(startx, stopx)
	_stopx := sanitize.GetMax(startx, stopx)
	_starty := sanitize.GetMin(starty, stopy)
	_stopy := sanitize.GetMax(starty, stopy)

	updateVal(&b.data.startx, _startx, js.Min)
	updateVal(&b.data.starty, _starty, js.Min)
	updateVal(&b.data.stopx, _stopx, js.Max)
	updateVal(&b.data.stopy, _stopy, js.Max)

	b.updateBounds(_startx, _starty, _stopx, _stopy)
}

// actorActivations is actorActivations(actor).
func (b *boundsState) actorActivations(actor string) []*activation {
	var out []*activation
	for _, a := range b.activations {
		if a.actor == actor {
			out = append(out, a)
		}
	}
	return out
}

// newActivation is bounds.newActivation(message, diagram, actors).
func (b *boundsState) newActivation(msg *message, diagram *dom.Node, actors *js.Map[string, *actor]) error {
	actorRect := actors.Get(msg.from)
	if actorRect == nil {
		return errors.New("TypeError: Cannot read properties of undefined (reading 'x')")
	}
	stackedSize := float64(len(b.actorActivations(msg.from)))
	x := actorRect.x + float64(actorRect.width/2) + float64(((stackedSize-1)*b.conf.activationWidth)/2)
	a := &activation{actor: msg.from, anchored: anchorElement(diagram)}
	a.startx.set(x)
	a.starty.set(b.verticalPos + 2)
	a.stopx.set(x + b.conf.activationWidth)
	b.activations = append(b.activations, a)
	return nil
}

// lastActivationIndex is activations.map((a) => a.actor).lastIndexOf(actor).
func (b *boundsState) lastActivationIndex(actor string) int {
	for i := len(b.activations) - 1; i >= 0; i-- {
		if b.activations[i].actor == actor {
			return i
		}
	}
	return -1
}

// spliceActivation is activations.splice(index, 1)[0]. A negative index
// counts from the end, as in Array.prototype.splice. The result is nil when
// nothing is removed.
func (b *boundsState) spliceActivation(index int) *activation {
	n := len(b.activations)
	if index < 0 {
		index += n
		if index < 0 {
			index = 0
		}
	}
	if index >= n {
		return nil
	}
	removed := b.activations[index]
	b.activations = append(b.activations[:index:index], b.activations[index+1:]...)
	return removed
}

// endActivation is bounds.endActivation(message): it finds the most recent
// activation for the given actor.
func (b *boundsState) endActivation(msg *message) *activation {
	return b.spliceActivation(b.lastActivationIndex(msg.from))
}

// createLoop is bounds.createLoop(title, fill). title is nil for the default
// { message: undefined, wrap: false, width: undefined }.
func (b *boundsState) createLoop(title *message, fill any) *loopModel {
	l := &loopModel{height: 0, fill: fill}
	l.starty.set(b.verticalPos)
	if title != nil {
		l.title, l.hasTitle = title.message, true
		l.wrap = title.wrap
		l.width, l.hasWidth = title.width, title.hasWidth
	}
	return l
}

// newLoop is bounds.newLoop(title, fill).
func (b *boundsState) newLoop(title *message, fill any) {
	b.sequenceItems = append(b.sequenceItems, b.createLoop(title, fill))
}

// endLoop is bounds.endLoop(). The result is nil for an empty stack.
func (b *boundsState) endLoop() *loopModel {
	n := len(b.sequenceItems)
	if n == 0 {
		return nil
	}
	l := b.sequenceItems[n-1]
	b.sequenceItems = b.sequenceItems[:n-1]
	return l
}

// isLoopOverlap is bounds.isLoopOverlap().
func (b *boundsState) isLoopOverlap() bool {
	if n := len(b.sequenceItems); n > 0 {
		return b.sequenceItems[n-1].overlap
	}
	return false
}

// addSectionToLoop is bounds.addSectionToLoop(message).
func (b *boundsState) addSectionToLoop(msg *message) error {
	loop := b.endLoop()
	if loop == nil {
		return errors.New("TypeError: Cannot read properties of undefined (reading 'sections')")
	}
	loop.sections = append(loop.sections, &loopSection{y: b.getVerticalPos(), height: 0})
	loop.sectionTitles = append(loop.sectionTitles, msg)
	b.sequenceItems = append(b.sequenceItems, loop)
	return nil
}

// saveVerticalPos is bounds.saveVerticalPos().
func (b *boundsState) saveVerticalPos() {
	if b.isLoopOverlap() {
		b.savedVerticalPos.set(b.verticalPos)
	}
}

// resetVerticalPos is bounds.resetVerticalPos().
func (b *boundsState) resetVerticalPos() {
	if b.isLoopOverlap() {
		b.verticalPos = b.savedVerticalPos.nan()
	}
}

// bumpVerticalPos is bounds.bumpVerticalPos(bump).
func (b *boundsState) bumpVerticalPos(bump float64) {
	b.verticalPos = b.verticalPos + bump
	b.data.stopy.set(sanitize.GetMax(b.data.stopy.nan(), b.verticalPos))
}

// getVerticalPos is bounds.getVerticalPos().
func (b *boundsState) getVerticalPos() float64 { return b.verticalPos }
