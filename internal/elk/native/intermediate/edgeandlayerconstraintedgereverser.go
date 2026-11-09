// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/EdgeAndLayerConstraintEdgeReverser.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package intermediate

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("EdgeAndLayerConstraintEdgeReverser", func() lgraph.Processor {
		return NewEdgeAndLayerConstraintEdgeReverser()
	})
}

// EdgeAndLayerConstraintEdgeReverser is the processor
// EdgeAndLayerConstraintEdgeReverser: it makes sure that a node with an edge
// constraint or a layer constraint has only incoming or only outgoing edges.
// It also reverses the edges of a node with fixed port sides whose ports are
// all reversed.
type EdgeAndLayerConstraintEdgeReverser struct{}

// NewEdgeAndLayerConstraintEdgeReverser is new
// EdgeAndLayerConstraintEdgeReverser().
func NewEdgeAndLayerConstraintEdgeReverser() *EdgeAndLayerConstraintEdgeReverser {
	return &EdgeAndLayerConstraintEdgeReverser{}
}

// Process is process(layeredGraph, monitor).
func (r *EdgeAndLayerConstraintEdgeReverser) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	// Apply edge reversal to all FIRST and FIRST_SEPARATE as well as LAST and
	// LAST_SEPARATE nodes
	remainingNodes := r.handleOuterNodes(layeredGraph)

	// Apply edge reversal to remaining nodes
	r.handleInnerNodes(layeredGraph, remainingNodes)
}

// edgeConstraintOf returns the edge constraint that a layer constraint
// implies. ok is false where the Java variable stays null.
func edgeConstraintOf(layerConstraint lenum.LayerConstraint) (edgeConstraint lenum.EdgeConstraint, ok bool) {
	switch layerConstraint {
	case lenum.LayerConstraintFirst, lenum.LayerConstraintFirstSeparate:
		return lenum.EdgeConstraintOutgoingOnly, true
	case lenum.LayerConstraintLast, lenum.LayerConstraintLastSeparate:
		return lenum.EdgeConstraintIncomingOnly, true
	}
	return lenum.EdgeConstraintNone, false
}

// Outer Nodes

// handleOuterNodes ensures that the edges of the outer nodes are correct. It
// returns the nodes that are not outer nodes.
func (r *EdgeAndLayerConstraintEdgeReverser) handleOuterNodes(layeredGraph *lgraph.LGraph) []*lgraph.LNode {
	remainingNodes := make([]*lgraph.LNode, 0, len(layeredGraph.LayerlessNodes))

	for _, node := range layeredGraph.LayerlessNodes {
		// Check if there is a layer constraint
		layerConstraint := prop.Get(node, lopts.LayeringLayerConstraint)
		edgeConstraint, hasEdgeConstraint := edgeConstraintOf(layerConstraint)

		if hasEdgeConstraint {
			// Set the edge constraint on the node. The Java code sets
			// OUTGOING_ONLY for each outer node.
			prop.Set(node, iprops.EdgeConstraint, lenum.EdgeConstraintOutgoingOnly)

			if edgeConstraint == lenum.EdgeConstraintIncomingOnly {
				r.reverseEdges(layeredGraph, node, layerConstraint, lenum.PortTypeInput)
			} else if edgeConstraint == lenum.EdgeConstraintOutgoingOnly {
				r.reverseEdges(layeredGraph, node, layerConstraint, lenum.PortTypeOutput)
			}
		} else {
			// Remember this node for the next loop
			remainingNodes = append(remainingNodes, node)
		}
	}

	return remainingNodes
}

// Inner Nodes

func (r *EdgeAndLayerConstraintEdgeReverser) handleInnerNodes(layeredGraph *lgraph.LGraph, remainingNodes []*lgraph.LNode) {
	// Iterate over the remaining nodes
	for _, node := range remainingNodes {
		// Check if there is a layer constraint
		layerConstraint := prop.Get(node, lopts.LayeringLayerConstraint)
		edgeConstraint, hasEdgeConstraint := edgeConstraintOf(layerConstraint)

		if hasEdgeConstraint {
			// Set the edge constraint on the node
			prop.Set(node, iprops.EdgeConstraint, lenum.EdgeConstraintOutgoingOnly)

			if edgeConstraint == lenum.EdgeConstraintIncomingOnly {
				r.reverseEdges(layeredGraph, node, layerConstraint, lenum.PortTypeInput)
			} else if edgeConstraint == lenum.EdgeConstraintOutgoingOnly {
				r.reverseEdges(layeredGraph, node, layerConstraint, lenum.PortTypeOutput)
			}
		} else {
			// If the port sides are fixed, but all ports are reversed, that
			// probably means that we have a feedback node. Normally, the
			// connected edges would be routed around the node, but that hides
			// the feedback node character. We thus simply reverse all
			// connected edges and thus make ELK Layered think we have a
			// regular node
			//
			// Note that this behavior is only desired if none of the
			// connected nodes have layer constraints set. Otherwise this
			// processing causes issues with an external port dummy with
			// FIRST_SEPARATE and an inverted ports on the target node's EAST
			// side.
			if prop.Get(node, lopts.PortConstraints).IsSideFixed() && len(node.Ports) != 0 {
				allPortsReversed := true

				for _, port := range node.Ports {
					// A non-reversed eastern port has more outgoing than
					// incoming edges (net flow <= 0). A non-reversed western
					// port has more incoming than outgoing edges (net flow >=
					// 0). If this port doesn't violate one of these
					// conditions, not all of the node's ports are reversed.
					if !(port.Side == core.PortSideEast && port.NetFlow() > 0 ||
						port.Side == core.PortSideWest && port.NetFlow() < 0) {

						allPortsReversed = false
						break
					}

					// no LAST or LAST_SEPARATE allowed for the target of
					// outgoing edges
					for _, e := range port.OutgoingEdges {
						lc := prop.Get(e.Target.Node(), lopts.LayeringLayerConstraint)
						if lc == lenum.LayerConstraintLast || lc == lenum.LayerConstraintLastSeparate {
							allPortsReversed = false
							break
						}
					}

					// no FIRST or FIRST_SEPARATE allowed for the source of
					// incoming edgs
					for _, e := range port.IncomingEdges {
						lc := prop.Get(e.Source.Node(), lopts.LayeringLayerConstraint)
						if lc == lenum.LayerConstraintFirst || lc == lenum.LayerConstraintFirstSeparate {
							allPortsReversed = false
							break
						}
					}
				}

				if allPortsReversed {
					// Reverse all kinds of edges
					r.reverseEdges(layeredGraph, node, layerConstraint, lenum.PortTypeUndefined)
				}
			}
		}
	}
}

// Edge Reversal

// reverseEdges reverses edges of the node. targetPortType tells what the
// ports become: for OUTPUT each incoming edge is reversed, so that the ports
// become pure output ports.
func (r *EdgeAndLayerConstraintEdgeReverser) reverseEdges(layeredGraph *lgraph.LGraph, node *lgraph.LNode, nodeLayerConstraint lenum.LayerConstraint, targetPortType lenum.PortType) {
	// Iterate through the node's edges and reverse them, if necessary. The
	// Java code walks arrays, because a reversal changes the lists.
	for _, port := range append([]*lgraph.LPort(nil), node.Ports...) {
		// Only incoming edges
		if targetPortType == lenum.PortTypeInput || targetPortType == lenum.PortTypeUndefined {
			outgoing := append([]*lgraph.LEdge(nil), port.OutgoingEdges...)

			for _, edge := range outgoing {
				// Reverse the edge if we're allowed to do so
				if r.canReverseOutgoingEdge(nodeLayerConstraint, edge) {
					edge.Reverse(layeredGraph, true)
				}
			}
		}

		// Only outgoing edges
		if targetPortType == lenum.PortTypeOutput || targetPortType == lenum.PortTypeUndefined {
			incoming := append([]*lgraph.LEdge(nil), port.IncomingEdges...)

			for _, edge := range incoming {
				// Reverse the edge if we're allowed to do so
				if r.canReverseIncomingEdge(nodeLayerConstraint, edge) {
					edge.Reverse(layeredGraph, true)
				}
			}
		}
	}
}

// Reversal Testing

// canReverseOutgoingEdge tells that an outgoing edge can be reversed. It
// cannot be reversed if it already has been, or if it connects a node in the
// LAST layer to a node in the LAST_SEPARATE layer or to a label dummy node.
func (r *EdgeAndLayerConstraintEdgeReverser) canReverseOutgoingEdge(sourceNodeLayerConstraint lenum.LayerConstraint, edge *lgraph.LEdge) bool {
	// If the edge is already reversed, we don't want to reverse it again
	if prop.Get(edge, iprops.Reversed) {
		return false
	}

	targetNode := edge.Target.Node()

	// If the node is supposed to be in the lAST layer...
	if sourceNodeLayerConstraint == lenum.LayerConstraintLast {
		// ...and is connected to a label dummy, we won't reverse it since
		// that dummy has to be placed between the LAST and the LAST_SEPARATE
		// layer
		if targetNode.Type == lgraph.NodeTypeLabel {
			return false
		}
	}

	// If the target is a node in the LAST_SEPARATE layer, we won't reverse it
	targetLayerConstraint := prop.Get(targetNode, lopts.LayeringLayerConstraint)
	if targetLayerConstraint == lenum.LayerConstraintLastSeparate {
		return false
	}

	return true
}

// canReverseIncomingEdge tells that an incoming edge can be reversed. It
// cannot be reversed if it already has been, or if it connects a node in the
// FIRST layer to a node in the FIRST_SEPARATE layer or to a label dummy node.
func (r *EdgeAndLayerConstraintEdgeReverser) canReverseIncomingEdge(targetNodeLayerConstraint lenum.LayerConstraint, edge *lgraph.LEdge) bool {
	// If the edge is already reversed, we don't want to reverse it again
	if prop.Get(edge, iprops.Reversed) {
		return false
	}

	sourceNode := edge.Source.Node()

	// If the node is supposed to be in the FIRST layer...
	if targetNodeLayerConstraint == lenum.LayerConstraintFirst {
		// ...and is connected to a label dummy, we won't reverse it since
		// that dummy has to be placed between the FIRST_SEPARATE and the
		// FIRST layer
		if sourceNode.Type == lgraph.NodeTypeLabel {
			return false
		}
	}

	// If the source is a node in the FIRST_SEPARATE layer, we won't reverse
	// it
	sourceLayerConstraint := prop.Get(sourceNode, lopts.LayeringLayerConstraint)
	if sourceLayerConstraint == lenum.LayerConstraintFirstSeparate {
		return false
	}

	return true
}
