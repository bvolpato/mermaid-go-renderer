// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/coordinate-system.js
// (MIT).

package dagre

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
)

func coordinateSystemAdjust(g *graphlib.Graph) {
	rankDir := strings.ToLower(graphOf(g).rankdir)
	if rankDir == "lr" || rankDir == "rl" {
		swapWidthHeight(g)
	}
}

func coordinateSystemUndo(g *graphlib.Graph) {
	rankDir := strings.ToLower(graphOf(g).rankdir)
	if rankDir == "bt" || rankDir == "rl" {
		reverseY(g)
	}

	if rankDir == "lr" || rankDir == "rl" {
		swapXY(g)
		swapWidthHeight(g)
	}
}

func swapWidthHeight(g *graphlib.Graph) {
	for _, v := range g.Nodes() {
		attrs := nodeOf(g, v)
		attrs.width, attrs.height = attrs.height, attrs.width
	}
	for _, e := range g.Edges() {
		attrs := edgeOf(g, e)
		attrs.width, attrs.height = attrs.height, attrs.width
	}
}

func reverseY(g *graphlib.Graph) {
	for _, v := range g.Nodes() {
		attrs := nodeOf(g, v)
		attrs.y = -attrs.y
	}

	for _, e := range g.Edges() {
		edge := edgeOf(g, e)
		for i := range edge.points {
			edge.points[i].Y = -edge.points[i].Y
		}
		if edge.hasXY {
			edge.y = -edge.y
		}
	}
}

func swapXY(g *graphlib.Graph) {
	for _, v := range g.Nodes() {
		attrs := nodeOf(g, v)
		attrs.x, attrs.y = attrs.y, attrs.x
	}

	for _, e := range g.Edges() {
		edge := edgeOf(g, e)
		for i := range edge.points {
			edge.points[i].X, edge.points[i].Y = edge.points[i].Y, edge.points[i].X
		}
		if edge.hasXY {
			edge.x, edge.y = edge.y, edge.x
		}
	}
}
