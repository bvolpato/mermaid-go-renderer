// Package kanban is the kanban diagram: the diagram type kanban.
//
// Ported from mermaid 12.2.0, diagrams/kanban/kanban-definition.ts,
// detector.ts and kanbanRenderer.ts (MIT). The other files of the package port
// kanbanDb.ts, kanbanTypes.ts, the semantic actions of parser/kanban.jison and
// the node shape kanbanItem of the unified renderer.
//
// The renderer places the sections and the items itself. It draws a section
// with the cluster shape kanbanSection (internal/ru/clusters) and an item
// with the node shape kanbanItem, which this package registers.
package kanban

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
)

func init() {
	render.Register(&render.Definition{
		ID:  "kanban",
		New: func(ctx *render.Context) render.DB { return NewDB(ctx) },
		Parse: func(_ *render.Context, db render.DB, text string) error {
			return parseText(db.(*DB), text)
		},
		Draw: func(ctx *render.Context, db render.DB, _ string) error {
			return draw(ctx, db.(*DB))
		},
	})
}

// sectionWidth is `conf?.kanban?.sectionWidth || 200`.
func sectionWidth(conf *js.Obj[any]) float64 {
	if v := js.Path(conf, "kanban", "sectionWidth"); js.Truthy(v) {
		return js.ToNumber(v)
	}
	return 200
}

// cssClassesText is node.cssClasses in a string concatenation: an
// undefined value prints as "undefined".
func cssClassesText(node *ru.Node) string {
	if !node.CSSClassesSet && node.CSSClasses == "" {
		return "undefined"
	}
	return node.CSSClasses
}

// kanbanConfig is `conf.mindmap?.<key> ?? defaultConfig.kanban.<key>`. The
// renderer reads the value under mindmap and the default under kanban.
func kanbanConfig(ctx *render.Context, conf *js.Obj[any], key string) any {
	if v := js.Path(conf, "mindmap", key); !js.IsNullish(v) {
		return v
	}
	return js.Path(ctx.ConfigState.DefaultSection("kanban"), key)
}

// draw is draw(text, id, _version, diagObj) of kanbanRenderer.ts.
func draw(ctx *render.Context, db *DB) error {
	data4Layout, err := db.GetData()
	if err != nil {
		return err
	}

	// getConfig() returns a copy of the config.
	conf := js.Clone(ctx.Config).(*js.Obj[any])
	conf.Set("htmlLabels", false)

	svg := ctx.SelectSvgElement()

	// Prefix all node domIds with the diagram's SVG element ID to ensure uniqueness
	// across multiple diagrams on the same page. This mirrors render.ts lines 63-70.
	for _, node := range data4Layout.Nodes {
		node.DomID = ctx.ID + "-" + node.ID
	}

	// Draw the graph and start with drawing the nodes without proper position
	// this gives us the size of the nodes and we can set the positions later

	sectionsElem := svg.Append("g")
	sectionsElem.Attr("class", "sections")
	nodesElem := svg.Append("g")
	nodesElem.Attr("class", "items")
	var sections []*ru.Node
	for _, node := range data4Layout.Nodes {
		if node.IsGroup {
			sections = append(sections, node)
		}
	}
	cnt := 0.0
	// TODO set padding
	const padding = 10

	var sectionObjects []*ru.ClusterResult
	maxLabelHeight := 25.0
	for _, section := range sections {
		WIDTH := sectionWidth(conf)
		cnt = cnt + 1
		section.X = float64(WIDTH*cnt) + float64(float64((cnt-1)*padding)/2)
		section.Width = WIDTH
		section.Y = 0
		section.Height = float64(WIDTH * 3)
		section.Rx = 5
		section.Ry = 5

		// Todo, use theme variable THEME_COLOR_LIMIT instead of 10
		section.CSSClasses = cssClassesText(section) + " section-" + js.Str(cnt)
		section.CSSClassesSet = true
		sectionObj, err := clusters.InsertCluster(ctx, sectionsElem, section)
		if err != nil {
			return err
		}
		// sectionObj?.labelBBox?.height
		labelHeight := ru.Undef
		if sectionObj != nil && sectionObj.HasLabelBBox {
			labelHeight = sectionObj.LabelBBox.Height
		}
		maxLabelHeight = js.Max(maxLabelHeight, labelHeight)
		sectionObjects = append(sectionObjects, sectionObj)
	}
	i := 0
	for _, section := range sections {
		sectionObj := sectionObjects[i]
		i = i + 1
		WIDTH := sectionWidth(conf)
		top := float64(float64(-WIDTH*3)/2) + maxLabelHeight
		y := top
		for _, item := range data4Layout.Nodes {
			// data4Layout.nodes.filter((node) => node.parentId === section.id)
			if item.ParentID != section.ID {
				continue
			}
			if item.IsGroup {
				// Kanban diagrams should not have groups within groups
				// this should never happen
				return &Error{Message: "Groups within groups are not allowed in Kanban diagrams"}
			}
			item.X = section.X
			item.Width = WIDTH - 1.5*padding
			nodeEl, err := ru.InsertNode(ctx, nodesElem, item, &ru.ShapeRenderOptions{Config: conf})
			if err != nil {
				return err
			}
			bbox := nodeEl.BBox()
			item.Y = y + float64(bbox.Height/2)
			ru.PositionNode(ctx, ru.PositionedNode{
				ID: item.ID, X: item.X, Y: item.Y, Width: item.Width, Height: item.Height,
				Diff: item.Diff, ClusterNode: item.ClusterNode,
			})
			y = item.Y + float64(bbox.Height/2) + padding/2
		}
		rect := sectionObj.Cluster.Select("rect")
		height := js.Max(y-top+3*padding, 50) + (maxLabelHeight - 25)
		rect.Attr("height", height)
	}

	// Setup the view box and size of the svg element
	render.SetupGraphViewbox(
		svg,
		js.ToNumber(kanbanConfig(ctx, conf, "padding")),
		js.Truthy(kanbanConfig(ctx, conf, "useMaxWidth")),
	)
	return nil
}
