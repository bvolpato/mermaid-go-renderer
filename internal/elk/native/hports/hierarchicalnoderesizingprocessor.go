// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/HierarchicalNodeResizingProcessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package hports has the intermediate processors of the layered algorithm
// for hierarchical graphs and their hierarchical ports: the resizing of a
// compound node to its nested graph, the dummy nodes of inverted ports, and
// the constraints, sizes, positions and edge routes of external port dummy
// nodes.
package hports

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("HierarchicalNodeResizingProcessor", func() lgraph.Processor { return NewHierarchicalNodeResizingProcessor() })
}

// The default minimum size of a node: ElkUtil.DEFAULT_MIN_WIDTH and
// ElkUtil.DEFAULT_MIN_HEIGHT.
const (
	defaultMinWidth  = 20.0
	defaultMinHeight = 20.0
)

// HierarchicalNodeResizingProcessor is the processor
// HierarchicalNodeResizingProcessor: it resizes a child graph to fit the
// parent node. It is the last processor of a graph of a hierarchical layout
// that is not hierarchy aware.
type HierarchicalNodeResizingProcessor struct{}

// NewHierarchicalNodeResizingProcessor is new
// HierarchicalNodeResizingProcessor().
func NewHierarchicalNodeResizingProcessor() *HierarchicalNodeResizingProcessor {
	return &HierarchicalNodeResizingProcessor{}
}

// Process is process(graph, progressMonitor).
func (*HierarchicalNodeResizingProcessor) Process(graph *lgraph.LGraph, ctx *lgraph.Context) {
	for _, layer := range graph.Layers {
		graph.LayerlessNodes = append(graph.LayerlessNodes, layer.Nodes...)
		layer.Nodes = layer.Nodes[:0]
	}
	for nodeIndex := 0; nodeIndex < len(graph.LayerlessNodes); nodeIndex++ {
		graph.LayerlessNodes[nodeIndex].SetLayer(nil)
	}
	graph.Layers = graph.Layers[:0]
	resizeGraph(graph)
	if isNested(graph) {
		graphLayoutToNode(graph.ParentNode, graph)
	}
}

// graphPropertiesForChange returns the set of the property graphProperties
// of a graph, to add a constant to it. For a graph without the property,
// elkjs adds the constant to the default value, which each later layout then
// reads. The port does not follow that.
func graphPropertiesForChange(graph *lgraph.LGraph) *prop.EnumSet[lenum.GraphProperties] {
	if graph == nil || !prop.Has(graph, iprops.GraphProperties) {
		lgraph.Bail("hports: a graph without the property graphProperties")
	}
	return prop.Get(graph, iprops.GraphProperties)
}

// graphLayoutToNode transfers the layout of the given graph to the given
// associated node: the compound node that the graph is nested in.
func graphLayoutToNode(node *lgraph.LNode, graph *lgraph.LGraph) {
	// Process external ports
	for nodeIndex := 0; nodeIndex < len(graph.LayerlessNodes); nodeIndex++ {
		childNode := graph.LayerlessNodes[nodeIndex]
		origin := prop.Get(childNode, iprops.Origin)
		if port, isPort := origin.(*lgraph.LPort); isPort {
			portPosition := lgraph.GetExternalPortPosition(graph, childNode, port.Size.X, port.Size.Y)
			port.Pos.X = portPosition.X
			port.Pos.Y = portPosition.Y
			port.SetSide(prop.Get(childNode, iprops.ExtPortSide))
		}
	}

	// Setup the parent node
	actualGraphSize := graph.ActualSize()
	if prop.Get(graph, iprops.GraphProperties).Contains(lenum.GraphPropertiesExternalPorts) {
		// Ports have positions assigned
		prop.Set(node, lopts.PortConstraints, core.PortConstraintsFixedPos)
		graphPropertiesForChange(node.Graph()).Add(lenum.GraphPropertiesNonFreePorts)
		lgraph.ResizeNode(node, actualGraphSize, false, true)
	} else {
		// Ports have not been positioned yet - leave this for next layouter
		lgraph.ResizeNode(node, actualGraphSize, true, true)
	}
}

func isNested(graph *lgraph.LGraph) bool {
	return graph.ParentNode != nil
}

////////////////////////////////////////////////////////////////////////////
// Graph Postprocessing (Size and External Ports)

// resizeGraph sets the size of the given graph such that size constraints
// are adhered to. Furthermore, the padding is added to the graph size and
// the graph offset.
//
// Major parts of this method are adapted from ElkUtil.resizeNode.
//
// Note: This method doesn't care about labels of compound nodes since those
// labels are not attached to the graph.
func resizeGraph(graph *lgraph.LGraph) {
	sizeConstraint := prop.Get(graph, lopts.NodeSizeConstraints)
	sizeOptions := prop.Get(graph, lopts.NodeSizeOptions)

	// getActualSize() used to take the border spacing (what is now included
	// in the padding) into account, which is why by this point it had to be
	// cleared since it had already been applied to the offset and the graph
	// size. It currently does not take the padding into account anymore, but
	// if it does, it needs to be cleared again
	calculatedSize := graph.ActualSize()
	adjustedSize := calculatedSize.Clone()

	// calculate the new size
	if sizeConstraint.Contains(core.SizeConstraintMinimumSize) {
		minSize := prop.Get(graph, lopts.NodeSizeMinimum)

		// if minimum width or height are not set, maybe default to default
		// values
		if sizeOptions.Contains(core.SizeOptionsDefaultMinimumSize) {
			if minSize.X <= 0 {
				minSize.X = defaultMinWidth
			}

			if minSize.Y <= 0 {
				minSize.Y = defaultMinHeight
			}
		}

		// apply new size including border spacing
		adjustedSize.X = max(calculatedSize.X, minSize.X)
		adjustedSize.Y = max(calculatedSize.Y, minSize.Y)
	}

	resizeGraphNoReallyIMeanIt(graph, calculatedSize, adjustedSize)
}

// resizeGraphNoReallyIMeanIt applies a new effective size to a graph that
// previously had an old size calculated by the layout algorithm. This method
// takes care of adjusting content alignments as well as external ports that
// would be misplaced if the new size is larger than the old one.
func resizeGraphNoReallyIMeanIt(graph *lgraph.LGraph, oldSize, newSize *emath.KVector) {
	// obey to specified alignment constraints
	contentAlignment := prop.Get(graph, lopts.ContentAlignment)

	// horizontal alignment
	if newSize.X > oldSize.X {
		if contentAlignment.Contains(core.ContentAlignmentHCenter) {
			graph.Offset.X += float64((newSize.X - oldSize.X) / 2)
		} else if contentAlignment.Contains(core.ContentAlignmentHRight) {
			graph.Offset.X += newSize.X - oldSize.X
		}
	}

	// vertical alignment
	if newSize.Y > oldSize.Y {
		if contentAlignment.Contains(core.ContentAlignmentVCenter) {
			graph.Offset.Y += float64((newSize.Y - oldSize.Y) / 2)
		} else if contentAlignment.Contains(core.ContentAlignmentVBottom) {
			graph.Offset.Y += newSize.Y - oldSize.Y
		}
	}

	// correct the position of eastern and southern hierarchical ports, if
	// necessary
	if prop.Get(graph, iprops.GraphProperties).Contains(lenum.GraphPropertiesExternalPorts) &&
		(newSize.X > oldSize.X || newSize.Y > oldSize.Y) {

		// iterate over the graph's nodes, looking for eastern / southern
		// external ports (at this point, the graph's nodes are not divided
		// into layers anymore)
		for _, node := range graph.LayerlessNodes {
			// we're only looking for external port dummies
			if node.Type == lgraph.NodeTypeExternalPort {
				// check which side the external port is on
				extPortSide := prop.Get(node, iprops.ExtPortSide)
				if extPortSide == core.PortSideEast {
					node.Pos.X += newSize.X - oldSize.X
				} else if extPortSide == core.PortSideSouth {
					node.Pos.Y += newSize.Y - oldSize.Y
				}
			}
		}
	}

	// Actually apply the new size
	padding := &graph.Padding
	graph.Size.X = newSize.X - padding.Left - padding.Right
	graph.Size.Y = newSize.Y - padding.Top - padding.Bottom
}
