// Ported from dagre-d3-es 7.0.14,
// dagre-d3-es/src/dagre/order/resolve-conflicts.js (MIT).

package dagre

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// conflictEntry is an entry of resolveConflicts. The fields vs, i,
// barycenter and weight are the result. hasBarycenter is true when the
// properties barycenter and weight exist.
type conflictEntry struct {
	indegree float64
	in       []*conflictEntry
	out      []*conflictEntry
	vs       []string
	i        float64

	barycenter    float64
	weight        float64
	hasBarycenter bool

	merged bool
}

// resolveConflicts takes a list of entries of the form {v, barycenter,
// weight} and a constraint graph, and resolves any conflicts between the
// constraint graph and the barycenters of the entries. If the barycenters
// of an entry would violate a constraint in the constraint graph, the nodes
// in the conflict are coalesced into a new node that respects the
// constraint and aggregates barycenter and weight information.
//
// This implementation is based on the description in Forster, "A Fast and
// Simple Heuristic for Constrained Two-Level Crossing Reduction," though it
// differs in some specific details.
//
// Pre-conditions:
//
//  1. Each entry has the form {v, barycenter, weight}, or if the node has
//     no barycenter, then {v}.
//
// Returns:
//
//	A new list of entries of the form {vs, i, barycenter, weight}. The list
//	`vs` may either be a singleton or it may be an aggregation of nodes
//	ordered such that they do not violate constraints from the constraint
//	graph. The property `i` is the lowest original index of any of the
//	elements in `vs`.
func resolveConflicts(entries []*barycenterEntry, cg *graphlib.Graph) []*conflictEntry {
	mappedEntries := js.NewObj[*conflictEntry]()
	for i, entry := range entries {
		tmp := &conflictEntry{
			indegree:   0,
			in:         []*conflictEntry{},
			out:        []*conflictEntry{},
			vs:         []string{entry.v},
			i:          float64(i),
			barycenter: nan,
			weight:     nan,
		}
		mappedEntries.Set(entry.v, tmp)
		if entry.hasBarycenter {
			tmp.barycenter = entry.barycenter
			tmp.weight = entry.weight
			tmp.hasBarycenter = true
		}
	}

	// lookup is mappedEntries[v]. The object has no entry for a node of the
	// constraint graph that is not in this layer. For such a node with the
	// name of an Object.prototype property, JavaScript finds the inherited
	// function. Then inherited is true.
	lookup := func(v string) (entry *conflictEntry, inherited bool) {
		if entry, ok := mappedEntries.Lookup(v); ok {
			return entry, false
		}
		return nil, isObjectPrototypeKey(v)
	}

	for _, e := range cg.Edges() {
		entryV, inheritedV := lookup(e.V)
		entryW, inheritedW := lookup(e.W)
		if (entryV != nil || inheritedV) && (entryW != nil || inheritedW) {
			if inheritedV {
				// entryV.out is undefined.
				panic(typeError("Cannot read properties of undefined (reading 'push')"))
			}
			if entryW != nil {
				entryW.indegree++
			}
			// For an inherited entryW this pushes a function. The use of it
			// in handleOut throws a TypeError, as the nil entry does here.
			entryV.out = append(entryV.out, entryW)
		}
	}

	sourceSet := []*conflictEntry{}
	mappedEntries.Each(func(_ string, entry *conflictEntry) {
		if !truthy(entry.indegree) {
			sourceSet = append(sourceSet, entry)
		}
	})

	return doResolveConflicts(sourceSet)
}

func doResolveConflicts(sourceSet []*conflictEntry) []*conflictEntry {
	entries := []*conflictEntry{}

	handleIn := func(vEntry *conflictEntry) func(uEntry *conflictEntry) {
		return func(uEntry *conflictEntry) {
			if uEntry.merged {
				return
			}
			if !uEntry.hasBarycenter ||
				!vEntry.hasBarycenter ||
				uEntry.barycenter >= vEntry.barycenter {
				mergeEntries(vEntry, uEntry)
			}
		}
	}

	handleOut := func(vEntry *conflictEntry) func(wEntry *conflictEntry) {
		return func(wEntry *conflictEntry) {
			wEntry.in = append(wEntry.in, vEntry)
			wEntry.indegree--
			if wEntry.indegree == 0 {
				sourceSet = append(sourceSet, wEntry)
			}
		}
	}

	for len(sourceSet) > 0 {
		entry := sourceSet[len(sourceSet)-1]
		sourceSet = sourceSet[:len(sourceSet)-1]
		entries = append(entries, entry)
		// entry['in'].reverse() reverses in place.
		for i, j := 0, len(entry.in)-1; i < j; i, j = i+1, j-1 {
			entry.in[i], entry.in[j] = entry.in[j], entry.in[i]
		}
		in := handleIn(entry)
		for _, uEntry := range entry.in {
			in(uEntry)
		}
		out := handleOut(entry)
		for _, wEntry := range entry.out {
			out(wEntry)
		}
	}

	result := []*conflictEntry{}
	for _, entry := range entries {
		if !entry.merged {
			// _.pick(entry, ['vs', 'i', 'barycenter', 'weight'])
			picked := &conflictEntry{
				vs:            entry.vs,
				i:             entry.i,
				barycenter:    entry.barycenter,
				weight:        entry.weight,
				hasBarycenter: entry.hasBarycenter,
			}
			result = append(result, picked)
		}
	}
	return result
}

func mergeEntries(target, source *conflictEntry) {
	sum := 0.0
	weight := 0.0

	if truthy(target.weight) {
		sum += float64(target.barycenter * target.weight)
		weight += target.weight
	}

	if truthy(source.weight) {
		sum += float64(source.barycenter * source.weight)
		weight += source.weight
	}

	vs := make([]string, 0, len(source.vs)+len(target.vs))
	vs = append(vs, source.vs...)
	vs = append(vs, target.vs...)
	target.vs = vs
	target.barycenter = sum / weight
	target.weight = weight
	target.hasBarycenter = true
	target.i = min(source.i, target.i)
	source.merged = true
}
