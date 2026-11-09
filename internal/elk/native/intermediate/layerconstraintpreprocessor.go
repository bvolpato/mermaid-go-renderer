// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/LayerConstraintPreprocessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package intermediate

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("LayerConstraintPreprocessor", func() lgraph.Processor { return NewLayerConstraintPreprocessor() })
	prop.RegisterEnum[HiddenNodeConnections]("org.eclipse.elk.alg.layered.intermediate.LayerConstraintPreprocessor$HiddenNodeConnections", namesHiddenNodeConnections)
}

// HiddenNodeConnections is the enum
// LayerConstraintPreprocessor.HiddenNodeConnections: the kinds of hidden
// nodes that a node had connections to.
type HiddenNodeConnections int

// The constants of HiddenNodeConnections, in the order of the Java enum.
const (
	// HiddenNodeConnectionsNone is NONE: no connections to hidden nodes
	// found yet.
	HiddenNodeConnectionsNone HiddenNodeConnections = iota
	// HiddenNodeConnectionsFirstSeparate is FIRST_SEPARATE: only connections
	// to FIRST_SEPARATE nodes found so far.
	HiddenNodeConnectionsFirstSeparate
	// HiddenNodeConnectionsLastSeparate is LAST_SEPARATE: only connections
	// to LAST_SEPARATE nodes found so far.
	HiddenNodeConnectionsLastSeparate
	// HiddenNodeConnectionsBoth is BOTH: connections to both kinds of hidden
	// nodes found.
	HiddenNodeConnectionsBoth
)

var namesHiddenNodeConnections = []string{"NONE", "FIRST_SEPARATE", "LAST_SEPARATE", "BOTH"}

// String returns the name of the Java constant.
func (c HiddenNodeConnections) String() string {
	return prop.EnumName(namesHiddenNodeConnections, int(c))
}

// combine is HiddenNodeConnections.combine(layerConstraint).
func (c HiddenNodeConnections) combine(layerConstraint lenum.LayerConstraint) HiddenNodeConnections {
	switch c {
	case HiddenNodeConnectionsNone:
		if layerConstraint == lenum.LayerConstraintFirstSeparate {
			return HiddenNodeConnectionsFirstSeparate
		}
		return HiddenNodeConnectionsLastSeparate

	case HiddenNodeConnectionsFirstSeparate:
		if layerConstraint == lenum.LayerConstraintFirstSeparate {
			return HiddenNodeConnectionsFirstSeparate
		}
		return HiddenNodeConnectionsBoth

	case HiddenNodeConnectionsLastSeparate:
		if layerConstraint == lenum.LayerConstraintFirstSeparate {
			return HiddenNodeConnectionsBoth
		}
		return HiddenNodeConnectionsLastSeparate

	default:
		return HiddenNodeConnectionsBoth
	}
}

// hiddenNodeConnectionsProperty is HIDDEN_NODE_CONNECTIONS: the connections
// of a node to hidden nodes.
var hiddenNodeConnectionsProperty = prop.NewDefault("separateLayerConnections", HiddenNodeConnectionsNone)

// LayerConstraintPreprocessor is the processor LayerConstraintPreprocessor:
// it hides the nodes with the layer constraint FIRST_SEPARATE or
// LAST_SEPARATE from the layering phase.
type LayerConstraintPreprocessor struct{}

// NewLayerConstraintPreprocessor is new LayerConstraintPreprocessor().
func NewLayerConstraintPreprocessor() *LayerConstraintPreprocessor {
	return &LayerConstraintPreprocessor{}
}

// Process is process(layeredGraph, monitor).
func (p *LayerConstraintPreprocessor) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	var hiddenNodes []*lgraph.LNode

	// The Java code removes a hidden node through the list iterator, so the
	// node after it gets its index.
	for i := 0; i < len(layeredGraph.LayerlessNodes); {
		lNode := layeredGraph.LayerlessNodes[i]
		i++

		if p.isRelevantNode(lNode) {
			p.hide(lNode)
			hiddenNodes = append(hiddenNodes, lNode)
			i--
			layeredGraph.LayerlessNodes = append(layeredGraph.LayerlessNodes[:i], layeredGraph.LayerlessNodes[i+1:]...)
		}
	}

	if len(hiddenNodes) != 0 {
		prop.Set(layeredGraph, iprops.HiddenNodes, hiddenNodes)
	}
}

// isRelevantNode tells that the node has one of the layer constraints that
// this processor handles.
func (p *LayerConstraintPreprocessor) isRelevantNode(lNode *lgraph.LNode) bool {
	switch prop.Get(lNode, lopts.LayeringLayerConstraint) {
	case lenum.LayerConstraintFirstSeparate, lenum.LayerConstraintLastSeparate:
		return true
	default:
		return false
	}
}

// Node Hiding

// hide disconnects the node from its adjacent nodes.
func (p *LayerConstraintPreprocessor) hide(lNode *lgraph.LNode) {
	p.ensureNoInacceptableEdges(lNode)

	// getConnectedEdges is a lazy view of the edge lists of the ports that
	// the node has at the call. Hiding an edge can remove it from a list of
	// this node (a self loop), so the walk reads each list again at each
	// step, as the iterators of GWT do.
	for _, port := range append([]*lgraph.LPort(nil), lNode.Ports...) {
		for i := 0; i < len(port.IncomingEdges); i++ {
			p.hideEdge(lNode, port.IncomingEdges[i])
		}
		for i := 0; i < len(port.OutgoingEdges); i++ {
			p.hideEdge(lNode, port.OutgoingEdges[i])
		}
	}
}

// hideEdge is hide(lNode, lEdge): it hides the edge of the node that is
// being hidden.
func (p *LayerConstraintPreprocessor) hideEdge(lNode *lgraph.LNode, lEdge *lgraph.LEdge) {
	if lEdge.Source == nil {
		// Java fails with a NullPointerException: a self loop of the node,
		// which lost its source at an earlier step.
		lgraph.Bail("LayerConstraintPreprocessor: an edge of a hidden node has no source")
	}
	isOutgoing := lEdge.Source.Node() == lNode
	var oppositePort *lgraph.LPort
	if isOutgoing {
		oppositePort = lEdge.Target
	} else {
		oppositePort = lEdge.Source
	}

	// Hide the edge and remember its original target or source
	if isOutgoing {
		lEdge.SetTarget(nil)
	} else {
		lEdge.SetSource(nil)
	}

	prop.Set(lEdge, iprops.OriginalOppositePort, oppositePort)

	// We may want to control where the other node will end up in the
	// layering...
	if oppositePort == nil {
		// Java fails with a NullPointerException: a self loop of the node,
		// which lost its target at an earlier step.
		lgraph.Bail("LayerConstraintPreprocessor: an edge of a hidden node has no opposite port")
	}
	p.updateOppositeNodeLayerConstraints(lNode, oppositePort.Node())
}

// updateOppositeNodeLayerConstraints possibly sets a layer constraint on the
// opposite node if it has none yet. If the hidden node was the only
// connection of the opposite node, the opposite node gets the constraint
// FIRST or LAST, so that it stays near the hidden node. That happens only if
// the opposite node had connections only to FIRST_SEPARATE nodes or only to
// LAST_SEPARATE nodes.
func (p *LayerConstraintPreprocessor) updateOppositeNodeLayerConstraints(hiddenNode, oppositeNode *lgraph.LNode) {
	// If the node already has a layer constraint, don't bother
	if prop.Has(oppositeNode, lopts.LayeringLayerConstraint) {
		return
	}

	// Update which kinds of hidden nodes the opposite node is connected to
	connections := prop.Get(oppositeNode, hiddenNodeConnectionsProperty).
		combine(prop.Get(hiddenNode, lopts.LayeringLayerConstraint))
	prop.Set(oppositeNode, hiddenNodeConnectionsProperty, connections)

	// If the node is still connected to things, don't bother
	for _, port := range oppositeNode.Ports {
		if len(port.IncomingEdges) != 0 || len(port.OutgoingEdges) != 0 {
			return
		}
	}

	// Our hidden node is the last node the opposite node was connected to.
	// Check if we should add a constraint
	switch connections {
	case HiddenNodeConnectionsFirstSeparate:
		prop.Set(oppositeNode, lopts.LayeringLayerConstraint, lenum.LayerConstraintFirst)
	case HiddenNodeConnectionsLastSeparate:
		prop.Set(oppositeNode, lopts.LayeringLayerConstraint, lenum.LayerConstraintLast)
	}
}

// Checks

// ensureNoInacceptableEdges stops the layout if the node has an edge that it
// must not have. Java throws an UnsupportedConfigurationException.
func (p *LayerConstraintPreprocessor) ensureNoInacceptableEdges(lNode *lgraph.LNode) {
	layerConstraint := prop.Get(lNode, lopts.LayeringLayerConstraint)

	// Ensure the node only has acceptable incident edges
	if layerConstraint == lenum.LayerConstraintFirstSeparate {
		for _, inEdge := range lNode.IncomingEdges() {
			if !p.isAcceptableIncidentEdge(inEdge) {
				lgraph.Bail("Node '%s' has its layer constraint set to FIRST_SEPARATE, but has at least one incoming edge. "+
					"FIRST_SEPARATE nodes must not have incoming edges.", lNode.Designation())
			}
		}
	} else if layerConstraint == lenum.LayerConstraintLastSeparate {
		for _, outEdge := range lNode.OutgoingEdges() {
			if !p.isAcceptableIncidentEdge(outEdge) {
				lgraph.Bail("Node '%s' has its layer constraint set to LAST_SEPARATE, but has at least one outgoing edge. "+
					"LAST_SEPARATE nodes must not have outgoing edges.", lNode.Designation())
			}
		}
	}
}

// isAcceptableIncidentEdge tells that the edge is allowed: both of its nodes
// are external port dummies.
func (p *LayerConstraintPreprocessor) isAcceptableIncidentEdge(edge *lgraph.LEdge) bool {
	sourceNode := edge.Source.Node()
	targetNode := edge.Target.Node()

	// If both nodes are external port dummies, that's fine
	return sourceNode.Type == lgraph.NodeTypeExternalPort && targetNode.Type == lgraph.NodeTypeExternalPort
}
