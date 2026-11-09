// Ported from mermaid 12.2.0, diagrams/user-journey/journeyDb.js (MIT).

package journey

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// Task is one element of the task list.
type Task struct {
	// Section and Type are the section that was open when the task was added.
	Section string
	Type    string
	// People holds the names of the actors of the task. It is nil for a
	// task that addTaskOrg made: such a task has no people property.
	People []string
	// Task is the text of the task.
	Task string
	// Score is the score of the task. It is NaN for a score that is not a
	// number.
	Score float64
	// Description and Classes are set by addTaskOrg only.
	Description string
	Classes     []string

	// processed stands in for rawTask.processed. Upstream never sets it.
	processed bool

	// The renderer adds these properties to the task object.
	x, y          float64
	width, height float64
	colour, fill  any
	num           float64
	actors        actorTable
}

// DB is the state of journeyDb.js. Upstream keeps it in module variables.
// The embedded CommonDB gives setDiagramTitle, getDiagramTitle, setAccTitle,
// getAccTitle, setAccDescription and getAccDescription.
type DB struct {
	render.CommonDB

	currentSection string

	sections []string
	tasks    []*Task
	rawTasks []*Task
}

// Clear is clear().
func (d *DB) Clear() {
	d.sections = d.sections[:0]
	d.tasks = d.tasks[:0]
	d.currentSection = ""
	d.rawTasks = d.rawTasks[:0]
	d.ClearCommon()
}

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

// updateActors returns the sorted names of the actors of the task list.
func (d *DB) updateActors() []string {
	var tempActors []string
	for _, task := range d.tasks {
		if task.People != nil {
			tempActors = append(tempActors, task.People...)
		}
	}

	unique := js.NewSet(tempActors...)
	actors := unique.Values()
	js.SortStrings(actors)
	return actors
}

// AddTask is addTask(descr, taskData).
func (d *DB) AddTask(descr, taskData string) {
	pieces := strings.Split(js.StringSliceFrom(taskData, 1), ":")

	var score float64
	peeps := []string{}
	if len(pieces) == 1 {
		score = js.Number(pieces[0])
	} else {
		score = js.Number(pieces[0])
		peeps = strings.Split(pieces[1], ",")
	}
	peopleList := make([]string, len(peeps))
	for i, s := range peeps {
		peopleList[i] = js.TrimSpace(s)
	}

	rawTask := &Task{
		Section: d.currentSection,
		Type:    d.currentSection,
		People:  peopleList,
		Task:    descr,
		Score:   score,
	}

	d.rawTasks = append(d.rawTasks, rawTask)
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

// compileTasks returns true when all raw tasks are processed.
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

// GetActors is getActors().
func (d *DB) GetActors() []string {
	return d.updateActors()
}
