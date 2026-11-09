// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/graph/LGraphAdapters.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package adapters

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// LabelSideProperty is LabelSide.LABEL_SIDE: the side that a layout
// algorithm chose for an edge label or a port label. It is not the internal
// property labelSide of the layered algorithm.
//
// TODO(core): move to the package core, next to the enum LabelSide.
var LabelSideProperty = prop.NewDefault("org.eclipse.elk.labelSide", core.LabelSideUnknown)

// SelfLoopHolder is the part of the class SelfLoopHolder of the self loop
// processors that the port adapter reads. The value of the internal
// property selfLoopHolder of a node must implement it.
//
// TODO(loops): the package that ports
// org.eclipse.elk.alg.layered.intermediate.loops implements this interface
// on its SelfLoopHolder.
type SelfLoopHolder interface {
	// SelfLoopEdgesOfPort returns the edges of the self loop port of the
	// port (getSLPortMap().get(port)): the LEdge of each of its incoming
	// self loop edges, and the LEdge of each of its outgoing self loop
	// edges, in order. ok is false when the holder has no self loop port
	// for the port.
	SelfLoopEdgesOfPort(port *lgraph.LPort) (incoming, outgoing []*lgraph.LEdge, ok bool)
}

// The adapters of the LGraph implement the adapter interfaces.
var (
	_ GraphAdapter = (*LGraphAdapter)(nil)
	_ NodeAdapter  = (*LNodeAdapter)(nil)
	_ PortAdapter  = (*LPortAdapter)(nil)
	_ LabelAdapter = (*LLabelAdapter)(nil)
	_ EdgeAdapter  = (*LEdgeAdapter)(nil)
)

// AdaptLGraph is LGraphAdapters.adapt(graph).
func AdaptLGraph(graph *lgraph.LGraph) *LGraphAdapter {
	return AdaptLGraphTransparent(graph, false)
}

// AdaptLGraphTransparent is LGraphAdapters.adapt(graph,
// transparentNorthSouthEdges). With transparentNorthSouthEdges the edges
// that are connected to north/south port dummies appear to be connected to
// the original north/south ports.
func AdaptLGraphTransparent(graph *lgraph.LGraph, transparentNorthSouthEdges bool) *LGraphAdapter {
	return &LGraphAdapter{
		element:                    graph,
		transparentNorthSouthEdges: transparentNorthSouthEdges,
		nodeFilter:                 func(*lgraph.LNode) bool { return true },
	}
}

// AdaptLGraphFiltered is LGraphAdapters.adapt(graph,
// transparentNorthSouthEdges, transparentCommentNodes, nodeFilter). With
// transparentCommentNodes the comment nodes that were removed from the
// graph and attached to a node are nodes of the adapted graph. The filter
// returns true for a node that the adapted graph shows. It must not be nil.
func AdaptLGraphFiltered(graph *lgraph.LGraph, transparentNorthSouthEdges, transparentCommentNodes bool,
	nodeFilter func(*lgraph.LNode) bool) *LGraphAdapter {

	return &LGraphAdapter{
		element:                    graph,
		transparentNorthSouthEdges: transparentNorthSouthEdges,
		transparentCommentNodes:    transparentCommentNodes,
		nodeFilter:                 nodeFilter,
	}
}

// AdaptLNode is LGraphAdapters.adapt(node, transparentNorthSouthEdges): the
// adapter of a single node. It has no graph.
func AdaptLNode(node *lgraph.LNode, transparentNorthSouthEdges bool) *LNodeAdapter {
	return newLNodeAdapter(nil, node, transparentNorthSouthEdges)
}

// AdaptLLabel is LGraphAdapters.adapt(label).
func AdaptLLabel(label *lgraph.LLabel) *LLabelAdapter {
	return &LLabelAdapter{element: label}
}

// LGraphAdapter is the class LGraphAdapters.LGraphAdapter.
type LGraphAdapter struct {
	element *lgraph.LGraph
	// nodeAdapters is the cached list of node adapters.
	nodeAdapters []NodeAdapter
	nodesCached  bool
	// transparentNorthSouthEdges tells to show the edges of north/south
	// port dummies at the north/south ports.
	transparentNorthSouthEdges bool
	// transparentCommentNodes tells to include the attached comment nodes
	// that are not part of the graph.
	transparentCommentNodes bool
	// nodeFilter decides which nodes the adapter shows.
	nodeFilter func(*lgraph.LNode) bool
}

// Element returns the adapted graph.
func (a *LGraphAdapter) Element() *lgraph.LGraph { return a.element }

// Size is getSize().
func (a *LGraphAdapter) Size() *emath.KVector { return &a.element.Size }

// SetSize is setSize(size).
func (a *LGraphAdapter) SetSize(size *emath.KVector) {
	a.element.Size.X = size.X
	a.element.Size.Y = size.Y
}

// Position is getPosition(). Java throws an UnsupportedOperationException.
func (a *LGraphAdapter) Position() *emath.KVector {
	lgraph.Bail("Not supported by LGraph")
	return nil
}

// SetPosition is setPosition(pos). Java throws an
// UnsupportedOperationException.
func (a *LGraphAdapter) SetPosition(pos *emath.KVector) {
	lgraph.Bail("Not supported by LGraph")
}

// Holder returns the graph as a property holder.
func (a *LGraphAdapter) Holder() prop.Holder { return a.element }

// Nodes is getNodes(): the adapters of the nodes of the layers that pass
// the filter. The layerless nodes are not part of the adapted graph.
func (a *LGraphAdapter) Nodes() []NodeAdapter {
	if !a.nodesCached {
		a.nodesCached = true
		a.nodeAdapters = nil
		// We completely ignore layerless nodes here since they are currently
		// not of interest to anyone using these adapters
		for _, l := range a.element.Layers {
			for _, n := range l.Nodes {
				if a.nodeFilter(n) {
					a.nodeAdapters = append(a.nodeAdapters, newLNodeAdapter(a, n, a.transparentNorthSouthEdges))
					if a.transparentCommentNodes {
						if prop.Has(n, iprops.TopComments) {
							for _, comment := range prop.Get(n, iprops.TopComments) {
								a.nodeAdapters = append(a.nodeAdapters, newLNodeAdapter(a, comment, false))
							}
						}
						if prop.Has(n, iprops.BottomComments) {
							for _, comment := range prop.Get(n, iprops.BottomComments) {
								a.nodeAdapters = append(a.nodeAdapters, newLNodeAdapter(a, comment, false))
							}
						}
					}
				}
			}
		}
	}
	return a.nodeAdapters
}

// VolatileID is getVolatileId().
func (a *LGraphAdapter) VolatileID() int { return a.element.ID }

// SetVolatileID is setVolatileId(volatileId).
func (a *LGraphAdapter) SetVolatileID(volatileID int) { a.element.ID = volatileID }

// LNodeAdapter is the class LGraphAdapters.LNodeAdapter.
type LNodeAdapter struct {
	element *lgraph.LNode
	// parentGraphAdapter is the graph adapter that made this adapter, or
	// nil.
	parentGraphAdapter *LGraphAdapter
	// labelAdapters is the cached list of label adapters.
	labelAdapters []LabelAdapter
	labelsCached  bool
	// portAdapters is the cached list of port adapters.
	portAdapters []PortAdapter
	portsCached  bool
	// transparentNorthSouthEdges tells to show the edges of north/south
	// port dummies at the north/south ports.
	transparentNorthSouthEdges bool
}

func newLNodeAdapter(parent *LGraphAdapter, element *lgraph.LNode, transparentNorthSouthEdges bool) *LNodeAdapter {
	return &LNodeAdapter{
		element:                    element,
		parentGraphAdapter:         parent,
		transparentNorthSouthEdges: transparentNorthSouthEdges,
	}
}

// Element returns the adapted node.
func (a *LNodeAdapter) Element() *lgraph.LNode { return a.element }

// Size is getSize().
func (a *LNodeAdapter) Size() *emath.KVector { return &a.element.Size }

// SetSize is setSize(size).
func (a *LNodeAdapter) SetSize(size *emath.KVector) {
	a.element.Size.X = size.X
	a.element.Size.Y = size.Y
}

// Position is getPosition().
func (a *LNodeAdapter) Position() *emath.KVector { return &a.element.Pos }

// SetPosition is setPosition(pos).
func (a *LNodeAdapter) SetPosition(pos *emath.KVector) {
	a.element.Pos.X = pos.X
	a.element.Pos.Y = pos.Y
}

// Holder returns the node as a property holder.
func (a *LNodeAdapter) Holder() prop.Holder { return a.element }

// VolatileID is getVolatileId().
func (a *LNodeAdapter) VolatileID() int { return a.element.ID }

// SetVolatileID is setVolatileId(volatileId).
func (a *LNodeAdapter) SetVolatileID(volatileID int) { a.element.ID = volatileID }

// Graph is getGraph(): the graph adapter that made this adapter, or nil.
func (a *LNodeAdapter) Graph() GraphAdapter {
	if a.parentGraphAdapter == nil {
		return nil
	}
	return a.parentGraphAdapter
}

// Labels is getLabels().
func (a *LNodeAdapter) Labels() []LabelAdapter {
	if !a.labelsCached {
		a.labelsCached = true
		a.labelAdapters = make([]LabelAdapter, 0, len(a.element.Labels))
		for _, l := range a.element.Labels {
			a.labelAdapters = append(a.labelAdapters, &LLabelAdapter{element: l})
		}
	}
	return a.labelAdapters
}

// Ports is getPorts().
func (a *LNodeAdapter) Ports() []PortAdapter {
	if !a.portsCached {
		a.portsCached = true
		a.portAdapters = make([]PortAdapter, 0, len(a.element.Ports))
		for _, p := range a.element.Ports {
			a.portAdapters = append(a.portAdapters, &LPortAdapter{
				element:                    p,
				transparentNorthSouthEdges: a.transparentNorthSouthEdges,
			})
		}
	}
	return a.portAdapters
}

// IncomingEdges is getIncomingEdges(). A node of the LGraph has no edges
// that are connected to the node itself.
func (a *LNodeAdapter) IncomingEdges() []EdgeAdapter { return nil }

// OutgoingEdges is getOutgoingEdges(). A node of the LGraph has no edges
// that are connected to the node itself.
func (a *LNodeAdapter) OutgoingEdges() []EdgeAdapter { return nil }

// SortPortList is sortPortList().
func (a *LNodeAdapter) SortPortList() {
	a.SortPortListWith(DefaultPortListSorter)
}

// SortPortListWith is sortPortList(comparator). The comparator must be a
// func(port1, port2 *lgraph.LPort) int.
func (a *LNodeAdapter) SortPortListWith(comparator any) {
	if prop.Get(a.element, lopts.PortConstraints).IsOrderFixed() {
		// We need to sort the port list accordingly
		cmp, ok := comparator.(func(port1, port2 *lgraph.LPort) int)
		if !ok {
			lgraph.Bail("LGraphAdapters: the port comparator has the type %T", comparator)
		}
		gwt.SortFunc(a.element.Ports, cmp)
	}
}

// IsCompoundNode is isCompoundNode().
func (a *LNodeAdapter) IsCompoundNode() bool {
	return prop.Get(a.element, iprops.CompoundNode)
}

// Padding is getPadding(): a copy of the padding of the node.
func (a *LNodeAdapter) Padding() *emath.ElkPadding {
	lPadding := &a.element.Padding
	return emath.NewElkPadding(lPadding.Top, lPadding.Right, lPadding.Bottom, lPadding.Left)
}

// SetPadding is setPadding(padding).
func (a *LNodeAdapter) SetPadding(padding *emath.ElkPadding) {
	a.element.Padding.Left = padding.Left
	a.element.Padding.Top = padding.Top
	a.element.Padding.Right = padding.Right
	a.element.Padding.Bottom = padding.Bottom
}

// Margin is getMargin(): a copy of the margin of the node.
func (a *LNodeAdapter) Margin() *emath.ElkMargin {
	lmargins := &a.element.Margin
	return emath.NewElkMargin(lmargins.Top, lmargins.Right, lmargins.Bottom, lmargins.Left)
}

// SetMargin is setMargin(margin).
func (a *LNodeAdapter) SetMargin(margin *emath.ElkMargin) {
	a.element.Margin.Left = margin.Left
	a.element.Margin.Top = margin.Top
	a.element.Margin.Right = margin.Right
	a.element.Margin.Bottom = margin.Bottom
}

// LPortAdapter is the class LGraphAdapters.LPortAdapter.
type LPortAdapter struct {
	element *lgraph.LPort
	// labelAdapters is the cached list of label adapters.
	labelAdapters []LabelAdapter
	labelsCached  bool
	// incomingEdgeAdapters is the cached list of adapters of the incoming
	// edges.
	incomingEdgeAdapters []EdgeAdapter
	incomingCached       bool
	// outgoingEdgeAdapters is the cached list of adapters of the outgoing
	// edges.
	outgoingEdgeAdapters []EdgeAdapter
	outgoingCached       bool
	// transparentNorthSouthEdges tells to show the edges of north/south
	// port dummies at the north/south ports.
	transparentNorthSouthEdges bool
}

// Element returns the adapted port.
func (a *LPortAdapter) Element() *lgraph.LPort { return a.element }

// Size is getSize().
func (a *LPortAdapter) Size() *emath.KVector { return &a.element.Size }

// SetSize is setSize(size).
func (a *LPortAdapter) SetSize(size *emath.KVector) {
	a.element.Size.X = size.X
	a.element.Size.Y = size.Y
}

// Position is getPosition().
func (a *LPortAdapter) Position() *emath.KVector { return &a.element.Pos }

// SetPosition is setPosition(pos).
func (a *LPortAdapter) SetPosition(pos *emath.KVector) {
	a.element.Pos.X = pos.X
	a.element.Pos.Y = pos.Y
}

// Holder returns the port as a property holder.
func (a *LPortAdapter) Holder() prop.Holder { return a.element }

// VolatileID is getVolatileId().
func (a *LPortAdapter) VolatileID() int { return a.element.ID }

// SetVolatileID is setVolatileId(volatileId).
func (a *LPortAdapter) SetVolatileID(volatileID int) { a.element.ID = volatileID }

// Side is getSide().
func (a *LPortAdapter) Side() core.PortSide { return a.element.Side }

// Labels is getLabels().
func (a *LPortAdapter) Labels() []LabelAdapter {
	if !a.labelsCached {
		a.labelsCached = true
		a.labelAdapters = make([]LabelAdapter, 0, len(a.element.Labels))
		for _, l := range a.element.Labels {
			a.labelAdapters = append(a.labelAdapters, &LLabelAdapter{element: l})
		}
	}
	return a.labelAdapters
}

// Margin is getMargin(). The LPort of this port has no margin field: no
// code of ELK layered sets the margin of a port, so it is zero.
//
// TODO(core): LPort.Margin (LPort.getMargin of Java).
func (a *LPortAdapter) Margin() *emath.ElkMargin {
	return &emath.ElkMargin{}
}

// SetMargin is setMargin(margin). The LPort of this port has no margin
// field, and no ported code sets the margin of a port, so this stops the
// layout.
func (a *LPortAdapter) SetMargin(margin *emath.ElkMargin) {
	lgraph.Bail("LGraphAdapters: the margin of an LPort is not ported")
}

// selfLoopEdges returns the edges of the self loop port of the port, from
// the self loop holder of its node, if the node has one.
func (a *LPortAdapter) selfLoopEdges() (incoming, outgoing []*lgraph.LEdge) {
	node := a.element.Node()
	if !prop.Has(node, iprops.SelfLoopHolder) {
		return nil, nil
	}
	slh, ok := prop.Get(node, iprops.SelfLoopHolder).(SelfLoopHolder)
	if !ok {
		lgraph.Bail("LGraphAdapters: the self loop holder of the type %T is not ported", prop.Get(node, iprops.SelfLoopHolder))
	}
	incoming, outgoing, _ = slh.SelfLoopEdgesOfPort(a.element)
	return incoming, outgoing
}

// IncomingEdges is getIncomingEdges().
func (a *LPortAdapter) IncomingEdges() []EdgeAdapter {
	if a.transparentNorthSouthEdges && a.element.Node().Type == lgraph.NodeTypeNorthSouthPort {
		return nil
	} else if !a.incomingCached {
		a.incomingCached = true
		a.incomingEdgeAdapters = nil
		for _, e := range a.element.IncomingEdges {
			a.incomingEdgeAdapters = append(a.incomingEdgeAdapters, &LEdgeAdapter{element: e})
		}

		if a.transparentNorthSouthEdges {
			portDummy := prop.Get(a.element, iprops.PortDummy)
			if portDummy != nil {
				for _, e := range portDummy.IncomingEdges() {
					a.incomingEdgeAdapters = append(a.incomingEdgeAdapters, &LEdgeAdapter{element: e})
				}
			}
		}

		// Add the incoming edge from the self loop holder if asked for one.
		incoming, _ := a.selfLoopEdges()
		for _, e := range incoming {
			a.incomingEdgeAdapters = append(a.incomingEdgeAdapters, &LEdgeAdapter{element: e})
		}
	}
	return a.incomingEdgeAdapters
}

// OutgoingEdges is getOutgoingEdges().
func (a *LPortAdapter) OutgoingEdges() []EdgeAdapter {
	if a.transparentNorthSouthEdges && a.element.Node().Type == lgraph.NodeTypeNorthSouthPort {
		return nil
	} else if !a.outgoingCached {
		a.outgoingCached = true
		a.outgoingEdgeAdapters = nil
		for _, e := range a.element.OutgoingEdges {
			a.outgoingEdgeAdapters = append(a.outgoingEdgeAdapters, &LEdgeAdapter{element: e})
		}

		if a.transparentNorthSouthEdges {
			portDummy := prop.Get(a.element, iprops.PortDummy)
			if portDummy != nil {
				for _, e := range portDummy.OutgoingEdges() {
					a.outgoingEdgeAdapters = append(a.outgoingEdgeAdapters, &LEdgeAdapter{element: e})
				}
			}
		}

		// Add the outgoing edge from the self loop holder if asked for one.
		_, outgoing := a.selfLoopEdges()
		for _, e := range outgoing {
			a.outgoingEdgeAdapters = append(a.outgoingEdgeAdapters, &LEdgeAdapter{element: e})
		}
	}
	return a.outgoingEdgeAdapters
}

// HasCompoundConnections is hasCompoundConnections().
func (a *LPortAdapter) HasCompoundConnections() bool {
	return prop.Get(a.element, iprops.InsideConnections)
}

// LLabelAdapter is the class LGraphAdapters.LLabelAdapter.
type LLabelAdapter struct {
	element *lgraph.LLabel
}

// Element returns the adapted label.
func (a *LLabelAdapter) Element() *lgraph.LLabel { return a.element }

// Size is getSize().
func (a *LLabelAdapter) Size() *emath.KVector { return &a.element.Size }

// SetSize is setSize(size).
func (a *LLabelAdapter) SetSize(size *emath.KVector) {
	a.element.Size.X = size.X
	a.element.Size.Y = size.Y
}

// Position is getPosition().
func (a *LLabelAdapter) Position() *emath.KVector { return &a.element.Pos }

// SetPosition is setPosition(pos).
func (a *LLabelAdapter) SetPosition(pos *emath.KVector) {
	a.element.Pos.X = pos.X
	a.element.Pos.Y = pos.Y
}

// Holder returns the label as a property holder.
func (a *LLabelAdapter) Holder() prop.Holder { return a.element }

// VolatileID is getVolatileId().
func (a *LLabelAdapter) VolatileID() int { return a.element.ID }

// SetVolatileID is setVolatileId(volatileId).
func (a *LLabelAdapter) SetVolatileID(volatileID int) { a.element.ID = volatileID }

// Side is getSide().
func (a *LLabelAdapter) Side() core.LabelSide {
	return prop.Get(a.element, LabelSideProperty)
}

// Text is getText(). ok is false for a label whose Java text is null.
func (a *LLabelAdapter) Text() (text string, ok bool) {
	return a.element.Text, a.element.HasText
}

// LEdgeAdapter is the class LGraphAdapters.LEdgeAdapter.
type LEdgeAdapter struct {
	// element is the wrapped edge.
	element *lgraph.LEdge
	// labelAdapters is the cached list of label adapters.
	labelAdapters []LabelAdapter
	labelsCached  bool
}

// Element returns the adapted edge.
func (a *LEdgeAdapter) Element() *lgraph.LEdge { return a.element }

// Labels is getLabels().
func (a *LEdgeAdapter) Labels() []LabelAdapter {
	if !a.labelsCached {
		a.labelsCached = true
		a.labelAdapters = make([]LabelAdapter, 0, len(a.element.Labels))
		for _, l := range a.element.Labels {
			a.labelAdapters = append(a.labelAdapters, &LLabelAdapter{element: l})
		}
	}
	return a.labelAdapters
}

// DefaultPortListSorter is DEFAULT_PORTLIST_SORTER: the comparison of
// PortComparator. It sorts ports by side (north, east, south, west),
// clockwise from the top left corner, then by the port index, then by the
// position.
func DefaultPortListSorter(port1, port2 *lgraph.LPort) int {
	ordinalDifference := int(port1.Side) - int(port2.Side)

	// Sort by side first
	if ordinalDifference != 0 {
		return ordinalDifference
	}

	// In case of equal sides, sort by port index property
	index1, has1 := prop.Lookup(port1, lopts.PortIndex)
	index2, has2 := prop.Lookup(port2, lopts.PortIndex)
	if has1 && has2 {
		indexDifference := index1 - index2
		if indexDifference != 0 {
			return indexDifference
		}
	}

	// In case of equal index, sort by position
	switch port1.Side {
	case core.PortSideNorth:
		// Compare x coordinates
		return gwt.DoubleCompare(port1.Pos.X, port2.Pos.X)
	case core.PortSideEast:
		// Compare y coordinates
		return gwt.DoubleCompare(port1.Pos.Y, port2.Pos.Y)
	case core.PortSideSouth:
		// Compare x coordinates in reversed order
		return gwt.DoubleCompare(port2.Pos.X, port1.Pos.X)
	case core.PortSideWest:
		// Compare y coordinates in reversed order
		return gwt.DoubleCompare(port2.Pos.Y, port1.Pos.Y)
	default:
		// Port sides should not be undefined
		lgraph.Bail("Port side is undefined")
		return 0
	}
}
