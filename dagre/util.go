package dagre

import (
	"fmt"
	"math"
	"sync/atomic"
)

var idCounter atomic.Int64

// UniqueID generates a unique node identifier with the given prefix.
func UniqueID(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, idCounter.Add(1))
}

// MaxRank returns the maximum rank across all nodes in the graph.
func MaxRank(g *Graph) int {
	max := math.MinInt64
	for _, v := range g.Nodes() {
		n := g.Node(v)
		if n != nil && n.HasRank && n.Rank > max {
			max = n.Rank
		}
	}
	if max == math.MinInt64 {
		return 0
	}
	return max
}

// BuildLayerMatrix builds a 2D array of nodes grouped by rank.
func BuildLayerMatrix(g *Graph) [][]string {
	maxR := MaxRank(g)
	layers := make([][]string, maxR+1)
	for i := range layers {
		layers[i] = []string{}
	}
	for _, v := range g.Nodes() {
		n := g.Node(v)
		if n != nil && n.HasRank && n.Rank >= 0 && n.Rank <= maxR {
			layers[n.Rank] = append(layers[n.Rank], v)
		}
	}
	// Sort each layer by order
	for _, layer := range layers {
		sortByOrder(g, layer)
	}
	return layers
}

func sortByOrder(g *Graph, vs []string) {
	// Simple insertion sort — layers are small
	for i := 1; i < len(vs); i++ {
		key := vs[i]
		keyOrder := nodeOrder(g, key)
		j := i - 1
		for j >= 0 && nodeOrder(g, vs[j]) > keyOrder {
			vs[j+1] = vs[j]
			j--
		}
		vs[j+1] = key
	}
}

func nodeOrder(g *Graph, v string) int {
	n := g.Node(v)
	if n != nil && n.HasOrder {
		return n.Order
	}
	return 0
}

// Simplify returns a new graph with multi-edges collapsed into single edges
// with summed weights and minimum minlens.
func Simplify(g *Graph) *Graph {
	s := NewGraph()
	s.isDirected = g.isDirected
	s.isMultigraph = false
	s.isCompound = false
	s.label = g.label

	for _, v := range g.Nodes() {
		s.SetNode(v, g.Node(v))
	}
	for _, e := range g.Edges() {
		el := g.EdgeByKey(e)
		existing := s.EdgeVW(e.V, e.W)
		if existing != nil {
			existing.Weight += el.Weight
			if el.MinLen < existing.MinLen {
				existing.MinLen = el.MinLen
			}
		} else {
			newLabel := &EdgeLabel{
				Weight: el.Weight,
				MinLen: el.MinLen,
			}
			s.SetEdgeVW(e.V, e.W, newLabel)
		}
	}
	return s
}

// AsNonCompoundGraph returns the graph with compound features stripped.
func AsNonCompoundGraph(g *Graph) *Graph {
	result := NewGraph()
	result.isDirected = g.isDirected
	result.isMultigraph = g.isMultigraph
	result.isCompound = false
	result.label = g.label

	for _, v := range g.Nodes() {
		if !g.isCompound || len(g.Children(v)) == 0 {
			result.SetNode(v, g.Node(v))
		}
	}
	for _, e := range g.Edges() {
		result.SetEdge(e, g.EdgeByKey(e))
	}
	return result
}

// AddDummyNode creates a dummy node with the given type and label, returning its id.
func AddDummyNode(g *Graph, dummyType, edgeLabelObj string, rank int) string {
	v := UniqueID("_" + dummyType)
	label := &NodeLabel{
		Width:   0,
		Height:  0,
		Rank:    rank,
		HasRank: true,
		Dummy:   dummyType,
	}
	g.SetNode(v, label)
	return v
}

// MinBy returns the element from items that minimizes the given function.
func MinBy[T any](items []T, fn func(T) float64) (T, bool) {
	var best T
	bestVal := math.Inf(1)
	found := false
	for _, item := range items {
		v := fn(item)
		if v < bestVal {
			bestVal = v
			best = item
			found = true
		}
	}
	return best, found
}

// RangeInts returns a slice of ints from start (inclusive) to end (exclusive).
func RangeInts(start, end int) []int {
	if end <= start {
		return nil
	}
	r := make([]int, end-start)
	for i := range r {
		r[i] = start + i
	}
	return r
}

// RangeIntsStep returns a slice of ints from start, stepping by step, until past end.
func RangeIntsStep(start, end, step int) []int {
	if step == 0 {
		return nil
	}
	var r []int
	if step > 0 {
		for i := start; i < end; i += step {
			r = append(r, i)
		}
	} else {
		for i := start; i > end; i += step {
			r = append(r, i)
		}
	}
	return r
}

// Partition splits items into two slices: those satisfying the predicate and those not.
func Partition[T any](items []T, pred func(T) bool) (lhs, rhs []T) {
	for _, item := range items {
		if pred(item) {
			lhs = append(lhs, item)
		} else {
			rhs = append(rhs, item)
		}
	}
	return
}

// Preorder returns nodes in preorder traversal from roots over the given graph.
func Preorder(g *Graph, roots []string) []string {
	var result []string
	visited := make(map[string]bool)
	var dfs func(string)
	dfs = func(v string) {
		if visited[v] {
			return
		}
		visited[v] = true
		result = append(result, v)
		for _, w := range g.Neighbors(v) {
			dfs(w)
		}
	}
	for _, r := range roots {
		dfs(r)
	}
	return result
}

// Postorder returns nodes in postorder traversal from roots over the given graph.
func Postorder(g *Graph, roots []string) []string {
	var result []string
	visited := make(map[string]bool)
	var dfs func(string)
	dfs = func(v string) {
		if visited[v] {
			return
		}
		visited[v] = true
		for _, w := range g.Neighbors(v) {
			dfs(w)
		}
		result = append(result, v)
	}
	for _, r := range roots {
		dfs(r)
	}
	return result
}
