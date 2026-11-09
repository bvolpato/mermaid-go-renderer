// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/loops/ordering/PortSideAssigner.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package ordering has the classes that give the hidden self loop ports a
// side and put them back into the port list of their node
// (org.eclipse.elk.alg.layered.intermediate.loops.ordering), and the
// processor SelfLoopPortRestorer that uses them.
package ordering

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// PortSideAssigner is the class PortSideAssigner: it gives each hidden self
// loop port a side. The self loop distribution strategy of the node decides
// how. It does not make the ports visible again.
type PortSideAssigner struct{}

// NewPortSideAssigner is new PortSideAssigner().
func NewPortSideAssigner() *PortSideAssigner { return &PortSideAssigner{} }

// AssignPortSides is assignPortSides(slHolder): it gives each hidden port a
// side.
func (a *PortSideAssigner) AssignPortSides(slHolder *loops.SelfLoopHolder) {
	switch prop.Get(slHolder.LNode, lopts.EdgeRoutingSelfLoopDistribution) {
	case lenum.SelfLoopDistributionStrategyNorth:
		a.assignToNorthSide(slHolder)

	case lenum.SelfLoopDistributionStrategyNorthSouth:
		a.assignToNorthOrSouthSide(slHolder)

	case lenum.SelfLoopDistributionStrategyEqually:
		a.assignToAllSides(slHolder)
	}
}

////////////////////////////////////////////////////////////////////////////
// North

// assignToNorthSide is assignToNorthSide(slHolder): each self loop goes to
// the north side.
func (a *PortSideAssigner) assignToNorthSide(slHolder *loops.SelfLoopHolder) {
	for _, slLoop := range slHolder.SLHyperLoops {
		for _, slPort := range a.hiddenSelfLoopPorts(slLoop) {
			slPort.LPort.SetSide(core.PortSideNorth)
		}
	}
}

////////////////////////////////////////////////////////////////////////////
// North, South

// assignToNorthOrSouthSide is assignToNorthOrSouthSide(slHolder): the self
// loops go to the north side or to the south side.
func (a *PortSideAssigner) assignToNorthOrSouthSide(slHolder *loops.SelfLoopHolder) {
	// A self loop can have more than two ports. The aim is a similar number
	// of ports on both sides, not an equal number of self loops. This is a
	// greedy algorithm: the side with fewer ports gets the ports of the next
	// loop.
	northPorts := 0
	southPorts := 0

	for _, slLoop := range slHolder.SLHyperLoops {
		slHiddenPorts := a.hiddenSelfLoopPorts(slLoop)

		// Choose the port side.
		var newPortSide core.PortSide
		if northPorts <= southPorts {
			newPortSide = core.PortSideNorth
			northPorts += len(slHiddenPorts)
		} else {
			newPortSide = core.PortSideSouth
			southPorts += len(slHiddenPorts)
		}

		// Assign the side to the ports.
		for _, slPort := range slHiddenPorts {
			slPort.LPort.SetSide(newPortSide)
		}
	}
}

////////////////////////////////////////////////////////////////////////////
// Equal Distribution

// target is the enum PortSideAssigner.Target: the places that the self loops
// go to, one after the other.
type target struct {
	// firstSide is the first port side of the target.
	firstSide core.PortSide
	// secondSide is the second port side of the target.
	secondSide core.PortSide
}

// targets has the constants of Target in the order of the Java enum: NORTH,
// SOUTH, EAST, WEST, NORTH_WEST_CORNER, NORTH_EAST_CORNER,
// SOUTH_WEST_CORNER, SOUTH_EAST_CORNER.
var targets = [...]target{
	{core.PortSideNorth, core.PortSideNorth},
	{core.PortSideSouth, core.PortSideSouth},
	{core.PortSideEast, core.PortSideEast},
	{core.PortSideWest, core.PortSideWest},
	{core.PortSideWest, core.PortSideNorth},
	{core.PortSideNorth, core.PortSideEast},
	{core.PortSideSouth, core.PortSideWest},
	{core.PortSideEast, core.PortSideSouth},
}

// isCornerTarget is isCornerTarget(): the target spans the corner between
// two sides.
func (t target) isCornerTarget() bool { return t.firstSide != t.secondSide }

// assignToAllSides is assignToAllSides(slHolder): the self loops go to all
// sides.
func (a *PortSideAssigner) assignToAllSides(slHolder *loops.SelfLoopHolder) {
	// Sort the hyper loops by their number of ports, the largest first. So
	// a hyper loop with many ports tends to go to one side, not across a
	// corner.
	slSortedLoops := append([]*loops.SelfHyperLoop(nil), slHolder.SLHyperLoops...)
	gwt.SortFunc(slSortedLoops, func(slLoop1, slLoop2 *loops.SelfHyperLoop) int {
		return gwt.IntCompare(len(slLoop2.SLPorts), len(slLoop1.SLPorts))
	})

	// Assign each self loop to the next target.
	currLoop := 0

	for _, slLoop := range slSortedLoops {
		currTarget := targets[currLoop%len(targets)]
		a.assignToTarget(slLoop, currTarget)

		currLoop++
	}
}

// assignToTarget is assignToTarget(slLoop, target).
func (a *PortSideAssigner) assignToTarget(slLoop *loops.SelfHyperLoop, target target) {
	// The ports of the self loop.
	slPorts := slLoop.SLPorts

	// For a corner target, sort the list of ports by net flow. Ports with
	// more outgoing edges go to the first side, and ports with more incoming
	// edges go to the second side, so that most edges point clockwise. For
	// another target all ports go to the same side, and the order does not
	// matter.
	if target.isCornerTarget() {
		// This sorts the port list of the loop itself. That list has no
		// fixed order before the PortRestorer runs.
		gwt.SortFunc(slPorts, func(slPort1, slPort2 *loops.SelfLoopPort) int {
			return gwt.IntCompare(slPort1.LPort.NetFlow(), slPort2.LPort.NetFlow())
		})
	}

	// The first half of the ports goes to the first side, and the second
	// half to the second side. Only a hidden port gets a side: its side is
	// free.
	secondHalfStartIndex := len(slPorts) / 2

	for i := 0; i < secondHalfStartIndex; i++ {
		slPort := slPorts[i]
		if slPort.IsHidden() {
			slPort.LPort.SetSide(target.firstSide)
		}
	}

	for i := secondHalfStartIndex; i < len(slPorts); i++ {
		slPort := slPorts[i]
		if slPort.IsHidden() {
			slPort.LPort.SetSide(target.secondSide)
		}
	}
}

////////////////////////////////////////////////////////////////////////////
// Utilities

// hiddenSelfLoopPorts is hiddenSelfLoopPortStream(slLoop): the hidden ports
// of the loop in order.
func (a *PortSideAssigner) hiddenSelfLoopPorts(slLoop *loops.SelfHyperLoop) []*loops.SelfLoopPort {
	var hidden []*loops.SelfLoopPort
	for _, slPort := range slLoop.SLPorts {
		if slPort.IsHidden() {
			hidden = append(hidden, slPort)
		}
	}
	return hidden
}
