// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p5edges/OrthogonalEdgeRouter.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package p5edges has the edge routers of the layered algorithm: phase 5.
package p5edges

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p5edges/orthogonal"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p5edges/orthogonal/direction"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("OrthogonalEdgeRouter", func() lgraph.Processor { return NewOrthogonalEdgeRouter() })
}

// OrthogonalEdgeRouter is the phase OrthogonalEdgeRouter: it routes each edge
// with orthogonal bend points. It gives each node its horizontal coordinate
// and sets the width of the graph.
//
// getLayoutProcessorConfiguration of the Java class is ported in the package
// layered, with the configurations of the other phases.
type OrthogonalEdgeRouter struct{}

// NewOrthogonalEdgeRouter is new OrthogonalEdgeRouter().
func NewOrthogonalEdgeRouter() *OrthogonalEdgeRouter { return &OrthogonalEdgeRouter{} }

// Process is process(layeredGraph, monitor).
func (*OrthogonalEdgeRouter) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	// Retrieve some generic values
	nodeNodeSpacing := prop.Get(layeredGraph, lopts.SpacingNodeNodeBetweenLayers)
	edgeEdgeSpacing := prop.Get(layeredGraph, lopts.SpacingEdgeEdgeBetweenLayers)
	edgeNodeSpacing := prop.Get(layeredGraph, lopts.SpacingEdgeNodeBetweenLayers)

	// Prepare for iteration!
	routingGenerator := orthogonal.NewOrthogonalRoutingGenerator(direction.RoutingDirectionWestToEast, edgeEdgeSpacing, "phase5")
	// xpos is a float in Java. GWT computes it as a double.
	xpos := 0.0
	// layerIter is the index of the next layer of the list iterator.
	layerIter := 0
	var leftLayer *lgraph.Layer
	var rightLayer *lgraph.Layer
	var leftLayerNodes []*lgraph.LNode
	var rightLayerNodes []*lgraph.LNode
	leftLayerIndex := -1
	rightLayerIndex := -1

	// Iterate!
	for {
		// Fetch the next layer, if any
		rightLayer = nil
		if layerIter < len(layeredGraph.Layers) {
			rightLayer = layeredGraph.Layers[layerIter]
			layerIter++
		}
		rightLayerNodes = nil
		if rightLayer != nil {
			rightLayerNodes = rightLayer.Nodes
		}
		rightLayerIndex = layerIter - 1

		// Place the left layer's nodes, if any
		if leftLayer != nil {
			lgraph.PlaceNodesHorizontally(leftLayer, xpos)
			xpos += leftLayer.Size.X
		}

		// Route edges between the two layers
		startPos := xpos
		if leftLayer != nil {
			startPos = xpos + edgeNodeSpacing
		}
		slotsCount := routingGenerator.RouteEdges(layeredGraph, leftLayerNodes, leftLayerIndex, rightLayerNodes, startPos)

		isLeftLayerExternal := leftLayer == nil || allNodes(leftLayerNodes, PredExternalWestOrEastPort)
		isRightLayerExternal := rightLayer == nil || allNodes(rightLayerNodes, PredExternalWestOrEastPort)

		if slotsCount > 0 {
			// Compute routing area's width
			routingWidth := float64(float64(slotsCount-1) * edgeEdgeSpacing)

			if leftLayer != nil {
				routingWidth += edgeNodeSpacing
			}

			if rightLayer != nil {
				routingWidth += edgeNodeSpacing
			}

			// If we are between two layers, make sure their minimal spacing is
			// preserved
			if routingWidth < nodeNodeSpacing && !isLeftLayerExternal && !isRightLayerExternal {
				routingWidth = nodeNodeSpacing
			}
			xpos += routingWidth
		} else if !isLeftLayerExternal && !isRightLayerExternal {
			// If all edges are straight, use the usual spacing
			xpos += nodeNodeSpacing
		}

		leftLayer = rightLayer
		leftLayerNodes = rightLayerNodes
		leftLayerIndex = rightLayerIndex

		if rightLayer == nil {
			break
		}
	}

	layeredGraph.Size.X = xpos
}
