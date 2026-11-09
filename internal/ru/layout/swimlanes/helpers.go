// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/helpers.ts (MIT).

// Package swimlanes is the port of mermaid's swimlane layout: the layout
// algorithm that rendering-util/render.ts registers under the name
// "swimlane" (rendering-util/layout-algorithms/swimlanes). Importing the
// package registers the layout with ru.RegisterLayout.
//
// The layout is a Sugiyama pipeline with lanes (cycle removal, layering,
// dummy nodes, ordering, coordinates), an orthogonal edge router and a
// chain of cleanup passes on the routed polylines. Each top-level group of
// the layout data is a lane.
//
// # Data model
//
// Upstream works on the node and edge objects of the layout data. Here
// these are *ru.Node and *ru.Edge. A number that is undefined is NaN, as
// in package ru. The properties that only this layout sets are extra
// properties of the node: "direction" (a string) and "swimlaneContentTop"
// (a float64). A point of an edge is a value (ru.Point): the places where
// upstream changes a point object in place work on fresh copies, so no
// sharing is lost.
//
// A lane id that is null upstream is the empty string here. A lane id is
// the id of a group, which is never empty.
package swimlanes

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// NodeId is the id of a node.
type NodeId = string

// EdgeId is the id of an edge.
type EdgeId = string

// Point is a point of an edge.
type Point = ru.Point

// Node is a node of the layout data.
type Node = ru.Node

// EdgeRef is an edge of the graph view. Upstream also names it Edge.
type EdgeRef struct {
	ID  EdgeId
	Src NodeId
	Dst NodeId
	// Weight is undefined (NaN): nothing sets it.
	Weight float64
	Ref    *ru.Edge
}

// Graph is the view of the layout data that the Sugiyama phases work on.
type Graph struct {
	Nodes    []NodeId
	Edges    []*EdgeRef
	Layout   *ru.LayoutData
	NodeById *js.Map[NodeId, *ru.Node]
}

// Layering is the result of the layering phase. RankOf has no entry for a
// node whose rank is undefined.
type Layering struct {
	Layers [][]NodeId
	RankOf map[NodeId]int
	Dummy  *js.Set[NodeId]
}

// OrderedLayers is the result of the ordering phase.
type OrderedLayers struct {
	Layers [][]NodeId
}

// Coordinates is the result of the coordinate phase.
type Coordinates struct {
	X map[NodeId]float64
	Y map[NodeId]float64
}

// DEFAULT_SWIMLANE_ID is the id of the lane that holds the nodes without a
// lane.
const DEFAULT_SWIMLANE_ID = "__swimlane_default__"

// WriteBackOptions are the options of writeBackToLayoutData. A gap that is
// undefined is NaN.
type WriteBackOptions struct {
	LayerGap float64
	NodeGap  float64
}

// Captured swimlane fixtures use 21px as the stable single-line label height.
const TOP_LANE_TITLE_BAND_HEIGHT = 21
const MIN_TOP_LANE_HORIZONTAL_PADDING = 20

func topLaneHorizontalPadding(lane *ru.Node) float64 {
	return js.Max(ru.NN(lane.Padding, MIN_TOP_LANE_HORIZONTAL_PADDING), MIN_TOP_LANE_HORIZONTAL_PADDING)
}

// isFiniteNumber is `typeof v === 'number' && Number.isFinite(v)`.
func isFiniteNumber(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// swimlaneContentTop is node.swimlaneContentTop, or undefined (NaN).
func swimlaneContentTop(node *ru.Node) float64 {
	if v, ok := node.Get("swimlaneContentTop").(float64); ok {
		return v
	}
	return ru.Undef
}

// nodeDirection is node.direction, or "" when it is undefined.
func nodeDirection(node *ru.Node) (string, bool) {
	v, ok := node.Get("direction").(string)
	return v, ok
}

func assignTopLaneTitleRect(lane *ru.Node) {
	x, y, width, height := lane.X, lane.Y, lane.Width, lane.Height
	contentTop := swimlaneContentTop(lane)
	if !isFiniteNumber(x) ||
		!isFiniteNumber(y) ||
		!isFiniteNumber(width) ||
		!isFiniteNumber(height) ||
		!isFiniteNumber(contentTop) ||
		width <= 0 ||
		height <= 0 {
		lane.GroupTitleRect = nil
		return
	}

	top := y - float64(height/2)
	headerBottom := js.Min(contentTop, y+float64(height/2))
	titleHeight := js.Min(TOP_LANE_TITLE_BAND_HEIGHT, js.Max(0, headerBottom-top))
	bottom := top + titleHeight
	if bottom <= top {
		lane.GroupTitleRect = nil
		return
	}

	lane.GroupTitleRect = &ru.GroupTitleRect{
		Left:   x - float64(width/2),
		Right:  x + float64(width/2),
		Top:    top,
		Bottom: bottom,
	}
}

// prepareLayoutForSwimlanes makes every top-level group a lane and puts the
// nodes without a lane into a default lane.
func prepareLayoutForSwimlanes(layout *ru.LayoutData) {
	direction := layout.Direction
	for _, node := range layout.Nodes {
		if node.IsGroup && node.ParentID == "" {
			node.Shape = "swimlane"
			if direction != "" {
				node.Set("direction", direction)
			}
		}
	}

	var looseNodes []*ru.Node
	for _, node := range layout.Nodes {
		if !node.IsGroup && node.ParentID == "" {
			looseNodes = append(looseNodes, node)
		}
	}
	if len(looseNodes) == 0 {
		return
	}

	var defaultLane *ru.Node
	for _, node := range layout.Nodes {
		if node.ID == DEFAULT_SWIMLANE_ID {
			defaultLane = node
			break
		}
	}
	if defaultLane == nil {
		// Synthesised, so nothing upstream gave it a `look` or a slot. Without
		// `look` it renders classic in a handDrawn diagram; slot 0 would clash
		// with the first lane.
		defaultLane = ru.NewNode(DEFAULT_SWIMLANE_ID)
		defaultLane.Label, defaultLane.HasLabel = "", true
		defaultLane.IsGroup = true
		defaultLane.Shape = "swimlane"
		defaultLane.Padding = 20
		if look := js.Path(layout.Config, "look"); !js.IsNullish(look) {
			defaultLane.Look = js.ToString(look)
		}
		max := -1.0
		for _, node := range layout.Nodes {
			max = js.Max(max, ru.NN(node.ColorIndex, -1))
		}
		defaultLane.ColorIndex = max + 1
		if direction != "" {
			defaultLane.Set("direction", direction)
		}
		layout.Nodes = append(layout.Nodes, defaultLane)
	} else if defaultLane.IsGroup {
		defaultLane.Shape = "swimlane"
		if direction != "" {
			defaultLane.Set("direction", direction)
		}
	}

	for _, node := range looseNodes {
		node.ParentID = DEFAULT_SWIMLANE_ID
	}
}

// toGraphView builds the graph view of the layout data.
func toGraphView(layout *ru.LayoutData) *Graph {
	nodeById := js.NewMap[NodeId, *ru.Node]()
	for _, n := range layout.Nodes {
		nodeById.Set(n.ID, n)
	}

	edges := []*EdgeRef{}
	for _, e := range layout.Edges {
		src := e.Start
		dst := e.End
		if src == "" || dst == "" {
			continue
		}
		// Exclude labelled originals from Sugiyama: their routing is carried by
		// the two layout-only virtual edges A→label and label→B, which create
		// the correct layer/ordering constraints. Including the original as
		// well would double-count rank pressure and inflate crossing penalties.
		if e.LabelNodeID != "" {
			continue
		}
		edges = append(edges, &EdgeRef{ID: e.ID, Src: src, Dst: dst, Weight: ru.Undef, Ref: e})
	}

	var groupNodes, nonGroupNodes []*ru.Node
	for _, n := range layout.Nodes {
		if n.IsGroup {
			groupNodes = append(groupNodes, n)
		} else {
			nonGroupNodes = append(nonGroupNodes, n)
		}
	}

	nodes := make([]NodeId, 0, len(layout.Nodes))
	for i := len(groupNodes) - 1; i >= 0; i-- {
		nodes = append(nodes, groupNodes[i].ID)
	}
	for _, n := range nonGroupNodes {
		nodes = append(nodes, n.ID)
	}
	return &Graph{Nodes: nodes, Edges: edges, Layout: layout, NodeById: nodeById}
}

type groupBound struct {
	minX, maxX, minY, maxY float64
}

// writeBackToLayoutData writes the layers, the orders and the coordinates
// to the nodes of the layout data and computes the bounds of the groups.
func writeBackToLayoutData(g *Graph, ordered *OrderedLayers, coords *Coordinates, opts *WriteBackOptions) {
	layout := g.Layout
	nodeMap := g.NodeById
	layerGap := 100.0
	nodeGap := 40.0
	if opts != nil {
		layerGap = ru.NN(opts.LayerGap, 100)
		nodeGap = ru.NN(opts.NodeGap, 40)
	}

	layerIndex := 0
	for _, layer := range ordered.Layers {
		orderIndex := 0
		for _, id := range layer {
			node, ok := nodeMap.Lookup(id)
			if !ok || node == nil {
				orderIndex++
				continue
			}
			node.Layer = float64(layerIndex)
			node.Order = float64(orderIndex)
			x, ok := coords.X[id]
			if !ok {
				x = float64(orderIndex) * nodeGap
			}
			y, ok := coords.Y[id]
			if !ok {
				y = float64(layerIndex) * layerGap
			}
			node.X = x
			node.Y = y
			orderIndex++
		}
		layerIndex++
	}

	allNodes := layout.Nodes
	groupBounds := js.NewMap[NodeId, groupBound]()
	var topLevelGroups []*ru.Node
	for _, group := range allNodes {
		if group == nil || !group.IsGroup {
			continue
		}
		if group.ParentID == "" {
			topLevelGroups = append(topLevelGroups, group)
		}
		minX := math.Inf(1)
		maxX := math.Inf(-1)
		minY := math.Inf(1)
		maxY := math.Inf(-1)
		for _, child := range allNodes {
			if child.ParentID != group.ID {
				continue
			}
			// child.x ?? coords.x[child.id]
			cx, hasCx := child.X, !ru.IsUndef(child.X)
			if !hasCx {
				cx, hasCx = coords.X[child.ID]
			}
			cy, hasCy := child.Y, !ru.IsUndef(child.Y)
			if !hasCy {
				cy, hasCy = coords.Y[child.ID]
			}
			cw := ru.NN(child.Width, 0)
			ch := ru.NN(child.Height, 0)
			if hasCx && hasCy {
				minX = js.Min(minX, cx-float64(cw/2))
				maxX = js.Max(maxX, cx+float64(cw/2))
				minY = js.Min(minY, cy-float64(ch/2))
				maxY = js.Max(maxY, cy+float64(ch/2))
			}
		}
		if minX == math.Inf(1) || minY == math.Inf(1) {
			group.X = ru.NN(group.X, 0)
			group.Y = ru.NN(group.Y, 0)
			group.Width = ru.NN(group.Width, 0)
			group.Height = ru.NN(group.Height, 0)
		} else {
			pad := ru.NN(group.Padding, 20)
			horizontalPad := float64(2 * topLaneHorizontalPadding(group))
			if group.ParentID != "" {
				horizontalPad = pad
			}
			verticalPad := pad
			w := js.Max(0, maxX-minX) + horizontalPad
			h := js.Max(0, maxY-minY) + verticalPad
			cx := (minX + maxX) / 2
			cy := (minY + maxY) / 2
			group.X = cx
			group.Y = cy
			group.Width = w
			group.Height = h
			groupBounds.Set(group.ID, groupBound{minX: minX, maxX: maxX, minY: minY, maxY: maxY})
		}
	}

	if len(topLevelGroups) > 0 && groupBounds.Len() > 0 {
		globalMinY := math.Inf(1)
		globalMaxY := math.Inf(-1)
		maxPad := 0.0
		for _, lane := range topLevelGroups {
			pad := ru.NN(lane.Padding, 20)
			if pad > maxPad {
				maxPad = pad
			}
			b, ok := groupBounds.Lookup(lane.ID)
			if !ok {
				continue
			}
			globalMinY = js.Min(globalMinY, b.minY)
			globalMaxY = js.Max(globalMaxY, b.maxY)
		}
		if globalMinY != math.Inf(1) && globalMaxY != math.Inf(-1) {
			contentHeight := js.Max(0, globalMaxY-globalMinY)
			const minHeaderMargin = 36
			verticalMargin := js.Max(maxPad, minHeaderMargin)
			laneHeight := contentHeight + float64(2*verticalMargin)
			centerY := (globalMinY + globalMaxY) / 2
			for _, lane := range topLevelGroups {
				lane.Y = centerY
				lane.Height = laneHeight
				lane.Set("swimlaneContentTop", globalMinY)
			}

			sortedLanes := append([]*ru.Node{}, topLevelGroups...)
			sortFunc(sortedLanes, func(a, b *ru.Node) float64 {
				ax := ru.NN(a.X, 0)
				bx := ru.NN(b.X, 0)
				return ax - bx
			})

			var laneIds []NodeId
			var centers []float64
			var baseWidths []float64

			for _, lane := range sortedLanes {
				b, ok := groupBounds.Lookup(lane.ID)
				if !ok {
					continue
				}
				contentWidth := js.Max(0, b.maxX-b.minX) + float64(2*topLaneHorizontalPadding(lane))
				cx := (b.minX + b.maxX) / 2
				laneIds = append(laneIds, lane.ID)
				centers = append(centers, cx)
				baseWidths = append(baseWidths, contentWidth)
			}

			count := len(laneIds)
			if count > 0 {
				laneWidths := map[NodeId]float64{}

				if count == 1 {
					laneWidths[laneIds[0]] = baseWidths[0]
				} else {
					d := make([]float64, 0, count-1)
					for i := 0; i < count-1; i++ {
						d = append(d, centers[i+1]-centers[i])
					}

					u := make([]float64, count)
					u[0] = 0
					for i := 0; i < count-1; i++ {
						u[i+1] = float64(2*d[i]) - u[i]
					}

					lowerBound := 0.0
					upperBound := math.Inf(1)
					for i := 0; i < count; i++ {
						baseW := baseWidths[i]
						if i%2 == 0 {
							lowerBound = js.Max(lowerBound, baseW-u[i])
						} else {
							upperBound = js.Min(upperBound, u[i]-baseW)
						}
					}

					x := lowerBound
					if lowerBound <= upperBound {
						x = (lowerBound + upperBound) / 2
					} else {
						x = lowerBound
					}

					for i := 0; i < count; i++ {
						w := u[i] - x
						if i%2 == 0 {
							w = u[i] + x
						}
						finalWidth := js.Max(baseWidths[i], w)
						laneWidths[laneIds[i]] = finalWidth
					}
				}

				for _, lane := range topLevelGroups {
					if w, ok := laneWidths[lane.ID]; ok {
						lane.Width = w
					}
					assignTopLaneTitleRect(lane)
				}
			}
		}
	}
}
