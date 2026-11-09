// Ported from d3-hierarchy 3.1.2, d3-hierarchy/src/treemap: squarify.js,
// resquarify.js, binary.js, dice.js, slice.js and sliceDice.js (ISC).

package hierarchy

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// phi is the golden ratio: the default ratio of Squarify. It is a variable
// so that Go computes it with float64 arithmetic, as JavaScript does.
var phi = (1 + math.Sqrt(5)) / 2

// truthy reports whether a value is truthy in JavaScript: not zero and not
// NaN. NaN also stands for a value that is undefined.
func truthy(value float64) bool {
	return value != 0 && !math.IsNaN(value)
}

// squarifyRow is a row of the squarify tiling: some adjacent children that
// share one side.
type squarifyRow[T any] struct {
	value    float64
	dice     bool
	children []*Node[T]
}

// squarifyRows is the result of squarifyRatio, with the ratio that
// Resquarify stores with it.
type squarifyRows[T any] struct {
	rows  []*squarifyRow[T]
	ratio float64
}

// dice is treemapDice for a list of nodes with the value of their parent.
func dice[T any](nodes []*Node[T], value, x0, y0, x1, y1 float64) {
	// k = parent.value && (x1 - x0) / parent.value
	k := value
	if truthy(value) {
		k = (x1 - x0) / value
	}

	for _, node := range nodes {
		node.Y0, node.Y1 = y0, y1
		node.X0 = x0
		x0 += float64(node.Value * k)
		node.X1 = x0
	}
}

// slice is treemapSlice for a list of nodes with the value of their parent.
func slice[T any](nodes []*Node[T], value, x0, y0, x1, y1 float64) {
	// k = parent.value && (y1 - y0) / parent.value
	k := value
	if truthy(value) {
		k = (y1 - y0) / value
	}

	for _, node := range nodes {
		node.X0, node.X1 = x0, x1
		node.Y0 = y0
		y0 += float64(node.Value * k)
		node.Y1 = y0
	}
}

// Dice is d3.treemapDice: it divides the rectangle horizontally.
func Dice[T any](parent *Node[T], x0, y0, x1, y1 float64) {
	dice(parent.Children, parent.Value, x0, y0, x1, y1)
}

// Slice is d3.treemapSlice: it divides the rectangle vertically.
func Slice[T any](parent *Node[T], x0, y0, x1, y1 float64) {
	slice(parent.Children, parent.Value, x0, y0, x1, y1)
}

// SliceDice is d3.treemapSliceDice: Slice for a node of odd depth and Dice
// for a node of even depth.
func SliceDice[T any](parent *Node[T], x0, y0, x1, y1 float64) {
	if parent.Depth&1 != 0 {
		Slice(parent, x0, y0, x1, y1)
	} else {
		Dice(parent, x0, y0, x1, y1)
	}
}

func squarifyRatio[T any](ratio float64, parent *Node[T], x0, y0, x1, y1 float64) []*squarifyRow[T] {
	var rows []*squarifyRow[T]
	nodes := parent.Children
	i0 := 0
	i1 := 0
	n := len(nodes)
	value := parent.Value

	for i0 < n {
		dx, dy := x1-x0, y1-y0

		// Find the next non-empty node.
		var sumValue float64
		for {
			sumValue = nodes[i1].Value
			i1++
			if !(!truthy(sumValue) && i1 < n) {
				break
			}
		}
		minValue, maxValue := sumValue, sumValue
		alpha := js.Max(dy/dx, dx/dy) / (value * ratio)
		beta := sumValue * sumValue * alpha
		minRatio := js.Max(maxValue/beta, beta/minValue)

		// Keep adding nodes while the aspect ratio maintains or improves.
		for ; i1 < n; i1++ {
			nodeValue := nodes[i1].Value
			sumValue += nodeValue
			if nodeValue < minValue {
				minValue = nodeValue
			}
			if nodeValue > maxValue {
				maxValue = nodeValue
			}
			beta = sumValue * sumValue * alpha
			newRatio := js.Max(maxValue/beta, beta/minValue)
			if newRatio > minRatio {
				sumValue -= nodeValue
				break
			}
			minRatio = newRatio
		}

		// Position and record the row orientation. The row has a copy of
		// the children, as nodes.slice(i0, i1) is. JavaScript reads the
		// arguments from left to right, so the row starts at the value that
		// x0 or y0 has before the assignment.
		row := &squarifyRow[T]{value: sumValue, dice: dx < dy, children: append([]*Node[T](nil), nodes[i0:i1]...)}
		rows = append(rows, row)
		if row.dice {
			rowY0, rowY1 := y0, y1
			if truthy(value) {
				y0 += float64(dy * sumValue / value)
				rowY1 = y0
			}
			dice(row.children, row.value, x0, rowY0, x1, rowY1)
		} else {
			rowX0, rowX1 := x0, x1
			if truthy(value) {
				x0 += float64(dx * sumValue / value)
				rowX1 = x0
			}
			slice(row.children, row.value, rowX0, y0, rowX1, y1)
		}
		value -= sumValue
		i0 = i1
	}

	return rows
}

// SquarifyRatio is d3.treemapSquarify.ratio(ratio): the squarify tiling
// method with the desired aspect ratio of the rectangles. A ratio that is
// not above one counts as one.
func SquarifyRatio[T any](ratio float64) Tile[T] {
	if !(ratio > 1) {
		ratio = 1
	}
	return func(parent *Node[T], x0, y0, x1, y1 float64) {
		squarifyRatio(ratio, parent, x0, y0, x1, y1)
	}
}

// Squarify is d3.treemapSquarify: the squarified tiling method with the
// golden ratio.
func Squarify[T any](parent *Node[T], x0, y0, x1, y1 float64) {
	squarifyRatio(phi, parent, x0, y0, x1, y1)
}

// ResquarifyRatio is d3.treemapResquarify.ratio(ratio): as SquarifyRatio,
// but a node keeps the rows of its first layout, so that a later layout with
// other values changes only the sizes of the rectangles.
func ResquarifyRatio[T any](ratio float64) Tile[T] {
	if !(ratio > 1) {
		ratio = 1
	}
	return func(parent *Node[T], x0, y0, x1, y1 float64) {
		if rows := parent.squarify; rows != nil && rows.ratio == ratio {
			value := parent.Value

			for _, row := range rows.rows {
				nodes := row.children
				row.value = 0
				for _, node := range nodes {
					row.value += node.Value
				}
				if row.dice {
					rowY0, rowY1 := y0, y1
					if truthy(value) {
						y0 += float64((y1 - y0) * row.value / value)
						rowY1 = y0
					}
					dice(row.children, row.value, x0, rowY0, x1, rowY1)
				} else {
					rowX0, rowX1 := x0, x1
					if truthy(value) {
						x0 += float64((x1 - x0) * row.value / value)
						rowX1 = x0
					}
					slice(row.children, row.value, rowX0, y0, rowX1, y1)
				}
				value -= row.value
			}
		} else {
			parent.squarify = &squarifyRows[T]{rows: squarifyRatio(ratio, parent, x0, y0, x1, y1), ratio: ratio}
		}
	}
}

// Resquarify is d3.treemapResquarify: ResquarifyRatio with the golden
// ratio.
func Resquarify[T any]() Tile[T] {
	return ResquarifyRatio[T](phi)
}

// Binary is d3.treemapBinary: it divides the children into two groups of
// about the same value, recursively.
func Binary[T any](parent *Node[T], x0, y0, x1, y1 float64) {
	nodes := parent.Children
	n := len(nodes)
	if n == 0 {
		return
	}
	sums := make([]float64, n+1)

	sum := 0.0
	for i := 0; i < n; i++ {
		sum += nodes[i].Value
		sums[i+1] = sum
	}

	var partition func(i, j int, value, x0, y0, x1, y1 float64)
	partition = func(i, j int, value, x0, y0, x1, y1 float64) {
		if i >= j-1 {
			node := nodes[i]
			node.X0, node.Y0 = x0, y0
			node.X1, node.Y1 = x1, y1
			return
		}

		valueOffset := sums[i]
		valueTarget := float64(value/2) + valueOffset
		k := i + 1
		hi := j - 1

		for k < hi {
			mid := int(uint32(k+hi) >> 1)
			if sums[mid] < valueTarget {
				k = mid + 1
			} else {
				hi = mid
			}
		}

		if (valueTarget-sums[k-1]) < (sums[k]-valueTarget) && i+1 < k {
			k--
		}

		valueLeft := sums[k] - valueOffset
		valueRight := value - valueLeft

		if (x1 - x0) > (y1 - y0) {
			xk := x1
			if truthy(value) {
				xk = (float64(x0*valueRight) + float64(x1*valueLeft)) / value
			}
			partition(i, k, valueLeft, x0, y0, xk, y1)
			partition(k, j, valueRight, xk, y0, x1, y1)
		} else {
			yk := y1
			if truthy(value) {
				yk = (float64(y0*valueRight) + float64(y1*valueLeft)) / value
			}
			partition(i, k, valueLeft, x0, y0, x1, yk)
			partition(k, j, valueRight, x0, yk, x1, y1)
		}
	}

	partition(0, n, parent.Value, x0, y0, x1, y1)
}
