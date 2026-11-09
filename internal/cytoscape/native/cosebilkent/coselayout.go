// Ported from cose-base 1.0.3 (MIT license): src/CoSELayout.js.

package cosebilkent

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

func (l *Layout) newGraph() *Graph { return newGraph(l.graphManager) }

// newNode is CoSELayout.newNode(vNode): a node without a rectangle.
func (l *Layout) newNode() *Node { return newNodeWithoutRect(l.graphManager) }

func (l *Layout) newEdge() *Edge { return newEdge() }

// initParameters is CoSELayout.initParameters.
func (l *Layout) initParameters() {
	l.fdInitParameters()
	if !l.isSubLayout {
		if defaultEdgeLength < 10 {
			l.idealEdgeLength = 10
		} else {
			l.idealEdgeLength = defaultEdgeLength
		}

		l.useSmartIdealEdgeLengthCalculation = defaultUseSmartIdealEdgeLengthCalculation
		l.springConstant = defaultSpringStrength
		l.repulsionConstant = defaultRepulsionStrength
		l.gravityConstant = defaultGravityStrength
		l.compoundGravityConstant = defaultCompoundGravityStrength
		l.gravityRangeFactor = defaultGravityRangeFactor
		l.compoundGravityRangeFactor = defaultCompoundGravityRangeFactor

		// variables for tree reduction support
		l.prunedNodesAll = nil
		l.growTreeIterations = 0
		l.afterGrowthIterations = 0
		l.isTreeGrowing = false
		l.isGrowthFinished = false

		// variables for cooling
		l.coolingCycle = 0
		l.maxCoolingCycle = l.maxIterations / convergenceCheckPeriod
		l.finalTemperature = convergenceCheckPeriod / l.maxIterations
		l.coolingAdjuster = 1
	}
}

// layout is CoSELayout.layout.
func (l *Layout) layout() bool {
	if defaultCreateBendsAsNeeded {
		bail.Out("cose-bilkent: bend points")
	}

	l.level = 0
	return l.classicLayout()
}

func (l *Layout) classicLayout() bool {
	l.nodesWithGravity = l.calculateNodesToApplyGravitationTo()
	l.graphManager.setAllNodesToApplyGravitation(l.nodesWithGravity)
	l.calcNoOfChildrenForAllNodes()
	l.graphManager.calcLowestCommonAncestors()
	l.graphManager.calcInclusionTreeDepths(l.graphManager.rootGraph, 1)
	l.graphManager.getRoot().calcEstimatedSize()
	l.calcIdealEdgeLengths()

	if !l.incremental {
		forest := l.getFlatForest()

		// The graph associated with this layout is flat and a forest
		if len(forest) > 0 {
			l.positionNodesRadially(forest)
		} else {
			// The graph associated with this layout is not flat or a forest:
			// the JavaScript reduces the trees and scatters the nodes.
			bail.Out("cose-bilkent: the graph is not a forest")
		}
	} else {
		bail.Out("cose-bilkent: an incremental layout")
	}

	l.initSpringEmbedder()
	if l.trace != nil {
		l.trace("cose.initial", l.tracePositions(nil))
	}
	l.runSpringEmbedder()

	return true
}

// tick is one iteration of the spring embedder. It returns true when the
// layout ended.
func (l *Layout) tick() bool {
	l.totalIterations++

	if l.totalIterations == l.maxIterations && !l.isTreeGrowing && !l.isGrowthFinished {
		if len(l.prunedNodesAll) > 0 {
			l.isTreeGrowing = true
		} else {
			return true
		}
	}

	if math.Mod(l.totalIterations, convergenceCheckPeriod) == 0 && !l.isTreeGrowing && !l.isGrowthFinished {
		if l.isConverged() {
			if len(l.prunedNodesAll) > 0 {
				l.isTreeGrowing = true
			} else {
				return true
			}
		}

		l.coolingCycle++

		if l.layoutQuality == 0 {
			// quality - "draft"
			l.coolingAdjuster = l.coolingCycle
		} else if l.layoutQuality == 1 {
			// quality - "default"
			l.coolingAdjuster = l.coolingCycle / 3
		}

		// cooling schedule is based on http://www.btluke.com/simanf1.html -> cooling schedule 3
		l.coolingFactor = max(l.initialCoolingFactor-float64(js.Pow(l.coolingCycle, js.Log(100*(l.initialCoolingFactor-l.finalTemperature))/js.Log(l.maxCoolingCycle))/100*l.coolingAdjuster), l.finalTemperature)
		l.animationPeriod = math.Ceil(l.initialAnimationPeriod * math.Sqrt(l.coolingFactor))
	}
	// Operations while tree is growing again
	if l.isTreeGrowing {
		bail.Out("cose-bilkent: the growth of pruned trees")
	}
	// Operations after growth is finished
	if l.isGrowthFinished {
		bail.Out("cose-bilkent: the growth of pruned trees")
	}

	gridUpdateAllowed := !l.isTreeGrowing && !l.isGrowthFinished
	forceToNodeSurroundingUpdate := l.growTreeIterations%10 == 1 && l.isTreeGrowing || l.afterGrowthIterations%10 == 1 && l.isGrowthFinished

	l.totalDisplacement = 0
	l.graphManager.updateBounds()
	l.calcSpringForces()
	l.calcRepulsionForces(gridUpdateAllowed, forceToNodeSurroundingUpdate)
	l.calcGravitationalForces()
	if l.trace != nil && l.traceIteration() {
		l.trace("cose.forces", l.traceForces())
	}
	l.moveNodes()
	l.animate()

	return false // Layout is not ended yet return false
}

func (l *Layout) runSpringEmbedder() {
	l.initialAnimationPeriod = 25
	l.animationPeriod = l.initialAnimationPeriod
	layoutEnded := false

	// If aminate option is 'during' signal that layout is supposed to start iterating
	if layoutConstantsAnimateDuring {
		bail.Out("cose-bilkent: an animation during the layout")
	} else {
		// If aminate option is 'during' tick() function will be called on index.js
		for !layoutEnded {
			if l.cancelled != nil && math.Mod(l.totalIterations, 64) == 0 && l.cancelled() {
				bail.Out("cose-bilkent: the caller cancelled the layout")
			}
			layoutEnded = l.tick()
			if l.trace != nil && (l.traceIteration() || layoutEnded) {
				l.trace("cose.tick", l.tracePositions([]float64{l.totalIterations, l.coolingFactor, l.totalDisplacement, boolFloat(layoutEnded)}))
			}
		}

		l.graphManager.updateBounds()
	}
}

func (l *Layout) calculateNodesToApplyGravitationTo() []*Node {
	var nodeList []*Node

	graphs := l.graphManager.getGraphs()
	for _, graph := range graphs {
		graph.updateConnected()

		if !graph.isConnected {
			nodeList = append(nodeList, graph.getNodes()...)
		}
	}

	return nodeList
}

func (l *Layout) positionNodesRadially(forest [][]*Node) {
	// We tile the trees to a grid row by row; first tree starts at (0,0)
	currentStartingPoint := PointD{0, 0}
	numberOfColumns := int(math.Ceil(math.Sqrt(float64(len(forest)))))
	height := 0.0
	currentY := 0.0
	currentX := 0.0
	point := PointD{0, 0}

	for i := 0; i < len(forest); i++ {
		if i%numberOfColumns == 0 {
			// Start of a new row, make the x coordinate 0, increment the
			// y coordinate with the max height of the previous row
			currentX = 0
			currentY = height

			if i != 0 {
				currentY += defaultComponentSeperation
			}

			height = 0
		}

		tree := forest[i]

		// Find the center of the tree
		centerNode := findCenterOfTree(tree)

		// Set the staring point of the next tree
		currentStartingPoint.x = currentX
		currentStartingPoint.y = currentY

		// Do a radial layout starting with the center
		point = radialLayout(tree, centerNode, currentStartingPoint)

		if point.y > height {
			height = math.Floor(point.y)
		}

		currentX = math.Floor(point.x + defaultComponentSeperation)
	}

	l.transform(PointD{worldCenterX - float64(point.x/2), worldCenterY - float64(point.y/2)})
}

// radialLayout is CoSELayout.radialLayout: it lays the tree out on circles
// around the center node, moves it to the starting point, and returns the
// bottom right corner of the tree.
func radialLayout(tree []*Node, centerNode *Node, startingPoint PointD) PointD {
	radialSep := max(maxDiagonalInTree(tree), defaultRadialSeparation)
	branchRadialLayout(centerNode, nil, 0, 359, 0, radialSep)
	bounds := calculateBounds(tree)

	transform := newTransform()
	transform.setDeviceOrgX(bounds.getMinX())
	transform.setDeviceOrgY(bounds.getMinY())
	transform.setWorldOrgX(startingPoint.x)
	transform.setWorldOrgY(startingPoint.y)

	for i := 0; i < len(tree); i++ {
		node := tree[i]
		node.transform(transform)
	}

	bottomRight := PointD{bounds.getMaxX(), bounds.getMaxY()}

	return transform.inverseTransformPoint(bottomRight)
}

// branchRadialLayout is CoSELayout.branchRadialLayout.
func branchRadialLayout(node, parentOfNode *Node, startAngle, endAngle, distance, radialSeparation float64) {
	// First, position this node by finding its angle.
	halfInterval := float64((endAngle - startAngle + 1) / 2)

	if halfInterval < 0 {
		halfInterval += 180
	}

	nodeAngle := math.Mod(halfInterval+startAngle, 360)
	teta := nodeAngle * twoPi / 360

	// Make polar to java cordinate conversion.
	x := float64(distance * js.Cos(teta))
	y := float64(distance * js.Sin(teta))

	node.setCenter(x, y)

	// Traverse all neighbors of this node and recursively call this
	// function.
	neighborEdges := append([]*Edge(nil), node.getEdges()...)
	childCount := len(neighborEdges)

	if parentOfNode != nil {
		childCount--
	}

	branchCount := 0

	incEdgesCount := len(neighborEdges)
	var startIndex int

	edges := node.getEdgesBetween(parentOfNode)

	// If there are multiple edges, prune them until there remains only one
	// edge.
	for len(edges) > 1 {
		temp := edges[0]
		edges = edges[1:]
		index := -1
		for k, e := range neighborEdges {
			if e == temp {
				index = k
				break
			}
		}
		if index >= 0 {
			neighborEdges = append(neighborEdges[:index], neighborEdges[index+1:]...)
		}
		incEdgesCount--
		childCount--
	}

	if parentOfNode != nil {
		// assert edges.length == 1;
		index := -1
		for k, e := range neighborEdges {
			if e == edges[0] {
				index = k
				break
			}
		}
		startIndex = (index + 1) % incEdgesCount
	} else {
		startIndex = 0
	}

	stepAngle := math.Abs(endAngle-startAngle) / float64(childCount)

	for i := startIndex; branchCount != childCount; i = (i + 1) % incEdgesCount {
		currentNeighbor := neighborEdges[i].getOtherEnd(node)

		// Don't back traverse to root node in current tree.
		if currentNeighbor == parentOfNode {
			continue
		}

		childStartAngle := math.Mod(startAngle+float64(float64(branchCount)*stepAngle), 360)
		childEndAngle := math.Mod(childStartAngle+stepAngle, 360)

		branchRadialLayout(currentNeighbor, node, childStartAngle, childEndAngle, distance+radialSeparation, radialSeparation)

		branchCount++
	}
}

func maxDiagonalInTree(tree []*Node) float64 {
	maxDiagonal := float64(integerMinValue)

	for i := 0; i < len(tree); i++ {
		node := tree[i]
		diagonal := node.getDiagonal()

		if diagonal > maxDiagonal {
			maxDiagonal = diagonal
		}
	}

	return maxDiagonal
}

func (l *Layout) calcRepulsionRange() float64 {
	// formula is 2 x (level + 1) x idealEdgeLength
	return 2 * (l.level + 1) * l.idealEdgeLength
}

// ---- Tiling ----
//
// The tiling packs the nodes without edges, and the compound nodes whose
// members have no edges, outside of the force simulation. A flat graph has no
// such compound node. It has a group of nodes to tile when two or more nodes
// have no edge: that case is not ported.

func (l *Layout) tilingPreLayout() {
	if tile {
		// Find zero degree nodes and create a compound for each level
		l.groupZeroDegreeMembers()
		// Tile and clear children of each compound
		l.clearCompounds()
		// Separately tile and clear zero degree nodes for each level:
		// clearZeroDegreeMembers has no group to tile.
	}
}

func (l *Layout) tilingPostLayout() {
	// repopulateZeroDegreeMembers and repopulateCompounds have no group to
	// put back.
}

func (l *Layout) groupZeroDegreeMembers() {
	// List of zero degree nodes whose parents are not to be tiled
	zeroDegree := 0
	allNodes := l.graphManager.getAllNodes()

	// Fill zero degree list
	for _, node := range allNodes {
		parent := node.getParent()
		// If a node has zero degree and its parent is not to be tiled if exists add that node to zeroDegres list
		if l.getNodeDegreeWithChildren(node) == 0 && (!parent.hasID || !l.getToBeTiled(parent)) {
			zeroDegree++
		}
	}

	// If there are at least two nodes at a level, create a dummy compound for them
	if zeroDegree > 1 {
		bail.Out("cose-bilkent: two or more nodes without edges")
	}
}

func (l *Layout) clearCompounds() {
	// Get compound ordering by finding the inner one first
	for _, child := range l.graphManager.getRoot().getNodes() {
		if child.getChild() != nil || l.getToBeTiled(child) {
			bail.Out("cose-bilkent: a compound node")
		}
	}

	l.graphManager.resetAllNodes()
}

func (l *Layout) getToBeTiled(node *Node) bool {
	// only compound nodes are to be tiled
	childGraph := node.getChild()
	if childGraph == nil {
		return false
	}
	bail.Out("cose-bilkent: a compound node")
	return false
}

// getNodeDegree gets the degree of a node, without the edges from a node to
// itself.
func (l *Layout) getNodeDegree(node *Node) int {
	edges := node.getEdges()
	degree := 0

	// For the edges connected
	for _, edge := range edges {
		if edge.getSource().id != edge.getTarget().id {
			degree = degree + 1
		}
	}
	return degree
}

// getNodeDegreeWithChildren gets the degree of a node, with the degrees of
// the nodes inside it.
func (l *Layout) getNodeDegreeWithChildren(node *Node) int {
	degree := l.getNodeDegree(node)
	if node.getChild() == nil {
		return degree
	}
	children := node.getChild().getNodes()
	for _, child := range children {
		degree += l.getNodeDegreeWithChildren(child)
	}
	return degree
}

func boolFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
