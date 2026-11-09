// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/loops/ordering/PortRestorer.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package ordering

import (
	"slices"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// portSideArea is the enum PortRestorer.PortSideArea: the three areas of a
// port side. The areas are in clockwise order: the start of the north side
// is at its left, the start of the east side is at its top.
type portSideArea int

const (
	// portSideAreaStart is the beginning of a side.
	portSideAreaStart portSideArea = iota
	// portSideAreaMiddle is the middle of a side.
	portSideAreaMiddle
	// portSideAreaEnd is the end of a side.
	portSideAreaEnd
	portSideAreaCount
)

// addMode is the enum PortRestorer.AddMode: whether to prepend or append
// items to a list.
type addMode int

const (
	// addModePrepend prepends to a target list.
	addModePrepend addMode = iota
	// addModeAppend appends to a target list.
	addModeAppend
)

// selfLoopTypeCount is SelfLoopType.values().length.
const selfLoopTypeCount = int(loops.SelfLoopTypeFourSides) + 1

// PortRestorer is the class PortRestorer: it puts the hidden ports back into
// the port list of their node, for port constraints that fix the sides.
type PortRestorer struct {
	// slLoopsByType has the hyper loops of each SelfLoopType, by the ordinal
	// of the type.
	slLoopsByType [selfLoopTypeCount][]*loops.SelfHyperLoop
	// targetAreas is the table of the areas around the node that get self
	// loop ports: one list for each side and each area.
	targetAreas [loops.PortSideCount][portSideAreaCount][]*loops.SelfLoopPort
}

// NewPortRestorer is new PortRestorer().
func NewPortRestorer() *PortRestorer { return &PortRestorer{} }

// RestorePorts is restorePorts(slHolder, monitor): it puts each hidden port
// back at a proper place, and resets the hidden state of the ports and of
// the holder.
func (r *PortRestorer) RestorePorts(slHolder *loops.SelfLoopHolder) {
	// Make the port lists empty. They are merged into the port list of the
	// node at the end.
	r.initTargetAreas()

	// Collect the loops by their type, which mainly is their number of port
	// sides.
	r.gatherSelfLoopsByType(slHolder)

	// Add the loops in an order that keeps the number of edge crossings
	// low. The order does not take into account the regular eastern and
	// western ports that are part of self loops too.
	r.processOneSideLoops(prop.Get(slHolder.LNode, lopts.EdgeRoutingSelfLoopOrdering))
	r.processTwoSideCornerLoops()
	r.processThreeSideLoops()
	r.processFourSideLoops()
	r.processTwoSideOpposingLoops()

	// Add the ports to the port list of the node.
	r.restorePorts(slHolder)

	// No port is hidden from here on.
	for side := range r.targetAreas {
		for area := range r.targetAreas[side] {
			for _, slPort := range r.targetAreas[side][area] {
				slPort.SetHidden(false)
			}
		}
	}
	slHolder.SetPortsHidden(false)

	// Reset the loop lists.
	r.slLoopsByType = [selfLoopTypeCount][]*loops.SelfHyperLoop{}
}

// initTargetAreas is initTargetAreas(): after it there is an empty port list
// for each side and each area.
func (r *PortRestorer) initTargetAreas() {
	r.targetAreas = [loops.PortSideCount][portSideAreaCount][]*loops.SelfLoopPort{}
}

////////////////////////////////////////////////////////////////////////////
// Self Loop Gathering

// gatherSelfLoopsByType is gatherSelfLoopsByType(slHolder): it sorts the
// self loops into slLoopsByType.
func (r *PortRestorer) gatherSelfLoopsByType(slHolder *loops.SelfLoopHolder) {
	r.slLoopsByType = [selfLoopTypeCount][]*loops.SelfHyperLoop{}
	for _, slLoop := range slHolder.SLHyperLoops {
		slType := slLoop.GetSelfLoopType()
		r.slLoopsByType[slType] = append(r.slLoopsByType[slType], slLoop)
	}
}

////////////////////////////////////////////////////////////////////////////
// Self Loop Placement (One Side)

func (r *PortRestorer) processOneSideLoops(ordering lenum.SelfLoopOrderingStrategy) {
	if ordering == lenum.SelfLoopOrderingStrategyReverseStacked {
		slices.Reverse(r.slLoopsByType[loops.SelfLoopTypeOneSide])
	}
	for _, slLoop := range r.slLoopsByType[loops.SelfLoopTypeOneSide] {
		// The port side of the loop.
		side := slLoop.SLPorts[0].LPort.Side

		// Sort the ports: a port with more outgoing edges goes to the left
		// of a port with more incoming edges.
		sortedPorts := append([]*loops.SelfLoopPort(nil), slLoop.SLPorts...)
		gwt.SortFunc(sortedPorts, func(slPort1, slPort2 *loops.SelfLoopPort) int {
			return gwt.IntCompare(slPort1.SLNetFlow(), slPort2.SLNetFlow())
		})

		switch ordering {
		case lenum.SelfLoopOrderingStrategySequenced:
			// Add the ports in the order of the sorted list.
			r.addToTargetArea(sortedPorts, side, portSideAreaMiddle, addModeAppend)

		case lenum.SelfLoopOrderingStrategyReverseStacked, lenum.SelfLoopOrderingStrategyStacked:
			// Split the ports into a first group and a second group.
			splitIndex := r.computePortListSplitIndex(sortedPorts)

			// Prepend the first group to the middle list, and append the
			// second group to the same list.
			r.addToTargetArea(sortedPorts[:splitIndex], side, portSideAreaMiddle, addModePrepend)
			r.addToTargetArea(sortedPorts[splitIndex:], side, portSideAreaMiddle, addModeAppend)
		}
	}
}

// computePortListSplitIndex is computePortListSplitIndex(sortedPorts): a
// good split index for a port list that is sorted by net flow. The port at
// the split index starts the second half. It is the first port with a
// positive net flow, or the port at the center of the list. It is never the
// first port.
func (r *PortRestorer) computePortListSplitIndex(sortedPorts []*loops.SelfLoopPort) int {
	// Find the index of the first port with a positive net flow.
	positiveNetFlowIndex := 0
	for ; positiveNetFlowIndex < len(sortedPorts); positiveNetFlowIndex++ {
		if sortedPorts[positiveNetFlowIndex].SLNetFlow() > 0 {
			break
		}
	}

	// Return the index, unless it is the first port or the last port.
	if positiveNetFlowIndex > 0 && positiveNetFlowIndex < len(sortedPorts)-1 {
		return positiveNetFlowIndex
	}

	// Find the index of the first port with a net flow that is not
	// negative. The Java code tests for a positive net flow here too.
	nonNegativeNetFlowIndex := 0
	for ; nonNegativeNetFlowIndex < len(sortedPorts); nonNegativeNetFlowIndex++ {
		if sortedPorts[nonNegativeNetFlowIndex].SLNetFlow() > 0 {
			break
		}
	}

	// Return the index, unless it is the first port or the last port.
	if nonNegativeNetFlowIndex > 0 && positiveNetFlowIndex < len(sortedPorts)-1 {
		return nonNegativeNetFlowIndex
	}

	// Return the index of the port at the center.
	return len(sortedPorts) / 2
}

////////////////////////////////////////////////////////////////////////////
// Self Loop Placement (Two Sides)

func (r *PortRestorer) processTwoSideCornerLoops() {
	for _, slLoop := range r.slLoopsByType[loops.SelfLoopTypeTwoSidesCorner] {
		// The port sides in clockwise order.
		sides := SortedTwoSideLoopPortSides(slLoop)

		// Add the ports to their target areas.
		r.addLoopToTargetArea(slLoop, sides[0], portSideAreaEnd, addModePrepend)
		r.addLoopToTargetArea(slLoop, sides[1], portSideAreaStart, addModeAppend)
	}
}

func (r *PortRestorer) processTwoSideOpposingLoops() {
	for _, slLoop := range r.slLoopsByType[loops.SelfLoopTypeTwoSidesOpposing] {
		// The port sides in clockwise order.
		sides := SortedTwoSideLoopPortSides(slLoop)

		// Prepend to the end area of the first side, and append to the
		// start area of the second side.
		r.addLoopToTargetArea(slLoop, sides[0], portSideAreaEnd, addModePrepend)
		r.addLoopToTargetArea(slLoop, sides[1], portSideAreaStart, addModeAppend)
	}
}

// SortedTwoSideLoopPortSides is
// PortRestorer.sortedTwoSideLoopPortSides(slLoop): the two port sides of a
// two-side self loop in clockwise order.
func SortedTwoSideLoopPortSides(slLoop *loops.SelfHyperLoop) [2]core.PortSide {
	// Sort the port sides, so that they are in clockwise order.
	keys := slLoop.PortSides().KeySet()
	if len(keys) != 2 {
		// Java gets an array of another length here. A self loop of a
		// two-side type has ports on two sides.
		lgraph.Bail("PortRestorer: a two-side self loop has ports on %d sides", len(keys))
	}
	sides := [2]core.PortSide{keys[0], keys[1]}
	if sides[0] > sides[1] {
		sides[0], sides[1] = sides[1], sides[0]
	}

	// NORTH and WEST are the exception: WEST comes before NORTH in
	// clockwise order.
	if sides[0] == core.PortSideNorth && sides[1] == core.PortSideWest {
		sides[0] = core.PortSideWest
		sides[1] = core.PortSideNorth
	}

	return sides
}

////////////////////////////////////////////////////////////////////////////
// Self Loop Placement (Three Sides)

// The constellations that a three-side loop can have.
var (
	nes = [3]core.PortSide{core.PortSideNorth, core.PortSideEast, core.PortSideSouth}
	esw = [3]core.PortSide{core.PortSideEast, core.PortSideSouth, core.PortSideWest}
	swn = [3]core.PortSide{core.PortSideSouth, core.PortSideWest, core.PortSideNorth}
	wne = [3]core.PortSide{core.PortSideWest, core.PortSideNorth, core.PortSideEast}
)

func (r *PortRestorer) processThreeSideLoops() {
	for _, slLoop := range r.slLoopsByType[loops.SelfLoopTypeThreeSides] {
		// The start side, the middle side and the end side of the loop.
		sides := r.determineLoopConstellation(slLoop)

		// Prepend to the area of the start side, append to the other
		// areas.
		r.addLoopToTargetArea(slLoop, sides[0], portSideAreaEnd, addModePrepend)
		r.addLoopToTargetArea(slLoop, sides[1], portSideAreaMiddle, addModeAppend)
		r.addLoopToTargetArea(slLoop, sides[2], portSideAreaStart, addModeAppend)
	}
}

func (r *PortRestorer) determineLoopConstellation(slLoop *loops.SelfHyperLoop) [3]core.PortSide {
	portSides := slLoop.PortSides()

	if !portSides.ContainsKey(core.PortSideNorth) {
		return esw
	} else if !portSides.ContainsKey(core.PortSideEast) {
		return swn
	} else if !portSides.ContainsKey(core.PortSideSouth) {
		return wne
	} else if !portSides.ContainsKey(core.PortSideWest) {
		return nes
	}
	// Java returns null here, and fails on the first use of the array.
	lgraph.Bail("PortRestorer: a three-side self loop has ports on four sides")
	return nes
}

////////////////////////////////////////////////////////////////////////////
// Self Loop Placement (Four Sides)

func (r *PortRestorer) processFourSideLoops() {
	for _, slLoop := range r.slLoopsByType[loops.SelfLoopTypeFourSides] {
		// Append to the middle area of each port side.
		for _, side := range slLoop.PortSides().KeySet() {
			r.addLoopToTargetArea(slLoop, side, portSideAreaMiddle, addModeAppend)
		}
	}
}

////////////////////////////////////////////////////////////////////////////
// Self Loop Placement (Utilities)

// addLoopToTargetArea is addToTargetArea(slLoop, portSide, area, addMode):
// it adds the ports of the loop on the given side to a target area of that
// side.
func (r *PortRestorer) addLoopToTargetArea(slLoop *loops.SelfHyperLoop, portSide core.PortSide, area portSideArea, addMode addMode) {
	r.addToTargetArea(slLoop.SLPortsOnSide(portSide), portSide, area, addMode)
}

// addToTargetArea is addToTargetArea(slPorts, portSide, area, addMode): it
// adds the hidden ones of the given ports to a target area, in reverse
// order, at the start or at the end of the list of the area.
func (r *PortRestorer) addToTargetArea(slPorts []*loops.SelfLoopPort, portSide core.PortSide, area portSideArea, addMode addMode) {
	// Gather the ports that are hidden. A port that is not hidden needs no
	// restoring.
	var hiddenPorts []*loops.SelfLoopPort
	for _, slPort := range slPorts {
		if slPort.IsHidden() {
			hiddenPorts = append(hiddenPorts, slPort)
		}
	}

	slices.Reverse(hiddenPorts)
	targetArea := r.targetAreas[portSide][area]
	if addMode == addModePrepend {
		targetArea = append(hiddenPorts, targetArea...)
	} else {
		targetArea = append(targetArea, hiddenPorts...)
	}
	r.targetAreas[portSide][area] = targetArea
}

////////////////////////////////////////////////////////////////////////////
// Port Restoring

// restorePorts is restorePorts(slHolder): it builds the new port list of the
// node from the old ports and the target areas.
func (r *PortRestorer) restorePorts(slHolder *loops.SelfLoopHolder) {
	// Build a new port list for the node. That is faster than the insertion
	// of ports into the middle of the old list.
	lNode := slHolder.LNode

	// The old ports are added in runs. nextOldPortIndex is the start of the
	// next run.
	oldPortList := append([]*lgraph.LPort(nil), lNode.Ports...)
	nextOldPortIndex := 0

	// The Java code clears the port list of the node and adds to it.
	lNode.Ports = make([]*lgraph.LPort, 0, len(oldPortList))

	north := core.PortSideNorth
	east := core.PortSideEast
	south := core.PortSideSouth
	west := core.PortSideWest

	// Add the target areas between the runs of the regular ports.
	r.addAll(r.targetAreas[north][portSideAreaStart], lNode)
	nextOldPortIndex = r.addAllThat(oldPortList, nextOldPortIndex, func(lPort *lgraph.LPort) bool {
		return lPort.Side == north && r.isNorthSouthPortWithWestOrWestEastConnections(lPort)
	}, lNode)
	r.addAll(r.targetAreas[north][portSideAreaMiddle], lNode)
	nextOldPortIndex = r.addAllThat(oldPortList, nextOldPortIndex, func(lPort *lgraph.LPort) bool {
		return lPort.Side == north
	}, lNode)
	r.addAll(r.targetAreas[north][portSideAreaEnd], lNode)

	r.addAll(r.targetAreas[east][portSideAreaStart], lNode)
	r.addAll(r.targetAreas[east][portSideAreaMiddle], lNode)
	nextOldPortIndex = r.addAllThat(oldPortList, nextOldPortIndex, func(lPort *lgraph.LPort) bool {
		return lPort.Side == east
	}, lNode)
	r.addAll(r.targetAreas[east][portSideAreaEnd], lNode)

	r.addAll(r.targetAreas[south][portSideAreaStart], lNode)
	nextOldPortIndex = r.addAllThat(oldPortList, nextOldPortIndex, func(lPort *lgraph.LPort) bool {
		return lPort.Side == south && r.isNorthSouthPortWithEastConnections(lPort)
	}, lNode)
	r.addAll(r.targetAreas[south][portSideAreaMiddle], lNode)
	nextOldPortIndex = r.addAllThat(oldPortList, nextOldPortIndex, func(lPort *lgraph.LPort) bool {
		return lPort.Side == south
	}, lNode)
	r.addAll(r.targetAreas[south][portSideAreaEnd], lNode)

	r.addAll(r.targetAreas[west][portSideAreaStart], lNode)
	r.addAllThat(oldPortList, nextOldPortIndex, func(lPort *lgraph.LPort) bool {
		return lPort.Side == west
	}, lNode)
	r.addAll(r.targetAreas[west][portSideAreaMiddle], lNode)
	r.addAll(r.targetAreas[west][portSideAreaEnd], lNode)
}

// addAll is addAll(slPorts, lNode): it adds the LPort of each SelfLoopPort
// to the node with LPort.SetNode, which appends the port to the port list.
func (r *PortRestorer) addAll(slPorts []*loops.SelfLoopPort, lNode *lgraph.LNode) {
	for _, slPort := range slPorts {
		slPort.LPort.SetNode(lNode)
	}
}

// addAllThat is addAllThat(lPorts, fromIndex, condition, target): from the
// given index on, it appends each port of the list that satisfies the
// condition to the port list of the node, until a port does not satisfy it.
// It returns the index of that port, which is the next start index.
func (r *PortRestorer) addAllThat(lPorts []*lgraph.LPort, fromIndex int, condition func(*lgraph.LPort) bool, lNode *lgraph.LNode) int {
	for i := fromIndex; i < len(lPorts); i++ {
		// Add the port to the list if it satisfies the condition.
		lPort := lPorts[i]

		if condition(lPort) {
			lNode.Ports = append(lNode.Ports, lPort)
		} else {
			return i
		}
	}

	return len(lPorts)
}

// isNorthSouthPortWithWestOrWestEastConnections is
// isNorthSouthPortWithWestOrWestEastConnections(lPort): the port is a north
// or south port that has connections to the west, with or without
// connections to the east.
func (r *PortRestorer) isNorthSouthPortWithWestOrWestEastConnections(lPort *lgraph.LPort) bool {
	connections := r.northSouthPortConnectionSides(lPort)
	eastConnections := connections.Contains(core.PortSideEast)
	westConnections := connections.Contains(core.PortSideWest)

	return westConnections || (westConnections && eastConnections)
}

// isNorthSouthPortWithEastConnections is
// isNorthSouthPortWithEastConnections(lPort): the port is a north or south
// port that has connections to the east.
func (r *PortRestorer) isNorthSouthPortWithEastConnections(lPort *lgraph.LPort) bool {
	connections := r.northSouthPortConnectionSides(lPort)
	return connections.Contains(core.PortSideEast)
}

// northSouthPortConnectionSides is northSouthPortConnectionSides(lPort): the
// sides to which the given north or south port has connections.
func (r *PortRestorer) northSouthPortConnectionSides(lPort *lgraph.LPort) *prop.EnumSet[core.PortSide] {
	connectionSides := prop.NoneOf[core.PortSide]()

	portDummy := prop.Get(lPort, iprops.PortDummy)
	if portDummy != nil {
		for _, dummyLPort := range portDummy.Ports {
			if prop.Get(dummyLPort, iprops.Origin) == any(lPort) {
				// This dummy port was made for the given port.
				if len(dummyLPort.IncomingEdges) != 0 || len(dummyLPort.OutgoingEdges) != 0 {
					// The dummy port has edges.
					connectionSides.Add(dummyLPort.Side)
				}
			}
		}
	}

	return connectionSides
}
