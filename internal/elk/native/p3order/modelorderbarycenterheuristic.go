// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/ModelOrderBarycenterHeuristic.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p3order

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/preserveorder"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// ModelOrderBarycenterHeuristic is the class ModelOrderBarycenterHeuristic:
// it minimizes crossings with the barycenter method, but it does not change
// the node order that the model gives. Only the dummy nodes are sorted into
// the real nodes, which are sorted already.
type ModelOrderBarycenterHeuristic struct {
	*BarycenterHeuristic

	// biggerThan has, for each node, the nodes that it is bigger than.
	biggerThan *gwt.Map[*lgraph.LNode, *gwt.Set[*lgraph.LNode]]
	// smallerThan has, for each node, the nodes that it is smaller than.
	smallerThan *gwt.Map[*lgraph.LNode, *gwt.Set[*lgraph.LNode]]
}

// NewModelOrderBarycenterHeuristic is new ModelOrderBarycenterHeuristic(
// constraintResolver, random, portDistributor, graph).
func NewModelOrderBarycenterHeuristic(constraintResolver *ForsterConstraintResolver, random *gwt.Random,
	portDistributor *AbstractBarycenterPortDistributor, graph [][]*lgraph.LNode) *ModelOrderBarycenterHeuristic {
	h := &ModelOrderBarycenterHeuristic{
		BarycenterHeuristic: NewBarycenterHeuristic(constraintResolver, random, portDistributor, graph),
	}
	h.ClearTransitiveOrdering()
	h.modelOrder = h
	h.minimizeCrossingsOfLayer = h.minimizeCrossings
	h.barycenterStateComparator = func(n1, n2 *lgraph.LNode) int {
		transitiveComparison := h.compareBasedOnTansitiveDependencies(n1, n2)
		if transitiveComparison != 0 {
			return transitiveComparison
		}
		if prop.Has(n1, iprops.ModelOrder) && prop.Has(n2, iprops.ModelOrder) {
			value := gwt.IntCompare(prop.Get(n1, iprops.ModelOrder), prop.Get(n2, iprops.ModelOrder))
			if value < 0 {
				h.updateBiggerAndSmallerAssociations(n1, n2)
			} else if value > 0 {
				h.updateBiggerAndSmallerAssociations(n2, n1)
			}
			return value
		}
		return h.compareBasedOnBarycenter(n1, n2)
	}
	return h
}

// minimizeCrossings is minimizeCrossings(layer, preOrdered, randomize,
// forward). It changes the order of the nodes of the given list in place.
func (h *ModelOrderBarycenterHeuristic) minimizeCrossings(layer []*lgraph.LNode, preOrdered, randomize, forward bool) {
	if randomize {
		// Randomize barycenters (we don't need to update the edge count in
		// this case; there are no edges of interest since we're only
		// concerned with one layer)
		h.randomizeBarycenters(layer)
	} else {
		// Calculate barycenters and assign barycenters to barycenterless
		// node groups
		h.calculateBarycenters(layer, forward)
		h.fillInUnknownBarycenters(layer, preOrdered)
	}

	if len(layer) > 1 {
		// Sort the vertices according to their barycenters
		if prop.Get(layer[0].Graph(), lopts.CrossingMinimizationForceNodeModelOrder) {
			insertionSort(layer, h.barycenterStateComparator, h)
		} else {
			gwt.SortFunc(layer, h.barycenterStateComparator)
		}

		// Resolve ordering constraints
		if !prop.Get(layer[0].Graph(), lopts.CrossingMinimizationForceNodeModelOrder) {
			h.constraintResolver.ProcessConstraints(layer)
		}
	}
}

// compareBasedOnTansitiveDependencies compares two nodes by the transitive
// dependencies that were collected already. It returns -1 if n1 < n2, 0 if
// no ordering could be found, and 1 if n1 > n2.
func (h *ModelOrderBarycenterHeuristic) compareBasedOnTansitiveDependencies(n1, n2 *lgraph.LNode) int {
	if !h.biggerThan.ContainsKey(n1) {
		h.biggerThan.Put(n1, gwt.NewSet[*lgraph.LNode]())
	} else if h.biggerThan.Get(n1).Contains(n2) {
		return 1
	}
	if !h.biggerThan.ContainsKey(n2) {
		h.biggerThan.Put(n2, gwt.NewSet[*lgraph.LNode]())
	} else if h.biggerThan.Get(n2).Contains(n1) {
		return -1
	}
	if !h.smallerThan.ContainsKey(n1) {
		h.smallerThan.Put(n1, gwt.NewSet[*lgraph.LNode]())
	} else if h.smallerThan.Get(n1).Contains(n2) {
		return -1
	}
	if !h.smallerThan.ContainsKey(n2) {
		h.smallerThan.Put(n2, gwt.NewSet[*lgraph.LNode]())
	} else if h.smallerThan.Get(n2).Contains(n1) {
		return 1
	}
	return 0
}

// compareBasedOnBarycenter orders two nodes by their barycenter and updates
// their transitive dependencies.
func (h *ModelOrderBarycenterHeuristic) compareBasedOnBarycenter(n1, n2 *lgraph.LNode) int {
	s1 := h.stateOf(n1)
	s2 := h.stateOf(n2)
	if s1.HasBarycenter && s2.HasBarycenter {
		value := gwt.DoubleCompare(s1.Barycenter, s2.Barycenter)
		if value < 0 {
			h.updateBiggerAndSmallerAssociations(n1, n2)
		} else if value > 0 {
			h.updateBiggerAndSmallerAssociations(n2, n1)
		}
		return value
	} else if s1.HasBarycenter {
		h.updateBiggerAndSmallerAssociations(n1, n2)
		return -1
	} else if s2.HasBarycenter {
		h.updateBiggerAndSmallerAssociations(n2, n1)
		return 1
	}
	return 0
}

// updateBiggerAndSmallerAssociations saves the transitive ordering
// dependencies of nodes for later use. The Java method is a copy of the one
// of ModelOrderNodeComparator.
func (h *ModelOrderBarycenterHeuristic) updateBiggerAndSmallerAssociations(bigger, smaller *lgraph.LNode) {
	preserveorder.UpdateBiggerAndSmallerAssociations(h.biggerThan, h.smallerThan, bigger, smaller)
}

// insertionSort is ModelOrderBarycenterHeuristic.insertionSort(layer,
// comparator, barycenterHeuristic): it sorts the layer with insertion sort,
// so that the transitive ordering constraints are respected. No real node
// moves past another real node if the model order forbids it.
func insertionSort(layer []*lgraph.LNode, comparator func(n1, n2 *lgraph.LNode) int,
	barycenterHeuristic *ModelOrderBarycenterHeuristic) {
	for i := 1; i < len(layer); i++ {
		temp := layer[i]
		j := i
		for j > 0 && comparator(layer[j-1], temp) > 0 {
			layer[j] = layer[j-1]
			j--
		}
		layer[j] = temp
	}
	barycenterHeuristic.ClearTransitiveOrdering()
}

// ClearTransitiveOrdering is clearTransitiveOrdering().
func (h *ModelOrderBarycenterHeuristic) ClearTransitiveOrdering() {
	h.biggerThan = gwt.NewMap[*lgraph.LNode, *gwt.Set[*lgraph.LNode]]()
	h.smallerThan = gwt.NewMap[*lgraph.LNode, *gwt.Set[*lgraph.LNode]]()
}
