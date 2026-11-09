// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/position/index.js
// (MIT).

package dagre

import "github.com/bvolpato/mermaid-go-renderer/internal/graphlib"

func (l *layout) position(g *graphlib.Graph) {
	g = asNonCompoundGraph(g)

	positionY(g)
	l.positionX(g).Each(func(v string, x float64) {
		nodeOf(g, v).x = x
	})
}

func positionY(g *graphlib.Graph) {
	layering := buildLayerMatrix(g)
	rankSep := graphOf(g).ranksep
	prevY := 0.0
	for _, layer := range layering {
		heights := make([]float64, len(layer))
		for i, v := range layer {
			heights[i] = nodeOf(g, v).height
		}
		maxHeight := orNaN(lodashMax(heights))
		for _, v := range layer {
			nodeOf(g, v).y = prevY + float64(maxHeight/2)
		}
		prevY += maxHeight + rankSep
	}
}
