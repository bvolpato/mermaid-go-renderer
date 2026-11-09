// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/internal/NodeLabelLocation.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package internal has the data holders of the node size calculation of ELK:
// the node context, the port contexts and the node label locations.
package internal

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/cellsystem"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// NodeLabelLocation is the enum NodeLabelLocation: each place where a node
// label can be, with the alignments and the container area that belong to
// it.
type NodeLabelLocation int

// The constants of NodeLabelLocation, in the order of the Java enum.
const (
	// NodeLabelLocationOutTL is OUT_T_L: outside top left.
	NodeLabelLocationOutTL NodeLabelLocation = iota
	// NodeLabelLocationOutTC is OUT_T_C: outside top center.
	NodeLabelLocationOutTC
	// NodeLabelLocationOutTR is OUT_T_R: outside top right.
	NodeLabelLocationOutTR
	// NodeLabelLocationOutBL is OUT_B_L: outside bottom left.
	NodeLabelLocationOutBL
	// NodeLabelLocationOutBC is OUT_B_C: outside bottom center.
	NodeLabelLocationOutBC
	// NodeLabelLocationOutBR is OUT_B_R: outside bottom right.
	NodeLabelLocationOutBR
	// NodeLabelLocationOutLT is OUT_L_T: outside left top.
	NodeLabelLocationOutLT
	// NodeLabelLocationOutLC is OUT_L_C: outside left center.
	NodeLabelLocationOutLC
	// NodeLabelLocationOutLB is OUT_L_B: outside left bottom.
	NodeLabelLocationOutLB
	// NodeLabelLocationOutRT is OUT_R_T: outside right top.
	NodeLabelLocationOutRT
	// NodeLabelLocationOutRC is OUT_R_C: outside right center.
	NodeLabelLocationOutRC
	// NodeLabelLocationOutRB is OUT_R_B: outside right bottom.
	NodeLabelLocationOutRB
	// NodeLabelLocationInTL is IN_T_L: inside top left.
	NodeLabelLocationInTL
	// NodeLabelLocationInTC is IN_T_C: inside top center.
	NodeLabelLocationInTC
	// NodeLabelLocationInTR is IN_T_R: inside top right.
	NodeLabelLocationInTR
	// NodeLabelLocationInCL is IN_C_L: inside center left.
	NodeLabelLocationInCL
	// NodeLabelLocationInCC is IN_C_C: inside center center.
	NodeLabelLocationInCC
	// NodeLabelLocationInCR is IN_C_R: inside center right.
	NodeLabelLocationInCR
	// NodeLabelLocationInBL is IN_B_L: inside bottom left.
	NodeLabelLocationInBL
	// NodeLabelLocationInBC is IN_B_C: inside bottom center.
	NodeLabelLocationInBC
	// NodeLabelLocationInBR is IN_B_R: inside bottom right.
	NodeLabelLocationInBR
	// NodeLabelLocationUndefined is UNDEFINED: undefined or not decidable.
	NodeLabelLocationUndefined
)

// NodeLabelLocationCount is NodeLabelLocation.values().length.
const NodeLabelLocationCount = int(NodeLabelLocationUndefined) + 1

var namesNodeLabelLocation = []string{
	"OUT_T_L", "OUT_T_C", "OUT_T_R", "OUT_B_L", "OUT_B_C", "OUT_B_R",
	"OUT_L_T", "OUT_L_C", "OUT_L_B", "OUT_R_T", "OUT_R_C", "OUT_R_B",
	"IN_T_L", "IN_T_C", "IN_T_R", "IN_C_L", "IN_C_C", "IN_C_R",
	"IN_B_L", "IN_B_C", "IN_B_R", "UNDEFINED",
}

// String returns the name of the Java constant.
func (l NodeLabelLocation) String() string { return prop.EnumName(namesNodeLabelLocation, int(l)) }

// ValuesNodeLabelLocation is NodeLabelLocation.values().
func ValuesNodeLabelLocation() []NodeLabelLocation {
	out := make([]NodeLabelLocation, NodeLabelLocationCount)
	for i := range out {
		out[i] = NodeLabelLocation(i)
	}
	return out
}

// nodeLabelLocationData has the fields of one constant.
type nodeLabelLocationData struct {
	// horizontalAlignment is the horizontal text alignment of the location.
	horizontalAlignment cellsystem.HorizontalLabelAlignment
	// verticalAlignment is the vertical text alignment of the location.
	verticalAlignment cellsystem.VerticalLabelAlignment
	// containerRow is the label row in a container cell.
	containerRow cellsystem.ContainerArea
	// containerColumn is the label column in a container cell.
	containerColumn cellsystem.ContainerArea
	// assignedPlacements has the placements that lead to the location.
	assignedPlacements []*prop.EnumSet[core.NodeLabelPlacement]
}

func placements(values ...core.NodeLabelPlacement) *prop.EnumSet[core.NodeLabelPlacement] {
	return prop.Of(values...)
}

// Short names for the tables.
const (
	hLeft   = cellsystem.HorizontalLabelAlignmentLeft
	hCenter = cellsystem.HorizontalLabelAlignmentCenter
	hRight  = cellsystem.HorizontalLabelAlignmentRight
	vTop    = cellsystem.VerticalLabelAlignmentTop
	vCenter = cellsystem.VerticalLabelAlignmentCenter
	vBottom = cellsystem.VerticalLabelAlignmentBottom
	begin   = cellsystem.ContainerAreaBegin
	center  = cellsystem.ContainerAreaCenter
	end     = cellsystem.ContainerAreaEnd

	pInside    = core.NodeLabelPlacementInside
	pOutside   = core.NodeLabelPlacementOutside
	pHLeft     = core.NodeLabelPlacementHLeft
	pHCenter   = core.NodeLabelPlacementHCenter
	pHRight    = core.NodeLabelPlacementHRight
	pVTop      = core.NodeLabelPlacementVTop
	pVCenter   = core.NodeLabelPlacementVCenter
	pVBottom   = core.NodeLabelPlacementVBottom
	pHPriority = core.NodeLabelPlacementHPriority
)

// nodeLabelLocations has the data of the constants, by ordinal. The values
// of UNDEFINED are null in Java. No code reads them.
var nodeLabelLocations = [NodeLabelLocationCount]nodeLabelLocationData{
	NodeLabelLocationOutTL: {hLeft, vBottom, begin, begin, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pOutside, pVTop, pHLeft)}},
	NodeLabelLocationOutTC: {hCenter, vBottom, begin, center, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pOutside, pVTop, pHCenter),
		placements(pOutside, pVTop, pHCenter, pHPriority)}},
	NodeLabelLocationOutTR: {hRight, vBottom, begin, end, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pOutside, pVTop, pHRight)}},
	NodeLabelLocationOutBL: {hLeft, vTop, end, begin, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pOutside, pVBottom, pHLeft)}},
	NodeLabelLocationOutBC: {hCenter, vTop, end, center, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pOutside, pVBottom, pHCenter),
		placements(pOutside, pVBottom, pHCenter, pHPriority)}},
	NodeLabelLocationOutBR: {hRight, vTop, end, end, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pOutside, pVBottom, pHRight)}},
	NodeLabelLocationOutLT: {hRight, vTop, begin, begin, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pOutside, pHLeft, pVTop, pHPriority)}},
	NodeLabelLocationOutLC: {hRight, vCenter, center, begin, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pOutside, pHLeft, pVCenter),
		placements(pOutside, pHLeft, pVCenter, pHPriority)}},
	NodeLabelLocationOutLB: {hRight, vBottom, end, begin, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pOutside, pHLeft, pVBottom, pHPriority)}},
	NodeLabelLocationOutRT: {hLeft, vTop, begin, end, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pOutside, pHRight, pVTop, pHPriority)}},
	NodeLabelLocationOutRC: {hLeft, vCenter, center, end, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pOutside, pHRight, pVCenter),
		placements(pOutside, pHRight, pVCenter, pHPriority)}},
	NodeLabelLocationOutRB: {hLeft, vBottom, end, end, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pOutside, pHRight, pVBottom, pHPriority)}},
	NodeLabelLocationInTL: {hLeft, vTop, begin, begin, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pInside, pVTop, pHLeft),
		placements(pInside, pVTop, pHLeft, pHPriority)}},
	NodeLabelLocationInTC: {hCenter, vTop, begin, center, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pInside, pVTop, pHCenter),
		placements(pInside, pVTop, pHCenter, pHPriority)}},
	NodeLabelLocationInTR: {hRight, vTop, begin, end, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pInside, pVTop, pHRight),
		placements(pInside, pVTop, pHRight, pHPriority)}},
	NodeLabelLocationInCL: {hLeft, vCenter, center, begin, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pInside, pVCenter, pHLeft),
		placements(pInside, pVCenter, pHLeft, pHPriority)}},
	NodeLabelLocationInCC: {hCenter, vCenter, center, center, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pInside, pVCenter, pHCenter),
		placements(pInside, pVCenter, pHCenter, pHPriority)}},
	NodeLabelLocationInCR: {hRight, vCenter, center, end, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pInside, pVCenter, pHRight),
		placements(pInside, pVCenter, pHRight, pHPriority)}},
	NodeLabelLocationInBL: {hLeft, vBottom, end, begin, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pInside, pVBottom, pHLeft),
		placements(pInside, pVBottom, pHLeft, pHPriority)}},
	NodeLabelLocationInBC: {hCenter, vBottom, end, center, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pInside, pVBottom, pHCenter),
		placements(pInside, pVBottom, pHCenter, pHPriority)}},
	NodeLabelLocationInBR: {hRight, vBottom, end, end, []*prop.EnumSet[core.NodeLabelPlacement]{
		placements(pInside, pVBottom, pHRight),
		placements(pInside, pVBottom, pHRight, pHPriority)}},
	NodeLabelLocationUndefined: {},
}

// NodeLabelLocationFromNodeLabelPlacement is
// NodeLabelLocation.fromNodeLabelPlacement(labelPlacement): the location of
// a set of placements, or UNDEFINED for a set that is no valid combination.
func NodeLabelLocationFromNodeLabelPlacement(labelPlacement *prop.EnumSet[core.NodeLabelPlacement]) NodeLabelLocation {
	for _, location := range ValuesNodeLabelLocation() {
		for _, assigned := range nodeLabelLocations[location].assignedPlacements {
			if assigned.Equals(labelPlacement) {
				return location
			}
		}
	}
	return NodeLabelLocationUndefined
}

// HorizontalAlignment is getHorizontalAlignment().
func (l NodeLabelLocation) HorizontalAlignment() cellsystem.HorizontalLabelAlignment {
	return nodeLabelLocations[l].horizontalAlignment
}

// VerticalAlignment is getVerticalAlignment().
func (l NodeLabelLocation) VerticalAlignment() cellsystem.VerticalLabelAlignment {
	return nodeLabelLocations[l].verticalAlignment
}

// ContainerRow is getContainerRow().
func (l NodeLabelLocation) ContainerRow() cellsystem.ContainerArea {
	return nodeLabelLocations[l].containerRow
}

// ContainerColumn is getContainerColumn().
func (l NodeLabelLocation) ContainerColumn() cellsystem.ContainerArea {
	return nodeLabelLocations[l].containerColumn
}

// IsInsideLocation is isInsideLocation().
func (l NodeLabelLocation) IsInsideLocation() bool {
	switch l {
	case NodeLabelLocationInTL, NodeLabelLocationInTC, NodeLabelLocationInTR,
		NodeLabelLocationInCL, NodeLabelLocationInCC, NodeLabelLocationInCR,
		NodeLabelLocationInBL, NodeLabelLocationInBC, NodeLabelLocationInBR:
		return true
	default:
		return false
	}
}

// OutsideSide is getOutsideSide(): the side of the node of an outside
// location, or UNDEFINED for an inside location and for UNDEFINED.
func (l NodeLabelLocation) OutsideSide() core.PortSide {
	switch l {
	case NodeLabelLocationOutTL, NodeLabelLocationOutTC, NodeLabelLocationOutTR:
		return core.PortSideNorth
	case NodeLabelLocationOutBL, NodeLabelLocationOutBC, NodeLabelLocationOutBR:
		return core.PortSideSouth
	case NodeLabelLocationOutLT, NodeLabelLocationOutLC, NodeLabelLocationOutLB:
		return core.PortSideWest
	case NodeLabelLocationOutRT, NodeLabelLocationOutRC, NodeLabelLocationOutRB:
		return core.PortSideEast
	default:
		return core.PortSideUndefined
	}
}
