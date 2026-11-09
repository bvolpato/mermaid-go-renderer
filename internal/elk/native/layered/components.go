// Ported from ComponentsProcessor.java, AbstractGraphPlacer.java and
// SimpleRowGraphPlacer.java in
// plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/components
// of Eclipse ELK 0.9.1 (EPL-2.0).
//
// The placers for graphs with external ports (ComponentGroupGraphPlacer and
// the model order variants) and the ComponentsCompactor are not ported. A
// graph that needs one of them stops the layout.

package layered

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// SplitComponents is ComponentsProcessor.split(graph): the connected
// components of the graph, each in a new graph, or the graph itself when the
// components are not separated.
func SplitComponents(graph *lgraph.LGraph) []*lgraph.LGraph {
	var result []*lgraph.LGraph

	// Whether separate components processing is requested
	separate := prop.Get(graph, lopts.SeparateConnectedComponents)

	// Whether the graph contains external ports
	extPorts := prop.Get(graph, iprops.GraphProperties).Contains(lenum.GraphPropertiesExternalPorts)

	// The graph may only be separated if the port constraints allow it
	extPortConstraints := prop.Get(graph, lopts.PortConstraints)
	compatiblePortConstraints := !extPortConstraints.IsOrderFixed()

	if separate && (compatiblePortConstraints || !extPorts) {
		if extPorts {
			lgraph.Bail("ComponentsProcessor: the placers for a graph with external ports are not ported")
		}
		// Set id of all nodes to 0
		for _, node := range graph.LayerlessNodes {
			node.ID = 0
		}

		// Perform DFS starting on each node, collecting connected components
		result = []*lgraph.LGraph{}
		// The loop changes the graph of each node, not the node list.
		for _, node := range graph.LayerlessNodes {
			component, extPortSides := dfs(node, nil, nil)
			if component != nil {
				newGraph := lgraph.NewLGraph()
				prop.CopyProperties(newGraph, graph)
				prop.Set(newGraph, iprops.ExtPortConnections, extPortSides)
				newGraph.Padding.Copy(&graph.Padding.Spacing)
				prop.Unset(newGraph, lopts.NodeSizeMinimum)
				for _, n := range *component {
					newGraph.LayerlessNodes = append(newGraph.LayerlessNodes, n)
					n.SetGraph(newGraph)
				}
				result = append(result, newGraph)
			}
		}
	} else {
		result = []*lgraph.LGraph{graph}
	}

	// If model order should be preserved the connected components should be
	// ordered by their elements
	if prop.Get(graph, lopts.ConsiderModelOrderComponents) != lenum.ComponentOrderingStrategyNone {
		gwt.SortFunc(result, func(g1, g2 *lgraph.LGraph) int {
			return gwt.IntCompare(lgraph.GetMinimalModelOrder(g1), lgraph.GetMinimalModelOrder(g2))
		})
	}

	return result
}

// dfs is ComponentsProcessor.dfs(node, data): it adds the nodes that can be
// reached from node to the component, and returns the component. It returns
// nil for a node that a search visited before.
func dfs(node *lgraph.LNode, component *[]*lgraph.LNode, extPortSides *prop.EnumSet[core.PortSide]) (*[]*lgraph.LNode, *prop.EnumSet[core.PortSide]) {
	if node.ID != 0 {
		return nil, nil
	}
	// Mark the node as visited
	node.ID = 1

	// Check if we already have a list of nodes for the connected component
	if component == nil {
		component = &[]*lgraph.LNode{}
		extPortSides = prop.NoneOf[core.PortSide]()
	}

	// Add this node to the component
	*component = append(*component, node)

	// Check if this node is an external port dummy and, if so, add its
	// external port side to the set of connections
	if node.Type == lgraph.NodeTypeExternalPort {
		extPortSides.Add(prop.Get(node, iprops.ExtPortSide))
	}

	// DFS
	for _, port1 := range node.Ports {
		for _, port2 := range port1.ConnectedPorts() {
			dfs(port2.Node(), component, extPortSides)
		}
	}

	return component, extPortSides
}

// CombineComponents is ComponentsProcessor.combine(components, target) with
// the SimpleRowGraphPlacer.
func CombineComponents(components []*lgraph.LGraph, target *lgraph.LGraph) {
	if len(components) == 1 {
		source := components[0]
		if source != target {
			target.LayerlessNodes = target.LayerlessNodes[:0]
			moveGraph(target, source, 0, 0)
			prop.CopyProperties(target, source)
			target.Padding.Copy(&source.Padding.Spacing)
			target.Size.X = source.Size.X
			target.Size.Y = source.Size.Y
		}
		return
	} else if len(components) == 0 {
		target.LayerlessNodes = target.LayerlessNodes[:0]
		target.Size.X = 0
		target.Size.Y = 0
		return
	}

	sortComponents(components, target)

	firstComponent := components[0]
	target.LayerlessNodes = target.LayerlessNodes[:0]
	prop.CopyProperties(target, firstComponent)

	// determine the maximal row width by the maximal box width and the
	// total area
	maxRowWidth := 0.0
	totalArea := 0.0
	for _, graph := range components {
		size := &graph.Size
		maxRowWidth = max(maxRowWidth, size.X)
		totalArea += float64(size.X * size.Y)
	}
	maxRowWidth = max(maxRowWidth, math.Sqrt(totalArea)*prop.Get(target, lopts.AspectRatio))
	componentSpacing := prop.Get(target, lopts.SpacingComponentComponent)

	placeComponents(components, target, maxRowWidth, componentSpacing)

	// if compaction is desired, do so!
	if prop.Get(firstComponent, lopts.CompactionConnectedComponents) {
		lgraph.Bail("ComponentsProcessor: the ComponentsCompactor is not ported")
	}

	// this sets the graph's nodes to the target
	for _, source := range components {
		moveGraph(target, source, 0, 0)
	}
}

// sortComponents is SimpleRowGraphPlacer.sortComponents.
func sortComponents(components []*lgraph.LGraph, target *lgraph.LGraph) {
	if prop.Get(target, lopts.ConsiderModelOrderComponents) == lenum.ComponentOrderingStrategyNone {
		// assign priorities
		for _, graph := range components {
			priority := 0
			for _, node := range graph.LayerlessNodes {
				priority += prop.Get(node, lopts.Priority)
			}
			graph.ID = priority
		}

		// sort the components by their priority and size
		gwt.SortFunc(components, func(graph1, graph2 *lgraph.LGraph) int {
			prio := graph2.ID - graph1.ID
			if prio == 0 {
				size1 := graph1.Size.X * graph1.Size.Y
				size2 := graph2.Size.X * graph2.Size.Y
				return gwt.DoubleCompare(size1, size2)
			}
			return prio
		})
	}
}

// placeComponents is SimpleRowGraphPlacer.placeComponents.
func placeComponents(components []*lgraph.LGraph, target *lgraph.LGraph, maxRowWidth, componentSpacing float64) {
	// place nodes iteratively into rows
	xpos, ypos, highestBox, broadestRow := 0.0, 0.0, 0.0, componentSpacing
	for _, graph := range components {
		size := &graph.Size
		if xpos+size.X > maxRowWidth {
			// place the graph into the next row
			xpos = 0
			ypos += highestBox + componentSpacing
			highestBox = 0
		}
		offset := &graph.Offset
		offsetGraph(graph, xpos+offset.X, ypos+offset.Y)
		offset.Reset()
		broadestRow = max(broadestRow, xpos+size.X)
		highestBox = max(highestBox, size.Y)
		xpos += size.X + componentSpacing
	}

	target.Size.X = broadestRow
	target.Size.Y = ypos + highestBox
}

// moveGraph is AbstractGraphPlacer.moveGraph. The offset of the source graph
// changes, as in Java.
func moveGraph(destGraph, sourceGraph *lgraph.LGraph, offsetx, offsety float64) {
	graphOffset := sourceGraph.Offset.AddXY(offsetx, offsety)
	for _, node := range sourceGraph.LayerlessNodes {
		moveNode(node, graphOffset)
		destGraph.LayerlessNodes = append(destGraph.LayerlessNodes, node)
		node.SetGraph(destGraph)
	}
}

// offsetGraph is AbstractGraphPlacer.offsetGraph.
func offsetGraph(graph *lgraph.LGraph, offsetx, offsety float64) {
	graphOffset := emath.NewKVector(offsetx, offsety)
	for _, node := range graph.LayerlessNodes {
		moveNode(node, graphOffset)
	}
}

func moveNode(node *lgraph.LNode, graphOffset *emath.KVector) {
	node.Pos.Add(graphOffset)
	for _, port := range node.Ports {
		for _, edge := range port.OutgoingEdges {
			edge.BendPoints.Offset(graphOffset)
			if junctionPoints := prop.Get(edge, lopts.JunctionPoints); junctionPoints != nil {
				junctionPoints.Offset(graphOffset)
			}
			for _, label := range edge.Labels {
				label.Pos.Add(graphOffset)
			}
		}
	}
}
