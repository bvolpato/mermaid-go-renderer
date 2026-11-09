// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/LayerSweepCrossingMinimizer.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p3order

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/greedyswitch"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p3order/counting"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/preserveorder"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// CrossMinType is the enum LayerSweepCrossingMinimizer.CrossMinType: the
// type of the crossing minimizer.
type CrossMinType = greedyswitch.CrossMinType

// The constants of CrossMinType.
const (
	// CrossMinTypeBarycenter uses the BarycenterHeuristic.
	CrossMinTypeBarycenter = greedyswitch.CrossMinTypeBarycenter
	// CrossMinTypeOneSidedGreedySwitch uses the one-sided
	// GreedySwitchHeuristic.
	CrossMinTypeOneSidedGreedySwitch = greedyswitch.CrossMinTypeOneSidedGreedySwitch
	// CrossMinTypeTwoSidedGreedySwitch uses the two-sided
	// GreedySwitchHeuristic.
	CrossMinTypeTwoSidedGreedySwitch = greedyswitch.CrossMinTypeTwoSidedGreedySwitch
)

func init() {
	// The barycenter type is the phase 3 of the layered algorithm. The
	// greedy switch types are intermediate processors before phase 4.
	lgraph.Register("LayerSweepCrossingMinimizer", func() lgraph.Processor {
		return NewLayerSweepCrossingMinimizer(CrossMinTypeBarycenter)
	})
	lgraph.Register("LayerSweepCrossingMinimizer:ONE_SIDED_GREEDY_SWITCH", func() lgraph.Processor {
		return NewLayerSweepCrossingMinimizer(CrossMinTypeOneSidedGreedySwitch)
	})
	lgraph.Register("LayerSweepCrossingMinimizer:TWO_SIDED_GREEDY_SWITCH", func() lgraph.Processor {
		return NewLayerSweepCrossingMinimizer(CrossMinTypeTwoSidedGreedySwitch)
	})
}

// LayerSweepCrossingMinimizer is the class LayerSweepCrossingMinimizer. It
// minimizes crossings by sweeping through a graph: it holds the order of
// the nodes of one layer fixed and switches the nodes of the other layer.
// After each step it sorts the ports of the two layers.
//
// Each graph of a hierarchy is handled bottom-up or hierarchically, as the
// LayerSweepTypeDecider says. Bottom-up: the nodes of a child graph are
// sorted, then the order of the ports of the edges that cross the hierarchy
// border is fixed, then the parent graph is laid out with the child graph as
// an atomic node. Hierarchical: when the sweep reaches a node with such a
// child graph, the ports are sorted on the outside of the graph, the nodes
// on the inside are sorted by the order of the ports, the sweep goes
// through the child graph, the ports of the parent node are sorted by the
// order of the nodes of the last layer of the child graph, and the sweep
// through the parent graph goes on.
//
// So it is a hierarchical processor: it runs one time, for the root graph.
type LayerSweepCrossingMinimizer struct {
	// graphInfoHolders has the collected information about each graph.
	graphInfoHolders []*GraphInfoHolder
	// graphsWhoseNodeOrderChanged has the graphs whose node order changed.
	// Only their orders must be saved.
	graphsWhoseNodeOrderChanged *gwt.Set[*GraphInfoHolder]
	random                      *gwt.Random
	randomSeed                  int64
	crossMinType                CrossMinType
}

// NewLayerSweepCrossingMinimizer is new LayerSweepCrossingMinimizer(cT).
func NewLayerSweepCrossingMinimizer(cT CrossMinType) *LayerSweepCrossingMinimizer {
	return &LayerSweepCrossingMinimizer{crossMinType: cT}
}

// HierarchyAware marks the processor as an IHierarchyAwareLayoutProcessor.
func (m *LayerSweepCrossingMinimizer) HierarchyAware() {}

// Process is process(layeredGraph, progressMonitor).
func (m *LayerSweepCrossingMinimizer) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	defer preserveorder.BailOnPanic("LayerSweepCrossingMinimizer")

	// Short-circuit cases in which no crossings can be minimized. Note that
	// in-layer edges may be subject to crossing minimization if |layers| =
	// 1 and that hierarchical crossing minimization may dive into a graph
	// with a single node. There can be graphs that consist only of empty
	// layers, for example due to inside self-loops with unconnected ports
	// (see #298)
	emptyGraph := true
	for _, layer := range layeredGraph.Layers {
		if len(layer.Nodes) != 0 {
			emptyGraph = false
			break
		}
	}
	singleNode := len(layeredGraph.Layers) == 1 && len(layeredGraph.Layers[0].Nodes) == 1
	hierarchicalLayout := prop.Get(layeredGraph, lopts.HierarchyHandling) == core.HierarchyHandlingIncludeChildren

	if emptyGraph || (singleNode && !hierarchicalLayout) {
		return
	}

	graphsToSweepOn := m.initialize(layeredGraph)

	minimizingMethod := m.chooseMinimizingMethod(graphsToSweepOn)

	m.minimizeCrossings(graphsToSweepOn, minimizingMethod)

	m.transferNodeAndPortOrdersToGraph()
}

func (m *LayerSweepCrossingMinimizer) chooseMinimizingMethod(graphsToSweepOn []*GraphInfoHolder) func(*GraphInfoHolder) {
	parent := graphsToSweepOn[0]
	if !parent.CrossMinDeterministic() {
		return m.compareDifferentRandomizedLayouts
	} else if parent.CrossMinAlwaysImproves() {
		return m.minimizeCrossingsNoCounter
	}
	return func(gData *GraphInfoHolder) { m.minimizeCrossingsWithCounter(gData) }
}

func (m *LayerSweepCrossingMinimizer) minimizeCrossings(graphsToSweepOn []*GraphInfoHolder, minimizingMethod func(*GraphInfoHolder)) {
	for _, gData := range graphsToSweepOn {
		if len(gData.CurrentNodeOrder()) > 0 {
			minimizingMethod(gData)
			if gData.HasParent() {
				m.setPortOrderOnParentGraph(gData)
			}
		}
	}
}

func (m *LayerSweepCrossingMinimizer) setPortOrderOnParentGraph(gData *GraphInfoHolder) {
	if gData.HasExternalPorts() {
		bestSweep := gData.GetBestSweep()
		// Sort ports on left and right side of the parent node
		m.sortPortsByDummyPositionsInLastLayer(bestSweep.Nodes(), gData.Parent(), true)
		m.sortPortsByDummyPositionsInLastLayer(bestSweep.Nodes(), gData.Parent(), false)
		prop.Set(gData.Parent(), lopts.PortConstraints, core.PortConstraintsFixedOrder)
	}
}

// minimizeCrossingsNoCounter is for use with any two-layer crossing
// minimizer which always improves crossings (e.g. two-sided greedy switch).
func (m *LayerSweepCrossingMinimizer) minimizeCrossingsNoCounter(gData *GraphInfoHolder) {
	isForwardSweep := m.random.NextBoolean()
	improved := true
	for improved {
		improved = gData.CrossMinimizer().SetFirstLayerOrder(gData.CurrentNodeOrder(), isForwardSweep)
		// The Java operator |= evaluates its right side in each case.
		swept := m.sweepReducingCrossings(gData, isForwardSweep, false)
		improved = improved || swept
		isForwardSweep = !isForwardSweep
	}
	m.setCurrentlyBestNodeOrders()
}

func (m *LayerSweepCrossingMinimizer) compareDifferentRandomizedLayouts(gData *GraphInfoHolder) {
	// Reset the seed, otherwise copies of hierarchical graphs in different
	// parent nodes are layouted differently.
	m.random.SetSeed(m.randomSeed)

	// In order to only copy graphs whose node order has changed, save them
	// in a set.
	m.graphsWhoseNodeOrderChanged.Clear()

	// The Java code tests the node influence two times, and not the port
	// influence.
	if prop.Get(gData.LGraph(), lopts.ConsiderModelOrderCrossingCounterNodeInfluence) != 0 ||
		prop.Get(gData.LGraph(), lopts.ConsiderModelOrderCrossingCounterNodeInfluence) != 0 {
		bestCrossings := math.MaxFloat64
		if prop.Get(gData.LGraph(), lopts.ConsiderModelOrderStrategy) != lenum.OrderingStrategyNone {
			// The first run should begin with a forward sweep. If the order
			// causes no additional crossings this preserves the current
			// order.
			prop.Set(gData.LGraph(), iprops.FirstTryWithInitialOrder, true)
		}
		thouroughness := prop.Get(gData.LGraph(), lopts.Thoroughness)
		for i := 0; i < thouroughness; i++ {
			crossings := m.minimizeCrossingsNodePortOrderWithCounter(gData)
			if crossings < bestCrossings {
				bestCrossings = crossings
				m.saveAllNodeOrdersOfChangedGraphs()
				if bestCrossings == 0 {
					break
				}
			}
		}
	} else {
		bestCrossings := 2147483647
		if prop.Get(gData.LGraph(), lopts.ConsiderModelOrderStrategy) != lenum.OrderingStrategyNone {
			// The first run should begin with a forward sweep. If the order
			// causes no additional crossings this preserves the current
			// order.
			prop.Set(gData.LGraph(), iprops.FirstTryWithInitialOrder, true)
		}
		thouroughness := prop.Get(gData.LGraph(), lopts.Thoroughness)
		for i := 0; i < thouroughness; i++ {
			crossings := m.minimizeCrossingsWithCounter(gData)
			if crossings < bestCrossings {
				bestCrossings = crossings
				m.saveAllNodeOrdersOfChangedGraphs()
				if bestCrossings == 0 {
					break
				}
			}
		}
	}
}

func (m *LayerSweepCrossingMinimizer) minimizeCrossingsWithCounter(gData *GraphInfoHolder) int {
	isForwardSweep := m.random.NextBoolean()

	// If the first, initial ordering is already optimal, do not change
	// anything. If the initial order should be tried, model order should be
	// active. So if the initial ordering has no crossings, just do not
	// change anything in the graph. Normally a first sweep should change
	// the order of not connected ports, which is bad.
	initialCrossings := m.countCurrentNumberOfCrossings(gData)
	if initialCrossings == 0 && prop.Get(gData.LGraph(), iprops.FirstTryWithInitialOrder) {
		// E.g. model order is already correct.
		return 0
	}

	if (!prop.Get(gData.LGraph(), iprops.FirstTryWithInitialOrder) &&
		!prop.Get(gData.LGraph(), iprops.SecondTryWithInitialOrder)) ||
		prop.Get(gData.LGraph(), lopts.ConsiderModelOrderStrategy) == lenum.OrderingStrategyNone {
		gData.CrossMinimizer().SetFirstLayerOrder(gData.CurrentNodeOrder(), isForwardSweep)
	} else {
		isForwardSweep = prop.Get(gData.LGraph(), iprops.FirstTryWithInitialOrder)
	}
	m.sweepReducingCrossings(gData, isForwardSweep, true)
	if prop.Get(gData.LGraph(), iprops.SecondTryWithInitialOrder) {
		prop.Set(gData.LGraph(), iprops.SecondTryWithInitialOrder, false)
	}
	if prop.Get(gData.LGraph(), iprops.FirstTryWithInitialOrder) {
		prop.Set(gData.LGraph(), iprops.FirstTryWithInitialOrder, false)
		prop.Set(gData.LGraph(), iprops.SecondTryWithInitialOrder, true)
	}
	crossingsInGraph := m.countCurrentNumberOfCrossings(gData)
	var oldNumberOfCrossings int
	for {
		m.setCurrentlyBestNodeOrders()

		if crossingsInGraph == 0 {
			return 0
		}

		isForwardSweep = !isForwardSweep
		oldNumberOfCrossings = crossingsInGraph
		m.sweepReducingCrossings(gData, isForwardSweep, false)
		crossingsInGraph = m.countCurrentNumberOfCrossings(gData)
		if !(oldNumberOfCrossings > crossingsInGraph) {
			break
		}
	}

	return oldNumberOfCrossings
}

func (m *LayerSweepCrossingMinimizer) minimizeCrossingsNodePortOrderWithCounter(gData *GraphInfoHolder) float64 {
	isForwardSweep := m.random.NextBoolean()

	// If the first, initial ordering is already optimal, do not change
	// anything. If the initial order should be tried, model order should be
	// active. So if the initial ordering has no crossings, just do not
	// change anything in the graph. Normally a first sweep should change
	// the order of not connected ports, which is bad.
	initialCrossings := m.countCurrentNumberOfCrossingsNodePortOrder(gData)
	if initialCrossings == 0 && prop.Get(gData.LGraph(), iprops.FirstTryWithInitialOrder) {
		// E.g. model order is already correct.
		return 0
	}

	if (!prop.Get(gData.LGraph(), iprops.FirstTryWithInitialOrder) &&
		!prop.Get(gData.LGraph(), iprops.SecondTryWithInitialOrder)) ||
		prop.Get(gData.LGraph(), lopts.ConsiderModelOrderStrategy) == lenum.OrderingStrategyNone {
		gData.CrossMinimizer().SetFirstLayerOrder(gData.CurrentNodeOrder(), isForwardSweep)
	} else {
		isForwardSweep = prop.Get(gData.LGraph(), iprops.FirstTryWithInitialOrder)
	}
	m.sweepReducingCrossings(gData, isForwardSweep, true)
	if prop.Get(gData.LGraph(), iprops.SecondTryWithInitialOrder) {
		prop.Set(gData.LGraph(), iprops.SecondTryWithInitialOrder, false)
	}
	if prop.Get(gData.LGraph(), iprops.FirstTryWithInitialOrder) {
		prop.Set(gData.LGraph(), iprops.FirstTryWithInitialOrder, false)
		prop.Set(gData.LGraph(), iprops.SecondTryWithInitialOrder, true)
	}
	crossingsInGraph := m.countCurrentNumberOfCrossingsNodePortOrder(gData)
	var oldNumberOfCrossings float64
	for {
		m.setCurrentlyBestNodeOrders()

		if crossingsInGraph == 0 {
			return 0
		}

		isForwardSweep = !isForwardSweep
		oldNumberOfCrossings = crossingsInGraph
		m.sweepReducingCrossings(gData, isForwardSweep, false)
		crossingsInGraph = m.countCurrentNumberOfCrossingsNodePortOrder(gData)
		if !(oldNumberOfCrossings > crossingsInGraph) {
			break
		}
	}

	return oldNumberOfCrossings
}

// countModelOrderNodeChanges compares all nodes of each layer and counts
// how often they are not in model order. The SortByInputModelProcessor must
// have run before.
func (m *LayerSweepCrossingMinimizer) countModelOrderNodeChanges(layers [][]*lgraph.LNode, strategy lenum.OrderingStrategy) int {
	previousLayer := -1
	wrongModelOrder := 0
	for _, layer := range layers {
		previous := layers[0]
		if previousLayer != -1 {
			previous = layers[previousLayer]
		}
		comp := preserveorder.NewModelOrderNodeComparatorOfNodes(previous, strategy, lenum.LongEdgeOrderingStrategyEqual)
		for i := 0; i < len(layer); i++ {
			for j := i + 1; j < len(layer); j++ {
				if prop.Has(layer[i], iprops.ModelOrder) && prop.Has(layer[j], iprops.ModelOrder) &&
					comp.Compare(layer[i], layer[j]) > 0 {
					wrongModelOrder++
				}
			}
		}
		previousLayer++
	}
	return wrongModelOrder
}

// countModelOrderPortChanges compares all ports of each node and counts how
// often they are not in model order. The SortByInputModelProcessor must
// have run before.
func (m *LayerSweepCrossingMinimizer) countModelOrderPortChanges(layers [][]*lgraph.LNode) int {
	previousLayer := -1
	wrongModelOrder := 0
	for _, layer := range layers {
		for _, lNode := range layer {
			previous := layers[0]
			if previousLayer != -1 {
				previous = layers[previousLayer]
			}
			comp := preserveorder.NewModelOrderPortComparatorOfNodes(previous,
				prop.Get(lNode.Graph(), lopts.ConsiderModelOrderStrategy),
				preserveorder.LongEdgeTargetNodePreprocessing(lNode),
				prop.Get(lNode.Graph(), lopts.ConsiderModelOrderPortModelOrder))
			for i := 0; i < len(lNode.Ports); i++ {
				for j := i + 1; j < len(lNode.Ports); j++ {
					if comp.Compare(lNode.Ports[i], lNode.Ports[j]) > 0 {
						wrongModelOrder++
					}
				}
			}
		}
		previousLayer++
	}
	return wrongModelOrder
}

// countCurrentNumberOfCrossings counts the crossings of the current graph
// and of the graphs below it that are processed hierarchically.
func (m *LayerSweepCrossingMinimizer) countCurrentNumberOfCrossings(currentGraph *GraphInfoHolder) int {
	totalCrossings := 0
	countCrossingsIn := []*GraphInfoHolder{currentGraph}
	for len(countCrossingsIn) != 0 {
		last := len(countCrossingsIn) - 1
		gD := countCrossingsIn[last]
		countCrossingsIn = countCrossingsIn[:last]
		totalCrossings += gD.CrossCounter().CountAllCrossings(gD.CurrentNodeOrder())
		for _, childLGraph := range gD.ChildGraphs() {
			child := m.graphInfoHolders[childLGraph.ID]
			if !child.DontSweepInto() {
				totalCrossings += m.countCurrentNumberOfCrossings(child)
			}
		}
	}

	return totalCrossings
}

// countCurrentNumberOfCrossingsNodePortOrder counts the crossings and the
// violations of the node and port order of the current graph, and the
// crossings of the graphs below it that are processed hierarchically.
func (m *LayerSweepCrossingMinimizer) countCurrentNumberOfCrossingsNodePortOrder(currentGraph *GraphInfoHolder) float64 {
	totalCrossings := 0.0
	countCrossingsIn := []*GraphInfoHolder{currentGraph}
	for len(countCrossingsIn) != 0 {
		last := len(countCrossingsIn) - 1
		gD := countCrossingsIn[last]
		countCrossingsIn = countCrossingsIn[:last]
		modelOrderInfluence := 0.0
		// Additionally add changes to the model order to the number of
		// crossings. The influence of port and node order can be
		// configured.
		modelOrderStrategy := prop.Get(currentGraph.LGraph(), lopts.ConsiderModelOrderStrategy)
		crossingCounterNodeInfluence := prop.Get(currentGraph.LGraph(), lopts.ConsiderModelOrderCrossingCounterNodeInfluence)
		crossingCounterPortInfluence := prop.Get(currentGraph.LGraph(), lopts.ConsiderModelOrderCrossingCounterPortInfluence)
		if modelOrderStrategy != lenum.OrderingStrategyNone {
			modelOrderInfluence += float64(crossingCounterNodeInfluence *
				float64(m.countModelOrderNodeChanges(gD.CurrentNodeOrder(), modelOrderStrategy)))
			modelOrderInfluence += float64(crossingCounterPortInfluence *
				float64(m.countModelOrderPortChanges(gD.CurrentNodeOrder())))
		}
		totalCrossings += float64(gD.CrossCounter().CountAllCrossings(gD.CurrentNodeOrder())) + modelOrderInfluence
		for _, childLGraph := range gD.ChildGraphs() {
			child := m.graphInfoHolders[childLGraph.ID]
			if !child.DontSweepInto() {
				totalCrossings += float64(m.countCurrentNumberOfCrossings(child))
			}
		}
	}

	return totalCrossings
}

func (m *LayerSweepCrossingMinimizer) sweepReducingCrossings(graph *GraphInfoHolder, forward, firstSweep bool) bool {
	nodes := graph.CurrentNodeOrder()
	length := len(nodes)

	// The Java operator |= evaluates its right side in each case.
	improved := graph.PortDistributor().DistributePortsWhileSweeping(nodes, m.firstIndex(forward, length), forward)
	firstLayer := nodes[m.firstIndex(forward, length)]
	swept := m.sweepInHierarchicalNodes(firstLayer, forward, firstSweep)
	improved = improved || swept
	for i := m.firstFree(forward, length); m.isNotEnd(length, i, forward); i += m.next(forward) {
		minimized := graph.CrossMinimizer().MinimizeCrossings(nodes, i, forward,
			firstSweep && !prop.Get(graph.LGraph(), iprops.FirstTryWithInitialOrder) &&
				!prop.Get(graph.LGraph(), iprops.SecondTryWithInitialOrder))
		improved = improved || minimized
		distributed := graph.PortDistributor().DistributePortsWhileSweeping(nodes, i, forward)
		improved = improved || distributed
		swept := m.sweepInHierarchicalNodes(nodes[i], forward, firstSweep)
		improved = improved || swept
	}

	m.graphsWhoseNodeOrderChanged.Add(graph)
	return improved
}

func (m *LayerSweepCrossingMinimizer) sweepInHierarchicalNodes(layer []*lgraph.LNode, isForwardSweep, isFirstSweep bool) bool {
	improved := false
	for _, node := range layer {
		if m.hasNestedGraph(node) && !m.graphInfoHolders[node.NestedGraph.ID].DontSweepInto() {
			swept := m.sweepInHierarchicalNode(isForwardSweep, node, isFirstSweep)
			improved = improved || swept
		}
	}
	return improved
}

func (m *LayerSweepCrossingMinimizer) sweepInHierarchicalNode(isForwardSweep bool, node *lgraph.LNode, isFirstSweep bool) bool {
	nestedLGraph := node.NestedGraph
	nestedGraph := m.graphInfoHolders[nestedLGraph.ID]
	nestedGraphNodeOrder := nestedGraph.CurrentNodeOrder()
	startIndex := m.firstIndex(isForwardSweep, len(nestedGraphNodeOrder))
	firstNode := nestedGraphNodeOrder[startIndex][0]

	if m.isExternalPortDummy(firstNode) {
		nestedGraphNodeOrder[startIndex] = m.sortPortDummiesByPortPositions(node,
			nestedGraphNodeOrder[startIndex], m.sideOpposedSweepDirection(isForwardSweep))
	} else {
		nestedGraph.CrossMinimizer().SetFirstLayerOrder(nestedGraphNodeOrder, isForwardSweep)
	}

	improved := m.sweepReducingCrossings(nestedGraph, isForwardSweep, isFirstSweep)
	m.sortPortsByDummyPositionsInLastLayer(nestedGraph.CurrentNodeOrder(), nestedGraph.Parent(), isForwardSweep)

	return improved
}

func (m *LayerSweepCrossingMinimizer) sortPortsByDummyPositionsInLastLayer(nodeOrder [][]*lgraph.LNode, parent *lgraph.LNode, onRightMostLayer bool) {
	endIndex := m.endIndex(onRightMostLayer, len(nodeOrder))
	lastLayer := nodeOrder[endIndex]
	// Check whether the node to check is an external port dummy.
	j := m.firstIndex(onRightMostLayer, len(lastLayer))
	if !m.isExternalPortDummy(lastLayer[j]) {
		return
	}

	ports := parent.Ports
	for i := 0; i < len(ports); i++ {
		port := ports[i]
		if m.isOnEndOfSweepSide(port, onRightMostLayer) && m.isHierarchical(port) {
			// Only on external port dummy node has a port as its origin.
			ports[i] = m.originPort(lastLayer[j])
			j += m.next(onRightMostLayer)
		}
	}
}

func (m *LayerSweepCrossingMinimizer) sortPortDummiesByPortPositions(parentNode *lgraph.LNode, layerCloseToNodeEdge []*lgraph.LNode, side core.PortSide) []*lgraph.LNode {
	ports := counting.InNorthSouthEastWestOrder(parentNode, side)

	sortedDummies := make([]*lgraph.LNode, len(layerCloseToNodeEdge))

	i := 0
	for _, port := range ports {
		if m.isHierarchical(port) {
			sortedDummies[i] = m.dummyNodeFor(port)
			i++
		}
	}

	if i < len(layerCloseToNodeEdge) {
		lgraph.Bail("Expected %d hierarchical ports, but found only %d.", len(layerCloseToNodeEdge), i)
	}
	return sortedDummies
}

func (m *LayerSweepCrossingMinimizer) saveAllNodeOrdersOfChangedGraphs() {
	for _, graph := range m.graphsWhoseNodeOrderChanged.Values() {
		graph.SetBestNodeNPortOrder(NewSweepCopyOf(graph.CurrentlyBestNodeAndPortOrder()))
	}
}

func (m *LayerSweepCrossingMinimizer) setCurrentlyBestNodeOrders() {
	for _, graph := range m.graphsWhoseNodeOrderChanged.Values() {
		graph.SetCurrentlyBestNodeAndPortOrder(NewSweepCopy(graph.CurrentNodeOrder()))
	}
}

func (m *LayerSweepCrossingMinimizer) firstIndex(isForwardSweep bool, length int) int {
	if isForwardSweep {
		return 0
	}
	return length - 1
}

func (m *LayerSweepCrossingMinimizer) endIndex(isForwardSweep bool, length int) int {
	if isForwardSweep {
		return length - 1
	}
	return 0
}

func (m *LayerSweepCrossingMinimizer) firstFree(isForwardSweep bool, length int) int {
	if isForwardSweep {
		return 1
	}
	return length - 2
}

func (m *LayerSweepCrossingMinimizer) next(isForwardSweep bool) int {
	if isForwardSweep {
		return 1
	}
	return -1
}

func (m *LayerSweepCrossingMinimizer) isNotEnd(length, freeLayerIndex int, isForwardSweep bool) bool {
	if isForwardSweep {
		return freeLayerIndex < length
	}
	return freeLayerIndex >= 0
}

func (m *LayerSweepCrossingMinimizer) hasNestedGraph(node *lgraph.LNode) bool {
	return node.NestedGraph != nil
}

func (m *LayerSweepCrossingMinimizer) sideOpposedSweepDirection(isForwardSweep bool) core.PortSide {
	if isForwardSweep {
		return core.PortSideWest
	}
	return core.PortSideEast
}

func (m *LayerSweepCrossingMinimizer) isExternalPortDummy(firstNode *lgraph.LNode) bool {
	return firstNode.Type == lgraph.NodeTypeExternalPort
}

func (m *LayerSweepCrossingMinimizer) originPort(node *lgraph.LNode) *lgraph.LPort {
	return counting.OriginPort(node)
}

func (m *LayerSweepCrossingMinimizer) isHierarchical(port *lgraph.LPort) bool {
	return prop.Get(port, iprops.InsideConnections)
}

func (m *LayerSweepCrossingMinimizer) dummyNodeFor(port *lgraph.LPort) *lgraph.LNode {
	return prop.Get(port, iprops.PortDummy)
}

func (m *LayerSweepCrossingMinimizer) isOnEndOfSweepSide(port *lgraph.LPort, isForwardSweep bool) bool {
	if isForwardSweep {
		return port.Side == core.PortSideEast
	}
	return port.Side == core.PortSideWest
}

// initialize traverses the inclusion tree breadth-first and initializes
// each graph. It sets the id of each graph, and it draws from the random
// number generator of the root graph and of each graph.
func (m *LayerSweepCrossingMinimizer) initialize(rootGraph *lgraph.LGraph) []*GraphInfoHolder {
	m.graphInfoHolders = nil
	m.random = prop.Get(rootGraph, iprops.Random)
	m.randomSeed = m.random.NextLong()
	var graphsToSweepOn []*GraphInfoHolder
	graphs := []*lgraph.LGraph{rootGraph}
	i := 0
	for i < len(graphs) {
		graph := graphs[i]
		graph.ID = i
		i++
		gData := NewGraphInfoHolder(graph, m.crossMinType, m.graphInfoHolders)
		graphs = append(graphs, gData.ChildGraphs()...)
		m.graphInfoHolders = append(m.graphInfoHolders, gData)
		if gData.DontSweepInto() {
			// graphsToSweepOn.add(0, gData)
			graphsToSweepOn = append(graphsToSweepOn, nil)
			copy(graphsToSweepOn[1:], graphsToSweepOn)
			graphsToSweepOn[0] = gData
		}
	}

	m.graphsWhoseNodeOrderChanged = gwt.NewSet[*GraphInfoHolder]()

	return graphsToSweepOn
}

func (m *LayerSweepCrossingMinimizer) transferNodeAndPortOrdersToGraph() {
	for _, gD := range m.graphInfoHolders {
		bestSweep := gD.GetBestSweep()
		if bestSweep != nil {
			bestSweep.TransferNodeAndPortOrdersToGraph(gD.LGraph(), true)
		}
	}
}

// GraphData is getGraphData().
func (m *LayerSweepCrossingMinimizer) GraphData() []*GraphInfoHolder {
	return m.graphInfoHolders
}
