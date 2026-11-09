// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p4nodes/bk/BKNodePlacer.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package bk

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("BKNodePlacer", func() lgraph.Processor { return NewBKNodePlacer() })
}

// BKNodePlacer is the phase BKNodePlacer: the node placement of Brandes and
// Koepf, with the extensions of ELK for ports, node sizes and node margins.
//
// The names of the directions are for a layout from the left to the right:
// the layers are columns, and the placer gives each node its y coordinate.
//
// The placer has five steps. The first step marks the type 1 conflicts: a
// short edge that crosses a long edge. The next three steps run for each of
// the four combinations of UP or DOWN and LEFT or RIGHT: the vertical
// alignment groups the nodes into blocks, the inside block shift places the
// nodes inside each block, and the horizontal compaction places the blocks.
// The last step takes one of the four layouts, or the median of them for a
// balanced layout.
//
// The graph must be layered with ordered nodes and arranged ports. The placer
// sets the y coordinate of each node.
//
// getLayoutProcessorConfiguration is in the package layered, with the
// configurations of the other phases.
type BKNodePlacer struct {
	lGraph *lgraph.LGraph
	// markedEdges has the edges of type 1 conflicts.
	markedEdges map[*lgraph.LEdge]struct{}
	// ni has the neighbors of each node.
	ni *NeighborhoodInformation
	// produceBalancedLayout tells whether to produce a balanced layout.
	produceBalancedLayout bool
}

// NewBKNodePlacer is new BKNodePlacer().
func NewBKNodePlacer() *BKNodePlacer {
	return &BKNodePlacer{markedEdges: map[*lgraph.LEdge]struct{}{}}
}

// Process is process(layeredGraph, monitor).
func (p *BKNodePlacer) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	p.lGraph = layeredGraph

	// Precalculate some information that we require during the
	// following processes.
	p.ni = buildFor(layeredGraph)

	// a balanced layout is desired if
	//  a) no specific alignment is set and straight edges are not desired
	//  b) a balanced alignment is enforced
	align := prop.Get(layeredGraph, lopts.NodePlacementBkFixedAlignment)
	if !prop.Has(layeredGraph, lopts.NodePlacementFavorStraightEdges) {
		// The option has no default value. The Java code fails with a
		// NullPointerException when it unboxes null.
		lgraph.Bail("BKNodePlacer: the option favorStraightEdges is null")
	}
	favorStraightEdges := prop.Get(layeredGraph, lopts.NodePlacementFavorStraightEdges)
	p.produceBalancedLayout = (align == lenum.FixedAlignmentNone && !favorStraightEdges) ||
		align == lenum.FixedAlignmentBalanced

	// Phase which marks type 1 conflicts, no difference between the directions so only
	// one run is required.
	p.markConflicts(layeredGraph)

	// Initialize four layouts which result from the two possible directions respectively.
	layouts := make([]*BKAlignedLayout, 0, 4)
	switch prop.Get(layeredGraph, lopts.NodePlacementBkFixedAlignment) {
	case lenum.FixedAlignmentLeftdown:
		leftdown := NewBKAlignedLayout(layeredGraph, p.ni.nodeCount, VDirectionDown, HDirectionLeft)
		layouts = append(layouts, leftdown)
	case lenum.FixedAlignmentLeftup:
		leftup := NewBKAlignedLayout(layeredGraph, p.ni.nodeCount, VDirectionUp, HDirectionLeft)
		layouts = append(layouts, leftup)
	case lenum.FixedAlignmentRightdown:
		rightdown := NewBKAlignedLayout(layeredGraph, p.ni.nodeCount, VDirectionDown, HDirectionRight)
		layouts = append(layouts, rightdown)
	case lenum.FixedAlignmentRightup:
		rightup := NewBKAlignedLayout(layeredGraph, p.ni.nodeCount, VDirectionUp, HDirectionRight)
		layouts = append(layouts, rightup)
	default:
		leftdown := NewBKAlignedLayout(layeredGraph, p.ni.nodeCount, VDirectionDown, HDirectionLeft)
		leftup := NewBKAlignedLayout(layeredGraph, p.ni.nodeCount, VDirectionUp, HDirectionLeft)
		rightdown := NewBKAlignedLayout(layeredGraph, p.ni.nodeCount, VDirectionDown, HDirectionRight)
		rightup := NewBKAlignedLayout(layeredGraph, p.ni.nodeCount, VDirectionUp, HDirectionRight)
		layouts = append(layouts, rightdown)
		layouts = append(layouts, rightup)
		layouts = append(layouts, leftdown)
		layouts = append(layouts, leftup)
	}

	aligner := NewBKAligner(layeredGraph, p.ni)
	for _, bal := range layouts {
		// Phase which determines the nodes' memberships in blocks. This happens in four different
		// ways, either from processing the nodes from the first layer to the last or vice versa.
		aligner.verticalAlignment(bal, p.markedEdges)

		// Additional phase which is not included in the original Brandes-Koepf Algorithm.
		// It makes sure that the connected ports within a block are aligned to avoid unnecessary
		// bend points. Also, the required size of each block is determined.
		aligner.insideBlockShift(bal)
	}

	var compacter ICompactor = NewBKCompactor(layeredGraph, p.ni)
	for _, bal := range layouts {
		// This phase determines the y coordinates of the blocks and thus the vertical coordinates
		// of all nodes.
		compacter.horizontalCompaction(bal)
	}

	// Choose a layout from the four calculated layouts. Layouts that contain errors are skipped.
	// The layout with the smallest size is selected. If more than one smallest layout exists,
	// the first one of the competing layouts is selected.
	var chosenLayout *BKAlignedLayout

	// If layout options chose to use the balanced layout, it is calculated and added here.
	// If it is broken for any reason, one of the four other layouts is selected by the
	// given criteria.
	if p.produceBalancedLayout {
		balanced := p.createBalancedLayout(layouts, p.ni.nodeCount)
		if p.checkOrderConstraint(layeredGraph, balanced) {
			chosenLayout = balanced
		}
	}

	// Either if no balanced layout is requested, or, if the balanced layout
	// violates order constraints, pick the one with the smallest height
	if chosenLayout == nil {
		for _, bal := range layouts {
			if p.checkOrderConstraint(layeredGraph, bal) {
				if chosenLayout == nil || chosenLayout.layoutSize() > bal.layoutSize() {
					chosenLayout = bal
				}
			}
		}
	}

	// If no layout is correct (which should never happen but is not strictly impossible),
	// the RIGHTDOWN layout is chosen by default.
	if chosenLayout == nil {
		chosenLayout = layouts[0] // there has to be at least one layout in the list
	}

	// Apply calculated positions to nodes.
	for _, layer := range layeredGraph.Layers {
		for _, node := range layer.Nodes {
			node.Pos.Y = chosenLayout.yAt(node.ID) + chosenLayout.innerShift[node.ID]
		}
	}

	// cleanup
	for _, bal := range layouts {
		bal.cleanup()
	}
	p.ni.cleanup()
	clear(p.markedEdges)
}

/////////////////////////////////////////////////////////////////////////////////////////////////////
// Conflict Detection

// minLayersForConflicts is MIN_LAYERS_FOR_CONFLICTS: the minimum number of
// layers we need to have conflicts.
const minLayersForConflicts = 3

// markConflicts is markConflicts(layeredGraph): it marks the type 1
// conflicts.
//
// An inner segment is an edge between two dummy nodes, a part of a long edge.
// A type 1 conflict is a crossing of an inner segment and an edge that is not
// an inner segment. The marks later solve the conflicts in favor of the long
// edges.
func (p *BKNodePlacer) markConflicts(layeredGraph *lgraph.LGraph) {
	numberOfLayers := len(layeredGraph.Layers)

	// Check if there are enough layers to detect conflicts
	if numberOfLayers < minLayersForConflicts {
		return
	}

	// We'll need the number of nodes in the different layers quite often in this method, so save
	// them up front
	layerSize := make([]int, numberOfLayers)
	layerIndex := 0
	for _, layer := range layeredGraph.Layers {
		layerSize[layerIndex] = len(layer.Nodes)
		layerIndex++
	}

	// The layer iterator of the Java code starts at the index 2.
	for i := 1; i < numberOfLayers-1; i++ {
		// The variable naming here follows the notation of the corresponding paper
		currentLayer := layeredGraph.Layers[i+1]

		k0 := 0
		l := 0

		for l1 := 0; l1 < layerSize[i+1]; l1++ {
			// In the paper, l and i are indices for the layer and the position in the layer
			vli := currentLayer.Nodes[l1]

			if l1 == ((layerSize[i+1])-1) || p.incidentToInnerSegment(vli, i+1, i) {
				k1 := layerSize[i] - 1
				if p.incidentToInnerSegment(vli, i+1, i) {
					leftNeighbors := p.ni.leftNeighbors[vli.ID]
					if len(leftNeighbors) == 0 {
						// The Java code fails with an IndexOutOfBoundsException.
						lgraph.Bail("BKNodePlacer: a node of an inner segment has no left neighbor")
					}
					k1 = p.ni.nodeIndex[leftNeighbors[0].first.ID]
				}

				for l <= l1 {
					vl := currentLayer.Nodes[l]

					if !p.incidentToInnerSegment(vl, i+1, i) {
						for _, upperNeighbor := range p.ni.leftNeighbors[vl.ID] {
							k := p.ni.nodeIndex[upperNeighbor.first.ID]

							if k < k0 || k > k1 {
								// Marked edge can't return null here, because the upper neighbor
								// relationship between v_l and upperNeighbor enforces the existence
								// of at least one edge between the two nodes
								p.markedEdges[upperNeighbor.second] = struct{}{}
							}
						}
					}

					l++
				}

				k0 = k1
			}
		}
	}
}

/////////////////////////////////////////////////////////////////////////////////////////////////////
// Layout Balancing

// createBalancedLayout is createBalancedLayout(layouts, nodeCount): the
// median of the four layouts.
//
// The layout with the smallest size is the base. The other layouts are
// aligned to it, and the position of a node is the median of its positions in
// the four layouts. The positions include the inner shift of the node.
func (p *BKNodePlacer) createBalancedLayout(layouts []*BKAlignedLayout, nodeCount int) *BKAlignedLayout {
	noOfLayouts := len(layouts)
	balanced := NewBKAlignedLayout(p.lGraph, nodeCount, VDirectionNone, HDirectionNone)
	width := make([]float64, noOfLayouts)
	minY := make([]float64, noOfLayouts)
	maxY := make([]float64, noOfLayouts)
	minWidthLayout := 0

	// Find the smallest layout
	for i := 0; i < noOfLayouts; i++ {
		minY[i] = math.MaxInt32
		maxY[i] = math.MinInt32
	}

	for i := 0; i < noOfLayouts; i++ {
		bal := layouts[i]
		width[i] = bal.layoutSize()
		if width[minWidthLayout] > width[i] {
			minWidthLayout = i
		}

		for _, l := range p.lGraph.Layers {
			for _, n := range l.Nodes {
				nodePosY := bal.yAt(n.ID) + bal.innerShift[n.ID]
				minY[i] = min(minY[i], nodePosY)
				maxY[i] = max(maxY[i], nodePosY+n.Size.Y)
			}
		}
	}

	// Find the shift between the smallest and the four layouts
	shift := make([]float64, noOfLayouts)
	for i := 0; i < noOfLayouts; i++ {
		if layouts[i].vdir == VDirectionDown {
			shift[i] = minY[minWidthLayout] - minY[i]
		} else {
			shift[i] = maxY[minWidthLayout] - maxY[i]
		}
	}

	// Calculated y-coordinates for a balanced placement
	calculatedYs := make([]float64, noOfLayouts)
	if noOfLayouts < 3 {
		// The code below reads the entries 1 and 2 of the array. The placer
		// makes a balanced layout only from four layouts.
		lgraph.Bail("BKNodePlacer: a balanced layout needs four layouts")
	}
	for _, layer := range p.lGraph.Layers {
		for _, node := range layer.Nodes {
			for i := 0; i < noOfLayouts; i++ {
				// it's important to include the innerShift here!
				calculatedYs[i] = layouts[i].yAt(node.ID) + layouts[i].innerShift[node.ID] + shift[i]
			}

			gwt.SortFloats(calculatedYs)
			balanced.setY(node.ID, float64((calculatedYs[1]+calculatedYs[2])/2.0))
			// since we include the inner shift in the calculation of a balanced y
			// coordinate we don't need it any more
			// note that after this step no further processing of the graph that
			// would include the inner shift is possible
			balanced.innerShift[node.ID] = 0
		}
	}

	return balanced
}

/////////////////////////////////////////////////////////////////////////////////////////////////////
// Utility Methods

// incidentToInnerSegment is incidentToInnerSegment(node, layer1, layer2): it
// tells whether the node is part of a long edge between the two layers.
// layer1 is the layer of the node, and layer2 is the layer before it.
func (p *BKNodePlacer) incidentToInnerSegment(node *lgraph.LNode, layer1, layer2 int) bool {
	if node.Type == lgraph.NodeTypeLongEdge {
		for _, port := range node.Ports {
			for _, edge := range port.IncomingEdges {
				sourceNodeType := edge.Source.Node().Type

				if sourceNodeType == lgraph.NodeTypeLongEdge &&
					p.layerIndexOf(edge.Source.Node()) == layer2 &&
					p.layerIndexOf(node) == layer1 {

					return true
				}
			}
		}
	}
	return false
}

// layerIndexOf is ni.layerIndex[node.getLayer().id]. A source node of a self
// loop or of an edge inside a layer is in a layer of the graph, and an edge
// from another layer was checked when the neighbors were collected.
func (p *BKNodePlacer) layerIndexOf(node *lgraph.LNode) int {
	layer := node.Layer()
	if layer == nil || layer.ID < 0 || layer.ID >= len(p.ni.layerIndex) {
		lgraph.Bail("BKNodePlacer: a node is outside the layers of the graph")
	}
	return p.ni.layerIndex[layer.ID]
}

// getEdge is getEdge(source, target): an edge between two nodes, or nil.
func getEdge(source, target *lgraph.LNode) *lgraph.LEdge {
	// getConnectedEdges() has for each port the incoming edges, then the
	// outgoing edges.
	for _, port := range source.Ports {
		for _, edge := range port.IncomingEdges {
			if edge.Target.Node() == target || edge.Source.Node() == target {
				return edge
			}
		}
		for _, edge := range port.OutgoingEdges {
			if edge.Target.Node() == target || edge.Source.Node() == target {
				return edge
			}
		}
	}

	return nil
}

// getBlocks is getBlocks(bal): the nodes of each block of a layout, by the
// root of the block. The map iterates in the order in which the walk over
// the layers finds the roots.
//
// The method getClasses of the Java class only serves the debug output and
// is not ported.
func getBlocks(bal *BKAlignedLayout) *gwt.Map[*lgraph.LNode, []*lgraph.LNode] {
	blocks := gwt.NewMap[*lgraph.LNode, []*lgraph.LNode]()

	for _, layer := range bal.layeredGraph.Layers {
		for _, node := range layer.Nodes {
			root := bal.root[node.ID]
			blockContents := blocks.Get(root)
			blocks.Put(root, append(blockContents, node))
		}
	}

	return blocks
}

// checkOrderConstraint is checkOrderConstraint(layeredGraph, bal, monitor):
// it tells whether the nodes of each layer are in the right order and do not
// overlap.
func (p *BKNodePlacer) checkOrderConstraint(layeredGraph *lgraph.LGraph, bal *BKAlignedLayout) bool {
	// Flag indicating whether the layout is feasible or not
	feasible := true

	// Iterate over the layers
	for _, layer := range layeredGraph.Layers {
		// Current Y position in the layer
		pos := math.Inf(-1)

		// Iterate through the layer's nodes
		for _, node := range layer.Nodes {
			// For the layout to be correct, both the node's top border and its bottom border must
			// be beyond the current position in the layer
			top := bal.yAt(node.ID) + bal.innerShift[node.ID] - node.Margin.Top
			bottom := bal.yAt(node.ID) + bal.innerShift[node.ID] + node.Size.Y + node.Margin.Bottom

			if top > pos && bottom > pos {
				// Update the position inside the layer
				pos = bal.yAt(node.ID) + bal.innerShift[node.ID] + node.Size.Y + node.Margin.Bottom
			} else {
				// We've found an overlap
				feasible = false
				break
			}
		}

		// Don't bother continuing if we've already determined that the layout is infeasible
		if !feasible {
			break
		}
	}

	return feasible
}
