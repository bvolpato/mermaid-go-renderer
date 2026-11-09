// Ported from cose-base 2.2.0 (MIT license): src/ConstraintHandler.js.

package fcose

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// adjacent is an entry of an adjacency list of the graph of the relative
// placement constraints: { id, gap, direction }.
type adjacent struct {
	id        string
	gap       float64
	direction string
}

// dagGraph is a Map of the JavaScript from a node id to its adjacency list.
// The order of the keys is the order of insertion.
type dagGraph = *js.Map[string, []adjacent]

// constraintCoords are the coordinates that handleConstraints works on.
type constraintCoords struct {
	nodeIndexes map[string]int
	xCoords     []float64
	yCoords     []float64
}

// index is nodeIndexes.get(id). The JavaScript reads xCoords[undefined] for
// an id that is not a node without children, which the port does not cover.
func (c *constraintCoords) index(id string) int {
	i, ok := c.nodeIndexes[id]
	if !ok {
		bail.Out("fcose: a constraint on a node that the layout does not have")
	}
	return i
}

// handleConstraints is ConstraintHandler.handleConstraints: it moves the
// nodes so that the first positions satisfy the constraints.
func handleConstraints(layout *Layout) {
	// get constraints from layout
	constraints := &layout.constraints

	c := &constraintCoords{nodeIndexes: map[string]int{}}

	allNodes := layout.getAllNodes()
	index := 0
	// fill index map and coordinates
	for _, node := range allNodes {
		if node.getChild() == nil {
			c.nodeIndexes[node.id] = index
			index++
			c.xCoords = append(c.xCoords, node.getCenterX())
			c.yCoords = append(c.yCoords, node.getCenterY())
		}
	}
	xCoords, yCoords := c.xCoords, c.yCoords

	// if there exists relative placement constraint without gap value, set
	// it to default
	for _, constraint := range constraints.Relative {
		if constraint.Gap != constraint.Gap {
			bail.Out("fcose: a relative placement without a gap")
		}
	}

	/* auxiliary functions */

	// calculate average position of the nodes. nodeIDSet is a Set of the
	// JavaScript: each id one time, in the order of insertion.
	calculateAvgPosition := func(nodeIDSet []string) (x, y float64) {
		xPosSum := 0.0
		yPosSum := 0.0
		for _, nodeID := range nodeIDSet {
			xPosSum += xCoords[c.index(nodeID)]
			yPosSum += yCoords[c.index(nodeID)]
		}

		return xPosSum / float64(len(nodeIDSet)), yPosSum / float64(len(nodeIDSet))
	}

	// find transformation based on rel. placement constraints if there are
	// both alignment and rel. placement constraints
	applyReflectionForRelativePlacement := func(relativePlacementConstraints []RelativePlacement) {
		// variables to count votes
		reflectOnY, notReflectOnY := 0, 0
		reflectOnX, notReflectOnX := 0, 0

		for _, constraint := range relativePlacementConstraints {
			if constraint.Left != "" {
				if xCoords[c.index(constraint.Left)]-xCoords[c.index(constraint.Right)] >= 0 {
					reflectOnY++
				} else {
					notReflectOnY++
				}
			} else {
				if yCoords[c.index(constraint.Top)]-yCoords[c.index(constraint.Bottom)] >= 0 {
					reflectOnX++
				} else {
					notReflectOnX++
				}
			}
		}

		if reflectOnY > notReflectOnY && reflectOnX > notReflectOnX {
			for i := 0; i < len(c.nodeIndexes); i++ {
				xCoords[i] = -1 * xCoords[i]
				yCoords[i] = -1 * yCoords[i]
			}
		} else if reflectOnY > notReflectOnY {
			for i := 0; i < len(c.nodeIndexes); i++ {
				xCoords[i] = -1 * xCoords[i]
			}
		} else if reflectOnX > notReflectOnX {
			for i := 0; i < len(c.nodeIndexes); i++ {
				yCoords[i] = -1 * yCoords[i]
			}
		}
	}

	/****  apply transformation to the initial draft layout to better align with constrained nodes ****/
	// solve the Orthogonal Procrustean Problem to rotate and/or reflect initial draft layout
	// here we follow the solution in Chapter 20.2 of Borg, I. & Groenen, P. (2005) Modern Multidimensional Scaling: Theory and Applications

	/* construct source and target configurations */

	var targetMatrix [][]float64 // A - target configuration
	var sourceMatrix [][]float64 // B - source configuration
	// false for no transformation, true for standart (Procrustes)
	// transformation (rotation and/or reflection)
	standardTransformation := false
	reflectionType := false
	// adjacency list to keep directed acyclic graph (dag) that consists of
	// relative placement constraints
	dag := js.NewMap[string, []adjacent]()

	// construct dag from relative placement constraints
	if constraints.Relative != nil {
		for _, constraint := range constraints.Relative {
			from, to, direction := constraint.Left, constraint.Right, "horizontal"
			if constraint.Left == "" {
				from, to, direction = constraint.Top, constraint.Bottom, "vertical"
			}
			if dag.Has(from) {
				dag.Set(from, append(dag.Get(from), adjacent{id: to, gap: constraint.Gap, direction: direction}))
			} else {
				dag.Set(from, []adjacent{{id: to, gap: constraint.Gap, direction: direction}})
			}
			if !dag.Has(to) {
				dag.Set(to, []adjacent{})
			}
		}
		// The JavaScript also computes the undirected graph and its
		// components here. Only the branch without an alignment constraint
		// reads them.
	}

	if layout.params.transformOnConstraintHandling {
		// first check fixed node constraint: the port has none. then check
		// alignment constraint
		if constraints.HasAlignment {
			count := 0
			if constraints.Vertical != nil {
				verticalAlign := constraints.Vertical

				for i := range verticalAlign {
					alignmentSet := uniqueStrings(verticalAlign[i])
					xPos, _ := calculateAvgPosition(alignmentSet)

					for _, nodeID := range verticalAlign[i] {
						targetMatrix = append(targetMatrix, []float64{xPos, yCoords[c.index(nodeID)]})
						sourceMatrix = append(sourceMatrix, []float64{xCoords[c.index(nodeID)], yCoords[c.index(nodeID)]})
						count++
					}
				}
				standardTransformation = true
			}
			if constraints.Horizontal != nil {
				horizontalAlign := constraints.Horizontal

				for i := range horizontalAlign {
					alignmentSet := uniqueStrings(horizontalAlign[i])
					_, yPos := calculateAvgPosition(alignmentSet)

					for _, nodeID := range horizontalAlign[i] {
						targetMatrix = append(targetMatrix, []float64{xCoords[c.index(nodeID)], yPos})
						sourceMatrix = append(sourceMatrix, []float64{xCoords[c.index(nodeID)], yCoords[c.index(nodeID)]})
						count++
					}
				}
				standardTransformation = true
			}
			if constraints.Relative != nil {
				reflectionType = true
			}
		} else if constraints.Relative != nil {
			bail.Out("fcose: a relative placement constraint without an alignment constraint")
		}

		// if transformation is required, then calculate and apply transformation matrix
		if standardTransformation {
			if len(targetMatrix) == 0 {
				// The JavaScript throws for an empty matrix.
				bail.Out("fcose: an alignment constraint without nodes")
			}
			/* calculate transformation matrix */
			targetMatrixTranspose := transpose(targetMatrix) // A'
			sourceMatrixTranspose := transpose(sourceMatrix) // B'

			// centralize transpose matrices
			for i := 0; i < len(targetMatrixTranspose); i++ {
				targetMatrixTranspose[i] = multGamma(targetMatrixTranspose[i])
				sourceMatrixTranspose[i] = multGamma(sourceMatrixTranspose[i])
			}

			// do actual calculation for transformation matrix
			tempMatrix := multMat(targetMatrixTranspose, transpose(sourceMatrixTranspose)) // tempMatrix = A'B
			svdResult := svd(tempMatrix)                                                   // SVD(A'B) = USV', svd function returns U, S and V
			transformationMatrix := multMat(svdResult.V, transpose(svdResult.U))           // transformationMatrix = T = VU'

			/* apply found transformation matrix to obtain final draft layout */
			for i := 0; i < len(c.nodeIndexes); i++ {
				temp1 := []float64{xCoords[i], yCoords[i]}
				temp2 := []float64{transformationMatrix[0][0], transformationMatrix[1][0]}
				temp3 := []float64{transformationMatrix[0][1], transformationMatrix[1][1]}
				xCoords[i] = dotProduct(temp1, temp2)
				yCoords[i] = dotProduct(temp1, temp3)
			}

			// applied only both alignment and rel. placement constraints exist
			if reflectionType {
				applyReflectionForRelativePlacement(constraints.Relative)
			}
		}
	}

	if enforceConstraints {
		/****  enforce constraints on the transformed draft layout ****/

		/* first enforce fixed node constraint: the port has none */

		/* then enforce alignment constraint */

		if constraints.HasAlignment {
			if constraints.Vertical != nil {
				xAlign := constraints.Vertical

				for i := range xAlign {
					alignmentSet := uniqueStrings(xAlign[i])
					xPos, _ := calculateAvgPosition(alignmentSet)

					for _, nodeID := range alignmentSet {
						xCoords[c.index(nodeID)] = xPos
					}
				}
			}
			if constraints.Horizontal != nil {
				yAlign := constraints.Horizontal

				for i := range yAlign {
					alignmentSet := uniqueStrings(yAlign[i])
					_, yPos := calculateAvgPosition(alignmentSet)

					for _, nodeID := range alignmentSet {
						yCoords[c.index(nodeID)] = yPos
					}
				}
			}
		}

		/* finally enforce relative placement constraint */

		if constraints.Relative != nil {
			nodeToDummyForVerticalAlignment := map[string]string{}
			nodeToDummyForHorizontalAlignment := map[string]string{}
			dummyToNodeForVerticalAlignment := map[string][]string{}
			dummyToNodeForHorizontalAlignment := map[string][]string{}
			dummyPositionsForVerticalAlignment := map[string]float64{}
			dummyPositionsForHorizontalAlignment := map[string]float64{}

			// fill maps and sets
			if constraints.HasAlignment {
				if constraints.Vertical != nil {
					verticalAlignment := constraints.Vertical

					for i := range verticalAlignment {
						dummy := dummyID(i)
						dummyToNodeForVerticalAlignment[dummy] = []string{}
						for _, nodeID := range verticalAlignment[i] {
							nodeToDummyForVerticalAlignment[nodeID] = dummy
							dummyToNodeForVerticalAlignment[dummy] = append(dummyToNodeForVerticalAlignment[dummy], nodeID)
						}
						dummyPositionsForVerticalAlignment[dummy] = xCoords[c.index(verticalAlignment[i][0])]
					}
				}
				if constraints.Horizontal != nil {
					horizontalAlignment := constraints.Horizontal

					for i := range horizontalAlignment {
						dummy := dummyID(i)
						dummyToNodeForHorizontalAlignment[dummy] = []string{}
						for _, nodeID := range horizontalAlignment[i] {
							nodeToDummyForHorizontalAlignment[nodeID] = dummy
							dummyToNodeForHorizontalAlignment[dummy] = append(dummyToNodeForHorizontalAlignment[dummy], nodeID)
						}
						dummyPositionsForHorizontalAlignment[dummy] = yCoords[c.index(horizontalAlignment[i][0])]
					}
				}
			}

			// construct horizontal and vertical dags (subgraphs) from overall dag
			dagOnHorizontal := js.NewMap[string, []adjacent]()
			dagOnVertical := js.NewMap[string, []adjacent]()

			for _, nodeID := range dag.Keys() {
				for _, adj := range dag.Get(nodeID) {
					var sub dagGraph
					var nodeToDummy map[string]string
					if adj.direction == "horizontal" {
						sub, nodeToDummy = dagOnHorizontal, nodeToDummyForVerticalAlignment
					} else {
						sub, nodeToDummy = dagOnVertical, nodeToDummyForHorizontalAlignment
					}
					// The JavaScript tests the dummy id for truth: an id is
					// a string that is not empty.
					sourceID := nodeID
					if d := nodeToDummy[nodeID]; d != "" {
						sourceID = d
					}
					targetNode := adj
					if d := nodeToDummy[adj.id]; d != "" {
						targetNode = adjacent{id: d, gap: adj.gap, direction: adj.direction}
					}
					if sub.Has(sourceID) {
						sub.Set(sourceID, append(sub.Get(sourceID), targetNode))
					} else {
						sub.Set(sourceID, []adjacent{targetNode})
					}
					if !sub.Has(targetNode.id) {
						sub.Set(targetNode.id, []adjacent{})
					}
				}
			}

			// find source nodes of each component in horizontal and vertical dags
			undirectedOnHorizontal := dagToUndirected(dagOnHorizontal)
			undirectedOnVertical := dagToUndirected(dagOnVertical)
			componentsOnHorizontal := findComponents(undirectedOnHorizontal)
			componentsOnVertical := findComponents(undirectedOnVertical)
			reversedDagOnHorizontal := dagToReversed(dagOnHorizontal)
			reversedDagOnVertical := dagToReversed(dagOnVertical)
			componentSourcesOnHorizontal := make([][]string, len(componentsOnHorizontal))
			componentSourcesOnVertical := make([][]string, len(componentsOnVertical))

			for index, component := range componentsOnHorizontal {
				for _, nodeID := range component {
					if len(reversedDagOnHorizontal.Get(nodeID)) == 0 {
						componentSourcesOnHorizontal[index] = append(componentSourcesOnHorizontal[index], nodeID)
					}
				}
			}

			for index, component := range componentsOnVertical {
				for _, nodeID := range component {
					if len(reversedDagOnVertical.Get(nodeID)) == 0 {
						componentSourcesOnVertical[index] = append(componentSourcesOnVertical[index], nodeID)
					}
				}
			}

			// calculate appropriate positioning for subgraphs
			positionMapHorizontal := c.findAppropriatePositionForRelativePlacement(dagOnHorizontal, "horizontal", dummyPositionsForVerticalAlignment, componentSourcesOnHorizontal)
			positionMapVertical := c.findAppropriatePositionForRelativePlacement(dagOnVertical, "vertical", dummyPositionsForHorizontalAlignment, componentSourcesOnVertical)

			// update positions of the nodes based on relative placement constraints

			for _, key := range positionMapHorizontal.Keys() {
				// The JavaScript tests the list for truth.
				if members, ok := dummyToNodeForVerticalAlignment[key]; ok {
					for _, nodeID := range members {
						xCoords[c.index(nodeID)] = positionMapHorizontal.Get(key)
					}
				} else {
					xCoords[c.index(key)] = positionMapHorizontal.Get(key)
				}
			}

			for _, key := range positionMapVertical.Keys() {
				if members, ok := dummyToNodeForHorizontalAlignment[key]; ok {
					for _, nodeID := range members {
						yCoords[c.index(nodeID)] = positionMapVertical.Get(key)
					}
				} else {
					yCoords[c.index(key)] = positionMapVertical.Get(key)
				}
			}
		}
	}

	// assign new coordinates to nodes after constraint handling
	for _, node := range allNodes {
		if node.getChild() == nil {
			node.setCenter(xCoords[c.nodeIndexes[node.id]], yCoords[c.nodeIndexes[node.id]])
		}
	}
}

// uniqueStrings is new Set(list): each string one time, in the order of the
// first appearance.
func uniqueStrings(list []string) []string {
	out := make([]string, 0, len(list))
	for _, s := range list {
		if !includes(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// position is the coordinate of a node of a dag before the enforcement: the
// coordinate of the node, or the position of a dummy node.
func (c *constraintCoords) position(nodeID, direction string, dummyPositions map[string]float64) float64 {
	if i, ok := c.nodeIndexes[nodeID]; ok {
		if direction == "horizontal" {
			return c.xCoords[i]
		}
		return c.yCoords[i]
	}
	p, ok := dummyPositions[nodeID]
	if !ok {
		// undefined in JavaScript.
		return nan
	}
	return p
}

// findAppropriatePositionForRelativePlacement finds a position for each node
// of the dag so that the relative placement constraints hold. It is the call
// of the enforcement phase: fixedNodes is a set, which is empty in the port.
func (c *constraintCoords) findAppropriatePositionForRelativePlacement(graph dagGraph, direction string, dummyPositions map[string]float64, componentSources [][]string) *js.Map[string, float64] {
	// find indegree count for each node
	inDegrees := js.NewMap[string, int]()

	for _, key := range graph.Keys() {
		inDegrees.Set(key, 0)
	}
	for _, key := range graph.Keys() {
		for _, adj := range graph.Get(key) {
			inDegrees.Set(adj.id, inDegrees.Get(adj.id)+1)
		}
	}

	positionMap := js.NewMap[string, float64]()     // keeps the position for each node
	pastMap := js.NewMap[string, *js.Set[string]]() // keeps the predecessors(past) of a node
	var queue []string
	for _, key := range inDegrees.Keys() {
		if inDegrees.Get(key) == 0 {
			queue = append(queue, key)
		} else {
			positionMap.Set(key, math.Inf(-1))
		}
		pastMap.Set(key, js.NewSet(key))
	}

	// align sources of each component in enforcement phase
	for _, component := range componentSources {
		position := 0.0
		for _, nodeID := range component {
			position += c.position(nodeID, direction, dummyPositions)
		}
		position = position / float64(len(component))
		for _, nodeID := range component {
			positionMap.Set(nodeID, position)
		}
	}

	// calculate positions of the nodes
	for head := 0; head < len(queue); head++ {
		currentNode := queue[head]
		neighbors := graph.Get(currentNode)
		for _, neighbor := range neighbors {
			if mapNumber(positionMap, neighbor.id) < mapNumber(positionMap, currentNode)+neighbor.gap {
				positionMap.Set(neighbor.id, mapNumber(positionMap, currentNode)+neighbor.gap)
			}
			inDegrees.Set(neighbor.id, inDegrees.Get(neighbor.id)-1)
			if inDegrees.Get(neighbor.id) == 0 {
				queue = append(queue, neighbor.id)
			}
			// setUnion(pastMap.get(currentNode), pastMap.get(neighbor.id))
			union := js.NewSet(pastMap.Get(currentNode).Values()...)
			for _, elem := range pastMap.Get(neighbor.id).Values() {
				union.Add(elem)
			}
			pastMap.Set(neighbor.id, union)
		}
	}

	// readjust position of the nodes after enforcement
	sinkNodes := map[string]bool{}

	for _, key := range graph.Keys() {
		if len(graph.Get(key)) == 0 {
			sinkNodes[key] = true
		}
	}

	var components []*js.Set[string]
	for _, key := range pastMap.Keys() {
		value := pastMap.Get(key)
		if sinkNodes[key] {
			// No component has a fixed node.
			isExist := false
			existAt := 0
			first := value.Values()[0]
			for index, component := range components {
				if component.Has(first) {
					isExist = true
					existAt = index
				}
			}
			if !isExist {
				components = append(components, js.NewSet(value.Values()...))
			} else {
				for _, ele := range value.Values() {
					components[existAt].Add(ele)
				}
			}
		}
	}

	for _, component := range components {
		minBefore := math.Inf(1)
		minAfter := math.Inf(1)
		maxBefore := math.Inf(-1)
		maxAfter := math.Inf(-1)

		for _, nodeID := range component.Values() {
			posBefore := c.position(nodeID, direction, dummyPositions)
			posAfter := mapNumber(positionMap, nodeID)
			if posBefore < minBefore {
				minBefore = posBefore
			}
			if posBefore > maxBefore {
				maxBefore = posBefore
			}
			if posAfter < minAfter {
				minAfter = posAfter
			}
			if posAfter > maxAfter {
				maxAfter = posAfter
			}
		}

		diff := float64((minBefore+maxBefore)/2) - float64((minAfter+maxAfter)/2)

		for _, nodeID := range component.Values() {
			positionMap.Set(nodeID, mapNumber(positionMap, nodeID)+diff)
		}
	}

	return positionMap
}

// mapNumber is map.get(key) in arithmetic: a key that the map does not have
// gives undefined, which is NaN.
func mapNumber(m *js.Map[string, float64], key string) float64 {
	v, ok := m.Lookup(key)
	if !ok {
		return nan
	}
	return v
}

// findComponents finds the weakly connected components of an undirected
// graph.
func findComponents(graph dagGraph) [][]string {
	var components [][]string
	visited := map[string]bool{}

	for _, key := range graph.Keys() {
		if !visited[key] {
			component := []string{key}
			queue := []string{key}
			visited[key] = true

			for head := 0; head < len(queue); head++ {
				currentNode := queue[head]
				neighbors := graph.Get(currentNode)
				for _, neighbor := range neighbors {
					if !visited[neighbor.id] {
						queue = append(queue, neighbor.id)
						visited[neighbor.id] = true
						component = append(component, neighbor.id)
					}
				}
			}
			components = append(components, component)
		}
	}
	return components
}

// dagToUndirected returns the undirected version of the dag.
func dagToUndirected(dag dagGraph) dagGraph {
	undirected := js.NewMap[string, []adjacent]()

	for _, key := range dag.Keys() {
		undirected.Set(key, []adjacent{})
	}

	for _, key := range dag.Keys() {
		for _, adj := range dag.Get(key) {
			undirected.Set(key, append(undirected.Get(key), adj))
			undirected.Set(adj.id, append(undirected.Get(adj.id), adjacent{id: key, gap: adj.gap, direction: adj.direction}))
		}
	}

	return undirected
}

// dagToReversed returns the dag with the directions inverted.
func dagToReversed(dag dagGraph) dagGraph {
	reversed := js.NewMap[string, []adjacent]()

	for _, key := range dag.Keys() {
		reversed.Set(key, []adjacent{})
	}

	for _, key := range dag.Keys() {
		for _, adj := range dag.Get(key) {
			reversed.Set(adj.id, append(reversed.Get(adj.id), adjacent{id: key, gap: adj.gap, direction: adj.direction}))
		}
	}

	return reversed
}
