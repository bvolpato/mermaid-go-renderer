// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/internal/algorithm/LabelPlacer.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package algorithm

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/internal"
)

// PlaceLabels is LabelPlacer.placeLabels(nodeContext): it places the
// containers of the outer node labels, and then each node label and each
// port label.
func PlaceLabels(nodeContext *internal.NodeContext) {
	// Properly place all label cells for outer node labels
	placeOuterNodeLabelContainers(nodeContext)

	// Tell all node label cells to place their labels
	for _, labelCell := range nodeContext.NodeLabelCells {
		if labelCell != nil {
			labelCell.ApplyLabelLayout()
		}
	}

	// Tell all port label cells to place their labels
	for _, portContext := range nodeContext.PortContexts.Values() {
		if portContext.PortLabelCell != nil {
			portContext.PortLabelCell.ApplyLabelLayout()
		}
	}
}

// placeOuterNodeLabelContainers places the outer node label containers from
// the size constraints and the node size.
func placeOuterNodeLabelContainers(nodeContext *internal.NodeContext) {
	outerNodeLabelsOverhang := nodeContext.SizeOptions.Contains(core.SizeOptionsOutsideNodeLabelsOverhang)

	placeHorizontalOuterNodeLabelContainer(nodeContext, outerNodeLabelsOverhang, core.PortSideNorth)
	placeHorizontalOuterNodeLabelContainer(nodeContext, outerNodeLabelsOverhang, core.PortSideSouth)
	placeVerticalOuterNodeLabelContainer(nodeContext, outerNodeLabelsOverhang, core.PortSideEast)
	placeVerticalOuterNodeLabelContainer(nodeContext, outerNodeLabelsOverhang, core.PortSideWest)
}

// placeHorizontalOuterNodeLabelContainer places the outer node label
// container of the northern or the southern side.
func placeHorizontalOuterNodeLabelContainer(nodeContext *internal.NodeContext, outerNodeLabelsOverhang bool,
	portSide core.PortSide) {

	nodeSize := nodeContext.NodeSize
	nodeLabelContainer := nodeContext.OutsideNodeLabelContainers[portSide]
	nodeLabelContainerRect := nodeLabelContainer.CellRectangle()

	// Set the container's width and height to its minimum width and height
	nodeLabelContainerRect.Width = nodeLabelContainer.MinimumWidth()
	nodeLabelContainerRect.Height = nodeLabelContainer.MinimumHeight()

	// The container must be at least as wide as the node is
	nodeLabelContainerRect.Width = max(nodeLabelContainerRect.Width, nodeSize.X)

	// If node labels are not allowed to overhang and if they would do so
	// right now, make the container smaller
	if nodeLabelContainerRect.Width > nodeSize.X && !outerNodeLabelsOverhang {
		nodeLabelContainerRect.Width = nodeSize.X
	}

	// Container's x coordinate
	nodeLabelContainerRect.X = -(nodeLabelContainerRect.Width - nodeSize.X) / 2

	// Container's y coordinate depends on whether we place the thing on the
	// northern or southern side
	switch portSide {
	case core.PortSideNorth:
		nodeLabelContainerRect.Y = -nodeLabelContainerRect.Height
	case core.PortSideSouth:
		nodeLabelContainerRect.Y = nodeSize.Y
	}

	// Layout the container's children
	nodeLabelContainer.LayoutChildrenHorizontally()
	nodeLabelContainer.LayoutChildrenVertically()
}

// placeVerticalOuterNodeLabelContainer places the outer node label container
// of the eastern or the western side.
func placeVerticalOuterNodeLabelContainer(nodeContext *internal.NodeContext, outerNodeLabelsOverhang bool,
	portSide core.PortSide) {

	nodeSize := nodeContext.NodeSize
	nodeLabelContainer := nodeContext.OutsideNodeLabelContainers[portSide]
	nodeLabelContainerRect := nodeLabelContainer.CellRectangle()

	// Set the container's width and height to its minimum width and height
	nodeLabelContainerRect.Width = nodeLabelContainer.MinimumWidth()
	nodeLabelContainerRect.Height = nodeLabelContainer.MinimumHeight()

	// The container must be at least as high as the node is
	nodeLabelContainerRect.Height = max(nodeLabelContainerRect.Height, nodeSize.Y)

	// If node labels are not allowed to overhang and if they would do so
	// right now, make the container smaller
	if nodeLabelContainerRect.Height > nodeSize.Y && !outerNodeLabelsOverhang {
		nodeLabelContainerRect.Height = nodeSize.Y
	}

	// Container's y coordinate
	nodeLabelContainerRect.Y = -(nodeLabelContainerRect.Height - nodeSize.Y) / 2

	// Container's x coordinate depends on whether we place the thing on the
	// eastern or western side
	switch portSide {
	case core.PortSideWest:
		nodeLabelContainerRect.X = -nodeLabelContainerRect.Width
	case core.PortSideEast:
		nodeLabelContainerRect.X = nodeSize.X
	}

	// Layout the container's children
	nodeLabelContainer.LayoutChildrenHorizontally()
	nodeLabelContainer.LayoutChildrenVertically()
}
