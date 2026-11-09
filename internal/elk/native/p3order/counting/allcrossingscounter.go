// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/counting/AllCrossingsCounter.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package counting

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
)

// AllCrossingsCounter is the class AllCrossingsCounter: it counts all
// crossings in a graph. It must be initialized with Init.
type AllCrossingsCounter struct {
	DefaultInit

	crossingCounter           *CrossingsCounter
	hasHyperEdgesEastOfIndex  []bool
	hyperedgeCrossingsCounter *HyperedgeCrossingsCounter

	// For initialization.
	inLayerEdgeCounts  []int
	hasNorthSouthPorts []bool
	nPorts             int
}

// NewAllCrossingsCounter is new AllCrossingsCounter(graph).
func NewAllCrossingsCounter(graph [][]*lgraph.LNode) *AllCrossingsCounter {
	return &AllCrossingsCounter{
		inLayerEdgeCounts:        make([]int, len(graph)),
		hasNorthSouthPorts:       make([]bool, len(graph)),
		hasHyperEdgesEastOfIndex: make([]bool, len(graph)),
	}
}

// CountAllCrossings is countAllCrossings(currentOrder): the number of
// crossings in the graph with the given node order.
func (a *AllCrossingsCounter) CountAllCrossings(currentOrder [][]*lgraph.LNode) int {
	if len(currentOrder) == 0 {
		return 0
	}
	crossings := a.crossingCounter.CountInLayerCrossingsOnSide(currentOrder[0], core.PortSideWest)
	crossings += a.crossingCounter.CountInLayerCrossingsOnSide(currentOrder[len(currentOrder)-1], core.PortSideEast)
	for layerIndex := 0; layerIndex < len(currentOrder); layerIndex++ {
		crossings += a.countCrossingsAt(layerIndex, currentOrder)
	}
	return crossings
}

func (a *AllCrossingsCounter) countCrossingsAt(layerIndex int, currentOrder [][]*lgraph.LNode) int {
	totalCrossings := 0
	leftLayer := currentOrder[layerIndex]
	if layerIndex < len(currentOrder)-1 {
		rightLayer := currentOrder[layerIndex+1]
		if a.hasHyperEdgesEastOfIndex[layerIndex] {
			totalCrossings = a.hyperedgeCrossingsCounter.CountCrossings(leftLayer, rightLayer)
			totalCrossings += a.crossingCounter.CountInLayerCrossingsOnSide(leftLayer, core.PortSideEast)
			totalCrossings += a.crossingCounter.CountInLayerCrossingsOnSide(rightLayer, core.PortSideWest)
		} else {
			totalCrossings = a.crossingCounter.CountCrossingsBetweenLayers(leftLayer, rightLayer)
		}
	}

	if a.hasNorthSouthPorts[layerIndex] {
		totalCrossings += a.crossingCounter.CountNorthSouthPortCrossingsInLayer(leftLayer)
	}

	return totalCrossings
}

// InitAtNodeLevel is initAtNodeLevel(l, n, nodeOrder).
func (a *AllCrossingsCounter) InitAtNodeLevel(l, n int, nodeOrder [][]*lgraph.LNode) {
	node := nodeOrder[l][n]
	a.hasNorthSouthPorts[l] = a.hasNorthSouthPorts[l] || node.Type == lgraph.NodeTypeNorthSouthPort
}

// InitAtPortLevel is initAtPortLevel(l, n, p, nodeOrder). It sets the id of
// the port.
func (a *AllCrossingsCounter) InitAtPortLevel(l, n, p int, nodeOrder [][]*lgraph.LNode) {
	port := nodeOrder[l][n].Ports[p]
	port.ID = a.nPorts
	a.nPorts++
	if len(port.OutgoingEdges)+len(port.IncomingEdges) > 1 {
		if port.Side == core.PortSideEast {
			a.hasHyperEdgesEastOfIndex[l] = true
		} else if port.Side == core.PortSideWest && l > 0 {
			a.hasHyperEdgesEastOfIndex[l-1] = true
		}
	}
}

// InitAtEdgeLevel is initAtEdgeLevel(l, n, p, e, edge, nodeOrder).
func (a *AllCrossingsCounter) InitAtEdgeLevel(l, n, p, e int, edge *lgraph.LEdge, nodeOrder [][]*lgraph.LNode) {
	port := nodeOrder[l][n].Ports[p]
	if edge.Source == port && edge.Source.Node().Layer() == edge.Target.Node().Layer() {
		a.inLayerEdgeCounts[l]++
	}
}

// InitAfterTraversal is initAfterTraversal().
func (a *AllCrossingsCounter) InitAfterTraversal() {
	portPos := make([]int, a.nPorts)
	a.hyperedgeCrossingsCounter = NewHyperedgeCrossingsCounter(a.inLayerEdgeCounts, a.hasNorthSouthPorts, portPos)
	a.crossingCounter = NewCrossingsCounter(portPos)
}
