// Package p4test has the test helpers of the node placers: RunProcessor for
// the phase goldens of nativetest, and RunExtra for the extra goldens of
// tools/reference/gen-elk-p4nodes.mjs.
package p4test

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
)

// coreStop is the reason with which lgraph.IndividualOrDefault stops a
// layout: the core does not port the individual spacings of a node.
const coreStop = "Spacings: individual spacings are not ported"

// RunProcessor is nativetest.RunProcessor with one difference: a step that
// stops in the core because a node has individual spacings does not fail the
// test. The test logs the number of these steps. Each other step must give
// the state of elkjs, bit for bit.
//
// minSteps is the smallest number of steps that the goldens must have for
// the processor.
func RunProcessor(t *testing.T, name string, minSteps int, newProcessor lgraph.Factory) {
	t.Helper()
	files, err := nativetest.Files(name)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("the phase goldens have no step of the processor %q", name)
	}
	steps, failed, stopped := 0, 0, 0
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
				res := nativetest.CheckStep(run, step, newProcessor())
				if res.OK() {
					continue
				}
				if res.Err != nil && strings.Contains(res.Err.Error(), coreStop) {
					stopped++
					continue
				}
				failed++
				if failed > 5 {
					continue
				}
				where := fmt.Sprintf("%s (%s), run %d, step %d (%s, slot %d, graph %d)", c.Name, file, ri, si, step.Kind, step.Slot, step.Graph)
				if res.Err != nil {
					t.Errorf("%s: %v", where, res.Err)
				} else {
					t.Errorf("%s: the state after the processor differs from elkjs:\n  %s", where, strings.Join(res.Diffs, "\n  "))
				}
			}
		}
	}
	t.Logf("%s: %d steps in %d graphs, %d steps equal elkjs, %d steps differ, %d steps stop in the core (individual spacings)",
		name, steps, len(files), steps-failed-stopped, failed, stopped)
	if failed > 0 {
		t.Errorf("%s: %d of %d steps differ from elkjs", name, failed, steps)
	}
	if steps < minSteps {
		t.Errorf("%s: the goldens have %d steps, the test expects at least %d", name, steps, minSteps)
	}
}

// extraStep is one step of an extra golden case: a step of the phase goldens,
// and the mark of a step where the processor of elkjs throws.
type extraStep struct {
	nativetest.Step
	Throws bool `json:"throws"`
}

// extraCase is one line of an extra golden file of
// tools/reference/gen-elk-p4nodes.mjs.
type extraCase struct {
	Name   string            `json:"name"`
	Steps  []*extraStep      `json:"steps"`
	States []json.RawMessage `json:"states"`
}

// RunExtra tests a processor against the extra goldens of one file: the
// states of elkjs before and after the processor, for random graphs with the
// options that select the branches of the node placers. Each step must give
// the state of elkjs, bit for bit. Where the processor of elkjs throws, the
// Go processor must stop with lgraph.Bail.
//
// As in RunProcessor, a step that stops in the core because a node has
// individual spacings does not fail the test.
//
// The environment variable ELK_PHASE_CASE limits the run to the cases whose
// name contains its value.
func RunExtra(t *testing.T, file, name string, minSteps int, newProcessor lgraph.Factory) {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	filter := os.Getenv("ELK_PHASE_CASE")
	steps, failed, stopped, threw, cases := 0, 0, 0, 0, 0
	scanner := bufio.NewScanner(zr)
	scanner.Buffer(make([]byte, 1<<20), 1<<28)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		c := &extraCase{}
		if err := json.Unmarshal(line, c); err != nil {
			t.Fatal(err)
		}
		if filter != "" && !strings.Contains(c.Name, filter) {
			continue
		}
		cases++
		run := &nativetest.Run{States: c.States}
		// One processor serves each graph of one layout, as in ELK: the
		// steps of a case share it.
		processor := newProcessor()
		for si, step := range c.Steps {
			if step.Proc != name {
				continue
			}
			steps++
			where := fmt.Sprintf("%s, step %d (%s, slot %d, graph %d)", c.Name, si, step.Kind, step.Slot, step.Graph)
			if step.Throws {
				threw++
				if err := runThrowingStep(run, step, processor); err != nil {
					failed++
					if failed <= 5 {
						t.Errorf("%s: %v", where, err)
					}
				}
				// A processor that stopped keeps the state of the graph it
				// stopped in. A layout does not use it again.
				processor = newProcessor()
				continue
			}
			res := nativetest.CheckStep(run, &step.Step, processor)
			if res.OK() {
				continue
			}
			if res.Err != nil && strings.Contains(res.Err.Error(), coreStop) {
				stopped++
				continue
			}
			failed++
			if failed > 5 {
				continue
			}
			if res.Err != nil {
				t.Errorf("%s: %v", where, res.Err)
			} else {
				t.Errorf("%s: the state after the processor differs from elkjs:\n  %s", where, strings.Join(res.Diffs, "\n  "))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s, extra goldens: %d steps in %d graphs, %d steps equal elkjs or stop where elkjs throws (%d), %d steps differ, %d steps stop in the core (individual spacings)",
		name, steps, cases, steps-failed-stopped, threw, failed, stopped)
	if failed > 0 {
		t.Errorf("%s: %d of %d steps differ from elkjs", name, failed, steps)
	}
	if filter == "" && steps < minSteps {
		t.Errorf("%s: the extra goldens have %d steps, the test expects at least %d", name, steps, minSteps)
	}
}

// runThrowingStep runs a step where the processor of elkjs throws. The Go
// processor must stop with lgraph.Bail.
func runThrowingStep(run *nativetest.Run, step *extraStep, p lgraph.Processor) error {
	if step.Before == nil {
		return fmt.Errorf("the step has no state before")
	}
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
	if err == nil {
		return fmt.Errorf("elkjs throws in this step, and the processor did not stop")
	}
	if !strings.Contains(err.Error(), "the processor stopped") {
		return fmt.Errorf("elkjs throws in this step, and the processor did not stop with lgraph.Bail: %v", err)
	}
	return nil
}
