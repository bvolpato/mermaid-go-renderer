// The choice between the Go ports of the layouts (internal/cytoscape/native)
// and the JavaScript in the interpreter.

package cytoscape

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native"
)

// Mode selects how a layout is computed.
type Mode string

const (
	// ModeGoja runs the JavaScript in the interpreter for each layout.
	ModeGoja Mode = "goja"
	// ModeNative uses the Go port (internal/cytoscape/native) for the
	// inputs that it covers, and the JavaScript for each other input.
	ModeNative Mode = "native"
	// ModeCheck runs both and returns the result of the JavaScript. When
	// the Go port covers the input and its result is not the result of the
	// JavaScript bit for bit, the call returns a *CheckError.
	ModeCheck Mode = "check"
)

// defaultMode is the mode when Env.Mode is empty and the environment
// variable MMDG_CYTOSCAPE is not set to a mode.
const defaultMode = ModeNative

var envMode = sync.OnceValue(func() Mode {
	switch m := Mode(os.Getenv("MMDG_CYTOSCAPE")); m {
	case ModeGoja, ModeNative, ModeCheck:
		return m
	}
	return defaultMode
})

// mode is the mode of a call.
func (e Env) mode() Mode {
	switch e.Mode {
	case ModeGoja, ModeNative, ModeCheck:
		return e.Mode
	}
	return envMode()
}

// CheckError is the error of ModeCheck for a layout whose native result is
// not the result of the JavaScript.
type CheckError struct {
	// Layout is "cose-bilkent" or "fcose".
	Layout string
	// Difference describes the first value that differs.
	Difference string
	// InterpreterErr is the error of the JavaScript, when the Go port laid
	// the input out and the JavaScript failed.
	InterpreterErr error
}

func (e *CheckError) Error() string {
	if e.InterpreterErr != nil {
		return "cytoscape: check: the native " + e.Layout + " layout succeeded and the JavaScript failed: " + e.InterpreterErr.Error()
	}
	return "cytoscape: check: the native " + e.Layout + " layout differs from the JavaScript: " + e.Difference
}

// Stats counts the layouts of the process by how they were computed.
type Stats struct {
	// Native is the number of layouts that the Go port computed, and
	// Fallback the number of layouts of ModeNative and ModeCheck that the Go
	// port did not cover. Interpreter is the number of runs of the
	// JavaScript.
	Native, Fallback, Interpreter int64
	// Reasons counts the fallbacks by the reason of the Go port.
	Reasons map[string]int64
}

var stats struct {
	sync.Mutex
	Stats
}

// ReadStats returns the counters of the process.
func ReadStats() Stats {
	stats.Lock()
	defer stats.Unlock()
	s := stats.Stats
	s.Reasons = make(map[string]int64, len(stats.Reasons))
	for k, v := range stats.Reasons {
		s.Reasons[k] = v
	}
	return s
}

// String lists the counters, with the reasons in the order of their counts.
func (s Stats) String() string {
	text := fmt.Sprintf("%d native, %d not covered, %d runs of the JavaScript", s.Native, s.Fallback, s.Interpreter)
	type row struct {
		reason string
		n      int64
	}
	var rows []row
	for r, n := range s.Reasons {
		rows = append(rows, row{r, n})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].n > rows[j].n || rows[i].n == rows[j].n && rows[i].reason < rows[j].reason
	})
	for _, r := range rows {
		text += fmt.Sprintf("\n%6d  %s", r.n, r.reason)
	}
	return text
}

func countNative() {
	stats.Lock()
	stats.Native++
	stats.Unlock()
}

func countFallback(reason string) {
	stats.Lock()
	stats.Fallback++
	if stats.Reasons == nil {
		stats.Reasons = map[string]int64{}
	}
	stats.Reasons[reason]++
	stats.Unlock()
}

func countInterpreter() {
	stats.Lock()
	stats.Interpreter++
	stats.Unlock()
}

// pageLog records what a layout reads from the page, so that a second run of
// the same layout reads the same values and the page is left as one run
// leaves it.
type pageLog struct {
	env Env
	// draws are the results of Env.Random, in order.
	draws []float64
	// widths are the results of Env.MeasureText.
	widths map[[2]string]float64
}

// recording returns the environment that records.
func (p *pageLog) recording() Env {
	env := p.env
	if p.env.Random != nil {
		env.Random = func() float64 {
			v := p.env.Random()
			p.draws = append(p.draws, v)
			return v
		}
	}
	if p.env.MeasureText != nil {
		env.MeasureText = func(font, text string) float64 {
			w := p.env.MeasureText(font, text)
			if p.widths == nil {
				p.widths = map[[2]string]float64{}
			}
			p.widths[[2]string{font, text}] = w
			return w
		}
	}
	return env
}

// replaying returns the environment of the second run: it reads the recorded
// values first. used counts the draws that the second run takes.
func (p *pageLog) replaying(used *int) Env {
	env := p.env
	if p.env.Random != nil {
		env.Random = func() float64 {
			i := *used
			*used++
			if i < len(p.draws) {
				return p.draws[i]
			}
			return p.env.Random()
		}
	}
	if p.env.MeasureText != nil {
		env.MeasureText = func(font, text string) float64 {
			if w, ok := p.widths[[2]string{font, text}]; ok {
				return w
			}
			return p.env.MeasureText(font, text)
		}
	}
	return env
}

// nativeRun is one run of the Go port: its context and its start. The port
// has the time limit of the interpreter, DefaultTimeout.
type nativeRun struct {
	ctx   context.Context
	start time.Time
}

func newNativeRun(ctx context.Context) *nativeRun {
	return &nativeRun{ctx: ctx, start: time.Now()}
}

// err is the reason for which the run must stop, or nil.
func (r *nativeRun) err() error {
	if r.ctx != nil {
		if err := r.ctx.Err(); err != nil {
			return err
		}
	}
	if time.Since(r.start) > DefaultTimeout {
		return context.DeadlineExceeded
	}
	return nil
}

// env is the environment of the Go port for env.
func (r *nativeRun) env(env Env) *native.Env {
	return &native.Env{
		Random:      env.Random,
		MeasureText: env.MeasureText,
		Trace:       env.trace,
		Cancelled:   func() bool { return r.err() != nil },
	}
}

// interrupted tells whether err is the end of the context and not a result
// of the layout.
func interrupted(ctx context.Context, err error) bool {
	return (ctx != nil && ctx.Err() != nil) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

func sameBits(a, b float64) bool {
	return math.Float64bits(a) == math.Float64bits(b) || (a != a && b != b)
}

// ---- cose-bilkent ----

func nativeCoseBilkent(run *nativeRun, env Env, nodes []Node, edges []Edge) (*CoseBilkentResult, string) {
	in := make([]native.Node, len(nodes))
	for i, n := range nodes {
		in[i] = native.Node{ID: n.ID, Width: n.Width, Height: n.Height, X: n.X, Y: n.Y}
	}
	inEdges := make([]native.Edge, len(edges))
	for i, e := range edges {
		inEdges[i] = native.Edge{ID: e.ID, Start: e.Start, End: e.End}
	}
	res, reason := native.CoseBilkent(run.env(env), in, inEdges)
	if reason != "" {
		return nil, reason
	}
	out := &CoseBilkentResult{}
	for _, n := range res.Nodes {
		out.Nodes = append(out.Nodes, PositionedNode{ID: n.ID, X: n.X, Y: n.Y})
	}
	for _, e := range res.Edges {
		out.Edges = append(out.Edges, PositionedEdge{
			ID: e.ID, Source: e.Source, Target: e.Target,
			StartX: e.StartX, StartY: e.StartY,
			MidX: e.MidX, MidY: e.MidY,
			EndX: e.EndX, EndY: e.EndY,
		})
	}
	return out, ""
}

// diffCoseBilkent returns the first difference between two results, or "".
func diffCoseBilkent(got, want *CoseBilkentResult) string {
	if len(got.Nodes) != len(want.Nodes) || len(got.Edges) != len(want.Edges) {
		return fmt.Sprintf("%d nodes and %d edges, the JavaScript has %d and %d", len(got.Nodes), len(got.Edges), len(want.Nodes), len(want.Edges))
	}
	for i, w := range want.Nodes {
		g := got.Nodes[i]
		if g.ID != w.ID || !sameBits(g.X, w.X) || !sameBits(g.Y, w.Y) {
			return fmt.Sprintf("node %d: %+v, the JavaScript has %+v", i, g, w)
		}
	}
	for i, w := range want.Edges {
		g := got.Edges[i]
		if g.ID != w.ID || g.Source != w.Source || g.Target != w.Target ||
			!sameBits(g.StartX, w.StartX) || !sameBits(g.StartY, w.StartY) ||
			!sameBits(g.MidX, w.MidX) || !sameBits(g.MidY, w.MidY) ||
			!sameBits(g.EndX, w.EndX) || !sameBits(g.EndY, w.EndY) {
			return fmt.Sprintf("edge %d: %+v, the JavaScript has %+v", i, g, w)
		}
	}
	return ""
}

func coseBilkentByMode(ctx context.Context, env Env, nodes []Node, edges []Edge) (*CoseBilkentResult, error) {
	switch env.mode() {
	case ModeNative:
		if ctx != nil && ctx.Err() != nil {
			return nil, fmt.Errorf("cytoscape: layout not started: %w", ctx.Err())
		}
		log := &pageLog{env: env}
		run := newNativeRun(ctx)
		res, reason := nativeCoseBilkent(run, log.recording(), nodes, edges)
		if reason == "" {
			countNative()
			return res, nil
		}
		if err := run.err(); err != nil {
			return nil, fmt.Errorf("cytoscape: layout interrupted: %w", err)
		}
		countFallback(reason)
		used := 0
		return coseBilkent(ctx, log.replaying(&used), runtimeOptions{}, nodes, edges)
	case ModeCheck:
		log := &pageLog{env: env}
		want, err := coseBilkent(ctx, log.recording(), runtimeOptions{}, nodes, edges)
		if err != nil && interrupted(ctx, err) {
			return nil, err
		}
		used := 0
		got, reason := nativeCoseBilkent(newNativeRun(ctx), log.replaying(&used), nodes, edges)
		if reason != "" {
			countFallback(reason)
			return want, err
		}
		countNative()
		if err != nil {
			return nil, &CheckError{Layout: "cose-bilkent", InterpreterErr: err}
		}
		if d := diffCoseBilkent(got, want); d != "" {
			return nil, &CheckError{Layout: "cose-bilkent", Difference: d}
		}
		if used != len(log.draws) {
			return nil, &CheckError{Layout: "cose-bilkent", Difference: fmt.Sprintf("%d Math.random calls, the JavaScript has %d", used, len(log.draws))}
		}
		return want, nil
	}
	return coseBilkent(ctx, env, runtimeOptions{}, nodes, edges)
}

// ---- fcose for the architecture diagram ----

// nativeInput converts the input of Architecture to the type of the Go port.
func nativeInput(in *ArchitectureInput) *native.ArchitectureInput {
	out := &native.ArchitectureInput{
		Config:    native.ArchitectureConfig(in.Config),
		Alignment: native.AlignmentConstraint(in.Alignment),
	}
	for _, n := range in.Groups {
		out.Groups = append(out.Groups, native.ArchitectureNode(n))
	}
	for _, n := range in.Services {
		out.Services = append(out.Services, native.ArchitectureNode(n))
	}
	for _, n := range in.Junctions {
		out.Junctions = append(out.Junctions, native.ArchitectureNode(n))
	}
	for _, e := range in.Edges {
		out.Edges = append(out.Edges, native.ArchitectureEdge(e))
	}
	for _, r := range in.RelativePlacement {
		out.RelativePlacement = append(out.RelativePlacement, native.RelativePlacement(r))
	}
	return out
}

func nativeArchitecture(run *nativeRun, env Env, in *ArchitectureInput) (*ArchitectureResult, string) {
	res, reason := native.Architecture(run.env(env), nativeInput(in))
	if reason != "" {
		return nil, reason
	}
	out := &ArchitectureResult{}
	for _, n := range res.Nodes {
		out.Nodes = append(out.Nodes, ArchitectureNodeResult{ID: n.ID, Type: n.Type, Index: n.Index, BoundingBox: Box(n.BoundingBox), X: n.X, Y: n.Y})
	}
	for _, e := range res.Edges {
		out.Edges = append(out.Edges, ArchitectureEdgeResult{
			ID: e.ID, Index: e.Index,
			SourceEndpoint: Point(e.SourceEndpoint), Midpoint: Point(e.Midpoint), TargetEndpoint: Point(e.TargetEndpoint),
			HasRScratch: e.HasRScratch,
		})
	}
	return out, ""
}

// diffArchitecture returns the first difference between two results, or "".
func diffArchitecture(got, want *ArchitectureResult) string {
	if len(got.Nodes) != len(want.Nodes) || len(got.Edges) != len(want.Edges) {
		return fmt.Sprintf("%d nodes and %d edges, the JavaScript has %d and %d", len(got.Nodes), len(got.Edges), len(want.Nodes), len(want.Edges))
	}
	for i, w := range want.Nodes {
		g := got.Nodes[i]
		gb, wb := g.BoundingBox, w.BoundingBox
		if g.ID != w.ID || g.Type != w.Type || g.Index != w.Index || !sameBits(g.X, w.X) || !sameBits(g.Y, w.Y) ||
			!sameBits(gb.X1, wb.X1) || !sameBits(gb.Y1, wb.Y1) || !sameBits(gb.X2, wb.X2) || !sameBits(gb.Y2, wb.Y2) || !sameBits(gb.W, wb.W) || !sameBits(gb.H, wb.H) {
			return fmt.Sprintf("node %d: %+v, the JavaScript has %+v", i, g, w)
		}
	}
	for i, w := range want.Edges {
		g := got.Edges[i]
		if g.ID != w.ID || g.Index != w.Index || g.HasRScratch != w.HasRScratch ||
			!sameBits(g.SourceEndpoint.X, w.SourceEndpoint.X) || !sameBits(g.SourceEndpoint.Y, w.SourceEndpoint.Y) ||
			!sameBits(g.Midpoint.X, w.Midpoint.X) || !sameBits(g.Midpoint.Y, w.Midpoint.Y) ||
			!sameBits(g.TargetEndpoint.X, w.TargetEndpoint.X) || !sameBits(g.TargetEndpoint.Y, w.TargetEndpoint.Y) {
			return fmt.Sprintf("edge %d: %+v, the JavaScript has %+v", i, g, w)
		}
	}
	return ""
}

func architectureByMode(ctx context.Context, env Env, in *ArchitectureInput) (*ArchitectureResult, error) {
	if in == nil {
		return architecture(ctx, env, runtimeOptions{}, in)
	}
	switch env.mode() {
	case ModeNative:
		if ctx != nil && ctx.Err() != nil {
			return nil, fmt.Errorf("cytoscape: layout not started: %w", ctx.Err())
		}
		log := &pageLog{env: env}
		run := newNativeRun(ctx)
		res, reason := nativeArchitecture(run, log.recording(), in)
		if reason == "" {
			countNative()
			return res, nil
		}
		if err := run.err(); err != nil {
			return nil, fmt.Errorf("cytoscape: layout interrupted: %w", err)
		}
		countFallback(reason)
		used := 0
		return architecture(ctx, log.replaying(&used), runtimeOptions{}, in)
	case ModeCheck:
		log := &pageLog{env: env}
		want, err := architecture(ctx, log.recording(), runtimeOptions{}, in)
		if err != nil && interrupted(ctx, err) {
			return nil, err
		}
		used := 0
		got, reason := nativeArchitecture(newNativeRun(ctx), log.replaying(&used), in)
		if reason != "" {
			countFallback(reason)
			return want, err
		}
		countNative()
		if err != nil {
			return nil, &CheckError{Layout: "fcose", InterpreterErr: err}
		}
		if d := diffArchitecture(got, want); d != "" {
			return nil, &CheckError{Layout: "fcose", Difference: d}
		}
		if used != len(log.draws) {
			return nil, &CheckError{Layout: "fcose", Difference: fmt.Sprintf("%d Math.random calls, the JavaScript has %d", used, len(log.draws))}
		}
		return want, nil
	}
	return architecture(ctx, env, runtimeOptions{}, in)
}
