// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/greedyswitch/BetweenLayerEdgeTwoNodeCrossingsCounter.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package greedyswitch

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p3order/counting"
)

// BetweenLayerEdgeTwoNodeCrossingsCounter is the class
// BetweenLayerEdgeTwoNodeCrossingsCounter: it calculates the number of
// crossings for edges incident to two nodes.
type BetweenLayerEdgeTwoNodeCrossingsCounter struct {
	upperLowerCrossings int
	lowerUpperCrossings int
	upperAdjacencies    *adjacencyList
	lowerAdjacencies    *adjacencyList
	currentNodeOrder    [][]*lgraph.LNode
	freeLayerIndex      int
	portPositions       map[*lgraph.LPort]int
	easternAdjacencies  map[*lgraph.LNode]*adjacencyList
	westernAdjacencies  map[*lgraph.LNode]*adjacencyList
}

// NewBetweenLayerEdgeTwoNodeCrossingsCounter is new
// BetweenLayerEdgeTwoNodeCrossingsCounter(currentNodeOrder, freeLayerIndex).
// The names assume a left-right layer ordering.
func NewBetweenLayerEdgeTwoNodeCrossingsCounter(currentNodeOrder [][]*lgraph.LNode, freeLayerIndex int) *BetweenLayerEdgeTwoNodeCrossingsCounter {
	c := &BetweenLayerEdgeTwoNodeCrossingsCounter{
		portPositions:      map[*lgraph.LPort]int{},
		easternAdjacencies: map[*lgraph.LNode]*adjacencyList{},
		westernAdjacencies: map[*lgraph.LNode]*adjacencyList{},
		currentNodeOrder:   currentNodeOrder,
		freeLayerIndex:     freeLayerIndex,
	}
	c.setPortPositionsForNeighbouringLayers()
	return c
}

func (c *BetweenLayerEdgeTwoNodeCrossingsCounter) setPortPositionsForNeighbouringLayers() {
	if c.freeLayerIsNotFirstLayer() {
		c.setPortPositionsForLayer(c.freeLayerIndex-1, core.PortSideEast)
	}
	if c.freeLayerIsNotLastLayer() {
		c.setPortPositionsForLayer(c.freeLayerIndex+1, core.PortSideWest)
	}
}

func (c *BetweenLayerEdgeTwoNodeCrossingsCounter) freeLayerIsNotFirstLayer() bool {
	return c.freeLayerIndex > 0
}

func (c *BetweenLayerEdgeTwoNodeCrossingsCounter) freeLayerIsNotLastLayer() bool {
	return c.freeLayerIndex < len(c.currentNodeOrder)-1
}

func (c *BetweenLayerEdgeTwoNodeCrossingsCounter) setPortPositionsForLayer(layerIndex int, portSide core.PortSide) {
	portID := 0
	for _, node := range c.currentNodeOrder[layerIndex] {
		ports := counting.InNorthSouthEastWestOrder(node, portSide)
		for _, port := range ports {
			c.portPositions[port] = portID
			portID++
		}
	}
}

// CountEasternEdgeCrossings is countEasternEdgeCrossings(upperNode,
// lowerNode): it calculates the number of crossings for the edges on the
// east side of the two nodes, for both orders of the nodes.
func (c *BetweenLayerEdgeTwoNodeCrossingsCounter) CountEasternEdgeCrossings(upperNode, lowerNode *lgraph.LNode) {
	c.resetCrossingCount()
	if upperNode == lowerNode {
		return
	}
	c.addEasternCrossings(upperNode, lowerNode)
}

// CountWesternEdgeCrossings is countWesternEdgeCrossings(upperNode,
// lowerNode): it calculates the number of crossings for the edges on the
// west side of the two nodes, for both orders of the nodes.
func (c *BetweenLayerEdgeTwoNodeCrossingsCounter) CountWesternEdgeCrossings(upperNode, lowerNode *lgraph.LNode) {
	c.resetCrossingCount()
	if upperNode == lowerNode {
		return
	}
	c.addWesternCrossings(upperNode, lowerNode)
}

// CountBothSideCrossings is countBothSideCrossings(upperNode, lowerNode): it
// calculates the number of crossings for the edges on both sides of the two
// nodes, for both orders of the nodes.
func (c *BetweenLayerEdgeTwoNodeCrossingsCounter) CountBothSideCrossings(upperNode, lowerNode *lgraph.LNode) {
	c.resetCrossingCount()
	if upperNode == lowerNode {
		return
	}
	c.addWesternCrossings(upperNode, lowerNode)
	c.addEasternCrossings(upperNode, lowerNode)
}

func (c *BetweenLayerEdgeTwoNodeCrossingsCounter) resetCrossingCount() {
	c.upperLowerCrossings = 0
	c.lowerUpperCrossings = 0
}

func (c *BetweenLayerEdgeTwoNodeCrossingsCounter) addEasternCrossings(upperNode, lowerNode *lgraph.LNode) {
	c.upperAdjacencies = c.getAdjacencyFor(upperNode, core.PortSideEast, c.easternAdjacencies)
	c.lowerAdjacencies = c.getAdjacencyFor(lowerNode, core.PortSideEast, c.easternAdjacencies)
	if c.upperAdjacencies.size() == 0 || c.lowerAdjacencies.size() == 0 {
		return
	}
	c.countCrossingsByMergingAdjacencyLists()
}

// getAdjacencyFor returns the adjacency list of a node. Since calculating
// adjacencies is a little expensive, it is only done once for each
// configuration and the sorted adjacencies are saved in maps.
func (c *BetweenLayerEdgeTwoNodeCrossingsCounter) getAdjacencyFor(node *lgraph.LNode, side core.PortSide,
	adjacencies map[*lgraph.LNode]*adjacencyList) *adjacencyList {
	if len(adjacencies) == 0 {
		for _, n := range c.currentNodeOrder[c.freeLayerIndex] {
			adjacencies[n] = c.newAdjacencyList(n, side)
		}
	}
	aL := adjacencies[node]
	aL.reset()
	return aL
}

func (c *BetweenLayerEdgeTwoNodeCrossingsCounter) addWesternCrossings(upperNode, lowerNode *lgraph.LNode) {
	c.upperAdjacencies = c.getAdjacencyFor(upperNode, core.PortSideWest, c.westernAdjacencies)
	c.lowerAdjacencies = c.getAdjacencyFor(lowerNode, core.PortSideWest, c.westernAdjacencies)
	if c.upperAdjacencies.size() == 0 || c.lowerAdjacencies.size() == 0 {
		return
	}
	c.countCrossingsByMergingAdjacencyLists()
}

// countCrossingsByMergingAdjacencyLists is the main algorithm. Adjacency
// lists are lists of ports connected to a node. By merging the adjacency
// lists of the two nodes, both the number of between-layer crossings for the
// order upper - lower and for the opposite order can be found.
func (c *BetweenLayerEdgeTwoNodeCrossingsCounter) countCrossingsByMergingAdjacencyLists() {
	for !c.upperAdjacencies.isEmpty() && !c.lowerAdjacencies.isEmpty() {
		if c.isBelow(c.upperAdjacencies.first(), c.lowerAdjacencies.first()) {
			c.upperLowerCrossings += c.upperAdjacencies.size()
			c.lowerAdjacencies.removeFirst()
		} else if c.isBelow(c.lowerAdjacencies.first(), c.upperAdjacencies.first()) {
			c.lowerUpperCrossings += c.lowerAdjacencies.size()
			c.upperAdjacencies.removeFirst()
		} else {
			c.upperLowerCrossings += c.upperAdjacencies.countAdjacenciesBelowNodeOfFirstPort()
			c.lowerUpperCrossings += c.lowerAdjacencies.countAdjacenciesBelowNodeOfFirstPort()
			c.upperAdjacencies.removeFirst()
			c.lowerAdjacencies.removeFirst()
		}
	}
}

func (c *BetweenLayerEdgeTwoNodeCrossingsCounter) isBelow(firstPort, secondPort int) bool {
	return firstPort > secondPort
}

// adjacencyList is the class AdjacencyList: the positions of the ports in a
// neighboring layer that a node is connected to on one side. A remove does
// not delete an entry: currentIndex, currentSize and currentCardinality show
// the current state of the list, and reset restores the first state.
type adjacencyList struct {
	counter       *BetweenLayerEdgeTwoNodeCrossingsCounter
	node          *lgraph.LNode
	adjacencyList []*adjacency
	side          core.PortSide
	sz            int
	currentSize   int
	currentIndex  int
}

func (c *BetweenLayerEdgeTwoNodeCrossingsCounter) newAdjacencyList(node *lgraph.LNode, side core.PortSide) *adjacencyList {
	a := &adjacencyList{counter: c, node: node, side: side}
	a.getAdjacenciesSortedByPosition()
	return a
}

func (a *adjacencyList) getAdjacenciesSortedByPosition() {
	a.iterateTroughEdgesCollectingAdjacencies()

	gwt.SortFunc(a.adjacencyList, (*adjacency).compareTo)
}

func (a *adjacencyList) iterateTroughEdgesCollectingAdjacencies() {
	ports := counting.InNorthSouthEastWestOrder(a.node, a.side)
	for _, port := range ports {
		edges := a.getEdgesConnectedTo(port)
		for _, edge := range edges {
			if !edge.IsSelfLoop() && a.isNotInLayer(edge) {
				a.addAdjacencyOf(edge)
				a.sz++
				a.currentSize++
			}
		}
	}
}

func (a *adjacencyList) getEdgesConnectedTo(port *lgraph.LPort) []*lgraph.LEdge {
	if a.side == core.PortSideWest {
		return port.IncomingEdges
	}
	return port.OutgoingEdges
}

func (a *adjacencyList) isNotInLayer(edge *lgraph.LEdge) bool {
	return edge.Source.Node().Layer() != edge.Target.Node().Layer()
}

func (a *adjacencyList) addAdjacencyOf(edge *lgraph.LEdge) {
	adjacentPort := a.adjacentPortOf(edge, a.side)
	adjacentPortPosition, ok := a.counter.portPositions[adjacentPort]
	if !ok {
		// The Java code unboxes null here.
		lgraph.Bail("greedyswitch: an adjacent port has no position")
	}
	lastIndex := len(a.adjacencyList) - 1
	if len(a.adjacencyList) != 0 && a.adjacencyList[lastIndex].position == adjacentPortPosition {
		a.adjacencyList[lastIndex].cardinality++
		a.adjacencyList[lastIndex].currentCardinality++
	} else {
		a.adjacencyList = append(a.adjacencyList, newAdjacency(adjacentPortPosition, adjacentPort))
	}
}

func (a *adjacencyList) adjacentPortOf(e *lgraph.LEdge, s core.PortSide) *lgraph.LPort {
	if s == core.PortSideWest {
		return e.Source
	}
	return e.Target
}

func (a *adjacencyList) reset() {
	a.currentIndex = 0
	a.currentSize = a.sz
	if !a.isEmpty() {
		a.currentAdjacency().reset()
	}
}

func (a *adjacencyList) countAdjacenciesBelowNodeOfFirstPort() int {
	return a.currentSize - a.currentAdjacency().currentCardinality
}

func (a *adjacencyList) removeFirst() {
	if a.isEmpty() {
		return
	}
	currentEntry := a.currentAdjacency()
	if currentEntry.currentCardinality == 1 {
		a.incrementCurrentIndex()
	} else {
		currentEntry.currentCardinality--
	}

	a.currentSize--
}

func (a *adjacencyList) incrementCurrentIndex() {
	a.currentIndex++
	// reset Adjacency for reuse
	if a.currentIndex < len(a.adjacencyList) {
		a.currentAdjacency().reset()
	}
}

func (a *adjacencyList) isEmpty() bool { return a.currentSize == 0 }

func (a *adjacencyList) first() int { return a.currentAdjacency().position }

func (a *adjacencyList) size() int { return a.currentSize }

func (a *adjacencyList) currentAdjacency() *adjacency { return a.adjacencyList[a.currentIndex] }

// adjacency is the class Adjacency: the position of a port, and the number
// of adjacencies with that position.
type adjacency struct {
	// position is the position of the port.
	position int
	// cardinality is the number of adjacencies with the same position.
	cardinality int
	// currentCardinality is the current number of adjacencies with the same
	// position.
	currentCardinality int
}

func newAdjacency(adjacentPortPosition int, port *lgraph.LPort) *adjacency {
	return &adjacency{position: adjacentPortPosition, cardinality: 1, currentCardinality: 1}
}

func (a *adjacency) reset() { a.currentCardinality = a.cardinality }

func (a *adjacency) compareTo(o *adjacency) int {
	if a.position < o.position {
		return -1
	} else if a.position == o.position {
		return 0
	}
	return 1
}

// UpperLowerCrossings is getUpperLowerCrossings().
func (c *BetweenLayerEdgeTwoNodeCrossingsCounter) UpperLowerCrossings() int {
	return c.upperLowerCrossings
}

// LowerUpperCrossings is getLowerUpperCrossings().
func (c *BetweenLayerEdgeTwoNodeCrossingsCounter) LowerUpperCrossings() int {
	return c.lowerUpperCrossings
}
