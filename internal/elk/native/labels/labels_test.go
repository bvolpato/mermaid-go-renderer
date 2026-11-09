package labels_test

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/labels"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/labels/labelstest"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
)

func init() {
	// The other properties of the golden states whose values are Java
	// objects. The harness keeps them in their stored form. The package
	// labelstest registers the codec of the property endLabels itself.
	labelstest.RegisterOpaque("selfLoopHolder", "crossHierarchyMap", "breakingPoint.info", "org.eclipse.elk.spacing.individual", "splines.route.start")
}

func TestLabelDummyInserter(t *testing.T) {
	testProcessor(t, "LabelDummyInserter", func() lgraph.Processor { return labels.NewLabelDummyInserter() })
}

func TestLabelDummySwitcher(t *testing.T) {
	testProcessor(t, "LabelDummySwitcher", func() lgraph.Processor { return labels.NewLabelDummySwitcher() })
}

func TestLabelSideSelector(t *testing.T) {
	testProcessor(t, "LabelSideSelector", func() lgraph.Processor { return labels.NewLabelSideSelector() })
}

func TestLabelDummyRemover(t *testing.T) {
	testProcessor(t, "LabelDummyRemover", func() lgraph.Processor { return labels.NewLabelDummyRemover() })
}

func TestEndLabelPreprocessor(t *testing.T) {
	testProcessor(t, "EndLabelPreprocessor", func() lgraph.Processor { return labels.NewEndLabelPreprocessor() })
}

func TestEndLabelPostprocessor(t *testing.T) {
	testProcessor(t, "EndLabelPostprocessor", func() lgraph.Processor { return labels.NewEndLabelPostprocessor() })
}

func TestEndLabelSorter(t *testing.T) {
	testProcessor(t, "EndLabelSorter", func() lgraph.Processor { return labels.NewEndLabelSorter() })
}

// testProcessor checks a processor against the shared phase goldens, and
// against the goldens of this package: the steps of the label processors for
// graphs with label options that the shared goldens do not reach.
// tools/reference/gen-elk-labels.mjs writes testdata/label-steps.json.gz.
func testProcessor(t *testing.T, name string, newProcessor lgraph.Factory) {
	t.Run("phases", func(t *testing.T) { nativetest.RunProcessor(t, name, newProcessor) })
	t.Run("labels", func(t *testing.T) { runCases(t, name, labelCases(t), newProcessor) })
}

var (
	labelCasesOnce sync.Once
	labelCasesData []*nativetest.Case
	labelCasesErr  error
)

// labelCases reads the goldens of this package. The environment variable
// ELK_PHASE_CASE limits them to the cases whose name contains its value.
func labelCases(t *testing.T) []*nativetest.Case {
	t.Helper()
	labelCasesOnce.Do(func() {
		f, err := os.Open(filepath.Join("testdata", "label-steps.json.gz"))
		if err != nil {
			labelCasesErr = err
			return
		}
		defer f.Close()
		zr, err := gzip.NewReader(f)
		if err != nil {
			labelCasesErr = err
			return
		}
		labelCasesErr = json.NewDecoder(zr).Decode(&labelCasesData)
	})
	if labelCasesErr != nil {
		t.Fatal(labelCasesErr)
	}
	filter := os.Getenv("ELK_PHASE_CASE")
	var out []*nativetest.Case
	for _, c := range labelCasesData {
		if filter == "" || strings.Contains(c.Name, filter) {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		t.Skipf("no golden of this package has a name with %q", filter)
	}
	return out
}

// runCases is nativetest.RunProcessor for cases that are not in the files of
// the shared goldens: it checks each step with the given processor name.
func runCases(t *testing.T, name string, cases []*nativetest.Case, newProcessor lgraph.Factory) {
	t.Helper()
	steps, failed, failedCases, changed := 0, 0, 0, 0
	for _, c := range cases {
		caseFailed := false
		for ri, run := range c.Runs {
			for si, step := range run.Steps {
				if step.Proc != name {
					continue
				}
				steps++
				if step.Before != nil && step.After != nil && *step.Before != *step.After {
					changed++
				}
				res := nativetest.CheckStep(run, step, newProcessor())
				if res.OK() {
					continue
				}
				failed++
				if !caseFailed {
					caseFailed = true
					failedCases++
				}
				if failedCases > 5 {
					continue
				}
				where := fmt.Sprintf("%s, run %d, step %d (%s, slot %d, graph %d)", c.Name, ri, si, step.Kind, step.Slot, step.Graph)
				if res.Err != nil {
					t.Errorf("%s: %v", where, res.Err)
				} else {
					t.Errorf("%s: the state after the processor differs from elkjs:\n  %s", where, strings.Join(res.Diffs, "\n  "))
				}
			}
		}
	}
	t.Logf("%s: %d steps in %d graphs (%d steps change the state), %d steps differ from elkjs", name, steps, len(cases), changed, failed)
	if steps == 0 {
		t.Errorf("%s: the goldens of this package have no step of the processor", name)
	}
	if failed > 0 {
		t.Errorf("%s: %d of %d steps differ from elkjs, in %d graphs", name, failed, steps, failedCases)
	}
}

// processors has the processors of this package by name.
var processors = map[string]lgraph.Factory{
	"LabelDummyInserter":    func() lgraph.Processor { return labels.NewLabelDummyInserter() },
	"LabelDummySwitcher":    func() lgraph.Processor { return labels.NewLabelDummySwitcher() },
	"LabelSideSelector":     func() lgraph.Processor { return labels.NewLabelSideSelector() },
	"LabelDummyRemover":     func() lgraph.Processor { return labels.NewLabelDummyRemover() },
	"EndLabelPreprocessor":  func() lgraph.Processor { return labels.NewEndLabelPreprocessor() },
	"EndLabelPostprocessor": func() lgraph.Processor { return labels.NewEndLabelPostprocessor() },
	"EndLabelSorter":        func() lgraph.Processor { return labels.NewEndLabelSorter() },
}

// TestProcessorsDoNotPanic runs each processor on the states of other points
// of the layout: graphs without layers, graphs with dummy nodes that the
// processor does not expect, graphs whose labels have positions already. A
// processor must end, or stop with lgraph.Bail. It must not panic.
//
// The test reads each third file of the shared goldens. The environment
// variable ELK_LABELS_ALL_STATES makes it read each file.
func TestProcessorsDoNotPanic(t *testing.T) {
	files, err := nativetest.Files("")
	if err != nil {
		t.Fatal(err)
	}
	stride := 3
	if os.Getenv("ELK_LABELS_ALL_STATES") != "" {
		stride = 1
	}
	if testing.Short() {
		stride = 12
	}
	states, runs, stopped := 0, 0, 0
	reasons := map[string]int{}
	for i := 0; i < len(files); i += stride {
		c, err := nativetest.LoadCase(files[i])
		if err != nil {
			t.Fatal(err)
		}
		for ri, run := range c.Runs {
			seen := map[int]bool{}
			for _, step := range run.Steps {
				if step.Before == nil || seen[*step.Before] {
					continue
				}
				seen[*step.Before] = true
				state, err := run.State(*step.Before)
				if err != nil {
					t.Fatal(err)
				}
				states++
				for name, newProcessor := range processors {
					// The processor changes the graph, so each one gets its
					// own.
					built, err := nativetest.Build(state)
					if err != nil {
						t.Fatalf("%s, run %d, state %d: %v", c.Name, ri, *step.Before, err)
					}
					if step.Graph < 0 || step.Graph >= len(built.Graphs) {
						continue
					}
					runs++
					err = nativetest.Process(newProcessor(), built.Graphs[step.Graph], lgraph.NewContext())
					if err == nil {
						continue
					}
					stopped++
					if strings.Contains(err.Error(), "panicked") {
						t.Errorf("%s, run %d, the state before %s: %s: %v", c.Name, ri, step.Proc, name, err)
						continue
					}
					reasons[name+": "+err.Error()]++
				}
			}
		}
	}
	t.Logf("%d processor runs on %d states: %d stopped with lgraph.Bail", runs, states, stopped)
	for reason, n := range reasons {
		t.Logf("%5d %s", n, reason)
	}
}
