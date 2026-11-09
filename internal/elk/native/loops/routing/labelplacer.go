// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/loops/routing/LabelPlacer.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package routing

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// LabelPlacer is the class LabelPlacer: it decides where the labels of the
// self loops go. The side of the node and the alignment on that side give
// the place of the labels of a loop. For labels on the north side or the
// south side it computes the x coordinate, and for labels on the east side
// or the west side the y coordinate. The self loop router computes the other
// coordinate.
//
// The label management of the Java class is not here: elkjs has no label
// manager, and the self loop router stops the layout for a graph that has
// one.
type LabelPlacer struct{}

// NewLabelPlacer is new LabelPlacer().
func NewLabelPlacer() *LabelPlacer { return &LabelPlacer{} }

// PlaceLabels is placeLabels(slHolder, labelManager, monitor) without a
// label manager.
func (p *LabelPlacer) PlaceLabels(slHolder *loops.SelfLoopHolder) {
	p.assignSideAndAlignment(slHolder)

	for _, slLoop := range slHolder.SLHyperLoops {
		if slLoop.SLLabels != nil {
			p.computeCoordinates(slLoop)
		}
	}
}

////////////////////////////////////////////////////////////////////////////
// Side and Alignment

func (p *LabelPlacer) assignSideAndAlignment(slHolder *loops.SelfLoopHolder) {
	// Most loops are simple. The loops with ports on the north side only or
	// on the south side only need more work when they are sequenced. Stacked
	// loops get centered labels. Sequenced loops are handled in pairs from
	// the outer loops to the inner loops, and the labels of a pair are
	// aligned away from each other. So these loops are collected first.
	var northernOneSidedSLLoops []*loops.SelfHyperLoop
	var southernOneSidedSLLoops []*loops.SelfHyperLoop

	orderingStrategy := prop.Get(slHolder.LNode, lopts.EdgeRoutingSelfLoopOrdering)
	sequenced := orderingStrategy == lenum.SelfLoopOrderingStrategySequenced

	// Assign the sides and the alignments. The rules depend on the type of
	// the loop.
	for _, slLoop := range slHolder.SLHyperLoops {
		// A loop without labels needs nothing.
		slLabels := slLoop.SLLabels
		if slLabels == nil {
			continue
		}

		// The placement of the labels depends on the self loop type.
		switch slLoop.GetSelfLoopType() {
		case loops.SelfLoopTypeOneSide:
			occupied := slLoop.OccupiedPortSides.Values()
			if len(occupied) == 0 {
				// The iterator of the empty set throws.
				lgraph.Bail("LabelPlacer: a self loop occupies no port side")
			}
			loopSide := occupied[0]

			if sequenced && loopSide == core.PortSideNorth {
				// Collect the loop for later.
				northernOneSidedSLLoops = append(northernOneSidedSLLoops, slLoop)
			} else if sequenced && loopSide == core.PortSideSouth {
				// Collect the loop for later.
				southernOneSidedSLLoops = append(southernOneSidedSLLoops, slLoop)
			} else {
				p.assignOneSidedSimpleSideAndAlignment(slLoop, loopSide)
			}

		case loops.SelfLoopTypeTwoSidesCorner:
			p.assignTwoSidesCornerSideAndAlignment(slLoop)

		case loops.SelfLoopTypeTwoSidesOpposing, loops.SelfLoopTypeThreeSides:
			p.assignTwoSidesOpposingAndThreeSidesSideAndAlignment(slLoop)

		case loops.SelfLoopTypeFourSides:
			p.assignFourSidesSideAndAlignment(slLoop)
		}
	}

	// Process the collected loops.
	if sequenced {
		if len(northernOneSidedSLLoops) != 0 {
			p.assignOneSidedSequencedSideAndAlignment(northernOneSidedSLLoops, core.PortSideNorth)
		}

		if len(southernOneSidedSLLoops) != 0 {
			p.assignOneSidedSequencedSideAndAlignment(southernOneSidedSLLoops, core.PortSideSouth)
		}
	}
}

// assignOneSidedSimpleSideAndAlignment is
// assignOneSidedSimpleSideAndAlignment(slLoop, loopSide): the side and the
// alignment of a one-sided self loop where the choice is clear. That is so
// for eastern and western self loops, and for northern and southern self
// loops that are stacked.
func (p *LabelPlacer) assignOneSidedSimpleSideAndAlignment(slLoop *loops.SelfHyperLoop, loopSide core.PortSide) {
	switch loopSide {
	case core.PortSideEast, core.PortSideWest:
		// The alignment is relative to the topmost port, which is the
		// leftmost port or the rightmost port.
		topmostPort := slLoop.LeftmostPort
		if slLoop.RightmostPort.LPort.Pos.Y < topmostPort.LPort.Pos.Y {
			topmostPort = slLoop.RightmostPort
		}

		p.assign(slLoop, loopSide, loops.AlignmentTop, topmostPort)

	case core.PortSideNorth, core.PortSideSouth:
		p.assign(slLoop, loopSide, loops.AlignmentCenter, nil)
	}
}

func (p *LabelPlacer) assignOneSidedSequencedSideAndAlignment(slLoops []*loops.SelfHyperLoop, portSide core.PortSide) {
	// Give each port its index in the port list as id.
	id := 0
	for _, lPort := range slLoops[0].SLHolder.LNode.Ports {
		lPort.ID = id
		id++
	}

	// Sort the list by the id of the leftmost port. For northern loops the
	// list is then sorted from left to right. For southern loops the order
	// is descending, so that the loop at the left is first too.
	if portSide == core.PortSideNorth {
		gwt.SortFunc(slLoops, func(slLoop1, slLoop2 *loops.SelfHyperLoop) int {
			return gwt.IntCompare(slLoop1.LeftmostPort.LPort.ID, slLoop2.LeftmostPort.LPort.ID)
		})
	} else {
		gwt.SortFunc(slLoops, func(slLoop1, slLoop2 *loops.SelfHyperLoop) int {
			return gwt.IntCompare(slLoop2.LeftmostPort.LPort.ID, slLoop1.LeftmostPort.LPort.ID)
		})
	}

	// Go from the outer loops to the inner loops in pairs.
	leftSlLoopIdx := 0
	rightSlLoopIdx := len(slLoops) - 1

	for leftSlLoopIdx < rightSlLoopIdx {
		leftSlLoop := slLoops[leftSlLoopIdx]
		rightSlLoop := slLoops[rightSlLoopIdx]

		// On the north side the leftmost port is at the left of the
		// rightmost port. On the south side it is at the right.
		var leftLoopAlignmentReference, rightLoopAlignmentReference *loops.SelfLoopPort
		if portSide == core.PortSideNorth {
			leftLoopAlignmentReference = leftSlLoop.RightmostPort
			rightLoopAlignmentReference = rightSlLoop.LeftmostPort
		} else {
			leftLoopAlignmentReference = leftSlLoop.LeftmostPort
			rightLoopAlignmentReference = rightSlLoop.RightmostPort
		}

		p.assign(leftSlLoop, portSide, loops.AlignmentRight, leftLoopAlignmentReference)
		p.assign(rightSlLoop, portSide, loops.AlignmentLeft, rightLoopAlignmentReference)

		leftSlLoopIdx++
		rightSlLoopIdx--
	}

	// One loop can remain in the middle.
	if leftSlLoopIdx == rightSlLoopIdx {
		p.assign(slLoops[leftSlLoopIdx], portSide, loops.AlignmentCenter, nil)
	}
}

func (p *LabelPlacer) assignTwoSidesCornerSideAndAlignment(slLoop *loops.SelfHyperLoop) {
	// The side and the alignment depend on the sides of the leftmost port
	// and of the rightmost port. With the leftmost port on the north side
	// the loop goes to the east side, and the label is left-aligned on the
	// north side. The other cases are similar.
	leftmostPortSide := slLoop.LeftmostPort.LPort.Side
	rightmostPortSide := slLoop.RightmostPort.LPort.Side

	if leftmostPortSide == core.PortSideNorth {
		p.assign(slLoop, core.PortSideNorth, loops.AlignmentLeft, slLoop.LeftmostPort)
	} else if rightmostPortSide == core.PortSideNorth {
		p.assign(slLoop, core.PortSideNorth, loops.AlignmentRight, slLoop.RightmostPort)
	} else if leftmostPortSide == core.PortSideSouth {
		p.assign(slLoop, core.PortSideSouth, loops.AlignmentRight, slLoop.LeftmostPort)
	} else if rightmostPortSide == core.PortSideSouth {
		p.assign(slLoop, core.PortSideSouth, loops.AlignmentLeft, slLoop.RightmostPort)
	}
}

func (p *LabelPlacer) assignTwoSidesOpposingAndThreeSidesSideAndAlignment(slLoop *loops.SelfHyperLoop) {
	// The place of the labels depends on the port side that the self loop
	// does not occupy. Without the north side, the loop covers the whole
	// south side, and the label is centered there. Without the west side,
	// the label is left-aligned on the north side.
	occupiedSides := slLoop.OccupiedPortSides

	if !occupiedSides.Contains(core.PortSideNorth) {
		p.assign(slLoop, core.PortSideSouth, loops.AlignmentCenter, nil)
	} else if !occupiedSides.Contains(core.PortSideSouth) {
		p.assign(slLoop, core.PortSideNorth, loops.AlignmentCenter, nil)
	} else if !occupiedSides.Contains(core.PortSideWest) {
		p.assign(slLoop, core.PortSideNorth, loops.AlignmentLeft, slLoop.LeftmostPort)
	} else if !occupiedSides.Contains(core.PortSideEast) {
		p.assign(slLoop, core.PortSideNorth, loops.AlignmentRight, slLoop.RightmostPort)
	}
}

func (p *LabelPlacer) assignFourSidesSideAndAlignment(slLoop *loops.SelfHyperLoop) {
	// There are two cases: the loop covers the whole north side, or the
	// loop covers the whole south side. The label is centered on the side
	// that is covered. A side with the leftmost port or the rightmost port
	// is not covered. The Java code reads the side of the leftmost port for
	// both variables.
	leftmostPortSide := slLoop.LeftmostPort.LPort.Side
	rightmostPortSide := slLoop.LeftmostPort.LPort.Side

	if leftmostPortSide == core.PortSideNorth || rightmostPortSide == core.PortSideNorth {
		p.assign(slLoop, core.PortSideSouth, loops.AlignmentCenter, nil)
	} else {
		p.assign(slLoop, core.PortSideNorth, loops.AlignmentCenter, nil)
	}
}

// assign is assignSideAndAlignment(slLoop, side, alignment,
// alignmentReference).
func (p *LabelPlacer) assign(slLoop *loops.SelfHyperLoop, side core.PortSide, alignment loops.Alignment, alignmentReference *loops.SelfLoopPort) {
	slLoop.SLLabels.SetSideAndAlignment(side, alignment, alignmentReference)
}

////////////////////////////////////////////////////////////////////////////
// Coordinate Computation

func (p *LabelPlacer) computeCoordinates(slLoop *loops.SelfHyperLoop) {
	// The coordinates depend on the alignment only: LEFT, CENTER and RIGHT
	// are for the north side and the south side, and TOP is for the east
	// side and the west side.
	slLabels := slLoop.SLLabels
	if !slLabels.Placed {
		// The alignment is null, and Java fails.
		lgraph.Bail("LabelPlacer: the labels of a self loop have no alignment")
	}

	alignRef := slLabels.AlignmentReferenceSLPort
	size := &slLabels.Size
	pos := &slLabels.Position

	if slLabels.Alignment != loops.AlignmentCenter && alignRef == nil {
		// Java fails with a NullPointerException.
		lgraph.Bail("LabelPlacer: the labels of a self loop have no alignment reference port")
	}

	switch slLabels.Alignment {
	case loops.AlignmentCenter:
		pos.X = (slLoop.SLHolder.LNode.Size.X - size.X) / 2

	case loops.AlignmentLeft:
		pos.X = alignRef.LPort.Pos.X + alignRef.LPort.Anchor.X

	case loops.AlignmentRight:
		pos.X = alignRef.LPort.Pos.X + alignRef.LPort.Anchor.X - size.X

	case loops.AlignmentTop:
		pos.Y = alignRef.LPort.Pos.Y + alignRef.LPort.Anchor.Y
	}
}
