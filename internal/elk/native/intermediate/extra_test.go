package intermediate

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
)

// extraFactories has the processors of this package by the name of their
// golden steps.
var extraFactories = map[string]lgraph.Factory{
	"GraphTransformer:TO_INPUT_DIRECTION": func() lgraph.Processor { return NewGraphTransformer(ModeToInputDirection) },
	"GraphTransformer:TO_INTERNAL_LTR":    func() lgraph.Processor { return NewGraphTransformer(ModeToInternalLTR) },
	"EdgeAndLayerConstraintEdgeReverser":  func() lgraph.Processor { return NewEdgeAndLayerConstraintEdgeReverser() },
	"LayerConstraintPreprocessor":         func() lgraph.Processor { return NewLayerConstraintPreprocessor() },
	"LayerConstraintPostprocessor":        func() lgraph.Processor { return NewLayerConstraintPostprocessor() },
	"LongEdgeSplitter":                    func() lgraph.Processor { return NewLongEdgeSplitter() },
	"LongEdgeJoiner":                      func() lgraph.Processor { return NewLongEdgeJoiner() },
	"ReversedEdgeRestorer":                func() lgraph.Processor { return NewReversedEdgeRestorer() },
	"PortSideProcessor":                   func() lgraph.Processor { return NewPortSideProcessor() },
	"PortListSorter":                      func() lgraph.Processor { return NewPortListSorter() },
	"InLayerConstraintProcessor":          func() lgraph.Processor { return NewInLayerConstraintProcessor() },
	"LayerSizeAndGraphHeightCalculator":   func() lgraph.Processor { return NewLayerSizeAndGraphHeightCalculator() },
	"NorthSouthPortPreprocessor":          func() lgraph.Processor { return NewNorthSouthPortPreprocessor() },
	"NorthSouthPortPostprocessor":         func() lgraph.Processor { return NewNorthSouthPortPostprocessor() },
	"HyperedgeDummyMerger":                func() lgraph.Processor { return NewHyperedgeDummyMerger() },
}

// extraStep is a step of the extra goldens. Throws is the message of the
// exception that the processor throws in elkjs. Such a step has no state
// after it.
type extraStep struct {
	nativetest.Step
	Throws string `json:"throws"`
}

type extraCase struct {
	Name string `json:"name"`
	Runs []struct {
		Steps  []*extraStep      `json:"steps"`
		States []json.RawMessage `json:"states"`
	} `json:"runs"`
}

// checkThrows runs the processor on the state before a step where elkjs
// throws an exception. The processor must stop with lgraph.Bail.
func checkThrows(run *nativetest.Run, step *nativetest.Step, p lgraph.Processor) error {
	before, err := run.State(*step.Before)
	if err != nil {
		return err
	}
	built, err := nativetest.Build(before)
	if err != nil {
		return err
	}
	if step.Graph < 0 || step.Graph >= len(built.Graphs) {
		return fmt.Errorf("the step names the graph %d, the state has %d graphs", step.Graph, len(built.Graphs))
	}
	err = nativetest.Process(p, built.Graphs[step.Graph], lgraph.NewContext())
	switch {
	case err == nil:
		return fmt.Errorf("the processor did not stop")
	case !strings.HasPrefix(err.Error(), "the processor stopped: "):
		return err
	}
	return nil
}

// TestExtraPhases runs the processors of this package against the goldens of
// tools/reference/gen-elk-intermediate.mjs: graphs that reach the branches
// which the shared phase goldens do not reach. The states come from the same
// instrumented elkjs, in the same format. Where elkjs throws an exception,
// the processor must stop with lgraph.Bail. ELK_PHASE_CASE limits the cases.
func TestExtraPhases(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "phases.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Cases []*extraCase `json:"cases"`
	}
	if err := json.NewDecoder(zr).Decode(&golden); err != nil {
		t.Fatal(err)
	}
	filter := os.Getenv("ELK_PHASE_CASE")
	type count struct{ steps, changed, throws, failed int }
	counts := map[string]*count{}
	reported := 0
	for _, c := range golden.Cases {
		if filter != "" && !strings.Contains(c.Name, filter) {
			continue
		}
		for ri, r := range c.Runs {
			run := &nativetest.Run{States: r.States}
			for si, step := range r.Steps {
				factory := extraFactories[step.Proc]
				if factory == nil {
					t.Fatalf("%s: no processor for the step %s", c.Name, step.Proc)
				}
				n := counts[step.Proc]
				if n == nil {
					n = &count{}
					counts[step.Proc] = n
				}
				n.steps++
				var diffs []string
				var err error
				switch {
				case step.Before == nil:
					err = fmt.Errorf("the step has no state before it")
				case step.Throws != "":
					n.throws++
					err = checkThrows(run, &step.Step, factory())
				default:
					if step.After != nil && *step.Before != *step.After {
						n.changed++
					}
					diffs, err = checkStepCanonical(run, &step.Step, factory())
				}
				if err == nil && len(diffs) == 0 {
					continue
				}
				n.failed++
				if reported++; reported > 8 {
					continue
				}
				where := fmt.Sprintf("%s, run %d, step %d (%s, %s, slot %d, graph %d)", c.Name, ri, si, step.Proc, step.Kind, step.Slot, step.Graph)
				switch {
				case err != nil && step.Throws != "":
					t.Errorf("%s: elkjs throws %q: %v", where, step.Throws, err)
				case err != nil:
					t.Errorf("%s: %v", where, err)
				default:
					t.Errorf("%s: the state after the processor differs from elkjs:\n  %s", where, strings.Join(diffs, "\n  "))
				}
			}
		}
	}
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		n := counts[name]
		t.Logf("%s: %d steps (%d steps change the state, elkjs throws in %d steps), %d steps differ from elkjs", name, n.steps, n.changed, n.throws, n.failed)
		if n.failed > 0 {
			t.Errorf("%s: %d of %d steps differ from elkjs", name, n.failed, n.steps)
		}
	}
	if filter == "" && len(counts) != len(extraFactories) {
		t.Errorf("the goldens have steps of %d processors, want %d", len(counts), len(extraFactories))
	}
}
