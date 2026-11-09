// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/index.ts and the swimlane part of
// rendering-util/render.ts (registerDefaultLayoutLoaders) (MIT).

package swimlanes

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	rugraph "github.com/bvolpato/mermaid-go-renderer/internal/ru/graph"
)

func init() {
	ru.RegisterLayout("swimlane", "", Render)
}

func prepareSwimlaneLayout(data4Layout *ru.LayoutData, _ *rugraph.RenderContext) (any, error) {
	prepareLayoutForSwimlanes(data4Layout)

	nodes, edges := createEdgeLabelNodes(data4Layout)
	data4Layout.Nodes = nodes
	data4Layout.Edges = edges
	return nil, nil
}

// Render is the render function of the swimlane layout: it lays out and
// draws layout data. The phases are the ones of the common layout renderer:
// the prepare phase makes the lanes and the label nodes, the default
// measure phase draws and measures the nodes, the layout core computes the
// positions, the default paint draws the lanes, the nodes and the edges,
// and the hops are drawn after the paint.
var Render = rugraph.CreateCommonLayoutRenderer(rugraph.Definition{
	PrepareLayout: prepareSwimlaneLayout,
	RunLayoutCore: func(data4Layout *ru.LayoutData, _ *rugraph.RenderContext) (any, error) {
		return runSwimlaneLayoutCore(data4Layout)
	},
	AfterPaint: applySwimlaneLineJumps,
})
