// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/internal/NodeContext.java
// of Eclipse ELK 0.9.1 (EPL-2.0), with the change of the pull request 955
// that elkjs 0.9.3 has (the field topdownLayout).

package internal

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/coreopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/cellsystem"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// portSideCount is PortSide.values().length.
const portSideCount = int(core.PortSideWest) + 1

// PortContextMultimap is the TreeMultimap<PortSide, PortContext> of the node
// context: the port contexts of each side. The sides are in the order of
// the enum, and the contexts of a side are sorted with ComparePortContexts,
// which is left to right and top to bottom.
type PortContextMultimap struct {
	bySide [portSideCount][]*PortContext
}

// Put is put(portSide, portContext). The contexts of a side are a TreeSet:
// a context that compares equal to one of the side is not added.
func (m *PortContextMultimap) Put(portSide core.PortSide, portContext *PortContext) bool {
	list := m.bySide[portSide]
	index := len(list)
	for i, other := range list {
		c := ComparePortContexts(portContext, other)
		if c == 0 {
			return false
		}
		if c < 0 {
			index = i
			break
		}
	}
	list = append(list, nil)
	copy(list[index+1:], list[index:])
	list[index] = portContext
	m.bySide[portSide] = list
	return true
}

// Get is get(portSide): the contexts of the side in order. The caller must
// not change the slice.
func (m *PortContextMultimap) Get(portSide core.PortSide) []*PortContext {
	return m.bySide[portSide]
}

// Values is values(): the contexts of each side, in the order of the sides.
func (m *PortContextMultimap) Values() []*PortContext {
	var out []*PortContext
	for _, list := range m.bySide {
		out = append(out, list...)
	}
	return out
}

// NodeContext is the class NodeContext: the settings of a node and the cell
// system for it, during the node size calculation. The algorithm keeps the
// size of the node in NodeSize and applies it at the end with
// ApplyNodeSize.
type NodeContext struct {
	// Node is the node we calculate stuff for.
	Node adapters.NodeAdapter
	// NodeSize is the size of the node, to be applied (or not) at the end.
	NodeSize *emath.KVector
	// TopdownLayout tells that the node is laid out in the top-down mode.
	TopdownLayout bool
	// TreatAsCompoundNode tells that the node has stuff inside it.
	TreatAsCompoundNode bool
	// SizeConstraints has the size constraints of the node.
	SizeConstraints *prop.EnumSet[core.SizeConstraint]
	// SizeOptions has the size options of the node.
	SizeOptions *prop.EnumSet[core.SizeOptions]
	// PortConstraints has the port constraints of the node.
	PortConstraints core.PortConstraints
	// PortLabelsPlacement tells whether port labels are inside or outside.
	PortLabelsPlacement *prop.EnumSet[core.PortLabelPlacement]
	// PortLabelsTreatAsGroup tells that the port labels of a port are
	// centered as a group next to an eastern or western port.
	PortLabelsTreatAsGroup bool
	// NodeLabelPlacement is the default placement of the node labels.
	NodeLabelPlacement *prop.EnumSet[core.NodeLabelPlacement]
	// NodeLabelsPadding is the space to leave around the node label area.
	NodeLabelsPadding *emath.ElkPadding
	// NodeLabelSpacing is the space between a node and its outside labels.
	NodeLabelSpacing float64
	// LabelLabelSpacing is the space between two labels.
	LabelLabelSpacing float64
	// LabelCellSpacing is the space between two different label cells.
	LabelCellSpacing float64
	// PortPortSpacing is the space between a port and another port.
	PortPortSpacing float64
	// PortLabelSpacingHorizontal is the horizontal space between a port and
	// its labels.
	PortLabelSpacingHorizontal float64
	// PortLabelSpacingVertical is the vertical space between a port and its
	// labels.
	PortLabelSpacingVertical float64
	// SurroundingPortMargins is the margin to leave around the set of ports
	// on each side.
	SurroundingPortMargins *emath.ElkMargin

	// PortContexts has the context of each port, sorted left to right and
	// top to bottom.
	PortContexts PortContextMultimap

	// NodeContainer is the main cell that holds all the cells of the node.
	NodeContainer *cellsystem.StripContainerCell
	// NodeContainerMiddleRow is the middle row of the main cell.
	NodeContainerMiddleRow *cellsystem.StripContainerCell
	// InsideNodeLabelContainer is the grid container for the area of the
	// inside node labels and the client area.
	InsideNodeLabelContainer *cellsystem.GridContainerCell
	// InsidePortLabelCells has the cells for the space of the ports and the
	// inside port labels, by port side. Their paddings also have the space
	// between the ports and the node border.
	InsidePortLabelCells [portSideCount]*cellsystem.AtomicCell
	// OutsideNodeLabelContainers has the container cells for the label
	// cells of the outside node labels, by port side.
	OutsideNodeLabelContainers [portSideCount]*cellsystem.StripContainerCell
	// NodeLabelCells has the label cells of the node labels, inside and
	// outside, by location.
	NodeLabelCells [NodeLabelLocationCount]*cellsystem.LabelCell
}

// NewNodeContext is new NodeContext(parentGraph, node). The constructor does
// not read the parent graph. It reads the graph of the node.
func NewNodeContext(parentGraph adapters.GraphAdapter, node adapters.NodeAdapter) *NodeContext {
	c := &NodeContext{}
	c.Node = node
	c.NodeSize = node.Size().Clone()

	// Top-down layout
	c.TopdownLayout = adapters.GetProperty(node, coreopts.TopdownLayout)

	// Compound node
	c.TreatAsCompoundNode = node.IsCompoundNode() || adapters.GetProperty(node, coreopts.InsideSelfLoopsActivate)

	// Core size settings
	c.SizeConstraints = adapters.GetProperty(node, coreopts.NodeSizeConstraints)
	c.SizeOptions = adapters.GetProperty(node, coreopts.NodeSizeOptions)
	c.PortConstraints = adapters.GetProperty(node, coreopts.PortConstraints)
	c.PortLabelsPlacement = adapters.GetProperty(node, coreopts.PortLabelsPlacement)
	if !portLabelPlacementIsValid(c.PortLabelsPlacement) {
		lgraph.Bail("Invalid port label placement: %v", c.PortLabelsPlacement)
	}
	c.PortLabelsTreatAsGroup = adapters.GetProperty(node, coreopts.PortLabelsTreatAsGroup)
	c.NodeLabelPlacement = adapters.GetProperty(node, coreopts.NodeLabelsPlacement)
	if !nodeLabelPlacementIsValid(c.NodeLabelPlacement) {
		lgraph.Bail("Invalid node label placement: %v", c.NodeLabelPlacement)
	}

	// Copy spacings for convenience
	c.NodeLabelsPadding = getIndividualOrInherited(node, coreopts.NodeLabelsPadding)
	c.NodeLabelSpacing = getIndividualOrInherited(node, coreopts.SpacingLabelNode)
	c.LabelLabelSpacing = getIndividualOrInherited(node, coreopts.SpacingLabelLabel)
	c.PortPortSpacing = getIndividualOrInherited(node, coreopts.SpacingPortPort)
	c.PortLabelSpacingHorizontal = getIndividualOrInherited(node, coreopts.SpacingLabelPortHorizontal)
	c.PortLabelSpacingVertical = getIndividualOrInherited(node, coreopts.SpacingLabelPortVertical)
	c.SurroundingPortMargins = getIndividualOrInherited(node, coreopts.SpacingPortsSurrounding)

	c.LabelCellSpacing = 2 * c.LabelLabelSpacing

	// Create main cells (the others will be created later)
	symmetry := !c.SizeOptions.Contains(core.SizeOptionsAsymmetrical)
	c.NodeContainer = cellsystem.NewStripContainerCell(cellsystem.StripVertical, symmetry, 0)
	c.NodeContainerMiddleRow = cellsystem.NewStripContainerCell(cellsystem.StripHorizontal, symmetry, 0)
	c.NodeContainer.SetCell(cellsystem.ContainerAreaCenter, c.NodeContainerMiddleRow)
	return c
}

// portLabelPlacementIsValid is PortLabelPlacement.isValid(placement): at
// most one of INSIDE and OUTSIDE, and at most one of ALWAYS_SAME_SIDE,
// ALWAYS_OTHER_SAME_SIDE and SPACE_EFFICIENT.
//
// TODO(core): move to core/methods.go.
func portLabelPlacementIsValid(placement *prop.EnumSet[core.PortLabelPlacement]) bool {
	validInsideOutside := prop.Of(core.PortLabelPlacementInside, core.PortLabelPlacementOutside)
	validInsideOutside.RetainAll(placement)
	if validInsideOutside.Len() > 1 {
		return false
	}

	validPosition := prop.Of(core.PortLabelPlacementAlwaysSameSide, core.PortLabelPlacementAlwaysOtherSameSide,
		core.PortLabelPlacementSpaceEfficient)
	validPosition.RetainAll(placement)
	return validPosition.Len() <= 1
}

// nodeLabelPlacementIsValid is NodeLabelPlacement.isValid(placement): at
// most one of INSIDE and OUTSIDE, at most one horizontal placement and at
// most one vertical placement.
//
// TODO(core): move to core/methods.go.
func nodeLabelPlacementIsValid(placement *prop.EnumSet[core.NodeLabelPlacement]) bool {
	validInsideOutside := prop.Of(core.NodeLabelPlacementInside, core.NodeLabelPlacementOutside)
	validInsideOutside.RetainAll(placement)
	if validInsideOutside.Len() > 1 {
		return false
	}

	validHorizontal := prop.Of(core.NodeLabelPlacementHLeft, core.NodeLabelPlacementHCenter, core.NodeLabelPlacementHRight)
	validHorizontal.RetainAll(placement)
	if validHorizontal.Len() > 1 {
		return false
	}

	validVertical := prop.Of(core.NodeLabelPlacementVTop, core.NodeLabelPlacementVCenter, core.NodeLabelPlacementVBottom)
	validVertical.RetainAll(placement)
	return validVertical.Len() <= 1
}

// PortLabelPlacementIsFixed is PortLabelPlacement.isFixed(placement): the
// labels are neither inside nor outside, so they keep their positions.
//
// TODO(core): move to core/methods.go.
func PortLabelPlacementIsFixed(placement *prop.EnumSet[core.PortLabelPlacement]) bool {
	return !placement.Contains(core.PortLabelPlacementInside) && !placement.Contains(core.PortLabelPlacementOutside)
}

// ApplyNodeSize is applyNodeSize(): it sets the size of the node to
// NodeSize.
func (c *NodeContext) ApplyNodeSize() {
	c.Node.SetSize(c.NodeSize)
}

// GetPortAlignment is getPortAlignment(portSide): the port alignment of the
// side, or the default port alignment of the node.
func (c *NodeContext) GetPortAlignment(portSide core.PortSide) core.PortAlignment {
	switch portSide {
	case core.PortSideNorth:
		if adapters.HasProperty(c.Node, coreopts.PortAlignmentNorth) {
			return adapters.GetProperty(c.Node, coreopts.PortAlignmentNorth)
		}
	case core.PortSideSouth:
		if adapters.HasProperty(c.Node, coreopts.PortAlignmentSouth) {
			return adapters.GetProperty(c.Node, coreopts.PortAlignmentSouth)
		}
	case core.PortSideEast:
		if adapters.HasProperty(c.Node, coreopts.PortAlignmentEast) {
			return adapters.GetProperty(c.Node, coreopts.PortAlignmentEast)
		}
	case core.PortSideWest:
		if adapters.HasProperty(c.Node, coreopts.PortAlignmentWest) {
			return adapters.GetProperty(c.Node, coreopts.PortAlignmentWest)
		}
	}

	// Fall back to basic port alignment if we haven't found a more specific
	// one yet
	return adapters.GetProperty(c.Node, coreopts.PortAlignmentDefault)
}

// ComparePortSides is comparePortSides(portSide1, portSide2).
func ComparePortSides(portSide1, portSide2 core.PortSide) int {
	return gwt.IntCompare(int(portSide1), int(portSide2))
}

// ComparePortContexts is comparePortContexts(portContext1, portContext2).
func ComparePortContexts(portContext1, portContext2 *PortContext) int {
	portSideComparison := ComparePortSides(portContext1.Port.Side(), portContext2.Port.Side())
	if portSideComparison != 0 {
		return portSideComparison
	}

	// Two ports on the same side. Depending on the side, we need an
	// ascending or a descending comparison, because ports are numbered in a
	// clockwise manner, but we want them to be sorted left-to-right /
	// top-to-bottom
	switch portContext1.Port.Side() {
	case core.PortSideNorth, core.PortSideEast:
		return gwt.IntCompare(portContext1.Port.VolatileID(), portContext2.Port.VolatileID())
	case core.PortSideSouth, core.PortSideWest:
		return gwt.IntCompare(portContext2.Port.VolatileID(), portContext1.Port.VolatileID())
	}
	return 0
}
