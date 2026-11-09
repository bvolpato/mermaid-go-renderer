// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/cose-bilkent/index.ts, render.ts, layout.ts
// and types.ts (MIT).

// Package cosebilkent is the port of mermaid's cose-bilkent layout: the
// layout algorithm that rendering-util/render.ts registers under the name
// "cose-bilkent" and that the mindmap diagram uses. Importing the package
// registers the layout with ru.RegisterLayout.
//
// The layout draws the nodes to measure them, positions them with the
// cose-bilkent layout of cytoscape (internal/cytoscape runs the JavaScript
// of cytoscape for this) and draws each edge through the three points that
// cytoscape computes for it.
//
// The part of upstream that talks to cytoscape (cytoscape-setup.ts) is in
// internal/cytoscape, as JavaScript.
package cosebilkent

import (
	"context"
	"errors"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
)

func init() {
	ru.RegisterLayout("cose-bilkent", "", Render)
}

// typeError is a TypeError of JavaScript.
type typeError string

func (e typeError) Error() string { return string(e) }

// PositionedNode is PositionedNode of types.ts: a node after the layout.
type PositionedNode struct {
	ID   string
	X, Y float64
}

// PositionedEdge is PositionedEdge of types.ts: an edge after the layout.
type PositionedEdge struct {
	ID             string
	Source, Target string
	StartX, StartY float64
	MidX, MidY     float64
	EndX, EndY     float64
}

// LayoutResult is LayoutResult of types.ts.
type LayoutResult struct {
	Nodes []PositionedNode
	Edges []PositionedEdge
}

// nodeWithPosition is NodeWithPosition of render.ts: a shallow copy of a
// layout node, to which the render adds the measured size, the element and
// the position.
type nodeWithPosition struct {
	*ru.Node
	// domEl is domId when the render replaced it with the element of the
	// node. For a group node domId stays the id text.
	domEl *dom.Node
}

// Render is the render function of the cose-bilkent layout algorithm.
//
// This follows the same pattern as ELK and dagre renderers:
//  1. Insert nodes into DOM to get their actual dimensions
//  2. Run the layout algorithm to calculate positions
//  3. Position the nodes and edges based on layout results
func Render(ctx *render.Context, data4Layout *ru.LayoutData, svg *dom.Node, _ string) error {
	nodeDb := map[string]*nodeWithPosition{}
	// clusterDb is a plain object upstream, and insertEdge reads it as a
	// Map. An edge that is cut at a cluster fails there, as it does here
	// with the nil map.
	var clusterDb edges.ClusterDb

	// Insert markers for edges
	element := svg.Select("g")
	if err := edges.InsertMarkers(ctx, element, data4Layout.Markers, data4Layout.Type, data4Layout.DiagramID); err != nil {
		return err
	}

	// Create container groups
	subGraphsEl := element.Insert("g").Attr("class", "subgraphs")
	edgePaths := element.Insert("g").Attr("class", "edgePaths")
	edgeLabels := element.Insert("g").Attr("class", "edgeLabels")
	nodes := element.Insert("g").Attr("class", "nodes")

	// Step 1: Insert nodes into DOM to get their actual dimensions
	err := ctx.All(len(data4Layout.Nodes), func(i int) error {
		node := data4Layout.Nodes[i]
		if node.IsGroup {
			// Handle subgraphs/clusters
			c := *node
			clusterNode := &nodeWithPosition{Node: &c}
			nodeDb[node.ID] = clusterNode

			// Insert cluster to get dimensions
			if _, err := clusters.InsertCluster(ctx, subGraphsEl, node); err != nil {
				return err
			}
		} else {
			// Handle regular nodes. The copy is made before the node is
			// drawn, so it does not get what the shape writes to the node:
			// the copy has no intersect function.
			c := *node
			nodeWithPosition := &nodeWithPosition{Node: &c}
			nodeDb[node.ID] = nodeWithPosition

			// Insert node to get actual dimensions
			dir := data4Layout.Direction
			if dir == "" {
				dir = "TB"
			}
			nodeEl, err := ru.InsertNode(ctx, nodes, node, &ru.ShapeRenderOptions{
				Config:  data4Layout.Config,
				Dir:     dir,
				Padding: ru.Undef,
			})
			if err != nil {
				return err
			}

			// Get the actual bounding box after insertion
			if nodeEl == nil {
				return typeError("Cannot read properties of null (reading 'getBBox')")
			}
			boundingBox := nodeEl.BBox()
			nodeWithPosition.Width = boundingBox.Width
			nodeWithPosition.Height = boundingBox.Height
			nodeWithPosition.domEl = nodeEl
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Step 2: Run the cose-bilkent layout algorithm

	// Update the layout data with actual dimensions
	updatedNodes := make([]*ru.Node, len(data4Layout.Nodes))
	for i, node := range data4Layout.Nodes {
		nodeWithDimensions := nodeDb[node.ID]
		c := *node
		c.Width = nodeWithDimensions.Width
		c.Height = nodeWithDimensions.Height
		updatedNodes[i] = &c
	}

	layoutResult, err := executeCoseBilkentLayout(ctx, data4Layout, updatedNodes)
	if err != nil {
		return err
	}

	// Step 3: Position the nodes based on layout results
	for _, positionedNode := range layoutResult.Nodes {
		node := nodeDb[positionedNode.ID]
		if node == nil {
			continue
		}
		if node.domEl == nil {
			if node.DomID != "" {
				// The domId of a group node is still its id text.
				return typeError("node.domId.attr is not a function")
			}
			continue
		}
		// Position the node at the calculated coordinates
		// The positionedNode.x/y represents the center of the node, so use directly
		node.domEl.Attr("transform", "translate("+js.Str(positionedNode.X)+", "+js.Str(positionedNode.Y)+")")

		// Store the final position
		node.X = positionedNode.X
		node.Y = positionedNode.Y
	}

	for _, positionedEdge := range layoutResult.Edges {
		for _, edge := range data4Layout.Edges {
			if edge.ID == positionedEdge.ID {
				// Update the edge data with positioned coordinates
				edge.Points = []ru.Point{
					{X: positionedEdge.StartX, Y: positionedEdge.StartY},
					{X: positionedEdge.MidX, Y: positionedEdge.MidY},
					{X: positionedEdge.EndX, Y: positionedEdge.EndY},
				}
				break
			}
		}
	}

	// Step 4: Insert and position edges
	err = ctx.All(len(data4Layout.Edges), func(i int) error {
		edge := data4Layout.Edges[i]
		// Insert edge label first
		if _, err := edges.InsertEdgeLabel(ctx, edgeLabels, edge); err != nil {
			return err
		}

		// Get start and end nodes
		startNode := nodeDb[edge.Start]
		endNode := nodeDb[edge.End]

		if startNode != nil && endNode != nil {
			// Find the positioned edge data
			found := false
			for _, e := range layoutResult.Edges {
				if e.ID == edge.ID {
					found = true
					break
				}
			}

			// Create edge path with positioned coordinates
			edgeWithPath := *edge
			if !found {
				// Fallback: create a simple straight line between nodes
				edgeWithPath.Points = []ru.Point{
					{X: ru.Or(startNode.X, 0), Y: ru.Or(startNode.Y, 0)},
					{X: ru.Or(endNode.X, 0), Y: ru.Or(endNode.Y, 0)},
				}
			}

			// Insert the edge path
			paths, err := edges.InsertEdge(
				ctx,
				edgePaths,
				&edgeWithPath,
				clusterDb,
				data4Layout.Type,
				startNode.Node,
				endNode.Node,
				data4Layout.DiagramID,
				false,
				"",
			)
			if err != nil {
				return err
			}

			// Position the edge label
			if err := edges.PositionEdgeLabel(ctx, &edgeWithPath, paths); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	// The await of the caller. See render/async.go for the convention.
	ctx.Await()
	return nil
}

// executeCoseBilkentLayout is executeCoseBilkentLayout(data, config): it
// runs the cose-bilkent layout of cytoscape on the layout data. nodes are
// the nodes of the layout data with their measured sizes.
func executeCoseBilkentLayout(ctx *render.Context, data *ru.LayoutData, nodes []*ru.Node) (*LayoutResult, error) {
	// Validate layout data structure
	if err := validateLayoutData(data); err != nil {
		return nil, err
	}

	// Create and configure cytoscape instance, and extract the positioned
	// nodes and edges after layout: addNodes, addEdges,
	// createCytoscapeInstance, extractPositionedNodes and
	// extractPositionedEdges of cytoscape-setup.ts run in JavaScript.
	cyNodes := make([]cytoscape.Node, len(nodes))
	for i, node := range nodes {
		cyNodes[i] = cytoscape.Node{
			ID:     node.ID,
			Width:  node.Width,
			Height: node.Height,
			// position: { x: node.x ?? 0, y: node.y ?? 0 }
			X: ru.NN(node.X, 0),
			Y: ru.NN(node.Y, 0),
		}
	}
	cyEdges := make([]cytoscape.Edge, len(data.Edges))
	for i, edge := range data.Edges {
		cyEdges[i] = cytoscape.Edge{ID: edge.ID, Start: edge.Start, End: edge.End}
	}
	// The layout stops when the context of the caller is done.
	cancel := ctx.Cancel
	if cancel == nil {
		cancel = context.Background()
	}
	res, err := cytoscape.CoseBilkent(cancel, cytoscape.Env{Random: ctx.Random, Now: ctx.Now}, cyNodes, cyEdges)
	if err != nil {
		// A context that is done ends the render with the error of the
		// context, as every long step of a render does.
		ctx.CheckCanceled()
		return nil, err
	}
	result := &LayoutResult{
		Nodes: make([]PositionedNode, len(res.Nodes)),
		Edges: make([]PositionedEdge, len(res.Edges)),
	}
	for i, n := range res.Nodes {
		result.Nodes[i] = PositionedNode{ID: n.ID, X: n.X, Y: n.Y}
	}
	for i, e := range res.Edges {
		result.Edges[i] = PositionedEdge{
			ID: e.ID, Source: e.Source, Target: e.Target,
			StartX: e.StartX, StartY: e.StartY,
			MidX: e.MidX, MidY: e.MidY,
			EndX: e.EndX, EndY: e.EndY,
		}
	}
	// The await of the caller.
	ctx.Await()
	return result, nil
}

// validateLayoutData is validateLayoutData(data): it returns the error that
// upstream throws for layout data that the layout cannot use.
func validateLayoutData(data *ru.LayoutData) error {
	if data == nil {
		return errors.New("Layout data is required")
	}

	if data.Config == nil {
		return errors.New("Configuration is required in layout data")
	}

	// rootNode is a property that the mindmap db adds to its layout data.
	if !js.Truthy(data.Extra["rootNode"]) {
		return errors.New("Root node is required")
	}

	if data.Nodes == nil {
		// A diagram db returns an array, which can be empty. A nil slice
		// stands for a value that is not an array.
		return errors.New("No nodes found in layout data")
	}

	if data.Edges == nil {
		return errors.New("Edges array is required in layout data")
	}

	return nil
}
