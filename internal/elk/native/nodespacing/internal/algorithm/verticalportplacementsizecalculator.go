// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/internal/algorithm/VerticalPortPlacementSizeCalculator.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package algorithm

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/internal"
)

// CalculateVerticalPortPlacementSize is
// VerticalPortPlacementSizeCalculator.calculateVerticalPortPlacementSize(
// nodeContext): the space that the ports of the eastern and the western
// side need. Without fixed port positions it also sets the top and bottom
// paddings of the inside port label cells to the space around the ports.
func CalculateVerticalPortPlacementSize(nodeContext *internal.NodeContext) {
	// We need to calculate the space required by the ports even if ports are
	// not part of the size constraints, since we will use that later to
	// place them
	switch nodeContext.PortConstraints {
	case core.PortConstraintsFixedPos:
		// We don't have any freedom at all, so simply calculate where the
		// bottommost port is on each side
		calculateVerticalNodeSizeRequiredByFixedPosPorts(nodeContext, core.PortSideEast)
		calculateVerticalNodeSizeRequiredByFixedPosPorts(nodeContext, core.PortSideWest)

	case core.PortConstraintsFixedRatio:
		// We can require the node to be large enough to avoid spacing
		// violations with fixed ratio ports
		calculateVerticalNodeSizeRequiredByFixedRatioPorts(nodeContext, core.PortSideEast)
		calculateVerticalNodeSizeRequiredByFixedRatioPorts(nodeContext, core.PortSideWest)

	default:
		// If we are free to place things, make the node large enough to
		// place everything properly
		calculateVerticalNodeSizeRequiredByFreePorts(nodeContext, core.PortSideEast)
		calculateVerticalNodeSizeRequiredByFreePorts(nodeContext, core.PortSideWest)
	}
}

// calculateVerticalNodeSizeRequiredByFixedPosPorts sets up the inside port
// label space of a side for ports with fixed positions.
func calculateVerticalNodeSizeRequiredByFixedPosPorts(nodeContext *internal.NodeContext, portSide core.PortSide) {
	bottommostPortBorder := 0.0

	// Check all ports on the correct side
	for _, portContext := range nodeContext.PortContexts.Get(portSide) {
		bottommostPortBorder = max(
			bottommostPortBorder,
			portContext.PortPosition.Y+portContext.Port.Size().Y)
	}

	// Set the cell size and remove top padding since the cell size itself
	// already includes all the space we need
	cell := nodeContext.InsidePortLabelCells[portSide]
	cell.Padding().Top = 0
	cell.MinimumContentAreaSize().Y = bottommostPortBorder
}

// calculateVerticalNodeSizeRequiredByFixedRatioPorts sets up the inside port
// label space of a side for ports with fixed ratios.
func calculateVerticalNodeSizeRequiredByFixedRatioPorts(nodeContext *internal.NodeContext, portSide core.PortSide) {
	cell := nodeContext.InsidePortLabelCells[portSide]

	// Fetch the port contexts on the given side and abort if there are none
	portContexts := nodeContext.PortContexts.Get(portSide)
	if len(portContexts) == 0 {
		cell.Padding().Top = 0
		cell.Padding().Bottom = 0
		return
	}

	portLabelsInside := nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementInside)
	minHeight := 0.0

	// If port labels are to be respected, we need to calculate the port's
	// margins to do so (we ignore the first port in this process if
	// space-efficient mode is active, port labels are to be placed outside
	// and the port labels are not placed next to the port)
	if nodeContext.SizeConstraints.Contains(core.SizeConstraintPortLabels) {
		setupVerticalPortMargins(nodeContext, portSide)
	}

	// Go over all pairs of consecutive ports
	var previousPortContext *internal.PortContext
	previousPortRatio := 0.0
	previousPortHeight := 0.0

	for _, currentPortContext := range portContexts {
		// Get the next port and find out things about it
		currentPortRatio := adapters.GetProperty(currentPortContext.Port, PortRatioOrPosition)
		currentPortHeight := currentPortContext.Port.Size().Y

		if previousPortContext == nil {
			// This is the first port, so find out how high the node needs to
			// be to respect the top surrounding port margins, if any
			if nodeContext.SurroundingPortMargins != nil && nodeContext.SurroundingPortMargins.Top > 0 {
				minHeight = max(
					minHeight,
					minSizeRequiredToRespectSpacing(
						nodeContext.SurroundingPortMargins.Top+currentPortContext.PortMargin.Top,
						0,
						currentPortRatio))
			}
		} else {
			requiredSpace := previousPortHeight + previousPortContext.PortMargin.Bottom +
				nodeContext.PortPortSpacing + currentPortContext.PortMargin.Top
			minHeight = max(
				minHeight,
				minSizeRequiredToRespectSpacing(
					requiredSpace,
					previousPortRatio,
					currentPortRatio))
		}

		// Our current port is going to be the previous port during the next
		// iteration
		previousPortContext = currentPortContext
		previousPortRatio = currentPortRatio
		previousPortHeight = currentPortHeight
	}

	// if there are bottom surrounding port margins, apply those as well
	if nodeContext.SurroundingPortMargins != nil && nodeContext.SurroundingPortMargins.Bottom > 0 {
		// We're using the port's bare height here because we don't care
		// about label sizes on the bottommost port
		requiredSpace := previousPortHeight + nodeContext.SurroundingPortMargins.Bottom

		// We're only interested in the port's bottom margin if it's label is
		// placed inside
		if portLabelsInside {
			requiredSpace += previousPortContext.PortMargin.Bottom
		}

		minHeight = max(
			minHeight,
			minSizeRequiredToRespectSpacing(
				requiredSpace,
				previousPortRatio,
				1))
	}

	// Set the cell size and remove top padding since the cell size itself
	// already includes all the space we need
	cell.Padding().Top = 0
	cell.MinimumContentAreaSize().Y = minHeight
}

// calculateVerticalNodeSizeRequiredByFreePorts sets up the inside port label
// space of a side for free ports.
func calculateVerticalNodeSizeRequiredByFreePorts(nodeContext *internal.NodeContext, portSide core.PortSide) {
	cell := nodeContext.InsidePortLabelCells[portSide]

	// Handle the common case first: if there are no ports, set everything to
	// zero
	if len(nodeContext.PortContexts.Get(portSide)) == 0 {
		cell.Padding().Top = 0
		cell.Padding().Bottom = 0
		return
	}

	// Set the padding to match the surrounding port space
	cell.Padding().Top = nodeContext.SurroundingPortMargins.Top
	cell.Padding().Bottom = nodeContext.SurroundingPortMargins.Bottom

	// If we are to take labels into account, we need to setup the port
	// margins such that they include the space required for their labels
	if nodeContext.SizeConstraints.Contains(core.SizeConstraintPortLabels) {
		setupVerticalPortMargins(nodeContext, portSide)
	}

	height := portHeightPlusPortPortSpacing(nodeContext, portSide)

	// For distributed port alignment, we need to surround the ports by a
	// port-port spacing on each side
	if nodeContext.GetPortAlignment(portSide) == core.PortAlignmentDistributed {
		height += float64(2 * nodeContext.PortPortSpacing)
	}

	// Set the cell size
	cell.MinimumContentAreaSize().Y = height
}

// setupVerticalPortMargins is setupPortMargins(nodeContext, portSide) of
// VerticalPortPlacementSizeCalculator: it sets the port margins of the ports
// of a side so that they include the space for the labels, with the options
// that change the label placement.
func setupVerticalPortMargins(nodeContext *internal.NodeContext, portSide core.PortSide) {
	portContexts := nodeContext.PortContexts.Get(portSide)
	portLabelsOutside := nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementOutside)
	alwaysSameSide := nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementAlwaysSameSide)
	alwaysSameSideAbove := nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementAlwaysOtherSameSide)
	spaceEfficient := nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementSpaceEfficient)
	uniformPortSpacing := nodeContext.SizeOptions.Contains(core.SizeOptionsUniformPortSpacing)

	spaceEfficientPortLabels := !alwaysSameSide && !alwaysSameSideAbove &&
		(spaceEfficient || len(portContexts) == 2)

	// Set the vertical port margins of all ports according to how their
	// labels will be placed. We'll be modifying the margins soon enough.
	computeVerticalPortMargins(nodeContext, portSide, portLabelsOutside)

	// The topmost and bottommost ports are possibly required
	var topmostPortContext *internal.PortContext
	var bottommostPortContext *internal.PortContext

	// If port labels are placed outside, there's stuff we can do
	if portLabelsOutside {
		// Find topmost and bottommost ports. Each caller has at least one
		// port on the side.
		topmostPortContext = portContexts[0]
		bottommostPortContext = portContexts[len(portContexts)-1]

		// The topmost and bottommost ports don't need their top and bottom
		// margin, respectively
		topmostPortContext.PortMargin.Top = 0
		bottommostPortContext.PortMargin.Bottom = 0

		// If we place port labels space-efficiently and the topmost port
		// doesn't have its label placed right next to it, it doesn't need
		// its bottom margin either since its label will be placed above
		if spaceEfficientPortLabels && !topmostPortContext.LabelsNextToPort {
			topmostPortContext.PortMargin.Bottom = 0
		}
	}

	// If ports are placed uniformly, we reflect that here by equalizing all
	// port margins
	if uniformPortSpacing {
		unifyVerticalPortMargins(portContexts)

		// Uniforming may have reset the topmost port's top and bottommost
		// port's bottom margins
		if portLabelsOutside {
			topmostPortContext.PortMargin.Top = 0
			bottommostPortContext.PortMargin.Bottom = 0
		}
	}
}

// computeVerticalPortMargins sets the vertical margins of the ports of a
// side so that they include the space for their labels.
func computeVerticalPortMargins(nodeContext *internal.NodeContext, portSide core.PortSide, portLabelsOutside bool) {
	for _, portContext := range nodeContext.PortContexts.Get(portSide) {
		labelHeight := 0.0
		if portContext.PortLabelCell != nil {
			labelHeight = portContext.PortLabelCell.MinimumHeight()
		}

		if labelHeight > 0 {
			if portContext.LabelsNextToPort {
				// The label is placed next to the port
				portHeight := portContext.Port.Size().Y
				if labelHeight > portHeight {
					if nodeContext.PortLabelsTreatAsGroup || len(portContext.PortLabelCell.Labels()) == 1 {
						// We are to center all of the labels
						overhang := (labelHeight - portHeight) / 2
						portContext.PortMargin.Top = overhang
						portContext.PortMargin.Bottom = overhang
					} else {
						// Simulate centering the first port label
						firstLabelHeight := portContext.PortLabelCell.Labels()[0].Size().Y
						firstLabelOverhang := float64((firstLabelHeight - portHeight) / 2)
						portContext.PortMargin.Top = max(0, firstLabelOverhang)
						portContext.PortMargin.Bottom = labelHeight - firstLabelOverhang - portHeight
					}
				}
			} else {
				// The label is either placed outside (below the port) or
				// possibly inside, but for a compound node, which means that
				// it is placed below the port as well to keep it from
				// overlapping with inside edges
				portContext.PortMargin.Bottom = nodeContext.PortLabelSpacingVertical + labelHeight
			}
		} else if internal.PortLabelPlacementIsFixed(nodeContext.PortLabelsPlacement) {
			// The fixed port label is not considered with
			// portContext.portLabelCell. Nevertheless, a port margin must be
			// added if necessary.
			labelsBounds := getLabelsBounds(portContext.Port)
			if labelsBounds.Y < 0 {
				// Add the part of the label that is above the port to the
				// top margin
				portContext.PortMargin.Top = -labelsBounds.Y
			}
			if labelsBounds.Y+labelsBounds.Height > portContext.Port.Size().Y {
				// Add the part of the label that is below the port to the
				// bottom margin
				portContext.PortMargin.Bottom = labelsBounds.Y + labelsBounds.Height - portContext.Port.Size().Y
			}
		}
	}
}

// unifyVerticalPortMargins is unifyPortMargins(portContexts) of
// VerticalPortPlacementSizeCalculator: it sets the top and bottom margins of
// each port to the largest margins.
func unifyVerticalPortMargins(portContexts []*internal.PortContext) {
	maxTop := 0.0
	maxBottom := 0.0

	// Find maximum
	for _, portContext := range portContexts {
		maxTop = max(maxTop, portContext.PortMargin.Top)
		maxBottom = max(maxBottom, portContext.PortMargin.Bottom)
	}

	// Apply maximum
	for _, portContext := range portContexts {
		portContext.PortMargin.Top = maxTop
		portContext.PortMargin.Bottom = maxBottom
	}
}

// portHeightPlusPortPortSpacing is the sum of the heights of the ports of a
// side with their margins, and of the port-port spacings between them.
func portHeightPlusPortPortSpacing(nodeContext *internal.NodeContext, portSide core.PortSide) float64 {
	result := 0.0

	portContexts := nodeContext.PortContexts.Get(portSide)
	for i, portContext := range portContexts {
		result += portContext.PortMargin.Top + portContext.Port.Size().Y + portContext.PortMargin.Bottom

		if i+1 < len(portContexts) {
			result += nodeContext.PortPortSpacing
		}
	}

	return result
}
