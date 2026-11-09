// Ported from mermaid 12.2.0, diagrams/sequence/sequenceDb.ts and
// diagrams/sequence/types.ts (MIT).

package sequence

import (
	"errors"
	"strings"

	"github.com/dlclark/regexp2"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// LINETYPE of sequenceDb.ts.
const (
	lineSolid                         = 0
	lineDotted                        = 1
	lineNote                          = 2
	lineSolidCross                    = 3
	lineDottedCross                   = 4
	lineSolidOpen                     = 5
	lineDottedOpen                    = 6
	lineLoopStart                     = 10
	lineLoopEnd                       = 11
	lineAltStart                      = 12
	lineAltElse                       = 13
	lineAltEnd                        = 14
	lineOptStart                      = 15
	lineOptEnd                        = 16
	lineActiveStart                   = 17
	lineActiveEnd                     = 18
	lineParStart                      = 19
	lineParAnd                        = 20
	lineParEnd                        = 21
	lineRectStart                     = 22
	lineRectEnd                       = 23
	lineSolidPoint                    = 24
	lineDottedPoint                   = 25
	lineAutonumber                    = 26
	lineCriticalStart                 = 27
	lineCriticalOption                = 28
	lineCriticalEnd                   = 29
	lineBreakStart                    = 30
	lineBreakEnd                      = 31
	lineParOverStart                  = 32
	lineBidirectionalSolid            = 33
	lineBidirectionalDotted           = 34
	lineSolidTop                      = 41
	lineSolidBottom                   = 42
	lineStickTop                      = 43
	lineStickBottom                   = 44
	lineSolidArrowTopReverse          = 45
	lineSolidArrowBottomReverse       = 46
	lineStickArrowTopReverse          = 47
	lineStickArrowBottomReverse       = 48
	lineSolidTopDotted                = 51
	lineSolidBottomDotted             = 52
	lineStickTopDotted                = 53
	lineStickBottomDotted             = 54
	lineSolidArrowTopReverseDotted    = 55
	lineSolidArrowBottomReverseDotted = 56
	lineStickArrowTopReverseDotted    = 57
	lineStickArrowBottomReverseDotted = 58
	lineCentralConnection             = 59
	lineCentralConnectionReverse      = 60
	lineCentralConnectionDual         = 61
)

// PLACEMENT of sequenceDb.ts.
const (
	placementLeftOf  = 0
	placementRightOf = 1
	placementOver    = 2
)

// PARTICIPANT_TYPE of sequenceDb.ts.
const (
	participantActor       = "actor"
	participantBoundary    = "boundary"
	participantCollections = "collections"
	participantControl     = "control"
	participantDatabase    = "database"
	participantEntity      = "entity"
	participantParticipant = "participant"
	participantQueue       = "queue"
)

// box is the Box interface of types.ts, with the rendering data that the
// renderer adds to the object.
type box struct {
	name string
	// nameUndefined is true when the box has no title: name is undefined.
	nameUndefined bool
	wrap          bool
	fill          string
	actorKeys     []string

	// Rendering data.
	x, y, width, height, margin  float64
	textMaxHeight                float64
	startx, starty, stopx, stopy float64
	stroke                       string
}

// actor is the Actor interface of types.ts, with the rendering data that the
// renderer adds to the object.
type actor struct {
	box         *box
	name        string
	description string
	// descriptionRaw is the description when it is not a string: an alias of
	// the participant metadata that is a number, for example.
	descriptionRaw any
	wrap           bool
	// prevActor and nextActor are "" for undefined.
	prevActor  string
	nextActor  string
	links      *js.Obj[any]
	properties *js.Obj[any]
	// actorCnt is null until the renderer draws the actor.
	actorCnt    int
	hasActorCnt bool
	rectData    *rectData
	// typ is the type. typRaw is the type when it is not a string.
	typ    string
	typRaw any

	// Rendering data. A zero value stands for undefined: upstream reads
	// these properties with `||`.
	x, width, height, margin float64
	starty, stopy            float64
	actorTextHeight          float64
	hasActorTextHeight       bool
}

// Kinds of a message object.
const (
	kindSignal = iota
	kindNote
	kindSequenceIndex
)

// autonumber is the message of a sequenceIndex record.
type autonumber struct {
	start, step       float64
	hasStart, hasStep bool
	visible           bool
}

// message is the Message interface of types.ts, with the models that the
// renderer adds to the object.
type message struct {
	kind int
	id   string
	// from and to are undefined when hasFrom and hasTo are false.
	from, to       string
	hasFrom, hasTo bool
	message        string
	auto           *autonumber
	wrap           bool
	typ            int
	activate       bool
	placement      int
	// isNote is `placement !== undefined`.
	isNote            bool
	centralConnection int

	// Rendering data. width is undefined when hasWidth is false.
	noteModel *noteModel
	msgModel  *msgModel
	width     float64
	hasWidth  bool
}

// note is the Note interface of types.ts.
type note struct {
	actor     []string
	placement int
	message   string
	wrap      bool
}

// parsedMessage is the result of parseMessage and of the text part of parseBoxData.
type parsedMessage struct {
	text string
	// wrap is undefined when hasWrap is false.
	wrap    bool
	hasWrap bool
}

// boxData is the result of parseBoxData.
type boxData struct {
	text    string
	hasText bool
	color   string
	wrap    bool
	hasWrap bool
}

// db is SequenceDB.
type db struct {
	render.CommonDB
	ctx *render.Context

	// The records of the ImperativeState.
	prevActor              string
	actors                 *js.Map[string, *actor]
	createdActors          *js.Map[string, int]
	destroyedActors        *js.Map[string, int]
	boxes                  []*box
	messages               []*message
	notes                  []*note
	sequenceNumbersEnabled bool
	// wrapEnabled is undefined when hasWrapEnabled is false.
	wrapEnabled    bool
	hasWrapEnabled bool
	currentBox     *box
	// lastCreated and lastDestroyed are actor ids. They are undefined when
	// the has field is false.
	lastCreated      string
	hasLastCreated   bool
	lastDestroyed    string
	hasLastDestroyed bool
}

// newDB is the constructor of SequenceDB followed by the db.clear() of
// Diagram.fromText. The constructor calls setWrap(getConfig().wrap), and
// clear() resets the state, so wrapEnabled is undefined again.
func newDB(ctx *render.Context) *db {
	d := &db{ctx: ctx}
	d.CommonDB.Sanitize = ctx.SanitizeText
	d.clear()
	return d
}

// getConfig is getConfig() of diagramAPI.
func (d *db) getConfig() *js.Obj[any] { return d.ctx.Config }

// sanitizeText is sanitizeText(text, getConfig()).
func (d *db) sanitizeText(text string) string {
	return d.ctx.SanitizeText(text)
}

// clear is clear().
func (d *db) clear() {
	d.prevActor = ""
	d.actors = js.NewMap[string, *actor]()
	d.createdActors = js.NewMap[string, int]()
	d.destroyedActors = js.NewMap[string, int]()
	d.boxes = nil
	d.messages = nil
	d.notes = nil
	d.sequenceNumbersEnabled = false
	d.wrapEnabled, d.hasWrapEnabled = false, false
	d.currentBox = nil
	d.lastCreated, d.hasLastCreated = "", false
	d.lastDestroyed, d.hasLastDestroyed = "", false
	d.ClearCommon()
}

// addBox is addBox(data).
func (d *db) addBox(data *boxData) {
	b := &box{
		name:          data.text,
		nameUndefined: !data.hasText,
		fill:          data.color,
		actorKeys:     []string{},
	}
	if data.hasWrap {
		b.wrap = data.wrap
	} else {
		b.wrap = d.autoWrap()
	}
	d.boxes = append(d.boxes, b)
	d.currentBox = b
}

// boxName is the name of a box inside a template literal.
func boxName(b *box) string {
	if b.nameUndefined {
		return "undefined"
	}
	return b.name
}

// participantMetaData is the part of ParticipantMetaData that addActor reads.
type participantMetaData struct {
	typ, alias any
}

// addActor is addActor(id, name, description, type, metadata). description
// is nil for undefined. typ is undefined when hasType is false. metadata is
// undefined when hasMetadata is false.
func (d *db) addActor(id, name string, description *parsedMessage, typ string, hasType bool, metadata string, hasMetadata bool) error {
	assignedBox := d.currentBox
	var doc *participantMetaData
	if hasMetadata {
		var yamlData string
		// detect if shapeData contains a newline character
		if !strings.Contains(metadata, "\n") {
			yamlData = "{\n" + metadata + "\n}"
		} else {
			yamlData = metadata + "\n"
		}
		var err error
		doc, err = loadParticipantMetaData(yamlData)
		if err != nil {
			return err
		}
	}
	// type = doc?.type ?? type
	var typRaw any
	if hasType {
		typRaw = typ
	}
	if doc != nil && !js.IsNullish(doc.typ) {
		typRaw = doc.typ
	}

	// description as a dynamic value: nil, or text and wrap.
	var descText any
	var descWrap *bool
	hasDescription := description != nil
	if description != nil {
		descText = description.text
		if description.hasWrap {
			w := description.wrap
			descWrap = &w
		}
	}

	// If alias is provided in metadata and description is not already set, use the alias
	if doc != nil && js.Truthy(doc.alias) && (!hasDescription || js.StrictEquals(descText, name)) {
		descText = doc.alias
		hasDescription = true
	}

	old := d.actors.Get(id)
	if old != nil {
		// If already set and trying to set to a new one throw error
		if d.currentBox != nil && old.box != nil && d.currentBox != old.box {
			return errors.New("A same participant should only be defined in one Box: " + old.name +
				" can't be in '" + boxName(old.box) + "' and in '" + boxName(d.currentBox) + "' at the same time.")
		}

		// Don't change the box if already
		if old.box != nil {
			assignedBox = old.box
		} else {
			assignedBox = d.currentBox
		}
		old.box = assignedBox

		// Don't allow description nulling
		if name == old.name && !hasDescription {
			return nil
		}
	}

	// Don't allow null descriptions, either
	if !hasDescription || js.IsNullish(descText) {
		descText = name
		descWrap = nil
	}
	if js.IsNullish(typRaw) || js.IsNullish(descText) {
		descText = name
		descWrap = nil
	}

	a := &actor{
		box:        assignedBox,
		name:       name,
		prevActor:  d.prevActor,
		links:      js.NewObj[any](),
		properties: js.NewObj[any](),
	}
	if s, ok := descText.(string); ok {
		a.description = s
	} else {
		a.description = js.ToString(descText)
		a.descriptionRaw = descText
	}
	if descWrap != nil {
		a.wrap = *descWrap
	} else {
		a.wrap = d.autoWrap()
	}
	switch t := typRaw.(type) {
	case nil:
		a.typ = participantParticipant
	case string:
		a.typ = t
	default:
		if js.IsNullish(t) {
			a.typ = participantParticipant
		} else {
			a.typ = js.ToString(t)
			a.typRaw = t
		}
	}
	d.actors.Set(id, a)
	if d.prevActor != "" {
		if prevActorInRecords := d.actors.Get(d.prevActor); prevActorInRecords != nil {
			prevActorInRecords.nextActor = id
		}
	}

	if d.currentBox != nil {
		d.currentBox.actorKeys = append(d.currentBox.actorKeys, id)
	}
	d.prevActor = id
	return nil
}

// activationCount is activationCount(part).
func (d *db) activationCount(part string) int {
	count := 0
	if part == "" {
		return 0
	}
	for _, m := range d.messages {
		if m.kind != kindSignal || !m.hasFrom {
			continue
		}
		if m.typ == lineActiveStart && m.from == part {
			count++
		}
		if m.typ == lineActiveEnd && m.from == part {
			count--
		}
	}
	return count
}

// nextID is messages.length.toString().
func (d *db) nextID() string {
	return js.Str(float64(len(d.messages)))
}

// addSignal is addSignal(idFrom, idTo, message, messageType, activate,
// centralConnection). msg is nil for undefined.
func (d *db) addSignal(idFrom string, hasFrom bool, idTo string, hasTo bool, msg *parsedMessage, messageType int, activate bool, centralConnection int) error {
	if messageType == lineActiveEnd {
		from := ""
		if hasFrom {
			from = idFrom
		}
		cnt := d.activationCount(from)
		if cnt < 1 {
			// Bail out as there is an activation signal from an inactive participant
			fromText := "undefined"
			if hasFrom {
				fromText = idFrom
			}
			return errors.New("Trying to inactivate an inactive participant (" + fromText + ")")
		}
	}
	m := &message{
		kind:              kindSignal,
		id:                d.nextID(),
		from:              idFrom,
		hasFrom:           hasFrom,
		to:                idTo,
		hasTo:             hasTo,
		typ:               messageType,
		activate:          activate,
		centralConnection: centralConnection,
	}
	if msg != nil {
		m.message = msg.text
	}
	if msg != nil && msg.hasWrap {
		m.wrap = msg.wrap
	} else {
		m.wrap = d.autoWrap()
	}
	d.messages = append(d.messages, m)
	return nil
}

func (d *db) hasAtLeastOneBox() bool { return len(d.boxes) > 0 }

func (d *db) hasAtLeastOneBoxWithTitle() bool {
	for _, b := range d.boxes {
		if !b.nameUndefined && b.name != "" {
			return true
		}
	}
	return false
}

func (d *db) getActorKeys() []string { return d.actors.Keys() }

func (d *db) enableSequenceNumbers()    { d.sequenceNumbersEnabled = true }
func (d *db) disableSequenceNumbers()   { d.sequenceNumbersEnabled = false }
func (d *db) showSequenceNumbers() bool { return d.sequenceNumbersEnabled }

// The regular expressions of extractWrap and parseBoxData. They run on
// regexp2 so that the classes \s and \w and the dot have the JavaScript
// meaning.
const (
	reS   = `[\t\n\v\f\r \u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000\ufeff]`
	reW   = `[A-Za-z0-9_]`
	reDot = `[^\n\r\u2028\u2029]`
)

var (
	reWrap    = regexp2.MustCompile(`^:?wrap:`, regexp2.None)
	reNoWrap  = regexp2.MustCompile(`^:?nowrap:`, regexp2.None)
	reAnyWrap = regexp2.MustCompile(`^:?(?:no)?wrap:`, regexp2.None)
	// /^((?:rgba?|hsla?)\s*\(.*\)|\w*)(.*)$/
	reBoxData = regexp2.MustCompile(`^((?:rgba?|hsla?)`+reS+`*\(`+reDot+`*\)|`+reW+`*)(`+reDot+`*)$`, regexp2.None)
)

func reTest(re *regexp2.Regexp, s string) bool {
	ok, err := re.MatchString(s)
	return err == nil && ok
}

// extractWrap is extractWrap(text) for a defined text.
func extractWrap(text string) parsedMessage {
	text = js.TrimSpace(text)
	var out parsedMessage
	if reTest(reWrap, text) {
		out.wrap, out.hasWrap = true, true
	} else if reTest(reNoWrap, text) {
		out.wrap, out.hasWrap = false, true
	}
	cleaned := text
	if out.hasWrap {
		// The match is ASCII, so its length in bytes is its length in the text.
		if m, err := reAnyWrap.FindStringMatch(text); err == nil && m != nil {
			cleaned = text[len(m.String()):]
		}
	}
	out.text = js.TrimSpace(cleaned)
	return out
}

// autoWrap is autoWrap().
func (d *db) autoWrap() bool {
	// if setWrap has been called, use that value, otherwise use the value from the config
	if d.hasWrapEnabled {
		return d.wrapEnabled
	}
	wrap := js.Path(d.getConfig(), "sequence", "wrap")
	if js.IsNullish(wrap) {
		return false
	}
	return js.Truthy(wrap)
}

// parseMessage is parseMessage(str).
func (d *db) parseMessage(str string) *parsedMessage {
	trimmedStr := js.TrimSpace(str)
	m := extractWrap(trimmedStr)
	return &m
}

// parseBoxData is parseBoxData(str). The box statement has the color first
// and then the description. The color can be rgb, rgba, hsl, hsla or a CSS
// color name. The first segment is the color and the rest of the line is the
// text.
func (d *db) parseBoxData(str string) *boxData {
	color := "transparent"
	title, hasTitle := "", false
	if m, err := reBoxData.FindStringMatch(str); err == nil && m != nil {
		if g := m.GroupByNumber(1).String(); g != "" {
			color = js.TrimSpace(g)
		}
		if g := m.GroupByNumber(2).String(); g != "" {
			title, hasTitle = js.TrimSpace(g), true
		}
	}

	// check that the string is a color
	if !cssSupportsColor(color) {
		color = "transparent"
		title, hasTitle = js.TrimSpace(str), true
	}
	out := &boxData{color: color}
	if hasTitle {
		m := extractWrap(title)
		out.wrap, out.hasWrap = m.wrap, m.hasWrap
		if m.text != "" {
			out.text, out.hasText = d.sanitizeText(m.text), true
		}
	}
	return out
}

// addNote is addNote(actor, placement, message). actor is one id, or the two
// ids of a note over two actors.
func (d *db) addNote(actors []string, placement int, msg *parsedMessage) {
	wrap := d.autoWrap()
	if msg.hasWrap {
		wrap = msg.wrap
	}
	d.notes = append(d.notes, &note{actor: actors, placement: placement, message: msg.text, wrap: wrap})

	// Coerce actor into a [to, from, ...] array: [].concat(actor, actor).
	both := append(append([]string{}, actors...), actors...)
	wrap = d.autoWrap()
	if msg.hasWrap {
		wrap = msg.wrap
	}
	m := &message{
		kind:      kindNote,
		id:        d.nextID(),
		message:   msg.text,
		wrap:      wrap,
		typ:       lineNote,
		placement: placement,
		isNote:    true,
	}
	if len(both) > 0 {
		m.from, m.hasFrom = both[0], true
	}
	if len(both) > 1 {
		m.to, m.hasTo = both[1], true
	}
	d.messages = append(d.messages, m)
}

// utf16Index is String.prototype.indexOf for a one-byte needle: the index in
// UTF-16 code units, or -1.
func utf16Index(s string, needle byte) int {
	i := strings.IndexByte(s, needle)
	if i < 0 {
		return -1
	}
	return js.UTF16Len(s[:i])
}

// addLinks is addLinks(actorId, text).
func (d *db) addLinks(actorID string, text *parsedMessage) {
	// find the actor
	a := d.actors.Get(actorID)
	// JSON.parse the text
	sanitizedText := d.sanitizeText(text.text)
	sanitizedText = strings.ReplaceAll(sanitizedText, "&equals;", "=")
	sanitizedText = strings.ReplaceAll(sanitizedText, "&amp;", "&")
	links, err := js.ParseJSON([]byte(sanitizedText))
	if err != nil || a == nil {
		// log.error('error while parsing actor link text', e)
		return
	}
	// add the deserialized text to the actor's links field.
	insertRecord(a.links, links)
}

// addALink is addALink(actorId, text).
func (d *db) addALink(actorID string, text *parsedMessage) {
	// find the actor
	a := d.actors.Get(actorID)
	sanitizedText := d.sanitizeText(text.text)
	sep := utf16Index(sanitizedText, '@')
	sanitizedText = strings.ReplaceAll(sanitizedText, "&equals;", "=")
	sanitizedText = strings.ReplaceAll(sanitizedText, "&amp;", "&")
	label := js.TrimSpace(js.StringSlice(sanitizedText, 0, sep-1))
	link := js.TrimSpace(js.StringSliceFrom(sanitizedText, sep+1))
	if a == nil {
		return
	}
	// add the deserialized text to the actor's links field.
	a.links.Set(label, link)
}

// insertRecord is the loop of insertLinks and insertProperties:
// `for (const key in src) dst[key] = src[key]`. dst is never null: addActor
// sets it to an empty object.
func insertRecord(dst *js.Obj[any], src any) {
	switch s := src.(type) {
	case *js.Obj[any]:
		s.Each(func(key string, v any) {
			if key == "__proto__" {
				// The assignment sets the prototype and adds no property.
				return
			}
			dst.Set(key, v)
		})
	case []any:
		for i, v := range s {
			dst.Set(js.Str(float64(i)), v)
		}
	case string:
		// for...in on a string visits the index of every code unit.
		for i, u := range js.UTF16Units(s) {
			dst.Set(js.Str(float64(i)), js.FromUTF16Units([]rune{u}))
		}
	}
}

// addProperties is addProperties(actorId, text).
func (d *db) addProperties(actorID string, text *parsedMessage) {
	// find the actor
	a := d.actors.Get(actorID)
	// JSON.parse the text
	sanitizedText := d.sanitizeText(text.text)
	properties, err := js.ParseJSON([]byte(sanitizedText))
	if err != nil || a == nil {
		// log.error('error while parsing actor properties text', e)
		return
	}
	// add the deserialized text to the actor's property field.
	insertRecord(a.properties, properties)
}

// boxEnd is boxEnd().
func (d *db) boxEnd() { d.currentBox = nil }

// addDetails is addDetails(actorId, text). Upstream reads the JSON text from
// the element of the page that has the id text.text. The page of a render
// has no element with JSON content: the body holds only the container of the
// diagram. JSON.parse then throws and the function changes nothing.
func (d *db) addDetails(string, *parsedMessage) {}

// param is one object that a semantic action of the grammar builds: the
// argument of apply.
type param struct {
	typ string
	// actor is the id of the participant. For an addNote of a note over two
	// actors, actors holds both ids.
	actor  string
	actors []string
	// description is undefined when it is nil.
	description *parsedMessage
	// draw is undefined when hasDraw is false.
	draw    string
	hasDraw bool
	// config is the participant metadata. It is undefined when hasConfig is false.
	config    string
	hasConfig bool

	signalType        int
	from, to          string
	msg               *parsedMessage
	activate          bool
	centralConnection int
	placement         int
	// text is the text of a note, of links, of properties and of details,
	// and the title of a loop, alt, opt, par, critical, break, or the color
	// of a rect.
	text    *parsedMessage
	boxData *boxData

	sequenceIndex        float64
	hasSequenceIndex     bool
	sequenceIndexStep    float64
	hasSequenceIndexStep bool
	sequenceVisible      bool
}

// apply is apply(param). An array is applied element by element. A value
// that is not an object of the grammar, such as the string of a title
// statement, has no type and does nothing.
func (d *db) apply(p any) error {
	switch v := p.(type) {
	case []any:
		for _, item := range v {
			if err := d.apply(item); err != nil {
				return err
			}
		}
	case *param:
		return d.applyParam(v)
	}
	return nil
}

func (d *db) applyParam(p *param) error {
	none := func(signalType int, text *parsedMessage) error {
		return d.addSignal("", false, "", false, text, signalType, false, 0)
	}
	switch p.typ {
	case "sequenceIndex":
		d.messages = append(d.messages, &message{
			kind: kindSequenceIndex,
			id:   d.nextID(),
			auto: &autonumber{
				start:    p.sequenceIndex,
				hasStart: p.hasSequenceIndex,
				step:     p.sequenceIndexStep,
				hasStep:  p.hasSequenceIndexStep,
				visible:  p.sequenceVisible,
			},
			wrap: false,
			typ:  p.signalType,
		})
	case "addParticipant":
		return d.addActor(p.actor, p.actor, p.description, p.draw, p.hasDraw, p.config, p.hasConfig)
	case "createParticipant":
		if d.actors.Has(p.actor) {
			return errors.New("It is not possible to have actors with the same id, even if one is destroyed before the next is created. Use 'AS' aliases to simulate the behavior")
		}
		d.lastCreated, d.hasLastCreated = p.actor, true
		if err := d.addActor(p.actor, p.actor, p.description, p.draw, p.hasDraw, p.config, p.hasConfig); err != nil {
			return err
		}
		d.createdActors.Set(p.actor, len(d.messages))
	case "destroyParticipant":
		d.lastDestroyed, d.hasLastDestroyed = p.actor, true
		d.destroyedActors.Set(p.actor, len(d.messages))
	case "activeStart", "centralConnection", "centralConnectionReverse", "activeEnd":
		return d.addSignal(p.actor, true, "", false, nil, p.signalType, false, 0)
	case "addNote":
		actors := p.actors
		if actors == nil {
			actors = []string{p.actor}
		}
		d.addNote(actors, p.placement, p.text)
	case "addLinks":
		d.addLinks(p.actor, p.text)
	case "addALink":
		d.addALink(p.actor, p.text)
	case "addProperties":
		d.addProperties(p.actor, p.text)
	case "addDetails":
		d.addDetails(p.actor, p.text)
	case "addMessage":
		// lastCreated and lastDestroyed hold the id, a string. Upstream
		// reads its name property for the message, which is undefined.
		if d.hasLastCreated && d.lastCreated != "" {
			if p.to != d.lastCreated {
				return errors.New("The created participant undefined does not have an associated creating message after its declaration. Please check the sequence diagram.")
			}
			d.lastCreated, d.hasLastCreated = "", false
		} else if d.hasLastDestroyed && d.lastDestroyed != "" {
			if p.to != d.lastDestroyed && p.from != d.lastDestroyed {
				return errors.New("The destroyed participant undefined does not have an associated destroying message after its declaration. Please check the sequence diagram.")
			}
			d.lastDestroyed, d.hasLastDestroyed = "", false
		}
		return d.addSignal(p.from, true, p.to, true, p.msg, p.signalType, p.activate, p.centralConnection)
	case "boxStart":
		d.addBox(p.boxData)
	case "boxEnd":
		d.boxEnd()
	case "loopStart", "rectStart", "optStart", "altStart", "else", "parStart", "and",
		"criticalStart", "option", "breakStart":
		return none(p.signalType, p.text)
	case "loopEnd", "rectEnd", "optEnd", "altEnd", "parEnd", "criticalEnd", "breakEnd":
		return none(p.signalType, nil)
	case "setAccTitle":
		d.SetAccTitle(p.text.text)
	}
	return nil
}
