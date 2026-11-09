// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/layout.js (the
// attribute lists of buildLayoutGraph and updateInputGraph) (MIT).

// Package dagre is a port of the dagre layout of dagre-d3-es 7.0.14.
//
// Layout returns the same coordinates as the JavaScript layout(), bit for
// bit. The port keeps the iteration order of JavaScript objects, the
// floating-point expression shapes, the lodash helper semantics and the
// sort algorithm of V8.
package dagre

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
)

// Num is a JavaScript number property that can be absent. The zero value is
// an absent property. dagre uses its default for an absent input property.
//
// A JavaScript label that has the property with the value undefined is not
// absent: dagre converts the value with Number(), which gives NaN. Use
// N(math.NaN()) for that case.
type Num struct {
	V   float64
	Set bool
}

// N returns a number property that is present.
func N(v float64) Num { return Num{V: v, Set: true} }

// Point is one point of an edge path.
type Point struct {
	X float64
	Y float64
}

// GraphLabel holds the attributes of the graph label that dagre reads and
// writes. Property names in JavaScript are the lower-case field names.
type GraphLabel struct {
	// Rankdir is "TB", "BT", "LR" or "RL". Empty is the default "tb".
	//
	// The default is not the same as "TB". For "TB" and "BT" in upper case,
	// dagre adds the label offset to the width of an edge label. For every
	// other value, also the default, it adds the offset to the height.
	Rankdir string
	// Nodesep is the separation between nodes of one rank. The default is 50.
	Nodesep Num
	// Edgesep is the separation between edges of one rank. The default is 20.
	Edgesep Num
	// Ranksep is the separation between ranks. The default is 50.
	Ranksep Num
	// Marginx and Marginy are the margins around the graph. An absent
	// JavaScript property is 0, and so are undefined and NaN.
	Marginx float64
	Marginy float64
	// Acyclicer is "greedy" or empty.
	Acyclicer string
	// Ranker is "network-simplex", "tight-tree" or "longest-path". Empty is
	// "network-simplex".
	Ranker string
	// Align is "UL", "UR", "DL" or "DR" in any case, or empty.
	Align string

	// Width and Height are the size of the laid out graph. Layout sets them.
	Width  float64
	Height float64
}

// NodeLabel holds the attributes of a node label that dagre reads and writes.
type NodeLabel struct {
	// Width and Height are the size of the node. An absent JavaScript
	// property is 0. A property with the value undefined is NaN, see Num.
	// Layout sets both for a node that has children.
	Width  float64
	Height float64

	// X and Y are the center of the node. Layout sets them.
	X float64
	Y float64
}

// EdgeLabel holds the attributes of an edge label that dagre reads and writes.
type EdgeLabel struct {
	// Minlen is the minimum number of ranks between the two nodes. The
	// default is 1.
	Minlen Num
	// Weight is the weight of the edge. The default is 1.
	Weight Num
	// Width and Height are the size of the edge label. An absent JavaScript
	// property is 0. A property with the value undefined is NaN, see Num.
	Width  float64
	Height float64
	// Labeloffset is the distance between the label and the edge. The
	// default is 10.
	Labeloffset Num
	// Labelpos is "l", "c" or "r". Empty is the default "r". dagre accepts
	// the upper-case letters in some stages only, as the JavaScript does.
	Labelpos string

	// Points is the path of the edge. Layout sets it.
	Points []Point
	// X and Y are the center of the edge label. Layout sets them only for
	// an edge whose label got a position: a label with a non-zero width
	// and height, or a self-loop. Otherwise Layout does not change them.
	X Num
	Y Num
}

// GraphAttrs is the interface of a graph label that Layout accepts. A type
// that embeds GraphLabel implements it.
type GraphAttrs interface {
	DagreGraph() *GraphLabel
}

// NodeAttrs is the interface of a node label that Layout accepts. A type
// that embeds NodeLabel implements it.
type NodeAttrs interface {
	DagreNode() *NodeLabel
}

// EdgeAttrs is the interface of an edge label that Layout accepts. A type
// that embeds EdgeLabel implements it.
type EdgeAttrs interface {
	DagreEdge() *EdgeLabel
}

// DagreGraph returns the receiver.
func (l *GraphLabel) DagreGraph() *GraphLabel { return l }

// DagreNode returns the receiver.
func (l *NodeLabel) DagreNode() *NodeLabel { return l }

// DagreEdge returns the receiver.
func (l *EdgeLabel) DagreEdge() *EdgeLabel { return l }

// The labels of the layout graph follow. JavaScript adds and removes
// properties on these objects as the layout runs. In the port, a number
// property that does not exist is NaN: an undefined value and NaN give the
// same result in arithmetic, in comparisons, in a truth test and in _.min
// and _.max. A property has an explicit flag where the JavaScript code tests
// whether it exists.

var nan = math.NaN()

// graphLabel is the label of the layout graph.
type graphLabel struct {
	ranksep float64
	edgesep float64
	nodesep float64
	marginx float64
	marginy float64
	rankdir string

	acyclicer string
	ranker    string
	align     string

	nestingRoot    string
	nodeRankFactor float64
	dummyChains    []string

	width  float64
	height float64
}

type selfEdge struct {
	e     graphlib.Edge
	label *edgeLabel
}

// nodeLabel is the label of a node of the layout graph.
type nodeLabel struct {
	width  float64
	height float64
	x      float64
	y      float64

	rank    float64
	hasRank bool
	order   float64

	// dummy is the type of a dummy node: "root", "border", "edge",
	// "edge-label", "edge-proxy" or "selfedge". It is empty for a real node.
	dummy string

	// Attributes of a node that has children.
	borderTop    string
	borderBottom string
	// borderLeft and borderRight are sparse arrays indexed by rank. A hole
	// is the empty string.
	borderLeft  []string
	borderRight []string
	minRank     float64
	maxRank     float64
	hasMinRank  bool

	selfEdges []selfEdge

	// Attributes of dummy nodes.
	e           graphlib.Edge // "edge-proxy" and "selfedge"
	label       *edgeLabel    // "selfedge"
	edgeLabel   *edgeLabel    // "edge" and "edge-label"
	edgeObj     graphlib.Edge // "edge" and "edge-label"
	labelpos    string        // "edge-label"
	hasLabelpos bool
	borderType  string // "border" nodes of border segments
}

// newNodeLabel returns a node label without properties ({} in JavaScript).
func newNodeLabel() *nodeLabel {
	return &nodeLabel{width: nan, height: nan, x: nan, y: nan, rank: nan, order: nan, minRank: nan, maxRank: nan}
}

// edgeLabel is the label of an edge of the layout graph.
type edgeLabel struct {
	minlen      float64
	weight      float64
	width       float64
	height      float64
	labeloffset float64
	labelpos    string

	x float64
	y float64
	// hasXY is true when the properties x and y exist.
	hasXY bool
	// points is nil when the property does not exist.
	points    []Point
	labelRank float64

	reversed    bool
	forwardName string
	nestingEdge bool
}

// newEdgeLabel returns an edge label without properties ({} in JavaScript).
func newEdgeLabel() *edgeLabel {
	return &edgeLabel{minlen: nan, weight: nan, width: nan, height: nan, labeloffset: nan, x: nan, y: nan, labelRank: nan}
}

// truthy is the JavaScript truth test of a number: not 0 and not NaN.
func truthy(x float64) bool {
	return x != 0 && x == x
}

// typeError stands for the TypeError that JavaScript throws for a property
// access on undefined. Layout recovers it and returns an error.
type typeError string

func (e typeError) Error() string { return string(e) }

func nodeOf(g *graphlib.Graph, v string) *nodeLabel {
	n, _ := g.Node(v).(*nodeLabel)
	return n
}

func edgeOf(g *graphlib.Graph, e graphlib.Edge) *edgeLabel {
	l, _ := g.EdgeObj(e).(*edgeLabel)
	return l
}

func edgeOfVW(g *graphlib.Graph, v, w string) *edgeLabel {
	l, _ := g.Edge(v, w).(*edgeLabel)
	return l
}

func graphOf(g *graphlib.Graph) *graphLabel {
	l, _ := g.Graph().(*graphLabel)
	return l
}

// setEdge is g.setEdge(v, w, label, name). An empty name is undefined:
// names here come from edge objects, which never have an empty name.
func setEdge(g *graphlib.Graph, v, w string, label any, name string) {
	var err error
	if name != "" {
		err = g.SetEdge(v, w, label, name)
	} else {
		err = g.SetEdge(v, w, label)
	}
	if err != nil {
		panic(err)
	}
}

func setEdgeObj(g *graphlib.Graph, e graphlib.Edge, label any) {
	if err := g.SetEdgeObj(e, label); err != nil {
		panic(err)
	}
}

// setParent is g.setParent(v, parent). ok is false for an undefined parent.
func setParent(g *graphlib.Graph, v, parent string, ok bool) {
	var err error
	if ok {
		err = g.SetParent(v, parent)
	} else {
		err = g.SetParent(v)
	}
	if err != nil {
		panic(err)
	}
}

// objectPrototypeKeys are the property names of Object.prototype. dagre
// keeps some maps from node id to value in plain objects and reads them
// without a hasOwnProperty test. For these ids the read returns an inherited
// function, not undefined. The port follows this behavior where it changes
// the result.
var objectPrototypeKeys = map[string]string{
	"constructor":          "function Object() { [native code] }",
	"__defineGetter__":     "function __defineGetter__() { [native code] }",
	"__defineSetter__":     "function __defineSetter__() { [native code] }",
	"hasOwnProperty":       "function hasOwnProperty() { [native code] }",
	"__lookupGetter__":     "function __lookupGetter__() { [native code] }",
	"__lookupSetter__":     "function __lookupSetter__() { [native code] }",
	"isPrototypeOf":        "function isPrototypeOf() { [native code] }",
	"propertyIsEnumerable": "function propertyIsEnumerable() { [native code] }",
	"toString":             "function toString() { [native code] }",
	"valueOf":              "function valueOf() { [native code] }",
	"__proto__":            "[object Object]",
	"toLocaleString":       "function toLocaleString() { [native code] }",
}

func isObjectPrototypeKey(k string) bool {
	_, ok := objectPrototypeKeys[k]
	return ok
}
