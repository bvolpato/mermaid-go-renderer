// Package dagre is a Go port of the dagre layout engine for directed graphs.
//
// Original JS source: https://github.com/dagrejs/dagre (MIT License)
// Copyright (c) 2012-2014 Chris Pettitt
package dagre

import "math"

// Edge identifies a directed edge between two nodes.
type Edge struct {
	V, W string
	Name  string // for multigraph support
}

// edgeKey returns a canonical key for storing/looking-up an edge.
func edgeKey(v, w, name string) string {
	return v + "\x00" + w + "\x00" + name
}

// Point is an x/y coordinate.
type Point struct {
	X, Y float64
}

// NodeLabel carries all properties for a node used during layout.
type NodeLabel struct {
	Width  float64
	Height float64
	X      float64
	Y      float64
	Rank   int
	Order  int

	HasRank  bool
	HasOrder bool
	HasX     bool
	HasY     bool

	Dummy      string // "edge","border","edge-label","edge-proxy","selfedge","root"
	BorderType string // "borderLeft","borderRight"
	BorderTop  string
	BorderBot  string
	BorderLeft  []string
	BorderRight []string
	MinRank    int
	MaxRank    int
	HasMinRank bool
	HasMaxRank bool

	Label    string
	LabelPos string // "l","c","r"
	Padding  float64
	PaddingX float64
	PaddingY float64

	EdgeLabel *EdgeLabel // for dummy nodes
	EdgeObj   *Edge      // for dummy nodes

	SelfEdges []SelfEdge

	Extra map[string]interface{}
}

// SelfEdge stores a self-edge that was removed from the graph.
type SelfEdge struct {
	E     Edge
	Label EdgeLabel
}

// EdgeLabel carries all properties for an edge used during layout.
type EdgeLabel struct {
	Points      []Point
	Width       float64
	Height      float64
	MinLen      int
	Weight      int
	LabelPos    string // "l","c","r"
	LabelOffset float64
	LabelRank   int
	X           float64
	Y           float64

	HasX         bool
	HasY         bool
	HasLabelRank bool

	Reversed    bool
	ForwardName string
	SelfEdge    bool
	NestingEdge bool
	Cutvalue    int
	HasCutvalue bool

	EdgeLabel *EdgeLabel
	EdgeObj   *Edge
}

// GraphLabel holds graph-level configuration for layout.
type GraphLabel struct {
	Width       float64
	Height      float64
	Compound    bool
	RankDir     string // "TB","BT","LR","RL"
	Align       string // "UL","UR","DL","DR"
	RankAlign   string // "top","center","bottom"
	NodeSep     float64
	EdgeSep     float64
	RankSep     float64
	MarginX     float64
	MarginY     float64
	Acyclicer   string // "greedy"
	Ranker      string // "network-simplex","tight-tree","longest-path"
	NestingRoot string
	NodeRankFactor int
	DummyChains []string
	MaxRank     int
}

// Graph is a directed multigraph with compound (parent/child) support.
// This is a Go port of @dagrejs/graphlib's Graph class.
type Graph struct {
	isDirected  bool
	isMultigraph bool
	isCompound  bool

	label GraphLabel

	nodes      map[string]*NodeLabel
	nodeOrder  []string // insertion order
	inEdges    map[string]map[string]bool // node -> set of edge keys
	outEdges   map[string]map[string]bool
	edges      map[string]*EdgeLabel      // edgeKey -> label
	edgeObjs   map[string]Edge            // edgeKey -> Edge

	parent   map[string]string   // child -> parent
	children map[string]map[string]bool // parent -> children (GRAPH_NODE = "\x00")
}

const graphNode = "\x00" // sentinel for root parent

// NewGraph creates a new directed multigraph with compound support.
func NewGraph() *Graph {
	g := &Graph{
		isDirected:  true,
		isMultigraph: true,
		isCompound:  true,
		nodes:       make(map[string]*NodeLabel),
		inEdges:     make(map[string]map[string]bool),
		outEdges:    make(map[string]map[string]bool),
		edges:       make(map[string]*EdgeLabel),
		edgeObjs:    make(map[string]Edge),
		parent:      make(map[string]string),
		children:    make(map[string]map[string]bool),
	}
	g.children[graphNode] = make(map[string]bool)
	g.label = GraphLabel{
		RankDir:   "TB",
		RankAlign: "center",
		NodeSep:   50,
		EdgeSep:   20,
		RankSep:   50,
	}
	return g
}

// NewSimpleGraph creates a non-multigraph, non-compound directed graph.
func NewSimpleGraph() *Graph {
	g := NewGraph()
	g.isMultigraph = false
	g.isCompound = false
	return g
}

// NewUndirectedGraph creates an undirected graph (for spanning trees).
func NewUndirectedGraph() *Graph {
	g := NewGraph()
	g.isDirected = false
	g.isMultigraph = false
	g.isCompound = false
	return g
}

func (g *Graph) SetGraph(label GraphLabel) { g.label = label }
func (g *Graph) GraphLabel() *GraphLabel    { return &g.label }
func (g *Graph) IsMultigraph() bool         { return g.isMultigraph }
func (g *Graph) IsCompound() bool           { return g.isCompound }
func (g *Graph) NodeCount() int             { return len(g.nodes) }
func (g *Graph) EdgeCount() int             { return len(g.edges) }

func (g *Graph) SetNode(v string, label *NodeLabel) {
	if _, exists := g.nodes[v]; !exists {
		g.nodeOrder = append(g.nodeOrder, v)
		g.inEdges[v] = make(map[string]bool)
		g.outEdges[v] = make(map[string]bool)
		if g.isCompound {
			g.parent[v] = graphNode
			g.children[graphNode][v] = true
			if g.children[v] == nil {
				g.children[v] = make(map[string]bool)
			}
		}
	}
	g.nodes[v] = label
}

func (g *Graph) Node(v string) *NodeLabel {
	return g.nodes[v]
}

func (g *Graph) HasNode(v string) bool {
	_, ok := g.nodes[v]
	return ok
}

func (g *Graph) RemoveNode(v string) {
	if !g.HasNode(v) {
		return
	}
	// Remove incident edges
	for ek := range g.inEdges[v] {
		delete(g.edges, ek)
		e := g.edgeObjs[ek]
		delete(g.edgeObjs, ek)
		if out, ok := g.outEdges[e.V]; ok {
			delete(out, ek)
		}
	}
	delete(g.inEdges, v)
	for ek := range g.outEdges[v] {
		delete(g.edges, ek)
		e := g.edgeObjs[ek]
		delete(g.edgeObjs, ek)
		if in, ok := g.inEdges[e.W]; ok {
			delete(in, ek)
		}
	}
	delete(g.outEdges, v)

	delete(g.nodes, v)
	// Remove from nodeOrder
	for i, n := range g.nodeOrder {
		if n == v {
			g.nodeOrder = append(g.nodeOrder[:i], g.nodeOrder[i+1:]...)
			break
		}
	}

	if g.isCompound {
		// Re-parent children to parent of v
		parent := g.parent[v]
		for child := range g.children[v] {
			g.parent[child] = parent
			g.children[parent][child] = true
		}
		delete(g.children, v)
		delete(g.children[parent], v)
		delete(g.parent, v)
	}
}

func (g *Graph) Nodes() []string {
	result := make([]string, len(g.nodeOrder))
	copy(result, g.nodeOrder)
	return result
}

func (g *Graph) Sources() []string {
	var result []string
	for _, v := range g.nodeOrder {
		if len(g.inEdges[v]) == 0 {
			result = append(result, v)
		}
	}
	return result
}

func (g *Graph) SetParent(v, parent string) {
	if !g.isCompound {
		return
	}
	if parent == "" {
		parent = graphNode
	}
	// Ensure parent node exists (except for root sentinel)
	if parent != graphNode && !g.HasNode(parent) {
		g.SetNode(parent, &NodeLabel{})
	}
	oldParent := g.parent[v]
	g.parent[v] = parent
	delete(g.children[oldParent], v)
	if g.children[parent] == nil {
		g.children[parent] = make(map[string]bool)
	}
	g.children[parent][v] = true
}

func (g *Graph) Parent(v string) string {
	if !g.isCompound {
		return ""
	}
	p, ok := g.parent[v]
	if !ok || p == graphNode {
		return ""
	}
	return p
}

func (g *Graph) Children(v string) []string {
	if !g.isCompound {
		return nil
	}
	if v == "" {
		v = graphNode
	}
	ch, ok := g.children[v]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(ch))
	for c := range ch {
		result = append(result, c)
	}
	return result
}

func (g *Graph) SetEdge(e Edge, label *EdgeLabel) {
	ek := edgeKey(e.V, e.W, e.Name)

	if !g.HasNode(e.V) {
		g.SetNode(e.V, &NodeLabel{})
	}
	if !g.HasNode(e.W) {
		g.SetNode(e.W, &NodeLabel{})
	}

	g.edges[ek] = label
	g.edgeObjs[ek] = e
	g.outEdges[e.V][ek] = true
	if g.isDirected {
		g.inEdges[e.W][ek] = true
	} else {
		g.inEdges[e.V][ek] = true
		g.outEdges[e.W][ek] = true
		g.inEdges[e.W][ek] = true
	}
}

func (g *Graph) SetEdgeVW(v, w string, label *EdgeLabel) {
	g.SetEdge(Edge{V: v, W: w}, label)
}

func (g *Graph) SetEdgeNamed(v, w, name string, label *EdgeLabel) {
	g.SetEdge(Edge{V: v, W: w, Name: name}, label)
}

func (g *Graph) EdgeByKey(e Edge) *EdgeLabel {
	ek := edgeKey(e.V, e.W, e.Name)
	return g.edges[ek]
}

func (g *Graph) EdgeVW(v, w string) *EdgeLabel {
	return g.EdgeByKey(Edge{V: v, W: w})
}

func (g *Graph) HasEdge(v, w string) bool {
	ek := edgeKey(v, w, "")
	_, ok := g.edges[ek]
	return ok
}

func (g *Graph) HasEdgeObj(e Edge) bool {
	ek := edgeKey(e.V, e.W, e.Name)
	_, ok := g.edges[ek]
	return ok
}

func (g *Graph) RemoveEdge(e Edge) {
	ek := edgeKey(e.V, e.W, e.Name)
	if _, ok := g.edges[ek]; !ok {
		return
	}
	delete(g.edges, ek)
	delete(g.edgeObjs, ek)
	if out, ok := g.outEdges[e.V]; ok {
		delete(out, ek)
	}
	if g.isDirected {
		if in, ok := g.inEdges[e.W]; ok {
			delete(in, ek)
		}
	} else {
		if in, ok := g.inEdges[e.V]; ok {
			delete(in, ek)
		}
		if out, ok := g.outEdges[e.W]; ok {
			delete(out, ek)
		}
		if in, ok := g.inEdges[e.W]; ok {
			delete(in, ek)
		}
	}
}

func (g *Graph) RemoveEdgeVW(v, w string) {
	g.RemoveEdge(Edge{V: v, W: w})
}

func (g *Graph) Edges() []Edge {
	result := make([]Edge, 0, len(g.edgeObjs))
	for _, e := range g.edgeObjs {
		result = append(result, e)
	}
	return result
}

func (g *Graph) InEdges(v string) []Edge {
	in, ok := g.inEdges[v]
	if !ok {
		return nil
	}
	result := make([]Edge, 0, len(in))
	for ek := range in {
		result = append(result, g.edgeObjs[ek])
	}
	return result
}

func (g *Graph) OutEdges(v string) []Edge {
	out, ok := g.outEdges[v]
	if !ok {
		return nil
	}
	result := make([]Edge, 0, len(out))
	for ek := range out {
		result = append(result, g.edgeObjs[ek])
	}
	return result
}

func (g *Graph) OutEdgesVW(v, w string) []Edge {
	out, ok := g.outEdges[v]
	if !ok {
		return nil
	}
	var result []Edge
	for ek := range out {
		e := g.edgeObjs[ek]
		if e.W == w {
			result = append(result, e)
		}
	}
	return result
}

func (g *Graph) NodeEdges(v string) []Edge {
	var result []Edge
	seen := make(map[string]bool)
	for ek := range g.inEdges[v] {
		if !seen[ek] {
			seen[ek] = true
			result = append(result, g.edgeObjs[ek])
		}
	}
	for ek := range g.outEdges[v] {
		if !seen[ek] {
			seen[ek] = true
			result = append(result, g.edgeObjs[ek])
		}
	}
	return result
}

func (g *Graph) Predecessors(v string) []string {
	in, ok := g.inEdges[v]
	if !ok {
		return nil
	}
	seen := make(map[string]bool)
	var result []string
	for ek := range in {
		e := g.edgeObjs[ek]
		u := e.V
		if u == v {
			u = e.W
		}
		if !seen[u] {
			seen[u] = true
			result = append(result, u)
		}
	}
	return result
}

func (g *Graph) Successors(v string) []string {
	out, ok := g.outEdges[v]
	if !ok {
		return nil
	}
	seen := make(map[string]bool)
	var result []string
	for ek := range out {
		e := g.edgeObjs[ek]
		w := e.W
		if w == v {
			w = e.V
		}
		if !seen[w] {
			seen[w] = true
			result = append(result, w)
		}
	}
	return result
}

func (g *Graph) Neighbors(v string) []string {
	seen := make(map[string]bool)
	var result []string
	for ek := range g.inEdges[v] {
		e := g.edgeObjs[ek]
		u := e.V
		if u == v {
			u = e.W
		}
		if !seen[u] {
			seen[u] = true
			result = append(result, u)
		}
	}
	for ek := range g.outEdges[v] {
		e := g.edgeObjs[ek]
		w := e.W
		if w == v {
			w = e.V
		}
		if !seen[w] {
			seen[w] = true
			result = append(result, w)
		}
	}
	return result
}

// FilterNodes returns a copy with only nodes passing the filter.
func (g *Graph) FilterNodes(keep func(string) bool) *Graph {
	result := NewGraph()
	result.label = g.label
	result.isDirected = g.isDirected
	result.isMultigraph = g.isMultigraph
	result.isCompound = g.isCompound

	for _, v := range g.nodeOrder {
		if keep(v) {
			nl := *g.nodes[v]
			result.SetNode(v, &nl)
		}
	}
	for ek, e := range g.edgeObjs {
		if result.HasNode(e.V) && result.HasNode(e.W) {
			el := *g.edges[ek]
			result.SetEdge(e, &el)
		}
	}
	if g.isCompound {
		for _, v := range result.Nodes() {
			p := g.Parent(v)
			if p != "" && result.HasNode(p) {
				result.SetParent(v, p)
			}
		}
	}
	return result
}

// IntersectRect computes where a line from point to the center of rect
// intersects the rectangle boundary.
func IntersectRect(rect *NodeLabel, point Point) Point {
	x := rect.X
	y := rect.Y
	dx := point.X - x
	dy := point.Y - y
	w := rect.Width / 2
	h := rect.Height / 2

	if dx == 0 && dy == 0 {
		return Point{X: x, Y: y}
	}

	var sx, sy float64
	if math.Abs(dy)*w > math.Abs(dx)*h {
		if dy < 0 {
			h = -h
		}
		sx = h * dx / dy
		sy = h
	} else {
		if dx < 0 {
			w = -w
		}
		sx = w
		sy = w * dy / dx
	}
	return Point{X: x + sx, Y: y + sy}
}
