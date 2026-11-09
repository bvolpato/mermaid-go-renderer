// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/SelfLoopRouter.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package routing

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("SelfLoopRouter", func() lgraph.Processor { return NewSelfLoopRouter() })
}

// labelManager is LabelManagementOptions.LABEL_MANAGER. elkjs has no class
// that implements ILabelManager, so the value is null in each layout that
// elkjs can do.
var labelManager = prop.New[any]("org.eclipse.elk.labelManager")

// SelfLoopRouter is the processor SelfLoopRouter: it computes the bend
// points of the self loops and places the self loop labels.
type SelfLoopRouter struct {
	// routingDirector computes how the loops go around the node.
	routingDirector *RoutingDirector
	// labelPlacer decides the placement of the labels.
	labelPlacer *LabelPlacer
	// routingSlotAssigner gives the self loops their routing slots.
	routingSlotAssigner *RoutingSlotAssigner
}

// NewSelfLoopRouter is new SelfLoopRouter().
func NewSelfLoopRouter() *SelfLoopRouter {
	return &SelfLoopRouter{
		routingDirector:     NewRoutingDirector(),
		labelPlacer:         NewLabelPlacer(),
		routingSlotAssigner: NewRoutingSlotAssigner(),
	}
}

// Process is process(graph, progressMonitor).
func (r *SelfLoopRouter) Process(graph *lgraph.LGraph, ctx *lgraph.Context) {
	router, notPorted := r.routerForGraph(graph)
	if prop.Has(graph, labelManager) {
		// elkjs fails with a ClassCastException for a label manager.
		lgraph.Bail("SelfLoopRouter: label management is not ported")
	}

	// Process each node that has self loops.
	for _, layer := range graph.Layers {
		for _, lNode := range layer.Nodes {
			if lNode.Type == lgraph.NodeTypeNormal && prop.Has(lNode, iprops.SelfLoopHolder) {
				if notPorted != "" {
					lgraph.Bail("SelfLoopRouter: the %s is not ported", notPorted)
				}
				r.processNode(loops.Of(lNode), router)
			}
		}
	}
}

// routerForGraph is routerForGraph(graph). For a router that is not ported
// it returns the name of the router. The layout stops only when a node has
// self loops to route.
func (r *SelfLoopRouter) routerForGraph(graph *lgraph.LGraph) (router AbstractSelfLoopRouter, notPorted string) {
	switch prop.Get(graph, lopts.EdgeRouting) {
	case core.EdgeRoutingPolyline:
		return nil, "PolylineSelfLoopRouter"

	case core.EdgeRoutingSplines:
		return nil, "SplineSelfLoopRouter"

	default:
		return NewOrthogonalSelfLoopRouter(), ""
	}
}

func (r *SelfLoopRouter) processNode(slHolder *loops.SelfLoopHolder, slRouter AbstractSelfLoopRouter) {
	// Compute how each hyper loop goes around the node: from its leftmost
	// port to its rightmost port. The choice of the two ports changes the
	// number of crossings.
	r.routingDirector.DetermineLoopRoutes(slHolder)

	// Place the self loop labels. The routing slot assigner then keeps
	// labels that overlap out of one slot.
	r.labelPlacer.PlaceLabels(slHolder)

	// Assign the routing slots of each hyper loop on its port sides, so
	// that the trunks of the self loops do not intersect. A trunk is the
	// part of a self loop that runs around the node, parallel to its border.
	r.routingSlotAssigner.AssignRoutingSlots(slHolder)

	// Route the self loops.
	slRouter.RouteSelfLoops(slHolder)
}
