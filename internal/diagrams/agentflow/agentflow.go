// Package agentflow is the agentflow diagram: the diagram type agentflow
// (the keyword agentflow-beta).
//
// Ported from mermaid 12.2.0, diagrams/agentflow/diagram.ts, afDetector.ts and
// renderer.ts (MIT). The other files of the package port agentflowDb.ts,
// shapes.ts, transformData.ts, colorSlots.ts, diagnostics.ts, types.ts,
// parser/agentflowParser.ts and the semantic actions of
// parser/agentflow.jison. The style sheet (styles.ts) is in internal/styles,
// and the detector is in internal/config.
//
// The diagram is drawn by the unified renderer (internal/ru). The shapes,
// the edges, the clusters and the layouts register themselves there: a
// program must link those packages too.
package agentflow

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// typeID is the id of the diagram type: the id of afDetector.ts.
const typeID = "agentflow"

func init() {
	render.Register(NewDefinition())
}

// NewDefinition is the diagram object of diagram.ts: the diagram definition
// of agentflow.
func NewDefinition() *render.Definition {
	return &render.Definition{
		ID:   typeID,
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
//	  cnf.agentflow ??= {};
//	  if (cnf.layout) { setConfig({ layout: cnf.layout }); }
//	  cnf.flowchart ??= {};
//	  cnf.flowchart.arrowMarkerAbsolute = cnf.arrowMarkerAbsolute;
//	  setConfig({ flowchart: { arrowMarkerAbsolute: cnf.arrowMarkerAbsolute } });
//	}
//
// cnf is a copy of the config that only Diagram.fromText reads after the
// hook, so the assignments to cnf have no effect that this port needs. The
// edges of the unified renderer read arrowMarkerAbsolute in the flowchart
// section, so the hook copies the global flag to that section.
func initDiagram(ctx *render.Context, _ render.DB) {
	cnf := ctx.Config
	if layout := cnf.Get("layout"); js.Truthy(layout) {
		conf := js.NewObj[any]()
		conf.Set("layout", layout)
		// setConfig can fail only for a theme that the config state has
		// accepted before, with the same directives.
		_, _ = ctx.ConfigState.SetConfig(conf)
	}

	section := js.NewObj[any]()
	section.Set("arrowMarkerAbsolute", cnf.Get("arrowMarkerAbsolute"))
	conf := js.NewObj[any]()
	conf.Set("flowchart", section)
	_, _ = ctx.ConfigState.SetConfig(conf)
}

// getClasses is getClasses(text, diagramObj) of the renderer: the class
// definitions of the diagram, which become CSS rules.
func getClasses(d *DB) *js.Map[string, *render.ClassDef] {
	return d.GetClasses()
}

// spacing is `v || 50` for the nodeSpacing and the rankSpacing of the
// layout data. The config can have the value as a string, and upstream
// hands the string to the layout. The field of the layout data has its
// number, and Extra keeps the JavaScript value under the name of the
// property.
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
func layoutData(ctx *render.Context, d *DB) *LayoutData {
	conf := ctx.Cfg("agentflow")
	layout := ctx.Cfg("layout")

	// Scope generated domIds to this diagram's svg id so click handlers bind to
	// this diagram's nodes and not to an identically-named node in another
	// diagram on the same page.
	d.SetDiagramId(ctx.ID)

	// The getData method provided in all supported diagrams is used to extract the data from the parsed structure
	// into the Layout data format
	// `getData()` already applies the agentflow shape transformations and emits
	// the SHAPE_* diagnostics, so the data arrives render-ready.
	data4Layout := d.GetData()
	direction, _ := d.GetDirection()

	data4Layout.Type = ctx.Type
	data4Layout.LayoutAlgorithm = ru.GetRegisteredLayoutAlgorithm(layoutName(layout), "")
	data4Layout.Direction = direction
	data4Layout.NodeSpacing = spacing(data4Layout.LayoutData, "nodeSpacing", js.Path(conf, "nodeSpacing"))
	data4Layout.RankSpacing = spacing(data4Layout.LayoutData, "rankSpacing", js.Path(conf, "rankSpacing"))
	data4Layout.Markers = []string{"point", "circle", "cross", "hierarchy"}

	data4Layout.DiagramID = ctx.ID
	return data4Layout
}

// draw is draw(text, id, _version, diag) of renderer.ts.
func draw(ctx *render.Context, d *DB) error {
	conf := ctx.Cfg("agentflow")

	data4Layout := layoutData(ctx, d)
	// Create the root SVG
	svg := ctx.GetDiagramElement()

	if err := ru.Render(ctx, data4Layout.LayoutData, svg); err != nil {
		return err
	}
	// conf?.diagramPadding ?? 8
	padding := 8.0
	if v := js.Path(conf, "diagramPadding"); !js.IsNullish(v) {
		padding = toNumber(v)
	}
	// conf?.titleTopMargin ?? 0
	titleTopMargin := 0.0
	if v := js.Path(conf, "titleTopMargin"); !js.IsNullish(v) {
		titleTopMargin = toNumber(v)
	}
	render.InsertTitle(svg, "agentflowTitleText", titleTopMargin, d.GetDiagramTitle())
	// conf?.useMaxWidth ?? true
	useMaxWidth := true
	if v := js.Path(conf, "useMaxWidth"); !js.IsNullish(v) {
		useMaxWidth = js.Truthy(v)
	}
	render.SetupViewPortForSVG(svg, padding, "agentflow", useMaxWidth)
	return nil
}
