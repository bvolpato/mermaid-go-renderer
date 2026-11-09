// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p4nodes/NetworkSimplexPlacer.java
// and from the methods of options/NodeFlexibility.java in the same plug-in
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p4nodes

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/networksimplex"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

func init() {
	lgraph.Register("NetworkSimplexPlacer", func() lgraph.Processor { return NewNetworkSimplexPlacer() })
}

// The weights of the edges of the auxiliary network simplex graph.
const (
	// edgeWeightBase is EDGE_WEIGHT_BASE: the basis for the weight of edges.
	edgeWeightBase = 4.0
	// smallEdgeWeight is SMALL_EDGE_WEIGHT: a smaller weight than the
	// default, since horizontal edges are more important.
	smallEdgeWeight = 0.1
	// longEdgeVsPathFactor is LONG_EDGE_VS_PATH_FACTOR. With a factor below
	// one, straight long edges are more important than straight paths.
	longEdgeVsPathFactor = 2.0
	// nodeSizeWeightStatic is NODE_SIZE_WEIGHT_STATIC: the weight of a node
	// that must not change in size.
	nodeSizeWeightStatic = 10000.0
	// nodeSizeWeightFlexible is NODE_SIZE_WEIGHT_FLEXIBLE: the weight of a
	// node that may change in size.
	nodeSizeWeightFlexible = 1.0
	// epsilon is EPSILON, for the test of two doubles for equality.
	epsilon = 0.00001
)

// The states of a node during the edge straightening.
const (
	// nodeStateVisited is VISITED: the node has been visited.
	nodeStateVisited = -1
	// nodeStateOther is OTHER: the node is not a junction.
	nodeStateOther = 0
	// nodeStateJunction is JUNCTION: the node has more than one incoming
	// edge, or more than one outgoing edge, or exactly one incident edge.
	nodeStateJunction = 2
)

// NetworkSimplexPlacer is the phase NetworkSimplexPlacer: the node placement
// of Gansner et al. It converts the problem into an auxiliary graph and
// layers that graph with the network simplex algorithm. The layer of an
// auxiliary node is a y coordinate.
//
// Layers are integers, so the placer rounds: it adds up the position, the
// height, the margin and the spacing of a node and takes the ceiling, and it
// moves each port to the nearest integer position.
//
// With the option favorStraightEdges the placer changes the weights of paths
// and moves the middle node of a path of two edges after the network simplex
// run, to remove bends. A node with the option nodeFlexibility has two
// auxiliary nodes, for its top and its bottom border, and one for each port
// on the west and the east side, so that its ports can move and its height
// can grow.
//
// The graph must be layered with ordered nodes and arranged ports. The placer
// sets the y coordinate of each node. For a flexible node it also changes
// the port positions, the height and the label positions.
//
// getLayoutProcessorConfiguration is in the package layered, with the
// configurations of the other phases.
type NetworkSimplexPlacer struct {
	// lGraph is the graph to handle.
	lGraph *lgraph.LGraph
	// spacings has the spacings of the graph.
	spacings *lgraph.Spacings
	// nGraph is the network simplex graph.
	nGraph *networksimplex.NGraph

	// nodeReps has the representation of each node, at the id of the node.
	nodeReps []*nodeRep
	// edgeReps has the representation of each edge, at the id of the edge.
	edgeReps []*edgeRep
	// portMap has the network simplex node of each port.
	portMap map[*lgraph.LPort]*networksimplex.NNode

	// nodeCount and edgeCount are the numbers of nodes and edges of lGraph.
	nodeCount int
	edgeCount int

	// nodeState, twoPaths and crossing serve the edge straightening.
	// nodeState has the state of each node, at the id of the node.
	nodeState []int
	twoPaths  []path
	crossing  []bool

	// flexibleWhereSpacePermitsEdges has the node size edges of the nodes
	// with the flexibility NODE_SIZE_WHERE_SPACE_PERMITS. The Java field is
	// a set. The code only tests it for emptiness and gives each of its
	// edges the same weight.
	flexibleWhereSpacePermitsEdges []*networksimplex.NEdge
}

// NewNetworkSimplexPlacer is new NetworkSimplexPlacer().
func NewNetworkSimplexPlacer() *NetworkSimplexPlacer {
	return &NetworkSimplexPlacer{portMap: map[*lgraph.LPort]*networksimplex.NNode{}}
}

// favorStraightEdges reads the option favorStraightEdges of the graph. The
// option has no default value, and the Java code fails with a
// NullPointerException when it unboxes null.
func favorStraightEdges(graph *lgraph.LGraph) bool {
	if !prop.Has(graph, lopts.NodePlacementFavorStraightEdges) {
		lgraph.Bail("NetworkSimplexPlacer: the option favorStraightEdges is null")
	}
	return prop.Get(graph, lopts.NodePlacementFavorStraightEdges)
}

// Process is process(layeredGraph, progressMonitor).
func (p *NetworkSimplexPlacer) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	p.lGraph = layeredGraph
	p.spacings = prop.Get(layeredGraph, iprops.Spacings)

	// -------------------------------
	// #1 build the auxiliary graph
	// -------------------------------
	p.prepare()

	p.buildInitialAuxiliaryGraph()

	p.insertNorthSouthAuxiliaryEdges()
	p.insertInLayerEdgeAuxiliaryEdges()

	if favorStraightEdges(p.lGraph) {
		p.preferStraightEdges()
	}

	// make sure the ngraph is connected. Cases where this doesn't have to be the case include
	//  hierarchical nodes with unconnected ports that are (in the case of hierarchical layout)
	//  converted into unconnected dummy nodes
	p.nGraph.MakeConnected()

	// --------------------------------
	// #2 execute the network simplex
	// --------------------------------
	// compared to {@link NetworkSimplexLayerer} a significantly larger iteration limit
	// is selected here because the node placement uses an auxiliary graph with
	// larger node and edge count
	iterLimit := prop.Get(layeredGraph, lopts.Thoroughness) * len(p.nGraph.Nodes)

	networksimplex.ForGraph(p.nGraph).
		WithIterationLimit(iterLimit).
		WithBalancing(false).
		Execute()

	// every individual node can be 'flexible where space permits'.
	// thus we cannot check for the property here but must rely on the fact that the
	// affected nodes have been collected before. In fact, the edges that represent
	// the size of the node were collected
	if len(p.flexibleWhereSpacePermitsEdges) != 0 {
		p.insertFlexibleWhereSpaceAuxiliaryEdges()
		// now the nodes may resize -> alter the weights
		for _, edge := range p.flexibleWhereSpacePermitsEdges {
			edge.Weight = nodeSizeWeightFlexible
		}

		// run network simplex a second time
		networksimplex.ForGraph(p.nGraph).
			WithIterationLimit(iterLimit).
			WithBalancing(false).
			Execute()
	}

	// post process 'two paths' that have been identified during the previous call of #preferStraightEdges()
	if favorStraightEdges(layeredGraph) {
		p.postProcessTwoPaths()
	}

	// --------------------------------
	// #3 apply positions
	// --------------------------------
	p.applyPositions()

	p.cleanup()
}

// ------------------------------------------------------------------------------------------------
//                                         Preparation
// ------------------------------------------------------------------------------------------------

// roundToDouble is the Java expression (double) Math.round(y) as GWT compiles
// it: Math.round of JavaScript, then the conversion to a long and back.
func roundToDouble(y float64) float64 {
	r := js.Round(y)
	if -17592186044416 < r && r < 17592186044416 {
		// A long in this range stays a number in GWT. The conversion back
		// turns -0 into 0.
		if r == 0 {
			return 0
		}
		return r
	}
	// A long outside this range is an object with three parts. NaN becomes
	// 0, and the value is limited to the range of long.
	if r != r {
		return 0
	}
	if r >= 9223372036854775807 {
		return 9223372036854775807
	}
	if r < -9223372036854775808 {
		return -9223372036854775808
	}
	return r
}

// prepare is prepare(): it makes port anchors and port positions integers,
// and it gives the nodes and the edges their ids.
func (p *NetworkSimplexPlacer) prepare() {
	p.nGraph = networksimplex.NewNGraph()

	// "integerify" port anchor and port positions
	//   note that margin.top and margin.bottom are not required to be integral
	//   since they do not influence the offset calculation for the edges
	// ... while we're at it, we assign ids to the nodes and edges
	nodeIdx := 0
	edgeIdx := 0
	var nodes []*lgraph.LNode
	for _, l := range p.lGraph.Layers {
		for _, lNode := range l.Nodes {
			lNode.ID = nodeIdx
			nodeIdx++
			nodes = append(nodes, lNode)
			for _, port := range lNode.Ports {
				for _, e := range port.OutgoingEdges {
					e.ID = edgeIdx
					edgeIdx++
				}
			}

			// if a node is flexible, an edge attaches to the port itself within
			//  the auxiliary graph, thus the anchor must be integer
			// otherwise the port position can be altered such that it accounts for the anchor's position as well
			anchorMustBeInteger := isFlexibleNode(lNode)
			for _, port := range lNode.Ports {
				if anchorMustBeInteger {
					// anchor
					y := port.Anchor.Y
					if y != math.Floor(y) {
						offset := y - roundToDouble(y)
						port.Anchor.Y -= offset
					}
				}

				// port + anchor
				y := port.Pos.Y + port.Anchor.Y
				if y != math.Floor(y) {
					offset := y - roundToDouble(y)
					port.Pos.Y -= offset
				}
			}
		}
	}

	// The Java code reads its arrays at the id of the node at the other end
	// of each edge. Only the nodes of the layers have an id that is an index
	// of these arrays, and only their outgoing edges have an edge id.
	inLayers := func(node *lgraph.LNode) bool {
		return node.ID >= 0 && node.ID < len(nodes) && nodes[node.ID] == node
	}
	for _, lNode := range nodes {
		for _, port := range lNode.Ports {
			for _, e := range port.IncomingEdges {
				if !inLayers(e.Source.Node()) {
					lgraph.Bail("NetworkSimplexPlacer: an edge comes from a node outside the layers of the graph")
				}
			}
			for _, e := range port.OutgoingEdges {
				if !inLayers(e.Target.Node()) {
					lgraph.Bail("NetworkSimplexPlacer: an edge leads to a node outside the layers of the graph")
				}
			}
		}
	}

	p.nodeCount = nodeIdx
	p.edgeCount = edgeIdx
	p.nodeReps = make([]*nodeRep, nodeIdx)
	p.edgeReps = make([]*edgeRep, edgeIdx)
	p.flexibleWhereSpacePermitsEdges = p.flexibleWhereSpacePermitsEdges[:0]
}

// cleanup is cleanup().
func (p *NetworkSimplexPlacer) cleanup() {
	p.lGraph = nil
	p.nGraph = nil

	p.nodeReps = nil
	p.edgeReps = nil
	clear(p.portMap)

	p.nodeState = nil
	p.crossing = nil
	p.twoPaths = nil
	p.flexibleWhereSpacePermitsEdges = nil
}

// rep is nodeReps[node.id]. The Java code fails for a node without a
// representation: a node that is not in a layer of the graph.
func (p *NetworkSimplexPlacer) rep(node *lgraph.LNode) *nodeRep {
	if node.ID < 0 || node.ID >= len(p.nodeReps) {
		lgraph.Bail("NetworkSimplexPlacer: a node is outside the layers of the graph")
	}
	rep := p.nodeReps[node.ID]
	if rep == nil || rep.origin != node {
		lgraph.Bail("NetworkSimplexPlacer: a node is outside the layers of the graph")
	}
	return rep
}

// repOfPortDummy is nodeReps[other.id] for the node of the property portDummy
// of a port. For a north or south port the node is a dummy node of a layer
// of this graph.
//
// For an external port of a compound node, CompoundGraphPreprocessor sets the
// property to a dummy node of the nested graph. Its id counts in the nested
// graph, and the Java code reads the array of this graph at that id all the
// same. No golden reaches that case, so the port does not copy it: it stops
// the layout, and elkjs gives the result.
func (p *NetworkSimplexPlacer) repOfPortDummy(other *lgraph.LNode) *nodeRep {
	return p.rep(other)
}

// edgeRepOf is edgeReps[edge.id]. The Java code fails for an edge without a
// representation.
func (p *NetworkSimplexPlacer) edgeRepOf(edge *lgraph.LEdge) *edgeRep {
	if edge.ID < 0 || edge.ID >= len(p.edgeReps) || p.edgeReps[edge.ID] == nil {
		lgraph.Bail("NetworkSimplexPlacer: an edge has no representation in the auxiliary graph")
	}
	return p.edgeReps[edge.ID]
}

// ------------------------------------------------------------------------------------------------
//                                      Auxiliary Graph
// ------------------------------------------------------------------------------------------------

// buildInitialAuxiliaryGraph is buildInitialAuxiliaryGraph(): it builds the
// auxiliary graph that the network simplex algorithm runs on.
func (p *NetworkSimplexPlacer) buildInitialAuxiliaryGraph() {
	for _, l := range p.lGraph.Layers {
		p.transformLayer(l)
	}
	p.transformEdges()
}

// transformLayer is transformLayer(layer): it makes the representation of
// each node of the layer. It adds an edge between two nodes that follow each
// other, to keep the order of the nodes and the spacing.
func (p *NetworkSimplexPlacer) transformLayer(layer *lgraph.Layer) {
	var lastRep *nodeRep
	for _, lNode := range layer.Nodes {

		var rep *nodeRep
		if isFlexibleNode(lNode) {
			rep = p.transformFixedOrderNode(lNode)
		} else {
			rep = p.transformFixedPosNode(lNode)
		}
		p.nodeReps[lNode.ID] = rep

		// if there is a previous node in the layer, we have to create a separation edge
		if lastRep != nil {
			spacing := lastRep.origin.Margin.Bottom +
				verticalSpacing(p.spacings, lastRep.origin, lNode) +
				lNode.Margin.Top

			if !lastRep.isFlexible {
				// for non-flexible nodes their height must be included
				// in the minimal length of the separation edge
				spacing += lastRep.origin.Size.Y
			}

			networksimplex.NEdgeOf().
				Delta(gwt.RoundInt(math.Ceil(spacing))).
				Weight(0).
				Source(lastRep.tail).
				Target(rep.head).
				Create()
		}

		// remember current elements for next iteration
		lastRep = rep
	}
}

// isEastOrWest is PortSide.SIDES_EAST_WEST.contains(side).
func isEastOrWest(side core.PortSide) bool {
	return side == core.PortSideEast || side == core.PortSideWest
}

// transformFixedPosNode is transformFixedPosNode(lNode): the representation
// of a node that is not flexible. It has one network simplex node.
func (p *NetworkSimplexPlacer) transformFixedPosNode(lNode *lgraph.LNode) *nodeRep {
	singleNode := networksimplex.NNodeOf().
		Origin(lNode).
		Type("non-flexible").
		Create(p.nGraph)

	// register the ports with the node
	for _, port := range lNode.Ports {
		if isEastOrWest(port.Side) {
			p.portMap[port] = singleNode
		}
	}

	return &nodeRep{origin: lNode, isFlexible: false, head: singleNode, tail: singleNode}
}

// transformFixedOrderNode is transformFixedOrderNode(lNode): the
// representation of a flexible node. It has a network simplex node for the
// top border, one for the bottom border, and one for each port on the west
// and the east side.
func (p *NetworkSimplexPlacer) transformFixedOrderNode(lNode *lgraph.LNode) *nodeRep {
	// -----------------------------------
	//          corner creation
	// -----------------------------------
	topLeft := networksimplex.NNodeOf().
		Origin(lNode).
		Type("flexible-head").
		Create(p.nGraph)
	bottomLeft := networksimplex.NNodeOf().
		Origin(lNode).
		Type("flexible-tail").
		Create(p.nGraph)
	corners := &nodeRep{origin: lNode, isFlexible: true, head: topLeft, tail: bottomLeft}

	// -----------------------------------
	//       weight & minimum length
	// -----------------------------------
	// the LabelAndNodeSizeProcessor ensures that any node size constraint is satisfied,
	//  i.e. the node's size is at least the specified minimum size and, if desired,
	//  the node is resized according to owned ports and labels
	// as such we can safely use the current height as minimum height
	minHeight := lNode.Size.Y

	nf := getNodeFlexibility(lNode)
	sizeWeight := nodeSizeWeightStatic
	if isFlexibleSize(nf) {
		// we are allowed to enlarge to node
		//  nevertheless, a little weight is good, otherwise the node can become arbitrarily tall
		//  especially since the edges that connect the node's do not carry nay weight
		sizeWeight = nodeSizeWeightFlexible
	}

	nodeSizeEdge := networksimplex.NEdgeOf().
		Weight(sizeWeight).
		Delta(gwt.RoundInt(math.Ceil(minHeight))).
		Source(topLeft).
		Target(bottomLeft).
		Create()

	if nf == lenum.NodeFlexibilityNodeSizeWhereSpacePermits {
		p.flexibleWhereSpacePermitsEdges = append(p.flexibleWhereSpacePermitsEdges, nodeSizeEdge)
	}

	// -----------------------------------
	//          port transformation
	// -----------------------------------
	// convert the ports to NNodes, note that the list of westward ports
	// must be reversed since their original order is from bottom to top
	p.transformPorts(lNode.PortSideView(core.PortSideWest), true, corners)
	p.transformPorts(lNode.PortSideView(core.PortSideEast), false, corners)

	// return the corners for further processing
	return corners
}

// transformPorts is transformPorts(ports, corners): it makes a network
// simplex node for each port and connects the nodes from the top border to
// the bottom border. reverse tells to walk the ports from the last one to
// the first one, as Lists.reverse does.
func (p *NetworkSimplexPlacer) transformPorts(ports []*lgraph.LPort, reverse bool, corners *nodeRep) {
	if len(ports) == 0 {
		// nothing to do ...
		// the top and bottom border of the node are already safely spaced apart
		return
	}

	portSpacing := lgraph.IndividualOrDefault(corners.origin, lopts.SpacingPortPort)
	portSurrounding := lgraph.IndividualOrDefault(corners.origin, lopts.SpacingPortsSurrounding)
	if portSurrounding == nil {
		// No additional port spacing set
		portSurrounding = emath.NewElkMargin(0, 0, 0, 0)
	}

	lastNNode := corners.head
	var lastPort *lgraph.LPort
	for i := range ports {
		port := ports[i]
		if reverse {
			port = ports[len(ports)-1-i]
		}

		// spacing between the current pair of ports (or to the top border of the node)
		var spacing float64
		if lastPort == nil {
			spacing = portSurrounding.Top
		} else {
			spacing = portSpacing
			spacing += lastPort.Size.Y
		}

		// create NNode for the port
		nNode := networksimplex.NNodeOf().
			Origin(port).
			Type("port").
			Create(p.nGraph)
		p.portMap[port] = nNode

		// connect with previous NNode
		networksimplex.NEdgeOf().
			Weight(0).
			Delta(gwt.RoundInt(math.Ceil(spacing))).
			Source(lastNNode).
			Target(nNode).
			Create()

		lastPort = port
		lastNNode = nNode
	}

	// and connect to the bottom border
	networksimplex.NEdgeOf().
		Weight(0).
		Delta(gwt.RoundInt(math.Ceil(portSurrounding.Bottom + lastPort.Size.Y))).
		Source(lastNNode).
		Target(corners.tail).
		Create()
}

// isHandledEdge is isHandledEdge(edge): the edge is not a self loop and not
// an edge inside a layer.
func isHandledEdge(edge *lgraph.LEdge) bool {
	return !edge.IsSelfLoop() && !edge.IsInLayerEdge()
}

// transformEdges is transformEdges().
func (p *NetworkSimplexPlacer) transformEdges() {
	for _, layer := range p.lGraph.Layers {
		for _, node := range layer.Nodes {
			for _, port := range node.Ports {
				for _, e := range port.OutgoingEdges {
					if isHandledEdge(e) {
						p.transformEdge(e)
					}
				}
			}
		}
	}
}

// transformEdge is transformEdge(lEdge): an edge becomes a dummy node with
// one edge to the source and one edge to the target.
func (p *NetworkSimplexPlacer) transformEdge(lEdge *lgraph.LEdge) {
	// a dummy node
	dummy := networksimplex.NNodeOf().
		Type("edge").
		Create(p.nGraph)

	// calculate port offsets
	srcRep := p.rep(lEdge.Source.Node())
	tgtRep := p.rep(lEdge.Target.Node())
	srcPort := lEdge.Source
	tgtPort := lEdge.Target

	srcOffset := srcPort.Anchor.Y
	tgtOffset := tgtPort.Anchor.Y
	// for non-flexible nodes, ports are relative to node positions
	if !srcRep.isFlexible {
		srcOffset += srcPort.Pos.Y
	}
	if !tgtRep.isFlexible {
		tgtOffset += tgtPort.Pos.Y
	}

	tgtDelta := gwt.RoundInt(max(0, srcOffset-tgtOffset))
	srcDelta := gwt.RoundInt(max(0, tgtOffset-srcOffset))

	weight := getEdgeWeight(lEdge)

	// an edge to the source. Create stops the layout for a port without a
	// network simplex node, where the Java code throws.
	left := networksimplex.NEdgeOf().
		Weight(weight).
		Delta(srcDelta).
		Source(dummy).
		Target(p.portMap[lEdge.Source]).
		Create()

	// an edge to the target
	right := networksimplex.NEdgeOf().
		Weight(weight).
		Delta(tgtDelta).
		Source(dummy).
		Target(p.portMap[lEdge.Target]).
		Create()

	// remember
	p.edgeReps[lEdge.ID] = &edgeRep{left: left, right: right}
}

// insertInLayerEdgeAuxiliaryEdges is insertInLayerEdgeAuxiliaryEdges(): it
// adds edges that keep the edges of inverted ports short.
func (p *NetworkSimplexPlacer) insertInLayerEdgeAuxiliaryEdges() {
	for _, l := range p.lGraph.Layers {
		for _, n := range l.Nodes {
			if n.Type != lgraph.NodeTypeNormal {
				continue
			}
			// getConnectedEdges() has for each port the incoming edges, then
			// the outgoing edges.
			for _, port := range n.Ports {
				for _, e := range port.IncomingEdges {
					if e.IsInLayerEdge() {
						p.insertInLayerEdgeAuxiliaryEdge(e)
					}
				}
				for _, e := range port.OutgoingEdges {
					if e.IsInLayerEdge() {
						p.insertInLayerEdgeAuxiliaryEdge(e)
					}
				}
			}
		}
	}
}

// insertInLayerEdgeAuxiliaryEdge is the body of the loop of
// insertInLayerEdgeAuxiliaryEdges() for one edge inside a layer.
func (p *NetworkSimplexPlacer) insertInLayerEdgeAuxiliaryEdge(inLayerEdge *lgraph.LEdge) {
	srcIsDummy := inLayerEdge.Source.Node().Type != lgraph.NodeTypeNormal

	thePort := inLayerEdge.Source
	if srcIsDummy {
		thePort = inLayerEdge.Target
	}
	dummyNode := inLayerEdge.OtherPort(thePort).Node()

	portRep := p.portMap[thePort]
	dummyRep := p.rep(dummyNode).head // head/tail doesn't matter since it's a dummy node

	var src, tgt *networksimplex.NNode
	if thePort.Node().Index() < dummyNode.Index() {
		// port --> dummy
		src = portRep
		tgt = dummyRep
	} else {
		// dummy --> port
		src = dummyRep
		tgt = portRep
	}

	networksimplex.NEdgeOf().
		Delta(0).
		Weight(edgeWeightBase).
		Source(src).
		Target(tgt).
		Create()
}

// insertNorthSouthAuxiliaryEdges is insertNorthSouthAuxiliaryEdges(): it
// adds edges that keep the edges of north and south ports short.
func (p *NetworkSimplexPlacer) insertNorthSouthAuxiliaryEdges() {
	for _, l := range p.lGraph.Layers {
		for _, n := range l.Nodes {
			for _, sp := range n.PortSideView(core.PortSideSouth) {
				other := prop.Get(sp, iprops.PortDummy)
				// if no edge was attached to the port, no dummy was created ...
				if other != nil {
					networksimplex.NEdgeOf().
						Delta(0). // doesn't matter, separation is already taken care off
						Weight(smallEdgeWeight).
						Source(p.nodeReps[n.ID].tail).
						Target(p.repOfPortDummy(other).head).
						Create()
				}
			}

			for _, sp := range n.PortSideView(core.PortSideNorth) {
				other := prop.Get(sp, iprops.PortDummy)
				// if no edge was attached to the port, no dummy was created ...
				if other != nil {
					networksimplex.NEdgeOf().
						Delta(0). // doesn't matter
						Weight(smallEdgeWeight).
						Source(p.repOfPortDummy(other).tail).
						Target(p.nodeReps[n.ID].head).
						Create()
				}
			}
		}
	}
}

// insertFlexibleWhereSpaceAuxiliaryEdges is
// insertFlexibleWhereSpaceAuxiliaryEdges(): the edges for the nodes with the
// flexibility NODE_SIZE_WHERE_SPACE_PERMITS.
func (p *NetworkSimplexPlacer) insertFlexibleWhereSpaceAuxiliaryEdges() {
	if len(p.nGraph.Nodes) == 0 {
		// The Java code fails with a NoSuchElementException.
		lgraph.Bail("NetworkSimplexPlacer: the auxiliary graph has no nodes")
	}
	minLayer := p.nGraph.Nodes[0].Layer
	maxLayer := p.nGraph.Nodes[0].Layer
	for _, n := range p.nGraph.Nodes {
		minLayer = min(minLayer, n.Layer)
		maxLayer = max(maxLayer, n.Layer)
	}
	usedLayers := maxLayer - minLayer

	globalSource := networksimplex.NNodeOf().Create(p.nGraph)
	globalSink := networksimplex.NNodeOf().Create(p.nGraph)

	// make sure the distance between source and sink is preserved
	networksimplex.NEdgeOf().
		Weight(nodeSizeWeightStatic * 2).
		Delta(usedLayers).
		Source(globalSource).
		Target(globalSink).
		Create()

	// fix the position of most non-flexible nodes and make sure the flexible nodes
	// can only increase in size
	for _, nr := range p.nodeReps {
		if nr.origin.Type != lgraph.NodeTypeNormal {
			continue
		}
		if len(nr.origin.Ports) <= 1 { // allow leaves to move
			continue
		}
		networksimplex.NEdgeOf().
			Weight(0).
			Delta(nr.tail.Layer - minLayer).
			Source(globalSource).
			Target(nr.tail).
			Create()

		networksimplex.NEdgeOf().
			Weight(0).
			Delta(usedLayers - nr.head.Layer).
			Source(nr.head).
			Target(globalSink).
			Create()
	}
}

// ------------------------------------------------------------------------------------------------
//                                       Apply Layout
// ------------------------------------------------------------------------------------------------

// applyPositions is applyPositions().
func (p *NetworkSimplexPlacer) applyPositions() {
	for _, l := range p.lGraph.Layers {
		for _, lNode := range l.Nodes {
			// find the node's corners
			rep := p.nodeReps[lNode.ID]
			minY := float64(rep.head.Layer)
			maxY := float64(rep.tail.Layer)

			// set new position and size
			lNode.Pos.Y = minY

			sizeDelta := (maxY - minY) - lNode.Size.Y

			flexibleNode := isFlexibleNode(lNode)
			nf := getNodeFlexibility(lNode)

			// modify the size?
			if flexibleNode && isFlexibleSizeWhereSpacePermits(nf) {
				lNode.Size.Y += sizeDelta
			}

			// reposition ports if allowed
			if flexibleNode && isFlexiblePorts(nf) {
				for _, port := range lNode.Ports {
					if isEastOrWest(port.Side) {
						nNode := p.portMap[port]
						if nNode == nil {
							// The Java code fails with a NullPointerException.
							lgraph.Bail("NetworkSimplexPlacer: a port of a flexible node has no network simplex node")
						}
						port.Pos.Y = float64(nNode.Layer) - minY
					}
				}
				// when the node got resized, the positions of labels and south ports have to be adjusted
				for _, label := range lNode.Labels {
					adjustLabelPosition(lNode, label, sizeDelta)
				}
				if isFlexibleSizeWhereSpacePermits(nf) {
					for _, port := range lNode.PortSideView(core.PortSideSouth) {
						port.Pos.Y += sizeDelta
					}
				}
			}
		}
	}
}

// adjustLabelPosition is adjustLabelPosition(node, label, sizeDelta).
func adjustLabelPosition(node *lgraph.LNode, label *lgraph.LLabel, sizeDelta float64) {
	placement := prop.Get(node, lopts.NodeLabelsPlacement)
	if placement.Contains(core.NodeLabelPlacementVBottom) {
		label.Pos.Y += sizeDelta
	} else if placement.Contains(core.NodeLabelPlacementVCenter) {
		label.Pos.Y += float64(sizeDelta / 2.0)
	}
	// V_TOP placement does not require adjustment
}

// ------------------------------------------------------------------------------------------------
//                                        Convenience
// ------------------------------------------------------------------------------------------------

// getEdgeWeight is getEdgeWeight(edge): the weight of an edge as Gansner et
// al. give it. An edge of a long edge dummy has a higher weight, so that long
// edges are straight with a higher priority. The option
// priority.straightness counts too.
func getEdgeWeight(edge *lgraph.LEdge) float64 {
	priority := max(1, prop.Get(edge, lopts.PriorityStraightness))
	edgeTypeWeight := getEdgeTypeWeight(edge.Source.Node().Type, edge.Target.Node().Type)
	return float64(float64(priority) * edgeTypeWeight)
}

// getEdgeTypeWeight is getEdgeWeight(nodeType1, nodeType2).
func getEdgeTypeWeight(nodeType1, nodeType2 lgraph.NodeType) float64 {
	if nodeType1 == lgraph.NodeTypeNormal && nodeType2 == lgraph.NodeTypeNormal {
		return 1 * edgeWeightBase
	} else if nodeType1 == lgraph.NodeTypeNormal || nodeType2 == lgraph.NodeTypeNormal {
		return 2 * edgeWeightBase
	}
	return 8 * edgeWeightBase
}

// nodeRep is NetworkSimplexPlacer.NodeRep: the representation of a node in
// the auxiliary graph.
type nodeRep struct {
	origin *lgraph.LNode
	// head is the border of the node with the lower y coordinate.
	head *networksimplex.NNode
	// tail is the border of the node with the larger y coordinate.
	tail *networksimplex.NNode
	// isFlexible tells that the node is flexible: head and tail are two
	// nodes.
	isFlexible bool
}

// edgeRep is NetworkSimplexPlacer.EdgeRep: the two edges of the auxiliary
// graph that stand for one edge.
type edgeRep struct {
	left  *networksimplex.NEdge
	right *networksimplex.NEdge
}

// isStraight is isStraight(): the source and the target of the edge have
// the same y coordinate.
func (e *edgeRep) isStraight() bool {
	return e.notStraightBy() == 0
}

// notStraightBy is notStraightBy(): the difference of the y coordinates of
// the source and the target of the edge. A value above 0 tells that the
// source is below the target.
func (e *edgeRep) notStraightBy() int {
	return (e.left.Target.Layer - e.left.Delta) - (e.right.Target.Layer - e.right.Delta)
}

// ------------------------------------------------------------------------------------------------
//                                       Flexible Ports
// ------------------------------------------------------------------------------------------------

// getNodeFlexibility is NodeFlexibility.getNodeFlexibility(lNode): the
// option nodeFlexibility of the node, or the option nodeFlexibility.default
// of its graph for a node without the option.
//
// TODO(core): move to lgraph or lenum. The methods of the Java enum
// NodeFlexibility are here because lenum has no enum methods.
func getNodeFlexibility(lNode *lgraph.LNode) lenum.NodeFlexibility {
	if prop.Has(lNode, lopts.NodePlacementNetworkSimplexNodeFlexibility) {
		return prop.Get(lNode, lopts.NodePlacementNetworkSimplexNodeFlexibility)
	}
	graph := lNode.Graph()
	if graph == nil {
		lgraph.Bail("NetworkSimplexPlacer: a node has no graph")
	}
	return prop.Get(graph, lopts.NodePlacementNetworkSimplexNodeFlexibilityDefault)
}

// isFlexibleSize is NodeFlexibility.isFlexibleSize().
func isFlexibleSize(nf lenum.NodeFlexibility) bool {
	return nf == lenum.NodeFlexibilityNodeSize
}

// isFlexibleSizeWhereSpacePermits is
// NodeFlexibility.isFlexibleSizeWhereSpacePermits().
func isFlexibleSizeWhereSpacePermits(nf lenum.NodeFlexibility) bool {
	return nf == lenum.NodeFlexibilityNodeSizeWhereSpacePermits || nf == lenum.NodeFlexibilityNodeSize
}

// isFlexiblePorts is NodeFlexibility.isFlexiblePorts().
func isFlexiblePorts(nf lenum.NodeFlexibility) bool {
	return nf == lenum.NodeFlexibilityPortPosition || nf == lenum.NodeFlexibilityNodeSizeWhereSpacePermits || nf == lenum.NodeFlexibilityNodeSize
}

// isFlexibleNode is isFlexibleNode(lNode). A node is flexible if its port
// constraints are not FIXED_POS, its flexibility is not NONE, and it is tall
// enough to give the ports on the west side and on the east side the room to
// move. A node that may not grow and is too small for its ports keeps the
// port positions that the label and node size processor computed.
func isFlexibleNode(lNode *lgraph.LNode) bool {
	// dummies are not flexible!
	if lNode.Type != lgraph.NodeTypeNormal {
		return false
	}

	// at least two ports are required ...
	if len(lNode.Ports) <= 1 {
		return false
	}

	// if ports may not be moved there's no use in enlarging the node
	pc := prop.Get(lNode, lopts.PortConstraints)
	if pc.IsPosFixed() {
		return false
	}

	nf := getNodeFlexibility(lNode)
	if nf == lenum.NodeFlexibilityNone {
		return false
	}

	// if we cannot resize the node, and the given height is not enough to properly accommodate
	// all ports, reuse the existing port positions
	if !isFlexibleSizeWhereSpacePermits(nf) {
		// we are not allowed to increase the node's size
		portSpacing := lgraph.IndividualOrDefault(lNode, lopts.SpacingPortPort)
		additionalPortSpacing := prop.Get(lNode, lopts.SpacingPortsSurrounding)
		if additionalPortSpacing == nil {
			additionalPortSpacing = emath.NewElkMargin(portSpacing, portSpacing, portSpacing, portSpacing)
		}

		// check west side
		westPorts := lNode.PortSideView(core.PortSideWest)
		requiredWestHeight := additionalPortSpacing.Top +
			additionalPortSpacing.Bottom +
			float64(float64(len(westPorts)-1)*portSpacing)
		if requiredWestHeight > lNode.Size.Y {
			return false
		}

		// check east side
		eastPorts := lNode.PortSideView(core.PortSideEast)
		requiredEastHeight := additionalPortSpacing.Top +
			additionalPortSpacing.Bottom +
			float64(float64(len(eastPorts)-1)*portSpacing)
		if requiredEastHeight > lNode.Size.Y {
			return false
		}
	}
	return true
}

// ------------------------------------------------------------------------------------------------
//                                      Edge Straightening
// ------------------------------------------------------------------------------------------------

// path is NetworkSimplexPlacer.Path: a list of edges.
type path []*lgraph.LEdge

// containsLongEdgeDummy is containsLongEdgeDummy(): a node of the path has
// the type LONG_EDGE.
func (pa path) containsLongEdgeDummy() bool {
	if len(pa) == 0 {
		return false
	}
	if pa[0].Source.Node().Type == lgraph.NodeTypeLongEdge {
		return true
	}
	for _, e := range pa {
		if e.Target.Node().Type == lgraph.NodeTypeLongEdge {
			return true
		}
	}
	return false
}

// containsFlexibleNode is containsFlexibleNode(p): the flexibility of a node
// of the path fulfills the predicate.
func (pa path) containsFlexibleNode(predicate func(lenum.NodeFlexibility) bool) bool {
	if len(pa) == 0 {
		return false
	}
	nf := getNodeFlexibility(pa[0].Source.Node())
	if predicate(nf) {
		return true
	}
	for _, e := range pa {
		if predicate(getNodeFlexibility(e.Target.Node())) {
			return true
		}
	}
	return false
}

// orderTwoPath is orderTwoPath(): it orders the two edges of the path so
// that the shared node is the target of the first edge and the source of the
// second edge.
func (pa path) orderTwoPath() {
	if len(pa) != 2 {
		// The Java code throws an IllegalStateException.
		lgraph.Bail("NetworkSimplexPlacer: Order only allowed for two paths.")
	}
	first := pa[0]
	second := pa[1]
	if first.Target.Node() != second.Source.Node() {
		pa[0] = second
		pa[1] = first
	}
}

// isTwoPathCenterNodeFlexible is isTwoPathCenterNodeFlexible(): the target
// node of the first edge is flexible. The path must be ordered.
func (pa path) isTwoPathCenterNodeFlexible() bool {
	return isFlexibleNode(pa[0].Target.Node())
}

// preferStraightEdges is preferStraightEdges(): it changes the weights of
// paths in the way of long edges, and it collects the paths of two edges for
// the post processing.
func (p *NetworkSimplexPlacer) preferStraightEdges() {
	// the nodes were counted and indexed during #prepare
	p.nodeState = make([]int, p.nodeCount)
	p.twoPaths = nil

	// record node states
	for _, l := range p.lGraph.Layers {
		for _, n := range l.Nodes {
			p.nodeState[n.ID] = getNodeState(n)
		}
	}

	p.markEdgeCrossings()
	identifiedPaths := p.identifyPaths()

	// essentially 'long paths' are treated like 'long edges':
	// the weights of the connecting edges are altered to straighten the path as much as possible
	for _, pa := range identifiedPaths {

		if len(pa) <= 1 {
			continue
		}

		// remember 'two paths' for processing after network simplex has been executed
		if len(pa) == 2 {
			pa.orderTwoPath()
			if !pa.isTwoPathCenterNodeFlexible() {
				p.twoPaths = append(p.twoPaths, pa)
			}
			continue
		}

		// ignore paths that contain long edge dummies, and paths that contain flexible nodes that allow resizing
		if pa.containsLongEdgeDummy() || pa.containsFlexibleNode(isFlexibleSizeWhereSpacePermits) {
			continue
		}

		var last *lgraph.LEdge
		for i, cur := range pa {
			curRep := p.edgeRepOf(cur)

			var weight float64
			if last == nil || i == len(pa)-1 {
				// first or last segment
				weight = getEdgeTypeWeight(lgraph.NodeTypeNormal, lgraph.NodeTypeLongEdge)
			} else {
				weight = getEdgeTypeWeight(lgraph.NodeTypeLongEdge, lgraph.NodeTypeLongEdge)
			}

			// at this point one can decide whether long edges are more important than "paths"
			weight = float64(weight * longEdgeVsPathFactor)

			oldLeftWeight := curRep.left.Weight
			curRep.left.Weight = max(oldLeftWeight, oldLeftWeight+(weight-oldLeftWeight))
			oldRightWeight := curRep.right.Weight
			curRep.right.Weight = max(oldRightWeight, oldRightWeight+(weight-oldRightWeight))

			last = cur
		}
	}
}

// postProcessTwoPaths is postProcessTwoPaths(): it improves the paths of two
// edges until it can remove no more bends.
func (p *NetworkSimplexPlacer) postProcessTwoPaths() {
	var s []path
	for _, pa := range p.twoPaths {
		tryAgain := p.improveTwoPath(pa, true)
		if tryAgain {
			s = append(s, pa)
		}
	}

	for len(s) != 0 {
		pa := s[len(s)-1]
		s = s[:len(s)-1]
		p.improveTwoPath(pa, false)
	}
}

// fuzzyEquals is DoubleMath.fuzzyEquals(a, b, tolerance) of Guava.
func fuzzyEquals(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance || a == b || (a != a && b != b)
}

// improveTwoPath is improveTwoPath(path, probe). A path of two edges
// (u, center, v) has this form in the auxiliary graph:
//
//	u  <--leftEdge.left-- d1 --leftEdge.right--> center <--rightEdge.left-- d2 --rightEdge.right--> v
//
// The path is ordered: the center node is the target of the first edge. The
// center node is not flexible.
//
// It returns true if it changed nothing and the path must be checked again
// later, and false if the path needs no more processing.
func (p *NetworkSimplexPlacer) improveTwoPath(pa path, probe bool) bool {
	leftEdge := p.edgeRepOf(pa[0])
	rightEdge := p.edgeRepOf(pa[1])

	// is the edge already straight?
	if leftEdge.isStraight() && rightEdge.isStraight() {
		return false
	}

	// get center node
	centerOrigin := leftEdge.right.Target.Origin
	// only two paths without flexible nodes are allowed here
	// can be a node or a port
	centerNode, ok := centerOrigin.(*lgraph.LNode)
	if !ok || centerNode == nil {
		return false
	}
	nNode := p.rep(centerNode)

	// identify on which side there is more space
	nodeIndex := centerNode.Index()
	aboveDist := math.Inf(1)
	if nodeIndex > 0 {
		above := centerNode.Layer().Nodes[nodeIndex-1]
		aboveRep := p.rep(above)
		spacing := math.Ceil(verticalSpacing(p.spacings, above, centerNode))
		aboveDist = (float64(nNode.head.Layer) - centerNode.Margin.Top) -
			(float64(aboveRep.head.Layer) + above.Size.Y + above.Margin.Bottom) -
			spacing
	}
	belowDist := math.Inf(1)
	if nodeIndex < len(centerNode.Layer().Nodes)-1 {
		if nodeIndex < 0 {
			lgraph.Bail("NetworkSimplexPlacer: a node is not in its layer")
		}
		below := centerNode.Layer().Nodes[nodeIndex+1]
		belowRep := p.rep(below)
		spacing := math.Ceil(verticalSpacing(p.spacings, below, centerNode))
		belowDist = (float64(belowRep.head.Layer) - below.Margin.Top) -
			(float64(nNode.head.Layer) + centerNode.Size.Y + centerNode.Margin.Bottom) -
			spacing
	}

	// same space on both sides, check again later
	if probe && fuzzyEquals(aboveDist, belowDist, epsilon) {
		return true
	}

	// from the way the auxiliary graph is built, one can differentiate four cases (A-D):
	// Case A:          Case B:         Case C:         Case D:
	//          o-v        u-o
	//         /              \            u-o   o-v         o-c-o
	//      o-c                c-o            \ /           /     \
	//     /                      \            c           u       v
	//    u                        v
	// Note that case C and D only occur if node c is blocked either above or below, respectively.
	// Otherwise the edge would be straight.
	//
	// In every case there are two non-straight edges. When routing edges orthogonally later on,
	// one saves a bend point if only one of the four edges is non-straight. To straighten one of the
	// two candidate edges, node c has to be moved up or down. It depends on the adjacent nodes in c's layer
	// if this is possible.

	// the following variables represent the length of each of the four edges,
	// i.e. if possible node c has to be moved by this distance (either up or down)
	// O--a--o--b--O--c--o--d--O
	a := +length(leftEdge.left)
	b := -length(leftEdge.right)
	c := -length(rightEdge.left)
	d := +length(rightEdge.right)

	caseD := leftEdge.notStraightBy() > 0 && rightEdge.notStraightBy() < 0
	caseC := leftEdge.notStraightBy() < 0 && rightEdge.notStraightBy() > 0
	caseB := leftEdge.left.Target.Layer+leftEdge.right.Delta < rightEdge.right.Target.Layer+rightEdge.left.Delta
	caseA := leftEdge.left.Target.Layer+leftEdge.right.Delta > rightEdge.right.Target.Layer+rightEdge.left.Delta

	move := 0
	if !caseD && !caseC {
		if caseA {
			if aboveDist+float64(c) > 0 {
				move = c
			} else if belowDist-float64(a) > 0 {
				move = a
			}
		} else if caseB {
			if aboveDist+float64(b) > 0 {
				move = b
			} else if belowDist-float64(d) > 0 {
				move = d
			}
		}
	}

	// move the center node
	nNode.head.Layer += move
	if nNode.isFlexible {
		nNode.tail.Layer += move
	}

	return false
}

// length is length(edge): the length of an edge of the auxiliary graph above
// its minimum length.
func length(edge *networksimplex.NEdge) int {
	diff := edge.Source.Layer - edge.Target.Layer
	if diff < 0 {
		diff = -diff
	}
	return diff - edge.Delta
}

// ------------------------------------------------------------------------------------------------
//                                      Path identification
// ------------------------------------------------------------------------------------------------

// identifyPaths is identifyPaths(): the paths that start at a junction.
func (p *NetworkSimplexPlacer) identifyPaths() []path {
	var paths []path
	for _, l := range p.lGraph.Layers {
		for _, junction := range l.Nodes {
			if p.nodeState[junction.ID] != nodeStateJunction {
				continue
			}
			// getConnectedEdges() has for each port the incoming edges, then
			// the outgoing edges.
			for _, port := range junction.Ports {
				for _, e := range port.IncomingEdges {
					paths = p.addPath(paths, e, junction)
				}
				for _, e := range port.OutgoingEdges {
					paths = p.addPath(paths, e, junction)
				}
			}
		}
	}
	return paths
}

// addPath is the body of the loop of identifyPaths() for one edge of a
// junction.
func (p *NetworkSimplexPlacer) addPath(paths []path, e *lgraph.LEdge, junction *lgraph.LNode) []path {
	if !isHandledEdge(e) {
		return paths
	}
	pa := p.follow(e, junction, nil)
	if len(pa) > 1 {
		paths = append(paths, pa)
	}
	return paths
}

// follow is follow(edge, current, path): it adds the edge to the path and
// goes on at the other end of the edge, until it comes to a node that is
// visited or a junction, or to an edge that crosses another edge.
func (p *NetworkSimplexPlacer) follow(edge *lgraph.LEdge, current *lgraph.LNode, pa path) path {
	for {
		other := edge.OtherNode(current)
		pa = append(pa, edge)

		// stop criteria
		if p.nodeState[other.ID] == nodeStateVisited ||
			p.nodeState[other.ID] == nodeStateJunction ||
			p.crossing[edge.ID] {
			return pa
		}

		// recurse
		p.nodeState[other.ID] = nodeStateVisited
		next := firstOtherHandledEdge(other, edge)
		if next == nil {
			return pa
		}
		edge, current = next, other
	}
}

// firstOtherHandledEdge returns the first connected edge of the node that is
// handled and is not the given edge, or nil.
func firstOtherHandledEdge(node *lgraph.LNode, edge *lgraph.LEdge) *lgraph.LEdge {
	for _, port := range node.Ports {
		for _, incident := range port.IncomingEdges {
			if !isHandledEdge(incident) || incident == edge {
				continue
			}
			return incident
		}
		for _, incident := range port.OutgoingEdges {
			if !isHandledEdge(incident) || incident == edge {
				continue
			}
			return incident
		}
	}
	return nil
}

// getNodeState is getNodeState(node): JUNCTION for a node with more than one
// incoming edge, with more than one outgoing edge, or with exactly one
// incident edge. Self loops do not count.
func getNodeState(node *lgraph.LNode) int {
	inco := 0
	ouco := 0
	for _, port := range node.Ports {
		for _, e := range port.IncomingEdges {
			if !e.IsSelfLoop() {
				inco++
			}
		}
		for _, e := range port.OutgoingEdges {
			if !e.IsSelfLoop() {
				ouco++
			}
		}
		if inco > 1 || ouco > 1 {
			return nodeStateJunction
		}
	}
	if inco+ouco == 1 {
		return nodeStateJunction
	}
	return nodeStateOther
}

// ------------------------------------------------------------------------------------------------
//                                      Mark Crossings
// ------------------------------------------------------------------------------------------------

// markEdgeCrossings is markEdgeCrossings(): it marks each edge that crosses
// another edge in the array crossing.
func (p *NetworkSimplexPlacer) markEdgeCrossings() {
	p.crossing = make([]bool, p.edgeCount)
	layers := p.lGraph.Layers
	for i := 0; i+1 < len(layers); i++ {
		p.markCrossingEdges(layers[i], layers[i+1])
	}
}

// markCrossingEdges is markCrossingEdges(left, right): it marks the
// crossings of the edges between two layers.
func (p *NetworkSimplexPlacer) markCrossingEdges(left, right *lgraph.Layer) {
	var openEdges []*lgraph.LEdge

	// add all edges in the order they occur in the left layer
	for _, node := range left.Nodes {
		for _, port := range node.PortSideView(core.PortSideEast) {
			for _, edge := range port.OutgoingEdges {
				if edge.IsInLayerEdge() || edge.IsSelfLoop() || edge.Target.Node().Layer() != right {
					continue
				}
				openEdges = append(openEdges, edge)
			}
		}
	}

	// close the edges one after another, recording edge crossings
	for ni := len(right.Nodes) - 1; ni >= 0; ni-- {
		node := right.Nodes[ni]
		for _, port := range node.PortSideView(core.PortSideWest) { // don't reverse, bottom up is correct
			for _, edge := range port.IncomingEdges {
				if edge.IsInLayerEdge() || edge.IsSelfLoop() || edge.Source.Node().Layer() != left {
					continue
				}
				if len(openEdges) != 0 {
					// The list iterator of the Java code starts at the end.
					i := len(openEdges) - 1
					last := openEdges[i]
					for last != edge && i > 0 {
						// mark both edges as being part of an edge crossing
						p.crossing[last.ID] = true
						p.crossing[edge.ID] = true
						i--
						last = openEdges[i]
					}
					if i > 0 {
						openEdges = append(openEdges[:i], openEdges[i+1:]...)
					}
				}
			}
		}
	}
}
