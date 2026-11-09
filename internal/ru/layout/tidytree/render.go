// Ported from @mermaid-js/layout-tidy-tree 1.0.1, src/render.ts and
// src/layouts.ts (MIT).

// Package tidytree is the port of @mermaid-js/layout-tidy-tree 1.0.1: the
// layout "tidy-tree", which is not a part of mermaid. A page adds it with
// mermaid.registerLayoutLoaders, and mermaid-cli 12 does that before it
// renders. So importing the package registers the layout for the page of
// mermaid-cli only (render.PageMermaidCLI). In a plain page the name
// "tidy-tree" is unknown, and a diagram falls back to another layout.
//
// Every diagram of the unified renderer can ask for the layout with
// `layout: tidy-tree` in its config: flowchart, state, class, ER,
// requirement, mindmap, usecase and agentflow. The layout takes the edges
// as the branches of a tree. It puts the root in the middle and the
// subtrees of its children in turn on the left and on the right.
//
// The tree layout is the algorithm of non-layered-tidy-tree-layout 2.0.2,
// which the package bundles (algorithm.go and helpers.go).
package tidytree

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
)

func init() {
	// layouts.ts: { name: 'tidy-tree', loader, algorithm: 'tidy-tree' }.
	ru.RegisterPageLayout(render.PageMermaidCLI, "tidy-tree", "tidy-tree", Render)
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

// Render is the render function for the bidirectional tidy-tree layout
// algorithm.
//
// This follows the same pattern as ELK and dagre renderers:
//  1. Insert nodes into DOM to get their actual dimensions
//  2. Run the bidirectional tidy-tree layout algorithm to calculate positions
//  3. Position the nodes and edges based on layout results
//
// The bidirectional layout creates two trees that grow horizontally in
// opposite directions from a central root node:
//   - Left tree: grows horizontally to the left (children: 1st, 3rd, 5th...)
//   - Right tree: grows horizontally to the right (children: 2nd, 4th, 6th...)
func Render(ctx *render.Context, data4Layout *ru.LayoutData, svg *dom.Node, _ string) error {
	nodeDb := map[string]*nodeWithPosition{}
	// clusterDb is a plain object upstream, and insertEdge reads it as a
	// Map. An edge that is cut at a cluster fails there, as it does here
	// with the nil map.
	var clusterDb edges.ClusterDb

	element := svg.Select("g")
	if err := edges.InsertMarkers(ctx, element, data4Layout.Markers, data4Layout.Type, data4Layout.DiagramID); err != nil {
		return err
	}

	subGraphsEl := element.Insert("g").Attr("class", "subgraphs")
	edgePaths := element.Insert("g").Attr("class", "edgePaths")
	edgeLabels := element.Insert("g").Attr("class", "edgeLabels")
	nodes := element.Insert("g").Attr("class", "nodes")
	// Step 1: Insert nodes into DOM to get their actual dimensions
	err := ctx.All(len(data4Layout.Nodes), func(i int) error {
		node := data4Layout.Nodes[i]
		if node.IsGroup {
			c := *node
			clusterNode := &nodeWithPosition{Node: &c}
			nodeDb[node.ID] = clusterNode
			if _, err := clusters.InsertCluster(ctx, subGraphsEl, node); err != nil {
				return err
			}
		} else {
			// The copy is made before the node is drawn, so it does not get
			// what the shape writes to the node: the copy has no intersect
			// function.
			c := *node
			nodeWithPosition := &nodeWithPosition{Node: &c}
			nodeDb[node.ID] = nodeWithPosition

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
	// Step 2: Run the bidirectional tidy-tree layout algorithm
	updatedNodes := make([]*ru.Node, len(data4Layout.Nodes))
	for i, node := range data4Layout.Nodes {
		nodeWithDimensions := nodeDb[node.ID]
		c := *node
		c.Width = ru.NN(ru.NN(nodeWithDimensions.Width, node.Width), 100)
		c.Height = ru.NN(ru.NN(nodeWithDimensions.Height, node.Height), 50)
		updatedNodes[i] = &c
	}

	layoutResult, err := executeTidyTreeLayout(&env{random: ctx.Random, check: ctx.CheckCanceled}, updatedNodes, data4Layout.Edges)
	// await executeTidyTreeLayout(...): the function is not async, and it
	// returns a promise.
	ctx.Await()
	if err != nil {
		return err
	}
	// Step 3: Position the nodes based on bidirectional layout results
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
		// Position the node at the calculated coordinates from bidirectional layout
		// The layout algorithm has already calculated positions for:
		// - Root node at center (0, 0)
		// - Left tree nodes with negative x coordinates (growing left)
		// - Right tree nodes with positive x coordinates (growing right)
		node.domEl.Attr("transform", "translate("+js.Str(positionedNode.X)+", "+js.Str(positionedNode.Y)+")")
		// Store the final position
		node.X = positionedNode.X
		node.Y = positionedNode.Y
	}

	err = ctx.All(len(data4Layout.Edges), func(i int) error {
		edge := data4Layout.Edges[i]
		if _, err := edges.InsertEdgeLabel(ctx, edgeLabels, edge); err != nil {
			return err
		}

		startNode := nodeDb[edge.Start]
		endNode := nodeDb[edge.End]

		if startNode != nil && endNode != nil {
			// layoutResult.edges.find((e) => e.id === edge.id)
			var positionedEdge *PositionedEdge
			for _, e := range layoutResult.Edges {
				if e.ID == edge.ID {
					positionedEdge = e
					break
				}
			}

			edgeWithPath := *edge
			if positionedEdge != nil {
				edgeWithPath.Points = positionedEdge.Points
			} else {
				edgeWithPath.Points = []ru.Point{
					{X: ru.NN(startNode.X, 0), Y: ru.NN(startNode.Y, 0)},
					{X: ru.NN(endNode.X, 0), Y: ru.NN(endNode.Y, 0)},
				}
			}
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
