// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/InvertedPortProcessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package hports

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("InvertedPortProcessor", func() lgraph.Processor { return NewInvertedPortProcessor() })
}

// InvertedPortProcessor is the processor InvertedPortProcessor: it inserts
// dummy nodes to cope with inverted ports.
//
// The problem is with edges coming from the left of a node being connected
// to a port that's on its right side, or the other way around. Let a node of
// that kind be in layer i. This processor takes the offending edge and
// connects it to a new dummy node, also in layer i. Finally, the dummy is
// connected with the offending port. This means that once one of these cases
// occurs in the graph, the layering is not proper anymore.
//
// The dummy nodes are long edge dummy nodes. The processor does nothing for
// self loops.
type InvertedPortProcessor struct{}

// NewInvertedPortProcessor is new InvertedPortProcessor().
func NewInvertedPortProcessor() *InvertedPortProcessor { return &InvertedPortProcessor{} }

// Process is process(layeredGraph, monitor).
func (*InvertedPortProcessor) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	// Iterate through the layers and for each layer create a list of dummy
	// nodes that were created, but not yet assigned to the layer (to avoid
	// concurrent modification exceptions)
	var currentLayer *lgraph.Layer
	var unassignedNodes []*lgraph.LNode

	for layerIndex := 0; layerIndex < len(layeredGraph.Layers); layerIndex++ {
		// Update previous and current layers
		previousLayer := currentLayer
		currentLayer = layeredGraph.Layers[layerIndex]

		// If the previous layer had unassigned nodes, assign them now and
		// clear the list
		for _, node := range unassignedNodes {
			node.SetLayer(previousLayer)
		}
		unassignedNodes = unassignedNodes[:0]

		// Iterate through the layer's nodes
		for nodeIndex := 0; nodeIndex < len(currentLayer.Nodes); nodeIndex++ {
			node := currentLayer.Nodes[nodeIndex]
			// Skip dummy nodes
			if node.Type != lgraph.NodeTypeNormal {
				continue
			}

			// Skip nodes whose port sides are not fixed (because in that
			// case, the odd port side problem won't appear)
			if !prop.Get(node, lopts.PortConstraints).IsSideFixed() {
				continue
			}

			// Look for input ports on the right side
			for _, port := range node.PortsByTypeAndSide(lenum.PortTypeInput, core.PortSideEast) {
				// For every edge going into this port, insert dummy nodes
				// (do this using a copy of the current list of edges, since
				// the edges are modified when dummy nodes are created)
				edgeArray := append([]*lgraph.LEdge(nil), port.IncomingEdges...)

				for _, edge := range edgeArray {
					unassignedNodes = createEastPortSideDummies(layeredGraph, port, edge, unassignedNodes)
				}
			}

			// Look for ports on the left side connected to edges going to
			// higher layers
			for _, port := range node.PortsByTypeAndSide(lenum.PortTypeOutput, core.PortSideWest) {
				// For every edge going out of this port, insert dummy nodes
				// (do this using a copy of the current list of edges, since
				// the edges are modified when dummy nodes are created)
				edgeArray := append([]*lgraph.LEdge(nil), port.OutgoingEdges...)

				for _, edge := range edgeArray {
					unassignedNodes = createWestPortSideDummies(layeredGraph, port, edge, unassignedNodes)
				}
			}
		}
	}

	// There may be unassigned nodes left
	for _, node := range unassignedNodes {
		node.SetLayer(currentLayer)
	}
}

// createEastPortSideDummies creates the necessary dummy nodes for an input
// port on the east side of a node, provided that the edge connects two
// different nodes. layerNodeList is the list of unassigned nodes belonging to
// the layer of the node the port belongs to. The new dummy node is added to
// this list and must be assigned to the layer later. The function returns
// the list.
func createEastPortSideDummies(layeredGraph *lgraph.LGraph, eastwardPort *lgraph.LPort, edge *lgraph.LEdge, layerNodeList []*lgraph.LNode) []*lgraph.LNode {
	// Ignore self loops
	if edge.Source.Node() == eastwardPort.Node() {
		return layerNodeList
	}

	// Dummy node in the same layer
	dummy := lgraph.NewLNode(layeredGraph)
	dummy.Type = lgraph.NodeTypeLongEdge
	prop.Set[any](dummy, iprops.Origin, edge)
	prop.Set(dummy, lopts.PortConstraints, core.PortConstraintsFixedPos)
	layerNodeList = append(layerNodeList, dummy)

	dummyInput := lgraph.NewLPort()
	dummyInput.SetNode(dummy)
	dummyInput.SetSide(core.PortSideWest)

	dummyOutput := lgraph.NewLPort()
	dummyOutput.SetNode(dummy)
	dummyOutput.SetSide(core.PortSideEast)

	// Reroute the original edge
	edge.SetTarget(dummyInput)

	// Connect the dummy with the original port
	dummyEdge := lgraph.NewLEdge()
	prop.CopyProperties(dummyEdge, edge)
	prop.Unset(dummyEdge, lopts.JunctionPoints)
	dummyEdge.SetSource(dummyOutput)
	dummyEdge.SetTarget(eastwardPort)

	// Set LONG_EDGE_SOURCE and LONG_EDGE_TARGET properties on the LONG_EDGE
	// dummy
	setLongEdgeSourceAndTarget(dummy, dummyInput, dummyOutput, eastwardPort)

	// Move head labels from the old edge over to the new one
	for labelIndex := 0; labelIndex < len(edge.Labels); {
		label := edge.Labels[labelIndex]
		labelPlacement := prop.Get(label, lopts.EdgeLabelsPlacement)

		if labelPlacement != core.EdgeLabelPlacementHead {
			labelIndex++
			continue
		}
		// Remember which edge the label originally belonged to, unless it
		// already knows
		if !prop.Has(label, iprops.EndLabelEdge) {
			prop.Set(label, iprops.EndLabelEdge, edge)
		}

		edge.Labels = append(edge.Labels[:labelIndex], edge.Labels[labelIndex+1:]...)
		dummyEdge.Labels = append(dummyEdge.Labels, label)
	}
	return layerNodeList
}

// createWestPortSideDummies creates the necessary dummy nodes for an output
// port on the west side of a node, provided that the edge connects two
// different nodes. layerNodeList is the list of unassigned nodes belonging to
// the layer of the node the port belongs to. The new dummy node is added to
// this list and must be assigned to the layer later. The function returns
// the list.
func createWestPortSideDummies(layeredGraph *lgraph.LGraph, westwardPort *lgraph.LPort, edge *lgraph.LEdge, layerNodeList []*lgraph.LNode) []*lgraph.LNode {
	// Ignore self loops
	if edge.Target.Node() == westwardPort.Node() {
		return layerNodeList
	}

	// Dummy node in the same layer
	dummy := lgraph.NewLNode(layeredGraph)
	dummy.Type = lgraph.NodeTypeLongEdge
	prop.Set[any](dummy, iprops.Origin, edge)
	prop.Set(dummy, lopts.PortConstraints, core.PortConstraintsFixedPos)
	layerNodeList = append(layerNodeList, dummy)

	dummyInput := lgraph.NewLPort()
	dummyInput.SetNode(dummy)
	dummyInput.SetSide(core.PortSideWest)

	dummyOutput := lgraph.NewLPort()
	dummyOutput.SetNode(dummy)
	dummyOutput.SetSide(core.PortSideEast)

	// Reroute the original edge
	originalTarget := edge.Target
	edge.SetTarget(dummyInput)

	// Connect the dummy with the original port
	dummyEdge := lgraph.NewLEdge()
	prop.CopyProperties(dummyEdge, edge)
	prop.Unset(dummyEdge, lopts.JunctionPoints)
	dummyEdge.SetSource(dummyOutput)
	dummyEdge.SetTarget(originalTarget)

	// Move any head labels over to the new dummy edge
	for labelIndex := 0; labelIndex < len(edge.Labels); {
		label := edge.Labels[labelIndex]

		if prop.Get(label, lopts.EdgeLabelsPlacement) != core.EdgeLabelPlacementHead {
			labelIndex++
			continue
		}
		// Remember which edge the label belonged to originally
		prop.Set(label, iprops.EndLabelEdge, edge)

		edge.Labels = append(edge.Labels[:labelIndex], edge.Labels[labelIndex+1:]...)
		dummyEdge.Labels = append(dummyEdge.Labels, label)
	}

	// Set LONG_EDGE_SOURCE and LONG_EDGE_TARGET properties on the LONG_EDGE
	// dummy
	setLongEdgeSourceAndTarget(dummy, dummyInput, dummyOutput, westwardPort)
	return layerNodeList
}

// setLongEdgeSourceAndTarget properly sets the properties longEdgeSource and
// longEdgeTarget for the given long edge dummy. This is required for the
// HyperedgeDummyMerger to work correctly. The Java method does not use the
// odd port that prompted the dummy to be created.
func setLongEdgeSourceAndTarget(longEdgeDummy *lgraph.LNode, dummyInputPort, dummyOutputPort, oddPort *lgraph.LPort) {
	// There's exactly one edge connected to the input and output port
	sourcePort := dummyInputPort.IncomingEdges[0].Source
	sourceNode := sourcePort.Node()
	sourceNodeType := sourceNode.Type

	targetPort := dummyOutputPort.OutgoingEdges[0].Target
	targetNode := targetPort.Node()
	targetNodeType := targetNode.Type

	// Set the LONG_EDGE_SOURCE property
	if sourceNodeType == lgraph.NodeTypeLongEdge {
		// The source is a LONG_EDGE node; use its LONG_EDGE_SOURCE
		prop.Set(longEdgeDummy, iprops.LongEdgeSource, prop.Get(sourceNode, iprops.LongEdgeSource))
	} else {
		// The target is the original node; use it
		prop.Set(longEdgeDummy, iprops.LongEdgeSource, sourcePort)
	}

	// Set the LONG_EDGE_TARGET property
	if targetNodeType == lgraph.NodeTypeLongEdge {
		// The target is a LONG_EDGE node; use its LONG_EDGE_TARGET
		prop.Set(longEdgeDummy, iprops.LongEdgeTarget, prop.Get(targetNode, iprops.LongEdgeTarget))
	} else {
		// The target is the original node; use it
		prop.Set(longEdgeDummy, iprops.LongEdgeTarget, targetPort)
	}
}
