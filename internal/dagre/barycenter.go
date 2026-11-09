// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/order/barycenter.js
// (MIT).

package dagre

import "github.com/bvolpato/mermaid-go-renderer/internal/graphlib"

// barycenterEntry is {v, barycenter, weight}. hasBarycenter is false for
// the form {v}.
type barycenterEntry struct {
	v             string
	barycenter    float64
	weight        float64
	hasBarycenter bool
}

func barycenter(g *graphlib.Graph, movable []string) []*barycenterEntry {
	result := make([]*barycenterEntry, len(movable))
	for i, v := range movable {
		inV := g.InEdges(v)
		if len(inV) == 0 {
			if inV == nil {
				// g.inEdges(v) is undefined for a node that is not in the graph.
				panic(typeError("Cannot read properties of undefined (reading 'length')"))
			}
			result[i] = &barycenterEntry{v: v, barycenter: nan, weight: nan}
		} else {
			sum := 0.0
			weight := 0.0
			for _, e := range inV {
				edge := g.EdgeObj(e).(*layerEdge)
				nodeU := layerNodeOf(g, e.V)
				sum = sum + float64(edge.weight*nodeU.order)
				weight = weight + edge.weight
			}

			result[i] = &barycenterEntry{
				v:             v,
				barycenter:    sum / weight,
				weight:        weight,
				hasBarycenter: true,
			}
		}
	}
	return result
}
