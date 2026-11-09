// Package eventmodeling is the event modeling diagram: the diagram type
// eventmodeling.
//
// Ported from mermaid 12.2.0, diagrams/eventmodeling/diagram.ts, detector.ts,
// parser.ts and renderer.ts (MIT). The other files of the package port db.ts
// and types.ts, and the grammar
// parser/src/language/eventmodeling/event-modeling.langium. styles.js gives no
// style rules.
package eventmodeling

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:  "eventmodeling",
		New: func(ctx *render.Context) render.DB { return NewDB(ctx) },
		Parse: func(_ *render.Context, db render.DB, text string) error {
			return parseDiagram(db.(*DB), text)
		},
		Draw: func(ctx *render.Context, db render.DB, _ string) error {
			return draw(ctx, db.(*DB))
		},
	})
}

// parseDiagram is parser.parse of parser.ts.
func parseDiagram(db *DB, input string) error {
	ast, err := parse(input)
	if err != nil {
		return err
	}
	if err := db.SetAst(ast); err != nil {
		return err
	}
	// populateCommonDb(ast, db)
	if ast.AccDescr != nil && *ast.AccDescr != "" {
		db.SetAccDescription(*ast.AccDescr)
	}
	if ast.AccTitle != nil && *ast.AccTitle != "" {
		db.SetAccTitle(*ast.AccTitle)
	}
	if ast.Title != nil && *ast.Title != "" {
		db.SetDiagramTitle(*ast.Title)
	}
	return nil
}

// themeColor is `themeVariables.<key> ?? fallback`.
func themeColor(themeVariables any, key string, fallback any) any {
	if v, _ := js.GetProperty(themeVariables, key); !js.IsNullish(v) {
		return v
	}
	return fallback
}

// renderD3Box is the function that renderD3Box(diagram, diagramProps)
// returns: it draws one box.
func renderD3Box(diagram *dom.Node, diagramProps DiagramProps, box *Box) {
	y := box.Swimlane.Y + diagramProps.SwimlanePadding

	g := diagram.Append("g").Attr("class", "em-box")

	g.Append("rect").
		Attr("x", box.X).
		Attr("y", y).
		Attr("rx", "3").
		Attr("width", box.Dimension.Width).
		Attr("height", box.Dimension.Height).
		Attr("stroke", box.Visual.Stroke).
		Attr("fill", box.Visual.Fill)

	f := g.
		Append("foreignObject").
		Attr("x", box.X+diagramProps.BoxPadding).
		Attr("y", y+10).
		Attr("width", box.Dimension.Width-float64(2*diagramProps.BoxPadding)).
		Attr("height", box.Dimension.Height-float64(2*diagramProps.BoxPadding))

	text := f.
		Append("xhtml:div").
		Style("display", "table").
		Style("height", "100%").
		Style("width", "100%")

	text.
		Append("span").
		Style("display", "table-cell").
		Style("text-align", "center").
		Style("vertical-align", "middle").
		HTML(box.Text)
}

// dirUpwards is dirUpwards(sourceY, targetY).
func dirUpwards(sourceY, targetY float64) bool {
	return sourceY > targetY
}

// renderD3Relation is the function that renderD3Relation(diagram,
// diagramProps, arrowheadId, themeVariables) returns: it draws one
// relation.
func renderD3Relation(diagram *dom.Node, diagramProps DiagramProps, arrowheadId string, themeVariables any, relation *Relation) {
	sourceBoxY := relation.SourceBox.Swimlane.Y + diagramProps.SwimlanePadding
	targetBoxY := relation.TargetBox.Swimlane.Y + diagramProps.SwimlanePadding

	upwards := dirUpwards(sourceBoxY, targetBoxY)

	sourceX := relation.SourceBox.X + float64(relation.SourceBox.Dimension.Width*2)/3
	targetX := relation.TargetBox.X + relation.TargetBox.Dimension.Width/3

	var sourceY, targetY float64

	if upwards {
		sourceY = sourceBoxY
		targetY = targetBoxY + relation.TargetBox.Dimension.Height
	} else {
		sourceY = sourceBoxY + relation.SourceBox.Dimension.Height
		targetY = targetBoxY
	}

	relationStroke := themeColor(themeVariables, "emRelationStroke", relation.Visual.Stroke)

	diagram.
		Append("path").
		Attr("class", "em-relation").
		Attr("fill", relation.Visual.Fill).
		Attr("stroke", relationStroke).
		Attr("stroke-width", "1").
		Attr("marker-end", "url(#"+arrowheadId+")").
		Attr("d", "M"+js.Str(sourceX)+" "+js.Str(sourceY)+" L"+js.Str(targetX)+" "+js.Str(targetY))
}

// renderD3Swimlane is the function that renderD3Swimlane(diagram, maxR,
// diagramProps, themeVariables) returns: it draws one swimlane.
func renderD3Swimlane(diagram *dom.Node, maxR float64, diagramProps DiagramProps, themeVariables any, swimlane *Swimlane) {
	g := diagram.Append("g").Attr("class", "em-swimlane")

	oddBackground := themeColor(themeVariables, "emSwimlaneBackgroundOdd", "rgb(250,250,250)")
	backgroundStroke := themeColor(themeVariables, "emSwimlaneBackgroundStroke", "rgb(240,240,240)")

	g.Append("rect").
		Attr("x", 0).
		Attr("y", swimlane.Y).
		Attr("rx", "3").
		Attr("width", maxR+diagramProps.SwimlanePadding).
		Attr("height", swimlane.Height).
		Attr("fill", oddBackground).
		Attr("stroke", backgroundStroke)

	g.Append("text").
		Attr("font-weight", diagramProps.SwimlaneTextFontWeight).
		Attr("x", 30).
		Attr("y", swimlane.Y+30).
		Text(swimlane.Label)
}

// draw is draw(txt, id, ver, diagObj) of renderer.ts.
func draw(ctx *render.Context, db *DB) error {
	// DEFAULT_EVENTMODELING_CONFIG is the eventmodeling config at the time
	// when the module of the renderer loads. The diagram loads during the
	// first render of an event model, after the config of that render is
	// set. A page makes one render here, so the value is the config of the
	// render.
	if !js.Truthy(ctx.Cfg("eventmodeling")) {
		return &Error{Message: "EventModeling config not found"}
	}
	themeVariables := ctx.Cfg("themeVariables")
	config := ctx.Cfg("eventmodeling")

	diagram := ctx.GetDiagramElement()

	diagramProps := db.GetDiagramProps()
	state, err := db.GetState()
	if err != nil {
		return err
	}

	arrowheadId := "em-arrowhead-" + ctx.ID
	if js.IsNullish(themeVariables) {
		return &Error{Name: "TypeError", Message: "Cannot read properties of " + js.ToString(themeVariables) + " (reading 'emArrowhead')"}
	}
	arrowheadColor := themeColor(themeVariables, "emArrowhead", "#000000")

	for _, swimlane := range state.SortedSwimlanesArray {
		renderD3Swimlane(diagram, state.MaxR, diagramProps, themeVariables, swimlane)
	}
	for _, box := range state.Boxes {
		renderD3Box(diagram, diagramProps, box)
	}
	for _, relation := range state.Relations {
		renderD3Relation(diagram, diagramProps, arrowheadId, themeVariables, relation)
	}

	marker := diagram.
		Append("defs").
		Append("marker").
		Attr("id", arrowheadId).
		Attr("markerWidth", "10").
		Attr("markerHeight", "7").
		Attr("refX", "10").
		Attr("refY", "3.5").
		Attr("orient", "auto")

	marker.Append("polygon").Attr("points", "0 0, 10 3.5, 0 7").Attr("fill", arrowheadColor)

	// config?.padding ?? 30
	padding := js.Path(config, "padding")
	if js.IsNullish(padding) {
		padding = 30.0
	}
	render.SetupGraphViewbox(diagram, js.ToNumber(padding), js.Truthy(js.Path(config, "useMaxWidth")))
	return nil
}
