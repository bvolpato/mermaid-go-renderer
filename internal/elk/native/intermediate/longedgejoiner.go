// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/LongEdgeJoiner.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package intermediate

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("LongEdgeJoiner", func() lgraph.Processor { return NewLongEdgeJoiner() })
}

// LongEdgeJoiner is the processor LongEdgeJoiner: it removes the long edge
// dummy nodes and joins the edges of each one.
type LongEdgeJoiner struct{}

// NewLongEdgeJoiner is new LongEdgeJoiner().
func NewLongEdgeJoiner() *LongEdgeJoiner { return &LongEdgeJoiner{} }

// Process is process(layeredGraph, monitor).
func (*LongEdgeJoiner) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	addUnnecessaryBendpoints := prop.Get(layeredGraph, lopts.UnnecessaryBendpoints)

	// Iterate through the layers
	for _, layer := range layeredGraph.Layers {
		// The Java code removes a dummy node through the list iterator of the
		// node list. The removed node keeps its layer.
		for i := 0; i < len(layer.Nodes); {
			node := layer.Nodes[i]
			i++

			// Check if it's a dummy edge we're looking for
			if node.Type == lgraph.NodeTypeLongEdge {
				JoinAt(node, addUnnecessaryBendpoints)

				// Remove the node
				i--
				layer.Nodes = append(layer.Nodes[:i], layer.Nodes[i+1:]...)
			}
		}
	}
}

// firstPortOfSide is node.getPorts(side).iterator().next(). Java throws a
// NoSuchElementException for a node without a port on the side.
func firstPortOfSide(node *lgraph.LNode, side core.PortSide) *lgraph.LPort {
	for _, port := range node.Ports {
		if port.Side == side {
			return port
		}
	}
	lgraph.Bail("the dummy node %s has no port on the side %v", node, side)
	return nil
}

// JoinAt is LongEdgeJoiner.joinAt(longEdgeDummy, addUnnecessaryBendpoints):
// it joins the edges of the dummy node. The dummy node can then be removed
// from the graph. With addUnnecessaryBendpoints, each edge gets a bend point
// at the position of the dummy node.
func JoinAt(longEdgeDummy *lgraph.LNode, addUnnecessaryBendpoints bool) {
	// Get the input and output port (of which we assume to have only one, on
	// the western side and on the eastern side, respectively); the incoming
	// edges are retained, and the outgoing edges are discarded
	inputPort := firstPortOfSide(longEdgeDummy, core.PortSideWest)
	outputPort := firstPortOfSide(longEdgeDummy, core.PortSideEast)
	edgeCount := len(inputPort.IncomingEdges)

	// If we are to add unnecessary bend points, we need to know where. We
	// take the position of the first port we find. (It doesn't really matter
	// which port we're using, so we opt to keep it surprisingly simple.)
	unnecessaryBendpoint := longEdgeDummy.Ports[0].AbsoluteAnchor()

	// The following code assumes that edges with the same indices in the two
	// lists originate from the same long edge, which is true for the current
	// implementation of LongEdgeSplitter and HyperedgeDummyMerger
	for ; edgeCount > 0; edgeCount-- {
		// Get the two edges
		if len(inputPort.IncomingEdges) == 0 || len(outputPort.OutgoingEdges) == 0 {
			// Java throws an IndexOutOfBoundsException.
			lgraph.Bail("the long edge dummy %s has more incoming edges than outgoing edges", longEdgeDummy)
		}
		survivingEdge := inputPort.IncomingEdges[0]
		droppedEdge := outputPort.OutgoingEdges[0]
		if droppedEdge.Target == nil {
			// Java throws a NullPointerException.
			lgraph.Bail("an outgoing edge of the long edge dummy %s has no target", longEdgeDummy)
		}

		// The surviving edge's target needs to be set to the old target of
		// the dropped edge. However, this doesn't replace the dropped edge
		// with the surviving edge in the list of incoming edges of the (new)
		// target port, but instead appends the surviving edge. That in turn
		// messes with the implicit assumption that edges with the same index
		// on input and output ports of long edge dummies belong to each
		// other. Thus, we need to ensure that the surviving edge is at the
		// correct index in the list of incoming edges. (KIPRA-1670)
		targetIncomingEdges := droppedEdge.Target.IncomingEdges
		droppedEdgeListIndex := -1
		for i, e := range targetIncomingEdges {
			if e == droppedEdge {
				droppedEdgeListIndex = i
				break
			}
		}
		if droppedEdgeListIndex < 0 {
			// Java throws an IndexOutOfBoundsException.
			lgraph.Bail("an edge of the long edge dummy %s is not an incoming edge of its target", longEdgeDummy)
		}
		survivingEdge.SetTargetAndInsertAtIndex(droppedEdge.Target, droppedEdgeListIndex)

		// Remove the dropped edge from the graph
		droppedEdge.SetSource(nil)
		droppedEdge.SetTarget(nil)

		// Join their bend points and add possibly an unnecessary one
		survivingBendPoints := survivingEdge.BendPoints

		if addUnnecessaryBendpoints {
			survivingBendPoints.Add(unnecessaryBendpoint.Clone())
		}

		for _, bendPoint := range droppedEdge.BendPoints.Points {
			survivingBendPoints.Add(bendPoint.Clone())
		}

		// Join their labels
		survivingEdge.Labels = append(survivingEdge.Labels, droppedEdge.Labels...)

		// Join their junction points
		survivingJunctionPoints := prop.Get(survivingEdge, lopts.JunctionPoints)
		droppedJunctionsPoints := prop.Get(droppedEdge, lopts.JunctionPoints)
		if droppedJunctionsPoints != nil {
			if survivingJunctionPoints == nil {
				survivingJunctionPoints = emath.NewKVectorChain()
				prop.Set(survivingEdge, lopts.JunctionPoints, survivingJunctionPoints)
			}
			for _, jp := range droppedJunctionsPoints.Points {
				survivingJunctionPoints.Add(jp.Clone())
			}
		}
	}
}
