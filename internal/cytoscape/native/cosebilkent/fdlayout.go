// Ported from layout-base 1.0.2 (MIT license): src/fd/FDLayout.js.

package cosebilkent

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"
)

// fdInitParameters is FDLayout.initParameters.
func (l *Layout) fdInitParameters() {
	l.layoutInitParameters()

	l.totalIterations = 0
	l.notAnimatedIterations = 0

	l.useFRGridVariant = defaultUseSmartRepulsionRangeCalculation

	l.grid.reset(0, 0)
}

func (l *Layout) calcIdealEdgeLengths() {
	allEdges := l.getGraphManager().getAllEdges()
	for _, edge := range allEdges {
		edge.idealLength = l.idealEdgeLength

		if edge.isInterGraph {
			bail.Out("cose-bilkent: an edge between two graphs")
		}
	}
}

func (l *Layout) initSpringEmbedder() {
	s := float64(len(l.getAllNodes()))
	// A variable, so that 1 - 0.33 is the float64 difference of JavaScript
	// and not the exact constant 0.67.
	one := 1.0
	if l.incremental {
		if s > adaptationLowerNodeLimit {
			l.coolingFactor = max(float64(l.coolingFactor*coolingAdaptationFactor), l.coolingFactor-float64((s-adaptationLowerNodeLimit)/(adaptationUpperNodeLimit-adaptationLowerNodeLimit)*l.coolingFactor*(one-coolingAdaptationFactor)))
		}
		l.maxNodeDisplacement = maxNodeDisplacementIncremental
	} else {
		if s > adaptationLowerNodeLimit {
			l.coolingFactor = max(coolingAdaptationFactor, 1.0-float64((s-adaptationLowerNodeLimit)/(adaptationUpperNodeLimit-adaptationLowerNodeLimit)*(one-coolingAdaptationFactor)))
		} else {
			l.coolingFactor = 1.0
		}
		l.initialCoolingFactor = l.coolingFactor
		l.maxNodeDisplacement = maxNodeDisplacement
	}

	l.maxIterations = max(float64(len(l.getAllNodes()))*5, l.maxIterations)

	l.totalDisplacementThreshold = l.displacementThresholdPerNode * float64(len(l.getAllNodes()))

	l.repulsionRange = l.calcRepulsionRange()
}

func (l *Layout) calcSpringForces() {
	lEdges := l.getAllEdges()

	for _, edge := range lEdges {
		l.calcSpringForce(edge, edge.idealLength)
	}
}

func (l *Layout) calcRepulsionForces(gridUpdateAllowed, forceToNodeSurroundingUpdate bool) {
	lNodes := l.getAllNodes()

	if l.useFRGridVariant {
		if math.Mod(l.totalIterations, gridCalculationCheckPeriod) == 1 && gridUpdateAllowed {
			l.updateGrid()
		}

		// processedNodeSet is a new Set: a node is in it when its field
		// processed has the stamp of this call.
		l.stamp++
		processedNodeSet := l.stamp

		// calculate repulsion forces between each nodes and its surrounding
		for _, nodeA := range lNodes {
			l.calculateRepulsionForceOfANode(nodeA, processedNodeSet, gridUpdateAllowed, forceToNodeSurroundingUpdate)
			nodeA.processed = processedNodeSet
		}
	} else {
		bail.Out("cose-bilkent: repulsion without the grid")
	}
}

func (l *Layout) calcGravitationalForces() {
	lNodes := l.getAllNodesToApplyGravitation()

	for _, node := range lNodes {
		l.calcGravitationalForce(node)
	}
}

func (l *Layout) moveNodes() {
	lNodes := l.getAllNodes()

	for _, node := range lNodes {
		node.move()
	}
}

func (l *Layout) calcSpringForce(edge *Edge, idealLength float64) {
	sourceNode := edge.getSource()
	targetNode := edge.getTarget()

	// Update edge length
	if l.uniformLeafNodeSizes && sourceNode.getChild() == nil && targetNode.getChild() == nil {
		bail.Out("cose-bilkent: uniform leaf node sizes")
	} else {
		edge.updateLength()

		if edge.isOverlapingSourceAndTarget {
			return
		}
	}

	length := edge.getLength()

	if length == 0 {
		return
	}

	// Calculate spring forces
	springForce := l.springConstant * (length - idealLength)

	// Project force onto x and y axes
	springForceX := float64(springForce * (edge.lengthX / length))
	springForceY := float64(springForce * (edge.lengthY / length))

	// Apply forces on the end nodes
	sourceNode.springForceX += springForceX
	sourceNode.springForceY += springForceY
	targetNode.springForceX -= springForceX
	targetNode.springForceY -= springForceY
}

func (l *Layout) calcRepulsionForce(nodeA, nodeB *Node) {
	rectA := nodeA.getRect()
	rectB := nodeB.getRect()
	var overlapAmount [2]float64
	clipPoints := [4]float64{nan, nan, nan, nan}
	var distanceX, distanceY float64

	if rectA.intersects(rectB) {
		// two nodes overlap

		// calculate separation amount in x and y directions
		calcSeparationAmount(rectA, rectB, &overlapAmount, defaultEdgeLength/2.0)

		repulsionForceX := 2 * overlapAmount[0]
		repulsionForceY := 2 * overlapAmount[1]

		childrenConstant := nodeA.noOfChildren * nodeB.noOfChildren / (nodeA.noOfChildren + nodeB.noOfChildren)

		// Apply forces on the two nodes
		nodeA.repulsionForceX -= float64(childrenConstant * repulsionForceX)
		nodeA.repulsionForceY -= float64(childrenConstant * repulsionForceY)
		nodeB.repulsionForceX += float64(childrenConstant * repulsionForceX)
		nodeB.repulsionForceY += float64(childrenConstant * repulsionForceY)
	} else {
		// no overlap

		// calculate distance

		if l.uniformLeafNodeSizes && nodeA.getChild() == nil && nodeB.getChild() == nil {
			// simply base repulsion on distance of node centers
			distanceX = rectB.getCenterX() - rectA.getCenterX()
			distanceY = rectB.getCenterY() - rectA.getCenterY()
		} else {
			// use clipping points
			getIntersection2(rectA, rectB, &clipPoints)

			distanceX = clipPoints[2] - clipPoints[0]
			distanceY = clipPoints[3] - clipPoints[1]
		}

		// No repulsion range. FR grid variant should take care of this.
		if math.Abs(distanceX) < minRepulsionDist {
			distanceX = iMathSign(distanceX) * minRepulsionDist
		}

		if math.Abs(distanceY) < minRepulsionDist {
			distanceY = iMathSign(distanceY) * minRepulsionDist
		}

		distanceSquared := float64(distanceX*distanceX) + float64(distanceY*distanceY)
		distance := math.Sqrt(distanceSquared)

		repulsionForce := l.repulsionConstant * nodeA.noOfChildren * nodeB.noOfChildren / distanceSquared

		// Project force onto x and y axes
		repulsionForceX := repulsionForce * distanceX / distance
		repulsionForceY := repulsionForce * distanceY / distance

		// Apply forces on the two nodes
		nodeA.repulsionForceX -= repulsionForceX
		nodeA.repulsionForceY -= repulsionForceY
		nodeB.repulsionForceX += repulsionForceX
		nodeB.repulsionForceY += repulsionForceY
	}
}

func (l *Layout) calcGravitationalForce(node *Node) {
	ownerGraph := node.getOwner()

	ownerCenterX := float64((ownerGraph.getRight() + ownerGraph.getLeft()) / 2)
	ownerCenterY := float64((ownerGraph.getTop() + ownerGraph.getBottom()) / 2)
	distanceX := node.getCenterX() - ownerCenterX
	distanceY := node.getCenterY() - ownerCenterY
	absDistanceX := math.Abs(distanceX) + float64(node.getWidth()/2)
	absDistanceY := math.Abs(distanceY) + float64(node.getHeight()/2)

	if node.getOwner() == l.graphManager.getRoot() {
		// in the root graph
		estimatedSize := ownerGraph.getEstimatedSize() * l.gravityRangeFactor

		if absDistanceX > estimatedSize || absDistanceY > estimatedSize {
			node.gravitationForceX = float64(-l.gravityConstant * distanceX)
			node.gravitationForceY = float64(-l.gravityConstant * distanceY)
		}
	} else {
		// inside a compound
		estimatedSize := ownerGraph.getEstimatedSize() * l.compoundGravityRangeFactor

		if absDistanceX > estimatedSize || absDistanceY > estimatedSize {
			node.gravitationForceX = float64(-l.gravityConstant * distanceX * l.compoundGravityConstant)
			node.gravitationForceY = float64(-l.gravityConstant * distanceY * l.compoundGravityConstant)
		}
	}
}

func (l *Layout) isConverged() bool {
	oscilating := false

	if l.totalIterations > l.maxIterations/3 {
		oscilating = math.Abs(l.totalDisplacement-l.oldTotalDisplacement) < 2
	}

	converged := l.totalDisplacement < l.totalDisplacementThreshold

	l.oldTotalDisplacement = l.totalDisplacement

	return converged || oscilating
}

// animate is FDLayout.animate. The layout of mermaid has no animation.
func (l *Layout) animate() {
	if l.animationDuringLayout && !l.isSubLayout {
		bail.Out("cose-bilkent: an animation during the layout")
	}
}

func (l *Layout) calcNoOfChildrenForAllNodes() {
	allNodes := l.graphManager.getAllNodes()

	for _, node := range allNodes {
		node.noOfChildren = node.getNoOfChildren()
	}
}

// maxGridCells bounds the grid of calcGrid. A larger grid stands for
// positions that the JavaScript handles with errors or with a very long run.
const maxGridCells = 1 << 22

// gridSize is parseInt(Math.ceil(x)) for a size of the grid. The JavaScript
// throws a RangeError for a size that is not an array length.
func gridSize(x float64) int {
	x = math.Ceil(x)
	if !(x >= 0 && x <= maxGridCells) {
		bail.Out("cose-bilkent: a grid of the size %v", x)
	}
	return int(x)
}

// gridIndex is parseInt(Math.floor(x)) for a coordinate of the grid.
func gridIndex(x float64) int {
	x = math.Floor(x)
	if !(x >= -maxGridCells && x <= maxGridCells) {
		bail.Out("cose-bilkent: a grid coordinate %v", x)
	}
	return int(x)
}

// nodeGrid is this.grid: for each cell of a grid over the root graph, the
// nodes that touch the cell. The JavaScript makes a new array of arrays for
// each update. The port keeps one list of cells, column by column, and
// empties the cells that the last update filled.
type nodeGrid struct {
	sizeX, sizeY int
	cells        [][]*Node
	touched      []int
}

// reset empties the grid and gives it a new size.
func (g *nodeGrid) reset(sizeX, sizeY int) {
	for _, k := range g.touched {
		g.cells[k] = g.cells[k][:0]
	}
	g.touched = g.touched[:0]
	g.sizeX, g.sizeY = sizeX, sizeY
	if n := sizeX * sizeY; n > len(g.cells) {
		// The cells keep their lists, which are empty now, for the next
		// update.
		cells := make([][]*Node, max(n, 2*len(g.cells)))
		copy(cells, g.cells)
		g.cells = cells
	}
}

// at is grid[i][j]. The JavaScript throws for a cell outside of the grid.
func (g *nodeGrid) at(i, j int) []*Node {
	if i < 0 || i >= g.sizeX || j < 0 || j >= g.sizeY {
		bail.Out("cose-bilkent: a grid cell outside of the grid")
	}
	return g.cells[i*g.sizeY+j]
}

// push is grid[i][j].push(v).
func (g *nodeGrid) push(i, j int, v *Node) {
	if i < 0 || i >= g.sizeX || j < 0 || j >= g.sizeY {
		bail.Out("cose-bilkent: a grid cell outside of the grid")
	}
	k := i*g.sizeY + j
	if len(g.cells[k]) == 0 {
		g.touched = append(g.touched, k)
	}
	g.cells[k] = append(g.cells[k], v)
}

// calcGrid is FDLayout.calcGrid: it sizes the grid for the bounds of the
// graph.
func (l *Layout) calcGrid(graph *Graph) {
	sizeX := gridSize((graph.getRight() - graph.getLeft()) / l.repulsionRange)
	sizeY := gridSize((graph.getBottom() - graph.getTop()) / l.repulsionRange)
	if sizeX*sizeY > maxGridCells {
		bail.Out("cose-bilkent: a grid of %d by %d cells", sizeX, sizeY)
	}

	l.grid.reset(sizeX, sizeY)
}

func (l *Layout) addNodeToGrid(v *Node, left, top float64) {
	startX := gridIndex((v.getRect().x - left) / l.repulsionRange)
	finishX := gridIndex((v.getRect().width + v.getRect().x - left) / l.repulsionRange)
	startY := gridIndex((v.getRect().y - top) / l.repulsionRange)
	finishY := gridIndex((v.getRect().height + v.getRect().y - top) / l.repulsionRange)

	for i := startX; i <= finishX; i++ {
		for j := startY; j <= finishY; j++ {
			l.grid.push(i, j, v)
			v.setGridCoordinates(startX, finishX, startY, finishY)
		}
	}
}

func (l *Layout) updateGrid() {
	lNodes := l.getAllNodes()

	l.calcGrid(l.graphManager.getRoot())

	// put all nodes to proper grid cells
	for _, nodeA := range lNodes {
		l.addNodeToGrid(nodeA, l.graphManager.getRoot().getLeft(), l.graphManager.getRoot().getTop())
	}
}

func (l *Layout) calculateRepulsionForceOfANode(nodeA *Node, processedNodeSet uint64, gridUpdateAllowed, forceToNodeSurroundingUpdate bool) {
	if math.Mod(l.totalIterations, gridCalculationCheckPeriod) == 1 && gridUpdateAllowed || forceToNodeSurroundingUpdate {
		// surrounding is a new Set, which keeps the order of insertion: the
		// list is that order, and the stamp is the membership.
		l.stamp++
		surrounding := l.stamp
		nodeA.surrounding = nodeA.surrounding[:0]
		grid := &l.grid

		for i := nodeA.startX - 1; i < nodeA.finishX+2; i++ {
			for j := nodeA.startY - 1; j < nodeA.finishY+2; j++ {
				if !(i < 0 || j < 0 || i >= grid.sizeX || j >= grid.sizeY) {
					cell := grid.at(i, j)
					for k := 0; k < len(cell); k++ {
						nodeB := cell[k]

						// If both nodes are not members of the same graph,
						// or both nodes are the same, skip.
						if nodeA.getOwner() != nodeB.getOwner() || nodeA == nodeB {
							continue
						}

						// check if the repulsion force between
						// nodeA and nodeB has already been calculated
						if nodeB.processed != processedNodeSet && nodeB.inSurrounding != surrounding {
							distanceX := math.Abs(nodeA.getCenterX()-nodeB.getCenterX()) - (float64(nodeA.getWidth()/2) + float64(nodeB.getWidth()/2))
							distanceY := math.Abs(nodeA.getCenterY()-nodeB.getCenterY()) - (float64(nodeA.getHeight()/2) + float64(nodeB.getHeight()/2))

							// if the distance between nodeA and nodeB
							// is less then calculation range
							if distanceX <= l.repulsionRange && distanceY <= l.repulsionRange {
								// then add nodeB to surrounding of nodeA
								nodeB.inSurrounding = surrounding
								nodeA.surrounding = append(nodeA.surrounding, nodeB)
							}
						}
					}
				}
			}
		}
	}
	for i := 0; i < len(nodeA.surrounding); i++ {
		l.calcRepulsionForce(nodeA, nodeA.surrounding[i])
	}
}
