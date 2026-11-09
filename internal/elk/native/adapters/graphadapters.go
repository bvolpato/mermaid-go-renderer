// Ported from plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/util/adapters/GraphAdapters.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package adapters has the graph adapters of ELK: the interfaces of
// org.eclipse.elk.core.util.adapters.GraphAdapters, which let the node size
// and node margin code work on any graph model, and LGraphAdapters, their
// implementation for the LGraph.
//
// The Java interfaces have a type parameter for the adapted element. No
// ported code reads it, so the Go interfaces have none.
package adapters

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// GraphElementAdapter is GraphAdapters.GraphElementAdapter: the base of the
// adapters.
type GraphElementAdapter interface {
	// Size is getSize(): the size vector of the element, or a copy of the
	// size. The caller must not change it: SetSize sets the size.
	Size() *emath.KVector
	// SetSize is setSize(size).
	SetSize(size *emath.KVector)
	// Position is getPosition(): the position vector of the element, or a
	// copy of the position. The caller must not change it: SetPosition sets
	// the position.
	Position() *emath.KVector
	// SetPosition is setPosition(pos).
	SetPosition(pos *emath.KVector)
	// Holder returns the property holder of the adapted element. A Go
	// interface cannot have the generic methods getProperty and hasProperty
	// of the Java interface: they are the functions GetProperty and
	// HasProperty, which read this holder.
	Holder() prop.Holder
	// VolatileID is getVolatileId().
	VolatileID() int
	// SetVolatileID is setVolatileId(volatileId).
	SetVolatileID(volatileID int)
}

// GetProperty is getProperty(prop) of an adapter: the value of the adapted
// element, or the default value of the property. As in Java, a cloneable
// default value is stored in the element first.
func GetProperty[T any](a GraphElementAdapter, p *prop.Property[T]) T {
	return prop.Get(a.Holder(), p)
}

// HasProperty is hasProperty(prop) of an adapter.
func HasProperty(a GraphElementAdapter, p prop.Any) bool {
	return prop.Has(a.Holder(), p)
}

// GraphAdapter is GraphAdapters.GraphAdapter.
type GraphAdapter interface {
	GraphElementAdapter
	// Nodes is getNodes(): the adapters of the nodes of the graph.
	Nodes() []NodeAdapter
}

// NodeAdapter is GraphAdapters.NodeAdapter.
type NodeAdapter interface {
	GraphElementAdapter
	// Graph is getGraph(): the graph of the node. A node without a graph
	// must return the nil interface, not a nil pointer of an adapter type.
	Graph() GraphAdapter
	// Labels is getLabels().
	Labels() []LabelAdapter
	// Ports is getPorts().
	Ports() []PortAdapter
	// IncomingEdges is getIncomingEdges(): the edges that end at the node
	// itself, not at one of its ports.
	IncomingEdges() []EdgeAdapter
	// OutgoingEdges is getOutgoingEdges().
	OutgoingEdges() []EdgeAdapter
	// SortPortList is sortPortList(): it sorts the ports by side, clockwise
	// from the top left corner.
	SortPortList()
	// SortPortListWith is sortPortList(comparator). The comparator compares
	// the ports of the adapted graph model, so its type depends on the
	// adapter.
	SortPortListWith(comparator any)
	// IsCompoundNode is isCompoundNode().
	IsCompoundNode() bool
	// Padding is getPadding(): a copy of the padding.
	Padding() *emath.ElkPadding
	// SetPadding is setPadding(padding).
	SetPadding(padding *emath.ElkPadding)
	// Margin is getMargin(): a copy of the margin.
	Margin() *emath.ElkMargin
	// SetMargin is setMargin(margin).
	SetMargin(margin *emath.ElkMargin)
}

// PortAdapter is GraphAdapters.PortAdapter.
type PortAdapter interface {
	GraphElementAdapter
	// Side is getSide().
	Side() core.PortSide
	// Labels is getLabels().
	Labels() []LabelAdapter
	// Margin is getMargin(): a copy of the margin.
	Margin() *emath.ElkMargin
	// SetMargin is setMargin(margin).
	SetMargin(margin *emath.ElkMargin)
	// IncomingEdges is getIncomingEdges().
	IncomingEdges() []EdgeAdapter
	// OutgoingEdges is getOutgoingEdges().
	OutgoingEdges() []EdgeAdapter
	// HasCompoundConnections is hasCompoundConnections(): the port has edges
	// to the inside of its node.
	HasCompoundConnections() bool
}

// LabelAdapter is GraphAdapters.LabelAdapter.
type LabelAdapter interface {
	GraphElementAdapter
	// Side is getSide().
	Side() core.LabelSide
	// Text is getText(). ok is false where Java returns null.
	Text() (text string, ok bool)
}

// EdgeAdapter is GraphAdapters.EdgeAdapter.
type EdgeAdapter interface {
	// Labels is getLabels().
	Labels() []LabelAdapter
}
