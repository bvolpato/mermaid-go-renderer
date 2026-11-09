// Ported from @mermaid-js/layout-tidy-tree 1.0.1, src/layout.ts and
// src/types.ts (MIT).
//
// validateLayoutData of layout.ts is not here: the package exports it, and
// its render does not call it.

package tidytree

import (
	"errors"
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// PositionedNode is PositionedNode of types.ts: a node after the layout
// calculation.
type PositionedNode struct {
	ID   string
	X, Y float64
	// Section is "root", "left" or "right".
	Section       string
	Width, Height float64
	OriginalNode  *ru.Node
}

// PositionedEdge is PositionedEdge of types.ts: an edge after the layout
// calculation. An edge with an end that the layout did not position has
// zero coordinates, one point, no sections and undefined (NaN) sizes.
type PositionedEdge struct {
	ID             string
	Source, Target string
	StartX, StartY float64
	MidX, MidY     float64
	EndX, EndY     float64
	Points         []ru.Point
	// SourceSection and TargetSection are "" when they are undefined.
	SourceSection, TargetSection string
	SourceWidth, SourceHeight    float64
	TargetWidth, TargetHeight    float64
}

// LayoutResult is LayoutResult of types.ts: the result of the layout
// algorithm execution.
type LayoutResult struct {
	Nodes []*PositionedNode
	Edges []*PositionedEdge
}

// tidyTreeNode is TidyTreeNode of types.ts: the node structure that
// non-layered-tidy-tree-layout reads and writes. x and y are undefined
// (NaN) before the layout.
type tidyTreeNode struct {
	id            string
	width, height float64
	x, y          float64
	children      []*tidyTreeNode
	originalNode  *ru.Node
}

// rangeError is a RangeError of JavaScript.
type rangeError string

func (e rangeError) Error() string { return string(e) }

// env is what the layout takes from the page.
type env struct {
	// random is Math.random.
	random func() float64
	// check stops a render whose context is done. It can be nil.
	check func()
	// converted counts the nodes that the conversion to a tree made.
	converted int
}

// executeTidyTreeLayout is executeTidyTreeLayout(data): it executes the
// tidy-tree layout algorithm on generic layout data.
//
// This function takes layout data and uses the non-layered-tidy-tree-layout
// algorithm to calculate optimal node positions for tree structures. nodes
// and edges are data.nodes and data.edges. The error is the value that the
// promise of upstream rejects with.
func executeTidyTreeLayout(e *env, nodes []*ru.Node, edges []*ru.Edge) (result *LayoutResult, err error) {
	intersectionShift := 50.0

	// try { ... } catch (error) { reject(error) }
	defer func() {
		if r := recover(); r != nil {
			switch thrown := r.(type) {
			case typeError:
				result, err = nil, thrown
			case rangeError:
				result, err = nil, thrown
			default:
				panic(r)
			}
		}
	}()

	if len(nodes) == 0 {
		return nil, errors.New("No nodes found in layout data")
	}

	// A value of data.edges that is not an array becomes an empty array. A
	// nil slice is an empty array here too.

	leftTree, rightTree, rootNode := convertToDualTreeFormat(e, nodes, edges)

	const gap = 20
	const bottomPadding = 40
	intersectionShift = 30

	bb := &boundingBox{gap: gap, bottomPadding: bottomPadding}
	layout := &treeLayout{bb: bb}

	var leftResult, rightResult *tidyTreeNode

	if leftTree != nil {
		leftResult, _ = layout.layout(leftTree)
	}

	if rightTree != nil {
		rightResult, _ = layout.layout(rightTree)
	}

	positionedNodes := combineAndPositionTrees(rootNode, leftResult, rightResult)
	positionedEdges := calculateEdgePositions(edges, positionedNodes, intersectionShift)
	return &LayoutResult{
		Nodes: positionedNodes,
		Edges: positionedEdges,
	}, nil
}

// convertToDualTreeFormat converts the layout data to the dual-tree format
// (left and right trees).
//
// This function builds two separate tree structures from the nodes and
// edges, alternating children between left and right trees.
func convertToDualTreeFormat(e *env, nodes []*ru.Node, edges []*ru.Edge) (leftTree, rightTree, rootNode *tidyTreeNode) {
	nodeMap := map[string]*ru.Node{}
	for _, node := range nodes {
		nodeMap[node.ID] = node
	}

	children := map[string][]string{}
	parents := map[string]string{}

	for _, edge := range edges {
		parentID := edge.Start
		childID := edge.End

		if parentID != "" && childID != "" {
			children[parentID] = append(children[parentID], childID)
			parents[childID] = parentID
		}
	}

	var rootNodeData *ru.Node
	for _, node := range nodes {
		if _, has := parents[node.ID]; !has {
			rootNodeData = node
			break
		}
	}
	// The caller checked that nodes is not empty.

	actualRoot := rootNodeData
	if actualRoot == nil {
		actualRoot = nodes[0]
	}

	rootNode = &tidyTreeNode{
		id:           actualRoot.ID,
		width:        ru.NN(actualRoot.Width, 100),
		height:       ru.NN(actualRoot.Height, 50),
		x:            ru.Undef,
		y:            ru.Undef,
		originalNode: actualRoot,
	}

	rootChildren := children[actualRoot.ID]
	leftChildren := []string{}
	rightChildren := []string{}

	for index, childID := range rootChildren {
		if index%2 == 0 {
			leftChildren = append(leftChildren, childID)
		} else {
			rightChildren = append(rightChildren, childID)
		}
	}

	if len(leftChildren) > 0 {
		leftTree = buildSubTree(e, leftChildren, children, nodeMap)
	}

	if len(rightChildren) > 0 {
		rightTree = buildSubTree(e, rightChildren, children, nodeMap)
	}

	return leftTree, rightTree, rootNode
}

// buildSubTree builds a subtree from a list of root children. For
// horizontal trees, the width and the height are transposed, since the tree
// will be rotated by 90 degrees.
func buildSubTree(e *env, rootChildren []string, children map[string][]string, nodeMap map[string]*ru.Node) *tidyTreeNode {
	virtualRoot := &tidyTreeNode{
		id:     "virtual-root-" + js.Str(e.random()),
		width:  1,
		height: 1,
		x:      ru.Undef,
		y:      ru.Undef,
	}
	virtualRoot.children = convertChildren(e, rootChildren, children, nodeMap, map[string]bool{})

	return virtualRoot
}

// convertChildren is the expression
// ids.map((childId) => nodeMap.get(childId)).filter((child) => child !== undefined).map((child) => convertNodeToTidyTreeTransposed(child, children, nodeMap)).
// The result is never nil.
func convertChildren(e *env, ids []string, children map[string][]string, nodeMap map[string]*ru.Node, path map[string]bool) []*tidyTreeNode {
	found := []*ru.Node{}
	for _, childID := range ids {
		if child, ok := nodeMap[childID]; ok {
			found = append(found, child)
		}
	}
	converted := make([]*tidyTreeNode, 0, len(found))
	for _, child := range found {
		converted = append(converted, convertNodeToTidyTreeTransposed(e, child, children, nodeMap, path))
	}
	return converted
}

// convertNodeToTidyTreeTransposed converts a node and its children to the
// tidy-tree format, recursively. This version transposes the width and the
// height for the horizontal tree layout.
//
// path has the ids of the nodes whose conversion is running. The conversion
// depends only on the id, so upstream never returns from a node that it
// reaches a second time on one path: it throws a RangeError when the call
// stack is full.
func convertNodeToTidyTreeTransposed(e *env, node *ru.Node, children map[string][]string, nodeMap map[string]*ru.Node, path map[string]bool) *tidyTreeNode {
	if path[node.ID] {
		panic(rangeError("Maximum call stack size exceeded"))
	}
	// A node with several parents is converted once for each of them, so a
	// graph can make a tree that is much larger than itself.
	e.converted++
	if e.check != nil && e.converted%4096 == 0 {
		e.check()
	}
	path[node.ID] = true
	childIDs := children[node.ID]
	childNodes := convertChildren(e, childIDs, children, nodeMap, path)
	delete(path, node.ID)

	result := &tidyTreeNode{
		id:           node.ID,
		width:        ru.NN(node.Height, 50),
		height:       ru.NN(node.Width, 100),
		x:            ru.Undef,
		y:            ru.Undef,
		originalNode: node,
	}
	if len(childNodes) > 0 {
		result.children = childNodes
	}
	return result
}

// combineAndPositionTrees combines and positions the left and right trees
// around the root node. It creates a bidirectional layout where the left
// tree grows left and the right tree grows right.
func combineAndPositionTrees(rootNode, leftResult, rightResult *tidyTreeNode) []*PositionedNode {
	positionedNodes := []*PositionedNode{}

	rootX := 0.0
	rootY := 0.0

	treeSpacing := float64(rootNode.width/2) + 30
	leftTreeNodes := []*PositionedNode{}
	rightTreeNodes := []*PositionedNode{}

	if leftResult != nil && leftResult.children != nil {
		leftTreeNodes = positionLeftTreeBidirectional(leftResult.children, leftTreeNodes, rootX-treeSpacing, rootY)
	}

	if rightResult != nil && rightResult.children != nil {
		rightTreeNodes = positionRightTreeBidirectional(rightResult.children, rightTreeNodes, rootX+treeSpacing, rootY)
	}

	leftTreeCenterY := 0.0
	rightTreeCenterY := 0.0

	if len(leftTreeNodes) > 0 {
		leftTreeXPositions := uniqueX(leftTreeNodes)
		js.SortFunc(leftTreeXPositions, func(a, b float64) float64 { return b - a })
		firstLevelLeftX := leftTreeXPositions[0]
		firstLevelLeftNodes := withX(leftTreeNodes, firstLevelLeftX)

		if len(firstLevelLeftNodes) > 0 {
			leftMinY := js.Min(tops(firstLevelLeftNodes)...)
			leftMaxY := js.Max(bottoms(firstLevelLeftNodes)...)
			leftTreeCenterY = (leftMinY + leftMaxY) / 2
		}
	}

	if len(rightTreeNodes) > 0 {
		rightTreeXPositions := uniqueX(rightTreeNodes)
		js.SortFunc(rightTreeXPositions, func(a, b float64) float64 { return a - b })
		firstLevelRightX := rightTreeXPositions[0]
		firstLevelRightNodes := withX(rightTreeNodes, firstLevelRightX)

		if len(firstLevelRightNodes) > 0 {
			rightMinY := js.Min(tops(firstLevelRightNodes)...)
			rightMaxY := js.Max(bottoms(firstLevelRightNodes)...)
			rightTreeCenterY = (rightMinY + rightMaxY) / 2
		}
	}

	leftTreeOffset := -leftTreeCenterY
	rightTreeOffset := -rightTreeCenterY

	positionedNodes = append(positionedNodes, &PositionedNode{
		ID:           rootNode.id,
		X:            rootX,
		Y:            rootY + 20,
		Section:      "root",
		Width:        ru.NN(rootNode.originalNode.Width, rootNode.width),
		Height:       ru.NN(rootNode.originalNode.Height, rootNode.height),
		OriginalNode: rootNode.originalNode,
	})

	leftTreeNodesWithOffset := make([]*PositionedNode, len(leftTreeNodes))
	for i, node := range leftTreeNodes {
		leftTreeNodesWithOffset[i] = &PositionedNode{
			ID:           node.ID,
			X:            node.X - float64(ru.NN(node.Width, 0)/2),
			Y:            node.Y + leftTreeOffset + float64(ru.NN(node.Height, 0)/2),
			Section:      "left",
			Width:        node.Width,
			Height:       node.Height,
			OriginalNode: node.OriginalNode,
		}
	}

	rightTreeNodesWithOffset := make([]*PositionedNode, len(rightTreeNodes))
	for i, node := range rightTreeNodes {
		rightTreeNodesWithOffset[i] = &PositionedNode{
			ID:           node.ID,
			X:            node.X + float64(ru.NN(node.Width, 0)/2),
			Y:            node.Y + rightTreeOffset + float64(ru.NN(node.Height, 0)/2),
			Section:      "right",
			Width:        node.Width,
			Height:       node.Height,
			OriginalNode: node.OriginalNode,
		}
	}

	positionedNodes = append(positionedNodes, leftTreeNodesWithOffset...)
	positionedNodes = append(positionedNodes, rightTreeNodesWithOffset...)

	return positionedNodes
}

// uniqueX is [...new Set(nodes.map((node) => node.x))]: the x values in the
// order of their first node. A Set holds -0 and +0 as one value.
func uniqueX(nodes []*PositionedNode) []float64 {
	seen := map[float64]bool{}
	xs := []float64{}
	for _, node := range nodes {
		if !seen[node.X] {
			seen[node.X] = true
			xs = append(xs, node.X)
		}
	}
	return xs
}

// withX is nodes.filter((node) => node.x === x).
func withX(nodes []*PositionedNode, x float64) []*PositionedNode {
	var found []*PositionedNode
	for _, node := range nodes {
		if node.X == x {
			found = append(found, node)
		}
	}
	return found
}

// tops is nodes.map((node) => node.y - (node.height ?? 50) / 2).
func tops(nodes []*PositionedNode) []float64 {
	values := make([]float64, len(nodes))
	for i, node := range nodes {
		values[i] = node.Y - float64(ru.NN(node.Height, 50)/2)
	}
	return values
}

// bottoms is nodes.map((node) => node.y + (node.height ?? 50) / 2).
func bottoms(nodes []*PositionedNode) []float64 {
	values := make([]float64, len(nodes))
	for i, node := range nodes {
		values[i] = node.Y + float64(ru.NN(node.Height, 50)/2)
	}
	return values
}

// positionLeftTreeBidirectional positions the nodes of the left tree in a
// bidirectional layout (it grows to the left). It rotates the tree by 90
// degrees counterclockwise, so that it grows horizontally to the left. It
// returns positionedNodes with the nodes added.
func positionLeftTreeBidirectional(nodes []*tidyTreeNode, positionedNodes []*PositionedNode, offsetX, offsetY float64) []*PositionedNode {
	for _, node := range nodes {
		distanceFromRoot := ru.NN(node.y, 0)
		verticalPosition := ru.NN(node.x, 0)

		originalWidth := 100.0
		originalHeight := 50.0
		if node.originalNode != nil {
			originalWidth = ru.NN(node.originalNode.Width, 100)
			originalHeight = ru.NN(node.originalNode.Height, 50)
		}

		adjustedY := offsetY + verticalPosition

		positionedNodes = append(positionedNodes, &PositionedNode{
			ID:           node.id,
			X:            offsetX - distanceFromRoot,
			Y:            adjustedY,
			Width:        originalWidth,
			Height:       originalHeight,
			OriginalNode: node.originalNode,
		})

		if node.children != nil {
			positionedNodes = positionLeftTreeBidirectional(node.children, positionedNodes, offsetX, offsetY)
		}
	}
	return positionedNodes
}

// positionRightTreeBidirectional positions the nodes of the right tree in a
// bidirectional layout (it grows to the right). It rotates the tree by 90
// degrees clockwise, so that it grows horizontally to the right. It returns
// positionedNodes with the nodes added.
func positionRightTreeBidirectional(nodes []*tidyTreeNode, positionedNodes []*PositionedNode, offsetX, offsetY float64) []*PositionedNode {
	for _, node := range nodes {
		distanceFromRoot := ru.NN(node.y, 0)
		verticalPosition := ru.NN(node.x, 0)

		originalWidth := 100.0
		originalHeight := 50.0
		if node.originalNode != nil {
			originalWidth = ru.NN(node.originalNode.Width, 100)
			originalHeight = ru.NN(node.originalNode.Height, 50)
		}

		adjustedY := offsetY + verticalPosition

		positionedNodes = append(positionedNodes, &PositionedNode{
			ID:           node.id,
			X:            offsetX + distanceFromRoot,
			Y:            adjustedY,
			Width:        originalWidth,
			Height:       originalHeight,
			OriginalNode: node.originalNode,
		})

		if node.children != nil {
			positionedNodes = positionRightTreeBidirectional(node.children, positionedNodes, offsetX, offsetY)
		}
	}
	return positionedNodes
}

// computeCircleEdgeIntersection calculates the intersection point of a line
// with a circle. circle has the center and the size of the circle, and
// lineStart and lineEnd are the ends of the line.
func computeCircleEdgeIntersection(circle ru.Bounds, lineStart, lineEnd ru.Point) ru.Point {
	radius := js.Min(circle.Width, circle.Height) / 2

	dx := lineEnd.X - lineStart.X
	dy := lineEnd.Y - lineStart.Y
	length := math.Sqrt(float64(dx*dx) + float64(dy*dy))

	if length == 0 {
		return lineStart
	}

	nx := dx / length
	ny := dy / length

	return ru.Point{
		X: circle.X - float64(nx*radius),
		Y: circle.Y - float64(ny*radius),
	}
}

// falsy is `!v` for a number: true for 0, for NaN and for undefined.
func falsy(v float64) bool {
	return v == 0 || v != v
}

func intersection(node *PositionedNode, outsidePoint, insidePoint ru.Point) ru.Point {
	x := node.X
	y := node.Y

	if falsy(node.Width) || falsy(node.Height) {
		return ru.Point{X: outsidePoint.X, Y: outsidePoint.Y}
	}
	// Upstream gives r a first value here, from dx = Math.abs(x -
	// insidePoint.x), and never reads it: both branches assign r again.
	w := float64(node.Width / 2)
	var r float64
	h := float64(node.Height / 2)

	Q := math.Abs(outsidePoint.Y - insidePoint.Y)
	R := math.Abs(outsidePoint.X - insidePoint.X)

	if math.Abs(y-outsidePoint.Y)*w > math.Abs(x-outsidePoint.X)*h {
		// Intersection is top or bottom of rect.
		var q float64
		if insidePoint.Y < outsidePoint.Y {
			q = outsidePoint.Y - h - y
		} else {
			q = y - h - outsidePoint.Y
		}
		r = (R * q) / Q
		var res ru.Point
		if insidePoint.X < outsidePoint.X {
			res.X = insidePoint.X + r
		} else {
			res.X = insidePoint.X - R + r
		}
		if insidePoint.Y < outsidePoint.Y {
			res.Y = insidePoint.Y + Q - q
		} else {
			res.Y = insidePoint.Y - Q + q
		}

		if r == 0 {
			res.X = outsidePoint.X
			res.Y = outsidePoint.Y
		}
		if R == 0 {
			res.X = outsidePoint.X
		}
		if Q == 0 {
			res.Y = outsidePoint.Y
		}

		return res
	} else {
		if insidePoint.X < outsidePoint.X {
			r = outsidePoint.X - w - x
		} else {
			r = x - w - outsidePoint.X
		}
		q := (Q * r) / R
		var _x, _y float64
		if insidePoint.X < outsidePoint.X {
			_x = insidePoint.X + R - r
		} else {
			_x = insidePoint.X - R + r
		}
		if insidePoint.Y < outsidePoint.Y {
			_y = insidePoint.Y + q
		} else {
			_y = insidePoint.Y - q
		}

		if r == 0 {
			_x = outsidePoint.X
			_y = outsidePoint.Y
		}
		if R == 0 {
			_x = outsidePoint.X
		}
		if Q == 0 {
			_y = outsidePoint.Y
		}

		return ru.Point{X: _x, Y: _y}
	}
}

// isRound is ['circle', 'cloud', 'bang'].includes(node.originalNode?.shape ?? ”).
func isRound(node *PositionedNode) bool {
	shape := ""
	if node.OriginalNode != nil {
		shape = node.OriginalNode.Shape
	}
	return shape == "circle" || shape == "cloud" || shape == "bang"
}

// circleOf is the first argument of computeCircleEdgeIntersection for a
// node: its center, and its size with 100 for an undefined side.
func circleOf(node *PositionedNode) ru.Bounds {
	return ru.Bounds{
		X:      node.X,
		Y:      node.Y,
		Width:  ru.NN(node.Width, 100),
		Height: ru.NN(node.Height, 100),
	}
}

// calculateEdgePositions calculates the edge positions based on the
// positioned nodes. It includes the tree membership and the node dimensions
// for precise edge calculations. Edges stop at shape boundaries instead of
// extending to centers.
func calculateEdgePositions(edges []*ru.Edge, positionedNodes []*PositionedNode, intersectionShift float64) []*PositionedEdge {
	nodeInfo := map[string]*PositionedNode{}
	for _, node := range positionedNodes {
		nodeInfo[node.ID] = node
	}

	result := make([]*PositionedEdge, len(edges))
	for i, edge := range edges {
		sourceNode := nodeInfo[edge.Start]
		targetNode := nodeInfo[edge.End]

		if sourceNode == nil || targetNode == nil {
			result[i] = &PositionedEdge{
				ID:           edge.ID,
				Source:       edge.Start,
				Target:       edge.End,
				StartX:       0,
				StartY:       0,
				MidX:         0,
				MidY:         0,
				EndX:         0,
				EndY:         0,
				Points:       []ru.Point{{X: 0, Y: 0}},
				SourceWidth:  ru.Undef,
				SourceHeight: ru.Undef,
				TargetWidth:  ru.Undef,
				TargetHeight: ru.Undef,
			}
			continue
		}

		sourceCenter := ru.Point{X: sourceNode.X, Y: sourceNode.Y}
		targetCenter := ru.Point{X: targetNode.X, Y: targetNode.Y}

		isSourceRound := isRound(sourceNode)
		isTargetRound := isRound(targetNode)

		var startPos ru.Point
		if isSourceRound {
			startPos = computeCircleEdgeIntersection(circleOf(sourceNode), targetCenter, sourceCenter)
		} else {
			startPos = intersection(sourceNode, sourceCenter, targetCenter)
		}

		var endPos ru.Point
		if isTargetRound {
			endPos = computeCircleEdgeIntersection(circleOf(targetNode), sourceCenter, targetCenter)
		} else {
			endPos = intersection(targetNode, targetCenter, sourceCenter)
		}

		midX := (startPos.X + endPos.X) / 2
		midY := (startPos.Y + endPos.Y) / 2

		points := []ru.Point{startPos}

		if sourceNode.Section == "left" {
			points = append(points, ru.Point{
				X: sourceNode.X - float64(ru.NN(sourceNode.Width, 0)/2) - intersectionShift,
				Y: sourceNode.Y,
			})
		} else if sourceNode.Section == "right" {
			points = append(points, ru.Point{
				X: sourceNode.X + float64(ru.NN(sourceNode.Width, 0)/2) + intersectionShift,
				Y: sourceNode.Y,
			})
		} else if sourceNode.Section == "root" {
			if targetNode.Section == "right" {
				points = append(points, ru.Point{
					X: sourceNode.X + float64(ru.NN(sourceNode.Width, 0)/2) + intersectionShift,
					Y: sourceNode.Y,
				})
			} else if targetNode.Section == "left" {
				points = append(points, ru.Point{
					X: sourceNode.X - float64(ru.NN(sourceNode.Width, 0)/2) - intersectionShift,
					Y: sourceNode.Y,
				})
			}
		}

		if targetNode.Section == "left" {
			points = append(points, ru.Point{
				X: targetNode.X + float64(ru.NN(targetNode.Width, 0)/2) + intersectionShift,
				Y: targetNode.Y,
			})
		} else if targetNode.Section == "right" {
			points = append(points, ru.Point{
				X: targetNode.X - float64(ru.NN(targetNode.Width, 0)/2) - intersectionShift,
				Y: targetNode.Y,
			})
		} else if targetNode.Section == "root" {
			if sourceNode.Section == "right" {
				points = append(points, ru.Point{
					X: targetNode.X + float64(ru.NN(targetNode.Width, 0)/2) + intersectionShift,
					Y: targetNode.Y,
				})
			} else if sourceNode.Section == "left" {
				points = append(points, ru.Point{
					X: targetNode.X - float64(ru.NN(targetNode.Width, 0)/2) - intersectionShift,
					Y: targetNode.Y,
				})
			}
		}

		points = append(points, endPos)

		secondPoint := targetCenter
		if len(points) > 1 {
			secondPoint = points[1]
		}
		if isSourceRound {
			startPos = computeCircleEdgeIntersection(circleOf(sourceNode), secondPoint, sourceCenter)
		} else {
			startPos = intersection(sourceNode, secondPoint, sourceCenter)
		}
		points[0] = startPos

		secondLastPoint := sourceCenter
		if len(points) > 1 {
			secondLastPoint = points[len(points)-2]
		}
		if isTargetRound {
			endPos = computeCircleEdgeIntersection(circleOf(targetNode), secondLastPoint, targetCenter)
		} else {
			endPos = intersection(targetNode, secondLastPoint, targetCenter)
		}
		points[len(points)-1] = endPos

		result[i] = &PositionedEdge{
			ID:            edge.ID,
			Source:        edge.Start,
			Target:        edge.End,
			StartX:        startPos.X,
			StartY:        startPos.Y,
			MidX:          midX,
			MidY:          midY,
			EndX:          endPos.X,
			EndY:          endPos.Y,
			Points:        points,
			SourceSection: sourceNode.Section,
			TargetSection: targetNode.Section,
			SourceWidth:   sourceNode.Width,
			SourceHeight:  sourceNode.Height,
			TargetWidth:   targetNode.Width,
			TargetHeight:  targetNode.Height,
		}
	}
	return result
}
