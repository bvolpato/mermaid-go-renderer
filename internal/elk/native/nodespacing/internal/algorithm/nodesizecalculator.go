// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/internal/algorithm/NodeSizeCalculator.java
// of Eclipse ELK 0.9.1 (EPL-2.0), with the change of the pull request 955
// that elkjs 0.9.3 has (the top-down layout keeps a larger node size).

package algorithm

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/coreopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/internal"
)

// nodeSizeFixedGraphSize is the value of NODE_SIZE_FIXED_GRAPH_SIZE of the
// graph of the node. Java throws a NullPointerException for a node without
// a graph.
func nodeSizeFixedGraphSize(nodeContext *internal.NodeContext) bool {
	graph := nodeContext.Node.Graph()
	if graph == nil {
		lgraph.Bail("NodeSizeCalculator: the node has no graph")
	}
	return adapters.GetProperty(graph, coreopts.NodeSizeFixedGraphSize)
}

// SetNodeWidth is NodeSizeCalculator.setNodeWidth(nodeContext): it sets the
// width of the node from the size constraints, sets that width on the cell
// system, and makes the cell system compute its horizontal layout.
func SetNodeWidth(nodeContext *internal.NodeContext) {
	nodeSize := nodeContext.NodeSize
	var width float64

	if AreSizeConstraintsFixed(nodeContext) {
		// Simply use the node's current width
		width = nodeSize.X
	} else {
		// Ask the cell system how wide it would like to be or take the
		// node's width if it has already been set to a greater value
		if nodeContext.TopdownLayout {
			width = max(nodeSize.X, nodeContext.NodeContainer.MinimumWidth())
		} else {
			width = nodeContext.NodeContainer.MinimumWidth()
		}

		// If we include node labels and outside node labels are not to
		// overhang, we need to include those as well
		if nodeContext.SizeConstraints.Contains(core.SizeConstraintNodeLabels) &&
			!nodeContext.SizeOptions.Contains(core.SizeOptionsOutsideNodeLabelsOverhang) {

			width = max(width, nodeContext.OutsideNodeLabelContainers[core.PortSideNorth].MinimumWidth())
			width = max(width, nodeContext.OutsideNodeLabelContainers[core.PortSideSouth].MinimumWidth())
		}

		// The node might have a minimum size set...
		minNodeSize := GetMinimumNodeSize(nodeContext)
		if minNodeSize != nil {
			width = max(width, minNodeSize.X)
		}
	}

	// Set the node's width
	if nodeSizeFixedGraphSize(nodeContext) {
		nodeSize.X = max(nodeSize.X, width)
	} else {
		nodeSize.X = width
	}

	// Set the cell system's width and tell it to compute horizontal
	// coordinates and widths
	nodeCellRectangle := nodeContext.NodeContainer.CellRectangle()
	nodeCellRectangle.X = 0
	nodeCellRectangle.Width = width
	nodeContext.NodeContainer.LayoutChildrenHorizontally()
}

// SetNodeHeight is NodeSizeCalculator.setNodeHeight(nodeContext): it sets
// the height of the node from the size constraints, sets that height on the
// cell system, and makes the cell system compute its vertical layout.
func SetNodeHeight(nodeContext *internal.NodeContext) {
	nodeSize := nodeContext.NodeSize
	var height float64

	if AreSizeConstraintsFixed(nodeContext) {
		// Simply use the node's current height
		height = nodeSize.Y
	} else {
		// Ask the cell system how high it would like to be or take the
		// node's height if it has already been set to a greater value
		if nodeContext.TopdownLayout {
			height = max(nodeSize.Y, nodeContext.NodeContainer.MinimumHeight())
		} else {
			height = nodeContext.NodeContainer.MinimumHeight()
		}

		// If we include node labels and outside node labels are not to
		// overhang, we need to include those as well
		if nodeContext.SizeConstraints.Contains(core.SizeConstraintNodeLabels) &&
			!nodeContext.SizeOptions.Contains(core.SizeOptionsOutsideNodeLabelsOverhang) {

			height = max(height, nodeContext.OutsideNodeLabelContainers[core.PortSideEast].MinimumHeight())
			height = max(height, nodeContext.OutsideNodeLabelContainers[core.PortSideWest].MinimumHeight())
		}

		// The node might have a minimum size set...
		minNodeSize := GetMinimumNodeSize(nodeContext)
		if minNodeSize != nil {
			height = max(height, minNodeSize.Y)
		}

		// If size constraints are set to include ports, but port constraints
		// are FIXED_POS or FIXED_RATIO, we need to manually apply the height
		// required to place eastern and western ports because those heights
		// don't come out of the cell system
		if nodeContext.SizeConstraints.Contains(core.SizeConstraintPorts) {
			if nodeContext.PortConstraints == core.PortConstraintsFixedRatio ||
				nodeContext.PortConstraints == core.PortConstraintsFixedPos {

				height = max(height, nodeContext.InsidePortLabelCells[core.PortSideEast].MinimumHeight())
				height = max(height, nodeContext.InsidePortLabelCells[core.PortSideWest].MinimumHeight())
			}
		}
	}

	// Set the node's height
	if nodeSizeFixedGraphSize(nodeContext) {
		nodeSize.Y = max(nodeSize.Y, height)
	} else {
		nodeSize.Y = height
	}

	// Set the cell system's height and tell it to compute vertical
	// coordinates and heights
	nodeCellRectangle := nodeContext.NodeContainer.CellRectangle()
	nodeCellRectangle.Y = 0
	nodeCellRectangle.Height = height
	nodeContext.NodeContainer.LayoutChildrenVertically()
}
