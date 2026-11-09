// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/LabelDummySwitcher.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package labels

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("LabelDummySwitcher", func() lgraph.Processor { return NewLabelDummySwitcher() })
}

// IncludeLabel is LabelDummySwitcher.INCLUDE_LABEL: a property for the
// analyses that can run on a graph.
var IncludeLabel = prop.NewDefault("edgelabelcenterednessanalysis.includelabel", false)

// LabelDummySwitcher is the processor LabelDummySwitcher: it moves each label
// dummy node to the best layer that its long edge crosses. To do so it swaps
// the label dummy with a long edge dummy.
type LabelDummySwitcher struct {
	// layerWidths has the width of each layer, by layer id.
	layerWidths []float64
}

// NewLabelDummySwitcher is new LabelDummySwitcher().
func NewLabelDummySwitcher() *LabelDummySwitcher { return &LabelDummySwitcher{} }

// usesLabelSizeInformation is
// CenterEdgeLabelPlacementStrategy.usesLabelSizeInformation().
func usesLabelSizeInformation(strategy lenum.CenterEdgeLabelPlacementStrategy) bool {
	return strategy == lenum.CenterEdgeLabelPlacementStrategyWidestLayer ||
		strategy == lenum.CenterEdgeLabelPlacementStrategyCenterLayer ||
		strategy == lenum.CenterEdgeLabelPlacementStrategySpaceEfficientLayer
}

// Process is process(layeredGraph, monitor).
func (p *LabelDummySwitcher) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	// The default placement strategy to be assumed if no more specific
	// strategy is
	defaultPlacementStrategy := prop.Get(layeredGraph, lopts.EdgeLabelsCenterLabelPlacementStrategy)

	// Assign layer IDs
	p.assignIdsToLayers(layeredGraph)

	// Collect label dummy infos by placement strategy
	labelDummyInfos := p.gatherLabelDummyInfos(layeredGraph, defaultPlacementStrategy)

	// If at least one width-based strategy is active, calculate layers
	// widths. After this point, layer widths are either all zero or equal to
	// each layer's widest non-dummy node
	p.layerWidths = make([]float64, len(layeredGraph.Layers))
	// Reset data
	defer func() { p.layerWidths = nil }()

	strategies := lenum.ValuesCenterEdgeLabelPlacementStrategy()
	for _, strategy := range strategies {
		if usesLabelSizeInformation(strategy) && len(labelDummyInfos[strategy]) != 0 {
			// We have found an active width-based strategy
			p.calculateLayerWidths(layeredGraph)
			break
		}
	}

	// Work through the non-width-based strategies first. They might change
	// the width of layers, which might influence size-based strategies later
	// on
	for _, strategy := range strategies {
		if !usesLabelSizeInformation(strategy) {
			p.processStrategy(labelDummyInfos[strategy])
		}
	}

	// Now execute size-based strategies
	for _, strategy := range strategies {
		if usesLabelSizeInformation(strategy) {
			p.processStrategy(labelDummyInfos[strategy])
		}
	}
}

// assignIdsToLayers gives the layers of the graph ids from zero.
func (*LabelDummySwitcher) assignIdsToLayers(layeredGraph *lgraph.LGraph) {
	layerIndex := 0
	for _, layer := range layeredGraph.Layers {
		layer.ID = layerIndex
		layerIndex++
	}
}

// gatherLabelDummyInfos makes a labelDummyInfo for each label dummy of the
// graph and returns them by placement strategy. The index of the result is
// the ordinal of the strategy.
func (*LabelDummySwitcher) gatherLabelDummyInfos(layeredGraph *lgraph.LGraph, defaultPlacementStrategy lenum.CenterEdgeLabelPlacementStrategy) [][]*labelDummyInfo {
	// Initialize the map with empty lists
	infos := make([][]*labelDummyInfo, len(lenum.ValuesCenterEdgeLabelPlacementStrategy()))

	// From the graph's nodes, filter out the label dummies and add them to
	// the appropriate lists
	for _, layer := range layeredGraph.Layers {
		for _, node := range layer.Nodes {
			if node.Type != lgraph.NodeTypeLabel {
				continue
			}
			dummyInfo := newLabelDummyInfo(node, defaultPlacementStrategy)
			strategy := int(dummyInfo.placementStrategy)
			if strategy < 0 || strategy >= len(infos) {
				lgraph.Bail("LabelDummySwitcher: unknown center label placement strategy %d", strategy)
			}
			infos[strategy] = append(infos[strategy], dummyInfo)
		}
	}

	return infos
}

// calculateLayerWidths fills the layer widths, by the layer ids that
// assignIdsToLayers gave.
func (p *LabelDummySwitcher) calculateLayerWidths(layeredGraph *lgraph.LGraph) {
	for _, layer := range layeredGraph.Layers {
		p.layerWidths[layer.ID] = lgraph.FindMaxNonDummyNodeWidth(layer, false)
	}
}

// processStrategy runs one strategy on its label dummies. The strategy
// SPACE_EFFICIENT_LAYER places all dummies at once.
func (p *LabelDummySwitcher) processStrategy(labelDummyInfos []*labelDummyInfo) {
	// There might not be anything to do for this strategy
	if len(labelDummyInfos) == 0 {
		return
	}

	// Check if the strategy has a special processing method
	if labelDummyInfos[0].placementStrategy == lenum.CenterEdgeLabelPlacementStrategySpaceEfficientLayer {
		p.computeSpaceEfficientAssignment(labelDummyInfos)
		return
	}

	// Execute the strategy for each label dummy
	for _, info := range labelDummyInfos {
		switch info.placementStrategy {
		case lenum.CenterEdgeLabelPlacementStrategyCenterLayer:
			p.assignLayer(info, p.findCenterLayerTargetId(info))

		case lenum.CenterEdgeLabelPlacementStrategyMedianLayer:
			p.assignLayer(info, p.findMedianLayerTargetId(info))

		case lenum.CenterEdgeLabelPlacementStrategyWidestLayer:
			p.assignLayer(info, p.findWidestLayerTargetId(info))

		case lenum.CenterEdgeLabelPlacementStrategyHeadLayer:
			p.setEndLayerNodeAlignment(info)
			p.assignLayer(info, p.findEndLayerTargetId(info, true))

		case lenum.CenterEdgeLabelPlacementStrategyTailLayer:
			p.setEndLayerNodeAlignment(info)
			p.assignLayer(info, p.findEndLayerTargetId(info, false))
		}

		p.updateLongEdgeSourceLabelDummyInfo(info)
	}
}

// Widest Layer

// findWidestLayerTargetId finds the layer for a label dummy: the widest of
// the layers that the long edge has a dummy in.
func (p *LabelDummySwitcher) findWidestLayerTargetId(info *labelDummyInfo) int {
	// Find the widest layer among those the long edge dummies are placed in
	widestLayerIndex := info.leftmostLayerId

	for index := widestLayerIndex + 1; index <= info.rightmostLayerId; index++ {
		if p.width(index) > p.width(widestLayerIndex) {
			widestLayerIndex = index
		}
	}

	// Return the dummy node in the widest layer
	return widestLayerIndex
}

// Center Layer

// findCenterLayerTargetId finds the layer for a label dummy: the layer that
// is expected to be closest to the physical center of the edge.
func (p *LabelDummySwitcher) findCenterLayerTargetId(info *labelDummyInfo) int {
	// Sum up the widths of all the layers this thing spans
	layerWidthSums := p.computeLayerWidthSums(info)
	if len(layerWidthSums) == 0 {
		lgraph.Bail("LabelDummySwitcher: a label dummy that spans no layer")
	}

	// Find the first layer that exceeds half the width
	threshold := layerWidthSums[len(layerWidthSums)-1] / 2

	for i := 0; i < len(layerWidthSums); i++ {
		if layerWidthSums[i] >= threshold {
			return info.leftmostLayerId + i
		}
	}

	// This should actually not happen
	return info.leftmostLayerId + len(info.leftLongEdgeDummies)
}

// computeLayerWidthSums returns an array in which the entry i is the combined
// width of the layers [0, i] of the long edge, with a guess of the layer
// spacing.
func (p *LabelDummySwitcher) computeLayerWidthSums(info *labelDummyInfo) []float64 {
	// The minimum space that we think will be left between
	graph := info.labelDummy.Graph()
	edgeNodeSpacing := prop.Get(graph, lopts.SpacingEdgeNodeBetweenLayers) * 2
	nodeNodeSpacing := prop.Get(graph, lopts.SpacingNodeNodeBetweenLayers)
	minSpaceBetweenLayers := max(edgeNodeSpacing, nodeNodeSpacing)

	// The array that will hold the accumulated widths
	count := info.totalDummyCount()
	if count != len(info.leftLongEdgeDummies)+1+len(info.rightLongEdgeDummies) {
		lgraph.Bail("LabelDummySwitcher: the dummies of a long edge are not in consecutive layers")
	}
	layerWidthSums := make([]float64, count)

	currentWidthSum := -minSpaceBetweenLayers
	currentIndex := 0

	for _, leftDummy := range info.leftLongEdgeDummies {
		currentWidthSum += p.width(layerID(leftDummy)) + minSpaceBetweenLayers
		layerWidthSums[currentIndex] = currentWidthSum
		currentIndex++
	}

	currentWidthSum += p.width(layerID(info.labelDummy)) + minSpaceBetweenLayers
	layerWidthSums[currentIndex] = currentWidthSum
	currentIndex++

	for _, rightDummy := range info.rightLongEdgeDummies {
		currentWidthSum += p.width(layerID(rightDummy)) + minSpaceBetweenLayers
		layerWidthSums[currentIndex] = currentWidthSum
		currentIndex++
	}

	return layerWidthSums
}

// Median Layer

// findMedianLayerTargetId finds the layer for a label dummy: the median of
// the layers that the long edge spans.
func (*LabelDummySwitcher) findMedianLayerTargetId(info *labelDummyInfo) int {
	// Find the median of the layers spanned by the long edge this label
	// dummy is part of
	layers := info.totalDummyCount()
	lowerMedian := (layers - 1) / 2

	return info.leftmostLayerId + lowerMedian
}

// End Layer

// findEndLayerTargetId finds the layer for a label dummy: the layer of the
// dummy that is closest to the head or the tail of the original edge.
func (p *LabelDummySwitcher) findEndLayerTargetId(info *labelDummyInfo, headLayer bool) int {
	reversed := p.isPartOfReversedEdge(info)

	if (headLayer && !reversed) || (!headLayer && reversed) {
		return info.rightmostLayerId
	}
	return info.leftmostLayerId
}

// setEndLayerNodeAlignment sets the alignment of a label dummy whose strategy
// is HEAD_LAYER or TAIL_LAYER.
func (p *LabelDummySwitcher) setEndLayerNodeAlignment(info *labelDummyInfo) {
	isHeadLabel := info.placementStrategy == lenum.CenterEdgeLabelPlacementStrategyHeadLayer
	isPartOfReversedEdge := p.isPartOfReversedEdge(info)

	if (isHeadLabel && !isPartOfReversedEdge) || (!isHeadLabel && isPartOfReversedEdge) {
		prop.Set(info.labelDummy, lopts.Alignment, core.AlignmentRight)
	} else {
		prop.Set(info.labelDummy, lopts.Alignment, core.AlignmentLeft)
	}
}

// isPartOfReversedEdge tells that the label dummy is part of a reversed edge.
func (*LabelDummySwitcher) isPartOfReversedEdge(info *labelDummyInfo) bool {
	// Find incoming and outgoing edge
	incoming := firstIncomingEdge(info.labelDummy)
	outgoing := firstOutgoingEdge(info.labelDummy)

	return prop.Get(incoming, iprops.Reversed) || prop.Get(outgoing, iprops.Reversed)
}

// Space Efficient

// computeSpaceEfficientAssignment runs a heuristic that looks for the label
// dummy to layer assignment with the least width.
func (p *LabelDummySwitcher) computeSpaceEfficientAssignment(labelDummyInfos []*labelDummyInfo) {
	// We start by assigning all label dummies that only have a single layer
	// to choose from or that can be assigned to a layer large enough for them
	// and removing them from our list
	nonTrivialLabels := p.performTrivialAssignments(labelDummyInfos)
	if len(nonTrivialLabels) == 0 {
		return
	}

	// The remaining labels are not as easy to assign. Sort descendingly by
	// size.
	gwt.SortFunc(nonTrivialLabels, func(info1, info2 *labelDummyInfo) int {
		return gwt.DoubleCompare(info2.labelDummy.Size.X, info1.labelDummy.Size.X)
	})

	labelCount := len(nonTrivialLabels)
	for labelIndex := 0; labelIndex < labelCount; labelIndex++ {
		p.assignLayer(nonTrivialLabels[labelIndex], p.findPotentiallyWidestLayer(nonTrivialLabels, labelIndex))
	}
}

// performTrivialAssignments assigns each label dummy that has one layer only,
// or that fits a layer without making it wider. It returns the other label
// dummies.
func (p *LabelDummySwitcher) performTrivialAssignments(labelDummyInfos []*labelDummyInfo) []*labelDummyInfo {
	remainingLabels := make([]*labelDummyInfo, 0, len(labelDummyInfos))

	for _, info := range labelDummyInfos {
		if info.leftmostLayerId == info.rightmostLayerId {
			// Assign to only available layer and remove from list
			p.assignLayer(info, info.leftmostLayerId)
		} else if !p.assignToWiderLayer(info) {
			// Ending up here means that we didn't find a layer wide enough
			// for the node
			remainingLabels = append(remainingLabels, info)
		}
	}

	return remainingLabels
}

// assignToWiderLayer assigns the label dummy to the first layer that is wide
// enough for it. It returns false when there is no such layer.
func (p *LabelDummySwitcher) assignToWiderLayer(info *labelDummyInfo) bool {
	// Check if the label dummy can be assigned a layer that already is at
	// least as wide as the dummy
	dummyWidth := info.labelDummy.Size.X
	layers := info.labelDummy.Graph().Layers
	from, to := info.leftmostLayerId, info.rightmostLayerId+1
	if from < 0 || to > len(layers) || from > to {
		lgraph.Bail("LabelDummySwitcher: the layers %d to %d of a long edge are not in the graph", from, to)
	}

	for _, layer := range layers[from:to] {
		if layer.Size.X >= dummyWidth {
			p.assignLayer(info, layer.ID)
			return true
		}
	}

	// Ending up here means that we didn't find a layer wide enough for our
	// label
	return false
}

// findPotentiallyWidestLayer returns the index of the layer with the largest
// potential width for the label dummy with the index labelIndex. The label
// dummy infos are sorted by label width, the widest first, and the label
// dummies before labelIndex have their layers.
func (p *LabelDummySwitcher) findPotentiallyWidestLayer(labelDummyInfos []*labelDummyInfo, labelIndex int) int {
	labelCount := len(labelDummyInfos)
	info := labelDummyInfos[labelIndex]
	labelDummyWidth := info.labelDummy.Size.X

	// Iterate over the label's valid layers
	widestLayerIndex := info.leftmostLayerId
	widestLayerWidth := 0.0

	for layer := info.leftmostLayerId; layer <= info.rightmostLayerId; layer++ {
		// If the layer is already at least as large as the current label,
		// simply return that layer
		if labelDummyWidth <= p.width(layer) {
			return layer
		}

		// The initial potential width is less wide than the label (otherwise
		// we would have already returned)
		potentialWidth := p.width(layer)

		// Find the largest unassigned label that is part of this layer
		var largestUnassignedLabel *labelDummyInfo
		for label := labelIndex + 1; label < labelCount; label++ {
			// Check if the label can be placed in the current layer
			currLabelInfo := labelDummyInfos[label]
			if currLabelInfo.leftmostLayerId <= layer && currLabelInfo.rightmostLayerId >= layer {
				largestUnassignedLabel = currLabelInfo
			}
		}

		// Update layer's potential size
		if largestUnassignedLabel != nil {
			potentialWidth = max(potentialWidth, largestUnassignedLabel.labelDummy.Size.X)
		}

		// Update widest layer (if there are multiple widest layers, we use
		// the leftmost one)
		if potentialWidth > widestLayerWidth {
			widestLayerIndex = layer
			widestLayerWidth = potentialWidth
		}
	}

	return widestLayerIndex
}

// Swapping Utilities

// width returns the width of the layer with the given id. Java fails for an
// id that is not the index of a layer.
func (p *LabelDummySwitcher) width(id int) float64 {
	if id < 0 || id >= len(p.layerWidths) {
		lgraph.Bail("LabelDummySwitcher: no layer with the id %d", id)
	}
	return p.layerWidths[id]
}

// assignLayer moves the label dummy to the layer with the given index, and
// makes the width of that layer at least the width of the label dummy.
func (p *LabelDummySwitcher) assignLayer(info *labelDummyInfo, targetLayerIndex int) {
	// If the label dummy is not in the target layer yet, swap it with the
	// long edge dummy that is
	if targetLayerIndex != info.leftmostLayerId+len(info.leftLongEdgeDummies) {
		p.swapNodes(info.labelDummy, info.ithDummyNode(targetLayerIndex-info.leftmostLayerId))
	}

	// Update the size information of the label dummy's new layer
	newLayerId := layerID(info.labelDummy)
	p.layerWidths[newLayerId] = max(p.width(newLayerId), info.labelDummy.Size.X)

	for _, label := range prop.Get(info.labelDummy, iprops.RepresentedLabels) {
		prop.Set(label, IncludeLabel, true)
	}
}

// swapNodes swaps the label dummy with a long edge dummy of the same long
// edge. Each node keeps the position in the node list of its new layer that
// the other node had: the processor runs after the crossing minimization.
func (*LabelDummySwitcher) swapNodes(labelDummy, longEdgeDummy *lgraph.LNode) {
	// Find the layers and the positions inside the layers of the dummy nodes
	layer1 := labelDummy.Layer()
	layer2 := longEdgeDummy.Layer()
	if layer1 == nil || layer2 == nil {
		lgraph.Bail("LabelDummySwitcher: a dummy node without a layer")
	}

	dummy1LayerPosition := labelDummy.Index()
	dummy2LayerPosition := longEdgeDummy.Index()

	// Detect incoming and outgoing ports of the nodes (this of course
	// assumes that there's just one of each kind, which should be true for
	// long edge and label dummy nodes)
	inputPort1 := firstPort(labelDummy, lenum.PortTypeInput)
	outputPort1 := firstPort(labelDummy, lenum.PortTypeOutput)
	inputPort2 := firstPort(longEdgeDummy, lenum.PortTypeInput)
	outputPort2 := firstPort(longEdgeDummy, lenum.PortTypeOutput)

	// Store incoming and outgoing edges
	incomingEdges1 := append([]*lgraph.LEdge(nil), inputPort1.IncomingEdges...)
	outgoingEdges1 := append([]*lgraph.LEdge(nil), outputPort1.OutgoingEdges...)
	incomingEdges2 := append([]*lgraph.LEdge(nil), inputPort2.IncomingEdges...)
	outgoingEdges2 := append([]*lgraph.LEdge(nil), outputPort2.OutgoingEdges...)

	// Put first dummy into second dummy's layer and reroute second dummy's
	// edges to first dummy
	setLayerAt(labelDummy, dummy2LayerPosition, layer2)
	for _, edge := range incomingEdges2 {
		edge.SetTarget(inputPort1)
	}
	for _, edge := range outgoingEdges2 {
		edge.SetSource(outputPort1)
	}

	// Put second dummy into first dummy's layer and reroute first dummy's
	// edges to second dummy
	setLayerAt(longEdgeDummy, dummy1LayerPosition, layer1)
	for _, edge := range incomingEdges1 {
		edge.SetTarget(inputPort2)
	}
	for _, edge := range outgoingEdges1 {
		edge.SetSource(outputPort2)
	}
}

// setLayerAt is LNode.setLayer(index, layer). Java throws an
// IllegalArgumentException for an index outside the node list.
func setLayerAt(node *lgraph.LNode, index int, layer *lgraph.Layer) {
	if index < 0 || index > len(layer.Nodes) {
		lgraph.Bail("LabelDummySwitcher: index must be >= 0 and <= layer node count")
	}
	node.SetLayerAt(index, layer)
}

// firstPort is node.getPorts(portType).iterator().next(). Java throws a
// NoSuchElementException for a node without such a port.
func firstPort(node *lgraph.LNode, portType lenum.PortType) *lgraph.LPort {
	for _, port := range node.Ports {
		if portType == lenum.PortTypeInput && len(port.IncomingEdges) != 0 ||
			portType == lenum.PortTypeOutput && len(port.OutgoingEdges) != 0 {
			return port
		}
	}
	lgraph.Bail("LabelDummySwitcher: a dummy node without an input port or an output port")
	return nil
}

// firstIncomingEdge is node.getIncomingEdges().iterator().next(). Java throws
// a NoSuchElementException for a node without incoming edges.
func firstIncomingEdge(node *lgraph.LNode) *lgraph.LEdge {
	for _, port := range node.Ports {
		if len(port.IncomingEdges) != 0 {
			return port.IncomingEdges[0]
		}
	}
	lgraph.Bail("label dummies: a dummy node without an incoming edge")
	return nil
}

// firstOutgoingEdge is node.getOutgoingEdges().iterator().next(). Java throws
// a NoSuchElementException for a node without outgoing edges.
func firstOutgoingEdge(node *lgraph.LNode) *lgraph.LEdge {
	for _, port := range node.Ports {
		if len(port.OutgoingEdges) != 0 {
			return port.OutgoingEdges[0]
		}
	}
	lgraph.Bail("label dummies: a dummy node without an outgoing edge")
	return nil
}

// layerID is node.getLayer().id. Java throws a NullPointerException for a
// node without a layer.
func layerID(node *lgraph.LNode) int {
	layer := node.Layer()
	if layer == nil {
		lgraph.Bail("LabelDummySwitcher: a dummy node without a layer")
	}
	return layer.ID
}

// updateLongEdgeSourceLabelDummyInfo sets the property
// LONG_EDGE_BEFORE_LABEL_DUMMY of the long edge dummies before the label
// dummy.
func (p *LabelDummySwitcher) updateLongEdgeSourceLabelDummyInfo(info *labelDummyInfo) {
	// Predecessors
	p.doUpdateLongEdgeLabelDummyInfo(
		info.labelDummy,
		func(node *lgraph.LNode) *lgraph.LNode { return sourceNode(firstIncomingEdge(node)) },
		true)

	// We may want to do things to the successors as well at some point
}

// doUpdateLongEdgeLabelDummyInfo sets the property of each long edge dummy
// that nextElement reaches from the label dummy.
func (*LabelDummySwitcher) doUpdateLongEdgeLabelDummyInfo(labelDummy *lgraph.LNode, nextElement func(*lgraph.LNode) *lgraph.LNode, value bool) {
	// A chain of dummies that is a cycle has no end. Java does not stop.
	steps := 0
	longEdgeDummy := nextElement(labelDummy)
	for longEdgeDummy.Type == lgraph.NodeTypeLongEdge {
		prop.Set(longEdgeDummy, iprops.LongEdgeBeforeLabelDummy, value)
		longEdgeDummy = nextElement(longEdgeDummy)
		if steps++; steps > maxChainSteps {
			lgraph.Bail("LabelDummySwitcher: the long edge dummies of a label dummy make a cycle")
		}
	}
}

// maxChainSteps bounds a walk along the dummy nodes of one long edge.
const maxChainSteps = 1 << 20

// sourceNode is edge.getSource().getNode().
func sourceNode(edge *lgraph.LEdge) *lgraph.LNode {
	if edge.Source == nil || edge.Source.Node() == nil {
		lgraph.Bail("label dummies: an edge without a source node")
	}
	return edge.Source.Node()
}

// targetNode is edge.getTarget().getNode().
func targetNode(edge *lgraph.LEdge) *lgraph.LNode {
	if edge.Target == nil || edge.Target.Node() == nil {
		lgraph.Bail("label dummies: an edge without a target node")
	}
	return edge.Target.Node()
}

// Label Dummy Info Class

// labelDummyInfo is LabelDummySwitcher.LabelDummyInfo: a label dummy with the
// long edge dummies on its left and on its right.
type labelDummyInfo struct {
	// labelDummy is the label dummy node.
	labelDummy *lgraph.LNode
	// placementStrategy is the label placement strategy of this label.
	placementStrategy lenum.CenterEdgeLabelPlacementStrategy
	// leftLongEdgeDummies has the long edge dummies on the left of the label
	// dummy, in the order of the layers.
	leftLongEdgeDummies []*lgraph.LNode
	// rightLongEdgeDummies has the long edge dummies on the right of the
	// label dummy, in the order of the layers.
	rightLongEdgeDummies []*lgraph.LNode
	// leftmostLayerId is the id of the first layer in which the long edge of
	// the label dummy has a dummy node.
	leftmostLayerId int
	// rightmostLayerId is the id of the last layer in which the long edge of
	// the label dummy has a dummy node.
	rightmostLayerId int
}

// newLabelDummyInfo is new LabelDummyInfo(labelDummy,
// defaultPlacementStrategy). It collects the long edge dummies on both sides.
func newLabelDummyInfo(labelDummy *lgraph.LNode, defaultPlacementStrategy lenum.CenterEdgeLabelPlacementStrategy) *labelDummyInfo {
	info := &labelDummyInfo{labelDummy: labelDummy, placementStrategy: defaultPlacementStrategy}

	// Gather long edge dummies that are part of the label dummy's edge and
	// find the end layer IDs
	info.gatherLeftLongEdgeDummies()
	info.gatherRightLongEdgeDummies()

	if len(info.leftLongEdgeDummies) == 0 {
		info.leftmostLayerId = layerID(labelDummy)
	} else {
		info.leftmostLayerId = layerID(info.leftLongEdgeDummies[0])
	}

	if len(info.rightLongEdgeDummies) == 0 {
		info.rightmostLayerId = layerID(labelDummy)
	} else {
		info.rightmostLayerId = layerID(info.rightLongEdgeDummies[len(info.rightLongEdgeDummies)-1])
	}

	// Check if the label wants to deviate from the default placement strategy
	if !prop.Has(labelDummy, iprops.RepresentedLabels) {
		lgraph.Bail("LabelDummySwitcher: a label dummy without represented labels")
	}
	for _, label := range prop.Get(labelDummy, iprops.RepresentedLabels) {
		// Take the first override we can find
		if prop.Has(label, lopts.EdgeLabelsCenterLabelPlacementStrategy) {
			info.placementStrategy = prop.Get(label, lopts.EdgeLabelsCenterLabelPlacementStrategy)
			break
		}
	}
	return info
}

// gatherLeftLongEdgeDummies collects the long edge dummies on the left of the
// label dummy that are part of its long edge, in the order of the layers.
func (info *labelDummyInfo) gatherLeftLongEdgeDummies() {
	source := info.labelDummy
	for {
		source = sourceNode(firstIncomingEdge(source))
		if source.Type != lgraph.NodeTypeLongEdge {
			break
		}
		info.leftLongEdgeDummies = append(info.leftLongEdgeDummies, source)
		if len(info.leftLongEdgeDummies) > maxChainSteps {
			lgraph.Bail("LabelDummySwitcher: the long edge dummies of a label dummy make a cycle")
		}
	}

	// The list is currently not in the order we would expect, so produce a
	// reversed version
	for i, j := 0, len(info.leftLongEdgeDummies)-1; i < j; i, j = i+1, j-1 {
		info.leftLongEdgeDummies[i], info.leftLongEdgeDummies[j] = info.leftLongEdgeDummies[j], info.leftLongEdgeDummies[i]
	}
}

// gatherRightLongEdgeDummies collects the long edge dummies on the right of
// the label dummy that are part of its long edge, in the order of the layers.
func (info *labelDummyInfo) gatherRightLongEdgeDummies() {
	target := info.labelDummy
	for {
		target = targetNode(firstOutgoingEdge(target))
		if target.Type != lgraph.NodeTypeLongEdge {
			break
		}
		info.rightLongEdgeDummies = append(info.rightLongEdgeDummies, target)
		if len(info.rightLongEdgeDummies) > maxChainSteps {
			lgraph.Bail("LabelDummySwitcher: the long edge dummies of a label dummy make a cycle")
		}
	}
}

// totalDummyCount returns the number of long edge dummies and label dummies
// along the edge of the label dummy.
func (info *labelDummyInfo) totalDummyCount() int {
	return info.rightmostLayerId - info.leftmostLayerId + 1
}

// ithDummyNode returns the i-th dummy node of the edge through the long edge
// dummies and the label dummy. Java throws for an index outside the lists.
func (info *labelDummyInfo) ithDummyNode(i int) *lgraph.LNode {
	left := len(info.leftLongEdgeDummies)
	switch {
	case i < 0 || i-left-1 >= len(info.rightLongEdgeDummies):
		lgraph.Bail("LabelDummySwitcher: no dummy node with the index %d on a long edge", i)
		return nil
	case i < left:
		// The i-th dummy is a long edge dummy to the label dummy's left
		return info.leftLongEdgeDummies[i]
	case i == left:
		// The i-th dummy is the label dummy itself
		return info.labelDummy
	default:
		// the i-th dummy is a long edge dummy to the label dummy's right
		return info.rightLongEdgeDummies[i-left-1]
	}
}
