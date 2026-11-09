// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/LabelDummyRemover.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package labels

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/intermediate"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("LabelDummyRemover", func() lgraph.Processor { return NewLabelDummyRemover() })
}

// LabelDummyRemover is the processor LabelDummyRemover: it removes the label
// dummy nodes, places their labels at the positions of the dummy nodes and
// gives the labels back to their edges.
type LabelDummyRemover struct{}

// NewLabelDummyRemover is new LabelDummyRemover().
func NewLabelDummyRemover() *LabelDummyRemover { return &LabelDummyRemover{} }

// Process is process(layeredGraph, monitor).
func (p *LabelDummyRemover) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	edgeLabelSpacing := prop.Get(layeredGraph, lopts.SpacingEdgeLabel)
	labelLabelSpacing := prop.Get(layeredGraph, lopts.SpacingLabelLabel)
	layoutDirection := prop.Get(layeredGraph, lopts.Direction)

	for _, layer := range layeredGraph.Layers {
		// The walk is over indices, since dummy nodes might be removed
		for nodeIndex := 0; nodeIndex < len(layer.Nodes); nodeIndex++ {
			node := layer.Nodes[nodeIndex]

			if node.Type != lgraph.NodeTypeLabel {
				continue
			}
			// First, place labels on position of dummy node
			originEdge := originEdgeOf(node)
			thickness := prop.Get(originEdge, lopts.EdgeThickness)
			labelsBelowEdge := prop.Get(node, iprops.LabelSide) == core.LabelSideBelow

			currLabelPos := node.Pos.Clone()

			// If the labels are to be placed below their edge, we need to
			// move the first label's position down a bit to respect the
			// label spacing
			if labelsBelowEdge {
				currLabelPos.Y += thickness + edgeLabelSpacing
			}

			// Calculate the space available for the placement of labels
			if !prop.Has(node, iprops.RepresentedLabels) {
				lgraph.Bail("LabelDummyRemover: a label dummy without represented labels")
			}
			labelSpaceOffset := -thickness - edgeLabelSpacing
			if node.IsInlineEdgeLabel() {
				labelSpaceOffset = 0
			}
			labelSpace := emath.NewKVector(
				node.Size.X,
				node.Size.Y+labelSpaceOffset)

			// Place labels
			representedLabels := prop.Get(node, iprops.RepresentedLabels)

			if layoutDirection.IsVertical() {
				p.placeLabelsForVerticalLayout(
					representedLabels,
					currLabelPos,
					labelLabelSpacing,
					labelSpace,
					labelsBelowEdge,
					layoutDirection)
			} else {
				p.placeLabelsForHorizontalLayout(
					representedLabels,
					currLabelPos,
					labelLabelSpacing,
					labelSpace)
			}

			// Add represented labels back to the original edge
			originEdge.Labels = append(originEdge.Labels, representedLabels...)

			// Whether we need to add unnecessary bend points around the label
			// dummy depends on the edge router. For orthogonal edge routing,
			// they are not necessary. For splines, they may even be harmful.
			// For polylines, they are necessary to keep the routes edges take
			// close to their labels
			intermediate.JoinAt(node, prop.Get(layeredGraph, lopts.EdgeRouting) == core.EdgeRoutingPolyline)

			// Remove the node
			layer.Nodes = append(layer.Nodes[:nodeIndex], layer.Nodes[nodeIndex+1:]...)
			nodeIndex--
		}
	}
}

func (*LabelDummyRemover) placeLabelsForHorizontalLayout(labels []*lgraph.LLabel, labelPos *emath.KVector, labelSpacing float64, labelSpace *emath.KVector) {
	for _, label := range labels {
		label.Pos.X = labelPos.X + float64((labelSpace.X-label.Size.X)/2.0)
		label.Pos.Y = labelPos.Y

		labelPos.Y += label.Size.Y + labelSpacing
	}
}

func (*LabelDummyRemover) placeLabelsForVerticalLayout(labels []*lgraph.LLabel, labelPos *emath.KVector, labelSpacing float64, labelSpace *emath.KVector, leftAligned bool, layoutDirection core.Direction) {
	// We may have to override the alignment if all labels here are inline
	// labels
	inline := true
	for _, label := range labels {
		if !prop.Get(label, lopts.EdgeLabelsInline) {
			inline = false
			break
		}
	}

	// Due to the way layout directions work, we need to pay attention to the
	// order in which we place labels. While we can simply place them as they
	// come for the DOWN direction, doing the same for the UP direction will
	// reverse the label order in the final result. Thus, in that case we
	// iterate over the reversed label list
	effectiveLabels := labels
	if layoutDirection == core.DirectionUp {
		effectiveLabels = make([]*lgraph.LLabel, len(labels))
		for i, label := range labels {
			effectiveLabels[len(labels)-1-i] = label
		}
	}

	for _, label := range effectiveLabels {
		label.Pos.X = labelPos.X

		if inline {
			label.Pos.Y = labelPos.Y + float64((labelSpace.Y-label.Size.Y)/2)
		} else if leftAligned {
			label.Pos.Y = labelPos.Y
		} else {
			label.Pos.Y = labelPos.Y + labelSpace.Y - label.Size.Y
		}

		labelPos.X += label.Size.X + labelSpacing
	}
}
