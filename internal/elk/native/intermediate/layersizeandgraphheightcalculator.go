// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/LayerSizeAndGraphHeightCalculator.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package intermediate

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("LayerSizeAndGraphHeightCalculator", func() lgraph.Processor {
		return NewLayerSizeAndGraphHeightCalculator()
	})
}

// LayerSizeAndGraphHeightCalculator is the processor
// LayerSizeAndGraphHeightCalculator: it sets the size of each layer, and the
// height and the vertical offset of the graph.
type LayerSizeAndGraphHeightCalculator struct{}

// NewLayerSizeAndGraphHeightCalculator is new
// LayerSizeAndGraphHeightCalculator().
func NewLayerSizeAndGraphHeightCalculator() *LayerSizeAndGraphHeightCalculator {
	return &LayerSizeAndGraphHeightCalculator{}
}

// Process is process(layeredGraph, monitor).
func (*LayerSizeAndGraphHeightCalculator) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	// Remember the lowest and the highest y coordinates
	minY := math.Inf(1)
	maxY := math.Inf(-1)

	foundNodes := false
	for _, layer := range layeredGraph.Layers {
		// Retrieve and reset layer size (just to be sure...)
		layerSize := &layer.Size
		layerSize.X = 0.0
		layerSize.Y = 0.0

		// If the layer is empty, skip the rest (should not usually happen)
		if len(layer.Nodes) == 0 {
			continue
		}

		foundNodes = true

		// Calculate the layer's width
		for _, node := range layer.Nodes {
			nodeSize := &node.Size
			nodeMargin := &node.Margin

			layerSize.X = max(layerSize.X, nodeSize.X+nodeMargin.Left+nodeMargin.Right)
		}

		// Calculate the layer's height
		firstNode := layer.Nodes[0]
		top := firstNode.Pos.Y - firstNode.Margin.Top
		if firstNode.Type == lgraph.NodeTypeExternalPort {
			// Consider port spacing options for first node
			top -= prop.Get(layeredGraph, lopts.SpacingPortsSurrounding).Top
		}
		lastNode := layer.Nodes[len(layer.Nodes)-1]
		bottom := lastNode.Pos.Y + lastNode.Size.Y + lastNode.Margin.Bottom
		if lastNode.Type == lgraph.NodeTypeExternalPort {
			// Consider port spacing options for last node
			bottom += prop.Get(layeredGraph, lopts.SpacingPortsSurrounding).Bottom
		}
		layerSize.Y = bottom - top

		// Update the lowest and highest encountered Y coordinate
		minY = min(minY, top)
		maxY = max(maxY, bottom)
	}

	// If we didn't find any nodes in the graph, be sure to set the values
	// properly. This can happen if a connected component only contains
	// external ports.
	if !foundNodes {
		minY = 0
		maxY = 0
	}

	// Set the graph's height
	layeredGraph.Size.Y = maxY - minY
	layeredGraph.Offset.Y -= minY
}
