package layered

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
)

// configurator runs GraphConfigurator.configureGraphProperties as a
// processor. The processor list that prepareGraphForLayout also builds is
// not part of a state.
type configurator struct{}

func (configurator) Process(graph *lgraph.LGraph, ctx *lgraph.Context) {
	ConfigureGraphProperties(graph, ctx)
}

func TestGraphConfigurator(t *testing.T) {
	nativetest.RunProcessor(t, "GraphConfigurator", func() lgraph.Processor { return configurator{} })
}

// TestProcessorNames compares the processor list of each graph of each
// golden run with the steps that elkjs ran for that graph.
func TestProcessorNames(t *testing.T) {
	files, err := nativetest.Files("")
	if err != nil {
		t.Fatal(err)
	}
	graphs, failed := 0, 0
	for _, file := range files {
		c, err := nativetest.LoadCase(file)
		if err != nil {
			t.Fatal(err)
		}
		for ri, run := range c.Runs {
			// The steps of each graph, by the index of the graph in the
			// states, and the state before the first processor.
			steps := map[int][]string{}
			first := -1
			for _, step := range run.Steps {
				if step.Kind != "hierarchical" && step.Kind != "layout" {
					continue
				}
				if step.Comp != nil && *step.Comp > 0 {
					// Each component runs the same list.
					continue
				}
				if first < 0 && step.Before != nil {
					first = *step.Before
				}
				steps[step.Graph] = append(steps[step.Graph], step.Proc)
			}
			if first < 0 {
				continue
			}
			state, err := run.State(first)
			if err != nil {
				t.Fatal(err)
			}
			built, err := nativetest.Build(state)
			if err != nil {
				t.Fatalf("%s run %d: %v", file, ri, err)
			}
			for index, want := range steps {
				graphs++
				graph := built.Graphs[index]
				names, err := processorNames(graph)
				if err != nil {
					// The list needs a phase that is not ported.
					continue
				}
				if graph.ParentNode != nil {
					// A hierarchy aware processor runs for the root graph
					// only.
					var kept []string
					for _, name := range names {
						if name != "LayerSweepCrossingMinimizer" {
							kept = append(kept, name)
						}
					}
					names = kept
				}
				if strings.Join(names, " ") != strings.Join(want, " ") {
					failed++
					if failed <= 5 {
						t.Errorf("%s run %d graph %d:\n  got  %s\n  want %s", c.Name, ri, index, strings.Join(names, " "), strings.Join(want, " "))
					}
				}
			}
		}
	}
	t.Logf("%d graphs, %d with a processor list that differs from elkjs", graphs, failed)
}

func processorNames(graph *lgraph.LGraph) (names []string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	return ProcessorNames(graph), nil
}

// TestSplitComponents compares the connected components of each golden run
// without hierarchy with the components of elkjs.
func TestSplitComponents(t *testing.T) {
	files, err := nativetest.Files("ComponentsProcessor.split")
	if err != nil {
		t.Fatal(err)
	}
	runs, multi, failed := 0, 0, 0
	for _, file := range files {
		c, err := nativetest.LoadCase(file)
		if err != nil {
			t.Fatal(err)
		}
		for ri, run := range c.Runs {
			for si, step := range run.Steps {
				if step.Proc != "ComponentsProcessor.split" || si == 0 || run.Steps[si-1].After == nil {
					continue
				}
				runs++
				if len(step.AfterList) > 1 {
					multi++
				}
				state, err := run.State(*run.Steps[si-1].After)
				if err != nil {
					t.Fatal(err)
				}
				built, err := nativetest.Build(state)
				if err != nil {
					t.Fatalf("%s run %d: %v", file, ri, err)
				}
				var problem string
				func() {
					defer func() {
						if r := recover(); r != nil {
							problem = fmt.Sprint(r)
						}
					}()
					components := SplitComponents(built.Root())
					if len(components) != len(step.AfterList) {
						problem = fmt.Sprintf("%d components, elkjs has %d", len(components), len(step.AfterList))
						return
					}
					for i, comp := range components {
						want, err := run.State(step.AfterList[i])
						if err != nil {
							problem = err.Error()
							return
						}
						got, err := nativetest.Dump(comp)
						if err != nil {
							problem = err.Error()
							return
						}
						if d := nativetest.Diff(got, want, 6); len(d) > 0 {
							problem = fmt.Sprintf("component %d: %s", i, strings.Join(d, "; "))
							return
						}
					}
				}()
				if problem != "" {
					failed++
					if failed <= 5 {
						t.Errorf("%s run %d: %s", c.Name, ri, problem)
					}
				}
			}
		}
	}
	t.Logf("%d runs (%d with more than one component), %d differ from elkjs", runs, multi, failed)
}
