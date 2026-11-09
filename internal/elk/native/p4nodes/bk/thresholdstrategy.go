// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p4nodes/bk/ThresholdStrategy.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package bk

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
)

// thresholdLimit is the constant THRESHOLD (Double.MAX_VALUE).
const thresholdLimit = math.MaxFloat64

// thresholdStrategy is the abstract class ThresholdStrategy: the part of the
// compaction that decides how far a block may move to keep one more edge
// straight.
type thresholdStrategy interface {
	// init is init(theBal, theNi): it resets the state for a layout.
	init(theBal *BKAlignedLayout, theNi *NeighborhoodInformation)
	// finishBlock is finishBlock(n): it marks the block with the root n as
	// placed.
	finishBlock(n *lgraph.LNode)
	// calculateThreshold is calculateThreshold(oldThresh, blockRoot,
	// currentNode): a bound that keeps one more edge straight. currentNode is
	// a node of the block, and it can be the root.
	calculateThreshold(oldThresh float64, blockRoot, currentNode *lgraph.LNode) float64
	// postProcess is postProcess(): it handles the nodes that can lead to
	// more straight edges after each block has a first position.
	postProcess()
}

// thresholdStrategyBase has the fields and the methods of the abstract class
// ThresholdStrategy.
type thresholdStrategyBase struct {
	// bal is the layout that is processed.
	bal *BKAlignedLayout
	// ni is the neighborhood information of the graph.
	ni *NeighborhoodInformation
	// blockFinished has the roots of the blocks that are placed.
	blockFinished map[*lgraph.LNode]struct{}
	// postProcessablesQueue has the blocks that the compaction postponed, in
	// order. queueHead is the index of the first one that is not handled.
	postProcessablesQueue []*postprocessable
	queueHead             int
	// postProcessablesStack has the postponed blocks that are handled a
	// second time, in reverse order.
	postProcessablesStack []*postprocessable
}

// init is init(theBal, theNi).
func (s *thresholdStrategyBase) init(theBal *BKAlignedLayout, theNi *NeighborhoodInformation) {
	s.bal = theBal
	s.ni = theNi
	if s.blockFinished == nil {
		s.blockFinished = map[*lgraph.LNode]struct{}{}
	} else {
		clear(s.blockFinished)
	}
	s.postProcessablesQueue = s.postProcessablesQueue[:0]
	s.queueHead = 0
	s.postProcessablesStack = s.postProcessablesStack[:0]
}

// finishBlock is finishBlock(n).
func (s *thresholdStrategyBase) finishBlock(n *lgraph.LNode) {
	s.blockFinished[n] = struct{}{}
}

// getOther is getOther(edge, n): the node at the other end of the edge.
func getOther(edge *lgraph.LEdge, n *lgraph.LNode) *lgraph.LNode {
	if edge.Source.Node() == n {
		return edge.Target.Node()
	} else if edge.Target.Node() == n {
		return edge.Source.Node()
	}
	// The Java code throws an IllegalArgumentException.
	lgraph.Bail("ThresholdStrategy: the node %v is neither source nor target of the edge %v", n, edge)
	return nil
}

// nullThresholdStrategy is ThresholdStrategy.NullThresholdStrategy: the
// compaction of the original algorithm. Its threshold has no effect.
type nullThresholdStrategy struct {
	thresholdStrategyBase
}

func newNullThresholdStrategy() *nullThresholdStrategy { return &nullThresholdStrategy{} }

// calculateThreshold is calculateThreshold(oldThresh, blockRoot,
// currentNode).
func (s *nullThresholdStrategy) calculateThreshold(oldThresh float64, blockRoot, currentNode *lgraph.LNode) float64 {
	if s.bal.vdir == VDirectionUp {
		// new value calculated using min(a,thresh) --> thresh = +infty has no effect
		return math.Inf(1)
	}
	return math.Inf(-1)
}

// postProcess is postProcess().
func (s *nullThresholdStrategy) postProcess() {}

// simpleThresholdStrategy is ThresholdStrategy.SimpleThresholdStrategy. It
// computes a threshold only for the first node and for the last node of a
// block, and it takes the first edge that is valid.
type simpleThresholdStrategy struct {
	thresholdStrategyBase
}

func newSimpleThresholdStrategy() *simpleThresholdStrategy { return &simpleThresholdStrategy{} }

// calculateThreshold is calculateThreshold(oldThresh, blockRoot,
// currentNode).
func (s *simpleThresholdStrategy) calculateThreshold(oldThresh float64, blockRoot, currentNode *lgraph.LNode) float64 {
	// just the root or last node of a block

	// Remember that for blocks with a single node both flags can be true
	isRoot := blockRoot == currentNode
	isLast := s.bal.align[currentNode.ID] == blockRoot

	if !(isRoot || isLast) {
		return oldThresh
	}

	// Remember two things:
	//  1) it is not guaranteed that adjacent nodes are already placed
	//  2) blocks can consist of a single node implying that the current
	//     node is both the root and the last node

	// The Java code has the same statements for the direction RIGHT and for
	// the direction LEFT.
	t := oldThresh
	if isRoot {
		t = s.getBound(blockRoot, true)
	}
	if math.IsInf(t, 0) && isLast {
		t = s.getBound(currentNode, false)
	}

	return t
}

// pickEdge is pickEdge(pp): it looks for an edge of the node that can be
// straightened. It sets pp.edge to the edge or to nil, and pp.hasEdges tells
// whether the node has edges that can become valid later.
func (s *simpleThresholdStrategy) pickEdge(pp *postprocessable) *postprocessable {
	bal := s.bal

	var incoming bool
	if pp.isRoot {
		incoming = bal.hdir == HDirectionRight
	} else {
		incoming = bal.hdir == HDirectionLeft
	}

	hasEdges := false
	// The Java code walks getIncomingEdges() or getOutgoingEdges(): the
	// edges of each port, in port order.
	for _, port := range pp.free.Ports {
		edges := port.OutgoingEdges
		if incoming {
			edges = port.IncomingEdges
		}
		for _, e := range edges {

			// ignore in-layer edges unless the block is solely connected by in-layer edges
			//  rationale: With self-loops and feedback edges it can happen that blocks contain only dummy nodes,
			//  thus are not connected to other blocks by non-inlayer edges. To avoid unnecessarily long edges
			//  in these cases, such blocks are allowed to be handled here as well (to shorten the edges)
			onlyDummies := bal.od[bal.root[pp.free.ID].ID]
			if !onlyDummies && e.IsInLayerEdge() {
				continue
			}

			// in order to straighten 'e' the block represented by 'pp.free'
			// would have to be moved. However, since that block is already
			// part of a straightened edge, it cannot be moved again
			// (the Java code tests the same flag two times)
			if bal.su[bal.root[pp.free.ID].ID] {
				continue
			}

			hasEdges = true

			// if the other node does not have a position yet, ignore this edge
			if _, finished := s.blockFinished[bal.root[getOther(e, pp.free).ID]]; finished {
				pp.hasEdges = true
				pp.edge = e
				return pp
			}
		}
	}

	// no edge picked
	pp.hasEdges = hasEdges
	pp.edge = nil
	return pp
}

// getBound is getBound(blockNode, isRoot): the position that makes an edge
// of the node straight, or a value without effect.
func (s *simpleThresholdStrategy) getBound(blockNode *lgraph.LNode, isRoot bool) float64 {
	bal := s.bal

	invalid := math.Inf(-1)
	if bal.vdir == VDirectionUp {
		invalid = math.Inf(1)
	}

	pick := s.pickEdge(&postprocessable{free: blockNode, isRoot: isRoot})

	// if edges exist but we couldn't find a good one
	if pick.edge == nil && pick.hasEdges {
		s.postProcessablesQueue = append(s.postProcessablesQueue, pick)
		return invalid
	} else if pick.edge != nil {

		var threshold float64
		left := pick.edge.Source
		right := pick.edge.Target

		if isRoot {
			// We handle the root (first) node of a block here
			rootPort, otherPort := left, right
			if bal.hdir == HDirectionRight {
				rootPort, otherPort = right, left
			}

			otherRoot := bal.root[otherPort.Node().ID]
			threshold = bal.yAt(otherRoot.ID) +
				bal.innerShift[otherPort.Node().ID] +
				otherPort.Pos.Y +
				otherPort.Anchor.Y -
				// root node
				bal.innerShift[rootPort.Node().ID] -
				rootPort.Pos.Y -
				rootPort.Anchor.Y
		} else {

			// ... and the last node of a block here
			rootPort, otherPort := left, right
			if bal.hdir == HDirectionLeft {
				rootPort, otherPort = right, left
			}

			threshold = bal.yAt(bal.root[otherPort.Node().ID].ID) +
				bal.innerShift[otherPort.Node().ID] +
				otherPort.Pos.Y +
				otherPort.Anchor.Y -
				// root node
				bal.innerShift[rootPort.Node().ID] -
				rootPort.Pos.Y -
				rootPort.Anchor.Y
		}

		// we are not allowed to move this block anymore
		// in order to straighten another edge
		bal.su[bal.root[left.Node().ID].ID] = true
		bal.su[bal.root[right.Node().ID].ID] = true

		return threshold
	}
	return invalid
}

// postProcess is postProcess().
func (s *simpleThresholdStrategy) postProcess() {
	bal := s.bal

	// try original iteration order
	for s.queueHead < len(s.postProcessablesQueue) {

		// first is the node, second whether it is regarded as root
		pp := s.postProcessablesQueue[s.queueHead]
		s.queueHead++
		pick := s.pickEdge(pp)

		if pick.edge == nil {
			continue
		}

		edge := pick.edge

		// ignore in-layer edges
		onlyDummies := bal.od[bal.root[pp.free.ID].ID]
		if !onlyDummies && edge.IsInLayerEdge() {
			continue
		}

		// try to straighten the edge ...
		moved := s.process(pp)
		// if it wasn't possible try again later in the opposite iteration direction
		if !moved {
			s.postProcessablesStack = append(s.postProcessablesStack, pp)
		}

	}

	// reversed iteration order
	for len(s.postProcessablesStack) != 0 {
		last := len(s.postProcessablesStack) - 1
		pp := s.postProcessablesStack[last]
		s.postProcessablesStack = s.postProcessablesStack[:last]
		s.process(pp)
	}
}

// process is process(pp): it moves the block of the node so that the edge of
// pp becomes straight, as far as the neighbors allow. It tells whether the
// block moved.
func (s *simpleThresholdStrategy) process(pp *postprocessable) bool {
	bal := s.bal

	edge := pp.edge
	var fix *lgraph.LPort
	if edge.Source.Node() == pp.free {
		fix = edge.Target
	} else {
		fix = edge.Source
	}
	var block *lgraph.LPort
	if edge.Source.Node() == pp.free {
		block = edge.Source
	} else {
		block = edge.Target
	}

	// t has to be the root node of a different block
	delta := bal.calculateDelta(fix, block)

	if delta > 0 && delta < thresholdLimit {
		// target y larger than source y --> shift upwards?
		availableSpace := bal.checkSpaceAbove(block.Node(), delta, s.ni)
		bal.shiftBlock(block.Node(), -availableSpace)
		return availableSpace > 0
	} else if delta < 0 && -delta < thresholdLimit {

		// direction is up, we possibly shifted some blocks too far upward
		// for an edge to be straight, so check if we can shift down again
		availableSpace := bal.checkSpaceBelow(block.Node(), -delta, s.ni)
		bal.shiftBlock(block.Node(), availableSpace)
		return availableSpace > 0
	}

	return false
}

// postprocessable is ThresholdStrategy.Postprocessable: a unit of the post
// processing.
type postprocessable struct {
	// free is the node whose block can move.
	free *lgraph.LNode
	// isRoot tells that free is the root node of its block.
	isRoot bool
	// hasEdges tells that free has edges.
	hasEdges bool
	// edge is the edge that was selected to be straightened.
	edge *lgraph.LEdge
}
