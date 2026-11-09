// Ported from plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/util/adapters/ElkGraphAdapters.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package elkadapters adapts the ELK graph to the interfaces of package
// adapters, which the node spacing code reads. The importer of the layered
// algorithm uses it for the size and the padding of a node with labels.
package elkadapters

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/coreopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/elkgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// labelSide is LabelSide.LABEL_SIDE.
var labelSide = prop.NewDefault("org.eclipse.elk.labelSide", core.LabelSideUnknown)

// Adapt is ElkGraphAdapters.adapt(graph).
func Adapt(graph *elkgraph.Node) *GraphAdapter {
	return &GraphAdapter{element: graph}
}

// AdaptSingleNode is ElkGraphAdapters.adaptSingleNode(node). The graph of
// the adapter is nil for a node without a parent.
func AdaptSingleNode(node *elkgraph.Node) *NodeAdapter {
	if node.Parent == nil {
		return &NodeAdapter{element: node}
	}
	return &NodeAdapter{parent: Adapt(node.Parent), element: node}
}

// GraphAdapter is ElkGraphAdapter.
type GraphAdapter struct {
	element    *elkgraph.Node
	id         int
	childNodes []adapters.NodeAdapter
}

// Size is getSize().
func (a *GraphAdapter) Size() *emath.KVector {
	return emath.NewKVector(a.element.Width, a.element.Height)
}

// SetSize is setSize(size).
func (a *GraphAdapter) SetSize(size *emath.KVector) {
	a.element.Width, a.element.Height = size.X, size.Y
}

// Position is getPosition().
func (a *GraphAdapter) Position() *emath.KVector { return emath.NewKVector(a.element.X, a.element.Y) }

// SetPosition is setPosition(pos).
func (a *GraphAdapter) SetPosition(pos *emath.KVector) { a.element.X, a.element.Y = pos.X, pos.Y }

// Holder returns the node, which has the properties.
func (a *GraphAdapter) Holder() prop.Holder { return a.element }

// VolatileID is getVolatileId().
func (a *GraphAdapter) VolatileID() int { return a.id }

// SetVolatileID is setVolatileId(id).
func (a *GraphAdapter) SetVolatileID(id int) { a.id = id }

// Nodes is getNodes().
func (a *GraphAdapter) Nodes() []adapters.NodeAdapter {
	if a.childNodes == nil {
		a.childNodes = make([]adapters.NodeAdapter, 0, len(a.element.Children))
		for _, n := range a.element.Children {
			a.childNodes = append(a.childNodes, &NodeAdapter{parent: a, element: n})
		}
	}
	return a.childNodes
}

// NodeAdapter is ElkNodeAdapter.
type NodeAdapter struct {
	parent   *GraphAdapter
	element  *elkgraph.Node
	id       int
	labels   []adapters.LabelAdapter
	in, out  []adapters.EdgeAdapter
	hasIn    bool
	hasOut   bool
	ports    []adapters.PortAdapter
	hasPorts bool
}

// Size is getSize().
func (a *NodeAdapter) Size() *emath.KVector {
	return emath.NewKVector(a.element.Width, a.element.Height)
}

// SetSize is setSize(size).
func (a *NodeAdapter) SetSize(size *emath.KVector) {
	a.element.Width, a.element.Height = size.X, size.Y
}

// Position is getPosition().
func (a *NodeAdapter) Position() *emath.KVector { return emath.NewKVector(a.element.X, a.element.Y) }

// SetPosition is setPosition(pos).
func (a *NodeAdapter) SetPosition(pos *emath.KVector) { a.element.X, a.element.Y = pos.X, pos.Y }

// Holder returns the node, which has the properties.
func (a *NodeAdapter) Holder() prop.Holder { return a.element }

// VolatileID is getVolatileId().
func (a *NodeAdapter) VolatileID() int { return a.id }

// SetVolatileID is setVolatileId(id).
func (a *NodeAdapter) SetVolatileID(id int) { a.id = id }

// Graph is getGraph(): the adapter of the parent node, or nil.
func (a *NodeAdapter) Graph() adapters.GraphAdapter {
	if a.parent == nil {
		return nil
	}
	return a.parent
}

// Labels is getLabels().
func (a *NodeAdapter) Labels() []adapters.LabelAdapter {
	if a.labels == nil {
		a.labels = make([]adapters.LabelAdapter, 0, len(a.element.Labels))
		for _, l := range a.element.Labels {
			a.labels = append(a.labels, &LabelAdapter{element: l})
		}
	}
	return a.labels
}

// Ports is getPorts(). The list is made one time: a later sort of the port
// list of the node does not change it, as in Java.
func (a *NodeAdapter) Ports() []adapters.PortAdapter {
	if !a.hasPorts {
		a.ports = make([]adapters.PortAdapter, 0, len(a.element.Ports))
		for _, p := range a.element.Ports {
			a.ports = append(a.ports, &PortAdapter{element: p})
		}
		a.hasPorts = true
	}
	return a.ports
}

func edgeAdapters(edges []*elkgraph.Edge) []adapters.EdgeAdapter {
	out := make([]adapters.EdgeAdapter, 0, len(edges))
	for _, e := range edges {
		out = append(out, &EdgeAdapter{element: e})
	}
	return out
}

// IncomingEdges is getIncomingEdges(): the incoming edges of the node, then
// of each port.
func (a *NodeAdapter) IncomingEdges() []adapters.EdgeAdapter {
	if !a.hasIn {
		edges := append([]*elkgraph.Edge(nil), a.element.IncomingEdges...)
		for _, p := range a.element.Ports {
			edges = append(edges, p.IncomingEdges...)
		}
		a.in, a.hasIn = edgeAdapters(edges), true
	}
	return a.in
}

// OutgoingEdges is getOutgoingEdges().
func (a *NodeAdapter) OutgoingEdges() []adapters.EdgeAdapter {
	if !a.hasOut {
		a.out, a.hasOut = edgeAdapters(elkgraph.AllOutgoingEdges(a.element)), true
	}
	return a.out
}

// SortPortList is sortPortList(): it sorts the port list of the node with
// the default comparator when the order of the ports is fixed.
func (a *NodeAdapter) SortPortList() {
	a.SortPortListWith(func(port1, port2 *elkgraph.Port) int { return comparePorts(port1, port2) })
}

// SortPortListWith is sortPortList(comparator). The comparator must be a
// func(a, b *elkgraph.Port) int.
func (a *NodeAdapter) SortPortListWith(comparator any) {
	if prop.Get(a.element, coreopts.PortConstraints).IsOrderFixed() {
		cmp, ok := comparator.(func(a, b *elkgraph.Port) int)
		if !ok {
			lgraph.Bail("ElkGraphAdapters: a port comparator of the type %T", comparator)
		}
		gwt.SortFunc(a.element.Ports, cmp)
	}
}

// comparePorts is ElkGraphAdapters.PortComparator.compare.
func comparePorts(port1, port2 *elkgraph.Port) int {
	side1 := prop.Get(port1, coreopts.PortSide)
	ordinalDifference := int(side1) - int(prop.Get(port2, coreopts.PortSide))

	// if the ports are on different sides, sort by side
	if ordinalDifference != 0 {
		return ordinalDifference
	}

	// if the port index is set, compare by index
	index1, has1 := prop.Lookup(port1, coreopts.PortIndex)
	index2, has2 := prop.Lookup(port2, coreopts.PortIndex)
	if has1 && has2 {
		if indexDifference := index1 - index2; indexDifference != 0 {
			return indexDifference
		}
	}

	// if the ports have no index, compare by position
	switch side1 {
	case core.PortSideNorth:
		// clockwise, so increasing x
		return gwt.DoubleCompare(port1.X, port2.X)
	case core.PortSideEast:
		// clockwise, so increasing y
		return gwt.DoubleCompare(port1.Y, port2.Y)
	case core.PortSideSouth:
		// clockwise, so decreasing x
		return gwt.DoubleCompare(port2.X, port1.X)
	case core.PortSideWest:
		// clockwise, so decreasing y
		return gwt.DoubleCompare(port2.Y, port1.Y)
	}
	lgraph.Bail("Port side is undefined")
	return 0
}

// PortAdapter is ElkPortAdapter.
type PortAdapter struct {
	element *elkgraph.Port
	id      int
	labels  []adapters.LabelAdapter
	in, out []adapters.EdgeAdapter
	hasIn   bool
	hasOut  bool
}

// Size is getSize().
func (a *PortAdapter) Size() *emath.KVector {
	return emath.NewKVector(a.element.Width, a.element.Height)
}

// SetSize is setSize(size).
func (a *PortAdapter) SetSize(size *emath.KVector) {
	a.element.Width, a.element.Height = size.X, size.Y
}

// Position is getPosition().
func (a *PortAdapter) Position() *emath.KVector { return emath.NewKVector(a.element.X, a.element.Y) }

// SetPosition is setPosition(pos).
func (a *PortAdapter) SetPosition(pos *emath.KVector) { a.element.X, a.element.Y = pos.X, pos.Y }

// Holder returns the port, which has the properties.
func (a *PortAdapter) Holder() prop.Holder { return a.element }

// VolatileID is getVolatileId().
func (a *PortAdapter) VolatileID() int { return a.id }

// SetVolatileID is setVolatileId(id).
func (a *PortAdapter) SetVolatileID(id int) { a.id = id }

// Side is getSide().
func (a *PortAdapter) Side() core.PortSide { return prop.Get(a.element, coreopts.PortSide) }

// Labels is getLabels().
func (a *PortAdapter) Labels() []adapters.LabelAdapter {
	if a.labels == nil {
		a.labels = make([]adapters.LabelAdapter, 0, len(a.element.Labels))
		for _, l := range a.element.Labels {
			a.labels = append(a.labels, &LabelAdapter{element: l})
		}
	}
	return a.labels
}

// Margin is getMargin().
func (a *PortAdapter) Margin() *emath.ElkMargin {
	if margins := prop.Get(a.element, coreopts.Margins); margins != nil {
		return margins
	}
	return &emath.ElkMargin{}
}

// SetMargin is setMargin(margin).
func (a *PortAdapter) SetMargin(margin *emath.ElkMargin) {
	prop.Set(a.element, coreopts.Margins, margin.Clone())
}

// IncomingEdges is getIncomingEdges().
func (a *PortAdapter) IncomingEdges() []adapters.EdgeAdapter {
	if !a.hasIn {
		a.in, a.hasIn = edgeAdapters(a.element.IncomingEdges), true
	}
	return a.in
}

// OutgoingEdges is getOutgoingEdges().
func (a *PortAdapter) OutgoingEdges() []adapters.EdgeAdapter {
	if !a.hasOut {
		a.out, a.hasOut = edgeAdapters(a.element.OutgoingEdges), true
	}
	return a.out
}

// HasCompoundConnections is hasCompoundConnections().
func (a *PortAdapter) HasCompoundConnections() bool {
	node := a.element.Parent
	for _, edge := range a.element.OutgoingEdges {
		for _, target := range edge.Targets {
			if elkgraph.IsDescendant(target.ShapeNode(), node) {
				return true
			} else if target.ShapeNode() == node && prop.Get(edge, coreopts.InsideSelfLoopsYo) {
				return true
			}
		}
	}
	for _, edge := range a.element.IncomingEdges {
		for _, source := range edge.Sources {
			if elkgraph.IsDescendant(source.ShapeNode(), node) {
				return true
			}
		}
	}
	return false
}

// IsCompoundNode is isCompoundNode().
func (a *NodeAdapter) IsCompoundNode() bool {
	return len(a.element.Children) != 0 || prop.Get(a.element, coreopts.InsideSelfLoopsActivate)
}

// Padding is getPadding(): a copy of the padding of the node.
func (a *NodeAdapter) Padding() *emath.ElkPadding {
	if padding := prop.Get(a.element, coreopts.Padding); padding != nil {
		return padding.Clone()
	}
	return &emath.ElkPadding{}
}

// SetPadding is setPadding(padding).
func (a *NodeAdapter) SetPadding(padding *emath.ElkPadding) {
	prop.Set(a.element, coreopts.Padding, padding.Clone())
}

// Margin is getMargin().
func (a *NodeAdapter) Margin() *emath.ElkMargin {
	if margins := prop.Get(a.element, coreopts.Margins); margins != nil {
		return margins
	}
	return &emath.ElkMargin{}
}

// SetMargin is setMargin(margin).
func (a *NodeAdapter) SetMargin(margin *emath.ElkMargin) {
	prop.Set(a.element, coreopts.Margins, margin.Clone())
}

// LabelAdapter is ElkLabelAdapter.
type LabelAdapter struct {
	element *elkgraph.Label
	id      int
}

// Size is getSize().
func (a *LabelAdapter) Size() *emath.KVector {
	return emath.NewKVector(a.element.Width, a.element.Height)
}

// SetSize is setSize(size).
func (a *LabelAdapter) SetSize(size *emath.KVector) {
	a.element.Width, a.element.Height = size.X, size.Y
}

// Position is getPosition().
func (a *LabelAdapter) Position() *emath.KVector { return emath.NewKVector(a.element.X, a.element.Y) }

// SetPosition is setPosition(pos).
func (a *LabelAdapter) SetPosition(pos *emath.KVector) { a.element.X, a.element.Y = pos.X, pos.Y }

// Holder returns the label, which has the properties.
func (a *LabelAdapter) Holder() prop.Holder { return a.element }

// VolatileID is getVolatileId().
func (a *LabelAdapter) VolatileID() int { return a.id }

// SetVolatileID is setVolatileId(id).
func (a *LabelAdapter) SetVolatileID(id int) { a.id = id }

// Side is getSide().
func (a *LabelAdapter) Side() core.LabelSide { return prop.Get(a.element, labelSide) }

// Text is getText(). ok is false for a label without a text.
func (a *LabelAdapter) Text() (string, bool) { return a.element.Text, a.element.HasText }

// EdgeAdapter is ElkEdgeAdapter.
type EdgeAdapter struct {
	element *elkgraph.Edge
	labels  []adapters.LabelAdapter
}

// Labels is getLabels().
func (a *EdgeAdapter) Labels() []adapters.LabelAdapter {
	if a.labels == nil {
		a.labels = make([]adapters.LabelAdapter, 0, len(a.element.Labels))
		for _, l := range a.element.Labels {
			a.labels = append(a.labels, &LabelAdapter{element: l})
		}
	}
	return a.labels
}
