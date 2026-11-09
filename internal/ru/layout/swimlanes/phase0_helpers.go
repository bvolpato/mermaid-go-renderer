// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/phase0.helpers.ts (MIT).
//
// outgoing and isAcyclic of upstream are not ported: nothing in the layout
// calls them, and the build of mermaid does not contain them.

package swimlanes

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// normalizeGraph validates a graph view: it drops the edges with an unknown
// end and the duplicates of an edge (same id and ends).
func normalizeGraph(g *Graph) *Graph {
	nodeById := js.NewMap[NodeId, *Node]()
	g.NodeById.Each(func(k NodeId, v *Node) { nodeById.Set(k, v) })
	// Filter edges with unknown endpoints and de-duplicate by id+endpoints.
	seen := map[string]bool{}
	edges := []*EdgeRef{}
	for _, e := range g.Edges {
		if !nodeById.Has(e.Src) || !nodeById.Has(e.Dst) {
			continue
		}
		key := e.ID + ":" + e.Src + "->" + e.Dst
		if seen[key] {
			continue
		}
		seen[key] = true
		edges = append(edges, e)
	}
	nodes := nodeById.Keys()
	return &Graph{Nodes: nodes, Edges: edges, Layout: g.Layout, NodeById: nodeById}
}

// incoming returns the incoming edges of v.
func incoming(g *Graph, v NodeId) []*EdgeRef {
	var out []*EdgeRef
	for _, e := range g.Edges {
		if e.Dst == v {
			out = append(out, e)
		}
	}
	return out
}

// buildSuccessorMap returns the successors of every node, in edge order.
// The graph must be normalized: every end of an edge is a node.
func buildSuccessorMap(g *Graph) map[NodeId][]NodeId {
	succs := make(map[NodeId][]NodeId, len(g.Nodes))
	for _, v := range g.Nodes {
		succs[v] = []NodeId{}
	}
	for _, e := range g.Edges {
		succs[e.Src] = append(succs[e.Src], e.Dst)
	}
	return succs
}

func buildSortedSuccessorMap(g *Graph) map[NodeId][]NodeId {
	succs := buildSuccessorMap(g)
	for _, successors := range succs {
		sortFunc(successors, func(a, b NodeId) float64 { return localeCompare(a, b) })
	}
	return succs
}

// buildInDegreeMap returns the in-degree of every node. The keys are in
// the order of g.Nodes.
func buildInDegreeMap(g *Graph) *js.Map[NodeId, int] {
	indeg := js.NewMap[NodeId, int]()
	for _, v := range g.Nodes {
		indeg.Set(v, 0)
	}
	for _, e := range g.Edges {
		indeg.Set(e.Dst, indeg.Get(e.Dst)+1)
	}
	return indeg
}

func sortedZeroInDegreeNodes(g *Graph, indeg *js.Map[NodeId, int]) []NodeId {
	sorted := []NodeId{}
	indeg.Each(func(id NodeId, degree int) {
		if degree == 0 {
			sorted = append(sorted, id)
		}
	})
	sortFunc(sorted, func(a, b NodeId) float64 { return localeCompare(a, b) })

	// Nodes without any edge are unconstrained, so sorting them by ID is
	// arbitrary and reorders a lane of unconnected nodes alphabetically
	// (#8355). Refill their slots in source order (`indeg` is keyed in
	// `g.nodes` order) and leave every other node in place.
	linked := map[NodeId]bool{}
	for _, e := range g.Edges {
		linked[e.Src] = true
		linked[e.Dst] = true
	}
	isUnconnected := func(id NodeId) bool {
		if linked[id] {
			return false
		}
		node := g.NodeById.Get(id)
		return !(node != nil && node.IsGroup)
	}
	unconnected := []NodeId{}
	for _, id := range indeg.Keys() {
		if indeg.Get(id) == 0 && isUnconnected(id) {
			unconnected = append(unconnected, id)
		}
	}
	next := 0
	out := make([]NodeId, len(sorted))
	for i, id := range sorted {
		if isUnconnected(id) {
			out[i] = unconnected[next]
			next++
		} else {
			out[i] = id
		}
	}
	return out
}

// buildPredecessorSuccessorMaps returns the predecessors and successors of
// every node over the edges that includeEdge accepts. A nil includeEdge
// accepts every edge.
func buildPredecessorSuccessorMaps(g *Graph, includeEdge func(edge *EdgeRef) bool) (preds, succs map[NodeId][]NodeId) {
	preds = make(map[NodeId][]NodeId, len(g.Nodes))
	succs = make(map[NodeId][]NodeId, len(g.Nodes))
	for _, v := range g.Nodes {
		preds[v] = []NodeId{}
		succs[v] = []NodeId{}
	}
	for _, e := range g.Edges {
		if includeEdge != nil && !includeEdge(e) {
			continue
		}
		succs[e.Src] = append(succs[e.Src], e.Dst)
		preds[e.Dst] = append(preds[e.Dst], e.Src)
	}
	return preds, succs
}

// rankOr0 is `rankOf[v] ?? 0`.
func rankOr0(rankOf map[NodeId]int, v NodeId) int {
	return rankOf[v]
}

func buildLayersFromRanks(g *Graph, order []NodeId, rankOf map[NodeId]int, skipGroups bool) [][]NodeId {
	isGroup := func(v NodeId) bool {
		node := g.NodeById.Get(v)
		return node != nil && node.IsGroup
	}
	maxRank := 0
	for _, v := range g.Nodes {
		if skipGroups && isGroup(v) {
			continue
		}
		maxRank = max(maxRank, rankOr0(rankOf, v))
	}

	layers := make([][]NodeId, maxRank+1)
	for i := range layers {
		layers[i] = []NodeId{}
	}
	for _, v := range order {
		if skipGroups && isGroup(v) {
			continue
		}
		at := max(0, rankOr0(rankOf, v))
		layers[at] = append(layers[at], v)
	}
	return layers
}

// topoSortIfAcyclic is a topological sort (Kahn). It returns nil when the
// graph has a cycle.
func topoSortIfAcyclic(g *Graph) []NodeId {
	indeg := buildInDegreeMap(g)
	queue := sortedZeroInDegreeNodes(g, indeg)
	order := []NodeId{}
	adj := buildSortedSuccessorMap(g)

	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		order = append(order, u)
		for _, v := range adj[u] {
			indeg.Set(v, indeg.Get(v)-1)
			if indeg.Get(v) == 0 {
				// Insert keeping sorted order for determinism. The comparison
				// is the < of JavaScript strings.
				i := 0
				for i < len(queue) && js.CompareStrings(queue[i], v) < 0 {
					i++
				}
				next := make([]NodeId, 0, len(queue)+1)
				next = append(next, queue[:i]...)
				next = append(next, v)
				next = append(next, queue[i:]...)
				queue = next
			}
		}
	}

	if len(order) == len(g.Nodes) {
		return order
	}
	return nil
}

// buildLayerIndex returns the position of every node of a layer.
func buildLayerIndex(layer []NodeId) map[NodeId]int {
	m := make(map[NodeId]int, len(layer))
	index := 0
	for _, id := range layer {
		m[id] = index
		index++
	}
	return m
}

// countInversions counts the inversions of values with a merge sort. It
// sorts values in place.
func countInversions(values []int) int {
	tmp := make([]int, len(values))
	var count func(left, right int) int
	count = func(left, right int) int {
		if right-left <= 1 {
			return 0
		}
		mid := (left + right) >> 1
		inversions := count(left, mid) + count(mid, right)
		i := left
		j := mid
		k := left
		for i < mid || j < right {
			if j >= right || (i < mid && values[i] <= values[j]) {
				tmp[k] = values[i]
				k++
				i++
			} else {
				tmp[k] = values[j]
				k++
				j++
				inversions += mid - i
			}
		}
		for t := left; t < right; t++ {
			values[t] = tmp[t]
		}
		return inversions
	}
	return count(0, len(values))
}
