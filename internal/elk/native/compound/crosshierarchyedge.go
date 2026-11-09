// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/compound/CrossHierarchyEdge.java
// of Eclipse ELK 0.9.1 (EPL-2.0), and from the HashMultimap of the Guava
// version in elk-worker.js of elkjs 0.9.3.

// Package compound has the two processors that run around the layout of a
// compound graph: the preprocessor splits each edge that crosses a hierarchy
// border into one segment for each graph, and the postprocessor joins the
// segments again.
package compound

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// CrossHierarchyEdge is the class CrossHierarchyEdge: one segment of an edge
// that crosses a hierarchy border. The preprocessor passes the segments to
// the postprocessor in the property crossHierarchyMap of the top-level graph.
type CrossHierarchyEdge struct {
	// newEdge is the edge of the layered graph that the layout is computed
	// for.
	newEdge *lgraph.LEdge
	// graph is the layered graph in which the layout is computed.
	graph *lgraph.LGraph
	// portType is the flow direction: input or output.
	portType lenum.PortType
}

// NewCrossHierarchyEdge is new CrossHierarchyEdge(newEdge, graph, type).
func NewCrossHierarchyEdge(newEdge *lgraph.LEdge, graph *lgraph.LGraph, portType lenum.PortType) *CrossHierarchyEdge {
	return &CrossHierarchyEdge{newEdge: newEdge, graph: graph, portType: portType}
}

// String is toString().
func (c *CrossHierarchyEdge) String() string {
	return c.portType.String() + ":" + c.newEdge.String()
}

// Edge is getEdge(): the dummy edge of the segment.
func (c *CrossHierarchyEdge) Edge() *lgraph.LEdge { return c.newEdge }

// Graph is getGraph(): the graph of the dummy edge.
func (c *CrossHierarchyEdge) Graph() *lgraph.LGraph { return c.graph }

// Type is getType(). An input segment points to a deeper hierarchy level,
// and an output segment to a shallower one.
func (c *CrossHierarchyEdge) Type() lenum.PortType { return c.portType }

// originPort is (LPort) node.getProperty(ORIGIN). Java throws a
// ClassCastException for another origin, and elkjs does too.
func originPort(node *lgraph.LNode) *lgraph.LPort {
	origin, has := prop.Lookup(node, iprops.Origin)
	if !has {
		return nil
	}
	port, isPort := origin.(*lgraph.LPort)
	if !isPort {
		lgraph.Bail("CrossHierarchyEdge: the origin of an external port dummy is not a port")
	}
	return port
}

// ActualSource is getActualSource(): the source port of the dummy edge, or
// the port of the containing node when the source is an external port dummy.
func (c *CrossHierarchyEdge) ActualSource() *lgraph.LPort {
	if c.newEdge.Source.Node().Type == lgraph.NodeTypeExternalPort {
		return originPort(c.newEdge.Source.Node())
	}
	return c.newEdge.Source
}

// ActualTarget is getActualTarget(): the target port of the dummy edge, or
// the port of the containing node when the target is an external port dummy.
func (c *CrossHierarchyEdge) ActualTarget() *lgraph.LPort {
	if c.newEdge.Target.Node().Type == lgraph.NodeTypeExternalPort {
		return originPort(c.newEdge.Target.Node())
	}
	return c.newEdge.Target
}

// CrossHierarchyMap is the HashMultimap<LEdge, CrossHierarchyEdge> of the
// property crossHierarchyMap: for each original edge the segments that
// replace it. Neither class has its own hashCode, so in elkjs the keys and
// the values of a key iterate in the order of their insertion.
type CrossHierarchyMap struct {
	m *gwt.Map[*lgraph.LEdge, *gwt.Set[*CrossHierarchyEdge]]
}

// NewCrossHierarchyMap is HashMultimap.create().
func NewCrossHierarchyMap() *CrossHierarchyMap {
	return &CrossHierarchyMap{m: gwt.NewMap[*lgraph.LEdge, *gwt.Set[*CrossHierarchyEdge]]()}
}

// Put is put(key, value). It returns true when the map changed.
func (m *CrossHierarchyMap) Put(key *lgraph.LEdge, value *CrossHierarchyEdge) bool {
	collection := m.m.Get(key)
	if collection == nil {
		collection = gwt.NewSet[*CrossHierarchyEdge]()
		collection.Add(value)
		m.m.Put(key, collection)
		return true
	}
	return collection.Add(value)
}

// KeySet is keySet() as a slice in iteration order.
func (m *CrossHierarchyMap) KeySet() []*lgraph.LEdge { return m.m.Keys() }

// Get is get(key) as a new slice in iteration order.
func (m *CrossHierarchyMap) Get(key *lgraph.LEdge) []*CrossHierarchyEdge {
	collection := m.m.Get(key)
	if collection == nil {
		return nil
	}
	return collection.Values()
}
