// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/order/sort-subgraph.js
// (MIT).

package dagre

import "github.com/bvolpato/mermaid-go-renderer/internal/graphlib"

func (l *layout) sortSubgraph(g *graphlib.Graph, v string, cg *graphlib.Graph, biasRight bool) *sortResult {
	movable := g.Children(v)
	// The label is undefined for the root of the layer graph. It is
	// {borderLeft, borderRight} for a subgraph.
	bl, br := "", ""
	switch node := g.Node(v).(type) {
	case *layerSubgraphLabel:
		bl = node.borderLeft
		br = node.borderRight
	case *nodeLabel:
		// The label of the layout graph: its borderLeft is an array. The
		// layer graph has this label only for a subgraph that is outside of
		// the rank, which a valid ranking does not produce.
		if node.borderLeft != nil {
			panic(typeError("layer graph: subgraph " + v + " is outside of its rank range"))
		}
	}
	subgraphs := map[string]*sortResult{}

	if bl != "" {
		filtered := []string{}
		for _, w := range movable {
			if w != bl && w != br {
				filtered = append(filtered, w)
			}
		}
		movable = filtered
	}

	barycenters := barycenter(g, movable)
	for _, entry := range barycenters {
		if len(g.Children(entry.v)) > 0 {
			subgraphResult := l.sortSubgraph(g, entry.v, cg, biasRight)
			subgraphs[entry.v] = subgraphResult
			if subgraphResult.hasBarycenter {
				mergeBarycenters(entry, subgraphResult)
			}
		}
	}

	entries := resolveConflicts(barycenters, cg)
	expandSubgraphs(entries, subgraphs)

	result := l.sort(entries, biasRight)

	if bl != "" {
		vs := make([]string, 0, len(result.vs)+2)
		vs = append(vs, bl)
		vs = append(vs, result.vs...)
		vs = append(vs, br)
		result.vs = vs
		if len(g.Predecessors(bl)) > 0 {
			blPred := layerNodeOf(g, g.Predecessors(bl)[0])
			brPred := layerNodeOf(g, g.Predecessors(br)[0])
			if !result.hasBarycenter {
				result.barycenter = 0
				result.weight = 0
				result.hasBarycenter = true
			}
			result.barycenter =
				(float64(result.barycenter*result.weight) + blPred.order + brPred.order) / (result.weight + 2)
			result.weight += 2
		}
	}

	return result
}

func expandSubgraphs(entries []*conflictEntry, subgraphs map[string]*sortResult) {
	for _, entry := range entries {
		vs := make([]string, 0, len(entry.vs))
		for _, v := range entry.vs {
			if sub, ok := subgraphs[v]; ok {
				vs = append(vs, sub.vs...)
			} else if isObjectPrototypeKey(v) {
				// subgraphs is a plain object. subgraphs[v] is an inherited
				// function for this id, and its vs property is undefined.
				// The undefined element makes the caller throw: it reads
				// the label of the node undefined.
				panic(typeError("Cannot set properties of undefined (setting 'order')"))
			} else {
				vs = append(vs, v)
			}
		}
		entry.vs = vs
	}
}

func mergeBarycenters(target *barycenterEntry, other *sortResult) {
	if target.hasBarycenter {
		target.barycenter =
			(float64(target.barycenter*target.weight) + float64(other.barycenter*other.weight)) /
				(target.weight + other.weight)
		target.weight += other.weight
	} else {
		target.barycenter = other.barycenter
		target.weight = other.weight
		target.hasBarycenter = true
	}
}
