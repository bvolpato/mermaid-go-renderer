// Ported from RecursiveGraphLayoutEngine.java,
// data/LayoutAlgorithmResolver.java and data/DeprecatedLayoutOptionReplacer.java
// in plugins/org.eclipse.elk.core/src/org/eclipse/elk/core of Eclipse ELK
// 0.9.1 (EPL-2.0).
//
// Top-down layout, inside self loops, node scaling and each algorithm other
// than the layered one are not ported. A graph that needs one of them stops
// the layout.

package native

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/coreopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/elkgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/layered"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// engine is RecursiveGraphLayoutEngine for one layout.
type engine struct {
	provider *layered.Provider
}

// elements calls fn for each element of the graph that has properties: the
// nodes with their labels and ports, and the edges with their labels.
func elements(root *elkgraph.Node, edges []*elkgraph.Edge, fn func(h prop.Holder, node *elkgraph.Node)) {
	var walk func(n *elkgraph.Node)
	walk = func(n *elkgraph.Node) {
		fn(n, n)
		for _, l := range n.Labels {
			fn(l, nil)
		}
		for _, port := range n.Ports {
			fn(port, nil)
			for _, l := range port.Labels {
				fn(l, nil)
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	for _, e := range edges {
		fn(e, nil)
		for _, l := range e.Labels {
			fn(l, nil)
		}
	}
}

// replaceDeprecatedOptions is DeprecatedLayoutOptionReplacer.visit.
//
// For an element without its own portLabels.placement, elkjs adds the new
// constant to the default set of the option, which the next elements and the
// next layouts of the same engine then see: the EnumSet of GWT is not cloned
// (see prop.NewFresh). That result depends on the layouts before it, so such
// an element stops the layout.
func replaceDeprecatedOptions(h prop.Holder) {
	if prop.Has(h, coreopts.PortLabelsNextToPortIfPossible) {
		if !prop.Has(h, coreopts.PortLabelsPlacement) {
			lgraph.Bail("the deprecated option portLabels.nextToPortIfPossible on an element without portLabels.placement changes a shared default in elkjs")
		}
		prop.Get(h, coreopts.PortLabelsPlacement).Add(core.PortLabelPlacementNextToPortIfPossible)
		prop.Unset(h, coreopts.PortLabelsNextToPortIfPossible)
	}
	if prop.Has(h, coreopts.NodeSizeOptions) {
		if prop.Get(h, coreopts.NodeSizeOptions).Contains(core.SizeOptionsSpaceEfficientPortLabels) {
			if !prop.Has(h, coreopts.PortLabelsPlacement) {
				lgraph.Bail("the deprecated option SPACE_EFFICIENT_PORT_LABELS on an element without portLabels.placement changes a shared default in elkjs")
			}
			prop.Get(h, coreopts.PortLabelsPlacement).Add(core.PortLabelPlacementSpaceEfficient)
			prop.Get(h, coreopts.NodeSizeOptions).Remove(core.SizeOptionsSpaceEfficientPortLabels)
		}
	}
}

// resolveAlgorithm is LayoutAlgorithmResolver.resolveAlgorithm.
func resolveAlgorithm(node *elkgraph.Node) {
	algorithmID, _ := prop.Lookup(node, coreopts.Algorithm)
	if data := elkgraph.AlgorithmBySuffix(algorithmID); data != nil {
		prop.Set[any](node, coreopts.ResolvedAlgorithm, data)
		return
	}
	mustResolve := !prop.Has(node, coreopts.ResolvedAlgorithm) && (len(node.Children) != 0 || prop.Get(node, coreopts.InsideSelfLoopsActivate))
	if mustResolve {
		if gwt.Trim(algorithmID) == "" {
			prop.Set[any](node, coreopts.ResolvedAlgorithm, elkgraph.AlgorithmBySuffix(elkgraph.LayeredAlgorithmID))
			return
		}
		lgraph.Bail("Layout algorithm '%s' not found", algorithmID)
	}
}

// layout is RecursiveGraphLayoutEngine.layout(layoutGraph, monitor).
func (e *engine) layout(root *elkgraph.Node, edges []*elkgraph.Edge) {
	elements(root, edges, func(h prop.Holder, _ *elkgraph.Node) { replaceDeprecatedOptions(h) })
	if !prop.Has(root, coreopts.ResolvedAlgorithm) {
		elements(root, edges, func(_ prop.Holder, node *elkgraph.Node) {
			if node != nil && !prop.Get(node, coreopts.NoLayout) {
				resolveAlgorithm(node)
			}
		})
	}
	e.layoutRecursively(root)
}

// evaluateHierarchyHandlingInheritance is the method of the same name.
func evaluateHierarchyHandlingInheritance(node *elkgraph.Node) {
	if prop.Get(node, coreopts.HierarchyHandling) == core.HierarchyHandlingInherit {
		if node.Parent == nil {
			// Set root node to SEPARATE_CHILDREN handling
			prop.Set(node, coreopts.HierarchyHandling, core.HierarchyHandlingSeparateChildren)
		} else {
			// Set hierarchy handling to the value of the parent. It is safe
			// to assume that the parent has been set to the right value
			// before.
			prop.Set(node, coreopts.HierarchyHandling, prop.Get(node.Parent, coreopts.HierarchyHandling))
		}
	}
}

func resolved(node *elkgraph.Node) *elkgraph.AlgorithmData {
	v, _ := prop.Lookup(node, coreopts.ResolvedAlgorithm)
	data, _ := v.(*elkgraph.AlgorithmData)
	return data
}

// applyConfiguredNodeScaling is ElkUtil.applyConfiguredNodeScaling. A scale
// factor other than 1 is not ported.
func applyConfiguredNodeScaling(node *elkgraph.Node) {
	if prop.Get(node, coreopts.ScaleFactor) != 1 {
		lgraph.Bail("the option scaleFactor is not ported")
	}
}

// layoutRecursively is layoutRecursively(layoutNode, testController,
// monitor).
func (e *engine) layoutRecursively(node *elkgraph.Node) {
	if prop.Get(node, coreopts.NoLayout) {
		return
	}

	hasChildren := len(node.Children) != 0
	// gatherInsideSelfLoops
	if prop.Get(node, coreopts.InsideSelfLoopsActivate) {
		lgraph.Bail("inside self loops are not ported")
	}

	if !hasChildren {
		return
	}

	// this node has children and is thus a compound node; fetch the layout
	// algorithm that should be used to compute a layout for its content
	algorithmData := resolved(node)
	if algorithmData == nil {
		lgraph.Bail("Resolved algorithm is not set; apply a LayoutAlgorithmResolver before computing layout.")
	}

	// Persist the Hierarchy Handling in the nodes by querying the parent
	// node
	evaluateHierarchyHandlingInheritance(node)

	// The layered algorithm supports compound graphs. For each other
	// algorithm the layout stops below, in executeAlgorithm.
	layeredAlgorithm := algorithmData.ID == elkgraph.LayeredAlgorithmID

	// If the node should be layouted with hierarchy, it is at least
	// necessary to process the children that have a different algorithm
	if prop.Get(node, coreopts.HierarchyHandling) == core.HierarchyHandlingIncludeChildren && layeredAlgorithm {
		if prop.Get(node, coreopts.TopdownLayout) {
			lgraph.Bail("Topdown layout cannot be used together with hierarchy handling.")
		}

		// the layout algorithm will compute a layout for multiple levels of
		// hierarchy under the current one
		queue := append([]*elkgraph.Node(nil), node.Children...)
		for len(queue) > 0 {
			n := queue[0]
			queue = queue[1:]
			// Persist the Hierarchy Handling in the nodes by querying the
			// parent node
			evaluateHierarchyHandlingInheritance(n)
			stopHierarchy := prop.Get(n, coreopts.HierarchyHandling) == core.HierarchyHandlingSeparateChildren

			// Hierarchical layout is stopped by explicitly disabling or
			// switching the algorithm. In that case, a separate recursive
			// layout run has to be triggered.
			if stopHierarchy || (prop.Has(n, coreopts.Algorithm) && algorithmData != resolved(n)) {
				e.layoutRecursively(n)

				// Explicitly disable hierarchical layout for the child node.
				// Simplifies the handling of switching algorithms in the
				// layouter.
				prop.Set(n, coreopts.HierarchyHandling, core.HierarchyHandlingSeparateChildren)

				// Apply the LayoutOptions.SCALE_FACTOR if present
				applyConfiguredNodeScaling(n)
			} else {
				// Child should be included in current layout, possibly
				// adding its own children
				queue = append(queue, n.Children...)
			}
		}
	} else {
		if !layeredAlgorithm {
			lgraph.Bail("the algorithm %s is not ported", algorithmData.ID)
		}
		// layout each compound node contained in this node separately
		if prop.Get(node, coreopts.TopdownLayout) {
			lgraph.Bail("top-down layout is not ported")
		}
		for _, child := range node.Children {
			e.layoutRecursively(child)
			// Apply the LayoutOptions.SCALE_FACTOR if present
			applyConfiguredNodeScaling(child)
		}
	}

	// executeAlgorithm
	if !layeredAlgorithm {
		lgraph.Bail("the algorithm %s is not ported", algorithmData.ID)
	}
	e.provider.Layout(node)
}
