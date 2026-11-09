// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/phase1.cycles.ts (MIT).

package swimlanes

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// CycleRemovalResult is the result of removeCycles_DFS. Reversed has the
// edges that were reversed, in their original orientation.
type CycleRemovalResult struct {
	Acyclic  *Graph
	Reversed []*EdgeRef
}

func edgeRefKey(e *EdgeRef) string {
	return e.ID + ":" + e.Src + "->" + e.Dst
}

// removeCycles_DFS is a deterministic DFS-based cycle removal (similar to
// the back-edge marking of Graphviz dot).
func removeCycles_DFS(g *Graph) *CycleRemovalResult {
	gn := normalizeGraph(g)

	// Build adjacency with deterministic order.
	adj := make(map[NodeId][]*EdgeRef, len(gn.Nodes))
	for _, v := range gn.Nodes {
		adj[v] = []*EdgeRef{}
	}
	for _, e := range gn.Edges {
		adj[e.Src] = append(adj[e.Src], e)
	}
	for _, arr := range adj {
		sortFunc(arr, func(a, b *EdgeRef) float64 {
			if a.Dst == b.Dst {
				return localeCompare(a.ID, b.ID)
			}
			return localeCompare(a.Dst, b.Dst)
		})
	}

	color := make(map[NodeId]int, len(gn.Nodes))
	for _, v := range gn.Nodes {
		color[v] = 0 // 0 white, 1 gray, 2 black
	}

	reversed := []*EdgeRef{}

	var dfs func(u NodeId)
	dfs = func(u NodeId) {
		color[u] = 1
		for _, e := range adj[u] {
			v := e.Dst
			if color[v] == 0 {
				dfs(v)
			} else if color[v] == 1 {
				// back-edge u->v; mark for reversal
				reversed = append(reversed, e)
			}
		}
		color[u] = 2
	}

	// Seed the DFS from true sources (in-degree 0) first, in declaration
	// order, then sweep any remaining unvisited nodes in declaration order.
	// This follows dot (Gansner et al. 1993, §2.1). Seeding from an arbitrary
	// root can reverse an edge that is not the natural back edge.
	// gn.Nodes keeps the insertion order of nodeById, which is parser order.
	inDegree := buildInDegreeMap(gn)
	seeds := make([]NodeId, 0, len(gn.Nodes))
	for _, v := range gn.Nodes {
		if inDegree.Get(v) == 0 {
			seeds = append(seeds, v)
		}
	}
	for _, v := range gn.Nodes {
		if inDegree.Get(v) != 0 {
			seeds = append(seeds, v)
		}
	}
	for _, v := range seeds {
		if color[v] == 0 {
			dfs(v)
		}
	}

	// Build the acyclic edge set by reversing all marked edges.
	toReverse := map[string]bool{}
	for _, e := range reversed {
		toReverse[edgeRefKey(e)] = true
	}
	acycEdges := make([]*EdgeRef, len(gn.Edges))
	for i, e := range gn.Edges {
		if toReverse[edgeRefKey(e)] {
			acycEdges[i] = &EdgeRef{ID: e.ID, Src: e.Dst, Dst: e.Src, Weight: e.Weight, Ref: e.Ref}
		} else {
			acycEdges[i] = e
		}
	}

	nodeById := js.NewMap[NodeId, *Node]()
	gn.NodeById.Each(func(k NodeId, v *Node) { nodeById.Set(k, v) })
	acyclic := &Graph{
		Nodes:    append([]NodeId{}, gn.Nodes...),
		Edges:    acycEdges,
		Layout:   gn.Layout,
		NodeById: nodeById,
	}
	return &CycleRemovalResult{Acyclic: acyclic, Reversed: reversed}
}
