// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/HierarchicalPortOrthogonalEdgeRouter.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package hports

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p5edges/orthogonal"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p5edges/orthogonal/direction"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("HierarchicalPortOrthogonalEdgeRouter", func() lgraph.Processor { return NewHierarchicalPortOrthogonalEdgeRouter() })
}

// HierarchicalPortOrthogonalEdgeRouter is the processor
// HierarchicalPortOrthogonalEdgeRouter: it routes edges connected to
// northern or southern hierarchical ports orthogonally, and it sets the
// coordinates of the external port dummy nodes.
//
// The HierarchicalPortConstraintProcessor replaced each northern or southern
// external port dummy by dummies in the layers. This processor restores the
// original dummies, places them above and below the graph, routes the edges
// between the layer dummies and the original dummies, and removes the layer
// dummies. Then it fixes the coordinates of all external port dummies and
// corrects the edge segments that became slanted.
type HierarchicalPortOrthogonalEdgeRouter struct {
	// northernExtPortEdgeRoutingHeight is the amount of space necessary to
	// route edges connected to northern external ports.
	northernExtPortEdgeRoutingHeight float64
}

// NewHierarchicalPortOrthogonalEdgeRouter is new
// HierarchicalPortOrthogonalEdgeRouter().
func NewHierarchicalPortOrthogonalEdgeRouter() *HierarchicalPortOrthogonalEdgeRouter {
	return &HierarchicalPortOrthogonalEdgeRouter{}
}

// Process is process(layeredGraph, monitor).
func (r *HierarchicalPortOrthogonalEdgeRouter) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	r.northernExtPortEdgeRoutingHeight = 0.0

	// Step 1
	// Restore any north / south port dummies removed by the
	// HierarchicalPortConstraintProcessor and connect them to the dummies
	// created in their stead.
	northSouthDummies := restoreNorthSouthDummies(layeredGraph)

	// Step 2
	// Calculate coordinates for the north / south port dummies. Coordinates
	// for the east / west port dummies have already been calculated prior to
	// this processor's execution. The coordinates are relative to the node's
	// content area, just like normal node coordinates. (the content area is
	// the node size minus padding minus offset)
	setNorthSouthDummyCoordinates(layeredGraph, northSouthDummies)

	// Step 3
	// Orthogonal edge routing.
	r.routeEdges(layeredGraph, northSouthDummies)

	// Step 4
	// Removal of the temporarily created north / south port dummies.
	removeTemporaryNorthSouthDummies(layeredGraph)

	// Step 5
	// Finally, the coordinates of east / west hierarchical port dummies have
	// to be corrected and set. The x coordinate must be set, and if north /
	// south port routing resulted in a change of offset or graph size, the y
	// coordinates have to be adjusted if port constraints are at FIXED_RATIO
	// or FIXED_POS. The graph's width may also have to be adjusted.
	fixCoordinates(layeredGraph)

	// Step 6
	// Fixing the dummy coordinates can easily lead to slanted edge segments,
	// which needs to be corrected.
	correctSlantedEdgeSegments(layeredGraph)
}

////////////////////////////////////////////////////////////////////////////
// STEP 1: RESTORE NORTH / SOUTH DUMMIES

// lastLayer is layers.get(layers.size() - 1), which fails for a graph
// without layers.
func lastLayer(layeredGraph *lgraph.LGraph) *lgraph.Layer {
	if len(layeredGraph.Layers) == 0 {
		lgraph.Bail("HierarchicalPortOrthogonalEdgeRouter: IndexOutOfBoundsException, the graph has no layers")
	}
	return layeredGraph.Layers[len(layeredGraph.Layers)-1]
}

// firstLayer is layers.get(0), which fails for a graph without layers.
func firstLayer(layeredGraph *lgraph.LGraph) *lgraph.Layer {
	if len(layeredGraph.Layers) == 0 {
		lgraph.Bail("HierarchicalPortOrthogonalEdgeRouter: IndexOutOfBoundsException, the graph has no layers")
	}
	return layeredGraph.Layers[0]
}

// firstPort is node.getPorts().get(0), which fails for a node without
// ports.
func firstPort(node *lgraph.LNode) *lgraph.LPort {
	if len(node.Ports) == 0 {
		lgraph.Bail("HierarchicalPortOrthogonalEdgeRouter: IndexOutOfBoundsException, an external port dummy has no port")
	}
	return node.Ports[0]
}

// restoreNorthSouthDummies iterates through all layers, restoring
// hierarchical port dummy nodes along the way. The restored nodes are
// connected to the temporary dummy nodes created for them. The restored
// nodes are added to the last layer. (which layer they are added to doesn't
// make any difference) It returns the list of restored external port
// dummies.
func restoreNorthSouthDummies(layeredGraph *lgraph.LGraph) []*lgraph.LNode {
	var restoredDummies []*lgraph.LNode

	if !prop.Has(layeredGraph, iprops.ExtPortReplacedDummies) {
		return restoredDummies
	}

	// Restore the original external port dummies
	for _, dummy := range prop.Get(layeredGraph, iprops.ExtPortReplacedDummies) {
		restoreDummy(dummy, layeredGraph)
		restoredDummies = append(restoredDummies, dummy)
	}

	// Looking for hierarchical port dummies that replaced the restored
	// hierarchical port dummy
	for _, layer := range layeredGraph.Layers {
		for nodeIndex := 0; nodeIndex < len(layer.Nodes); nodeIndex++ {
			node := layer.Nodes[nodeIndex]
			if node.Type != lgraph.NodeTypeExternalPort {
				// Not a hierarchical port dummy - we're not interested. Move
				// along, please, there's nothing to see here.
				continue
			}

			replacedDummy := prop.Get(node, iprops.ExtPortReplacedDummy)
			if replacedDummy != nil {
				// Restore the origin and connect the node to it
				connectNodeToDummy(layeredGraph, node, replacedDummy)
			}
		}
	}

	// Assign the restored dummies to the graph's last layer (this has nothing
	// to do anymore with where they'll be placed; we just need some layer to
	// assign them to, and we simply choose the last one)
	for _, dummy := range restoredDummies {
		dummy.SetLayer(lastLayer(layeredGraph))
	}

	return restoredDummies
}

// restoreDummy restores the given dummy.
func restoreDummy(dummy *lgraph.LNode, graph *lgraph.LGraph) {
	// Depending on the hierarchical port's side, we set the port side of the
	// dummy's ports to be able to route properly (northern dummies must have
	// a southern port)
	portSide := prop.Get(dummy, iprops.ExtPortSide)
	dummyPort := firstPort(dummy)

	if portSide == core.PortSideNorth {
		dummyPort.SetSide(core.PortSideSouth)
	} else if portSide == core.PortSideSouth {
		dummyPort.SetSide(core.PortSideNorth)
	}

	// Since the dummy node was hidden from the algorithm, its port labels
	// are not placed properly and its margins are not set accordingly. That
	// needs to be fixed (if port labels are to be taken into consideration)
	if prop.Get(graph, lopts.NodeSizeConstraints).Contains(core.SizeConstraintPortLabels) {
		// The ElkGraphImporter has set the relevant spacings on the dummy
		// node
		portLabelSpacingHorizontal := prop.Get(dummy, lopts.SpacingLabelPortHorizontal)
		portLabelSpacingVertical := prop.Get(dummy, lopts.SpacingLabelPortVertical)
		labelLabelSpacing := prop.Get(dummy, lopts.SpacingLabelLabel)

		portLabelPlacement := prop.Get(graph, lopts.PortLabelsPlacement)
		if portLabelPlacement.Contains(core.PortLabelPlacementInside) {
			currentY := portLabelSpacingVertical
			xCenterRelativeToPort := float64(dummy.Size.X/2) - dummyPort.Pos.X

			for _, label := range dummyPort.Labels {
				label.Pos.Y = currentY
				label.Pos.X = xCenterRelativeToPort - float64(label.Size.X/2)

				currentY += label.Size.Y + labelLabelSpacing
			}
		} else if portLabelPlacement.Contains(core.PortLabelPlacementOutside) {
			// Port labels have a vertical size of 0, but we need to set
			// their x coordinate
			for _, label := range dummyPort.Labels {
				label.Pos.X = portLabelSpacingHorizontal + dummy.Size.X - dummyPort.Pos.X
			}
		}

		// Calculate margins
		nodespacing.GetNodeMarginCalculator(adapters.AdaptLGraphTransparent(graph, false)).
			ProcessNode(adapters.AdaptLNode(dummy, false))
	}
}

// connectNodeToDummy adds a port to the given node and connects that to the
// given dummy node.
func connectNodeToDummy(layeredGraph *lgraph.LGraph, node, dummy *lgraph.LNode) {
	// First, add a port to the node. The port side depends on the node's
	// hierarchical port side
	outPort := lgraph.NewLPort()
	outPort.SetNode(node)

	extPortSide := prop.Get(node, iprops.ExtPortSide)
	outPort.SetSide(extPortSide)

	// Find the dummy node's port
	inPort := firstPort(dummy)

	// Connect the two nodes
	edge := lgraph.NewLEdge()
	edge.SetSource(outPort)
	edge.SetTarget(inPort)
}

////////////////////////////////////////////////////////////////////////////
// STEP 2: SET NORTH / SOUTH DUMMY COORDINATES

// setNorthSouthDummyCoordinates sets coordinates for northern and southern
// external port dummy nodes.
func setNorthSouthDummyCoordinates(layeredGraph *lgraph.LGraph, northSouthDummies []*lgraph.LNode) {
	constraints := prop.Get(layeredGraph, lopts.PortConstraints)
	graphSize := &layeredGraph.Size
	graphPadding := &layeredGraph.Padding
	graphWidth := graphSize.X + graphPadding.Left + graphPadding.Right
	northY := 0 - graphPadding.Top - layeredGraph.Offset.Y
	southY := graphSize.Y + graphPadding.Top + graphPadding.Bottom - layeredGraph.Offset.Y

	// Lists of northern and southern external port dummies
	var northernDummies []*lgraph.LNode
	var southernDummies []*lgraph.LNode

	for _, dummy := range northSouthDummies {
		// Set x coordinate
		switch constraints {
		case core.PortConstraintsFree, core.PortConstraintsFixedSide, core.PortConstraintsFixedOrder:
			calculateNorthSouthDummyPositions(dummy)
		case core.PortConstraintsFixedRatio:
			applyNorthSouthDummyRatio(dummy, graphWidth)
			dummy.BorderToContentAreaCoordinates(true, false)
		case core.PortConstraintsFixedPos:
			applyNorthSouthDummyPosition(dummy)
			dummy.BorderToContentAreaCoordinates(true, false)

			// Ensure that the graph is wide enough to hold the port
			graphSize.X = max(graphSize.X, dummy.Pos.X+float64(dummy.Size.X/2.0))
		}

		// Set y coordinates and add the dummy to its respective list
		switch prop.Get(dummy, iprops.ExtPortSide) {
		case core.PortSideNorth:
			dummy.Pos.Y = northY
			northernDummies = append(northernDummies, dummy)
		case core.PortSideSouth:
			dummy.Pos.Y = southY
			southernDummies = append(southernDummies, dummy)
		}
	}

	// Check for correct ordering of nodes in the FIXED_ORDER case and for
	// dummy nodes that have been put on top of one another
	switch constraints {
	case core.PortConstraintsFree, core.PortConstraintsFixedSide:
		ensureUniquePositions(northernDummies, layeredGraph)
		ensureUniquePositions(southernDummies, layeredGraph)
	case core.PortConstraintsFixedOrder:
		restoreProperOrder(northernDummies, layeredGraph)
		restoreProperOrder(southernDummies, layeredGraph)
	}
}

// anchorOffset is the x coordinate of the port anchor of a dummy, or 0 for
// a dummy without a port anchor.
func anchorOffset(dummy *lgraph.LNode) float64 {
	anchor := prop.Get(dummy, lopts.PortAnchor)
	if anchor == nil {
		return 0
	}
	return anchor.X
}

// calculateNorthSouthDummyPositions calculates the positions of northern
// and southern dummy nodes. The position is based on the nodes the dummy
// nodes are connected to, which is taken as the position of the port's
// center.
func calculateNorthSouthDummyPositions(dummy *lgraph.LNode) {
	// We use a simple algorithm that simply adds the horizontal positions of
	// all connected ports and divides the sum by the number of connected
	// ports

	// First, get the dummy's port (it has only one)
	dummyInPort := firstPort(dummy)

	// It might be that the node is not actually connected to anything. In
	// that case, we assign a default position
	if dummyInPort.Degree() == 0 {
		dummy.Pos.X = 0
	} else {
		// Now, iterate over all connected ports, adding their horizontal
		// position
		posSum := 0.0
		for _, connectedPort := range dummyInPort.ConnectedPorts() {
			posSum += connectedPort.Node().Pos.X + connectedPort.Pos.X + connectedPort.Anchor.X
		}

		// Assign the dummy's x coordinate
		offset := anchorOffset(dummy)
		dummy.Pos.X = posSum/float64(dummyInPort.Degree()) - offset
	}
}

// applyNorthSouthDummyRatio sets the dummy's x coordinate to respect the
// ratio defined for its original port. width is the graph width.
func applyNorthSouthDummyRatio(dummy *lgraph.LNode, width float64) {
	offset := anchorOffset(dummy)
	dummy.Pos.X = float64(width*prop.Get(dummy, iprops.PortRatioOrPosition)) - offset
}

// applyNorthSouthDummyPosition sets the dummy's x coordinate to its original
// port's x coordinate.
func applyNorthSouthDummyPosition(dummy *lgraph.LNode) {
	offset := anchorOffset(dummy)
	dummy.Pos.X = prop.Get(dummy, iprops.PortRatioOrPosition) - offset
}

// ensureUniquePositions ensures that no two dummy nodes in the given list
// are assigned the same x coordinate. This method must only be called if
// port constraints are set to FREE or FIXED_SIDE as it may not preserve the
// original order of dummy nodes.
func ensureUniquePositions(dummies []*lgraph.LNode, graph *lgraph.LGraph) {
	if len(dummies) == 0 {
		return
	}

	// Turn the list into an array of dummy nodes and sort that by their x
	// coordinate
	dummyArray := append([]*lgraph.LNode(nil), dummies...)
	gwt.SortFunc(dummyArray, func(a, b *lgraph.LNode) int {
		return gwt.DoubleCompare(a.Pos.X, b.Pos.X)
	})

	assignAscendingCoordinates(dummyArray, graph)
}

// restoreProperOrder checks if the automatically calculated node coordinates
// violate their fixed order and fixes the coordinates. Calling this method
// only makes sense if port constraints are set to FIXED_ORDER.
func restoreProperOrder(dummies []*lgraph.LNode, graph *lgraph.LGraph) {
	if len(dummies) == 0 {
		return
	}

	// Turn the list into an array of dummy nodes and sort that by their
	// original x coordinate
	dummyArray := append([]*lgraph.LNode(nil), dummies...)
	gwt.SortFunc(dummyArray, func(a, b *lgraph.LNode) int {
		return gwt.DoubleCompare(prop.Get(a, iprops.PortRatioOrPosition), prop.Get(b, iprops.PortRatioOrPosition))
	})

	assignAscendingCoordinates(dummyArray, graph)
}

// assignAscendingCoordinates iterates over the given array of dummy nodes,
// making sure that their x coordinates are strictly ascending. Dummy nodes
// whose coordinates are in violation of this rule are moved to the right.
// Once this method is finished, the coordinates of the dummy nodes reflect
// their order in the array. The array has one dummy or more.
func assignAscendingCoordinates(dummies []*lgraph.LNode, graph *lgraph.LGraph) {
	// Find the port-port spacing
	spacing := prop.Get(graph, lopts.SpacingPortPort)

	// Now, iterate over the array, remembering the last assigned position.
	// If we find a position that is less than or equal to the last position,
	// assign a new position of "lastPosition + edgeSpacing"
	nextValidCoordinate := dummies[0].Pos.X + dummies[0].Size.X + dummies[0].Margin.Right + spacing

	for index := 1; index < len(dummies); index++ {
		currentPosition := &dummies[index].Pos
		currentSize := &dummies[index].Size
		currentMargin := &dummies[index].Margin

		// Ensure spacings are adhered to
		delta := currentPosition.X - currentMargin.Left - nextValidCoordinate
		if delta < 0 {
			currentPosition.X -= delta
		}

		// Ensure the graph is large enough for this node
		graphSize := &graph.Size
		graphSize.X = max(graphSize.X, currentPosition.X+currentSize.X)

		// Compute next valid coordinate
		nextValidCoordinate = currentPosition.X + currentSize.X + currentMargin.Right + spacing
	}
}

////////////////////////////////////////////////////////////////////////////
// STEP 3: EDGE ROUTING

// routeEdges routes northern and southern hierarchical port edges and
// adjusts the graph's height and offsets accordingly. northSouthDummies has
// the restored northern and southern port dummies.
func (r *HierarchicalPortOrthogonalEdgeRouter) routeEdges(layeredGraph *lgraph.LGraph, northSouthDummies []*lgraph.LNode) {
	// Prepare south and target layers for northern and southern routing. The
	// sets are LinkedHashSets in Java.
	northernSourceLayer := gwt.NewSet[*lgraph.LNode]()
	northernTargetLayer := gwt.NewSet[*lgraph.LNode]()
	southernSourceLayer := gwt.NewSet[*lgraph.LNode]()
	southernTargetLayer := gwt.NewSet[*lgraph.LNode]()

	// Find some routing parameters
	nodeSpacing := prop.Get(layeredGraph, lopts.SpacingNodeNode)
	edgeSpacing := prop.Get(layeredGraph, lopts.SpacingEdgeEdge)

	// Assemble the northern and southern hierarchical port dummies and the
	// nodes they are connected to
	for _, hierarchicalPortDummy := range northSouthDummies {
		portSide := prop.Get(hierarchicalPortDummy, iprops.ExtPortSide)

		if portSide == core.PortSideNorth {
			northernTargetLayer.Add(hierarchicalPortDummy)

			for _, edge := range hierarchicalPortDummy.IncomingEdges() {
				northernSourceLayer.Add(edge.Source.Node())
			}
		} else if portSide == core.PortSideSouth {
			southernTargetLayer.Add(hierarchicalPortDummy)

			for _, edge := range hierarchicalPortDummy.IncomingEdges() {
				southernSourceLayer.Add(edge.Source.Node())
			}
		}
	}

	// Northern routing
	if !northernSourceLayer.IsEmpty() {
		// Route the edges using a south-to-north orthogonal edge router
		routingGenerator := orthogonal.NewOrthogonalRoutingGenerator(direction.RoutingDirectionSouthToNorth, edgeSpacing, "extnorth")

		slots := routingGenerator.RouteEdges(layeredGraph, northernSourceLayer.Values(), 0, northernTargetLayer.Values(),
			-nodeSpacing-layeredGraph.Offset.Y)

		// If anything was routed, adjust the graph's offset and height
		if slots > 0 {
			r.northernExtPortEdgeRoutingHeight = nodeSpacing + float64(float64(slots-1)*edgeSpacing)
			layeredGraph.Offset.Y += r.northernExtPortEdgeRoutingHeight
			layeredGraph.Size.Y += r.northernExtPortEdgeRoutingHeight
		}
	}

	// Southern routing
	if !southernSourceLayer.IsEmpty() {
		// Route the edges using a north-to-south orthogonal edge router
		routingGenerator := orthogonal.NewOrthogonalRoutingGenerator(direction.RoutingDirectionNorthToSouth, edgeSpacing, "extsouth")

		slots := routingGenerator.RouteEdges(layeredGraph, southernSourceLayer.Values(), 0, southernTargetLayer.Values(),
			layeredGraph.Size.Y+nodeSpacing-layeredGraph.Offset.Y)

		// Adjust graph height.
		if slots > 0 {
			layeredGraph.Size.Y += nodeSpacing + float64(float64(slots-1)*edgeSpacing)
		}
	}
}

////////////////////////////////////////////////////////////////////////////
// STEP 4: REMOVE TEMPORARY DUMMIES

// removeTemporaryNorthSouthDummies removes the temporary hierarchical port
// dummies, reconnecting their incoming and outgoing edges to the original
// dummies and setting the appropriate bend points.
func removeTemporaryNorthSouthDummies(layeredGraph *lgraph.LGraph) {
	var nodesToRemove []*lgraph.LNode

	// Iterate through all layers
	for _, layer := range layeredGraph.Layers {
		for nodeIndex := 0; nodeIndex < len(layer.Nodes); nodeIndex++ {
			node := layer.Nodes[nodeIndex]
			if node.Type != lgraph.NodeTypeExternalPort {
				// We're only looking for hierarchical port dummies
				continue
			}

			if !prop.Has(node, iprops.ExtPortReplacedDummy) {
				// We're only looking for temporary north / south dummies
				continue
			}

			// There must be a port where all edges come in, another port
			// where edges go out, and a port with an edge connecting node
			// and origin (that one was added previously by this processor)
			var nodeInPort, nodeOutPort, nodeOriginPort *lgraph.LPort

			for _, port := range node.Ports {
				switch port.Side {
				case core.PortSideWest:
					nodeInPort = port
				case core.PortSideEast:
					nodeOutPort = port
				default:
					nodeOriginPort = port
				}
			}
			if nodeInPort == nil || nodeOutPort == nil || nodeOriginPort == nil || len(nodeOriginPort.OutgoingEdges) == 0 {
				lgraph.Bail("HierarchicalPortOrthogonalEdgeRouter: NullPointerException, a temporary external port dummy lacks one of its three ports")
			}

			// Find the edge connecting this dummy to the original external
			// port dummy that we restored just a while ago
			nodeToOriginEdge := nodeOriginPort.OutgoingEdges[0]

			// Compute bend points for incoming edges
			incomingEdgeBendPoints := nodeToOriginEdge.BendPoints.Clone()
			firstBendPoint := nodeOriginPort.Pos.Clone()
			firstBendPoint.Add(&node.Pos)
			incomingEdgeBendPoints.Insert(0, firstBendPoint)

			// Compute bend points for outgoing edges
			outgoingEdgeBendPoints := emath.ReverseChain(nodeToOriginEdge.BendPoints)
			lastBendPoint := nodeOriginPort.Pos.Clone()
			lastBendPoint.Add(&node.Pos)
			outgoingEdgeBendPoints.Add(lastBendPoint)

			// Retrieve the original hierarchical port dummy
			replacedDummy := prop.Get(node, iprops.ExtPortReplacedDummy)
			replacedDummyPort := firstPort(replacedDummy)

			// Reroute all the input port's edges
			edges := append([]*lgraph.LEdge(nil), nodeInPort.IncomingEdges...)
			for _, edge := range edges {
				edge.SetTarget(replacedDummyPort)
				edge.BendPoints.AddAllAsCopies(edge.BendPoints.Len(), incomingEdgeBendPoints.Points)
			}

			// Reroute all the output port's edges
			edges = append([]*lgraph.LEdge(nil), nodeOutPort.OutgoingEdges...)
			for _, edge := range edges {
				edge.SetSource(replacedDummyPort)
				edge.BendPoints.AddAllAsCopies(0, outgoingEdgeBendPoints.Points)
			}

			// Remove connection between node and original hierarchical port
			// dummy
			nodeToOriginEdge.SetSource(nil)
			nodeToOriginEdge.SetTarget(nil)

			// Remember the temporary node for removal
			nodesToRemove = append(nodesToRemove, node)
		}
	}

	// Remove nodes
	for _, node := range nodesToRemove {
		node.SetLayer(nil)
	}
}

////////////////////////////////////////////////////////////////////////////
// STEP 5: FIX DUMMY COORDINATES

// fixCoordinates fixes all hierarchical port dummy coordinates. For east /
// west external port dummies, this means setting the x coordinate
// appropriately, and, in case of FIXED_RATIO, checking that the ratio is
// respected. For north / south hierarchical port dummies, this means setting
// the y coordinate appropriately.
func fixCoordinates(layeredGraph *lgraph.LGraph) {
	constraints := prop.Get(layeredGraph, lopts.PortConstraints)

	// East port dummies are in the first layer; all other dummies are in the
	// last layer
	fixCoordinatesOfLayer(firstLayer(layeredGraph), constraints, layeredGraph)
	fixCoordinatesOfLayer(lastLayer(layeredGraph), constraints, layeredGraph)
}

// fixCoordinatesOfLayer is fixCoordinates(layer, constraints, graph): it
// fixes the coordinates of the nodes in the given layer.
func fixCoordinatesOfLayer(layer *lgraph.Layer, constraints core.PortConstraints, graph *lgraph.LGraph) {
	// Get some geometric values from the graph
	padding := &graph.Padding
	offset := &graph.Offset
	graphActualSize := graph.ActualSize()
	newActualGraphHeight := graphActualSize.Y

	// During the first iteration, EAST and WEST dummy nodes are fixed. This
	// may change the height of the graph, so we're setting y coordinates of
	// NORTH and SOUTH dummies in a second iteration
	for _, node := range layer.Nodes {
		if node.Type != lgraph.NodeTypeExternalPort {
			// We're only looking for hierarchical port dummies
			continue
		}

		extPortSide := prop.Get(node, iprops.ExtPortSide)
		extPortSize := prop.Get(node, iprops.ExtPortSize)
		nodePosition := &node.Pos

		// Set x coordinate
		switch extPortSide {
		case core.PortSideEast:
			nodePosition.X = graph.Size.X + padding.Right - offset.X
		case core.PortSideWest:
			nodePosition.X = -offset.X - padding.Left
		}

		// Set y coordinate
		requiredActualGraphHeight := 0.0
		switch extPortSide {
		case core.PortSideEast, core.PortSideWest:
			if constraints == core.PortConstraintsFixedRatio {
				ratio := prop.Get(node, iprops.PortRatioOrPosition)
				nodePosition.Y = float64(graphActualSize.Y*ratio) - portAnchor(node).Y
				requiredActualGraphHeight = nodePosition.Y + extPortSize.Y
				node.BorderToContentAreaCoordinates(false, true)
			} else if constraints == core.PortConstraintsFixedPos {
				nodePosition.Y = prop.Get(node, iprops.PortRatioOrPosition) - portAnchor(node).Y
				requiredActualGraphHeight = nodePosition.Y + extPortSize.Y
				node.BorderToContentAreaCoordinates(false, true)
			}
		}

		newActualGraphHeight = max(newActualGraphHeight, requiredActualGraphHeight)
	}

	// Make the graph larger, if necessary
	graph.Size.Y += newActualGraphHeight - graphActualSize.Y

	// Iterate over NORTH and SOUTH dummies now that the graph's height is
	// fixed
	for _, node := range layer.Nodes {
		if node.Type != lgraph.NodeTypeExternalPort {
			// We're only looking for hierarchical port dummies
			continue
		}

		extPortSide := prop.Get(node, iprops.ExtPortSide)
		nodePosition := &node.Pos

		// Set y coordinate
		switch extPortSide {
		case core.PortSideNorth:
			nodePosition.Y = -offset.Y - padding.Top
		case core.PortSideSouth:
			nodePosition.Y = graph.Size.Y + padding.Bottom - offset.Y
		}
	}
}

////////////////////////////////////////////////////////////////////////////
// STEP 6: SLANTED EDGE SEGMENT CORRECTION

// correctSlantedEdgeSegments goes over the eastern and western hierarchical
// dummy nodes and checks whether their incident edges have slanted segments.
func correctSlantedEdgeSegments(layeredGraph *lgraph.LGraph) {
	// East port dummies are in the first layer; all other dummies are in the
	// last layer
	correctSlantedEdgeSegmentsOfLayer(firstLayer(layeredGraph))
	correctSlantedEdgeSegmentsOfLayer(lastLayer(layeredGraph))
}

// correctSlantedEdgeSegmentsOfLayer is correctSlantedEdgeSegments(layer): it
// goes over the eastern and western hierarchical dummy nodes in the given
// layer and checks whether their incident edges have slanted segments. Note
// that only the first and last segment needs to be checked.
func correctSlantedEdgeSegmentsOfLayer(layer *lgraph.Layer) {
	for _, node := range layer.Nodes {
		if node.Type != lgraph.NodeTypeExternalPort {
			// We're only looking for hierarchical port dummies
			continue
		}

		extPortSide := prop.Get(node, iprops.ExtPortSide)
		if extPortSide == core.PortSideEast || extPortSide == core.PortSideWest {
			for _, edge := range node.ConnectedEdges() {
				bendPoints := edge.BendPoints
				if bendPoints.IsEmpty() {
					// The edge has no bend points yet, but may still be
					// slanted. The Java code leaves that case as it is.
					continue
				}

				// Correct a slanted segment connected to the source port if
				// it belongs to our node
				sourcePort := edge.Source
				if sourcePort.Node() == node {
					firstBendPoint := bendPoints.First()
					firstBendPoint.Y = sourcePort.AbsoluteAnchor().Y
				}

				// Correct a slanted segment connected to the target port if
				// it belongs to our node
				targetPort := edge.Target
				if targetPort.Node() == node {
					lastBendPoint := bendPoints.Last()
					lastBendPoint.Y = targetPort.AbsoluteAnchor().Y
				}
			}
		}
	}
}
