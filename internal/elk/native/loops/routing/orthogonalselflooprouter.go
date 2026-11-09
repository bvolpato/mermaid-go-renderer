// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/loops/routing/OrthogonalSelfLoopRouter.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package routing

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
)

// edgeRoutingDirection is the enum
// OrthogonalSelfLoopRouter.EdgeRoutingDirection: the direction in which an
// edge goes around a node to its target.
type edgeRoutingDirection int

const (
	// clockwise is CLOCKWISE.
	clockwise edgeRoutingDirection = iota
	// counterClockwise is COUNTER_CLOCKWISE.
	counterClockwise
)

// OrthogonalSelfLoopRouter is the class OrthogonalSelfLoopRouter: it routes
// self loops orthogonally.
type OrthogonalSelfLoopRouter struct{}

// NewOrthogonalSelfLoopRouter is new OrthogonalSelfLoopRouter().
func NewOrthogonalSelfLoopRouter() *OrthogonalSelfLoopRouter { return &OrthogonalSelfLoopRouter{} }

// RouteSelfLoops is routeSelfLoops(slHolder).
func (r *OrthogonalSelfLoopRouter) RouteSelfLoops(slHolder *loops.SelfLoopHolder) {
	lNode := slHolder.LNode

	nodeSize := &lNode.Size
	nodeMargins := &lNode.Margin

	edgeEdgeDistance := lgraph.IndividualOrInherited(lNode, lopts.SpacingEdgeEdge)
	edgeLabelDistance := lgraph.IndividualOrInherited(lNode, lopts.SpacingEdgeLabel)
	nodeSLDistance := lgraph.IndividualOrInherited(lNode, lopts.SpacingNodeSelfLoop)

	newNodeMargins := &emath.ElkMargin{}
	newNodeMargins.Copy(&nodeMargins.Spacing)

	// Compute the distance of each routing slot on each side from the node.
	// The labels are part of that.
	routingSlotPositions := r.computeRoutingSlotPositions(slHolder, edgeEdgeDistance, edgeLabelDistance, nodeSLDistance)

	for _, slLoop := range slHolder.SLHyperLoops {
		for _, slEdge := range slLoop.SLEdges {
			lEdge := slEdge.LEdge

			routingDirection := r.computeEdgeRoutingDirection(slEdge)

			// Compute orthogonal bend points. The method modifyBendPoints of
			// this class does not change them.
			bendPoints := r.computeOrthogonalBendPoints(slEdge, routingDirection, routingSlotPositions)

			lEdge.BendPoints.Clear()
			lEdge.BendPoints.AddAll(bendPoints.Points...)

			for _, bp := range bendPoints.Points {
				r.updateNewNodeMargins(nodeSize, newNodeMargins, bp)
			}
		}

		// Place the labels of the self loop. The routes of the edges leave
		// enough space for them.
		slLabels := slLoop.SLLabels
		if slLabels != nil {
			r.placeLabels(slLoop, slLabels, routingSlotPositions, edgeLabelDistance)
			r.updateNewNodeMarginsForLabels(nodeSize, newNodeMargins, slLabels)
		}
	}

	// The margins of the node now include the space of the self loops.
	nodeMargins.Copy(&newNodeMargins.Spacing)
}

// computeEdgeRoutingDirection is computeEdgeRoutingDirection(slEdge): how
// the edge reaches its target.
func (r *OrthogonalSelfLoopRouter) computeEdgeRoutingDirection(slEdge *loops.SelfLoopEdge) edgeRoutingDirection {
	sourceLPort := slEdge.SLSource.LPort
	sourcePortSide := sourceLPort.Side

	targetLPort := slEdge.SLTarget.LPort
	targetPortSide := targetLPort.Side

	if sourcePortSide == targetPortSide {
		// With both ports on the same side the result has no effect. The
		// port ids decide.
		if sourceLPort.ID < targetLPort.ID {
			return clockwise
		}
		return counterClockwise
	} else if sourcePortSide.Right() == targetPortSide {
		return clockwise
	} else if sourcePortSide.Left() == targetPortSide {
		return counterClockwise
	}

	// The ports are on opposite sides. The port sides that the hyper loop
	// occupies decide, and clockwise routing comes first.
	slLoop := slEdge.SLHyperLoop
	if slLoop.OccupiedPortSides.Contains(sourcePortSide.Right()) {
		return clockwise
	}
	return counterClockwise
}

// placeLabels is placeLabels(slLoop, slLabels, routingSlotPositions,
// edgeLabelDistance): it places the labels of the self loop.
func (r *OrthogonalSelfLoopRouter) placeLabels(slLoop *loops.SelfHyperLoop, slLabels *loops.SelfHyperLoopLabels, routingSlotPositions *[loops.PortSideCount][]float64, edgeLabelDistance float64) {
	if !slLabels.Placed {
		// The side is null, and Java fails.
		lgraph.Bail("OrthogonalSelfLoopRouter: the labels of a self loop have no side")
	}

	// Find the baseline of the routing slot. The spacing between the label
	// and the edge is added to it below.
	labelSide := slLabels.Side
	labelPosition := slotPosition(routingSlotPositions, labelSide, slLoop.GetRoutingSlot(labelSide))

	switch labelSide {
	case core.PortSideNorth:
		labelPosition -= edgeLabelDistance + slLabels.Size.Y
		slLabels.Position.Y = labelPosition

	case core.PortSideSouth:
		labelPosition += edgeLabelDistance
		slLabels.Position.Y = labelPosition

	case core.PortSideWest:
		labelPosition -= edgeLabelDistance + slLabels.Size.X
		slLabels.Position.X = labelPosition

	case core.PortSideEast:
		labelPosition += edgeLabelDistance
		slLabels.Position.X = labelPosition
	}
}

// updateNewNodeMargins is updateNewNodeMargins(nodeSize, newNodeMargins,
// bendPoint): it makes the margins large enough for the bend point.
func (r *OrthogonalSelfLoopRouter) updateNewNodeMargins(nodeSize *emath.KVector, newNodeMargins *emath.ElkMargin, bendPoint *emath.KVector) {
	newNodeMargins.Left = max(newNodeMargins.Left, -bendPoint.X)
	newNodeMargins.Right = max(newNodeMargins.Right, bendPoint.X-nodeSize.X)

	newNodeMargins.Top = max(newNodeMargins.Top, -bendPoint.Y)
	newNodeMargins.Bottom = max(newNodeMargins.Bottom, bendPoint.Y-nodeSize.Y)
}

// updateNewNodeMarginsForLabels is updateNewNodeMargins(nodeSize,
// newNodeMargins, slLabels): it makes the margins large enough for the
// labels of a self loop.
func (r *OrthogonalSelfLoopRouter) updateNewNodeMarginsForLabels(nodeSize *emath.KVector, newNodeMargins *emath.ElkMargin, slLabels *loops.SelfHyperLoopLabels) {
	pos := slLabels.Position.Clone()
	r.updateNewNodeMargins(nodeSize, newNodeMargins, pos)

	pos.Add(&slLabels.Size)
	r.updateNewNodeMargins(nodeSize, newNodeMargins, pos)
}

////////////////////////////////////////////////////////////////////////////
// Routing Slot Positions

// computeRoutingSlotPositions is computeRoutingSlotPositions(slHolder,
// edgeEdgeDistance, edgeLabelDistance, nodeSLDistance): the position of each
// routing slot on each side, by the ordinal of the side. Slots on the north
// side and on the south side that are next to each other have enough space
// between them for labels.
func (r *OrthogonalSelfLoopRouter) computeRoutingSlotPositions(slHolder *loops.SelfLoopHolder, edgeEdgeDistance, edgeLabelDistance, nodeSLDistance float64) *[loops.PortSideCount][]float64 {
	// Make an array for each side.
	positions := &[loops.PortSideCount][]float64{}
	for _, portSide := range core.ValuesPortSide() {
		positions[portSide] = make([]float64, slHolder.RoutingSlotCount[portSide])
	}

	// The space between two routing slots depends on the size of the
	// labels. Find it first, for the north side and the south side.
	r.initializeWithMaxLabelHeight(positions, slHolder, core.PortSideNorth)
	r.initializeWithMaxLabelHeight(positions, slHolder, core.PortSideSouth)

	// Compute the positions for each side.
	r.computePositions(positions, slHolder, core.PortSideNorth, edgeEdgeDistance, edgeLabelDistance, nodeSLDistance)
	r.computePositions(positions, slHolder, core.PortSideEast, edgeEdgeDistance, edgeLabelDistance, nodeSLDistance)
	r.computePositions(positions, slHolder, core.PortSideSouth, edgeEdgeDistance, edgeLabelDistance, nodeSLDistance)
	r.computePositions(positions, slHolder, core.PortSideWest, edgeEdgeDistance, edgeLabelDistance, nodeSLDistance)

	return positions
}

// initializeWithMaxLabelHeight is initializeWithMaxLabelHeight(positions,
// slHolder, portSide): it sets the entry of each routing slot on the north
// side or the south side to the largest height of the labels in the slot.
func (r *OrthogonalSelfLoopRouter) initializeWithMaxLabelHeight(positions *[loops.PortSideCount][]float64, slHolder *loops.SelfLoopHolder, portSide core.PortSide) {
	sidePositions := positions[portSide]

	for _, slLoop := range slHolder.SLHyperLoops {
		slLabels := slLoop.SLLabels
		if slLabels != nil && slLabels.Side == portSide {
			routingSlot := slLoop.GetRoutingSlot(portSide)
			if routingSlot < 0 || routingSlot >= len(sidePositions) {
				// Java fails. elkjs makes the array longer and goes on with
				// a position that is not a number.
				lgraph.Bail("OrthogonalSelfLoopRouter: the labels of a self loop are on a side without a routing slot for the loop")
			}
			sidePositions[routingSlot] = max(sidePositions[routingSlot], slLabels.Size.Y)
		}
	}
}

// computePositions is computePositions(positions, slHolder, portSide,
// edgeEdgeDistance, edgeLabelDistance, nodeSelfLoopDistance): the position
// of each routing slot of one side, with space for the labels.
func (r *OrthogonalSelfLoopRouter) computePositions(positions *[loops.PortSideCount][]float64, slHolder *loops.SelfLoopHolder, portSide core.PortSide, edgeEdgeDistance, edgeLabelDistance, nodeSelfLoopDistance float64) {
	currPos := r.computeBaselinePosition(slHolder, portSide, nodeSelfLoopDistance)

	// On the north side and on the west side the positions decrease.
	factor := 1.0
	if portSide == core.PortSideNorth || portSide == core.PortSideWest {
		factor = -1
	}

	sidePositions := positions[portSide]
	for slot := 0; slot < len(sidePositions); slot++ {
		// The entry of the slot has the size of the largest label in the
		// slot.
		largestLabelSize := sidePositions[slot]
		if largestLabelSize > 0 {
			// Add the spacing between the edge and the label.
			largestLabelSize += edgeLabelDistance
		}

		// Put the slot at the current position, and move the position on.
		sidePositions[slot] = currPos
		currPos += float64(factor * (largestLabelSize + edgeEdgeDistance))
	}
}

// computeBaselinePosition is computeBaselinePosition(slHolder, portSide,
// nodeSelfLoopDistance): the offset from the origin of the node that leaves
// the area of the ports, which the margins of the node describe.
func (r *OrthogonalSelfLoopRouter) computeBaselinePosition(slHolder *loops.SelfLoopHolder, portSide core.PortSide, nodeSelfLoopDistance float64) float64 {
	lNode := slHolder.LNode
	lMargins := &lNode.Margin

	switch portSide {
	case core.PortSideNorth:
		return -lMargins.Top - nodeSelfLoopDistance
	case core.PortSideEast:
		return lNode.Size.X + lMargins.Right + nodeSelfLoopDistance
	case core.PortSideSouth:
		return lNode.Size.Y + lMargins.Bottom + nodeSelfLoopDistance
	case core.PortSideWest:
		return -lMargins.Left - nodeSelfLoopDistance
	}
	return -1
}

////////////////////////////////////////////////////////////////////////////
// High-Level Bend Point Computation

// computeOrthogonalBendPoints is computeOrthogonalBendPoints(slEdge,
// routingDirection, routingSlotPositions): the bend points that route the
// self loop edge orthogonally.
func (r *OrthogonalSelfLoopRouter) computeOrthogonalBendPoints(slEdge *loops.SelfLoopEdge, routingDirection edgeRoutingDirection, routingSlotPositions *[loops.PortSideCount][]float64) *emath.KVectorChain {
	bendPoints := emath.NewKVectorChain()

	r.addOuterBendPoint(slEdge, slEdge.SLSource, routingSlotPositions, bendPoints)
	r.addCornerBendPoints(slEdge, routingDirection, routingSlotPositions, bendPoints)
	r.addOuterBendPoint(slEdge, slEdge.SLTarget, routingSlotPositions, bendPoints)

	return bendPoints
}

////////////////////////////////////////////////////////////////////////////
// Actual Bend Point Computation

func (r *OrthogonalSelfLoopRouter) addOuterBendPoint(slEdge *loops.SelfLoopEdge, slPort *loops.SelfLoopPort, routingSlotPositions *[loops.PortSideCount][]float64, bendPoints *emath.KVectorChain) {
	slLoop := slEdge.SLHyperLoop
	lPort := slPort.LPort
	portSide := lPort.Side

	// Start with the coordinate of the routing slot.
	result := r.getBaseVector(portSide, slLoop.GetRoutingSlot(portSide), routingSlotPositions)

	// Add the coordinate of the port anchor.
	anchor := lPort.Pos.Clone().Add(&lPort.Anchor)
	switch lPort.Side {
	case core.PortSideNorth, core.PortSideSouth:
		result.X += anchor.X

	case core.PortSideEast, core.PortSideWest:
		result.Y += anchor.Y
	}

	bendPoints.Add(result)
}

func (r *OrthogonalSelfLoopRouter) addCornerBendPoints(slEdge *loops.SelfLoopEdge, routingDirection edgeRoutingDirection, routingSlotPositions *[loops.PortSideCount][]float64, bendPoints *emath.KVectorChain) {
	// Ports on one side need no corner bend points.
	lSourcePort := slEdge.SLSource.LPort
	lTargetPort := slEdge.SLTarget.LPort

	if lSourcePort.Side == lTargetPort.Side {
		return
	}

	slLoop := slEdge.SLHyperLoop

	// Compute the corner points.
	currPortSide := lSourcePort.Side

	for currPortSide != lTargetPort.Side {
		// The next port side depends on the routing direction.
		var nextPortSide core.PortSide
		if routingDirection == clockwise {
			nextPortSide = currPortSide.Right()
		} else {
			nextPortSide = currPortSide.Left()
		}

		// Compute the coordinates that the current port side and the next
		// port side give.
		currPortSideComponent := r.getBaseVector(currPortSide, slLoop.GetRoutingSlot(currPortSide), routingSlotPositions)
		nextPortSideComponent := r.getBaseVector(nextPortSide, slLoop.GetRoutingSlot(nextPortSide), routingSlotPositions)

		// One vector has the x coordinate and the other one has the y
		// coordinate. Their sum is the bend point.
		bendPoints.Add(currPortSideComponent.Add(nextPortSideComponent))

		// Go to the next port side.
		currPortSide = nextPortSide
	}
}

// slotPosition is routingSlotPositions[portSide.ordinal()][routingSlot]. The
// layout stops for a slot that the side does not have: Java fails there, and
// elkjs goes on with a position that is not a number.
func slotPosition(routingSlotPositions *[loops.PortSideCount][]float64, portSide core.PortSide, routingSlot int) float64 {
	sidePositions := routingSlotPositions[portSide]
	if routingSlot < 0 || routingSlot >= len(sidePositions) {
		lgraph.Bail("OrthogonalSelfLoopRouter: a self loop has no routing slot on the side %v", portSide)
	}
	return sidePositions[routingSlot]
}

func (r *OrthogonalSelfLoopRouter) getBaseVector(portSide core.PortSide, routingSlot int, routingSlotPositions *[loops.PortSideCount][]float64) *emath.KVector {
	position := slotPosition(routingSlotPositions, portSide, routingSlot)

	switch portSide {
	case core.PortSideNorth, core.PortSideSouth:
		return emath.NewKVector(0, position)

	case core.PortSideEast, core.PortSideWest:
		return emath.NewKVector(position, 0)
	}
	// The side UNDEFINED has no routing slot, so slotPosition stops the
	// layout before this.
	lgraph.Bail("OrthogonalSelfLoopRouter: a self loop port has no side")
	return nil
}
