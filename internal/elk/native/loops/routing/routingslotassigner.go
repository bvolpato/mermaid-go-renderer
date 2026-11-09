// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/loops/routing/RoutingSlotAssigner.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package routing

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p5edges/orthogonal"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// RoutingSlotAssigner is the class RoutingSlotAssigner: it gives each
// SelfHyperLoop a routing slot on each port side that the loop spans, and
// tries to keep the number of crossings between self loops low. It uses the
// cycle breaker of the orthogonal edge router, which orders the vertical
// segments of hyperedges in the same way.
type RoutingSlotAssigner struct {
	// hyperEdgeSegments has the segments of the crossing graph.
	hyperEdgeSegments []*orthogonal.HyperEdgeSegment
	// slLoopToSegmentMap has the segment of each loop.
	slLoopToSegmentMap map[*loops.SelfHyperLoop]*orthogonal.HyperEdgeSegment
	// slLoopActivityOverPorts has for each loop an array that tells, for the
	// id of each port, whether the loop runs along the port.
	slLoopActivityOverPorts map[*loops.SelfHyperLoop][]bool
}

// NewRoutingSlotAssigner is new RoutingSlotAssigner().
func NewRoutingSlotAssigner() *RoutingSlotAssigner { return &RoutingSlotAssigner{} }

// AssignRoutingSlots is assignRoutingSlots(slHolder): it gives each self
// loop a routing slot on each port side that the loop spans.
func (a *RoutingSlotAssigner) AssignRoutingSlots(slHolder *loops.SelfLoopHolder) {
	// The overlap matrix tells the later steps which labels can overlap.
	labelCrossingMatrix := a.computeLabelCrossingMatrix(slHolder)

	// Make the crossing graph of the loops, and break its cycles with the
	// cycle breaker of the orthogonal edge router.
	a.createCrossingGraph(slHolder, labelCrossingMatrix)
	graph := slHolder.LNode.Graph()
	if graph == nil {
		lgraph.Bail("RoutingSlotAssigner: the node of a self loop has no graph")
	}
	orthogonal.BreakNonCriticalCycles(a.hyperEdgeSegments, prop.Get(graph, iprops.Random))

	// Assign the routing slots from the graph.
	a.doAssignRoutingSlots(slHolder, labelCrossingMatrix)

	// Clean up.
	a.hyperEdgeSegments = nil
	a.slLoopToSegmentMap = nil
	a.slLoopActivityOverPorts = nil
}

////////////////////////////////////////////////////////////////////////////
// Label Crossing Matrix

// computeLabelCrossingMatrix is computeLabelCrossingMatrix(slHolder): a
// matrix whose elements are true where the labels with the two ids overlap.
func (a *RoutingSlotAssigner) computeLabelCrossingMatrix(slHolder *loops.SelfLoopHolder) [][]bool {
	// Give each label an id first.
	labelID := 0
	for _, slLoop := range slHolder.SLHyperLoops {
		if slLoop.SLLabels != nil {
			slLoop.SLLabels.SetID(labelID)
			labelID++
		}
	}

	crossingMatrix := make([][]bool, labelID)
	for i := range crossingMatrix {
		crossingMatrix[i] = make([]bool, labelID)
	}

	// Check for each pair of labels whether they overlap.
	slLoops := slHolder.SLHyperLoops
	for sl1Idx := 0; sl1Idx < len(slLoops); sl1Idx++ {
		slLoop1 := slLoops[sl1Idx]

		if slLoop1.SLLabels != nil {
			for sl2Idx := sl1Idx + 1; sl2Idx < len(slLoops); sl2Idx++ {
				slLoop2 := slLoops[sl2Idx]

				if slLoop2.SLLabels != nil {
					overlap := a.labelsOverlap(slLoop1, slLoop2)

					crossingMatrix[slLoop1.SLLabels.ID][slLoop2.SLLabels.ID] = overlap
					crossingMatrix[slLoop2.SLLabels.ID][slLoop1.SLLabels.ID] = overlap
				}
			}
		}
	}

	return crossingMatrix
}

// labelsOverlap is labelsOverlap(slLoop1, slLoop2): the labels of the two
// loops would overlap if the loops had the same routing slot.
func (a *RoutingSlotAssigner) labelsOverlap(slLoop1, slLoop2 *loops.SelfHyperLoop) bool {
	slLabels1 := slLoop1.SLLabels
	slLabels2 := slLoop2.SLLabels

	// Labels overlap only when both loops have labels.
	if slLabels1 == nil || slLabels2 == nil {
		return false
	}

	// The labels must be on the same side, and that side must be north or
	// south.
	if slLabels1.Side != slLabels2.Side || slLabels1.Side == core.PortSideEast || slLabels1.Side == core.PortSideWest {
		return false
	}

	// Check whether the labels overlap horizontally. The label placer set
	// the horizontal coordinates of north labels and south labels before.
	start1 := slLabels1.Position.X
	end1 := start1 + slLabels1.Size.X
	start2 := slLabels2.Position.X
	end2 := start2 + slLabels2.Size.X

	return start1 <= end2 && end1 >= start2
}

////////////////////////////////////////////////////////////////////////////
// Crossing Graph

// createCrossingGraph is createCrossingGraph(slHolder, labelCrossingMatrix):
// it makes one HyperEdgeSegment for each SelfHyperLoop, and the dependencies
// between the segments.
func (a *RoutingSlotAssigner) createCrossingGraph(slHolder *loops.SelfLoopHolder, labelCrossingMatrix [][]bool) {
	slLoops := slHolder.SLHyperLoops

	// Make an edge segment for each loop.
	a.hyperEdgeSegments = make([]*orthogonal.HyperEdgeSegment, 0, len(slLoops))
	a.slLoopToSegmentMap = map[*loops.SelfHyperLoop]*orthogonal.HyperEdgeSegment{}

	for _, slLoop := range slLoops {
		segment := orthogonal.NewHyperEdgeSegment(nil)
		a.hyperEdgeSegments = append(a.hyperEdgeSegments, segment)
		a.slLoopToSegmentMap[slLoop] = segment
	}

	// Remember for each loop whether it is active at each port id. That
	// makes the count of the crossings fast.
	a.slLoopActivityOverPorts = map[*loops.SelfHyperLoop][]bool{}
	a.computeLoopActivity(slHolder)

	// For each pair of hyper loops, count the crossings with one segment
	// above the other and with the other one above. This also makes
	// dependencies between self loops that share no port. That is intended:
	// each segment gets its own routing slot, so segments cannot collide on
	// a side without ports. A later step makes the routing space compact.
	for firstIdx := 0; firstIdx < len(slLoops)-1; firstIdx++ {
		slLoop1 := slHolder.SLHyperLoops[firstIdx]
		for secondIdx := firstIdx + 1; secondIdx < len(slLoops); secondIdx++ {
			a.createDependencies(slLoop1, slHolder.SLHyperLoops[secondIdx], labelCrossingMatrix)
		}
	}
}

// computeLoopActivity is computeLoopActivity(slHolder): it fills
// slLoopActivityOverPorts. Each loop gets an array, indexed by port ids,
// that tells whether the loop runs along the port.
func (a *RoutingSlotAssigner) computeLoopActivity(slHolder *loops.SelfLoopHolder) {
	slLoops := slHolder.SLHyperLoops
	lPorts := slHolder.LNode.Ports

	for _, slLoop := range slLoops {
		loopActivity := make([]bool, len(lPorts))
		a.slLoopActivityOverPorts[slLoop] = loopActivity

		// The ids of the ports of a loop are indices of the port list of the
		// node, when each port of the loop is in that list. With another id
		// the Java code fails, and the loop below would not end in elkjs.
		for _, slPort := range slLoop.SLPorts {
			if id := slPort.LPort.ID; id < 0 || id >= len(lPorts) {
				lgraph.Bail("RoutingSlotAssigner: a self loop port is not in the port list of its node")
			}
		}

		// Go from the start port of the loop to its end port, past the end
		// of the port list if needed, and mark each port on the way.
		lPortIdx := slLoop.LeftmostPort.LPort.ID - 1
		lPortTargetIdx := slLoop.RightmostPort.LPort.ID

		for lPortIdx != lPortTargetIdx {
			lPortIdx = (lPortIdx + 1) % len(lPorts)
			loopActivity[lPortIdx] = true
		}
	}
}

// createDependencies is createDependencies(slLoop1, slLoop2,
// labelCrossingMatrix): it makes the dependencies between the segments of
// the two loops.
func (a *RoutingSlotAssigner) createDependencies(slLoop1, slLoop2 *loops.SelfHyperLoop, labelCrossingMatrix [][]bool) {
	// Count the crossings with the first segment above the second one, and
	// with the second segment above the first one.
	firstAboveSecondCrossings := a.countCrossings(slLoop1, slLoop2)
	secondAboveFirstCrossings := a.countCrossings(slLoop2, slLoop1)

	// Make the dependencies.
	segment1 := a.slLoopToSegmentMap[slLoop1]
	segment2 := a.slLoopToSegmentMap[slLoop2]

	if firstAboveSecondCrossings < secondAboveFirstCrossings {
		// The first loop goes above the second loop.
		orthogonal.CreateAndAddRegular(segment1, segment2, secondAboveFirstCrossings-firstAboveSecondCrossings)
	} else if secondAboveFirstCrossings < firstAboveSecondCrossings {
		// The second loop goes above the first loop.
		orthogonal.CreateAndAddRegular(segment2, segment1, firstAboveSecondCrossings-secondAboveFirstCrossings)
	} else if firstAboveSecondCrossings != 0 || a.labelsOverlapInMatrix(slLoop1, slLoop2, labelCrossingMatrix) {
		// Both orders cause the same number of crossings, which is not
		// zero, or the labels of the two loops overlap. The loops must go
		// to different slots.
		orthogonal.CreateAndAddRegular(segment1, segment2, 0)
		orthogonal.CreateAndAddRegular(segment2, segment1, 0)
	}
}

// countCrossings is countCrossings(slUpperLoop, slLowerLoop): the number of
// crossings between a loop in a higher routing slot and a loop in a lower
// one. Each port of the upper loop where the lower loop is active causes a
// crossing.
func (a *RoutingSlotAssigner) countCrossings(slUpperLoop, slLowerLoop *loops.SelfHyperLoop) int {
	lowerLoopActivity := a.slLoopActivityOverPorts[slLowerLoop]
	crossings := 0

	for _, slPort := range slUpperLoop.SLPorts {
		if lowerLoopActivity[slPort.LPort.ID] {
			crossings++
		}
	}

	return crossings
}

// labelsOverlapInMatrix is labelsOverlap(slLoop1, slLoop2,
// labelCrossingMatrix): the label crossing matrix tells whether the labels
// of the two loops overlap.
func (a *RoutingSlotAssigner) labelsOverlapInMatrix(slLoop1, slLoop2 *loops.SelfHyperLoop, labelCrossingMatrix [][]bool) bool {
	if slLoop1.SLLabels == nil || slLoop2.SLLabels == nil {
		return false
	}
	return labelCrossingMatrix[slLoop1.SLLabels.ID][slLoop2.SLLabels.ID]
}

////////////////////////////////////////////////////////////////////////////
// Slot Assignment

// doAssignRoutingSlots is doAssignRoutingSlots(slHolder,
// labelCrossingMatrix): with the crossing graph free of cycles, it gives
// each self loop a routing slot on each of its port sides.
func (a *RoutingSlotAssigner) doAssignRoutingSlots(slHolder *loops.SelfLoopHolder, labelCrossingMatrix [][]bool) {
	// Compute the raw slots first.
	a.assignRawRoutingSlotsToSegments()

	// Assign the raw routing slots, then make the assignment compact.
	a.assignRawRoutingSlotsToLoops(slHolder)
	a.shiftTowardsNode(slHolder, labelCrossingMatrix)
}

// assignRawRoutingSlotsToSegments is assignRawRoutingSlotsToSegments(): it
// gives the segments routing slots as if the four port sides depended on
// each other.
func (a *RoutingSlotAssigner) assignRawRoutingSlotsToSegments() {
	var sinks []*orthogonal.HyperEdgeSegment

	// Fill the queue of the sinks, which are routed nearest to the node.
	// Also set the in weight and the out weight of each segment to its
	// number of incoming and of outgoing dependencies.
	for _, segment := range a.hyperEdgeSegments {
		segment.SetInWeight(len(segment.IncomingSegmentDependencies()))
		segment.SetOutWeight(len(segment.OutgoingSegmentDependencies()))

		if segment.OutWeight() == 0 {
			segment.SetRoutingSlot(0)
			sinks = append(sinks, segment)
		}
	}

	// Assign the raw routing slots.
	for len(sinks) != 0 {
		segment := sinks[0]
		sinks = sinks[1:]
		nextRoutingSlot := segment.RoutingSlot() + 1

		for _, inDependency := range segment.IncomingSegmentDependencies() {
			sourceSegment := inDependency.Source()
			sourceSegment.SetRoutingSlot(max(sourceSegment.RoutingSlot(), nextRoutingSlot))

			sourceSegment.SetOutWeight(sourceSegment.OutWeight() - 1)
			if sourceSegment.OutWeight() == 0 {
				sinks = append(sinks, sourceSegment)
			}
		}
	}
}

// assignRawRoutingSlotsToLoops is assignRawRoutingSlotsToLoops(slHolder): it
// gives each loop the raw routing slot of its segment on each side that the
// loop occupies.
func (a *RoutingSlotAssigner) assignRawRoutingSlotsToLoops(slHolder *loops.SelfLoopHolder) {
	for _, slLoop := range slHolder.SLHyperLoops {
		slot := a.slLoopToSegmentMap[slLoop].RoutingSlot()
		for _, portSide := range slLoop.OccupiedPortSides.Values() {
			slLoop.SetRoutingSlot(portSide, slot)
		}
	}
}

// shiftTowardsNode is shiftTowardsNode(slHolder, labelCrossingMatrix): it
// moves the self loops towards the node on each side, so that no routing
// slot stays empty.
func (a *RoutingSlotAssigner) shiftTowardsNode(slHolder *loops.SelfLoopHolder, labelCrossingMatrix [][]bool) {
	// For each port, this array has the next routing slot that a self loop
	// on the side of the port can get.
	nextFreeRoutingSlotAtPort := make([]int, len(slHolder.LNode.Ports))

	a.shiftTowardsNodeOnSide(slHolder, core.PortSideNorth, nextFreeRoutingSlotAtPort, labelCrossingMatrix)
	a.shiftTowardsNodeOnSide(slHolder, core.PortSideEast, nextFreeRoutingSlotAtPort, labelCrossingMatrix)
	a.shiftTowardsNodeOnSide(slHolder, core.PortSideSouth, nextFreeRoutingSlotAtPort, labelCrossingMatrix)
	a.shiftTowardsNodeOnSide(slHolder, core.PortSideWest, nextFreeRoutingSlotAtPort, labelCrossingMatrix)
}

func (a *RoutingSlotAssigner) shiftTowardsNodeOnSide(slHolder *loops.SelfLoopHolder, side core.PortSide, nextFreeRoutingSlotAtPort []int, labelCrossingMatrix [][]bool) {
	// The self loops that occupy the port side, in ascending order of their
	// routing slots.
	var slLoops []*loops.SelfHyperLoop
	for _, slLoop := range slHolder.SLHyperLoops {
		if slLoop.OccupiedPortSides.Contains(side) {
			slLoops = append(slLoops, slLoop)
		}
	}
	gwt.SortFunc(slLoops, func(sl1, sl2 *loops.SelfHyperLoop) int {
		return gwt.IntCompare(sl1.GetRoutingSlot(side), sl2.GetRoutingSlot(side))
	})

	// Find the index of the first port and of the last port on the port
	// side.
	minLPortIndex := math.MaxInt32
	maxLPortIndex := math.MinInt32
	for _, lPort := range slHolder.LNode.Ports {
		if lPort.Side == side {
			minLPortIndex = min(minLPortIndex, lPort.ID)
			maxLPortIndex = max(maxLPortIndex, lPort.ID)
		}
	}

	if minLPortIndex == math.MaxInt32 {
		// This side has no ports. The loops get the slots from 0 on, one
		// after the other. That causes no label overlaps.
		for i, slLoop := range slLoops {
			slLoop.SetRoutingSlot(side, i)
		}
	} else {
		slotAssignedToLabel := make([]int, len(labelCrossingMatrix))
		for i := range slotAssignedToLabel {
			slotAssignedToLabel[i] = -1
		}

		// This side has ports. Find the lowest slot that is free at each
		// port that the loop spans. No label that conflicts with the label
		// of the loop may be in that slot.
		for _, slLoop := range slLoops {
			activeAtPort := a.slLoopActivityOverPorts[slLoop]
			lowestAvailableSlot := 0

			for portIndex := minLPortIndex; portIndex <= maxLPortIndex; portIndex++ {
				if activeAtPort[portIndex] {
					lowestAvailableSlot = max(lowestAvailableSlot, nextFreeRoutingSlotAtPort[portIndex])
				}
			}

			// The label of the loop can conflict with a label in the lowest
			// available slot, and with labels in higher slots. So collect
			// the slots of the conflicting labels that have a slot, and find
			// the first slot without a conflict.
			if slLoop.SLLabels != nil {
				ourLabelIdx := slLoop.SLLabels.ID
				slotsWithLabelConflicts := map[int]bool{}

				for otherLabelIdx := 0; otherLabelIdx < len(labelCrossingMatrix); otherLabelIdx++ {
					if labelCrossingMatrix[ourLabelIdx][otherLabelIdx] {
						slotsWithLabelConflicts[slotAssignedToLabel[otherLabelIdx]] = true
					}
				}

				// Find the first slot, from the lowest available one on,
				// that is not in the set.
				for slotsWithLabelConflicts[lowestAvailableSlot] {
					lowestAvailableSlot++
				}
			}

			// Assign the routing slot to the loop, and update the array of
			// the next free slots.
			slLoop.SetRoutingSlot(side, lowestAvailableSlot)
			for portIndex := minLPortIndex; portIndex <= maxLPortIndex; portIndex++ {
				if activeAtPort[portIndex] {
					nextFreeRoutingSlotAtPort[portIndex] = lowestAvailableSlot + 1
				}
			}

			// Remember the routing slot of the label of the loop.
			if slLoop.SLLabels != nil {
				slotAssignedToLabel[slLoop.SLLabels.ID] = lowestAvailableSlot
			}
		}
	}
}
