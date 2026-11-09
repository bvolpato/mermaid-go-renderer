// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/greedyswitch/CrossingMatrixFiller.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package greedyswitch

import "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"

// CrossingMatrixFiller is the class CrossingMatrixFiller: it manages the
// crossing matrix and fills it on demand. It is made again for each free
// layer. The ids of the nodes of a layer must be set from 0 to the size of
// the layer minus 1.
type CrossingMatrixFiller struct {
	isCrossingMatrixFilled        [][]bool
	crossingMatrix                [][]int
	inBetweenLayerCrossingCounter *BetweenLayerEdgeTwoNodeCrossingsCounter
	direction                     CrossingCountSide
	oneSided                      bool
}

// NewCrossingMatrixFiller is new CrossingMatrixFiller(greedySwitchType,
// graph, freeLayerIndex, direction).
func NewCrossingMatrixFiller(greedySwitchType CrossMinType, graph [][]*lgraph.LNode, freeLayerIndex int,
	direction CrossingCountSide) *CrossingMatrixFiller {
	f := &CrossingMatrixFiller{
		direction: direction,
		oneSided:  greedySwitchType == CrossMinTypeOneSidedGreedySwitch,
	}

	freeLayer := graph[freeLayerIndex]
	f.isCrossingMatrixFilled = make([][]bool, len(freeLayer))
	f.crossingMatrix = make([][]int, len(freeLayer))
	for i := range freeLayer {
		f.isCrossingMatrixFilled[i] = make([]bool, len(freeLayer))
		f.crossingMatrix[i] = make([]int, len(freeLayer))
	}

	f.inBetweenLayerCrossingCounter = NewBetweenLayerEdgeTwoNodeCrossingsCounter(graph, freeLayerIndex)
	return f
}

// GetCrossingMatrixEntry is getCrossingMatrixEntry(upperNode, lowerNode):
// the crossings between the edges incident to two nodes, where upperNode is
// above lowerNode in the layer.
func (f *CrossingMatrixFiller) GetCrossingMatrixEntry(upperNode, lowerNode *lgraph.LNode) int {
	if !f.isCrossingMatrixFilled[upperNode.ID][lowerNode.ID] {
		f.fillCrossingMatrix(upperNode, lowerNode)
		f.isCrossingMatrixFilled[upperNode.ID][lowerNode.ID] = true
		f.isCrossingMatrixFilled[lowerNode.ID][upperNode.ID] = true
	}
	return f.crossingMatrix[upperNode.ID][lowerNode.ID]
}

func (f *CrossingMatrixFiller) fillCrossingMatrix(upperNode, lowerNode *lgraph.LNode) {
	if f.oneSided {
		switch f.direction {
		case CrossingCountSideEast:
			f.inBetweenLayerCrossingCounter.CountEasternEdgeCrossings(upperNode, lowerNode)
		case CrossingCountSideWest:
			f.inBetweenLayerCrossingCounter.CountWesternEdgeCrossings(upperNode, lowerNode)
		}
	} else {
		f.inBetweenLayerCrossingCounter.CountBothSideCrossings(upperNode, lowerNode)
	}
	f.crossingMatrix[upperNode.ID][lowerNode.ID] = f.inBetweenLayerCrossingCounter.UpperLowerCrossings()
	f.crossingMatrix[lowerNode.ID][upperNode.ID] = f.inBetweenLayerCrossingCounter.LowerUpperCrossings()
}
