// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/internal/algorithm/NodeLabelCellCreator.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package algorithm

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/coreopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/cellsystem"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/internal"
)

// CreateNodeLabelCells is NodeLabelCellCreator.createNodeLabelCells(
// nodeContext, onlyInside, horizontalLayoutMode): it makes the cell
// containers and a label cell for each location that has a node label.
// With onlyInside it handles only the inside node labels, to compute the
// space that they need.
func CreateNodeLabelCells(nodeContext *internal.NodeContext, onlyInside, horizontalLayoutMode bool) {
	// Make sure all the relevant containers exist
	createNodeLabelCellContainers(nodeContext, onlyInside)

	// Handle each of the node's labels
	for _, label := range nodeContext.Node.Labels() {
		handleNodeLabel(nodeContext, label, onlyInside, horizontalLayoutMode)
	}
}

// handleNodeLabel adds the label to the label cell of its location. It
// makes the cell when it does not exist.
func handleNodeLabel(nodeContext *internal.NodeContext, label adapters.LabelAdapter, onlyInside, horizontalLayoutMode bool) {
	// Find the effective label location
	labelPlacement := nodeContext.NodeLabelPlacement
	if adapters.HasProperty(label, coreopts.NodeLabelsPlacement) {
		labelPlacement = adapters.GetProperty(label, coreopts.NodeLabelsPlacement)
	}
	labelLocation := internal.NodeLabelLocationFromNodeLabelPlacement(labelPlacement)

	// If the label has its location fixed, we will ignore it
	if labelLocation == internal.NodeLabelLocationUndefined {
		return
	}

	// If the label's location is on the node's outside but we only want
	// inside node labels, we will ignore it
	if onlyInside && !labelLocation.IsInsideLocation() {
		return
	}

	retrieveNodeLabelCell(nodeContext, labelLocation, horizontalLayoutMode).AddLabel(label)
}

// createNodeLabelCellContainers makes the node label containers.
func createNodeLabelCellContainers(nodeContext *internal.NodeContext, onlyInside bool) {
	symmetry := !nodeContext.SizeOptions.Contains(core.SizeOptionsAsymmetrical)
	tabularNodeLabels := nodeContext.SizeOptions.Contains(core.SizeOptionsForceTabularNodeLabels)

	// Inside container
	nodeContext.InsideNodeLabelContainer = cellsystem.NewGridContainerCell(
		tabularNodeLabels, symmetry, nodeContext.LabelCellSpacing)
	if nodeContext.NodeLabelsPadding != nil {
		nodeContext.InsideNodeLabelContainer.Padding().Copy(&nodeContext.NodeLabelsPadding.Spacing)
	}
	nodeContext.NodeContainerMiddleRow.SetCell(cellsystem.ContainerAreaCenter, nodeContext.InsideNodeLabelContainer)

	// Outside containers, if requested
	if !onlyInside {
		northContainer := cellsystem.NewStripContainerCell(
			cellsystem.StripHorizontal, symmetry, nodeContext.LabelCellSpacing)
		northContainer.Padding().Bottom = nodeContext.NodeLabelSpacing
		nodeContext.OutsideNodeLabelContainers[core.PortSideNorth] = northContainer

		southContainer := cellsystem.NewStripContainerCell(
			cellsystem.StripHorizontal, symmetry, nodeContext.LabelCellSpacing)
		southContainer.Padding().Top = nodeContext.NodeLabelSpacing
		nodeContext.OutsideNodeLabelContainers[core.PortSideSouth] = southContainer

		westContainer := cellsystem.NewStripContainerCell(
			cellsystem.StripVertical, symmetry, nodeContext.LabelCellSpacing)
		westContainer.Padding().Right = nodeContext.NodeLabelSpacing
		nodeContext.OutsideNodeLabelContainers[core.PortSideWest] = westContainer

		eastContainer := cellsystem.NewStripContainerCell(
			cellsystem.StripVertical, symmetry, nodeContext.LabelCellSpacing)
		eastContainer.Padding().Left = nodeContext.NodeLabelSpacing
		nodeContext.OutsideNodeLabelContainers[core.PortSideEast] = eastContainer
	}
}

// retrieveNodeLabelCell returns the node label cell of the location. It
// makes the cell and adds it to its container when it does not exist.
func retrieveNodeLabelCell(nodeContext *internal.NodeContext, nodeLabelLocation internal.NodeLabelLocation,
	horizontalLayoutMode bool) *cellsystem.LabelCell {

	nodeLabelCell := nodeContext.NodeLabelCells[nodeLabelLocation]

	if nodeLabelCell == nil {
		// The node label cell doesn't exist yet, so create one and add it to
		// the relevant container
		nodeLabelCell = cellsystem.NewLabelCellLocationMode(nodeContext.LabelLabelSpacing, nodeLabelLocation, horizontalLayoutMode)
		nodeContext.NodeLabelCells[nodeLabelLocation] = nodeLabelCell

		// Find the correct container and add the cell to it
		if nodeLabelLocation.IsInsideLocation() {
			nodeContext.InsideNodeLabelContainer.SetCell(
				nodeLabelLocation.ContainerRow(),
				nodeLabelLocation.ContainerColumn(),
				nodeLabelCell)
		} else {
			outsideSide := nodeLabelLocation.OutsideSide()
			containerCell := nodeContext.OutsideNodeLabelContainers[outsideSide]

			switch outsideSide {
			case core.PortSideNorth, core.PortSideSouth:
				nodeLabelCell.SetContributesToMinimumHeight(true)
				containerCell.SetCell(nodeLabelLocation.ContainerColumn(), nodeLabelCell)
			case core.PortSideWest, core.PortSideEast:
				nodeLabelCell.SetContributesToMinimumWidth(true)
				containerCell.SetCell(nodeLabelLocation.ContainerRow(), nodeLabelCell)
			}
		}
	}

	return nodeLabelCell
}
