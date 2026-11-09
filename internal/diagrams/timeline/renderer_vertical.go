// Ported from mermaid 12.2.0, diagrams/timeline/timelineRendererVertical.ts
// (MIT).

package timeline

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

const (
	nodeWidth       = 200.0
	nodePadding     = 5.0
	nodeTotalWidth  = nodeWidth + nodePadding*2
	eventWidth      = nodeWidth + 100
	eventTotalWidth = eventWidth + nodePadding*2
	eventSpacing    = 10.0
	eventVertGap    = 0.0
	sectionTaskGap  = 20.0
	taskAxisGap     = 20.0
	taskVerticalGap = 30.0
	eventAxisGap    = 50.0
)

// drawVertical is draw(text, id, version, diagObj) of
// timelineRendererVertical.ts.
func (r *renderer) drawVertical(db *DB) error {
	// 1. Fetch the configuration.
	conf := r.conf
	leftMargin := nullishOr(js.Path(conf, "timeline", "leftMargin"), 50)

	svg := r.ctx.SelectSvgElement()

	svg.Append("g")

	// 4. Fetch the diagram data.
	tasks := db.GetTasks()
	title := db.GetCommonDb().GetDiagramTitle()

	// 5. Initialize the diagram. Upstream gives no id here.
	r.svgDraw.initGraphics(svg, undefinedID)

	// Fetch the sections.
	sections := db.GetSections()

	maxSectionHeight := 0.0
	maxTaskHeight := 0.0
	masterX := 50 + leftMargin
	masterY := 50.0
	contentTopY := masterY
	sectionBeginX := masterX
	const leftWidth = nodeTotalWidth + taskAxisGap
	const rightWidth = eventTotalWidth + eventAxisGap
	axisX := sectionBeginX + leftWidth
	sectionNumber := 0.0
	hasSections := len(sections) > 0
	timelineX := masterX + leftWidth
	if hasSections {
		timelineX = axisX
	}

	sectionWidth := js.Max(50, leftWidth+rightWidth-nodePadding*2)
	// Calculate the max height of the sections.
	for _, section := range sections {
		sectionNode := &block{
			descr:     section,
			section:   sectionNumber,
			width:     sectionWidth,
			padding:   nodePadding,
			maxHeight: maxSectionHeight,
		}
		sectionHeight := r.svgDraw.getVirtualNodeHeight(svg, sectionNode, conf)
		maxSectionHeight = js.Max(maxSectionHeight, sectionHeight)
	}

	// The tasks length and the max event stack height.
	maxEventStackHeight := 0.0

	for _, task := range tasks {
		taskNode := &block{
			descr:     objectText,
			width:     nodeWidth,
			padding:   nodePadding,
			maxHeight: maxTaskHeight,
		}
		taskHeight := r.svgDraw.getVirtualNodeHeight(svg, taskNode, conf)
		maxTaskHeight = js.Max(maxTaskHeight, taskHeight)

		// Calculate maxEventStackHeight (the vertical stack height for events).
		maxEventStackHeightTemp := 0.0
		for _, event := range task.Events {
			eventNode := &block{
				descr:     event,
				width:     eventWidth,
				padding:   nodePadding,
				maxHeight: 50,
			}
			maxEventStackHeightTemp += r.svgDraw.getVirtualNodeHeight(svg, eventNode, conf)
		}
		if len(task.Events) > 0 {
			maxEventStackHeightTemp += float64(float64(len(task.Events)-1) * eventSpacing)
		}
		maxEventStackHeight = js.Max(maxEventStackHeight, maxEventStackHeightTemp) + eventVertGap
	}

	taskBlockHeight := js.Max(maxTaskHeight, maxEventStackHeight)
	taskSpacing := taskBlockHeight + taskVerticalGap

	if hasSections {
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
				width:     sectionWidth,
				padding:   nodePadding,
				maxHeight: maxSectionHeight,
			}
			sectionNodeWrapper := svg.Append("g")
			node, err := r.svgDraw.drawNode(sectionNodeWrapper, sectionNode, sectionNumber, conf, undefinedID, false)
			if err != nil {
				return err
			}

			sectionX := timelineX - leftWidth
			sectionNodeWrapper.Attr("transform", "translate("+js.Str(sectionX)+", "+js.Str(masterY)+")")

			taskStartY := masterY + node.height + sectionTaskGap
			// Draw the tasks for this section.
			if len(tasksForSection) > 0 {
				if err := r.drawTasksVertical(svg, tasksForSection, sectionNumber, timelineX, taskStartY, maxTaskHeight, taskSpacing, false); err != nil {
					return err
				}
			}

			taskCount := float64(len(tasksForSection))
			gaps := 0.0
			if taskCount > 0 {
				gaps = taskVerticalGap * 2
			}
			sectionHeight := node.height +
				sectionTaskGap +
				float64(taskSpacing*js.Max(taskCount, 1)) -
				gaps
			masterY += sectionHeight
			sectionNumber++
		}
	} else {
		// Draw the tasks.
		if err := r.drawTasksVertical(svg, tasks, sectionNumber, timelineX, masterY, maxTaskHeight, taskSpacing, true); err != nil {
			return err
		}
	}

	// Get the BBox of the diagram (before drawing the axis line).
	box := svg.BBox()

	if title != "" {
		svg.
			Append("text").
			Text(title).
			Attr("x", float64(box.Width/2)-leftMargin).
			Attr("font-size", "4ex").
			Attr("font-weight", "bold").
			Attr("y", 20)
		box = svg.BBox()
	}

	fontSize, _, ok := utils.ParseFontSize(conf.Get("fontSize"))
	if !ok {
		fontSize = 16
	}
	arrowTopOffset := float64(fontSize * 2)
	arrowBottomPadding := float64(fontSize*0.5) + 20

	lineWrapper := svg.Append("g").Attr("class", "lineWrapper")
	// Draw the activity line.
	lineWrapper.
		Append("line").
		Attr("x1", timelineX).
		Attr("y1", contentTopY-arrowTopOffset).
		Attr("x2", timelineX).
		Attr("y2", box.Y+box.Height+arrowBottomPadding).
		Attr("stroke-width", 4).
		Attr("stroke", "black").
		Attr("marker-end", "url(#arrowhead)")
	lineWrapper.Lower()

	// Set up the view box and the size of the svg element.
	render.SetupGraphViewbox(
		svg,
		nullishOr(js.Path(conf, "timeline", "padding"), 50),
		js.Truthy(js.Path(conf, "timeline", "useMaxWidth")),
	)
	return nil
}

// drawTasksVertical is drawTasks of timelineRendererVertical.ts.
func (r *renderer) drawTasksVertical(
	diagram *dom.Node,
	tasks []*Task,
	sectionColor float64,
	timelineX float64,
	masterY float64,
	maxTaskHeight float64,
	taskSpacing float64,
	isWithoutSections bool,
) error {
	conf := r.conf
	// Draw the tasks.
	for _, task := range tasks {
		// Create a node from the task.
		taskNode := &block{
			descr:     task.Task,
			section:   sectionColor,
			width:     nodeWidth,
			padding:   nodePadding,
			maxHeight: maxTaskHeight,
		}

		// Create the task wrapper.
		taskWrapper := diagram.Append("g").Attr("class", "taskWrapper")
		node, err := r.svgDraw.drawNode(taskWrapper, taskNode, sectionColor, conf, undefinedID, false)
		if err != nil {
			return err
		}
		taskHeight := node.height
		taskX := timelineX - taskAxisGap - node.width
		taskWrapper.Attr("transform", "translate("+js.Str(taskX)+", "+js.Str(masterY)+")")

		// Update the max task height.
		maxTaskHeight = js.Max(maxTaskHeight, taskHeight)

		// If the task has events, draw them.
		if len(task.Events) > 0 {
			eventsStartY := masterY
			eventsX := timelineX + eventAxisGap
			if err := r.drawEventsVertical(diagram, task.Events, sectionColor, timelineX, eventsX, eventsStartY); err != nil {
				return err
			}
		}

		masterY = masterY + taskSpacing
		if isWithoutSections && !js.Truthy(js.Path(conf, "timeline", "disableMulticolor")) {
			sectionColor++
		}
	}
	return nil
}

// drawEventsVertical is drawEvents of timelineRendererVertical.ts. Upstream
// returns the height of the events, and no caller reads it.
func (r *renderer) drawEventsVertical(
	diagram *dom.Node,
	events []string,
	sectionColor float64,
	axisX float64,
	eventsX float64,
	startY float64,
) error {
	currentY := startY
	// Draw the events.
	for _, event := range events {
		// Create a node from the event.
		eventNode := &block{
			descr:     event,
			section:   sectionColor,
			width:     eventWidth,
			padding:   nodePadding,
			maxHeight: 0,
		}

		// Create the event wrapper.
		eventWrapper := diagram.Append("g").Attr("class", "eventWrapper")
		node, err := r.svgDraw.drawNode(eventWrapper, eventNode, sectionColor, r.conf, undefinedID, false)
		if err != nil {
			return err
		}
		eventHeight := node.height
		eventWrapper.Attr("transform", "translate("+js.Str(eventsX)+", "+js.Str(currentY)+")")

		lineWrapper := diagram.Append("g").Attr("class", "lineWrapper")
		lineY := currentY + float64(eventHeight/2)
		lineWrapper.
			Append("line").
			Attr("x1", axisX).
			Attr("y1", lineY).
			Attr("x2", eventsX).
			Attr("y2", lineY).
			Attr("stroke-width", 2).
			Attr("stroke", "black").
			Attr("marker-end", "url(#arrowhead)").
			Attr("stroke-dasharray", "5,5")

		currentY = currentY + eventHeight + eventSpacing
	}
	return nil
}
