// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/EndLabelSorter.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package labels

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/cellsystem"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("EndLabelSorter", func() lgraph.Processor { return NewEndLabelSorter() })
}

// EndLabelSorter is the processor EndLabelSorter: it sorts the end labels of
// each port by the order of the nodes that their edges come from or go to.
// Without it the order of the labels of a port with several edges looks
// random.
type EndLabelSorter struct{}

// NewEndLabelSorter is new EndLabelSorter().
func NewEndLabelSorter() *EndLabelSorter { return &EndLabelSorter{} }

// Process is process(layeredGraph, monitor).
func (p *EndLabelSorter) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	for _, layer := range layeredGraph.Layers {
		for _, node := range layer.Nodes {
			if node.Type == lgraph.NodeTypeNormal {
				p.processNode(node)
			}
		}
	}
}

// Node Processing and Initialization

func (p *EndLabelSorter) processNode(node *lgraph.LNode) {
	initializeMethodCalled := false

	if prop.Has(node, iprops.EndLabels) {
		labelCellMap := endLabelCellsOf(node)

		// Iterate over all ports and check for each port if it requires its
		// labels to be sorted
		for _, port := range node.Ports {
			if p.needsSorting(port) {
				// Check if we need to initialize
				if !initializeMethodCalled {
					p.initialize(graphOf(node))
					initializeMethodCalled = true
				}

				p.sort(port, labelCellMap.Get(port))
			}
		}
	}
}

// needsSorting tells that the end labels of a port must be sorted: the port
// has end labels of at least two edges.
func (*EndLabelSorter) needsSorting(port *lgraph.LPort) bool {
	edgesWithEndLabels := 0

	for _, inEdge := range port.IncomingEdges {
		if hasLabelWithPlacement(inEdge, core.EdgeLabelPlacementHead) {
			edgesWithEndLabels++
		}
	}

	for _, outEdge := range port.OutgoingEdges {
		if hasLabelWithPlacement(outEdge, core.EdgeLabelPlacementTail) {
			edgesWithEndLabels++
		}
	}

	return edgesWithEndLabels >= 2
}

// hasLabelWithPlacement tells that the edge has a label with the placement.
func hasLabelWithPlacement(edge *lgraph.LEdge, placement core.EdgeLabelPlacement) bool {
	for _, label := range edge.Labels {
		if prop.Get(label, lopts.EdgeLabelsPlacement) == placement {
			return true
		}
	}
	return false
}

// initialize runs when the processor finds the first labels to sort: it
// gives the nodes and the ports ascending ids.
func (*EndLabelSorter) initialize(lGraph *lgraph.LGraph) {
	// Give nodes and ports ascending IDs
	nextElementID := 0
	for _, layer := range lGraph.Layers {
		for _, node := range layer.Nodes {
			node.ID = nextElementID
			nextElementID++

			for _, port := range node.Ports {
				port.ID = nextElementID
				nextElementID++
			}
		}
	}
}

// Sorting

// sort sorts the labels of the port, which the label cell has.
func (p *EndLabelSorter) sort(port *lgraph.LPort, portLabelCell *cellsystem.LabelCell) {
	if portLabelCell == nil {
		lgraph.Bail("EndLabelSorter: a port with end labels of two edges has no label cell")
	}
	labelGroups := p.createLabelGroups(portLabelCell)
	gwt.SortFunc(labelGroups, compareLabelGroups)

	// Re-add the label cell's labels in the proper order. The cell keeps its
	// size.
	portLabelCellLabels := make([]adapters.LabelAdapter, 0, len(portLabelCell.Labels()))
	for _, group := range labelGroups {
		portLabelCellLabels = append(portLabelCellLabels, group.labels...)
	}
	portLabelCell.SetLabels(portLabelCellLabels)
}

// createLabelGroups makes the label groups of a label cell: one group for the
// labels of each edge.
func (*EndLabelSorter) createLabelGroups(portLabelCell *cellsystem.LabelCell) []*labelGroup {
	// The key is nil for a label without the property END_LABEL_EDGE.
	edgeToGroupMap := gwt.NewMap[*lgraph.LEdge, *labelGroup]()

	// Make sure every label is contained in a label group
	for _, label := range portLabelCell.Labels() {
		edge := prop.Get(label.Holder(), iprops.EndLabelEdge)

		// Create a new label group for the edge if we see it for the first
		// time. Note that this works even if the label didn't have an
		// END_LABEL_EDGE property set
		if !edgeToGroupMap.ContainsKey(edge) {
			edgeToGroupMap.Put(edge, newLabelGroup(edge))
		}

		group := edgeToGroupMap.Get(edge)
		group.labels = append(group.labels, label)
	}

	return edgeToGroupMap.Values()
}

// labelGroup is EndLabelSorter.LabelGroup: the labels of one edge, in an
// order that does not change. A group can be sorted by the source and the
// target of its edge.
type labelGroup struct {
	// edge is the edge of the labels. It can be a dummy edge if a label dummy
	// broke the original edge.
	edge *lgraph.LEdge
	// labels has the labels of the group.
	labels []adapters.LabelAdapter
}

// newLabelGroup is new LabelGroup(edge).
func newLabelGroup(edge *lgraph.LEdge) *labelGroup { return &labelGroup{edge: edge} }

// compareLabelGroups is LABEL_GROUP_COMPARATOR. Java fails for a group whose
// edge is null, and for an edge without the source or the target that the
// comparison reads.
func compareLabelGroups(group1, group2 *labelGroup) int {
	// If they are not connected to the same source port, use the difference
	// as a basis for the comparison
	sourcePortDiff := gwt.IntCompare(
		groupSource(group1).ID,
		groupSource(group2).ID)
	if sourcePortDiff != 0 {
		return sourcePortDiff
	}

	// They are connected to the same source port. Sort by target node, if
	// that makes a difference.
	targetNodeDiff := gwt.IntCompare(
		groupTargetNode(group1).ID,
		groupTargetNode(group2).ID)
	if targetNodeDiff != 0 {
		return targetNodeDiff
	}

	// They are connected to the same source port and to the same target
	// node. Compare target ports, but backwards. Since western ports are
	// ordered from bottom to top, not sorting backwards would yield the
	// opposite of our desires.
	return gwt.IntCompare(
		group2.edge.Target.ID,
		group1.edge.Target.ID)
}

// groupSource is group.edge.getSource().
func groupSource(group *labelGroup) *lgraph.LPort {
	if group.edge == nil || group.edge.Source == nil {
		lgraph.Bail("EndLabelSorter: an end label whose edge has no source")
	}
	return group.edge.Source
}

// groupTargetNode is group.edge.getTarget().getNode().
func groupTargetNode(group *labelGroup) *lgraph.LNode {
	if group.edge.Target == nil || group.edge.Target.Node() == nil {
		lgraph.Bail("EndLabelSorter: an end label whose edge has no target")
	}
	return group.edge.Target.Node()
}
