// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p4nodes/LinearSegmentsNodePlacer.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p4nodes

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("LinearSegmentsNodePlacer", func() lgraph.Processor { return NewLinearSegmentsNodePlacer() })
}

// LinearSegment is LinearSegmentsNodePlacer.LinearSegment: a single regular
// node, or all dummy nodes of a long edge.
type LinearSegment struct {
	// nodes has the nodes of the linear segment.
	nodes []*lgraph.LNode
	// id is the index in the segments array.
	id int
	// indexInLastLayer is the index in the previous layer. The cycle
	// avoidance uses it.
	indexInLastLayer int
	// lastLayer is the last layer where a node of this segment was found.
	// The cycle avoidance uses it.
	lastLayer int
	// deflection is the accumulated force of the nodes.
	deflection float64
	// weight is the current weight of the nodes.
	weight int
	// refSegment is the reference segment, if this segment was unified with
	// another one.
	refSegment *LinearSegment
}

// newLinearSegment is new LinearSegment().
func newLinearSegment() *LinearSegment {
	return &LinearSegment{indexInLastLayer: -1, lastLayer: -1}
}

// region is region(): the reference segment of the region of this segment.
func (s *LinearSegment) region() *LinearSegment {
	seg := s
	for seg.refSegment != nil {
		seg = seg.refSegment
	}
	return seg
}

// split is split(node, newId): it splits the segment before the node. The
// new segment gets the nodes from the given node onward, and each of them
// gets the new id.
func (s *LinearSegment) split(node *lgraph.LNode, newID int) *LinearSegment {
	nodeIndex := -1
	for i, n := range s.nodes {
		if n == node {
			nodeIndex = i
			break
		}
	}
	if nodeIndex < 0 {
		// The Java code fails with an IndexOutOfBoundsException.
		lgraph.Bail("LinearSegmentsNodePlacer: a segment does not have the node to split at")
	}

	// Prepare the new segment
	newSegment := newLinearSegment()
	newSegment.id = newID

	// Move nodes to the new segment
	for _, movedNode := range s.nodes[nodeIndex:] {
		movedNode.ID = newID
		newSegment.nodes = append(newSegment.nodes, movedNode)
	}
	for i := nodeIndex; i < len(s.nodes); i++ {
		s.nodes[i] = nil
	}
	s.nodes = s.nodes[:nodeIndex]

	return newSegment
}

// equals is equals(Object): two segments with the same id are equal.
func (s *LinearSegment) equals(other *LinearSegment) bool {
	return other != nil && s.id == other.id
}

// The properties that the placer leaves on each node.
var (
	// inputPrio is INPUT_PRIO: the maximal priority of the incoming edges.
	inputPrio = prop.NewDefault("linearSegments.inputPrio", 0)
	// outputPrio is OUTPUT_PRIO: the maximal priority of the outgoing edges.
	outputPrio = prop.NewDefault("linearSegments.outputPrio", 0)
)

// LinearSegmentsNodePlacer is the phase LinearSegmentsNodePlacer: a node
// placement that keeps each long edge straight. A linear segment has a
// single regular node or all dummy nodes of a long edge. The placer orders
// the segments, places them from the top, and balances the placement with a
// pendulum method.
//
// The graph must be layered with ordered nodes and arranged ports. The placer
// sets the y coordinate of each node, and it changes the height of each
// layer.
//
// getLayoutProcessorConfiguration is in the package layered, with the
// configurations of the other phases.
type LinearSegmentsNodePlacer struct {
	// linearSegments is the array of sorted linear segments.
	linearSegments []*LinearSegment
	// spacings has the spacing values.
	spacings *lgraph.Spacings
}

// NewLinearSegmentsNodePlacer is new LinearSegmentsNodePlacer().
func NewLinearSegmentsNodePlacer() *LinearSegmentsNodePlacer { return &LinearSegmentsNodePlacer{} }

// Process is process(layeredGraph, monitor).
func (p *LinearSegmentsNodePlacer) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	p.spacings = prop.Get(layeredGraph, iprops.Spacings)

	// sort the linear segments of the layered graph
	p.sortLinearSegments(layeredGraph)

	// create an unbalanced placement from the sorted segments
	p.createUnbalancedPlacement(layeredGraph)

	// balance the placement
	p.balancePlacement(layeredGraph)

	// post-process the placement for small corrections
	p.postProcess(layeredGraph)

	// release the created resources
	p.linearSegments = nil
	p.spacings = nil
}

// /////////////////////////////////////////////////////////////////////////////
// Linear Segments Creation

// sortLinearSegments is sortLinearSegments(layeredGraph, monitor): it sorts
// the linear segments of the graph with a topological ordering of the
// segment ordering graph.
func (p *LinearSegmentsNodePlacer) sortLinearSegments(layeredGraph *lgraph.LGraph) []*LinearSegment {
	// set the identifier and input / output priority for all nodes
	segmentList := []*LinearSegment{}
	for _, layer := range layeredGraph.Layers {
		for _, node := range layer.Nodes {
			node.ID = -1
			inprio, outprio := math.MinInt32, math.MinInt32
			for _, port := range node.Ports {
				for _, edge := range port.IncomingEdges {
					prio := prop.Get(edge, lopts.PriorityStraightness)
					inprio = max(inprio, prio)
				}
				for _, edge := range port.OutgoingEdges {
					prio := prop.Get(edge, lopts.PriorityStraightness)
					outprio = max(outprio, prio)
				}
			}
			prop.Set(node, inputPrio, inprio)
			prop.Set(node, outputPrio, outprio)
		}
	}

	// create linear segments for the layered graph, ignoring odd port side dummies
	nextLinearSegmentID := 0
	for _, layer := range layeredGraph.Layers {
		for _, node := range layer.Nodes {
			// Test for the node ID; calls to fillSegment(...) may have caused the node ID
			// to be != -1.
			if node.ID < 0 {
				segment := newLinearSegment()
				segment.id = nextLinearSegmentID
				nextLinearSegmentID++
				p.fillSegment(node, segment)
				segmentList = append(segmentList, segment)
			}
		}
	}

	// create and initialize segment ordering graph
	outgoingList := make([][]*LinearSegment, len(segmentList), len(segmentList)+4)
	incomingCountList := make([]int, len(segmentList), len(segmentList)+4)

	// create edges for the segment ordering graph
	segmentList, outgoingList, incomingCountList = p.createDependencyGraphEdges(layeredGraph, segmentList, outgoingList, incomingCountList)

	// turn lists into arrays
	segments := segmentList
	outgoing := outgoingList
	incomingCount := incomingCountList

	// gather the sources of the segment ordering graph
	nextRank := 0
	noIncoming := []*LinearSegment{}
	for i := 0; i < len(segments); i++ {
		if incomingCount[i] == 0 {
			noIncoming = append(noIncoming, segments[i])
		}
	}

	// find a topological ordering of the segment ordering graph
	newRanks := make([]int, len(segments))
	for head := 0; head < len(noIncoming); head++ {
		segment := noIncoming[head]
		newRanks[segment.id] = nextRank
		nextRank++

		for len(outgoing[segment.id]) != 0 {
			target := outgoing[segment.id][0]
			outgoing[segment.id] = outgoing[segment.id][1:]
			incomingCount[target.id]--

			if incomingCount[target.id] == 0 {
				noIncoming = append(noIncoming, target)
			}
		}
	}

	// apply the new ordering to the array of linear segments
	p.linearSegments = make([]*LinearSegment, len(segments))
	for i := 0; i < len(segments); i++ {
		ls := segments[i]
		rank := newRanks[i]
		p.linearSegments[rank] = ls
		ls.id = rank
		for _, node := range ls.nodes {
			node.ID = rank
		}
	}
	for _, ls := range p.linearSegments {
		if ls == nil {
			// The segment ordering graph has a cycle, and two segments have
			// the same rank. The code that follows reads each entry of the
			// array, and the Java code fails there for the entry that is
			// null.
			lgraph.Bail("LinearSegmentsNodePlacer: the segment ordering graph has a cycle")
		}
	}

	return p.linearSegments
}

// createDependencyGraphEdges is createDependencyGraphEdges(monitor,
// layeredGraph, segmentList, outgoingList, incomingCountList): it fills the
// dependency graph. Where a dependency would make a cycle, it splits the
// linear segment into two. It returns the three lists, which can be longer.
//
// The walk goes through the layers and through the nodes of each layer. The
// segment of a node gets a dependency to the segment of the next node. The
// order of two segments can change from one layer to the next since north
// and south port dummies exist, so the walk remembers the index of each
// segment in the layer before. When a segment x comes after a segment y that
// was before x in the layer before, x is split at the current layer, and the
// second part depends on y.
func (p *LinearSegmentsNodePlacer) createDependencyGraphEdges(layeredGraph *lgraph.LGraph, segmentList []*LinearSegment, outgoingList [][]*LinearSegment, incomingCountList []int) ([]*LinearSegment, [][]*LinearSegment, []int) {
	nextLinearSegmentID := len(segmentList)
	layerIndex := 0
	for _, layer := range layeredGraph.Layers {
		nodes := layer.Nodes
		if len(nodes) == 0 {
			// Ignore empty layers
			continue
		}

		nodeIter := 0
		indexInLayer := 0

		// We carry the previous node with us for dependency management
		var previousNode *lgraph.LNode

		// Get the layer's first node
		currentNode := nodes[nodeIter]
		nodeIter++

		for currentNode != nil {
			// Get the current node's segment
			currentSegment := segmentList[currentNode.ID]

			/*
			 * Check if we have a cycle. That's the case if the following holds: - The current
			 * segment appeared in the previous layer as well. - In the previous layer, we find
			 * a segment after the current segment that appears before the current segment in
			 * the current layer.
			 */
			if currentSegment.indexInLastLayer >= 0 {
				var cycleSegment *LinearSegment
				for cycleNodesIter := indexInLayer + 1; cycleNodesIter < len(layer.Nodes); cycleNodesIter++ {
					cycleNode := layer.Nodes[cycleNodesIter]
					cycleSegment = segmentList[cycleNode.ID]

					if cycleSegment.lastLayer == currentSegment.lastLayer &&
						cycleSegment.indexInLastLayer < currentSegment.indexInLastLayer {

						break
					} else {
						cycleSegment = nil
					}
				}

				// If we have found a cycle segment, we need to split the current linear segment
				if cycleSegment != nil {
					// Update the current segment before it's split
					if previousNode != nil {
						incomingCountList[currentNode.ID] = incomingCountList[currentNode.ID] - 1
						outgoingList[previousNode.ID] = removeSegment(outgoingList[previousNode.ID], currentSegment)
					}

					currentSegment = currentSegment.split(currentNode, nextLinearSegmentID)
					nextLinearSegmentID++
					segmentList = append(segmentList, currentSegment)
					outgoingList = append(outgoingList, nil)

					if previousNode != nil {
						outgoingList[previousNode.ID] = append(outgoingList[previousNode.ID], currentSegment)
						incomingCountList = append(incomingCountList, 1)
					} else {
						incomingCountList = append(incomingCountList, 0)
					}
				}
			}

			// Now add a dependency to the next node, if any
			var nextNode *lgraph.LNode
			if nodeIter < len(nodes) {
				nextNode = nodes[nodeIter]
				nodeIter++
				nextSegment := segmentList[nextNode.ID]

				outgoingList[currentNode.ID] = append(outgoingList[currentNode.ID], nextSegment)
				incomingCountList[nextNode.ID] = incomingCountList[nextNode.ID] + 1
			}

			// Update segment's layer information
			currentSegment.lastLayer = layerIndex
			currentSegment.indexInLastLayer = indexInLayer
			indexInLayer++

			// Cycle nodes
			previousNode = currentNode
			currentNode = nextNode
		}

		layerIndex++
	}
	return segmentList, outgoingList, incomingCountList
}

// removeSegment is List.remove(Object): it removes the first segment of the
// list that equals the given one.
func removeSegment(list []*LinearSegment, segment *LinearSegment) []*LinearSegment {
	for i, s := range list {
		if segment.equals(s) {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

// fillSegment is fillSegment(node, segment): it puts a node into the linear
// segment, and then the next part of a long edge. It returns true if the
// node was not part of another segment and so was added to this one.
func (p *LinearSegmentsNodePlacer) fillSegment(node *lgraph.LNode, segment *LinearSegment) bool {
	nodeType := node.Type

	if node.ID >= 0 {
		// The node is already part of another linear segment
		return false
	}
	// Add the node to the given linear segment
	node.ID = segment.id
	segment.nodes = append(segment.nodes, node)

	if nodeType == lgraph.NodeTypeLongEdge || nodeType == lgraph.NodeTypeNorthSouthPort {

		// This is a LONG_EDGE, NORTH_SOUTH_PORT or BIG_NODE dummy; check if any of its
		// successors are of one of these types too. If so, we can form a linear segment
		// with one of them. (not with more than one, though)
		// Note 1: LONG_EDGES and NORTH_SOUTH_PORTs can share a common linear segment
		// Note 2: we must take care not to make a segment out of nodes that are in the same layer
		// Note 3: for BIG_NODEs also the first BIG_NODE_INITIAL which is no actual dummy node has
		// to be considered here
		for _, sourcePort := range node.Ports {
			for _, edge := range sourcePort.OutgoingEdges {
				targetPort := edge.Target
				targetNode := targetPort.Node()
				targetNodeType := targetNode.Type

				if node.Layer() != targetNode.Layer() {
					// current no bignode and next node is LONG_EDGE and NORTH_SOUTH_PORT
					if targetNodeType == lgraph.NodeTypeLongEdge || targetNodeType == lgraph.NodeTypeNorthSouthPort {
						if p.fillSegment(targetNode, segment) {
							// We just added another node to this node's linear segment.
							// That's quite enough.
							return true
						}
					}
				}
			}
		}
	}

	return true
}

// /////////////////////////////////////////////////////////////////////////////
// Unbalanced Placement

// layerIndexOf is node.getLayer().getIndex(). The result is an index of an
// array that has one entry for each layer of the graph.
func layerIndexOf(node *lgraph.LNode, layerCount int) int {
	layer := node.Layer()
	if layer == nil {
		lgraph.Bail("LinearSegmentsNodePlacer: a node has no layer")
	}
	index := layer.Index()
	if index < 0 || index >= layerCount {
		lgraph.Bail("LinearSegmentsNodePlacer: a node is outside the layers of the graph")
	}
	return index
}

// createUnbalancedPlacement is createUnbalancedPlacement(layeredGraph): it
// places the sorted linear segments from the top, without a balance.
func (p *LinearSegmentsNodePlacer) createUnbalancedPlacement(layeredGraph *lgraph.LGraph) {
	layerCount := len(layeredGraph.Layers)

	// How many nodes are currently placed in each layer
	nodeCount := make([]int, layerCount)

	// The node most recently placed in a given layer
	recentNode := make([]*lgraph.LNode, layerCount)

	// Iterate through the linear segments (in proper order!) and place them
	for _, segment := range p.linearSegments {
		// Determine the uppermost placement for the linear segment
		uppermostPlace := 0.0
		for _, node := range segment.nodes {
			layerIndex := layerIndexOf(node, layerCount)
			nodeCount[layerIndex]++

			// Calculate how much space to leave between the linear segment and the last
			// node of the given layer
			spacing := prop.Get(layeredGraph, lopts.SpacingEdgeEdge)
			if nodeCount[layerIndex] > 0 {
				if recentNode[layerIndex] != nil {
					spacing = verticalSpacing(p.spacings, recentNode[layerIndex], node)
				}
			}

			uppermostPlace = max(uppermostPlace, node.Layer().Size.Y+spacing)
		}

		// Apply the uppermost placement to all elements
		for _, node := range segment.nodes {
			// Set the node position
			node.Pos.Y = uppermostPlace + node.Margin.Top

			// Adjust layer size
			layer := node.Layer()
			layer.Size.Y = uppermostPlace + node.Margin.Top + node.Size.Y + node.Margin.Bottom

			recentNode[layerIndexOf(node, layerCount)] = node
		}
	}
}

// /////////////////////////////////////////////////////////////////////////////
// Balanced Placement

// mode is the enum Mode: the balancing modes.
type mode int

const (
	modeForwPendulum mode = iota
	modeBackwPendulum
	modeRubber
)

const (
	// thresholdFactor is THRESHOLD_FACTOR: the factor for the threshold after
	// which the balancing stops.
	thresholdFactor = 20.0
	// pendulumItersStart is PENDULUM_ITERS: the minimal number of iterations
	// in pendulum mode.
	pendulumItersStart = 4
	// finalItersStart is FINAL_ITERS: the number of iterations after the
	// abort condition was met.
	finalItersStart = 3
)

// balancePlacement is balancePlacement(layeredGraph): it balances the first
// placement with forces that move regions.
//
// First come the pendulum iterations, which look at one direction of edges,
// then the rubber iterations, which look at the incoming and the outgoing
// edges. Each iteration computes the deflection of each linear segment: the
// change of position that balances it against its adjacent segments. Then it
// merges the regions that touch each other, with the mean of their
// deflections, and it applies the deflections. The iterations stop when
// there is no more improvement.
func (p *LinearSegmentsNodePlacer) balancePlacement(layeredGraph *lgraph.LGraph) {
	deflectionDampening := prop.Get(layeredGraph, lopts.NodePlacementLinearSegmentsDeflectionDampening)

	// Determine a suitable number of pendulum iterations
	thoroughness := prop.Get(layeredGraph, lopts.Thoroughness)
	pendulumIters := pendulumItersStart
	finalIters := finalItersStart
	threshold := thresholdFactor / float64(thoroughness)

	// Iterate the balancing
	ready := false
	mode := modeForwPendulum
	lastTotalDeflection := float64(math.MaxInt32)
	for {

		// Calculate force for every linear segment
		incoming := mode != modeBackwPendulum
		outgoing := mode != modeForwPendulum
		totalDeflection := 0.0
		for _, segment := range p.linearSegments {
			segment.refSegment = nil
			p.calcDeflection(segment, incoming, outgoing, deflectionDampening)
			totalDeflection += math.Abs(segment.deflection)
		}

		// Merge linear segments to form regions
		for {
			merged := p.mergeRegions(layeredGraph)
			if !merged {
				break
			}
		}

		// Move the nodes according to the deflection value of their region
		for _, segment := range p.linearSegments {
			deflection := segment.region().deflection
			if deflection != 0 {
				for _, node := range segment.nodes {
					node.Pos.Y += deflection
				}
			}
		}

		// Update the balancing mode
		if mode == modeForwPendulum || mode == modeBackwPendulum {
			pendulumIters--
			if pendulumIters <= 0 && (totalDeflection < lastTotalDeflection || -pendulumIters > thoroughness) {
				mode = modeRubber
				lastTotalDeflection = float64(math.MaxInt32)
			} else if mode == modeForwPendulum {
				mode = modeBackwPendulum
				lastTotalDeflection = totalDeflection
			} else {
				mode = modeForwPendulum
				lastTotalDeflection = totalDeflection
			}
		} else {
			ready = totalDeflection >= lastTotalDeflection || lastTotalDeflection-totalDeflection < threshold
			lastTotalDeflection = totalDeflection
			if ready {
				finalIters--
			}
		}
		if ready && finalIters <= 0 {
			break
		}
	}
}

// segmentOf is linearSegments[node.id]. Each node of the layers has the id
// of its segment. A node outside the layers of the graph has no segment.
func (p *LinearSegmentsNodePlacer) segmentOf(node *lgraph.LNode) *LinearSegment {
	if node.ID < 0 || node.ID >= len(p.linearSegments) {
		lgraph.Bail("LinearSegmentsNodePlacer: a node has no linear segment")
	}
	return p.linearSegments[node.ID]
}

// calcDeflection is calcDeflection(segment, incoming, outgoing,
// deflectionDampening): it computes the force on a linear segment and keeps
// it in the field deflection. incoming and outgoing tell which edges count.
func (p *LinearSegmentsNodePlacer) calcDeflection(segment *LinearSegment, incoming, outgoing bool, deflectionDampening float64) {
	segmentDeflection := 0.0
	nodeWeightSum := 0
	for _, node := range segment.nodes {
		nodeDeflection := 0.0
		edgeWeightSum := 0
		inputPrioValue := math.MinInt32
		if incoming {
			inputPrioValue = prop.Get(node, inputPrio)
		}
		outputPrioValue := math.MinInt32
		if outgoing {
			outputPrioValue = prop.Get(node, outputPrio)
		}
		minPrio := max(inputPrioValue, outputPrioValue)

		// Calculate force for every port/edge
		for _, port := range node.Ports {
			portpos := node.Pos.Y + port.Pos.Y + port.Anchor.Y
			if outgoing {
				for _, edge := range port.OutgoingEdges {
					otherPort := edge.Target
					otherNode := otherPort.Node()
					if segment != p.segmentOf(otherNode) {
						otherPrio := max(prop.Get(otherNode, inputPrio), prop.Get(otherNode, outputPrio))
						prio := prop.Get(edge, lopts.PriorityStraightness)
						if prio >= minPrio && prio >= otherPrio {
							nodeDeflection += otherNode.Pos.Y + otherPort.Pos.Y + otherPort.Anchor.Y - portpos
							edgeWeightSum++
						}
					}
				}
			}

			if incoming {
				for _, edge := range port.IncomingEdges {
					otherPort := edge.Source
					otherNode := otherPort.Node()
					if segment != p.segmentOf(otherNode) {
						otherPrio := max(prop.Get(otherNode, inputPrio), prop.Get(otherNode, outputPrio))
						prio := prop.Get(edge, lopts.PriorityStraightness)
						if prio >= minPrio && prio >= otherPrio {
							nodeDeflection += otherNode.Pos.Y + otherPort.Pos.Y + otherPort.Anchor.Y - portpos
							edgeWeightSum++
						}
					}
				}
			}
		}

		// Avoid division by zero
		if edgeWeightSum > 0 {
			segmentDeflection += float64(nodeDeflection / float64(edgeWeightSum))
			nodeWeightSum++
		}
	}
	if nodeWeightSum > 0 {
		segment.deflection = float64(deflectionDampening*segmentDeflection) / float64(nodeWeightSum)
		segment.weight = nodeWeightSum
	} else {
		segment.deflection = 0
		segment.weight = 0
	}
}

// overlapDetect is OVERLAP_DETECT: the factor for the threshold within which
// an overlap of nodes is detected.
const overlapDetect = 0.0001

// mergeRegions is mergeRegions(layeredGraph): it merges the regions that
// overlap after the deflection is applied. It returns true if it merged two
// regions.
func (p *LinearSegmentsNodePlacer) mergeRegions(layeredGraph *lgraph.LGraph) bool {
	changed := false
	nodeSpacing := prop.Get(layeredGraph, lopts.SpacingNodeNode)
	threshold := float64(overlapDetect * nodeSpacing)
	for _, layer := range layeredGraph.Layers {
		if len(layer.Nodes) == 0 {
			// The Java code fails with a NoSuchElementException.
			lgraph.Bail("LinearSegmentsNodePlacer: a layer has no nodes")
		}

		// Get the first node
		node1 := layer.Nodes[0]
		region1 := p.segmentOf(node1).region()

		// While there are still nodes following the current node
		for _, node2 := range layer.Nodes[1:] {
			// Test whether nodes have different regions
			region2 := p.segmentOf(node2).region()

			if region1 != region2 {
				// Calculate how much space is allowed between the nodes
				spacing := verticalSpacing(p.spacings, node1, node2)

				node1Extent := node1.Pos.Y + node1.Size.Y + node1.Margin.Bottom + region1.deflection + spacing
				node2Extent := node2.Pos.Y - node2.Margin.Top + region2.deflection

				// Test if the nodes are overlapping
				if node1Extent > node2Extent+threshold {
					// Merge the first region under the second top level segment
					weightSum := region1.weight + region2.weight
					region2.deflection = (float64(float64(region2.weight)*region2.deflection) +
						float64(float64(region1.weight)*region1.deflection)) / float64(weightSum)
					region2.weight = weightSum
					region1.refSegment = region2
					changed = true
				}
			}

			node1 = node2
			region1 = region2
		}
	}
	return changed
}

// /////////////////////////////////////////////////////////////////////////////
// Post Processing for Correction

// postProcess is postProcess(layeredGraph): it moves a linear segment where
// that makes an edge straight and the neighbors leave the room.
func (p *LinearSegmentsNodePlacer) postProcess(layeredGraph *lgraph.LGraph) {
	// process each linear segment independently
	for _, segment := range p.linearSegments {
		minRoomAbove, minRoomBelow := float64(math.MaxInt32), float64(math.MaxInt32)

		for _, node := range segment.nodes {
			var roomAbove, roomBelow float64
			index := node.Index()

			// determine the amount by which the linear segment can be moved up without overlap
			if index > 0 {
				neighbor := node.Layer().Nodes[index-1]
				spacing := verticalSpacing(p.spacings, node, neighbor)
				roomAbove = node.Pos.Y - node.Margin.Top -
					(neighbor.Pos.Y + neighbor.Size.Y + neighbor.Margin.Bottom + spacing)
			} else {
				roomAbove = node.Pos.Y - node.Margin.Top
			}
			minRoomAbove = min(roomAbove, minRoomAbove)

			// determine the amount by which the linear segment can be moved down without
			// overlap
			if index < len(node.Layer().Nodes)-1 {
				// An index below 0 does not come here: each node of a segment
				// is in a layer.
				if index < 0 {
					lgraph.Bail("LinearSegmentsNodePlacer: a node is not in its layer")
				}
				neighbor := node.Layer().Nodes[index+1]
				spacing := verticalSpacing(p.spacings, node, neighbor)
				roomBelow = neighbor.Pos.Y - neighbor.Margin.Top -
					(node.Pos.Y + node.Size.Y + node.Margin.Bottom + spacing)
			} else {
				roomBelow = float64(2 * node.Pos.Y)
			}
			minRoomBelow = min(roomBelow, minRoomBelow)
		}

		minDisplacement := float64(math.MaxInt32)
		foundPlace := false

		if len(segment.nodes) == 0 {
			// The Java code fails with an IndexOutOfBoundsException.
			lgraph.Bail("LinearSegmentsNodePlacer: a linear segment has no nodes")
		}

		// determine the minimal displacement that would make one incoming edge straight
		firstNode := segment.nodes[0]
		for _, target := range firstNode.Ports {
			pos := firstNode.Pos.Y + target.Pos.Y + target.Anchor.Y
			for _, edge := range target.IncomingEdges {
				source := edge.Source
				d := source.Node().Pos.Y + source.Pos.Y + source.Anchor.Y - pos
				room := minRoomBelow
				if d < 0 {
					room = minRoomAbove
				}
				if math.Abs(d) < math.Abs(minDisplacement) && math.Abs(d) < room {
					minDisplacement = d
					foundPlace = true
				}
			}
		}

		// determine the minimal displacement that would make one outgoing edge straight
		lastNode := segment.nodes[len(segment.nodes)-1]
		for _, source := range lastNode.Ports {
			pos := lastNode.Pos.Y + source.Pos.Y + source.Anchor.Y
			for _, edge := range source.OutgoingEdges {
				target := edge.Target
				d := target.Node().Pos.Y + target.Pos.Y + target.Anchor.Y - pos
				room := minRoomBelow
				if d < 0 {
					room = minRoomAbove
				}
				if math.Abs(d) < math.Abs(minDisplacement) && math.Abs(d) < room {
					minDisplacement = d
					foundPlace = true
				}
			}
		}

		// if such a displacement could be found, apply it to the whole linear segment
		if foundPlace && minDisplacement != 0 {
			for _, node := range segment.nodes {
				node.Pos.Y += minDisplacement
			}
		}
	}
}
