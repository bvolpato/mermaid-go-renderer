// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/NorthSouthPortPreprocessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package intermediate

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("NorthSouthPortPreprocessor", func() lgraph.Processor { return NewNorthSouthPortPreprocessor() })
}

// NorthSouthPortPreprocessor is the processor NorthSouthPortPreprocessor: it
// makes a dummy node for each northern and southern port with edges, and it
// moves the edges of the port to the dummy node.
//
// The Java class has the constant USE_NEW_APPROACH, which is true. GWT
// removes the code of the old approach, and so does this port: the dummy
// nodes of the northern side have the regular node as successor, and the
// regular node has the dummy nodes of the southern side as successors.
type NorthSouthPortPreprocessor struct{}

// NewNorthSouthPortPreprocessor is new NorthSouthPortPreprocessor().
func NewNorthSouthPortPreprocessor() *NorthSouthPortPreprocessor {
	return &NorthSouthPortPreprocessor{}
}

// originPortOf returns the port that the first port of a dummy node was made
// for. Java fails with an IndexOutOfBoundsException for a dummy node without
// a port.
func originPortOf(dummy *lgraph.LNode) *lgraph.LPort {
	if len(dummy.Ports) == 0 {
		lgraph.Bail("NorthSouthPortPreprocessor: a dummy node has no port")
	}
	return dummyPortOrigin(dummy.Ports[0])
}

// addSuccessorConstraint is
// node.getProperty(IN_LAYER_SUCCESSOR_CONSTRAINTS).add(successor).
func addSuccessorConstraint(node, successor *lgraph.LNode) {
	constraints := prop.Get(node, iprops.InLayerSuccessorConstraints)
	prop.Set(node, iprops.InLayerSuccessorConstraints, append(constraints, successor))
}

// Process is process(layeredGraph, monitor).
func (p *NorthSouthPortPreprocessor) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	var pointer int
	var northDummyNodes []*lgraph.LNode
	var southDummyNodes []*lgraph.LNode

	// Iterate through the layers
	for _, layer := range layeredGraph.Layers {
		// The pointer indicates the index of the current node while northern
		// ports are processed, and the index of the most recently inserted
		// dummy while south ports are processed
		pointer = -1

		// Iterate through the nodes (use an array to avoid concurrent
		// modification exceptions)
		nodeArray := append([]*lgraph.LNode(nil), layer.Nodes...)
		for _, node := range nodeArray {
			pointer++

			// We only care about non-dummy nodes with fixed port sides
			if !(node.Type == lgraph.NodeTypeNormal && prop.Get(node, lopts.PortConstraints).IsSideFixed()) {
				continue
			}

			// Sort the port list if we have control over the port order
			if !prop.Get(node, lopts.PortConstraints).IsOrderFixed() {
				p.sortPortList(node)
			}

			// Nodes form their own layout unit
			prop.Set(node, iprops.InLayerLayoutUnit, node)

			// Clear the lists of northern and southern dummy nodes
			northDummyNodes = northDummyNodes[:0]
			southDummyNodes = southDummyNodes[:0]

			// Create a list of barycenter associates for the node
			var barycenterAssociates []*lgraph.LNode

			// Prepare a list of ports on the northern side, sorted from left
			// to right (when viewed in the diagram); create the appropriate
			// dummy nodes and assign them to the layer
			portList := node.PortsBySide(core.PortSideNorth)

			p.createDummyNodes(layeredGraph, portList, &northDummyNodes, &southDummyNodes, &barycenterAssociates)

			// We will now iterate over all nodes and insert successor
			// constraints to each one of them. Each dummy on the northern
			// side has its regular node as a successor while the node has all
			// southern dummies as its successors.

			insertPoint := pointer
			successor := node
			for _, dummy := range northDummyNodes {
				dummy.SetLayerAt(insertPoint, layer)
				pointer++

				// The dummy nodes form a layout unit identified by the node
				// they were created from. In addition, northern dummy nodes
				// must appear before the regular node
				prop.Set(dummy, iprops.InLayerLayoutUnit, node)

				// If originPort has port constraint
				// ALLOW_NON_FLOW_PORTS_TO_SWITCH_SIDES, do not apply
				// successor constraints to the dummy node dummy. Their
				// position will be determined according to their barycenter
				// value. If originPort does not have the port constraint
				// ALLOW_NON_FLOW_PORTS_TO_SWITCH_SIDES, the dummy node dummy
				// needs to appear before the regular node.

				// Each dummy node has least one port (there may be two if an
				// odd port has both an incoming and an outgoing edge, however
				// the origin is the same). The origin is the port the dummy
				// node was created for.
				originPort := originPortOf(dummy)

				if !prop.Get(originPort, lopts.AllowNonFlowPortsToSwitchSides) {
					addSuccessorConstraint(dummy, successor)
				}
			}

			// Do the same for ports on the southern side; the list of ports
			// must be built in reversed order, since ports on the southern
			// side are listed from right to left
			southPorts := node.PortsBySide(core.PortSideSouth)
			portList = make([]*lgraph.LPort, 0, len(southPorts))
			for i := len(southPorts) - 1; i >= 0; i-- {
				portList = append(portList, southPorts[i])
			}

			p.createDummyNodes(layeredGraph, portList, &southDummyNodes, nil, &barycenterAssociates)

			predecessor := node
			for _, dummy := range southDummyNodes {
				pointer++
				dummy.SetLayerAt(pointer, layer)

				// The dummy nodes form a layout unit identified by the node
				// they were created from. In addition, southern dummy nodes
				// must appear after the regular node
				prop.Set(dummy, iprops.InLayerLayoutUnit, node)

				// See the comment in the loop over the northern dummy nodes.
				originPort := originPortOf(dummy)

				if !prop.Get(originPort, lopts.AllowNonFlowPortsToSwitchSides) {
					addSuccessorConstraint(predecessor, dummy)
				}
			}

			// If the list of barycenter associates contains nodes, set the
			// appropriate property
			if len(barycenterAssociates) != 0 {
				prop.Set(node, iprops.BarycenterAssociates, barycenterAssociates)
			}
		}
	}
}

// PORT LIST SORTING

// sortPortList sorts the northern and southern ports such that ports with
// only incoming edges end up left, ports with only outgoing edges end up
// right, and ports with both end up in between. The ports on the eastern and
// western sides keep their order.
func (p *NorthSouthPortPreprocessor) sortPortList(node *lgraph.LNode) {
	ports := len(node.Ports)

	// Next IDs for ports with a given configuration of input and output
	// edges. The choice of initial IDs ensures that port IDs will be unique
	inPorts := 0
	inOutPorts := ports
	outPorts := 2 * ports

	// Iterate over the list of ports and set their IDs
	for _, port := range node.Ports {
		switch port.Side {
		case core.PortSideEast, core.PortSideWest:
			port.ID = -1

		case core.PortSideNorth, core.PortSideSouth:
			incoming := len(port.IncomingEdges)
			outgoing := len(port.OutgoingEdges)

			if incoming > 0 && outgoing > 0 {
				port.ID = inOutPorts
				inOutPorts++
			} else if incoming > 0 {
				port.ID = inPorts
				inPorts++
			} else if outgoing > 0 {
				port.ID = outPorts
				outPorts++
			} else {
				// Unconnected ports are placed between input ports...
				port.ID = inPorts
				inPorts++
			}
		}
	}

	// With all IDs assigned, sort the port list
	gwt.SortFunc(node.Ports, func(port1, port2 *lgraph.LPort) int {
		side1 := port1.Side
		side2 := port2.Side

		if side1 != side2 {
			// sort according to the node side
			return int(side1) - int(side2)
		}
		if port1.ID == port2.ID {
			// Eastern and western ports have the same ID and have to retain
			// their order
			return 0
		}
		if side1 == core.PortSideNorth {
			return port1.ID - port2.ID
		}
		return port2.ID - port1.ID
	})
}

// DUMMY NODE CREATION

// createDummyNodes makes the dummy nodes for the given ports, which must be
// sorted by position from left to right. It adds each dummy node to
// dummyNodes. It adds the dummy nodes that north-south self-loops need on
// the southern side to opposingSideDummyNodes, which can be nil for the
// southern ports. It adds the dummy nodes that are not for self-loops to
// barycenterAssociates.
func (p *NorthSouthPortPreprocessor) createDummyNodes(layeredGraph *lgraph.LGraph, ports []*lgraph.LPort, dummyNodes, opposingSideDummyNodes, barycenterAssociates *[]*lgraph.LNode) {
	// We'll assemble lists of ports with only incoming, ports with only
	// outgoing and ports with both, incoming and outgoing edges
	inPorts := make([]*lgraph.LPort, 0, len(ports))
	outPorts := make([]*lgraph.LPort, 0, len(ports))
	inOutPorts := make([]*lgraph.LPort, 0, len(ports))
	sameSideSelfLoopEdges := make([]*lgraph.LEdge, 0, len(ports))
	northSouthSelfLoopEdges := make([]*lgraph.LEdge, 0, len(ports))

	for _, port := range ports {
		// Go through the port's outgoing edges, looking for self-loops that
		// need special handling
		for _, edge := range port.OutgoingEdges {
			// Check for self loops we'd be interested in
			if edge.Source.Node() == edge.Target.Node() {
				// Check which sides the ports are on
				if port.Side == edge.Target.Side {
					// Same side
					sameSideSelfLoopEdges = append(sameSideSelfLoopEdges, edge)
					continue
				} else if port.Side == core.PortSideNorth && edge.Target.Side == core.PortSideSouth {
					// North->South self-loop. Due to the SelfLoopProcessor, a
					// South->North self-loop cannot happen
					northSouthSelfLoopEdges = append(northSouthSelfLoopEdges, edge)
					continue
				}
			}
		}
	}

	// First, create the dummy nodes that handle north->south self-loops. For
	// now, we always route north->south self-loops east to the node. This
	// could later change, though.
	for _, edge := range northSouthSelfLoopEdges {
		p.createNorthSouthSelfLoopDummyNodes(layeredGraph, edge, dummyNodes, opposingSideDummyNodes, core.PortSideEast)
	}

	// Second, create the dummy nodes that handle same-side self-loops
	for _, edge := range sameSideSelfLoopEdges {
		p.createSameSideSelfLoopDummyNode(layeredGraph, edge, dummyNodes)
	}

	// Now we iterate over the ports again, with certain self-loop edges
	// already removed, and check if they are input ports, output ports, or
	// both
	for _, port := range ports {
		// Find out if the port has incoming or outgoing edges
		in := len(port.IncomingEdges) > 0
		out := len(port.OutgoingEdges) > 0

		if in && out {
			inOutPorts = append(inOutPorts, port)
		} else if in {
			inPorts = append(inPorts, port)
		} else if out {
			outPorts = append(outPorts, port)
		}
	}

	// Give the rest of input and output ports their dummy nodes
	for _, inPort := range inPorts {
		*barycenterAssociates = append(*barycenterAssociates, p.createDummyNode(layeredGraph, inPort, nil, dummyNodes))
	}

	for _, outPort := range outPorts {
		*barycenterAssociates = append(*barycenterAssociates, p.createDummyNode(layeredGraph, nil, outPort, dummyNodes))
	}

	// in / out ports get their own dummy nodes
	for _, inOutPort := range inOutPorts {
		*barycenterAssociates = append(*barycenterAssociates, p.createDummyNode(layeredGraph, inOutPort, inOutPort, dummyNodes))
	}
}

// createDummyNode makes a dummy node for the given ports. The edges that go
// into inPort go to the input port of the dummy node. The edges that leave
// outPort leave the output port of the dummy node. Both arguments can be the
// same port, and each can be nil. The origin of each port of the dummy node
// is the port whose edges it got.
func (p *NorthSouthPortPreprocessor) createDummyNode(layeredGraph *lgraph.LGraph, inPort, outPort *lgraph.LPort, dummyNodes *[]*lgraph.LNode) *lgraph.LNode {
	dummy := lgraph.NewLNode(layeredGraph)
	dummy.Type = lgraph.NodeTypeNorthSouthPort
	prop.Set(dummy, lopts.PortConstraints, core.PortConstraintsFixedPos)

	crossingHint := 0

	// Input port
	if inPort != nil {
		dummyInputPort := lgraph.NewLPort()
		prop.Set[any](dummyInputPort, iprops.Origin, inPort)
		prop.Set[any](dummy, iprops.Origin, inPort.Node())
		dummyInputPort.SetSide(core.PortSideWest)
		dummyInputPort.SetNode(dummy)

		// Reroute edges
		edgeArray := append([]*lgraph.LEdge(nil), inPort.IncomingEdges...)
		for _, edge := range edgeArray {
			edge.SetTarget(dummyInputPort)
		}

		// Make sure the inPort knows about the dummy node
		prop.Set(inPort, iprops.PortDummy, dummy)

		crossingHint++
	}

	// Output port
	if outPort != nil {
		dummyOutputPort := lgraph.NewLPort()
		prop.Set[any](dummy, iprops.Origin, outPort.Node())
		prop.Set[any](dummyOutputPort, iprops.Origin, outPort)
		dummyOutputPort.SetSide(core.PortSideEast)
		dummyOutputPort.SetNode(dummy)

		// Reroute edges
		edgeArray := append([]*lgraph.LEdge(nil), outPort.OutgoingEdges...)
		for _, edge := range edgeArray {
			edge.SetSource(dummyOutputPort)
		}

		// Make sure the outPort knows about the dummy node
		prop.Set(outPort, iprops.PortDummy, dummy)

		crossingHint++
	}

	// Set the crossing hint used for cross counting later
	prop.Set(dummy, iprops.CrossingHint, crossingHint)

	*dummyNodes = append(*dummyNodes, dummy)

	return dummy
}

// createSameSideSelfLoopDummyNode makes a dummy node for a self-loop edge
// whose ports are on the same side. The origin of the dummy node is the
// edge. The dummy node has two ports, one for each port of the edge, with
// that port as origin.
func (p *NorthSouthPortPreprocessor) createSameSideSelfLoopDummyNode(layeredGraph *lgraph.LGraph, selfLoop *lgraph.LEdge, dummyNodes *[]*lgraph.LNode) {
	dummy := lgraph.NewLNode(layeredGraph)
	dummy.Type = lgraph.NodeTypeNorthSouthPort
	prop.Set(dummy, lopts.PortConstraints, core.PortConstraintsFixedPos)
	prop.Set[any](dummy, iprops.Origin, selfLoop)

	// Input port
	dummyInputPort := lgraph.NewLPort()
	prop.Set[any](dummyInputPort, iprops.Origin, selfLoop.Target)
	dummyInputPort.SetSide(core.PortSideWest)
	dummyInputPort.SetNode(dummy)

	// Output port
	dummyOutputPort := lgraph.NewLPort()
	prop.Set[any](dummyOutputPort, iprops.Origin, selfLoop.Source)
	dummyOutputPort.SetSide(core.PortSideEast)
	dummyOutputPort.SetNode(dummy)

	// Make sure the ports know about the dummy node
	prop.Set(selfLoop.Source, iprops.PortDummy, dummy)
	prop.Set(selfLoop.Target, iprops.PortDummy, dummy)

	// Disconnect the edge
	selfLoop.SetSource(nil)
	selfLoop.SetTarget(nil)

	*dummyNodes = append(*dummyNodes, dummy)

	// Set the crossing hint used for cross counting later
	prop.Set(dummy, iprops.CrossingHint, 2)
}

// createNorthSouthSelfLoopDummyNodes makes two dummy nodes for a north-south
// self-loop edge. Each dummy node has one port, on the given side, with the
// port of the edge as origin. The northern dummy node goes to the start of
// northDummyNodes, and the southern one to the end of southDummyNodes.
func (p *NorthSouthPortPreprocessor) createNorthSouthSelfLoopDummyNodes(layeredGraph *lgraph.LGraph, selfLoop *lgraph.LEdge, northDummyNodes, southDummyNodes *[]*lgraph.LNode, portSide core.PortSide) {
	// North dummy
	northDummy := lgraph.NewLNode(layeredGraph)
	northDummy.Type = lgraph.NodeTypeNorthSouthPort
	prop.Set(northDummy, lopts.PortConstraints, core.PortConstraintsFixedPos)
	prop.Set[any](northDummy, iprops.Origin, selfLoop.Source.Node())

	northDummyOutputPort := lgraph.NewLPort()
	prop.Set[any](northDummyOutputPort, iprops.Origin, selfLoop.Source)
	northDummyOutputPort.SetSide(portSide)
	northDummyOutputPort.SetNode(northDummy)

	prop.Set(selfLoop.Source, iprops.PortDummy, northDummy)

	// South dummy
	southDummy := lgraph.NewLNode(layeredGraph)
	southDummy.Type = lgraph.NodeTypeNorthSouthPort
	prop.Set(southDummy, lopts.PortConstraints, core.PortConstraintsFixedPos)
	prop.Set[any](southDummy, iprops.Origin, selfLoop.Target.Node())

	southDummyInputPort := lgraph.NewLPort()
	prop.Set[any](southDummyInputPort, iprops.Origin, selfLoop.Target)
	southDummyInputPort.SetSide(portSide)
	southDummyInputPort.SetNode(southDummy)

	prop.Set(selfLoop.Target, iprops.PortDummy, southDummy)

	// Reroute the edge
	selfLoop.SetSource(northDummyOutputPort)
	selfLoop.SetTarget(southDummyInputPort)

	if southDummyNodes == nil {
		// Java fails with a NullPointerException. It cannot happen: only a
		// northern port starts a north-south self-loop.
		lgraph.Bail("NorthSouthPortPreprocessor: a north-south self-loop at a southern port")
	}
	*northDummyNodes = append([]*lgraph.LNode{northDummy}, *northDummyNodes...)
	*southDummyNodes = append(*southDummyNodes, southDummy)

	// Set the crossing hints used for cross counting later
	prop.Set(northDummy, iprops.CrossingHint, 1)
	prop.Set(southDummy, iprops.CrossingHint, 1)
}
