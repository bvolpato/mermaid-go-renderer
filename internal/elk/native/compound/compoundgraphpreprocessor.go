// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/compound/CompoundGraphPreprocessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package compound

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("CompoundGraphPreprocessor", func() lgraph.Processor { return NewCompoundGraphPreprocessor() })
}

// CompoundGraphPreprocessor is the processor CompoundGraphPreprocessor: it
// splits each edge that crosses a hierarchy border into one dummy edge for
// each graph on its way, with an external port and an external port dummy
// node at each border. The result is in the property crossHierarchyMap of the
// top-level graph.
//
// The processor assumes that only the top-level graph has external port
// dummy nodes before it runs.
type CompoundGraphPreprocessor struct {
	// crossHierarchyMap maps the original edges to their segments.
	crossHierarchyMap *CrossHierarchyMap
	// dummyNodeMap maps the ports to their dummy nodes in the nested graphs.
	// It is a HashMap of Java: the ports iterate in the order of insertion.
	dummyNodeMap *gwt.Map[*lgraph.LPort, *lgraph.LNode]
}

// NewCompoundGraphPreprocessor is new CompoundGraphPreprocessor().
func NewCompoundGraphPreprocessor() *CompoundGraphPreprocessor {
	return &CompoundGraphPreprocessor{dummyNodeMap: gwt.NewMap[*lgraph.LPort, *lgraph.LNode]()}
}

// Process is process(graph, monitor).
func (p *CompoundGraphPreprocessor) Process(graph *lgraph.LGraph, ctx *lgraph.Context) {
	// This can be a HashMultimap since the values are sorted prior to working
	// with them
	p.crossHierarchyMap = NewCrossHierarchyMap()

	// create new dummy edges at hierarchy bounds and move the labels around
	// accordingly
	p.transformHierarchyEdges(graph, nil)
	p.moveLabelsAndRemoveOriginalEdges(graph)

	p.setSidesOfPortsToSidesOfDummyNodes()

	// Attach cross hierarchy map to the graph and cleanup
	prop.Set[any](graph, iprops.CrossHierarchyMap, p.crossHierarchyMap)
	p.crossHierarchyMap = nil
	p.dummyNodeMap.Clear()
}

// graphPropertiesForChange returns the set of the property graphProperties
// of a graph, to add a constant to it. For a graph without the property,
// elkjs adds the constant to the default value, which each later layout then
// reads. The port does not follow that.
func graphPropertiesForChange(graph *lgraph.LGraph) *prop.EnumSet[lenum.GraphProperties] {
	if graph == nil || !prop.Has(graph, iprops.GraphProperties) {
		lgraph.Bail("compound: a graph without the property graphProperties")
	}
	return prop.Get(graph, iprops.GraphProperties)
}

// lgraphCreateExternalPortDummy is LGraphUtil.createExternalPortDummy. The
// compound graph preprocessor passes no node size and, for inside self
// loops, no port position. Java then fails with a NullPointerException for
// the port constraints that read them.
func lgraphCreateExternalPortDummy(propertyHolder prop.Holder, portConstraints core.PortConstraints, portSide core.PortSide, netFlow int, portPosition, portSize *emath.KVector, layoutDirection core.Direction, layeredGraph *lgraph.LGraph) *lgraph.LNode {
	if portConstraints.IsOrderFixed() {
		usesIndex := portConstraints == core.PortConstraintsFixedOrder && prop.Has(propertyHolder, lopts.PortIndex)
		if !usesIndex && (portPosition == nil || portConstraints.IsRatioFixed()) {
			lgraph.Bail("CompoundGraphPreprocessor: NullPointerException, an external port dummy for the port constraints %v needs the size of the node", portConstraints)
		}
	}
	return lgraph.CreateExternalPortDummy(propertyHolder, portConstraints, portSide, netFlow, nil, portPosition, portSize, layoutDirection, layeredGraph)
}

// setSidesOfPortsToSidesOfDummyNodes ensures that for each dummy node the
// external port and vice versa is set. Since the side of the dummy node has
// also already been calculated, we set this fixed here. Therefore the
// compound node has a fixed side constraint, with some sides still set to
// UNDEFINED. This must be dealt with later.
func (p *CompoundGraphPreprocessor) setSidesOfPortsToSidesOfDummyNodes() {
	p.dummyNodeMap.Each(func(externalPort *lgraph.LPort, dummyNode *lgraph.LNode) bool {
		prop.Set[any](dummyNode, iprops.Origin, externalPort)
		prop.Set(externalPort, iprops.PortDummy, dummyNode)
		prop.Set(externalPort, iprops.InsideConnections, true)
		externalPort.SetSide(prop.Get(dummyNode, iprops.ExtPortSide))
		prop.Set(externalPort.Node(), lopts.PortConstraints, core.PortConstraintsFixedSide)
		graphPropertiesForChange(externalPort.Node().Graph()).Add(lenum.GraphPropertiesNonFreePorts)
		return true
	})
}

// transformHierarchyEdges recursively transforms cross-hierarchy edges into
// sequences of dummy ports and dummy edges. parentNode is the node that
// represents the graph in the upper hierarchy level, or nil for the top-level
// graph. The result has the external ports created to split edges that cross
// the boundary of the parent node.
func (p *CompoundGraphPreprocessor) transformHierarchyEdges(graph *lgraph.LGraph, parentNode *lgraph.LNode) []*extPort {
	// process all children and recurse down to gather their external ports
	var containedExternalPorts []*extPort

	for nodeIndex := 0; nodeIndex < len(graph.LayerlessNodes); nodeIndex++ {
		node := graph.LayerlessNodes[nodeIndex]
		nestedGraph := node.NestedGraph
		if nestedGraph == nil {
			continue
		}
		// recursively process the child graph
		childPorts := p.transformHierarchyEdges(nestedGraph, node)
		containedExternalPorts = append(containedExternalPorts, childPorts...)

		// process inside self loops
		p.processInsideSelfLoops(nestedGraph, node)

		// make sure that all hierarchical ports have had dummy nodes created
		// for them (some will already have been created, but perhaps not all)
		if !prop.Get(nestedGraph, iprops.GraphProperties).Contains(lenum.GraphPropertiesExternalPorts) {
			continue
		}

		// We need the port constraints to position external ports (Issues
		// KIPRA-1528, ELK-48)
		portConstraints := prop.Get(node, lopts.PortConstraints)
		insidePortLabels := prop.Get(node, lopts.PortLabelsPlacement).Contains(core.PortLabelPlacementInside)

		for portIndex := 0; portIndex < len(node.Ports); portIndex++ {
			port := node.Ports[portIndex]
			// Make sure that every port has a dummy node created for it
			dummyNode := p.dummyNodeMap.Get(port)
			if dummyNode == nil {
				dummyNode = lgraphCreateExternalPortDummy(port, portConstraints, port.Side, -port.NetFlow(),
					&emath.KVector{}, &port.Size, prop.Get(nestedGraph, lopts.Direction), nestedGraph)
				prop.Set[any](dummyNode, iprops.Origin, port)
				p.dummyNodeMap.Put(port, dummyNode)
				nestedGraph.LayerlessNodes = append(nestedGraph.LayerlessNodes, dummyNode)
			}

			// We need to reserve space for external port labels by adding
			// those labels to our dummy nodes
			dummyNodePort := dummyNode.Ports[0]

			for _, extPortLabel := range port.Labels {
				dummyPortLabel := lgraph.NewLLabel("")
				dummyPortLabel.Size.X = extPortLabel.Size.X
				dummyPortLabel.Size.Y = extPortLabel.Size.Y
				dummyNodePort.Labels = append(dummyNodePort.Labels, dummyPortLabel)

				// If port labels are placed outside, modify the size. At this
				// point, the port's side may not be known yet if port
				// constraints are free. If they are, however, we know that the
				// port will end up on either the east or west side. (see
				// #596). But if the port labels are fixed, we should consider
				// the part that is inside the node.
				if !insidePortLabels {
					side := port.Side
					insidePart := 0.0
					if portLabelPlacementIsFixed(prop.Get(node, lopts.PortLabelsPlacement)) {
						// We use 0 as port border offset here, as we only
						// want the label part that is inside the node "after"
						// the port.
						insidePart = computeInsidePart(&extPortLabel.Pos, &extPortLabel.Size, &port.Size, 0, side)
					}
					if portConstraints == core.PortConstraintsFree || side == core.PortSideEast || side == core.PortSideWest {
						dummyPortLabel.Size.X = insidePart
					} else {
						dummyPortLabel.Size.Y = insidePart
					}
				}
			}
		}
	}

	// this will be the list of external ports we will export
	var exportedExternalPorts []*extPort

	// process the cross-hierarchy edges connected to the inside of the child
	// nodes
	exportedExternalPorts = p.processInnerHierarchicalEdgeSegments(graph, parentNode, containedExternalPorts, exportedExternalPorts)

	// process the cross-hierarchy edges connected to the outside of the
	// parent node
	if parentNode != nil {
		exportedExternalPorts = p.processOuterHierarchicalEdgeSegments(graph, parentNode, exportedExternalPorts)
	}

	return exportedExternalPorts
}

// moveLabelsAndRemoveOriginalEdges moves all labels of the original edges to
// the appropriate dummy edges and removes the original edges from the graph.
// graph is the top-level graph.
func (p *CompoundGraphPreprocessor) moveLabelsAndRemoveOriginalEdges(graph *lgraph.LGraph) {
	comparator := NewCrossHierarchyEdgeComparator(graph)
	for _, origEdge := range p.crossHierarchyMap.KeySet() {
		// if the original edge had any labels, we need to move them to the
		// newly introduced edge segments
		if len(origEdge.Labels) > 0 {
			// retrieve and sort the edge segments introduced for the original
			// edge
			edgeSegments := p.crossHierarchyMap.Get(origEdge)
			gwt.SortFunc(edgeSegments, comparator.Compare)

			// iterate over the labels and move them to the edge segments
			for labelIndex := 0; labelIndex < len(origEdge.Labels); {
				currLabel := origEdge.Labels[labelIndex]

				// find the index of the dummy edge we will move the label to
				targetDummyEdgeIndex := -1
				switch prop.Get(currLabel, lopts.EdgeLabelsPlacement) {
				case core.EdgeLabelPlacementHead:
					targetDummyEdgeIndex = len(edgeSegments) - 1
				case core.EdgeLabelPlacementCenter:
					targetDummyEdgeIndex = getShallowestEdgeSegment(edgeSegments)
				case core.EdgeLabelPlacementTail:
					targetDummyEdgeIndex = 0
				default:
					// we have no idea what to do with the label, so ignore it
				}

				// move the label if we were lucky enough to find a new home
				// for it
				if targetDummyEdgeIndex == -1 {
					labelIndex++
					continue
				}
				targetSegment := edgeSegments[targetDummyEdgeIndex]
				targetEdge := targetSegment.Edge()
				targetEdge.Labels = append(targetEdge.Labels, currLabel)
				graphPropertiesForChange(targetEdge.Source.Node().Graph()).Add(lenum.GraphPropertiesEndLabels)
				graphPropertiesForChange(targetEdge.Source.Node().Graph()).Add(lenum.GraphPropertiesCenterLabels)

				origEdge.Labels = append(origEdge.Labels[:labelIndex], origEdge.Labels[labelIndex+1:]...)
				prop.Set(currLabel, iprops.OriginalLabelEdge, origEdge)
			}
		}

		// remove original edge
		origEdge.SetSource(nil)
		origEdge.SetTarget(nil)
	}
}

// getShallowestEdgeSegment determines the index of the shallowest edge
// segment in a sorted list of the segments of one edge. The shallowest
// segment is the last edge segment with the type OUTPUT. If no such element
// exists, the first segment is the shallowest segment.
func getShallowestEdgeSegment(edgeSegments []*CrossHierarchyEdge) int {
	result := -1
	index := 0

	for _, crossHierarchyEdge := range edgeSegments {
		if crossHierarchyEdge.Type() == lenum.PortTypeInput {
			// We have an edge pointing downwards here. If this is the first
			// segment we take this element. If there was a segment before, we
			// take that one
			if index == 0 {
				result = 0
			} else {
				result = index - 1
			}
			break
		} else if index == len(edgeSegments)-1 {
			// If this is the last segment and we didn't find a descending
			// edge, this edge is always ascending. Place the label at the
			// last segment here.
			result = index
		}
		index++
	}

	return result
}

////////////////////////////////////////////////////////////////////////////
// Inner Hierarchical Edge Segment Processing

// processInnerHierarchicalEdgeSegments deals with the inner segments of
// hierarchical edges by breaking them between external ports. For each
// external port contained in child nodes, this method adds appropriate new
// external ports and / or dummy edges. It returns exportedExternalPorts with
// the external ports that it creates.
func (p *CompoundGraphPreprocessor) processInnerHierarchicalEdgeSegments(graph *lgraph.LGraph, parentNode *lgraph.LNode, containedExternalPorts, exportedExternalPorts []*extPort) []*extPort {
	// we remember the ports and the dummy nodes we create to add them to the
	// graph afterwards (this is not strictly necessary, but allows us to
	// reuse methods we also use for outer hierarchy edge segments)
	var createdExternalPorts []*extPort

	// iterate over the list of contained external ports
	for portIndex := 0; portIndex < len(containedExternalPorts); portIndex++ {
		externalPort := containedExternalPorts[portIndex]
		var currentExternalPort *extPort

		// we process output ports and input ports a bit differently
		if externalPort.portType == lenum.PortTypeOutput {
			// iterate over the port's original edges
			for edgeIndex := 0; edgeIndex < len(externalPort.origEdges); edgeIndex++ {
				outEdge := externalPort.origEdges[edgeIndex]
				// at this point, we distinguish three cases:
				//  1. The edge connects to a direct child of the parent node.
				//  2. connects two external ports of direct children of the
				//     parent node.
				//  3. The edge comes from a child node and either connects
				//     directly to the parent node or leaves it.
				targetNode := outEdge.Target.Node()
				if targetNode.Graph() == graph {
					// case 1: edge connects to a direct child
					p.connectChild(graph, externalPort, outEdge, externalPort.dummyPort, outEdge.Target)
				} else if parentNode == nil || lgraph.IsDescendant(targetNode, parentNode) {
					// case 2: edge connects two direct children
					p.connectSiblings(graph, externalPort, containedExternalPorts, outEdge)
				} else {
					// case 3: edge connects to parent node or to the outside
					// world
					newExternalPort := p.introduceHierarchicalEdgeSegment(graph, parentNode, outEdge, externalPort.dummyPort, lenum.PortTypeOutput, currentExternalPort)
					if newExternalPort != currentExternalPort {
						createdExternalPorts = append(createdExternalPorts, newExternalPort)
					}

					// the port is our new current external port if it is
					// exported
					if newExternalPort.exported {
						currentExternalPort = newExternalPort
					}
				}
			}
		} else {
			// iterate over the port's original edges
			for edgeIndex := 0; edgeIndex < len(externalPort.origEdges); edgeIndex++ {
				inEdge := externalPort.origEdges[edgeIndex]
				// at this point, we distinguish three cases:
				//  1. The edge comes from a direct child of the parent node.
				//  2. connects two external ports of direct children of the
				//     parent node. (we don't deal with that case here; the
				//     code that handles output ports above does that)
				//  3. The edge comes from the parent node or from the
				//     outside.
				sourceNode := inEdge.Source.Node()
				if sourceNode.Graph() == graph {
					// case 1: edge comes from a direct child
					p.connectChild(graph, externalPort, inEdge, inEdge.Source, externalPort.dummyPort)
				} else if parentNode == nil || lgraph.IsDescendant(sourceNode, parentNode) {
					// case 2: edge connects two direct children; this case is
					// handled in the code for output ports above, so there's
					// nothing to do here
					continue
				} else {
					// case 3: edge comes from the parent node or from the
					// outside
					newExternalPort := p.introduceHierarchicalEdgeSegment(graph, parentNode, inEdge, externalPort.dummyPort, lenum.PortTypeInput, currentExternalPort)
					if newExternalPort != currentExternalPort {
						createdExternalPorts = append(createdExternalPorts, newExternalPort)
					}

					// the port is our new current external port if it is
					// exported
					if newExternalPort.exported {
						currentExternalPort = newExternalPort
					}
				}
			}
		}
	}

	// add dummy nodes and exported external ports
	return addCreatedExternalPorts(graph, createdExternalPorts, exportedExternalPorts)
}

// addCreatedExternalPorts adds the dummy nodes of the created external ports
// to the graph, and the exported ones to exportedExternalPorts. Both segment
// processing methods of the Java class end with this loop.
func addCreatedExternalPorts(graph *lgraph.LGraph, createdExternalPorts, exportedExternalPorts []*extPort) []*extPort {
	for _, externalPort := range createdExternalPorts {
		if !containsNode(graph.LayerlessNodes, externalPort.dummyNode) {
			graph.LayerlessNodes = append(graph.LayerlessNodes, externalPort.dummyNode)
		}

		if externalPort.exported {
			exportedExternalPorts = append(exportedExternalPorts, externalPort)
		}
	}
	return exportedExternalPorts
}

func containsNode(nodes []*lgraph.LNode, node *lgraph.LNode) bool {
	for _, n := range nodes {
		if n == node {
			return true
		}
	}
	return false
}

func containsEdge(edges []*lgraph.LEdge, edge *lgraph.LEdge) bool {
	for _, e := range edges {
		if e == edge {
			return true
		}
	}
	return false
}

// connectChild connects an external port with a child node of the given
// graph. To this end, a new dummy edge is inserted and associated with the
// original hierarchy-crossing edge in the cross hierarchy map.
func (p *CompoundGraphPreprocessor) connectChild(graph *lgraph.LGraph, externalPort *extPort, origEdge *lgraph.LEdge, sourcePort, targetPort *lgraph.LPort) {
	// add new dummy edge and connect properly
	dummyEdge := createDummyEdge(graph, origEdge)
	dummyEdge.SetSource(sourcePort)
	dummyEdge.SetTarget(targetPort)

	p.crossHierarchyMap.Put(origEdge, NewCrossHierarchyEdge(dummyEdge, graph, externalPort.portType))
}

// connectSiblings connects external ports of two child nodes of the given
// graph. To this end, the provided list of external ports is searched for
// the counterpart of the provided external output port, and a new dummy edge
// is created to connect the two. The dummy edge is associated with the
// original hierarchy-crossing edge in the cross hierarchy map.
func (p *CompoundGraphPreprocessor) connectSiblings(graph *lgraph.LGraph, externalOutputPort *extPort, containedExternalPorts []*extPort, origEdge *lgraph.LEdge) {
	// find the opposite external port
	var targetExternalPort *extPort
	for _, externalPort2 := range containedExternalPorts {
		if externalPort2 != externalOutputPort && containsEdge(externalPort2.origEdges, origEdge) {
			targetExternalPort = externalPort2
			break
		}
	}
	if targetExternalPort == nil {
		lgraph.Bail("CompoundGraphPreprocessor: NullPointerException, an edge between two child nodes has no external target port")
	}

	// add new dummy edge and connect properly
	dummyEdge := createDummyEdge(graph, origEdge)
	dummyEdge.SetSource(externalOutputPort.dummyPort)
	dummyEdge.SetTarget(targetExternalPort.dummyPort)

	p.crossHierarchyMap.Put(origEdge, NewCrossHierarchyEdge(dummyEdge, graph, externalOutputPort.portType))
}

////////////////////////////////////////////////////////////////////////////
// Outer Hierarchical Edge Segment Processing

// processOuterHierarchicalEdgeSegments deals with the outer segments of
// hierarchical edges by breaking them at their source or target. For each
// hierarchical edge that starts or ends at one of the graph's children, this
// method adds appropriate new external ports and / or dummy edges to the
// graph. It returns exportedExternalPorts with the external ports that it
// creates.
func (p *CompoundGraphPreprocessor) processOuterHierarchicalEdgeSegments(graph *lgraph.LGraph, parentNode *lgraph.LNode, exportedExternalPorts []*extPort) []*extPort {
	// we need to remember the ports and the dummy nodes we create to add them
	// to the graph afterwards (to avoid concurrent modification exceptions)
	var createdExternalPorts []*extPort

	// iterate over all ports of the graph's child nodes
	for nodeIndex := 0; nodeIndex < len(graph.LayerlessNodes); nodeIndex++ {
		childNode := graph.LayerlessNodes[nodeIndex]
		for portIndex := 0; portIndex < len(childNode.Ports); portIndex++ {
			childPort := childNode.Ports[portIndex]
			// we treat outgoing and incoming edges separately
			var currentExternalOutputPort *extPort
			for _, outEdge := range toEdgeArray(childPort.OutgoingEdges) {
				if !lgraph.IsDescendant(outEdge.Target.Node(), parentNode) {
					// the edge goes to the outside or to the parent node
					// itself, so create an external port if necessary and
					// introduce a new dummy edge
					newExternalPort := p.introduceHierarchicalEdgeSegment(graph, parentNode, outEdge, outEdge.Source, lenum.PortTypeOutput, currentExternalOutputPort)
					if newExternalPort != currentExternalOutputPort {
						createdExternalPorts = append(createdExternalPorts, newExternalPort)
					}

					// the port is our new current external port if it is
					// exported
					if newExternalPort.exported {
						currentExternalOutputPort = newExternalPort
					}
				}
			}

			var currentExternalInputPort *extPort
			for _, inEdge := range toEdgeArray(childPort.IncomingEdges) {
				if !lgraph.IsDescendant(inEdge.Source.Node(), parentNode) {
					// the edge comes from the outside or from the parent
					// node itself, so create an external port if necessary
					// and introduce a new dummy edge
					newExternalPort := p.introduceHierarchicalEdgeSegment(graph, parentNode, inEdge, inEdge.Target, lenum.PortTypeInput, currentExternalInputPort)
					if newExternalPort != currentExternalInputPort {
						createdExternalPorts = append(createdExternalPorts, newExternalPort)
					}

					// the port is our new current external port if it is
					// exported
					if newExternalPort.exported {
						currentExternalInputPort = newExternalPort
					}
				}
			}
		}
	}

	// add dummy nodes and exported external ports
	return addCreatedExternalPorts(graph, createdExternalPorts, exportedExternalPorts)
}

// toEdgeArray is LGraphUtil.toEdgeArray(edges): a copy of the list.
//
// TODO(core): move to lgraph.
func toEdgeArray(edges []*lgraph.LEdge) []*lgraph.LEdge {
	return append([]*lgraph.LEdge(nil), edges...)
}

////////////////////////////////////////////////////////////////////////////
// Inside Self Loop Processing

func (p *CompoundGraphPreprocessor) processInsideSelfLoops(nestedGraph *lgraph.LGraph, node *lgraph.LNode) {
	// Check if inside self loops are enabled for the node
	if !prop.Get(node, lopts.InsideSelfLoopsActivate) {
		return
	}

	// Iterate over the edges and look for an inside self loop
	for portIndex := 0; portIndex < len(node.Ports); portIndex++ {
		lport := node.Ports[portIndex]
		// Avoid ConcurrentModificationExceptions
		outEdges := toEdgeArray(lport.OutgoingEdges)

		for _, outEdge := range outEdges {
			isSelfLoop := outEdge.Target.Node() == node
			isInsideSelfLoop := isSelfLoop && prop.Get(outEdge, lopts.InsideSelfLoopsYo)

			if !isInsideSelfLoop {
				continue
			}
			// Check if the ports have already been transformed into external
			// port dummies
			sourcePort := outEdge.Source
			sourceExtPortDummy := p.dummyNodeMap.Get(sourcePort)
			if sourceExtPortDummy == nil {
				sourceExtPortDummy = lgraphCreateExternalPortDummy(sourcePort, core.PortConstraintsFree, sourcePort.Side, -1,
					nil, &sourcePort.Size, prop.Get(nestedGraph, lopts.Direction), nestedGraph)
				prop.Set[any](sourceExtPortDummy, iprops.Origin, sourcePort)
				p.dummyNodeMap.Put(sourcePort, sourceExtPortDummy)
				nestedGraph.LayerlessNodes = append(nestedGraph.LayerlessNodes, sourceExtPortDummy)
			}

			targetPort := outEdge.Target
			targetExtPortDummy := p.dummyNodeMap.Get(targetPort)
			if targetExtPortDummy == nil {
				targetExtPortDummy = lgraphCreateExternalPortDummy(targetPort, core.PortConstraintsFree, targetPort.Side, 1,
					nil, &targetPort.Size, prop.Get(nestedGraph, lopts.Direction), nestedGraph)
				prop.Set[any](targetExtPortDummy, iprops.Origin, targetPort)
				p.dummyNodeMap.Put(targetPort, targetExtPortDummy)
				nestedGraph.LayerlessNodes = append(nestedGraph.LayerlessNodes, targetExtPortDummy)
			}

			// Create a new dummy edge
			dummyEdge := createDummyEdge(nestedGraph, outEdge)
			dummyEdge.SetSource(sourceExtPortDummy.Ports[0])
			dummyEdge.SetTarget(targetExtPortDummy.Ports[0])

			// Remember the new edge
			p.crossHierarchyMap.Put(outEdge, NewCrossHierarchyEdge(dummyEdge, nestedGraph, lenum.PortTypeOutput))

			graphPropertiesForChange(nestedGraph).Add(lenum.GraphPropertiesExternalPorts)
		}
	}
}

////////////////////////////////////////////////////////////////////////////
// General Hierarchical Edge Segment Processing

// introduceHierarchicalEdgeSegment does the actual work of creating a new
// hierarchical edge segment between an external port and a given opposite
// port. The external port used for the segment is returned.
//
// The method first decides on an external port to use for the segment. If
// the default external port passed to the method is not nil and if external
// ports are to be merged in the current graph, the default external port is
// reused. An exception are segments that start or end in the parent node;
// each such segment gets its own external port.
//
// If a new external port is created, the method also creates a dummy node
// for it as well as an actual port on the parent node, if no such port
// already exists, as well as a dummy edge for the connection. Thus, the newly
// created external port has everything it needs to be properly represented
// and initialized.
//
// The original edge is added to the list of original edges in the external
// port used for the segment. The dummy edge is associated with the original
// hierarchy-crossing edge in the cross hierarchy map.
func (p *CompoundGraphPreprocessor) introduceHierarchicalEdgeSegment(graph *lgraph.LGraph, parentNode *lgraph.LNode, origEdge *lgraph.LEdge, oppositePort *lgraph.LPort, portType lenum.PortType, defaultExternalPort *extPort) *extPort {
	// check if external ports are to be merged
	mergeExternalPorts := prop.Get(graph, lopts.MergeHierarchyEdges)

	// check if the edge connects to the parent node instead of to the outside
	// world; if so, the parentEndPort will be non-nil
	var parentEndPort *lgraph.LPort
	if portType == lenum.PortTypeInput && origEdge.Source.Node() == parentNode {
		parentEndPort = origEdge.Source
	} else if portType == lenum.PortTypeOutput && origEdge.Target.Node() == parentNode {
		parentEndPort = origEdge.Target
	}

	// only create a new external port if the current one is nil or if ports
	// are not to be merged or if the connection actually ends at the parent
	// node
	externalPort := defaultExternalPort
	if externalPort == nil || !mergeExternalPorts || parentEndPort != nil {
		// create a dummy node that will represent the external port
		externalPortSide := core.PortSideUndefined
		if parentEndPort != nil {
			externalPortSide = parentEndPort.Side
		} else if prop.Get(parentNode, lopts.PortConstraints).IsSideFixed() {
			// We try to infer the port side from the port type if its node
			// has its port constraints set to at least FIXED_SIDE; this may
			// produce strange effects, so the safest thing is for people to
			// set compound node port constraints to FREE
			if portType == lenum.PortTypeInput {
				externalPortSide = core.PortSideWest
			} else {
				externalPortSide = core.PortSideEast
			}
		}
		dummyNode := p.createExternalPortDummy(graph, parentNode, portType, externalPortSide, origEdge)

		// create a dummy edge to be connected to the port
		dummyEdge := createDummyEdge(parentNode.Graph(), origEdge)

		if portType == lenum.PortTypeInput {
			// if the external port is an input port, the source of the edge
			// must be connected to the new dummy node
			dummyEdge.SetSource(dummyNode.Ports[0])
			dummyEdge.SetTarget(oppositePort)
		} else {
			// if the external port is an output port, the target of the edge
			// must be connected to the new dummy node
			dummyEdge.SetSource(oppositePort)
			dummyEdge.SetTarget(dummyNode.Ports[0])
		}

		// create the external port (the port is to be exported if the
		// connection is not just to the parent node)
		externalPort = newExtPort(origEdge, dummyEdge, dummyNode, originPort(dummyNode), portType, parentEndPort == nil)
	} else {
		// we use an existing external port, so simply add the original edge
		// to its list of original edges
		externalPort.origEdges = append(externalPort.origEdges, origEdge)

		// merge the properties of the original edges
		thickness := max(prop.Get(externalPort.newEdge, lopts.EdgeThickness), prop.Get(origEdge, lopts.EdgeThickness))
		prop.Set(externalPort.newEdge, lopts.EdgeThickness, thickness)
	}

	p.crossHierarchyMap.Put(origEdge, NewCrossHierarchyEdge(externalPort.newEdge, graph, portType))

	return externalPort
}

// createDummyEdge creates and initializes a new dummy edge for the given
// original hierarchy-crossing edge. All that remains to be done afterwards is
// to properly connect the edge. The Java method does not use the graph.
func createDummyEdge(graph *lgraph.LGraph, origEdge *lgraph.LEdge) *lgraph.LEdge {
	dummyEdge := lgraph.NewLEdge()
	prop.CopyProperties(dummyEdge, origEdge)
	prop.Unset(dummyEdge, lopts.JunctionPoints)
	return dummyEdge
}

// calculateNetFlow is adapted from ElkGraphImporter. It counts how many
// edges want the port to be an output port of the parent and how many want
// it to be an input port. An edge coming into the port from the inside votes
// for the port to be an output port of the parent, as does an edge leaving
// the port for the outside. The result is the net flow as fed into
// createExternalPortDummy.
func calculateNetFlow(port *lgraph.LPort) int {
	node := port.Node()
	insideSelfLoopsEnabled := prop.Get(node, lopts.InsideSelfLoopsActivate)

	outputPortVote, inputPortVote := 0, 0

	// Iterate over outgoing edges
	for _, outgoingEdge := range port.OutgoingEdges {
		isSelfLoop := outgoingEdge.IsSelfLoop()
		isInsideSelfLoop := isSelfLoop && insideSelfLoopsEnabled && prop.Get(outgoingEdge, lopts.InsideSelfLoopsYo)
		targetNode := outgoingEdge.Target.Node()

		if isSelfLoop && isInsideSelfLoop {
			inputPortVote++
		} else if isSelfLoop && !isInsideSelfLoop {
			outputPortVote++
		} else if targetNode.Graph().ParentNode == node {
			inputPortVote++
		} else {
			outputPortVote++
		}
	}

	// Iterate over incoming edges
	for _, incomingEdge := range port.IncomingEdges {
		isSelfLoop := incomingEdge.IsSelfLoop()
		isInsideSelfLoop := isSelfLoop && insideSelfLoopsEnabled && prop.Get(incomingEdge, lopts.InsideSelfLoopsYo)
		sourceNode := incomingEdge.Source.Node()

		if isSelfLoop && isInsideSelfLoop {
			outputPortVote++
		} else if isSelfLoop && !isInsideSelfLoop {
			inputPortVote++
		} else if sourceNode.Graph().ParentNode == node {
			outputPortVote++
		} else {
			inputPortVote++
		}
	}

	return outputPortVote - inputPortVote
}

// createExternalPortDummy retrieves a dummy node to be used to represent a
// new external port of the parent node and to connect a new segment of the
// given hierarchical edge to. A proper dummy node might already have been
// created; if so, that one is returned.
func (p *CompoundGraphPreprocessor) createExternalPortDummy(graph *lgraph.LGraph, parentNode *lgraph.LNode, portType lenum.PortType, portSide core.PortSide, edge *lgraph.LEdge) *lgraph.LNode {
	var dummyNode *lgraph.LNode

	// find the port on the outside of its parent node that the edge connects
	// to
	outsidePort := edge.Target
	if portType == lenum.PortTypeInput {
		outsidePort = edge.Source
	}
	layoutDirection := lgraph.GetDirection(graph)

	// check if the edge connects to the parent node or to something way
	// outside...
	if outsidePort.Node() == parentNode {
		// we need to check if a dummy node has already been created for the
		// port
		dummyNode = p.dummyNodeMap.Get(outsidePort)
		if dummyNode == nil {
			// Ticket #160 explains why we need to pass on a position here.
			// While that works fine to determine port orders, the exact port
			// positions might yet need to be adjusted for some inside
			// paddings
			dummyNode = lgraphCreateExternalPortDummy(outsidePort, prop.Get(parentNode, lopts.PortConstraints), portSide,
				calculateNetFlow(outsidePort), &outsidePort.Pos, &outsidePort.Size, layoutDirection, graph)
			prop.Set[any](dummyNode, iprops.Origin, outsidePort)
			p.dummyNodeMap.Put(outsidePort, dummyNode)
		}
	} else {
		// We create a new dummy node in any case, and since there is no port
		// yet we have to create one as well. We do need to pass a position
		// vector to keep an NPE from being thrown (#160), but it is
		// questionable whether this part of the code should actually be used
		// with anything other than fixed port constraints.
		netFlow := 1
		if portType == lenum.PortTypeInput {
			netFlow = -1
		}
		// A dummy port is not considered to have a size (#766)
		dummyNode = lgraphCreateExternalPortDummy(createExternalPortProperties(graph), prop.Get(parentNode, lopts.PortConstraints), portSide,
			netFlow, &emath.KVector{}, emath.NewKVector(0, 0), layoutDirection, graph)
		dummyPort := createPortForDummy(dummyNode, parentNode, portType)
		prop.Set[any](dummyNode, iprops.Origin, dummyPort)
		p.dummyNodeMap.Put(dummyPort, dummyNode)
	}

	// set a few graph properties
	graphPropertiesForChange(graph).Add(lenum.GraphPropertiesExternalPorts)
	if prop.Get(graph, lopts.PortConstraints).IsSideFixed() {
		prop.Set(graph, lopts.PortConstraints, core.PortConstraintsFixedSide)
	} else {
		prop.Set(graph, lopts.PortConstraints, core.PortConstraintsFree)
	}

	return dummyNode
}

// createExternalPortProperties creates suitable port properties for dummy
// external ports.
func createExternalPortProperties(graph *lgraph.LGraph) prop.Holder {
	propertyHolder := &prop.MapHolder{}
	// FIXME No idea why ...
	offset := prop.Get(graph, lopts.SpacingEdgeEdge) / 2
	prop.Set(propertyHolder, lopts.PortBorderOffset, offset)
	return propertyHolder
}

// createPortForDummy creates a port for an existing external port dummy node
// on the parent node to which it is attached.
func createPortForDummy(dummyNode, parentNode *lgraph.LNode, portType lenum.PortType) *lgraph.LPort {
	graph := parentNode.Graph()
	layoutDirection := lgraph.GetDirection(graph)
	port := lgraph.NewLPort()
	port.SetNode(parentNode)
	switch portType {
	case lenum.PortTypeInput:
		port.SetSide(core.PortSideFromDirection(layoutDirection).Opposed())
	case lenum.PortTypeOutput:
		port.SetSide(core.PortSideFromDirection(layoutDirection))
	}
	prop.Set(port, lopts.PortBorderOffset, prop.Get(dummyNode, lopts.PortBorderOffset))
	return port
}

// portLabelPlacementIsFixed is PortLabelPlacement.isFixed(placement): the
// labels are placed neither inside nor outside.
//
// TODO(core): move to core.
func portLabelPlacementIsFixed(placement *prop.EnumSet[core.PortLabelPlacement]) bool {
	return !placement.Contains(core.PortLabelPlacementInside) && !placement.Contains(core.PortLabelPlacementOutside)
}

// computeInsidePart is ElkUtil.computeInsidePart(labelPosition, labelSize,
// portSize, portBorderOffset, portSide): the part of a port label that is
// inside the node.
//
// TODO(core): move to a package of ElkUtil.
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

////////////////////////////////////////////////////////////////////////////
// Class ExternalPort

// extPort is the class ExternalPort: an internal representation for
// external ports. It passes information gathered on one hierarchy level to
// the containing hierarchy level. An instance is created whenever a
// cross-hierarchy edge crosses the hierarchy bounds of a parent node; the
// instance represents the split point of the edge.
type extPort struct {
	// origEdges is the list of original edges for which the port is created.
	origEdges []*lgraph.LEdge
	// newEdge is the new edge by which the original edge is replaced.
	newEdge *lgraph.LEdge
	// dummyNode is the dummy node used by the algorithm as representative
	// for the external port.
	dummyNode *lgraph.LNode
	// dummyPort is the dummy port used by the algorithm as representative
	// for the external port.
	dummyPort *lgraph.LPort
	// portType is the flow direction: input or output.
	portType lenum.PortType
	// exported tells whether the external port will be exported to the
	// outside or not. (it will not be exported if the port was introduced
	// for connections from an inside node to its parent)
	exported bool
}

// newExtPort is new ExternalPort(origEdge, newEdge, dummyNode,
// dummyPort, portType, exported).
func newExtPort(origEdge, newEdge *lgraph.LEdge, dummyNode *lgraph.LNode, dummyPort *lgraph.LPort, portType lenum.PortType, exported bool) *extPort {
	return &extPort{
		origEdges: []*lgraph.LEdge{origEdge},
		newEdge:   newEdge,
		dummyNode: dummyNode,
		dummyPort: dummyPort,
		portType:  portType,
		exported:  exported,
	}
}
