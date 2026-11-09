// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/greedyswitch/GreedySwitchHeuristic.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package greedyswitch

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p3order/counting"
)

// GreedySwitchHeuristic is the class GreedySwitchHeuristic: for two
// neighboring nodes it checks whether the exchange of their positions
// reduces the number of crossings, and switches them when it does. It goes
// on through each layer of the graph.
//
// The one-sided type fixes the order of one layer and changes the order of
// a neighboring layer by the number of crossings to that layer. The
// two-sided type counts the crossings to both neighboring layers.
//
// It is an ICrossingMinimizationHeuristic of the package p3order.
type GreedySwitchHeuristic struct {
	counting.DefaultInit

	greedySwitchType CrossMinType
	currentNodeOrder [][]*lgraph.LNode
	switchDecider    *SwitchDecider
	portPositions    []int
	graphData        GraphData
	nPorts           int
}

// NewGreedySwitchHeuristic is new GreedySwitchHeuristic(greedyType,
// graphData).
func NewGreedySwitchHeuristic(greedyType CrossMinType, graphData GraphData) *GreedySwitchHeuristic {
	return &GreedySwitchHeuristic{graphData: graphData, greedySwitchType: greedyType}
}

// MinimizeCrossings is minimizeCrossings(order, freeLayerIndex,
// forwardSweep, isFirstSweep).
func (h *GreedySwitchHeuristic) MinimizeCrossings(order [][]*lgraph.LNode, freeLayerIndex int, forwardSweep, isFirstSweep bool) bool {
	h.setUp(order, freeLayerIndex, forwardSweep)
	return h.continueSwitchingUntilNoImprovementInLayer(freeLayerIndex)
}

// SetFirstLayerOrder is setFirstLayerOrder(currentOrder, isForwardSweep).
func (h *GreedySwitchHeuristic) SetFirstLayerOrder(currentOrder [][]*lgraph.LNode, isForwardSweep bool) bool {
	startIndex := h.startIndex(isForwardSweep, len(currentOrder))
	h.setUp(currentOrder, startIndex, isForwardSweep)
	return h.sweepDownwardInLayer(startIndex)
}

func (h *GreedySwitchHeuristic) setUp(order [][]*lgraph.LNode, freeLayerIndex int, forwardSweep bool) {
	h.currentNodeOrder = order
	side := CrossingCountSideEast
	if forwardSweep {
		side = CrossingCountSideWest
	}
	h.switchDecider = h.getNewSwitchDecider(freeLayerIndex, side)
}

func (h *GreedySwitchHeuristic) getNewSwitchDecider(freeLayerIndex int, side CrossingCountSide) *SwitchDecider {
	crossingMatrixFiller := NewCrossingMatrixFiller(h.greedySwitchType, h.currentNodeOrder, freeLayerIndex, side)
	return NewSwitchDecider(freeLayerIndex, h.currentNodeOrder, crossingMatrixFiller, h.portPositions,
		h.graphData, h.greedySwitchType == CrossMinTypeOneSidedGreedySwitch)
}

func (h *GreedySwitchHeuristic) continueSwitchingUntilNoImprovementInLayer(freeLayerIndex int) bool {
	improved := false
	for {
		continueSwitching := h.sweepDownwardInLayer(freeLayerIndex)
		improved = improved || continueSwitching
		if !continueSwitching {
			break
		}
	}
	return improved
}

func (h *GreedySwitchHeuristic) sweepDownwardInLayer(layerIndex int) bool {
	continueSwitching := false
	lengthOfFreeLayer := len(h.currentNodeOrder[layerIndex])
	for upperNodeIndex := 0; upperNodeIndex < lengthOfFreeLayer-1; upperNodeIndex++ {
		lowerNodeIndex := upperNodeIndex + 1

		// The Java operator |= evaluates its right side in each case.
		switched := h.switchIfImproves(layerIndex, upperNodeIndex, lowerNodeIndex)
		continueSwitching = continueSwitching || switched
	}
	return continueSwitching
}

func (h *GreedySwitchHeuristic) switchIfImproves(layerIndex, upperNodeIndex, lowerNodeIndex int) bool {
	continueSwitching := false

	if h.switchDecider.DoesSwitchReduceCrossings(upperNodeIndex, lowerNodeIndex) {
		h.exchangeNodes(upperNodeIndex, lowerNodeIndex, layerIndex)

		continueSwitching = true
	}
	return continueSwitching
}

func (h *GreedySwitchHeuristic) exchangeNodes(indexOne, indexTwo, layerIndex int) {
	h.switchDecider.NotifyOfSwitch(h.currentNodeOrder[layerIndex][indexOne], h.currentNodeOrder[layerIndex][indexTwo])
	layer := h.currentNodeOrder[layerIndex]
	temp := layer[indexTwo]
	layer[indexTwo] = layer[indexOne]
	layer[indexOne] = temp
}

func (h *GreedySwitchHeuristic) startIndex(isForwardSweep bool, length int) int {
	if isForwardSweep {
		return 0
	}
	return length - 1
}

// AlwaysImproves is alwaysImproves().
func (h *GreedySwitchHeuristic) AlwaysImproves() bool {
	return !(h.greedySwitchType == CrossMinTypeOneSidedGreedySwitch)
}

// IsDeterministic is isDeterministic().
func (h *GreedySwitchHeuristic) IsDeterministic() bool { return true }

// InitAtPortLevel is initAtPortLevel(l, n, p, nodeOrder).
func (h *GreedySwitchHeuristic) InitAtPortLevel(l, n, p int, nodeOrder [][]*lgraph.LNode) {
	h.nPorts++
}

// InitAtLayerLevel is initAtLayerLevel(l, nodeOrder). It sets the id of the
// layer.
func (h *GreedySwitchHeuristic) InitAtLayerLevel(l int, nodeOrder [][]*lgraph.LNode) {
	nodeOrder[l][0].Layer().ID = l
}

// InitAfterTraversal is initAfterTraversal().
func (h *GreedySwitchHeuristic) InitAfterTraversal() {
	h.portPositions = make([]int, h.nPorts)
}
