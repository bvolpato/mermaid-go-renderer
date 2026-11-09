// Package nativetest tests the processors of internal/elk/native against the
// phase goldens: the states of the LGraph that elkjs 0.9.3 had before and
// after each processor, for graphs that mermaid produces and for generic
// graphs. tools/reference/gen-elk-phases.mjs writes the goldens.
//
// A test of a processor is one call:
//
//	func TestLongEdgeSplitter(t *testing.T) {
//		nativetest.RunProcessor(t, "LongEdgeSplitter", func() lgraph.Processor { return NewLongEdgeSplitter() })
//	}
//
// The environment variable ELK_PHASE_CASE limits a run to the cases whose
// name contains its value. ELK_PHASE_DIFFS sets the number of differences
// that a failure prints.
package nativetest

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
)

// Step is one processor call of a run, or one of the pseudo steps of the
// layered algorithm.
type Step struct {
	// Kind is "hierarchical" or "layout" for a processor of the processor
	// list, "compound" for the compound graph preprocessor and
	// postprocessor, "configure" and "configure-plain" for
	// GraphConfigurator.prepareGraphForLayout, and "split" and "combine" for
	// the components processor.
	Kind string `json:"kind"`
	// Proc is the simple name of the Java class.
	Proc string `json:"proc"`
	// Slot is the index in the processor list.
	Slot int `json:"slot"`
	// Graph is the index of the processed graph in the table of graphs of
	// the state before.
	Graph int `json:"graph"`
	// Comp is the index of the connected component that a "layout" step
	// processes.
	Comp *int `json:"comp"`
	// Before and After are indices of Run.States.
	Before *int `json:"before"`
	After  *int `json:"after"`
	// BeforeList has the states of the components before a "combine" step,
	// and AfterList the states of the components after a "split" step.
	BeforeList []int `json:"beforeList"`
	AfterList  []int `json:"afterList"`
	// Draws and DrawsAfter count the calls of java.util.Random of the layout
	// before and after the step.
	Draws      int `json:"draws"`
	DrawsAfter int `json:"drawsAfter"`
}

// Run is one run of the layered algorithm: ELK runs it for the root node,
// and for each node that separates its children.
type Run struct {
	// Elk is the identifier of the ELK node that the run lays out.
	Elk string `json:"elk"`
	// Components is the number of connected components of a run without
	// hierarchy.
	Components int `json:"components"`
	// Imported is the state after ElkGraphImporter, and Final the state
	// before ElkGraphLayoutTransferrer.
	Imported int               `json:"imported"`
	Final    int               `json:"final"`
	Steps    []*Step           `json:"steps"`
	States   []json.RawMessage `json:"states"`
}

// Case is one graph with its runs.
type Case struct {
	Name string `json:"name"`
	// Source is "mermaid" for a graph that mermaid produced for the corpus,
	// and "golden" for a graph of tools/reference/gen-elk.mjs.
	Source string `json:"source"`
	// In is the ELK JSON graph. Out is the result of elkjs without the "$H"
	// members.
	In   json.RawMessage `json:"in"`
	Out  json.RawMessage `json:"out"`
	Runs []*Run          `json:"runs"`
}

// State returns the parsed state with the given index.
func (r *Run) State(i int) (map[string]any, error) {
	if i < 0 || i >= len(r.States) {
		return nil, fmt.Errorf("no state %d", i)
	}
	return ParseState(r.States[i])
}

type caseIndex struct {
	Cases []struct {
		File   string `json:"file"`
		Name   string `json:"name"`
		Source string `json:"source"`
	} `json:"cases"`
	Processors map[string][]string `json:"processors"`
}

// Dir returns the directory of the phase goldens.
func Dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", "phases")
}

var (
	indexOnce sync.Once
	indexData *caseIndex
	indexErr  error
)

func loadIndex() (*caseIndex, error) {
	indexOnce.Do(func() {
		data, err := os.ReadFile(filepath.Join(Dir(), "index.json"))
		if err != nil {
			indexErr = err
			return
		}
		indexData = &caseIndex{}
		indexErr = json.Unmarshal(data, indexData)
	})
	return indexData, indexErr
}

// LoadCase reads one golden file.
func LoadCase(file string) (*Case, error) {
	f, err := os.Open(filepath.Join(Dir(), file))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	c := &Case{}
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(c); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return c, nil
}

// Files returns the golden files that have a step of the processor, or each
// file for an empty name. The environment variable ELK_PHASE_CASE filters
// them by case name.
func Files(processor string) ([]string, error) {
	idx, err := loadIndex()
	if err != nil {
		return nil, err
	}
	filter := os.Getenv("ELK_PHASE_CASE")
	names := map[string]string{}
	var all []string
	for _, c := range idx.Cases {
		names[c.File] = c.Name
		all = append(all, c.File)
	}
	files := all
	if processor != "" {
		files = idx.Processors[processor]
	}
	var out []string
	for _, f := range files {
		if filter == "" || strings.Contains(names[f], filter) || strings.Contains(f, filter) {
			out = append(out, f)
		}
	}
	return out, nil
}

// Processors returns the names of the processors that the goldens have steps
// for.
func Processors() ([]string, error) {
	idx, err := loadIndex()
	if err != nil {
		return nil, err
	}
	var out []string
	for name := range idx.Processors {
		out = append(out, name)
	}
	return out, nil
}

func maxDiffs() int {
	if n, err := strconv.Atoi(os.Getenv("ELK_PHASE_DIFFS")); err == nil && n > 0 {
		return n
	}
	return 12
}

// StepResult is the result of one step that CheckStep ran.
type StepResult struct {
	// Diffs describes the differences between the state after the processor
	// and the golden state. It is empty when the step passes.
	Diffs []string
	// Err is a failure other than a difference: a state that cannot be
	// built, or a processor that stopped with Bail or a panic.
	Err error
}

// OK tells that the step gave the golden state.
func (r StepResult) OK() bool { return r.Err == nil && len(r.Diffs) == 0 }

// CheckStep builds the graph of the state before a step, runs the processor
// on it and compares the result with the state after the step.
func CheckStep(run *Run, step *Step, p lgraph.Processor) (res StepResult) {
	if step.Before == nil || step.After == nil {
		return StepResult{Err: fmt.Errorf("the step %s has no single state before and after", step.Proc)}
	}
	before, err := run.State(*step.Before)
	if err != nil {
		return StepResult{Err: err}
	}
	want, err := run.State(*step.After)
	if err != nil {
		return StepResult{Err: err}
	}
	built, err := Build(before)
	if err != nil {
		return StepResult{Err: err}
	}
	if step.Graph < 0 || step.Graph >= len(built.Graphs) {
		return StepResult{Err: fmt.Errorf("the step names the graph %d, the state has %d graphs", step.Graph, len(built.Graphs))}
	}
	if err := Process(p, built.Graphs[step.Graph], lgraph.NewContext()); err != nil {
		return StepResult{Err: err}
	}
	got, err := Dump(built.Root())
	if err != nil {
		return StepResult{Err: err}
	}
	return StepResult{Diffs: Diff(got, want, maxDiffs())}
}

// Process runs a processor and turns a Bail or a panic into an error.
func Process(p lgraph.Processor, graph *lgraph.LGraph, ctx *lgraph.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if b, ok := r.(*lgraph.BailError); ok {
				err = fmt.Errorf("the processor stopped: %s", b.Reason)
				return
			}
			buf := make([]byte, 4096)
			buf = buf[:runtime.Stack(buf, false)]
			err = fmt.Errorf("the processor panicked: %v\n%s", r, buf)
		}
	}()
	p.Process(graph, ctx)
	return nil
}

// RunProcessor tests a processor against each golden step that has its name.
// newProcessor makes the processor for one step.
func RunProcessor(t *testing.T, name string, newProcessor lgraph.Factory) {
	t.Helper()
	files, err := Files(name)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("the phase goldens have no step of the processor %q (ELK_PHASE_CASE=%q)", name, os.Getenv("ELK_PHASE_CASE"))
	}
	steps, failed, failedCases, changed := 0, 0, 0, 0
	for _, file := range files {
		c, err := LoadCase(file)
		if err != nil {
			t.Fatal(err)
		}
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
				res := CheckStep(run, step, newProcessor())
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
				where := fmt.Sprintf("%s (%s), run %d, step %d (%s, slot %d, graph %d)", c.Name, file, ri, si, step.Kind, step.Slot, step.Graph)
				if res.Err != nil {
					t.Errorf("%s: %v", where, res.Err)
				} else {
					t.Errorf("%s: the state after the processor differs from elkjs:\n  %s", where, strings.Join(res.Diffs, "\n  "))
				}
			}
		}
	}
	t.Logf("%s: %d steps in %d graphs (%d steps change the state), %d steps differ from elkjs", name, steps, len(files), changed, failed)
	if failed > 0 {
		t.Errorf("%s: %d of %d steps differ from elkjs, in %d graphs", name, failed, steps, failedCases)
	}
}
