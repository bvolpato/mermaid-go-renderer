// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/graph/transform/ElkGraphLayoutTransferrer.java
// and from ElkUtil.resizeNode, ElkUtil.effectiveMinSizeConstraintFor and
// ElkUtil.applyVectorChain in
// plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/util/ElkUtil.java of
// Eclipse ELK 0.9.1 (EPL-2.0).

package layered

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/coreopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/elkgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// includeLabel is LabelDummySwitcher.INCLUDE_LABEL.
var includeLabel = prop.NewDefault("edgelabelcenterednessanalysis.includelabel", false)

// ApplyLayout is ElkGraphLayoutTransferrer.applyLayout(lgraph): it writes
// the layout of the LGraph to the ELK graph that it was made from.
func ApplyLayout(graph *lgraph.LGraph) {
	graphOrigin, _ := prop.Lookup(graph, iprops.Origin)
	parentElkNode, ok := graphOrigin.(*elkgraph.Node)
	if !ok {
		return
	}
	parentLNode := graph.ParentNode

	// Get the offset to be added to all coordinates
	offset := graph.Offset.Clone()

	// Adjust offset (and with it the positions) by the requested padding
	lPadding := &graph.Padding
	offset.X += lPadding.Left
	offset.Y += lPadding.Top

	// Set node padding, if it was computed during layout
	sizeOptions := prop.Get(parentElkNode, lopts.NodeSizeOptions)
	if sizeOptions.Contains(core.SizeOptionsComputePadding) {
		padding := prop.Get(parentElkNode, lopts.Padding)
		padding.Bottom = lPadding.Bottom
		padding.Top = lPadding.Top
		padding.Left = lPadding.Left
		padding.Right = lPadding.Right
	}

	// Along the way, we collect the list of edges to be processed later
	var edgeList []*lgraph.LEdge

	// Process the nodes
	for _, lnode := range graph.LayerlessNodes {
		origin, _ := prop.Lookup(lnode, iprops.Origin)
		if _, isNode := origin.(*elkgraph.Node); isNode {
			applyNodeLayout(lnode, offset)
		} else if elkport, isPort := origin.(*elkgraph.Port); isPort && parentLNode == nil {
			// We have an external port here on the top-most hierarchy
			// level of the current (possibly hierarchical) layout run; set
			// its position
			portPosition := lgraph.GetExternalPortPosition(graph, lnode, elkport.Width, elkport.Height)
			elkport.X, elkport.Y = portPosition.X, portPosition.Y
		}

		// Collect edges, except if they go into a nested subgraph (those
		// edges need to be processed later)
		for _, port := range lnode.Ports {
			for _, edge := range port.OutgoingEdges {
				if !lgraph.IsDescendant(edge.Target.Node(), lnode) {
					edgeList = append(edgeList, edge)
				}
			}
		}
	}

	// Collect edges that go from the parent node into its nested subgraph
	if parentLNode != nil {
		for _, port := range parentLNode.Ports {
			for _, edge := range port.OutgoingEdges {
				if lgraph.IsDescendant(edge.Target.Node(), parentLNode) {
					edgeList = append(edgeList, edge)
				}
			}
		}
	}

	// Iterate through all edges
	routing := prop.Get(parentElkNode, lopts.EdgeRouting)
	for _, ledge := range edgeList {
		applyEdgeLayout(ledge, routing, offset)
	}

	// Setup the parent node
	applyParentNodeLayout(graph)

	// Process nested subgraphs
	for _, lnode := range graph.LayerlessNodes {
		if nested := lnode.NestedGraph; nested != nil {
			ApplyLayout(nested)
		}
	}
}

// nodeFlexibility is NodeFlexibility.getNodeFlexibility(lNode).
func nodeFlexibility(node *lgraph.LNode) lenum.NodeFlexibility {
	if nf, ok := prop.Lookup(node, lopts.NodePlacementNetworkSimplexNodeFlexibility); ok {
		return nf
	}
	return prop.Get(node.Graph(), lopts.NodePlacementNetworkSimplexNodeFlexibilityDefault)
}

// applyNodeLayout is applyNodeLayout(lnode, offset).
func applyNodeLayout(lnode *lgraph.LNode, offset *emath.KVector) {
	origin, _ := prop.Lookup(lnode, iprops.Origin)
	elknode := origin.(*elkgraph.Node)

	// Set the node position
	nodeID := prop.Get(lnode, lopts.CrossingMinimizationPositionId)
	layerID := prop.Get(lnode, lopts.LayeringLayerId)
	prop.Set(elknode, lopts.CrossingMinimizationPositionId, nodeID)
	prop.Set(elknode, lopts.LayeringLayerId, layerID)
	elknode.X = lnode.Pos.X + offset.X
	elknode.Y = lnode.Pos.Y + offset.Y

	// Set the node size, if necessary
	flexible := false
	if prop.Get(lnode.Graph(), lopts.NodePlacementStrategy) == lenum.NodePlacementStrategyNetworkSimplex {
		nf := nodeFlexibility(lnode)
		flexible = nf == lenum.NodeFlexibilityNodeSizeWhereSpacePermits || nf == lenum.NodeFlexibilityNodeSize
	}
	if !prop.Get(elknode, lopts.NodeSizeConstraints).IsEmpty() || lnode.NestedGraph != nil || flexible {
		elknode.Width = lnode.Size.X
		elknode.Height = lnode.Size.Y
	}

	// Set port positions
	for _, lport := range lnode.Ports {
		portOrigin, _ := prop.Lookup(lport, iprops.Origin)
		if elkport, isPort := portOrigin.(*elkgraph.Port); isPort {
			elkport.X, elkport.Y = lport.Pos.X, lport.Pos.Y
			prop.Set(elkport, lopts.PortSide, lport.Side)
		}
	}

	// Set node label positions, if they were not fixed (that is the case if
	// the node has specific node label placement set that can be overridden
	// by the labels themselves)
	nodeHasLabelPlacement := !prop.Get(lnode, lopts.NodeLabelsPlacement).IsEmpty()
	for _, llabel := range lnode.Labels {
		if nodeHasLabelPlacement || !prop.Get(llabel, lopts.NodeLabelsPlacement).IsEmpty() {
			labelOrigin, _ := prop.Lookup(llabel, iprops.Origin)
			elklabel := labelOrigin.(*elkgraph.Label)
			elklabel.Width, elklabel.Height = llabel.Size.X, llabel.Size.Y
			elklabel.X, elklabel.Y = llabel.Pos.X, llabel.Pos.Y
		}
	}

	// Set port label positions, if they were not fixed
	placement := prop.Get(lnode, lopts.PortLabelsPlacement)
	if placement.Contains(core.PortLabelPlacementInside) || placement.Contains(core.PortLabelPlacementOutside) {
		for _, lport := range lnode.Ports {
			for _, llabel := range lport.Labels {
				labelOrigin, _ := prop.Lookup(llabel, iprops.Origin)
				elklabel := labelOrigin.(*elkgraph.Label)
				elklabel.Width = llabel.Size.X
				elklabel.Height = llabel.Size.Y
				elklabel.X, elklabel.Y = llabel.Pos.X, llabel.Pos.Y
			}
		}
	}
}

// applyEdgeLayout is applyEdgeLayout(ledge, routing, offset,
// additionalPadding).
func applyEdgeLayout(ledge *lgraph.LEdge, routing core.EdgeRouting, offset *emath.KVector) {
	origin, _ := prop.Lookup(ledge, iprops.Origin)
	elkedge, _ := origin.(*elkgraph.Edge)

	// Only edges with an original ElkEdge are processed
	if elkedge == nil {
		return
	}

	bendPoints := ledge.BendPoints
	edgeOffset := offset.Clone()

	// Adapt the offset value and add the source port position to the vector
	// chain
	edgeOffset.Add(calculateHierarchicalOffset(ledge))
	var sourcePoint *emath.KVector
	if lgraph.IsDescendant(ledge.Target.Node(), ledge.Source.Node()) {
		// If the edge's target node is a descendant of its source node, the
		// source point's coordinates are relative to the source node's
		// position
		sourcePort := ledge.Source
		sourcePoint = emath.Sum(&sourcePort.Pos, &sourcePort.Anchor)
		sourcePoint.Sub(offset)
	} else {
		sourcePoint = ledge.Source.AbsoluteAnchor()
	}
	bendPoints.AddFirst(sourcePoint)

	// Add the target port position to the vector chain, including additional
	// offset
	targetPoint := ledge.Target.AbsoluteAnchor()
	if targetOffset := prop.Get(ledge, iprops.TargetOffset); targetOffset != nil {
		targetPoint.Add(targetOffset)
	}
	bendPoints.Add(targetPoint)

	// Translate the bend points by the offset and apply the bend points
	bendPoints.Offset(edgeOffset)

	// Give the edge a proper edge section to store routing information:
	// ElkGraphUtil.firstEdgeSection(elkedge, true, true)
	var section *elkgraph.EdgeSection
	if len(elkedge.Sections) == 0 {
		section = &elkgraph.EdgeSection{}
		elkedge.Sections = append(elkedge.Sections, section)
	} else {
		section = elkedge.Sections[0]
		*section = elkgraph.EdgeSection{}
		elkedge.Sections = elkedge.Sections[:1]
	}
	section.IncomingShape = elkedge.Sources[0]
	section.OutgoingShape = elkedge.Targets[0]

	// ElkUtil.applyVectorChain(bendPoints, elkedgeSection)
	if bendPoints.Len() < 2 {
		lgraph.Bail("The vector chain must contain at least a source and a target point.")
	}
	first := bendPoints.First()
	section.StartX, section.StartY = first.X, first.Y
	section.BendPoints = section.BendPoints[:0]
	for _, p := range bendPoints.Points[1 : bendPoints.Len()-1] {
		section.BendPoints = append(section.BendPoints, *p)
	}
	last := bendPoints.Last()
	section.EndX, section.EndY = last.X, last.Y

	// Apply layout to labels
	for _, llabel := range ledge.Labels {
		labelOrigin, _ := prop.Lookup(llabel, iprops.Origin)
		elklabel := labelOrigin.(*elkgraph.Label)
		elklabel.Width = llabel.Size.X
		elklabel.Height = llabel.Size.Y
		elklabel.X = llabel.Pos.X + edgeOffset.X
		elklabel.Y = llabel.Pos.Y + edgeOffset.Y
		prop.Set(elklabel, includeLabel, prop.Get(llabel, includeLabel))
	}

	// Copy junction points
	if junctionPoints := prop.Get(ledge, lopts.JunctionPoints); junctionPoints != nil {
		junctionPoints.Offset(edgeOffset)
		prop.Set(elkedge, lopts.JunctionPoints, junctionPoints)
	} else {
		prop.Unset(elkedge, lopts.JunctionPoints)
	}

	// Mark the edge with information about its routing
	if routing == core.EdgeRoutingSplines {
		// SPLINES means that bend points shall be interpreted as control
		// points for splines
		prop.Set(elkedge, lopts.EdgeRouting, core.EdgeRoutingSplines)
	} else {
		// null means that bend points shall be interpreted as bend points
		prop.Unset(elkedge, lopts.EdgeRouting)
	}
}

// calculateHierarchicalOffset is calculateHierarchicalOffset(ledge).
func calculateHierarchicalOffset(ledge *lgraph.LEdge) *emath.KVector {
	result := &emath.KVector{}
	targetCoordinateSystem := prop.Get(ledge, iprops.CoordinateSystemOrigin)
	if targetCoordinateSystem != nil {
		// Transform everything
		currentGraph := ledge.Source.Node().Graph()
		for currentGraph != targetCoordinateSystem {
			// The graph should have a representing node
			representingNode := currentGraph.ParentNode
			currentGraph = representingNode.Graph()

			result.Add(&representingNode.Pos).Add(&currentGraph.Offset).AddXY(currentGraph.Padding.Left, currentGraph.Padding.Top)
		}
	}
	return result
}

// applyParentNodeLayout is applyParentNodeLayout(lgraph).
func applyParentNodeLayout(graph *lgraph.LGraph) {
	origin, _ := prop.Lookup(graph, iprops.Origin)
	elknode := origin.(*elkgraph.Node)

	// Since the node size constraints are reset below, remember whether they
	// include port labels
	sizeConstraintsIncludedPortLabels := prop.Get(elknode, lopts.NodeSizeConstraints).Contains(core.SizeConstraintPortLabels)

	// We need to resize the root node
	if graph.ParentNode == nil {
		graphProps := prop.Get(graph, iprops.GraphProperties)
		actualGraphSize := graph.ActualSize()

		if graphProps.Contains(lenum.GraphPropertiesExternalPorts) {
			// Ports have positions assigned
			prop.Set(elknode, lopts.PortConstraints, core.PortConstraintsFixedPos)
			resizeElkNode(elknode, actualGraphSize.X, actualGraphSize.Y, false, true)
		} else {
			// Ports have not been positioned yet - leave this for next layouter
			if !prop.Get(elknode, lopts.NodeSizeFixedGraphSize) {
				resizeElkNode(elknode, actualGraphSize.X, actualGraphSize.Y, true, true)
			}
		}
	}

	// Set the size constraints. Either to port labels or fixed.
	if sizeConstraintsIncludedPortLabels {
		prop.Set(elknode, lopts.NodeSizeConstraints, prop.Of(core.SizeConstraintPortLabels))
	} else {
		prop.Set(elknode, lopts.NodeSizeConstraints, prop.NoneOf[core.SizeConstraint]())
	}
}

// effectiveMinSizeConstraintFor is ElkUtil.effectiveMinSizeConstraintFor.
func effectiveMinSizeConstraintFor(node *elkgraph.Node) *emath.KVector {
	sizeConstraint := prop.Get(node, coreopts.NodeSizeConstraints)
	if sizeConstraint.Contains(core.SizeConstraintMinimumSize) {
		sizeOptions := prop.Get(node, coreopts.NodeSizeOptions)
		minSize := prop.Get(node, coreopts.NodeSizeMinimum).Clone()

		// If minimum width or height are not set, maybe default to default
		// values
		if sizeOptions.Contains(core.SizeOptionsDefaultMinimumSize) {
			if minSize.X <= 0 {
				minSize.X = 20
			}
			if minSize.Y <= 0 {
				minSize.Y = 20
			}
		}
		return minSize
	}
	return &emath.KVector{}
}

// resizeElkNode is ElkUtil.resizeNode(node, newWidth, newHeight, movePorts,
// moveLabels).
func resizeElkNode(node *elkgraph.Node, newWidth, newHeight float64, movePorts, moveLabels bool) {
	oldSize := emath.NewKVector(node.Width, node.Height)
	newSize := effectiveMinSizeConstraintFor(node)
	newSize.X = max(newSize.X, newWidth)
	newSize.Y = max(newSize.Y, newHeight)
	widthRatio := newSize.X / oldSize.X
	heightRatio := newSize.Y / oldSize.Y
	widthDiff := newSize.X - oldSize.X
	heightDiff := newSize.Y - oldSize.Y

	// update port positions
	if movePorts {
		var direction core.Direction
		if node.Parent == nil {
			direction = prop.Get(node, coreopts.Direction)
		} else {
			direction = prop.Get(node.Parent, coreopts.Direction)
		}
		fixedPorts := prop.Get(node, coreopts.PortConstraints) == core.PortConstraintsFixedPos
		for _, port := range node.Ports {
			portSide := prop.Get(port, coreopts.PortSide)
			if portSide == core.PortSideUndefined {
				portSide = elkCalcPortSide(port, direction)
				prop.Set(port, coreopts.PortSide, portSide)
			}
			switch portSide {
			case core.PortSideNorth:
				if !fixedPorts {
					port.X = port.X * widthRatio
				}
			case core.PortSideEast:
				port.X = port.X + widthDiff
				if !fixedPorts {
					port.Y = port.Y * heightRatio
				}
			case core.PortSideSouth:
				if !fixedPorts {
					port.X = port.X * widthRatio
				}
				port.Y = port.Y + heightDiff
			case core.PortSideWest:
				if !fixedPorts {
					port.Y = port.Y * heightRatio
				}
			}
		}
	}

	// resize the node AFTER ports have been placed, since calcPortSide needs
	// the old size
	node.Width, node.Height = newSize.X, newSize.Y

	// update label positions
	if moveLabels {
		for _, label := range node.Labels {
			midx := label.X + float64(label.Width/2)
			midy := label.Y + float64(label.Height/2)
			widthPercent := midx / oldSize.X
			heightPercent := midy / oldSize.Y

			if widthPercent+heightPercent >= 1 {
				if widthPercent-heightPercent > 0 && midy >= 0 {
					// label is on the right
					label.X += widthDiff
					label.Y += float64(heightDiff * heightPercent)
				} else if widthPercent-heightPercent < 0 && midx >= 0 {
					// label is on the bottom
					label.X += float64(widthDiff * widthPercent)
					label.Y += heightDiff
				}
			}
		}
	}

	// set fixed size option for the node: now the size is assumed to stay as
	// determined here
	prop.Set(node, coreopts.NodeSizeConstraints, prop.NoneOf[core.SizeConstraint]())
}
