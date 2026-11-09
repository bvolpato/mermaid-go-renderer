// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/edgeLabelNodes.ts (MIT).

package swimlanes

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// createEdgeLabelNodes transforms edges with labels into label nodes plus
// layout-only virtual edges (the label-as-waypoint variant).
//
// For each edge with a label it
//  1. creates a label node with the label text,
//  2. assigns the label node to the source or target lane (cross-lane
//     edges prefer the target lane for tighter routing),
//  3. stamps labelNodeId on the original edge,
//  4. appends two layout-only virtual edges (A→label, label→B), so that
//     Sugiyama places the label between source and target. The router
//     skips these; only the original edge is routed.
//
// It returns the new node and edge lists. The edges of data are changed in
// place.
func createEdgeLabelNodes(data *ru.LayoutData) (newNodes []*ru.Node, newEdges []*ru.Edge) {
	var nodesToAdd []*ru.Node
	var layoutOnlyEdges []*ru.Edge

	nodeById := make(map[string]*ru.Node, len(data.Nodes))
	for _, node := range data.Nodes {
		nodeById[node.ID] = node
	}

	for _, edge := range data.Edges {
		if edge.Label == "" {
			continue
		}
		if edge.IsLayoutOnly {
			continue
		}
		// Guard against double-processing if the caller invokes us twice.
		if edge.LabelNodeID != "" {
			continue
		}

		var sourceNode, targetNode *ru.Node
		if edge.Start != "" {
			sourceNode = nodeById[edge.Start]
		}
		if edge.End != "" {
			targetNode = nodeById[edge.End]
		}

		if sourceNode == nil || targetNode == nil {
			// log.warn: the edge has a missing source or target node.
			continue
		}

		labelNodeId := "edge-label-" + edge.Start + "-" + edge.End + "-" + edge.ID

		// For cross-lane edges, assign to the target lane for better routing:
		// it keeps the label closer to where the edge is heading and avoids
		// long detours back to the source lane.
		isCrossLane := sourceNode.ParentID != targetNode.ParentID
		labelLane := sourceNode.ParentID
		if isCrossLane {
			labelLane = targetNode.ParentID
		}

		labelNode := ru.NewNode(labelNodeId)
		labelNode.Label, labelNode.HasLabel = edge.Label, true
		labelNode.EdgeStart = edge.Start
		labelNode.EdgeEnd = edge.End
		labelNode.Shape = "labelRect"
		// The size is set when the node is rendered.
		labelNode.Width = 0
		labelNode.Height = 0
		labelNode.IsEdgeLabel = true
		labelNode.IsDummy = true
		labelNode.ParentID = labelLane
		labelNode.IsGroup = false
		// Array.isArray(edge.labelStyle) ? edge.labelStyle[0] : (edge.labelStyle ?? '')
		if edge.LabelStyleIsText {
			labelNode.LabelStyle = edge.LabelStyleText
		} else if len(edge.LabelStyle) > 0 {
			labelNode.LabelStyle = edge.LabelStyle[0]
		} else {
			// The first element of an empty array is undefined: the label
			// group gets no style attribute. A flowchart edge always has an
			// array. An edge with no labelStyle at all has no array either,
			// and upstream gives its label node the style ''; ru.Edge cannot
			// tell the two apart.
			labelNode.LabelStyleUndefined = true
		}
		if sourceNode.Dir != "" {
			labelNode.Dir = sourceNode.Dir
		}

		nodesToAdd = append(nodesToAdd, labelNode)

		// Stamp the original edge so the router can decompose routing through
		// the label's center when producing a single polyline.
		edge.LabelNodeID = labelNodeId

		// Ownership of the label text moves to the label node. Clear the
		// label off the original edge so the edge renderer does not draw it a
		// second time alongside the label node's own text.
		edge.Label, edge.HasLabel = "", false
		edge.Text = ""

		// Layout-only virtual edges: Sugiyama uses these to place the label
		// node between source and target. They are not routed or rendered.
		toLabelVirtual := ru.NewEdge(edge.ID + "-to-label")
		toLabelVirtual.Start = edge.Start
		toLabelVirtual.End = labelNodeId
		toLabelVirtual.Type = "normal"
		toLabelVirtual.IsLayoutOnly = true
		fromLabelVirtual := ru.NewEdge(edge.ID + "-from-label")
		fromLabelVirtual.Start = labelNodeId
		fromLabelVirtual.End = edge.End
		fromLabelVirtual.Type = "normal"
		fromLabelVirtual.IsLayoutOnly = true

		layoutOnlyEdges = append(layoutOnlyEdges, toLabelVirtual, fromLabelVirtual)
	}

	newNodes = append(append([]*ru.Node{}, data.Nodes...), nodesToAdd...)
	newEdges = append(append([]*ru.Edge{}, data.Edges...), layoutOnlyEdges...)
	return newNodes, newEdges
}
