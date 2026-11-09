package preserveorder

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
)

func TestSortByInputModelProcessor(t *testing.T) {
	runProcessor(t, "SortByInputModelProcessor", func() lgraph.Processor { return NewSortByInputModelProcessor() })
}

// runProcessor is nativetest.RunProcessor with one change: before the
// comparison it numbers the objects that the harness does not understand
// (the "$o" values, for example a SelfLoopHolder) again, in the same order
// for both states. elkjs numbers such an object in the order of its dump
// walk, and the harness keeps the number of the state before the step. So a
// processor that changes the order of nodes gives other numbers for objects
// that it did not touch.
func runProcessor(t *testing.T, name string, newProcessor lgraph.Factory) {
	t.Helper()
	files, err := nativetest.Files(name)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("the phase goldens have no step of the processor %q (ELK_PHASE_CASE=%q)", name, os.Getenv("ELK_PHASE_CASE"))
	}
	maxDiffs := 12
	if n, err := strconv.Atoi(os.Getenv("ELK_PHASE_DIFFS")); err == nil && n > 0 {
		maxDiffs = n
	}
	steps, failed, changed, printed := 0, 0, 0, 0
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
				if step.Before != nil && step.After != nil && *step.Before != *step.After {
					changed++
				}
				diffs, err := checkStep(run, step, newProcessor(), maxDiffs)
				if err == nil && len(diffs) == 0 {
					continue
				}
				failed++
				if printed++; printed > 5 {
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
	t.Logf("%s: %d steps in %d graphs (%d steps change the state), %d steps differ from elkjs", name, steps, len(files), changed, failed)
	if failed > 0 {
		t.Errorf("%s: %d of %d steps differ from elkjs", name, failed, steps)
	}
}

// checkStep is nativetest.CheckStep with the numbering of the "$o" objects
// made the same on both sides.
func checkStep(run *nativetest.Run, step *nativetest.Step, p lgraph.Processor, maxDiffs int) ([]string, error) {
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
	built, err := nativetest.Build(before)
	if err != nil {
		return nil, err
	}
	if step.Graph < 0 || step.Graph >= len(built.Graphs) {
		return nil, fmt.Errorf("the step names the graph %d, the state has %d graphs", step.Graph, len(built.Graphs))
	}
	if err := nativetest.Process(p, built.Graphs[step.Graph], lgraph.NewContext()); err != nil {
		return nil, err
	}
	got, err := nativetest.Dump(built.Root())
	if err != nil {
		return nil, err
	}
	renumberObjects(got)
	renumberObjects(want)
	return nativetest.Diff(got, want, maxDiffs), nil
}

// renumberObjects gives each "$o" object of a state a new number: the order
// in which a walk over the tables, with the members of an object in the
// order of their names, finds it.
func renumberObjects(state map[string]any) {
	ids := map[float64]float64{}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case []any:
			for _, e := range x {
				walk(e)
			}
		case map[string]any:
			if old, ok := x["$o"].(float64); ok {
				id, seen := ids[old]
				if !seen {
					id = float64(len(ids))
					ids[old] = id
				}
				x["$o"] = id
			}
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(x[k])
			}
		}
	}
	for _, table := range []string{"g", "n", "p", "e", "l"} {
		walk(state[table])
	}
}
