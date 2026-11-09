package intermediate

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// The golden states give each Java object that the harness does not know a
// number: {"$o": i, "c": class, "f": fields} where the walk of the dump finds
// the object first, and {"$o": i} at each later reference. elkjs numbers the
// objects again in each state, in the order of the walk. nativetest.Build
// keeps such a property value as it is stored, and nativetest.Dump writes it
// back. So a processor that changes the order of the walk (it removes a node
// or an edge) gets a result with the same objects under other numbers. The
// walk of the result also finds the elements that only such objects name at
// another time, because only the first reference of the state before has the
// fields.
//
// inlineObjects and canonicalObjects take both effects away. They change no
// content of a state.
//
// TODO(core): move to nativetest (Build and Dump can do both).

func isOrderKey(k string) bool { return strings.HasPrefix(k, "\x00") }

// collectObjects returns the full form of each numbered object of a state.
func collectObjects(state any) map[float64]map[string]any {
	full := map[float64]map[string]any{}
	var collect func(v any)
	collect = func(v any) {
		switch x := v.(type) {
		case []any:
			for _, e := range x {
				collect(e)
			}
		case map[string]any:
			if id, ok := x["$o"].(float64); ok {
				if _, isFull := x["c"]; isFull {
					if _, seen := full[id]; !seen {
						full[id] = x
					}
				}
			}
			for k, e := range x {
				if !isOrderKey(k) {
					collect(e)
				}
			}
		}
	}
	collect(state)
	return full
}

// inlineObjects makes each property value of a state complete: the first
// reference to a numbered object in a value gets the full form of the
// object. The walk of nativetest.Dump then finds the elements that the
// object names where the walk of elkjs finds them: at the first reference to
// the object. The member order of each object stays, because the walk visits
// the fields in that order.
func inlineObjects(state map[string]any) {
	full := collectObjects(state)
	var inline func(v any, seen map[float64]bool) any
	inline = func(v any, seen map[float64]bool) any {
		switch x := v.(type) {
		case []any:
			out := make([]any, len(x))
			for i, e := range x {
				out[i] = inline(e, seen)
			}
			return out
		case map[string]any:
			src := x
			if id, ok := x["$o"].(float64); ok {
				if seen[id] {
					return map[string]any{"$o": id}
				}
				seen[id] = true
				if def := full[id]; def != nil {
					src = def
				}
			}
			out := make(map[string]any, len(src))
			// The fields in the order of the JSON text: a later reference
			// to an object must be the short one.
			keys := make([]string, 0, len(src))
			var order []string
			for k, e := range src {
				if isOrderKey(k) {
					order, _ = e.([]string)
					out[k] = e
					continue
				}
				keys = append(keys, k)
			}
			if order != nil {
				keys = order
			} else {
				sort.Strings(keys)
			}
			for _, k := range keys {
				out[k] = inline(src[k], seen)
			}
			return out
		}
		return v
	}
	props := func(row map[string]any) {
		pr, _ := row["pr"].(map[string]any)
		for id, v := range pr {
			if !isOrderKey(id) {
				pr[id] = inline(v, map[float64]bool{})
			}
		}
	}
	for _, table := range []string{"g", "n", "p", "e", "l"} {
		rows, _ := state[table].([]any)
		for _, r := range rows {
			row, _ := r.(map[string]any)
			props(row)
			layers, _ := row["ly"].([]any)
			for _, l := range layers {
				if layer, ok := l.(map[string]any); ok {
					props(layer)
				}
			}
		}
	}
}

// canonicalObjects numbers the objects of a state again in one fixed order,
// and puts the full form at the first reference in that order. Two states
// are equal after that when they have the same objects with the same
// references.
func canonicalObjects(state map[string]any) map[string]any {
	full := collectObjects(state)
	keysOf := func(m map[string]any) []string {
		keys := make([]string, 0, len(m))
		for k := range m {
			if !isOrderKey(k) {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		return keys
	}
	numbers := map[float64]float64{}
	var rewrite func(v any) any
	rewrite = func(v any) any {
		switch x := v.(type) {
		case []any:
			out := make([]any, len(x))
			for i, e := range x {
				out[i] = rewrite(e)
			}
			return out
		case map[string]any:
			if id, ok := x["$o"].(float64); ok {
				if n, seen := numbers[id]; seen {
					return map[string]any{"$o": n}
				}
				n := float64(len(numbers))
				numbers[id] = n
				out := map[string]any{"$o": n}
				if def := full[id]; def != nil {
					for _, k := range keysOf(def) {
						if k != "$o" {
							out[k] = rewrite(def[k])
						}
					}
				}
				return out
			}
			out := make(map[string]any, len(x))
			for _, k := range keysOf(x) {
				out[k] = rewrite(x[k])
			}
			return out
		}
		return v
	}
	out := map[string]any{}
	for _, table := range []string{"g", "n", "p", "e", "l"} {
		if v, ok := state[table]; ok {
			out[table] = rewrite(v)
		}
	}
	return out
}

// restoreSharedValues restores object identities that a stored state does
// not have. GraphTransformer changes some property values in place, and
// elkjs has one object for some values of two holders:
//
//   - ElkGraphImporter copies the properties of one ElkNode to the LNode and
//     to its nested LGraph. Both have the same object for the options
//     nodeLabels.placement (an EnumSet), position (a KVector) and
//     nodeLabels.padding (an ElkPadding).
//   - CompoundGraphPreprocessor makes the external port dummy of a port of a
//     node with the size vector of the port as the value of the property
//     externalPortSize, and with the port as origin. A dummy that replaces
//     such a dummy has a copy of its properties, so the same vector.
//
// nativetest.Build makes one object for each holder. This function gives
// the two holders one object again when their values are equal.
//
// TODO(core): move to nativetest, or store the identity of shared property
// values in the state.
func restoreSharedValues(built *nativetest.Built) {
	for _, node := range built.Nodes {
		if nested := node.NestedGraph; nested != nil {
			if prop.Has(node, lopts.NodeLabelsPlacement) && prop.Has(nested, lopts.NodeLabelsPlacement) {
				if v := prop.Get(node, lopts.NodeLabelsPlacement); v.Equals(prop.Get(nested, lopts.NodeLabelsPlacement)) {
					prop.Set(nested, lopts.NodeLabelsPlacement, v)
				}
			}
			if prop.Has(node, lopts.Position) && prop.Has(nested, lopts.Position) {
				if v := prop.Get(node, lopts.Position); v.Equals(prop.Get(nested, lopts.Position)) {
					prop.Set(nested, lopts.Position, v)
				}
			}
			if prop.Has(node, lopts.NodeLabelsPadding) && prop.Has(nested, lopts.NodeLabelsPadding) {
				if v := prop.Get(node, lopts.NodeLabelsPadding); v.Equals(&prop.Get(nested, lopts.NodeLabelsPadding).Spacing) {
					prop.Set(nested, lopts.NodeLabelsPadding, v)
				}
			}
		}
		if node.Type == lgraph.NodeTypeExternalPort && prop.Has(node, iprops.ExtPortSize) {
			if port, ok := prop.Get(node, iprops.Origin).(*lgraph.LPort); ok && port != nil && prop.Get(node, iprops.ExtPortSize).Equals(&port.Size) {
				prop.Set(node, iprops.ExtPortSize, &port.Size)
			}
		}
	}
}

func maxDiffs() int {
	if n, err := strconv.Atoi(os.Getenv("ELK_PHASE_DIFFS")); err == nil && n > 0 {
		return n
	}
	return 12
}

// checkStepCanonical is nativetest.CheckStep with inlineObjects on the state
// before the step, restoreSharedValues on the graph that Build makes from
// it, and canonicalObjects on both states of the comparison. Each field,
// property and float bit is compared as CheckStep compares it.
func checkStepCanonical(run *nativetest.Run, step *nativetest.Step, p lgraph.Processor) ([]string, error) {
	if step.Before == nil || step.After == nil {
		return nil, fmt.Errorf("the step %s has no single state before and after", step.Proc)
	}
	before, err := run.State(*step.Before)
	if err != nil {
		return nil, err
	}
	want, err := run.State(*step.After)
	if err != nil {
		return nil, err
	}
	inlineObjects(before)
	built, err := nativetest.Build(before)
	if err != nil {
		return nil, err
	}
	if step.Graph < 0 || step.Graph >= len(built.Graphs) {
		return nil, fmt.Errorf("the step names the graph %d, the state has %d graphs", step.Graph, len(built.Graphs))
	}
	restoreSharedValues(built)
	if err := nativetest.Process(p, built.Graphs[step.Graph], lgraph.NewContext()); err != nil {
		return nil, err
	}
	got, err := nativetest.Dump(built.Root())
	if err != nil {
		return nil, err
	}
	return nativetest.Diff(canonicalObjects(got), canonicalObjects(want), maxDiffs()), nil
}

// runProcessorCanonical is nativetest.RunProcessor with checkStepCanonical.
func runProcessorCanonical(t *testing.T, name string, newProcessor lgraph.Factory) {
	t.Helper()
	files, err := nativetest.Files(name)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("the phase goldens have no step of the processor %q", name)
	}
	steps, failed, reported := 0, 0, 0
	for _, file := range files {
		c, err := nativetest.LoadCase(file)
		if err != nil {
			t.Fatal(err)
		}
		for ri, run := range c.Runs {
			for si, step := range run.Steps {
				if step.Proc != name {
					continue
				}
				steps++
				diffs, err := checkStepCanonical(run, step, newProcessor())
				if err == nil && len(diffs) == 0 {
					continue
				}
				failed++
				if reported++; reported > 5 {
					continue
				}
				where := fmt.Sprintf("%s (%s), run %d, step %d (%s, slot %d, graph %d)", c.Name, file, ri, si, step.Kind, step.Slot, step.Graph)
				if err != nil {
					t.Errorf("%s: %v", where, err)
				} else {
					t.Errorf("%s: the state after the processor differs from elkjs:\n  %s", where, strings.Join(diffs, "\n  "))
				}
			}
		}
	}
	t.Logf("%s: %d steps in %d graphs, %d steps differ from elkjs", name, steps, len(files), failed)
	if failed > 0 {
		t.Errorf("%s: %d of %d steps differ from elkjs", name, failed, steps)
	}
}
