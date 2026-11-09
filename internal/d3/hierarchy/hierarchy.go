// Ported from d3-hierarchy 3.1.2, d3-hierarchy/src/hierarchy: index.js,
// ancestors.js, count.js, descendants.js, each.js, eachAfter.js,
// eachBefore.js, find.js, iterator.js, leaves.js, links.js, path.js, sort.js
// and sum.js (ISC).

// Package hierarchy is a port of the parts of d3-hierarchy 3.1.2 that
// mermaid uses: the hierarchy of nodes and the treemap layout with its
// tiling methods.
package hierarchy

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Node is a node of a hierarchy. T is the type of the data.
type Node[T any] struct {
	// Data is the datum of the node.
	Data T
	// Depth is the number of links to the root. Height is the number of
	// links to the deepest leaf below the node.
	Depth  int
	Height int
	// Parent is nil for the root.
	Parent *Node[T]
	// Children is nil for a leaf. In d3 the property does not exist then.
	Children []*Node[T]
	// Value is node.value. It is NaN until Sum or Count sets it: the
	// property is undefined in d3 then, and NaN gives the same results in
	// every computation of this package.
	Value float64
	// X0, Y0, X1, Y1 are the bounds that the treemap layout sets.
	X0, Y0, X1, Y1 float64

	// squarify is node._squarify: the rows of the last Resquarify tiling.
	squarify *squarifyRows[T]
}

// newNode is new Node(data).
func newNode[T any](data T) *Node[T] {
	return &Node[T]{Data: data, Value: math.NaN()}
}

// New is d3.hierarchy(data, children). The children function returns the
// children of a datum. A datum with no children is a leaf.
func New[T any](data T, children func(d T) []T) *Node[T] {
	root := newNode(data)
	nodes := []*Node[T]{root}

	for len(nodes) > 0 {
		node := nodes[len(nodes)-1]
		nodes = nodes[:len(nodes)-1]
		if childs := children(node.Data); len(childs) > 0 {
			n := len(childs)
			node.Children = make([]*Node[T], n)
			for i := n - 1; i >= 0; i-- {
				child := newNode(childs[i])
				node.Children[i] = child
				nodes = append(nodes, child)
				child.Parent = node
				child.Depth = node.Depth + 1
			}
		}
	}

	return root.EachBefore(func(node *Node[T], _ int) { computeHeight(node) })
}

// Copy is node.copy(): a copy of the subtree with the node as the root. The
// depths of the copy start at zero. The copy has the values of the nodes and
// not the bounds.
func (n *Node[T]) Copy() *Node[T] {
	// hierarchy(this).eachBefore(copyData): a hierarchy whose data are the
	// nodes, and then each new node takes the value and the data of its
	// node.
	type pair struct{ source, copy *Node[T] }
	root := &Node[T]{Data: n.Data, Value: n.Value}
	nodes := []pair{{n, root}}

	for len(nodes) > 0 {
		p := nodes[len(nodes)-1]
		nodes = nodes[:len(nodes)-1]
		if childs := p.source.Children; len(childs) > 0 {
			n := len(childs)
			p.copy.Children = make([]*Node[T], n)
			for i := n - 1; i >= 0; i-- {
				child := &Node[T]{Data: childs[i].Data, Value: childs[i].Value}
				p.copy.Children[i] = child
				nodes = append(nodes, pair{childs[i], child})
				child.Parent = p.copy
				child.Depth = p.copy.Depth + 1
			}
		}
	}

	return root.EachBefore(func(node *Node[T], _ int) { computeHeight(node) })
}

func computeHeight[T any](node *Node[T]) {
	height := 0
	for {
		node.Height = height
		node = node.Parent
		if node == nil {
			break
		}
		height++
		if !(node.Height < height) {
			break
		}
	}
}

// Ancestors is node.ancestors(): the node and then its ancestors up to the
// root.
func (n *Node[T]) Ancestors() []*Node[T] {
	node := n
	nodes := []*Node[T]{node}
	for node = node.Parent; node != nil; node = node.Parent {
		nodes = append(nodes, node)
	}
	return nodes
}

// Count is node.count(): the value of each node becomes the number of
// leaves below it.
func (n *Node[T]) Count() *Node[T] {
	return n.EachAfter(func(node *Node[T], _ int) {
		sum := 0.0
		children := node.Children
		i := len(children)
		if i == 0 {
			sum = 1
		} else {
			for i--; i >= 0; i-- {
				sum += children[i].Value
			}
		}
		node.Value = sum
	})
}

// Descendants is node.descendants(): the node and its descendants in
// breadth-first order.
func (n *Node[T]) Descendants() []*Node[T] {
	var nodes []*Node[T]
	n.iterate(func(node *Node[T]) bool {
		nodes = append(nodes, node)
		return true
	})
	return nodes
}

// Each is node.each(callback): it calls the callback for the node and its
// descendants in breadth-first order. index counts the calls.
func (n *Node[T]) Each(callback func(node *Node[T], index int)) *Node[T] {
	index := -1
	n.iterate(func(node *Node[T]) bool {
		index++
		callback(node, index)
		return true
	})
	return n
}

// EachAfter is node.eachAfter(callback): post-order traversal. The callback
// gets a node after all of its descendants.
func (n *Node[T]) EachAfter(callback func(node *Node[T], index int)) *Node[T] {
	nodes := []*Node[T]{n}
	var next []*Node[T]
	index := -1
	for len(nodes) > 0 {
		node := nodes[len(nodes)-1]
		nodes = nodes[:len(nodes)-1]
		next = append(next, node)
		if children := node.Children; children != nil {
			for i, n := 0, len(children); i < n; i++ {
				nodes = append(nodes, children[i])
			}
		}
	}
	for len(next) > 0 {
		node := next[len(next)-1]
		next = next[:len(next)-1]
		index++
		callback(node, index)
	}
	return n
}

// EachBefore is node.eachBefore(callback): pre-order traversal. The callback
// gets a node before its descendants.
func (n *Node[T]) EachBefore(callback func(node *Node[T], index int)) *Node[T] {
	nodes := []*Node[T]{n}
	index := -1
	for len(nodes) > 0 {
		node := nodes[len(nodes)-1]
		nodes = nodes[:len(nodes)-1]
		index++
		callback(node, index)
		if children := node.Children; children != nil {
			for i := len(children) - 1; i >= 0; i-- {
				nodes = append(nodes, children[i])
			}
		}
	}
	return n
}

// Find is node.find(callback): the first node in breadth-first order for
// which the callback returns true, or nil.
func (n *Node[T]) Find(callback func(node *Node[T], index int) bool) *Node[T] {
	index := -1
	var found *Node[T]
	n.iterate(func(node *Node[T]) bool {
		index++
		if callback(node, index) {
			found = node
			return false
		}
		return true
	})
	return found
}

// iterate is node[Symbol.iterator](): it yields the node and its descendants
// in breadth-first order until yield returns false. The children of a node
// are read after the node was yielded, as the generator of d3 does.
func (n *Node[T]) iterate(yield func(node *Node[T]) bool) {
	next := []*Node[T]{n}
	for {
		// current = next.reverse(), then current.pop() until it is empty:
		// the nodes of next in order.
		current := next
		next = nil
		for _, node := range current {
			if !yield(node) {
				return
			}
			if children := node.Children; children != nil {
				for i, n := 0, len(children); i < n; i++ {
					next = append(next, children[i])
				}
			}
		}
		if len(next) == 0 {
			return
		}
	}
}

// Leaves is node.leaves(): the leaves in pre-order.
func (n *Node[T]) Leaves() []*Node[T] {
	var leaves []*Node[T]
	n.EachBefore(func(node *Node[T], _ int) {
		if node.Children == nil {
			leaves = append(leaves, node)
		}
	})
	return leaves
}

// Link is a link of node.links(): a parent and one of its children.
type Link[T any] struct {
	Source, Target *Node[T]
}

// Links is node.links(): a link for each descendant of the node.
func (n *Node[T]) Links() []Link[T] {
	root := n
	var links []Link[T]
	root.Each(func(node *Node[T], _ int) {
		if node != root { // Don’t include the root’s parent, if any.
			links = append(links, Link[T]{Source: node.Parent, Target: node})
		}
	})
	return links
}

// Path is node.path(end): the shortest path from the node to end, through
// their least common ancestor. Both nodes must be in the same hierarchy.
func (n *Node[T]) Path(end *Node[T]) []*Node[T] {
	start := n
	ancestor := leastCommonAncestor(start, end)
	nodes := []*Node[T]{start}
	for start != ancestor && start != nil {
		start = start.Parent
		nodes = append(nodes, start)
	}
	k := len(nodes)
	for end != ancestor && end != nil {
		// nodes.splice(k, 0, end)
		nodes = append(nodes, nil)
		copy(nodes[k+1:], nodes[k:])
		nodes[k] = end
		end = end.Parent
	}
	return nodes
}

func leastCommonAncestor[T any](a, b *Node[T]) *Node[T] {
	if a == b {
		return a
	}
	aNodes := a.Ancestors()
	bNodes := b.Ancestors()
	var c *Node[T]
	pop := func(nodes *[]*Node[T]) *Node[T] {
		if len(*nodes) == 0 {
			return nil
		}
		node := (*nodes)[len(*nodes)-1]
		*nodes = (*nodes)[:len(*nodes)-1]
		return node
	}
	a = pop(&aNodes)
	b = pop(&bNodes)
	for a == b && a != nil {
		c = a
		a = pop(&aNodes)
		b = pop(&bNodes)
	}
	return c
}

// Sort is node.sort(compare): it sorts the children of the node and of each
// descendant, in pre-order, with Array.prototype.sort of current Chromium.
func (n *Node[T]) Sort(compare func(a, b *Node[T]) float64) *Node[T] {
	return n.SortWith(js.V8PowerSort, compare)
}

// SortWith is Sort with the Array.prototype.sort of a V8 version. The
// versions order the children differently only when the comparator is not
// consistent, for example when a value is NaN.
func (n *Node[T]) SortWith(mode js.V8Sort, compare func(a, b *Node[T]) float64) *Node[T] {
	return n.EachBefore(func(node *Node[T], _ int) {
		if node.Children != nil {
			js.SortV8(mode, node.Children, compare)
		}
	})
}

// Sum is node.sum(value): the value of each node becomes the result of the
// value function for its datum plus the values of its children. A result
// that is NaN counts as zero.
func (n *Node[T]) Sum(value func(d T) float64) *Node[T] {
	return n.EachAfter(func(node *Node[T], _ int) {
		sum := value(node.Data)
		if math.IsNaN(sum) || sum == 0 {
			// +value(node.data) || 0
			sum = 0
		}
		children := node.Children
		for i := len(children) - 1; i >= 0; i-- {
			sum += children[i].Value
		}
		node.Value = sum
	})
}
