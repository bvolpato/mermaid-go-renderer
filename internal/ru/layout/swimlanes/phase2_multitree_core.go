// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/phase2.multitree.core.ts (MIT).

package swimlanes

// annotateMinimumLayers returns, for every node of the trees below nodes,
// the minimum layer in its subtree.
func annotateMinimumLayers(nodes []NodeId, children map[NodeId][]NodeId, rankOf map[NodeId]int) map[NodeId]int {
	minLayer := map[NodeId]int{}

	var annotate func(node NodeId)
	annotate = func(node NodeId) {
		minL := rankOr0(rankOf, node)
		childList := append([]NodeId{}, children[node]...)
		sortFunc(childList, compareByRankThenId(rankOf))
		for _, child := range childList {
			annotate(child)
			if childMin, ok := minLayer[child]; ok {
				minL = min(minL, childMin)
			}
		}
		minLayer[node] = minL
	}

	for _, node := range nodes {
		annotate(node)
	}

	return minLayer
}

func compareByRankThenId(rankOf map[NodeId]int) func(a, b NodeId) float64 {
	return func(a, b NodeId) float64 {
		ra := rankOr0(rankOf, a)
		rb := rankOr0(rankOf, b)
		if ra == rb {
			return localeCompare(a, b)
		}
		return float64(ra - rb)
	}
}

// pushToLayer is `if (!layers[layer]) layers[layer] = [];
// layers[layer].push(node)`: it grows the list for a layer beyond its end.
// A negative layer is a property of the array object upstream that the
// result does not have; here its nodes are dropped.
func pushToLayer(layers [][]NodeId, layer int, node NodeId) [][]NodeId {
	if layer < 0 {
		return layers
	}
	for len(layers) <= layer {
		layers = append(layers, []NodeId{})
	}
	layers[layer] = append(layers[layer], node)
	return layers
}

// emitNodesInTreeOrder emits nodes into layers in tree traversal order,
// using orderChildren to determine the order of children.
func emitNodesInTreeOrder(roots []NodeId, allNodes []NodeId, rankOf map[NodeId]int, orderChildren func(node NodeId) []NodeId) [][]NodeId {
	maxRank := 0
	for _, node := range allNodes {
		r := rankOr0(rankOf, node)
		if r > maxRank {
			maxRank = r
		}
	}

	layers := make([][]NodeId, maxRank+1)
	for i := range layers {
		layers[i] = []NodeId{}
	}
	emitted := map[NodeId]bool{}

	var emit func(node NodeId)
	emit = func(node NodeId) {
		if emitted[node] {
			return
		}
		emitted[node] = true
		layer := rankOr0(rankOf, node)
		layers = pushToLayer(layers, layer, node)
		for _, child := range orderChildren(node) {
			emit(child)
		}
	}

	for _, root := range roots {
		emit(root)
	}

	// Fallback for isolated nodes (if any were not connected through the
	// spanning forest).
	for _, node := range allNodes {
		if !emitted[node] {
			layer := rankOr0(rankOf, node)
			layers = pushToLayer(layers, layer, node)
			emitted[node] = true
		}
	}

	return layers
}

// deduplicateLayers removes duplicate nodes from each layer while
// preserving order.
func deduplicateLayers(layers [][]NodeId) [][]NodeId {
	result := make([][]NodeId, 0, len(layers))
	for _, layer := range layers {
		seen := map[NodeId]bool{}
		deduped := []NodeId{}
		for _, id := range layer {
			if seen[id] {
				continue
			}
			seen[id] = true
			deduped = append(deduped, id)
		}
		result = append(result, deduped)
	}
	return result
}
