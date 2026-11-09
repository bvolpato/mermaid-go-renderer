// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/graph/transform/ElkGraphImporter.java
// and from ElkSpacings.AbstractSpacingsBuilder in
// plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/util/ElkSpacings.java
// and ElkUtil.calcPortSide, calcPortOffset and computeInsidePart in
// plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/util/ElkUtil.java of
// Eclipse ELK 0.9.1 (EPL-2.0).

package layered

import (
	"math"

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

// ElkNodeSpacing gives the importer the results of NodeLabelAndSizeCalculator
// for a node of the ELK graph (through ElkGraphAdapters in Java). The package
// that ports the node spacing sets NodeSpacing. Without it, a node with
// labels or with size constraints stops the layout.
type ElkNodeSpacing interface {
	// ComputeInsideNodeLabelPadding is
	// NodeLabelAndSizeCalculator.computeInsideNodeLabelPadding for the node,
	// with the direction RIGHT.
	ComputeInsideNodeLabelPadding(node *elkgraph.Node) *emath.ElkPadding
	// MinimumSize is NodeLabelAndSizeCalculator.process(parent graph, node,
	// false, true): the size that the node needs.
	MinimumSize(node *elkgraph.Node) *emath.KVector
}

// NodeSpacing is the node spacing for ELK nodes. Set it in an init function.
var NodeSpacing ElkNodeSpacing

// importer is ElkGraphImporter.
type importer struct {
	// nodeMap is nodeAndPortMap for nodes.
	nodeMap map[*elkgraph.Node]*lgraph.LNode
	// portMap is nodeAndPortMap for ports: the port of the LGraph, and for
	// an external port its dummy node.
	portMap      map[*elkgraph.Port]*lgraph.LPort
	portDummyMap map[*elkgraph.Port]*lgraph.LNode
}

// ImportGraph is ElkGraphImporter.importGraph(elkgraph).
func ImportGraph(elkGraph *elkgraph.Node) *lgraph.LGraph {
	im := &importer{
		nodeMap:      map[*elkgraph.Node]*lgraph.LNode{},
		portMap:      map[*elkgraph.Port]*lgraph.LPort{},
		portDummyMap: map[*elkgraph.Port]*lgraph.LNode{},
	}
	return im.importGraph(elkGraph)
}

func (im *importer) importGraph(elkGraph *elkgraph.Node) *lgraph.LGraph {
	// Create the layered graph
	topLevelGraph := im.createLGraph(elkGraph)

	// Assign defined port sides to all external ports. It is assumed that
	// each port has a defined port side from here on.
	for _, elkport := range elkGraph.Ports {
		ensureDefinedPortSide(topLevelGraph, elkport)
	}

	// Transform the external ports, if any
	graphProperties := prop.Get(topLevelGraph, iprops.GraphProperties)
	checkExternalPorts(elkGraph, graphProperties)
	if graphProperties.Contains(lenum.GraphPropertiesExternalPorts) {
		for _, elkport := range elkGraph.Ports {
			im.transformExternalPort(elkGraph, topLevelGraph, elkport)
		}
	}

	// If we should apply the minimum size constraint, calculate it
	if shouldCalculateMinimumGraphSize(elkGraph) {
		im.calculateMinimumGraphSize(elkGraph, topLevelGraph)
	}

	if prop.Get(topLevelGraph, lopts.PartitioningActivate) {
		graphProperties.Add(lenum.GraphPropertiesPartitions)
	}

	// Import the graph either with or without multiple nested levels of
	// hierarchy
	if baseValue, ok := prop.Lookup(topLevelGraph, lopts.SpacingBaseValue); ok {
		applySpacingsBaseValue(topLevelGraph, baseValue)
	}
	if prop.Get(elkGraph, lopts.HierarchyHandling) == core.HierarchyHandlingIncludeChildren {
		im.importHierarchicalGraph(elkGraph, topLevelGraph)
	} else {
		im.importFlatGraph(elkGraph, topLevelGraph)
	}

	return topLevelGraph
}

// spacingFactors is the factor map of LayeredSpacings.LayeredSpacingsBuilder:
// each spacing option with its default value divided by the default value of
// the base option, spacing.nodeNode.
var spacingOptions = []*prop.Property[float64]{
	lopts.SpacingNodeNode,
	lopts.SpacingComponentComponent,
	lopts.SpacingEdgeEdge,
	lopts.SpacingEdgeLabel,
	lopts.SpacingEdgeNode,
	lopts.SpacingLabelLabel,
	lopts.SpacingLabelNode,
	lopts.SpacingLabelPortHorizontal,
	lopts.SpacingLabelPortVertical,
	lopts.SpacingNodeSelfLoop,
	lopts.SpacingPortPort,
	lopts.SpacingEdgeEdgeBetweenLayers,
	lopts.SpacingEdgeNodeBetweenLayers,
	lopts.SpacingNodeNodeBetweenLayers,
}

// applySpacingsBaseValue is LayeredSpacings.withBaseValue(base).apply(graph):
// it sets each spacing option that the graph does not set to its default
// value scaled by base over the default of spacing.nodeNode.
func applySpacingsBaseValue(graph *lgraph.LGraph, baseSpacing float64) {
	baseDefault, _ := lopts.SpacingNodeNode.Default()
	// DoubleMath.fuzzyEquals with the tolerance 1.0E-4
	if math.Abs(baseSpacing-baseDefault) <= 1.0e-4 || baseSpacing == baseDefault || (baseSpacing != baseSpacing && baseDefault != baseDefault) {
		return
	}
	for i, option := range spacingOptions {
		factor := 1.0
		if i > 0 {
			def, _ := option.Default()
			factor = def / baseDefault
		}
		if !prop.Has(graph, option) {
			prop.Set(graph, option, factor*baseSpacing)
		}
	}
}

func shouldCalculateMinimumGraphSize(elkGraph *elkgraph.Node) bool {
	return !prop.Get(elkGraph, lopts.NodeSizeConstraints).IsEmpty()
}

// calculateMinimumGraphSize is calculateMinimumGraphSize(elkgraph, lgraph).
func (im *importer) calculateMinimumGraphSize(elkGraph *elkgraph.Node, graph *lgraph.LGraph) {
	if elkGraph.Parent == nil {
		// Without a parent node, the graph adapter can't be created
		return
	}
	sizeConstraints := prop.Get(graph, lopts.NodeSizeConstraints)

	// NodeLabelAndSizeCalculator relies on port constraints being set
	if prop.Get(elkGraph, lopts.PortConstraints) == core.PortConstraintsUndefined {
		prop.Set(elkGraph, lopts.PortConstraints, core.PortConstraintsFree)
	}

	if NodeSpacing == nil {
		lgraph.Bail("ElkGraphImporter: the minimum size of a node with size constraints needs the node spacing, which is not linked")
	}
	minSize := NodeSpacing.MinimumSize(elkGraph)

	// Set the calculated minimum size on the graph
	sizeConstraints.Add(core.SizeConstraintMinimumSize)
	configuredMinSize := prop.Get(graph, lopts.NodeSizeMinimum)
	configuredMinSize.X = max(minSize.X, configuredMinSize.X)
	configuredMinSize.Y = max(minSize.Y, configuredMinSize.Y)
}

// importFlatGraph is importFlatGraph(elkgraph, lgraph).
func (im *importer) importFlatGraph(elkGraph *elkgraph.Node, graph *lgraph.LGraph) {
	// Transform the node's children, unless we're told not to
	index := 0
	for _, child := range elkGraph.Children {
		if !prop.Get(child, lopts.NoLayout) {
			if needsModelOrder(child) {
				// Assign a model order to the nodes as they are read
				prop.Set(child, iprops.ModelOrder, index)
				index++
			}
			im.transformNode(child, graph)
		}
	}

	// Iterate through the node's edges. Edges that connect the node with a
	// sibling or with the graph are transformed.
	index = 0
	for _, elkedge := range elkGraph.ContainedEdges {
		if needsModelOrderBasedOnParent(elkGraph) {
			// Assign a model order to the edges as they are read
			prop.Set(elkedge, iprops.ModelOrder, index)
			index++
		}
		checkConnected(elkedge)
		source := elkedge.SourceNode()
		target := elkedge.TargetNode()

		// Is inside self loop processing enabled for this node?
		enableInsideSelfLoops := prop.Get(source, lopts.InsideSelfLoopsActivate)

		// Find out basic information about the edge
		isToBeLaidOut := !prop.Get(elkedge, lopts.NoLayout)
		isInsideSelfLoop := enableInsideSelfLoops && elkedge.IsSelfloop() && prop.Get(elkedge, lopts.InsideSelfLoopsYo)
		connectsSiblings := source.Parent == elkGraph && source.Parent == target.Parent
		connectsToGraph := (source.Parent == elkGraph && target == elkGraph) != (target.Parent == elkGraph && source == elkGraph)

		if isToBeLaidOut && !isInsideSelfLoop && (connectsToGraph || connectsSiblings) {
			im.transformEdge(elkedge, elkGraph, graph)
		}
	}

	// Now collect inside self loops, which belong to the parent node
	if elkGraph.Parent != nil {
		for _, elkedge := range elkGraph.Parent.ContainedEdges {
			checkConnected(elkedge)
			source := elkedge.SourceNode()
			if source == elkGraph && elkedge.IsSelfloop() {
				isInsideSelfLoop := prop.Get(source, lopts.InsideSelfLoopsActivate) && prop.Get(elkedge, lopts.InsideSelfLoopsYo)
				if isInsideSelfLoop {
					im.transformEdge(elkedge, elkGraph, graph)
				}
			}
		}
	}
}

// checkConnected stops the layout for an edge without a source or a target,
// for which the Java code fails.
func checkConnected(e *elkgraph.Edge) {
	if len(e.Sources) == 0 || len(e.Targets) == 0 {
		lgraph.Bail("ElkGraphImporter: an edge without a source or a target")
	}
}

// importHierarchicalGraph is importHierarchicalGraph(elkgraph, lgraph).
func (im *importer) importHierarchicalGraph(elkGraph *elkgraph.Node, graph *lgraph.LGraph) {
	var queue []*elkgraph.Node

	parentGraphDirection := prop.Get(graph, lopts.Direction)

	// Model order index for nodes
	index := 0
	// Transform the node's children
	queue = append(queue, elkGraph.Children...)
	for len(queue) > 0 {
		elknode := queue[0]
		queue = queue[1:]

		if needsModelOrder(elknode) {
			// Assign a model order to the nodes as they are read
			prop.Set(elknode, iprops.ModelOrder, index)
			index++
		}

		// Check if the current node is to be laid out in the first place
		isNodeToBeLaidOut := !prop.Get(elknode, lopts.NoLayout)
		if isNodeToBeLaidOut {
			// Check if there has to be an LGraph for this node (which is the
			// case if it has children or inside self loops)
			hasChildren := len(elknode.Children) != 0
			hasInsideSelfLoops := hasInsideSelfLoops(elknode)
			hasHierarchyHandlingEnabled := prop.Get(elknode, lopts.HierarchyHandling) == core.HierarchyHandlingIncludeChildren
			usesElkLayered := usesElkLayered(elknode)

			var nestedGraph *lgraph.LGraph
			if usesElkLayered && hasHierarchyHandlingEnabled && (hasChildren || hasInsideSelfLoops) {
				nestedGraph = im.createLGraph(elknode)
				prop.Set(nestedGraph, lopts.Direction, parentGraphDirection)

				// Apply a spacing configuration, for subgraphs this has to be
				// done here
				if baseValue, ok := prop.Lookup(nestedGraph, lopts.SpacingBaseValue); ok {
					applySpacingsBaseValue(nestedGraph, baseValue)
				}

				// If the node's size constraints include minimum size, we
				// cache the minimum size
				if shouldCalculateMinimumGraphSize(elknode) {
					// Assign defined port sides to all external ports
					for _, elkport := range elknode.Ports {
						ensureDefinedPortSide(nestedGraph, elkport)
					}
					im.calculateMinimumGraphSize(elknode, nestedGraph)
				}
			}

			// Transform the node
			parentLGraph := graph
			if parentLNode := im.nodeMap[elknode.Parent]; parentLNode != nil {
				parentLGraph = parentLNode.NestedGraph
			}

			lnode := im.transformNode(elknode, parentLGraph)

			// Setup hierarchical relationships
			if nestedGraph != nil {
				lnode.NestedGraph = nestedGraph
				nestedGraph.ParentNode = lnode
				queue = append(queue, elknode.Children...)
			}
		}
	}

	// Model order index for edges
	index = 0
	// Transform the edges
	queue = append(queue, elkGraph)
	for len(queue) > 0 {
		elkGraphNode := queue[0]
		queue = queue[1:]
		for _, elkedge := range elkGraphNode.ContainedEdges {
			// We don't support hyperedges
			checkEdgeValidity(elkedge)
			if needsModelOrderBasedOnParent(elkGraph) {
				// Assign a model order to the edges as they are read
				prop.Set(elkedge, iprops.ModelOrder, index)
				index++
			}

			sourceNode := elkedge.SourceNode()
			targetNode := elkedge.TargetNode()

			// Don't bother if either the edge or at least one of its
			// endpoints are excluded from layout
			if prop.Get(elkedge, lopts.NoLayout) || prop.Get(sourceNode, lopts.NoLayout) || prop.Get(targetNode, lopts.NoLayout) {
				continue
			}

			// Check if this edge is an inside self-loop
			isInsideSelfLoop := elkedge.IsSelfloop() && prop.Get(sourceNode, lopts.InsideSelfLoopsActivate) && prop.Get(elkedge, lopts.InsideSelfLoopsYo)

			// Find the graph the edge will be placed in. Basically, if the
			// edge is an inside self loop or connects its source to a
			// descendant, the edge is placed in the graph that represents
			// the source node's insides. Otherwise, it is placed in the
			// graph that represents the source node's parent node.
			parentElkGraph := elkGraphNode
			if isInsideSelfLoop || elkgraph.IsDescendant(targetNode, sourceNode) {
				parentElkGraph = sourceNode
			} else if elkgraph.IsDescendant(sourceNode, targetNode) {
				parentElkGraph = targetNode
			}

			parentLGraph := graph
			if parentLNode := im.nodeMap[parentElkGraph]; parentLNode != nil {
				parentLGraph = parentLNode.NestedGraph
			}

			// Transform the edge and add it to the list of edges if it was
			// transformed
			ledge := im.transformEdge(elkedge, parentElkGraph, parentLGraph)

			// Find the graph the edge's coordinates will have to be made
			// relative to during export
			prop.Set(ledge, iprops.CoordinateSystemOrigin, im.findCoordinateSystemOrigin(elkedge, elkGraph, graph))
		}

		// We only process child nodes if hierarchy handling is enabled for
		// the current node
		if prop.Get(elkGraphNode, lopts.HierarchyHandling) == core.HierarchyHandlingIncludeChildren {
			for _, child := range elkGraphNode.Children {
				partOfSameLayoutRun := prop.Get(child, lopts.HierarchyHandling) == core.HierarchyHandlingIncludeChildren
				if usesElkLayered(child) && partOfSameLayoutRun {
					queue = append(queue, child)
				}
			}
		}
	}
}

// usesElkLayered tells that a node has no algorithm, or an algorithm whose
// id is a suffix of the id of the layered algorithm.
func usesElkLayered(node *elkgraph.Node) bool {
	algorithm, ok := prop.Lookup(node, coreopts.Algorithm)
	if !ok {
		return true
	}
	const id = "org.eclipse.elk.layered"
	return len(algorithm) <= len(id) && id[len(id)-len(algorithm):] == algorithm
}

func needsModelOrder(child *elkgraph.Node) bool {
	return needsModelOrderBasedOnParent(child.Parent) && !prop.Get(child, lopts.ConsiderModelOrderNoModelOrder)
}

func needsModelOrderBasedOnParent(elkGraph *elkgraph.Node) bool {
	return (prop.Get(elkGraph, lopts.ConsiderModelOrderStrategy) != lenum.OrderingStrategyNone ||
		prop.Get(elkGraph, lopts.CycleBreakingStrategy) == lenum.CycleBreakingStrategyModelOrder ||
		prop.Get(elkGraph, lopts.CycleBreakingStrategy) == lenum.CycleBreakingStrategyGreedyModelOrder ||
		prop.Get(elkGraph, lopts.CrossingMinimizationForceNodeModelOrder) ||
		prop.Get(elkGraph, lopts.ConsiderModelOrderComponents) != lenum.ComponentOrderingStrategyNone) ||
		prop.Get(elkGraph, lopts.LayeringNodePromotionStrategy) == lenum.NodePromotionStrategyModelOrderLeftToRight ||
		prop.Get(elkGraph, lopts.LayeringNodePromotionStrategy) == lenum.NodePromotionStrategyModelOrderRightToLeft ||
		prop.Get(elkGraph, lopts.LayeringStrategy) == lenum.LayeringStrategyBfModelOrder ||
		prop.Get(elkGraph, lopts.LayeringStrategy) == lenum.LayeringStrategyDfModelOrder
}

func hasInsideSelfLoops(elknode *elkgraph.Node) bool {
	if prop.Get(elknode, lopts.InsideSelfLoopsActivate) {
		for _, edge := range elkgraph.AllOutgoingEdges(elknode) {
			if edge.IsSelfloop() {
				if prop.Get(edge, lopts.InsideSelfLoopsYo) {
					return true
				}
			}
		}
	}
	return false
}

// findCoordinateSystemOrigin is findCoordinateSystemOrigin(elkedge,
// topLevelElkGraph, topLevelLGraph).
func (im *importer) findCoordinateSystemOrigin(elkedge *elkgraph.Edge, topLevelElkGraph *elkgraph.Node, topLevelLGraph *lgraph.LGraph) *lgraph.LGraph {
	source := elkedge.SourceNode()
	target := elkedge.TargetNode()

	// Coordinates of simple edges are relative to the graph that contains
	// them, as usual
	if source.Parent == target.Parent {
		return nil
	}

	// The same goes for edges that connect a node to one of its descendants
	if elkgraph.IsDescendant(target, source) {
		return nil
	}

	origin := elkedge.ContainingNode
	if origin == topLevelElkGraph {
		return topLevelLGraph
	}
	if lnode := im.nodeMap[origin]; lnode != nil {
		if nested := lnode.NestedGraph; nested != nil {
			return nested
		}
	}
	return nil
}

// createLGraph is createLGraph(elkgraph).
func (im *importer) createLGraph(elkGraph *elkgraph.Node) *lgraph.LGraph {
	graph := lgraph.NewLGraph()

	// Copy the properties of the ElkGraph to the layered graph
	prop.CopyProperties(graph, elkGraph)
	if prop.Get(graph, lopts.Direction) == core.DirectionUndefined {
		prop.Set(graph, lopts.Direction, lgraph.GetDirection(graph))
	}

	// The label manager is never set in elkjs.

	// Remember the ElkGraph's origin
	prop.Set[any](graph, iprops.Origin, elkGraph)

	// Initialize the graph properties discovered during the transformations
	prop.Set(graph, iprops.GraphProperties, prop.NoneOf[lenum.GraphProperties]())

	// Adjust the padding to respect inside labels. A node without labels
	// has no label cells, so its label padding is zero.
	nodeLabelPadding := &emath.ElkPadding{}
	if NodeSpacing != nil {
		nodeLabelPadding = NodeSpacing.ComputeInsideNodeLabelPadding(elkGraph)
	} else if len(elkGraph.Labels) != 0 {
		lgraph.Bail("ElkGraphImporter: the padding of a node with labels needs the node spacing, which is not linked")
	}
	nodePadding := prop.Get(graph, lopts.Padding)
	lPadding := &graph.Padding
	lPadding.Add(&nodePadding.Spacing)
	lPadding.Add(&nodeLabelPadding.Spacing)

	return graph
}

// transformNode is transformNode(elknode, lgraph).
func (im *importer) transformNode(elknode *elkgraph.Node, graph *lgraph.LGraph) *lgraph.LNode {
	// add a new node to the layered graph, copying its position
	lnode := lgraph.NewLNode(graph)
	prop.CopyProperties(lnode, elknode)
	prop.Set[any](lnode, iprops.Origin, elknode)

	lnode.Size.X = elknode.Width
	lnode.Size.Y = elknode.Height
	lnode.Pos.X = elknode.X
	lnode.Pos.Y = elknode.Y

	graph.LayerlessNodes = append(graph.LayerlessNodes, lnode)
	im.nodeMap[elknode] = lnode

	// check if the node is a compound node in the original graph
	if len(elknode.Children) != 0 || prop.Get(elknode, lopts.InsideSelfLoopsActivate) {
		prop.Set(lnode, iprops.CompoundNode, true)
	}

	graphProperties := prop.Get(graph, iprops.GraphProperties)

	// port constraints and sides cannot be undefined
	portConstraints := prop.Get(lnode, lopts.PortConstraints)
	if portConstraints == core.PortConstraintsUndefined {
		prop.Set(lnode, lopts.PortConstraints, core.PortConstraintsFree)
	} else if portConstraints != core.PortConstraintsFree {
		// if the port constraints are not free, set the appropriate graph
		// property
		graphProperties.Add(lenum.GraphPropertiesNonFreePorts)
	}

	// transform the ports
	portModelOrder := 0
	direction := prop.Get(graph, lopts.Direction)
	for _, elkport := range elknode.Ports {
		if needsModelOrder(elknode) {
			prop.Set(elkport, iprops.ModelOrder, portModelOrder)
			portModelOrder++
		}
		if !prop.Get(elkport, lopts.NoLayout) {
			im.transformPort(elkport, lnode, graphProperties, direction, portConstraints)
		}
	}

	// add the node's labels
	for _, elklabel := range elknode.Labels {
		if !prop.Get(elklabel, lopts.NoLayout) && elklabel.Text != "" {
			lnode.Labels = append(lnode.Labels, transformLabel(elklabel))
		}
	}

	if prop.Get(lnode, lopts.CommentBox) {
		graphProperties.Add(lenum.GraphPropertiesComments)
	}

	// if we have a hypernode without ports, create a default input and
	// output port
	if prop.Get(lnode, lopts.Hypernode) {
		graphProperties.Add(lenum.GraphPropertiesHypernodes)
		graphProperties.Add(lenum.GraphPropertiesHyperedges)
		prop.Set(lnode, lopts.PortConstraints, core.PortConstraintsFree)
	}

	return lnode
}

// checkEdgeValidity is checkEdgeValidity(edge).
func checkEdgeValidity(edge *elkgraph.Edge) {
	if len(edge.Sources) == 0 {
		lgraph.Bail("Edges must have a source.")
	} else if len(edge.Targets) == 0 {
		lgraph.Bail("Edges must have a target.")
	} else if edge.IsHyperedge() {
		lgraph.Bail("Hyperedges are not supported.")
	}
}

// transformEdge is transformEdge(elkedge, elkparent, lgraph). An edge has no
// section here: the JSON importer rejects edges with sections.
func (im *importer) transformEdge(elkedge *elkgraph.Edge, elkparent *elkgraph.Node, graph *lgraph.LGraph) *lgraph.LEdge {
	checkEdgeValidity(elkedge)

	// Get a few basic information about the edge
	elkSourceShape := elkedge.Sources[0]
	elkTargetShape := elkedge.Targets[0]
	elkSourceNode := elkSourceShape.ShapeNode()
	elkTargetNode := elkTargetShape.ShapeNode()

	// Find the layered graph adapters for the source and target nodes
	sourceLNode := im.nodeMap[elkSourceNode]
	targetLNode := im.nodeMap[elkTargetNode]
	var sourceLPort, targetLPort *lgraph.LPort

	// Find the layered graph adapters for the source and target ports, if
	// any
	if elkSourcePort, isPort := elkSourceShape.(*elkgraph.Port); isPort {
		// If the ElkGraph port is not an external port, we get an LPort.
		// Otherwise, we get an LNode.
		if lport := im.portMap[elkSourcePort]; lport != nil {
			sourceLPort = lport
		} else if dummy := im.portDummyMap[elkSourcePort]; dummy != nil {
			sourceLNode = dummy
			sourceLPort = dummy.Ports[0]
		}
	}
	if elkTargetPort, isPort := elkTargetShape.(*elkgraph.Port); isPort {
		if lport := im.portMap[elkTargetPort]; lport != nil {
			targetLPort = lport
		} else if dummy := im.portDummyMap[elkTargetPort]; dummy != nil {
			targetLNode = dummy
			targetLPort = dummy.Ports[0]
		}
	}

	if sourceLNode == nil || targetLNode == nil {
		lgraph.Bail("The source or the target of edge %s could not be found.", elkedge.Identifier)
	}

	// Create a layered edge
	ledge := lgraph.NewLEdge()
	prop.CopyProperties(ledge, elkedge)
	prop.Set[any](ledge, iprops.Origin, elkedge)

	// Clear junction points, since they are recomputed
	prop.Unset(ledge, lopts.JunctionPoints)

	graphProperties := prop.Get(graph, iprops.GraphProperties)

	// If we have a self-loop, set the appropriate graph property
	if sourceLNode == targetLNode {
		graphProperties.Add(lenum.GraphPropertiesSelfLoops)
	}

	// Create missing ports. An edge section would give the point of a
	// port: the edges have none here.
	if sourceLPort == nil {
		sourceLPort = lgraph.CreatePort(sourceLNode, nil, lenum.PortTypeOutput, graph)
	}
	if targetLPort == nil {
		targetLPort = lgraph.CreatePort(targetLNode, nil, lenum.PortTypeInput, targetLNode.Graph())
	}

	// Finally set the source and target of the edge
	ledge.SetSource(sourceLPort)
	ledge.SetTarget(targetLPort)

	// If we have a hyperedge, set the appropriate graph property
	if len(sourceLPort.IncomingEdges) > 1 || len(sourceLPort.OutgoingEdges) > 1 || len(targetLPort.IncomingEdges) > 1 || len(targetLPort.OutgoingEdges) > 1 {
		graphProperties.Add(lenum.GraphPropertiesHyperedges)
	}

	// Transform the edge's labels
	for _, elklabel := range elkedge.Labels {
		if !prop.Get(elklabel, lopts.NoLayout) && elklabel.Text != "" {
			llabel := transformLabel(elklabel)
			ledge.Labels = append(ledge.Labels, llabel)

			// edge label placement defaults to CENTER
			switch prop.Get(llabel, lopts.EdgeLabelsPlacement) {
			case core.EdgeLabelPlacementHead, core.EdgeLabelPlacementTail:
				graphProperties.Add(lenum.GraphPropertiesEndLabels)
			case core.EdgeLabelPlacementCenter:
				graphProperties.Add(lenum.GraphPropertiesCenterLabels)
				prop.Set(llabel, lopts.EdgeLabelsPlacement, core.EdgeLabelPlacementCenter)
			}
		}
	}

	// Copy the original bend points of the edge in case they are required:
	// only an edge with a section has bend points. The Java code reads the
	// two strategies in each case.
	prop.Get(graph, lopts.CrossingMinimizationStrategy)
	prop.Get(graph, lopts.NodePlacementStrategy)

	return ledge
}

// transformLabel is transformLabel(elklabel).
func transformLabel(elklabel *elkgraph.Label) *lgraph.LLabel {
	newLabel := lgraph.NewLLabel(elklabel.Text)
	prop.CopyProperties(newLabel, elklabel)
	prop.Set[any](newLabel, iprops.Origin, elklabel)
	newLabel.Size.X = elklabel.Width
	newLabel.Size.Y = elklabel.Height
	newLabel.Pos.X = elklabel.X
	newLabel.Pos.Y = elklabel.Y
	return newLabel
}

// elkCalcPortSide is ElkUtil.calcPortSide(port, direction).
func elkCalcPortSide(port *elkgraph.Port, direction core.Direction) core.PortSide {
	node := port.Parent
	// if the node has zero size, we cannot decide anything
	nodeWidth, nodeHeight := node.Width, node.Height
	if nodeWidth <= 0 && nodeHeight <= 0 {
		return core.PortSideUndefined
	}

	// check whether the port is inside the node
	xpos, ypos := port.X, port.Y
	switch direction {
	case core.DirectionLeft, core.DirectionRight:
		if xpos < 0 {
			return core.PortSideWest
		} else if xpos+port.Width > nodeWidth {
			return core.PortSideEast
		}
	case core.DirectionUp, core.DirectionDown:
		if ypos < 0 {
			return core.PortSideNorth
		} else if ypos+port.Height > nodeHeight {
			return core.PortSideSouth
		}
	}

	// check whether the port is in the upper left or the lower right half
	widthPercent := (xpos + float64(port.Width/2)) / nodeWidth
	heightPercent := (ypos + float64(port.Height/2)) / nodeHeight
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

// elkCalcPortOffset is ElkUtil.calcPortOffset(port, side).
func elkCalcPortOffset(port *elkgraph.Port, side core.PortSide) float64 {
	node := port.Parent
	switch side {
	case core.PortSideNorth:
		return -(port.Y + port.Height)
	case core.PortSideEast:
		return port.X - node.Width
	case core.PortSideSouth:
		return port.Y - node.Height
	case core.PortSideWest:
		return -(port.X + port.Width)
	}
	return 0
}

// computeInsidePart is ElkUtil.computeInsidePart(labelPosition, labelSize,
// portSize, portBorderOffset, portSide).
func computeInsidePart(labelPosition, labelSize, portSize *emath.KVector, portBorderOffset float64, portSide core.PortSide) float64 {
	insidePart := 0.0
	switch portSide {
	case core.PortSideNorth:
		insidePart = max(0, labelSize.Y+labelPosition.Y-(portSize.Y+portBorderOffset))
	case core.PortSideSouth:
		insidePart = max(0, -labelPosition.Y-portBorderOffset)
	case core.PortSideEast:
		insidePart = max(0, -labelPosition.X-portBorderOffset)
	case core.PortSideWest:
		insidePart = max(0, labelSize.X+labelPosition.X-(portSize.X+portBorderOffset))
	}
	return insidePart
}

// incidentEdges is ElkGraphUtil.allIncidentEdges(port): the incoming edges,
// then the outgoing edges.
func incidentEdges(port *elkgraph.Port) []*elkgraph.Edge {
	out := append([]*elkgraph.Edge(nil), port.IncomingEdges...)
	return append(out, port.OutgoingEdges...)
}

// ensureDefinedPortSide is ensureDefinedPortSide(lgraph, elkport).
func ensureDefinedPortSide(graph *lgraph.LGraph, elkport *elkgraph.Port) {
	layoutDirection := prop.Get(graph, lopts.Direction)
	portSide := prop.Get(elkport, lopts.PortSide)
	portConstraints := prop.Get(graph, lopts.PortConstraints)

	if !portConstraints.IsSideFixed() {
		// We are free to assign a side if one is not defined yet
		netFlow := calculateNetFlow(elkport)
		if netFlow > 0 {
			portSide = core.PortSideFromDirection(layoutDirection)
		} else {
			portSide = core.PortSideFromDirection(layoutDirection).Opposed()
		}
	} else {
		// Use the user-specified side or calculate it based on the port's
		// position
		if portSide == core.PortSideUndefined {
			portSide = elkCalcPortSide(elkport, layoutDirection)
			if portSide == core.PortSideUndefined {
				portSide = core.PortSideFromDirection(layoutDirection)
			}
		}
	}

	prop.Set(elkport, lopts.PortSide, portSide)
}

// shapeIn tells that a list of shapes has the port.
func shapeIn(shapes []elkgraph.Shape, port *elkgraph.Port) bool {
	for _, s := range shapes {
		if s == elkgraph.Shape(port) {
			return true
		}
	}
	return false
}

// checkExternalPorts is checkExternalPorts(elkgraph, graphProperties).
func checkExternalPorts(elkGraph *elkgraph.Node, graphProperties *prop.EnumSet[lenum.GraphProperties]) {
	enableSelfLoops := prop.Get(elkGraph, lopts.InsideSelfLoopsActivate)
	portLabelPlacement := prop.Get(elkGraph, lopts.PortLabelsPlacement)

	// We're iterating over the ports until we've determined that we have
	// both external ports and hyperedges, or if there are no more ports
	// left
	hasExternalPorts := false
	hasHyperedges := false

	for i := 0; i < len(elkGraph.Ports) && (!hasExternalPorts || !hasHyperedges); i++ {
		elkport := elkGraph.Ports[i]

		// Find out if there are edges connected to external ports of the
		// graph (this is the case if the edge connects this port to one of
		// the graph's children or to itself)
		externalPortEdges := 0
		for _, elkedge := range incidentEdges(elkport) {
			checkConnected(elkedge)
			isInsideSelfLoop := enableSelfLoops && elkedge.IsSelfloop() && prop.Get(elkedge, lopts.InsideSelfLoopsYo)
			var connectsToChild bool
			if shapeIn(elkedge.Sources, elkport) {
				connectsToChild = elkGraph == elkedge.Targets[0].ShapeNode().Parent
			} else {
				connectsToChild = elkGraph == elkedge.Sources[0].ShapeNode().Parent
			}

			if isInsideSelfLoop || connectsToChild {
				externalPortEdges++
				if externalPortEdges > 1 {
					break
				}
			}
		}

		// External ports?
		if externalPortEdges > 0 {
			hasExternalPorts = true
		} else if portLabelPlacement.Contains(core.PortLabelPlacementInside) && len(elkport.Labels) > 0 {
			// If the port has no edges, but does have a label that is
			// supposed to be placed inside the node, we need to have it be
			// transformed
			hasExternalPorts = true
		}

		// Hyperedges, even?
		if externalPortEdges > 1 {
			hasHyperedges = true
		}
	}

	// Update graph properties
	if hasExternalPorts {
		graphProperties.Add(lenum.GraphPropertiesExternalPorts)
	}
	if hasHyperedges {
		graphProperties.Add(lenum.GraphPropertiesHyperedges)
	}
}

// transformExternalPort is transformExternalPort(elkgraph, lgraph, elkport).
func (im *importer) transformExternalPort(elkGraph *elkgraph.Node, graph *lgraph.LGraph, elkport *elkgraph.Port) {
	// We need some information about the port
	elkportPosition := emath.NewKVector(elkport.X+float64(elkport.Width/2.0), elkport.Y+float64(elkport.Height/2.0))
	netFlow := calculateNetFlow(elkport)
	portConstraints := prop.Get(elkGraph, lopts.PortConstraints)
	portSide := prop.Get(elkport, lopts.PortSide)

	// If we don't have a proper port offset, calculate one
	if !prop.Has(elkport, lopts.PortBorderOffset) {
		// if port coordinates are (0,0), we default to port offset 0 to
		// make the common case frustration-free
		portOffset := 0.0
		if !(elkport.X == 0.0 && elkport.Y == 0.0) {
			portOffset = elkCalcPortOffset(elkport, portSide)
		}
		prop.Set(elkport, lopts.PortBorderOffset, portOffset)
	}

	// Create the external port dummy node
	graphSize := emath.NewKVector(elkGraph.Width, elkGraph.Height)
	dummy := lgraph.CreateExternalPortDummy(elkport, portConstraints, portSide, netFlow, graphSize, elkportPosition,
		emath.NewKVector(elkport.Width, elkport.Height), prop.Get(graph, lopts.Direction), graph)
	prop.Set[any](dummy, iprops.Origin, elkport)

	// The dummy only has one port
	dummyPort := dummy.Ports[0]
	dummyPort.ConnectedToExternalNodes = isConnectedToExternalNodes(elkport)
	prop.Set(dummy, lopts.PortLabelsPlacement, prop.Of(core.PortLabelPlacementOutside))

	// If the compound node wants to have its port labels placed on the
	// inside, we need to leave enough space for them by creating an LLabel
	// for the KLabels
	insidePortLabels := prop.Get(elkGraph, lopts.PortLabelsPlacement).Contains(core.PortLabelPlacementInside)

	// Transform all of the port's labels
	for _, elklabel := range elkport.Labels {
		if !prop.Get(elklabel, lopts.NoLayout) && elklabel.Text != "" {
			llabel := transformLabel(elklabel)
			dummyPort.Labels = append(dummyPort.Labels, llabel)

			// If port labels are placed outside, modify the size
			if !insidePortLabels {
				insidePart := 0.0
				placement := prop.Get(elkGraph, lopts.PortLabelsPlacement)
				if !placement.Contains(core.PortLabelPlacementInside) && !placement.Contains(core.PortLabelPlacementOutside) {
					// We use 0 as port border offset here, as we only want
					// the label part that is inside of the port
					insidePart = computeInsidePart(emath.NewKVector(elklabel.X, elklabel.Y), emath.NewKVector(elklabel.Width, elklabel.Height),
						emath.NewKVector(elkport.Width, elkport.Height), 0, portSide)
				}
				switch portSide {
				case core.PortSideEast, core.PortSideWest:
					llabel.Size.X = insidePart
				case core.PortSideNorth, core.PortSideSouth:
					llabel.Size.Y = insidePart
				}
			}
		}
	}

	// Remember the relevant spacings that will apply to the labels here. It's
	// not the spacings in the graph, but in the graph's parent.
	prop.Set(dummy, lopts.SpacingLabelPortHorizontal, prop.Get(elkGraph.Parent, lopts.SpacingLabelPortHorizontal))
	prop.Set(dummy, lopts.SpacingLabelPortVertical, prop.Get(elkGraph.Parent, lopts.SpacingLabelPortVertical))
	prop.Set(dummy, lopts.SpacingLabelLabel, prop.Get(elkGraph.Parent, lopts.SpacingLabelLabel))

	// Put the external port dummy into our graph and remember it
	graph.LayerlessNodes = append(graph.LayerlessNodes, dummy)
	im.portDummyMap[elkport] = dummy
}

// calculateNetFlow is calculateNetFlow(elkport).
func calculateNetFlow(elkport *elkgraph.Port) int {
	elkGraph := elkport.Parent
	insideSelfLoopsEnabled := prop.Get(elkGraph, lopts.InsideSelfLoopsActivate)

	outputPortVote, inputPortVote := 0, 0

	// Iterate over outgoing edges
	for _, outgoingEdge := range elkport.OutgoingEdges {
		checkConnected(outgoingEdge)
		isSelfLoop := outgoingEdge.IsSelfloop()
		isInsideSelfLoop := isSelfLoop && insideSelfLoopsEnabled && prop.Get(outgoingEdge, lopts.InsideSelfLoopsYo)
		targetNode := outgoingEdge.Targets[0].ShapeNode()

		if isSelfLoop && isInsideSelfLoop {
			inputPortVote++
		} else if isSelfLoop && !isInsideSelfLoop {
			outputPortVote++
		} else if targetNode.Parent == elkGraph || targetNode == elkGraph {
			inputPortVote++
		} else {
			outputPortVote++
		}
	}

	// Iterate over incoming edges
	for _, incomingEdge := range elkport.IncomingEdges {
		checkConnected(incomingEdge)
		isSelfLoop := incomingEdge.IsSelfloop()
		isInsideSelfLoop := isSelfLoop && insideSelfLoopsEnabled && prop.Get(incomingEdge, lopts.InsideSelfLoopsYo)
		sourceNode := incomingEdge.Sources[0].ShapeNode()

		if isSelfLoop && isInsideSelfLoop {
			outputPortVote++
		} else if isSelfLoop && !isInsideSelfLoop {
			inputPortVote++
		} else if sourceNode.Parent == elkGraph || sourceNode == elkGraph {
			outputPortVote++
		} else {
			inputPortVote++
		}
	}

	return outputPortVote - inputPortVote
}

// isConnectedToExternalNodes is isConnectedToExternalNodes(elkport).
func isConnectedToExternalNodes(elkport *elkgraph.Port) bool {
	parent := elkport.Parent
	for _, outEdge := range elkport.OutgoingEdges {
		if !elkgraph.IsDescendant(outEdge.Targets[0].ShapeNode(), parent) {
			return true
		}
	}
	for _, inEdge := range elkport.IncomingEdges {
		if !elkgraph.IsDescendant(inEdge.Sources[0].ShapeNode(), parent) {
			return true
		}
	}
	return false
}

// transformPort is transformPort(elkport, parentLNode, graphProperties,
// layoutDirection, portConstraints).
func (im *importer) transformPort(elkport *elkgraph.Port, parentLNode *lgraph.LNode, graphProperties *prop.EnumSet[lenum.GraphProperties], layoutDirection core.Direction, portConstraints core.PortConstraints) *lgraph.LPort {
	// create layered port, copying its position
	lport := lgraph.NewLPort()
	prop.CopyProperties(lport, elkport)
	lport.SetSide(prop.Get(elkport, lopts.PortSide))
	prop.Set[any](lport, iprops.Origin, elkport)
	lport.SetNode(parentLNode)

	lport.Size.X = elkport.Width
	lport.Size.Y = elkport.Height
	lport.Pos.X = elkport.X
	lport.Pos.Y = elkport.Y

	im.portMap[elkport] = lport

	// check if the original port has any outgoing connections to
	// descendants of its node
	connectionsToDescendants := false
	for _, edge := range elkport.OutgoingEdges {
		for _, target := range edge.Targets {
			if elkgraph.IsDescendant(target.ShapeNode(), elkport.Parent) {
				connectionsToDescendants = true
			}
		}
	}

	// there could be yet incoming connections from descendants
	if !connectionsToDescendants {
		for _, edge := range elkport.IncomingEdges {
			for _, source := range edge.Sources {
				if elkgraph.IsDescendant(source.ShapeNode(), elkport.Parent) {
					connectionsToDescendants = true
				}
			}
		}
	}

	// if there are still no connections to descendants, there might yet be
	// inside self loops involved
	if !connectionsToDescendants {
		for _, edge := range elkport.OutgoingEdges {
			if edge.IsSelfloop() && prop.Get(edge, lopts.InsideSelfLoopsYo) {
				connectionsToDescendants = true
				break
			}
		}
	}

	// if we have found connections to / from descendants, mark the port
	// accordingly
	prop.Set(lport, iprops.InsideConnections, connectionsToDescendants)

	// initialize the port's side, offset, and anchor point
	lgraph.InitializePort(lport, portConstraints, layoutDirection, prop.Get(elkport, lopts.PortAnchor))

	// create the port's labels
	for _, elklabel := range elkport.Labels {
		if !prop.Get(elklabel, lopts.NoLayout) && elklabel.Text != "" {
			lport.Labels = append(lport.Labels, transformLabel(elklabel))
		}
	}

	switch layoutDirection {
	case core.DirectionLeft, core.DirectionRight:
		if lport.Side == core.PortSideNorth || lport.Side == core.PortSideSouth {
			graphProperties.Add(lenum.GraphPropertiesNorthSouthPorts)
		}
	case core.DirectionUp, core.DirectionDown:
		if lport.Side == core.PortSideEast || lport.Side == core.PortSideWest {
			graphProperties.Add(lenum.GraphPropertiesNorthSouthPorts)
		}
	}

	return lport
}
