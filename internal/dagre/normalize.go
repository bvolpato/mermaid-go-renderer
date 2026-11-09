// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/normalize.js (MIT).

package dagre

import "github.com/bvolpato/mermaid-go-renderer/internal/graphlib"

// normalizeRun breaks any long edges in the graph into short segments that
// span one layer each. normalizeUndo reverses it.
//
// Pre-conditions:
//
//  1. The input graph is a DAG.
//  2. Each node in the graph has a "rank" property.
//
// Post-conditions:
//
//  1. All edges in the graph have a length of 1.
//  2. Dummy nodes are added where edges have been split into segments.
//  3. The graph has a "dummyChains" attribute with the first dummy of each
//     chain of dummy nodes.
func (l *layout) normalizeRun(g *graphlib.Graph) {
	graphOf(g).dummyChains = []string{}
	for _, edge := range g.Edges() {
		l.normalizeEdge(g, edge)
	}
}

func (l *layout) normalizeEdge(g *graphlib.Graph, e graphlib.Edge) {
	v := e.V
	vRank := nodeOf(g, v).rank
	w := e.W
	wRank := nodeOf(g, w).rank
	name := e.Name
	edgeLabel := edgeOf(g, e)
	labelRank := edgeLabel.labelRank

	if wRank == vRank+1 {
		return
	}

	g.RemoveEdgeObj(e)

	i := 0
	for vRank++; vRank < wRank; i, vRank = i+1, vRank+1 {
		edgeLabel.points = []Point{}
		attrs := newNodeLabel()
		attrs.width = 0
		attrs.height = 0
		attrs.edgeLabel = edgeLabel
		attrs.edgeObj = e
		attrs.rank = vRank
		attrs.hasRank = true
		dummy := l.addDummyNode(g, "edge", attrs, "_d")
		if vRank == labelRank {
			attrs.width = edgeLabel.width
			attrs.height = edgeLabel.height
			attrs.dummy = "edge-label"
			attrs.labelpos = edgeLabel.labelpos
			attrs.hasLabelpos = true
		}
		setEdge(g, v, dummy, segmentLabel(edgeLabel.weight), name)
		if i == 0 {
			graph := graphOf(g)
			graph.dummyChains = append(graph.dummyChains, dummy)
		}
		v = dummy
	}

	setEdge(g, v, w, segmentLabel(edgeLabel.weight), name)
}

// segmentLabel is the edge label { weight: weight }.
func segmentLabel(weight float64) *edgeLabel {
	label := newEdgeLabel()
	label.weight = weight
	return label
}

func normalizeUndo(g *graphlib.Graph) {
	for _, v := range graphOf(g).dummyChains {
		node := nodeOf(g, v)
		origLabel := node.edgeLabel
		setEdgeObj(g, node.edgeObj, origLabel)
		for node.dummy != "" {
			w := g.Successors(v)[0]
			g.RemoveNode(v)
			origLabel.points = append(origLabel.points, Point{X: node.x, Y: node.y})
			if node.dummy == "edge-label" {
				origLabel.x = node.x
				origLabel.y = node.y
				origLabel.hasXY = true
				origLabel.width = node.width
				origLabel.height = node.height
			}
			v = w
			node = nodeOf(g, v)
		}
	}
}
