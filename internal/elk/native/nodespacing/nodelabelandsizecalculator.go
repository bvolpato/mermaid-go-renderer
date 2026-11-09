// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/NodeLabelAndSizeCalculator.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package nodespacing has the node dimension calculation of ELK
// (org.eclipse.elk.alg.common.nodespacing): the size of a node, the
// placement of its ports and labels, and the margins of a node. It works on
// the adapters of the package adapters, so it serves each graph model.
package nodespacing

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/coreopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/cellsystem"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/internal"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/internal/algorithm"
)

// Process is NodeLabelAndSizeCalculator.process(graph): it processes each
// node of the graph.
func Process(graph adapters.GraphAdapter) {
	// Process all of the graph's direct children
	for _, node := range graph.Nodes() {
		ProcessNode(graph, node, true, false)
	}
}

// ProcessNode is NodeLabelAndSizeCalculator.process(graph, node, applyStuff,
// ignoreInsidePortLabels): it computes the size of the node and the
// positions of its ports and labels, and returns the size. The node is
// taken to be a child of the graph. This is not checked.
//
// With applyStuff the node is resized and its ports and labels are placed.
// Without it, only the size is returned. With ignoreInsidePortLabels the
// port labels that are to be placed inside are not placed: a layout
// algorithm that handles them itself gets a lower bound of the size of a
// hierarchical node.
//
// The node calculation sees the layout of a node as a system of cells for
// the labels and the ports. Each cell has a padding and a minimum size.
func ProcessNode(graph adapters.GraphAdapter, node adapters.NodeAdapter, applyStuff, ignoreInsidePortLabels bool) *emath.KVector {
	// Note that, upon Miro's request, each phase of the algorithm was given
	// a code name in the first version of this code. We happily carry on
	// fulfilling this request in this, the second version.

	/* PREPARATORY PREPARATIONS
	 *
	 * Create the context objects that hold all of the information relevant
	 * to our calculations, including pointers to all the components of the
	 * cell system. The different method calls will often just update
	 * information in the context object (or nested objects) that subsequent
	 * method calls will make use of. Creating the port contexts will also
	 * create label cells for each port that has labels.
	 */
	nodeContext := internal.NewNodeContext(graph, node)
	algorithm.CreatePortContexts(nodeContext, ignoreInsidePortLabels)

	/* PHASE 1: WONDEROUS WATERFOWL
	 *          Setup All Cells
	 *
	 * Create all the label cells that will hold node labels, as well as the
	 * cell containers that will hold them, for both inside and outside node
	 * labels. Also, assign node labels to the relevant label cells. If port
	 * labels are to be placed on the inside, setup the inside port label
	 * cells with the appropriate paddings, and set the width of the eastern
	 * and western ones to the maximum width of the labels they will contain.
	 * We can't do that for the northern and southern cells yet because port
	 * label placement is more complicated there.
	 */
	horizontalLayoutMode := true
	// If the graph is null, no layout direction is specified, or the layout
	// direction is set to undefined, use horizontal layout mode (which
	// yields vertically stacked labels).
	if graph != nil && adapters.HasProperty(graph, coreopts.Direction) {
		layoutDirection := adapters.GetProperty(graph, coreopts.Direction)
		horizontalLayoutMode = layoutDirection == core.DirectionUndefined || layoutDirection.IsHorizontal()
	}
	algorithm.CreateNodeLabelCells(nodeContext, false, horizontalLayoutMode)
	algorithm.CreateInsidePortLabelCells(nodeContext)

	/* PHASE 2: DEFECTIVE DUCK
	 *          Setup Client Area Space and Node Cell Padding
	 *
	 * Apply the minimum client area size to the central grid container cell.
	 * Also, reserve space for ports that extend inside the node due to
	 * negative port border offsets by setting up appropriate paddings on the
	 * main node container cell.
	 */
	algorithm.SetupMinimumClientAreaSize(nodeContext)
	algorithm.SetupNodePaddingForPortsWithOffset(nodeContext)

	/* PHASE 3: SALVAGEABLE SWAN
	 *          Minimum Space Required to Place Ports
	 *
	 * It is now time to find out how large the node needs to be if all ports
	 * are to be placed in a way that satisfies all spacing constraints. This
	 * may or may not include the labels of ports. We remember these
	 * information by setting the minimum width of north / south inside port
	 * label cells and the minimum height of east / west inside port label
	 * cells. Since the east / west cells are surrounded by the north / south
	 * cells, their height may be updated later once we know how high the
	 * north / south cells will be.
	 */
	algorithm.CalculateHorizontalPortPlacementSize(nodeContext)
	algorithm.CalculateVerticalPortPlacementSize(nodeContext)

	/* PHASE 4: DAMNABLE DUCKLING
	 *          Setup Cell System Size Contribution Flags
	 *
	 * Depending on the size constraints, the different cells may contribute
	 * to the height or to the width of the node. In this phase, we setup the
	 * size contribution flags according to the size constraints. This lays
	 * the groundwork for letting the cell system calculate stuff.
	 */
	algorithm.ConfigureCellSystemSizeContributions(nodeContext)

	/* PHASE 5: DUCK AND COVER
	 *          Set Node Width and Place Horizontal Ports
	 *
	 * We can now set the node's width and place the ports (and port labels)
	 * along the horizontal sides. Since we have no idea how high the node is
	 * going to be yet, we place southern ports with the assumption that the
	 * node has a height of 0. We will later have to offset those ports by
	 * the node's height. Setting the node width has the side effect of
	 * computing a horizontal layout for the cell system.
	 */
	algorithm.SetNodeWidth(nodeContext)
	algorithm.PlaceHorizontalPorts(nodeContext)
	algorithm.PlaceHorizontalPortLabels(nodeContext)

	/* PHASE 6: GIGANTIC GOOSE
	 *          Set Node Height and Place Vertical Ports
	 *
	 * We can now calculate the node's height and place the ports (and port
	 * labels) along the vertical sides. Also, since we now know the node's
	 * height, we can finally correct the southern port positions. Before we
	 * can do all that, however, we might need to update the height and
	 * padding of the eastern and western inside port label cells to be sure
	 * that free ports are positioned properly.
	 *
	 * Note that if we are to not apply stuff, we're done once we know the
	 * node's height. Which is why we stop at that point.
	 */
	algorithm.UpdateVerticalInsidePortLabelCellPadding(nodeContext)
	algorithm.SetNodeHeight(nodeContext)

	if !applyStuff {
		return nodeContext.NodeSize
	}

	algorithm.OffsetSouthernPortsByNodeSize(nodeContext)
	algorithm.PlaceVerticalPorts(nodeContext)
	algorithm.PlaceVerticalPortLabels(nodeContext)

	/* PHASE 7: THANKSGIVING
	 *          Place Labels and Apply Stuff
	 *
	 * Since we now have the node's final size, we can now calculate the
	 * positions of the containers for outer node labels and place all inner
	 * and outer node labels. Also, the port label cells have positions
	 * assigned to them and can be told to apply positions to their labels.
	 * Finally, we can apply the node's size and all of the port positions we
	 * have calculated.
	 */
	algorithm.PlaceLabels(nodeContext)
	algorithm.SetNodePadding(nodeContext)
	algorithm.ApplyStuff(nodeContext)

	// Return the size
	return nodeContext.NodeSize
}

// ComputeInsideNodeLabelPadding is
// NodeLabelAndSizeCalculator.computeInsideNodeLabelPadding(graph, node,
// layoutDirection): the padding that the inside node labels outside of the
// center need. A layout algorithm uses it to keep space around the content
// of a hierarchical node. The layout direction is the one to assume for the
// labels. The method does not read the graph.
func ComputeInsideNodeLabelPadding(graph adapters.GraphAdapter, node adapters.NodeAdapter, layoutDirection core.Direction) *emath.ElkPadding {
	// Create a node context and fill it with all the inside node labels
	nodeContext := internal.NewNodeContext(nil, node)
	algorithm.CreateNodeLabelCells(nodeContext, true, !layoutDirection.IsVertical())

	labelCellContainer := nodeContext.InsideNodeLabelContainer
	padding := &emath.ElkPadding{}

	// Top
	for _, col := range cellsystem.ValuesContainerArea() {
		labelCell := labelCellContainer.Cell(cellsystem.ContainerAreaBegin, col)
		if labelCell != nil {
			padding.Top = max(padding.Top, labelCell.MinimumHeight())
		}
	}

	// Bottom
	for _, col := range cellsystem.ValuesContainerArea() {
		labelCell := labelCellContainer.Cell(cellsystem.ContainerAreaEnd, col)
		if labelCell != nil {
			padding.Bottom = max(padding.Bottom, labelCell.MinimumHeight())
		}
	}

	// Left
	for _, row := range cellsystem.ValuesContainerArea() {
		labelCell := labelCellContainer.Cell(row, cellsystem.ContainerAreaBegin)
		if labelCell != nil {
			padding.Left = max(padding.Left, labelCell.MinimumWidth())
		}
	}

	// Right
	for _, row := range cellsystem.ValuesContainerArea() {
		labelCell := labelCellContainer.Cell(row, cellsystem.ContainerAreaEnd)
		if labelCell != nil {
			padding.Right = max(padding.Right, labelCell.MinimumWidth())
		}
	}

	// Apply insets and gap where necessary
	if padding.Top > 0 {
		padding.Top += labelCellContainer.Padding().Top
		padding.Top += labelCellContainer.Gap()
	}

	if padding.Bottom > 0 {
		padding.Bottom += labelCellContainer.Padding().Bottom
		padding.Bottom += labelCellContainer.Gap()
	}

	if padding.Left > 0 {
		padding.Left += labelCellContainer.Padding().Left
		padding.Left += labelCellContainer.Gap()
	}

	if padding.Right > 0 {
		padding.Right += labelCellContainer.Padding().Right
		padding.Right += labelCellContainer.Gap()
	}

	return padding
}
