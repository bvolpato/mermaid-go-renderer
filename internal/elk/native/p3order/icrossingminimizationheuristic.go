// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/ICrossingMinimizationHeuristic.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package p3order has the third phase of the layered algorithm, the crossing
// minimization: the layer sweep crossing minimizer with the barycenter
// heuristic, the constraint resolver, the port distributors, and the
// minimizer that does nothing.
package p3order

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p3order/counting"
)

// ICrossingMinimizationHeuristic is the interface
// ICrossingMinimizationHeuristic: it determines the node order of a free
// layer so that the edge crossings between the free layer and a neighboring
// layer with a fixed node order are few.
type ICrossingMinimizationHeuristic interface {
	counting.IInitializable

	// AlwaysImproves is alwaysImproves(): whether the heuristic always
	// improves.
	AlwaysImproves() bool

	// SetFirstLayerOrder is setFirstLayerOrder(order, forwardSweep): it sets
	// the order of the first fixed layer of a sweep as the heuristic needs
	// it.
	SetFirstLayerOrder(order [][]*lgraph.LNode, forwardSweep bool) bool

	// MinimizeCrossings is minimizeCrossings(order, freeLayerIndex,
	// forwardSweep, isFirstSweep): it minimizes the crossings in the layer
	// with the given index.
	MinimizeCrossings(order [][]*lgraph.LNode, freeLayerIndex int, forwardSweep, isFirstSweep bool) bool

	// IsDeterministic is isDeterministic(): whether the heuristic is
	// deterministic or has random decisions.
	IsDeterministic() bool
}
