// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/graph/LGraphUtil.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package lgraph

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// ResizeNode is LGraphUtil.resizeNode(node, newSize, movePorts, moveLabels).
// The float casts of the Java code do nothing in elkjs.
func ResizeNode(node *LNode, newSize *emath.KVector, movePorts, moveLabels bool) {
	oldSize := node.Size
	widthRatio := newSize.X / oldSize.X
	heightRatio := newSize.Y / oldSize.Y
	widthDiff := newSize.X - oldSize.X
	heightDiff := newSize.Y - oldSize.Y

	// Update port positions
	if movePorts {
		fixedPorts := prop.Get(node, lopts.PortConstraints) == core.PortConstraintsFixedPos
		for _, port := range node.Ports {
			switch port.Side {
			case core.PortSideNorth:
				if !fixedPorts {
					port.Pos.X *= widthRatio
				}
			case core.PortSideEast:
				port.Pos.X += widthDiff
				if !fixedPorts {
					port.Pos.Y *= heightRatio
				}
			case core.PortSideSouth:
				if !fixedPorts {
					port.Pos.X *= widthRatio
				}
				port.Pos.Y += heightDiff
			case core.PortSideWest:
				if !fixedPorts {
					port.Pos.Y *= heightRatio
				}
			}
		}
	}

	// Update label positions
	if moveLabels {
		for _, label := range node.Labels {
			midx := label.Pos.X + float64(label.Size.X/2)
			midy := label.Pos.Y + float64(label.Size.Y/2)
			widthPercent := midx / oldSize.X
			heightPercent := midy / oldSize.Y

			if widthPercent+heightPercent >= 1 {
				if widthPercent-heightPercent > 0 && midy >= 0 {
					// label is on the right
					label.Pos.X += widthDiff
					label.Pos.Y += float64(heightDiff * heightPercent)
				} else if widthPercent-heightPercent < 0 && midx >= 0 {
					// label is on the bottom
					label.Pos.X += float64(widthDiff * widthPercent)
					label.Pos.Y += heightDiff
				}
			}
		}
	}

	// Set the new node size
	node.Size.X = newSize.X
	node.Size.Y = newSize.Y

	// Set fixed size option for the node: now the size is assumed to stay as
	// determined here
	prop.Set(node, lopts.NodeSizeConstraints, prop.NoneOf[core.SizeConstraint]())
}

// OffsetGraphs is LGraphUtil.offsetGraphs(graphs, offsetx, offsety).
func OffsetGraphs(graphs []*LGraph, offsetx, offsety float64) {
	for _, graph := range graphs {
		OffsetGraph(graph, offsetx, offsety)
	}
}

// OffsetGraph is LGraphUtil.offsetGraph(graph, offsetx, offsety).
func OffsetGraph(graph *LGraph, offsetx, offsety float64) {
	graphOffset := emath.NewKVector(offsetx, offsety)
	for _, node := range graph.LayerlessNodes {
		node.Pos.Add(graphOffset)
		for _, port := range node.Ports {
			for _, edge := range port.OutgoingEdges {
				edge.BendPoints.Offset(graphOffset)
				if junctionPoints := prop.Get(edge, lopts.JunctionPoints); junctionPoints != nil {
					junctionPoints.Offset(graphOffset)
				}
				for _, label := range edge.Labels {
					label.Pos.Add(graphOffset)
				}
			}
		}
	}
}

// PlaceNodesHorizontally is LGraphUtil.placeNodesHorizontally(layer,
// xoffset).
func PlaceNodesHorizontally(layer *Layer, xoffset float64) {
	// determine maximal left and right margin
	maxLeftMargin, maxRightMargin := 0.0, 0.0
	for _, node := range layer.Nodes {
		maxLeftMargin = max(maxLeftMargin, node.Margin.Left)
		maxRightMargin = max(maxRightMargin, node.Margin.Right)
	}

	// CHECKSTYLEOFF MagicNumber
	for _, node := range layer.Nodes {
		alignment := prop.Get(node, lopts.Alignment)
		var ratio float64
		switch alignment {
		case core.AlignmentLeft:
			ratio = 0.0
		case core.AlignmentRight:
			ratio = 1.0
		case core.AlignmentCenter:
			ratio = 0.5
		default:
			// determine the number of input and output ports for the node
			inports, outports := 0, 0
			for _, port := range node.Ports {
				if len(port.IncomingEdges) != 0 {
					inports++
				}
				if len(port.OutgoingEdges) != 0 {
					outports++
				}
			}
			// calculate node placement based on the port numbers
			if inports+outports == 0 {
				ratio = 0.5
			} else {
				ratio = float64(outports) / float64(inports+outports)
			}
		}

		// align nodes to the layer's maximal margin
		size := &layer.Size
		nodeSize := node.Size.X
		xpos := float64((size.X - nodeSize) * ratio)
		if ratio > 0.5 {
			xpos -= float64(maxRightMargin * 2 * (ratio - 0.5))
		} else if ratio < 0.5 {
			xpos += float64(maxLeftMargin * 2 * (0.5 - ratio))
		}

		// consider the node's individual margin
		leftMargin := node.Margin.Left
		if xpos < leftMargin {
			xpos = leftMargin
		}
		rightMargin := node.Margin.Right
		if xpos > size.X-rightMargin-nodeSize {
			xpos = size.X - rightMargin - nodeSize
		}

		node.Pos.X = xoffset + xpos
	}
}

// FindMaxNonDummyNodeWidth is LGraphUtil.findMaxNonDummyNodeWidth(layer,
// respectNodeMargins).
func FindMaxNonDummyNodeWidth(layer *Layer, respectNodeMargins bool) float64 {
	// since the graph is transformed to a left-to-right layout during
	// layout, the width of nodes is not relevant for vertical directions
	if prop.Get(layer.Graph(), lopts.Direction).IsVertical() {
		return 0.0
	}

	maxWidth := 0.0
	for _, node := range layer.Nodes {
		if node.Type == NodeTypeNormal {
			width := node.Size.X
			if respectNodeMargins {
				width += node.Margin.Left + node.Margin.Right
			}
			maxWidth = max(maxWidth, width)
		}
	}
	return maxWidth
}

// ComputeGraphProperties is LGraphUtil.computeGraphProperties(layeredGraph).
func ComputeGraphProperties(layeredGraph *LGraph) {
	props := prop.Get(layeredGraph, graphProperties)
	if !props.IsEmpty() {
		props.Clear()
	}

	direction := GetDirection(layeredGraph)
	for _, node := range layeredGraph.LayerlessNodes {
		if prop.Get(node, lopts.CommentBox) {
			// a comment box
			props.Add(lenum.GraphPropertiesComments)
		} else if prop.Get(node, lopts.Hypernode) {
			// a hypernode
			props.Add(lenum.GraphPropertiesHypernodes)
			props.Add(lenum.GraphPropertiesHyperedges)
		} else if node.Type == NodeTypeExternalPort {
			// an external port
			props.Add(lenum.GraphPropertiesExternalPorts)
		}

		portConstraints := prop.Get(node, lopts.PortConstraints)
		if portConstraints == core.PortConstraintsUndefined {
			// correct the port constraints value
			prop.Set(node, lopts.PortConstraints, core.PortConstraintsFree)
		} else if portConstraints != core.PortConstraintsFree {
			props.Add(lenum.GraphPropertiesNonFreePorts)
		}

		for _, port := range node.Ports {
			if len(port.IncomingEdges)+len(port.OutgoingEdges) > 1 {
				props.Add(lenum.GraphPropertiesHyperedges)
			}

			portSide := port.Side
			switch direction {
			case core.DirectionUp, core.DirectionDown:
				if portSide == core.PortSideEast || portSide == core.PortSideWest {
					props.Add(lenum.GraphPropertiesNorthSouthPorts)
				}
			default:
				if portSide == core.PortSideNorth || portSide == core.PortSideSouth {
					props.Add(lenum.GraphPropertiesNorthSouthPorts)
				}
			}

			for _, edge := range port.OutgoingEdges {
				if edge.Target.Node() == node {
					props.Add(lenum.GraphPropertiesSelfLoops)
				}

				for _, label := range edge.Labels {
					switch prop.Get(label, lopts.EdgeLabelsPlacement) {
					case core.EdgeLabelPlacementCenter:
						props.Add(lenum.GraphPropertiesCenterLabels)
					case core.EdgeLabelPlacementHead, core.EdgeLabelPlacementTail:
						props.Add(lenum.GraphPropertiesEndLabels)
					}
				}
			}
		}
	}
}

// CreatePort is LGraphUtil.createPort(node, endPoint, type, layeredGraph).
// endPoint can be nil.
func CreatePort(node *LNode, endPoint *emath.KVector, portType lenum.PortType, layeredGraph *LGraph) *LPort {
	var port *LPort
	direction := GetDirection(layeredGraph)
	mergePorts := prop.Get(layeredGraph, lopts.MergeEdges)

	if (mergePorts || prop.Get(node, lopts.Hypernode)) && !prop.Get(node, lopts.PortConstraints).IsSideFixed() {
		// Hypernodes have one output port and one input port
		defaultSide := core.PortSideFromDirection(direction)
		side := defaultSide.Opposed()
		if portType == lenum.PortTypeOutput {
			side = defaultSide
		}
		port = ProvideCollectorPort(layeredGraph, node, portType, side)
	} else {
		port = NewLPort()
		port.SetNode(node)

		if endPoint != nil {
			pos := &port.Pos
			pos.X = endPoint.X - node.Pos.X
			pos.Y = endPoint.Y - node.Pos.Y
			pos.Bound(0, 0, node.Size.X, node.Size.Y)
			port.SetSide(CalcPortSide(port, direction))
		} else {
			defaultSide := core.PortSideFromDirection(direction)
			if portType == lenum.PortTypeOutput {
				port.SetSide(defaultSide)
			} else {
				port.SetSide(defaultSide.Opposed())
			}
		}

		gp := prop.Get(layeredGraph, graphProperties)
		portSide := port.Side
		switch direction {
		case core.DirectionLeft, core.DirectionRight:
			if portSide == core.PortSideNorth || portSide == core.PortSideSouth {
				gp.Add(lenum.GraphPropertiesNorthSouthPorts)
			}
		case core.DirectionUp, core.DirectionDown:
			if portSide == core.PortSideEast || portSide == core.PortSideWest {
				gp.Add(lenum.GraphPropertiesNorthSouthPorts)
			}
		}
	}

	return port
}

// CalcPortSide is LGraphUtil.calcPortSide(port, direction).
func CalcPortSide(port *LPort, direction core.Direction) core.PortSide {
	node := port.Node()
	// if the node has zero size, we cannot decide anything
	nodeWidth := node.Size.X
	nodeHeight := node.Size.Y
	if nodeWidth <= 0 && nodeHeight <= 0 {
		return core.PortSideUndefined
	}

	// check whether the port is inside the node
	xpos := port.Pos.X
	ypos := port.Pos.Y
	width := port.Size.X
	height := port.Size.Y
	switch direction {
	case core.DirectionLeft, core.DirectionRight:
		if xpos < 0 {
			return core.PortSideWest
		} else if xpos+width > nodeWidth {
			return core.PortSideEast
		}
	case core.DirectionUp, core.DirectionDown:
		if ypos < 0 {
			return core.PortSideNorth
		} else if ypos+height > nodeHeight {
			return core.PortSideSouth
		}
	}

	// check whether the port is in the upper left or the lower right half
	widthPercent := (xpos + float64(width/2)) / nodeWidth
	heightPercent := (ypos + float64(height/2)) / nodeHeight
	if widthPercent+heightPercent <= 1 && widthPercent-heightPercent <= 0 {
		// port is on the left
		return core.PortSideWest
	} else if widthPercent+heightPercent >= 1 && widthPercent-heightPercent >= 0 {
		// port is on the right
		return core.PortSideEast
	} else if heightPercent < 0.5 {
		// port is on the top
		return core.PortSideNorth
	}
	// port is on the bottom
	return core.PortSideSouth
}

// CalcPortOffset is LGraphUtil.calcPortOffset(port, side).
func CalcPortOffset(port *LPort, side core.PortSide) float64 {
	node := port.Node()
	switch side {
	case core.PortSideNorth:
		return -(port.Pos.Y + port.Size.Y)
	case core.PortSideEast:
		return port.Pos.X - node.Size.X
	case core.PortSideSouth:
		return port.Pos.Y - node.Size.Y
	case core.PortSideWest:
		return -(port.Pos.X + port.Size.X)
	}
	return 0
}

// CenterPoint is LGraphUtil.centerPoint(point, boundary, side).
func CenterPoint(point, boundary *emath.KVector, side core.PortSide) {
	switch side {
	case core.PortSideNorth:
		point.X = boundary.X / 2
		point.Y = 0
	case core.PortSideEast:
		point.X = boundary.X
		point.Y = boundary.Y / 2
	case core.PortSideSouth:
		point.X = boundary.X / 2
		point.Y = boundary.Y
	case core.PortSideWest:
		point.X = 0
		point.Y = boundary.Y / 2
	}
}

// ProvideCollectorPort is LGraphUtil.provideCollectorPort(layeredGraph, node,
// type, side).
func ProvideCollectorPort(layeredGraph *LGraph, node *LNode, portType lenum.PortType, side core.PortSide) *LPort {
	var port *LPort
	switch portType {
	case lenum.PortTypeInput:
		for _, inport := range node.Ports {
			if prop.Get(inport, inputCollect) {
				return inport
			}
		}
		port = NewLPort()
		prop.Set(port, inputCollect, true)
	case lenum.PortTypeOutput:
		for _, outport := range node.Ports {
			if prop.Get(outport, outputCollect) {
				return outport
			}
		}
		port = NewLPort()
		prop.Set(port, outputCollect, true)
	}
	if port != nil {
		port.SetNode(node)
		port.SetSide(side)
		CenterPoint(&port.Pos, &node.Size, side)
	}
	return port
}

// InitializePort is LGraphUtil.initializePort(port, portConstraints,
// direction, anchorPos). anchorPos can be nil.
func InitializePort(port *LPort, portConstraints core.PortConstraints, direction core.Direction, anchorPos *emath.KVector) {
	portSide := port.Side

	if portSide == core.PortSideUndefined && portConstraints.IsSideFixed() {
		// calculate the port side and offset from the port's position
		portSide = CalcPortSide(port, direction)
		port.SetSide(portSide)

		// if port coordinates are (0,0), we default to port offset 0 to make
		// the common case frustration-free
		if !prop.Has(port, lopts.PortBorderOffset) && portSide != core.PortSideUndefined && (port.Pos.X != 0 || port.Pos.Y != 0) {
			prop.Set(port, lopts.PortBorderOffset, CalcPortOffset(port, portSide))
		}
	}

	if portConstraints.IsRatioFixed() {
		ratio := 0.0

		switch portSide {
		case core.PortSideNorth, core.PortSideSouth:
			nodeWidth := port.Node().Size.X
			if nodeWidth > 0 {
				ratio = port.Pos.X / nodeWidth
			}
		case core.PortSideEast, core.PortSideWest:
			nodeHeight := port.Node().Size.Y
			if nodeHeight > 0 {
				ratio = port.Pos.Y / nodeHeight
			}
		}

		prop.Set(port, portRatioOrPosition, ratio)
	}

	portSize := &port.Size
	portAnchor := &port.Anchor

	// if the port anchor property is set, use it as anchor point
	if anchorPos != nil {
		portAnchor.X = anchorPos.X
		portAnchor.Y = anchorPos.Y
		port.ExplicitlySuppliedPortAnchor = true
	} else if portConstraints.IsSideFixed() && portSide != core.PortSideUndefined {
		// set the anchor point according to the port side
		switch portSide {
		case core.PortSideNorth:
			portAnchor.X = portSize.X / 2
		case core.PortSideEast:
			portAnchor.X = portSize.X
			portAnchor.Y = portSize.Y / 2
		case core.PortSideSouth:
			portAnchor.X = portSize.X / 2
			portAnchor.Y = portSize.Y
		case core.PortSideWest:
			portAnchor.Y = portSize.Y / 2
		}
	} else {
		// the port side will be decided later, so set the anchor to the
		// center point
		portAnchor.X = portSize.X / 2
		portAnchor.Y = portSize.Y / 2
	}
}

// CreateExternalPortDummy is LGraphUtil.createExternalPortDummy. The property
// holder is the port that the dummy stands for. portSize becomes the value
// of the property externalPortSize, and the anchor that the function makes
// becomes the value of port.anchor of the dummy.
func CreateExternalPortDummy(propertyHolder prop.Holder, portConstraints core.PortConstraints, portSide core.PortSide, netFlow int, portNodeSize, portPosition, portSize *emath.KVector, layoutDirection core.Direction, layeredGraph *LGraph) *LNode {
	finalExternalPortSide := portSide

	// Create the dummy with one port
	dummy := NewLNode(layeredGraph)
	dummy.Type = NodeTypeExternalPort
	prop.Set(dummy, extPortSize, portSize)
	prop.Set(dummy, lopts.PortConstraints, core.PortConstraintsFixedPos)
	// set some default port border offset
	portBorderOffset := prop.Get(propertyHolder, lopts.PortBorderOffset)
	prop.Set(dummy, lopts.PortBorderOffset, portBorderOffset)

	dummyPort := NewLPort()
	dummyPort.SetNode(dummy)

	// If the port constraints are free, we need to determine the port side
	// based on the net flow of the port
	if !portConstraints.IsSideFixed() {
		if netFlow >= 0 {
			finalExternalPortSide = core.PortSideFromDirection(layoutDirection)
		} else {
			finalExternalPortSide = core.PortSideFromDirection(layoutDirection).Opposed()
		}
		prop.Set(propertyHolder, lopts.PortSide, finalExternalPortSide)
	}

	// With the port side at hand, set the necessary properties and place the
	// dummy's port at the dummy's center
	anchor := &emath.KVector{}
	explicitAnchor := false

	if prop.Has(propertyHolder, lopts.PortAnchor) {
		anchor.Set(prop.Get(propertyHolder, lopts.PortAnchor))
		explicitAnchor = true
	} else {
		anchor.SetXY(portSize.X/2, portSize.Y/2)
	}

	switch finalExternalPortSide {
	case core.PortSideWest:
		prop.Set(dummy, lopts.LayeringLayerConstraint, lenum.LayerConstraintFirstSeparate)
		prop.Set(dummy, edgeConstraint, lenum.EdgeConstraintOutgoingOnly)
		dummy.Size.Y = portSize.Y
		if portBorderOffset < 0 {
			dummy.Size.X = -portBorderOffset
		}
		dummyPort.SetSide(core.PortSideEast)
		if !explicitAnchor {
			anchor.X = portSize.X
		}
		anchor.X -= portSize.X
	case core.PortSideEast:
		prop.Set(dummy, lopts.LayeringLayerConstraint, lenum.LayerConstraintLastSeparate)
		prop.Set(dummy, edgeConstraint, lenum.EdgeConstraintIncomingOnly)
		dummy.Size.Y = portSize.Y
		if portBorderOffset < 0 {
			dummy.Size.X = -portBorderOffset
		}
		dummyPort.SetSide(core.PortSideWest)
		if !explicitAnchor {
			anchor.X = 0
		}
	case core.PortSideNorth:
		prop.Set(dummy, inLayerConstraint, lenum.InLayerConstraintTop)
		dummy.Size.X = portSize.X
		if portBorderOffset < 0 {
			dummy.Size.Y = -portBorderOffset
		}
		dummyPort.SetSide(core.PortSideSouth)
		if !explicitAnchor {
			anchor.Y = portSize.Y
		}
		anchor.Y -= portSize.Y
	case core.PortSideSouth:
		prop.Set(dummy, inLayerConstraint, lenum.InLayerConstraintBottom)
		dummy.Size.X = portSize.X
		if portBorderOffset < 0 {
			dummy.Size.Y = -portBorderOffset
		}
		dummyPort.SetSide(core.PortSideNorth)
		if !explicitAnchor {
			anchor.Y = 0
		}
	}
	dummyPort.Pos.Set(anchor)
	prop.Set(dummy, lopts.PortAnchor, anchor)

	if portConstraints.IsOrderFixed() {
		// The order of ports is fixed in some way, so what we will have to
		// do is to remember information about it
		informationAboutIt := 0.0

		if portIndex, ok := prop.Lookup(propertyHolder, lopts.PortIndex); ok && portConstraints == core.PortConstraintsFixedOrder {
			// Use the port's index. Java negates the double value, so the
			// index 0 gives -0 on the south and west sides, which
			// Double.compare puts before 0.
			switch finalExternalPortSide {
			case core.PortSideNorth, core.PortSideEast:
				informationAboutIt = float64(portIndex)
			case core.PortSideSouth, core.PortSideWest:
				informationAboutIt = float64(-1 * float64(portIndex))
			}
		} else {
			// Use the port's position or ratio
			switch finalExternalPortSide {
			case core.PortSideWest, core.PortSideEast:
				informationAboutIt = portPosition.Y
				if portConstraints.IsRatioFixed() {
					informationAboutIt /= portNodeSize.Y
				}
			case core.PortSideNorth, core.PortSideSouth:
				informationAboutIt = portPosition.X
				if portConstraints.IsRatioFixed() {
					informationAboutIt /= portNodeSize.X
				}
			}
		}

		prop.Set(dummy, portRatioOrPosition, informationAboutIt)
	}

	// Set the property that says about which side this dummy node is
	prop.Set(dummy, extPortSide, finalExternalPortSide)

	return dummy
}

// GetExternalPortPosition is LGraphUtil.getExternalPortPosition(graph,
// portDummy, portWidth, portHeight).
func GetExternalPortPosition(graph *LGraph, portDummy *LNode, portWidth, portHeight float64) *emath.KVector {
	portPosition := portDummy.Pos.Clone()
	portPosition.X += float64(portDummy.Size.X / 2.0)
	portPosition.Y += float64(portDummy.Size.Y / 2.0)
	portOffset := prop.Get(portDummy, lopts.PortBorderOffset)

	// Get some properties of the graph
	graphSize := &graph.Size
	padding := &graph.Padding
	graphOffset := &graph.Offset

	// The exact coordinates depend on the side of the external port
	switch prop.Get(portDummy, extPortSide) {
	case core.PortSideNorth:
		portPosition.X += padding.Left + graphOffset.X - float64(portWidth/2.0)
		portPosition.Y = -portHeight - portOffset
		portDummy.Pos.Y = -(padding.Top + portOffset + graphOffset.Y)
	case core.PortSideEast:
		portPosition.X = graphSize.X + padding.Left + padding.Right + portOffset
		portPosition.Y += padding.Top + graphOffset.Y - float64(portHeight/2.0)
		portDummy.Pos.X = graphSize.X + padding.Right + portOffset - graphOffset.X
	case core.PortSideSouth:
		portPosition.X += padding.Left + graphOffset.X - float64(portWidth/2.0)
		portPosition.Y = graphSize.Y + padding.Top + padding.Bottom + portOffset
		portDummy.Pos.Y = graphSize.Y + padding.Bottom + portOffset - graphOffset.Y
	case core.PortSideWest:
		portPosition.X = -portWidth - portOffset
		portPosition.Y += padding.Top + graphOffset.Y - float64(portHeight/2.0)
		portDummy.Pos.X = -(padding.Left + portOffset + graphOffset.X)
	}

	return portPosition
}

// IsDescendant is LGraphUtil.isDescendant(child, parent).
func IsDescendant(child, parent *LNode) bool {
	current := child
	next := current.Graph().ParentNode
	for next != nil {
		current = next
		if current == parent {
			return true
		}
		next = current.Graph().ParentNode
	}
	return false
}

// ChangeCoordSystem is LGraphUtil.changeCoordSystem(point, oldGraph,
// newGraph).
func ChangeCoordSystem(point *emath.KVector, oldGraph, newGraph *LGraph) {
	if oldGraph == newGraph {
		// nothing has to be done
		return
	}

	// transform to absolute coordinates
	graph := oldGraph
	var node *LNode
	for {
		point.Add(&graph.Offset)
		node = graph.ParentNode
		if node != nil {
			padding := &graph.Padding
			point.AddXY(padding.Left, padding.Top)
			point.Add(&node.Pos)
			graph = node.Graph()
		}
		if node == nil {
			break
		}
	}

	// transform to relative coordinates (to newGraph)
	graph = newGraph
	for {
		point.Sub(&graph.Offset)
		node = graph.ParentNode
		if node != nil {
			padding := &graph.Padding
			point.SubXY(padding.Left, padding.Top)
			point.Sub(&node.Pos)
			graph = node.Graph()
		}
		if node == nil {
			break
		}
	}
}

// IndividualOrInherited is LGraphUtil.getIndividualOrInherited(node,
// property). The individual spacings of a node are not ported.
func IndividualOrInherited[T any](node *LNode, property *prop.Property[T]) T {
	if prop.Has(node, lopts.SpacingIndividual) {
		Bail("LGraphUtil: individual spacings are not ported")
	}
	if g := node.Graph(); g != nil {
		return prop.Get(g, property)
	}
	var zero T
	return zero
}

// GetDirection is LGraphUtil.getDirection(graph).
func GetDirection(graph *LGraph) core.Direction {
	direction := prop.Get(graph, lopts.Direction)
	if direction == core.DirectionUndefined {
		aspectRatio := prop.Get(graph, lopts.AspectRatio)
		if aspectRatio >= 1 {
			return core.DirectionRight
		}
		return core.DirectionDown
	}
	return direction
}

// GetMinimalModelOrder is LGraphUtil.getMinimalModelOrder(graph).
func GetMinimalModelOrder(graph *LGraph) int {
	order := math.MaxInt32
	for _, node := range graph.LayerlessNodes {
		if v, ok := prop.Lookup(node, modelOrder); ok {
			order = min(order, v)
		}
	}
	return order
}
