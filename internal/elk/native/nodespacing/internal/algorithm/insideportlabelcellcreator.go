// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/internal/algorithm/InsidePortLabelCellCreator.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package algorithm

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/cellsystem"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/internal"
)

// CreateInsidePortLabelCells is
// InsidePortLabelCellCreator.createInsidePortLabelCells(nodeContext): it
// makes the inside port label cells. They exist also without inside port
// labels, because they have the space that the ports need along the node
// borders.
func CreateInsidePortLabelCells(nodeContext *internal.NodeContext) {
	// Create all inside port label cells
	createInsidePortLabelCell(nodeContext, nodeContext.NodeContainer, cellsystem.ContainerAreaBegin, core.PortSideNorth)
	createInsidePortLabelCell(nodeContext, nodeContext.NodeContainer, cellsystem.ContainerAreaEnd, core.PortSideSouth)
	createInsidePortLabelCell(nodeContext, nodeContext.NodeContainerMiddleRow, cellsystem.ContainerAreaBegin, core.PortSideWest)
	createInsidePortLabelCell(nodeContext, nodeContext.NodeContainerMiddleRow, cellsystem.ContainerAreaEnd, core.PortSideEast)

	setupNorthOrSouthPortLabelCell(nodeContext, core.PortSideNorth)
	setupNorthOrSouthPortLabelCell(nodeContext, core.PortSideSouth)
	setupEastOrWestPortLabelCell(nodeContext, core.PortSideEast)
	setupEastOrWestPortLabelCell(nodeContext, core.PortSideWest)
}

// createInsidePortLabelCell makes an atomic cell for inside port labels,
// puts it in the area of the container, and registers it for the port side.
func createInsidePortLabelCell(nodeContext *internal.NodeContext, container *cellsystem.StripContainerCell,
	containerArea cellsystem.ContainerArea, portSide core.PortSide) {

	portLabelCell := cellsystem.NewAtomicCell()
	container.SetCell(containerArea, portLabelCell)
	nodeContext.InsidePortLabelCells[portSide] = portLabelCell
}

// setupNorthOrSouthPortLabelCell sets the paddings of the cell. The height
// is not known before the inside node labels are placed.
func setupNorthOrSouthPortLabelCell(nodeContext *internal.NodeContext, portSide core.PortSide) {
	padding := nodeContext.InsidePortLabelCells[portSide].Padding()

	switch portSide {
	case core.PortSideNorth:
		// In case of negative port spacing, do not use it as a padding since
		// this would increase the node size. Needed to ensure that negative
		// label port spacing behaves the same as positive.
		if nodeContext.PortLabelSpacingVertical >= 0 {
			padding.Top = nodeContext.PortLabelSpacingVertical
		}
	case core.PortSideSouth:
		// In case of negative port spacing, do not use it as a padding since
		// this would increase the node size. Needed to ensure that negative
		// label port spacing behaves the same as positive.
		if nodeContext.PortLabelSpacingVertical >= 0 {
			padding.Bottom = nodeContext.PortLabelSpacingVertical
		}
	}

	if nodeContext.SurroundingPortMargins != nil {
		padding.Left = nodeContext.SurroundingPortMargins.Left
		padding.Right = nodeContext.SurroundingPortMargins.Right
	}
}

// setupEastOrWestPortLabelCell sets the width of the cell for inside port
// labels, and its top and bottom paddings.
func setupEastOrWestPortLabelCell(nodeContext *internal.NodeContext, portSide core.PortSide) {
	if nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementInside) {
		calculateWidthDueToLabels(nodeContext, portSide)
	}

	setupTopAndBottomPadding(nodeContext, portSide)
}

// calculateWidthDueToLabels sets the width of the cell to fit the largest
// label. With a label it also sets the padding that keeps the space between
// the ports and their inside labels.
func calculateWidthDueToLabels(nodeContext *internal.NodeContext, portSide core.PortSide) {
	// Retrieve the appropriate cell
	theAppropriateCell := nodeContext.InsidePortLabelCells[portSide]
	minCellSize := theAppropriateCell.MinimumContentAreaSize()

	for _, portContext := range nodeContext.PortContexts.Get(portSide) {
		// Update the maximum label width
		if portContext.PortLabelCell != nil {
			minCellSize.X = max(minCellSize.X, portContext.PortLabelCell.MinimumWidth())
		}
	}

	// If the cell has a minimum width by now, that means we actually have
	// labels in there. Which, in turn, means that we need to add a padding
	// to the cell to ensure enough space between ports and their inside
	// labels
	if minCellSize.X > 0 {
		switch portSide {
		case core.PortSideEast:
			theAppropriateCell.Padding().Right = nodeContext.PortLabelSpacingHorizontal
		case core.PortSideWest:
			theAppropriateCell.Padding().Left = nodeContext.PortLabelSpacingHorizontal
		}
	}
}

// setupTopAndBottomPadding sets the top and bottom paddings of the cell to
// the surrounding port margins.
func setupTopAndBottomPadding(nodeContext *internal.NodeContext, portSide core.PortSide) {
	if nodeContext.SurroundingPortMargins != nil {
		padding := nodeContext.InsidePortLabelCells[portSide].Padding()
		padding.Top = nodeContext.SurroundingPortMargins.Top
		padding.Bottom = nodeContext.SurroundingPortMargins.Bottom
	}
}
