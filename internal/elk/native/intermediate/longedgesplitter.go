// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/LongEdgeSplitter.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package intermediate has intermediate processors of the layered algorithm
// that need no helper package: edge reversal, layer constraints, long edges,
// port sides, layer sizes and the graph transformer.
package intermediate

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("LongEdgeSplitter", func() lgraph.Processor { return NewLongEdgeSplitter() })
}

// LongEdgeSplitter is the processor LongEdgeSplitter: it splits each edge
// that spans more than one layer with a dummy node in each layer between.
type LongEdgeSplitter struct{}

// NewLongEdgeSplitter is new LongEdgeSplitter().
func NewLongEdgeSplitter() *LongEdgeSplitter { return &LongEdgeSplitter{} }

// Process is process(layeredGraph, monitor).
func (*LongEdgeSplitter) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	if len(layeredGraph.Layers) <= 2 {
		return
	}
	// The Java code walks the layer list with a list iterator, and the dummy
	// nodes that a layer gets are visited when the walk reaches that layer.
	for layerIndex := 0; layerIndex+1 < len(layeredGraph.Layers); layerIndex++ {
		layer := layeredGraph.Layers[layerIndex]
		nextLayer := layeredGraph.Layers[layerIndex+1]
		for nodeIndex := 0; nodeIndex < len(layer.Nodes); nodeIndex++ {
			node := layer.Nodes[nodeIndex]
			for portIndex := 0; portIndex < len(node.Ports); portIndex++ {
				port := node.Ports[portIndex]
				for edgeIndex := 0; edgeIndex < len(port.OutgoingEdges); edgeIndex++ {
					edge := port.OutgoingEdges[edgeIndex]
					targetPort := edge.Target
					targetLayer := targetPort.Node().Layer()
					if targetLayer != layer && targetLayer != nextLayer {
						SplitEdge(edge, createDummyNode(layeredGraph, nextLayer, edge))
					}
				}
			}
		}
	}
}

func createDummyNode(layeredGraph *lgraph.LGraph, targetLayer *lgraph.Layer, edgeToSplit *lgraph.LEdge) *lgraph.LNode {
	dummyNode := lgraph.NewLNode(layeredGraph)
	dummyNode.Type = lgraph.NodeTypeLongEdge
	prop.Set[any](dummyNode, iprops.Origin, edgeToSplit)
	prop.Set(dummyNode, lopts.PortConstraints, core.PortConstraintsFixedPos)
	dummyNode.SetLayer(targetLayer)
	return dummyNode
}

// SplitEdge is LongEdgeSplitter.splitEdge(edge, dummyNode): it puts the dummy
// node on the edge and returns the new edge that leaves the dummy node.
func SplitEdge(edge *lgraph.LEdge, dummyNode *lgraph.LNode) *lgraph.LEdge {
	oldEdgeTarget := edge.Target

	// Set thickness of the edge
	thickness := prop.Get(edge, lopts.EdgeThickness)
	if thickness < 0 {
		thickness = 0
		prop.Set(edge, lopts.EdgeThickness, thickness)
	}
	dummyNode.Size.Y = thickness
	portPos := math.Floor(thickness / 2)

	// Create dummy input and output ports
	dummyInput := lgraph.NewLPort()
	dummyInput.SetSide(core.PortSideWest)
	dummyInput.SetNode(dummyNode)
	dummyInput.Pos.Y = portPos

	dummyOutput := lgraph.NewLPort()
	dummyOutput.SetSide(core.PortSideEast)
	dummyOutput.SetNode(dummyNode)
	dummyOutput.Pos.Y = portPos

	edge.SetTarget(dummyInput)

	// Create a dummy edge
	dummyEdge := lgraph.NewLEdge()
	prop.CopyProperties(dummyEdge, edge)
	prop.Unset(dummyEdge, lopts.JunctionPoints)
	dummyEdge.SetSource(dummyOutput)
	dummyEdge.SetTarget(oldEdgeTarget)

	setDummyNodeProperties(dummyNode, edge, dummyEdge)
	moveHeadLabels(edge, dummyEdge)

	return dummyEdge
}

func setDummyNodeProperties(dummyNode *lgraph.LNode, inEdge, outEdge *lgraph.LEdge) {
	inEdgeSourceNode := inEdge.Source.Node()
	outEdgeTargetNode := outEdge.Target.Node()

	switch {
	case inEdgeSourceNode.Type == lgraph.NodeTypeLongEdge:
		// The incoming edge originates from a long edge dummy node, so we
		// can just copy its properties
		prop.Set(dummyNode, iprops.LongEdgeSource, prop.Get(inEdgeSourceNode, iprops.LongEdgeSource))
		prop.Set(dummyNode, iprops.LongEdgeTarget, prop.Get(inEdgeSourceNode, iprops.LongEdgeTarget))
		prop.Set(dummyNode, iprops.LongEdgeHasLabelDummies, prop.Get(inEdgeSourceNode, iprops.LongEdgeHasLabelDummies))
	case inEdgeSourceNode.Type == lgraph.NodeTypeLabel:
		// The incoming edge originates from a label dummy node, so we can
		// just copy its properties
		prop.Set(dummyNode, iprops.LongEdgeSource, prop.Get(inEdgeSourceNode, iprops.LongEdgeSource))
		prop.Set(dummyNode, iprops.LongEdgeTarget, prop.Get(inEdgeSourceNode, iprops.LongEdgeTarget))
		prop.Set(dummyNode, iprops.LongEdgeHasLabelDummies, true)
	case outEdgeTargetNode.Type == lgraph.NodeTypeLabel:
		// The outgoing edge points to a label dummy node, so we can just copy
		// its properties
		prop.Set(dummyNode, iprops.LongEdgeSource, prop.Get(outEdgeTargetNode, iprops.LongEdgeSource))
		prop.Set(dummyNode, iprops.LongEdgeTarget, prop.Get(outEdgeTargetNode, iprops.LongEdgeTarget))
		prop.Set(dummyNode, iprops.LongEdgeHasLabelDummies, true)
	default:
		// The source is the input edge's source port, the target is the
		// output edge's target port
		prop.Set(dummyNode, iprops.LongEdgeSource, inEdge.Source)
		prop.Set(dummyNode, iprops.LongEdgeTarget, outEdge.Target)
	}
}

// moveHeadLabels moves the head labels of the old edge to the new edge.
func moveHeadLabels(oldEdge, newEdge *lgraph.LEdge) {
	kept := oldEdge.Labels[:0]
	for _, label := range oldEdge.Labels {
		if prop.Get(label, lopts.EdgeLabelsPlacement) == core.EdgeLabelPlacementHead {
			newEdge.Labels = append(newEdge.Labels, label)
			// Remember the original edge of the label
			if !prop.Has(label, iprops.EndLabelEdge) {
				prop.Set(label, iprops.EndLabelEdge, oldEdge)
			}
			continue
		}
		kept = append(kept, label)
	}
	// Clear the tail so that the removed labels are not kept alive.
	for i := len(kept); i < len(oldEdge.Labels); i++ {
		oldEdge.Labels[i] = nil
	}
	oldEdge.Labels = kept
}
