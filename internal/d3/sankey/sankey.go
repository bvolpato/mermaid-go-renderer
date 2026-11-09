// Ported from d3-sankey 0.12.3, d3-sankey/src/sankey.js (BSD-3-Clause).

// Package sankey is a port of d3-sankey 0.12.3: the layout of a flow diagram
// between the nodes of a directed acyclic graph.
package sankey

import (
	"errors"
	"fmt"
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/array"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Node is a node of the graph. The caller sets Data and optionally
// FixedValue. The layout sets the other fields.
type Node struct {
	// Data is the datum of the caller. The node id function reads it.
	Data any
	// FixedValue is node.fixedValue: the value of the node when it is not
	// nil. Without it the value is the larger sum of the link values of
	// the two sides.
	FixedValue *float64

	// Index is the position of the node in the list of nodes.
	Index int
	// SourceLinks are the links that start at the node, and TargetLinks
	// the links that end at it.
	SourceLinks []*Link
	TargetLinks []*Link
	// Value is the value of the node.
	Value float64
	// Depth is the number of links on the longest path from a node without
	// incoming links. Height is the same toward a node without outgoing
	// links. Layer is the column of the node.
	Depth  int
	Height int
	Layer  int
	// X0, X1 are the horizontal bounds and Y0, Y1 the vertical bounds.
	X0, X1, Y0, Y1 float64
}

// Link is a link of the graph. The caller sets Value and the two ends:
// either the nodes Source and Target, or the ids SourceID and TargetID.
type Link struct {
	// Data is the datum of the caller.
	Data any
	// Source and Target are the nodes of the link. When one is nil, the
	// layout finds the node by SourceID or TargetID and sets the field.
	Source, Target *Node
	// SourceID and TargetID are the ids of the nodes, as the node id
	// function returns them. They are compared as Go values.
	SourceID, TargetID any
	// Value is the value of the link.
	Value float64

	// Index is the position of the link in the list of links.
	Index int
	// Width is the width of the link, proportional to the value.
	Width float64
	// Y0 is the vertical middle of the link at the source node, and Y1 at
	// the target node.
	Y0, Y1 float64
}

// Graph is the argument of the layout: the nodes and the links.
type Graph struct {
	Nodes []*Node
	Links []*Link
}

// ErrCircular is the error "circular link" of d3-sankey.
var ErrCircular = errors.New("circular link")

func ascendingSourceBreadth(a, b *Link) float64 {
	if v := ascendingBreadth(a.Source, b.Source); v != 0 && !math.IsNaN(v) {
		return v
	}
	return float64(a.Index - b.Index)
}

func ascendingTargetBreadth(a, b *Link) float64 {
	if v := ascendingBreadth(a.Target, b.Target); v != 0 && !math.IsNaN(v) {
		return v
	}
	return float64(a.Index - b.Index)
}

func ascendingBreadth(a, b *Node) float64 {
	return a.Y0 - b.Y0
}

func value(d *Node, _ int, _ []*Node) float64 {
	return d.Value
}

func linkValue(d *Link, _ int, _ []*Link) float64 {
	return d.Value
}

func defaultID(d *Node, _ int, _ []*Node) any {
	return d.Index
}

// idString is the conversion of an id to a string for the error message.
func idString(id any) string {
	switch v := id.(type) {
	case int:
		return js.Str(float64(v))
	case fmt.Stringer:
		return v.String()
	}
	return js.ToString(id)
}

func find(nodeByID map[any]*Node, id any) (*Node, error) {
	node := nodeByID[id]
	if node == nil {
		return nil, fmt.Errorf("missing: %s", idString(id))
	}
	return node, nil
}

func computeLinkBreadths(graph *Graph) {
	for _, node := range graph.Nodes {
		y0 := node.Y0
		y1 := y0
		for _, link := range node.SourceLinks {
			link.Y0 = y0 + float64(link.Width/2)
			y0 += link.Width
		}
		for _, link := range node.TargetLinks {
			link.Y1 = y1 + float64(link.Width/2)
			y1 += link.Width
		}
	}
}

// sortKind is the state of a comparator option. d3-sankey tells apart a
// comparator that was never set (undefined), a comparator that is null and a
// function.
type sortKind uint8

const (
	sortUndefined sortKind = iota
	sortNull
	sortFunction
)

// Sankey is the layout generator: the function that d3.sankey() returns.
type Sankey struct {
	x0, y0, x1, y1 float64 // extent
	dx             float64 // nodeWidth
	dy, py         float64 // nodePadding
	id             func(d *Node, i int, nodes []*Node) any
	align          Align
	sortKind       sortKind
	sort           func(a, b *Node) float64
	linkSortKind   sortKind
	linkSort       func(a, b *Link) float64
	iterations     float64
	sortMode       js.V8Sort
}

// New is d3.sankey(): a generator with the default settings.
func New() *Sankey {
	return &Sankey{
		x0: 0, y0: 0, x1: 1, y1: 1,
		dx:         24,
		dy:         8,
		py:         math.NaN(),
		id:         defaultID,
		align:      Justify,
		iterations: 6,
		sortMode:   js.V8PowerSort,
	}
}

// Layout is sankey(graph): it computes the positions of the nodes and the
// links. The errors are the errors that d3-sankey throws: a link to a node
// that does not exist, and a circular link.
func (s *Sankey) Layout(graph *Graph) error {
	if err := s.computeNodeLinks(graph); err != nil {
		return err
	}
	s.computeNodeValues(graph)
	if err := s.computeNodeDepths(graph); err != nil {
		return err
	}
	if err := s.computeNodeHeights(graph); err != nil {
		return err
	}
	if err := s.computeNodeBreadths(graph); err != nil {
		return err
	}
	computeLinkBreadths(graph)
	return nil
}

// Update is sankey.update(graph): it computes the vertical positions of the
// links again after a node moved.
func (s *Sankey) Update(graph *Graph) {
	computeLinkBreadths(graph)
}

// NodeID is sankey.nodeId(id): the function that returns the id of a node.
// The default is the index of the node, as an int.
func (s *Sankey) NodeID(id func(d *Node, i int, nodes []*Node) any) *Sankey {
	s.id = id
	return s
}

// NodeAlign is sankey.nodeAlign(align). The default is Justify.
func (s *Sankey) NodeAlign(align Align) *Sankey {
	s.align = align
	return s
}

// NodeSort is sankey.nodeSort(sort): the order of the nodes in a column. A
// nil comparator is null in d3: the nodes keep the order of the input. A
// generator on which NodeSort was never called orders the nodes of a column
// by their position during the relaxation.
func (s *Sankey) NodeSort(sort func(a, b *Node) float64) *Sankey {
	s.sort = sort
	s.sortKind = sortFunction
	if sort == nil {
		s.sortKind = sortNull
	}
	return s
}

// NodeWidth is sankey.nodeWidth(width). The default is 24.
func (s *Sankey) NodeWidth(width float64) *Sankey {
	s.dx = width
	return s
}

// NodePadding is sankey.nodePadding(padding). The default is 8.
func (s *Sankey) NodePadding(padding float64) *Sankey {
	s.dy = padding
	s.py = padding
	return s
}

// LinkSort is sankey.linkSort(sort): the order of the links of a node. A nil
// comparator is null in d3: the links keep the order of the input. A
// generator on which LinkSort was never called orders the links by the
// position of the node at the other end.
func (s *Sankey) LinkSort(sort func(a, b *Link) float64) *Sankey {
	s.linkSort = sort
	s.linkSortKind = sortFunction
	if sort == nil {
		s.linkSortKind = sortNull
	}
	return s
}

// Size is sankey.size([width, height]): the extent from [0, 0].
func (s *Sankey) Size(width, height float64) *Sankey {
	s.x0, s.y0 = 0, 0
	s.x1, s.y1 = width, height
	return s
}

// Extent is sankey.extent([[x0, y0], [x1, y1]]). The default is
// [[0, 0], [1, 1]].
func (s *Sankey) Extent(x0, y0, x1, y1 float64) *Sankey {
	s.x0, s.x1, s.y0, s.y1 = x0, x1, y0, y1
	return s
}

// Iterations is sankey.iterations(iterations): the number of relaxation
// passes. The default is 6.
func (s *Sankey) Iterations(iterations float64) *Sankey {
	s.iterations = iterations
	return s
}

// SortAlgorithm selects the Array.prototype.sort of a V8 version. The
// versions order nodes and links differently only when a comparator is not
// consistent, for example when a position is NaN. The default is the
// algorithm of current Chromium (js.V8PowerSort).
func (s *Sankey) SortAlgorithm(mode js.V8Sort) *Sankey {
	s.sortMode = mode
	return s
}

func (s *Sankey) computeNodeLinks(graph *Graph) error {
	nodes, links := graph.Nodes, graph.Links
	for i, node := range nodes {
		node.Index = i
		node.SourceLinks = []*Link{}
		node.TargetLinks = []*Link{}
	}
	nodeByID := make(map[any]*Node, len(nodes))
	for i, d := range nodes {
		nodeByID[s.id(d, i, nodes)] = d
	}
	for i, link := range links {
		link.Index = i
		source, target := link.Source, link.Target
		var err error
		if source == nil {
			if source, err = find(nodeByID, link.SourceID); err != nil {
				return err
			}
			link.Source = source
		}
		if target == nil {
			if target, err = find(nodeByID, link.TargetID); err != nil {
				return err
			}
			link.Target = target
		}
		source.SourceLinks = append(source.SourceLinks, link)
		target.TargetLinks = append(target.TargetLinks, link)
	}
	if s.linkSortKind == sortFunction {
		for _, node := range nodes {
			js.SortV8(s.sortMode, node.SourceLinks, s.linkSort)
			js.SortV8(s.sortMode, node.TargetLinks, s.linkSort)
		}
	}
	return nil
}

func (s *Sankey) computeNodeValues(graph *Graph) {
	for _, node := range graph.Nodes {
		if node.FixedValue == nil {
			node.Value = js.Max(array.SumFunc(node.SourceLinks, linkValue), array.SumFunc(node.TargetLinks, linkValue))
		} else {
			node.Value = *node.FixedValue
		}
	}
}

// nodeSet is a JavaScript Set of nodes: it keeps the insertion order.
type nodeSet struct {
	list []*Node
	seen map[*Node]struct{}
}

func newNodeSet(nodes []*Node) *nodeSet {
	set := &nodeSet{seen: make(map[*Node]struct{}, len(nodes))}
	for _, node := range nodes {
		set.add(node)
	}
	return set
}

func (s *nodeSet) add(node *Node) {
	if _, ok := s.seen[node]; ok {
		return
	}
	s.seen[node] = struct{}{}
	s.list = append(s.list, node)
}

func (s *Sankey) computeNodeDepths(graph *Graph) error {
	nodes := graph.Nodes
	n := len(nodes)
	current := newNodeSet(nodes)
	next := newNodeSet(nil)
	x := 0
	for len(current.list) > 0 {
		for _, node := range current.list {
			node.Depth = x
			for _, link := range node.SourceLinks {
				next.add(link.Target)
			}
		}
		x++
		if x > n {
			return ErrCircular
		}
		current = next
		next = newNodeSet(nil)
	}
	return nil
}

func (s *Sankey) computeNodeHeights(graph *Graph) error {
	nodes := graph.Nodes
	n := len(nodes)
	current := newNodeSet(nodes)
	next := newNodeSet(nil)
	x := 0
	for len(current.list) > 0 {
		for _, node := range current.list {
			node.Height = x
			for _, link := range node.TargetLinks {
				next.add(link.Source)
			}
		}
		x++
		if x > n {
			return ErrCircular
		}
		current = next
		next = newNodeSet(nil)
	}
	return nil
}

func (s *Sankey) computeNodeLayers(graph *Graph) ([][]*Node, error) {
	nodes := graph.Nodes
	maxDepth, ok := array.MaxFunc(nodes, func(d *Node, _ int, _ []*Node) float64 { return float64(d.Depth) })
	if !ok {
		// max is undefined for no nodes, and new Array(NaN) throws.
		return nil, errors.New("Invalid array length")
	}
	x := int(maxDepth) + 1
	kx := (s.x1 - s.x0 - s.dx) / float64(x-1)
	columns := make([][]*Node, x)
	for _, node := range nodes {
		layer := js.Max(0, js.Min(float64(x-1), math.Floor(s.align(node, x))))
		if math.IsNaN(layer) {
			// d3 keeps such a node outside of all columns.
			return nil, errors.New("the node alignment is not a number")
		}
		i := int(layer)
		node.Layer = i
		node.X0 = s.x0 + float64(float64(i)*kx)
		node.X1 = node.X0 + s.dx
		columns[i] = append(columns[i], node)
	}
	for _, column := range columns {
		if column == nil {
			// A column without nodes is undefined in d3, and the first
			// use of it throws a TypeError.
			return nil, errors.New("Cannot read properties of undefined (reading 'length')")
		}
	}
	if s.sortKind == sortFunction {
		for _, column := range columns {
			js.SortV8(s.sortMode, column, s.sort)
		}
	}
	return columns, nil
}

func (s *Sankey) initializeNodeBreadths(columns [][]*Node) {
	ky, ok := array.MinFunc(columns, func(c []*Node, _ int, _ [][]*Node) float64 {
		return (s.y1 - s.y0 - float64(float64(len(c)-1)*s.py)) / array.SumFunc(c, value)
	})
	if !ok {
		// min is undefined, and a product with undefined is NaN.
		ky = math.NaN()
	}
	for _, nodes := range columns {
		y := s.y0
		for _, node := range nodes {
			node.Y0 = y
			node.Y1 = y + float64(node.Value*ky)
			y = node.Y1 + s.py
			for _, link := range node.SourceLinks {
				link.Width = float64(link.Value * ky)
			}
		}
		y = (s.y1 - y + s.py) / float64(len(nodes)+1)
		for i := 0; i < len(nodes); i++ {
			node := nodes[i]
			node.Y0 += float64(y * float64(i+1))
			node.Y1 += float64(y * float64(i+1))
		}
		s.reorderLinks(nodes)
	}
}

func (s *Sankey) computeNodeBreadths(graph *Graph) error {
	columns, err := s.computeNodeLayers(graph)
	if err != nil {
		return err
	}
	maxLen, _ := array.MaxFunc(columns, func(c []*Node, _ int, _ [][]*Node) float64 { return float64(len(c)) })
	s.py = js.Min(s.dy, (s.y1-s.y0)/(maxLen-1))
	s.initializeNodeBreadths(columns)
	for i := 0; float64(i) < s.iterations; i++ {
		alpha := js.Pow(0.99, float64(i))
		beta := js.Max(1-alpha, float64(i+1)/s.iterations)
		s.relaxRightToLeft(columns, alpha, beta)
		s.relaxLeftToRight(columns, alpha, beta)
	}
	return nil
}

// relaxLeftToRight repositions each node based on its incoming (target)
// links.
func (s *Sankey) relaxLeftToRight(columns [][]*Node, alpha, beta float64) {
	for i, n := 1, len(columns); i < n; i++ {
		column := columns[i]
		for _, target := range column {
			y := 0.0
			w := 0.0
			for _, link := range target.TargetLinks {
				source := link.Source
				v := float64(link.Value * float64(target.Layer-source.Layer))
				y += float64(s.targetTop(source, target) * v)
				w += v
			}
			if !(w > 0) {
				continue
			}
			dy := float64((y/w - target.Y0) * alpha)
			target.Y0 += dy
			target.Y1 += dy
			s.reorderNodeLinks(target)
		}
		if s.sortKind == sortUndefined {
			js.SortV8(s.sortMode, column, ascendingBreadth)
		}
		s.resolveCollisions(column, beta)
	}
}

// relaxRightToLeft repositions each node based on its outgoing (source)
// links.
func (s *Sankey) relaxRightToLeft(columns [][]*Node, alpha, beta float64) {
	for i := len(columns) - 2; i >= 0; i-- {
		column := columns[i]
		for _, source := range column {
			y := 0.0
			w := 0.0
			for _, link := range source.SourceLinks {
				target := link.Target
				v := float64(link.Value * float64(target.Layer-source.Layer))
				y += float64(s.sourceTop(source, target) * v)
				w += v
			}
			if !(w > 0) {
				continue
			}
			dy := float64((y/w - source.Y0) * alpha)
			source.Y0 += dy
			source.Y1 += dy
			s.reorderNodeLinks(source)
		}
		if s.sortKind == sortUndefined {
			js.SortV8(s.sortMode, column, ascendingBreadth)
		}
		s.resolveCollisions(column, beta)
	}
}

func (s *Sankey) resolveCollisions(nodes []*Node, alpha float64) {
	i := len(nodes) >> 1
	subject := nodes[i]
	s.resolveCollisionsBottomToTop(nodes, subject.Y0-s.py, i-1, alpha)
	s.resolveCollisionsTopToBottom(nodes, subject.Y1+s.py, i+1, alpha)
	s.resolveCollisionsBottomToTop(nodes, s.y1, len(nodes)-1, alpha)
	s.resolveCollisionsTopToBottom(nodes, s.y0, 0, alpha)
}

// resolveCollisionsTopToBottom pushes any overlapping nodes down.
func (s *Sankey) resolveCollisionsTopToBottom(nodes []*Node, y float64, i int, alpha float64) {
	for ; i < len(nodes); i++ {
		node := nodes[i]
		dy := float64((y - node.Y0) * alpha)
		if dy > 1e-6 {
			node.Y0 += dy
			node.Y1 += dy
		}
		y = node.Y1 + s.py
	}
}

// resolveCollisionsBottomToTop pushes any overlapping nodes up.
func (s *Sankey) resolveCollisionsBottomToTop(nodes []*Node, y float64, i int, alpha float64) {
	for ; i >= 0; i-- {
		node := nodes[i]
		dy := float64((node.Y1 - y) * alpha)
		if dy > 1e-6 {
			node.Y0 -= dy
			node.Y1 -= dy
		}
		y = node.Y0 - s.py
	}
}

func (s *Sankey) reorderNodeLinks(node *Node) {
	if s.linkSortKind == sortUndefined {
		for _, link := range node.TargetLinks {
			js.SortV8(s.sortMode, link.Source.SourceLinks, ascendingTargetBreadth)
		}
		for _, link := range node.SourceLinks {
			js.SortV8(s.sortMode, link.Target.TargetLinks, ascendingSourceBreadth)
		}
	}
}

func (s *Sankey) reorderLinks(nodes []*Node) {
	if s.linkSortKind == sortUndefined {
		for _, node := range nodes {
			js.SortV8(s.sortMode, node.SourceLinks, ascendingTargetBreadth)
			js.SortV8(s.sortMode, node.TargetLinks, ascendingSourceBreadth)
		}
	}
}

// targetTop returns the target.y0 that would produce an ideal link from
// source to target.
func (s *Sankey) targetTop(source, target *Node) float64 {
	y := source.Y0 - float64(float64(len(source.SourceLinks)-1)*s.py/2)
	for _, link := range source.SourceLinks {
		if link.Target == target {
			break
		}
		y += link.Width + s.py
	}
	for _, link := range target.TargetLinks {
		if link.Source == source {
			break
		}
		y -= link.Width
	}
	return y
}

// sourceTop returns the source.y0 that would produce an ideal link from
// source to target.
func (s *Sankey) sourceTop(source, target *Node) float64 {
	y := target.Y0 - float64(float64(len(target.TargetLinks)-1)*s.py/2)
	for _, link := range target.TargetLinks {
		if link.Source == source {
			break
		}
		y += link.Width + s.py
	}
	for _, link := range source.SourceLinks {
		if link.Target == target {
			break
		}
		y -= link.Width
	}
	return y
}
