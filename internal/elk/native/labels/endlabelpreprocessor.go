// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/EndLabelPreprocessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package labels

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/cellsystem"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/overlaps"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("EndLabelPreprocessor", func() lgraph.Processor { return NewEndLabelPreprocessor() })
}

// EndLabelCells is the type of the values of the property END_LABELS
// (iprops.EndLabels): the Java Map<LPort, LabelCell> of a node, with the
// label cell of each port that has end labels. The property holds a
// *EndLabelCells. The Java map is a HashMap with ports as keys, so it
// iterates in the order of insertion.
type EndLabelCells = gwt.Map[*lgraph.LPort, *cellsystem.LabelCell]

// endLabelCellsOf is node.getProperty(InternalProperties.END_LABELS) for a
// node that has the property.
func endLabelCellsOf(node *lgraph.LNode) *EndLabelCells {
	cells, ok := prop.Get(node, iprops.EndLabels).(*EndLabelCells)
	if !ok || cells == nil {
		lgraph.Bail("end labels: the property endLabels of a node is not a map of label cells")
	}
	return cells
}

// EndLabelPreprocessor is the processor EndLabelPreprocessor: it puts the end
// labels of the edges into label cells, one for each port, and stores them in
// the property END_LABELS of the node. The cells have positions relative to
// the top left corner of the node. The node margins grow to include the end
// labels.
type EndLabelPreprocessor struct{}

// NewEndLabelPreprocessor is new EndLabelPreprocessor().
func NewEndLabelPreprocessor() *EndLabelPreprocessor { return &EndLabelPreprocessor{} }

// Process is process(layeredGraph, monitor).
func (p *EndLabelPreprocessor) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	edgeLabelSpacing := prop.Get(layeredGraph, lopts.SpacingEdgeLabel)
	labelLabelSpacing := prop.Get(layeredGraph, lopts.SpacingLabelLabel)
	verticalLayout := prop.Get(layeredGraph, lopts.Direction).IsVertical()

	// We iterate over each node and place the end labels of its incident
	// edges
	for _, layer := range layeredGraph.Layers {
		for _, node := range layer.Nodes {
			p.processNode(node, edgeLabelSpacing, labelLabelSpacing, verticalLayout)
		}
	}
}

func (p *EndLabelPreprocessor) processNode(node *lgraph.LNode, edgeLabelSpacing, labelLabelSpacing float64, verticalLayout bool) {
	// Iterate over all ports and collect their labels in label cells
	portCount := len(node.Ports)
	portLabelCells := make([]*cellsystem.LabelCell, portCount)

	for portIndex := 0; portIndex < portCount; portIndex++ {
		port := node.Ports[portIndex]
		port.ID = portIndex

		labels, _ := GatherLabels(port)
		portLabelCells[portIndex] = p.createConfiguredLabelCell(labels, labelLabelSpacing, verticalLayout)
	}

	// Actually go off and place them labels!
	p.placeLabels(node, portLabelCells, labelLabelSpacing, edgeLabelSpacing, verticalLayout)

	// Turn the array into a map and save that in the node
	portToLabelCellMap := gwt.NewMap[*lgraph.LPort, *cellsystem.LabelCell]()
	for index := 0; index < len(portLabelCells); index++ {
		if portLabelCells[index] != nil {
			portToLabelCellMap.Put(node.Ports[index], portLabelCells[index])
		}
	}

	if !portToLabelCellMap.IsEmpty() {
		prop.Set[any](node, iprops.EndLabels, portToLabelCellMap)

		// Update the node's margins
		p.updateNodeMargins(node, portLabelCells)
	}
}

// createConfiguredLabelCell makes the label cell for the labels of one port,
// or returns nil without labels. The cell has the size that its labels need.
// The caller sets its alignment.
func (*EndLabelPreprocessor) createConfiguredLabelCell(labels []*lgraph.LLabel, labelLabelSpacing float64, verticalLayout bool) *cellsystem.LabelCell {
	if len(labels) == 0 {
		return nil
	}

	// Create the new label cell and setup its alignments depending on the
	// port's side
	labelCell := cellsystem.NewLabelCellMode(labelLabelSpacing, !verticalLayout)

	for _, label := range labels {
		labelCell.AddLabel(AdaptLabel(label))
	}

	// Setup the label cell's size
	labelCellRect := labelCell.CellRectangle()
	labelCellRect.Height = labelCell.MinimumHeight()
	labelCellRect.Width = labelCell.MinimumWidth()

	return labelCell
}

// Label Gathering

// noIncidentEdgeThickness is NO_INCIDENT_EDGE_THICKNESS: the value that tells
// that a port has no incident edges.
const noIncidentEdgeThickness = -1

// GatherLabels is EndLabelPreprocessor.gatherLabels(port): the end labels to
// be placed at the port. hasEdges is false where Java returns null: no edge
// is connected to the port.
//
// The function also handles a north or south port whose edges go to a north
// south port dummy. When the port has labels, it stores the maximum thickness
// of the edges of the port in the property MAX_EDGE_THICKNESS.
func GatherLabels(port *lgraph.LPort) (labels []*lgraph.LLabel, hasEdges bool) {
	labels = []*lgraph.LLabel{}

	// Gather labels of the port itself
	maxEdgeThickness := gatherLabelsOfPort(port, &labels)

	// If it has a dummy associated with it, we need to go through the dummy's
	// ports and process those that were created for the current port (see the
	// NorthSouthPortPreprocessor)
	dummyNode := prop.Get(port, iprops.PortDummy)
	if dummyNode != nil {
		for _, dummyPort := range dummyNode.Ports {
			if prop.Get(dummyPort, iprops.Origin) == any(port) {
				maxEdgeThickness = max(
					maxEdgeThickness,
					gatherLabelsOfPort(dummyPort, &labels))
			}
		}
	}

	// Only save the maximum edge thickness if we'll be interested in it later
	if len(labels) != 0 {
		prop.Set(port, iprops.MaxEdgeThickness, maxEdgeThickness)
	}

	return labels, maxEdgeThickness != noIncidentEdgeThickness
}

// gatherLabelsOfPort is the private gatherLabels(port, targetList): it adds
// the end labels of the edges of the port to the list. It returns the maximum
// thickness of the edges, or noIncidentEdgeThickness without an edge.
func gatherLabelsOfPort(port *lgraph.LPort, targetList *[]*lgraph.LLabel) float64 {
	maxEdgeThickness := -1.0

	// getConnectedEdges gives the incoming edges, then the outgoing edges.
	for _, edges := range [2][]*lgraph.LEdge{port.IncomingEdges, port.OutgoingEdges} {
		for _, incidentEdge := range edges {
			maxEdgeThickness = max(maxEdgeThickness, prop.Get(incidentEdge, lopts.EdgeThickness))

			// It's an outgoing edge; all tail labels belong to this port. Or
			// it's an incoming edge; all head labels belong to this port
			placement := core.EdgeLabelPlacementHead
			if incidentEdge.Source == port {
				placement = core.EdgeLabelPlacementTail
			}
			for _, label := range incidentEdge.Labels {
				if prop.Get(label, lopts.EdgeLabelsPlacement) != placement {
					continue
				}

				// Remember the edge each label came from
				if !prop.Has(label, iprops.EndLabelEdge) {
					prop.Set(label, iprops.EndLabelEdge, incidentEdge)
				}

				*targetList = append(*targetList, label)
			}
		}
	}

	return maxEdgeThickness
}

// Label Placement

// placeLabels places the end labels of all ports of the node.
func (p *EndLabelPreprocessor) placeLabels(node *lgraph.LNode, portLabelCells []*cellsystem.LabelCell, labelLabelSpacing, edgeLabelSpacing float64, verticalLayout bool) {
	// First, place them as we usually would. This step can result in overlaps
	// for northern / southern labels (for horizontal layout directions) or
	// for eastern / western labels (for vertical layout directions).
	for _, port := range node.Ports {
		if portLabelCells[port.ID] != nil {
			p.placeLabelsAtPort(port, portLabelCells[port.ID], edgeLabelSpacing)
		}
	}

	// If there are ports on the problematic sides, go ahead and remove
	// overlaps between them
	gap := float64(2 * labelLabelSpacing)
	if verticalLayout {
		p.removeLabelOverlaps(node, portLabelCells, core.PortSideEast, gap, edgeLabelSpacing)
		p.removeLabelOverlaps(node, portLabelCells, core.PortSideWest, gap, edgeLabelSpacing)
	} else {
		p.removeLabelOverlaps(node, portLabelCells, core.PortSideNorth, gap, edgeLabelSpacing)
		p.removeLabelOverlaps(node, portLabelCells, core.PortSideSouth, gap, edgeLabelSpacing)
	}
}

// placeLabelsAtPort is placeLabels(port, labelCell, edgeLabelSpacing): it
// places the cell with the end labels near the port.
func (p *EndLabelPreprocessor) placeLabelsAtPort(port *lgraph.LPort, labelCell *cellsystem.LabelCell, edgeLabelSpacing float64) {
	// Some necessary position information
	labelCellRect := labelCell.CellRectangle()
	if port.Node() == nil {
		lgraph.Bail("EndLabelPreprocessor: a port without a node")
	}
	nodeSize := &port.Node().Size
	nodeMargin := &port.Node().Margin
	portPos := &port.Pos
	portAnchor := emath.Sum(portPos, &port.Anchor)

	// Calculate cell position depending on port side
	switch port.Side {
	case core.PortSideNorth:
		labelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentBottom)
		labelCellRect.Y = -nodeMargin.Top -
			edgeLabelSpacing -
			labelCellRect.Height

		if p.getLabelSide(labelCell) == core.LabelSideAbove {
			labelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentRight)
			labelCellRect.X = portAnchor.X -
				p.maxEdgeThickness(port) -
				edgeLabelSpacing -
				labelCellRect.Width
		} else {
			labelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentLeft)
			labelCellRect.X = portAnchor.X +
				p.maxEdgeThickness(port) +
				edgeLabelSpacing
		}

	case core.PortSideEast:
		labelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentLeft)
		labelCellRect.X = nodeSize.X +
			nodeMargin.Right +
			edgeLabelSpacing

		if p.getLabelSide(labelCell) == core.LabelSideAbove {
			labelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentBottom)
			labelCellRect.Y = portAnchor.Y -
				p.maxEdgeThickness(port) -
				edgeLabelSpacing -
				labelCellRect.Height
		} else {
			labelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentTop)
			labelCellRect.Y = portAnchor.Y +
				p.maxEdgeThickness(port) +
				edgeLabelSpacing
		}

	case core.PortSideSouth:
		labelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentTop)
		labelCellRect.Y = nodeSize.Y +
			nodeMargin.Bottom +
			edgeLabelSpacing

		if p.getLabelSide(labelCell) == core.LabelSideAbove {
			labelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentRight)
			labelCellRect.X = portAnchor.X -
				p.maxEdgeThickness(port) -
				edgeLabelSpacing -
				labelCellRect.Width
		} else {
			labelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentLeft)
			labelCellRect.X = portAnchor.X +
				p.maxEdgeThickness(port) +
				edgeLabelSpacing
		}

	case core.PortSideWest:
		labelCell.SetHorizontalAlignment(cellsystem.HorizontalLabelAlignmentRight)
		labelCellRect.X = -nodeMargin.Left -
			edgeLabelSpacing -
			labelCellRect.Width

		if p.getLabelSide(labelCell) == core.LabelSideAbove {
			labelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentBottom)
			labelCellRect.Y = portAnchor.Y -
				p.maxEdgeThickness(port) -
				edgeLabelSpacing -
				labelCellRect.Height
		} else {
			labelCell.SetVerticalAlignment(cellsystem.VerticalLabelAlignmentTop)
			labelCellRect.Y = portAnchor.Y +
				p.maxEdgeThickness(port) +
				edgeLabelSpacing
		}
	}
}

// removeLabelOverlaps removes the overlaps between the end labels of the
// ports on one side of the node.
func (p *EndLabelPreprocessor) removeLabelOverlaps(node *lgraph.LNode, portLabelCells []*cellsystem.LabelCell, portSide core.PortSide, labelLabelSpacing, edgeLabelSpacing float64) {
	overlapRemover := overlaps.CreateForDirection(p.portSideToOverlapRemovalDirection(portSide)).
		WithGap(labelLabelSpacing, labelLabelSpacing).
		WithStartCoordinate(p.calculateOverlapStartCoordinate(node, portSide, edgeLabelSpacing))

	// Gather the rectangles
	for _, port := range node.PortsBySide(portSide) {
		if portLabelCells[port.ID] != nil {
			labelCellRect := portLabelCells[port.ID].CellRectangle()

			overlapRemover.AddRectangle(labelCellRect)
		}
	}

	// Remove overlaps. The remover has the rectangles of the label cells, so
	// there is nothing to apply.
	overlapRemover.RemoveOverlaps()
}

// calculateOverlapStartCoordinate returns the start coordinate of the overlap
// removal on the given port side.
func (*EndLabelPreprocessor) calculateOverlapStartCoordinate(node *lgraph.LNode, portSide core.PortSide, edgeLabelSpacing float64) float64 {
	nodeSize := &node.Size
	nodeMargin := &node.Margin

	switch portSide {
	case core.PortSideNorth:
		return -nodeMargin.Top - edgeLabelSpacing
	case core.PortSideSouth:
		return nodeSize.Y + nodeMargin.Bottom + edgeLabelSpacing
	case core.PortSideEast:
		return nodeSize.X + nodeMargin.Right + edgeLabelSpacing
	case core.PortSideWest:
		return -nodeMargin.Left - edgeLabelSpacing
	default:
		// Should never happen
		return 0
	}
}

// Node Margins

// updateNodeMargins makes the margins of the node include its end labels.
func (*EndLabelPreprocessor) updateNodeMargins(node *lgraph.LNode, labelCells []*cellsystem.LabelCell) {
	nodeMargin := &node.Margin
	nodeSize := &node.Size

	// Calculate the rectangle that describes the node's current margin
	nodeMarginRectangle := emath.NewElkRectangle(
		-nodeMargin.Left,
		-nodeMargin.Top,
		nodeMargin.Left+nodeSize.X+nodeMargin.Right,
		nodeMargin.Top+nodeSize.Y+nodeMargin.Bottom)

	// Union the rectangle with each rectangle that describes a label cell
	for _, labelCell := range labelCells {
		if labelCell != nil {
			nodeMarginRectangle.Union(labelCell.CellRectangle())
		}
	}

	// Reapply the new rectangle to the margin
	nodeMargin.Left = -nodeMarginRectangle.X
	nodeMargin.Top = -nodeMarginRectangle.Y
	nodeMargin.Right = nodeMarginRectangle.Width - nodeMargin.Left - nodeSize.X
	nodeMargin.Bottom = nodeMarginRectangle.Height - nodeMargin.Top - nodeSize.Y
}

// Utility Methods

// getLabelSide returns the side of the edge at which the labels of the cell
// go: the side of the first label.
func (*EndLabelPreprocessor) getLabelSide(labelCell *cellsystem.LabelCell) core.LabelSide {
	return prop.Get(labelCell.Labels()[0].Holder(), iprops.LabelSide)
}

// maxEdgeThickness returns the maximum thickness of the edges of the port.
func (*EndLabelPreprocessor) maxEdgeThickness(port *lgraph.LPort) float64 {
	return prop.Get(port, iprops.MaxEdgeThickness)
}

// portSideToOverlapRemovalDirection returns the overlap removal direction for
// a port side.
func (*EndLabelPreprocessor) portSideToOverlapRemovalDirection(portSide core.PortSide) overlaps.OverlapRemovalDirection {
	switch portSide {
	case core.PortSideNorth:
		return overlaps.OverlapRemovalDirectionUp
	case core.PortSideSouth:
		return overlaps.OverlapRemovalDirectionDown
	case core.PortSideEast:
		return overlaps.OverlapRemovalDirectionRight
	case core.PortSideWest:
		return overlaps.OverlapRemovalDirectionLeft
	default:
		// Shouldn't happen
		lgraph.Bail("EndLabelPreprocessor: no overlap removal direction for the port side %v", portSide)
		return 0
	}
}
