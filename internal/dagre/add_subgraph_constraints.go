// Ported from dagre-d3-es 7.0.14,
// dagre-d3-es/src/dagre/order/add-subgraph-constraints.js (MIT).

package dagre

import "github.com/bvolpato/mermaid-go-renderer/internal/graphlib"

func addSubgraphConstraints(g, cg *graphlib.Graph, vs []string) {
	prev := map[string]string{}
	rootPrev := ""

	for _, v := range vs {
		child, _ := g.Parent(v)
		for child != "" {
			var prevChild string
			parent, _ := g.Parent(child)
			if parent != "" {
				var ok bool
				prevChild, ok = prev[parent]
				if !ok && parent != "__proto__" {
					// prev is a plain object. For the name of an
					// Object.prototype property, the read gives the
					// inherited function, and setEdge converts it to a
					// string.
					prevChild = objectPrototypeKeys[parent]
				}
				if parent == "__proto__" {
					// The assignment prev.__proto__ = child has no effect,
					// so the read always gives Object.prototype.
					prevChild = objectPrototypeKeys[parent]
				} else {
					prev[parent] = child
				}
			} else {
				prevChild = rootPrev
				rootPrev = child
			}
			if prevChild != "" && prevChild != child {
				setEdge(cg, prevChild, child, nil, "")
				break
			}
			child = parent
		}
	}
}
