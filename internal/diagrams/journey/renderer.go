// Ported from mermaid 12.2.0, diagrams/user-journey/journeyRenderer.ts (MIT).

package journey

import (
	"errors"
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/svgdraw"
)

// journeyConf is the journey part of the config: getConfig().journey.
type journeyConf struct {
	obj *js.Obj[any]
}

// get reads a property of the journey config.
func (c *journeyConf) get(key string) any {
	return c.obj.Get(key)
}

// num reads a numeric property. A value that is not a number is converted
// as the arithmetic operators of JavaScript convert it.
func (c *journeyConf) num(key string) float64 {
	return js.ToNumber(c.obj.Get(key))
}

// attrValue converts a config value for selection.attr(name, value) and
// selection.style(name, value): null and undefined remove the attribute.
func attrValue(value any) any {
	if js.IsNullish(value) {
		return nil
	}
	return value
}

// actor is an entry of the actors object: the color and the position of a
// person in the legend.
type actor struct {
	color    any
	position float64
}

// actorTable is a plain object that maps the name of a person to an actor.
// The key "__proto__" is not a property of a JavaScript object: an
// assignment to it sets the prototype. Such an actor can be read by its
// name and is not in Object.keys.
type actorTable struct {
	own   *js.Obj[*actor]
	proto *actor
}

func newActorTable() actorTable {
	return actorTable{own: js.NewObj[*actor]()}
}

// set is table[name] = a.
func (t *actorTable) set(name string, a *actor) {
	if name == "__proto__" {
		t.proto = a
		return
	}
	t.own.Set(name, a)
}

// get is table[name], or nil for undefined.
func (t *actorTable) get(name string) *actor {
	if name == "__proto__" {
		return t.proto
	}
	if t.own == nil {
		return nil
	}
	return t.own.Get(name)
}

// keys is Object.keys(table).
func (t *actorTable) keys() []string {
	return t.own.Keys()
}

// renderer holds the module state of journeyRenderer.ts.
type renderer struct {
	ctx *render.Context

	// conf is the module variable conf: the journey config at the time when
	// the module loads, with the keys that setConf copies into it.
	conf *journeyConf
	// fills and textColours are conf.sectionFills and conf.sectionColours at
	// the time when the module loads.
	fills       []any
	textColours []any

	actors     actorTable
	maxWidth   float64
	leftMargin float64
	bounds     bounds
	svgDraw    svgDraw
}

// errNoJourneyConfig is the TypeError of upstream for a config that has no
// journey object.
var errNoJourneyConfig = errors.New("Cannot convert undefined or null to object")

// newRenderer evaluates the module-level statements of journeyRenderer.ts.
func newRenderer(ctx *render.Context) (*renderer, error) {
	r := &renderer{ctx: ctx, actors: newActorTable()}
	// const conf = getConfig().journey;
	obj, ok := js.Clone(ctx.Config.Get("journey")).(*js.Obj[any])
	if !ok {
		return nil, errNoJourneyConfig
	}
	r.conf = &journeyConf{obj: obj}
	// const fills = conf.sectionFills; const textColours = conf.sectionColours;
	r.fills, _ = obj.Get("sectionFills").([]any)
	r.textColours, _ = obj.Get("sectionColours").([]any)
	r.bounds.boxMargin = func() float64 {
		conf, err := r.getConf()
		if err != nil {
			return math.NaN()
		}
		return conf.num("boxMargin")
	}
	return r, nil
}

// setConf is setConf(cnf).
func (r *renderer) setConf(cnf any) {
	obj, ok := cnf.(*js.Obj[any])
	if !ok {
		return
	}
	for _, key := range obj.Keys() {
		r.conf.obj.Set(key, obj.Get(key))
	}
}

// getConf is getConfig().journey.
func (r *renderer) getConf() (*journeyConf, error) {
	obj, ok := r.ctx.Config.Get("journey").(*js.Obj[any])
	if !ok {
		return nil, errNoJourneyConfig
	}
	return &journeyConf{obj: obj}, nil
}

// indexOf is arr[index % arr.length]: nil for an empty array.
func indexOf(arr []any, index float64) any {
	i := math.Mod(index, float64(len(arr)))
	if math.IsNaN(i) {
		return nil
	}
	return arr[int(i)]
}

// drawActorLegend is drawActorLegend(diagram).
func (r *renderer) drawActorLegend(diagram *dom.Node) error {
	conf, err := r.getConf()
	if err != nil {
		return err
	}
	maxLabelWidth := conf.num("maxLabelWidth")
	r.maxWidth = 0
	yPos := 60.0

	for _, person := range r.actors.keys() {
		colour := r.actors.get(person).color
		circleData := circleData{
			cx:     20,
			cy:     yPos,
			r:      7,
			fill:   colour,
			stroke: "#000",
			pos:    r.actors.get(person).position,
		}
		drawCircle(diagram, circleData)

		// First, measure the full text width without wrapping.
		measureText := diagram.Append("text").Attr("visibility", "hidden").Text(person)
		fullTextWidth := measureText.BoundingClientRect().Width
		measureText.Remove()

		var lines []string

		// If the text is naturally within the max width, use it as a single line.
		if fullTextWidth <= maxLabelWidth {
			lines = []string{person}
		} else {
			// Otherwise, wrap the text.
			words := strings.Split(person, " ") // Split the text into words.
			currentLine := ""
			measureText = diagram.Append("text").Attr("visibility", "hidden")

			for _, word := range words {
				// Check the width of the line with the new word.
				testLine := word
				if currentLine != "" {
					testLine = currentLine + " " + word
				}
				measureText.Text(testLine)
				textWidth := measureText.BoundingClientRect().Width

				if textWidth > maxLabelWidth {
					// If adding the new word exceeds the max width, push the current line.
					if currentLine != "" {
						lines = append(lines, currentLine)
					}
					currentLine = word // Start a new line with the current word.

					// If the word itself is too long, break it with a hyphen.
					measureText.Text(word)
					if measureText.BoundingClientRect().Width > maxLabelWidth {
						brokenWord := ""
						for _, char := range codePoints(word) {
							brokenWord = js.Concat(brokenWord, char)
							measureText.Text(brokenWord + "-")
							if measureText.BoundingClientRect().Width > maxLabelWidth {
								// Push the broken part with a hyphen.
								lines = append(lines, js.StringSlice(brokenWord, 0, -1)+"-")
								brokenWord = char
							}
						}
						currentLine = brokenWord
					}
				} else {
					// If the line with the new word fits, add the new word to the current line.
					currentLine = testLine
				}
			}

			// Push the last line.
			if currentLine != "" {
				lines = append(lines, currentLine)
			}
			measureText.Remove() // Remove the text element used for measuring.
		}

		for index, line := range lines {
			labelData := svgdraw.TextData{
				X:          40,
				Y:          yPos + 7 + float64(index*20),
				Text:       line,
				TextMargin: 5,
			}
			if v := conf.get("boxTextMargin"); !js.IsNullish(v) {
				labelData.TextMargin = js.ToNumber(v)
			}

			// Draw the text and measure the width.
			textElement := drawText(diagram, labelData)
			lineWidth := textElement.BoundingClientRect().Width

			// Use conf.leftMargin as the initial spacing baseline, but expand
			// maxWidth if the line is wider.
			if lineWidth > r.maxWidth && lineWidth > conf.num("leftMargin")-lineWidth {
				r.maxWidth = lineWidth
			}
		}

		yPos += js.Max(20, float64(len(lines)*20))
	}
	return nil
}

// codePoints splits a string as the string iterator of JavaScript does: by
// code points. A lone surrogate from the lexer is one element.
func codePoints(s string) []string {
	units := js.UTF16Units(s)
	var out []string
	for i := 0; i < len(units); i++ {
		u := units[i]
		if u >= 0xd800 && u < 0xdc00 && i+1 < len(units) && units[i+1] >= 0xdc00 && units[i+1] < 0xe000 {
			out = append(out, js.FromUTF16Units(units[i:i+2]))
			i++
			continue
		}
		out = append(out, js.FromUTF16Units(units[i:i+1]))
	}
	return out
}

// draw is draw(text, id, version, diagObj).
func (r *renderer) draw(db *DB) error {
	conf := r.conf
	id := r.ctx.ID
	configObject := r.ctx.Config
	journey, ok := configObject.Get("journey").(*js.Obj[any])
	if !ok {
		return errNoJourneyConfig
	}
	titleColor := journey.Get("titleColor")
	titleFontSize := journey.Get("titleFontSize")
	titleFontFamily := journey.Get("titleFontFamily")

	r.bounds.init()
	diagram := r.ctx.Body.Select("#" + id)

	r.svgDraw.initGraphics(diagram, id)

	tasks := db.GetTasks()
	title := db.GetDiagramTitle()

	actorNames := db.GetActors()
	r.actors = newActorTable()
	actorColours, ok := conf.get("actorColours").([]any)
	if !ok {
		return errors.New("Cannot read properties of undefined (reading 'length')")
	}
	actorPos := 0.0
	for _, actorName := range actorNames {
		r.actors.set(actorName, &actor{
			color:    indexOf(actorColours, actorPos),
			position: actorPos,
		})
		actorPos++
	}

	if err := r.drawActorLegend(diagram); err != nil {
		return err
	}
	r.leftMargin = conf.num("leftMargin") + r.maxWidth
	r.bounds.insert(0, 0, r.leftMargin, float64(len(r.actors.keys())*50))
	if err := r.drawTasks(diagram, tasks, 0, id); err != nil {
		return err
	}

	box := r.bounds.getBounds()
	if title != "" {
		diagram.
			Append("text").
			Text(title).
			Attr("x", r.leftMargin).
			Attr("font-size", attrValue(titleFontSize)).
			Attr("font-weight", "bold").
			Attr("y", 25).
			Attr("fill", attrValue(titleColor)).
			Attr("font-family", attrValue(titleFontFamily))
	}

	height := box.stopy.value() - box.starty.value() + float64(2*conf.num("diagramMarginY"))
	width := r.leftMargin + box.stopx.value() + float64(2*conf.num("diagramMarginX"))

	render.ConfigureSvgSize(diagram, height, width, js.Truthy(conf.get("useMaxWidth")))

	// Draw the activity line.
	diagram.
		Append("line").
		Attr("x1", r.leftMargin).
		Attr("y1", conf.num("height")*4). // One section head + one task + margins.
		Attr("x2", width-r.leftMargin-4). // Subtract the stroke width so that the arrow point is retained.
		Attr("y2", conf.num("height")*4).
		Attr("stroke-width", 4).
		Attr("stroke", "black").
		Attr("marker-end", "url(#"+id+"-arrowhead)")

	extraVertForTitle := 0.0
	if title != "" {
		extraVertForTitle = 70
	}
	diagram.Attr("viewBox", js.Str(box.startx.value())+" -25 "+js.Str(width)+" "+js.Str(height+extraVertForTitle))
	diagram.Attr("preserveAspectRatio", "xMinYMin meet")
	diagram.Attr("height", height+extraVertForTitle+25)
	return nil
}

// optNumber is a number that can be undefined.
type optNumber struct {
	v       float64
	defined bool
}

// value is the number that arithmetic gets: NaN for undefined.
func (o optNumber) value() float64 {
	if !o.defined {
		return math.NaN()
	}
	return o.v
}

// boundsData is the data object of bounds, and also an item of
// sequenceItems.
type boundsData struct {
	startx, stopx, starty, stopy optNumber
}

// bounds is the bounds object of journeyRenderer.ts.
type bounds struct {
	data        boundsData
	verticalPos float64

	sequenceItems []*boundsData

	// boxMargin is conf.boxMargin of getConfig().journey, for updateBounds.
	boxMargin func() float64
}

// init is bounds.init().
func (b *bounds) init() {
	b.sequenceItems = nil
	b.data = boundsData{}
	b.verticalPos = 0
}

// updateVal is bounds.updateVal(obj, key, val, fun): key is a pointer to
// the property.
func (b *bounds) updateVal(key *optNumber, val float64, fun func(...float64) float64) {
	if !key.defined {
		*key = optNumber{v: val, defined: true}
	} else {
		key.v = fun(val, key.v)
	}
}

// updateBounds is bounds.updateBounds(startx, starty, stopx, stopy). The
// journey renderer never adds an item to sequenceItems.
func (b *bounds) updateBounds(startx, starty, stopx, stopy float64) {
	cnt := 0
	updateFn := func(activation bool) func(item *boundsData) {
		return func(item *boundsData) {
			boxMargin := b.boxMargin()
			cnt++
			// sequenceItems is a stack, so the biggest margins are at the
			// beginning of sequenceItems.
			n := float64(len(b.sequenceItems) - cnt + 1)
			b.updateVal(&item.starty, starty-float64(n*boxMargin), js.Min)
			b.updateVal(&item.stopy, stopy+float64(n*boxMargin), js.Max)

			b.updateVal(&b.data.startx, startx-float64(n*boxMargin), js.Min)
			b.updateVal(&b.data.stopx, stopx+float64(n*boxMargin), js.Max)

			if !activation {
				b.updateVal(&item.startx, startx-float64(n*boxMargin), js.Min)
				b.updateVal(&item.stopx, stopx+float64(n*boxMargin), js.Max)

				b.updateVal(&b.data.starty, starty-float64(n*boxMargin), js.Min)
				b.updateVal(&b.data.stopy, stopy+float64(n*boxMargin), js.Max)
			}
		}
	}

	fn := updateFn(false)
	for _, item := range b.sequenceItems {
		fn(item)
	}
}

// insert is bounds.insert(startx, starty, stopx, stopy).
func (b *bounds) insert(startx, starty, stopx, stopy float64) {
	_startx := js.Min(startx, stopx)
	_stopx := js.Max(startx, stopx)
	_starty := js.Min(starty, stopy)
	_stopy := js.Max(starty, stopy)

	b.updateVal(&b.data.startx, _startx, js.Min)
	b.updateVal(&b.data.starty, _starty, js.Min)
	b.updateVal(&b.data.stopx, _stopx, js.Max)
	b.updateVal(&b.data.stopy, _stopy, js.Max)

	b.updateBounds(_startx, _starty, _stopx, _stopy)
}

// bumpVerticalPos is bounds.bumpVerticalPos(bump).
func (b *bounds) bumpVerticalPos(bump float64) {
	b.verticalPos = b.verticalPos + bump
	b.data.stopy = optNumber{v: b.verticalPos, defined: true}
}

// getVerticalPos is bounds.getVerticalPos().
func (b *bounds) getVerticalPos() float64 {
	return b.verticalPos
}

// getBounds is bounds.getBounds().
func (b *bounds) getBounds() *boundsData {
	return &b.data
}

// drawTasks is drawTasks(diagram, tasks, verticalPos, diagramId).
func (r *renderer) drawTasks(diagram *dom.Node, tasks []*Task, verticalPos float64, diagramID string) error {
	conf, err := r.getConf()
	if err != nil {
		return err
	}
	fills, textColours := r.fills, r.textColours
	if fills == nil || textColours == nil {
		return errors.New("Cannot read properties of undefined (reading 'length')")
	}
	lastSection := ""
	sectionVHeight := float64(conf.num("height")*2) + conf.num("diagramMarginY")
	taskPos := verticalPos + sectionVHeight

	sectionNumber := 0.0
	var fill any = "#CCC"
	var colour any = "black"
	num := 0.0

	// Draw the tasks.
	for i, task := range tasks {
		fi := float64(i)
		if lastSection != task.Section {
			fill = indexOf(fills, sectionNumber)
			num = math.Mod(sectionNumber, float64(len(fills)))
			colour = indexOf(textColours, sectionNumber)

			// Count how many consecutive tasks have the same section.
			taskInSectionCount := 0.0
			currentSection := task.Section
			for taskIndex := i; taskIndex < len(tasks); taskIndex++ {
				if tasks[taskIndex].Section == currentSection {
					taskInSectionCount = taskInSectionCount + 1
				} else {
					break
				}
			}

			section := sectionData{
				x:         float64(fi*conf.num("taskMargin")) + float64(fi*conf.num("width")) + r.leftMargin,
				y:         50,
				text:      task.Section,
				fill:      fill,
				num:       num,
				colour:    colour,
				taskCount: taskInSectionCount,
			}

			drawSection(diagram, section, conf)
			lastSection = task.Section
			sectionNumber++
		}

		// Collect the actors involved in the task.
		if task.People == nil {
			return errors.New("Cannot read properties of undefined (reading 'reduce')")
		}
		taskActors := newActorTable()
		for _, actorName := range task.People {
			if a := r.actors.get(actorName); a != nil {
				taskActors.set(actorName, a)
			}
		}

		// Add some rendering data to the object.
		task.x = float64(fi*conf.num("taskMargin")) + float64(fi*conf.num("width")) + r.leftMargin
		task.y = taskPos
		task.width = conf.num("diagramMarginX")
		task.height = conf.num("diagramMarginY")
		task.colour = colour
		task.fill = fill
		task.num = num
		task.actors = taskActors

		// Draw the box with the attached line.
		if err := r.svgDraw.drawTask(diagram, task, conf, diagramID); err != nil {
			return err
		}
		r.bounds.insert(task.x, task.y, task.x+task.width+conf.num("taskMargin"), 300+5*30) // stopY is the length of the descenders.
	}
	return nil
}
