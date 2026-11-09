// Ported from mermaid 12.2.0, diagrams/timeline/timelineDb.js (MIT).

package timeline

import (
	"errors"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// Task is one element of the task list: a time period with its events.
type Task struct {
	// ID is the number of the raw task. HasID is false for a task that
	// addTaskOrg made: such a task has no id.
	ID    int
	HasID bool
	// Section and Type are the section that was open when the task was added.
	Section string
	Type    string
	// Task is the text of the time period.
	Task string
	// Score is the length argument of addTask, or 0.
	Score float64
	// Events holds the texts of the events of the period.
	Events []string
	// Description and Classes are set by addTaskOrg only.
	Description string
	Classes     []string

	// processed stands in for rawTask.processed. Upstream never sets it.
	processed bool
}

// DB is the state of timelineDb.js. Upstream keeps it in module variables.
type DB struct {
	// common is commonDb. The timeline db gives it out with getCommonDb and
	// has no getAccTitle, getAccDescription or setDiagramTitle of its own.
	// So the pipeline does not see the accessible title, the accessible
	// description or the front matter title of a timeline.
	common render.CommonDB

	currentSection string
	currentTaskID  int
	direction      string

	sections []string
	tasks    []*Task
	rawTasks []*Task
}

// newDB returns the state after clear() on a page that has just loaded.
func newDB() *DB {
	return &DB{direction: "LR"}
}

// GetCommonDb is getCommonDb().
func (d *DB) GetCommonDb() *render.CommonDB { return &d.common }

// Clear is clear(). It keeps currentTaskId, as upstream does.
func (d *DB) Clear() {
	d.sections = d.sections[:0]
	d.tasks = d.tasks[:0]
	d.currentSection = ""
	d.rawTasks = d.rawTasks[:0]
	d.direction = "LR"
	d.common.ClearCommon()
}

// SetDirection is setDirection(dir).
func (d *DB) SetDirection(dir string) { d.direction = dir }

// GetDirection is getDirection().
func (d *DB) GetDirection() string { return d.direction }

// AddSection is addSection(txt).
func (d *DB) AddSection(txt string) {
	d.currentSection = txt
	d.sections = append(d.sections, txt)
}

// GetSections is getSections().
func (d *DB) GetSections() []string { return d.sections }

// GetTasks is getTasks(). Each call appends the raw tasks to the task list
// again, as upstream does.
func (d *DB) GetTasks() []*Task {
	allItemsProcessed := d.compileTasks()
	const maxDepth = 100
	iterationCount := 0
	for !allItemsProcessed && iterationCount < maxDepth {
		allItemsProcessed = d.compileTasks()
		iterationCount++
	}

	d.tasks = append(d.tasks, d.rawTasks...)

	return d.tasks
}

// AddTask is addTask(period, length, event).
func (d *DB) AddTask(period string, length float64, event string) {
	rawTask := &Task{
		ID:      d.currentTaskID,
		HasID:   true,
		Section: d.currentSection,
		Type:    d.currentSection,
		Task:    period,
		Events:  []string{},
	}
	d.currentTaskID++
	// length ? length : 0. A NaN length is falsy.
	if length == length && length != 0 {
		rawTask.Score = length
	}
	// If event is defined, then add it to the events array.
	if event != "" {
		rawTask.Events = append(rawTask.Events, event)
	}
	d.rawTasks = append(d.rawTasks, rawTask)
}

// errNoCurrentTask is the TypeError of upstream for an event that comes
// before the first time period.
var errNoCurrentTask = errors.New("Cannot read properties of undefined (reading 'events')")

// AddEvent is addEvent(event).
func (d *DB) AddEvent(event string) error {
	// Fetch the current task with currentTaskId.
	var currentTask *Task
	for _, task := range d.rawTasks {
		if task.HasID && task.ID == d.currentTaskID-1 {
			currentTask = task
			break
		}
	}
	if currentTask == nil {
		return errNoCurrentTask
	}
	// Add the event to the events array.
	currentTask.Events = append(currentTask.Events, event)
	return nil
}

// AddTaskOrg is addTaskOrg(descr).
func (d *DB) AddTaskOrg(descr string) {
	newTask := &Task{
		Section:     d.currentSection,
		Type:        d.currentSection,
		Description: descr,
		Task:        descr,
		Classes:     []string{},
	}
	d.tasks = append(d.tasks, newTask)
}

// compileTasks compiles the raw tasks into a list of tasks with events. It
// returns true when all items are processed.
func (d *DB) compileTasks() bool {
	compileTask := func(pos int) bool {
		return d.rawTasks[pos].processed
	}

	allProcessed := true
	for i, rawTask := range d.rawTasks {
		compileTask(i)

		allProcessed = allProcessed && rawTask.processed
	}
	return allProcessed
}
