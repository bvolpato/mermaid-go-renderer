// Ported from LGraphElement.java, LShape.java, LGraph.java, Layer.java,
// LNode.java, LPort.java, LEdge.java, LLabel.java, LMargin.java and
// LPadding.java in
// plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/graph of
// Eclipse ELK 0.9.1 (EPL-2.0).

// Package lgraph has the graph model of the layered algorithm of ELK: the
// LGraph with its layers, nodes, ports, edges and labels.
//
// The lists of the Java classes are exported slices. The methods that keep
// both ends of a relation in step have the Java names: SetLayer, SetNode,
// SetSource, SetTarget. Use them, and change a slice directly only where the
// Java code changes the list directly (a sort, a reorder, a clear).
package lgraph

import (
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// Element is LGraphElement: an object with properties and an id. The id has
// no fixed meaning. Each processor uses it as it needs, and a later processor
// can read what an earlier one left there.
type Element struct {
	prop.MapHolder
	// ID is the public field id.
	ID int
}

// OriginRef is implemented by the values of the property origin that name an
// element of the ELK graph. ElkRef returns a text that identifies the
// element, for example "n:id" for a node.
type OriginRef interface {
	ElkRef() string
	// ElkIdentifier is the identifier of the element, or "" without one.
	ElkIdentifier() string
}

// designation is LGraphElement.getDesignation: the identifier of the origin.
func (e *Element) designation() (string, bool) {
	if o, ok := prop.Lookup(e, origin); ok {
		if r, ok := o.(OriginRef); ok {
			if id := r.ElkIdentifier(); id != "" {
				return id, true
			}
		}
	}
	return "", false
}

// LGraph is the class LGraph.
type LGraph struct {
	Element
	// Size is the size of the graph without the padding.
	Size emath.KVector
	// Padding is the padding of the graph.
	Padding emath.ElkPadding
	// Offset is the offset that is added to each coordinate.
	Offset emath.KVector
	// LayerlessNodes has the nodes that are in no layer.
	LayerlessNodes []*LNode
	// Layers has the layers in order.
	Layers []*Layer
	// ParentNode is the node whose nested graph this is, or nil.
	ParentNode *LNode
}

// NewLGraph is new LGraph().
func NewLGraph() *LGraph { return &LGraph{} }

// ActualSize is getActualSize(): the size with the padding.
func (g *LGraph) ActualSize() *emath.KVector {
	return emath.NewKVector(g.Size.X+g.Padding.Left+g.Padding.Right, g.Size.Y+g.Padding.Top+g.Padding.Bottom)
}

// ToNodeArray is toNodeArray(): a copy of the node lists of the layers.
func (g *LGraph) ToNodeArray() [][]*LNode {
	out := make([][]*LNode, len(g.Layers))
	for i, layer := range g.Layers {
		out[i] = append([]*LNode(nil), layer.Nodes...)
	}
	return out
}

// String is toString().
func (g *LGraph) String() string {
	if len(g.Layers) == 0 {
		return "G-unlayered" + nodesString(g.LayerlessNodes)
	} else if len(g.LayerlessNodes) == 0 {
		return "G-layered" + layersString(g.Layers)
	}
	return "G[layerless" + nodesString(g.LayerlessNodes) + ", layers" + layersString(g.Layers) + "]"
}

func nodesString(nodes []*LNode) string {
	parts := make([]string, len(nodes))
	for i, n := range nodes {
		parts[i] = n.String()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func layersString(layers []*Layer) string {
	parts := make([]string, len(layers))
	for i, l := range layers {
		parts[i] = l.String()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// Layer is the class Layer.
type Layer struct {
	Element
	// Size is the size of the layer.
	Size emath.KVector
	// Nodes has the nodes of the layer in order.
	Nodes []*LNode
	owner *LGraph
}

// NewLayer is new Layer(graph). It does not add the layer to the graph.
func NewLayer(graph *LGraph) *Layer { return &Layer{owner: graph} }

// Graph is getGraph().
func (l *Layer) Graph() *LGraph { return l.owner }

// Index is getIndex(): the index in the layer list of the graph, or -1.
func (l *Layer) Index() int {
	for i, x := range l.owner.Layers {
		if x == l {
			return i
		}
	}
	return -1
}

// String is toString().
func (l *Layer) String() string {
	return "L_" + strconv.Itoa(l.Index()) + nodesString(l.Nodes)
}

// NodeType is the enum LNode.NodeType.
type NodeType int

// The constants of NodeType, in the order of the Java enum.
const (
	NodeTypeNormal NodeType = iota
	NodeTypeLongEdge
	NodeTypeExternalPort
	NodeTypeNorthSouthPort
	NodeTypeLabel
	NodeTypeBreakingPoint
)

var namesNodeType = []string{"NORMAL", "LONG_EDGE", "EXTERNAL_PORT", "NORTH_SOUTH_PORT", "LABEL", "BREAKING_POINT"}

// String returns the name of the Java constant.
func (t NodeType) String() string { return prop.EnumName(namesNodeType, int(t)) }

func init() {
	prop.RegisterEnum[NodeType]("org.eclipse.elk.alg.layered.graph.LNode$NodeType", namesNodeType)
}

// PortSideRange is a range of indices of the port list of a node: the ports
// of one side when the list is sorted.
type PortSideRange struct {
	Side        core.PortSide
	First, Last int
}

// LNode is the class LNode.
type LNode struct {
	Element
	// Pos is the position (getPosition()).
	Pos emath.KVector
	// Size is the size (getSize()).
	Size emath.KVector
	// Type is the node type.
	Type NodeType
	// Ports has the ports in order.
	Ports []*LPort
	// Labels has the labels.
	Labels []*LLabel
	// NestedGraph is the graph inside the node, or nil.
	NestedGraph *LGraph
	// Margin is the margin around the node.
	Margin emath.ElkMargin
	// Padding is the padding inside the node.
	Padding emath.ElkPadding

	graph *LGraph
	layer *Layer
	// PortSidesCached and PortSideIndices are the cache of getPortSideView:
	// the index range of each side that has ports, in the order of the sides.
	PortSidesCached bool
	PortSideIndices []PortSideRange
}

// NewLNode is new LNode(graph). It does not add the node to the graph.
func NewLNode(graph *LGraph) *LNode { return &LNode{graph: graph} }

// Layer is getLayer().
func (n *LNode) Layer() *Layer { return n.layer }

func removeNode(list []*LNode, n *LNode) []*LNode {
	for i, x := range list {
		if x == n {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

// SetLayer is setLayer(layer): it moves the node to the end of the layer. A
// nil layer removes the node from its layer.
func (n *LNode) SetLayer(layer *Layer) {
	if n.layer != nil {
		n.layer.Nodes = removeNode(n.layer.Nodes, n)
	}
	n.layer = layer
	if n.layer != nil {
		n.layer.Nodes = append(n.layer.Nodes, n)
	}
}

// SetLayerAt is setLayer(index, layer): it moves the node to the given index
// of the layer. The index must be in the range of the node list.
func (n *LNode) SetLayerAt(index int, layer *Layer) {
	if layer != nil && (index < 0 || index > len(layer.Nodes)) {
		panic("lgraph: index must be >= 0 and <= layer node count")
	}
	if n.layer != nil {
		n.layer.Nodes = removeNode(n.layer.Nodes, n)
	}
	n.layer = layer
	if layer != nil {
		layer.Nodes = append(layer.Nodes, nil)
		copy(layer.Nodes[index+1:], layer.Nodes[index:])
		layer.Nodes[index] = n
	}
}

// SetLayerField sets the layer of the node without a change of the node list
// of a layer. Only code that rebuilds a graph from a stored state uses it.
func (n *LNode) SetLayerField(layer *Layer) { n.layer = layer }

// Graph is getGraph(): the graph of the node, or the graph of its layer.
func (n *LNode) Graph() *LGraph {
	if n.graph == nil && n.layer != nil {
		return n.layer.Graph()
	}
	return n.graph
}

// GraphField returns the field graph, which getGraph reads first.
func (n *LNode) GraphField() *LGraph { return n.graph }

// SetGraph is setGraph(graph).
func (n *LNode) SetGraph(graph *LGraph) { n.graph = graph }

// PortsByType is getPorts(PortType): the ports with incoming edges for INPUT,
// and the ports with outgoing edges for OUTPUT.
func (n *LNode) PortsByType(portType lenum.PortType) []*LPort {
	var out []*LPort
	switch portType {
	case lenum.PortTypeInput:
		for _, p := range n.Ports {
			if len(p.IncomingEdges) != 0 {
				out = append(out, p)
			}
		}
	case lenum.PortTypeOutput:
		for _, p := range n.Ports {
			if len(p.OutgoingEdges) != 0 {
				out = append(out, p)
			}
		}
	}
	return out
}

// PortsBySide is getPorts(PortSide): the ports of one side in list order. It
// returns nothing for the side UNDEFINED.
func (n *LNode) PortsBySide(side core.PortSide) []*LPort {
	if side == core.PortSideUndefined {
		return nil
	}
	var out []*LPort
	for _, p := range n.Ports {
		if p.Side == side {
			out = append(out, p)
		}
	}
	return out
}

// PortsByTypeAndSide is getPorts(PortType, PortSide).
func (n *LNode) PortsByTypeAndSide(portType lenum.PortType, side core.PortSide) []*LPort {
	if side == core.PortSideUndefined || (portType != lenum.PortTypeInput && portType != lenum.PortTypeOutput) {
		return nil
	}
	var out []*LPort
	for _, p := range n.Ports {
		if p.Side != side {
			continue
		}
		if portType == lenum.PortTypeInput && len(p.IncomingEdges) != 0 || portType == lenum.PortTypeOutput && len(p.OutgoingEdges) != 0 {
			out = append(out, p)
		}
	}
	return out
}

// PortSideView is getPortSideView(side): the part of the port list that has
// the ports of one side. It is right only when the list is sorted by side.
// The result shares the memory of the port list.
func (n *LNode) PortSideView(side core.PortSide) []*LPort {
	if !n.PortSidesCached {
		n.findPortIndices()
	}
	for _, r := range n.PortSideIndices {
		if r.Side == side {
			return n.Ports[r.First:r.Last:r.Last]
		}
	}
	return nil
}

// CachePortSides is cachePortSides().
func (n *LNode) CachePortSides() {
	n.PortSidesCached = true
	n.findPortIndices()
}

// findPortIndices is findPortIndices(). The Java map is an EnumMap, which
// iterates in the order of the sides.
func (n *LNode) findPortIndices() {
	var ranges [5]*PortSideRange
	first := 0
	currentSide := core.PortSideNorth
	i := 0
	for ; i < len(n.Ports); i++ {
		port := n.Ports[i]
		if port.Side != currentSide {
			if first != i {
				ranges[currentSide] = &PortSideRange{currentSide, first, i}
			}
			currentSide = port.Side
			first = i
		}
	}
	ranges[currentSide] = &PortSideRange{currentSide, first, i}
	n.PortSideIndices = n.PortSideIndices[:0]
	for _, r := range ranges {
		if r != nil {
			n.PortSideIndices = append(n.PortSideIndices, *r)
		}
	}
}

// IncomingEdges is getIncomingEdges(): the incoming edges of each port, in
// port order.
func (n *LNode) IncomingEdges() []*LEdge {
	var out []*LEdge
	for _, p := range n.Ports {
		out = append(out, p.IncomingEdges...)
	}
	return out
}

// OutgoingEdges is getOutgoingEdges().
func (n *LNode) OutgoingEdges() []*LEdge {
	var out []*LEdge
	for _, p := range n.Ports {
		out = append(out, p.OutgoingEdges...)
	}
	return out
}

// ConnectedEdges is getConnectedEdges(): for each port its incoming edges,
// then its outgoing edges.
func (n *LNode) ConnectedEdges() []*LEdge {
	var out []*LEdge
	for _, p := range n.Ports {
		out = append(out, p.IncomingEdges...)
		out = append(out, p.OutgoingEdges...)
	}
	return out
}

// Index is getIndex(): the index in the node list of the layer, or -1.
func (n *LNode) Index() int {
	if n.layer == nil {
		return -1
	}
	for i, x := range n.layer.Nodes {
		if x == n {
			return i
		}
	}
	return -1
}

// BorderToContentAreaCoordinates is borderToContentAreaCoordinates.
func (n *LNode) BorderToContentAreaCoordinates(horizontal, vertical bool) {
	g := n.Graph()
	if horizontal {
		n.Pos.X = n.Pos.X - g.Padding.Left - g.Offset.X
	}
	if vertical {
		n.Pos.Y = n.Pos.Y - g.Padding.Top - g.Offset.Y
	}
}

// IsInlineEdgeLabel is isInlineEdgeLabel().
func (n *LNode) IsInlineEdgeLabel() bool {
	if n.Type != NodeTypeLabel {
		return false
	}
	for _, label := range prop.Get(n, representedLabels) {
		if !prop.Get(label, edgeLabelsInline) {
			return false
		}
	}
	return true
}

// Designation is getDesignation().
func (n *LNode) Designation() string {
	if len(n.Labels) != 0 && n.Labels[0].Text != "" {
		return n.Labels[0].Text
	}
	if id, ok := n.designation(); ok {
		return id
	}
	return strconv.Itoa(n.Index())
}

// String is toString().
func (n *LNode) String() string {
	var b strings.Builder
	b.WriteString("n")
	if n.Type != NodeTypeNormal {
		b.WriteString("(" + strings.ToLower(n.Type.String()) + ")")
	}
	b.WriteString("_" + n.Designation())
	return b.String()
}

// LPort is the class LPort.
type LPort struct {
	Element
	// Pos is the position relative to the node (getPosition()).
	Pos emath.KVector
	// Size is the size (getSize()).
	Size emath.KVector
	// Side is the side of the node. Read it directly, and set it with
	// SetSide, which also moves the anchor.
	Side core.PortSide
	// Anchor is the point where edges attach, relative to the position.
	Anchor emath.KVector
	// ExplicitlySuppliedPortAnchor tells that SetSide keeps the anchor.
	ExplicitlySuppliedPortAnchor bool
	// ConnectedToExternalNodes is the flag of the same name. It starts true.
	ConnectedToExternalNodes bool
	// Labels has the labels.
	Labels []*LLabel
	// IncomingEdges and OutgoingEdges have the edges in order.
	IncomingEdges []*LEdge
	OutgoingEdges []*LEdge

	owner *LNode
}

// NewLPort is new LPort().
func NewLPort() *LPort { return &LPort{ConnectedToExternalNodes: true} }

// Node is getNode().
func (p *LPort) Node() *LNode { return p.owner }

// SetNode is setNode(node): it moves the port to the end of the port list of
// the node. A nil node removes the port from its node.
func (p *LPort) SetNode(node *LNode) {
	if p.owner != nil {
		for i, x := range p.owner.Ports {
			if x == p {
				p.owner.Ports = append(p.owner.Ports[:i], p.owner.Ports[i+1:]...)
				break
			}
		}
	}
	p.owner = node
	if p.owner != nil {
		p.owner.Ports = append(p.owner.Ports, p)
	}
}

// SetNodeField sets the owner without a change of a port list. Only code
// that rebuilds a graph from a stored state uses it.
func (p *LPort) SetNodeField(node *LNode) { p.owner = node }

// SetSide is setSide(side).
func (p *LPort) SetSide(side core.PortSide) {
	p.Side = side
	if !p.ExplicitlySuppliedPortAnchor {
		switch side {
		case core.PortSideNorth:
			p.Anchor.X = p.Size.X / 2
			p.Anchor.Y = 0
		case core.PortSideEast:
			p.Anchor.X = p.Size.X
			p.Anchor.Y = p.Size.Y / 2
		case core.PortSideSouth:
			p.Anchor.X = p.Size.X / 2
			p.Anchor.Y = p.Size.Y
		case core.PortSideWest:
			p.Anchor.X = 0
			p.Anchor.Y = p.Size.Y / 2
		}
	}
}

// AbsoluteAnchor is getAbsoluteAnchor().
func (p *LPort) AbsoluteAnchor() *emath.KVector {
	return emath.Sum(&p.owner.Pos, &p.Pos, &p.Anchor)
}

// Name is getName(): the text of the first label. ok is false without a
// label.
func (p *LPort) Name() (name string, ok bool) {
	if len(p.Labels) != 0 {
		return p.Labels[0].Text, true
	}
	return "", false
}

// Degree is getDegree().
func (p *LPort) Degree() int { return len(p.IncomingEdges) + len(p.OutgoingEdges) }

// NetFlow is getNetFlow().
func (p *LPort) NetFlow() int { return len(p.IncomingEdges) - len(p.OutgoingEdges) }

// ConnectedEdges is getConnectedEdges(): the incoming edges, then the
// outgoing edges.
func (p *LPort) ConnectedEdges() []*LEdge {
	out := make([]*LEdge, 0, len(p.IncomingEdges)+len(p.OutgoingEdges))
	out = append(out, p.IncomingEdges...)
	return append(out, p.OutgoingEdges...)
}

// PredecessorPorts is getPredecessorPorts(): the source of each incoming
// edge.
func (p *LPort) PredecessorPorts() []*LPort {
	out := make([]*LPort, len(p.IncomingEdges))
	for i, e := range p.IncomingEdges {
		out[i] = e.Source
	}
	return out
}

// SuccessorPorts is getSuccessorPorts(): the target of each outgoing edge.
func (p *LPort) SuccessorPorts() []*LPort {
	out := make([]*LPort, len(p.OutgoingEdges))
	for i, e := range p.OutgoingEdges {
		out[i] = e.Target
	}
	return out
}

// ConnectedPorts is getConnectedPorts(): the predecessors, then the
// successors.
func (p *LPort) ConnectedPorts() []*LPort {
	return append(p.PredecessorPorts(), p.SuccessorPorts()...)
}

// Index is getIndex(): the index in the port list of the node, or -1.
func (p *LPort) Index() int {
	if p.owner == nil {
		return -1
	}
	for i, x := range p.owner.Ports {
		if x == p {
			return i
		}
	}
	return -1
}

// Designation is getDesignation().
func (p *LPort) Designation() string {
	if len(p.Labels) != 0 && p.Labels[0].Text != "" {
		return p.Labels[0].Text
	}
	if id, ok := p.designation(); ok {
		return id
	}
	return strconv.Itoa(p.Index())
}

// String is toString().
func (p *LPort) String() string {
	var b strings.Builder
	b.WriteString("p_" + p.Designation())
	if p.owner != nil {
		b.WriteString("[" + p.owner.String() + "]")
	}
	if len(p.IncomingEdges) == 1 && len(p.OutgoingEdges) == 0 && p.IncomingEdges[0].Source != p {
		source := p.IncomingEdges[0].Source
		b.WriteString(" << " + source.Designation())
		b.WriteString("[" + source.owner.String() + "]")
	}
	if len(p.IncomingEdges) == 0 && len(p.OutgoingEdges) == 1 && p.OutgoingEdges[0].Target != p {
		target := p.OutgoingEdges[0].Target
		b.WriteString(" >> " + target.Designation())
		b.WriteString("[" + target.owner.String() + "]")
	}
	return b.String()
}

// LEdge is the class LEdge.
type LEdge struct {
	Element
	// BendPoints has the bend points. Reverse replaces the chain.
	BendPoints *emath.KVectorChain
	// Labels has the labels.
	Labels []*LLabel
	// Source and Target are the ports of the edge. Read them directly, and
	// set them with SetSource and SetTarget.
	Source *LPort
	Target *LPort
}

// NewLEdge is new LEdge().
func NewLEdge() *LEdge { return &LEdge{BendPoints: emath.NewKVectorChain()} }

func removeEdge(list []*LEdge, e *LEdge) []*LEdge {
	for i, x := range list {
		if x == e {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

// SetSource is setSource(port): it moves the edge to the end of the outgoing
// edges of the port.
func (e *LEdge) SetSource(source *LPort) {
	if e.Source != nil {
		e.Source.OutgoingEdges = removeEdge(e.Source.OutgoingEdges, e)
	}
	e.Source = source
	if e.Source != nil {
		e.Source.OutgoingEdges = append(e.Source.OutgoingEdges, e)
	}
}

// SetTarget is setTarget(port): it moves the edge to the end of the incoming
// edges of the port.
func (e *LEdge) SetTarget(target *LPort) {
	if e.Target != nil {
		e.Target.IncomingEdges = removeEdge(e.Target.IncomingEdges, e)
	}
	e.Target = target
	if e.Target != nil {
		e.Target.IncomingEdges = append(e.Target.IncomingEdges, e)
	}
}

// SetTargetAndInsertAtIndex is setTargetAndInsertAtIndex(port, index).
func (e *LEdge) SetTargetAndInsertAtIndex(target *LPort, index int) {
	if e.Target != nil {
		e.Target.IncomingEdges = removeEdge(e.Target.IncomingEdges, e)
	}
	e.Target = target
	if e.Target != nil {
		in := append(e.Target.IncomingEdges, nil)
		copy(in[index+1:], in[index:])
		in[index] = e
		e.Target.IncomingEdges = in
	}
}

// Reverse is reverse(layeredGraph, adaptPorts).
func (e *LEdge) Reverse(layeredGraph *LGraph, adaptPorts bool) {
	oldSource := e.Source
	oldTarget := e.Target
	e.SetSource(nil)
	e.SetTarget(nil)
	if adaptPorts && prop.Get(oldTarget, inputCollect) {
		e.SetSource(ProvideCollectorPort(layeredGraph, oldTarget.Node(), lenum.PortTypeOutput, core.PortSideEast))
	} else {
		e.SetSource(oldTarget)
	}
	if adaptPorts && prop.Get(oldSource, outputCollect) {
		e.SetTarget(ProvideCollectorPort(layeredGraph, oldSource.Node(), lenum.PortTypeInput, core.PortSideWest))
	} else {
		e.SetTarget(oldSource)
	}
	for _, label := range e.Labels {
		placement := prop.Get(label, edgeLabelsPlacement)
		if placement == core.EdgeLabelPlacementTail {
			prop.Set(label, edgeLabelsPlacement, core.EdgeLabelPlacementHead)
		} else if placement == core.EdgeLabelPlacementHead {
			prop.Set(label, edgeLabelsPlacement, core.EdgeLabelPlacementTail)
		}
	}
	prop.Set(e, reversed, !prop.Get(e, reversed))
	e.BendPoints = emath.ReverseChain(e.BendPoints)
}

// IsSelfLoop is isSelfLoop().
func (e *LEdge) IsSelfLoop() bool {
	if e.Source == nil || e.Target == nil {
		return false
	}
	return e.Source.Node() != nil && e.Source.Node() == e.Target.Node()
}

// IsInLayerEdge is isInLayerEdge().
func (e *LEdge) IsInLayerEdge() bool {
	return !e.IsSelfLoop() && e.Source.Node().Layer() == e.Target.Node().Layer()
}

// OtherPort is getOther(LPort). The port must be an end of the edge.
func (e *LEdge) OtherPort(port *LPort) *LPort {
	if port == e.Source {
		return e.Target
	} else if port == e.Target {
		return e.Source
	}
	panic("lgraph: 'port' must be either the source port or target port of the edge.")
}

// OtherNode is getOther(LNode). The node must be an end of the edge.
func (e *LEdge) OtherNode(node *LNode) *LNode {
	if node == e.Source.Node() {
		return e.Target.Node()
	} else if node == e.Target.Node() {
		return e.Source.Node()
	}
	panic("lgraph: 'node' must either be the source node or target node of the edge.")
}

// Designation is getDesignation(). ok is false where Java returns null.
func (e *LEdge) Designation() (string, bool) {
	if len(e.Labels) != 0 && e.Labels[0].Text != "" {
		return e.Labels[0].Text, true
	}
	return e.designation()
}

// String is toString().
func (e *LEdge) String() string {
	var b strings.Builder
	b.WriteString("e_")
	if d, ok := e.Designation(); ok {
		b.WriteString(d)
	}
	if e.Source != nil && e.Target != nil {
		b.WriteString(" " + e.Source.Designation())
		b.WriteString("[" + e.Source.Node().String() + "]")
		b.WriteString(" -> " + e.Target.Designation())
		b.WriteString("[" + e.Target.Node().String() + "]")
	}
	return b.String()
}

// LLabel is the class LLabel.
type LLabel struct {
	Element
	// Pos is the position (getPosition()).
	Pos emath.KVector
	// Size is the size (getSize()).
	Size emath.KVector
	// Text is the text. HasText is false for a label whose Java text is null.
	Text    string
	HasText bool
}

// NewLLabel is new LLabel(text).
func NewLLabel(text string) *LLabel { return &LLabel{Text: text, HasText: true} }

// Designation is getDesignation(). ok is false where Java returns null.
func (l *LLabel) Designation() (string, bool) {
	if l.Text != "" {
		return l.Text, true
	}
	return l.designation()
}

// String is toString().
func (l *LLabel) String() string {
	if d, ok := l.Designation(); ok {
		return "l_" + d
	}
	return "label"
}
