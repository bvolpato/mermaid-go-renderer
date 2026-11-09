// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p4nodes/bk/BKAlignedLayout.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package bk has the node placer of Brandes and Koepf (BKNodePlacer), the
// default node placement of the layered algorithm.
package bk

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// Math.min and Math.max are the builtin functions min and max in this
// package: they give NaN when an argument is NaN, as JavaScript does.

// VDirection is the enum BKAlignedLayout.VDirection. The balanced layout has
// the direction null, which is VDirectionNone.
type VDirection int

// The constants of VDirection.
const (
	VDirectionNone VDirection = iota - 1
	// VDirectionDown is DOWN: the iteration goes from the top to the bottom.
	VDirectionDown
	// VDirectionUp is UP: the iteration goes from the bottom to the top.
	VDirectionUp
)

// HDirection is the enum BKAlignedLayout.HDirection. The balanced layout has
// the direction null, which is HDirectionNone.
type HDirection int

// The constants of HDirection.
const (
	HDirectionNone HDirection = iota - 1
	// HDirectionRight is RIGHT: the layers are visited from the first one to
	// the last one.
	HDirectionRight
	// HDirectionLeft is LEFT: the layers are visited from the last one to the
	// first one.
	HDirectionLeft
)

// BKAlignedLayout is the class BKAlignedLayout: the data of one of the four
// layouts. Each slice has one entry for each node, at the id of the node.
//
// The Java arrays y and shift have the type Double[], and an entry can be
// null. yNull and shiftNull tell which entries are null. Each other Double
// entry has a value before the code reads it.
type BKAlignedLayout struct {
	// root is the root node of the block of each node.
	root []*lgraph.LNode
	// blockSize is the size of a block, at the id of its root.
	blockSize []float64
	// align is the next node of the block, or the root for the last node.
	align []*lgraph.LNode
	// innerShift is the offset of a node in its block.
	innerShift []float64
	// sink is the root of the class of a block root.
	sink []*lgraph.LNode
	// shift is the offset of a class, at the id of its sink.
	shift     []float64
	shiftNull []bool
	// y is the coordinate of each node.
	y     []float64
	yNull []bool
	// vdir and hdir are the directions of the layout.
	vdir VDirection
	hdir HDirection
	// su tells for a block root that its block is part of a straightened
	// edge.
	su []bool
	// od tells for a block root that its block has only dummy nodes.
	od []bool

	layeredGraph *lgraph.LGraph
	spacings     *lgraph.Spacings
}

// NewBKAlignedLayout is new BKAlignedLayout(layeredGraph, nodeCount, vdir,
// hdir).
func NewBKAlignedLayout(layeredGraph *lgraph.LGraph, nodeCount int, vdir VDirection, hdir HDirection) *BKAlignedLayout {
	bal := &BKAlignedLayout{
		layeredGraph: layeredGraph,
		spacings:     prop.Get(layeredGraph, iprops.Spacings),
		root:         make([]*lgraph.LNode, nodeCount),
		blockSize:    make([]float64, nodeCount),
		align:        make([]*lgraph.LNode, nodeCount),
		innerShift:   make([]float64, nodeCount),
		sink:         make([]*lgraph.LNode, nodeCount),
		shift:        make([]float64, nodeCount),
		shiftNull:    make([]bool, nodeCount),
		y:            make([]float64, nodeCount),
		yNull:        make([]bool, nodeCount),
		su:           make([]bool, nodeCount),
		od:           make([]bool, nodeCount),
		vdir:         vdir,
		hdir:         hdir,
	}
	for i := range bal.od {
		bal.od[i] = true
		bal.yNull[i] = true
		bal.shiftNull[i] = true
	}
	return bal
}

// cleanup is cleanup().
func (bal *BKAlignedLayout) cleanup() {
	bal.root = nil
	bal.blockSize = nil
	bal.align = nil
	bal.innerShift = nil
	bal.sink = nil
	bal.shift = nil
	bal.shiftNull = nil
	bal.y = nil
	bal.yNull = nil
}

// yAt is the read of y[id] as a double. The Java code fails with a
// NullPointerException for an entry that is null.
func (bal *BKAlignedLayout) yAt(id int) float64 {
	if bal.yNull[id] {
		lgraph.Bail("BKAlignedLayout: the coordinate of a node is null")
	}
	return bal.y[id]
}

// setY is y[id] = value.
func (bal *BKAlignedLayout) setY(id int, value float64) {
	bal.y[id] = value
	bal.yNull[id] = false
}

// verticalSpacing is spacings.getVerticalSpacing(n1, n2). The Java code fails
// with a NullPointerException for a graph without the property spacings.
func (bal *BKAlignedLayout) verticalSpacing(n1, n2 *lgraph.LNode) float64 {
	return verticalSpacing(bal.spacings, n1, n2)
}

func verticalSpacing(spacings *lgraph.Spacings, n1, n2 *lgraph.LNode) float64 {
	if spacings == nil {
		lgraph.Bail("BKNodePlacer: the graph has no spacings")
	}
	return spacings.VerticalSpacing(n1, n2)
}

// layoutSize is layoutSize(): the size of the layout for a comparison.
func (bal *BKAlignedLayout) layoutSize() float64 {
	minY := math.Inf(1)
	maxY := math.Inf(-1)
	for _, layer := range bal.layeredGraph.Layers {
		for _, n := range layer.Nodes {
			yMin := bal.yAt(n.ID)
			yMax := yMin + bal.blockSize[bal.root[n.ID].ID]
			minY = min(minY, yMin)
			maxY = max(maxY, yMax)
		}
	}
	return maxY - minY
}

// calculateDelta is calculateDelta(src, tgt): a value above 0 when the port
// tgt is below the port src.
func (bal *BKAlignedLayout) calculateDelta(src, tgt *lgraph.LPort) float64 {
	srcPos := bal.yAt(src.Node().ID) + bal.innerShift[src.Node().ID] + src.Pos.Y + src.Anchor.Y
	tgtPos := bal.yAt(tgt.Node().ID) + bal.innerShift[tgt.Node().ID] + tgt.Pos.Y + tgt.Anchor.Y
	return tgtPos - srcPos
}

// shiftBlock is shiftBlock(rootNode, delta): it moves each node of a block.
func (bal *BKAlignedLayout) shiftBlock(rootNode *lgraph.LNode, delta float64) {
	current := rootNode
	for {
		newPos := bal.yAt(current.ID) + delta
		bal.setY(current.ID, newPos)
		current = bal.align[current.ID]
		if current == rootNode {
			break
		}
	}
}

// checkSpaceAbove is checkSpaceAbove(blockRoot, delta, ni): the distance, at
// most delta, that the block can move up.
func (bal *BKAlignedLayout) checkSpaceAbove(blockRoot *lgraph.LNode, delta float64, ni *NeighborhoodInformation) float64 {
	availableSpace := delta
	rootNode := blockRoot
	// iterate through the block
	current := rootNode
	for {
		current = bal.align[current.ID]
		// get minimum possible position of the current node
		minYCurrent := bal.getMinY(current)

		neighbor := bal.getUpperNeighbor(current, ni)
		if neighbor != nil {
			maxYNeighbor := bal.getMaxY(neighbor)
			// minimal position at which the current block node could validly be placed
			availableSpace = min(availableSpace, minYCurrent-(maxYNeighbor+bal.verticalSpacing(current, neighbor)))
		}
		// until we wrap around
		if rootNode == current {
			break
		}
	}
	return availableSpace
}

// checkSpaceBelow is checkSpaceBelow(blockRoot, delta, ni): the distance, at
// most delta, that the block can move down.
func (bal *BKAlignedLayout) checkSpaceBelow(blockRoot *lgraph.LNode, delta float64, ni *NeighborhoodInformation) float64 {
	availableSpace := delta
	rootNode := blockRoot
	// iterate through the block
	current := rootNode
	for {
		current = bal.align[current.ID]
		// get maximum possible position of the current node
		maxYCurrent := bal.getMaxY(current)

		// get the lower neighbor and check its position allows shifting
		neighbor := bal.getLowerNeighbor(current, ni)
		if neighbor != nil {
			minYNeighbor := bal.getMinY(neighbor)

			// minimal position at which the current block node could validly be placed
			availableSpace = min(availableSpace, minYNeighbor-(maxYCurrent+bal.verticalSpacing(current, neighbor)))
		}
		// until we wrap around
		if rootNode == current {
			break
		}
	}
	return availableSpace
}

// getMinY is getMinY(n): the top of the node with its margin.
func (bal *BKAlignedLayout) getMinY(n *lgraph.LNode) float64 {
	rootNode := bal.root[n.ID]
	return bal.yAt(rootNode.ID) + bal.innerShift[n.ID] - n.Margin.Top
}

// getMaxY is getMaxY(n): the bottom of the node with its margin.
func (bal *BKAlignedLayout) getMaxY(n *lgraph.LNode) float64 {
	rootNode := bal.root[n.ID]
	return bal.yAt(rootNode.ID) + bal.innerShift[n.ID] + n.Size.Y + n.Margin.Bottom
}

// getLowerNeighbor is getLowerNeighbor(n, ni): the next node of the layer, or
// nil.
func (bal *BKAlignedLayout) getLowerNeighbor(n *lgraph.LNode, ni *NeighborhoodInformation) *lgraph.LNode {
	l := n.Layer()
	layerIndex := ni.nodeIndex[n.ID]
	if layerIndex < len(l.Nodes)-1 {
		return nodeAt(l.Nodes, layerIndex+1)
	}
	return nil
}

// getUpperNeighbor is getUpperNeighbor(n, ni): the node before n in the
// layer, or nil.
func (bal *BKAlignedLayout) getUpperNeighbor(n *lgraph.LNode, ni *NeighborhoodInformation) *lgraph.LNode {
	l := n.Layer()
	layerIndex := ni.nodeIndex[n.ID]
	if layerIndex > 0 {
		return nodeAt(l.Nodes, layerIndex-1)
	}
	return nil
}

// nodeAt is List.get(index). The Java code fails with an
// IndexOutOfBoundsException for an index outside the list.
func nodeAt(nodes []*lgraph.LNode, index int) *lgraph.LNode {
	if index < 0 || index >= len(nodes) {
		lgraph.Bail("BKNodePlacer: index %d is outside a list of %d nodes", index, len(nodes))
	}
	return nodes[index]
}

// String is toString().
func (bal *BKAlignedLayout) String() string {
	result := ""
	if bal.hdir == HDirectionRight {
		result += "RIGHT"
	} else if bal.hdir == HDirectionLeft {
		result += "LEFT"
	}
	if bal.vdir == VDirectionDown {
		result += "DOWN"
	} else if bal.vdir == VDirectionUp {
		result += "UP"
	} else {
		result += "BALANCED"
	}
	return result
}
