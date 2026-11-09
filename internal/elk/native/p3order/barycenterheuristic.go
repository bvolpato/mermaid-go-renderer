// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/BarycenterHeuristic.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p3order

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p3order/counting"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// BarycenterHeuristic is the class BarycenterHeuristic: it determines the
// node order of a free layer with the barycenter heuristic, so that the edge
// crossings between the free layer and a neighboring layer with a fixed node
// order are few.
type BarycenterHeuristic struct {
	counting.DefaultInit

	// portRanks is the array of port ranks.
	portRanks []float64
	// random is the random number generator.
	random *gwt.Random
	// constraintResolver is the constraint resolver for ordering
	// constraints.
	constraintResolver *ForsterConstraintResolver
	// barycenterState has the barycenter values of every node in the graph,
	// indexed by layer.id and node.id.
	barycenterState [][]*BarycenterState
	// portDistributor is asked for the port ranks.
	portDistributor *AbstractBarycenterPortDistributor

	// barycenterStateComparator compares two nodes by their barycenter
	// values. ModelOrderBarycenterHeuristic replaces it.
	barycenterStateComparator func(n1, n2 *lgraph.LNode) int
	// minimizeCrossingsOfLayer is the method minimizeCrossings(layer,
	// preOrdered, randomize, forward), which ModelOrderBarycenterHeuristic
	// overrides.
	minimizeCrossingsOfLayer func(layer []*lgraph.LNode, preOrdered, randomize, forward bool)
	// modelOrder is the object as a ModelOrderBarycenterHeuristic, or nil.
	modelOrder *ModelOrderBarycenterHeuristic
}

// NewBarycenterHeuristic is new BarycenterHeuristic(constraintResolver,
// random, portDistributor, graph).
func NewBarycenterHeuristic(constraintResolver *ForsterConstraintResolver, random *gwt.Random,
	portDistributor *AbstractBarycenterPortDistributor, graph [][]*lgraph.LNode) *BarycenterHeuristic {
	h := &BarycenterHeuristic{
		constraintResolver: constraintResolver,
		random:             random,
		portDistributor:    portDistributor,
	}
	h.barycenterStateComparator = h.compareByBarycenterState
	h.minimizeCrossingsOfLayer = h.minimizeCrossings
	return h
}

// minimizeCrossings is minimizeCrossings(layer, preOrdered, randomize,
// forward). It changes the order of the nodes of the given list in place.
func (h *BarycenterHeuristic) minimizeCrossings(layer []*lgraph.LNode, preOrdered, randomize, forward bool) {
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
			if h.modelOrder == nil {
				// The Java code casts this to ModelOrderBarycenterHeuristic.
				lgraph.Bail("BarycenterHeuristic: the node model order is forced, and the heuristic is not a ModelOrderBarycenterHeuristic")
			}
			insertionSort(layer, h.barycenterStateComparator, h.modelOrder)
		} else {
			gwt.SortFunc(layer, h.barycenterStateComparator)
		}

		// Resolve ordering constraints
		h.constraintResolver.ProcessConstraints(layer)
	}
}

// randomizeBarycenters randomizes the order of the nodes of the given
// layer.
func (h *BarycenterHeuristic) randomizeBarycenters(nodes []*lgraph.LNode) {
	for _, node := range nodes {
		// Set barycenters only for nodeGroups containing a single node.
		h.stateOf(node).Barycenter = h.random.NextDouble()
		h.stateOf(node).HasBarycenter = true
		h.stateOf(node).SummedWeight = h.stateOf(node).Barycenter
		h.stateOf(node).Degree = 1
	}
}

// fillInUnknownBarycenters tries to find appropriate barycenter values for
// the nodes whose barycenter is undefined. preOrdered tells whether the
// nodes have been ordered in a previous run.
func (h *BarycenterHeuristic) fillInUnknownBarycenters(nodes []*lgraph.LNode, preOrdered bool) {
	// Determine placements for nodes with undefined barycenter value
	if preOrdered {
		lastValue := -1.0
		for nodeIndex := 0; nodeIndex < len(nodes); nodeIndex++ {
			node := nodes[nodeIndex]
			value := h.stateOf(node).Barycenter

			if !h.stateOf(node).HasBarycenter {
				// The barycenter is undefined - take the center of the
				// previous value and the next defined value in the list
				nextValue := lastValue + 1

				for nextIndex := nodeIndex + 1; nextIndex < len(nodes); nextIndex++ {
					next := h.stateOf(nodes[nextIndex])
					if next.HasBarycenter {
						nextValue = next.Barycenter
						break
					}
				}

				value = (lastValue + nextValue) / 2
				h.stateOf(node).Barycenter = value
				h.stateOf(node).HasBarycenter = true
				h.stateOf(node).SummedWeight = value
				h.stateOf(node).Degree = 1
			}

			lastValue = value
		}
	} else {
		// No previous ordering - determine random placement for new nodes
		maxBary := 0.0
		for _, node := range nodes {
			if h.stateOf(node).HasBarycenter {
				maxBary = max(maxBary, h.stateOf(node).Barycenter)
			}
		}

		maxBary += 2
		for _, node := range nodes {
			if !h.stateOf(node).HasBarycenter {
				value := float64(h.random.NextFloat()*maxBary) - 1
				h.stateOf(node).Barycenter = value
				h.stateOf(node).HasBarycenter = true
				h.stateOf(node).SummedWeight = value
				h.stateOf(node).Degree = 1
			}
		}
	}
}

// calculateBarycenters calculates the barycenters of the given nodes.
func (h *BarycenterHeuristic) calculateBarycenters(nodes []*lgraph.LNode, forward bool) {
	// Set all visited flags to false
	for _, node := range nodes {
		h.stateOf(node).Visited = false
	}

	for _, node := range nodes {
		// Calculate the node groups's new barycenter (may be null)
		h.calculateBarycenter(node, forward)
	}
}

// The amount of random value to add to each calculated barycenter. The Java
// constant is the float 0.07f. The compiled JavaScript has the float value
// and the half of it as literals.
const (
	randomAmount     = 0.07000000029802322
	randomAmountHalf = 0.03500000014901161
)

// calculateBarycenter calculates the barycenter of the given node. This
// method is able to handle in-layer edges, but it may give incorrect
// results if the in-layer edges form a cycle.
func (h *BarycenterHeuristic) calculateBarycenter(node *lgraph.LNode, forward bool) {
	// Check if the node group's barycenter was already computed
	if h.stateOf(node).Visited {
		return
	}
	h.stateOf(node).Visited = true

	h.stateOf(node).Degree = 0
	h.stateOf(node).SummedWeight = 0.0
	h.stateOf(node).HasBarycenter = false
	h.stateOf(node).Barycenter = 0

	for _, freePort := range node.Ports {
		var portIterable []*lgraph.LPort
		if forward {
			portIterable = freePort.PredecessorPorts()
		} else {
			portIterable = freePort.SuccessorPorts()
		}
		for _, fixedPort := range portIterable {
			// If the node the fixed port belongs to is part of the free
			// layer (thus, if we have an in-layer edge), use that node's
			// barycenter calculation instead
			fixedNode := fixedPort.Node()

			if fixedNode.Layer() == node.Layer() {
				// Self-loops are ignored
				if fixedNode != node {
					// Find the fixed node's node group and calculate its
					// barycenter
					h.calculateBarycenter(fixedNode, forward)

					// Update this node group's values
					h.stateOf(node).Degree += h.stateOf(fixedNode).Degree
					h.stateOf(node).SummedWeight += h.stateOf(fixedNode).SummedWeight
				}
			} else {
				h.stateOf(node).SummedWeight += h.portRanks[fixedPort.ID]
				h.stateOf(node).Degree++
			}
		}
	}

	// Iterate over the node's barycenter associates
	barycenterAssociates := prop.Get(node, iprops.BarycenterAssociates)
	if barycenterAssociates != nil {
		for _, associate := range barycenterAssociates {
			// Make sure the associate is in the same layer as this node
			if node.Layer() == associate.Layer() {
				// Find the associate's node group and calculate its
				// barycenter
				h.calculateBarycenter(associate, forward)

				// Update this vertex's values
				h.stateOf(node).Degree += h.stateOf(associate).Degree
				h.stateOf(node).SummedWeight += h.stateOf(associate).SummedWeight
			}
		}
	}

	if h.stateOf(node).Degree > 0 {
		// add a small random perturbation in order to increase diversity of
		// solutions
		h.stateOf(node).SummedWeight += float64(h.random.NextFloat()*randomAmount) - randomAmountHalf
		h.stateOf(node).Barycenter = h.stateOf(node).SummedWeight / float64(h.stateOf(node).Degree)
		h.stateOf(node).HasBarycenter = true
	}
}

func (h *BarycenterHeuristic) stateOf(node *lgraph.LNode) *BarycenterState {
	return h.barycenterState[node.Layer().ID][node.ID]
}

// BarycenterState is the class BarycenterHeuristic.BarycenterState: the
// current barycenter state of a node.
type BarycenterState struct {
	// Node is the node this state holds data for.
	Node *lgraph.LNode
	// SummedWeight is the sum of the node weights. Each node weight is the
	// sum of the weights of the ports the node's ports are connected to.
	SummedWeight float64
	// Degree is the number of ports relevant to the barycenter calculation.
	Degree int
	// Barycenter is the barycenter value (summedWeight / degree). The Java
	// field is a Double that can be null: HasBarycenter is false then.
	Barycenter    float64
	HasBarycenter bool
	// Visited tells whether the node group has been visited in some
	// traversing algorithm.
	Visited bool
}

// NewBarycenterState is new BarycenterState(node).
func NewBarycenterState(node *lgraph.LNode) *BarycenterState {
	return &BarycenterState{Node: node}
}

// compareByBarycenterState is the comparator barycenterStateComparator of
// BarycenterHeuristic: it compares two nodes by their barycenter values.
func (h *BarycenterHeuristic) compareByBarycenterState(n1, n2 *lgraph.LNode) int {
	s1 := h.stateOf(n1)
	s2 := h.stateOf(n2)
	if s1.HasBarycenter && s2.HasBarycenter {
		return gwt.DoubleCompare(s1.Barycenter, s2.Barycenter)
	} else if s1.HasBarycenter {
		return -1
	} else if s2.HasBarycenter {
		return 1
	}
	return 0
}

// MinimizeCrossings is minimizeCrossings(order, freeLayerIndex,
// forwardSweep, isFirstSweep).
func (h *BarycenterHeuristic) MinimizeCrossings(order [][]*lgraph.LNode, freeLayerIndex int, forwardSweep, isFirstSweep bool) bool {
	if !h.isFirstLayer(order, freeLayerIndex, forwardSweep) {
		fixedLayer := order[freeLayerIndex-h.changeIndex(forwardSweep)]
		h.portDistributor.CalculatePortRanks(fixedLayer, h.portTypeFor(forwardSweep))
	}

	firstNodeInLayer := order[freeLayerIndex][0]
	preOrdered := !isFirstSweep || h.isExternalPortDummy(firstNodeInLayer)

	nodes := append([]*lgraph.LNode(nil), order[freeLayerIndex]...)
	h.minimizeCrossingsOfLayer(nodes, preOrdered, false, forwardSweep)
	// apply the new ordering
	copy(order[freeLayerIndex], nodes)

	return false // Does not always improve.
}

// SetFirstLayerOrder is setFirstLayerOrder(order, isForwardSweep).
func (h *BarycenterHeuristic) SetFirstLayerOrder(order [][]*lgraph.LNode, isForwardSweep bool) bool {
	startIndex := h.startIndex(isForwardSweep, len(order))
	nodes := append([]*lgraph.LNode(nil), order[startIndex]...)
	h.minimizeCrossingsOfLayer(nodes, false, true, isForwardSweep)
	copy(order[startIndex], nodes)

	return false // Does not always improve
}

func (h *BarycenterHeuristic) isExternalPortDummy(firstNode *lgraph.LNode) bool {
	return firstNode.Type == lgraph.NodeTypeExternalPort
}

func (h *BarycenterHeuristic) changeIndex(dir bool) int {
	if dir {
		return 1
	}
	return -1
}

func (h *BarycenterHeuristic) portTypeFor(direction bool) lenum.PortType {
	if direction {
		return lenum.PortTypeOutput
	}
	return lenum.PortTypeInput
}

func (h *BarycenterHeuristic) startIndex(dir bool, length int) int {
	if dir {
		return 0
	}
	return max(0, length-1)
}

func (h *BarycenterHeuristic) isFirstLayer(nodeOrder [][]*lgraph.LNode, currentIndex int, forwardSweep bool) bool {
	return currentIndex == h.startIndex(forwardSweep, len(nodeOrder))
}

// AlwaysImproves is alwaysImproves().
func (h *BarycenterHeuristic) AlwaysImproves() bool { return false }

// IsDeterministic is isDeterministic().
func (h *BarycenterHeuristic) IsDeterministic() bool { return false }

// InitAfterTraversal is initAfterTraversal().
func (h *BarycenterHeuristic) InitAfterTraversal() {
	h.barycenterState = h.constraintResolver.BarycenterStates()
	h.portRanks = h.portDistributor.PortRanks()
}

// InitAtLayerLevel is initAtLayerLevel(l, nodeOrder). It sets the id of the
// layer.
func (h *BarycenterHeuristic) InitAtLayerLevel(l int, nodeOrder [][]*lgraph.LNode) {
	nodeOrder[l][0].Layer().ID = l
}
