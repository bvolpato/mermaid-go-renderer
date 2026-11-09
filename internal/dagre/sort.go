// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/order/sort.js (MIT).

package dagre

import "github.com/bvolpato/mermaid-go-renderer/internal/js"

// sortResult is {vs, barycenter, weight}. hasBarycenter is true when the
// properties barycenter and weight exist.
type sortResult struct {
	vs            []string
	barycenter    float64
	weight        float64
	hasBarycenter bool
}

func (l *layout) sort(entries []*conflictEntry, biasRight bool) *sortResult {
	// util.partition(entries, entry => has(entry, 'barycenter'))
	sortable := []*conflictEntry{}
	rhs := []*conflictEntry{}
	for _, entry := range entries {
		if entry.hasBarycenter {
			sortable = append(sortable, entry)
		} else {
			rhs = append(rhs, entry)
		}
	}
	unsortable := sortBy(l.state.Sort, rhs, func(entry *conflictEntry) sortKey {
		return numKey(-entry.i)
	})
	vs := [][]string{}
	sum := 0.0
	weight := 0.0
	vsIndex := 0.0

	js.SortV8(l.state.Sort, sortable, compareWithBias(biasRight))

	vsIndex = consumeUnsortable(&vs, &unsortable, vsIndex)

	for _, entry := range sortable {
		vsIndex += float64(len(entry.vs))
		vs = append(vs, entry.vs)
		sum += float64(entry.barycenter * entry.weight)
		weight += entry.weight
		vsIndex = consumeUnsortable(&vs, &unsortable, vsIndex)
	}

	result := &sortResult{vs: flatten(vs), barycenter: nan, weight: nan}
	if truthy(weight) {
		result.barycenter = sum / weight
		result.weight = weight
		result.hasBarycenter = true
	}
	return result
}

func consumeUnsortable(vs *[][]string, unsortable *[]*conflictEntry, index float64) float64 {
	for len(*unsortable) > 0 {
		last := (*unsortable)[len(*unsortable)-1]
		if !(last.i <= index) {
			break
		}
		*unsortable = (*unsortable)[:len(*unsortable)-1]
		*vs = append(*vs, last.vs)
		index++
	}
	return index
}

func compareWithBias(bias bool) func(entryV, entryW *conflictEntry) float64 {
	return func(entryV, entryW *conflictEntry) float64 {
		if entryV.barycenter < entryW.barycenter {
			return -1
		} else if entryV.barycenter > entryW.barycenter {
			return 1
		}

		if !bias {
			return entryV.i - entryW.i
		}
		return entryW.i - entryV.i
	}
}

// flatten is _.flatten for an array of arrays of node ids.
func flatten(arrays [][]string) []string {
	n := 0
	for _, a := range arrays {
		n += len(a)
	}
	out := make([]string, 0, n)
	for _, a := range arrays {
		out = append(out, a...)
	}
	return out
}
