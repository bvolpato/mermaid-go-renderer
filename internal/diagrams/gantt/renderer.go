// Ported from mermaid 12.2.0, diagrams/gantt/ganttRenderer.js (MIT).

package gantt

import (
	"math"
	"strings"

	"github.com/dlclark/regexp2"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/axis"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/scale"
	d3time "github.com/bvolpato/mermaid-go-renderer/internal/d3/time"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/timefmt"
	"github.com/bvolpato/mermaid-go-renderer/internal/dayjs"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

// objectPrototypeKeys are the property names that every plain JavaScript
// object inherits. Upstream uses plain objects as maps from a section name.
var objectPrototypeKeys = map[string]bool{
	"__proto__": true, "constructor": true, "hasOwnProperty": true, "isPrototypeOf": true,
	"propertyIsEnumerable": true, "toLocaleString": true, "toString": true, "valueOf": true,
	"__defineGetter__": true, "__defineSetter__": true, "__lookupGetter__": true, "__lookupSetter__": true,
}

// getMaxIntersections finds the number of intersections between tasks that
// happen at any point in time. It is used to figure out how many rows are
// needed to display the tasks when the display mode is set to 'compact'.
// See https://github.com/mermaid-js/mermaid/issues/1618.
func getMaxIntersections(tasks []*Task, orderOffset float64) float64 {
	timeline := make([]float64, len(tasks))
	for i := range timeline {
		timeline[i] = math.Inf(-1)
	}
	sorted := append([]*Task(nil), tasks...)
	js.SortV8(js.V8PowerSort, sorted, func(a, b *Task) float64 {
		// a.startTime - b.startTime || a.order - b.order
		if d := a.StartTime.T - b.StartTime.T; d != 0 && !math.IsNaN(d) {
			return d
		}
		return a.Order - b.Order
	})
	maxIntersections := 0.0
	for _, element := range sorted {
		for j := 0; j < len(timeline); j++ {
			if element.StartTime.T >= timeline[j] {
				timeline[j] = element.EndTime.T
				element.Order = float64(j) + orderOffset
				if float64(j) > maxIntersections {
					maxIntersections = float64(j)
				}
				break
			}
		}
	}

	return maxIntersections
}

const maxTickCount = 10000

var reTickInterval = regexp2.MustCompile(`^([1-9]\d*)(millisecond|second|minute|hour|day|week|month)$`, regexp2.ECMAScript)

// checkUnique is checkUnique: the values of the array without duplicates.
func checkUnique(arr []string) []string {
	hash := map[string]bool{}
	result := []string{}
	for _, v := range arr {
		if !hash[v] {
			// hash is a plain object upstream: the assignment to the
			// property __proto__ does nothing.
			if v != "__proto__" {
				hash[v] = true
			}
			result = append(result, v)
		}
	}
	return result
}

// d3Min is d3.min(tasks, accessor) for Date values, and d3Max is d3.max.
// The result is NaN when no task has a valid date: d3 returns undefined.
func d3Min(tasks []*Task, accessor func(*Task) Date) float64 {
	min, found := math.NaN(), false
	for _, t := range tasks {
		value := accessor(t)
		if value.Defined && ((found && min > value.T) || (!found && value.T >= value.T)) {
			min, found = value.T, true
		}
	}
	return min
}

func d3Max(tasks []*Task, accessor func(*Task) Date) float64 {
	max, found := math.NaN(), false
	for _, t := range tasks {
		value := accessor(t)
		if value.Defined && ((found && max < value.T) || (!found && value.T >= value.T)) {
			max, found = value.T, true
		}
	}
	return max
}

// drawer holds the variables that the functions inside draw share.
type drawer struct {
	ctx  *render.Context
	db   *DB
	conf *js.Obj[any]
	id   string
	svg  *dom.Node

	timeScale       *scale.Time
	categories      []string
	categoryHeights *js.Obj[float64]
}

// num reads a number of the gantt config. A value that is a string becomes
// a number here. Upstream keeps it as a string, and its + operator then
// joins text: this port does not reproduce that.
func (d *drawer) num(key string) float64 { return js.ToNumber(d.conf.Get(key)) }

// ts is timeScale(date).
func (d *drawer) ts(date Date) float64 { return d.timeScale.Scale(date.T) }

// draw is draw(text, id, version, diagObj).
func draw(ctx *render.Context, dbAny render.DB, _ string) error {
	db := dbAny.(*DB)
	conf := js.AsObj(ctx.Cfg("gantt"))
	if conf == nil {
		conf = js.NewObj[any]()
	}
	d := &drawer{ctx: ctx, db: db, conf: conf, id: ctx.ID}

	db.SetDiagramID(ctx.ID)

	elem := ctx.GetDiagramElement()
	// elem.parentElement.offsetWidth
	w := ctx.Engine.ContainerWidth()

	if useWidth := conf.Get("useWidth"); useWidth != nil {
		w = js.ToNumber(useWidth)
	}

	taskArray, err := db.GetTasks()
	if err != nil {
		return err
	}

	// Filter out vertical markers to ensure they don't take up rows
	var tasksWithoutVert []*Task
	for _, task := range taskArray {
		if !task.Vert {
			tasksWithoutVert = append(tasksWithoutVert, task)
		}
	}

	// Set height based on number of tasks

	categories := []string{}

	for _, element := range tasksWithoutVert {
		categories = append(categories, element.Type)
	}

	categories = checkUnique(categories)
	d.categories = categories
	categoryHeights := js.NewObj[float64]()
	d.categoryHeights = categoryHeights

	h := 2 * d.num("topPadding")
	if db.GetDisplayMode() == "compact" || conf.Get("displayMode") == "compact" {
		categoryElements := js.NewObj[[]*Task]()
		for _, element := range tasksWithoutVert {
			if objectPrototypeKeys[element.Section] {
				// The lookup finds an inherited property, which has no push.
				return &typeError{"categoryElements[element.section].push is not a function"}
			}
			categoryElements.Set(element.Section, append(categoryElements.Get(element.Section), element))
		}

		intersections := 0.0
		for _, category := range categoryElements.Keys() {
			categoryHeight := getMaxIntersections(categoryElements.Get(category), intersections) + 1
			intersections += categoryHeight
			h += float64(categoryHeight * (d.num("barHeight") + d.num("barGap")))
			categoryHeights.Set(category, categoryHeight)
		}
	} else {
		h += float64(float64(len(tasksWithoutVert)) * (d.num("barHeight") + d.num("barGap")))
		for _, category := range categories {
			n := 0
			for _, task := range tasksWithoutVert {
				if task.Type == category {
					n++
				}
			}
			if category != "__proto__" {
				categoryHeights.Set(category, float64(n))
			}
		}
	}

	// Set viewBox
	elem.Attr("viewBox", "0 0 "+js.Str(w)+" "+js.Str(h))
	svg := ctx.Body.Select(`[id="` + d.id + `"]`)
	d.svg = svg

	// Set timescale
	d.timeScale = scale.NewTime(ctx.Location).
		Domain(
			d3Min(taskArray, func(t *Task) Date { return t.StartTime }),
			d3Max(taskArray, func(t *Task) Date { return t.EndTime }),
		).
		RangeRound(0, w-d.num("leftPadding")-d.num("rightPadding"))

	taskCompare := func(a, b *Task) float64 {
		taskA := a.StartTime.T
		taskB := b.StartTime.T
		result := 0.0
		if taskA > taskB {
			result = 1
		} else if taskA < taskB {
			result = -1
		}
		return result
	}

	// Sort the task array using the above taskCompare() so that
	// tasks are created based on their order of startTime
	js.SortV8(js.V8PowerSort, taskArray, taskCompare)

	if err := d.makeGantt(taskArray, w, h); err != nil {
		return err
	}

	render.ConfigureSvgSize(svg, h, w, js.Truthy(conf.Get("useMaxWidth")))

	svg.
		Append("text").
		Text(db.GetDiagramTitle()).
		Attr("x", w/2).
		Attr("y", conf.Get("titleTopMargin")).
		Attr("class", "titleText")
	return nil
}

// makeGantt is makeGantt(tasks, pageWidth, pageHeight). Upstream also makes
// a color scale here, which nothing reads.
func (d *drawer) makeGantt(tasks []*Task, pageWidth, pageHeight float64) error {
	barHeight := d.num("barHeight")
	gap := barHeight + d.num("barGap")
	topPadding := d.num("topPadding")
	leftPadding := d.num("leftPadding")

	if err := d.drawExcludeDays(gap, topPadding, leftPadding, pageWidth, pageHeight, tasks, d.db.GetExcludes(), d.db.GetIncludes()); err != nil {
		return err
	}
	d.makeGrid(leftPadding, topPadding, pageWidth, pageHeight)
	d.drawRects(tasks, gap, topPadding, leftPadding, barHeight, pageWidth)
	d.vertLabels(gap, topPadding)
	d.drawToday(leftPadding, topPadding, pageWidth, pageHeight)
	return nil
}

// sectionNumber is the index of the category of a task modulo the number of
// section styles, as text. ok is false when no category matches. With last
// set, the last category that matches wins, as in the loops of upstream
// that have no return.
func (d *drawer) sectionNumber(typ string, last bool) (string, bool) {
	found := -1
	for i, category := range d.categories {
		if typ == category {
			found = i
			if !last {
				break
			}
		}
	}
	if found < 0 {
		return "", false
	}
	return js.Str(math.Mod(float64(found), d.num("numberSectionStyles"))), true
}

// drawRects is drawRects(theArray, theGap, theTopPad, theSidePad,
// theBarHeight, theColorScale, w).
func (d *drawer) drawRects(theArray []*Task, theGap, theTopPad, theSidePad, theBarHeight, w float64) {
	conf := d.conf
	id := d.id
	// Sort theArray so that tasks with `vert` come last
	js.SortV8(js.V8PowerSort, theArray, func(a, b *Task) float64 {
		if a.Vert == b.Vert {
			return 0
		}
		if a.Vert {
			return 1
		}
		return -1
	})
	// Filter out vertical markers from background rects so that they don't take up rows
	var tasksWithoutVert []*Task
	for _, task := range theArray {
		if !task.Vert {
			tasksWithoutVert = append(tasksWithoutVert, task)
		}
	}
	// Get unique task orders. Required to draw the background rects when display mode is compact.
	uniqueTaskOrderIDs := js.NewSet[float64]()
	for _, item := range tasksWithoutVert {
		uniqueTaskOrderIDs.Add(item.Order)
	}
	var uniqueTasks []*Task
	for _, orderID := range uniqueTaskOrderIDs.Values() {
		for _, item := range tasksWithoutVert {
			if item.Order == orderID {
				uniqueTasks = append(uniqueTasks, item)
				break
			}
		}
	}
	// Draw background rects covering the entire width of the graph, these form the section rows.
	background := d.svg.Append("g")
	for _, t := range uniqueTasks {
		// Ignore the incoming i value and use our order instead
		i := t.Order
		class := "section section0"
		if secNum, ok := d.sectionNumber(t.Type, false); ok {
			class = "section section" + secNum
		}
		background.Append("rect").
			Attr("x", 0).
			Attr("y", float64(i*theGap)+theTopPad-2).
			Attr("width", w-float64(d.num("rightPadding")/2)).
			Attr("height", theGap).
			Attr("class", class)
	}

	// Draw the rects representing the tasks
	rectangles := d.svg.Append("g")

	// Render the tasks with links
	// Render the other tasks
	tasksWithoutVertHeight := float64(float64(len(tasksWithoutVert)) * (d.num("barHeight") + d.num("barGap")))
	for _, t := range theArray {
		var x float64
		if t.Milestone {
			x = d.ts(t.StartTime) +
				theSidePad +
				float64(0.5*(d.ts(t.EndTime)-d.ts(t.StartTime))) -
				float64(0.5*theBarHeight)
		} else {
			x = d.ts(t.StartTime) + theSidePad
		}

		// Ignore the incoming i value and use our order instead
		i := t.Order
		var y any
		if t.Vert {
			y = conf.Get("gridLineStartPadding")
		} else {
			y = float64(i*theGap) + theTopPad
		}

		var width float64
		if t.Milestone {
			width = theBarHeight
		} else if t.Vert {
			width = 0.08 * theBarHeight
		} else {
			width = d.ts(renderEndOrEnd(t)) - d.ts(t.StartTime)
		}

		height := theBarHeight
		if t.Vert {
			height = tasksWithoutVertHeight + float64(d.num("barHeight")*2)
		}

		transformOrigin := js.Str(d.ts(t.StartTime)+
			theSidePad+
			float64(0.5*(d.ts(t.EndTime)-d.ts(t.StartTime)))) +
			"px " +
			js.Str(float64(i*theGap)+theTopPad+float64(0.5*theBarHeight)) +
			"px"

		res := "task"

		classStr := ""
		if len(t.Classes) > 0 {
			classStr = strings.Join(t.Classes, " ")
		}

		secNum := "0"
		if n, ok := d.sectionNumber(t.Type, true); ok {
			secNum = n
		}

		taskClass := ""
		if t.Active {
			if t.Crit {
				taskClass += " activeCrit"
			} else {
				taskClass = " active"
			}
		} else if t.Done {
			if t.Crit {
				taskClass = " doneCrit"
			} else {
				taskClass = " done"
			}
		} else {
			if t.Crit {
				taskClass += " crit"
			}
		}

		if len(taskClass) == 0 {
			taskClass = " task"
		}

		if t.Milestone {
			taskClass = " milestone " + taskClass
		}
		if t.Vert {
			taskClass = " vert " + taskClass
		}

		taskClass += secNum

		taskClass += " " + classStr

		rectangles.Append("rect").
			Attr("id", id+"-"+idKey(t.ID, t.HasID)).
			Attr("rx", 3).
			Attr("ry", 3).
			Attr("x", x).
			Attr("y", y).
			Attr("width", width).
			Attr("height", height).
			Attr("transform-origin", transformOrigin).
			Attr("class", res+taskClass)
	}

	// Append task labels. d3 runs each attr call for all texts before the
	// next call, and two of the calls measure the text, so the loops below
	// keep that order.
	texts := make([]*dom.Node, len(theArray))
	for i := range theArray {
		texts[i] = rectangles.Append("text")
	}
	for i, t := range theArray {
		texts[i].Attr("id", id+"-"+idKey(t.ID, t.HasID)+"-text")
	}
	for i, t := range theArray {
		texts[i].Text(t.Task)
	}
	for i := range theArray {
		texts[i].Attr("font-size", conf.Get("fontSize"))
	}
	for i, t := range theArray {
		startX := d.ts(t.StartTime)
		endX := d.ts(renderEndOrEnd(t))
		if t.Milestone {
			startX += float64(0.5*(d.ts(t.EndTime)-d.ts(t.StartTime))) - float64(0.5*theBarHeight)
			endX = startX + theBarHeight
		}

		if t.Vert {
			texts[i].Attr("x", d.ts(t.StartTime)+theSidePad)
			continue
		}

		textWidth := texts[i].BBox().Width

		// Check id text width > width of rectangle
		var x float64
		if textWidth > endX-startX {
			if endX+textWidth+float64(1.5*d.num("leftPadding")) > w {
				x = startX + theSidePad - 5
			} else {
				x = endX + theSidePad + 5
			}
		} else {
			x = float64((endX-startX)/2) + startX + theSidePad
		}
		texts[i].Attr("x", x)
	}
	for i, t := range theArray {
		// Ignore the incoming i value and use our order instead
		if t.Vert {
			texts[i].Attr("y", d.num("gridLineStartPadding")+
				tasksWithoutVertHeight+
				60)
			continue
		}
		order := t.Order
		texts[i].Attr("y", float64(order*theGap)+float64(d.num("barHeight")/2)+(float64(d.num("fontSize")/2)-2)+theTopPad)
	}
	for i := range theArray {
		texts[i].Attr("text-height", theBarHeight)
	}
	for i, t := range theArray {
		startX := d.ts(t.StartTime)
		endX := d.ts(t.EndTime)
		if t.Milestone {
			endX = startX + theBarHeight
		}

		textWidth := texts[i].BBox().Width

		classStr := ""
		if len(t.Classes) > 0 {
			classStr = strings.Join(t.Classes, " ")
		}

		secNum := "0"
		if n, ok := d.sectionNumber(t.Type, true); ok {
			secNum = n
		}

		taskType := ""
		if t.Active {
			if t.Crit {
				taskType = "activeCritText" + secNum
			} else {
				taskType = "activeText" + secNum
			}
		}

		if t.Done {
			if t.Crit {
				taskType = taskType + " doneCritText" + secNum
			} else {
				taskType = taskType + " doneText" + secNum
			}
		} else {
			if t.Crit {
				taskType = taskType + " critText" + secNum
			}
		}

		if t.Milestone {
			taskType += " milestoneText"
		}

		if t.Vert {
			taskType += " vertText"
		}

		// Check id text width > width of rectangle
		var class string
		if textWidth > endX-startX {
			if endX+textWidth+float64(1.5*d.num("leftPadding")) > w {
				class = classStr + " taskTextOutsideLeft taskTextOutside" + secNum + " " + taskType
			} else {
				class = classStr +
					" taskTextOutsideRight taskTextOutside" +
					secNum +
					" " +
					taskType +
					" width-" +
					js.Str(textWidth)
			}
		} else {
			class = classStr + " taskText taskText" + secNum + " " + taskType + " width-" + js.Str(textWidth)
		}
		texts[i].Attr("class", class)
	}

	// Upstream wraps the tasks that have links in an anchor element when the
	// security level is "sandbox". The pipeline does not support that level.
}

// renderEndOrEnd is d.renderEndTime || d.endTime: a Date object is truthy.
func renderEndOrEnd(t *Task) Date {
	if t.RenderEndTime.Defined {
		return t.RenderEndTime
	}
	return t.EndTime
}

// drawExcludeDays is drawExcludeDays(theGap, theTopPad, theSidePad, w, h,
// tasks, excludes, includes).
func (d *drawer) drawExcludeDays(theGap, theTopPad, theSidePad, _, h float64, tasks []*Task, excludes, includes []string) error {
	if len(excludes) == 0 && len(includes) == 0 {
		return nil
	}

	minTime, maxTime := undefinedDate, undefinedDate
	for _, t := range tasks {
		if !minTime.Defined || t.StartTime.T < minTime.T {
			minTime = t.StartTime
		}
		if !maxTime.Defined || t.EndTime.T > maxTime.T {
			maxTime = t.EndTime
		}
	}

	if !minTime.Defined || !maxTime.Defined {
		return nil
	}

	dj := d.db.dayjs
	if dj.FromTime(maxTime.T).Diff(dj.FromTime(minTime.T), "year", false) > 5 {
		// The difference between the min and max time is more than 5 years.
		// Upstream skips the exclude days, because they would be slow.
		return nil
	}

	dateFormat := d.db.GetDateFormat()
	type excludeRange struct{ start, end *dayjs.Dayjs }
	var excludeRanges []excludeRange
	var rng *excludeRange
	day := dj.FromTime(minTime.T)
	for day.ValueOf() <= maxTime.T {
		invalid, err := d.db.IsInvalidDate(day, dateFormat, excludes, includes)
		if err != nil {
			return err
		}
		if invalid {
			if rng == nil {
				rng = &excludeRange{start: day, end: day}
			} else {
				rng.end = day
			}
		} else {
			if rng != nil {
				excludeRanges = append(excludeRanges, *rng)
				rng = nil
			}
		}
		day = day.Add(1, "d")
	}

	rectangles := d.svg.Append("g")

	for i, r := range excludeRanges {
		startText, err := r.start.Format("YYYY-MM-DD")
		if err != nil {
			return err
		}
		tsv := func(v *dayjs.Dayjs) float64 { return d.timeScale.Scale(v.ValueOf()) }
		rectangles.Append("rect").
			Attr("id", d.id+"-exclude-"+startText).
			Attr("x", tsv(r.start.StartOf("day"))+theSidePad).
			Attr("y", d.conf.Get("gridLineStartPadding")).
			Attr("width", tsv(r.end.EndOf("day"))-tsv(r.start.StartOf("day"))).
			Attr("height", h-theTopPad-d.num("gridLineStartPadding")).
			Attr("transform-origin",
				js.Str(tsv(r.start)+
					theSidePad+
					float64(0.5*(tsv(r.end)-tsv(r.start))))+
					"px "+
					js.Str(float64(float64(i)*theGap)+float64(0.5*h))+
					"px").
			Attr("class", "exclude-range")
	}
	return nil
}

// getEstimatedTickCount calculates the estimated number of ticks based on
// the time domain and tick interval.
func (d *drawer) getEstimatedTickCount(minTime, maxTime, every float64, interval string) float64 {
	if every <= 0 || minTime > maxTime {
		return math.Inf(1)
	}
	timeDiffMs := maxTime - minTime
	obj := js.NewObj[float64]()
	obj.Set(interval, every)
	intervalMs := d.db.dayjs.DurationOf(obj).AsMilliseconds()
	if intervalMs <= 0 {
		return math.Inf(1)
	}
	return math.Ceil(timeDiffMs / intervalMs)
}

// tickEvery returns the argument of axis.ticks for a tick interval: the d3
// time interval of the unit with a step. The pointer is nil where
// interval.every returns null.
func (d *drawer) tickEvery(interval string, every float64) *d3time.Interval {
	iv := d.timeScale.Intervals()
	switch interval {
	case "millisecond":
		return iv.Millisecond.Every(every)
	case "second":
		return iv.Second.Every(every)
	case "minute":
		return iv.Minute.Every(every)
	case "hour":
		return iv.Hour.Every(every)
	case "day":
		return iv.Day.Every(every)
	case "week":
		// mapWeekdayToTimeFunction[weekday]
		weekday := d.db.GetWeekday()
		if weekday == "" {
			weekday = js.ToString(d.conf.Get("weekday"))
		}
		byName := map[string]*d3time.Interval{
			"monday": iv.Monday, "tuesday": iv.Tuesday, "wednesday": iv.Wednesday, "thursday": iv.Thursday,
			"friday": iv.Friday, "saturday": iv.Saturday, "sunday": iv.Sunday,
		}
		if day := byName[weekday]; day != nil {
			return day.Every(every)
		}
		return nil
	case "month":
		return iv.Month.Every(every)
	}
	return nil
}

// makeGrid is makeGrid(theSidePad, theTopPad, w, h).
func (d *drawer) makeGrid(theSidePad, theTopPad, _, h float64) {
	conf := d.conf
	db := d.db
	dateFormat := db.GetDateFormat()
	userAxisFormat := db.GetAxisFormat()
	var axisFormat string
	if userAxisFormat != "" {
		axisFormat = userAxisFormat
	} else if dateFormat == "D" {
		axisFormat = "%d"
	} else if v := conf.Get("axisFormat"); !js.IsNullish(v) {
		axisFormat = js.ToString(v)
	} else {
		axisFormat = "%Y-%m-%d"
	}

	bottomXAxis := axis.Bottom(axis.TimeScale(d.timeScale)).
		TickSize(-h + theTopPad + d.num("gridLineStartPadding")).
		TickFormat(timefmt.Format(d.ctx.Location, axisFormat))

	// db.getTickInterval() || conf.tickInterval
	var tickIntervalText string
	if v, ok := db.GetTickInterval(); ok && v != "" {
		tickIntervalText = v
	} else {
		// A value that is not a string becomes a string in exec.
		tickIntervalText = js.ToString(conf.Get("tickInterval"))
	}
	resultTickInterval, _ := reTickInterval.FindStringMatch(tickIntervalText)

	var every float64
	var interval string
	var estimatedTicks float64
	customTicks := false
	if resultTickInterval != nil {
		every = js.ParseInt(resultTickInterval.GroupByNumber(1).String(), 10)
		if math.IsNaN(every) || every <= 0 {
			// Invalid tick interval value. Skipping custom tick interval.
		} else {
			customTicks = true
			interval = resultTickInterval.GroupByNumber(2).String()

			// Get the time domain to check tick count
			domain := d.timeScale.DomainValues()
			minTime := domain[0]
			maxTime := domain[1]
			estimatedTicks = d.getEstimatedTickCount(minTime, maxTime, every, interval)

			if estimatedTicks > maxTickCount {
				// The tick interval would generate too many ticks. D3 will
				// use its default automatic tick generation.
			} else {
				bottomXAxis.Ticks(d.tickEvery(interval, every))
			}
		}
	}

	grid := d.svg.
		Append("g").
		Attr("class", "grid").
		Attr("transform", "translate("+js.Str(theSidePad)+", "+js.Str(h-50)+")")
	_ = bottomXAxis.Call(grid)
	for _, text := range grid.SelectAll("text") {
		text.
			Style("text-anchor", "middle").
			Attr("fill", "#000").
			Attr("stroke", "none").
			Attr("font-size", 10).
			Attr("dy", "1em")
	}

	if db.TopAxisEnabled() || js.Truthy(conf.Get("topAxis")) {
		topXAxis := axis.Top(axis.TimeScale(d.timeScale)).
			TickSize(-h + theTopPad + d.num("gridLineStartPadding")).
			TickFormat(timefmt.Format(d.ctx.Location, axisFormat))

		if customTicks {
			// Only apply custom ticks if the count is reasonable
			if estimatedTicks <= maxTickCount {
				topXAxis.Ticks(d.tickEvery(interval, every))
			}
		}

		topGrid := d.svg.
			Append("g").
			Attr("class", "grid").
			Attr("transform", "translate("+js.Str(theSidePad)+", "+js.Str(theTopPad)+")")
		_ = topXAxis.Call(topGrid)
		for _, text := range topGrid.SelectAll("text") {
			text.
				Style("text-anchor", "middle").
				Attr("fill", "#000").
				Attr("stroke", "none").
				Attr("font-size", 10)
		}
	}
}

// vertLabels is vertLabels(theGap, theTopPad): the titles of the sections.
func (d *drawer) vertLabels(theGap, theTopPad float64) {
	prevGap := 0.0

	type occurrence struct {
		name  string
		count float64
	}
	var numOccurrences []occurrence
	for _, key := range d.categoryHeights.Keys() {
		numOccurrences = append(numOccurrences, occurrence{key, d.categoryHeights.Get(key)})
	}

	// without doing this, impossible to put grid lines behind text
	g := d.svg.Append("g")
	labels := make([]*dom.Node, len(numOccurrences))
	for i, o := range numOccurrences {
		rows := sanitize.LineBreakRegex.Split(o.name, -1)
		dy := -float64(len(rows)-1) / 2

		svgLabel := g.Append("text")
		svgLabel.Attr("dy", js.Str(dy)+"em")

		for j, row := range rows {
			tspan := svgLabel.Append("tspan")
			tspan.Attr("alignment-baseline", "central")
			tspan.Attr("x", "10")
			if j > 0 {
				tspan.Attr("dy", "1em")
			}
			tspan.Text(row)
		}
		labels[i] = svgLabel
	}
	for i, o := range numOccurrences {
		var y float64
		if i > 0 {
			prevGap += numOccurrences[i-1].count
			y = float64(float64(o.count*theGap)/2) + float64(prevGap*theGap) + theTopPad
		} else {
			y = float64(float64(o.count*theGap)/2) + theTopPad
		}
		class := "sectionTitle"
		if secNum, ok := d.sectionNumber(o.name, false); ok {
			class = "sectionTitle sectionTitle" + secNum
		}
		labels[i].
			Attr("x", 10).
			Attr("y", y).
			Attr("font-size", d.conf.Get("sectionFontSize")).
			Attr("class", class)
	}
}

// drawToday is drawToday(theSidePad, theTopPad, w, h).
func (d *drawer) drawToday(theSidePad, _, _, h float64) {
	todayMarker := d.db.GetTodayMarker()
	if todayMarker == "off" {
		return
	}

	todayG := d.svg.Append("g").Attr("class", "today")
	today := dateOf(float64(d.ctx.Now.UnixMilli()))
	todayLine := todayG.Append("line")

	todayLine.
		Attr("x1", d.ts(today)+theSidePad).
		Attr("x2", d.ts(today)+theSidePad).
		Attr("y1", d.conf.Get("titleTopMargin")).
		Attr("y2", h-d.num("titleTopMargin")).
		Attr("class", "today")

	if todayMarker != "" {
		todayLine.Attr("style", strings.ReplaceAll(todayMarker, ",", ";"))
	}
}
