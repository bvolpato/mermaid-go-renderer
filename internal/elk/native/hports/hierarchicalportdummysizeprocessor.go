// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/HierarchicalPortDummySizeProcessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package hports

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("HierarchicalPortDummySizeProcessor", func() lgraph.Processor { return NewHierarchicalPortDummySizeProcessor() })
}

// HierarchicalPortDummySizeProcessor is the processor
// HierarchicalPortDummySizeProcessor: it sets the width of hierarchical port
// dummies of northern and southern ports, and sets their layer alignment to
// CENTER. The widths make the vertical edge segments of the dummies of one
// layer keep the edge spacing.
type HierarchicalPortDummySizeProcessor struct{}

// NewHierarchicalPortDummySizeProcessor is new
// HierarchicalPortDummySizeProcessor().
func NewHierarchicalPortDummySizeProcessor() *HierarchicalPortDummySizeProcessor {
	return &HierarchicalPortDummySizeProcessor{}
}

// Process is process(layeredGraph, monitor).
func (*HierarchicalPortDummySizeProcessor) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	var northernDummies []*lgraph.LNode
	var southernDummies []*lgraph.LNode

	// Calculate the width difference (this assumes CENTER node alignment)
	//  The idea behind this is that ports are stacked on top of each other at
	//  the center of the node. By iteratively increasing their size by twice
	//  the edgeEdge spacing between layers, vertical edge segments are spaced
	//  by that amount since the edge's anchor will either be the very first
	//  or very last point of the port.
	edgeSpacing := prop.Get(layeredGraph, lopts.SpacingEdgeEdgeBetweenLayers)
	delta := float64(edgeSpacing * 2)

	// Iterate through the layers
	for _, layer := range layeredGraph.Layers {
		northernDummies = northernDummies[:0]
		southernDummies = southernDummies[:0]

		// Collect northern and southern hierarchical port dummies
		for _, node := range layer.Nodes {
			if node.Type == lgraph.NodeTypeExternalPort {
				side := prop.Get(node, iprops.ExtPortSide)
				if side == core.PortSideNorth {
					northernDummies = append(northernDummies, node)
				} else if side == core.PortSideSouth {
					southernDummies = append(southernDummies, node)
				}
			}
		}

		// Set widths
		setWidths(northernDummies, true, delta)
		setWidths(southernDummies, false, delta)
	}
}

// setWidths sets the widths of the given list of nodes and sets their layer
// alignment properly. With topDown the nodes widen with increasing index,
// else it is the other way round. delta is the width difference from one
// node to the next.
func setWidths(nodes []*lgraph.LNode, topDown bool, delta float64) {
	currentWidth := 0.0
	step := delta

	if !topDown {
		// Start with the widest node, decreasing node size
		currentWidth = float64(delta * float64(len(nodes)-1))
		step *= -1.0
	}

	for _, node := range nodes {
		prop.Set(node, lopts.Alignment, core.AlignmentCenter)
		node.Size.X = currentWidth

		// Move eastern ports to the node's right border
		for _, port := range node.PortsBySide(core.PortSideEast) {
			port.Pos.X = currentWidth
		}

		currentWidth += step
	}
}
