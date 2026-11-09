// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/internal/algorithm/PortPlacementCalculator.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package algorithm

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/coreopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/internal"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// PortRatioOrPosition is PortPlacementCalculator.PORT_RATIO_OR_POSITION: a
// copy of the internal property of the layered algorithm with the same id.
// For a port with a fixed ratio it is the ratio of its position to the size
// of the node.
var PortRatioOrPosition = prop.NewDefault("portRatioOrPosition", 0.0)

// PlaceHorizontalPorts is PortPlacementCalculator.placeHorizontalPorts(
// nodeContext): it places the ports of the northern and the southern side.
func PlaceHorizontalPorts(nodeContext *internal.NodeContext) {
	// How we are going to place the ports depends on their constraints
	switch nodeContext.PortConstraints {
	case core.PortConstraintsFixedPos:
		placeHorizontalFixedPosPorts(nodeContext, core.PortSideNorth)
		placeHorizontalFixedPosPorts(nodeContext, core.PortSideSouth)

	case core.PortConstraintsFixedRatio:
		placeHorizontalFixedRatioPorts(nodeContext, core.PortSideNorth)
		placeHorizontalFixedRatioPorts(nodeContext, core.PortSideSouth)

	default:
		placeHorizontalFreePorts(nodeContext, core.PortSideNorth)
		placeHorizontalFreePorts(nodeContext, core.PortSideSouth)
	}
}

// placeHorizontalFixedPosPorts places the ports of the northern or the
// southern side for FIXED_POS.
func placeHorizontalFixedPosPorts(nodeContext *internal.NodeContext, portSide core.PortSide) {
	for _, portContext := range nodeContext.PortContexts.Get(portSide) {
		// The port's x coordinate is already fixed anyway, so simply adjust
		// its y coordinate according to its offset, if any
		portContext.PortPosition.Y = calculateHorizontalPortYCoordinate(portContext)
	}
}

// placeHorizontalFixedRatioPorts places the ports of the northern or the
// southern side for FIXED_RATIO.
func placeHorizontalFixedRatioPorts(nodeContext *internal.NodeContext, portSide core.PortSide) {
	nodeWidth := nodeContext.NodeSize.X

	for _, portContext := range nodeContext.PortContexts.Get(portSide) {
		// The x coordinate is a function of the node's width and the port's
		// position ratio
		portContext.PortPosition.X = nodeWidth * adapters.GetProperty(portContext.Port, PortRatioOrPosition)
		portContext.PortPosition.Y = calculateHorizontalPortYCoordinate(portContext)
	}
}

// placeHorizontalFreePorts places the ports of the northern or the southern
// side for the port constraints that leave the positions free.
func placeHorizontalFreePorts(nodeContext *internal.NodeContext, portSide core.PortSide) {
	// If there are no ports on the given side, abort
	portContexts := nodeContext.PortContexts.Get(portSide)
	if len(portContexts) == 0 {
		return
	}

	// Retrieve the proper inside port label cell, which will give us hints
	// as to where to place our ports
	insidePortLabelCell := nodeContext.InsidePortLabelCells[portSide]
	insidePortLabelCellRectangle := insidePortLabelCell.CellRectangle()
	insidePortLabelCellPadding := insidePortLabelCell.Padding()

	// Note that we don't have to distinguish any cases here because the port
	// margins already include space required for labels, if such space is to
	// be reserved. Yay!
	portAlignment := nodeContext.GetPortAlignment(portSide)
	availableSpace := insidePortLabelCellRectangle.Width - insidePortLabelCellPadding.Left -
		insidePortLabelCellPadding.Right
	calculatedPortPlacementWidth := insidePortLabelCell.MinimumContentAreaSize().X
	currentXPos := insidePortLabelCellRectangle.X + insidePortLabelCellPadding.Left
	spaceBetweenPorts := nodeContext.PortPortSpacing

	// If the port alignment is distributed or justified, but there's only a
	// single port, we change the alignment to center to keep things from
	// looking stupid
	if (portAlignment == core.PortAlignmentDistributed || portAlignment == core.PortAlignmentJustified) &&
		len(portContexts) == 1 {

		calculatedPortPlacementWidth = modifiedPortPlacementSize(
			nodeContext, portAlignment, calculatedPortPlacementWidth)
		portAlignment = core.PortAlignmentCenter
	}

	if availableSpace < calculatedPortPlacementWidth &&
		!nodeContext.SizeOptions.Contains(core.SizeOptionsPortsOverhang) {

		// There is not enough space available for the ports, but they are
		// not allowed to overhang either. Reduce the space between them to
		// cram them into the available space.
		if portAlignment == core.PortAlignmentDistributed {
			spaceBetweenPorts += (availableSpace - calculatedPortPlacementWidth) /
				float64(len(portContexts)+1)
			currentXPos += spaceBetweenPorts
		} else {
			spaceBetweenPorts += (availableSpace - calculatedPortPlacementWidth) /
				float64(len(portContexts)-1)
		}
	} else {
		// We are allowed to overhang. Yay. However, if we use distributed or
		// justified alignment, this is another case where we should fall
		// back to centered alignment
		if availableSpace < calculatedPortPlacementWidth {
			calculatedPortPlacementWidth = modifiedPortPlacementSize(
				nodeContext, portAlignment, calculatedPortPlacementWidth)
			portAlignment = core.PortAlignmentCenter
		}

		// Calculate where we need to start placing ports (note that the node
		// size required by the port placement includes left and right
		// surrounding port margins, which changes the formulas a bit from
		// what you'd otherwise expect)
		switch portAlignment {
		case core.PortAlignmentBegin:
			// There's nothing to do here

		case core.PortAlignmentCenter:
			currentXPos += float64((availableSpace - calculatedPortPlacementWidth) / 2)

		case core.PortAlignmentEnd:
			currentXPos += availableSpace - calculatedPortPlacementWidth

		case core.PortAlignmentDistributed:
			// In this case, if there is not enough space available to place
			// the ports, we are allowed to overhang. We thus need to ensure
			// that we're only ever increasing the port spacing here
			additionalSpaceBetweenPorts := (availableSpace - calculatedPortPlacementWidth) /
				float64(len(portContexts)+1)
			spaceBetweenPorts += max(0, additionalSpaceBetweenPorts)
			currentXPos += spaceBetweenPorts

		case core.PortAlignmentJustified:
			// In this case, if there is not enough space available to place
			// the ports, we are allowed to overhang. We thus need to ensure
			// that we're only ever increasing the port spacing here
			additionalSpaceBetweenPorts := (availableSpace - calculatedPortPlacementWidth) /
				float64(len(portContexts)-1)
			spaceBetweenPorts += max(0, additionalSpaceBetweenPorts)
		}
	}

	// Iterate over all ports and place them
	for _, portContext := range portContexts {
		portContext.PortPosition.X = currentXPos + portContext.PortMargin.Left
		portContext.PortPosition.Y = calculateHorizontalPortYCoordinate(portContext)

		// Update the x coordinate for the next port
		currentXPos += portContext.PortMargin.Left +
			portContext.Port.Size().X +
			portContext.PortMargin.Right +
			spaceBetweenPorts
	}
}

// calculateHorizontalPortYCoordinate returns the y coordinate of a port of
// the northern or the southern side.
func calculateHorizontalPortYCoordinate(portContext *internal.PortContext) float64 {
	port := portContext.Port

	// The y coordinate is set according to the port's offset, if any
	if adapters.HasProperty(port, coreopts.PortBorderOffset) {
		if port.Side() == core.PortSideNorth {
			return -port.Size().Y - adapters.GetProperty(port, coreopts.PortBorderOffset)
		}
		return adapters.GetProperty(port, coreopts.PortBorderOffset)
	}
	if port.Side() == core.PortSideNorth {
		return -port.Size().Y
	}
	return 0
}

// PlaceVerticalPorts is PortPlacementCalculator.placeVerticalPorts(
// nodeContext): it places the ports of the eastern and the western side.
func PlaceVerticalPorts(nodeContext *internal.NodeContext) {
	// How we are going to place the ports depends on their constraints
	switch nodeContext.PortConstraints {
	case core.PortConstraintsFixedPos:
		placeVerticalFixedPosPorts(nodeContext, core.PortSideEast)
		placeVerticalFixedPosPorts(nodeContext, core.PortSideWest)

	case core.PortConstraintsFixedRatio:
		placeVerticalFixedRatioPorts(nodeContext, core.PortSideEast)
		placeVerticalFixedRatioPorts(nodeContext, core.PortSideWest)

	default:
		placeVerticalFreePorts(nodeContext, core.PortSideEast)
		placeVerticalFreePorts(nodeContext, core.PortSideWest)
	}
}

// placeVerticalFixedPosPorts places the ports of the eastern or the western
// side for FIXED_POS.
func placeVerticalFixedPosPorts(nodeContext *internal.NodeContext, portSide core.PortSide) {
	nodeWidth := nodeContext.NodeSize.X

	for _, portContext := range nodeContext.PortContexts.Get(portSide) {
		// The port's y coordinate is already fixed anyway, so simply adjust
		// its x coordinate according to its offset, if any
		portContext.PortPosition.X = calculateVerticalPortXCoordinate(portContext, nodeWidth)
	}
}

// placeVerticalFixedRatioPorts places the ports of the eastern or the
// western side for FIXED_RATIO.
func placeVerticalFixedRatioPorts(nodeContext *internal.NodeContext, portSide core.PortSide) {
	nodeSize := nodeContext.NodeSize

	for _, portContext := range nodeContext.PortContexts.Get(portSide) {
		// The y coordinate is a function of the node's height and the port's
		// position ratio
		portContext.PortPosition.X = calculateVerticalPortXCoordinate(portContext, nodeSize.X)
		portContext.PortPosition.Y = nodeSize.Y * adapters.GetProperty(portContext.Port, PortRatioOrPosition)
	}
}

// placeVerticalFreePorts places the ports of the eastern or the western side
// for the port constraints that leave the positions free.
func placeVerticalFreePorts(nodeContext *internal.NodeContext, portSide core.PortSide) {
	// If there are no ports on the given side, abort
	portContexts := nodeContext.PortContexts.Get(portSide)
	if len(portContexts) == 0 {
		return
	}

	// Retrieve the proper inside port label cell, which will give us hints
	// as to where to place our ports
	insidePortLabelCell := nodeContext.InsidePortLabelCells[portSide]
	insidePortLabelCellRectangle := insidePortLabelCell.CellRectangle()
	insidePortLabelCellPadding := insidePortLabelCell.Padding()

	// Note that we don't have to distinguish any cases here because the port
	// margins already include space required for labels, if such space is to
	// be reserved. Yay!
	portAlignment := nodeContext.GetPortAlignment(portSide)
	availableSpace := insidePortLabelCellRectangle.Height - insidePortLabelCellPadding.Top -
		insidePortLabelCellPadding.Bottom
	calculatedPortPlacementHeight := insidePortLabelCell.MinimumContentAreaSize().Y
	currentYPos := insidePortLabelCellRectangle.Y + insidePortLabelCellPadding.Top
	spaceBetweenPorts := nodeContext.PortPortSpacing
	nodeWidth := nodeContext.NodeSize.X

	// If the port alignment is distributed or justified, but there's only a
	// single port, we change the alignment to center to keep things from
	// looking stupid
	if (portAlignment == core.PortAlignmentDistributed || portAlignment == core.PortAlignmentJustified) &&
		len(portContexts) == 1 {

		calculatedPortPlacementHeight = modifiedPortPlacementSize(
			nodeContext, portAlignment, calculatedPortPlacementHeight)
		portAlignment = core.PortAlignmentCenter
	}

	if availableSpace < calculatedPortPlacementHeight &&
		!nodeContext.SizeOptions.Contains(core.SizeOptionsPortsOverhang) {

		// There is not enough space available for the ports, but they are
		// not allowed to overhang either. Reduce the space between them to
		// cram them into the available space.
		if portAlignment == core.PortAlignmentDistributed {
			spaceBetweenPorts += (availableSpace - calculatedPortPlacementHeight) /
				float64(len(portContexts)+1)
			currentYPos += spaceBetweenPorts
		} else {
			spaceBetweenPorts += (availableSpace - calculatedPortPlacementHeight) /
				float64(len(portContexts)-1)
		}
	} else {
		// We are allowed to overhang. Yay. However, if we use distributed or
		// justified alignment, this is another case where we should fall
		// back to centered alignment
		if availableSpace < calculatedPortPlacementHeight {
			calculatedPortPlacementHeight = modifiedPortPlacementSize(
				nodeContext, portAlignment, calculatedPortPlacementHeight)
			portAlignment = core.PortAlignmentCenter
		}

		// Calculate where we need to start placing ports (note that the node
		// size required by the port placement includes top and bottom
		// surrounding port margins, which changes the formulas a bit from
		// what you'd otherwise expect)
		switch portAlignment {
		case core.PortAlignmentBegin:
			// There's nothing to do here

		case core.PortAlignmentCenter:
			currentYPos += float64((availableSpace - calculatedPortPlacementHeight) / 2)

		case core.PortAlignmentEnd:
			currentYPos += availableSpace - calculatedPortPlacementHeight

		case core.PortAlignmentDistributed:
			// In this case, if there is not enough space available to place
			// the ports, we are allowed to overhang. We thus need to ensure
			// that we're only ever increasing the port spacing here
			additionalSpaceBetweenPorts := (availableSpace - calculatedPortPlacementHeight) /
				float64(len(portContexts)+1)
			spaceBetweenPorts += max(0, additionalSpaceBetweenPorts)
			currentYPos += spaceBetweenPorts

		case core.PortAlignmentJustified:
			// In this case, if there is not enough space available to place
			// the ports, we are allowed to overhang. We thus need to ensure
			// that we're only ever increasing the port spacing here
			additionalSpaceBetweenPorts := (availableSpace - calculatedPortPlacementHeight) /
				float64(len(portContexts)-1)
			spaceBetweenPorts += max(0, additionalSpaceBetweenPorts)
		}
	}

	// Iterate over all ports and place them
	for _, portContext := range portContexts {
		portContext.PortPosition.X = calculateVerticalPortXCoordinate(portContext, nodeWidth)
		portContext.PortPosition.Y = currentYPos + portContext.PortMargin.Top

		// Update the y coordinate for the next port
		currentYPos += portContext.PortMargin.Top +
			portContext.Port.Size().Y +
			portContext.PortMargin.Bottom +
			spaceBetweenPorts
	}
}

// calculateVerticalPortXCoordinate returns the x coordinate of a port of the
// eastern or the western side.
func calculateVerticalPortXCoordinate(portContext *internal.PortContext, nodeWidth float64) float64 {
	port := portContext.Port

	// The x coordinate is set according to the port's offset, if any
	if adapters.HasProperty(port, coreopts.PortBorderOffset) {
		if port.Side() == core.PortSideWest {
			return -port.Size().X - adapters.GetProperty(port, coreopts.PortBorderOffset)
		}
		return nodeWidth + adapters.GetProperty(port, coreopts.PortBorderOffset)
	}
	if port.Side() == core.PortSideWest {
		return -port.Size().X
	}
	return nodeWidth
}

// modifiedPortPlacementSize returns the port placement size for a change
// from the distributed or the justified alignment to the centered
// alignment. The size for the distributed alignment has two port-port
// spacings that the centered alignment does not need.
func modifiedPortPlacementSize(nodeContext *internal.NodeContext, oldPortAlignment core.PortAlignment,
	currentPortPlacementSize float64) float64 {

	if oldPortAlignment == core.PortAlignmentDistributed {
		return currentPortPlacementSize - float64(2*nodeContext.PortPortSpacing)
	}
	return currentPortPlacementSize
}
