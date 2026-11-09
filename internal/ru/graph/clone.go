// Ported from mermaid 12.2.0, rendering-util/cloneLayoutDataForMeasure.ts
// (MIT).

package graph

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// CloneLayoutDataForDomMeasure is cloneLayoutDataForDomMeasure(layout): a
// copy of the layout data for a measurement pass. The nodes and edges are
// shallow copies, the nodes without their Intersect and CalcIntersect
// functions, and the config is a shallow copy.
func CloneLayoutDataForDomMeasure(layout *ru.LayoutData) *ru.LayoutData {
	nodes := make([]*ru.Node, len(layout.Nodes))
	for i, node := range layout.Nodes {
		rest := *node
		rest.Intersect = nil
		rest.CalcIntersect = nil
		nodes[i] = &rest
	}
	edges := make([]*ru.Edge, len(layout.Edges))
	for i, e := range layout.Edges {
		c := *e
		edges[i] = &c
	}
	// { ...layout.config }: a copy of the top level.
	var config *js.Obj[any]
	if layout.Config != nil {
		config = js.NewObj[any]()
		layout.Config.Each(func(k string, v any) { config.Set(k, v) })
	}
	out := *layout
	out.Nodes = nodes
	out.Edges = edges
	out.Config = config
	return &out
}

// CopyMeasuredGraphOntoCanonical is copyMeasuredGraphOntoCanonical(canonical,
// measured): it puts the measured nodes and edges into canonical. The
// config of canonical stays as it is.
func CopyMeasuredGraphOntoCanonical(canonical, measured *ru.LayoutData) {
	canonical.Nodes = measured.Nodes
	canonical.Edges = measured.Edges
}
