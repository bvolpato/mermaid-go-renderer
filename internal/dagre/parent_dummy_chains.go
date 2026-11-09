// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/parent-dummy-chains.js
// (MIT).

package dagre

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
)

// pathNode is an element of a path through the node hierarchy: a node id,
// or undefined (ok is false) for the root.
type pathNode struct {
	v  string
	ok bool
}

type lowLim struct {
	low float64
	lim float64
}

func parentDummyChains(g *graphlib.Graph) {
	postorderNums := postorder(g)

	for _, v := range graphOf(g).dummyChains {
		node := nodeOf(g, v)
		edgeObj := node.edgeObj
		path, lca := findPath(g, postorderNums, edgeObj.V, edgeObj.W)
		pathIdx := 0
		pathV := pathAt(path, pathIdx)
		ascending := true

		for v != edgeObj.W {
			node = nodeOf(g, v)

			if ascending {
				for {
					pathV = pathAt(path, pathIdx)
					if pathV == lca || !(nodeOf(g, pathV.v).maxRank < node.rank) {
						break
					}
					pathIdx++
				}

				if pathV == lca {
					ascending = false
				}
			}

			if !ascending {
				for pathIdx < len(path)-1 {
					pathV = pathAt(path, pathIdx+1)
					if !(nodeOf(g, pathV.v).minRank <= node.rank) {
						break
					}
					pathIdx++
				}
				pathV = pathAt(path, pathIdx)
			}

			setParent(g, v, pathV.v, pathV.ok)
			v = g.Successors(v)[0]
		}
	}
}

// pathAt is path[i]. An index outside of the path gives undefined.
func pathAt(path []pathNode, i int) pathNode {
	if i < 0 || i >= len(path) {
		return pathNode{}
	}
	return path[i]
}

// findPath finds a path from v to w through the lowest common ancestor
// (LCA). It returns the full path and the LCA.
func findPath(g *graphlib.Graph, postorderNums map[string]*lowLim, v, w string) ([]pathNode, pathNode) {
	vPath := []pathNode{}
	wPath := []pathNode{}
	low := min(postorderNums[v].low, postorderNums[w].low)
	lim := max(postorderNums[v].lim, postorderNums[w].lim)

	// Traverse up from v to find the LCA.
	parent := pathNode{v, true}
	for {
		parent = parentOf(g, parent)
		vPath = append(vPath, parent)
		if !(parent.ok && parent.v != "" && (postorderNums[parent.v].low > low || lim > postorderNums[parent.v].lim)) {
			break
		}
	}
	lca := parent

	// Traverse from w to LCA.
	parent = pathNode{w, true}
	seenUndefined := false
	for {
		parent = parentOf(g, parent)
		if parent == lca {
			break
		}
		if !parent.ok {
			// The walk passed the root without reaching the LCA. This
			// happens when the LCA is the node with the empty id, which
			// the loop above takes for the root. From here the walk
			// repeats, so JavaScript pushes to wPath until the array is
			// too long.
			if seenUndefined {
				panic(typeError("Invalid array length"))
			}
			seenUndefined = true
		}
		wPath = append(wPath, parent)
	}

	for i := len(wPath) - 1; i >= 0; i-- {
		vPath = append(vPath, wPath[i])
	}
	return vPath, lca
}

// parentOf is g.parent(v) for a path node. JavaScript reads the parent of
// the node "undefined" when v is undefined.
func parentOf(g *graphlib.Graph, v pathNode) pathNode {
	key := v.v
	if !v.ok {
		key = "undefined"
	}
	p, ok := g.Parent(key)
	return pathNode{p, ok}
}

func postorder(g *graphlib.Graph) map[string]*lowLim {
	result := map[string]*lowLim{}
	lim := 0.0

	var dfs func(v string)
	dfs = func(v string) {
		low := lim
		for _, child := range g.Children(v) {
			dfs(child)
		}
		result[v] = &lowLim{low: low, lim: lim}
		lim++
	}
	for _, v := range g.Children() {
		dfs(v)
	}

	return result
}
