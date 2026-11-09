// Ported from Eclipse ELK 0.9.1,
// plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/graph/transform/ElkGraphImporter.java
// (createLGraph and calculateMinimumGraphSize) (EPL-2.0): the calls of
// NodeLabelAndSizeCalculator that ElkGraphImporter makes for a node of the
// ELK graph.

// Package elknodespacing connects the importer of the layered algorithm to
// the node spacing code. Linking the package sets layered.NodeSpacing.
package elknodespacing

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/elkadapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/elkgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/layered"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing"
)

func init() {
	layered.NodeSpacing = spacing{}
}

type spacing struct{}

// parentAdapter is ElkGraphAdapters.adapt(node.getParent()), or the nil
// interface for a node without a parent.
func parentAdapter(node *elkgraph.Node) adapters.GraphAdapter {
	if node.Parent == nil {
		return nil
	}
	return elkadapters.Adapt(node.Parent)
}

// ComputeInsideNodeLabelPadding is the call of createLGraph.
func (spacing) ComputeInsideNodeLabelPadding(node *elkgraph.Node) *emath.ElkPadding {
	return nodespacing.ComputeInsideNodeLabelPadding(parentAdapter(node), elkadapters.AdaptSingleNode(node), core.DirectionRight)
}

// MinimumSize is the call of calculateMinimumGraphSize. The node has a
// parent there.
func (spacing) MinimumSize(node *elkgraph.Node) *emath.KVector {
	return nodespacing.ProcessNode(elkadapters.Adapt(node.Parent), elkadapters.AdaptSingleNode(node), false, true)
}
