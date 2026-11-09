// Ported from the enum CrossMinType of
// plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/LayerSweepCrossingMinimizer.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package greedyswitch has the greedy switch heuristic of the layer sweep
// crossing minimizer: for two neighboring nodes of a layer it checks whether
// the exchange of their positions reduces the number of crossings.
package greedyswitch

import "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"

// CrossMinType is the enum LayerSweepCrossingMinimizer.CrossMinType: the
// type of the crossing minimizer. The Java enum is a member of a class of
// the package p3order. It is declared here because p3order imports this
// package, and p3order has an alias of it.
type CrossMinType int

// The constants of CrossMinType, in the order of the Java enum.
const (
	// CrossMinTypeBarycenter uses the BarycenterHeuristic.
	CrossMinTypeBarycenter CrossMinType = iota
	// CrossMinTypeOneSidedGreedySwitch uses the one-sided
	// GreedySwitchHeuristic.
	CrossMinTypeOneSidedGreedySwitch
	// CrossMinTypeTwoSidedGreedySwitch uses the two-sided
	// GreedySwitchHeuristic.
	CrossMinTypeTwoSidedGreedySwitch
)

var namesCrossMinType = []string{"BARYCENTER", "ONE_SIDED_GREEDY_SWITCH", "TWO_SIDED_GREEDY_SWITCH"}

// String returns the name of the Java constant.
func (t CrossMinType) String() string { return prop.EnumName(namesCrossMinType, int(t)) }

func init() {
	prop.RegisterEnum[CrossMinType]("org.eclipse.elk.alg.layered.p3order.LayerSweepCrossingMinimizer$CrossMinType", namesCrossMinType)
}
