// Ported from mermaid 12.2.0, diagrams/block/blockRenderer.ts (MIT).

package block

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
)

// getClasses is getClasses(text, diagObj).
func getClasses(db *DB) *js.Map[string, *ClassDef] {
	return db.GetClasses()
}

// isFinite is Number.isFinite(x).
func isFinite(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0)
}

// draw is draw(text, id, _version, diagObj).
func draw(ctx *render.Context, db *DB) error {
	// const { securityLevel, block: conf } = configApi.getConfig();
	conf := ctx.Cfg("block")
	id := ctx.ID
	db.SetDiagramId(id)

	svg := ctx.GetDiagramElement()

	// The look-specific stylesheet rules reference these by id. Block runs its own render
	// loop rather than `rendering-util/render.ts`, so it has to ask for them: without them a
	// `look: neo` block resolves `stroke: url(#…-gradient)` to nothing and loses its borders.
	ru.InsertLookDefs(svg, ctx.Config)

	// Define the supported markers for the diagram
	markers := []string{"point", "circle", "cross"}

	// Add the marker definitions to the svg as marker tags
	if err := edges.InsertMarkers(ctx, svg, markers, ctx.Type, id); err != nil {
		return err
	}

	bl := db.GetBlocks()
	blArr := db.GetBlocksFlat()
	edgeList := db.GetEdges()

	nodes := svg.Insert("g").Attr("class", "block")
	if err := calculateBlockSizes(ctx, nodes, bl, db); err != nil {
		return err
	}
	bounds, hasBounds, err := layout(db)
	if err != nil {
		return err
	}
	if err := insertBlocks(ctx, nodes, bl, db); err != nil {
		return err
	}
	if err := insertEdges(ctx, nodes, edgeList, blArr, db, id); err != nil {
		return err
	}

	// Establish svg dimensions and get width and height
	renderedBounds := nodes.BBox()
	bounds2 := bounds
	hasBounds2 := hasBounds
	if isFinite(renderedBounds.Width) && isFinite(renderedBounds.Height) {
		bounds2 = Bounds{X: renderedBounds.X, Y: renderedBounds.Y, Width: renderedBounds.Width, Height: renderedBounds.Height}
		hasBounds2 = true
	}
	if hasBounds2 {
		magicFactor := js.Max(1, js.Round(0.125*(bounds2.Width/bounds2.Height)))
		height := bounds2.Height + magicFactor + 10
		width := bounds2.Width + 10
		// const { useMaxWidth } = conf!;
		if js.IsNullish(conf) {
			return typeError("Cannot destructure property 'useMaxWidth' of 'conf' as it is " + js.ToString(conf) + ".")
		}
		useMaxWidth := js.Truthy(js.Path(conf, "useMaxWidth"))
		render.ConfigureSvgSize(svg, height, width, useMaxWidth)
		svg.Attr("viewBox", js.Str(bounds2.X-5)+" "+js.Str(bounds2.Y-5)+" "+js.Str(bounds2.Width+10)+" "+js.Str(bounds2.Height+10))
	}
	return nil
}
