package nodesize

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// The golden file testdata/nodespacing.json.gz has random graphs that use the
// options of the node size calculation. tools/reference/gen-elk-nodespacing.mjs
// writes it from elkjs 0.9.3 and documents the format.

// exact is a double that the golden file keeps exact: a JSON number, or one
// of the texts "-0", "NaN", "Infinity" and "-Infinity".
type exact float64

func (e *exact) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		switch s {
		case "-0":
			*e = exact(math.Copysign(0, -1))
		case "NaN":
			*e = exact(math.NaN())
		case "Infinity":
			*e = exact(math.Inf(1))
		case "-Infinity":
			*e = exact(math.Inf(-1))
		default:
			return fmt.Errorf("unknown number %q", s)
		}
		return nil
	}
	f, err := strconv.ParseFloat(string(data), 64)
	*e = exact(f)
	return err
}

type oracleNode struct {
	Size      [2]exact `json:"size"`
	SizeAll   [2]exact `json:"sizeAll"`
	PadH      [4]exact `json:"padH"`
	PadV      [4]exact `json:"padV"`
	PadSingle [4]exact `json:"padSingle"`
}

type oracleStep struct {
	Kind   string `json:"kind"`
	Proc   string `json:"proc"`
	Graph  int    `json:"graph"`
	Before int    `json:"before"`
	After  int    `json:"after"`
	// The direct calls before LabelAndNodeSizeProcessor.
	Nodes []oracleNode `json:"nodes"`
	// The direct calls before InnermostNodeMarginCalculator.
	Margins   [][4]exact `json:"margins"`
	MarginsNS [][4]exact `json:"marginsNS"`
	Single    [][4]exact `json:"single"`
}

type oracleRun struct {
	Elk    string            `json:"elk"`
	Steps  []*oracleStep     `json:"steps"`
	States []json.RawMessage `json:"states"`
}

type oracleCase struct {
	Name string       `json:"name"`
	Runs []*oracleRun `json:"runs"`
}

func loadOracle(t *testing.T) []*oracleCase {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "nodespacing.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []*oracleCase `json:"cases"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&file); err != nil {
		t.Fatal(err)
	}
	filter := os.Getenv("ELK_PHASE_CASE")
	var out []*oracleCase
	for _, c := range file.Cases {
		if filter == "" || strings.Contains(c.Name, filter) {
			out = append(out, c)
		}
	}
	return out
}

// testSelfLoopHolder stands for the SelfLoopHolder of the self loop
// processors, which the harness keeps as an opaque value. It has the self
// loop ports that the stored state has for the holder.
type testSelfLoopHolder struct {
	// stored is the value that the harness made for the property.
	stored   any
	incoming map[*lgraph.LPort][]*lgraph.LEdge
	outgoing map[*lgraph.LPort][]*lgraph.LEdge
	known    map[*lgraph.LPort]bool
}

func (h *testSelfLoopHolder) SelfLoopEdgesOfPort(port *lgraph.LPort) (incoming, outgoing []*lgraph.LEdge, ok bool) {
	return h.incoming[port], h.outgoing[port], h.known[port]
}

// storedObjects finds the Java objects of a stored state: the values
// {"$o": id, "c": class, "f": fields}. A second reference to an object is
// {"$o": id} alone.
func storedObjects(state any, objects map[float64]map[string]any) {
	switch x := state.(type) {
	case []any:
		for _, e := range x {
			storedObjects(e, objects)
		}
	case map[string]any:
		if id, ok := x["$o"].(float64); ok {
			if _, full := x["f"]; full {
				objects[id] = x
			}
		}
		for _, e := range x {
			storedObjects(e, objects)
		}
	}
}

// newTestSelfLoopHolder reads the map slPorts of a stored SelfLoopHolder.
func newTestSelfLoopHolder(raw any, objects map[float64]map[string]any, built *nativetest.Built) (*testSelfLoopHolder, error) {
	fields := func(v any) (map[string]any, error) {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("the self loop holder has %T where an object is expected", v)
		}
		if id, ok := m["$o"].(float64); ok {
			m = objects[id]
		}
		f, ok := m["f"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("the self loop holder has an object without fields")
		}
		return f, nil
	}
	list := func(v any, key string) []any {
		m, _ := v.(map[string]any)
		l, _ := m[key].([]any)
		return l
	}
	element := func(v any, key string, n int) (int, error) {
		m, _ := v.(map[string]any)
		i, ok := m[key].(float64)
		if !ok || i < 0 || int(i) >= n {
			return 0, fmt.Errorf("the self loop holder has %v where a reference %s is expected", v, key)
		}
		return int(i), nil
	}
	h := &testSelfLoopHolder{
		incoming: map[*lgraph.LPort][]*lgraph.LEdge{},
		outgoing: map[*lgraph.LPort][]*lgraph.LEdge{},
		known:    map[*lgraph.LPort]bool{},
	}
	holder, err := fields(raw)
	if err != nil {
		return nil, err
	}
	for _, row := range list(holder["slPorts"], "$map") {
		pair, ok := row.([]any)
		if !ok || len(pair) != 2 {
			return nil, fmt.Errorf("the self loop holder has a bad entry in slPorts")
		}
		pi, err := element(pair[0], "$p", len(built.Ports))
		if err != nil {
			return nil, err
		}
		port := built.Ports[pi]
		slPort, err := fields(pair[1])
		if err != nil {
			return nil, err
		}
		h.known[port] = true
		for _, dir := range []struct {
			field string
			into  map[*lgraph.LPort][]*lgraph.LEdge
		}{{"incomingSLEdges", h.incoming}, {"outgoingSLEdges", h.outgoing}} {
			for _, e := range list(slPort[dir.field], "$list") {
				slEdge, err := fields(e)
				if err != nil {
					return nil, err
				}
				ei, err := element(slEdge["lEdge"], "$e", len(built.Edges))
				if err != nil {
					return nil, err
				}
				dir.into[port] = append(dir.into[port], built.Edges[ei])
			}
		}
	}
	return h, nil
}

// wrapSelfLoopHolders gives each node with a stored self loop holder a
// holder that the port adapter can read. The returned function puts the
// values of the harness back, for the dump.
func wrapSelfLoopHolders(built *nativetest.Built, state map[string]any) (restore func(), err error) {
	var wrapped []*lgraph.LNode
	var holders []*testSelfLoopHolder
	var objects map[float64]map[string]any
	nodes, _ := state["n"].([]any)
	for i, n := range nodes {
		node, _ := n.(map[string]any)
		props, _ := node["pr"].(map[string]any)
		raw, ok := props["selfLoopHolder"]
		if !ok || i >= len(built.Nodes) {
			continue
		}
		if objects == nil {
			objects = map[float64]map[string]any{}
			storedObjects(state, objects)
		}
		h, err := newTestSelfLoopHolder(raw, objects, built)
		if err != nil {
			return nil, err
		}
		h.stored, _ = prop.Lookup(built.Nodes[i], iprops.SelfLoopHolder)
		prop.Set[any](built.Nodes[i], iprops.SelfLoopHolder, h)
		wrapped = append(wrapped, built.Nodes[i])
		holders = append(holders, h)
	}
	return func() {
		for i, node := range wrapped {
			prop.Set(node, iprops.SelfLoopHolder, holders[i].stored)
		}
	}, nil
}

// sameBits compares two doubles bit for bit. Each NaN is equal to each other
// NaN: JavaScript has one NaN, and the golden file has the text "NaN".
func sameBits(got float64, want exact) bool {
	if got != got {
		return float64(want) != float64(want)
	}
	return math.Float64bits(got) == math.Float64bits(float64(want))
}

func spacingDiff(got *emath.Spacing, want [4]exact) string {
	if sameBits(got.Top, want[0]) && sameBits(got.Right, want[1]) && sameBits(got.Bottom, want[2]) && sameBits(got.Left, want[3]) {
		return ""
	}
	return fmt.Sprintf("[top right bottom left] is [%v %v %v %v], elkjs has %v", got.Top, got.Right, got.Bottom, got.Left, want)
}

// buildStep builds the graph of the state before a step.
func buildStep(run *oracleRun, step *oracleStep) (built *nativetest.Built, graph *lgraph.LGraph, restore func(), err error) {
	states := &nativetest.Run{States: run.States}
	before, err := states.State(step.Before)
	if err != nil {
		return nil, nil, nil, err
	}
	built, err = nativetest.Build(before)
	if err != nil {
		return nil, nil, nil, err
	}
	if step.Graph < 0 || step.Graph >= len(built.Graphs) {
		return nil, nil, nil, fmt.Errorf("the step names the graph %d, the state has %d graphs", step.Graph, len(built.Graphs))
	}
	restore, err = wrapSelfLoopHolders(built, before)
	if err != nil {
		return nil, nil, nil, err
	}
	return built, built.Graphs[step.Graph], restore, nil
}

// checkProcessor runs the processor on the state before the step and
// compares the result with the state after the step.
func checkProcessor(run *oracleRun, step *oracleStep, p lgraph.Processor) []string {
	built, graph, restore, err := buildStep(run, step)
	if err != nil {
		return []string{err.Error()}
	}
	if err := nativetest.Process(p, graph, lgraph.NewContext()); err != nil {
		return []string{err.Error()}
	}
	restore()
	got, err := nativetest.Dump(built.Root())
	if err != nil {
		return []string{err.Error()}
	}
	want, err := (&nativetest.Run{States: run.States}).State(step.After)
	if err != nil {
		return []string{err.Error()}
	}
	return nativetest.Diff(got, want, 8)
}

// bailed runs fn and returns the reason when fn stops with lgraph.Bail.
func bailed(fn func()) (reason string) {
	defer func() {
		if r := recover(); r != nil {
			b, ok := r.(*lgraph.BailError)
			if !ok {
				panic(r)
			}
			reason = "the code stopped: " + b.Reason
		}
	}()
	fn()
	return ""
}

// checkDirectSizes makes the direct calls that the generator made before
// LabelAndNodeSizeProcessor, in the same order, and compares the results.
func checkDirectSizes(run *oracleRun, step *oracleStep) (problems []string) {
	_, graph, _, err := buildStep(run, step)
	if err != nil {
		return []string{err.Error()}
	}
	reason := bailed(func() {
		ga := adapters.AdaptLGraphFiltered(graph, true, true, func(node *lgraph.LNode) bool { return node.Type == lgraph.NodeTypeNormal })
		nodes := ga.Nodes()
		if len(nodes) != len(step.Nodes) {
			problems = append(problems, fmt.Sprintf("the adapted graph has %d nodes, elkjs has %d", len(nodes), len(step.Nodes)))
			return
		}
		for i, na := range nodes {
			want := step.Nodes[i]
			size := nodespacing.ProcessNode(ga, na, false, true)
			if !sameBits(size.X, want.Size[0]) || !sameBits(size.Y, want.Size[1]) {
				problems = append(problems, fmt.Sprintf("node %d: ProcessNode(graph, node, false, true) is (%v, %v), elkjs has %v", i, size.X, size.Y, want.Size))
			}
			sizeAll := nodespacing.ProcessNode(ga, na, false, false)
			if !sameBits(sizeAll.X, want.SizeAll[0]) || !sameBits(sizeAll.Y, want.SizeAll[1]) {
				problems = append(problems, fmt.Sprintf("node %d: ProcessNode(graph, node, false, false) is (%v, %v), elkjs has %v", i, sizeAll.X, sizeAll.Y, want.SizeAll))
			}
			padH := nodespacing.ComputeInsideNodeLabelPadding(ga, na, core.DirectionRight)
			if d := spacingDiff(&padH.Spacing, want.PadH); d != "" {
				problems = append(problems, fmt.Sprintf("node %d: ComputeInsideNodeLabelPadding(RIGHT) %s", i, d))
			}
			padV := nodespacing.ComputeInsideNodeLabelPadding(ga, na, core.DirectionDown)
			if d := spacingDiff(&padV.Spacing, want.PadV); d != "" {
				problems = append(problems, fmt.Sprintf("node %d: ComputeInsideNodeLabelPadding(DOWN) %s", i, d))
			}
			// The adapter of a single node has no graph, so the spacings
			// are the default values of the options.
			single := adapters.AdaptLNode(na.(*adapters.LNodeAdapter).Element(), false)
			padSingle := nodespacing.ComputeInsideNodeLabelPadding(nil, single, core.DirectionRight)
			if d := spacingDiff(&padSingle.Spacing, want.PadSingle); d != "" {
				problems = append(problems, fmt.Sprintf("node %d: ComputeInsideNodeLabelPadding(RIGHT) without a graph %s", i, d))
			}
		}
	})
	if reason != "" {
		problems = append(problems, reason)
	}
	return problems
}

// checkDirectMargins makes the direct calls that the generator made before
// InnermostNodeMarginCalculator, in the same order, and compares the margins.
func checkDirectMargins(run *oracleRun, step *oracleStep) (problems []string) {
	_, graph, _, err := buildStep(run, step)
	if err != nil {
		return []string{err.Error()}
	}
	var all []*lgraph.LNode
	for _, layer := range graph.Layers {
		all = append(all, layer.Nodes...)
	}
	compare := func(what string, want [][4]exact) {
		if len(all) != len(want) {
			problems = append(problems, fmt.Sprintf("%s: the layers have %d nodes, elkjs has %d", what, len(all), len(want)))
			return
		}
		for i, node := range all {
			if d := spacingDiff(&node.Margin.Spacing, want[i]); d != "" {
				problems = append(problems, fmt.Sprintf("%s: the margin of the node %d %s", what, i, d))
			}
		}
	}
	reason := bailed(func() {
		nodespacing.CalculateNodeMargins(adapters.AdaptLGraphTransparent(graph, false))
		compare("CalculateNodeMargins(adapt(graph, false))", step.Margins)
		nodespacing.CalculateNodeMargins(adapters.AdaptLGraphTransparent(graph, true))
		compare("CalculateNodeMargins(adapt(graph, true))", step.MarginsNS)
		calc := nodespacing.GetNodeMarginCalculator(adapters.AdaptLGraph(graph))
		if len(all) != len(step.Single) {
			problems = append(problems, fmt.Sprintf("ProcessNode: the layers have %d nodes, elkjs has %d", len(all), len(step.Single)))
			return
		}
		for i, node := range all {
			calc.ProcessNode(adapters.AdaptLNode(node, false))
			if d := spacingDiff(&node.Margin.Spacing, step.Single[i]); d != "" {
				problems = append(problems, fmt.Sprintf("ProcessNode(adapt(node, false)): the margin of the node %d %s", i, d))
			}
		}
	})
	if reason != "" {
		problems = append(problems, reason)
	}
	return problems
}

// TestNodeSpacingOracle tests the two processors and the entry points of the
// node spacing code against elkjs, for random graphs that use the options of
// the node size calculation.
func TestNodeSpacingOracle(t *testing.T) {
	cases := loadOracle(t)
	if len(cases) == 0 {
		t.Fatal("the golden file has no case")
	}
	processors := map[string]lgraph.Factory{
		"LabelAndNodeSizeProcessor":     func() lgraph.Processor { return NewLabelAndNodeSizeProcessor() },
		"InnermostNodeMarginCalculator": func() lgraph.Processor { return NewInnermostNodeMarginCalculator() },
	}
	steps, direct, failed := 0, 0, 0
	for _, c := range cases {
		for ri, run := range c.Runs {
			for si, step := range run.Steps {
				factory := processors[step.Proc]
				if factory == nil {
					t.Fatalf("%s: the step %s has no processor", c.Name, step.Proc)
				}
				steps++
				var problems []string
				for _, d := range checkProcessor(run, step, factory()) {
					problems = append(problems, "after the processor: "+d)
				}
				if step.Proc == "LabelAndNodeSizeProcessor" {
					direct += 5 * len(step.Nodes)
					problems = append(problems, checkDirectSizes(run, step)...)
				} else {
					direct += len(step.Margins) + len(step.MarginsNS) + len(step.Single)
					problems = append(problems, checkDirectMargins(run, step)...)
				}
				if len(problems) == 0 {
					continue
				}
				failed++
				if failed <= 6 {
					if len(problems) > 10 {
						problems = problems[:10]
					}
					t.Errorf("%s, run %d, step %d (%s, graph %d):\n  %s", c.Name, ri, si, step.Proc, step.Graph, strings.Join(problems, "\n  "))
				}
			}
		}
	}
	t.Logf("%d cases, %d steps, %d results of direct calls, %d steps differ from elkjs", len(cases), steps, direct, failed)
	if failed > 0 {
		t.Errorf("%d of %d steps differ from elkjs", failed, steps)
	}
}
