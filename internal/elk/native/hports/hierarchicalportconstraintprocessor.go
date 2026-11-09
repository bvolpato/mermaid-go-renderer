// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/HierarchicalPortConstraintProcessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package hports

import (
	"reflect"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("HierarchicalPortConstraintProcessor", func() lgraph.Processor { return NewHierarchicalPortConstraintProcessor() })
}

const (
	// dummyInputPort is DUMMY_INPUT_PORT: the index of the input port in the
	// list of ports of newly created north / south port dummy nodes.
	dummyInputPort = 0
	// dummyOutputPort is DUMMY_OUTPUT_PORT: the index of the output port in
	// the list of ports of newly created north / south port dummy nodes.
	dummyOutputPort = 1
)

// HierarchicalPortConstraintProcessor is the processor
// HierarchicalPortConstraintProcessor: it processes constraints imposed on
// hierarchical node dummies.
//
// Eastern and western ports cannot be ordered arbitrarily by the crossing
// minimizer if the port order is fixed. Thus, this processor inserts
// appropriate in-layer successor constraints to restrict the node ordering.
//
// Northern and southern external ports cannot be processed in the usual way
// with port constraints at least at FIXED_ORDER. Instead, they are replaced
// by new external port dummies. For each node connected to a northern or
// southern hierarchical port dummy, a new dummy is placed in the next layer,
// and the edges are rerouted appropriately. The original dummies are
// removed, to be reinserted later by the HierarchicalPortOrthogonalEdgeRouter.
// For simplification, this is also done in all other port constraint cases.
type HierarchicalPortConstraintProcessor struct{}

// NewHierarchicalPortConstraintProcessor is new
// HierarchicalPortConstraintProcessor().
func NewHierarchicalPortConstraintProcessor() *HierarchicalPortConstraintProcessor {
	return &HierarchicalPortConstraintProcessor{}
}

// nodeComparator is NodeComparator.compare: it compares nodes by their
// position values in ascending order. Nodes that are not hierarchical port
// dummies are sorted to the bottom of a list. The comparator is not
// consistent for two nodes that are not hierarchical port dummies.
func nodeComparator(node1, node2 *lgraph.LNode) int {
	nodeType1 := node1.Type
	nodePos1 := prop.Get(node1, iprops.PortRatioOrPosition)
	nodeType2 := node2.Type
	nodePos2 := prop.Get(node2, iprops.PortRatioOrPosition)

	if nodeType2 != lgraph.NodeTypeExternalPort {
		return -1
	} else if nodeType1 != lgraph.NodeTypeExternalPort {
		return 1
	}
	if nodePos1 == nodePos2 {
		return 0
	}
	if nodePos1 < nodePos2 {
		return -1
	}
	return 1
}

// Process is process(layeredGraph, monitor).
func (*HierarchicalPortConstraintProcessor) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	processEasternAndWesternPortDummies(layeredGraph)
	processNorthernAndSouthernPortDummies(layeredGraph)
}

////////////////////////////////////////////////////////////////////////////
// East / West Hierarchical Port Dummies

// processEasternAndWesternPortDummies processes eastern and western
// hierarchical port dummies.
func processEasternAndWesternPortDummies(layeredGraph *lgraph.LGraph) {
	// If the port constraints are not at least FIXED_ORDER, there's nothing
	// to be done here
	if !prop.Get(layeredGraph, lopts.PortConstraints).IsOrderFixed() {
		return
	}

	layers := layeredGraph.Layers
	if len(layers) == 0 {
		lgraph.Bail("HierarchicalPortConstraintProcessor: IndexOutOfBoundsException, the graph has no layers")
	}

	// This affects the first and last layer
	processEasternAndWesternPortDummiesOfLayer(layers[0])
	processEasternAndWesternPortDummiesOfLayer(layers[len(layers)-1])
}

// processEasternAndWesternPortDummiesOfLayer is
// processEasternAndWesternPortDummies(layer): it processes the eastern and
// western hierarchical port dummies present in the given layer.
func processEasternAndWesternPortDummiesOfLayer(layer *lgraph.Layer) {
	// Put the nodes into an array
	nodes := append([]*lgraph.LNode(nil), layer.Nodes...)

	// Sort the array; hierarchical port dummies are at the top, sorted by
	// position or ratio in ascending order
	gwt.SortFunc(nodes, nodeComparator)

	// Insert in-layer successor constraints where appropriate
	var lastHierarchicalDummy *lgraph.LNode
	for _, node := range nodes {
		if node.Type != lgraph.NodeTypeExternalPort {
			// No hierarchical port dummy nodes any more
			break
		}

		// Only process dummies created for eastern or western external ports
		externalPortSide := prop.Get(node, iprops.ExtPortSide)
		if externalPortSide != core.PortSideWest && externalPortSide != core.PortSideEast {
			continue
		}

		if lastHierarchicalDummy != nil {
			constraints := prop.Get(lastHierarchicalDummy, iprops.InLayerSuccessorConstraints)
			prop.Set(lastHierarchicalDummy, iprops.InLayerSuccessorConstraints, append(constraints, node))
		}

		lastHierarchicalDummy = node
	}
}

////////////////////////////////////////////////////////////////////////////
// North / South Hierarchical Port Dummies

// originKey returns the value of the property origin of a dummy node as the
// key of a map. The Java maps have the origin objects as keys, which have no
// hashCode of their own. A nil key stands for a missing origin.
func originKey(node *lgraph.LNode) any {
	origin, has := prop.Lookup(node, iprops.Origin)
	if !has || origin == nil {
		return nil
	}
	if !reflect.TypeOf(origin).Comparable() {
		lgraph.Bail("HierarchicalPortConstraintProcessor: the origin of an external port dummy cannot be the key of a map")
	}
	return origin
}

// processNorthernAndSouthernPortDummies processes northern and southern
// hierarchical port dummies.
func processNorthernAndSouthernPortDummies(layeredGraph *lgraph.LGraph) {
	// If the port constraints are not at least FIXED_SIDE, there's nothing to
	// do here
	portConstraints := prop.Get(layeredGraph, lopts.PortConstraints)
	if !portConstraints.IsSideFixed() {
		return
	}

	layerCount := len(layeredGraph.Layers)

	// For each layer, we keep a map of dummy nodes created for a given
	// original external port dummy. This lets us remember which new dummy
	// node was created for which original external port dummy in which
	// layer. We also keep a list of dummy nodes to be added to each layer
	// since iterating over the new dummy nodes in the map may mess up their
	// order since we don't have hashCode() implemented. Once we do, this code
	// can be reworked to just use the maps again.
	// We keep enough space to hold layerCount + 2 instances of each data
	// structure because we might have to add a new first and last layer,
	// even though that shouldn't normally happen
	extPortToDummyNodeMap := make([]map[any]*lgraph.LNode, 0, layerCount+2)
	newDummyNodes := make([][]*lgraph.LNode, 0, layerCount+2)

	// Add maps and lists for a new first layer that might have to be created
	// as well as for the current first layer. A map for the next layer is
	// added on each iteration of the for loop
	extPortToDummyNodeMap = append(extPortToDummyNodeMap, map[any]*lgraph.LNode{}, map[any]*lgraph.LNode{})
	newDummyNodes = append(newDummyNodes, nil, nil)

	// We remember each original external port dummy we encounter (they must
	// be removed from the layers later). The list becomes the value of a
	// property, where an empty list is not a missing value.
	originalExternalPortDummies := []*lgraph.LNode{}

	// Iterate through each layer
	for currLayerIdx := 0; currLayerIdx < layerCount; currLayerIdx++ {
		currentLayer := layeredGraph.Layers[currLayerIdx]

		// Dummy node maps and lists for the next and previous layer. The
		// lists are the elements currLayerIdx and currLayerIdx + 2 of
		// newDummyNodes.
		prevExtPortToDummyNodesMap := extPortToDummyNodeMap[currLayerIdx]
		nextExtPortToDummyNodesMap := map[any]*lgraph.LNode{}
		extPortToDummyNodeMap = append(extPortToDummyNodeMap, nextExtPortToDummyNodesMap)

		prev := currLayerIdx
		newDummyNodes = append(newDummyNodes, nil)
		next := len(newDummyNodes) - 1

		// Iterate through the layer's nodes, looking for normal nodes
		// connected to northern / southern hierarchical port dummies
		for nodeIndex := 0; nodeIndex < len(currentLayer.Nodes); nodeIndex++ {
			currentNode := currentLayer.Nodes[nodeIndex]
			if isNorthernOrSouthernDummy(currentNode) {
				// It's a northern or southern external port dummy. Schedule
				// for removal and move on to next node
				originalExternalPortDummies = append(originalExternalPortDummies, currentNode)
				continue
			}

			// Iterate over the node's incoming edges. The loop changes the
			// source of an edge only, so the lists of incoming edges of the
			// node stay as they are.
			for _, edge := range currentNode.IncomingEdges() {
				sourceNode := edge.Source.Node()

				// Check if it's a northern / southern dummy node
				if !isNorthernOrSouthernDummy(sourceNode) {
					continue
				}

				// See if a dummy has already been created for the previous
				// layer
				key := originKey(sourceNode)
				prevLayerDummy := prevExtPortToDummyNodesMap[key]
				if prevLayerDummy == nil {
					// No. Create one.
					prevLayerDummy = createDummy(layeredGraph, sourceNode)
					prevExtPortToDummyNodesMap[key] = prevLayerDummy
					newDummyNodes[prev] = append(newDummyNodes[prev], prevLayerDummy)
				}

				// Reroute the edge
				edge.SetSource(prevLayerDummy.Ports[dummyOutputPort])
			}

			// Iterate over the node's outgoing edges
			for _, edge := range currentNode.OutgoingEdges() {
				targetNode := edge.Target.Node()

				// Check if it's a northern / southern dummy node
				if !isNorthernOrSouthernDummy(targetNode) {
					continue
				}

				// See if a dummy has already been created for the next layer
				key := originKey(targetNode)
				nextLayerDummy := nextExtPortToDummyNodesMap[key]
				if nextLayerDummy == nil {
					// No. Create one.
					nextLayerDummy = createDummy(layeredGraph, targetNode)
					nextExtPortToDummyNodesMap[key] = nextLayerDummy
					newDummyNodes[next] = append(newDummyNodes[next], nextLayerDummy)
				}

				// Reroute the edge
				edge.SetTarget(nextLayerDummy.Ports[dummyInputPort])
			}
		}
	}

	// Add the newly created dummy nodes
	for i := 0; i < len(newDummyNodes); i++ {
		nodeList := newDummyNodes[i]
		if len(nodeList) == 0 {
			// No dummy nodes, so just move on
			continue
		}

		// Find the layer the dummy nodes should be added to
		var layer *lgraph.Layer
		if i == 0 {
			// A new first layer must be created
			layer = lgraph.NewLayer(layeredGraph)
			layeredGraph.Layers = append([]*lgraph.Layer{layer}, layeredGraph.Layers...)
		} else if i == len(extPortToDummyNodeMap)-1 {
			// A new last layer must be created
			layer = lgraph.NewLayer(layeredGraph)
			layeredGraph.Layers = append(layeredGraph.Layers, layer)
		} else {
			layer = layeredGraph.Layers[i-1]
		}

		for _, dummy := range nodeList {
			dummy.SetLayer(layer)
		}
	}

	// Iterate through the hierarchical port dummies and remove them
	for _, originalDummy := range originalExternalPortDummies {
		// Remove the original dummy; new dummy nodes have already been
		// created for it
		originalDummy.SetLayer(nil)
	}

	// Remember the original external port dummies in the graph
	prop.Set(layeredGraph, iprops.ExtPortReplacedDummies, originalExternalPortDummies)
}

// isNorthernOrSouthernDummy checks if the node represents a northern or
// southern external port.
func isNorthernOrSouthernDummy(node *lgraph.LNode) bool {
	nodeType := node.Type

	if nodeType == lgraph.NodeTypeExternalPort {
		portSide := prop.Get(node, iprops.ExtPortSide)

		return portSide == core.PortSideNorth || portSide == core.PortSideSouth
	}

	return false
}

// createDummy creates a dummy for the given original dummy. The property
// externalPortReplacedDummy of the new dummy names the original dummy. This
// way, the original dummy can later be restored, and the newly created dummy
// can be told apart from the original.
func createDummy(layeredGraph *lgraph.LGraph, originalDummy *lgraph.LNode) *lgraph.LNode {
	newDummy := lgraph.NewLNode(layeredGraph)
	prop.CopyProperties(newDummy, originalDummy)
	prop.Set(newDummy, iprops.ExtPortReplacedDummy, originalDummy)
	prop.Set(newDummy, lopts.PortConstraints, core.PortConstraintsFixedPos)
	prop.Set(newDummy, lopts.Alignment, core.AlignmentCenter)
	newDummy.Type = lgraph.NodeTypeExternalPort

	inputPort := lgraph.NewLPort()
	inputPort.SetNode(newDummy)
	inputPort.SetSide(core.PortSideWest)

	outputPort := lgraph.NewLPort()
	outputPort.SetNode(newDummy)
	outputPort.SetSide(core.PortSideEast)

	return newDummy
}
