// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p4nodes/bk/BKCompactor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package bk

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// BKCompactor is the class BKCompactor: it places the blocks of a layout.
// Where it can, it keeps one more edge straight at the cost of a less compact
// layout.
type BKCompactor struct {
	// layeredGraph is the graph to process.
	layeredGraph *lgraph.LGraph
	// threshStrategy is the threshold strategy of the compaction.
	threshStrategy thresholdStrategy
	// ni has the neighbors of each node and its index in the layer.
	ni *NeighborhoodInformation
	// spacings has the spacing values.
	spacings *lgraph.Spacings
	// sinkNodes is the class graph. The Java map is a HashMap with LNode
	// keys, which iterates in insertion order in GWT.
	sinkNodes *gwt.Map[*lgraph.LNode, *classNode]
}

// NewBKCompactor is new BKCompactor(layeredGraph, ni).
func NewBKCompactor(layeredGraph *lgraph.LGraph, ni *NeighborhoodInformation) *BKCompactor {
	c := &BKCompactor{
		layeredGraph: layeredGraph,
		ni:           ni,
		spacings:     prop.Get(layeredGraph, iprops.Spacings),
		sinkNodes:    gwt.NewMap[*lgraph.LNode, *classNode](),
	}

	// configure the requested threshold strategy
	if prop.Get(layeredGraph, lopts.NodePlacementBkEdgeStraightening) == lenum.EdgeStraighteningStrategyImproveStraightness {
		c.threshStrategy = newSimpleThresholdStrategy()
	} else {
		// mimics the original compaction strategy without additional straightening
		c.threshStrategy = newNullThresholdStrategy()
	}
	return c
}

// horizontalCompaction is horizontalCompaction(bal): it computes the
// coordinates of the blocks and of their nodes.
//
// First it places each block so that no two blocks overlap. Then it moves the
// classes of blocks toward each other where there is space.
func (c *BKCompactor) horizontalCompaction(bal *BKAlignedLayout) {
	// Initialize fields with basic values, partially depending on the direction
	for _, layer := range c.layeredGraph.Layers {
		for _, node := range layer.Nodes {
			bal.sink[node.ID] = node
			if bal.vdir == VDirectionUp {
				bal.shift[node.ID] = math.Inf(-1)
			} else {
				bal.shift[node.ID] = math.Inf(1)
			}
			bal.shiftNull[node.ID] = false
		}
	}
	// clear any previous sinks
	c.sinkNodes.Clear()

	// If the horizontal direction is LEFT, the layers are traversed from right to left
	layers := c.layeredGraph.Layers
	layerCount := len(layers)
	layerAt := func(i int) *lgraph.Layer {
		if bal.hdir == HDirectionLeft {
			return layers[layerCount-1-i]
		}
		return layers[i]
	}

	// init threshold strategy
	c.threshStrategy.init(bal, c.ni)
	// mark all blocks as unplaced
	for i := range bal.yNull {
		bal.yNull[i] = true
	}

	for li := 0; li < layerCount; li++ {
		layer := layerAt(li)
		// As with layers, we need a reversed iterator for blocks for different directions
		nodes := layer.Nodes
		nodeCount := len(nodes)

		// Do an initial placement for all blocks
		for ni := 0; ni < nodeCount; ni++ {
			v := nodes[ni]
			if bal.vdir == VDirectionUp {
				v = nodes[nodeCount-1-ni]
			}
			if bal.root[v.ID] == v {
				c.placeBlock(v, bal)
			}
		}
	}

	// Try to compact classes by shifting them towards each other if there is space between them.
	// Other than the original algorithm we use a "class graph" here in conjunction with a longest
	// path layering based on previously calculated separations between any pair of adjacent classes.
	// This allows to have different node sizes and disconnected graphs.
	c.placeClasses(bal)

	// apply final coordinates
	for li := 0; li < layerCount; li++ {
		layer := layerAt(li)
		for _, v := range layer.Nodes {
			rootID := bal.root[v.ID].ID
			bal.y[v.ID] = bal.y[rootID]
			bal.yNull[v.ID] = bal.yNull[rootID]

			// If this is the root node of the block, check if the whole block can be shifted to
			// further compact the drawing (the block's non-root nodes will be processed later by
			// this loop and will thus use the updated y position calculated here)
			if v == bal.root[v.ID] {
				sinkID := bal.sink[v.ID].ID
				if bal.shiftNull[sinkID] {
					// The Java code fails with a NullPointerException: the
					// class graph has a cycle, and a class got no shift.
					lgraph.Bail("BKCompactor: the shift of a class is null")
				}
				sinkShift := bal.shift[sinkID]

				if (bal.vdir == VDirectionUp && sinkShift > math.Inf(-1)) ||
					(bal.vdir == VDirectionDown && sinkShift < math.Inf(1)) {

					bal.setY(v.ID, bal.yAt(v.ID)+sinkShift)
				}
			}
		}
	}

	// all blocks were placed, shift latecomers
	c.threshStrategy.postProcess()
}

// placeBlock is placeBlock(root, bal): it places a block. It walks the nodes
// of the block and moves the whole block down or up where a block that is
// already placed is in the way.
func (c *BKCompactor) placeBlock(root *lgraph.LNode, bal *BKAlignedLayout) {
	// Skip if the block was already placed
	if !bal.yNull[root.ID] {
		return
	}

	// Initial placement
	// As opposed to the original algorithm we cannot rely on the fact that
	//  0.0 as initial block position is always feasible. This is due to
	//  the inside shift allowing for negative block positions in conjunction with
	//  a RIGHT (bottom-to-top) traversal direction. Computing the minimum with
	//  an initial position of 0.0 thus leads to wrong results.
	// The wrong behavior is documented in KIPRA-1426
	isInitialAssignment := true
	bal.setY(root.ID, 0.0)

	// Iterate through block and determine, where the block can be placed (until we arrive at the
	// block's root node again)
	currentNode := root
	thresh := math.Inf(1)
	if bal.vdir == VDirectionDown {
		thresh = math.Inf(-1)
	}
	for {
		currentIndexInLayer := c.ni.nodeIndex[currentNode.ID]
		currentLayerSize := len(currentNode.Layer().Nodes)

		// If the node is the top or bottom node of its layer, it can be placed safely since it is
		// the first to be placed in its layer. If it's not, we'll have to check its neighbours
		if (bal.vdir == VDirectionDown && currentIndexInLayer > 0) ||
			(bal.vdir == VDirectionUp && currentIndexInLayer < (currentLayerSize-1)) {

			// Get the node which is above / below the current node as well as the root of its block
			var neighbor *lgraph.LNode
			if bal.vdir == VDirectionUp {
				neighbor = nodeAt(currentNode.Layer().Nodes, currentIndexInLayer+1)
			} else {
				neighbor = nodeAt(currentNode.Layer().Nodes, currentIndexInLayer-1)
			}
			neighborRoot := bal.root[neighbor.ID]

			// Ensure the neighbor was already placed
			c.placeBlock(neighborRoot, bal)

			// calculate threshold value for additional straight edges
			// this call has to be _after_ place block, otherwise the
			// order of the elements in the postprocessing queue is wrong
			thresh = c.threshStrategy.calculateThreshold(thresh, root, currentNode)

			// Note that the two nodes and their blocks form a unit called class in the original
			// algorithm. These are combinations of blocks which play a role in the final compaction
			if bal.sink[root.ID] == root {
				bal.sink[root.ID] = bal.sink[neighborRoot.ID]
			}

			// Check if the blocks of the two nodes are members of the same class
			if bal.sink[root.ID] == bal.sink[neighborRoot.ID] {
				// They are part of the same class

				// The minimal spacing between the two nodes depends on their node type
				spacing := verticalSpacing(c.spacings, currentNode, neighbor)

				// Determine the block's final position
				if bal.vdir == VDirectionUp {

					currentBlockPosition := bal.yAt(root.ID)
					newPosition := bal.yAt(neighborRoot.ID) +
						bal.innerShift[neighbor.ID] -
						neighbor.Margin.Top -
						spacing -
						currentNode.Margin.Bottom -
						currentNode.Size.Y -
						bal.innerShift[currentNode.ID]

					if isInitialAssignment {
						isInitialAssignment = false
						bal.setY(root.ID, min(newPosition, thresh))
					} else {
						bal.setY(root.ID, min(currentBlockPosition, min(newPosition, thresh)))
					}

				} else { // DOWN

					currentBlockPosition := bal.yAt(root.ID)
					newPosition := bal.yAt(neighborRoot.ID) +
						bal.innerShift[neighbor.ID] +
						neighbor.Size.Y +
						neighbor.Margin.Bottom +
						spacing +
						currentNode.Margin.Top -
						bal.innerShift[currentNode.ID]

					if isInitialAssignment {
						isInitialAssignment = false
						bal.setY(root.ID, max(newPosition, thresh))
					} else {
						bal.setY(root.ID, max(currentBlockPosition, max(newPosition, thresh)))
					}
				}

			} else { // CLASSES

				// They are not part of the same class. Compute how the two classes can be compacted
				// later. Hence we determine a minimal required space between the two classes
				// relative two the two class sinks.
				spacing := prop.Get(c.layeredGraph, lopts.SpacingNodeNode)

				sinkNode := c.getOrCreateClassNode(bal.sink[root.ID], bal)
				neighborSink := c.getOrCreateClassNode(bal.sink[neighborRoot.ID], bal)

				if bal.vdir == VDirectionUp {

					//  possible setup:
					//  root         --> currentNode
					//  neighborRoot --> neighbor
					requiredSpace := bal.yAt(root.ID) +
						bal.innerShift[currentNode.ID] +
						currentNode.Size.Y +
						currentNode.Margin.Bottom +
						spacing -
						(bal.yAt(neighborRoot.ID) +
							bal.innerShift[neighbor.ID] -
							neighbor.Margin.Top)

					// add an edge to the class graph
					sinkNode.addEdge(neighborSink, requiredSpace)

				} else { // DOWN
					//  possible setup:
					//  neighborRoot --> neighbor
					//  root         --> currentNode
					requiredSpace := bal.yAt(root.ID) +
						bal.innerShift[currentNode.ID] -
						currentNode.Margin.Top -
						bal.yAt(neighborRoot.ID) -
						bal.innerShift[neighbor.ID] -
						neighbor.Size.Y -
						neighbor.Margin.Bottom -
						spacing

					// add an edge to the class graph
					sinkNode.addEdge(neighborSink, requiredSpace)
				}
			}
		} else {
			thresh = c.threshStrategy.calculateThreshold(thresh, root, currentNode)
		}

		// Get the next node in the block
		currentNode = bal.align[currentNode.ID]
		if currentNode == root {
			break
		}
	}

	c.threshStrategy.finishBlock(root)
}

// placeClasses is placeClasses(bal): it gives each class its shift, with a
// longest path layering of the class graph.
func (c *BKCompactor) placeClasses(bal *BKAlignedLayout) {
	// collect sinks of the class graph
	var sinks []*classNode
	for _, n := range c.sinkNodes.Values() {
		if n.indegree == 0 {
			sinks = append(sinks, n)
		}
	}

	// propagate shifts in a longest path layering fashion
	for head := 0; head < len(sinks); head++ {
		n := sinks[head]

		// position the root of the class node tree
		if n.classShiftNull {
			n.classShift = 0
			n.classShiftNull = false
		}

		for _, e := range n.outgoing {

			// initial position of a target does not depend on previous positions
			// (we need this as we cannot assume the top-most position to be 0)
			if e.target.classShiftNull {
				e.target.classShift = n.classShift + e.separation
				e.target.classShiftNull = false
			} else if bal.vdir == VDirectionDown {
				e.target.classShift = min(e.target.classShift, n.classShift+e.separation)
			} else {
				e.target.classShift = max(e.target.classShift, n.classShift+e.separation)
			}

			e.target.indegree--

			if e.target.indegree == 0 {
				sinks = append(sinks, e.target)
			}
		}
	}

	// remember final shifts for all classes such that they
	// can be applied as absolute coordinates
	for _, n := range c.sinkNodes.Values() {
		bal.shift[n.node.ID] = n.classShift
		bal.shiftNull[n.node.ID] = n.classShiftNull
	}
}

// getOrCreateClassNode is getOrCreateClassNode(sinkNode, bal).
func (c *BKCompactor) getOrCreateClassNode(sinkNode *lgraph.LNode, bal *BKAlignedLayout) *classNode {
	node := c.sinkNodes.Get(sinkNode)
	if node == nil {
		node = &classNode{classShiftNull: true}
		node.node = sinkNode
		c.sinkNodes.Put(node.node, node)
	}
	return node
}

// classNode is BKCompactor.ClassNode: a node of the class graph.
type classNode struct {
	// classShift is the shift of the class. classShiftNull tells that the
	// Java field is null.
	classShift     float64
	classShiftNull bool
	node           *lgraph.LNode
	outgoing       []classEdge
	indegree       int
}

// addEdge is addEdge(target, separation).
func (n *classNode) addEdge(target *classNode, separation float64) {
	se := classEdge{}
	se.target = target
	se.separation = separation
	target.indegree++
	n.outgoing = append(n.outgoing, se)
}

// classEdge is BKCompactor.ClassEdge: an edge of the class graph, with the
// separation that the two classes need.
type classEdge struct {
	separation float64
	target     *classNode
}
