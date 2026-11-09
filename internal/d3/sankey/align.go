// Ported from d3-sankey 0.12.3, d3-sankey/src/align.js (BSD-3-Clause).

package sankey

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/array"
)

// Align is a node alignment: it returns the column of a node. n is the
// number of columns. The layout takes the floor of the result and clamps it
// to the columns.
type Align func(node *Node, n int) float64

func targetDepth(d *Link, _ int, _ []*Link) float64 {
	return float64(d.Target.Depth)
}

// Left is d3.sankeyLeft: the column of a node is its depth.
func Left(node *Node, _ int) float64 {
	return float64(node.Depth)
}

// Right is d3.sankeyRight: the column of a node is n - 1 - height.
func Right(node *Node, n int) float64 {
	return float64(n - 1 - node.Height)
}

// Justify is d3.sankeyJustify: as Left, but a node without outgoing links
// goes to the last column.
func Justify(node *Node, n int) float64 {
	if len(node.SourceLinks) > 0 {
		return float64(node.Depth)
	}
	return float64(n - 1)
}

// Center is d3.sankeyCenter: as Left, but a node without incoming links
// goes next to the nearest of its targets.
func Center(node *Node, _ int) float64 {
	if len(node.TargetLinks) > 0 {
		return float64(node.Depth)
	}
	if len(node.SourceLinks) > 0 {
		min, ok := array.MinFunc(node.SourceLinks, targetDepth)
		if !ok {
			return math.NaN()
		}
		return min - 1
	}
	return 0
}
