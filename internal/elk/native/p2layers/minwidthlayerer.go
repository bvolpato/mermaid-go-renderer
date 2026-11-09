// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p2layers/MinWidthLayerer.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p2layers

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("MinWidthLayerer", func() lgraph.Processor { return NewMinWidthLayerer() })
}

// The recommended values of the algorithm: UPPERBOUND_ON_WIDTH_RANGE is the
// range from 1 to 4, and COMPENSATOR_RANGE the range from 1 to 2.
const (
	upperboundOnWidthRangeLower = 1
	upperboundOnWidthRangeUpper = 4
	compensatorRangeLower       = 1
	compensatorRangeUpper       = 2
)

// MinWidthLayerer is the phase MinWidthLayerer: the heuristic of Nikolov,
// Tarassov and Branke for a layering of minimum width, with the real sizes of
// the nodes.
type MinWidthLayerer struct {
	// Some variables used for considering real node sizes.
	dummySize       float64
	minimumNodeSize float64
	normSize        []float64
	avgSize         float64

	// Degrees of nodes without self loops, indexed by node.id
	inDegree  []int
	outDegree []int
}

// NewMinWidthLayerer is new MinWidthLayerer().
func NewMinWidthLayerer() *MinWidthLayerer { return &MinWidthLayerer{} }

// Process is process(layeredGraph, progressMonitor).
func (l *MinWidthLayerer) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	notInserted := layeredGraph.LayerlessNodes

	// The algorithm requires DAG G = (V, E). In this version self-loops are
	// allowed (as we're going to filter them). Additional properties as
	// described above (called UBW and c in the original paper):
	upperBoundOnWidth := prop.Get(layeredGraph, lopts.LayeringMinWidthUpperBoundOnWidth)
	compensator := prop.Get(layeredGraph, lopts.LayeringMinWidthUpperLayerEstimationScalingFactor)

	// First step to consider the real size of nodes: Initialize the dummy
	// size with the spacing properties
	l.dummySize = prop.Get(layeredGraph, lopts.SpacingEdgeEdge)

	// Compute the minimum nodes size (of the real nodes). We're going to use
	// this value in the next step to normalize the different node sizes.
	l.minimumNodeSize = math.Inf(1)
	for _, node := range notInserted {
		if node.Type != lgraph.NodeTypeNormal {
			continue
		}
		size := node.Size.Y
		l.minimumNodeSize = min(l.minimumNodeSize, size)
	}
	// The minimum nodes size might be zero. If This is the case, then simply
	// don't normalize the node sizes.
	l.minimumNodeSize = max(1, l.minimumNodeSize)

	// We initialize the nodes' id and use it to refer to its in- and
	// out-degree stored each in an array. We also compute the size of each
	// node in relation to the smallest real node in the graph (normalized
	// size) and store it in the same way.
	numOfNodes := len(notInserted)
	l.inDegree = make([]int, numOfNodes)
	l.outDegree = make([]int, numOfNodes)
	l.normSize = make([]float64, numOfNodes)
	i := 0
	l.avgSize = 0
	for _, node := range notInserted {
		// Warning: LNode.id is being redefined here!
		node.ID = i
		i++
		l.inDegree[node.ID] = countIncomingEdgesExceptSelfLoops(node)
		l.outDegree[node.ID] = countOutgoingEdgesExceptSelfLoops(node)
		l.normSize[node.ID] = node.Size.Y / l.minimumNodeSize
		// The average size of a node will also be based on the normalized
		// size.
		l.avgSize += l.normSize[node.ID]
	}
	// normalize dummy size, too:
	l.dummySize /= l.minimumNodeSize
	// Divide sum of normalized node sizes by the number of nodes to get an
	// actual mean.
	l.avgSize /= float64(numOfNodes)

	// Precalculate the successors of all nodes (as a Set) and put them in a
	// list.
	nodeSuccessors := precalcSuccessors(notInserted)

	// The ids are indices now. A successor that is not a layerless node of
	// the graph can never be placed, so the loop of computeMinWidthLayering
	// would not end.
	for _, successors := range nodeSuccessors {
		successors.Each(func(n *lgraph.LNode) bool {
			if n.ID < 0 || n.ID >= numOfNodes || notInserted[n.ID] != n {
				lgraph.Bail("MinWidthLayerer: an edge leads to a node that is not a layerless node of the graph")
			}
			return true
		})
	}

	// Guarantee ConditionSelect from the paper, which states that nodes with
	// maximum out-degree should be preferred during layer placement, by
	// ordering the nodes by descending maximum out-degree in advance.
	gwt.SortFunc(notInserted, func(o1, o2 *lgraph.LNode) int {
		// Collections.reverseOrder(new MinOutgoingEdgesComparator())
		return l.minOutgoingEdgesComparator(o2, o1)
	})

	// minimum width of a layer of maximum size in a computed layering
	// (primary criterion used for comparison, if more than one layering is
	// computed). It's a double as it takes in account the actual width based
	// on the normalized size of the nodes.
	minWidth := math.Inf(1)
	// minimum number of layers in a computed layering {@code minWidth}
	// (secondary criterion used for comparison, if more than one layering is
	// computed).
	minNumOfLayers := math.MaxInt32
	// holding the currently chosen candidate for the final layering as a List
	var candidateLayering [][]*lgraph.LNode
	haveCandidate := false

	// At first blindly set the parameters for the loose upper bound and the
	// compensator to the exact values, which have been configured via their
	// respective properties, so that only one layering will be computed ...
	ubwStart := upperBoundOnWidth
	ubwEnd := upperBoundOnWidth
	cStart := compensator
	cEnd := compensator

	// ... then check, whether any special values (i.e. negative values, which
	// aren't valid) have been used for the properties. In that case use the
	// recommended ranges described above ...
	if upperBoundOnWidth < 0 {
		ubwStart = upperboundOnWidthRangeLower
		ubwEnd = upperboundOnWidthRangeUpper
	}
	if compensator < 0 {
		cStart = compensatorRangeLower
		cEnd = compensatorRangeUpper
	}

	// ... Depending on the start- and end-values, this nested for-loop will
	// last for up to 8 iterations resulting in one, two, four or eight
	// different layerings.
	for ubw := ubwStart; ubw <= ubwEnd; ubw++ {
		for c := cStart; c <= cEnd; c++ {
			newWidth, layering := l.computeMinWidthLayering(ubw, c, notInserted, nodeSuccessors)

			// Important if more than one layering is computed: replace the
			// current candidate layering with a newly computed one, if it is
			// narrower or has the same maximum width but less layers.
			newNumOfLayers := len(layering)
			if newWidth < minWidth || (newWidth == minWidth && newNumOfLayers < minNumOfLayers) {
				minWidth = newWidth
				minNumOfLayers = newNumOfLayers
				candidateLayering = layering
				haveCandidate = true
			}
		}
	}
	if !haveCandidate {
		// The Java code fails here with a NullPointerException: no layering
		// has a width that is a number.
		lgraph.Bail("MinWidthLayerer: no layering was computed")
	}

	// Finally, add the winning layering to the Klay layered data structures.
	for _, layerList := range candidateLayering {
		currentLayer := lgraph.NewLayer(layeredGraph)
		for _, node := range layerList {
			node.SetLayer(currentLayer)
		}
		layeredGraph.Layers = append(layeredGraph.Layers, currentLayer)
	}

	// The algorithm constructs the layering bottom up, but ElkLayered expects
	// the list of layers to be ordered top down.
	layers := layeredGraph.Layers
	for iFront, iBack := 0, len(layers)-1; iFront < iBack; iFront, iBack = iFront+1, iBack-1 {
		layers[iFront], layers[iBack] = layers[iBack], layers[iFront]
	}
	// After the algorithm, there should be no nodes left to be put in a
	// layer, so we're gonna delete them.
	clear(notInserted)
	layeredGraph.LayerlessNodes = notInserted[:0]
}

// precalcSuccessors returns for each node the set of the targets of its
// outgoing edges, without self loops, in the order of the nodes.
func precalcSuccessors(nodes []*lgraph.LNode) []*gwt.Set[*lgraph.LNode] {
	successors := make([]*gwt.Set[*lgraph.LNode], 0, len(nodes))

	for _, node := range nodes {
		outNodes := gwt.NewSet[*lgraph.LNode]()

		for _, port := range node.Ports {
			for _, edge := range port.OutgoingEdges {
				if !selfLoopPredicate(edge) {
					outNodes.Add(edge.Target.Node())
				}
			}
		}

		successors = append(successors, outNodes)
	}

	return successors
}

// computeMinWidthLayering computes a layering of the nodes with the MinWidth
// heuristic for one upper bound on the width and one compensator. It returns
// the maximum width of the layering and the layers from the last one to the
// first one.
func (l *MinWidthLayerer) computeMinWidthLayering(upperBoundOnWidth, compensator int, nodes []*lgraph.LNode,
	nodeSuccessors []*gwt.Set[*lgraph.LNode]) (float64, [][]*lgraph.LNode) {

	layers := [][]*lgraph.LNode{}
	unplacedNodes := gwt.NewSet(nodes...)

	// One of the deviations from the paper is, that our upper bound is taking
	// node sizes into account:
	ubwConsiderSize := float64(float64(upperBoundOnWidth) * l.avgSize)

	// in- and out-degree of the currently considered node, see while-loop
	// below
	inDeg := 0
	outDeg := 0

	// The actual algorithm from the paper begins here:
	// In the Paper the first Set contains all nodes, which have already been
	// placed (in this version we consider only the nodes already placed in
	// the current layer), and the second contains all nodes already placed in
	// layers which have been determined before the currentLayer.
	alreadyPlacedInCurrentLayer := gwt.NewSet[*lgraph.LNode]()
	alreadyPlacedInOtherLayers := gwt.NewSet[*lgraph.LNode]()

	// Set up the first layer (algorithm is bottom up, so the List layer is
	// going to be reversed at the end.
	currentLayer := []*lgraph.LNode{}

	// Initial values for the width of the current layer and the estimated
	// width of the coming layers
	widthCurrent := 0.0
	widthUp := 0.0

	// Parameters needed for computing the width of a layering including dummy
	// nodes:
	maxWidth := 0.0
	realWidth := 0.0
	// Number of "started" edges that did not "finish" yet (now multiplied
	// with the normalized dummy size to consider actual node sizes)
	currentSpanningEdges := 0.0
	goingOutFromThisLayer := 0.0
	// No need for a variable "comingIntoThisLayer" as "widthUp" already gets
	// the job done.

	for !unplacedNodes.IsEmpty() {
		// Find a node, whose edges only point to nodes in the Set
		// alreadyPlacedInOtherLayers; will return {@code null} if such a node
		// doesn't exist.
		currentNode := selectNode(unplacedNodes, nodeSuccessors, alreadyPlacedInOtherLayers)

		// If a node is found in the previous step:
		if currentNode != nil {
			unplacedNodes.Remove(currentNode)
			currentLayer = append(currentLayer, currentNode)
			alreadyPlacedInCurrentLayer.Add(currentNode)

			outDeg = l.outDegree[currentNode.ID]
			// Take node sizes in account: use the normalized size of current
			// node and the normalized dummy size for each edge
			widthCurrent += l.normSize[currentNode.ID] - float64(float64(outDeg)*l.dummySize)

			inDeg = l.inDegree[currentNode.ID]
			// Take node sizes in account: use the normalized normalized dummy
			// size for each edge
			widthUp += float64(float64(inDeg) * l.dummySize)

			goingOutFromThisLayer += float64(float64(outDeg) * l.dummySize)

			realWidth += l.normSize[currentNode.ID]
		}

		// Go to the next layer if,
		// 1) no current node has been selected,
		// 2) there are no unplaced nodes left (last iteration of the
		// while-loop),
		// 3) The conditionGoUp from the paper (with the difference of ubw
		// being multiplied with the average normalized node size) is
		// satisfied, i.e.
		// 3.1) the width of the current layer is greater than the upper bound
		// on the width and the number of dummy nodes in the layer can't be
		// reduced, as only nodes with no outgoing edges are left for being
		// considered for the current layer; or:
		// 3.2) The estimated width of the not yet determined layers is
		// greater than the scaling factor/compensator times the upper bound
		// on the width.
		if currentNode == nil || unplacedNodes.IsEmpty() ||
			(widthCurrent >= ubwConsiderSize && l.normSize[currentNode.ID] > float64(outDeg)*l.dummySize) ||
			widthUp >= float64(compensator)*ubwConsiderSize {
			if currentNode == nil && len(currentLayer) == 0 {
				// No node can be placed, and the layer before was empty too:
				// the graph has a cycle. The Java loop does not end.
				lgraph.Bail("MinWidthLayerer: the graph has a cycle")
			}
			layers = append(layers, currentLayer)
			currentLayer = []*lgraph.LNode{}
			alreadyPlacedInCurrentLayer.Each(func(n *lgraph.LNode) bool {
				alreadyPlacedInOtherLayers.Add(n)
				return true
			})
			alreadyPlacedInCurrentLayer.Clear()

			// Remove all edges from the dummy node count, which are starting
			// at a node placed in this layer ...
			currentSpanningEdges -= goingOutFromThisLayer
			// ... Now we have the actual dummy node count (or rather the sum
			// of their widths) for this layer and can add it to the real
			// nodes for comparing the width.
			maxWidth = max(maxWidth, float64(currentSpanningEdges*l.dummySize)+realWidth)
			// In the next iteration we have to consider new dummy nodes from
			// edges coming into the layer we've just finished.
			currentSpanningEdges += widthUp

			widthCurrent = widthUp
			widthUp = 0
			goingOutFromThisLayer = 0
			realWidth = 0
		}
	}

	return maxWidth, layers
}

// selectNode returns the first node of the set whose outgoing edges end only
// in nodes of the set targets, without self loops. It returns nil without
// such a node.
func selectNode(nodes *gwt.Set[*lgraph.LNode], successors []*gwt.Set[*lgraph.LNode],
	targets *gwt.Set[*lgraph.LNode]) *lgraph.LNode {

	var found *lgraph.LNode
	nodes.Each(func(node *lgraph.LNode) bool {
		containsAll := true
		successors[node.ID].Each(func(successor *lgraph.LNode) bool {
			if !targets.Contains(successor) {
				containsAll = false
				return false
			}
			return true
		})
		if containsAll {
			found = node
			return false
		}
		return true
	})
	return found
}

// countIncomingEdgesExceptSelfLoops is
// countEdgesExceptSelfLoops(node.getIncomingEdges()).
func countIncomingEdgesExceptSelfLoops(node *lgraph.LNode) int {
	i := 0
	for _, port := range node.Ports {
		for _, edge := range port.IncomingEdges {
			if !selfLoopPredicate(edge) {
				i++
			}
		}
	}
	return i
}

// countOutgoingEdgesExceptSelfLoops is
// countEdgesExceptSelfLoops(node.getOutgoingEdges()).
func countOutgoingEdgesExceptSelfLoops(node *lgraph.LNode) int {
	i := 0
	for _, port := range node.Ports {
		for _, edge := range port.OutgoingEdges {
			if !selfLoopPredicate(edge) {
				i++
			}
		}
	}
	return i
}

// selfLoopPredicate is SelfLoopPredicate.apply(input): the source node of
// the edge is its target node.
func selfLoopPredicate(input *lgraph.LEdge) bool {
	return input.Source.Node() == input.Target.Node()
}

// minOutgoingEdgesComparator is MinOutgoingEdgesComparator.compare(o1, o2):
// it compares two nodes by the number of their outgoing edges.
func (l *MinWidthLayerer) minOutgoingEdgesComparator(o1, o2 *lgraph.LNode) int {
	outs1 := l.outDegree[o1.ID]
	outs2 := l.outDegree[o2.ID]

	if outs1 < outs2 {
		return -1
	}
	if outs1 == outs2 {
		return 0
	}
	return 1
}
