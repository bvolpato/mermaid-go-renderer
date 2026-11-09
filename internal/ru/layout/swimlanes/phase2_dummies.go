// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/phase2.dummies.ts (MIT).

package swimlanes

import (
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// makeProperLayering puts a dummy node on every layer that a long edge
// passes. It writes the ranks of the dummies into layering.RankOf, as
// upstream does.
func makeProperLayering(layering *Layering, gAcyclic *Graph) (*Layering, *Graph) {
	g := normalizeGraph(gAcyclic)
	rankOf := layering.RankOf
	layers := make([][]NodeId, len(layering.Layers))
	for i, l := range layering.Layers {
		layers[i] = append([]NodeId{}, l...)
	}
	dummy := js.NewSet[NodeId]()
	if layering.Dummy != nil {
		for _, id := range layering.Dummy.Values() {
			dummy.Add(id)
		}
	}

	// Helper to create a dummy node at layer L.
	dummySeq := 0
	nodeById := js.NewMap[NodeId, *Node]()
	g.NodeById.Each(func(k NodeId, v *Node) { nodeById.Set(k, v) })
	addDummyAt := func(L int) NodeId {
		id := "placeholder-" + strconv.Itoa(dummySeq)
		dummySeq++
		dn := ru.NewNode(id)
		dn.IsGroup = false
		dn.IsDummy = true
		dn.Width = 0
		dn.Height = 0
		nodeById.Set(id, dn)
		dummy.Add(id)
		// Ensure layers[L] exists.
		for len(layers) <= L {
			layers = append(layers, []NodeId{})
		}
		if L < 0 {
			// layers[L] is undefined for a negative L.
			panic(jsError("Cannot read properties of undefined (reading 'push')"))
		}
		layers[L] = append(layers[L], id)
		rankOf[id] = L
		return id
	}

	// Sort edges deterministically to stabilize dummy creation order.
	edgesSorted := append([]*EdgeRef{}, g.Edges...)
	sortFunc(edgesSorted, func(a, b *EdgeRef) float64 {
		if a.ID == b.ID {
			if a.Src == b.Src {
				return localeCompare(a.Dst, b.Dst)
			}
			return localeCompare(a.Src, b.Src)
		}
		return localeCompare(a.ID, b.ID)
	})

	newEdges := []*EdgeRef{}
	for _, e := range edgesSorted {
		rU := rankOr0(rankOf, e.Src)
		rV := rankOr0(rankOf, e.Dst)
		if rV-rU <= 1 {
			newEdges = append(newEdges, e)
			continue
		}
		// Need to insert dummies on intermediate layers rU+1..rV-1.
		prev := e.Src
		for L, k := rU+1, 0; L < rV; L, k = L+1, k+1 {
			d := addDummyAt(L)
			// chain prev -> d
			newEdges = append(newEdges, &EdgeRef{ID: e.ID + "#" + strconv.Itoa(k), Src: prev, Dst: d, Weight: e.Weight, Ref: e.Ref})
			prev = d
		}
		// last dummy (or src if no dummies) -> dst
		lastIndex := rV - rU - 2 // -2 because we added k from 0..(rV-rU-2)
		newEdges = append(newEdges, &EdgeRef{
			ID:     e.ID + "#" + strconv.Itoa(max(lastIndex+1, 0)),
			Src:    prev,
			Dst:    e.Dst,
			Weight: e.Weight,
			Ref:    e.Ref,
		})
	}

	// Build the new nodes list (preserve original order, then dummies by
	// creation order).
	inNodes := make(map[NodeId]bool, len(g.Nodes))
	for _, id := range g.Nodes {
		inNodes[id] = true
	}
	nodes := append([]NodeId{}, g.Nodes...)
	for _, id := range dummy.Values() {
		if !inNodes[id] {
			nodes = append(nodes, id)
		}
	}
	graphWithDummies := &Graph{Nodes: nodes, Edges: newEdges, Layout: g.Layout, NodeById: nodeById}

	return &Layering{Layers: layers, RankOf: rankOf, Dummy: dummy}, graphWithDummies
}
