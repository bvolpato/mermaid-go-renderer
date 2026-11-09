package loopstest

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
)

// RunProcessor checks a processor against the shared phase goldens (the
// subtest phases), and against the goldens of the self loop packages (the
// subtest loops): the steps of the self loop processors for graphs with self
// loops that the shared goldens do not have. Those are loops with ports on
// two, three and four sides, hyper loops, each distribution strategy and
// each ordering strategy, labels that touch, and loops whose routing slots
// need the random numbers of the graph.
// tools/reference/gen-elk-loops.mjs writes testdata/loop-steps.json.gz.
func RunProcessor(t *testing.T, name string, newProcessor lgraph.Factory) {
	t.Helper()
	t.Run("phases", func(t *testing.T) { nativetest.RunProcessor(t, name, newProcessor) })
	t.Run("loops", func(t *testing.T) { runCases(t, name, Cases(t), newProcessor) })
}

var (
	casesOnce sync.Once
	casesData []*nativetest.Case
	casesErr  error
)

// Cases reads the goldens of the self loop packages. The environment
// variable ELK_PHASE_CASE limits them to the cases whose name contains its
// value.
func Cases(t *testing.T) []*nativetest.Case {
	t.Helper()
	casesOnce.Do(func() {
		_, file, _, _ := runtime.Caller(0)
		f, err := os.Open(filepath.Join(filepath.Dir(file), "testdata", "loop-steps.json.gz"))
		if err != nil {
			casesErr = err
			return
		}
		defer f.Close()
		zr, err := gzip.NewReader(f)
		if err != nil {
			casesErr = err
			return
		}
		casesErr = json.NewDecoder(zr).Decode(&casesData)
	})
	if casesErr != nil {
		t.Fatal(casesErr)
	}
	filter := os.Getenv("ELK_PHASE_CASE")
	var out []*nativetest.Case
	for _, c := range casesData {
		if filter == "" || strings.Contains(c.Name, filter) {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		t.Skipf("no golden of the self loop packages has a name with %q", filter)
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
		t.Errorf("%s: the goldens of the self loop packages have no step of the processor", name)
	}
	if failed > 0 {
		t.Errorf("%s: %d of %d steps differ from elkjs, in %d graphs", name, failed, steps, failedCases)
	}
}
