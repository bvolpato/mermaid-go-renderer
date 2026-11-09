// Ported from ElkLayered.java and GraphConfigurator.java in
// plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered of
// Eclipse ELK 0.9.1 (EPL-2.0).

package layered

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// minEdgeSpacing is GraphConfigurator.MIN_EDGE_SPACING.
const minEdgeSpacing = 2.0

// ElkLayered is the class ElkLayered with its GraphConfigurator: one run of
// the layered algorithm. It keeps one processor of each kind, as the
// AlgorithmAssembler of ELK does.
type ElkLayered struct {
	ctx   *lgraph.Context
	cache map[string]lgraph.Processor
	// Observer, when set, is called before and after each step of the run.
	// The tests use it to compare the states with elkjs.
	Observer func(event StepEvent)
}

// StepEvent describes a step of a run for ElkLayered.Observer.
type StepEvent struct {
	// Name is the registry name of the processor, or "GraphConfigurator",
	// "CompoundGraphPreprocessor", "CompoundGraphPostprocessor",
	// "ComponentsProcessor.split" or "ComponentsProcessor.combine".
	Name string
	// Graph is the graph that the step processes.
	Graph *lgraph.LGraph
	// Done is false before the step and true after it.
	Done bool
}

// NewElkLayered returns the algorithm for one run.
func NewElkLayered(ctx *lgraph.Context) *ElkLayered {
	return &ElkLayered{ctx: ctx, cache: map[string]lgraph.Processor{}}
}

func (e *ElkLayered) observe(name string, graph *lgraph.LGraph, done bool) {
	if e.Observer != nil {
		e.Observer(StepEvent{Name: name, Graph: graph, Done: done})
	}
}

// processor returns the processor with a registry name. It stops the layout
// when no linked package ports the processor.
func (e *ElkLayered) processor(name string) lgraph.Processor {
	if p, ok := e.cache[name]; ok {
		return p
	}
	var p lgraph.Processor
	if factory := lgraph.Lookup(name); factory != nil {
		p = factory()
	} else {
		// The layout stops when it reaches the processor. The steps before
		// it still run, so that the tests can compare them with elkjs.
		p = missingProcessor(name)
	}
	e.cache[name] = p
	return p
}

// missingProcessor stands for a processor that no linked package ports.
type missingProcessor string

// Process stops the layout.
func (m missingProcessor) Process(*lgraph.LGraph, *lgraph.Context) {
	lgraph.Bail("the processor %s is not ported", string(m))
}

// ConfigureGraphProperties is GraphConfigurator.configureGraphProperties.
func ConfigureGraphProperties(graph *lgraph.LGraph, ctx *lgraph.Context) {
	// check the bounds of some layout options
	if edgeSpacing := prop.Get(graph, lopts.SpacingEdgeEdge); edgeSpacing < minEdgeSpacing {
		prop.Set(graph, lopts.SpacingEdgeEdge, minEdgeSpacing)
	}

	if prop.Get(graph, lopts.Direction) == core.DirectionUndefined {
		prop.Set(graph, lopts.Direction, lgraph.GetDirection(graph))
	}

	// set the random number generator based on the random seed option
	randomSeed := prop.Get(graph, lopts.RandomSeed)
	if randomSeed == 0 {
		prop.Set(graph, iprops.Random, ctx.NewRandom())
	} else {
		prop.Set(graph, iprops.Random, gwt.NewRandom(int64(randomSeed)))
	}

	if !prop.Has(graph, lopts.NodePlacementFavorStraightEdges) {
		prop.Set(graph, lopts.NodePlacementFavorStraightEdges, prop.Get(graph, lopts.EdgeRouting) == core.EdgeRoutingOrthogonal)
	}

	// copy the port constraints to keep a list of original port constraints
	copyPortConstraintsOfGraph(graph)

	// pre-calculate spacing information
	prop.Set(graph, iprops.Spacings, lgraph.NewSpacings(graph))
}

func copyPortConstraintsOfGraph(graph *lgraph.LGraph) {
	for _, node := range graph.LayerlessNodes {
		copyPortConstraints(node)
	}
	for _, layer := range graph.Layers {
		for _, node := range layer.Nodes {
			copyPortConstraints(node)
		}
	}
}

func copyPortConstraints(node *lgraph.LNode) {
	prop.Set(node, iprops.OriginalPortConstraints, prop.Get(node, lopts.PortConstraints))
	if nested := node.NestedGraph; nested != nil {
		copyPortConstraintsOfGraph(nested)
	}
}

// prepareGraphForLayout is GraphConfigurator.prepareGraphForLayout. It
// returns the processors of the graph. The Java code keeps them in the
// property processors, which no processor reads.
func (e *ElkLayered) prepareGraphForLayout(graph *lgraph.LGraph) []lgraph.Processor {
	e.observe("GraphConfigurator", graph, false)
	ConfigureGraphProperties(graph, e.ctx)
	names := ProcessorNames(graph)
	processors := make([]lgraph.Processor, len(names))
	for i, name := range names {
		processors[i] = e.processor(name)
	}
	e.observe("GraphConfigurator", graph, true)
	return processors
}

// DoLayout is doLayout(lgraph, monitor): the layout of a graph without
// hierarchy.
func (e *ElkLayered) DoLayout(graph *lgraph.LGraph) {
	processors := e.prepareGraphForLayout(graph)
	names := ProcessorNames(graph)

	// split the input graph into components and perform layout on them
	e.observe("ComponentsProcessor.split", graph, false)
	components := SplitComponents(graph)
	e.observe("ComponentsProcessor.split", graph, true)
	for _, comp := range components {
		e.layout(comp, processors, names)
	}
	e.observe("ComponentsProcessor.combine", graph, false)
	CombineComponents(components, graph)
	e.observe("ComponentsProcessor.combine", graph, true)
	resizeGraph(graph)
}

// layout is layout(lgraph, monitor): the processors of one component.
func (e *ElkLayered) layout(graph *lgraph.LGraph, processors []lgraph.Processor, names []string) {
	for i, p := range processors {
		e.observe(names[i], graph, false)
		p.Process(graph, e.ctx)
		e.observe(names[i], graph, true)
	}

	// Move all nodes away from the layers (we need to remove layered nodes
	// from their layers, then reset their layer references)
	for _, layer := range graph.Layers {
		graph.LayerlessNodes = append(graph.LayerlessNodes, layer.Nodes...)
		layer.Nodes = layer.Nodes[:0]
	}
	for _, node := range graph.LayerlessNodes {
		node.SetLayer(nil)
	}
	graph.Layers = graph.Layers[:0]
}

// DoCompoundLayout is doCompoundLayout(lgraph, monitor): the layout of a
// graph with its nested graphs.
func (e *ElkLayered) DoCompoundLayout(graph *lgraph.LGraph) {
	// Preprocess the compound graph by splitting cross-hierarchy edges
	pre := e.processor("CompoundGraphPreprocessor")
	e.observe("CompoundGraphPreprocessor", graph, false)
	pre.Process(graph, e.ctx)
	e.observe("CompoundGraphPreprocessor", graph, true)

	e.hierarchicalLayout(graph)

	// Postprocess the compound graph by combining split cross-hierarchy
	// edges
	post := e.processor("CompoundGraphPostprocessor")
	e.observe("CompoundGraphPostprocessor", graph, false)
	post.Process(graph, e.ctx)
	e.observe("CompoundGraphPostprocessor", graph, true)
}

// collectAllGraphsBottomUp is collectAllGraphsBottomUp(root): the graphs of
// the hierarchy with each nested graph before its parent.
func collectAllGraphsBottomUp(root *lgraph.LGraph) []*lgraph.LGraph {
	// Both Java collections are deques that grow at the front.
	collected := []*lgraph.LGraph{root}
	pending := []*lgraph.LGraph{root}
	for len(pending) > 0 {
		next := pending[0]
		pending = pending[1:]
		for _, node := range next.LayerlessNodes {
			if nested := node.NestedGraph; nested != nil {
				collected = append([]*lgraph.LGraph{nested}, collected...)
				pending = append([]*lgraph.LGraph{nested}, pending...)
			}
		}
	}
	return collected
}

// hierarchicalLayout is hierarchicalLayout(lgraph, monitor).
func (e *ElkLayered) hierarchicalLayout(graph *lgraph.LGraph) {
	// Perform a reversed breadth first search: The graphs in the lowest
	// hierarchy come first.
	graphs := collectAllGraphsBottomUp(graph)

	// We have to make sure that hierarchical processors don't break the
	// control flow of the following layout execution
	parentCms := prop.Get(graph, lopts.CrossingMinimizationStrategy)
	for _, child := range graphs {
		if prop.Get(child, lopts.CrossingMinimizationStrategy) != parentCms {
			lgraph.Bail("the hierarchy aware processor %v in a child node is only allowed if the root node specifies the same hierarchical processor", prop.Get(child, lopts.CrossingMinimizationStrategy))
		}
	}

	// Apply root's greedy switch hierarchical type
	rootType := prop.Get(graph, lopts.CrossingMinimizationGreedySwitchHierarchicalType)
	for _, g := range graphs {
		prop.Set(g, lopts.CrossingMinimizationGreedySwitchHierarchicalType, rootType)
	}

	// Get list of processors for each graph, since they can be different.
	// Iterators are used, so that processing of a graph can be paused and
	// continued easily.
	type graphAndAlgorithm struct {
		graph      *lgraph.LGraph
		processors []lgraph.Processor
		names      []string
		next       int
	}
	list := make([]*graphAndAlgorithm, len(graphs))
	for i, g := range graphs {
		processors := e.prepareGraphForLayout(g)
		list[i] = &graphAndAlgorithm{graph: g, processors: processors, names: ProcessorNames(g)}
	}

	// When the root processor has finished, the layout is finished and all
	// the child processors should have finished as well
	root := list[len(list)-1]
	for root.next < len(root.processors) {
		// Layout from bottom up
		for _, ga := range list {
			for ga.next < len(ga.processors) {
				processor := ga.processors[ga.next]
				name := ga.names[ga.next]
				ga.next++
				if _, aware := processor.(lgraph.HierarchyAware); aware {
					if ga.graph.ParentNode == nil {
						// If processors are hierarchical and reach the root
						// processor, they have to be executed. Continue with
						// the rest of the layout.
						e.observe(name, ga.graph, false)
						processor.Process(ga.graph, e.ctx)
						e.observe(name, ga.graph, true)
					}
					// When processors are hierarchical and do not reach the
					// root processor, break here. The hierarchical processor
					// for this graph is run from the root graph.
					break
				}
				e.observe(name, ga.graph, false)
				processor.Process(ga.graph, e.ctx)
				e.observe(name, ga.graph, true)
			}
		}
	}
}

// resizeGraph is resizeGraph(lgraph).
func resizeGraph(graph *lgraph.LGraph) {
	sizeConstraint := prop.Get(graph, lopts.NodeSizeConstraints)
	sizeOptions := prop.Get(graph, lopts.NodeSizeOptions)

	calculatedSize := graph.ActualSize()
	adjustedSize := calculatedSize.Clone()

	// calculate the new size
	if sizeConstraint.Contains(core.SizeConstraintMinimumSize) {
		minSize := prop.Get(graph, lopts.NodeSizeMinimum)

		// if minimum width or height are not set, maybe default to default
		// values
		if sizeOptions.Contains(core.SizeOptionsDefaultMinimumSize) {
			if minSize.X <= 0 {
				minSize.X = 20
			}
			if minSize.Y <= 0 {
				minSize.Y = 20
			}
		}

		adjustedSize.X = max(calculatedSize.X, minSize.X)
		adjustedSize.Y = max(calculatedSize.Y, minSize.Y)
	}

	if !prop.Get(graph, lopts.NodeSizeFixedGraphSize) {
		resizeGraphNoReallyIMeanIt(graph, calculatedSize, adjustedSize)
	}
}

// resizeGraphNoReallyIMeanIt is resizeGraphNoReallyIMeanIt(lgraph, oldSize,
// newSize).
func resizeGraphNoReallyIMeanIt(graph *lgraph.LGraph, oldSize, newSize *emath.KVector) {
	oldX, oldY := oldSize.X, oldSize.Y
	newX, newY := newSize.X, newSize.Y

	// obey to specified alignment constraints
	contentAlignment := prop.Get(graph, lopts.ContentAlignment)

	// horizontal alignment
	if newX > oldX {
		if contentAlignment.Contains(core.ContentAlignmentHCenter) {
			graph.Offset.X += float64((newX - oldX) / 2)
		} else if contentAlignment.Contains(core.ContentAlignmentHRight) {
			graph.Offset.X += newX - oldX
		}
	}

	// vertical alignment
	if newY > oldY {
		if contentAlignment.Contains(core.ContentAlignmentVCenter) {
			graph.Offset.Y += float64((newY - oldY) / 2)
		} else if contentAlignment.Contains(core.ContentAlignmentVBottom) {
			graph.Offset.Y += newY - oldY
		}
	}

	// correct the position of eastern and southern hierarchical ports, if
	// necessary
	if prop.Get(graph, iprops.GraphProperties).Contains(lenum.GraphPropertiesExternalPorts) && (newX > oldX || newY > oldY) {
		// iterate over all nodes
		for _, node := range graph.LayerlessNodes {
			if node.Type == lgraph.NodeTypeExternalPort {
				switch prop.Get(node, iprops.ExtPortSide) {
				case core.PortSideEast:
					node.Pos.X += newX - oldX
				case core.PortSideSouth:
					node.Pos.Y += newY - oldY
				}
			}
		}
	}

	// Actually apply the new size
	padding := &graph.Padding
	graph.Size.X = newX - padding.Left - padding.Right
	graph.Size.Y = newY - padding.Top - padding.Bottom
}
