// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/internal/algorithm/HorizontalPortPlacementSizeCalculator.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package algorithm

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/internal"
)

// CalculateHorizontalPortPlacementSize is
// HorizontalPortPlacementSizeCalculator.calculateHorizontalPortPlacementSize(
// nodeContext): the space that the ports of the northern and the southern
// side need. Without fixed port positions it also sets the left and right
// paddings of the inside port label cells to the space around the ports.
func CalculateHorizontalPortPlacementSize(nodeContext *internal.NodeContext) {
	// We need to calculate the space required by the ports even if ports are
	// not part of the size constraints, since we will use that later to
	// place them
	switch nodeContext.PortConstraints {
	case core.PortConstraintsFixedPos:
		// We don't have any freedom at all, so simply calculate where the
		// rightmost port is on each side
		calculateHorizontalNodeSizeRequiredByFixedPosPorts(nodeContext, core.PortSideNorth)
		calculateHorizontalNodeSizeRequiredByFixedPosPorts(nodeContext, core.PortSideSouth)

	case core.PortConstraintsFixedRatio:
		// We can require the node to be large enough to avoid spacing
		// violations with fixed ratio ports
		calculateHorizontalNodeSizeRequiredByFixedRatioPorts(nodeContext, core.PortSideNorth)
		calculateHorizontalNodeSizeRequiredByFixedRatioPorts(nodeContext, core.PortSideSouth)

	default:
		// If we are free to place things, make the node large enough to
		// place everything properly
		calculateHorizontalNodeSizeRequiredByFreePorts(nodeContext, core.PortSideNorth)
		calculateHorizontalNodeSizeRequiredByFreePorts(nodeContext, core.PortSideSouth)
	}
}

// calculateHorizontalNodeSizeRequiredByFixedPosPorts sets up the inside port
// label space of a side for ports with fixed positions.
func calculateHorizontalNodeSizeRequiredByFixedPosPorts(nodeContext *internal.NodeContext, portSide core.PortSide) {
	rightmostPortBorder := 0.0

	// Check all ports on the correct side
	for _, portContext := range nodeContext.PortContexts.Get(portSide) {
		rightmostPortBorder = max(
			rightmostPortBorder,
			portContext.PortPosition.X+portContext.Port.Size().X)
	}

	// Set the cell size and remove left padding since the cell size itself
	// already includes all the space we need
	cell := nodeContext.InsidePortLabelCells[portSide]
	cell.Padding().Left = 0
	cell.MinimumContentAreaSize().X = rightmostPortBorder
}

// calculateHorizontalNodeSizeRequiredByFixedRatioPorts sets up the inside
// port label space of a side for ports with fixed ratios.
func calculateHorizontalNodeSizeRequiredByFixedRatioPorts(nodeContext *internal.NodeContext, portSide core.PortSide) {
	cell := nodeContext.InsidePortLabelCells[portSide]

	// Fetch the port contexts on the given side and abort if there are none
	portContexts := nodeContext.PortContexts.Get(portSide)
	if len(portContexts) == 0 {
		cell.Padding().Left = 0
		cell.Padding().Right = 0
		return
	}

	portLabelsInside := nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementInside)
	minWidth := 0.0

	// Go over all pairs of consecutive ports
	var previousPortContext *internal.PortContext
	previousPortRatio := 0.0
	previousPortWidth := 0.0

	for _, currentPortContext := range portContexts {
		// Get the next port and find out things about it
		currentPortRatio := adapters.GetProperty(currentPortContext.Port, PortRatioOrPosition)
		currentPortWidth := currentPortContext.Port.Size().X

		// If port labels are to be respected, we need to calculate the
		// port's margins to do so (we ignore the first port in this process
		// if space-efficient mode is active and port labels are to be placed
		// outside). The Java code does this for each port of the loop.
		if nodeContext.SizeConstraints.Contains(core.SizeConstraintPortLabels) {
			setupHorizontalPortMargins(nodeContext, portSide)
		}

		if previousPortContext == nil {
			// This is the first port, so find out how wide the node needs to
			// be to respect the left surrounding port margins, if any
			if nodeContext.SurroundingPortMargins != nil && nodeContext.SurroundingPortMargins.Left > 0 {
				minWidth = max(
					minWidth,
					minSizeRequiredToRespectSpacing(
						nodeContext.SurroundingPortMargins.Left+currentPortContext.PortMargin.Left,
						0,
						currentPortRatio))
			}
		} else {
			requiredSpace := previousPortWidth + previousPortContext.PortMargin.Right +
				nodeContext.PortPortSpacing + currentPortContext.PortMargin.Left
			minWidth = max(
				minWidth,
				minSizeRequiredToRespectSpacing(
					requiredSpace,
					previousPortRatio,
					currentPortRatio))
		}

		// Our current port is going to be the previous port during the next
		// iteration
		previousPortContext = currentPortContext
		previousPortRatio = currentPortRatio
		previousPortWidth = currentPortWidth
	}

	// if there are right surrounding port margins, apply those as well
	if nodeContext.SurroundingPortMargins != nil && nodeContext.SurroundingPortMargins.Right > 0 {
		requiredSpace := previousPortWidth + nodeContext.SurroundingPortMargins.Right

		// We're only interested in the port's right margin if it's label is
		// placed inside
		if portLabelsInside {
			requiredSpace += previousPortContext.PortMargin.Right
		}

		minWidth = max(
			minWidth,
			minSizeRequiredToRespectSpacing(
				requiredSpace,
				previousPortRatio,
				1))
	}

	// Set the cell size and remove left padding since the cell size itself
	// already includes all the space we need
	cell.Padding().Left = 0
	cell.MinimumContentAreaSize().X = minWidth
}

// equalityTolerance is EQUALITY_TOLERANCE: the difference up to which two
// ratios are taken to be equal.
const equalityTolerance = 0.01

// minSizeRequiredToRespectSpacing is minSizeRequiredToRespectSpacing(
// spacing, firstRatio, secondRatio): the smallest size for which the
// distance between the two ratios of the size is the spacing. It is 0 for
// ratios that are equal (DoubleMath.fuzzyEquals with EQUALITY_TOLERANCE).
func minSizeRequiredToRespectSpacing(spacing, firstRatio, secondRatio float64) float64 {
	// Some failsafing
	if math.Abs(firstRatio-secondRatio) <= equalityTolerance || firstRatio == secondRatio ||
		firstRatio != firstRatio && secondRatio != secondRatio {
		return 0
	}
	return spacing / (secondRatio - firstRatio)
}

// calculateHorizontalNodeSizeRequiredByFreePorts sets up the inside port
// label space of a side for free ports.
func calculateHorizontalNodeSizeRequiredByFreePorts(nodeContext *internal.NodeContext, portSide core.PortSide) {
	cell := nodeContext.InsidePortLabelCells[portSide]

	// Handle the common case first: if there are no ports, set everything to
	// zero
	if len(nodeContext.PortContexts.Get(portSide)) == 0 {
		cell.Padding().Left = 0
		cell.Padding().Right = 0
		return
	}

	// Set the padding to match the surrounding port space
	cell.Padding().Left = nodeContext.SurroundingPortMargins.Left
	cell.Padding().Right = nodeContext.SurroundingPortMargins.Right

	// If we are to take labels into account, we need to setup the port
	// margins such that they include the space required for their labels
	if nodeContext.SizeConstraints.Contains(core.SizeConstraintPortLabels) {
		setupHorizontalPortMargins(nodeContext, portSide)
	}

	width := portWidthPlusPortPortSpacing(nodeContext, portSide)

	// For distributed port alignment, we need to surround the ports by a
	// port-port spacing on each side
	if nodeContext.GetPortAlignment(portSide) == core.PortAlignmentDistributed {
		width += float64(2 * nodeContext.PortPortSpacing)
	}

	// Set the cell size
	cell.MinimumContentAreaSize().X = width
}

// setupHorizontalPortMargins is setupPortMargins(nodeContext, portSide) of
// HorizontalPortPlacementSizeCalculator: it sets the port margins of the
// ports of a side so that they include the space for the labels, with the
// options that change the label placement.
func setupHorizontalPortMargins(nodeContext *internal.NodeContext, portSide core.PortSide) {
	portContexts := nodeContext.PortContexts.Get(portSide)
	portLabelsOutside := nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementOutside)
	alwaysSameSide := nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementAlwaysSameSide)
	alwaysSameSideAbove := nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementAlwaysOtherSameSide)
	spaceEfficient := nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementSpaceEfficient)
	uniformPortSpacing := nodeContext.SizeOptions.Contains(core.SizeOptionsUniformPortSpacing)

	spaceEfficientPortLabels := !alwaysSameSide && !alwaysSameSideAbove &&
		(spaceEfficient || len(portContexts) == 2)

	// Set the horizontal port margins of all ports according to how their
	// labels will be placed. We'll be modifying the margins soon enough.
	computeHorizontalPortMargins(nodeContext, portSide, portLabelsOutside)

	// The leftmost and rightmost ports are possibly required
	var leftmostPortContext *internal.PortContext
	var rightmostPortContext *internal.PortContext

	// If port labels are placed outside, there's stuff we can do
	if portLabelsOutside {
		// Find leftmost and rightmost ports. Each caller has at least one
		// port on the side.
		leftmostPortContext = portContexts[0]
		rightmostPortContext = portContexts[len(portContexts)-1]

		// The leftmost and rightmost ports don't need their left and right
		// margin, respectively
		leftmostPortContext.PortMargin.Left = 0
		rightmostPortContext.PortMargin.Right = 0

		// If we place port labels space-efficiently and the leftmost port
		// doesn't have its label placed right next to it, it doesn't need
		// its right margin either since its label will be placed to its left
		if spaceEfficientPortLabels && !leftmostPortContext.LabelsNextToPort {
			leftmostPortContext.PortMargin.Right = 0
		}
	}

	// If ports are placed uniformly, we reflect that here by equalizing all
	// port margins
	if uniformPortSpacing {
		unifyHorizontalPortMargins(portContexts)

		// Uniforming may have reset the leftmost port's left and rightmost
		// port's right margins
		if portLabelsOutside {
			leftmostPortContext.PortMargin.Left = 0
			rightmostPortContext.PortMargin.Right = 0
		}
	}
}

// computeHorizontalPortMargins sets the horizontal margins of the ports of a
// side so that they include the space for their labels.
func computeHorizontalPortMargins(nodeContext *internal.NodeContext, portSide core.PortSide, portLabelsOutside bool) {
	for _, portContext := range nodeContext.PortContexts.Get(portSide) {
		labelWidth := 0.0
		if portContext.PortLabelCell != nil {
			labelWidth = portContext.PortLabelCell.MinimumWidth()
		}

		if labelWidth > 0 {
			if portContext.LabelsNextToPort {
				// The label is placed next to the port
				portWidth := portContext.Port.Size().X
				if labelWidth > portWidth {
					overhang := (labelWidth - portWidth) / 2
					portContext.PortMargin.Left = overhang
					portContext.PortMargin.Right = overhang
				}
			} else {
				// The label is either placed outside (right to the port) or
				// possibly inside, but for a compound node, which means that
				// it is placed right of the port as well to keep it from
				// overlapping with inside edges
				portContext.PortMargin.Right = nodeContext.PortLabelSpacingHorizontal + labelWidth
			}
		} else if internal.PortLabelPlacementIsFixed(nodeContext.PortLabelsPlacement) {
			// The fixed port label is not considered with
			// portContext.portLabelCell. Nevertheless, a port margin must be
			// added if necessary.
			labelsBounds := getLabelsBounds(portContext.Port)
			if labelsBounds.X < 0 {
				// Add the part of the label that is on the left of the port
				// to the left margin
				portContext.PortMargin.Left = -labelsBounds.X
			}
			if labelsBounds.X+labelsBounds.Width > portContext.Port.Size().X {
				// Add the part of the label that is on the right of the port
				// to the right margin
				portContext.PortMargin.Right = labelsBounds.X + labelsBounds.Width - portContext.Port.Size().X
			}
		}
	}
}

// unifyHorizontalPortMargins is unifyPortMargins(portContexts) of
// HorizontalPortPlacementSizeCalculator: it sets the left and right margins
// of each port to the largest margins.
func unifyHorizontalPortMargins(portContexts []*internal.PortContext) {
	maxLeft := 0.0
	maxRight := 0.0

	// Find maximum
	for _, portContext := range portContexts {
		maxLeft = max(maxLeft, portContext.PortMargin.Left)
		maxRight = max(maxRight, portContext.PortMargin.Right)
	}

	// Apply maximum
	for _, portContext := range portContexts {
		portContext.PortMargin.Left = maxLeft
		portContext.PortMargin.Right = maxRight
	}
}

// portWidthPlusPortPortSpacing is the sum of the widths of the ports of a
// side with their margins, and of the port-port spacings between them.
func portWidthPlusPortPortSpacing(nodeContext *internal.NodeContext, portSide core.PortSide) float64 {
	result := 0.0

	portContexts := nodeContext.PortContexts.Get(portSide)
	for i, portContext := range portContexts {
		// Add the current port's width to our result
		result += portContext.PortMargin.Left + portContext.Port.Size().X + portContext.PortMargin.Right

		// If there is another port after this one, include the required
		// spacing
		if i+1 < len(portContexts) {
			result += nodeContext.PortPortSpacing
		}
	}

	return result
}
