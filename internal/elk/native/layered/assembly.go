// Ported from GraphConfigurator.java, LayeredPhases.java and
// intermediate/IntermediateProcessorStrategy.java in
// plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered, from
// the getLayoutProcessorConfiguration methods of the phase classes there, and
// from AlgorithmAssembler.java and LayoutProcessorConfiguration.java in
// plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/alg of Eclipse ELK
// 0.9.1 (EPL-2.0).

// Package layered has the driver of the layered algorithm of ELK: the import
// of an ELK graph into an LGraph, the list of processors for a graph, the
// run of the processors over a graph or a hierarchy of graphs, and the
// transfer of the layout back to the ELK graph.
package layered

import (
	"sort"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// strategy is a constant of IntermediateProcessorStrategy. The order of the
// constants is the order of the processors between two phases.
type strategy int

const (
	directionPreprocessor strategy = iota
	commentPreprocessor
	edgeAndLayerConstraintEdgeReverser
	interactiveExternalPortPositioner
	partitionPreprocessor
	labelDummyInserter
	selfLoopPreprocessor
	layerConstraintPreprocessor
	partitionMidprocessor
	highDegreeNodeLayerProcessor
	nodePromotion
	layerConstraintPostprocessor
	partitionPostprocessor
	hierarchicalPortConstraintProcessor
	semiInteractiveCrossminProcessor
	breakingPointInserter
	longEdgeSplitter
	portSideProcessor
	invertedPortProcessor
	portListSorter
	sortByInputOrderOfModel
	northSouthPortPreprocessor
	breakingPointProcessor
	oneSidedGreedySwitch
	twoSidedGreedySwitch
	selfLoopPortRestorer
	singleEdgeGraphWrapper
	inLayerConstraintProcessor
	endNodePortLabelManagementProcessor
	labelAndNodeSizeProcessor
	innermostNodeMarginCalculator
	selfLoopRouter
	commentNodeMarginCalculator
	endLabelPreprocessor
	labelDummySwitcher
	centerLabelManagementProcessor
	labelSideSelector
	hyperedgeDummyMerger
	hierarchicalPortDummySizeProcessor
	layerSizeAndGraphHeightCalculator
	hierarchicalPortPositionProcessor
	constraintsPostprocessor
	commentPostprocessor
	hypernodeProcessor
	hierarchicalPortOrthogonalEdgeRouter
	longEdgeJoiner
	selfLoopPostprocessor
	breakingPointRemover
	northSouthPortPostprocessor
	horizontalCompactor
	labelDummyRemover
	finalSplineBendpointsCalculator
	endLabelSorter
	reversedEdgeRestorer
	endLabelPostprocessor
	hierarchicalNodeResizer
	directionPostprocessor
)

// strategyProcessor is IntermediateProcessorStrategy.create: the name under
// which the processor of a strategy is registered. It is the simple name of
// the Java class, with the constructor argument after a colon where one
// class serves several strategies.
var strategyProcessor = map[strategy]string{
	directionPreprocessor:                "GraphTransformer:TO_INPUT_DIRECTION",
	commentPreprocessor:                  "CommentPreprocessor",
	edgeAndLayerConstraintEdgeReverser:   "EdgeAndLayerConstraintEdgeReverser",
	interactiveExternalPortPositioner:    "InteractiveExternalPortPositioner",
	partitionPreprocessor:                "PartitionPreprocessor",
	labelDummyInserter:                   "LabelDummyInserter",
	selfLoopPreprocessor:                 "SelfLoopPreProcessor",
	layerConstraintPreprocessor:          "LayerConstraintPreprocessor",
	partitionMidprocessor:                "PartitionMidprocessor",
	highDegreeNodeLayerProcessor:         "HighDegreeNodeLayeringProcessor",
	nodePromotion:                        "NodePromotion",
	layerConstraintPostprocessor:         "LayerConstraintPostprocessor",
	partitionPostprocessor:               "PartitionPostprocessor",
	hierarchicalPortConstraintProcessor:  "HierarchicalPortConstraintProcessor",
	semiInteractiveCrossminProcessor:     "SemiInteractiveCrossMinProcessor",
	breakingPointInserter:                "BreakingPointInserter",
	longEdgeSplitter:                     "LongEdgeSplitter",
	portSideProcessor:                    "PortSideProcessor",
	invertedPortProcessor:                "InvertedPortProcessor",
	portListSorter:                       "PortListSorter",
	sortByInputOrderOfModel:              "SortByInputModelProcessor",
	northSouthPortPreprocessor:           "NorthSouthPortPreprocessor",
	breakingPointProcessor:               "BreakingPointProcessor",
	oneSidedGreedySwitch:                 "LayerSweepCrossingMinimizer:ONE_SIDED_GREEDY_SWITCH",
	twoSidedGreedySwitch:                 "LayerSweepCrossingMinimizer:TWO_SIDED_GREEDY_SWITCH",
	selfLoopPortRestorer:                 "SelfLoopPortRestorer",
	singleEdgeGraphWrapper:               "SingleEdgeGraphWrapper",
	inLayerConstraintProcessor:           "InLayerConstraintProcessor",
	endNodePortLabelManagementProcessor:  "LabelManagementProcessor:false",
	labelAndNodeSizeProcessor:            "LabelAndNodeSizeProcessor",
	innermostNodeMarginCalculator:        "InnermostNodeMarginCalculator",
	selfLoopRouter:                       "SelfLoopRouter",
	commentNodeMarginCalculator:          "CommentNodeMarginCalculator",
	endLabelPreprocessor:                 "EndLabelPreprocessor",
	labelDummySwitcher:                   "LabelDummySwitcher",
	centerLabelManagementProcessor:       "LabelManagementProcessor:true",
	labelSideSelector:                    "LabelSideSelector",
	hyperedgeDummyMerger:                 "HyperedgeDummyMerger",
	hierarchicalPortDummySizeProcessor:   "HierarchicalPortDummySizeProcessor",
	layerSizeAndGraphHeightCalculator:    "LayerSizeAndGraphHeightCalculator",
	hierarchicalPortPositionProcessor:    "HierarchicalPortPositionProcessor",
	constraintsPostprocessor:             "ConstraintsPostprocessor",
	commentPostprocessor:                 "CommentPostprocessor",
	hypernodeProcessor:                   "HypernodesProcessor",
	hierarchicalPortOrthogonalEdgeRouter: "HierarchicalPortOrthogonalEdgeRouter",
	longEdgeJoiner:                       "LongEdgeJoiner",
	selfLoopPostprocessor:                "SelfLoopPostProcessor",
	breakingPointRemover:                 "BreakingPointRemover",
	northSouthPortPostprocessor:          "NorthSouthPortPostprocessor",
	horizontalCompactor:                  "HorizontalGraphCompactor",
	labelDummyRemover:                    "LabelDummyRemover",
	finalSplineBendpointsCalculator:      "FinalSplineBendpointsCalculator",
	endLabelSorter:                       "EndLabelSorter",
	reversedEdgeRestorer:                 "ReversedEdgeRestorer",
	endLabelPostprocessor:                "EndLabelPostprocessor",
	hierarchicalNodeResizer:              "HierarchicalNodeResizingProcessor",
	directionPostprocessor:               "GraphTransformer:TO_INTERNAL_LTR",
}

// The slots of a LayoutProcessorConfiguration: before each of the five
// phases, and after the last one.
const (
	beforeP1 = iota
	beforeP2
	beforeP3
	beforeP4
	beforeP5
	afterP5
	slotCount
)

// configuration is LayoutProcessorConfiguration: the intermediate processors
// of each slot.
type configuration [slotCount]map[strategy]bool

func (c *configuration) add(slot int, s strategy) {
	if c[slot] == nil {
		c[slot] = map[strategy]bool{}
	}
	c[slot][s] = true
}

// The names of the phase classes by strategy.
var (
	cycleBreakers = map[lenum.CycleBreakingStrategy]string{
		lenum.CycleBreakingStrategyGreedy:           "GreedyCycleBreaker",
		lenum.CycleBreakingStrategyDepthFirst:       "DepthFirstCycleBreaker",
		lenum.CycleBreakingStrategyInteractive:      "InteractiveCycleBreaker",
		lenum.CycleBreakingStrategyModelOrder:       "ModelOrderCycleBreaker",
		lenum.CycleBreakingStrategyGreedyModelOrder: "GreedyModelOrderCycleBreaker",
	}
	layerers = map[lenum.LayeringStrategy]string{
		lenum.LayeringStrategyNetworkSimplex:    "NetworkSimplexLayerer",
		lenum.LayeringStrategyLongestPath:       "LongestPathLayerer",
		lenum.LayeringStrategyLongestPathSource: "LongestPathSourceLayerer",
		lenum.LayeringStrategyCoffmanGraham:     "CoffmanGrahamLayerer",
		lenum.LayeringStrategyInteractive:       "InteractiveLayerer",
		lenum.LayeringStrategyStretchWidth:      "StretchWidthLayerer",
		lenum.LayeringStrategyMinWidth:          "MinWidthLayerer",
		lenum.LayeringStrategyBfModelOrder:      "BreadthFirstModelOrderLayerer",
		lenum.LayeringStrategyDfModelOrder:      "DepthFirstModelOrderLayerer",
	}
	crossingMinimizers = map[lenum.CrossingMinimizationStrategy]string{
		lenum.CrossingMinimizationStrategyLayerSweep:  "LayerSweepCrossingMinimizer",
		lenum.CrossingMinimizationStrategyInteractive: "InteractiveCrossingMinimizer",
		lenum.CrossingMinimizationStrategyNone:        "NoCrossingMinimizer",
	}
	nodePlacers = map[lenum.NodePlacementStrategy]string{
		lenum.NodePlacementStrategySimple:         "SimpleNodePlacer",
		lenum.NodePlacementStrategyInteractive:    "InteractiveNodePlacer",
		lenum.NodePlacementStrategyLinearSegments: "LinearSegmentsNodePlacer",
		lenum.NodePlacementStrategyBrandesKoepf:   "BKNodePlacer",
		lenum.NodePlacementStrategyNetworkSimplex: "NetworkSimplexPlacer",
	}
)

// phaseConfiguration is getLayoutProcessorConfiguration of a phase class: the
// intermediate processors that the phase needs for the graph. It stops the
// layout for a phase whose configuration is not ported.
func phaseConfiguration(c *configuration, phase string, graph *lgraph.LGraph) {
	gp := prop.Get(graph, iprops.GraphProperties)
	switch phase {
	case "GreedyCycleBreaker", "DepthFirstCycleBreaker", "ModelOrderCycleBreaker", "GreedyModelOrderCycleBreaker":
		c.add(afterP5, reversedEdgeRestorer)
	case "InteractiveCycleBreaker":
		c.add(beforeP1, interactiveExternalPortPositioner)
		c.add(afterP5, reversedEdgeRestorer)
	case "NetworkSimplexLayerer", "LongestPathLayerer", "LongestPathSourceLayerer", "CoffmanGrahamLayerer", "MinWidthLayerer", "StretchWidthLayerer":
		c.add(beforeP1, edgeAndLayerConstraintEdgeReverser)
		c.add(beforeP2, layerConstraintPreprocessor)
		c.add(beforeP3, layerConstraintPostprocessor)
	case "LayerSweepCrossingMinimizer", "NoCrossingMinimizer":
		c.add(beforeP3, longEdgeSplitter)
		c.add(beforeP4, inLayerConstraintProcessor)
		c.add(afterP5, longEdgeJoiner)
		c.add(beforeP3, portListSorter)
	case "BKNodePlacer", "NetworkSimplexPlacer", "LinearSegmentsNodePlacer", "SimpleNodePlacer":
		if gp.Contains(lenum.GraphPropertiesExternalPorts) {
			c.add(beforeP5, hierarchicalPortPositionProcessor)
		}
	case "OrthogonalEdgeRouter":
		if gp.Contains(lenum.GraphPropertiesHyperedges) {
			c.add(beforeP4, hyperedgeDummyMerger)
			c.add(beforeP3, invertedPortProcessor)
		}
		if gp.Contains(lenum.GraphPropertiesNonFreePorts) || prop.Get(graph, lopts.FeedbackEdges) {
			c.add(beforeP3, invertedPortProcessor)
			if gp.Contains(lenum.GraphPropertiesNorthSouthPorts) {
				c.add(beforeP3, northSouthPortPreprocessor)
				c.add(afterP5, northSouthPortPostprocessor)
			}
		}
		if gp.Contains(lenum.GraphPropertiesExternalPorts) {
			c.add(beforeP3, hierarchicalPortConstraintProcessor)
			c.add(beforeP4, hierarchicalPortDummySizeProcessor)
			c.add(afterP5, hierarchicalPortOrthogonalEdgeRouter)
		}
		if gp.Contains(lenum.GraphPropertiesSelfLoops) {
			c.add(beforeP1, selfLoopPreprocessor)
			c.add(afterP5, selfLoopPostprocessor)
			c.add(beforeP4, selfLoopPortRestorer)
			c.add(beforeP4, selfLoopRouter)
		}
		if gp.Contains(lenum.GraphPropertiesHypernodes) {
			c.add(afterP5, hypernodeProcessor)
		}
		if gp.Contains(lenum.GraphPropertiesCenterLabels) {
			c.add(beforeP2, labelDummyInserter)
			c.add(beforeP4, labelDummySwitcher)
			c.add(beforeP4, labelSideSelector)
			c.add(afterP5, labelDummyRemover)
		}
		if gp.Contains(lenum.GraphPropertiesEndLabels) {
			c.add(beforeP4, labelSideSelector)
			c.add(beforeP4, endLabelPreprocessor)
			c.add(afterP5, endLabelPostprocessor)
		}
	default:
		lgraph.Bail("the processor configuration of the phase %s is not ported", phase)
	}
}

// activateGreedySwitchFor is GraphConfigurator.activateGreedySwitchFor.
func activateGreedySwitchFor(graph *lgraph.LGraph) bool {
	if isHierarchicalLayout(graph) {
		return graph.ParentNode == nil && prop.Get(graph, lopts.CrossingMinimizationGreedySwitchHierarchicalType) != lenum.GreedySwitchTypeOff
	}
	greedySwitchType := prop.Get(graph, lopts.CrossingMinimizationGreedySwitchType)
	interactiveCrossMin := prop.Get(graph, lopts.CrossingMinimizationSemiInteractive) ||
		prop.Get(graph, lopts.CrossingMinimizationStrategy) == lenum.CrossingMinimizationStrategyInteractive
	activationThreshold := prop.Get(graph, lopts.CrossingMinimizationGreedySwitchActivationThreshold)
	graphSize := len(graph.LayerlessNodes)
	return !interactiveCrossMin && greedySwitchType != lenum.GreedySwitchTypeOff && (activationThreshold == 0 || activationThreshold > graphSize)
}

func isHierarchicalLayout(graph *lgraph.LGraph) bool {
	return prop.Get(graph, lopts.HierarchyHandling) == core.HierarchyHandlingIncludeChildren
}

// phaseIndependentConfiguration is
// GraphConfigurator.getPhaseIndependentLayoutProcessorConfiguration.
func phaseIndependentConfiguration(c *configuration, graph *lgraph.LGraph) {
	gp := prop.Get(graph, iprops.GraphProperties)

	// The baseline configuration.
	c.add(beforeP4, innermostNodeMarginCalculator)
	c.add(beforeP4, labelAndNodeSizeProcessor)
	c.add(beforeP5, layerSizeAndGraphHeightCalculator)
	c.add(afterP5, endLabelSorter)

	if prop.Get(graph, lopts.HierarchyHandling) == core.HierarchyHandlingIncludeChildren {
		c.add(afterP5, hierarchicalNodeResizer)
	}

	// port side processor, put to first slot only if requested and routing
	// is orthogonal
	if prop.Get(graph, lopts.FeedbackEdges) {
		c.add(beforeP1, portSideProcessor)
	} else {
		c.add(beforeP3, portSideProcessor)
	}

	// The label manager is never set in elkjs.

	if prop.Get(graph, lopts.InteractiveLayout) || prop.Get(graph, lopts.GeneratePositionAndLayerIds) {
		c.add(afterP5, constraintsPostprocessor)
	}

	// graph transformations for unusual layout directions
	switch prop.Get(graph, lopts.Direction) {
	case core.DirectionLeft, core.DirectionDown, core.DirectionUp:
		c.add(beforeP1, directionPreprocessor)
		c.add(afterP5, directionPostprocessor)
	}

	// Additional dependencies
	if gp.Contains(lenum.GraphPropertiesComments) {
		c.add(beforeP1, commentPreprocessor)
		c.add(beforeP4, commentNodeMarginCalculator)
		c.add(afterP5, commentPostprocessor)
	}

	// Node-Promotion application for reduction of dummy nodes after layering
	if prop.Get(graph, lopts.LayeringNodePromotionStrategy) != lenum.NodePromotionStrategyNone {
		c.add(beforeP3, nodePromotion)
	}

	// Preserve certain partitions during layering
	if gp.Contains(lenum.GraphPropertiesPartitions) {
		c.add(beforeP1, partitionPreprocessor)
		c.add(beforeP2, partitionMidprocessor)
		c.add(beforeP3, partitionPostprocessor)
	}

	// Additional horizontal compaction depends on orthogonal edge routing
	if prop.Get(graph, lopts.CompactionPostCompactionStrategy) != lenum.GraphCompactionStrategyNone &&
		prop.Get(graph, lopts.EdgeRouting) != core.EdgeRoutingPolyline {
		c.add(afterP5, horizontalCompactor)
	}

	// Move trees of high degree nodes to separate layers
	if prop.Get(graph, lopts.HighDegreeNodesTreatment) {
		c.add(beforeP3, highDegreeNodeLayerProcessor)
	}

	// Introduce in-layer constraints to preserve the order of regular nodes
	if prop.Get(graph, lopts.CrossingMinimizationSemiInteractive) {
		c.add(beforeP3, semiInteractiveCrossminProcessor)
	}

	// Configure greedy switch: for a hierarchical layout only the root graph
	// can activate it
	if activateGreedySwitchFor(graph) {
		var greedySwitchType lenum.GreedySwitchType
		if isHierarchicalLayout(graph) {
			greedySwitchType = prop.Get(graph, lopts.CrossingMinimizationGreedySwitchHierarchicalType)
		} else {
			greedySwitchType = prop.Get(graph, lopts.CrossingMinimizationGreedySwitchType)
		}
		if greedySwitchType == lenum.GreedySwitchTypeOneSided {
			c.add(beforeP4, oneSidedGreedySwitch)
		} else {
			c.add(beforeP4, twoSidedGreedySwitch)
		}
	}

	// Wrapping of graphs
	switch prop.Get(graph, lopts.WrappingStrategy) {
	case lenum.WrappingStrategySingleEdge:
		c.add(beforeP4, singleEdgeGraphWrapper)
	case lenum.WrappingStrategyMultiEdge:
		c.add(beforeP3, breakingPointInserter)
		c.add(beforeP4, breakingPointProcessor)
		c.add(afterP5, breakingPointRemover)
	}

	if prop.Get(graph, lopts.ConsiderModelOrderStrategy) != lenum.OrderingStrategyNone {
		c.add(beforeP3, sortByInputOrderOfModel)
	}
}

// phaseNames returns the names of the five phase classes of a graph.
func phaseNames(graph *lgraph.LGraph) [5]string {
	var router string
	switch prop.Get(graph, lopts.EdgeRouting) {
	case core.EdgeRoutingPolyline:
		router = "PolylineEdgeRouter"
	case core.EdgeRoutingSplines:
		router = "SplineEdgeRouter"
	default:
		router = "OrthogonalEdgeRouter"
	}
	return [5]string{
		cycleBreakers[prop.Get(graph, lopts.CycleBreakingStrategy)],
		layerers[prop.Get(graph, lopts.LayeringStrategy)],
		crossingMinimizers[prop.Get(graph, lopts.CrossingMinimizationStrategy)],
		nodePlacers[prop.Get(graph, lopts.NodePlacementStrategy)],
		router,
	}
}

// ProcessorNames is the processor list of AlgorithmAssembler.build for a
// graph, as registry names: before each phase its intermediate processors in
// the order of IntermediateProcessorStrategy, then the phase.
func ProcessorNames(graph *lgraph.LGraph) []string {
	phases := phaseNames(graph)
	var c configuration
	for _, phase := range phases {
		phaseConfiguration(&c, phase, graph)
	}
	phaseIndependentConfiguration(&c, graph)

	var names []string
	slot := func(i int) {
		strategies := make([]int, 0, len(c[i]))
		for s := range c[i] {
			strategies = append(strategies, int(s))
		}
		sort.Ints(strategies)
		for _, s := range strategies {
			names = append(names, strategyProcessor[strategy(s)])
		}
	}
	for i, phase := range phases {
		slot(i)
		names = append(names, phase)
	}
	slot(afterP5)
	return names
}
