// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/HierarchicalPortPositionProcessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package hports

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("HierarchicalPortPositionProcessor", func() lgraph.Processor { return NewHierarchicalPortPositionProcessor() })
}

// HierarchicalPortPositionProcessor is the processor
// HierarchicalPortPositionProcessor: it sets the y coordinate of external
// node dummies representing western or eastern hierarchical ports. Note that
// due to additional space required to route edges connected to northern
// external ports, the y coordinate set here may become invalid and may need
// to be fixed later. That fixing is part of what the
// HierarchicalPortOrthogonalEdgeRouter does.
//
// This processor is only necessary for node placers that do not respect the
// property portRatioOrPosition themselves.
type HierarchicalPortPositionProcessor struct{}

// NewHierarchicalPortPositionProcessor is new
// HierarchicalPortPositionProcessor().
func NewHierarchicalPortPositionProcessor() *HierarchicalPortPositionProcessor {
	return &HierarchicalPortPositionProcessor{}
}

// Process is process(layeredGraph, monitor).
func (*HierarchicalPortPositionProcessor) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	layers := layeredGraph.Layers

	// We're interested in EAST and WEST external port dummies only; since
	// they can only be in the first or last layer, only fix coordinates of
	// nodes in those two layers
	if len(layers) > 0 {
		fixPortPositionCoordinates(layers[0], layeredGraph)
	}

	if len(layers) > 1 {
		fixPortPositionCoordinates(layers[len(layers)-1], layeredGraph)
	}
}

// portAnchor is node.getProperty(LayeredOptions.PORT_ANCHOR) where the Java
// code reads a coordinate of the result: it fails with a
// NullPointerException for a node without the property.
func portAnchor(node *lgraph.LNode) *emath.KVector {
	anchor := prop.Get(node, lopts.PortAnchor)
	if anchor == nil {
		lgraph.Bail("hports: NullPointerException, an external port dummy has no port anchor")
	}
	return anchor
}

// fixPortPositionCoordinates is fixCoordinates(layer, layeredGraph) of the
// class HierarchicalPortPositionProcessor: it fixes the y coordinates of
// external port dummies in the given layer.
func fixPortPositionCoordinates(layer *lgraph.Layer, layeredGraph *lgraph.LGraph) {
	portConstraints := prop.Get(layeredGraph, lopts.PortConstraints)
	if !(portConstraints.IsRatioFixed() || portConstraints.IsPosFixed()) {
		// If coordinates are free to be set, we're done
		return
	}

	graphHeight := layeredGraph.ActualSize().Y

	// Iterate over the layer's nodes
	for _, node := range layer.Nodes {
		// We only care about external port dummies...
		if node.Type != lgraph.NodeTypeExternalPort {
			continue
		}

		// ...representing eastern or western ports.
		extPortSide := prop.Get(node, iprops.ExtPortSide)
		if extPortSide != core.PortSideEast && extPortSide != core.PortSideWest {
			continue
		}

		finalYCoordinate := prop.Get(node, iprops.PortRatioOrPosition)

		if portConstraints == core.PortConstraintsFixedRatio {
			// finalYCoordinate is a ratio that must be multiplied with the
			// graph's height
			finalYCoordinate = float64(finalYCoordinate * graphHeight)
		}

		// Apply the node's new Y coordinate
		node.Pos.Y = finalYCoordinate - portAnchor(node).Y
		node.BorderToContentAreaCoordinates(false, true)
	}
}
