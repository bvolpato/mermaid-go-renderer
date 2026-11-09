// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p4nodes/SimpleNodePlacer.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package p4nodes has the node placers of the layered algorithm that a user
// selects with the option nodePlacement.strategy: SimpleNodePlacer,
// LinearSegmentsNodePlacer and NetworkSimplexPlacer. The default placer,
// BKNodePlacer, is in the package bk.
//
// Math.min and Math.max are the builtin functions min and max in this
// package: they give NaN when an argument is NaN, as JavaScript does.
package p4nodes

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("SimpleNodePlacer", func() lgraph.Processor { return NewSimpleNodePlacer() })
}

// SimpleNodePlacer is the phase SimpleNodePlacer: it centers the nodes of
// each layer vertically.
//
// The graph must be layered with ordered nodes and arranged ports. The placer
// sets the y coordinate of each node and the height of each layer.
//
// getLayoutProcessorConfiguration is in the package layered, with the
// configurations of the other phases.
type SimpleNodePlacer struct{}

// NewSimpleNodePlacer is new SimpleNodePlacer().
func NewSimpleNodePlacer() *SimpleNodePlacer { return &SimpleNodePlacer{} }

// verticalSpacing is spacings.getVerticalSpacing(n1, n2). The Java code fails
// with a NullPointerException for a graph without the property spacings.
func verticalSpacing(spacings *lgraph.Spacings, n1, n2 *lgraph.LNode) float64 {
	if spacings == nil {
		lgraph.Bail("node placement: the graph has no spacings")
	}
	return spacings.VerticalSpacing(n1, n2)
}

// Process is process(layeredGraph, monitor).
func (*SimpleNodePlacer) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	spacings := prop.Get(layeredGraph, iprops.Spacings)

	// first iteration: determine the height of each layer
	maxHeight := 0.0
	for _, layer := range layeredGraph.Layers {
		layerSize := &layer.Size
		layerSize.Y = 0
		var lastNode *lgraph.LNode
		for _, node := range layer.Nodes {
			if lastNode != nil {
				// use normal spacing as soon as a regular node is involved
				layerSize.Y += verticalSpacing(spacings, node, lastNode)
			}
			layerSize.Y += node.Margin.Top + node.Size.Y + node.Margin.Bottom
			lastNode = node
		}
		maxHeight = max(maxHeight, layerSize.Y)
	}

	// second iteration: center the nodes of each layer around the tallest layer
	for _, layer := range layeredGraph.Layers {
		layerSize := &layer.Size
		pos := float64((maxHeight - layerSize.Y) / 2)
		var lastNode *lgraph.LNode
		for _, node := range layer.Nodes {
			if lastNode != nil {
				pos += verticalSpacing(spacings, node, lastNode)
			}
			pos += node.Margin.Top
			node.Pos.Y = pos
			pos += node.Size.Y + node.Margin.Bottom
			lastNode = node
		}
	}
}
