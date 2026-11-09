// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/LayerConstraintPostprocessor.java
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
	lgraph.Register("LayerConstraintPostprocessor", func() lgraph.Processor { return NewLayerConstraintPostprocessor() })
}

// LayerConstraintPostprocessor is the processor
// LayerConstraintPostprocessor: it moves the nodes with the layer constraint
// FIRST or LAST to the first or the last layer, and it restores the nodes
// that LayerConstraintPreprocessor hid in separate layers.
type LayerConstraintPostprocessor struct{}

// NewLayerConstraintPostprocessor is new LayerConstraintPostprocessor().
func NewLayerConstraintPostprocessor() *LayerConstraintPostprocessor {
	return &LayerConstraintPostprocessor{}
}

// insertLayerFirst is layers.add(0, layer).
func insertLayerFirst(layeredGraph *lgraph.LGraph, layer *lgraph.Layer) {
	layeredGraph.Layers = append(layeredGraph.Layers, nil)
	copy(layeredGraph.Layers[1:], layeredGraph.Layers)
	layeredGraph.Layers[0] = layer
}

// Process is process(layeredGraph, monitor).
func (p *LayerConstraintPostprocessor) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	// If the list of layers is non-empty, we may have FIRST and LAST nodes to
	// move around
	if len(layeredGraph.Layers) != 0 {
		// Retrieve the current first and last layers
		firstLayer := layeredGraph.Layers[0]
		lastLayer := layeredGraph.Layers[len(layeredGraph.Layers)-1]

		// We may also need label dummy layers between the very first / last
		// layers and the first / last layers
		firstLabelLayer := lgraph.NewLayer(layeredGraph)
		lastLabelLayer := lgraph.NewLayer(layeredGraph)

		// Move our nodes!
		p.moveFirstAndLastNodes(layeredGraph, firstLayer, lastLayer, firstLabelLayer, lastLabelLayer)

		// Add non-empty label layers
		if len(firstLabelLayer.Nodes) != 0 {
			insertLayerFirst(layeredGraph, firstLabelLayer)
		}

		if len(lastLabelLayer.Nodes) != 0 {
			layeredGraph.Layers = append(layeredGraph.Layers, lastLabelLayer)
		}
	}

	// Restore hidden FIRST_SEPARATE and LAST_SEPARATE nodes, if any
	if prop.Has(layeredGraph, iprops.HiddenNodes) {
		// Create the new first and last layers, in case they will be needed
		firstSeparateLayer := lgraph.NewLayer(layeredGraph)
		lastSeparateLayer := lgraph.NewLayer(layeredGraph)

		// Restore our nodes
		p.restoreHiddenNodes(layeredGraph, firstSeparateLayer, lastSeparateLayer)

		// Add non-empty separate layers
		if len(firstSeparateLayer.Nodes) != 0 {
			insertLayerFirst(layeredGraph, firstSeparateLayer)
		}

		if len(lastSeparateLayer.Nodes) != 0 {
			layeredGraph.Layers = append(layeredGraph.Layers, lastSeparateLayer)
		}
	}
}

// moveFirstAndLastNodes moves the nodes with the layer constraint FIRST or
// LAST to their layers and removes the layers that have become empty.
func (p *LayerConstraintPostprocessor) moveFirstAndLastNodes(layeredGraph *lgraph.LGraph, firstLayer, lastLayer, firstLabelLayer, lastLabelLayer *lgraph.Layer) {
	// We'll start by moving FIRST and LAST nodes (as well as connected label
	// dummies) to their proper layers
	for _, layer := range layeredGraph.Layers {
		// Iterate through a node array to avoid
		// ConcurrentModificationExceptions
		nodes := append([]*lgraph.LNode(nil), layer.Nodes...)

		for _, node := range nodes {
			switch prop.Get(node, lopts.LayeringLayerConstraint) {
			case lenum.LayerConstraintFirst:
				p.throwUpUnlessNoIncomingEdges(node)
				node.SetLayer(firstLayer)
				p.moveLabelsToLabelLayer(node, true, firstLabelLayer)

			case lenum.LayerConstraintLast:
				p.throwUpUnlessNoOutgoingEdges(node)
				node.SetLayer(lastLayer)
				p.moveLabelsToLabelLayer(node, false, lastLabelLayer)
			}
		}
	}

	// All the movement can cause layers to be empty. Instead of going to
	// great lengths trying to prevent that in the code above, we simply
	// iterate over all layers and remove the empty ones
	kept := layeredGraph.Layers[:0]
	for _, layer := range layeredGraph.Layers {
		if len(layer.Nodes) != 0 {
			kept = append(kept, layer)
		}
	}
	for i := len(kept); i < len(layeredGraph.Layers); i++ {
		layeredGraph.Layers[i] = nil
	}
	layeredGraph.Layers = kept
}

// moveLabelsToLabelLayer moves the label dummies on the incoming edges of
// the node (incoming is true) or on its outgoing edges to the label layer.
func (p *LayerConstraintPostprocessor) moveLabelsToLabelLayer(node *lgraph.LNode, incoming bool, labelLayer *lgraph.Layer) {
	var edges []*lgraph.LEdge
	if incoming {
		edges = node.IncomingEdges()
	} else {
		edges = node.OutgoingEdges()
	}
	for _, edge := range edges {
		var possibleLableDummy *lgraph.LNode
		if incoming {
			possibleLableDummy = edge.Source.Node()
		} else {
			possibleLableDummy = edge.Target.Node()
		}

		if possibleLableDummy.Type == lgraph.NodeTypeLabel {
			possibleLableDummy.SetLayer(labelLayer)
		}
	}
}

// restoreHiddenNodes restores the nodes that LayerConstraintPreprocessor hid.
func (p *LayerConstraintPostprocessor) restoreHiddenNodes(layeredGraph *lgraph.LGraph, firstSeparateLayer, lastSeparateLayer *lgraph.Layer) {
	for _, hiddenNode := range prop.Get(layeredGraph, iprops.HiddenNodes) {
		// Add the node to the appropriate layer
		switch prop.Get(hiddenNode, lopts.LayeringLayerConstraint) {
		case lenum.LayerConstraintFirstSeparate:
			hiddenNode.SetLayer(firstSeparateLayer)

		case lenum.LayerConstraintLastSeparate:
			hiddenNode.SetLayer(lastSeparateLayer)
		}

		// Restore the node's edges. getConnectedEdges is a lazy view of the
		// edge lists of the ports that the node has at the call. Restoring an
		// edge can add it to a list of this node, so the walk reads each list
		// again at each step, as the iterators of GWT do.
		for _, port := range append([]*lgraph.LPort(nil), hiddenNode.Ports...) {
			for i := 0; i < len(port.IncomingEdges); i++ {
				p.restoreHiddenEdge(port.IncomingEdges[i])
			}
			for i := 0; i < len(port.OutgoingEdges); i++ {
				p.restoreHiddenEdge(port.OutgoingEdges[i])
			}
		}
	}
}

// restoreHiddenEdge is the body of the edge loop of restoreHiddenNodes.
func (p *LayerConstraintPostprocessor) restoreHiddenEdge(hiddenEdge *lgraph.LEdge) {
	// A hidden node will usually be incident only to edges that were hidden
	// together with the node. There is one exception, though: if an edge
	// connects two hidden nodes, only one will have been responsible for
	// hiding it. Thus, we may find an edge here that is already restored!
	if hiddenEdge.Source != nil && hiddenEdge.Target != nil {
		return
	}

	// One end point is the hidden node, the other is null
	isOutgoing := hiddenEdge.Target == nil

	originalOppositePort := prop.Get(hiddenEdge, iprops.OriginalOppositePort)
	if isOutgoing {
		hiddenEdge.SetTarget(originalOppositePort)
	} else {
		hiddenEdge.SetSource(originalOppositePort)
	}
}

// Checks

// throwUpUnlessNoIncomingEdges stops the layout if the node has an incoming
// edge that does not come from a label dummy. Java throws an
// UnsupportedConfigurationException.
func (p *LayerConstraintPostprocessor) throwUpUnlessNoIncomingEdges(node *lgraph.LNode) {
	for _, incoming := range node.IncomingEdges() {
		if incoming.Source.Node().Type != lgraph.NodeTypeLabel {
			lgraph.Bail("Node '%s' has its layer constraint set to FIRST, but has at least one incoming edge that "+
				" does not come from a FIRST_SEPARATE node. That must not happen.", node.Designation())
		}
	}
}

// throwUpUnlessNoOutgoingEdges stops the layout if the node has an outgoing
// edge that does not go to a label dummy. Java throws an
// UnsupportedConfigurationException.
func (p *LayerConstraintPostprocessor) throwUpUnlessNoOutgoingEdges(node *lgraph.LNode) {
	for _, outgoing := range node.OutgoingEdges() {
		if outgoing.Target.Node().Type != lgraph.NodeTypeLabel {
			lgraph.Bail("Node '%s' has its layer constraint set to LAST, but has at least one outgoing edge that "+
				" does not go to a LAST_SEPARATE node. That must not happen.", node.Designation())
		}
	}
}
