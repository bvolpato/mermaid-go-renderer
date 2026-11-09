// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/phase2.crossCounts.ts (MIT).

package swimlanes

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// computeSubtreeCrossCounts computes crossing counts for each parent/child
// pair in the driving tree.
//
// It uses a binary-lifting LCA over the driving tree and counts, for every
// edge, how many layers it spans below the LCA. These per-layer counts are
// then accumulated to obtain, for each parent -> child edge in the tree, an
// approximate "subtree crossing" score used when ordering children.
func computeSubtreeCrossCounts(g *Graph, rankOf map[NodeId]int, tree *DrivingTree) map[NodeId]map[NodeId]int {
	nodes := append([]NodeId{}, g.Nodes...)
	indexOf := make(map[NodeId]int, len(nodes))
	for i, node := range nodes {
		indexOf[node] = i
	}
	n := len(nodes)

	parentIdx := make([]int, n)
	depth := make([]int, n)
	for i := range parentIdx {
		parentIdx[i] = -1
	}

	queue := []NodeId{}
	seen := map[NodeId]bool{}

	// Seed BFS from roots.
	for _, node := range nodes {
		parentId := tree.Parent[node]
		idx := indexOf[node]
		if parentId == "" {
			parentIdx[idx] = -1
			depth[idx] = 0
			if !seen[node] {
				seen[node] = true
				queue = append(queue, node)
			}
		}
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		currentIdx, ok := indexOf[current]
		if !ok {
			continue
		}
		childList := tree.Children[current]
		for _, child := range childList {
			if seen[child] {
				continue
			}
			childIdx, ok := indexOf[child]
			if !ok {
				continue
			}
			parentIdx[childIdx] = currentIdx
			depth[childIdx] = depth[currentIdx] + 1
			seen[child] = true
			queue = append(queue, child)
		}
	}

	// Ensure all nodes are represented even if disconnected in the tree.
	for _, node := range nodes {
		if seen[node] {
			continue
		}
		idx := indexOf[node]
		parentIdx[idx] = -1
		depth[idx] = 0
		seen[node] = true
	}

	maxLog := max(1, int(math.Ceil(js.Log2(float64(max(1, n)))))+1)
	up := make([][]int, maxLog)
	for k := range up {
		up[k] = make([]int, n)
		for i := range up[k] {
			up[k][i] = -1
		}
	}
	for i := 0; i < n; i++ {
		up[0][i] = parentIdx[i]
	}
	for k := 1; k < maxLog; k++ {
		for i := 0; i < n; i++ {
			prev := up[k-1][i]
			if prev == -1 {
				up[k][i] = -1
			} else {
				up[k][i] = up[k-1][prev]
			}
		}
	}

	lcaIndex := func(aIdx, bIdx int) int {
		if aIdx == -1 || bIdx == -1 {
			return -1
		}
		if depth[aIdx] < depth[bIdx] {
			aIdx, bIdx = bIdx, aIdx
		}
		diff := depth[aIdx] - depth[bIdx]
		for k := 0; k < maxLog; k++ {
			if (diff>>k)&1 != 0 {
				aIdx = up[k][aIdx]
				if aIdx == -1 {
					return -1
				}
			}
		}
		if aIdx == bIdx {
			return aIdx
		}
		for k := maxLog - 1; k >= 0; k-- {
			upA := up[k][aIdx]
			upB := up[k][bIdx]
			if upA == -1 || upB == -1 {
				continue
			}
			if upA != upB {
				aIdx = upA
				bIdx = upB
			}
		}
		return up[0][aIdx]
	}

	ownCounts := make([]map[int]int, n)
	for i := range ownCounts {
		ownCounts[i] = map[int]int{}
	}

	// Count, for each LCA, how many edges pass through each layer below it.
	for _, edge := range g.Edges {
		src := edge.Src
		dst := edge.Dst
		ru, hasRu := rankOf[src]
		rv, hasRv := rankOf[dst]
		if !hasRu || !hasRv {
			continue
		}
		if ru > rv {
			src, dst = dst, src
			ru, rv = rv, ru
		}
		if ru == rv {
			continue
		}
		upperIdx, ok := indexOf[src]
		if !ok {
			continue
		}
		lowerIdx, ok := indexOf[dst]
		if !ok {
			continue
		}
		lca := lcaIndex(upperIdx, lowerIdx)
		if lca == -1 {
			continue
		}
		bucket := ownCounts[lca]
		for layer := ru; layer < rv; layer++ {
			bucket[layer] = bucket[layer] + 1
		}
	}

	crossCounts := map[NodeId]map[NodeId]int{}

	mergeInto := func(target, source map[int]int) {
		for layer, value := range source {
			target[layer] = target[layer] + value
		}
	}

	visited := map[NodeId]bool{}
	var dfs func(node NodeId) map[int]int
	dfs = func(node NodeId) map[int]int {
		idx, hasIdx := indexOf[node]
		visited[node] = true
		accumulator := map[int]int{}
		if hasIdx {
			for layer, value := range ownCounts[idx] {
				accumulator[layer] = value
			}
		}
		childList := tree.Children[node]
		for _, child := range childList {
			childMap := dfs(child)
			parentLayer, hasParentLayer := rankOf[node]
			if hasParentLayer {
				m := crossCounts[node]
				if m == nil {
					m = map[NodeId]int{}
					crossCounts[node] = m
				}
				value := childMap[parentLayer]
				childLayer, hasChildLayer := rankOf[child]
				if hasChildLayer && childLayer > parentLayer {
					value += 1
				}
				m[child] = value
			}
			mergeInto(accumulator, childMap)
		}
		return accumulator
	}

	for _, root := range tree.Roots {
		if !visited[root] {
			dfs(root)
		}
	}
	for _, node := range nodes {
		if !visited[node] {
			dfs(node)
		}
	}

	return crossCounts
}
