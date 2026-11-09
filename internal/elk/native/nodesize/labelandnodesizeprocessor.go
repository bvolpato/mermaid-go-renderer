// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/LabelAndNodeSizeProcessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package nodesize has the intermediate processors of the layered algorithm
// that compute node sizes and node margins with the node spacing code:
// LabelAndNodeSizeProcessor and InnermostNodeMarginCalculator.
package nodesize

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("LabelAndNodeSizeProcessor", func() lgraph.Processor { return NewLabelAndNodeSizeProcessor() })
}

// LabelAndNodeSizeProcessor is the processor LabelAndNodeSizeProcessor: it
// computes the node sizes, places the ports, and places the node labels and
// the port labels.
//
// Before it, the graph is layered, the crossing minimization is done, the
// port constraints are at least FIXED_ORDER, and the port lists are sorted
// clockwise from the leftmost northern port. After it, the port positions
// are fixed.
type LabelAndNodeSizeProcessor struct{}

// NewLabelAndNodeSizeProcessor is new LabelAndNodeSizeProcessor().
func NewLabelAndNodeSizeProcessor() *LabelAndNodeSizeProcessor { return &LabelAndNodeSizeProcessor{} }

// Process is process(layeredGraph, monitor).
func (p *LabelAndNodeSizeProcessor) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	nodespacing.CalculateLabelAndNodeSizes(adapters.AdaptLGraphFiltered(
		layeredGraph,
		true,
		true,
		func(node *lgraph.LNode) bool { return node.Type == lgraph.NodeTypeNormal }))

	// If the graph has external ports, we need to treat labels of external
	// port dummies a bit differently, which is the reason why we haven't
	// handed them to the label and node size processing code
	if prop.Get(layeredGraph, iprops.GraphProperties).Contains(lenum.GraphPropertiesExternalPorts) {
		portLabelPlacement := prop.Get(layeredGraph, lopts.PortLabelsPlacement)
		placeNextToPort := portLabelPlacement.Contains(core.PortLabelPlacementNextToPortIfPossible)
		treatAsGroup := prop.Get(layeredGraph, lopts.PortLabelsTreatAsGroup)

		for _, layer := range layeredGraph.Layers {
			for _, dummy := range layer.Nodes {
				if dummy.Type == lgraph.NodeTypeExternalPort {
					p.placeExternalPortDummyLabels(dummy, portLabelPlacement, placeNextToPort, treatAsGroup)
				}
			}
		}
	}
}

// placeExternalPortDummyLabels places the labels of an external port dummy
// so that the node margins keep the space that the labels need when the
// label and node placement runs on the parent graph.
func (p *LabelAndNodeSizeProcessor) placeExternalPortDummyLabels(dummy *lgraph.LNode,
	graphPortLabelPlacement *prop.EnumSet[core.PortLabelPlacement], placeNextToPortIfPossible, treatAsGroup bool) {

	labelPortSpacingHorizontal := prop.Get(dummy, lopts.SpacingLabelPortHorizontal)
	labelPortSpacingVertical := prop.Get(dummy, lopts.SpacingLabelPortVertical)
	labelLabelSpacing := prop.Get(dummy, lopts.SpacingLabelLabel)

	dummySize := &dummy.Size

	// External port dummies have exactly one port (see ElkGraphImporter)
	if len(dummy.Ports) == 0 {
		lgraph.Bail("LabelAndNodeSizeProcessor: an external port dummy without a port")
	}
	dummyPort := dummy.Ports[0]
	dummyPortPos := &dummyPort.Pos

	portLabelBox := p.computePortLabelBox(dummyPort, labelLabelSpacing)
	if portLabelBox == nil {
		return
	}

	// Determine the position of the box
	// TODO We could handle FIXED here as well
	if graphPortLabelPlacement.Contains(core.PortLabelPlacementInside) {
		// (port label placement has to support this case first, though)
		switch prop.Get(dummy, iprops.ExtPortSide) {
		case core.PortSideNorth:
			portLabelBox.X = float64((dummySize.X-portLabelBox.Width)/2) - dummyPortPos.X
			portLabelBox.Y = labelPortSpacingVertical
		case core.PortSideSouth:
			portLabelBox.X = float64((dummySize.X-portLabelBox.Width)/2) - dummyPortPos.X
			portLabelBox.Y = -labelPortSpacingVertical - portLabelBox.Height
		case core.PortSideEast:
			if p.labelNextToPort(dummyPort, true, placeNextToPortIfPossible) {
				labelHeight := portLabelBox.Height
				if !treatAsGroup {
					labelHeight = dummyPort.Labels[0].Size.Y
				}
				portLabelBox.Y = float64((dummySize.Y-labelHeight)/2) - dummyPortPos.Y
			} else {
				portLabelBox.Y = dummySize.Y + labelPortSpacingVertical - dummyPortPos.Y
			}
			portLabelBox.X = -labelPortSpacingHorizontal - portLabelBox.Width
		case core.PortSideWest:
			if p.labelNextToPort(dummyPort, true, placeNextToPortIfPossible) {
				labelHeight := portLabelBox.Height
				if !treatAsGroup {
					labelHeight = dummyPort.Labels[0].Size.Y
				}
				portLabelBox.Y = float64((dummySize.Y-labelHeight)/2) - dummyPortPos.Y
			} else {
				portLabelBox.Y = dummySize.Y + labelPortSpacingVertical - dummyPortPos.Y
			}
			portLabelBox.X = labelPortSpacingHorizontal
		}
	} else if graphPortLabelPlacement.Contains(core.PortLabelPlacementOutside) {
		switch prop.Get(dummy, iprops.ExtPortSide) {
		case core.PortSideNorth, core.PortSideSouth:
			portLabelBox.X = dummyPortPos.X + labelPortSpacingHorizontal
		case core.PortSideEast, core.PortSideWest:
			if p.labelNextToPort(dummyPort, false, placeNextToPortIfPossible) {
				labelHeight := portLabelBox.Height
				if !treatAsGroup {
					labelHeight = dummyPort.Labels[0].Size.Y
				}
				portLabelBox.Y = float64((dummySize.Y-labelHeight)/2) - dummyPortPos.Y
			} else {
				portLabelBox.Y = dummyPortPos.Y + labelPortSpacingVertical
			}
		}
	}

	// Place the labels
	currentY := portLabelBox.Y
	for _, label := range dummyPort.Labels {
		labelPos := &label.Pos
		labelPos.X = portLabelBox.X
		labelPos.Y = currentY

		currentY += label.Size.Y + labelLabelSpacing
	}
}

// computePortLabelBox returns the space that the labels of the port need,
// or nil for a port without labels.
func (p *LabelAndNodeSizeProcessor) computePortLabelBox(dummyPort *lgraph.LPort, labelLabelSpacing float64) *emath.ElkRectangle {
	if len(dummyPort.Labels) == 0 {
		return nil
	}
	result := &emath.ElkRectangle{}

	for _, label := range dummyPort.Labels {
		labelSize := &label.Size

		result.Width = max(result.Width, labelSize.X)
		result.Height += labelSize.Y
	}

	result.Height += float64(float64(len(dummyPort.Labels)-1) * labelLabelSpacing)

	return result
}

// labelNextToPort tells whether the labels of the port are placed next to
// the port and not below it: the user asked for it, and the port has no
// connections.
func (p *LabelAndNodeSizeProcessor) labelNextToPort(dummyPort *lgraph.LPort, insideLabels, placeNextToPortIfPossible bool) bool {
	if !placeNextToPortIfPossible {
		return false
	}
	if insideLabels {
		return len(dummyPort.IncomingEdges) == 0 && len(dummyPort.OutgoingEdges) == 0
	}
	return !dummyPort.ConnectedToExternalNodes
}
