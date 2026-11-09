// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/NorthSouthPortPostprocessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package intermediate

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("NorthSouthPortPostprocessor", func() lgraph.Processor { return NewNorthSouthPortPostprocessor() })
}

// NorthSouthPortPostprocessor is the processor NorthSouthPortPostprocessor:
// it removes the dummy nodes of the northern and southern ports and moves
// their edges back to the ports, with a bend point for each dummy node.
type NorthSouthPortPostprocessor struct{}

// NewNorthSouthPortPostprocessor is new NorthSouthPortPostprocessor().
func NewNorthSouthPortPostprocessor() *NorthSouthPortPostprocessor {
	return &NorthSouthPortPostprocessor{}
}

// dummyPortOrigin is (LPort) port.getProperty(ORIGIN) for a port of a
// north/south port dummy: the port that the dummy port was made for. Java
// fails with a ClassCastException or, at the first use, a
// NullPointerException for another origin. Only NorthSouthPortPreprocessor
// makes such dummy nodes, and it gives each of their ports a port as origin.
func dummyPortOrigin(port *lgraph.LPort) *lgraph.LPort {
	origin, ok := prop.Get(port, iprops.Origin).(*lgraph.LPort)
	if !ok || origin == nil {
		lgraph.Bail("the origin of a port of a north/south port dummy is not a port")
	}
	return origin
}

// Process is process(layeredGraph, monitor).
func (p *NorthSouthPortPostprocessor) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	// Spline edge routing manages bend points in the edge router.
	// Differentiate behaviour depending on edge router.
	routing := prop.Get(layeredGraph, lopts.EdgeRouting)

	// Iterate through the layers
	for _, layer := range layeredGraph.Layers {
		// Iterate through the nodes (use an array to avoid concurrent
		// modification exceptions)
		nodeArray := append([]*lgraph.LNode(nil), layer.Nodes...)
		for _, node := range nodeArray {
			// We only care for North/South Port dummy nodes
			if node.Type != lgraph.NodeTypeNorthSouthPort {
				continue
			}

			if routing == core.EdgeRoutingSplines {
				// Iterate through ports
				for _, port := range node.Ports {
					if len(port.IncomingEdges) != 0 {
						p.processSplineInputPort(port)
					}
					if len(port.OutgoingEdges) != 0 {
						p.processSplineOutputPort(port)
					}
				}
			} else if _, isEdge := prop.Get(node, iprops.Origin).(*lgraph.LEdge); isEdge {
				// It's a self-loop
				p.processSelfLoop(node)
			} else {
				// Check if all ports were created for the same origin port
				var sameOriginPort bool
				if len(node.Ports) >= 2 {
					// Iterate over the dummy's ports to find out whether the
					// origin is always the same
					sameOriginPort = true

					currentPort := node.Ports[0]
					var previousPort *lgraph.LPort

					for i := 1; i < len(node.Ports); i++ {
						previousPort = currentPort
						currentPort = node.Ports[i]

						if dummyPortOrigin(previousPort) != dummyPortOrigin(currentPort) {
							// The two ports don't have the same origin
							sameOriginPort = false
							break
						}
					}
				} else {
					sameOriginPort = false
				}

				// Iterate through the ports
				for _, port := range node.Ports {
					if len(port.IncomingEdges) != 0 {
						p.processInputPort(port, sameOriginPort)
					}

					if len(port.OutgoingEdges) != 0 {
						p.processOutputPort(port, sameOriginPort)
					}
				}
			}

			// Remove the node
			node.SetLayer(nil)
		}
	}
}

// processInputPort moves the edges of the input port back to the port that
// the input port was made for. With addJunctionPoints, each edge gets a
// junction point at its new bend point. That is right for a dummy node whose
// ports were all made for the same port.
func (p *NorthSouthPortPostprocessor) processInputPort(inputPort *lgraph.LPort, addJunctionPoints bool) {
	// Retrieve the port the dummy node was created from
	originPort := dummyPortOrigin(inputPort)

	// Calculate the bend point
	x := originPort.AbsoluteAnchor().X
	y := inputPort.Node().Pos.Y

	// Reroute the edges, inserting a new bend point at the position of the
	// dummy node
	edgeArray := append([]*lgraph.LEdge(nil), inputPort.IncomingEdges...)
	for _, inEdge := range edgeArray {
		inEdge.SetTarget(originPort)
		inEdge.BendPoints.Add(emath.NewKVector(x, y))

		// Check if a junction point should be added
		if addJunctionPoints {
			junctionPoints := prop.Get(inEdge, lopts.JunctionPoints)
			if junctionPoints == nil {
				junctionPoints = emath.NewKVectorChain()
				prop.Set(inEdge, lopts.JunctionPoints, junctionPoints)
			}
			junctionPoints.Add(emath.NewKVector(x, y))
		}
	}
}

// processOutputPort moves the edges of the output port back to the port that
// the output port was made for. See processInputPort for addJunctionPoints.
func (p *NorthSouthPortPostprocessor) processOutputPort(outputPort *lgraph.LPort, addJunctionPoints bool) {
	// Retrieve the port the dummy node was created from
	originPort := dummyPortOrigin(outputPort)

	// Calculate the bend point
	x := originPort.AbsoluteAnchor().X
	y := outputPort.Node().Pos.Y

	// Reroute the edges, inserting a new bend point at the position of the
	// dummy node
	edgeArray := append([]*lgraph.LEdge(nil), outputPort.OutgoingEdges...)
	for _, outEdge := range edgeArray {
		outEdge.SetSource(originPort)
		outEdge.BendPoints.AddFirst(emath.NewKVector(x, y))

		// Check if a junction point should be added
		if addJunctionPoints {
			junctionPoints := prop.Get(outEdge, lopts.JunctionPoints)
			if junctionPoints == nil {
				junctionPoints = emath.NewKVectorChain()
				prop.Set(outEdge, lopts.JunctionPoints, junctionPoints)
			}
			junctionPoints.Add(emath.NewKVector(x, y))
		}
	}
}

// processSelfLoop connects the self-loop edge of the dummy node again and
// gives it two bend points.
func (p *NorthSouthPortPostprocessor) processSelfLoop(dummy *lgraph.LNode) {
	// Get the edge and the ports it was originally connected to
	selfLoop := prop.Get(dummy, iprops.Origin).(*lgraph.LEdge)
	inputPort := firstPortOfSide(dummy, core.PortSideWest)
	outputPort := firstPortOfSide(dummy, core.PortSideEast)
	originInputPort := dummyPortOrigin(inputPort)
	originOutputPort := dummyPortOrigin(outputPort)

	// Reconnect the edge
	selfLoop.SetSource(originOutputPort)
	selfLoop.SetTarget(originInputPort)

	// Add two bend points
	bendPoint := outputPort.Node().Pos.Clone()
	bendPoint.X = originOutputPort.AbsoluteAnchor().X
	selfLoop.BendPoints.Add(bendPoint)

	bendPoint = inputPort.Node().Pos.Clone()
	bendPoint.X = originInputPort.AbsoluteAnchor().X
	selfLoop.BendPoints.Add(bendPoint)
}

// processSplineInputPort moves the edges of the input port back to the port
// that the input port was made for.
func (p *NorthSouthPortPostprocessor) processSplineInputPort(inputPort *lgraph.LPort) {
	// Retrieve the port the dummy node was created from
	originPort := dummyPortOrigin(inputPort)
	prop.Set(originPort, iprops.SplineNsPortYCoord, inputPort.Node().Pos.Y)

	// Reroute the edges
	edgeArray := append([]*lgraph.LEdge(nil), inputPort.IncomingEdges...)
	for _, inEdge := range edgeArray {
		inEdge.SetTarget(originPort)
	}
}

// processSplineOutputPort moves the edges of the output port back to the
// port that the output port was made for.
func (p *NorthSouthPortPostprocessor) processSplineOutputPort(outputPort *lgraph.LPort) {
	// Retrieve the port the dummy node was created from
	originPort := dummyPortOrigin(outputPort)
	prop.Set(originPort, iprops.SplineNsPortYCoord, outputPort.Node().Pos.Y)

	// Reroute the edges
	edgeArray := append([]*lgraph.LEdge(nil), outputPort.OutgoingEdges...)
	for _, outEdge := range edgeArray {
		outEdge.SetSource(originPort)
	}
}
