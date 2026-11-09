// Ported from mermaid 12.2.0, rendering-util/layout-algorithms/dagre/index.js
// and mermaid-graphlib.js: the objects that these files store in the graph
// (MIT).

package dagre

import (
	"maps"
	"slices"

	dagrelib "github.com/bvolpato/mermaid-go-renderer/internal/dagre"
	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// Node is the label of a node of the layout graph.
//
// Upstream stores the node object of mermaid as the label, and the same
// object is the clusterData of a cluster, the node of a clusterDb entry,
// the argument of insertNode, insertCluster and positionNode, and the
// object that dagre writes x, y, width and height to. Here the embedded
// *ru.Node is that object: it is shared in the same way, and one *Node
// stands for one JavaScript object.
//
// dagre reads and writes its own attribute struct (see toDagre and
// fromDagre): the sizes are copied into it right before a layout, and the
// result is copied back to the ru.Node right after it.
type Node struct {
	*ru.Node

	// ClusterData is node.clusterData of the label that extractor makes
	// for a cluster that is rendered as a nested graph: the label that the
	// cluster had before.
	ClusterData *Node
	// Graph is node.graph of such a label: the nested graph.
	Graph *graphlib.Graph

	attrs dagrelib.NodeLabel
}

// DagreNode returns the attributes that dagre reads and writes.
func (n *Node) DagreNode() *dagrelib.NodeLabel { return &n.attrs }

// SelfLoop is edge.selfLoop of the three edges that stand for one
// self-loop: the id of the self-loop and the place of the segment.
type SelfLoop struct {
	ID    string
	Order int
}

// Edge is the label of an edge of the layout graph. The embedded *ru.Edge
// is the edge object of mermaid, see Node.
type Edge struct {
	*ru.Edge

	// OriginalEdge is edge.originalEdge of a self-loop segment: a copy of
	// the edge that the three segments stand for.
	OriginalEdge *ru.Edge
	// SelfLoop is edge.selfLoop, or nil.
	SelfLoop *SelfLoop

	attrs dagrelib.EdgeLabel
}

// DagreEdge returns the attributes that dagre reads and writes.
func (e *Edge) DagreEdge() *dagrelib.EdgeLabel { return &e.attrs }

// GraphLabel is the label of a layout graph: the object of setGraph.
type GraphLabel struct {
	// Rankdir is rankdir. Empty stands for undefined.
	Rankdir string
	// Nodesep and Ranksep are dynamic values (see internal/js/value.go),
	// because the top graph takes them from the config as they are. A nil
	// value is undefined, and dagre makes NaN of it.
	Nodesep any
	Ranksep any
	Marginx float64
	Marginy float64
	// Width and Height are the size of the laid-out graph. They are
	// undefined (NaN) before the layout.
	Width  float64
	Height float64

	attrs dagrelib.GraphLabel
}

// DagreGraph returns the attributes that dagre reads and writes.
func (l *GraphLabel) DagreGraph() *dagrelib.GraphLabel { return &l.attrs }

func newGraphLabel(rankdir string, nodesep, ranksep any) *GraphLabel {
	return &GraphLabel{
		Rankdir: rankdir, Nodesep: nodesep, Ranksep: ranksep,
		Marginx: 8, Marginy: 8, Width: ru.Undef, Height: ru.Undef,
	}
}

// newGraph is `new graphlib.Graph({ multigraph: true, compound: true })
// .setGraph(label).setDefaultEdgeLabel(() => ({}))`.
func newGraph(label *GraphLabel) *graphlib.Graph {
	return graphlib.New(graphlib.Options{Multigraph: true, Compound: true}).
		SetGraph(label).
		SetDefaultEdgeLabel(func(string, string, string) any {
			return &Edge{Edge: ru.NewEdge("")}
		})
}

// nodeOf is graph.node(v). The result is nil for a label that is undefined.
func nodeOf(g *graphlib.Graph, v string) *Node {
	n, _ := g.Node(v).(*Node)
	return n
}

// edgeOf is graph.edge(e). The result is nil for a label that is undefined.
func edgeOf(g *graphlib.Graph, e graphlib.Edge) *Edge {
	l, _ := g.EdgeObj(e).(*Edge)
	return l
}

// graphOf is graph.graph(). The result is nil for a label that is undefined.
func graphOf(g *graphlib.Graph) *GraphLabel {
	l, _ := g.Graph().(*GraphLabel)
	return l
}

// absentZero is the number that dagre reads from a size property. An
// undefined size of a ru.Node or ru.Edge stands for a property that the
// JavaScript object does not have, and dagre takes 0 for it.
func absentZero(v float64) float64 {
	if ru.IsUndef(v) {
		return 0
	}
	return v
}

// extraNum reads a number property that the ru types do not have from the
// extra properties. dagre converts the value with Number().
func extraNum(extra map[string]any, key string) dagrelib.Num {
	v, ok := extra[key]
	if !ok {
		return dagrelib.Num{}
	}
	return dagrelib.N(js.ToNumber(v))
}

// toDagre copies the attributes that dagre reads from the labels of the
// graph into the structs that dagre works on.
func toDagre(g *graphlib.Graph) {
	if label := graphOf(g); label != nil {
		label.attrs = dagrelib.GraphLabel{
			Rankdir: label.Rankdir,
			// The properties nodesep and ranksep always exist, so an
			// undefined value is NaN and not the default of dagre.
			Nodesep: dagrelib.N(js.ToNumber(label.Nodesep)),
			Ranksep: dagrelib.N(js.ToNumber(label.Ranksep)),
			Marginx: label.Marginx,
			Marginy: label.Marginy,
		}
	}
	for _, v := range g.Nodes() {
		if node := nodeOf(g, v); node != nil {
			node.attrs = dagrelib.NodeLabel{
				Width:  absentZero(node.Width),
				Height: absentZero(node.Height),
			}
		}
	}
	for _, e := range g.Edges() {
		edge := edgeOf(g, e)
		if edge == nil {
			continue
		}
		attrs := dagrelib.EdgeLabel{
			Weight:      extraNum(edge.Extra, "weight"),
			Width:       absentZero(edge.Width),
			Height:      absentZero(edge.Height),
			Labeloffset: extraNum(edge.Extra, "labeloffset"),
			Labelpos:    edge.Labelpos,
		}
		if !ru.IsUndef(edge.Minlen) {
			attrs.Minlen = dagrelib.N(edge.Minlen)
		}
		edge.attrs = attrs
	}
}

// fromDagre copies the result of a layout from the structs of dagre to the
// labels: the position of every node, the size of every node that has
// children, the points of every edge, the position of every edge label
// that got one, and the size of the graph.
func fromDagre(g *graphlib.Graph) {
	for _, v := range g.Nodes() {
		node := nodeOf(g, v)
		if node == nil {
			continue
		}
		node.X = node.attrs.X
		node.Y = node.attrs.Y
		if len(g.Children(v)) > 0 {
			node.Width = node.attrs.Width
			node.Height = node.attrs.Height
		}
	}
	for _, e := range g.Edges() {
		edge := edgeOf(g, e)
		if edge == nil {
			continue
		}
		points := make([]ru.Point, len(edge.attrs.Points))
		for i, p := range edge.attrs.Points {
			points[i] = ru.Point{X: p.X, Y: p.Y}
		}
		edge.Points = points
		if edge.attrs.X.Set {
			edge.X = edge.attrs.X.V
			edge.Y = edge.attrs.Y.V
		}
	}
	if label := graphOf(g); label != nil {
		label.Width = label.attrs.Width
		label.Height = label.attrs.Height
	}
}

// addNumber is `v + n` for a dynamic value v: a string is concatenated
// with the number, as JavaScript does, and every other value is converted
// to a number.
func addNumber(v any, n float64) any {
	if s, ok := v.(string); ok {
		return s + js.Str(n)
	}
	return js.ToNumber(v) + n
}

// cloneNodeJSON is JSON.parse(JSON.stringify(node)) for the node object of
// a cluster: a deep copy without the functions. A number that is undefined
// stays undefined.
func cloneNodeJSON(node *ru.Node) *ru.Node {
	c := *node
	c.Description = slices.Clone(node.Description)
	c.CSSStyles = slices.Clone(node.CSSStyles)
	c.CSSCompiledStyles = slices.Clone(node.CSSCompiledStyles)
	if node.LabelBBox != nil {
		b := *node.LabelBBox
		c.LabelBBox = &b
	}
	if node.GroupTitleRect != nil {
		r := *node.GroupTitleRect
		c.GroupTitleRect = &r
	}
	c.Intersect = nil
	c.CalcIntersect = nil
	if node.Metadata != nil {
		c.Metadata = js.AsObj(js.Clone(node.Metadata))
	}
	if node.Children != nil {
		c.Children = make([]*ru.Node, len(node.Children))
		for i, child := range node.Children {
			if child != nil {
				c.Children[i] = cloneNodeJSON(child)
			}
		}
	}
	c.Extra = cloneExtra(node.Extra)
	return &c
}

// structuredCloneEdge is structuredClone(edge): a deep copy.
func structuredCloneEdge(edge *ru.Edge) *ru.Edge {
	c := *edge
	c.Style = slices.Clone(edge.Style)
	c.CSSCompiledStyles = slices.Clone(edge.CSSCompiledStyles)
	c.LabelStyle = slices.Clone(edge.LabelStyle)
	c.CSSStyles = slices.Clone(edge.CSSStyles)
	c.Points = slices.Clone(edge.Points)
	c.TerminalLabelSizes = maps.Clone(edge.TerminalLabelSizes)
	c.TerminalLabelCenters = maps.Clone(edge.TerminalLabelCenters)
	if edge.Metadata != nil {
		c.Metadata = js.AsObj(js.Clone(edge.Metadata))
	}
	c.Extra = cloneExtra(edge.Extra)
	return &c
}

// cloneExtra copies the extra properties. Dynamic values are copied deep.
func cloneExtra(extra map[string]any) map[string]any {
	if extra == nil {
		return nil
	}
	out := make(map[string]any, len(extra))
	for k, v := range extra {
		out[k] = js.Clone(v)
	}
	return out
}
