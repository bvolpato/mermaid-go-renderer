// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/loops/routing/RoutingDirector.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package routing

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops/ordering"
)

// compareByID is COMPARE_BY_ID: it compares two self loop ports by the ids
// of their ports.
func compareByID(slPort1, slPort2 *loops.SelfLoopPort) int {
	return gwt.IntCompare(slPort1.LPort.ID, slPort2.LPort.ID)
}

const (
	// unconnectedPortPenalty is the penalty of an edge that passes a port
	// without connections.
	unconnectedPortPenalty = 1
	// connectedPortPenalty is the penalty of an edge that passes a port with
	// connections.
	connectedPortPenalty = 3
)

// RoutingDirector is the class RoutingDirector: it decides for each
// SelfHyperLoop which of its ports is the leftmost one and which is the
// rightmost one. That decides how the loop goes around the node.
type RoutingDirector struct {
	// portPenalties has, for the id of each port, the penalty of an edge
	// that passes each port from the first port of the node through that
	// port. It is nil until a penalty is needed.
	portPenalties []int
}

// NewRoutingDirector is new RoutingDirector().
func NewRoutingDirector() *RoutingDirector { return &RoutingDirector{} }

// DetermineLoopRoutes is determineLoopRoutes(slHolder): it sets the leftmost
// port and the rightmost port of each hyper loop of the holder.
func (r *RoutingDirector) DetermineLoopRoutes(slHolder *loops.SelfLoopHolder) {
	// Give each port its index in the port list as id.
	r.assignPortIds(slHolder.LNode.Ports)
	r.sortHyperLoopPortLists(slHolder)

	// Set the leftmost port and the rightmost port of each loop.
	for _, slLoop := range slHolder.SLHyperLoops {
		switch slLoop.GetSelfLoopType() {
		case loops.SelfLoopTypeOneSide:
			r.determineOneSideLoopRoutes(slLoop)

		case loops.SelfLoopTypeTwoSidesCorner:
			r.determineTwoSideCornerLoopRoutes(slLoop)

		case loops.SelfLoopTypeTwoSidesOpposing:
			r.determineTwoSideOpposingLoopRoutes(slLoop)

		case loops.SelfLoopTypeThreeSides:
			r.determineThreeSideLoopRoutes(slLoop)

		case loops.SelfLoopTypeFourSides:
			r.determineFourSideLoopRoutes(slLoop)
		}

		// Fill the set of the port sides that the loop occupies.
		r.computeOccupiedPortSides(slLoop)
	}

	r.portPenalties = nil
}

func (r *RoutingDirector) assignPortIds(lPorts []*lgraph.LPort) {
	for i, lPort := range lPorts {
		lPort.ID = i
	}
}

func (r *RoutingDirector) sortHyperLoopPortLists(slHolder *loops.SelfLoopHolder) {
	for _, slLoop := range slHolder.SLHyperLoops {
		gwt.SortFunc(slLoop.SLPorts, compareByID)
	}
}

func (r *RoutingDirector) computeOccupiedPortSides(slLoop *loops.SelfHyperLoop) {
	currPortSide := slLoop.LeftmostPort.LPort.Side
	targetSide := slLoop.RightmostPort.LPort.Side

	for currPortSide != targetSide {
		if currPortSide == core.PortSideUndefined {
			// The side after UNDEFINED is UNDEFINED, so this loop would not
			// end. A port of a self loop has a side at this point.
			lgraph.Bail("RoutingDirector: a self loop port has no side")
		}
		slLoop.OccupiedPortSides.Add(currPortSide)
		currPortSide = currPortSide.Right()
	}

	slLoop.OccupiedPortSides.Add(currPortSide)
}

////////////////////////////////////////////////////////////////////////////
// Route Determination (One Side)

// determineOneSideLoopRoutes is determineOneSideLoopRoutes(slLoop): a loop
// with ports on one side stays on that side.
func (r *RoutingDirector) determineOneSideLoopRoutes(slLoop *loops.SelfHyperLoop) {
	side := slLoop.SLPorts[0].LPort.Side
	r.assignLeftmostRightmostPorts(slLoop, side, side)
}

////////////////////////////////////////////////////////////////////////////
// Route Determination (Two Sides, corner)

// determineTwoSideCornerLoopRoutes is
// determineTwoSideCornerLoopRoutes(slLoop): a loop over a corner spans that
// corner.
func (r *RoutingDirector) determineTwoSideCornerLoopRoutes(slLoop *loops.SelfHyperLoop) {
	sides := ordering.SortedTwoSideLoopPortSides(slLoop)
	r.assignLeftmostRightmostPorts(slLoop, sides[0], sides[1])
}

////////////////////////////////////////////////////////////////////////////
// Route Determination (One Side, opposing)

// determineTwoSideOpposingLoopRoutes is
// determineTwoSideOpposingLoopRoutes(slLoop): a loop between two opposing
// sides can go around the node in two ways. The way with the lower penalty
// wins.
func (r *RoutingDirector) determineTwoSideOpposingLoopRoutes(slLoop *loops.SelfHyperLoop) {
	sides := slLoop.PortSides().KeySet()
	if len(sides) != 2 {
		lgraph.Bail("RoutingDirector: a self loop between two opposing sides has ports on %d sides", len(sides))
	}

	slHolder := slLoop.SLHolder

	// Compute the penalty of both options, and use the option with the
	// lower penalty.
	option1LeftmostPort := r.lowestPortOnSide(slLoop, sides[0])
	option1RightmostPort := r.highestPortOnSide(slLoop, sides[1])
	option1Penalty := r.computeEdgePenalty(slHolder, option1LeftmostPort, option1RightmostPort)

	option2LeftmostPort := r.lowestPortOnSide(slLoop, sides[1])
	option2RightmostPort := r.highestPortOnSide(slLoop, sides[0])
	option2Penalty := r.computeEdgePenalty(slHolder, option2LeftmostPort, option2RightmostPort)

	if option1Penalty <= option2Penalty {
		slLoop.SetLeftmostPort(option1LeftmostPort)
		slLoop.SetRightmostPort(option1RightmostPort)
	} else {
		slLoop.SetLeftmostPort(option2LeftmostPort)
		slLoop.SetRightmostPort(option2RightmostPort)
	}
}

////////////////////////////////////////////////////////////////////////////
// Route Determination (Three Sides)

// determineThreeSideLoopRoutes is determineThreeSideLoopRoutes(slLoop): a
// loop with ports on three sides has one way to go.
func (r *RoutingDirector) determineThreeSideLoopRoutes(slLoop *loops.SelfHyperLoop) {
	var leftmostSide, rightmostSide core.PortSide

	// Find the leftmost and the rightmost of the three port sides. A sort
	// of the sides does not give them, because a loop can span the corner
	// between WEST and NORTH.
	switch r.computeMissingPortSide(slLoop) {
	case core.PortSideNorth:
		leftmostSide = core.PortSideEast
		rightmostSide = core.PortSideWest

	case core.PortSideEast:
		leftmostSide = core.PortSideSouth
		rightmostSide = core.PortSideNorth

	case core.PortSideSouth:
		leftmostSide = core.PortSideWest
		rightmostSide = core.PortSideEast

	case core.PortSideWest:
		leftmostSide = core.PortSideNorth
		rightmostSide = core.PortSideSouth
	}

	r.assignLeftmostRightmostPorts(slLoop, leftmostSide, rightmostSide)
}

func (r *RoutingDirector) computeMissingPortSide(slLoop *loops.SelfHyperLoop) core.PortSide {
	sides := slLoop.PortSides()

	for _, side := range core.ValuesPortSide() {
		if side != core.PortSideUndefined && !sides.ContainsKey(side) {
			return side
		}
	}

	// This cannot occur. Java returns null and fails.
	lgraph.Bail("RoutingDirector: a three-side self loop has ports on four sides")
	return core.PortSideUndefined
}

////////////////////////////////////////////////////////////////////////////
// Route Determination (Four Sides)

// determineFourSideLoopRoutes is determineFourSideLoopRoutes(slLoop): a loop
// with ports on four sides gets a break between two ports that are next to
// each other in the port list. One becomes the leftmost port, the other the
// rightmost port. The break goes where an edge between the two ports would
// have the highest penalty.
func (r *RoutingDirector) determineFourSideLoopRoutes(slLoop *loops.SelfHyperLoop) {
	// The self loop ports are sorted by id.
	sortedSLPorts := slLoop.SLPorts
	slHolder := slLoop.SLHolder

	// Look at each pair of ports that are next to each other, and find the
	// pair with the highest penalty for an edge between the two ports. The
	// loop is split at that pair. The first pair is the last port and the
	// first port of the list. Each later pair replaces it only with a higher
	// penalty.
	worstLeftPort := sortedSLPorts[len(sortedSLPorts)-1]
	worstRightPort := sortedSLPorts[0]
	worstPenalty := r.computeEdgePenalty(slHolder, worstLeftPort, worstRightPort)

	for rightPortIndex := 1; rightPortIndex < len(sortedSLPorts); rightPortIndex++ {
		currLeftPort := sortedSLPorts[rightPortIndex-1]
		currRightPort := sortedSLPorts[rightPortIndex]
		currPenalty := r.computeEdgePenalty(slHolder, currLeftPort, currRightPort)

		if currPenalty > worstPenalty {
			worstLeftPort = currLeftPort
			worstRightPort = currRightPort
			worstPenalty = currPenalty
		}
	}

	// The self loop must not run between the two ports of the pair, so the
	// right port becomes the leftmost port and the left port the rightmost.
	slLoop.SetLeftmostPort(worstRightPort)
	slLoop.SetRightmostPort(worstLeftPort)
}

////////////////////////////////////////////////////////////////////////////
// Utility Methods

// assignLeftmostRightmostPorts is assignLeftmostRightmostPorts(slLoop,
// leftmostSide, rightmostSide): the leftmost port is the port with the
// lowest id on the leftmost side, and the rightmost port is the port with
// the highest id on the rightmost side.
func (r *RoutingDirector) assignLeftmostRightmostPorts(slLoop *loops.SelfHyperLoop, leftmostSide, rightmostSide core.PortSide) {
	slLoop.SetLeftmostPort(r.lowestPortOnSide(slLoop, leftmostSide))
	slLoop.SetRightmostPort(r.highestPortOnSide(slLoop, rightmostSide))
}

// lowestPortOnSide is lowestPortOnSide(slLoop, side): the port with the
// lowest id on the given side. It is Stream.min of GWT: of two ports with
// the same id the first one wins.
func (r *RoutingDirector) lowestPortOnSide(slLoop *loops.SelfHyperLoop, side core.PortSide) *loops.SelfLoopPort {
	slPorts := slLoop.SLPortsOnSide(side)
	if len(slPorts) == 0 {
		// Optional.get() throws.
		lgraph.Bail("RoutingDirector: a self loop has no port on the side %v", side)
	}
	result := slPorts[0]
	for _, slPort := range slPorts[1:] {
		if !(compareByID(result, slPort) <= 0) {
			result = slPort
		}
	}
	return result
}

// highestPortOnSide is highestPortOnSide(slLoop, side): the port with the
// highest id on the given side. It is Stream.max of GWT: of two ports with
// the same id the last one wins.
func (r *RoutingDirector) highestPortOnSide(slLoop *loops.SelfHyperLoop, side core.PortSide) *loops.SelfLoopPort {
	slPorts := slLoop.SLPortsOnSide(side)
	if len(slPorts) == 0 {
		// Optional.get() throws.
		lgraph.Bail("RoutingDirector: a self loop has no port on the side %v", side)
	}
	result := slPorts[0]
	for _, slPort := range slPorts[1:] {
		if compareByID(result, slPort) <= 0 {
			result = slPort
		}
	}
	return result
}

// computeEdgePenalty is computeEdgePenalty(slHolder, leftmostPort,
// rightmostPort): the penalty of an edge that runs clockwise from the
// leftmost port to the rightmost port.
func (r *RoutingDirector) computeEdgePenalty(slHolder *loops.SelfLoopHolder, leftmostPort, rightmostPort *loops.SelfLoopPort) int {
	// Compute the penalties when they are needed first.
	if r.portPenalties == nil {
		r.computePenalties(slHolder)
	}

	portCount := len(slHolder.LNode.Ports)

	leftmostPortID := leftmostPort.LPort.ID
	rightmostPortID := rightmostPort.LPort.ID
	leftOfRightmostPortID := rightmostPortID - 1

	// With leftmostPortId < rightmostPortId, leftOfRightmostPortId is a
	// valid index. With leftmostPortId > rightmostPortId, rightmostPortId
	// can be 0. Then leftOfRightmostPortId is the index of the last port.
	if leftOfRightmostPortID < 0 {
		leftOfRightmostPortID = portCount - 1
	}

	// The ids are indices of the port list of the node when each port of the
	// self loops is in that list. Java fails for another id.
	if leftmostPortID < 0 || leftmostPortID >= portCount || leftOfRightmostPortID < 0 || leftOfRightmostPortID >= portCount {
		lgraph.Bail("RoutingDirector: a self loop port is not in the port list of its node")
	}

	if leftmostPortID <= leftOfRightmostPortID {
		// The difference of the two sums is the penalty.
		return r.portPenalties[leftOfRightmostPortID] - r.portPenalties[leftmostPortID]
	}
	// The edge from the leftmost port to the rightmost port passes the top
	// left corner, where the indices start at zero again. The penalty is the
	// sum of two parts: from the leftmost port to the corner, and from the
	// corner to the rightmost port.
	return r.portPenalties[portCount-1] - r.portPenalties[leftmostPortID] + r.portPenalties[leftOfRightmostPortID]
}

// computePenalties is computePenalties(slHolder): it fills portPenalties
// with the sums of the port penalties. A port with connections has a higher
// penalty than a port without.
func (r *RoutingDirector) computePenalties(slHolder *loops.SelfLoopHolder) {
	// Add the penalty of each port to the sum.
	ports := slHolder.LNode.Ports
	r.portPenalties = make([]int, len(ports))
	penaltySum := 0

	for i, currPort := range ports {
		if len(currPort.IncomingEdges) == 0 && len(currPort.OutgoingEdges) == 0 {
			penaltySum += unconnectedPortPenalty
		} else {
			penaltySum += connectedPortPenalty
		}

		r.portPenalties[i] = penaltySum
	}
}
