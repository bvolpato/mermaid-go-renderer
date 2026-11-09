// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/internal/algorithm/PortLabelPlacementCalculator.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package algorithm

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/coreopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/cellsystem"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/internal"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/overlaps"
)

// PlaceHorizontalPortLabels is
// PortLabelPlacementCalculator.placeHorizontalPortLabels(nodeContext): it
// places the port labels of the northern and the southern ports. For inside
// port labels it sets the height that the placement needs as the height of
// the content area of the northern and southern inside port label cells.
func PlaceHorizontalPortLabels(nodeContext *internal.NodeContext) {
	placePortLabels(nodeContext, core.PortSideNorth)
	placePortLabels(nodeContext, core.PortSideSouth)
}

// PlaceVerticalPortLabels is
// PortLabelPlacementCalculator.placeVerticalPortLabels(nodeContext): it
// places the port labels of the eastern and the western ports.
func PlaceVerticalPortLabels(nodeContext *internal.NodeContext) {
	placePortLabels(nodeContext, core.PortSideEast)
	placePortLabels(nodeContext, core.PortSideWest)
}

// placePortLabels places the port labels of the ports of a side.
func placePortLabels(nodeContext *internal.NodeContext, portSide core.PortSide) {
	// If port labels were not taken into account when calculating the node
	// size or if port placement was set to fixed positions, we don't have an
	// arbitrary amount of freedom to place our labels
	constrainedPlacement := !nodeContext.SizeConstraints.Contains(core.SizeConstraintPortLabels) ||
		nodeContext.PortConstraints == core.PortConstraintsFixedPos

	if nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementInside) {
		if constrainedPlacement {
			constrainedInsidePortLabelPlacement(nodeContext, portSide)
		} else {
			simpleInsidePortLabelPlacement(nodeContext, portSide)
		}
	} else if nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementOutside) {
		if constrainedPlacement {
			constrainedOutsidePortLabelPlacement(nodeContext, portSide)
		} else {
			simpleOutsidePortLabelPlacement(nodeContext, portSide)
		}
	}
}

// simpleInsidePortLabelPlacement places the port label cells on the inside
// of the node.
func simpleInsidePortLabelPlacement(nodeContext *internal.NodeContext, portSide core.PortSide) {
	// For northern and southern port labels, we need to set the inside port
	// label cell's client height later
	insideNorthOrSouthPortLabelAreaHeight := 0.0

	// Some spacings we may need later
	labelBorderOffset := portLabelBorderOffsetForPortSide(nodeContext, portSide)
	portLabelSpacingHorizontal := nodeContext.PortLabelSpacingHorizontal
	portLabelSpacingVertical := nodeContext.PortLabelSpacingVertical

	for _, portContext := range nodeContext.PortContexts.Get(portSide) {
		// If the port doesn't have labels, skip
		if portContext.PortLabelCell == nil || !portContext.PortLabelCell.HasLabels() {
			continue
		}

		// Retrieve information about the port itself
		portSize := portContext.Port.Size()
		portBorderOffset := 0.0
		if adapters.HasProperty(portContext.Port, coreopts.PortBorderOffset) {
			portBorderOffset = adapters.GetProperty(portContext.Port, coreopts.PortBorderOffset)
		}

		// Retrieve the label cell and its rectangle and set the rectangle's
		// size (we will use the rectangle to place the cell relative to the
		// port below)
		portLabelCell := portContext.PortLabelCell
		portLabelCellRect := portLabelCell.CellRectangle()
		portLabelCellRect.Width = portLabelCell.MinimumWidth()
		portLabelCellRect.Height = portLabelCell.MinimumHeight()

		// Calculate the position of the port's label cell. If the node is a
		// compound node, we make an effort to place port labels such that
		// edges won't cross them
		switch portSide {
		case core.PortSideNorth:
			if portContext.LabelsNextToPort {
				portLabelCellRect.X = (portSize.X - portLabelCellRect.Width) / 2
			} else {
				portLabelCellRect.X = portSize.X + portLabelSpacingHorizontal
			}
			portLabelCellRect.Y = portSize.Y + portBorderOffset + labelBorderOffset
			portLabelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentCenter)
			portLabelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentTop)

		case core.PortSideSouth:
			if portContext.LabelsNextToPort {
				portLabelCellRect.X = (portSize.X - portLabelCellRect.Width) / 2
			} else {
				portLabelCellRect.X = portSize.X + portLabelSpacingHorizontal
			}
			portLabelCellRect.Y = -portBorderOffset - labelBorderOffset - portLabelCellRect.Height
			portLabelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentCenter)
			portLabelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentBottom)

		case core.PortSideEast:
			portLabelCellRect.X = -portBorderOffset - labelBorderOffset - portLabelCellRect.Width
			if portContext.LabelsNextToPort {
				labelHeight := portLabelCellRect.Height
				if !nodeContext.PortLabelsTreatAsGroup {
					labelHeight = portLabelCell.Labels()[0].Size().Y
				}
				portLabelCellRect.Y = (portSize.Y - labelHeight) / 2
			} else {
				portLabelCellRect.Y = portSize.Y + portLabelSpacingVertical
			}
			portLabelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentRight)
			portLabelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentCenter)

		case core.PortSideWest:
			portLabelCellRect.X = portSize.X + portBorderOffset + labelBorderOffset
			if portContext.LabelsNextToPort {
				labelHeight := portLabelCellRect.Height
				if !nodeContext.PortLabelsTreatAsGroup {
					labelHeight = portLabelCell.Labels()[0].Size().Y
				}
				portLabelCellRect.Y = (portSize.Y - labelHeight) / 2
			} else {
				portLabelCellRect.Y = portSize.Y + portLabelSpacingVertical
			}
			portLabelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentLeft)
			portLabelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentCenter)
		}

		// If we have a north or south port, update our port label area
		// height
		if portSide == core.PortSideNorth || portSide == core.PortSideSouth {
			insideNorthOrSouthPortLabelAreaHeight = max(
				insideNorthOrSouthPortLabelAreaHeight,
				portLabelCellRect.Height)
		}
	}

	// If we have a northern or southern label area height, apply it
	if insideNorthOrSouthPortLabelAreaHeight > 0 {
		nodeContext.InsidePortLabelCells[portSide].MinimumContentAreaSize().Y =
			insideNorthOrSouthPortLabelAreaHeight
	}
}

// portLabelBorderOffsetForPortSide returns the distance of the port labels
// of a side from the node border: the padding of the node cell plus the
// port-label spacing.
func portLabelBorderOffsetForPortSide(nodeContext *internal.NodeContext, portSide core.PortSide) float64 {
	switch portSide {
	case core.PortSideNorth:
		return nodeContext.NodeContainer.Padding().Top + nodeContext.PortLabelSpacingVertical
	case core.PortSideSouth:
		return nodeContext.NodeContainer.Padding().Bottom + nodeContext.PortLabelSpacingVertical
	case core.PortSideEast:
		return nodeContext.NodeContainer.Padding().Right + nodeContext.PortLabelSpacingHorizontal
	case core.PortSideWest:
		return nodeContext.NodeContainer.Padding().Left + nodeContext.PortLabelSpacingHorizontal
	default:
		return 0
	}
}

// constrainedInsidePortLabelPlacement places the port label cells on the
// inside of the node when the space may be too small to place them without
// overlaps.
func constrainedInsidePortLabelPlacement(nodeContext *internal.NodeContext, portSide core.PortSide) {
	portContexts := nodeContext.PortContexts.Get(portSide)

	// If it's neither the northern nor the southern side, simply revert to
	// simple port label placement
	if portSide == core.PortSideEast || portSide == core.PortSideWest {
		simpleInsidePortLabelPlacement(nodeContext, portSide)
		return
	}

	// Prepare things
	overlapRemovalDirection := overlaps.OverlapRemovalDirectionUp
	verticalLabelAlignment := cellsystem.VerticalLabelAlignmentBottom
	if portSide == core.PortSideNorth {
		overlapRemovalDirection = overlaps.OverlapRemovalDirectionDown
		verticalLabelAlignment = cellsystem.VerticalLabelAlignmentTop
	}

	// To keep labels from extending over the content area of the inside port
	// label container, we need to know where its content area's left and
	// right boundaries are. We also make sure to always keep a bit of space
	// to the node border
	insidePortLabelContainer := nodeContext.InsidePortLabelCells[portSide]
	labelContainerRect := insidePortLabelContainer.CellRectangle()
	leftBorder := labelContainerRect.X + elkMathMaxd(
		insidePortLabelContainer.Padding().Left,
		nodeContext.SurroundingPortMargins.Left,
		nodeContext.NodeLabelSpacing)
	rightBorder := labelContainerRect.X + labelContainerRect.Width - elkMathMaxd(
		insidePortLabelContainer.Padding().Right,
		nodeContext.SurroundingPortMargins.Right,
		nodeContext.NodeLabelSpacing)

	// Obtain a rectangle strip overlap remover, which will actually do most
	// of the work
	overlapRemover := overlaps.CreateForDirection(overlapRemovalDirection).
		WithGap(nodeContext.PortLabelSpacingHorizontal, nodeContext.PortLabelSpacingVertical)

	// Iterate over our ports and add rectangles to the overlap remover.
	// Also, calculate the start coordinate
	startCoordinate := math.Inf(1)
	if portSide == core.PortSideNorth {
		startCoordinate = math.Inf(-1)
	}

	for _, portContext := range portContexts {
		if portContext.PortLabelCell == nil || !portContext.PortLabelCell.HasLabels() {
			continue
		}

		portSize := portContext.Port.Size()
		portPosition := portContext.PortPosition
		portLabelCell := portContext.PortLabelCell
		portLabelCellRect := portLabelCell.CellRectangle()

		// Setup the less interesting cell properties
		portLabelCellRect.Width = portLabelCell.MinimumWidth()
		portLabelCellRect.Height = portLabelCell.MinimumHeight()

		portLabelCell.SetVerticalAlignment(verticalLabelAlignment)
		portLabelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentRight)

		// Center the label, but make sure it doesn't hang over the node
		// boundaries
		centerPortLabel(portLabelCellRect, portPosition, portSize, leftBorder, rightBorder)

		// Add the rectangle to the overlap remover
		overlapRemover.AddRectangle(portLabelCellRect)

		// Update start coordinate
		if portSide == core.PortSideNorth {
			startCoordinate = max(startCoordinate, portPosition.Y+portContext.Port.Size().Y)
		} else {
			startCoordinate = min(startCoordinate, portPosition.Y)
		}
	}

	// The start coordinate needs to be offset by the port-label space
	if portSide == core.PortSideNorth {
		startCoordinate += nodeContext.PortLabelSpacingVertical
	} else {
		startCoordinate += -nodeContext.PortLabelSpacingVertical
	}

	// Invoke the overlap remover
	stripHeight := overlapRemover.
		WithStartCoordinate(startCoordinate).
		RemoveOverlaps()

	if stripHeight > 0 {
		nodeContext.InsidePortLabelCells[portSide].MinimumContentAreaSize().Y = stripHeight
	}

	// We need to update the label cell's coordinates to be relative to the
	// ports
	for _, portContext := range portContexts {
		if portContext.PortLabelCell == nil || !portContext.PortLabelCell.HasLabels() {
			continue
		}

		portLabelCellRect := portContext.PortLabelCell.CellRectangle()

		// Setup the label cell's cell rectangle
		portLabelCellRect.X -= portContext.PortPosition.X
		portLabelCellRect.Y -= portContext.PortPosition.Y
	}
}

// elkMathMaxd is ElkMath.maxd(values...): the largest value. The maximum
// starts at the lowest finite double, as in the compiled engine.
//
// TODO(core): move to emath.
func elkMathMaxd(values ...float64) float64 {
	maximum := -math.MaxFloat64
	for _, v := range values {
		if v > maximum {
			maximum = v
		}
	}
	return maximum
}

// centerPortLabel centers the label under its port, and tries to keep it
// between the minimum and the maximum x coordinate. The label position is
// absolute, not relative to the port.
func centerPortLabel(portLabelCellRect *emath.ElkRectangle, portPosition, portSize *emath.KVector, minX, maxX float64) {
	// Center the label
	portLabelCellRect.X = portPosition.X - float64((portLabelCellRect.Width-portSize.X)/2)

	// Make sure that the label won't slide past the port
	actualMinX := min(minX, portPosition.X)
	actualMaxX := max(maxX, portPosition.X+portSize.X)

	// Make sure that the label stays inside the boundaries, but only correct
	// in one of the two possible directions
	if portLabelCellRect.X < actualMinX {
		portLabelCellRect.X = actualMinX
	} else if portLabelCellRect.X+portLabelCellRect.Width > actualMaxX {
		portLabelCellRect.X = actualMaxX - portLabelCellRect.Width
	}
}

// simpleOutsidePortLabelPlacement places the port label cells outside of the
// node.
func simpleOutsidePortLabelPlacement(nodeContext *internal.NodeContext, portSide core.PortSide) {
	portContexts := nodeContext.PortContexts.Get(portSide)

	// If there are only two ports on a side, we place the first port's label
	// on its other side to make it especially clear which port it belongs
	// to. The same applies if the user requested space-efficient mode
	placeFirstPortDifferently := IsFirstOutsidePortLabelPlacedDifferently(nodeContext, portSide)

	alwaysAbove := nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementAlwaysOtherSameSide)

	for _, portContext := range portContexts {
		// If the port doesn't have labels, skip
		if portContext.PortLabelCell == nil || !portContext.PortLabelCell.HasLabels() {
			continue
		}

		// Retrieve information about the port itself
		portSize := portContext.Port.Size()

		// Retrieve the label cell and its rectangle and set the rectangle's
		// size (we will use the rectangle to place the cell relative to the
		// port below)
		portLabelCell := portContext.PortLabelCell
		portLabelCellRect := portLabelCell.CellRectangle()
		portLabelCellRect.Width = portLabelCell.MinimumWidth()
		portLabelCellRect.Height = portLabelCell.MinimumHeight()

		// Calculate the position of the port's label space
		switch portSide {
		case core.PortSideNorth:
			if portContext.LabelsNextToPort {
				portLabelCellRect.X = (portSize.X - portLabelCellRect.Width) / 2
				portLabelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentCenter)
			} else if placeFirstPortDifferently || alwaysAbove {
				portLabelCellRect.X = -portLabelCellRect.Width - nodeContext.PortLabelSpacingHorizontal
				portLabelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentRight)
			} else {
				portLabelCellRect.X = portSize.X + nodeContext.PortLabelSpacingHorizontal
				portLabelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentLeft)
			}
			portLabelCellRect.Y = -portLabelCellRect.Height - nodeContext.PortLabelSpacingVertical
			portLabelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentBottom)

		case core.PortSideSouth:
			if portContext.LabelsNextToPort {
				portLabelCellRect.X = (portSize.X - portLabelCellRect.Width) / 2
				portLabelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentCenter)
			} else if placeFirstPortDifferently || alwaysAbove {
				portLabelCellRect.X = -portLabelCellRect.Width - nodeContext.PortLabelSpacingHorizontal
				portLabelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentRight)
			} else {
				portLabelCellRect.X = portSize.X + nodeContext.PortLabelSpacingHorizontal
				portLabelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentLeft)
			}
			portLabelCellRect.Y = portSize.Y + nodeContext.PortLabelSpacingVertical
			portLabelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentTop)

		case core.PortSideEast:
			if portContext.LabelsNextToPort {
				labelHeight := portLabelCellRect.Height
				if !nodeContext.PortLabelsTreatAsGroup {
					labelHeight = portLabelCell.Labels()[0].Size().Y
				}
				portLabelCellRect.Y = (portSize.Y - labelHeight) / 2
				portLabelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentCenter)
			} else if placeFirstPortDifferently || alwaysAbove {
				portLabelCellRect.Y = -portLabelCellRect.Height - nodeContext.PortLabelSpacingVertical
				portLabelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentBottom)
			} else {
				portLabelCellRect.Y = portSize.Y + nodeContext.PortLabelSpacingVertical
				portLabelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentTop)
			}
			portLabelCellRect.X = portSize.X + nodeContext.PortLabelSpacingHorizontal
			portLabelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentLeft)

		case core.PortSideWest:
			if portContext.LabelsNextToPort {
				labelHeight := portLabelCellRect.Height
				if !nodeContext.PortLabelsTreatAsGroup {
					labelHeight = portLabelCell.Labels()[0].Size().Y
				}
				portLabelCellRect.Y = (portSize.Y - labelHeight) / 2
				portLabelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentCenter)
			} else if placeFirstPortDifferently || alwaysAbove {
				portLabelCellRect.Y = -portLabelCellRect.Height - nodeContext.PortLabelSpacingVertical
				portLabelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentBottom)
			} else {
				portLabelCellRect.Y = portSize.Y + nodeContext.PortLabelSpacingVertical
				portLabelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentTop)
			}
			portLabelCellRect.X = -portLabelCellRect.Width - nodeContext.PortLabelSpacingHorizontal
			portLabelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentRight)
		}

		// The next port definitely doesn't have special needs anymore
		placeFirstPortDifferently = false
	}
}

// constrainedOutsidePortLabelPlacement places the port label cells outside
// of the node when the space may be too small to place them without
// overlaps.
func constrainedOutsidePortLabelPlacement(nodeContext *internal.NodeContext, portSide core.PortSide) {
	portContexts := nodeContext.PortContexts.Get(portSide)

	// If there are at most two ports on this port side, or if it's neither
	// the northern nor the southern side, simply revert to simple port label
	// placement
	if len(portContexts) <= 2 || portSide == core.PortSideEast || portSide == core.PortSideWest {
		simpleOutsidePortLabelPlacement(nodeContext, portSide)
		return
	}

	// If space-efficient port labels are active, the leftmost / topmost
	// port's label must be placed to its left / above it
	portWithSpecialNeeds := nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementSpaceEfficient)

	// Prepare things
	overlapRemovalDirection := overlaps.OverlapRemovalDirectionDown
	verticalLabelAlignment := cellsystem.VerticalLabelAlignmentTop
	if portSide == core.PortSideNorth {
		overlapRemovalDirection = overlaps.OverlapRemovalDirectionUp
		verticalLabelAlignment = cellsystem.VerticalLabelAlignmentBottom
	}

	// Obtain a rectangle strip overlap remover, which will actually do most
	// of the work
	overlapRemover := overlaps.CreateForDirection(overlapRemovalDirection).
		WithGap(nodeContext.PortLabelSpacingVertical, nodeContext.PortLabelSpacingHorizontal)

	// Iterate over our ports and add rectangles to the overlap remover.
	// Also, calculate the start coordinate
	startCoordinate := math.Inf(-1)
	if portSide == core.PortSideNorth {
		startCoordinate = math.Inf(1)
	}

	for _, portContext := range portContexts {
		if portContext.PortLabelCell == nil || !portContext.PortLabelCell.HasLabels() {
			continue
		}

		portSize := portContext.Port.Size()
		portPosition := portContext.PortPosition
		portLabelCell := portContext.PortLabelCell
		portLabelCellRect := portLabelCell.CellRectangle()

		// Setup the label cell's cell rectangle
		portLabelCellRect.Width = portLabelCell.MinimumWidth()
		portLabelCellRect.Height = portLabelCell.MinimumHeight()

		if portWithSpecialNeeds {
			portLabelCellRect.X =
				portPosition.X - portLabelCell.MinimumWidth() - nodeContext.PortLabelSpacingHorizontal
			portWithSpecialNeeds = false
		} else {
			portLabelCellRect.X = portPosition.X + portSize.X + nodeContext.PortLabelSpacingHorizontal
		}

		portLabelCell.SetVerticalAlignment(verticalLabelAlignment)
		portLabelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentRight)

		// Add the rectangle to the overlap remover
		overlapRemover.AddRectangle(portLabelCellRect)

		// Update start coordinate
		if portSide == core.PortSideNorth {
			startCoordinate = min(startCoordinate, portPosition.Y)
		} else {
			startCoordinate = max(startCoordinate, portPosition.Y+portContext.Port.Size().Y)
		}
	}

	// The start coordinate needs to be offset by the port-label space
	if portSide == core.PortSideNorth {
		startCoordinate += -nodeContext.PortLabelSpacingVertical
	} else {
		startCoordinate += nodeContext.PortLabelSpacingVertical
	}

	// Invoke the overlap remover
	overlapRemover.
		WithStartCoordinate(startCoordinate).
		RemoveOverlaps()

	// We need to update the label cell's coordinates to be relative to the
	// ports
	for _, portContext := range portContexts {
		if portContext.PortLabelCell == nil || !portContext.PortLabelCell.HasLabels() {
			continue
		}

		portLabelCellRect := portContext.PortLabelCell.CellRectangle()

		// Setup the label cell's cell rectangle
		portLabelCellRect.X -= portContext.PortPosition.X
		portLabelCellRect.Y -= portContext.PortPosition.Y
	}
}
