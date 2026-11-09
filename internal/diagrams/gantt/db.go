// Ported from mermaid 12.2.0, diagrams/gantt/ganttDb.js (MIT).

package gantt

import (
	"errors"
	"math"
	"strings"

	"github.com/dlclark/regexp2"

	"github.com/bvolpato/mermaid-go-renderer/internal/dayjs"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

// typeError is a JavaScript TypeError that the upstream code throws.
type typeError struct{ message string }

func (e *typeError) Error() string { return "TypeError: " + e.message }

// weekendStartDay is WEEKEND_START_DAY.
var weekendStartDay = map[string]float64{"friday": 5, "saturday": 6}

var tags = []string{"active", "done", "crit", "milestone", "vert"}

// Date is a value that upstream holds in task.startTime, task.endTime and
// task.renderEndTime: a Date object, or undefined (null for renderEndTime).
// T is the time value of the Date object. It is NaN for an invalid date, and
// for a value that is not defined, so that comparisons and arithmetic give
// the JavaScript result.
type Date struct {
	T       float64
	Defined bool
}

func dateOf(t float64) Date { return Date{T: t, Defined: true} }

var undefinedDate = Date{T: math.NaN()}

// rawStartTime is task.raw.startTime: how to find the start of the task.
type rawStartTime struct {
	// typ is "prevTaskEnd" or "getStartDate". It is "" for task data that
	// has no supported form: upstream then has no raw.startTime object.
	typ       string
	startData string
}

// Task is a task of the diagram: an element of rawTasks.
type Task struct {
	Section string
	// Type is the same as Section.
	Type          string
	Processed     bool
	ManualEndTime bool
	// RenderEndTime is null upstream when it is not defined.
	RenderEndTime Date
	// Raw is raw.data: the text after the colon.
	Raw          string
	rawStartTime rawStartTime
	rawEndData   string
	// Task is the description.
	Task    string
	Classes []string
	// ID is the id. HasID is false for task data that has no supported
	// form: the id is then undefined upstream.
	ID    string
	HasID bool
	// PrevTaskID is the id of the task before this one. HasPrevTaskID is
	// false when that id is undefined.
	PrevTaskID    string
	HasPrevTaskID bool

	Active, Done, Crit, Milestone, Vert bool

	Order     float64
	StartTime Date
	EndTime   Date
}

// DB is the gantt db: the module state of ganttDb.js.
type DB struct {
	render.CommonDB
	ctx   *render.Context
	dayjs *dayjs.Factory

	dateFormat      string
	axisFormat      string
	tickInterval    string
	hasTickInterval bool
	todayMarker     string
	includes        []string
	excludes        []string
	links           *js.Map[string, string]
	sections        []string
	currentSection  string
	displayMode     string
	// funs counts the click handlers that bindFunctions would install.
	funs              int
	diagramID         string
	inclusiveEndDates bool
	topAxis           bool
	weekday           string
	weekend           string

	// lastOrder is the serial order of the task in the script.
	lastOrder float64

	// daySteps counts the days that fixTaskDates visited. See maxDaySteps.
	daySteps int

	taskCnt       int
	lastTaskID    string
	hasLastTaskID bool
	rawTasks      []*Task
	// taskDb maps the id of a task to its index in rawTasks. Upstream never
	// clears it. A render has a fresh page in the reference, and a fresh DB
	// here.
	taskDb map[string]int
}

// NewDB returns an empty db: the state after clear().
func NewDB(ctx *render.Context) *DB {
	db := &DB{
		ctx:     ctx,
		dayjs:   dayjs.New(ctx.Location, ctx.Now),
		links:   js.NewMap[string, string](),
		weekday: "sunday",
		weekend: "saturday",
		taskDb:  map[string]int{},
	}
	db.CommonDB.Sanitize = ctx.SanitizeText
	return db
}

// SetDiagramID is setDiagramId.
func (db *DB) SetDiagramID(id string) { db.diagramID = id }

// SetAxisFormat is setAxisFormat.
func (db *DB) SetAxisFormat(txt string) { db.axisFormat = txt }

// GetAxisFormat is getAxisFormat.
func (db *DB) GetAxisFormat() string { return db.axisFormat }

// SetTickInterval is setTickInterval.
func (db *DB) SetTickInterval(txt string) { db.tickInterval, db.hasTickInterval = txt, true }

// GetTickInterval is getTickInterval. The second result is false for
// undefined.
func (db *DB) GetTickInterval() (string, bool) { return db.tickInterval, db.hasTickInterval }

// SetTodayMarker is setTodayMarker.
func (db *DB) SetTodayMarker(txt string) { db.todayMarker = txt }

// GetTodayMarker is getTodayMarker.
func (db *DB) GetTodayMarker() string { return db.todayMarker }

// SetDateFormat is setDateFormat.
func (db *DB) SetDateFormat(txt string) { db.dateFormat = txt }

// EnableInclusiveEndDates is enableInclusiveEndDates.
func (db *DB) EnableInclusiveEndDates() { db.inclusiveEndDates = true }

// EndDatesAreInclusive is endDatesAreInclusive.
func (db *DB) EndDatesAreInclusive() bool { return db.inclusiveEndDates }

// EnableTopAxis is enableTopAxis.
func (db *DB) EnableTopAxis() { db.topAxis = true }

// TopAxisEnabled is topAxisEnabled.
func (db *DB) TopAxisEnabled() bool { return db.topAxis }

// SetDisplayMode is setDisplayMode.
func (db *DB) SetDisplayMode(txt string) { db.displayMode = txt }

// GetDisplayMode is getDisplayMode.
func (db *DB) GetDisplayMode() string { return db.displayMode }

// GetDateFormat is getDateFormat.
func (db *DB) GetDateFormat() string { return db.dateFormat }

var reSpaceOrComma = regexp2.MustCompile(`[\s,]+`, regexp2.ECMAScript)

// splitRegexp is String.prototype.split with a regular expression that has
// no capture group.
func splitRegexp(re *regexp2.Regexp, s string) []string {
	var out []string
	runes := []rune(s)
	last := 0
	m, _ := re.FindRunesMatch(runes)
	for m != nil {
		if m.Length > 0 {
			out = append(out, string(runes[last:m.Index]))
			last = m.Index + m.Length
		}
		m, _ = re.FindNextMatch(m)
	}
	return append(out, string(runes[last:]))
}

func mergeTokens(existing []string, txt string) []string {
	tokens := splitRegexp(reSpaceOrComma, js.ToLowerCase(txt))
	set := js.NewSet(existing...)
	for _, t := range tokens {
		if t != "" {
			set.Add(t)
		}
	}
	return set.Values()
}

// SetIncludes is setIncludes.
func (db *DB) SetIncludes(txt string) { db.includes = mergeTokens(db.includes, txt) }

// GetIncludes is getIncludes.
func (db *DB) GetIncludes() []string { return db.includes }

// SetExcludes is setExcludes.
func (db *DB) SetExcludes(txt string) { db.excludes = mergeTokens(db.excludes, txt) }

// GetExcludes is getExcludes.
func (db *DB) GetExcludes() []string { return db.excludes }

// GetLinks is getLinks.
func (db *DB) GetLinks() *js.Map[string, string] { return db.links }

// AddSection is addSection.
func (db *DB) AddSection(txt string) {
	db.currentSection = txt
	db.sections = append(db.sections, txt)
}

// GetSections is getSections.
func (db *DB) GetSections() []string { return db.sections }

// GetTasks is getTasks. It resolves the dates of the tasks. The error is an
// exception of the upstream code, for example for a date that is not valid.
func (db *DB) GetTasks() ([]*Task, error) {
	allItemsProcessed, err := db.compileTasks()
	if err != nil {
		return nil, err
	}
	const maxDepth = 10
	iterationCount := 0
	for !allItemsProcessed && iterationCount < maxDepth {
		allItemsProcessed, err = db.compileTasks()
		if err != nil {
			return nil, err
		}
		iterationCount++
	}

	return db.rawTasks, nil
}

func includesString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// IsInvalidDate is isInvalidDate.
func (db *DB) IsInvalidDate(date *dayjs.Dayjs, dateFormat string, excludes, includes []string) (bool, error) {
	formattedDate, err := date.Format(js.TrimSpace(dateFormat))
	if err != nil {
		return false, err
	}
	dateOnly, err := date.Format("YYYY-MM-DD")
	if err != nil {
		return false, err
	}

	if includesString(includes, formattedDate) || includesString(includes, dateOnly) {
		return false, nil
	}
	if includesString(excludes, "weekends") &&
		(date.IsoWeekday() == weekendStartDay[db.weekend] ||
			date.IsoWeekday() == weekendStartDay[db.weekend]+1) {
		return true, nil
	}
	dddd, err := date.Format("dddd")
	if err != nil {
		return false, err
	}
	if includesString(excludes, js.ToLowerCase(dddd)) {
		return true, nil
	}
	return includesString(excludes, formattedDate) || includesString(excludes, dateOnly), nil
}

// SetWeekday is setWeekday.
func (db *DB) SetWeekday(txt string) { db.weekday = txt }

// GetWeekday is getWeekday.
func (db *DB) GetWeekday() string { return db.weekday }

// SetWeekend is setWeekend.
func (db *DB) SetWeekend(startDay string) { db.weekend = startDay }

// checkTaskDates is checkTaskDates.
func (db *DB) checkTaskDates(task *Task, dateFormat string, excludes, includes []string) error {
	if len(excludes) == 0 || task.ManualEndTime {
		return nil
	}
	// task.startTime and task.endTime are Date objects here.
	startTime := db.dayjs.FromTime(task.StartTime.T)
	startTime = startTime.Add(1, "d")

	originalEndTime := db.dayjs.FromTime(task.EndTime.T)
	fixedEndTime, renderEndTime, err := db.fixTaskDates(startTime, originalEndTime, dateFormat, excludes, includes)
	if err != nil {
		return err
	}
	task.EndTime = dateOf(fixedEndTime.ToDate())
	task.RenderEndTime = renderEndTime
	return nil
}

// maxDaySteps is the limit of this port for the loops that visit each day
// of a task when the diagram has excludes: the sum of the days that one db
// may visit. Upstream has no limit and visits a range of any length. One
// pass over a task whose dates have years of four digits stays below the
// limit. A visit of this many days takes this port about eight seconds.
const maxDaySteps = 5_000_000

var errTooManyDays = errors.New("gantt: the tasks span more than 5000000 days in a diagram with excludes: this is not supported")

// fixTaskDates is fixTaskDates: it makes the task longer by the days that
// are excluded. It returns the new end time and the end time to render.
func (db *DB) fixTaskDates(startTime, endTime *dayjs.Dayjs, dateFormat string, excludes, includes []string) (*dayjs.Dayjs, Date, error) {
	invalid := false
	renderEndTime := undefinedDate
	maxEndTime := endTime.Add(10000, "d")
	for startTime.ValueOf() <= endTime.ValueOf() {
		if db.daySteps++; db.daySteps > maxDaySteps {
			return nil, undefinedDate, errTooManyDays
		}
		if !invalid {
			renderEndTime = dateOf(endTime.ToDate())
		}
		var err error
		invalid, err = db.IsInvalidDate(startTime, dateFormat, excludes, includes)
		if err != nil {
			return nil, undefinedDate, err
		}
		if invalid {
			endTime = endTime.Add(1, "d")
			if endTime.ValueOf() > maxEndTime.ValueOf() {
				return nil, undefinedDate, errors.New("Failed to find a valid date that was not excluded by `excludes` after 10,000 iterations.")
			}
		}
		startTime = startTime.Add(1, "d")
	}
	return endTime, renderEndTime, nil
}

var (
	reDigits = regexp2.MustCompile(`^\d+$`, regexp2.ECMAScript)
	reAfter  = regexp2.MustCompile(`^after\s+(?<ids>[\d\w- ]+)`, regexp2.ECMAScript)
	reUntil  = regexp2.MustCompile(`^until\s+(?<ids>[\d\w- ]+)`, regexp2.ECMAScript)
	// cspell:disable-next-line
	reDuration = regexp2.MustCompile(`^(\d+(?:\.\d+)?)([Mdhmswy]|ms)$`, regexp2.ECMAScript)
)

func matchString(re *regexp2.Regexp, s string) bool {
	ok, _ := re.MatchString(s)
	return ok
}

// referencedIDs returns the ids of an "after" or "until" statement, or ok
// false when str is not such a statement.
func referencedIDs(re *regexp2.Regexp, str string) (ids []string, ok bool) {
	m, _ := re.FindStringMatch(str)
	if m == nil {
		return nil, false
	}
	for _, id := range strings.Split(m.GroupByName("ids").String(), " ") {
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids, true
}

// today is new Date() with setHours(0, 0, 0, 0): the start of today in
// local time.
func (db *DB) today() Date {
	today := js.NewDate(db.ctx.Location, float64(db.ctx.Now.UnixMilli()))
	today.SetHours(0, 0, 0, 0)
	return dateOf(today.GetTime())
}

// getStartDate is getStartDate(prevTime, dateFormat, str).
func (db *DB) getStartDate(dateFormat, str string) (Date, error) {
	str = js.TrimSpace(str)

	// Handle timestamp formats (x, X) with numeric strings
	if trimmedFormat := js.TrimSpace(dateFormat); (trimmedFormat == "x" || trimmedFormat == "X") && matchString(reDigits, str) {
		return dateOf(js.TimeClip(js.Number(str))), nil
	}
	// Test for after
	if referenced, ok := referencedIDs(reAfter, str); ok {
		// check all after ids and take the latest
		var latestTask *Task
		for _, id := range referenced {
			task := db.FindTaskByID(id)
			if task == nil {
				continue
			}
			if latestTask == nil || task.EndTime.T > latestTask.EndTime.T {
				latestTask = task
			}
		}

		if latestTask != nil {
			return latestTask.EndTime, nil
		}
		return db.today(), nil
	}

	// Check for actual date set using dayjs strict parsing
	mDate, err := db.dayjs.Parse(str, js.TrimSpace(dateFormat), true)
	if err != nil {
		return undefinedDate, err
	}
	if mDate.IsValid() {
		return dateOf(mDate.ToDate()), nil
	}

	// Timestamp formats can fall back to new Date()
	d := js.NewDateString(db.ctx.Location, str)
	// WebKit browsers can mis-parse invalid dates to be ridiculously huge
	// numbers. For the purposes of Gantt charts upstream treats any date
	// beyond 10,000 AD/BC as invalid.
	if math.IsNaN(d.GetTime()) || d.GetFullYear() < -10000 || d.GetFullYear() > 10000 {
		return undefinedDate, errors.New("Invalid date:" + str)
	}
	return dateOf(d.GetTime()), nil
}

// ParseDuration is parseDuration: it parses a string into the arguments for
// dayjs.add(). The value is NaN for a string that is not a duration.
func ParseDuration(str string) (value float64, unit string) {
	m, _ := reDuration.FindStringMatch(js.TrimSpace(str))
	if m != nil {
		return js.ParseFloat(m.GroupByNumber(1).String()), m.GroupByNumber(2).String()
	}
	// NaN means an invalid duration
	return math.NaN(), "ms"
}

// getEndDate is getEndDate(prevTime, dateFormat, str, inclusive).
func (db *DB) getEndDate(prevTime Date, dateFormat, str string, inclusive bool) (Date, error) {
	str = js.TrimSpace(str)

	// test for until
	if referenced, ok := referencedIDs(reUntil, str); ok {
		// check all until ids and take the earliest
		var earliestTask *Task
		for _, id := range referenced {
			task := db.FindTaskByID(id)
			if task == nil {
				continue
			}
			if earliestTask == nil || task.StartTime.T < earliestTask.StartTime.T {
				earliestTask = task
			}
		}

		if earliestTask != nil {
			return earliestTask.StartTime, nil
		}
		return db.today(), nil
	}

	// check for actual date
	parsedDate, err := db.dayjs.Parse(str, js.TrimSpace(dateFormat), true)
	if err != nil {
		return undefinedDate, err
	}
	if parsedDate.IsValid() {
		if inclusive {
			parsedDate = parsedDate.Add(1, "d")
		}
		return dateOf(parsedDate.ToDate()), nil
	}

	endTime := db.dayjs.FromTime(prevTime.T)
	durationValue, durationUnit := ParseDuration(str)
	if !math.IsNaN(durationValue) {
		newEndTime := endTime.Add(durationValue, durationUnit)
		if newEndTime.IsValid() {
			endTime = newEndTime
		}
	}
	return dateOf(endTime.ToDate()), nil
}

// parseID is parseId.
func (db *DB) parseID(idStr string, defined bool) string {
	if !defined {
		db.taskCnt = db.taskCnt + 1
		return "task" + js.Str(float64(db.taskCnt))
	}
	return idStr
}

// taskInfo is the object that parseData returns.
type taskInfo struct {
	id                                  string
	hasID                               bool
	startTime                           rawStartTime
	endData                             string
	active, done, crit, milestone, vert bool
}

// parseData is parseData(prevTaskId, dataStr). The forms of the data are:
//
//	id, startDate, endDate
//	id, startDate, length
//	id, after x, endDate
//	id, after x, length
//	startDate, endDate
//	startDate, length
//	after x, endDate
//	after x, length
//	endDate
//	length
func (db *DB) parseData(dataStr string) (*taskInfo, error) {
	ds := dataStr
	if js.Substr(dataStr, 0, 1) == ":" {
		ds = js.Substr(dataStr, 1, js.UTF16Len(dataStr))
	}

	data := strings.Split(ds, ",")

	task := &taskInfo{}

	// Get tags like active, done, crit, milestone, and vert
	data, err := getTaskTags(data, task)
	if err != nil {
		return nil, err
	}

	for i := range data {
		data[i] = js.TrimSpace(data[i])
	}

	switch len(data) {
	case 1:
		task.id, task.hasID = db.parseID("", false), true
		task.startTime = rawStartTime{typ: "prevTaskEnd"}
		task.endData = data[0]
	case 2:
		task.id, task.hasID = db.parseID("", false), true
		task.startTime = rawStartTime{typ: "getStartDate", startData: data[0]}
		task.endData = data[1]
	case 3:
		task.id, task.hasID = db.parseID(data[0], true), true
		task.startTime = rawStartTime{typ: "getStartDate", startData: data[1]}
		task.endData = data[2]
	default:
	}

	return task, nil
}

// idKey is the property name of a task id in taskDb. An id that is
// undefined becomes the name "undefined", as in JavaScript.
func idKey(id string, defined bool) string {
	if !defined {
		return "undefined"
	}
	return id
}

// AddTask is addTask(descr, data). The error is the TypeError that upstream
// throws for task data that has only tags.
func (db *DB) AddTask(descr, data string) error {
	rawTask := &Task{
		Section:       db.currentSection,
		Type:          db.currentSection,
		Processed:     false,
		ManualEndTime: false,
		RenderEndTime: undefinedDate,
		Raw:           data,
		Task:          descr,
		Classes:       []string{},
		StartTime:     undefinedDate,
		EndTime:       undefinedDate,
	}
	taskInfo, err := db.parseData(data)
	if err != nil {
		return err
	}
	rawTask.rawStartTime = taskInfo.startTime
	rawTask.rawEndData = taskInfo.endData
	rawTask.ID, rawTask.HasID = taskInfo.id, taskInfo.hasID
	rawTask.PrevTaskID, rawTask.HasPrevTaskID = db.lastTaskID, db.hasLastTaskID
	rawTask.Active = taskInfo.active
	rawTask.Done = taskInfo.done
	rawTask.Crit = taskInfo.crit
	rawTask.Milestone = taskInfo.milestone
	rawTask.Vert = taskInfo.vert

	if rawTask.Vert {
		rawTask.Order = -1
	} else {
		rawTask.Order = db.lastOrder
		db.lastOrder++
	}

	db.rawTasks = append(db.rawTasks, rawTask)
	pos := len(db.rawTasks)

	db.lastTaskID, db.hasLastTaskID = rawTask.ID, rawTask.HasID
	// Store cross ref. taskDb is a plain object upstream: an assignment to
	// the property __proto__ of it does nothing.
	if key := idKey(rawTask.ID, rawTask.HasID); key != "__proto__" {
		db.taskDb[key] = pos - 1
	}
	return nil
}

// FindTaskByID is findTaskById. It returns nil for undefined.
func (db *DB) FindTaskByID(id string) *Task {
	return db.findTask(id, true)
}

func (db *DB) findTask(id string, defined bool) *Task {
	pos, ok := db.taskDb[idKey(id, defined)]
	if !ok || pos < 0 || pos >= len(db.rawTasks) {
		return nil
	}
	return db.rawTasks[pos]
}

// manualEndTimeFormat is the format of an end date that the excludes do
// not move.
const manualEndTimeFormat = "YYYY-MM-DD"

// compileTasks is compileTasks: one pass over the tasks. It reports
// whether all tasks have dates.
func (db *DB) compileTasks() (bool, error) {
	dateFormat := db.dateFormat
	compileTask := func(task *Task) error {
		switch task.rawStartTime.typ {
		case "prevTaskEnd":
			prevTask := db.findTask(task.PrevTaskID, task.HasPrevTaskID)
			if prevTask == nil {
				return &typeError{"Cannot read properties of undefined (reading 'endTime')"}
			}
			task.StartTime = prevTask.EndTime
		case "getStartDate":
			startTime, err := db.getStartDate(dateFormat, task.rawStartTime.startData)
			if err != nil {
				return err
			}
			if startTime.Defined {
				task.StartTime = startTime
			}
		default:
			// The task has no raw.startTime object.
			return &typeError{"Cannot read properties of undefined (reading 'type')"}
		}

		if task.StartTime.Defined {
			endTime, err := db.getEndDate(task.StartTime, dateFormat, task.rawEndData, db.inclusiveEndDates)
			if err != nil {
				return err
			}
			task.EndTime = endTime
			if task.EndTime.Defined {
				task.Processed = true
				manual, err := db.dayjs.Parse(task.rawEndData, manualEndTimeFormat, true)
				if err != nil {
					return err
				}
				task.ManualEndTime = manual.IsValid()
				if err := db.checkTaskDates(task, dateFormat, db.excludes, db.includes); err != nil {
					return err
				}
			}
		}
		return nil
	}

	allProcessed := true
	for _, rawTask := range db.rawTasks {
		if err := compileTask(rawTask); err != nil {
			return false, err
		}

		allProcessed = allProcessed && rawTask.Processed
	}
	return allProcessed, nil
}

// SetLink is setLink: the parser calls it when it finds a link. ids is a
// list of ids with commas between them.
func (db *DB) SetLink(ids, linkStrIn string) {
	linkStr := linkStrIn
	if db.ctx.CfgStr("securityLevel") != "loose" {
		linkStr = sanitize.SanitizeUrl(linkStrIn)
	}
	for _, id := range strings.Split(ids, ",") {
		rawTask := db.FindTaskByID(id)
		if rawTask != nil {
			db.pushFun()
			db.links.Set(id, linkStr)
		}
	}
	db.SetClass(ids, "clickable")
}

// SetClass is setClass: the parser calls it for a special node, for
// example a node with a click handler.
func (db *DB) SetClass(ids, className string) {
	for _, id := range strings.Split(ids, ",") {
		rawTask := db.FindTaskByID(id)
		if rawTask != nil {
			rawTask.Classes = append(rawTask.Classes, className)
		}
	}
}

// setClickFun is setClickFun. The function that it registers runs on a
// click in a browser: it calls the function with the name functionName. It
// has no effect on the SVG, so this port only counts it.
func (db *DB) setClickFun(id string) {
	if db.ctx.CfgStr("securityLevel") != "loose" {
		return
	}

	rawTask := db.FindTaskByID(id)
	if rawTask != nil {
		db.pushFun()
	}
}

// pushFun is pushFun: upstream adds two functions that bind the click
// handler to the task and to its text.
func (db *DB) pushFun() { db.funs += 2 }

// SetClickEvent is setClickEvent(ids, functionName, functionArgs): the
// parser calls it for a click definition. The name and the arguments of the
// callback are not arguments here, because they have no effect on the SVG.
func (db *DB) SetClickEvent(ids string) {
	for _, id := range strings.Split(ids, ",") {
		db.setClickFun(id)
	}
	db.SetClass(ids, "clickable")
}

var reTags = func() []*regexp2.Regexp {
	out := make([]*regexp2.Regexp, len(tags))
	for i, t := range tags {
		out[i] = regexp2.MustCompile(`^\s*`+t+`\s*$`, regexp2.ECMAScript)
	}
	return out
}()

// getTaskTags is getTaskTags: it removes the tags at the start of the data
// and sets them on the task. The error is the TypeError that upstream
// throws when the tags use up all of the data.
func getTaskTags(data []string, task *taskInfo) ([]string, error) {
	matchFound := true
	for matchFound {
		matchFound = false
		for i, t := range tags {
			if len(data) == 0 {
				return nil, &typeError{"Cannot read properties of undefined (reading 'match')"}
			}
			if matchString(reTags[i], data[0]) {
				switch t {
				case "active":
					task.active = true
				case "done":
					task.done = true
				case "crit":
					task.crit = true
				case "milestone":
					task.milestone = true
				case "vert":
					task.vert = true
				}
				data = data[1:]
				matchFound = true
			}
		}
	}
	return data, nil
}
