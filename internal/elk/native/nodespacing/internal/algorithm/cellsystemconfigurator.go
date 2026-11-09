// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/internal/algorithm/CellSystemConfigurator.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package algorithm has the steps of the node size calculation of ELK: the
// creation of the cells, the size of the node, and the placement of the
// ports and of the labels.
package algorithm

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/internal"
)

// ConfigureCellSystemSizeContributions is
// CellSystemConfigurator.configureCellSystemSizeContributions(nodeContext):
// it sets the contribution flags of the cells from the size constraints, so
// that the cell system gives the space that the node needs.
func ConfigureCellSystemSizeContributions(nodeContext *internal.NodeContext) {
	// If the node has a fixed size, we don't need to change anything because
	// the cell system won't be used to calculate the node's size
	if nodeContext.SizeConstraints.IsEmpty() {
		return
	}

	// Go through the different size constraint components
	if nodeContext.SizeConstraints.Contains(core.SizeConstraintPorts) {
		// The northern and southern inside port label cells have the correct
		// width for the node
		nodeContext.InsidePortLabelCells[core.PortSideNorth].SetContributesToMinimumWidth(true)
		nodeContext.InsidePortLabelCells[core.PortSideSouth].SetContributesToMinimumWidth(true)

		// For the eastern and western cells, they only give a correct height
		// if port placement is free instead of constrained (the constrained
		// case is handled separately in the node size calculator). This is
		// due to the fact that in the cell layout, the north and south cells
		// are above and below the east and west cells, which would cause the
		// node to become too high
		freePortPlacement := nodeContext.PortConstraints != core.PortConstraintsFixedRatio &&
			nodeContext.PortConstraints != core.PortConstraintsFixedPos
		nodeContext.InsidePortLabelCells[core.PortSideEast].SetContributesToMinimumHeight(freePortPlacement)
		nodeContext.InsidePortLabelCells[core.PortSideWest].SetContributesToMinimumHeight(freePortPlacement)

		// The main row needs to contribute height for the east and west port
		// label cells to be able to contribute their height
		nodeContext.NodeContainerMiddleRow.SetContributesToMinimumHeight(freePortPlacement)

		// Port labels only contribute their size if ports are accounted for
		// as well
		if nodeContext.SizeConstraints.Contains(core.SizeConstraintPortLabels) {
			// The port label cells contribute the space they need for inside
			// port label placement
			nodeContext.InsidePortLabelCells[core.PortSideNorth].SetContributesToMinimumHeight(true)
			nodeContext.InsidePortLabelCells[core.PortSideSouth].SetContributesToMinimumHeight(true)
			nodeContext.InsidePortLabelCells[core.PortSideEast].SetContributesToMinimumWidth(true)
			nodeContext.InsidePortLabelCells[core.PortSideWest].SetContributesToMinimumWidth(true)

			// The main row needs to contribute width for the east and west
			// port label cells to be able to contribute their width
			nodeContext.NodeContainerMiddleRow.SetContributesToMinimumWidth(true)
		}
	}

	if nodeContext.SizeConstraints.Contains(core.SizeConstraintNodeLabels) {
		// The inside node label cell needs to contribute both width and
		// height, as needs the middle row
		nodeContext.InsideNodeLabelContainer.SetContributesToMinimumHeight(true)
		nodeContext.InsideNodeLabelContainer.SetContributesToMinimumWidth(true)
		nodeContext.NodeContainerMiddleRow.SetContributesToMinimumHeight(true)
		nodeContext.NodeContainerMiddleRow.SetContributesToMinimumWidth(true)

		// All node label cells need to contribute height and width, but
		// outside node labels only do so unless they are configured to
		// overhang
		overhang := nodeContext.SizeOptions.Contains(core.SizeOptionsOutsideNodeLabelsOverhang)
		for _, location := range internal.ValuesNodeLabelLocation() {
			labelCell := nodeContext.NodeLabelCells[location]
			if labelCell != nil {
				if location.IsInsideLocation() {
					labelCell.SetContributesToMinimumHeight(true)
					labelCell.SetContributesToMinimumWidth(true)
				} else {
					labelCell.SetContributesToMinimumHeight(!overhang)
					labelCell.SetContributesToMinimumWidth(!overhang)
				}
			}
		}
	}

	// If the middle cell contributes to the node size, we need to set that
	// up as well
	if nodeContext.SizeConstraints.Contains(core.SizeConstraintMinimumSize) &&
		nodeContext.SizeOptions.Contains(core.SizeOptionsMinimumSizeAccountsForPadding) {

		// The middle row now needs to contribute height. The Java code sets
		// the height flag two times and does not set the width flag.
		nodeContext.NodeContainerMiddleRow.SetContributesToMinimumHeight(true)
		nodeContext.NodeContainerMiddleRow.SetContributesToMinimumHeight(true)

		// If the inside node label container is not already contributing to
		// the minimum height and width, node labels are not to be regarded.
		// In that case, turn size contributions on, but limit them to the
		// node label container's center cell
		if !nodeContext.InsideNodeLabelContainer.IsContributingToMinimumHeight() {
			nodeContext.InsideNodeLabelContainer.SetContributesToMinimumHeight(true)
			nodeContext.InsideNodeLabelContainer.SetContributesToMinimumWidth(true)
			nodeContext.InsideNodeLabelContainer.SetOnlyCenterCellContributesToMinimumSize(true)
		}
	}
}

// UpdateVerticalInsidePortLabelCellPadding is
// CellSystemConfigurator.updateVerticalInsidePortLabelCellPadding(
// nodeContext). For free port placement it sets the top and bottom paddings
// of the east and west inside port label cells, so that the port placement
// starts below the northern and above the southern inside port label space
// and still respects the surrounding port margins.
func UpdateVerticalInsidePortLabelCellPadding(nodeContext *internal.NodeContext) {
	// We only care for the free port placement case
	if nodeContext.PortConstraints == core.PortConstraintsFixedRatio ||
		nodeContext.PortConstraints == core.PortConstraintsFixedPos {
		return
	}

	// Calculate where the east and west port cells will end up
	topBorderOffset := nodeContext.NodeContainer.Padding().Top +
		nodeContext.InsidePortLabelCells[core.PortSideNorth].MinimumHeight() +
		nodeContext.LabelCellSpacing
	bottomBorderOffset := nodeContext.NodeContainer.Padding().Bottom +
		nodeContext.InsidePortLabelCells[core.PortSideSouth].MinimumHeight() +
		nodeContext.LabelCellSpacing

	eastCell := nodeContext.InsidePortLabelCells[core.PortSideEast]
	westCell := nodeContext.InsidePortLabelCells[core.PortSideWest]

	// Calculate how much top padding we actually need
	topPadding := max(0, eastCell.Padding().Top-topBorderOffset)
	topPadding = max(topPadding, westCell.Padding().Top-topBorderOffset)
	bottomPadding := max(0, eastCell.Padding().Bottom-bottomBorderOffset)
	bottomPadding = max(bottomPadding, westCell.Padding().Bottom-bottomBorderOffset)

	// Update paddings
	eastCell.Padding().Top = topPadding
	westCell.Padding().Top = topPadding
	eastCell.Padding().Bottom = bottomPadding
	westCell.Padding().Bottom = bottomPadding
}
