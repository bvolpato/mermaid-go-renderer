// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/GreedyPortDistributor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p3order

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/greedyswitch"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p3order/counting"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// GreedyPortDistributor is the class GreedyPortDistributor: it distributes
// the ports of a single node greedily.
type GreedyPortDistributor struct {
	counting.DefaultInit

	crossingsCounter             *counting.CrossingsCounter
	nPorts                       int
	portPos                      []int
	hierarchicalCrossingsCounter *greedyswitch.BetweenLayerEdgeTwoNodeCrossingsCounter
}

// NewGreedyPortDistributor is new GreedyPortDistributor().
func NewGreedyPortDistributor() *GreedyPortDistributor { return &GreedyPortDistributor{} }

// DistributePortsWhileSweeping is distributePortsWhileSweeping(nodeOrder,
// currentIndex, isForwardSweep).
func (d *GreedyPortDistributor) DistributePortsWhileSweeping(nodeOrder [][]*lgraph.LNode, currentIndex int, isForwardSweep bool) bool {
	d.initialize(nodeOrder, currentIndex, isForwardSweep)

	return d.distributePortsInLayer(nodeOrder, currentIndex, isForwardSweep)
}

func (d *GreedyPortDistributor) distributePortsInLayer(nodeOrder [][]*lgraph.LNode, currentIndex int, isForwardSweep bool) bool {
	side := core.PortSideEast
	if isForwardSweep {
		side = core.PortSideWest
	}
	improved := false
	for _, node := range nodeOrder[currentIndex] {
		if prop.Get(node, lopts.PortConstraints).IsOrderFixed() {
			continue
		}
		nestedGraph := node.NestedGraph
		useHierarchicalCrossCounter := len(node.PortSideView(side)) != 0 && nestedGraph != nil
		if useHierarchicalCrossCounter {
			// A node with a nested graph whose port order is not fixed. The
			// pipeline of elkjs does not get here: a nested graph is laid
			// out completely before the greedy switch of the root graph
			// runs, and its parent node has fixed port positions then. No
			// golden covers the crossings counter of the nested graph, so
			// the layout stops here.
			lgraph.Bail("GreedyPortDistributor: the ports of a node with a nested graph have no fixed order, and this case has no golden")
			innerGraph := nestedGraph.ToNodeArray()
			freeLayerIndex := len(innerGraph) - 1
			if isForwardSweep {
				freeLayerIndex = 0
			}
			d.hierarchicalCrossingsCounter = greedyswitch.NewBetweenLayerEdgeTwoNodeCrossingsCounter(innerGraph, freeLayerIndex)
		}
		// The Java operator |= evaluates its right side in each case.
		nodeImproved := d.distributePortsOnNode(node, side, useHierarchicalCrossCounter)
		improved = improved || nodeImproved
	}
	return improved
}

// distributePortsOnNode distributes the ports of one side of a single node
// greedily.
func (d *GreedyPortDistributor) distributePortsOnNode(node *lgraph.LNode, side core.PortSide, useHierarchicalCrosscounter bool) bool {
	// The Java list is a view of the port list of the node, reversed for
	// the sides SOUTH and WEST. A switch changes the port list.
	view := node.PortSideView(side)
	reversed := side == core.PortSideSouth || side == core.PortSideWest
	at := func(i int) int {
		if reversed {
			return len(view) - 1 - i
		}
		return i
	}
	improved := false
	for {
		continueSwitching := false
		for i := 0; i < len(view)-1; i++ {
			upperPort := view[at(i)]
			lowerPort := view[at(i+1)]
			if d.switchingDecreasesCrossings(upperPort, lowerPort, node, useHierarchicalCrosscounter) {
				improved = true
				// switchPorts(ports, node, i, i + 1)
				d.crossingsCounter.SwitchPorts(view[at(i)], view[at(i+1)])
				lower := view[at(i+1)]
				view[at(i+1)] = view[at(i)]
				view[at(i)] = lower
				continueSwitching = true
			}
		}
		if !continueSwitching {
			break
		}
	}
	return improved
}

// initForLayers initializes the crossings counter for the given layers.
func (d *GreedyPortDistributor) initForLayers(leftLayer, rightLayer []*lgraph.LNode) {
	d.crossingsCounter.InitForCountingBetween(leftLayer, rightLayer)
}

func (d *GreedyPortDistributor) switchingDecreasesCrossings(upperPort, lowerPort *lgraph.LPort, node *lgraph.LNode, useHierarchicalCrosscounter bool) bool {
	upperLowerCrossings, lowerUpperCrossings := d.crossingsCounter.CountCrossingsBetweenPortsInBothOrders(upperPort, lowerPort)
	if useHierarchicalCrosscounter {
		upperNode := prop.Get(upperPort, iprops.PortDummy)
		lowerNode := prop.Get(lowerPort, iprops.PortDummy)
		if upperNode != nil && lowerNode != nil {
			d.hierarchicalCrossingsCounter.CountBothSideCrossings(upperNode, lowerNode)
			upperLowerCrossings += d.hierarchicalCrossingsCounter.UpperLowerCrossings()
			lowerUpperCrossings += d.hierarchicalCrossingsCounter.LowerUpperCrossings()
		}
	}
	return upperLowerCrossings > lowerUpperCrossings
}

func (d *GreedyPortDistributor) initialize(nodeOrder [][]*lgraph.LNode, currentIndex int, isForwardSweep bool) {
	if isForwardSweep && currentIndex > 0 {
		d.initForLayers(nodeOrder[currentIndex-1], nodeOrder[currentIndex])
	} else if !isForwardSweep && currentIndex < len(nodeOrder)-1 {
		d.initForLayers(nodeOrder[currentIndex], nodeOrder[currentIndex+1])
	} else {
		side := core.PortSideEast
		if isForwardSweep {
			side = core.PortSideWest
		}
		d.crossingsCounter.InitPortPositionsForInLayerCrossings(nodeOrder[currentIndex], side)
	}
}

// InitAtNodeLevel is initAtNodeLevel(l, n, nodeOrder).
func (d *GreedyPortDistributor) InitAtNodeLevel(l, n int, nodeOrder [][]*lgraph.LNode) {
	node := nodeOrder[l][n]
	d.nPorts += len(node.Ports)
}

// InitAfterTraversal is initAfterTraversal().
func (d *GreedyPortDistributor) InitAfterTraversal() {
	d.portPos = make([]int, d.nPorts)
	d.crossingsCounter = counting.NewCrossingsCounter(d.portPos)
}
