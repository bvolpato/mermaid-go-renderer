// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/loops/SelfLoopHolder.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package loops has the data model of the self loop processors of the
// layered algorithm (org.eclipse.elk.alg.layered.intermediate.loops), and
// the two processors that need only the model: SelfLoopPreProcessor and
// SelfLoopPostProcessor. The packages loops/ordering and loops/routing have
// the processors SelfLoopPortRestorer and SelfLoopRouter with their classes.
//
// The fields of the model are exported, as the lists of the package lgraph
// are. Change them with the methods that have the Java names where a method
// exists.
package loops

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// SelfLoopHolder is the class SelfLoopHolder: all that the self loop
// processors know about the self loops of one node. A node with self loops
// has its holder as the value of the property selfLoopHolder.
type SelfLoopHolder struct {
	// LNode is the node of this instance (getLNode()).
	LNode *lgraph.LNode
	// SLHyperLoops has the hyper loops of the node (getSLHyperLoops()).
	SLHyperLoops []*SelfHyperLoop
	// SLPortMap maps each port of a self loop to its SelfLoopPort
	// (getSLPortMap()). It is a LinkedHashMap: it iterates in the order of
	// insertion, and the hyper loops are made in that order.
	SLPortMap *gwt.Map[*lgraph.LPort, *SelfLoopPort]

	// PortsHidden is the field arePortsHidden: one self loop port or more is
	// hidden from the node. Set it with SetPortsHidden.
	PortsHidden bool
	// PortsHiddenAssigned tells that SetPortsHidden was called. The object
	// of elkjs has its own field arePortsHidden only from the first
	// assignment, and the stored states of the phase goldens show that
	// field only then.
	PortsHiddenAssigned bool
	// RoutingSlotCount is the number of routing slots on each side of the
	// node, by the ordinal of the side (getRoutingSlotCount()).
	RoutingSlotCount [PortSideCount]int
}

////////////////////////////////////////////////////////////////////////////
// Creation

// NewSelfLoopHolder is the private constructor new SelfLoopHolder(node). It
// does not look at the edges of the node: Install does that.
func NewSelfLoopHolder(node *lgraph.LNode) *SelfLoopHolder {
	return &SelfLoopHolder{LNode: node, SLPortMap: gwt.NewMap[*lgraph.LPort, *SelfLoopPort]()}
}

// Install is SelfLoopHolder.install(lNode): it makes the holder of the node
// and sets the property selfLoopHolder of the node. Call it only for a node
// that NeedsSelfLoopProcessing.
func Install(lNode *lgraph.LNode) *SelfLoopHolder {
	holder := NewSelfLoopHolder(lNode)
	prop.Set[any](lNode, iprops.SelfLoopHolder, holder)

	holder.initialize()

	return holder
}

// NeedsSelfLoopProcessing is SelfLoopHolder.needsSelfLoopProcessing(lNode):
// the node is a regular node with one self loop or more.
func NeedsSelfLoopProcessing(lNode *lgraph.LNode) bool {
	if lNode.Type != lgraph.NodeTypeNormal {
		return false
	}

	for _, edge := range lNode.OutgoingEdges() {
		if edge.IsSelfLoop() {
			return true
		}
	}
	return false
}

// Of returns the self loop holder of a node that has the property
// selfLoopHolder. The layout stops when the value is not a holder of this
// package.
func Of(lNode *lgraph.LNode) *SelfLoopHolder {
	holder, ok := prop.Get(lNode, iprops.SelfLoopHolder).(*SelfLoopHolder)
	if !ok || holder == nil {
		lgraph.Bail("SelfLoopHolder: the self loop holder of a node has the type %T", prop.Get(lNode, iprops.SelfLoopHolder))
	}
	return holder
}

////////////////////////////////////////////////////////////////////////////
// Initialization

const (
	// unvisited is the id of a port that the search did not visit yet.
	unvisited = 0
	// visited is the id of a port that the search visited.
	visited = 1
)

// initialize is initialize(): it fills the data model.
func (h *SelfLoopHolder) initialize() {
	// Make a self loop edge and the self loop ports for each self loop.
	for _, lEdge := range h.LNode.OutgoingEdges() {
		if lEdge.IsSelfLoop() {
			slSource := h.selfLoopPortFor(lEdge.Source)
			slTarget := h.selfLoopPortFor(lEdge.Target)
			NewSelfLoopEdge(lEdge, slSource, slTarget)
		}
	}

	// Reset the port ids for the breadth-first search.
	for _, slPort := range h.SLPortMap.Values() {
		slPort.LPort.ID = unvisited
	}

	// Search from each port to gather the edges into hyper loops.
	for _, slPort := range h.SLPortMap.Values() {
		if slPort.LPort.ID == unvisited {
			h.SLHyperLoops = append(h.SLHyperLoops, h.initializeHyperLoop(slPort))
		}
	}
}

// selfLoopPortFor is selfLoopPortFor(lport): the SelfLoopPort of the port.
// It makes one when the port has none.
func (h *SelfLoopHolder) selfLoopPortFor(lport *lgraph.LPort) *SelfLoopPort {
	if !h.SLPortMap.ContainsKey(lport) {
		h.SLPortMap.Put(lport, NewSelfLoopPort(lport))
	}

	return h.SLPortMap.Get(lport)
}

// initializeHyperLoop is initializeHyperLoop(slPort): it collects the self
// loops that can be reached from the port into one hyper loop.
func (h *SelfLoopHolder) initializeHyperLoop(slPort *SelfLoopPort) *SelfHyperLoop {
	slLoop := NewSelfHyperLoop(h)

	// Run a breadth-first search that starts at the port.
	bfsQueue := []*SelfLoopPort{slPort}

	for len(bfsQueue) != 0 {
		currentSLPort := bfsQueue[0]
		bfsQueue = bfsQueue[1:]
		currentSLPort.LPort.ID = visited

		// Add each outgoing edge to the hyper loop.
		for _, slEdge := range currentSLPort.OutgoingSLEdges {
			slLoop.AddSelfLoopEdge(slEdge)

			slTargetPort := slEdge.SLTarget
			if slTargetPort.LPort.ID == unvisited {
				bfsQueue = append(bfsQueue, slTargetPort)
			}
		}

		// Add each incoming edge to the hyper loop.
		for _, slEdge := range currentSLPort.IncomingSLEdges {
			slLoop.AddSelfLoopEdge(slEdge)

			slSourcePort := slEdge.SLSource
			if slSourcePort.LPort.ID == unvisited {
				bfsQueue = append(bfsQueue, slSourcePort)
			}
		}
	}

	return slLoop
}

////////////////////////////////////////////////////////////////////////////
// Accessors

// ArePortsHidden is arePortsHidden(): one self loop port or more is hidden
// from the node. SelfLoopPort.IsHidden tells which ones.
func (h *SelfLoopHolder) ArePortsHidden() bool { return h.PortsHidden }

// SetPortsHidden is setPortsHidden(hidden).
func (h *SelfLoopHolder) SetPortsHidden(hidden bool) {
	h.PortsHidden = hidden
	h.PortsHiddenAssigned = true
}

// SelfLoopEdgesOfPort returns the edges of the self loop port of the given
// port (getSLPortMap().get(port)): the LEdge of each of its incoming self
// loop edges and of each of its outgoing self loop edges, in order. ok is
// false when the holder has no self loop port for the port.
//
// The port adapter of the package adapters reads the self loops of a port
// through this method (the interface adapters.SelfLoopHolder).
func (h *SelfLoopHolder) SelfLoopEdgesOfPort(port *lgraph.LPort) (incoming, outgoing []*lgraph.LEdge, ok bool) {
	slPort, ok := h.SLPortMap.Lookup(port)
	if !ok || slPort == nil {
		return nil, nil, false
	}
	for _, slEdge := range slPort.IncomingSLEdges {
		incoming = append(incoming, slEdge.LEdge)
	}
	for _, slEdge := range slPort.OutgoingSLEdges {
		outgoing = append(outgoing, slEdge.LEdge)
	}
	return incoming, outgoing, true
}
