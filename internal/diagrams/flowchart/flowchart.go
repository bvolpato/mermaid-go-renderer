// Package flowchart is the flowchart diagram: the diagram types
// flowchart-v2 (the keywords graph and flowchart), flowchart-elk (the
// keyword flowchart-elk) and swimlane (the keyword swimlane-beta), which
// share one parser, one db and one renderer upstream.
//
// Ported from mermaid 12.2.0, diagrams/flowchart/flowDiagram.ts,
// flowRenderer-v3-unified.ts and diagrams/swimlanes/swimlanesDiagram.ts (MIT).
// The other files of the package port flowDb.ts, parser/flowParser.ts and the
// semantic actions of parser/flow.jison.
//
// The diagram is drawn by the unified renderer (internal/ru). The shapes,
// the edges, the clusters and the layouts register themselves there: a
// program must link those packages too.
package flowchart

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The diagram types that createFlowDiagram defines. flowchart-elk loads the
// same diagram object as flowchart-v2. swimlane is the flowchart diagram
// with its own style sheet, which the styles of the pipeline select by the
// diagram type.
var typeIDs = []string{"flowchart-v2", "flowchart-elk", "swimlane"}

func init() {
	for _, id := range typeIDs {
		render.Register(NewDefinition(id))
	}
}

// NewDefinition is createFlowDiagram(): the diagram definition of a
// flowchart for a diagram type id.
func NewDefinition(id string) *render.Definition {
	return &render.Definition{
		ID:   id,
		New:  func(ctx *render.Context) render.DB { return NewDB(ctx) },
		Init: initDiagram,
		Parse: func(_ *render.Context, db render.DB, text string) error {
			return parse(db.(*DB), text)
		},
		Draw: func(ctx *render.Context, db render.DB, text string) error {
			return draw(ctx, db.(*DB))
		},
		Classes: func(_ *render.Context, db render.DB, _ string) *js.Map[string, *render.ClassDef] {
			return getClasses(db.(*DB))
		},
	}
}

// initDiagram is the init hook of the diagram definition:
//
//	init: (cnf) => {
//	  if (!cnf.flowchart) { cnf.flowchart = {}; }
//	  cnf.flowchart.arrowMarkerAbsolute = cnf.arrowMarkerAbsolute;
//	  setConfig({ flowchart: { arrowMarkerAbsolute: cnf.arrowMarkerAbsolute } });
//	}
//
// The layout is not forced here: the keyword flowchart-elk does not change
// the `layout` of the config.
func initDiagram(ctx *render.Context, _ render.DB) {
	cnf := ctx.Config
	arrowMarkerAbsolute := cnf.Get("arrowMarkerAbsolute")
	// cnf is a copy of the config, so this assignment changes the copy only.
	flowchart := js.AsObj(cnf.Get("flowchart"))
	if !js.Truthy(cnf.Get("flowchart")) {
		flowchart = js.NewObj[any]()
		cnf.Set("flowchart", flowchart)
	}
	if flowchart != nil {
		flowchart.Set("arrowMarkerAbsolute", arrowMarkerAbsolute)
	}

	section := js.NewObj[any]()
	section.Set("arrowMarkerAbsolute", arrowMarkerAbsolute)
	conf := js.NewObj[any]()
	conf.Set("flowchart", section)
	// setConfig can fail only for a theme that the config state has
	// accepted before, with the same directives.
	_, _ = ctx.ConfigState.SetConfig(conf)
}

// getClasses is getClasses(text, diagramObj) of the renderer: the class
// definitions of the diagram, which become CSS rules.
func getClasses(d *DB) *js.Map[string, *render.ClassDef] {
	return d.GetClasses()
}

// orNumber is `v || d` for a config value that is a number.
func orNumber(v any, d float64) float64 {
	if !js.Truthy(v) {
		return d
	}
	return toNumber(v)
}

// spacing is `v || 50` for the nodeSpacing and the rankSpacing of the
// layout data. The config can have the value as a string (the corpus has
// rankSpacing: '100'), and upstream hands the string to the layout. The
// field of the layout data has its number, and Extra keeps the JavaScript
// value under the name of the property.
func spacing(data *ru.LayoutData, key string, v any) float64 {
	if !js.Truthy(v) {
		return 50
	}
	if n, ok := v.(float64); ok {
		return n
	}
	if data.Extra == nil {
		data.Extra = map[string]any{}
	}
	data.Extra[key] = v
	return toNumber(v)
}

// layoutName is the argument of getRegisteredLayoutAlgorithm for the
// `layout` of the config. Its default parameter is the empty string.
func layoutName(layout any) string {
	if layout == nil {
		return ""
	}
	return js.ToString(layout)
}

// layoutData is the part of draw that builds the layout data: getData()
// and the properties that the renderer adds.
func layoutData(ctx *render.Context, d *DB) *ru.LayoutData {
	conf := js.AsObj(ctx.Cfg("flowchart"))
	layout := ctx.Cfg("layout")

	// Set the diagram ID for DOM element uniqueness across multiple diagrams
	d.SetDiagramId(ctx.ID)

	// The getData method provided in all supported diagrams is used to extract the data from the parsed structure
	// into the Layout data format
	data4Layout := d.GetData()
	direction, _ := d.GetDirection()

	data4Layout.Type = ctx.Type
	data4Layout.LayoutAlgorithm = ru.GetRegisteredLayoutAlgorithm(layoutName(layout), "")
	data4Layout.Direction = direction
	data4Layout.NodeSpacing = spacing(data4Layout, "nodeSpacing", js.Path(conf, "nodeSpacing"))
	data4Layout.RankSpacing = spacing(data4Layout, "rankSpacing", js.Path(conf, "rankSpacing"))
	data4Layout.Markers = []string{"point", "circle", "cross"}

	data4Layout.DiagramID = ctx.ID
	return data4Layout
}

// checkLabels reports the node that upstream cannot draw: the label helper
// of the shapes draws `label[0]` for a label that is not a string, and for
// a number, a boolean or an object that is undefined.
func checkLabels(data *ru.LayoutData) error {
	for _, node := range data.Nodes {
		label, ok := node.Extra["label"]
		if !ok || node.HasLabel {
			continue
		}
		if _, isString := label.(string); !isString {
			return typeError("Cannot read properties of undefined (reading 'replace')")
		}
	}
	return nil
}

// draw is draw(text, id, _version, diag) of flowRenderer-v3-unified.ts.
func draw(ctx *render.Context, d *DB) error {
	conf := js.AsObj(ctx.Cfg("flowchart"))

	data4Layout := layoutData(ctx, d)
	// Create the root SVG
	svg := ctx.GetDiagramElement()

	if err := checkLabels(data4Layout); err != nil {
		return err
	}
	if err := ru.Render(ctx, data4Layout, svg); err != nil {
		return err
	}
	// data4Layout.config.flowchart?.diagramPadding ?? 8
	padding := 8.0
	if v := js.Path(data4Layout.Config, "flowchart", "diagramPadding"); !js.IsNullish(v) {
		padding = toNumber(v)
	}
	render.InsertTitle(
		svg,
		"flowchartTitleText",
		orNumber(js.Path(conf, "titleTopMargin"), 0),
		d.GetDiagramTitle(),
	)
	render.SetupViewPortForSVG(svg, padding, "flowchart", js.Truthy(js.Path(conf, "useMaxWidth")))
	return nil
}
