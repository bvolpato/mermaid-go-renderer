// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/LayerSweepTypeDecider.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p3order

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p3order/counting"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// LayerSweepTypeDecider is the class LayerSweepTypeDecider. To decide
// whether to sweep into a graph or not, it compares the number of paths to
// nodes whose position is decided by random decisions with the number of
// paths to nodes whose position depends on cross-hierarchy edges. The value
// (pathsToRandom - pathsToHierarchical) / allPaths is between -1 (many
// cross-hierarchical paths) and +1 (many random paths). The boundary
// CROSSING_MINIMIZATION_HIERARCHICAL_SWEEPINESS sets how likely the graph is
// handled hierarchically or bottom up.
//
// It must be initialized with counting.Init.
type LayerSweepTypeDecider struct {
	counting.DefaultInit

	nodeInfo  [][]*nodeInfo
	graphData *GraphInfoHolder
}

// NewLayerSweepTypeDecider is new LayerSweepTypeDecider(graphData).
func NewLayerSweepTypeDecider(graphData *GraphInfoHolder) *LayerSweepTypeDecider {
	return &LayerSweepTypeDecider{
		graphData: graphData,
		nodeInfo:  make([][]*nodeInfo, len(graphData.CurrentNodeOrder())),
	}
}

// UseBottomUp is useBottomUp(): it decides whether to use the bottom up or
// the cross-hierarchical sweep method.
func (d *LayerSweepTypeDecider) UseBottomUp() bool {
	boundary := prop.Get(d.graphData.LGraph(), lopts.CrossingMinimizationHierarchicalSweepiness)
	if d.bottomUpForced(boundary) || d.rootNode() || d.fixedPortOrder() || d.fewerThanTwoInOutEdges() {
		return true
	}

	if d.graphData.CrossMinDeterministic() {
		// The greedy switch would sweep into this nested graph. The pipeline
		// of elkjs does not get here: a nested graph has no greedy switch
		// processor of its own, so it is laid out completely before the
		// greedy switch of the root graph runs, and its parent node has
		// fixed port positions then. No golden covers a greedy switch that
		// sweeps into a nested graph, so the layout stops here.
		lgraph.Bail("LayerSweepTypeDecider: a greedy switch that sweeps into a nested graph has no golden")
		return false
	}

	pathsToRandom := 0
	pathsToHierarchical := 0

	var nsPortDummies []*lgraph.LNode
	for _, layer := range d.graphData.CurrentNodeOrder() {
		for _, node := range layer {
			// We must visit all sources of edges first, so we collect north
			// south dummies for later.
			if d.isNorthSouthDummy(node) {
				nsPortDummies = append(nsPortDummies, node)
				continue
			}

			currentNode := d.nodeInfoFor(node)
			// Check for hierarchical port dummies or random influence.
			if d.isExternalPortDummy(node) {
				currentNode.hierarchicalInfluence = 1
				if d.isEasternDummy(node) {
					pathsToHierarchical += currentNode.connectedEdges
				}
			} else if d.hasNoWesternPorts(node) {
				currentNode.randomInfluence = 1
			} else if d.hasNoEasternPorts(node) {
				pathsToRandom += currentNode.connectedEdges
			}

			// Increase counts of paths by the number outgoing edges times
			// the influence and transfer information to targets.
			for _, edge := range node.OutgoingEdges() {
				pathsToRandom += currentNode.randomInfluence
				pathsToHierarchical += currentNode.hierarchicalInfluence
				d.transferInfoToTarget(currentNode, edge)
			}

			// Do the same for north/south dummies: Increase counts of paths
			// by the number outgoing edges times the influence and transfer
			// information to dummies.
			northPorts := node.PortSideView(core.PortSideNorth)
			southPorts := node.PortSideView(core.PortSideSouth)
			for _, ports := range [2][]*lgraph.LPort{northPorts, southPorts} {
				for _, port := range ports {
					nsDummy := prop.Get(port, iprops.PortDummy)
					if nsDummy != nil {
						pathsToRandom += currentNode.randomInfluence
						pathsToHierarchical += currentNode.hierarchicalInfluence
						d.transferInfoTo(currentNode, nsDummy)
					}
				}
			}
		}

		// Now process nsPortDummies
		for _, node := range nsPortDummies {
			currentNode := d.nodeInfoFor(node)
			for _, edge := range node.OutgoingEdges() {
				pathsToRandom += currentNode.randomInfluence
				pathsToHierarchical += currentNode.hierarchicalInfluence
				d.transferInfoToTarget(currentNode, edge)
			}
		}
		nsPortDummies = nsPortDummies[:0]
	}

	allPaths := float64(pathsToRandom + pathsToHierarchical)
	normalized := math.Inf(1)
	if allPaths != 0 {
		normalized = float64(pathsToRandom-pathsToHierarchical) / allPaths
	}
	return normalized >= boundary
}

func (d *LayerSweepTypeDecider) fixedPortOrder() bool {
	return prop.Get(d.graphData.Parent(), lopts.PortConstraints).IsOrderFixed()
}

func (d *LayerSweepTypeDecider) transferInfoToTarget(currentNode *nodeInfo, edge *lgraph.LEdge) {
	target := d.targetNode(edge)
	d.transferInfoTo(currentNode, target)
}

func (d *LayerSweepTypeDecider) transferInfoTo(currentNode *nodeInfo, target *lgraph.LNode) {
	targetNodeInfo := d.nodeInfoFor(target)
	targetNodeInfo.transfer(currentNode)
	targetNodeInfo.connectedEdges++
}

// nodeInfo is the class LayerSweepTypeDecider.NodeInfo: for a single node,
// the number of paths to nodes with random or hierarchical influence.
type nodeInfo struct {
	connectedEdges        int
	hierarchicalInfluence int
	randomInfluence       int
}

func (i *nodeInfo) transfer(other *nodeInfo) {
	i.hierarchicalInfluence += other.hierarchicalInfluence
	i.randomInfluence += other.randomInfluence
	i.connectedEdges += other.connectedEdges
}

func (d *LayerSweepTypeDecider) fewerThanTwoInOutEdges() bool {
	return len(d.graphData.Parent().PortSideView(core.PortSideEast)) < 2 &&
		len(d.graphData.Parent().PortSideView(core.PortSideWest)) < 2
}

func (d *LayerSweepTypeDecider) rootNode() bool {
	return !d.graphData.HasParent()
}

func (d *LayerSweepTypeDecider) bottomUpForced(boundary float64) bool {
	return boundary < -1
}

func (d *LayerSweepTypeDecider) targetNode(edge *lgraph.LEdge) *lgraph.LNode {
	return edge.Target.Node()
}

func anyConnectedEdges(ports []*lgraph.LPort) bool {
	for _, p := range ports {
		if p.Degree() != 0 {
			return true
		}
	}
	return false
}

func (d *LayerSweepTypeDecider) hasNoEasternPorts(node *lgraph.LNode) bool {
	eastPorts := node.PortSideView(core.PortSideEast)
	return len(eastPorts) == 0 || !anyConnectedEdges(eastPorts)
}

func (d *LayerSweepTypeDecider) hasNoWesternPorts(node *lgraph.LNode) bool {
	westPorts := node.PortSideView(core.PortSideWest)
	return len(westPorts) == 0 || !anyConnectedEdges(westPorts)
}

func (d *LayerSweepTypeDecider) isExternalPortDummy(node *lgraph.LNode) bool {
	return node.Type == lgraph.NodeTypeExternalPort
}

func (d *LayerSweepTypeDecider) isNorthSouthDummy(node *lgraph.LNode) bool {
	return node.Type == lgraph.NodeTypeNorthSouthPort
}

func (d *LayerSweepTypeDecider) isEasternDummy(node *lgraph.LNode) bool {
	return counting.OriginPort(node).Side == core.PortSideEast
}

func (d *LayerSweepTypeDecider) nodeInfoFor(n *lgraph.LNode) *nodeInfo {
	return d.nodeInfo[n.Layer().ID][n.ID]
}

// InitAtLayerLevel is initAtLayerLevel(l, nodeOrder). It sets the id of the
// layer.
func (d *LayerSweepTypeDecider) InitAtLayerLevel(l int, nodeOrder [][]*lgraph.LNode) {
	nodeOrder[l][0].Layer().ID = l
	d.nodeInfo[l] = make([]*nodeInfo, len(nodeOrder[l]))
}

// InitAtNodeLevel is initAtNodeLevel(l, n, nodeOrder). It sets the id of
// the node.
func (d *LayerSweepTypeDecider) InitAtNodeLevel(l, n int, nodeOrder [][]*lgraph.LNode) {
	node := nodeOrder[l][n]
	node.ID = n
	d.nodeInfo[l][n] = &nodeInfo{}
}
