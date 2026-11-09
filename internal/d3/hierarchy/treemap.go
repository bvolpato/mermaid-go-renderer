// Ported from d3-hierarchy 3.1.2, d3-hierarchy/src/treemap/index.js and
// round.js (ISC).

package hierarchy

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Tile is a tiling method of the treemap layout: it divides the rectangle
// x0, y0, x1, y1 among the children of parent.
type Tile[T any] func(parent *Node[T], x0, y0, x1, y1 float64)

// Treemap is the treemap layout: the function that d3.treemap() returns.
type Treemap[T any] struct {
	tile          Tile[T]
	round         bool
	dx, dy        float64
	paddingStack  []float64
	paddingInner  func(node *Node[T]) float64
	paddingTop    func(node *Node[T]) float64
	paddingRight  func(node *Node[T]) float64
	paddingBottom func(node *Node[T]) float64
	paddingLeft   func(node *Node[T]) float64
}

func constantZero[T any](*Node[T]) float64 {
	return 0
}

func constant[T any](x float64) func(*Node[T]) float64 {
	return func(*Node[T]) float64 {
		return x
	}
}

// NewTreemap is d3.treemap(): a layout with the size [1, 1], no padding, no
// rounding and the squarify tiling method with the golden ratio.
func NewTreemap[T any]() *Treemap[T] {
	return &Treemap[T]{
		tile:          Squarify[T],
		round:         false,
		dx:            1,
		dy:            1,
		paddingStack:  []float64{0},
		paddingInner:  constantZero[T],
		paddingTop:    constantZero[T],
		paddingRight:  constantZero[T],
		paddingBottom: constantZero[T],
		paddingLeft:   constantZero[T],
	}
}

// Layout is treemap(root): it sets the bounds X0, Y0, X1, Y1 of root and of
// every descendant. Call Sum or Count on the root before.
func (t *Treemap[T]) Layout(root *Node[T]) *Node[T] {
	root.X0 = 0
	root.Y0 = 0
	root.X1 = t.dx
	root.Y1 = t.dy
	root.EachBefore(func(node *Node[T], _ int) { t.positionNode(node) })
	t.paddingStack = []float64{0}
	if t.round {
		root.EachBefore(func(node *Node[T], _ int) { roundNode(node) })
	}
	return root
}

// padding is paddingStack[depth]. An index that was never set is undefined
// in d3, and a sum with undefined is NaN.
func (t *Treemap[T]) padding(depth int) float64 {
	if depth < 0 || depth >= len(t.paddingStack) {
		return math.NaN()
	}
	return t.paddingStack[depth]
}

func (t *Treemap[T]) setPadding(depth int, p float64) {
	for len(t.paddingStack) <= depth {
		t.paddingStack = append(t.paddingStack, math.NaN())
	}
	t.paddingStack[depth] = p
}

func (t *Treemap[T]) positionNode(node *Node[T]) {
	p := t.padding(node.Depth)
	x0 := node.X0 + p
	y0 := node.Y0 + p
	x1 := node.X1 - p
	y1 := node.Y1 - p
	if x1 < x0 {
		x0 = (x0 + x1) / 2
		x1 = x0
	}
	if y1 < y0 {
		y0 = (y0 + y1) / 2
		y1 = y0
	}
	node.X0 = x0
	node.Y0 = y0
	node.X1 = x1
	node.Y1 = y1
	if node.Children != nil {
		p = float64(t.paddingInner(node) / 2)
		t.setPadding(node.Depth+1, p)
		x0 += t.paddingLeft(node) - p
		y0 += t.paddingTop(node) - p
		x1 -= t.paddingRight(node) - p
		y1 -= t.paddingBottom(node) - p
		if x1 < x0 {
			x0 = (x0 + x1) / 2
			x1 = x0
		}
		if y1 < y0 {
			y0 = (y0 + y1) / 2
			y1 = y0
		}
		t.tile(node, x0, y0, x1, y1)
	}
}

// Round is treemap.round(x): whether the layout rounds the bounds.
func (t *Treemap[T]) Round(x bool) *Treemap[T] {
	t.round = x
	return t
}

// Size is treemap.size([dx, dy]).
func (t *Treemap[T]) Size(dx, dy float64) *Treemap[T] {
	t.dx, t.dy = dx, dy
	return t
}

// Tile is treemap.tile(tile): the tiling method. The default is Squarify.
func (t *Treemap[T]) Tile(tile Tile[T]) *Treemap[T] {
	t.tile = tile
	return t
}

// Padding is treemap.padding(x): the inner and the outer padding.
func (t *Treemap[T]) Padding(x float64) *Treemap[T] {
	return t.PaddingInner(x).PaddingOuter(x)
}

// PaddingFunc is treemap.padding(x) for a function.
func (t *Treemap[T]) PaddingFunc(x func(node *Node[T]) float64) *Treemap[T] {
	return t.PaddingInnerFunc(x).PaddingOuterFunc(x)
}

// PaddingInner is treemap.paddingInner(x): the space between the children
// of a node.
func (t *Treemap[T]) PaddingInner(x float64) *Treemap[T] {
	t.paddingInner = constant[T](x)
	return t
}

// PaddingInnerFunc is treemap.paddingInner(x) for a function.
func (t *Treemap[T]) PaddingInnerFunc(x func(node *Node[T]) float64) *Treemap[T] {
	t.paddingInner = x
	return t
}

// PaddingOuter is treemap.paddingOuter(x): the padding of the four sides.
func (t *Treemap[T]) PaddingOuter(x float64) *Treemap[T] {
	return t.PaddingTop(x).PaddingRight(x).PaddingBottom(x).PaddingLeft(x)
}

// PaddingOuterFunc is treemap.paddingOuter(x) for a function.
func (t *Treemap[T]) PaddingOuterFunc(x func(node *Node[T]) float64) *Treemap[T] {
	return t.PaddingTopFunc(x).PaddingRightFunc(x).PaddingBottomFunc(x).PaddingLeftFunc(x)
}

// PaddingTop is treemap.paddingTop(x).
func (t *Treemap[T]) PaddingTop(x float64) *Treemap[T] {
	t.paddingTop = constant[T](x)
	return t
}

// PaddingTopFunc is treemap.paddingTop(x) for a function.
func (t *Treemap[T]) PaddingTopFunc(x func(node *Node[T]) float64) *Treemap[T] {
	t.paddingTop = x
	return t
}

// PaddingRight is treemap.paddingRight(x).
func (t *Treemap[T]) PaddingRight(x float64) *Treemap[T] {
	t.paddingRight = constant[T](x)
	return t
}

// PaddingRightFunc is treemap.paddingRight(x) for a function.
func (t *Treemap[T]) PaddingRightFunc(x func(node *Node[T]) float64) *Treemap[T] {
	t.paddingRight = x
	return t
}

// PaddingBottom is treemap.paddingBottom(x).
func (t *Treemap[T]) PaddingBottom(x float64) *Treemap[T] {
	t.paddingBottom = constant[T](x)
	return t
}

// PaddingBottomFunc is treemap.paddingBottom(x) for a function.
func (t *Treemap[T]) PaddingBottomFunc(x func(node *Node[T]) float64) *Treemap[T] {
	t.paddingBottom = x
	return t
}

// PaddingLeft is treemap.paddingLeft(x).
func (t *Treemap[T]) PaddingLeft(x float64) *Treemap[T] {
	t.paddingLeft = constant[T](x)
	return t
}

// PaddingLeftFunc is treemap.paddingLeft(x) for a function.
func (t *Treemap[T]) PaddingLeftFunc(x func(node *Node[T]) float64) *Treemap[T] {
	t.paddingLeft = x
	return t
}

// roundNode is the default export of round.js.
func roundNode[T any](node *Node[T]) {
	node.X0 = js.Round(node.X0)
	node.Y0 = js.Round(node.Y0)
	node.X1 = js.Round(node.X1)
	node.Y1 = js.Round(node.Y1)
}
