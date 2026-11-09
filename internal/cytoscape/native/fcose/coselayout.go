// Ported from cose-base 2.2.0 (MIT license): src/CoSELayout.js.

package fcose

import (
	"math"
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// RelativePlacement is one relative placement constraint: either Left and
// Right are set, or Top and Bottom.
type RelativePlacement struct {
	Left, Right string
	Top, Bottom string
	Gap         float64
}

// Constraints are the constraints of the layout (layout.constraints). The
// port has no fixed node constraint.
type Constraints struct {
	// HasAlignment tells that the alignment constraint exists. Vertical and
	// Horizontal are its two lists. nil stands for undefined.
	HasAlignment bool
	Vertical     [][]string
	Horizontal   [][]string
	// Relative is the relative placement constraint. nil stands for
	// undefined.
	Relative []RelativePlacement
}

// count is Object.keys(this.constraints).length.
func (c *Constraints) count() int {
	n := 0
	if c.HasAlignment {
		n++
	}
	if c.Relative != nil {
		n++
	}
	return n
}

// relativeEntry is an entry of nodeToRelativeConstraintMap: { right: id,
// gap } or { left: id, gap } for the horizontal map, and bottom or top for
// the vertical one. after tells that the other node is the right or the
// bottom one.
type relativeEntry struct {
	other string
	after bool
	gap   float64
}

// constraintVariables are the variables of initConstraintVariables.
type constraintVariables struct {
	idToNodeMap map[string]*Node

	dummyToNodeForVerticalAlignment   map[string][]string
	dummyToNodeForHorizontalAlignment map[string][]string

	nodesInRelativeHorizontal             []string
	nodesInRelativeVertical               []string
	nodeToRelativeConstraintMapHorizontal map[string][]relativeEntry
	nodeToRelativeConstraintMapVertical   map[string][]relativeEntry
	nodeToTempPositionMapHorizontal       map[string]float64
	nodeToTempPositionMapVertical         map[string]float64
}

func (l *Layout) newGraph() *Graph { return newGraph(l.graphManager) }

// newNode is CoSELayout.newNode(vNode): a node without a rectangle.
func (l *Layout) newNode() *Node { return newNodeWithoutRect(l.graphManager) }

func (l *Layout) newEdge() *Edge { return newEdge() }

// newGraphManagerForRun is CoSELayout.newGraphManager.
func (l *Layout) newGraphManagerForRun() *GraphManager {
	gm := newGraphManager(l)
	l.graphManager = gm
	return gm
}

// initParameters is CoSELayout.initParameters.
func (l *Layout) initParameters() {
	l.fdInitParameters()
	if !l.isSubLayout {
		if l.params.defaultEdgeLength < 10 {
			l.idealEdgeLength = 10
		} else {
			l.idealEdgeLength = l.params.defaultEdgeLength
		}

		l.useSmartIdealEdgeLengthCalculation = defaultUseSmartIdealEdgeLengthCalculation
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
	}
}

// initSpringEmbedder sets the parameters of CoSE that the spring embedder
// uses.
func (l *Layout) initSpringEmbedder() {
	l.fdInitSpringEmbedder()

	// variables for cooling
	l.coolingCycle = 0
	l.maxCoolingCycle = l.maxIterations / convergenceCheckPeriod
	l.finalTemperature = 0.04
	l.coolingAdjuster = 1
}

// layout is CoSELayout.layout.
func (l *Layout) layout() bool {
	if defaultCreateBendsAsNeeded {
		bail.Out("fcose: bend points")
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
		bail.Out("fcose: a layout that is not incremental")
	} else if treeReductionOnIncremental {
		bail.Out("fcose: the reduction of trees")
	}

	if l.constraints.count() > 0 {
		handleConstraints(l)
		if l.trace != nil {
			l.trace("cose.constraints", l.traceRects())
		}
		l.initConstraintVariables()
	}

	l.initSpringEmbedder()
	if l.trace != nil {
		l.trace("cose.initial", l.tracePositions([]float64{l.coolingFactor, l.maxIterations, l.totalDisplacementThreshold, l.repulsionRange}))
	}
	if applyLayout {
		l.runSpringEmbedder()
	}

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
		bail.Out("fcose: the growth of pruned trees")
	}
	// Operations after growth is finished
	if l.isGrowthFinished {
		bail.Out("fcose: the growth of pruned trees")
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
		bail.Out("fcose: an animation during the layout")
	} else {
		// If aminate option is 'during' tick() function will be called on index.js
		for !layoutEnded {
			if l.cancelled != nil && math.Mod(l.totalIterations, 64) == 0 && l.cancelled() {
				bail.Out("fcose: the caller cancelled the layout")
			}
			layoutEnded = l.tick()
			if l.trace != nil && (l.traceIteration() || layoutEnded) {
				l.trace("cose.tick", l.tracePositions([]float64{l.totalIterations, l.coolingFactor, l.totalDisplacement, boolFloat(layoutEnded)}))
			}
		}

		l.graphManager.updateBounds()
	}
}

// moveNodes overrides moveNodes of FDLayout.
func (l *Layout) moveNodes() {
	lNodes := l.getAllNodes()

	// calculate displacement for each node
	for _, node := range lNodes {
		node.calculateDisplacement()
	}

	if l.constraints.count() > 0 {
		l.updateDisplacements()
	}

	// move each node
	for _, node := range lNodes {
		node.move()
	}
}

// constraint related methods: initConstraintVariables and updateDisplacements

// initConstraintVariables initializes the variables of the constraints.
func (l *Layout) initConstraintVariables() {
	cv := &l.cv
	cv.idToNodeMap = map[string]*Node{}

	allNodes := l.graphManager.getAllNodes()

	// fill idToNodeMap
	for _, node := range allNodes {
		cv.idToNodeMap[node.id] = node
	}

	if l.constraints.Relative != nil {
		nodeToDummyForVerticalAlignment := map[string]string{}
		nodeToDummyForHorizontalAlignment := map[string]string{}
		cv.dummyToNodeForVerticalAlignment = map[string][]string{}
		cv.dummyToNodeForHorizontalAlignment = map[string][]string{}

		if l.constraints.HasAlignment {
			if l.constraints.Vertical != nil {
				verticalAlignment := l.constraints.Vertical
				for i := range verticalAlignment {
					dummy := dummyID(i)
					cv.dummyToNodeForVerticalAlignment[dummy] = []string{}
					for _, nodeID := range verticalAlignment[i] {
						nodeToDummyForVerticalAlignment[nodeID] = dummy
						cv.dummyToNodeForVerticalAlignment[dummy] = append(cv.dummyToNodeForVerticalAlignment[dummy], nodeID)
					}
				}
			}
			if l.constraints.Horizontal != nil {
				horizontalAlignment := l.constraints.Horizontal
				for i := range horizontalAlignment {
					dummy := dummyID(i)
					cv.dummyToNodeForHorizontalAlignment[dummy] = []string{}
					for _, nodeID := range horizontalAlignment[i] {
						nodeToDummyForHorizontalAlignment[nodeID] = dummy
						cv.dummyToNodeForHorizontalAlignment[dummy] = append(cv.dummyToNodeForHorizontalAlignment[dummy], nodeID)
					}
				}
			}
		}

		if !relaxMovementOnConstraints {
			bail.Out("fcose: constraints without relaxed movement")
		}

		cv.nodesInRelativeHorizontal = nil
		cv.nodesInRelativeVertical = nil
		cv.nodeToRelativeConstraintMapHorizontal = map[string][]relativeEntry{}
		cv.nodeToRelativeConstraintMapVertical = map[string][]relativeEntry{}
		cv.nodeToTempPositionMapHorizontal = map[string]float64{}
		cv.nodeToTempPositionMapVertical = map[string]float64{}

		// fill arrays and maps
		for _, constraint := range l.constraints.Relative {
			if constraint.Left != "" {
				nodeIDLeft := constraint.Left
				if d, ok := nodeToDummyForVerticalAlignment[constraint.Left]; ok {
					nodeIDLeft = d
				}
				nodeIDRight := constraint.Right
				if d, ok := nodeToDummyForVerticalAlignment[constraint.Right]; ok {
					nodeIDRight = d
				}

				for _, nodeID := range []string{nodeIDLeft, nodeIDRight} {
					if !includes(cv.nodesInRelativeHorizontal, nodeID) {
						cv.nodesInRelativeHorizontal = append(cv.nodesInRelativeHorizontal, nodeID)
						cv.nodeToRelativeConstraintMapHorizontal[nodeID] = []relativeEntry{}
						if members, ok := cv.dummyToNodeForVerticalAlignment[nodeID]; ok {
							cv.nodeToTempPositionMapHorizontal[nodeID] = cv.node(members[0]).getCenterX()
						} else {
							cv.nodeToTempPositionMapHorizontal[nodeID] = cv.node(nodeID).getCenterX()
						}
					}
				}

				cv.nodeToRelativeConstraintMapHorizontal[nodeIDLeft] = append(cv.nodeToRelativeConstraintMapHorizontal[nodeIDLeft], relativeEntry{other: nodeIDRight, after: true, gap: constraint.Gap})
				cv.nodeToRelativeConstraintMapHorizontal[nodeIDRight] = append(cv.nodeToRelativeConstraintMapHorizontal[nodeIDRight], relativeEntry{other: nodeIDLeft, after: false, gap: constraint.Gap})
			} else {
				nodeIDTop := constraint.Top
				if d, ok := nodeToDummyForHorizontalAlignment[constraint.Top]; ok {
					nodeIDTop = d
				}
				nodeIDBottom := constraint.Bottom
				if d, ok := nodeToDummyForHorizontalAlignment[constraint.Bottom]; ok {
					nodeIDBottom = d
				}

				for _, nodeID := range []string{nodeIDTop, nodeIDBottom} {
					if !includes(cv.nodesInRelativeVertical, nodeID) {
						cv.nodesInRelativeVertical = append(cv.nodesInRelativeVertical, nodeID)
						cv.nodeToRelativeConstraintMapVertical[nodeID] = []relativeEntry{}
						if members, ok := cv.dummyToNodeForHorizontalAlignment[nodeID]; ok {
							cv.nodeToTempPositionMapVertical[nodeID] = cv.node(members[0]).getCenterY()
						} else {
							cv.nodeToTempPositionMapVertical[nodeID] = cv.node(nodeID).getCenterY()
						}
					}
				}
				cv.nodeToRelativeConstraintMapVertical[nodeIDTop] = append(cv.nodeToRelativeConstraintMapVertical[nodeIDTop], relativeEntry{other: nodeIDBottom, after: true, gap: constraint.Gap})
				cv.nodeToRelativeConstraintMapVertical[nodeIDBottom] = append(cv.nodeToRelativeConstraintMapVertical[nodeIDBottom], relativeEntry{other: nodeIDTop, after: false, gap: constraint.Gap})
			}
		}
	}
}

// node is idToNodeMap.get(id). The JavaScript throws a TypeError for an id
// that is not in the map, when it reads a member of the result.
func (cv *constraintVariables) node(id string) *Node {
	n := cv.idToNodeMap[id]
	if n == nil {
		bail.Out("fcose: a constraint on a node that the layout does not have")
	}
	return n
}

func dummyID(i int) string { return "dummy" + strconv.Itoa(i) }

func includes(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// shuffle is the function shuffle of initConstraintVariables: it moves the
// last third of the array to random places.
func (l *Layout) shuffle(array []string) {
	n := float64(len(array))
	for i := len(array) - 1; float64(i) >= 2*n/3; i-- {
		j := int(math.Floor(l.random() * float64(i+1)))
		array[i], array[j] = array[j], array[i]
	}
}

// updateDisplacements updates the displacements of the nodes for the
// constraints.
func (l *Layout) updateDisplacements() {
	cv := &l.cv

	if l.constraints.HasAlignment {
		if l.constraints.Vertical != nil {
			allVerticalAlignments := l.constraints.Vertical
			for i := range allVerticalAlignments {
				totalDisplacementX := 0.0
				for j := range allVerticalAlignments[i] {
					totalDisplacementX += cv.node(allVerticalAlignments[i][j]).displacementX
				}
				averageDisplacementX := totalDisplacementX / float64(len(allVerticalAlignments[i]))
				for j := range allVerticalAlignments[i] {
					cv.node(allVerticalAlignments[i][j]).displacementX = averageDisplacementX
				}
			}
		}
		if l.constraints.Horizontal != nil {
			allHorizontalAlignments := l.constraints.Horizontal
			for i := range allHorizontalAlignments {
				totalDisplacementY := 0.0
				for j := range allHorizontalAlignments[i] {
					totalDisplacementY += cv.node(allHorizontalAlignments[i][j]).displacementY
				}
				averageDisplacementY := totalDisplacementY / float64(len(allHorizontalAlignments[i]))
				for j := range allHorizontalAlignments[i] {
					cv.node(allHorizontalAlignments[i][j]).displacementY = averageDisplacementY
				}
			}
		}
	}

	if l.constraints.Relative != nil {
		// shuffle array to randomize node processing order
		if math.Mod(l.totalIterations, 10) == 0 {
			l.shuffle(cv.nodesInRelativeHorizontal)
			l.shuffle(cv.nodesInRelativeVertical)
		}

		for _, nodeID := range cv.nodesInRelativeHorizontal {
			displacement := 0.0
			members, isDummy := cv.dummyToNodeForVerticalAlignment[nodeID]
			if isDummy {
				displacement = cv.node(members[0]).displacementX
			} else {
				displacement = cv.node(nodeID).displacementX
			}
			for _, constraint := range cv.nodeToRelativeConstraintMapHorizontal[nodeID] {
				if constraint.after {
					diff := cv.nodeToTempPositionMapHorizontal[constraint.other] - cv.nodeToTempPositionMapHorizontal[nodeID] - displacement
					if diff < constraint.gap {
						displacement -= constraint.gap - diff
					}
				} else {
					diff := cv.nodeToTempPositionMapHorizontal[nodeID] - cv.nodeToTempPositionMapHorizontal[constraint.other] + displacement
					if diff < constraint.gap {
						displacement += constraint.gap - diff
					}
				}
			}
			cv.nodeToTempPositionMapHorizontal[nodeID] = cv.nodeToTempPositionMapHorizontal[nodeID] + displacement
			if isDummy {
				for _, member := range members {
					cv.node(member).displacementX = displacement
				}
			} else {
				cv.node(nodeID).displacementX = displacement
			}
		}

		for _, nodeID := range cv.nodesInRelativeVertical {
			displacement := 0.0
			members, isDummy := cv.dummyToNodeForHorizontalAlignment[nodeID]
			if isDummy {
				displacement = cv.node(members[0]).displacementY
			} else {
				displacement = cv.node(nodeID).displacementY
			}
			for _, constraint := range cv.nodeToRelativeConstraintMapVertical[nodeID] {
				if constraint.after {
					diff := cv.nodeToTempPositionMapVertical[constraint.other] - cv.nodeToTempPositionMapVertical[nodeID] - displacement
					if diff < constraint.gap {
						displacement -= constraint.gap - diff
					}
				} else {
					diff := cv.nodeToTempPositionMapVertical[nodeID] - cv.nodeToTempPositionMapVertical[constraint.other] + displacement
					if diff < constraint.gap {
						displacement += constraint.gap - diff
					}
				}
			}
			cv.nodeToTempPositionMapVertical[nodeID] = cv.nodeToTempPositionMapVertical[nodeID] + displacement
			if isDummy {
				for _, member := range members {
					cv.node(member).displacementY = displacement
				}
			} else {
				cv.node(nodeID).displacementY = displacement
			}
		}
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

func (l *Layout) calcRepulsionRange() float64 {
	// formula is 2 x (level + 1) x idealEdgeLength
	return 2 * (l.level + 1) * l.idealEdgeLength
}

// tilingPreLayout and tilingPostLayout do nothing: the tiling is off when a
// constraint exists.
func (l *Layout) tilingPreLayout() {
	if tile {
		bail.Out("fcose: the tiling of nodes")
	}
}

func (l *Layout) tilingPostLayout() {
	if tile {
		bail.Out("fcose: the tiling of nodes")
	}
}

func boolFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
