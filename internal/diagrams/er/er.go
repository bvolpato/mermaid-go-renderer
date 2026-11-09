// Package er is the entity relationship diagram: the diagram type er (the
// keyword erDiagram).
//
// Ported from mermaid 12.2.0, diagrams/er/erDiagram.ts and
// erRenderer-unified.ts (MIT). The other files of the package port erDb.ts,
// erTypes.ts, the semantic actions of parser/erDiagram.jison and the shape
// rendering-util/rendering-elements/shapes/erBox.ts.
//
// The diagram is drawn by the unified renderer (internal/ru). The edges,
// the clusters, the shapes of the subgraphs and the layouts register
// themselves there: a program must link those packages too.
package er

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

func init() {
	render.Register(&render.Definition{
		ID:  "er",
		New: func(ctx *render.Context) render.DB { return NewDB(ctx) },
		Parse: func(_ *render.Context, db render.DB, text string) error {
			return parse(db.(*DB), text)
		},
		Draw: func(ctx *render.Context, db render.DB, _ string) error {
			return draw(ctx, db.(*DB))
		},
	})
	ru.RegisterShape("erBox", erBox)
}

// typeError is a TypeError of JavaScript.
type typeError string

func (e typeError) Error() string { return string(e) }

// orValue is `v || d` for a config value.
func orValue(v any, d any) any {
	if !js.Truthy(v) {
		return d
	}
	return v
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
func layoutData(ctx *render.Context, d *DB) (*ru.LayoutData, error) {
	conf := js.AsObj(ctx.Cfg("er"))
	layout := ctx.Cfg("layout")

	// The getData method provided in all supported diagrams is used to extract the data from the parsed structure
	// into the Layout data format
	data4Layout := d.GetData()

	data4Layout.Type = ctx.Type
	data4Layout.LayoutAlgorithm = ru.GetRegisteredLayoutAlgorithm(layoutName(layout), "")

	// Workaround as when rendering and setting up the graph it uses flowchart spacing before data4Layout spacing?
	flowchart := js.AsObj(data4Layout.Config.Get("flowchart"))
	if flowchart == nil {
		return nil, typeError("Cannot set properties of undefined (setting 'nodeSpacing')")
	}
	flowchart.Set("nodeSpacing", orValue(js.Path(conf, "nodeSpacing"), 140.0))
	flowchart.Set("rankSpacing", orValue(js.Path(conf, "rankSpacing"), 80.0))
	data4Layout.Direction = d.GetDirection()
	// The renderer sets no nodeSpacing and no rankSpacing in the layout data.
	data4Layout.NodeSpacing = ru.Undef
	data4Layout.RankSpacing = ru.Undef

	if js.StrictEquals(data4Layout.Config.Get("look"), "neo") {
		data4Layout.Markers = []string{
			"only_one_neo",
			"zero_or_one_neo",
			"one_or_more_neo",
			"zero_or_more_neo",
		}
	} else {
		data4Layout.Markers = []string{"only_one", "zero_or_one", "one_or_more", "zero_or_more"}
	}
	data4Layout.DiagramID = ctx.ID
	return data4Layout, nil
}

// draw is draw(text, id, _version, diag) of erRenderer-unified.ts.
func draw(ctx *render.Context, d *DB) error {
	conf := js.AsObj(ctx.Cfg("er"))

	data4Layout, err := layoutData(ctx, d)
	if err != nil {
		return err
	}
	// Create the root SVG - the element is the div containing the SVG element
	svg := ctx.GetDiagramElement()

	if err := ru.Render(ctx, data4Layout, svg); err != nil {
		return err
	}
	// Note: the layout render inserts the edge group between the cluster and
	// node groups (clusters < edges < nodes in paint order), so edge markers
	// are covered by the nodes they touch while edges still paint above
	// cluster backgrounds.

	// Sets the background nodes to the same position as their original counterparts.
	// Background nodes are created when the look is handDrawn so the ER diagram markers do not show underneath.
	for _, backgroundNode := range svg.SelectAll(`[id*="-background"]`) {
		backgroundID := backgroundNode.AttrOr("id", "")

		nonBackgroundID := strings.Replace(backgroundID, "-background", "", 1)
		nonBackgroundNode := svg.Select("#" + sanitize.CSSEscape(nonBackgroundID))

		if nonBackgroundNode != nil {
			if transform, ok := nonBackgroundNode.GetAttr("transform"); ok {
				backgroundNode.Attr("transform", transform)
			} else {
				backgroundNode.Attr("transform", nil)
			}
		}
	}

	const padding = 8
	// conf?.titleTopMargin ?? 25
	titleTopMargin := 25.0
	if v := js.Path(conf, "titleTopMargin"); !js.IsNullish(v) {
		titleTopMargin = js.ToNumber(v)
	}
	render.InsertTitle(svg, "erDiagramTitleText", titleTopMargin, d.GetDiagramTitle())

	// conf?.useMaxWidth ?? true
	useMaxWidth := true
	if v := js.Path(conf, "useMaxWidth"); !js.IsNullish(v) {
		useMaxWidth = js.Truthy(v)
	}
	render.SetupViewPortForSVG(svg, padding, "erDiagram", useMaxWidth)
	return nil
}
