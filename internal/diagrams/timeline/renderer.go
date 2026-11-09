// Ported from mermaid 12.2.0, diagrams/timeline/timelineRenderer.ts (MIT).

package timeline

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// objectText is the text that selection.text(task) writes for a task
// object. The renderers measure the height of a task with the task object
// as the description, and String(task) is this text.
const objectText = "[object Object]"

// nullishOr is value ?? def for a numeric config value.
func nullishOr(value any, def float64) float64 {
	if js.IsNullish(value) {
		return def
	}
	return js.ToNumber(value)
}

// attrValue converts a config value for selection.attr(name, value): null
// and undefined remove the attribute.
func attrValue(value any) any {
	if js.IsNullish(value) {
		return nil
	}
	return value
}

// renderer holds the arguments that every function of a draw shares.
type renderer struct {
	ctx     *render.Context
	conf    *js.Obj[any]
	svgDraw svgDraw
}

// draw is draw(text, id, version, diagObj) of timelineRenderer.ts.
func (r *renderer) draw(db *DB) error {
	// 1. Fetch the configuration.
	conf := r.conf
	id := r.ctx.ID
	look, theme, themeVariables := conf.Get("look"), conf.Get("theme"), conf.Get("themeVariables")
	useGradient := js.Path(themeVariables, "useGradient")
	gradientStart := js.Path(themeVariables, "gradientStart")
	gradientStop := js.Path(themeVariables, "gradientStop")
	leftMargin := nullishOr(js.Path(conf, "timeline", "leftMargin"), 50)

	svg := r.ctx.Body.Select("#" + id)

	svg.Append("g")

	// 4. Fetch the diagram data.
	tasks := db.GetTasks()
	title := db.GetCommonDb().GetDiagramTitle()

	// 5. Initialize the diagram.
	r.svgDraw.initGraphics(svg, id)

	// Fetch the sections.
	sections := db.GetSections()

	maxSectionHeight := 0.0
	maxTaskHeight := 0.0
	depthY := 0.0
	sectionBeginY := 0.0
	masterX := 50 + leftMargin
	masterY := 50.0
	sectionBeginY = 50
	// Draw the sections.
	sectionNumber := 0.0
	hasSections := true

	// Calculate the max height of the sections.
	for _, section := range sections {
		sectionNode := &block{
			descr:     section,
			section:   sectionNumber,
			width:     150,
			padding:   20,
			maxHeight: maxSectionHeight,
		}
		sectionHeight := r.svgDraw.getVirtualNodeHeight(svg, sectionNode, conf)
		maxSectionHeight = js.Max(maxSectionHeight, sectionHeight+20)
	}

	// Upstream also keeps maxEventCount here. Nothing reads it.
	maxEventLineLength := 0.0
	// Calculate the max task height.
	for _, task := range tasks {
		taskNode := &block{
			descr:     objectText,
			width:     150,
			padding:   20,
			maxHeight: maxTaskHeight,
		}
		taskHeight := r.svgDraw.getVirtualNodeHeight(svg, taskNode, conf)
		maxTaskHeight = js.Max(maxTaskHeight, taskHeight+20)

		// Calculate maxEventLineLength.
		maxEventLineLengthTemp := 0.0
		for _, event := range task.Events {
			eventNode := &block{
				descr:     event,
				width:     150,
				padding:   20,
				maxHeight: 50,
			}
			maxEventLineLengthTemp += r.svgDraw.getVirtualNodeHeight(svg, eventNode, conf)
		}
		// Add spacing between events (10px per event except the last one).
		if len(task.Events) > 0 {
			maxEventLineLengthTemp += float64((len(task.Events) - 1) * 10)
		}
		maxEventLineLength = js.Max(maxEventLineLength, maxEventLineLengthTemp)
	}

	if len(sections) > 0 {
		for _, section := range sections {
			// Filter the tasks where task.section == section.
			var tasksForSection []*Task
			for _, task := range tasks {
				if task.Section == section {
					tasksForSection = append(tasksForSection, task)
				}
			}

			sectionNode := &block{
				descr:     section,
				section:   sectionNumber,
				width:     float64(200*js.Max(float64(len(tasksForSection)), 1)) - 50,
				padding:   20,
				maxHeight: maxSectionHeight,
			}
			sectionNodeWrapper := svg.Append("g")
			if _, err := r.svgDraw.drawNode(sectionNodeWrapper, sectionNode, sectionNumber, conf, id, false); err != nil {
				return err
			}

			sectionNodeWrapper.Attr("transform", "translate("+js.Str(masterX)+", "+js.Str(sectionBeginY)+")")

			masterY += maxSectionHeight + 50

			// Draw the tasks for this section.
			if len(tasksForSection) > 0 {
				if err := r.drawTasks(svg, tasksForSection, sectionNumber, masterX, masterY, maxTaskHeight, maxEventLineLength, false, id); err != nil {
					return err
				}
			}
			// todo replace with total width of section and its tasks
			masterX += float64(200 * js.Max(float64(len(tasksForSection)), 1))

			masterY = sectionBeginY
			sectionNumber++
		}
	} else {
		// Draw the tasks.
		hasSections = false
		if err := r.drawTasks(svg, tasks, sectionNumber, masterX, masterY, maxTaskHeight, maxEventLineLength, true, id); err != nil {
			return err
		}
	}

	// Get the BBox of the diagram.
	box := svg.BBox()

	if title != "" {
		var x float64
		if look == "neo" {
			x = float64(box.X*2) + leftMargin
		} else {
			x = float64(box.Width/2) - leftMargin
		}
		svg.
			Append("text").
			Text(title).
			Attr("x", x).
			Attr("font-size", "4ex").
			Attr("font-weight", "bold").
			Attr("y", 20)
	}
	// 5. Draw the diagram.
	if hasSections {
		depthY = maxSectionHeight + maxTaskHeight + 150
	} else {
		depthY = maxTaskHeight + 100
	}

	lineWrapper := svg.Append("g").Attr("class", "lineWrapper")
	// Draw the activity line.
	lineWrapper.
		Append("line").
		Attr("x1", leftMargin).
		Attr("y1", depthY). // One section head + one task + margins.
		Attr("x2", box.Width+float64(3*leftMargin)).
		Attr("y2", depthY).
		Attr("stroke-width", 4).
		Attr("stroke", "black").
		Attr("marker-end", "url(#"+id+"-arrowhead)")

	// Do not apply the gradient for the neutral theme: it keeps its
	// grayscale color scheme.
	if look == "neo" && js.Truthy(useGradient) && theme != "neutral" {
		defsEl := svg.Select("defs")
		if defsEl == nil {
			defsEl = svg.Append("defs")
		}
		gradient := defsEl.
			Append("linearGradient").
			Attr("id", svg.AttrOr("id", "null")+"-gradient").
			Attr("gradientUnits", "objectBoundingBox").
			Attr("x1", "0%").
			Attr("y1", "0%").
			Attr("x2", "100%").
			Attr("y2", "0%")

		gradient.
			Append("stop").
			Attr("offset", "0%").
			Attr("stop-color", attrValue(gradientStart)).
			Attr("stop-opacity", 1)

		gradient.
			Append("stop").
			Attr("offset", "100%").
			Attr("stop-color", attrValue(gradientStop)).
			Attr("stop-opacity", 1)
	}

	// Set up the view box and the size of the svg element.
	render.SetupGraphViewbox(
		svg,
		nullishOr(js.Path(conf, "timeline", "padding"), 50),
		js.Truthy(js.Path(conf, "timeline", "useMaxWidth")),
	)
	return nil
}

// drawTasks is drawTasks of timelineRenderer.ts. The arguments
// maxEventCount and maxSectionHeight of upstream are not here: the function
// does not read them.
func (r *renderer) drawTasks(
	diagram *dom.Node,
	tasks []*Task,
	sectionColor float64,
	masterX float64,
	masterY float64,
	maxTaskHeight float64,
	maxEventLineLength float64,
	isWithoutSections bool,
	diagramID string,
) error {
	conf := r.conf
	// Draw the tasks.
	for _, task := range tasks {
		// Create a node from the task.
		taskNode := &block{
			descr:     task.Task,
			section:   sectionColor,
			width:     150,
			padding:   20,
			maxHeight: maxTaskHeight,
		}

		// Create the task wrapper.
		taskWrapper := diagram.Append("g").Attr("class", "taskWrapper")
		node, err := r.svgDraw.drawNode(taskWrapper, taskNode, sectionColor, conf, diagramID, false)
		if err != nil {
			return err
		}
		taskHeight := node.height
		taskWrapper.Attr("transform", "translate("+js.Str(masterX)+", "+js.Str(masterY)+")")

		// Update the max task height.
		maxTaskHeight = js.Max(maxTaskHeight, taskHeight)

		// If the task has events, draw them.
		if task.Events != nil {
			// Draw a line between the task and the events.
			lineWrapper := diagram.Append("g").Attr("class", "lineWrapper")
			// Add a margin to the task. Upstream adds the result of
			// drawEvents to a lineLength variable that nothing reads.
			masterY += 100
			if _, err := r.drawEvents(diagram, task.Events, sectionColor, masterX, masterY, diagramID); err != nil {
				return err
			}
			masterY -= 100

			lineWrapper.
				Append("line").
				Attr("x1", masterX+190.0/2).
				Attr("y1", masterY+maxTaskHeight). // Start from the bottom of the task box.
				Attr("x2", masterX+190.0/2).       // The same x coordinate for a vertical line.
				Attr("y2", masterY+maxTaskHeight+100+maxEventLineLength+100).
				Attr("stroke-width", 2).
				Attr("stroke", "black").
				Attr("marker-end", "url(#"+diagramID+"-arrowhead)").
				Attr("stroke-dasharray", "5,5")
		}

		masterX = masterX + 200
		if isWithoutSections && !js.Truthy(js.Path(conf, "timeline", "disableMulticolor")) {
			sectionColor++
		}
	}
	return nil
}

// drawEvents is drawEvents of timelineRenderer.ts.
func (r *renderer) drawEvents(
	diagram *dom.Node,
	events []string,
	sectionColor float64,
	masterX float64,
	masterY float64,
	diagramID string,
) (float64, error) {
	maxEventHeight := 0.0
	masterY = masterY + 100
	// Draw the events.
	for _, event := range events {
		// Create a node from the event.
		eventNode := &block{
			descr:     event,
			section:   sectionColor,
			width:     150,
			padding:   20,
			maxHeight: 50,
		}

		// Create the event wrapper.
		eventWrapper := diagram.Append("g").Attr("class", "eventWrapper")
		node, err := r.svgDraw.drawNode(eventWrapper, eventNode, sectionColor, r.conf, diagramID, true)
		if err != nil {
			return 0, err
		}
		eventHeight := node.height
		maxEventHeight = maxEventHeight + eventHeight
		eventWrapper.Attr("transform", "translate("+js.Str(masterX)+", "+js.Str(masterY)+")")
		masterY = masterY + 10 + eventHeight
	}
	return maxEventHeight, nil
}
