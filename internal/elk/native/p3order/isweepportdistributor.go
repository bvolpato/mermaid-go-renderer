// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/ISweepPortDistributor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p3order

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p3order/counting"
)

// ISweepPortDistributor is the interface ISweepPortDistributor: the port
// distributor that the layer sweep uses.
type ISweepPortDistributor interface {
	counting.IInitializable

	// DistributePortsWhileSweeping is distributePortsWhileSweeping(order,
	// freeLayerIndex, isForwardSweep): it distributes the ports of one
	// layer.
	DistributePortsWhileSweeping(order [][]*lgraph.LNode, freeLayerIndex int, isForwardSweep bool) bool
}

// CreateSweepPortDistributor is ISweepPortDistributor.create(cmt, r,
// currentOrder). It draws one random boolean, except for the two-sided
// greedy switch.
func CreateSweepPortDistributor(cmt CrossMinType, r *gwt.Random, currentOrder [][]*lgraph.LNode) ISweepPortDistributor {
	if cmt == CrossMinTypeTwoSidedGreedySwitch {
		return NewGreedyPortDistributor()
	} else if r.NextBoolean() {
		// Since both methods lead to different results, but neither is
		// clearly better, we choose randomly.
		return NewNodeRelativePortDistributor(len(currentOrder))
	}
	return NewLayerTotalPortDistributor(len(currentOrder))
}
