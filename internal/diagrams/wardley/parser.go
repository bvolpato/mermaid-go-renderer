// Ported from mermaid 12.2.0, diagrams/wardley/wardleyParser.ts (MIT).

package wardley

import (
	"errors"
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// toPercent is toPercent(value, context).
func toPercent(value float64, context string) (float64, error) {
	// Accept values in 0-1 range (converted to percentage) or 0-100 range (used as-is)
	normalized := value
	if value <= 1 {
		normalized = value * 100
	}
	if normalized < 0 || normalized > 100 {
		return 0, errors.New(context + " must be between 0-1 (decimal) or 0-100 (percentage). Received: " + js.Str(value))
	}
	return normalized, nil
}

// toCoordinates is toCoordinates(visibility, evolution, context).
func toCoordinates(visibility, evolution float64, context string) (Point, error) {
	x, err := toPercent(evolution, context+" evolution")
	if err != nil {
		return Point{}, err
	}
	y, err := toPercent(visibility, context+" visibility")
	if err != nil {
		return Point{}, err
	}
	return Point{X: x, Y: y}, nil
}

// getFlowFromPort is getFlowFromPort(port). The result is "" for
// undefined.
func getFlowFromPort(port *string) string {
	if port == nil || *port == "" {
		return ""
	}
	switch *port {
	case "+<>":
		return "bidirectional"
	case "+<":
		return "backward"
	case "+>":
		return "forward"
	}
	return ""
}

// reFlowLabel is /^\+'([^']*)'/.
var reFlowLabel = regexp.MustCompile(`^\+'([^']*)'`)

// extractFlowFromArrow is extractFlowFromArrow(arrow). flow is "" for
// undefined, and hasLabel is false for a label that is undefined.
func extractFlowFromArrow(arrow *string) (flow, label string, hasLabel bool) {
	if arrow == nil || !strings.HasPrefix(*arrow, "+") {
		return "", "", false
	}

	if labelMatch := reFlowLabel.FindStringSubmatch(*arrow); labelMatch != nil {
		label, hasLabel = labelMatch[1], true
	}

	if strings.Contains(*arrow, "<>") {
		return "bidirectional", label, hasLabel
	}
	if strings.Contains(*arrow, "<") {
		return "backward", label, hasLabel
	}
	if strings.Contains(*arrow, ">") {
		return "forward", label, hasLabel
	}
	return "", label, hasLabel
}

// labelOffsets is the pair of label offsets of a component:
// (label.negX ? -1 : 1) * label.offsetX and the same for y, or undefined
// without a label.
func labelOffsets(node *Node, label *Label) {
	if label == nil {
		return
	}
	sign := func(neg bool) float64 {
		if neg {
			return -1
		}
		return 1
	}
	node.LabelOffsetX, node.HasLabelOffsetX = sign(label.NegX)*label.OffsetX, true
	node.LabelOffsetY, node.HasLabelOffsetY = sign(label.NegY)*label.OffsetY, true
}

// populateDb is populateDb(ast, db).
func populateDb(ast *Wardley, db *DB) error {
	ast.Populate(db)

	// Set size if provided
	if ast.Size != nil {
		db.SetSize(ast.Size.Width, ast.Size.Height)
	}

	// Process evolution stages
	if ast.Evolution != nil {
		stages := []string{}
		stageBoundaries := []float64{}
		for _, stage := range ast.Evolution.Stages {
			// Handle dual-label stages like "Genesis / Concept"
			if stage.SecondName != nil && *stage.SecondName != "" {
				stages = append(stages, js.TrimSpace(stage.Name)+" / "+js.TrimSpace(*stage.SecondName))
			} else {
				stages = append(stages, js.TrimSpace(stage.Name))
			}
			if stage.Boundary != nil {
				stageBoundaries = append(stageBoundaries, *stage.Boundary)
			}
		}

		db.UpdateAxes(AxesConfig{Stages: stages, StageBoundaries: stageBoundaries})
	}

	// Add anchors as nodes with className 'anchor'
	for _, anchor := range ast.Anchors {
		coords, err := toCoordinates(anchor.Visibility, anchor.Evolution, `Anchor "`+anchor.Name+`"`)
		if err != nil {
			return err
		}
		db.AddNode(Node{ID: anchor.Name, Label: anchor.Name, X: coords.X, Y: coords.Y, ClassName: "anchor"})
	}

	// Add components
	for _, component := range ast.Components {
		coords, err := toCoordinates(component.Visibility, component.Evolution, `Component "`+component.Name+`"`)
		if err != nil {
			return err
		}
		node := Node{
			ID:         component.Name,
			Label:      component.Name,
			X:          coords.X,
			Y:          coords.Y,
			ClassName:  "component",
			Inertia:    component.Inertia,
			HasInertia: true,
		}
		labelOffsets(&node, component.Label)
		if component.Decorator != nil {
			node.SourceStrategy = component.Decorator.Strategy
		}

		db.AddNode(node)
	}

	// Add notes
	for _, note := range ast.Notes {
		coords, err := toCoordinates(note.Visibility, note.Evolution, `Note "`+note.Text+`"`)
		if err != nil {
			return err
		}
		db.AddNote(note.Text, coords.X, coords.Y)
	}

	// Process pipelines
	for _, pipeline := range ast.Pipelines {
		parentNode := db.GetNode(pipeline.Parent)
		if parentNode == nil {
			return errors.New(`Pipeline "` + pipeline.Parent + `" must reference an existing component with coordinates.`)
		}
		parentY := parentNode.Y

		db.StartPipeline(pipeline.Parent)
		for _, component := range pipeline.Components {
			// Create a synthetic ID for pipeline components
			componentID := pipeline.Parent + "_" + component.Name
			x, err := toPercent(component.Evolution, `Pipeline component "`+component.Name+`" evolution`)
			if err != nil {
				return err
			}

			// Add pipeline component node (it will be associated with the parent)
			node := Node{ID: componentID, Label: component.Name, X: x, Y: parentY, ClassName: "pipeline-component"}
			labelOffsets(&node, component.Label)
			db.AddNode(node)
			db.AddPipelineComponent(pipeline.Parent, componentID)
		}
	}

	// Add links
	for _, link := range ast.Links {
		isDashed := link.Arrow != nil && (strings.Contains(*link.Arrow, "-.->") || strings.Contains(*link.Arrow, ".-."))

		flow := getFlowFromPort(link.FromPort)
		if flow == "" {
			flow = getFlowFromPort(link.ToPort)
		}
		arrowFlow, flowLabel, hasFlowLabel := extractFlowFromArrow(link.Arrow)
		if flow == "" && arrowFlow != "" {
			flow = arrowFlow
		}

		// const label = flowLabel ?? annotation;
		label, hasLabel := flowLabel, hasFlowLabel
		if !hasFlowLabel && link.LinkLabel != nil {
			label, hasLabel = *link.LinkLabel, true
		}

		db.AddLink(LinkData{
			Source:   db.ResolveNodeID(link.From),
			Target:   db.ResolveNodeID(link.To),
			Dashed:   isDashed,
			Label:    label,
			HasLabel: hasLabel,
			Flow:     flow,
		})
	}

	// Add evolves (trends)
	for _, evolve := range ast.Evolves {
		// Get the component to find its current position
		if node := db.GetNode(evolve.Component); node != nil {
			target, err := toPercent(evolve.Target, `Evolve target for "`+evolve.Component+`"`)
			if err != nil {
				return err
			}
			db.AddTrend(evolve.Component, target, node.Y)
		}
	}

	// Set annotations box
	if len(ast.Annotations) > 0 {
		annotationsBox := ast.Annotations[0]
		coords, err := toCoordinates(annotationsBox.X, annotationsBox.Y, "Annotations box")
		if err != nil {
			return err
		}
		db.SetAnnotationsBox(coords.X, coords.Y)
	}

	// Add individual annotations
	for _, annotation := range ast.Annotation {
		coords, err := toCoordinates(annotation.X, annotation.Y, "Annotation "+js.Str(annotation.Number))
		if err != nil {
			return err
		}
		db.AddAnnotation(annotation.Number, []Point{{X: coords.X, Y: coords.Y}}, annotation.Text)
	}

	// Add accelerators
	for _, accelerator := range ast.Accelerators {
		coords, err := toCoordinates(accelerator.X, accelerator.Y, `Accelerator "`+accelerator.Name+`"`)
		if err != nil {
			return err
		}
		db.AddAccelerator(accelerator.Name, coords.X, coords.Y)
	}

	// Add deaccelerators
	for _, deaccelerator := range ast.Deaccelerators {
		coords, err := toCoordinates(deaccelerator.X, deaccelerator.Y, `Deaccelerator "`+deaccelerator.Name+`"`)
		if err != nil {
			return err
		}
		db.AddDeaccelerator(deaccelerator.Name, coords.X, coords.Y)
	}
	return nil
}

// parseDiagram is parser.parse(input).
func parseDiagram(_ *render.Context, rdb render.DB, input string) error {
	ast, err := parse(input)
	if err != nil {
		return err
	}
	return populateDb(ast, rdb.(*DB))
}
