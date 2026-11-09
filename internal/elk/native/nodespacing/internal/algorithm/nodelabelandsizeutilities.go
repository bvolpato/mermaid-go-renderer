// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/internal/algorithm/NodeLabelAndSizeUtilities.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package algorithm

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/coreopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/internal"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// SetupMinimumClientAreaSize is
// NodeLabelAndSizeUtilities.setupMinimumClientAreaSize(nodeContext): with a
// minimum size of the client area it sets that size on the inside node
// label container. It sets no contribution flag.
func SetupMinimumClientAreaSize(nodeContext *internal.NodeContext) {
	minSize := GetMinimumClientAreaSize(nodeContext)
	if minSize != nil {
		nodeContext.InsideNodeLabelContainer.SetCenterCellMinimumSize(minSize)
	}
}

// SetupNodePaddingForPortsWithOffset is
// NodeLabelAndSizeUtilities.setupNodePaddingForPortsWithOffset(nodeContext):
// it sets the padding of the main node cell for the ports that extend into
// the node.
func SetupNodePaddingForPortsWithOffset(nodeContext *internal.NodeContext) {
	nodeCellPadding := nodeContext.NodeContainer.Padding()

	for _, portContext := range nodeContext.PortContexts.Values() {
		// If the port extends into the node, ensure the inside port space is
		// enough
		portBorderOffset := 0.0

		if adapters.HasProperty(portContext.Port, coreopts.PortBorderOffset) {
			portBorderOffset = adapters.GetProperty(portContext.Port, coreopts.PortBorderOffset)

			if portBorderOffset < 0 {
				// The port does extend into the node, by -portBorderOffset
				switch portContext.Port.Side() {
				case core.PortSideNorth:
					nodeCellPadding.Top = max(nodeCellPadding.Top, -portBorderOffset)
				case core.PortSideSouth:
					nodeCellPadding.Bottom = max(nodeCellPadding.Bottom, -portBorderOffset)
				case core.PortSideEast:
					nodeCellPadding.Right = max(nodeCellPadding.Right, -portBorderOffset)
				case core.PortSideWest:
					nodeCellPadding.Left = max(nodeCellPadding.Left, -portBorderOffset)
				}
			}
		}
		if internal.PortLabelPlacementIsFixed(nodeContext.PortLabelsPlacement) {
			// Ensure that the part of the fixed label, that is inside the
			// node, has enough space.
			insidePart := computeInsidePartOfPort(portContext.Port, portBorderOffset)
			symmetry := !adapters.GetProperty(nodeContext.Node, coreopts.NodeSizeOptions).Contains(core.SizeOptionsAsymmetrical)
			insidePartIsBigger := false
			switch portContext.Port.Side() {
			case core.PortSideNorth:
				insidePartIsBigger = insidePart > nodeCellPadding.Top
				nodeCellPadding.Top = max(nodeCellPadding.Top, insidePart)
				if symmetry && insidePartIsBigger {
					nodeCellPadding.Top = max(nodeCellPadding.Top, nodeCellPadding.Bottom)
					// For symmetry, the portBorderOffset is not considered
					// (only label + label padding)
					nodeCellPadding.Bottom = nodeCellPadding.Top + portBorderOffset
				}
			case core.PortSideSouth:
				insidePartIsBigger = insidePart > nodeCellPadding.Bottom
				nodeCellPadding.Bottom = max(nodeCellPadding.Bottom, insidePart)
				if symmetry && insidePartIsBigger {
					nodeCellPadding.Bottom = max(nodeCellPadding.Bottom, nodeCellPadding.Top)
					// For symmetry, the portBorderOffset is not considered
					// (only label + label padding)
					nodeCellPadding.Top = nodeCellPadding.Bottom + portBorderOffset
				}
			case core.PortSideEast:
				insidePartIsBigger = insidePart > nodeCellPadding.Right
				nodeCellPadding.Right = max(nodeCellPadding.Right, insidePart)
				if symmetry && insidePartIsBigger {
					nodeCellPadding.Right = max(nodeCellPadding.Left, nodeCellPadding.Right)
					// For symmetry, the portBorderOffset is not considered
					// (only label + label padding)
					nodeCellPadding.Left = nodeCellPadding.Right + portBorderOffset
				}
			case core.PortSideWest:
				insidePartIsBigger = insidePart > nodeCellPadding.Left
				nodeCellPadding.Left = max(nodeCellPadding.Left, insidePart)
				if symmetry && insidePartIsBigger {
					nodeCellPadding.Left = max(nodeCellPadding.Left, nodeCellPadding.Right)
					// For symmetry, the portBorderOffset is not considered
					// (only label + label padding)
					nodeCellPadding.Right = nodeCellPadding.Left + portBorderOffset
				}
			}
		}
	}
}

// OffsetSouthernPortsByNodeSize is
// NodeLabelAndSizeUtilities.offsetSouthernPortsByNodeSize(nodeContext): the
// southern ports were placed relative to the y coordinate 0, and this moves
// them to the southern border of the node.
func OffsetSouthernPortsByNodeSize(nodeContext *internal.NodeContext) {
	nodeHeight := nodeContext.NodeSize.Y

	for _, portContext := range nodeContext.PortContexts.Get(core.PortSideSouth) {
		portPosition := portContext.PortPosition
		portPosition.Y += nodeHeight
	}
}

// SetNodePadding is NodeLabelAndSizeUtilities.setNodePadding(nodeContext):
// it computes and sets the padding of the node, if the size options ask for
// it.
func SetNodePadding(nodeContext *internal.NodeContext) {
	if !nodeContext.SizeOptions.Contains(core.SizeOptionsComputePadding) {
		return
	}

	nodeRect := nodeContext.NodeContainer.CellRectangle()
	clientArea := nodeContext.InsideNodeLabelContainer.CenterCellRectangle()

	nodePadding := &emath.ElkPadding{}

	// The following code assumes that the client area rectangle lies fully
	// inside the node rectangle, which should always be the case because of
	// how the client area rectangle is computed
	nodePadding.Left = clientArea.X - nodeRect.X
	nodePadding.Top = clientArea.Y - nodeRect.Y
	nodePadding.Right = (nodeRect.X + nodeRect.Width) - (clientArea.X + clientArea.Width)
	nodePadding.Bottom = (nodeRect.Y + nodeRect.Height) - (clientArea.Y + clientArea.Height)

	nodeContext.Node.SetPadding(nodePadding)
}

// ApplyStuff is NodeLabelAndSizeUtilities.applyStuff(nodeContext): it
// applies the node size to the node and the port positions to the ports.
func ApplyStuff(nodeContext *internal.NodeContext) {
	nodeContext.ApplyNodeSize()
	for _, pc := range nodeContext.PortContexts.Values() {
		pc.ApplyPortPosition()
	}
}

// GetMinimumClientAreaSize is
// NodeLabelAndSizeUtilities.getMinimumClientAreaSize(nodeContext): the
// minimum size of the client area, or nil when the size constraints give
// the client area no minimum size.
func GetMinimumClientAreaSize(nodeContext *internal.NodeContext) *emath.KVector {
	if nodeContext.SizeConstraints.Contains(core.SizeConstraintMinimumSize) &&
		nodeContext.SizeOptions.Contains(core.SizeOptionsMinimumSizeAccountsForPadding) {

		return GetMinimumNodeOrClientAreaSize(nodeContext)
	}
	return nil
}

// GetMinimumNodeSize is NodeLabelAndSizeUtilities.getMinimumNodeSize(
// nodeContext): the minimum size of the node, or nil when the size
// constraints give the node as a whole no minimum size.
func GetMinimumNodeSize(nodeContext *internal.NodeContext) *emath.KVector {
	if nodeContext.SizeConstraints.Contains(core.SizeConstraintMinimumSize) {
		if !nodeContext.SizeOptions.Contains(core.SizeOptionsMinimumSizeAccountsForPadding) {
			return GetMinimumNodeOrClientAreaSize(nodeContext)
		}
	}
	return nil
}

// GetMinimumNodeOrClientAreaSize is
// NodeLabelAndSizeUtilities.getMinimumNodeOrClientAreaSize(nodeContext): the
// minimum size that is set for the node, without regard for the size
// constraints.
func GetMinimumNodeOrClientAreaSize(nodeContext *internal.NodeContext) *emath.KVector {
	// Retrieve the minimum size
	minSize := adapters.GetProperty(nodeContext.Node, coreopts.NodeSizeMinimum).Clone()

	// If we are instructed to revert to a default minimum size, we check
	// whether we need to revert to that
	if nodeContext.SizeOptions.Contains(core.SizeOptionsDefaultMinimumSize) {
		if minSize.X <= 0 {
			minSize.X = elkUtilDefaultMinWidth
		}

		if minSize.Y <= 0 {
			minSize.Y = elkUtilDefaultMinHeight
		}
	}

	return minSize
}

// effectivelyFixedSizeConstraints is EFFECTIVELY_FIXED_SIZE_CONSTRAINTS:
// size constraints that only have the port labels do not resize a node.
var effectivelyFixedSizeConstraints = prop.Of(core.SizeConstraintPortLabels)

// AreSizeConstraintsFixed is
// NodeLabelAndSizeUtilities.areSizeConstraintsFixed(nodeContext): the size
// constraints are empty or have only PORT_LABELS, so the node keeps its
// size.
func AreSizeConstraintsFixed(nodeContext *internal.NodeContext) bool {
	return nodeContext.SizeConstraints.IsEmpty() ||
		nodeContext.SizeConstraints.Equals(effectivelyFixedSizeConstraints)
}

// IsFirstOutsidePortLabelPlacedDifferently is
// NodeLabelAndSizeUtilities.isFirstOutsidePortLabelPlacedDifferently(
// nodeContext, portSide). The label of an outside port is below or to the
// right of the port. The label of the first port of a side is on the other
// side when the port does not have its label next to it, ALWAYS_SAME_SIDE
// is not set, and the side has two ports or the placement is
// SPACE_EFFICIENT.
func IsFirstOutsidePortLabelPlacedDifferently(nodeContext *internal.NodeContext, portSide core.PortSide) bool {
	portContexts := nodeContext.PortContexts.Get(portSide)
	if len(portContexts) >= 2 {
		firstPort := portContexts[0]
		alwaysSameSide := nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementAlwaysSameSide)
		spaceEfficient := nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementSpaceEfficient)
		return !firstPort.LabelsNextToPort && !alwaysSameSide &&
			(len(portContexts) == 2 || spaceEfficient)
	}
	return false
}
