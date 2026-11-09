// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/GraphTransformer.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package intermediate

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// The registry names of the graph transformer have the mode after a colon.
// IntermediateProcessorStrategy makes the direction preprocessor with the
// mode TO_INPUT_DIRECTION and the direction postprocessor with the mode
// TO_INTERNAL_LTR.
func init() {
	lgraph.Register("GraphTransformer:TO_INTERNAL_LTR", func() lgraph.Processor {
		return NewGraphTransformer(ModeToInternalLTR)
	})
	lgraph.Register("GraphTransformer:TO_INPUT_DIRECTION", func() lgraph.Processor {
		return NewGraphTransformer(ModeToInputDirection)
	})
}

// GraphTransformerMode is the enum GraphTransformer.Mode.
type GraphTransformerMode int

// The constants of GraphTransformer.Mode, in the order of the Java enum.
const (
	// ModeToInternalLTR is TO_INTERNAL_LTR: from the direction of the input
	// graph to the internal direction, left to right.
	ModeToInternalLTR GraphTransformerMode = iota
	// ModeToInputDirection is TO_INPUT_DIRECTION: from the internal
	// direction back to the direction of the input graph.
	ModeToInputDirection
)

var namesGraphTransformerMode = []string{"TO_INTERNAL_LTR", "TO_INPUT_DIRECTION"}

// String returns the name of the Java constant.
func (m GraphTransformerMode) String() string {
	return prop.EnumName(namesGraphTransformerMode, int(m))
}

// GraphTransformer is the processor GraphTransformer: it mirrors and
// transposes the coordinates of a graph.
type GraphTransformer struct {
	mode GraphTransformerMode
}

// NewGraphTransformer is new GraphTransformer(themode).
func NewGraphTransformer(themode GraphTransformerMode) *GraphTransformer {
	return &GraphTransformer{mode: themode}
}

// Process is process(layeredGraph, monitor).
func (t *GraphTransformer) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	// We need to add all layerless nodes as well as all nodes in layers since
	// this processor is run twice -- once before layering, and once
	// afterwards
	nodes := append([]*lgraph.LNode{}, layeredGraph.LayerlessNodes...)
	for _, layer := range layeredGraph.Layers {
		nodes = append(nodes, layer.Nodes...)
	}

	// graph transformations for unusual layout directions
	congruency := prop.Get(layeredGraph, lopts.DirectionCongruency)
	if congruency == lenum.DirectionCongruencyReadingDirection {
		// variant that preserves reading direction
		switch prop.Get(layeredGraph, lopts.Direction) {
		case core.DirectionLeft:
			t.mirrorAllX(layeredGraph, nodes)
		case core.DirectionDown:
			t.transposeAll(layeredGraph, nodes)
		case core.DirectionUp:
			if t.mode == ModeToInternalLTR {
				t.transposeAll(layeredGraph, nodes)
				t.mirrorAllY(layeredGraph, nodes)
			} else {
				t.mirrorAllY(layeredGraph, nodes)
				t.transposeAll(layeredGraph, nodes)
			}
		}
	} else {
		if t.mode == ModeToInternalLTR {
			// to internally used left-to-right direction
			switch prop.Get(layeredGraph, lopts.Direction) {
			case core.DirectionLeft:
				t.mirrorAllX(layeredGraph, nodes)
				t.mirrorAllY(layeredGraph, nodes)
			case core.DirectionDown:
				t.rotate90Clockwise(layeredGraph, nodes)
			case core.DirectionUp:
				t.rotate90CounterClockwise(layeredGraph, nodes)
			}
		} else {
			// back to original direction
			switch prop.Get(layeredGraph, lopts.Direction) {
			case core.DirectionLeft:
				t.mirrorAllX(layeredGraph, nodes)
				t.mirrorAllY(layeredGraph, nodes)
			case core.DirectionDown:
				t.rotate90CounterClockwise(layeredGraph, nodes)
			case core.DirectionUp:
				t.rotate90Clockwise(layeredGraph, nodes)
			}
		}
	}
}

// Convenience

func (t *GraphTransformer) rotate90Clockwise(layeredGraph *lgraph.LGraph, nodes []*lgraph.LNode) {
	t.transposeAll(layeredGraph, nodes)
	t.mirrorAllX(layeredGraph, nodes)
}

func (t *GraphTransformer) rotate90CounterClockwise(layeredGraph *lgraph.LGraph, nodes []*lgraph.LNode) {
	t.mirrorAllX(layeredGraph, nodes)
	t.transposeAll(layeredGraph, nodes)
}

func (t *GraphTransformer) mirrorAllX(layeredGraph *lgraph.LGraph, nodes []*lgraph.LNode) {
	t.mirrorX(nodes, layeredGraph)
	mirrorSpacingX(&layeredGraph.Padding.Spacing)
	mirrorSpacingX(&prop.Get(layeredGraph, lopts.NodeLabelsPadding).Spacing)
}

func (t *GraphTransformer) mirrorAllY(layeredGraph *lgraph.LGraph, nodes []*lgraph.LNode) {
	t.mirrorY(nodes, layeredGraph)
	mirrorSpacingY(&layeredGraph.Padding.Spacing)
	mirrorSpacingY(&prop.Get(layeredGraph, lopts.NodeLabelsPadding).Spacing)
}

func (t *GraphTransformer) transposeAll(layeredGraph *lgraph.LGraph, nodes []*lgraph.LNode) {
	t.transpose(nodes)
	transposeEdgeLabelPlacement(layeredGraph)
	transposeVector(&layeredGraph.Offset)
	transposeVector(&layeredGraph.Size)
	transposeSpacing(&layeredGraph.Padding.Spacing)
	transposeSpacing(&prop.Get(layeredGraph, lopts.NodeLabelsPadding).Spacing)
}

// Mirror Horizontally

// mirrorX is mirrorX(nodes, graph): it mirrors the x coordinates of the
// graph.
func (t *GraphTransformer) mirrorX(nodes []*lgraph.LNode, graph *lgraph.LGraph) {
	// Assuming that no nodes extend into negative x coordinates, mirroring a
	// node means that the space left to its left border equals the space
	// right to its right border when mirrored:
	//     oldPosition.x = graphWidth - newPosition.x - nodeWidth
	// The offset variable has graphWidth. Once nodes extend into negative
	// coordinates, the offset becomes graphWidth - graphOffset.x.
	offset := 0.0

	// If the graph already had its size calculated, use that; if not, find
	// its width by iterating over its nodes
	if graph.Size.X == 0 {
		for _, node := range nodes {
			offset = max(offset, node.Pos.X+node.Size.X+node.Margin.Right)
		}
	} else {
		offset = graph.Size.X - graph.Offset.X
	}
	offset -= graph.Offset.X

	// mirror all nodes, ports, edges, and labels
	for _, node := range nodes {
		mirrorVectorX(&node.Pos, offset-node.Size.X)
		mirrorSpacingX(&node.Padding.Spacing)
		mirrorNodeLabelPlacementX(node)

		// mirror position
		if prop.Has(node, lopts.Position) {
			mirrorVectorX(prop.Get(node, lopts.Position), offset-node.Size.X)
		}

		// mirror the alignment
		switch prop.Get(node, lopts.Alignment) {
		case core.AlignmentLeft:
			prop.Set(node, lopts.Alignment, core.AlignmentRight)
		case core.AlignmentRight:
			prop.Set(node, lopts.Alignment, core.AlignmentLeft)
		}

		nodeSize := &node.Size
		for _, port := range node.Ports {
			mirrorVectorX(&port.Pos, nodeSize.X-port.Size.X)
			mirrorVectorX(&port.Anchor, port.Size.X)
			mirrorPortSideX(port)
			reverseIndex(port)

			for _, edge := range port.OutgoingEdges {
				// Mirror bend points
				for _, bendPoint := range edge.BendPoints.Points {
					mirrorVectorX(bendPoint, offset)
				}

				// Mirror junction points
				junctionPoints := prop.Get(edge, lopts.JunctionPoints)
				if junctionPoints != nil {
					for _, jp := range junctionPoints.Points {
						mirrorVectorX(jp, offset)
					}
				}

				// Mirror edge label positions
				for _, label := range edge.Labels {
					mirrorVectorX(&label.Pos, offset-label.Size.X)
				}
			}

			// Mirror port label positions
			for _, label := range port.Labels {
				mirrorVectorX(&label.Pos, port.Size.X-label.Size.X)
			}
		}

		// External port dummy?
		if node.Type == lgraph.NodeTypeExternalPort {
			mirrorExternalPortSideX(node)
			mirrorLayerConstraintX(node)
		}

		// Mirror node labels
		for _, label := range node.Labels {
			mirrorNodeLabelPlacementX(label)
			mirrorVectorX(&label.Pos, nodeSize.X-label.Size.X)
		}
	}
}

// mirrorVectorX is mirrorX(KVector, double): it mirrors the x coordinate of
// the vector and adds an offset.
func mirrorVectorX(v *emath.KVector, offset float64) {
	v.X = offset - v.X
}

// mirrorSpacingX is mirrorX(Spacing).
func mirrorSpacingX(spacing *emath.Spacing) {
	oldLeft := spacing.Left
	oldRight := spacing.Right

	spacing.Left = oldRight
	spacing.Right = oldLeft
}

// mirrorNodeLabelPlacementX mirrors the node label placement of a node or of
// a label horizontally, if one is set.
func mirrorNodeLabelPlacementX(shape prop.Holder) {
	if !prop.Has(shape, lopts.NodeLabelsPlacement) {
		return
	}

	oldPlacement := prop.Get(shape, lopts.NodeLabelsPlacement)
	if oldPlacement.Contains(core.NodeLabelPlacementHLeft) {
		oldPlacement.Remove(core.NodeLabelPlacementHLeft)
		oldPlacement.Add(core.NodeLabelPlacementHRight)
	} else if oldPlacement.Contains(core.NodeLabelPlacementHRight) {
		oldPlacement.Remove(core.NodeLabelPlacementHRight)
		oldPlacement.Add(core.NodeLabelPlacementHLeft)
	}
}

// mirrorPortSideX mirrors the side of the port. An undefined side stays.
func mirrorPortSideX(port *lgraph.LPort) {
	port.SetSide(getMirroredPortSideX(port.Side))
}

// mirrorExternalPortSideX mirrors the side of the external port that the
// dummy node stands for.
func mirrorExternalPortSideX(node *lgraph.LNode) {
	prop.Set(node, iprops.ExtPortSide, getMirroredPortSideX(prop.Get(node, iprops.ExtPortSide)))
}

// getMirroredPortSideX returns the side that is horizontally opposite.
func getMirroredPortSideX(side core.PortSide) core.PortSide {
	switch side {
	case core.PortSideEast:
		return core.PortSideWest
	case core.PortSideWest:
		return core.PortSideEast
	default:
		return side
	}
}

// mirrorLayerConstraintX mirrors the layer constraint of an external port
// dummy node horizontally.
func mirrorLayerConstraintX(node *lgraph.LNode) {
	switch prop.Get(node, lopts.LayeringLayerConstraint) {
	case lenum.LayerConstraintFirst:
		prop.Set(node, lopts.LayeringLayerConstraint, lenum.LayerConstraintLast)
	case lenum.LayerConstraintFirstSeparate:
		prop.Set(node, lopts.LayeringLayerConstraint, lenum.LayerConstraintLastSeparate)
	case lenum.LayerConstraintLast:
		prop.Set(node, lopts.LayeringLayerConstraint, lenum.LayerConstraintFirst)
	case lenum.LayerConstraintLastSeparate:
		prop.Set(node, lopts.LayeringLayerConstraint, lenum.LayerConstraintFirstSeparate)
	}
}

// Mirror Vertically

// mirrorY is mirrorY(nodes, graph): it mirrors the y coordinates of the
// graph.
func (t *GraphTransformer) mirrorY(nodes []*lgraph.LNode, graph *lgraph.LGraph) {
	// See mirrorX for an explanation of how the offset is calculated
	offset := 0.0
	if graph.Size.Y == 0 {
		for _, node := range nodes {
			offset = max(offset, node.Pos.Y+node.Size.Y+node.Margin.Bottom)
		}
	} else {
		offset = graph.Size.Y - graph.Offset.Y
	}
	offset -= graph.Offset.Y

	// mirror all nodes, ports, edges, and labels
	for _, node := range nodes {
		mirrorVectorY(&node.Pos, offset-node.Size.Y)
		mirrorSpacingY(&node.Padding.Spacing)
		mirrorNodeLabelPlacementY(node)

		// mirror position
		if prop.Has(node, lopts.Position) {
			mirrorVectorY(prop.Get(node, lopts.Position), offset-node.Size.Y)
		}

		// mirror the alignment
		switch prop.Get(node, lopts.Alignment) {
		case core.AlignmentTop:
			prop.Set(node, lopts.Alignment, core.AlignmentBottom)
		case core.AlignmentBottom:
			prop.Set(node, lopts.Alignment, core.AlignmentTop)
		}

		nodeSize := &node.Size
		for _, port := range node.Ports {
			mirrorVectorY(&port.Pos, nodeSize.Y-port.Size.Y)
			mirrorVectorY(&port.Anchor, port.Size.Y)
			mirrorPortSideY(port)
			reverseIndex(port)

			for _, edge := range port.OutgoingEdges {
				// Mirror bend points
				for _, bendPoint := range edge.BendPoints.Points {
					mirrorVectorY(bendPoint, offset)
				}

				// Mirror junction points
				junctionPoints := prop.Get(edge, lopts.JunctionPoints)
				if junctionPoints != nil {
					for _, jp := range junctionPoints.Points {
						mirrorVectorY(jp, offset)
					}
				}

				// Mirror edge label positions
				for _, label := range edge.Labels {
					mirrorVectorY(&label.Pos, offset-label.Size.Y)
				}
			}

			// Mirror port label positions
			for _, label := range port.Labels {
				mirrorVectorY(&label.Pos, port.Size.Y-label.Size.Y)
			}
		}

		// External port dummy?
		if node.Type == lgraph.NodeTypeExternalPort {
			mirrorExternalPortSideY(node)
			mirrorInLayerConstraintY(node)
		}

		// Mirror node labels
		for _, label := range node.Labels {
			mirrorNodeLabelPlacementY(label)
			mirrorVectorY(&label.Pos, nodeSize.Y-label.Size.Y)
		}
	}
}

// mirrorVectorY is mirrorY(KVector, double): it mirrors the y coordinate of
// the vector and adds an offset.
func mirrorVectorY(v *emath.KVector, offset float64) {
	v.Y = offset - v.Y
}

// mirrorSpacingY is mirrorY(Spacing).
func mirrorSpacingY(spacing *emath.Spacing) {
	oldTop := spacing.Top
	oldBottom := spacing.Bottom

	spacing.Top = oldBottom
	spacing.Bottom = oldTop
}

// mirrorNodeLabelPlacementY mirrors the node label placement of a node or of
// a label vertically, if one is set.
func mirrorNodeLabelPlacementY(shape prop.Holder) {
	if !prop.Has(shape, lopts.NodeLabelsPlacement) {
		return
	}

	oldPlacement := prop.Get(shape, lopts.NodeLabelsPlacement)
	if oldPlacement.Contains(core.NodeLabelPlacementVTop) {
		oldPlacement.Remove(core.NodeLabelPlacementVTop)
		oldPlacement.Add(core.NodeLabelPlacementVBottom)
	} else if oldPlacement.Contains(core.NodeLabelPlacementVBottom) {
		oldPlacement.Remove(core.NodeLabelPlacementVBottom)
		oldPlacement.Add(core.NodeLabelPlacementVTop)
	}
}

// mirrorPortSideY mirrors the side of the port. An undefined side stays.
func mirrorPortSideY(port *lgraph.LPort) {
	port.SetSide(getMirroredPortSideY(port.Side))
}

// mirrorExternalPortSideY mirrors the side of the external port that the
// dummy node stands for.
func mirrorExternalPortSideY(node *lgraph.LNode) {
	prop.Set(node, iprops.ExtPortSide, getMirroredPortSideY(prop.Get(node, iprops.ExtPortSide)))
}

// getMirroredPortSideY returns the side that is vertically opposite.
func getMirroredPortSideY(side core.PortSide) core.PortSide {
	switch side {
	case core.PortSideNorth:
		return core.PortSideSouth
	case core.PortSideSouth:
		return core.PortSideNorth
	default:
		return side
	}
}

// mirrorInLayerConstraintY mirrors the in-layer constraint of an external
// port dummy node vertically.
func mirrorInLayerConstraintY(node *lgraph.LNode) {
	switch prop.Get(node, iprops.InLayerConstraint) {
	case lenum.InLayerConstraintTop:
		prop.Set(node, iprops.InLayerConstraint, lenum.InLayerConstraintBottom)
	case lenum.InLayerConstraintBottom:
		prop.Set(node, iprops.InLayerConstraint, lenum.InLayerConstraintTop)
	}
}

// Transpose

// transpose is transpose(nodes): it swaps the x and y coordinates of the
// graph.
func (t *GraphTransformer) transpose(nodes []*lgraph.LNode) {
	// Transpose nodes
	for _, node := range nodes {
		transposeVector(&node.Pos)
		transposeVector(&node.Size)
		transposeSpacing(&node.Padding.Spacing)
		transposeNodeLabelPlacement(node)
		transposeProperties(node)

		// Transpose ports
		for _, port := range node.Ports {
			transposeVector(&port.Pos)
			transposeVector(&port.Anchor)
			transposeVector(&port.Size)
			transposePortSideOf(port)
			reverseIndex(port)

			// Transpose edges
			for _, edge := range port.OutgoingEdges {
				// Transpose bend points
				for _, bendPoint := range edge.BendPoints.Points {
					transposeVector(bendPoint)
				}

				// Transpose junction points
				junctionPoints := prop.Get(edge, lopts.JunctionPoints)
				if junctionPoints != nil {
					for _, jp := range junctionPoints.Points {
						transposeVector(jp)
					}
				}

				// Transpose edge labels
				for _, label := range edge.Labels {
					transposeVector(&label.Pos)
					transposeVector(&label.Size)
				}
			}

			// Transpose port labels
			for _, label := range port.Labels {
				transposeVector(&label.Pos)
				transposeVector(&label.Size)
			}
		}

		// External port dummy?
		if node.Type == lgraph.NodeTypeExternalPort {
			transposeExternalPortSide(node)
			transposeLayerConstraint(node)
		}

		// Transpose node labels
		for _, label := range node.Labels {
			transposeNodeLabelPlacement(label)
			transposeVector(&label.Size)
			transposeVector(&label.Pos)
		}
	}
}

// transposeVector is transpose(KVector).
func transposeVector(v *emath.KVector) {
	temp := v.X
	v.X = v.Y
	v.Y = temp
}

// transposeSpacing is transpose(Spacing).
func transposeSpacing(spacing *emath.Spacing) {
	oldTop := spacing.Top
	oldBottom := spacing.Bottom
	oldLeft := spacing.Left
	oldRight := spacing.Right

	spacing.Top = oldLeft
	spacing.Bottom = oldRight
	spacing.Left = oldTop
	spacing.Right = oldBottom
}

// transposeNodeLabelPlacement transposes the node label placement of a node
// or of a label, if one is set.
func transposeNodeLabelPlacement(shape prop.Holder) {
	if !prop.Has(shape, lopts.NodeLabelsPlacement) {
		return
	}
	oldPlacement := prop.Get(shape, lopts.NodeLabelsPlacement)
	if oldPlacement.IsEmpty() {
		return
	}

	// Build up a new node label placement enumeration
	newPlacement := prop.NoneOf[core.NodeLabelPlacement]()

	// Inside or outside
	if oldPlacement.Contains(core.NodeLabelPlacementInside) {
		newPlacement.Add(core.NodeLabelPlacementInside)
	} else {
		newPlacement.Add(core.NodeLabelPlacementOutside)
	}

	// Horizontal priority
	if !oldPlacement.Contains(core.NodeLabelPlacementHPriority) {
		newPlacement.Add(core.NodeLabelPlacementHPriority)
	}

	// Horizontal alignment
	if oldPlacement.Contains(core.NodeLabelPlacementHLeft) {
		newPlacement.Add(core.NodeLabelPlacementVTop)
	} else if oldPlacement.Contains(core.NodeLabelPlacementHCenter) {
		newPlacement.Add(core.NodeLabelPlacementVCenter)
	} else if oldPlacement.Contains(core.NodeLabelPlacementHRight) {
		newPlacement.Add(core.NodeLabelPlacementVBottom)
	}

	// Vertical alignment
	if oldPlacement.Contains(core.NodeLabelPlacementVTop) {
		newPlacement.Add(core.NodeLabelPlacementHLeft)
	} else if oldPlacement.Contains(core.NodeLabelPlacementVCenter) {
		newPlacement.Add(core.NodeLabelPlacementHCenter)
	} else if oldPlacement.Contains(core.NodeLabelPlacementVBottom) {
		newPlacement.Add(core.NodeLabelPlacementHRight)
	}

	// Apply new placement
	prop.Set(shape, lopts.NodeLabelsPlacement, newPlacement)
}

// transposePortSideOf is transposePortSide(LPort): it transposes the side of
// the port.
func transposePortSideOf(p *lgraph.LPort) {
	p.SetSide(transposePortSide(p.Side))
}

// transposePortSide is transposePortSide(PortSide): the transposed side.
func transposePortSide(side core.PortSide) core.PortSide {
	switch side {
	case core.PortSideNorth:
		return core.PortSideWest
	case core.PortSideWest:
		return core.PortSideNorth
	case core.PortSideSouth:
		return core.PortSideEast
	case core.PortSideEast:
		return core.PortSideSouth
	default:
		return core.PortSideUndefined
	}
}

// transposeEdgeLabelSideSelection is EdgeLabelSideSelection.transpose(): a
// downward selection becomes an upward one, and the reverse.
//
// TODO(core): move to lenum, as a method of EdgeLabelSideSelection.
func transposeEdgeLabelSideSelection(side lenum.EdgeLabelSideSelection) lenum.EdgeLabelSideSelection {
	switch side {
	case lenum.EdgeLabelSideSelectionAlwaysUp:
		return lenum.EdgeLabelSideSelectionAlwaysDown
	case lenum.EdgeLabelSideSelectionAlwaysDown:
		return lenum.EdgeLabelSideSelectionAlwaysUp
	case lenum.EdgeLabelSideSelectionDirectionUp:
		return lenum.EdgeLabelSideSelectionDirectionDown
	case lenum.EdgeLabelSideSelectionDirectionDown:
		return lenum.EdgeLabelSideSelectionDirectionUp
	case lenum.EdgeLabelSideSelectionSmartUp:
		return lenum.EdgeLabelSideSelectionSmartDown
	case lenum.EdgeLabelSideSelectionSmartDown:
		return lenum.EdgeLabelSideSelectionSmartUp
	}
	lgraph.Bail("GraphTransformer: unknown edge label side selection %d", int(side))
	return side
}

// transposeEdgeLabelPlacement transposes the placement of the edge labels of
// the graph. The property has a default, so its value is never null.
func transposeEdgeLabelPlacement(graph *lgraph.LGraph) {
	oldSide := prop.Get(graph, lopts.EdgeLabelsSideSelection)
	prop.Set(graph, lopts.EdgeLabelsSideSelection, transposeEdgeLabelSideSelection(oldSide))
}

// transposeExternalPortSide transposes the side of the external port that
// the dummy node stands for.
func transposeExternalPortSide(node *lgraph.LNode) {
	prop.Set(node, iprops.ExtPortSide, transposePortSide(prop.Get(node, iprops.ExtPortSide)))
}

// transposeLayerConstraint swaps the layer constraint and the in-layer
// constraint of an external port dummy node. A node with the layer
// constraint FIRST_SEPARATE gets the in-layer constraint TOP.
func transposeLayerConstraint(node *lgraph.LNode) {
	layerConstraint := prop.Get(node, lopts.LayeringLayerConstraint)
	inLayerConstraint := prop.Get(node, iprops.InLayerConstraint)

	if layerConstraint == lenum.LayerConstraintFirstSeparate {
		prop.Set(node, lopts.LayeringLayerConstraint, lenum.LayerConstraintNone)
		prop.Set(node, iprops.InLayerConstraint, lenum.InLayerConstraintTop)
	} else if layerConstraint == lenum.LayerConstraintLastSeparate {
		prop.Set(node, lopts.LayeringLayerConstraint, lenum.LayerConstraintNone)
		prop.Set(node, iprops.InLayerConstraint, lenum.InLayerConstraintBottom)
	} else if inLayerConstraint == lenum.InLayerConstraintTop {
		prop.Set(node, lopts.LayeringLayerConstraint, lenum.LayerConstraintFirstSeparate)
		prop.Set(node, iprops.InLayerConstraint, lenum.InLayerConstraintNone)
	} else if inLayerConstraint == lenum.InLayerConstraintBottom {
		prop.Set(node, lopts.LayeringLayerConstraint, lenum.LayerConstraintLastSeparate)
		prop.Set(node, iprops.InLayerConstraint, lenum.InLayerConstraintNone)
	}
}

// transposeProperties transposes the properties of a node that depend on the
// direction: the minimum size, the alignment and the position.
func transposeProperties(node *lgraph.LNode) {
	// Transpose MIN_HEIGHT and MIN_WIDTH
	minSize := prop.Get(node, lopts.NodeSizeMinimum)
	prop.Set(node, lopts.NodeSizeMinimum, emath.NewKVector(minSize.Y, minSize.X))

	// Transpose ALIGNMENT
	switch prop.Get(node, lopts.Alignment) {
	case core.AlignmentLeft:
		prop.Set(node, lopts.Alignment, core.AlignmentTop)
	case core.AlignmentRight:
		prop.Set(node, lopts.Alignment, core.AlignmentBottom)
	case core.AlignmentTop:
		prop.Set(node, lopts.Alignment, core.AlignmentLeft)
	case core.AlignmentBottom:
		prop.Set(node, lopts.Alignment, core.AlignmentRight)
	}

	// POSITION
	if prop.Has(node, lopts.Position) {
		pos := prop.Get(node, lopts.Position)
		tmp := pos.X
		pos.X = pos.Y
		pos.Y = tmp
	}
}

// reverseIndex negates the port index, if the port has one.
func reverseIndex(port *lgraph.LPort) {
	if index, ok := prop.Lookup(port, lopts.PortIndex); ok {
		prop.Set(port, lopts.PortIndex, -index)
	}
}
