// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/HyperedgeDummyMerger.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package intermediate

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("HyperedgeDummyMerger", func() lgraph.Processor { return NewHyperedgeDummyMerger() })
}

// HyperedgeDummyMerger is the processor HyperedgeDummyMerger: it merges
// long edge dummy nodes that are next to each other in a layer and belong
// to the same hyperedge.
type HyperedgeDummyMerger struct{}

// NewHyperedgeDummyMerger is new HyperedgeDummyMerger().
func NewHyperedgeDummyMerger() *HyperedgeDummyMerger { return &HyperedgeDummyMerger{} }

// Process is process(layeredGraph, monitor).
func (m *HyperedgeDummyMerger) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	m.identifyHyperedges(layeredGraph)

	// Iterate through the layers
	for _, layer := range layeredGraph.Layers {
		// If there are no nodes anyway, just move on to the next layer
		if len(layer.Nodes) == 0 {
			continue
		}

		var currNode *lgraph.LNode
		var lastNode *lgraph.LNode
		// The Java variables for the node types start as null. The flag
		// tells that lastNodeType has a value.
		var currNodeType, lastNodeType lgraph.NodeType
		hasLastNodeType := false

		// Iterate through the remaining nodes
		for nodeIndex := 0; nodeIndex < len(layer.Nodes); nodeIndex++ {
			// Get the next node
			currNode = layer.Nodes[nodeIndex]
			currNodeType = currNode.Type

			// We're only interested if the current and last nodes are long
			// edge dummies
			if currNodeType == lgraph.NodeTypeLongEdge && hasLastNodeType && lastNodeType == lgraph.NodeTypeLongEdge {
				// If the source or the target are identical and we are
				// allowed to merge, merge the current node into the last
				state := m.checkMergeAllowed(currNode, lastNode)
				if state.allowMerge {
					m.mergeNodes(currNode, lastNode, state.sameSource, state.sameTarget)

					// Remove the current node and make the last node the
					// current node. The removed node keeps its layer.
					layer.Nodes = append(layer.Nodes[:nodeIndex], layer.Nodes[nodeIndex+1:]...)
					nodeIndex--
					currNode = lastNode
					currNodeType = lastNodeType
				}
			}

			// Remember this node for the next iteration
			lastNode = currNode
			lastNodeType = currNodeType
			hasLastNodeType = true
		}
	}
}

// checkMergeAllowed tells whether the two long edge dummies can be merged.
func (m *HyperedgeDummyMerger) checkMergeAllowed(currNode, lastNode *lgraph.LNode) mergeState {
	currHasLabelDummies := prop.Get(currNode, iprops.LongEdgeHasLabelDummies)
	lastHasLabelDummies := prop.Get(lastNode, iprops.LongEdgeHasLabelDummies)

	// Get long edge source and target ports
	currNodeSource := prop.Get(currNode, iprops.LongEdgeSource)
	lastNodeSource := prop.Get(lastNode, iprops.LongEdgeSource)
	currNodeTarget := prop.Get(currNode, iprops.LongEdgeTarget)
	lastNodeTarget := prop.Get(lastNode, iprops.LongEdgeTarget)

	// Find out whether the two long edges come from the same source or target
	// (non-null!)
	sameSource := currNodeSource != nil && currNodeSource == lastNodeSource
	sameTarget := currNodeTarget != nil && currNodeTarget == lastNodeTarget

	if !currHasLabelDummies && !lastHasLabelDummies {
		// assumption: long edge dummies always have two ports, both have the
		// same id
		if len(currNode.Ports) == 0 || len(lastNode.Ports) == 0 {
			// Java throws a NoSuchElementException.
			lgraph.Bail("HyperedgeDummyMerger: a long edge dummy has no port")
		}
		return mergeState{currNode.Ports[0].ID == lastNode.Ports[0].ID, sameSource, sameTarget}
	}

	// If we can merge on grounds of the two long edges having the same
	// source, we need to be in front of the label dummy of each of the two
	// edges (if any)
	eligibleForSourceMerging :=
		(!prop.Get(currNode, iprops.LongEdgeHasLabelDummies) ||
			prop.Get(currNode, iprops.LongEdgeBeforeLabelDummy)) &&
			(!prop.Get(lastNode, iprops.LongEdgeHasLabelDummies) ||
				prop.Get(lastNode, iprops.LongEdgeBeforeLabelDummy))

	// If we can merge on grounds of the two long edges having the same
	// target, we need to be behind the label dummy of each of the two edges
	// (if any)
	eligibleForTargetMerging :=
		(!prop.Get(currNode, iprops.LongEdgeHasLabelDummies) ||
			!prop.Get(currNode, iprops.LongEdgeBeforeLabelDummy)) &&
			(!prop.Get(lastNode, iprops.LongEdgeHasLabelDummies) ||
				!prop.Get(lastNode, iprops.LongEdgeBeforeLabelDummy))

	return mergeState{
		(sameSource && eligibleForSourceMerging) || (sameTarget && eligibleForTargetMerging),
		sameSource, sameTarget,
	}
}

// mergeState is the class HyperedgeDummyMerger.MergeState.
type mergeState struct {
	allowMerge bool
	sameSource bool
	sameTarget bool
}

// mergeNodes merges the merge source node into the merge target node. Each
// edge of a port of the merge source goes to the merge target. Without
// keepSourcePort the long edge source of the merge target becomes null, and
// without keepTargetPort its long edge target: the two nodes have different
// ones.
func (m *HyperedgeDummyMerger) mergeNodes(mergeSource, mergeTarget *lgraph.LNode, keepSourcePort, keepTargetPort bool) {
	// We assume that the input port is west, and the output port east
	mergeTargetInputPort := firstPortOfSide(mergeTarget, core.PortSideWest)
	mergeTargetOutputPort := firstPortOfSide(mergeTarget, core.PortSideEast)

	for _, port := range mergeSource.Ports {
		for len(port.IncomingEdges) != 0 {
			port.IncomingEdges[0].SetTarget(mergeTargetInputPort)
		}

		for len(port.OutgoingEdges) != 0 {
			port.OutgoingEdges[0].SetSource(mergeTargetOutputPort)
		}
	}

	// Possibly reset source and target ports
	if !keepSourcePort {
		prop.Unset(mergeTarget, iprops.LongEdgeSource)
	}

	if !keepTargetPort {
		prop.Unset(mergeTarget, iprops.LongEdgeTarget)
	}
}

// identifyHyperedges gives the ports of each hyperedge one id.
func (m *HyperedgeDummyMerger) identifyHyperedges(lGraph *lgraph.LGraph) {
	var ports []*lgraph.LPort
	for _, l := range lGraph.Layers {
		for _, n := range l.Nodes {
			ports = append(ports, n.Ports...)
		}
	}

	for _, p := range ports {
		p.ID = -1
	}

	index := 0
	for _, p := range ports {
		if p.ID == -1 {
			m.dfs(p, index)
			index++
		}
	}
}

// dfs gives the id index to each port that the port p can reach. The Java
// method is recursive. The port keeps the order of the visits with a stack
// of the ports whose walk is not complete.
func (m *HyperedgeDummyMerger) dfs(start *lgraph.LPort, index int) {
	// A frame is one call of the Java method: the port, and how far its two
	// loops are. The first loop is over the connected ports: the sources of
	// the incoming edges, then the targets of the outgoing edges. The second
	// loop is over the ports of the node of a long edge dummy.
	type frame struct {
		p         *lgraph.LPort
		connected int
		sibling   int
	}
	visit := func(p *lgraph.LPort) frame {
		if p == nil {
			// Java throws a NullPointerException.
			lgraph.Bail("HyperedgeDummyMerger: an edge without a source or a target")
		}
		p.ID = index
		return frame{p: p}
	}
	stack := []frame{visit(start)}
	for len(stack) > 0 {
		f := &stack[len(stack)-1]
		p := f.p
		var next *lgraph.LPort

		// follow edges connected to the same port
		for next == nil && f.connected < len(p.IncomingEdges)+len(p.OutgoingEdges) {
			var p2 *lgraph.LPort
			if f.connected < len(p.IncomingEdges) {
				p2 = p.IncomingEdges[f.connected].Source
			} else {
				p2 = p.OutgoingEdges[f.connected-len(p.IncomingEdges)].Target
			}
			f.connected++
			if p2 == nil {
				// Java throws a NullPointerException.
				lgraph.Bail("HyperedgeDummyMerger: an edge without a source or a target")
			}
			if p2.ID == -1 {
				next = p2
			}
		}

		// follow edges connected to the same long edge dummy
		if next == nil && p.Node().Type == lgraph.NodeTypeLongEdge {
			siblings := p.Node().Ports
			for next == nil && f.sibling < len(siblings) {
				p2 := siblings[f.sibling]
				f.sibling++
				if p2 != p && p2.ID == -1 {
					next = p2
				}
			}
		}

		if next == nil {
			stack = stack[:len(stack)-1]
			continue
		}
		stack = append(stack, visit(next))
	}
}
