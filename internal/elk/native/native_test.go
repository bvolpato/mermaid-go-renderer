package native

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/compound"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/elkgraph"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/labels/labelstest" // the codec of the end label cells
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/layered"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops/loopstest" // the codec of the self loop holder
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
)

func init() {
	// The dump of the multimap of the compound graph preprocessor, in the
	// form of the phase goldens. The pipeline test only writes states.
	nativetest.RegisterCodec("crossHierarchyMap", nativetest.Codec{
		Decode: func(raw any, b *nativetest.Built) (any, error) {
			return b.Opaque(raw), nil
		},
		Encode: func(v any, d *nativetest.Dumper) any {
			m, ok := v.(*compound.CrossHierarchyMap)
			if !ok {
				return d.Value(v)
			}
			rows := []any{}
			for _, key := range m.KeySet() {
				k := map[string]any{"$e": float64(d.Ref(key))}
				values := []any{}
				for _, che := range m.Get(key) {
					id, first := d.ObjectID(che)
					o := map[string]any{"$o": float64(id)}
					if first {
						o["c"] = "alg.layered.compound.CrossHierarchyEdge"
						f := map[string]any{}
						f["newEdge"] = map[string]any{"$e": float64(d.Ref(che.Edge()))}
						f["graph_0"] = map[string]any{"$g": float64(d.Ref(che.Graph()))}
						f["type_0"] = che.Type().String()
						o["f"] = f
					}
					values = append(values, o)
				}
				rows = append(rows, []any{k, values})
			}
			return map[string]any{"$mm": rows}
		},
	})
}

// TestPipelineAgainstPhases lays out each graph of the phase goldens and
// compares, as far as the layout gets, the state after the import of each
// run and the state before and after each step with elkjs. A layout that
// falls back stops the comparison for that graph: the test then tells which
// processor is missing. Each compared state must be equal.
func TestPipelineAgainstPhases(t *testing.T) {
	files, err := nativetest.Files("")
	if err != nil {
		t.Fatal(err)
	}
	if testing.Short() {
		files = files[:30]
	}
	var done, fallback, differ, imports, steps, unreadable int
	reasons := map[string]int{}
	failures := 0
	for _, file := range files {
		c, err := nativetest.LoadCase(file)
		if err != nil {
			t.Fatal(err)
		}
		var problems []string
		problem := func(format string, args ...any) {
			if len(problems) < 3 {
				problems = append(problems, fmt.Sprintf(format, args...))
			}
		}
		runIndex := -1
		stepIndex := 0
		blind := false
		var run *nativetest.Run
		// compare checks the hierarchy below root against a golden state.
		compare := func(what string, root *lgraph.LGraph, state int) {
			want, err := run.State(state)
			if err != nil {
				problem("%s: %v", what, err)
				return
			}
			if blind {
				return
			}
			got, err := nativetest.Dump(root)
			if err != nil {
				if strings.Contains(err.Error(), "cannot store a property value") {
					// A property value of a processor package whose codec
					// lives in the test of that package. The states of this
					// graph cannot be compared from here on. The output
					// still is.
					blind = true
					unreadable++
					return
				}
				problem("%s: %v", what, err)
				return
			}
			if d := nativetest.Diff(got, want, 6); len(d) > 0 {
				problem("%s: %s", what, strings.Join(d, "; "))
			}
		}
		rootOf := func(g *lgraph.LGraph) *lgraph.LGraph {
			for g.ParentNode != nil {
				g = g.ParentNode.Graph()
			}
			return g
		}
		hooks := &Hooks{
			Imported: func(elkGraph *elkgraph.Node, graph *lgraph.LGraph) {
				runIndex++
				stepIndex = 0
				if runIndex >= len(c.Runs) {
					problem("run %d: elkjs has %d runs", runIndex, len(c.Runs))
					run = nil
					return
				}
				run = c.Runs[runIndex]
				if run.Elk != elkGraph.Identifier {
					problem("run %d lays out the node %q, elkjs %q", runIndex, elkGraph.Identifier, run.Elk)
				}
				imports++
				compare(fmt.Sprintf("run %d after the import", runIndex), graph, run.Imported)
			},
			Step: func(ev layered.StepEvent) {
				if run == nil || len(problems) > 0 {
					return
				}
				if ev.Name == "ComponentsProcessor.split" || ev.Name == "ComponentsProcessor.combine" {
					// The goldens have these steps with lists of states. The
					// states of the steps around them cover them.
					if ev.Done {
						stepIndex++
					}
					return
				}
				if stepIndex >= len(run.Steps) {
					problem("run %d: step %d (%s), elkjs has %d steps", runIndex, stepIndex, ev.Name, len(run.Steps))
					return
				}
				step := run.Steps[stepIndex]
				if step.Proc != ev.Name {
					problem("run %d step %d is %s, elkjs runs %s", runIndex, stepIndex, ev.Name, step.Proc)
					return
				}
				root := ev.Graph
				if step.Kind != "layout" && step.Kind != "configure-plain" {
					root = rootOf(ev.Graph)
				}
				if !ev.Done {
					if step.Before != nil {
						compare(fmt.Sprintf("run %d before step %d (%s)", runIndex, stepIndex, ev.Name), root, *step.Before)
					}
					return
				}
				if step.After != nil {
					compare(fmt.Sprintf("run %d after step %d (%s)", runIndex, stepIndex, ev.Name), root, *step.After)
				}
				steps++
				stepIndex++
			},
			Final: func(elkGraph *elkgraph.Node, graph *lgraph.LGraph) {
				if run != nil && len(problems) == 0 {
					compare(fmt.Sprintf("run %d before the transfer", runIndex), graph, run.Final)
				}
			},
		}
		out, reason := LayoutWithHooks(c.In, hooks)
		switch {
		case reason != "":
			fallback++
			key := reason
			if i := strings.IndexByte(key, '\n'); i >= 0 {
				key = key[:i]
			}
			reasons[key]++
		case string(out) != string(c.Out):
			differ++
			problem("the output differs from elkjs:\n   got  %s\n   want %s", clip(out), clip(c.Out))
		default:
			done++
		}
		if len(problems) > 0 {
			failures++
			if failures <= 6 {
				t.Errorf("%s (%s):\n  %s", c.Name, file, strings.Join(problems, "\n  "))
			}
		}
	}
	t.Logf("%d graphs: %d laid out and equal to elkjs, %d fall back, %d differ; %d imports and %d steps compared, %d graphs with a state that differs, %d graphs with states that this test cannot write", len(files), done, fallback, differ, imports, steps, failures, unreadable)
	for reason, n := range reasons {
		t.Logf("%4d fall back: %s", n, reason)
	}
	if failures > 0 {
		t.Errorf("%d graphs have a state or an output that differs from elkjs", failures)
	}
}

func clip(b []byte) string {
	if len(b) > 600 {
		return string(b[:600]) + "…"
	}
	return string(b)
}
