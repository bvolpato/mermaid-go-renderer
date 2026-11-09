// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/LabelSideSelector.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package labels

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("LabelSideSelector", func() lgraph.Processor { return NewLabelSideSelector() })
}

// LabelSideSelector is the processor LabelSideSelector: it decides for each
// edge label whether it goes above or below its edge, and moves the ports of
// the label dummy nodes to reserve the space on that side.
type LabelSideSelector struct{}

// NewLabelSideSelector is new LabelSideSelector().
func NewLabelSideSelector() *LabelSideSelector { return &LabelSideSelector{} }

// labelSideOpposite is LabelSide.opposite().
func labelSideOpposite(side core.LabelSide) core.LabelSide {
	switch side {
	case core.LabelSideAbove:
		return core.LabelSideBelow
	case core.LabelSideBelow:
		return core.LabelSideAbove
	case core.LabelSideInline:
		return core.LabelSideInline
	default:
		return core.LabelSideUnknown
	}
}

// Process is process(layeredGraph, monitor).
func (p *LabelSideSelector) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	mode := prop.Get(layeredGraph, lopts.EdgeLabelsSideSelection)

	// Calculate all label sides depending on the given strategy
	switch mode {
	case lenum.EdgeLabelSideSelectionAlwaysUp:
		p.sameSide(layeredGraph, core.LabelSideAbove)
	case lenum.EdgeLabelSideSelectionAlwaysDown:
		p.sameSide(layeredGraph, core.LabelSideBelow)
	case lenum.EdgeLabelSideSelectionDirectionUp:
		p.basedOnDirection(layeredGraph, core.LabelSideAbove)
	case lenum.EdgeLabelSideSelectionDirectionDown:
		p.basedOnDirection(layeredGraph, core.LabelSideBelow)
	case lenum.EdgeLabelSideSelectionSmartUp:
		p.smart(layeredGraph, core.LabelSideAbove)
	case lenum.EdgeLabelSideSelectionSmartDown:
		p.smart(layeredGraph, core.LabelSideBelow)
	}
}

// Simple Placement Strategies

// sameSide puts all labels on the given side.
func (p *LabelSideSelector) sameSide(graph *lgraph.LGraph, labelSide core.LabelSide) {
	for _, layer := range graph.Layers {
		for _, node := range layer.Nodes {
			if node.Type == lgraph.NodeTypeLabel {
				p.applyLabelSideToDummy(node, labelSide)
			}

			for _, port := range node.Ports {
				for _, edge := range port.OutgoingEdges {
					p.applyLabelSideToEdge(edge, labelSide)
				}
			}
		}
	}
}

// basedOnDirection puts the labels of an edge that points right on the given
// side, and the labels of the other edges on the opposite side.
func (p *LabelSideSelector) basedOnDirection(graph *lgraph.LGraph, sideForRightwardEdges core.LabelSide) {
	for _, layer := range graph.Layers {
		for _, node := range layer.Nodes {
			if node.Type == lgraph.NodeTypeLabel {
				side := labelSideOpposite(sideForRightwardEdges)
				if p.doesDummyEdgePointRight(node) {
					side = sideForRightwardEdges
				}
				p.applyLabelSideToDummy(node, side)
			}

			for _, port := range node.Ports {
				for _, edge := range port.OutgoingEdges {
					side := labelSideOpposite(sideForRightwardEdges)
					if p.doesEdgePointRight(edge) {
						side = sideForRightwardEdges
					}
					p.applyLabelSideToEdge(edge, side)
				}
			}
		}
	}
}

// Smart Placement Strategy

// smart chooses the label sides from patterns. In doubt it uses the default
// side.
func (p *LabelSideSelector) smart(graph *lgraph.LGraph, defaultSide core.LabelSide) {
	// We will collect consecutive runs of certain dummy nodes while we
	// iterate through layers
	var dummyNodeQueue []*lgraph.LNode

	for _, layer := range graph.Layers {
		// The first call to any method
		topGroup := true
		labelDummiesInQueue := 0

		for _, node := range layer.Nodes {
			switch node.Type {
			case lgraph.NodeTypeLabel:
				labelDummiesInQueue++
				// Intended fall-through to add the label dummy to the queue
				fallthrough
			case lgraph.NodeTypeLongEdge:
				dummyNodeQueue = append(dummyNodeQueue, node)

			case lgraph.NodeTypeNormal:
				p.smartForRegularNode(node, defaultSide)

				// Intended fall-through to handle the most recent dummy node
				// run, if any
				fallthrough
			default:
				// Empty dummy node queue
				if len(dummyNodeQueue) != 0 {
					p.smartForConsecutiveDummyNodeRun(dummyNodeQueue, labelDummiesInQueue, topGroup, false, defaultSide)
					dummyNodeQueue = dummyNodeQueue[:0]
				}

				// Reset things
				topGroup = false
				labelDummiesInQueue = 0
			}
		}

		// Do stuff with the nodes in the queue
		if len(dummyNodeQueue) != 0 {
			p.smartForConsecutiveDummyNodeRun(dummyNodeQueue, labelDummiesInQueue, topGroup, true, defaultSide)
			dummyNodeQueue = dummyNodeQueue[:0]
		}
	}
}

// smartForConsecutiveDummyNodeRun gives a label side to the label dummies of
// a run of dummy nodes that is not empty. topGroup and bottomGroup tell that
// the run has the first or the last nodes of its layer. The caller empties
// the queue.
func (p *LabelSideSelector) smartForConsecutiveDummyNodeRun(dummyNodes []*lgraph.LNode, labelDummyCount int, topGroup, bottomGroup bool, defaultSide core.LabelSide) {
	first := dummyNodes[0]
	last := dummyNodes[len(dummyNodes)-1]

	// We distinguish a number of special cases whose rules seem rather
	// complicated.
	if topGroup &&
		(!bottomGroup || len(dummyNodes) > 1) &&
		labelDummyCount == 1 &&
		first.Type == lgraph.NodeTypeLabel {

		// The current run of dummy nodes is at the top of the layer, has only
		// a single label dummy, and that label dummy is at the top of the
		// run; select the ABOVE side to ensure that its edge doesn't get too
		// long
		p.applyLabelSideToDummy(first, core.LabelSideAbove)

	} else if bottomGroup &&
		(!topGroup || len(dummyNodes) > 1) &&
		labelDummyCount == 1 &&
		last.Type == lgraph.NodeTypeLabel {

		// Symmetric to the previous case, only at the bottom of the layer
		p.applyLabelSideToDummy(last, core.LabelSideBelow)

	} else if len(dummyNodes) == 2 {
		// There's only a run of two edges, so place the label of the first
		// above its edge, and the label of the second below its edge
		p.applyLabelSideToDummy(dummyNodes[0], core.LabelSideAbove)
		p.applyLabelSideToDummy(dummyNodes[1], core.LabelSideBelow)

	} else {
		// Not one of the special cases above. Iterate over the dummy nodes
		// and assign the default label side, except if we find two
		// consecutive label dummies that connect the same nodes (tight loops
		// that can happen in control flow diagrams)
		p.applyForDummyNodeRunWithSimpleLoops(dummyNodes, defaultSide)
	}
}

// applyForDummyNodeRunWithSimpleLoops gives the default side to the label
// dummies of the run. A run of exactly two consecutive dummies that connect
// the same two nodes is a special case.
func (p *LabelSideSelector) applyForDummyNodeRunWithSimpleLoops(dummyNodes []*lgraph.LNode, defaultSide core.LabelSide) {
	// We keep track of runs of consecutive label dummy nodes that connect the
	// same two nodes
	labelDummyRun := make([]*lgraph.LNode, 0, len(dummyNodes))
	var prevLongEdgeSource, prevLongEdgeTarget *lgraph.LNode

	for _, currentDummy := range dummyNodes {
		// Check if we are continuing a previous run
		currLongEdgeSource := p.getLongEdgeEndNode(currentDummy, true)
		currLongEdgeTarget := p.getLongEdgeEndNode(currentDummy, false)

		if prevLongEdgeSource != currLongEdgeSource || prevLongEdgeTarget != currLongEdgeTarget {
			// We're starting a new run
			p.applyLabelSidesToLabelDummyRun(labelDummyRun, defaultSide)
			labelDummyRun = labelDummyRun[:0]

			prevLongEdgeSource = currLongEdgeSource
			prevLongEdgeTarget = currLongEdgeTarget
		}

		labelDummyRun = append(labelDummyRun, currentDummy)
	}

	// Assign label sides to whatever dummy nodes are left
	p.applyLabelSidesToLabelDummyRun(labelDummyRun, defaultSide)
}

// getLongEdgeEndNode returns the node of the long edge source or of the long
// edge target of a dummy node, or nil.
func (*LabelSideSelector) getLongEdgeEndNode(labelDummy *lgraph.LNode, source bool) *lgraph.LNode {
	endProperty := iprops.LongEdgeTarget
	if source {
		endProperty = iprops.LongEdgeSource
	}
	endPort := prop.Get(labelDummy, endProperty)
	if endPort == nil {
		return nil
	}
	return endPort.Node()
}

// applyLabelSidesToLabelDummyRun gives label sides to a list of consecutive
// dummy nodes. The caller empties the list.
func (p *LabelSideSelector) applyLabelSidesToLabelDummyRun(labelDummyRun []*lgraph.LNode, defaultSide core.LabelSide) {
	if len(labelDummyRun) == 0 {
		return
	}
	// If the list contains exactly two label dummies, we place labels
	// differently
	if len(labelDummyRun) == 2 {
		p.applyLabelSideToDummy(labelDummyRun[0], core.LabelSideAbove)
		p.applyLabelSideToDummy(labelDummyRun[1], core.LabelSideBelow)
	} else {
		for _, dummyNode := range labelDummyRun {
			p.applyLabelSideToDummy(dummyNode, defaultSide)
		}
	}
}

// smartForRegularNode gives label sides to the end labels at the ports of a
// node. The sides depend on the number of ports with end labels on each side
// of the node. The port list must be sorted.
func (p *LabelSideSelector) smartForRegularNode(node *lgraph.LNode, defaultSide core.LabelSide) {
	// Iterate over the node's list of ports on each side. Remember the ones
	// that have edges connected to them and make the label side decision
	// based on how many such ports there are
	endLabelQueue := make([][]*lgraph.LLabel, 0, len(node.Ports))
	// currentPortSide is null in Java before the first port.
	currentPortSide := core.PortSideUndefined
	hasCurrentPortSide := false

	// This is where we assume that the list of ports is properly sorted
	for _, port := range node.Ports {
		if !hasCurrentPortSide || port.Side != currentPortSide {
			if len(endLabelQueue) != 0 {
				p.smartForRegularNodePortEndLabels(endLabelQueue, currentPortSide, defaultSide)
			}

			endLabelQueue = endLabelQueue[:0]
			currentPortSide = port.Side
			hasCurrentPortSide = true
		}

		// Possibly add the port's end labels to our queue, if it has any
		portEndLabels, hasEdges := GatherLabels(port)
		if hasEdges {
			endLabelQueue = append(endLabelQueue, portEndLabels)
		}
	}

	// Clear remaining ports
	if len(endLabelQueue) != 0 {
		p.smartForRegularNodePortEndLabels(endLabelQueue, currentPortSide, defaultSide)
	}
}

// smartForRegularNodePortEndLabels handles the end labels of the ports of one
// side of a node.
func (p *LabelSideSelector) smartForRegularNodePortEndLabels(endLabelQueue [][]*lgraph.LLabel, portSide core.PortSide, defaultSide core.LabelSide) {
	if len(endLabelQueue) == 2 {
		// What we're going to do depends on which port side we are
		// traversing...
		if portSide == core.PortSideNorth || portSide == core.PortSideEast {
			p.applyLabelSideToLabels(endLabelQueue[0], core.LabelSideAbove)
			p.applyLabelSideToLabels(endLabelQueue[1], core.LabelSideBelow)
		} else {
			p.applyLabelSideToLabels(endLabelQueue[0], core.LabelSideBelow)
			p.applyLabelSideToLabels(endLabelQueue[1], core.LabelSideAbove)
		}
	} else {
		for _, labelList := range endLabelQueue {
			p.applyLabelSideToLabels(labelList, defaultSide)
		}
	}
}

// Helper Methods

// applyLabelSideToDummy is applyLabelSide(LNode, LabelSide): it sets the
// label side of a label dummy node and moves its ports, to reserve the space
// for the label on the correct side.
func (*LabelSideSelector) applyLabelSideToDummy(labelDummy *lgraph.LNode, side core.LabelSide) {
	// This method only does things to label dummy nodes
	if labelDummy.Type != lgraph.NodeTypeLabel {
		return
	}
	if !prop.Has(labelDummy, iprops.RepresentedLabels) {
		lgraph.Bail("LabelSideSelector: a label dummy without represented labels")
	}
	effectiveSide := side
	if labelDummy.IsInlineEdgeLabel() {
		effectiveSide = core.LabelSideInline
	}

	prop.Set(labelDummy, iprops.LabelSide, effectiveSide)

	// If the label is not below the edge, the ports need to be moved
	if effectiveSide != core.LabelSideBelow {
		originEdge := originEdgeOf(labelDummy)
		thickness := prop.Get(originEdge, lopts.EdgeThickness)

		// The new port position depends on the new placement
		portPos := 0.0
		if effectiveSide == core.LabelSideAbove {
			portPos = labelDummy.Size.Y - math.Ceil(thickness/2)
		} else if effectiveSide == core.LabelSideInline {
			// The label dummy has a superfluous label-edge spacing
			graph := graphOf(labelDummy)
			portPos = math.Ceil(labelDummy.Size.Y-
				prop.Get(graph, lopts.SpacingEdgeLabel)-
				thickness) /
				2.0
			labelDummy.Size.Y -= prop.Get(graph, lopts.SpacingEdgeLabel)
			labelDummy.Size.Y -= thickness
		}

		for _, port := range labelDummy.Ports {
			port.Pos.Y = portPos
		}
	}
}

// originEdgeOf is (LEdge) labelDummy.getProperty(InternalProperties.ORIGIN).
// Java fails for a label dummy whose origin is not an edge.
func originEdgeOf(labelDummy *lgraph.LNode) *lgraph.LEdge {
	originEdge, ok := prop.Get(labelDummy, iprops.Origin).(*lgraph.LEdge)
	if !ok || originEdge == nil {
		lgraph.Bail("label dummies: the origin of a label dummy is not an edge")
	}
	return originEdge
}

// graphOf is node.getGraph(). The callers read a property of the graph, and
// Java fails for a node without a graph.
func graphOf(node *lgraph.LNode) *lgraph.LGraph {
	graph := node.Graph()
	if graph == nil {
		lgraph.Bail("label dummies: a node without a graph")
	}
	return graph
}

// applyLabelSideToEdge is applyLabelSide(LEdge, LabelSide): it sets the label
// side of each label of the edge.
func (*LabelSideSelector) applyLabelSideToEdge(edge *lgraph.LEdge, side core.LabelSide) {
	for _, label := range edge.Labels {
		prop.Set(label, iprops.LabelSide, side)
	}
}

// applyLabelSideToLabels is applyLabelSide(List<LLabel>, LabelSide): it sets
// the label side of each label of the list.
func (*LabelSideSelector) applyLabelSideToLabels(labels []*lgraph.LLabel, side core.LabelSide) {
	for _, label := range labels {
		prop.Set(label, iprops.LabelSide, side)
	}
}

// doesEdgePointRight is doesEdgePointRight(LEdge): the edge points right in
// the final drawing. That is so for an edge that is not reversed.
func (*LabelSideSelector) doesEdgePointRight(edge *lgraph.LEdge) bool {
	return !prop.Get(edge, iprops.Reversed)
}

// doesDummyEdgePointRight is doesEdgePointRight(LNode): the label dummy is
// part of an edge that points right in the final drawing. That is so when one
// of its two edges points right.
func (p *LabelSideSelector) doesDummyEdgePointRight(labelDummy *lgraph.LNode) bool {
	// Find incoming and outgoing edge
	incoming := firstIncomingEdge(labelDummy)
	outgoing := firstOutgoingEdge(labelDummy)

	return p.doesEdgePointRight(incoming) || p.doesEdgePointRight(outgoing)
}
