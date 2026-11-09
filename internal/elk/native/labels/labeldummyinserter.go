// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/LabelDummyInserter.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package labels has the intermediate processors of the layered algorithm
// that handle edge labels: the label dummy nodes of center labels, the label
// sides, and the label cells of head and tail labels.
package labels

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/intermediate"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("LabelDummyInserter", func() lgraph.Processor { return NewLabelDummyInserter() })
}

// LabelDummyInserter is the processor LabelDummyInserter: it puts a dummy
// node on each edge that has center labels, to reserve space for them. An
// edge gets at most one dummy node.
type LabelDummyInserter struct{}

// NewLabelDummyInserter is new LabelDummyInserter().
func NewLabelDummyInserter() *LabelDummyInserter { return &LabelDummyInserter{} }

// centerLabel is the predicate CENTER_LABEL.
func centerLabel(label *lgraph.LLabel) bool {
	return prop.Get(label, lopts.EdgeLabelsPlacement) == core.EdgeLabelPlacementCenter
}

// Process is process(layeredGraph, monitor).
func (p *LabelDummyInserter) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	// The dummy nodes are added to the graph after the walk over its nodes.
	var newDummyNodes []*lgraph.LNode

	edgeLabelSpacing := prop.Get(layeredGraph, lopts.SpacingEdgeLabel)
	labelLabelSpacing := prop.Get(layeredGraph, lopts.SpacingLabelLabel)
	layoutDirection := prop.Get(layeredGraph, lopts.Direction)

	for nodeIndex := 0; nodeIndex < len(layeredGraph.LayerlessNodes); nodeIndex++ {
		node := layeredGraph.LayerlessNodes[nodeIndex]
		// getOutgoingEdges copies the port list and reads the edge list of
		// each port when the walk reaches it.
		ports := append([]*lgraph.LPort(nil), node.Ports...)
		for _, port := range ports {
			for edgeIndex := 0; edgeIndex < len(port.OutgoingEdges); edgeIndex++ {
				edge := port.OutgoingEdges[edgeIndex]
				if !p.edgeNeedsToBeProcessed(edge) {
					continue
				}
				thickness := p.retrieveThickness(edge)

				// Create dummy node and remember represented labels
				dummyNode := p.createLabelDummy(layeredGraph, edge, thickness)
				newDummyNodes = append(newDummyNodes, dummyNode)
				representedLabels := make([]*lgraph.LLabel, 0, len(edge.Labels))

				// Determine the size of the dummy node and move labels over
				// to it
				dummySize := &dummyNode.Size

				kept := edge.Labels[:0]
				for _, label := range edge.Labels {
					if prop.Get(label, lopts.EdgeLabelsPlacement) != core.EdgeLabelPlacementCenter {
						kept = append(kept, label)
						continue
					}
					// The way we stack labels depends on the layout direction
					if layoutDirection.IsVertical() {
						dummySize.X += label.Size.X + labelLabelSpacing
						dummySize.Y = max(dummySize.Y, label.Size.Y)
					} else {
						dummySize.X = max(dummySize.X, label.Size.X)
						dummySize.Y += label.Size.Y + labelLabelSpacing
					}

					// Move the label over to the dummy node's
					// REPRESENTED_LABELS property
					representedLabels = append(representedLabels, label)
				}
				for i := len(kept); i < len(edge.Labels); i++ {
					edge.Labels[i] = nil
				}
				edge.Labels = kept
				prop.Set(dummyNode, iprops.RepresentedLabels, representedLabels)

				// The dummy node now contains a superfluous label-label
				// spacing and does not include the edge-label spacing yet
				if layoutDirection.IsVertical() {
					dummySize.X -= labelLabelSpacing
					dummySize.Y += edgeLabelSpacing + thickness
				} else {
					dummySize.Y += edgeLabelSpacing - labelLabelSpacing + thickness
				}
			}
		}
	}

	// Add created dummies to graph
	layeredGraph.LayerlessNodes = append(layeredGraph.LayerlessNodes, newDummyNodes...)
}

// edgeNeedsToBeProcessed is edgeNeedsToBeProcessed(edge): the edge is not a
// self loop and has a center label.
func (*LabelDummyInserter) edgeNeedsToBeProcessed(edge *lgraph.LEdge) bool {
	if edge.Source == nil || edge.Target == nil {
		lgraph.Bail("LabelDummyInserter: an edge without a source or a target")
	}
	if edge.Source.Node() == edge.Target.Node() {
		return false
	}
	for _, label := range edge.Labels {
		if centerLabel(label) {
			return true
		}
	}
	return false
}

// retrieveThickness is retrieveThickness(edge): the thickness of the edge. A
// negative thickness becomes zero on the edge.
func (*LabelDummyInserter) retrieveThickness(edge *lgraph.LEdge) float64 {
	thickness := prop.Get(edge, lopts.EdgeThickness)
	if thickness < 0 {
		thickness = 0
		prop.Set(edge, lopts.EdgeThickness, thickness)
	}
	return thickness
}

// createLabelDummy is createLabelDummy(layeredGraph, edge, thickness,
// representedLabels). The caller sets the list of represented labels again
// when it is complete.
func (*LabelDummyInserter) createLabelDummy(layeredGraph *lgraph.LGraph, edge *lgraph.LEdge, thickness float64) *lgraph.LNode {
	dummyNode := lgraph.NewLNode(layeredGraph)
	dummyNode.Type = lgraph.NodeTypeLabel

	prop.Set[any](dummyNode, iprops.Origin, edge)
	prop.Set(dummyNode, iprops.RepresentedLabels, []*lgraph.LLabel{})
	prop.Set(dummyNode, lopts.PortConstraints, core.PortConstraintsFixedPos)
	prop.Set(dummyNode, iprops.LongEdgeSource, edge.Source)
	prop.Set(dummyNode, iprops.LongEdgeTarget, edge.Target)

	// Actually split the edge
	intermediate.SplitEdge(edge, dummyNode)

	// Place ports at the edge's center
	portPos := math.Floor(thickness / 2)
	for _, dummyPort := range dummyNode.Ports {
		dummyPort.Pos.Y = portPos
	}

	return dummyNode
}
