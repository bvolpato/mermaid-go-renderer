// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/greedyswitch/SwitchDecider.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package greedyswitch

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p3order/counting"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// GraphData is the part of the class GraphInfoHolder of the package p3order
// that the greedy switch heuristic reads. p3order imports this package, so
// the class is an interface here.
type GraphData interface {
	// HasParent is hasParent().
	HasParent() bool
	// DontSweepInto is dontSweepInto().
	DontSweepInto() bool
	// Parent is parent(): the node whose nested graph the graph is.
	Parent() *lgraph.LNode
	// CurrentNodeOrder is currentNodeOrder().
	CurrentNodeOrder() [][]*lgraph.LNode
	// PortPositions is portPositions().
	PortPositions() []int
	// ParentGraphData is parentGraphData(): the data of the graph of the
	// parent node, or nil.
	ParentGraphData() GraphData
}

// CrossingCountSide is the enum SwitchDecider.CrossingCountSide: the side on
// which the one-sided SwitchDecider counts crossings.
type CrossingCountSide int

// The constants of CrossingCountSide, in the order of the Java enum.
const (
	// CrossingCountSideWest considers crossings to the west of the free
	// layer.
	CrossingCountSideWest CrossingCountSide = iota
	// CrossingCountSideEast considers crossings to the east of the free
	// layer.
	CrossingCountSideEast
)

// SwitchDecider is the class SwitchDecider: it decides whether two
// neighboring nodes should be switched. The one-sided variant checks whether
// a switch reduces the crossings on one side of the layer. The two-sided
// variant checks both sides.
type SwitchDecider struct {
	freeLayer                        []*lgraph.LNode
	leftInLayerCounter               *counting.CrossingsCounter
	rightInLayerCounter              *counting.CrossingsCounter
	northSouthCounter                *NorthSouthEdgeNeighbouringNodeCrossingsCounter
	crossingMatrixFiller             *CrossingMatrixFiller
	graphData                        GraphData
	parentCrossCounter               *counting.CrossingsCounter
	countCrossingsCausedByPortSwitch bool
}

// NewSwitchDecider is new SwitchDecider(freeLayerIndex, graph,
// crossingMatrixFiller, portPositions, graphData, oneSided).
func NewSwitchDecider(freeLayerIndex int, graph [][]*lgraph.LNode, crossingMatrixFiller *CrossingMatrixFiller,
	portPositions []int, graphData GraphData, oneSided bool) *SwitchDecider {
	d := &SwitchDecider{crossingMatrixFiller: crossingMatrixFiller, graphData: graphData}
	if freeLayerIndex >= len(graph) {
		lgraph.Bail("Greedy SwitchDecider: Free layer not in graph.")
	}
	d.freeLayer = graph[freeLayerIndex]

	d.leftInLayerCounter = counting.NewCrossingsCounter(portPositions)
	d.leftInLayerCounter.InitPortPositionsForInLayerCrossings(d.freeLayer, core.PortSideWest)
	d.rightInLayerCounter = counting.NewCrossingsCounter(portPositions)
	d.rightInLayerCounter.InitPortPositionsForInLayerCrossings(d.freeLayer, core.PortSideEast)
	d.northSouthCounter = NewNorthSouthEdgeNeighbouringNodeCrossingsCounter(d.freeLayer)
	d.countCrossingsCausedByPortSwitch = !oneSided && graphData.HasParent() && !graphData.DontSweepInto() &&
		d.freeLayer[0].Type == lgraph.NodeTypeExternalPort
	if d.countCrossingsCausedByPortSwitch {
		d.initParentCrossingsCounters(freeLayerIndex, len(graph))
	}
	return d
}

func (d *SwitchDecider) initParentCrossingsCounters(freeLayerIndex, length int) {
	parentGraphData := d.graphData.ParentGraphData()
	parentNodeOrder := parentGraphData.CurrentNodeOrder()
	portPos := parentGraphData.PortPositions()
	d.parentCrossCounter = counting.NewCrossingsCounter(portPos)
	parentNodeLayerPos := d.graphData.Parent().Layer().ID
	var leftLayer []*lgraph.LNode
	if parentNodeLayerPos > 0 {
		leftLayer = parentNodeOrder[parentNodeLayerPos-1]
	}
	middleLayer := parentNodeOrder[parentNodeLayerPos]
	var rightLayer []*lgraph.LNode
	if parentNodeLayerPos < len(parentNodeOrder)-1 {
		rightLayer = parentNodeOrder[parentNodeLayerPos+1]
	}
	rightMostLayer := freeLayerIndex == length-1
	if rightMostLayer {
		d.parentCrossCounter.InitForCountingBetween(middleLayer, rightLayer)
	} else {
		d.parentCrossCounter.InitForCountingBetween(leftLayer, middleLayer)
	}
}

// NotifyOfSwitch is notifyOfSwitch(upperNode, lowerNode): it tells the
// in-layer counters that two nodes were switched.
func (d *SwitchDecider) NotifyOfSwitch(upperNode, lowerNode *lgraph.LNode) {
	d.leftInLayerCounter.SwitchNodes(upperNode, lowerNode, core.PortSideWest)
	d.rightInLayerCounter.SwitchNodes(upperNode, lowerNode, core.PortSideEast)
	if d.countCrossingsCausedByPortSwitch {
		upperPort := counting.OriginPort(upperNode)
		lowerPort := counting.OriginPort(lowerNode)
		d.parentCrossCounter.SwitchPorts(upperPort, lowerPort)
	}
}

// DoesSwitchReduceCrossings is doesSwitchReduceCrossings(upperNodeIndex,
// lowerNodeIndex): whether the switch of the nodes with the two indices
// reduces the number of crossings.
func (d *SwitchDecider) DoesSwitchReduceCrossings(upperNodeIndex, lowerNodeIndex int) bool {
	if d.constraintsPreventSwitch(upperNodeIndex, lowerNodeIndex) {
		return false
	}

	upperNode := d.freeLayer[upperNodeIndex]
	lowerNode := d.freeLayer[lowerNodeIndex]

	leftInlayerFirst, leftInlayerSecond :=
		d.leftInLayerCounter.CountInLayerCrossingsBetweenNodesInBothOrders(upperNode, lowerNode, core.PortSideWest)
	rightInlayerFirst, rightInlayerSecond :=
		d.rightInLayerCounter.CountInLayerCrossingsBetweenNodesInBothOrders(upperNode, lowerNode, core.PortSideEast)
	d.northSouthCounter.CountCrossings(upperNode, lowerNode)
	upperLowerCrossings := d.crossingMatrixFiller.GetCrossingMatrixEntry(upperNode, lowerNode) +
		leftInlayerFirst + rightInlayerFirst + d.northSouthCounter.UpperLowerCrossings()
	lowerUpperCrossings := d.crossingMatrixFiller.GetCrossingMatrixEntry(lowerNode, upperNode) +
		leftInlayerSecond + rightInlayerSecond + d.northSouthCounter.LowerUpperCrossings()

	if d.countCrossingsCausedByPortSwitch {
		upperPort := counting.OriginPort(upperNode)
		lowerPort := counting.OriginPort(lowerNode)
		first, second := d.parentCrossCounter.CountCrossingsBetweenPortsInBothOrders(upperPort, lowerPort)
		upperLowerCrossings += first
		lowerUpperCrossings += second
	}

	return upperLowerCrossings > lowerUpperCrossings
}

// constraintsPreventSwitch checks whether in-layer successor constraints or
// layout unit constraints prevent a switch, or whether the nodes are a
// normal node and a north south port dummy.
func (d *SwitchDecider) constraintsPreventSwitch(nodeIndex, lowerNodeIndex int) bool {
	upperNode := d.freeLayer[nodeIndex]
	lowerNode := d.freeLayer[lowerNodeIndex]

	return d.haveSuccessorConstraints(upperNode, lowerNode) ||
		d.haveLayoutUnitConstraints(upperNode, lowerNode) ||
		d.areNormalAndNorthSouthPortDummy(upperNode, lowerNode)
}

func (d *SwitchDecider) haveSuccessorConstraints(upperNode, lowerNode *lgraph.LNode) bool {
	// The read stores the empty default list in the node, as in Java.
	constraints := prop.Get(upperNode, iprops.InLayerSuccessorConstraints)
	for _, n := range constraints {
		if n == lowerNode {
			return true
		}
	}
	return false
}

func (d *SwitchDecider) haveLayoutUnitConstraints(upperNode, lowerNode *lgraph.LNode) bool {
	neitherNodeIsLongEdgeDummy := upperNode.Type != lgraph.NodeTypeLongEdge && lowerNode.Type != lgraph.NodeTypeLongEdge

	// If upperNode and lowerNode are part of a layout unit not only
	// containing themselves, then the layout units must be equal for a
	// switch to be allowed.
	upperLayoutUnit := prop.Get(upperNode, iprops.InLayerLayoutUnit)
	lowerLayoutUnit := prop.Get(lowerNode, iprops.InLayerLayoutUnit)

	areInDifferentLayoutUnits := upperLayoutUnit != lowerLayoutUnit

	nodesHaveLayoutUnits := d.partOfMultiNodeLayoutUnit(upperNode, upperLayoutUnit) ||
		d.partOfMultiNodeLayoutUnit(lowerNode, lowerLayoutUnit)

	upperNodeHasNorthernEdges := d.hasEdgesOnSide(upperNode, core.PortSideNorth)
	lowerNodeHasSouthernEdges := d.hasEdgesOnSide(lowerNode, core.PortSideSouth)

	// hotfix for #162, if north or south edges are present, there must be a
	// layout unit
	// The Java operator |= evaluates its right side in each case.
	hasNorthSouthEdges := d.hasEdgesOnSide(upperNode, core.PortSideSouth) || d.hasEdgesOnSide(lowerNode, core.PortSideNorth)
	nodesHaveLayoutUnits = nodesHaveLayoutUnits || hasNorthSouthEdges

	hasLayoutUnitConstraint := (nodesHaveLayoutUnits && areInDifferentLayoutUnits) ||
		(upperNodeHasNorthernEdges || lowerNodeHasSouthernEdges)

	return neitherNodeIsLongEdgeDummy && hasLayoutUnitConstraint
}

func (d *SwitchDecider) hasEdgesOnSide(node *lgraph.LNode, side core.PortSide) bool {
	ports := node.PortSideView(side)
	for _, port := range ports {
		if prop.Get(port, iprops.PortDummy) != nil || port.Degree() != 0 {
			return true
		}
	}
	return false
}

func (d *SwitchDecider) partOfMultiNodeLayoutUnit(node, layoutUnit *lgraph.LNode) bool {
	return layoutUnit != nil && layoutUnit != node
}

func (d *SwitchDecider) areNormalAndNorthSouthPortDummy(upperNode, lowerNode *lgraph.LNode) bool {
	return d.isNorthSouthPortNode(upperNode) && d.isNormalNode(lowerNode) ||
		d.isNorthSouthPortNode(lowerNode) && d.isNormalNode(upperNode)
}

func (d *SwitchDecider) isNormalNode(node *lgraph.LNode) bool {
	return node.Type == lgraph.NodeTypeNormal
}

func (d *SwitchDecider) isNorthSouthPortNode(node *lgraph.LNode) bool {
	return node.Type == lgraph.NodeTypeNorthSouthPort
}
