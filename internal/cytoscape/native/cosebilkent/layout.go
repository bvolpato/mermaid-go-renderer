// Ported from layout-base 1.0.2 (MIT license): src/Layout.js.
//
// The JavaScript has the classes Layout, FDLayout and CoSELayout, each one an
// extension of the one before. The extension creates a CoSELayout only, so
// Layout has the fields of the three. fdlayout.go and coselayout.go have the
// methods of the other two classes.

package cosebilkent

import "github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"

// Layout is a CoSELayout.
type Layout struct {
	// Layout.
	layoutQuality         int
	createBendsAsNeeded   bool
	incremental           bool
	animationOnLayout     bool
	animationDuringLayout bool
	animationPeriod       float64
	uniformLeafNodeSizes  bool
	graphManager          *GraphManager
	isLayoutFinished      bool
	isSubLayout           bool

	// FDLayout.
	useSmartIdealEdgeLengthCalculation bool
	idealEdgeLength                    float64
	springConstant                     float64
	repulsionConstant                  float64
	gravityConstant                    float64
	compoundGravityConstant            float64
	gravityRangeFactor                 float64
	compoundGravityRangeFactor         float64
	displacementThresholdPerNode       float64
	coolingFactor                      float64
	initialCoolingFactor               float64
	totalDisplacement                  float64
	oldTotalDisplacement               float64
	maxIterations                      float64
	totalIterations                    float64
	notAnimatedIterations              float64
	useFRGridVariant                   bool
	grid                               nodeGrid
	maxNodeDisplacement                float64
	totalDisplacementThreshold         float64
	repulsionRange                     float64

	// CoSELayout.
	prunedNodesAll         [][]*Node
	growTreeIterations     int
	afterGrowthIterations  int
	isTreeGrowing          bool
	isGrowthFinished       bool
	coolingCycle           float64
	maxCoolingCycle        float64
	finalTemperature       float64
	coolingAdjuster        float64
	level                  float64
	nodesWithGravity       []*Node
	initialAnimationPeriod float64

	// stamp numbers the sets of calcRepulsionForces. See Node.processed.
	stamp uint64

	// trace receives the state of the layout at the points that the oracle
	// records. nil for none.
	trace func(stage string, values []float64)
	// cancelled tells that the caller does not wait for the result.
	cancelled func() bool
}

// newLayout is new CoSELayout().
func newLayout() *Layout {
	l := &Layout{}
	// Layout.
	l.layoutQuality = quality
	l.createBendsAsNeeded = defaultCreateBendsAsNeeded
	l.incremental = defaultIncremental
	l.animationOnLayout = defaultAnimationOnLayout
	l.animationDuringLayout = defaultAnimationDuringLayout
	l.animationPeriod = defaultAnimationPeriod
	l.uniformLeafNodeSizes = defaultUniformLeafNodeSizes
	l.graphManager = newGraphManager(l)
	// FDLayout.
	l.useSmartIdealEdgeLengthCalculation = defaultUseSmartIdealEdgeLengthCalculation
	l.idealEdgeLength = defaultEdgeLength
	l.springConstant = defaultSpringStrength
	l.repulsionConstant = defaultRepulsionStrength
	l.gravityConstant = defaultGravityStrength
	l.compoundGravityConstant = defaultCompoundGravityStrength
	l.gravityRangeFactor = defaultGravityRangeFactor
	l.compoundGravityRangeFactor = defaultCompoundGravityRangeFactor
	l.displacementThresholdPerNode = 3.0 * defaultEdgeLength / 100
	l.coolingFactor = defaultCoolingFactorIncremental
	l.initialCoolingFactor = defaultCoolingFactorIncremental
	l.maxIterations = maxIterations
	return l
}

func (l *Layout) getGraphManager() *GraphManager { return l.graphManager }
func (l *Layout) getAllNodes() []*Node           { return l.graphManager.getAllNodes() }
func (l *Layout) getAllEdges() []*Edge           { return l.graphManager.getAllEdges() }

func (l *Layout) getAllNodesToApplyGravitation() []*Node {
	return l.graphManager.getAllNodesToApplyGravitation()
}

// checkLayoutSuccess is Layout.checkLayoutSuccess: true means that there is
// nothing to lay out.
func (l *Layout) checkLayoutSuccess() bool {
	return l.graphManager.getRoot() == nil || len(l.graphManager.getRoot().getNodes()) == 0 || l.graphManager.includesInvalidEdge()
}

func (l *Layout) runLayout() bool {
	l.isLayoutFinished = false

	l.tilingPreLayout()

	l.initParameters()
	var isLayoutSuccessfull bool

	if l.checkLayoutSuccess() {
		isLayoutSuccessfull = false
	} else {
		isLayoutSuccessfull = l.layout()
	}

	if layoutConstantsAnimateDuring {
		// If this is a 'during' layout animation. Layout is not finished yet.
		return false
	}

	if isLayoutSuccessfull {
		if !l.isSubLayout {
			l.doPostLayout()
		}
	}

	l.tilingPostLayout()

	l.isLayoutFinished = true

	return isLayoutSuccessfull
}

// doPostLayout propagates the geometric changes. Layout.update does nothing
// for the objects of the extension, which have no v-level object.
func (l *Layout) doPostLayout() {
	if !l.incremental {
		l.transform(PointD{0, 0})
	}
	if l.createBendsAsNeeded {
		bail.Out("cose-bilkent: bend points")
	}
}

// layoutInitParameters is Layout.initParameters.
func (l *Layout) layoutInitParameters() {
	if !l.isSubLayout {
		l.layoutQuality = quality
		l.animationDuringLayout = defaultAnimationDuringLayout
		l.animationPeriod = defaultAnimationPeriod
		l.animationOnLayout = defaultAnimationOnLayout
		l.incremental = defaultIncremental
		l.createBendsAsNeeded = defaultCreateBendsAsNeeded
		l.uniformLeafNodeSizes = defaultUniformLeafNodeSizes
	}

	if l.animationDuringLayout {
		l.animationOnLayout = false
	}
}

// transform is Layout.transform(newLeftTop): it moves the nodes so that the
// left top corner of the root graph is at newLeftTop.
func (l *Layout) transform(newLeftTop PointD) {
	// create a transformation object (from Eclipse to layout). When an
	// inverse transform is applied, we get upper-left coordinate of the
	// drawing or the root graph at given input coordinate (some margins
	// already included in calculation of left-top).

	trans := newTransform()
	leftTop, ok := l.graphManager.getRoot().updateLeftTop()

	if ok {
		trans.setWorldOrgX(newLeftTop.x)
		trans.setWorldOrgY(newLeftTop.y)

		trans.setDeviceOrgX(leftTop.x)
		trans.setDeviceOrgY(leftTop.y)

		nodes := l.getAllNodes()

		for _, node := range nodes {
			node.transform(trans)
		}
	}
}

// getFlatForest returns a list of trees, where each tree is a list of nodes.
// The list is empty when the graph is not flat, and when one of its
// components is not a tree.
func (l *Layout) getFlatForest() [][]*Node {
	var flatForest [][]*Node
	isForest := true

	// Quick reference for all nodes in the graph manager associated with
	// this layout. The list should not be changed.
	allNodes := l.graphManager.getRoot().getNodes()

	// First be sure that the graph is flat
	isFlat := true

	for i := 0; i < len(allNodes); i++ {
		if allNodes[i].getChild() != nil {
			isFlat = false
		}
	}

	// Return empty forest if the graph is not flat.
	if !isFlat {
		return flatForest
	}

	// Run BFS for each component of the graph.

	// visited is a Set of the JavaScript: the order of insertion is the
	// order of the nodes in a tree.
	visited := map[*Node]bool{}
	var visitedOrder []*Node
	var toBeVisited []*Node
	parents := map[*Node]*Node{}
	unProcessedNodes := append([]*Node(nil), allNodes...)

	// Each iteration of this loop finds a component of the graph and
	// decides whether it is a tree or not. If it is a tree, adds it to the
	// forest and continued with the next component.

	for len(unProcessedNodes) > 0 && isForest {
		toBeVisited = append(toBeVisited, unProcessedNodes[0])

		// Start the BFS. Each iteration of this loop visits a node in a
		// BFS manner.
		for len(toBeVisited) > 0 && isForest {
			// pool operation
			currentNode := toBeVisited[0]
			toBeVisited = toBeVisited[1:]
			if !visited[currentNode] {
				visited[currentNode] = true
				visitedOrder = append(visitedOrder, currentNode)
			}

			// Traverse all neighbors of this node
			neighborEdges := currentNode.getEdges()

			for i := 0; i < len(neighborEdges); i++ {
				currentNeighbor := neighborEdges[i].getOtherEnd(currentNode)

				// If BFS is not growing from this neighbor.
				if parents[currentNode] != currentNeighbor {
					// We haven't previously visited this neighbor.
					if !visited[currentNeighbor] {
						toBeVisited = append(toBeVisited, currentNeighbor)
						parents[currentNeighbor] = currentNode
					} else {
						// Since we have previously visited this neighbor and
						// this neighbor is not parent of currentNode, given
						// graph contains a component that is not tree, hence
						// it is not a forest.
						isForest = false
						break
					}
				}
			}
		}

		// The graph contains a component that is not a tree. Empty
		// previously found trees. The method will end.
		if !isForest {
			flatForest = nil
		} else {
			// Save currently visited nodes as a tree in our forest. Reset
			// visited and parents lists. Continue with the next component of
			// the graph, if any.
			temp := visitedOrder
			flatForest = append(flatForest, temp)
			for _, value := range temp {
				index := indexOfNode(unProcessedNodes, value)
				if index > -1 {
					unProcessedNodes = append(unProcessedNodes[:index], unProcessedNodes[index+1:]...)
				}
			}
			visited = map[*Node]bool{}
			visitedOrder = nil
			parents = map[*Node]*Node{}
		}
	}

	return flatForest
}

func indexOfNode(list []*Node, node *Node) int {
	for i, n := range list {
		if n == node {
			return i
		}
	}
	return -1
}

// findCenterOfTree is Layout.findCenterOfTree: it finds a center of the tree
// by removing the leaves again and again.
func findCenterOfTree(nodes []*Node) *Node {
	list := append([]*Node(nil), nodes...)

	var removedNodes []*Node
	remainingDegrees := map[*Node]int{}
	foundCenter := false
	var centerNode *Node

	if len(list) == 1 || len(list) == 2 {
		foundCenter = true
		centerNode = list[0]
	}

	for i := 0; i < len(list); i++ {
		node := list[i]
		degree := len(node.getNeighborsList())
		remainingDegrees[node] = degree

		if degree == 1 {
			removedNodes = append(removedNodes, node)
		}
	}

	tempList := append([]*Node(nil), removedNodes...)

	for !foundCenter {
		tempList = nil

		// The JavaScript removes the node at i from the list in each step,
		// so the loop visits every second node of the list.
		for i := 0; i < len(list); i++ {
			node := list[i]

			list = append(list[:i], list[i+1:]...)

			neighbours := node.getNeighborsList()

			for _, neighbour := range neighbours {
				if indexOfNode(removedNodes, neighbour) < 0 {
					otherDegree := remainingDegrees[neighbour]
					newDegree := otherDegree - 1

					if newDegree == 1 {
						tempList = append(tempList, neighbour)
					}

					remainingDegrees[neighbour] = newDegree
				}
			}
		}

		removedNodes = append(removedNodes, tempList...)

		if len(list) == 1 || len(list) == 2 {
			foundCenter = true
			centerNode = list[0]
		}
		if len(list) == 0 {
			// The JavaScript does not end for such a tree.
			bail.Out("cose-bilkent: a tree without a center")
		}
	}

	return centerNode
}
