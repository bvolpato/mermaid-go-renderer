// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/util.js (MIT).

package dagre

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
)

// addDummyNode adds a dummy node to the graph and returns its id.
func (l *layout) addDummyNode(g *graphlib.Graph, typ string, attrs *nodeLabel, name string) string {
	var v string
	for {
		v = l.state.UniqueID(name)
		if !g.HasNode(v) {
			break
		}
	}

	attrs.dummy = typ
	g.SetNode(v, attrs)
	return v
}

// simplify returns a new graph with only simple edges. It aggregates the
// data of multi-edges.
func simplify(g *graphlib.Graph) *graphlib.Graph {
	simplified := graphlib.New(graphlib.Options{}).SetGraph(g.Graph())
	for _, v := range g.Nodes() {
		simplified.SetNode(v, g.Node(v))
	}
	for _, e := range g.Edges() {
		simpleLabel := edgeOfVW(simplified, e.V, e.W)
		if simpleLabel == nil {
			simpleLabel = &edgeLabel{weight: 0, minlen: 1}
		}
		label := edgeOf(g, e)
		combined := newEdgeLabel()
		combined.weight = simpleLabel.weight + label.weight
		combined.minlen = math.Max(simpleLabel.minlen, label.minlen)
		setEdge(simplified, e.V, e.W, combined, "")
	}
	return simplified
}

func asNonCompoundGraph(g *graphlib.Graph) *graphlib.Graph {
	simplified := graphlib.New(graphlib.Options{Multigraph: g.IsMultigraph()}).SetGraph(g.Graph())
	for _, v := range g.Nodes() {
		if len(g.Children(v)) == 0 {
			simplified.SetNode(v, g.Node(v))
		}
	}
	for _, e := range g.Edges() {
		setEdgeObj(simplified, e, g.EdgeObj(e))
	}
	return simplified
}

// rect is the part of a node label that intersectRect reads.
type rect struct {
	x, y, width, height float64
}

// intersectRect finds where a line that starts at point and points at the
// center of the rectangle intersects the rectangle.
func intersectRect(rect rect, point Point) Point {
	x := rect.x
	y := rect.y

	// Rectangle intersection algorithm from:
	// http://math.stackexchange.com/questions/108113/find-edge-between-two-boxes
	dx := point.X - x
	dy := point.Y - y
	w := float64(rect.width / 2)
	h := float64(rect.height / 2)

	if !truthy(dx) && !truthy(dy) {
		panic(typeError("Not possible to find intersection inside of the rectangle"))
	}

	var sx, sy float64
	if math.Abs(dy)*w > math.Abs(dx)*h {
		// Intersection is top or bottom of rect.
		if dy < 0 {
			h = -h
		}
		sx = (h * dx) / dy
		sy = h
	} else {
		// Intersection is left or right of rect.
		if dx < 0 {
			w = -w
		}
		sx = w
		sy = (w * dy) / dx
	}

	return Point{X: x + sx, Y: y + sy}
}

// buildLayerMatrix returns a matrix with the ids of the nodes of each rank,
// in the order of the "order" attribute. A hole is the empty string.
func buildLayerMatrix(g *graphlib.Graph) [][]string {
	layering := make([][]string, len(rangeEnd(orNaN(maxRank(g))+1)))
	for i := range layering {
		layering[i] = []string{}
	}
	for _, v := range g.Nodes() {
		node := nodeOf(g, v)
		rank := node.rank
		if node.hasRank {
			layer := layerAt(layering, rank)
			if i, ok := arrayIndex(node.order); ok {
				for len(*layer) <= i {
					*layer = append(*layer, "")
				}
				(*layer)[i] = v
			}
		}
	}
	return layering
}

// layerAt returns layering[rank]. JavaScript throws a TypeError on the use
// of the result when rank is not the index of a layer.
func layerAt(layering [][]string, rank float64) *[]string {
	i, ok := arrayIndex(rank)
	if !ok || i >= len(layering) {
		panic(typeError("Cannot read properties of undefined (layer of rank " + fmtNum(rank) + ")"))
	}
	return &layering[i]
}

// normalizeRanks adjusts the ranks so that all are >= 0 and at least one
// node has the rank 0.
func normalizeRanks(g *graphlib.Graph) {
	ranks := make([]float64, 0, g.NodeCount())
	for _, v := range g.Nodes() {
		ranks = append(ranks, nodeOf(g, v).rank)
	}
	min := orNaN(lodashMin(ranks))
	for _, v := range g.Nodes() {
		node := nodeOf(g, v)
		if node.hasRank {
			node.rank -= min
		}
	}
}

func removeEmptyRanks(g *graphlib.Graph) {
	// Ranks may not start at 0, so we need to offset them.
	ranks := make([]float64, 0, g.NodeCount())
	for _, v := range g.Nodes() {
		ranks = append(ranks, nodeOf(g, v).rank)
	}
	offset := orNaN(lodashMin(ranks))

	// layers is a sparse array. A nil entry is a hole. A rank that is not
	// an array index (NaN for a node without a rank, or a fraction for some
	// edge label proxies) becomes a property that the loop below does not
	// visit, so the port drops it.
	layers := [][]string{}
	for _, v := range g.Nodes() {
		rank := nodeOf(g, v).rank - offset
		i, ok := arrayIndex(rank)
		if !ok {
			continue
		}
		for len(layers) <= i {
			layers = append(layers, nil)
		}
		layers[i] = append(layers[i], v)
	}

	delta := 0.0
	nodeRankFactor := graphOf(g).nodeRankFactor
	for i, vs := range layers {
		if vs == nil && math.Mod(float64(i), nodeRankFactor) != 0 {
			delta--
		} else if delta != 0 {
			for _, v := range vs {
				nodeOf(g, v).rank += delta
			}
		}
	}
}

// addBorderNode is the two-argument form of the JavaScript function: the
// node has no rank and no order.
func (l *layout) addBorderNode(g *graphlib.Graph, prefix string) string {
	node := newNodeLabel()
	node.width = 0
	node.height = 0
	return l.addDummyNode(g, "border", node, prefix)
}

// maxRank returns the greatest rank. ok is false when no node has a rank.
func maxRank(g *graphlib.Graph) (float64, bool) {
	ranks := make([]float64, 0, g.NodeCount())
	for _, v := range g.Nodes() {
		node := nodeOf(g, v)
		if node.hasRank {
			ranks = append(ranks, node.rank)
		}
	}
	return lodashMax(ranks)
}
