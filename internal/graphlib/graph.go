// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/graphlib/graph.js (MIT).

// Package graphlib is a port of the graphlib part of dagre-d3-es 7.0.14.
//
// The JavaScript graph stores nodes, edges and children in plain objects and
// enumerates them with Object.keys. The port keeps that enumeration order:
// integer-like ids first in numeric order, then the other ids in insertion
// order. Layout results depend on it.
//
// Labels are arbitrary values. A nil label is JavaScript undefined.
package graphlib

import (
	"errors"
	"math"
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

const (
	defaultEdgeName = "\x00"
	graphNode       = "\x00"
	edgeKeyDelim    = "\x01"
)

// Options are the constructor options of Graph. The zero value is the
// JavaScript default: a directed graph that is not a multigraph and not
// compound.
type Options struct {
	// Undirected is the inverse of the JavaScript option "directed".
	Undirected bool
	// Multigraph allows more than one edge between the same two nodes.
	// These edges have different names.
	Multigraph bool
	// Compound allows a node to be the parent of other nodes.
	Compound bool
}

// Edge identifies an edge. It is the JavaScript edge object {v, w, name}.
// Name is empty for an edge that has no name.
type Edge struct {
	V    string
	W    string
	Name string
}

// ErrNotCompound is the error of SetParent on a graph that is not compound.
var ErrNotCompound = errors.New("Cannot set parent in a non-compound graph")

// ErrNotMultigraph is the error of SetEdge with a name on a graph that is
// not a multigraph.
var ErrNotMultigraph = errors.New("Cannot set a named edge when isMultigraph = false")

// Graph is a directed or undirected graph with optional multi-edges and an
// optional node hierarchy.
type Graph struct {
	isDirected   bool
	isMultigraph bool
	isCompound   bool

	// label is the label of the graph itself.
	label any

	defaultNodeLabelFn func(v string) any
	defaultEdgeLabelFn func(v, w, name string) any

	// nodes maps v to the node label.
	nodes *js.Obj[any]

	// parent maps v to its parent. children maps v to the set of its
	// children. Both exist only in a compound graph.
	parent   map[string]string
	children map[string]*js.Obj[bool]

	// in maps v to the edges that end at v, keyed by edge id.
	in map[string]*js.Obj[Edge]
	// preds maps v to its predecessors with the number of edges from each.
	preds map[string]*js.Obj[float64]
	// out maps v to the edges that start at v, keyed by edge id.
	out map[string]*js.Obj[Edge]
	// sucs maps v to its successors with the number of edges to each.
	sucs map[string]*js.Obj[float64]

	// edgeObjs maps an edge id to the edge object.
	edgeObjs *js.Obj[Edge]
	// edgeLabels maps an edge id to the edge label.
	edgeLabels map[string]any

	nodeCount int
	edgeCount int
}

// New returns an empty graph.
func New(opts Options) *Graph {
	g := &Graph{
		isDirected:         !opts.Undirected,
		isMultigraph:       opts.Multigraph,
		isCompound:         opts.Compound,
		defaultNodeLabelFn: func(string) any { return nil },
		defaultEdgeLabelFn: func(string, string, string) any { return nil },
		nodes:              js.NewObj[any](),
		in:                 map[string]*js.Obj[Edge]{},
		preds:              map[string]*js.Obj[float64]{},
		out:                map[string]*js.Obj[Edge]{},
		sucs:               map[string]*js.Obj[float64]{},
		edgeObjs:           js.NewObj[Edge](),
		edgeLabels:         map[string]any{},
	}
	if g.isCompound {
		g.parent = map[string]string{}
		g.children = map[string]*js.Obj[bool]{}
		g.children[graphNode] = js.NewObj[bool]()
	}
	return g
}

/* === Graph functions ========= */

// IsDirected reports whether the graph is directed.
func (g *Graph) IsDirected() bool { return g.isDirected }

// IsMultigraph reports whether the graph allows named parallel edges.
func (g *Graph) IsMultigraph() bool { return g.isMultigraph }

// IsCompound reports whether the graph has a node hierarchy.
func (g *Graph) IsCompound() bool { return g.isCompound }

// SetGraph sets the label of the graph.
func (g *Graph) SetGraph(label any) *Graph {
	g.label = label
	return g
}

// Graph returns the label of the graph.
func (g *Graph) Graph() any { return g.label }

/* === Node functions ========== */

// SetDefaultNodeLabel sets the function that makes the label of a node that
// is created without a label. A nil function restores the default, which
// returns nil.
func (g *Graph) SetDefaultNodeLabel(newDefault func(v string) any) *Graph {
	if newDefault == nil {
		newDefault = func(string) any { return nil }
	}
	g.defaultNodeLabelFn = newDefault
	return g
}

// NodeCount is the number of nodes.
func (g *Graph) NodeCount() int { return g.nodeCount }

// Nodes returns the node ids in JavaScript object key order.
func (g *Graph) Nodes() []string { return g.nodes.Keys() }

// Sources returns the nodes that have no in-edges.
func (g *Graph) Sources() []string {
	res := []string{}
	for _, v := range g.Nodes() {
		if g.in[v].Len() == 0 {
			res = append(res, v)
		}
	}
	return res
}

// Sinks returns the nodes that have no out-edges.
func (g *Graph) Sinks() []string {
	res := []string{}
	for _, v := range g.Nodes() {
		if g.out[v].Len() == 0 {
			res = append(res, v)
		}
	}
	return res
}

// SetNodes calls SetNode for every id in vs.
func (g *Graph) SetNodes(vs []string, label ...any) *Graph {
	for _, v := range vs {
		g.SetNode(v, label...)
	}
	return g
}

// SetNode creates the node v when it does not exist. With a label argument
// it also sets the label of v. Without one, a new node gets the default
// label and the label of an existing node stays as it is.
func (g *Graph) SetNode(v string, label ...any) *Graph {
	if g.nodes.Has(v) {
		if len(label) > 0 {
			g.nodes.Set(v, label[0])
		}
		return g
	}

	if len(label) > 0 {
		g.nodes.Set(v, label[0])
	} else {
		g.nodes.Set(v, g.defaultNodeLabelFn(v))
	}
	if g.isCompound {
		g.parent[v] = graphNode
		g.children[v] = js.NewObj[bool]()
		g.children[graphNode].Set(v, true)
	}
	g.in[v] = js.NewObj[Edge]()
	g.preds[v] = js.NewObj[float64]()
	g.out[v] = js.NewObj[Edge]()
	g.sucs[v] = js.NewObj[float64]()
	g.nodeCount++
	return g
}

// Node returns the label of v. It returns nil when v has no label or does
// not exist.
func (g *Graph) Node(v string) any { return g.nodes.Get(v) }

// HasNode reports whether the node v exists.
func (g *Graph) HasNode(v string) bool { return g.nodes.Has(v) }

// RemoveNode removes v and its edges. In a compound graph the children of v
// become children of the root.
func (g *Graph) RemoveNode(v string) *Graph {
	if g.nodes.Has(v) {
		removeEdge := func(e string) { g.RemoveEdgeObj(g.edgeObjs.Get(e)) }
		g.nodes.Delete(v)
		if g.isCompound {
			g.removeFromParentsChildList(v)
			delete(g.parent, v)
			for _, child := range g.Children(v) {
				// The graph is compound and the root parent has no cycle.
				_ = g.SetParent(child)
			}
			delete(g.children, v)
		}
		for _, e := range g.in[v].Keys() {
			removeEdge(e)
		}
		delete(g.in, v)
		delete(g.preds, v)
		for _, e := range g.out[v].Keys() {
			removeEdge(e)
		}
		delete(g.out, v)
		delete(g.sucs, v)
		g.nodeCount--
	}
	return g
}

// CycleError is the error of SetParent when the new parent is v itself or a
// descendant of v.
type CycleError struct {
	Parent string
	V      string
}

func (e *CycleError) Error() string {
	return "Setting " + e.Parent + " as parent of " + e.V + " would create a cycle"
}

// SetParent makes v a child of parent. Without a parent argument, v becomes
// a child of the root. Both nodes are created when they do not exist.
func (g *Graph) SetParent(v string, parent ...string) error {
	if !g.isCompound {
		return ErrNotCompound
	}

	var p string
	if len(parent) == 0 {
		p = graphNode
	} else {
		p = parent[0]
		ancestor, ok := p, true
		for ok {
			if ancestor == v {
				return &CycleError{Parent: p, V: v}
			}
			ancestor, ok = g.Parent(ancestor)
		}

		g.SetNode(p)
	}

	g.SetNode(v)
	g.removeFromParentsChildList(v)
	g.parent[v] = p
	g.children[p].Set(v, true)
	return nil
}

func (g *Graph) removeFromParentsChildList(v string) {
	g.children[g.parent[v]].Delete(v)
}

// Parent returns the parent of v. The result is false when v is a child of
// the root, when v does not exist, or when the graph is not compound.
func (g *Graph) Parent(v string) (string, bool) {
	if g.isCompound {
		parent, ok := g.parent[v]
		if ok && parent != graphNode {
			return parent, true
		}
	}
	return "", false
}

// Children returns the children of v in JavaScript object key order.
// Without an argument it returns the children of the root. The result is
// nil where the JavaScript method returns undefined: for a node that does
// not exist.
func (g *Graph) Children(v ...string) []string {
	p := graphNode
	if len(v) > 0 {
		p = v[0]
	}

	if g.isCompound {
		children := g.children[p]
		if children != nil {
			return children.Keys()
		}
	} else if p == graphNode {
		return g.Nodes()
	} else if g.HasNode(p) {
		return []string{}
	}
	return nil
}

// Predecessors returns the nodes that have an edge to v. The result is nil
// when v does not exist.
func (g *Graph) Predecessors(v string) []string {
	predsV := g.preds[v]
	if predsV != nil {
		return predsV.Keys()
	}
	return nil
}

// Successors returns the nodes that v has an edge to. The result is nil
// when v does not exist.
func (g *Graph) Successors(v string) []string {
	sucsV := g.sucs[v]
	if sucsV != nil {
		return sucsV.Keys()
	}
	return nil
}

// Neighbors returns the predecessors of v followed by the successors of v
// that are not predecessors. The result is nil when v does not exist.
func (g *Graph) Neighbors(v string) []string {
	preds := g.Predecessors(v)
	if preds != nil {
		// _.union(preds, this.successors(v))
		seen := make(map[string]bool, len(preds))
		res := make([]string, 0, len(preds))
		for _, list := range [][]string{preds, g.Successors(v)} {
			for _, u := range list {
				if !seen[u] {
					seen[u] = true
					res = append(res, u)
				}
			}
		}
		return res
	}
	return nil
}

// IsLeaf reports whether v has no successors in a directed graph, or no
// neighbors in an undirected graph.
func (g *Graph) IsLeaf(v string) bool {
	var neighbors []string
	if g.IsDirected() {
		neighbors = g.Successors(v)
	} else {
		neighbors = g.Neighbors(v)
	}
	return len(neighbors) == 0
}

// FilterNodes returns a new graph with the nodes for which filter returns
// true, the edges between them, and the hierarchy between them. The labels
// are shared with the receiver.
func (g *Graph) FilterNodes(filter func(v string) bool) *Graph {
	cp := New(Options{
		Undirected: !g.isDirected,
		Multigraph: g.isMultigraph,
		Compound:   g.isCompound,
	})

	cp.SetGraph(g.Graph())

	if length, ok := arrayLikeLength(g.nodes); ok {
		// _.each treats an object with a numeric "length" property as an
		// array. this._nodes is such an object when the node "length" has a
		// label that is a non-negative integer. Then _.each visits the
		// indices 0 to length-1, not the nodes.
		for index := 0; index < length; index++ {
			v := strconv.Itoa(index)
			if filter(v) {
				cp.SetNode(v, g.nodes.Get(v))
			}
		}
	} else {
		g.nodes.Each(func(v string, value any) {
			if filter(v) {
				cp.SetNode(v, value)
			}
		})
	}

	g.edgeObjs.Each(func(_ string, e Edge) {
		if cp.HasNode(e.V) && cp.HasNode(e.W) {
			// The copy has the same multigraph option as the receiver.
			_ = cp.SetEdgeObj(e, g.EdgeObj(e))
		}
	})

	type parentEntry struct {
		parent string
		ok     bool
	}
	parents := map[string]parentEntry{}
	var findParent func(v string) (string, bool)
	findParent = func(v string) (string, bool) {
		parent, ok := g.Parent(v)
		if !ok || cp.HasNode(parent) {
			parents[v] = parentEntry{parent, ok}
			return parent, ok
		} else if p, in := parents[parent]; in {
			return p.parent, p.ok
		}
		return findParent(parent)
	}

	if g.isCompound {
		for _, v := range cp.Nodes() {
			if parent, ok := findParent(v); ok {
				_ = cp.SetParent(v, parent)
			} else {
				_ = cp.SetParent(v)
			}
		}
	}

	return cp
}

/* === Edge functions ========== */

// SetDefaultEdgeLabel sets the function that makes the label of an edge that
// is created without a label. The name argument is empty for an edge that
// has no name. A nil function restores the default, which returns nil.
func (g *Graph) SetDefaultEdgeLabel(newDefault func(v, w, name string) any) *Graph {
	if newDefault == nil {
		newDefault = func(string, string, string) any { return nil }
	}
	g.defaultEdgeLabelFn = newDefault
	return g
}

// EdgeCount is the number of edges.
func (g *Graph) EdgeCount() int { return g.edgeCount }

// Edges returns all edges in insertion order.
func (g *Graph) Edges() []Edge { return g.edgeObjs.Values() }

// SetPath creates an edge between each two consecutive nodes of vs.
func (g *Graph) SetPath(vs []string, label ...any) error {
	for i := 1; i < len(vs); i++ {
		if err := g.setEdge(vs[i-1], vs[i], "", false, label); err != nil {
			return err
		}
	}
	return nil
}

// SetEdge creates the edge from v to w when it does not exist, and sets its
// label. The nodes are created when they do not exist. The optional name
// selects one of several edges between v and w in a multigraph.
//
// This is the JavaScript call setEdge(v, w, label, name). A nil label is
// undefined. Use SetEdgeObj for the call that has no label argument.
//
// Do not use the empty name. As in JavaScript, the edge object of such an
// edge has no name, so EdgeObj, HasEdgeObj and RemoveEdgeObj do not find the
// edge, and RemoveNode leaves it in the graph as a dangling edge.
func (g *Graph) SetEdge(v, w string, label any, name ...string) error {
	if len(name) > 0 {
		return g.setEdge(v, w, name[0], true, []any{label})
	}
	return g.setEdge(v, w, "", false, []any{label})
}

// SetEdgeObj is SetEdge for an edge object. With a label argument it sets
// the label of the edge. Without one, a new edge gets the default label and
// the label of an existing edge stays as it is.
func (g *Graph) SetEdgeObj(e Edge, label ...any) error {
	return g.setEdge(e.V, e.W, e.Name, e.Name != "", label)
}

func (g *Graph) setEdge(v, w, name string, hasName bool, value []any) error {
	valueSpecified := len(value) > 0

	e := edgeArgsToID(g.isDirected, v, w, name, hasName)
	if _, ok := g.edgeLabels[e]; ok {
		if valueSpecified {
			g.edgeLabels[e] = value[0]
		}
		return nil
	}

	if hasName && !g.isMultigraph {
		return ErrNotMultigraph
	}

	// It did not exist, so we need to create it.
	// First ensure the nodes exist.
	g.SetNode(v)
	g.SetNode(w)

	if valueSpecified {
		g.edgeLabels[e] = value[0]
	} else {
		g.edgeLabels[e] = g.defaultEdgeLabelFn(v, w, name)
	}

	edgeObj := edgeArgsToObj(g.isDirected, v, w, name)
	// Ensure we add undirected edges in a consistent way.
	v = edgeObj.V
	w = edgeObj.W

	g.edgeObjs.Set(e, edgeObj)
	incrementOrInitEntry(g.preds[w], v)
	incrementOrInitEntry(g.sucs[v], w)
	g.in[w].Set(e, edgeObj)
	g.out[v].Set(e, edgeObj)
	g.edgeCount++
	return nil
}

// Edge returns the label of the edge from v to w with the optional name. It
// returns nil when the edge has no label or does not exist.
func (g *Graph) Edge(v, w string, name ...string) any {
	return g.edgeLabels[edgeArgsToID(g.isDirected, v, w, optName(name), len(name) > 0)]
}

// EdgeObj returns the label of the edge e. It is Edge for an edge object.
func (g *Graph) EdgeObj(e Edge) any {
	return g.edgeLabels[edgeObjToID(g.isDirected, e)]
}

// HasEdge reports whether the edge from v to w with the optional name exists.
func (g *Graph) HasEdge(v, w string, name ...string) bool {
	_, ok := g.edgeLabels[edgeArgsToID(g.isDirected, v, w, optName(name), len(name) > 0)]
	return ok
}

// HasEdgeObj reports whether the edge e exists.
func (g *Graph) HasEdgeObj(e Edge) bool {
	_, ok := g.edgeLabels[edgeObjToID(g.isDirected, e)]
	return ok
}

// RemoveEdge removes the edge from v to w with the optional name.
func (g *Graph) RemoveEdge(v, w string, name ...string) *Graph {
	return g.removeEdge(edgeArgsToID(g.isDirected, v, w, optName(name), len(name) > 0))
}

// RemoveEdgeObj removes the edge e.
func (g *Graph) RemoveEdgeObj(e Edge) *Graph {
	return g.removeEdge(edgeObjToID(g.isDirected, e))
}

func (g *Graph) removeEdge(e string) *Graph {
	edge, ok := g.edgeObjs.Lookup(e)
	if ok {
		v := edge.V
		w := edge.W
		delete(g.edgeLabels, e)
		g.edgeObjs.Delete(e)
		// A map is missing only for a dangling edge: an edge with the empty
		// name whose node was removed (see SetEdge). JavaScript throws a
		// TypeError in that case.
		if m := g.preds[w]; m != nil {
			decrementOrRemoveEntry(m, v)
		}
		if m := g.sucs[v]; m != nil {
			decrementOrRemoveEntry(m, w)
		}
		if m := g.in[w]; m != nil {
			m.Delete(e)
		}
		if m := g.out[v]; m != nil {
			m.Delete(e)
		}
		g.edgeCount--
	}
	return g
}

// InEdges returns the edges that end at v. With a non-empty u it returns
// only the edges that start at u. The result is nil when v does not exist.
func (g *Graph) InEdges(v string, u ...string) []Edge {
	inV := g.in[v]
	if inV != nil {
		edges := inV.Values()
		if len(u) == 0 || u[0] == "" {
			return edges
		}
		res := []Edge{}
		for _, edge := range edges {
			if edge.V == u[0] {
				res = append(res, edge)
			}
		}
		return res
	}
	return nil
}

// OutEdges returns the edges that start at v. With a non-empty w it returns
// only the edges that end at w. The result is nil when v does not exist.
func (g *Graph) OutEdges(v string, w ...string) []Edge {
	outV := g.out[v]
	if outV != nil {
		edges := outV.Values()
		if len(w) == 0 || w[0] == "" {
			return edges
		}
		res := []Edge{}
		for _, edge := range edges {
			if edge.W == w[0] {
				res = append(res, edge)
			}
		}
		return res
	}
	return nil
}

// NodeEdges returns the in-edges of v followed by the out-edges of v. With a
// non-empty w it returns only the edges between v and w. The result is nil
// when v does not exist.
func (g *Graph) NodeEdges(v string, w ...string) []Edge {
	inEdges := g.InEdges(v, w...)
	if inEdges != nil {
		return append(inEdges, g.OutEdges(v, w...)...)
	}
	return nil
}

// arrayLikeLength is the lodash test isArrayLike for the object that maps
// node ids to labels: the property "length" is a number that is a valid
// array length.
func arrayLikeLength(nodes *js.Obj[any]) (int, bool) {
	length, ok := nodes.Get("length").(float64)
	if ok && length > -1 && length == math.Trunc(length) && length <= 9007199254740991 {
		return int(length), true
	}
	return 0, false
}

func optName(name []string) string {
	if len(name) > 0 {
		return name[0]
	}
	return ""
}

// inheritedFunctions are the property names of Object.prototype that have a
// function as value. The JavaScript maps of predecessors and successors are
// plain objects, so a read of such a key that is not set gives the
// inherited function.
var inheritedFunctions = map[string]bool{
	"constructor":          true,
	"__defineGetter__":     true,
	"__defineSetter__":     true,
	"hasOwnProperty":       true,
	"__lookupGetter__":     true,
	"__lookupSetter__":     true,
	"isPrototypeOf":        true,
	"propertyIsEnumerable": true,
	"toString":             true,
	"valueOf":              true,
	"toLocaleString":       true,
}

// incrementOrInitEntry is:
//
//	if (map[k]) { map[k]++; } else { map[k] = 1; }
//
// For a node id that is the name of an inherited function, the first
// increment stores NaN (function++), and the next one stores 1. The port
// keeps this, because it changes the result of Predecessors and Successors
// after an edge is removed.
func incrementOrInitEntry(m *js.Obj[float64], k string) {
	n, ok := m.Lookup(k)
	switch {
	case !ok && inheritedFunctions[k]:
		m.Set(k, math.NaN())
	case ok && n != 0 && n == n:
		m.Set(k, n+1)
	default:
		m.Set(k, 1)
	}
}

// decrementOrRemoveEntry is:
//
//	if (!--map[k]) { delete map[k]; }
func decrementOrRemoveEntry(m *js.Obj[float64], k string) {
	n, ok := m.Lookup(k)
	if !ok {
		// The decrement of undefined or of an inherited function is NaN,
		// which is falsy: the entry is deleted again.
		return
	}
	n--
	if n == 0 || n != n {
		m.Delete(k)
	} else {
		m.Set(k, n)
	}
}

func edgeArgsToID(isDirected bool, v, w, name string, hasName bool) string {
	if !isDirected && js.CompareStrings(v, w) > 0 {
		v, w = w, v
	}
	if !hasName {
		name = defaultEdgeName
	}
	return v + edgeKeyDelim + w + edgeKeyDelim + name
}

func edgeArgsToObj(isDirected bool, v, w, name string) Edge {
	if !isDirected && js.CompareStrings(v, w) > 0 {
		v, w = w, v
	}
	// An empty name is falsy in JavaScript, so the edge object has no name.
	return Edge{V: v, W: w, Name: name}
}

func edgeObjToID(isDirected bool, e Edge) string {
	return edgeArgsToID(isDirected, e.V, e.W, e.Name, e.Name != "")
}
