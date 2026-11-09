// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/NodeMarginCalculator.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package nodespacing

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/coreopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
)

// NodeMarginCalculator is the class NodeMarginCalculator: it sets the node
// margins. The margin of a node is the space around it that its ports, its
// labels and the labels of its ports and edges take. The ports and the
// labels must have their positions.
type NodeMarginCalculator struct {
	includeLabels             bool
	includePorts              bool
	includePortLabels         bool
	includeEdgeHeadTailLabels bool
	adapter                   adapters.GraphAdapter
}

// newNodeMarginCalculator is new NodeMarginCalculator(adapter).
func newNodeMarginCalculator(adapter adapters.GraphAdapter) *NodeMarginCalculator {
	return &NodeMarginCalculator{
		includeLabels:             true,
		includePorts:              true,
		includePortLabels:         true,
		includeEdgeHeadTailLabels: true,
		adapter:                   adapter,
	}
}

// ExcludeLabels is excludeLabels(): the margins do not include the node
// labels. It returns the calculator.
func (c *NodeMarginCalculator) ExcludeLabels() *NodeMarginCalculator {
	c.includeLabels = false
	return c
}

// ExcludePorts is excludePorts(): the margins do not include the ports. It
// returns the calculator.
func (c *NodeMarginCalculator) ExcludePorts() *NodeMarginCalculator {
	c.includePorts = false
	return c
}

// ExcludePortLabels is excludePortLabels(): the margins do not include the
// port labels. It returns the calculator.
func (c *NodeMarginCalculator) ExcludePortLabels() *NodeMarginCalculator {
	c.includePortLabels = false
	return c
}

// ExcludeEdgeHeadTailLabels is excludeEdgeHeadTailLabels(): the margins do
// not include the head and tail labels of the edges. It returns the
// calculator.
func (c *NodeMarginCalculator) ExcludeEdgeHeadTailLabels() *NodeMarginCalculator {
	c.includeEdgeHeadTailLabels = false
	return c
}

// Process is process(): it computes and sets the margin of each node.
func (c *NodeMarginCalculator) Process() {
	spacing := adapters.GetProperty(c.adapter, coreopts.SpacingLabelNode)

	// Iterate through all nodes
	for _, node := range c.adapter.Nodes() {
		c.processNode(node, spacing)
	}
}

// ProcessNode is processNode(node): it computes and sets the margin of the
// node. The node must be a node of the graph of the calculator.
func (c *NodeMarginCalculator) ProcessNode(node adapters.NodeAdapter) {
	spacing := adapters.GetProperty(c.adapter, coreopts.SpacingLabelNode)
	c.processNode(node, spacing)
}

// processNode is processNode(node, labelSpacing).
func (c *NodeMarginCalculator) processNode(node adapters.NodeAdapter, labelSpacing float64) {
	// This will be our bounding box. We'll start with one that's the same
	// size as our node, and at the same position.
	boundingBox := emath.NewElkRectangle(
		node.Position().X,
		node.Position().Y,
		node.Size().X,
		node.Size().Y)

	// We'll reuse this rectangle as our box for elements to add to the
	// bounding box
	elementBox := &emath.ElkRectangle{}

	// Put the node's labels into the bounding box
	if c.includeLabels {
		for _, label := range node.Labels() {
			elementBox.X = label.Position().X + node.Position().X
			elementBox.Y = label.Position().Y + node.Position().Y
			elementBox.Width = label.Size().X
			elementBox.Height = label.Size().Y

			boundingBox.Union(elementBox)
		}
	}

	// Do the same for ports and their labels
	for _, port := range node.Ports() {
		// Calculate the port's upper left corner's x and y coordinate
		portX := port.Position().X + node.Position().X
		portY := port.Position().Y + node.Position().Y

		// The port itself
		if c.includePorts {
			elementBox.X = portX
			elementBox.Y = portY
			elementBox.Width = port.Size().X
			elementBox.Height = port.Size().Y

			boundingBox.Union(elementBox)
		}

		// The port's labels
		if c.includePortLabels {
			for _, label := range port.Labels() {
				elementBox.X = label.Position().X + portX
				elementBox.Y = label.Position().Y + portY
				elementBox.Width = label.Size().X
				elementBox.Height = label.Size().Y

				boundingBox.Union(elementBox)
			}
		}

		// End labels of edges connected to the port
		if c.includeEdgeHeadTailLabels {
			requiredPortLabelSpace := emath.NewKVector(-labelSpacing, -labelSpacing)

			// TODO: maybe leave space for manually placed ports
			if adapters.GetProperty(node, coreopts.PortLabelsPlacement).Contains(core.PortLabelPlacementOutside) {
				for _, label := range port.Labels() {
					requiredPortLabelSpace.X += label.Size().X + labelSpacing
					requiredPortLabelSpace.Y += label.Size().Y + labelSpacing
				}
			}

			requiredPortLabelSpace.X = max(requiredPortLabelSpace.X, 0.0)
			requiredPortLabelSpace.Y = max(requiredPortLabelSpace.Y, 0.0)

			c.processEdgeHeadTailLabels(boundingBox, port.OutgoingEdges(), port.IncomingEdges(),
				node, port, requiredPortLabelSpace, labelSpacing)
		}
	}

	// Process end labels of edges directly connected to the node
	if c.includeEdgeHeadTailLabels {
		c.processEdgeHeadTailLabels(boundingBox, node.OutgoingEdges(), node.IncomingEdges(),
			node, nil, nil, labelSpacing)
	}

	// Reset the margin (guard against very small double precision errors
	// which can cause the results to be small negative values, which doesn't
	// make sense -- see #616)
	margin := node.Margin().Clone()
	margin.Top = max(0, node.Position().Y-boundingBox.Y)
	margin.Bottom = max(0, boundingBox.Y+boundingBox.Height-(node.Position().Y+node.Size().Y))
	margin.Left = max(0, node.Position().X-boundingBox.X)
	margin.Right = max(0, boundingBox.X+boundingBox.Width-(node.Position().X+node.Size().X))
	node.SetMargin(margin)
}

// processEdgeHeadTailLabels adds the bounding boxes of the tail labels of
// the outgoing edges and of the head labels of the incoming edges to the
// bounding box. The port is nil for edges that are connected to the node
// itself. portLabelSpace is the space that the labels of the port need.
func (c *NodeMarginCalculator) processEdgeHeadTailLabels(boundingBox *emath.ElkRectangle,
	outgoingEdges, incomingEdges []adapters.EdgeAdapter, node adapters.NodeAdapter, port adapters.PortAdapter,
	portLabelSpace *emath.KVector, labelSpacing float64) {

	labelBox := &emath.ElkRectangle{}

	// For each edge, the tail labels of outgoing edges ...
	for _, edge := range outgoingEdges {
		for _, label := range edge.Labels() {
			if adapters.GetProperty(label, coreopts.EdgeLabelsPlacement) == core.EdgeLabelPlacementTail {
				c.computeLabelBox(labelBox, label, false, node, port, portLabelSpace, labelSpacing)
				boundingBox.Union(labelBox)
			}
		}
	}

	// ... and the head label of incoming edges shall be considered
	for _, edge := range incomingEdges {
		for _, label := range edge.Labels() {
			if adapters.GetProperty(label, coreopts.EdgeLabelsPlacement) == core.EdgeLabelPlacementHead {
				c.computeLabelBox(labelBox, label, true, node, port, portLabelSpace, labelSpacing)
				boundingBox.Union(labelBox)
			}
		}
	}
}

// computeLabelBox computes the bounding box of an edge label. The position
// of the box is an estimate: the label placement can give another position.
// incomingEdge tells that the edge of the label ends at the node. The port
// is nil for an edge that is connected to the node itself.
func (c *NodeMarginCalculator) computeLabelBox(labelBox *emath.ElkRectangle, label adapters.LabelAdapter,
	incomingEdge bool, node adapters.NodeAdapter, port adapters.PortAdapter, portLabelSpace *emath.KVector,
	labelSpacing float64) {

	labelBox.X = node.Position().X
	labelBox.Y = node.Position().Y
	if port != nil {
		labelBox.X += port.Position().X
		labelBox.Y += port.Position().Y
	}

	labelBox.Width = label.Size().X
	labelBox.Height = label.Size().Y

	if port == nil {
		// The edge is connected directly to the node
		if incomingEdge {
			// Assume the edge enters the node at its western side
			labelBox.X -= labelSpacing + label.Size().X
		} else {
			// Assume the edge leaves the node at its eastern side
			labelBox.X += node.Size().X + labelSpacing
		}
	} else {
		switch port.Side() {
		case core.PortSideUndefined, core.PortSideEast:
			labelBox.X += port.Size().X +
				labelSpacing +
				portLabelSpace.X +
				labelSpacing

		case core.PortSideWest:
			labelBox.X -= labelSpacing +
				portLabelSpace.X +
				labelSpacing +
				label.Size().X

		case core.PortSideNorth:
			labelBox.X += port.Size().X +
				labelSpacing
			labelBox.Y -= labelSpacing +
				portLabelSpace.Y +
				labelSpacing +
				label.Size().Y

		case core.PortSideSouth:
			labelBox.X += port.Size().X +
				labelSpacing
			labelBox.Y += port.Size().Y +
				labelSpacing +
				portLabelSpace.Y +
				labelSpacing
		}
	}
}
