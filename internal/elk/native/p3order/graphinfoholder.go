// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/GraphInfoHolder.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p3order

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/greedyswitch"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p3order/counting"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// GraphInfoHolder is the class GraphInfoHolder: it collects the data of one
// graph that the crossing minimization and the port distribution need.
type GraphInfoHolder struct {
	counting.DefaultInit

	// lGraph is the raw graph data.
	lGraph *lgraph.LGraph

	// Saved node orders.
	currentNodeOrder              [][]*lgraph.LNode
	currentlyBestNodeAndPortOrder *SweepCopy
	bestNodeAndPortOrder          *SweepCopy
	portPositions                 []int

	// Processing type information.
	useBottomUp bool

	// Hierarchy access.
	childGraphs           []*lgraph.LGraph
	hasExternalPorts      bool
	hasParent             bool
	parentGraphData       *GraphInfoHolder
	parent                *lgraph.LNode
	layerSweepTypeDecider *LayerSweepTypeDecider

	// Pre-initialized auxiliary objects.
	crossMinimizer   ICrossingMinimizationHeuristic
	portDistributor  ISweepPortDistributor
	crossingsCounter *counting.AllCrossingsCounter
	nPorts           int
}

// NewGraphInfoHolder is new GraphInfoHolder(graph, crossMinType, graphs).
// graphs has the holders of the graphs of the hierarchy that were made
// before, by graph id. The constructor draws one random boolean, except for
// the two-sided greedy switch.
func NewGraphInfoHolder(graph *lgraph.LGraph, crossMinType CrossMinType, graphs []*GraphInfoHolder) *GraphInfoHolder {
	g := &GraphInfoHolder{lGraph: graph}
	g.currentNodeOrder = graph.ToNodeArray()

	// Hierarchy information.
	g.parent = g.lGraph.ParentNode
	g.hasParent = g.parent != nil
	if g.hasParent {
		g.parentGraphData = graphs[g.parent.Graph().ID]
	}
	graphProperties := prop.Get(graph, iprops.GraphProperties)
	g.hasExternalPorts = graphProperties.Contains(lenum.GraphPropertiesExternalPorts)

	// Init all objects needing initialization by graph traversal.
	g.crossingsCounter = counting.NewAllCrossingsCounter(g.currentNodeOrder)
	random := prop.Get(g.lGraph, iprops.Random)
	g.portDistributor = CreateSweepPortDistributor(crossMinType, random, g.currentNodeOrder)
	g.layerSweepTypeDecider = NewLayerSweepTypeDecider(g)
	initializables := []counting.IInitializable{g, g.crossingsCounter, g.layerSweepTypeDecider, g.portDistributor}

	if crossMinType == CrossMinTypeBarycenter &&
		!prop.Get(graph, lopts.CrossingMinimizationForceNodeModelOrder) {
		constraintResolver := NewForsterConstraintResolver(g.currentNodeOrder)
		initializables = append(initializables, constraintResolver)
		g.crossMinimizer = NewBarycenterHeuristic(constraintResolver, random,
			asAbstractBarycenterPortDistributor(g.portDistributor), g.currentNodeOrder)
	} else if crossMinType == CrossMinTypeBarycenter &&
		prop.Get(graph, lopts.CrossingMinimizationForceNodeModelOrder) {
		constraintResolver := NewForsterConstraintResolver(g.currentNodeOrder)
		initializables = append(initializables, constraintResolver)
		g.crossMinimizer = NewModelOrderBarycenterHeuristic(constraintResolver, random,
			asAbstractBarycenterPortDistributor(g.portDistributor), g.currentNodeOrder)
	} else {
		g.crossMinimizer = greedyswitch.NewGreedySwitchHeuristic(crossMinType, g)
	}
	initializables = append(initializables, g.crossMinimizer)

	// Apply Initializer.
	counting.Init(initializables, g.currentNodeOrder)

	// calculate whether we need to use bottom up or sweep into this graph.
	g.useBottomUp = g.layerSweepTypeDecider.UseBottomUp()
	return g
}

// asAbstractBarycenterPortDistributor is the Java cast
// (AbstractBarycenterPortDistributor) portDistributor.
func asAbstractBarycenterPortDistributor(d ISweepPortDistributor) *AbstractBarycenterPortDistributor {
	b, ok := d.(barycenterPortDistributor)
	if !ok {
		lgraph.Bail("GraphInfoHolder: the port distributor is not a barycenter port distributor")
	}
	return b.abstractBarycenterPortDistributor()
}

// DontSweepInto is dontSweepInto().
func (g *GraphInfoHolder) DontSweepInto() bool { return g.useBottomUp }

// LGraph is lGraph().
func (g *GraphInfoHolder) LGraph() *lgraph.LGraph { return g.lGraph }

// CurrentNodeOrder is currentNodeOrder().
func (g *GraphInfoHolder) CurrentNodeOrder() [][]*lgraph.LNode { return g.currentNodeOrder }

// CurrentlyBestNodeAndPortOrder is currentlyBestNodeAndPortOrder().
func (g *GraphInfoHolder) CurrentlyBestNodeAndPortOrder() *SweepCopy {
	return g.currentlyBestNodeAndPortOrder
}

// SetCurrentlyBestNodeAndPortOrder is setCurrentlyBestNodeAndPortOrder(
// currentlyBestNodeAndPortOrder).
func (g *GraphInfoHolder) SetCurrentlyBestNodeAndPortOrder(currentlyBestNodeAndPortOrder *SweepCopy) {
	g.currentlyBestNodeAndPortOrder = currentlyBestNodeAndPortOrder
}

// BestNodeNPortOrder is bestNodeNPortOrder().
func (g *GraphInfoHolder) BestNodeNPortOrder() *SweepCopy { return g.bestNodeAndPortOrder }

// SetBestNodeNPortOrder is setBestNodeNPortOrder(bestNodeNPortOrder).
func (g *GraphInfoHolder) SetBestNodeNPortOrder(bestNodeNPortOrder *SweepCopy) {
	g.bestNodeAndPortOrder = bestNodeNPortOrder
}

// CrossCounter is crossCounter().
func (g *GraphInfoHolder) CrossCounter() *counting.AllCrossingsCounter { return g.crossingsCounter }

// CrossMinimizer is crossMinimizer().
func (g *GraphInfoHolder) CrossMinimizer() ICrossingMinimizationHeuristic { return g.crossMinimizer }

// PortDistributor is portDistributor().
func (g *GraphInfoHolder) PortDistributor() ISweepPortDistributor { return g.portDistributor }

// Parent is parent().
func (g *GraphInfoHolder) Parent() *lgraph.LNode { return g.parent }

// HasParent is hasParent().
func (g *GraphInfoHolder) HasParent() bool { return g.hasParent }

// ChildGraphs is childGraphs().
func (g *GraphInfoHolder) ChildGraphs() []*lgraph.LGraph { return g.childGraphs }

// HasExternalPorts is hasExternalPorts(): whether the parent node of the
// graph has external ports.
func (g *GraphInfoHolder) HasExternalPorts() bool { return g.hasExternalPorts }

// GetBestSweep is getBestSweep(): the copy of the node order of the best
// sweep.
func (g *GraphInfoHolder) GetBestSweep() *SweepCopy {
	if g.crossMinimizer.IsDeterministic() {
		return g.CurrentlyBestNodeAndPortOrder()
	}
	return g.BestNodeNPortOrder()
}

// ParentGraphData is parentGraphData(): the holder of the graph of the
// parent node. The result has the interface type that the package
// greedyswitch reads, and it is nil without a parent.
func (g *GraphInfoHolder) ParentGraphData() greedyswitch.GraphData {
	if g.parentGraphData == nil {
		return nil
	}
	return g.parentGraphData
}

// CrossMinDeterministic is crossMinDeterministic().
func (g *GraphInfoHolder) CrossMinDeterministic() bool { return g.crossMinimizer.IsDeterministic() }

// CrossMinAlwaysImproves is crossMinAlwaysImproves().
func (g *GraphInfoHolder) CrossMinAlwaysImproves() bool { return g.crossMinimizer.AlwaysImproves() }

// PortPositions is portPositions(): the port position array of the needed
// length. Each algorithm must initialize the positions itself.
func (g *GraphInfoHolder) PortPositions() []int { return g.portPositions }

// InitAtNodeLevel is initAtNodeLevel(l, n, nodeOrder).
func (g *GraphInfoHolder) InitAtNodeLevel(l, n int, nodeOrder [][]*lgraph.LNode) {
	node := nodeOrder[l][n]
	nestedGraph := node.NestedGraph
	if nestedGraph != nil {
		g.childGraphs = append(g.childGraphs, nestedGraph)
	}
}

// InitAtPortLevel is initAtPortLevel(l, n, p, nodeOrder).
func (g *GraphInfoHolder) InitAtPortLevel(l, n, p int, nodeOrder [][]*lgraph.LNode) {
	g.nPorts++
}

// InitAfterTraversal is initAfterTraversal().
func (g *GraphInfoHolder) InitAfterTraversal() {
	g.portPositions = make([]int, g.nPorts)
}
