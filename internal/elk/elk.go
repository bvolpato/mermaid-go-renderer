// This file ports no upstream file. It has the Go API of the layout engine:
// the Engine, its options and its modes.

// Package elk lays out graphs with the Eclipse Layout Kernel (ELK), as
// mermaid 12 does for flowchart, state, class, ER and requirement diagrams.
//
// mermaid calls elkjs 0.9.3: ELK compiled from Java to JavaScript. This
// package runs the same script in goja, a JavaScript engine in Go, so the
// layout is the layout of elkjs and no browser is needed.
//
// Layout takes the ELK JSON graph that mermaid passes to elk.layout and
// returns what JSON.stringify(await elk.layout(graph)) returns, with one
// difference: elkjs adds a member "$H" to each node object, and Layout
// removes it. "$H" is a Java identity hash code. Its value depends on the
// layouts that the JavaScript realm ran before, not on the graph. mermaid
// does not read it. The option Fresh keeps it.
//
// The numbers are the numbers of elkjs in V8, bit for bit. The golden test
// compares about 800 graphs with node. Three parts of the environment are
// fixed, because elkjs reads them and they are not a function of the graph:
//
//   - Math.sin, cos, acos, atan2, log and pow are the functions of
//     internal/js, which give the results of Chromium. goja computes them
//     with the Go math package, which differs in the last bit for 20 to 40
//     percent of the arguments. The layered algorithm does not call them
//     with the options that mermaid sets. The radial and tree algorithms,
//     the Eades force model and the edge wrapping do. Options.Math
//     replaces them.
//   - Math.random gives the same sequence in each layout.
//   - Date.now is constant. elkjs uses it as a random seed when the graph
//     sets elk.randomSeed to 0.
//
// An Engine can use a Go port of the layered algorithm instead
// (internal/elk/native), which gives the same bytes and is much faster. The
// port covers the graphs that mermaid produces as far as its processors are
// ported, and reports each other graph as not covered: the Engine then runs
// elkjs. Options.Mode, or the environment variable MMDG_ELK with the values
// native, goja and check, selects the path. The default is native.
//
// goja interprets the script and V8 compiles it, so a layout costs 10 to 20
// times the CPU instructions of elkjs in V8: about 0.15 s for a flowchart
// with 5 nodes, about 1 s for 20 nodes with subgraphs, about 4 s for 50
// nodes. A runtime of goja is not safe for concurrent use, so an Engine keeps
// a pool of runtimes, and layouts of different graphs run in parallel. The
// script is compiled when the first layout starts, which takes about 0.2 s.
// Warm moves that cost, and the cost of the first layout, off the path of a
// request.
package elk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// DefaultTimeout is the time limit of one layout when Options.Timeout is 0.
// elkjs does not terminate for some option values (for example the layering
// strategy STRETCH_WIDTH), so there is always a limit by default.
const DefaultTimeout = 30 * time.Second

// DefaultIdleTimeout is the time after which an Engine drops an unused
// runtime when Options.IdleTimeout is 0.
const DefaultIdleTimeout = time.Minute

// ErrClosed is the error of a layout on a closed Engine.
var ErrClosed = errors.New("elk: the engine is closed")

// ErrInvalidJSON is the error for a graph that is not valid JSON.
var ErrInvalidJSON = errors.New("elk: the graph is not valid JSON")

// Error is a failure that the layout engine reports: a Java exception of ELK,
// or a JavaScript error.
type Error struct {
	// Class is the Java class name of an ELK exception, for example
	// "org.eclipse.elk.core.UnsupportedConfigurationException", or the name
	// of a JavaScript error, for example "TypeError".
	Class string
	// Message is error.message of the error that elk.layout rejects with.
	// For an ELK exception it is the same text as in a browser.
	Message string
}

func (e *Error) Error() string { return "elk: " + e.Message }

// Math has the Math functions that elkjs calls and that are not exact by
// definition. A nil field selects the function of internal/js, which gives
// the result of Chromium.
type Math struct {
	Sin   func(x float64) float64
	Cos   func(x float64) float64
	Acos  func(x float64) float64
	Log   func(x float64) float64
	Atan2 func(y, x float64) float64
	Pow   func(x, y float64) float64
}

func (m *Math) withDefaults() Math {
	d := Math{Sin: js.Sin, Cos: js.Cos, Acos: js.Acos, Log: js.Log, Atan2: js.Atan2, Pow: js.Pow}
	if m == nil {
		return d
	}
	if m.Sin != nil {
		d.Sin = m.Sin
	}
	if m.Cos != nil {
		d.Cos = m.Cos
	}
	if m.Acos != nil {
		d.Acos = m.Acos
	}
	if m.Log != nil {
		d.Log = m.Log
	}
	if m.Atan2 != nil {
		d.Atan2 = m.Atan2
	}
	if m.Pow != nil {
		d.Pow = m.Pow
	}
	return d
}

// Options configures an Engine. The zero value is the default.
type Options struct {
	// MaxRuntimes bounds the number of JavaScript runtimes, and so the
	// number of layouts that run at the same time. A warm runtime holds
	// about 40 MB. The default is GOMAXPROCS, at most 4.
	MaxRuntimes int
	// Timeout bounds the run time of one layout. 0 selects DefaultTimeout.
	// A negative value means no limit other than the context.
	Timeout time.Duration
	// IdleTimeout drops a runtime that no layout used for this time, to
	// give its memory back. 0 selects DefaultIdleTimeout. A negative value
	// keeps idle runtimes until Close.
	IdleTimeout time.Duration
	// Fresh runs each layout in a new runtime and keeps the "$H" members.
	// The result is then the first layout of a new JavaScript realm, byte
	// for byte. Each layout pays for the load of the engine and for the
	// initialization of its classes, about 0.2 s. Use it for tests.
	Fresh bool
	// Math replaces Math functions of the JavaScript runtime.
	Math *Math
	// Mode selects the layout implementation. The zero value reads the
	// environment variable MMDG_ELK, and an unset variable selects
	// ModeNative.
	Mode Mode
}

// Mode selects how an Engine lays out a graph.
type Mode string

const (
	// ModeGoja runs elkjs in the JavaScript interpreter for each graph.
	ModeGoja Mode = "goja"
	// ModeNative uses the Go port of the layered algorithm
	// (internal/elk/native) for the graphs that it covers, and elkjs for
	// each other graph.
	ModeNative Mode = "native"
	// ModeCheck runs both and returns the result of elkjs. When the Go port
	// covers the graph and its result is not the result of elkjs byte for
	// byte, Layout returns a *CheckError.
	ModeCheck Mode = "check"
)

// CheckError is the error of ModeCheck for a graph whose native layout is
// not the layout of elkjs.
type CheckError struct {
	// Native is the layout of the Go port, and Elkjs the layout of elkjs.
	Native, Elkjs []byte
	// ElkjsErr is the error of elkjs, when the Go port laid the graph out
	// and elkjs rejected it.
	ElkjsErr error
}

func (e *CheckError) Error() string {
	if e.ElkjsErr != nil {
		return "elk: check: the native layout succeeded and elkjs failed: " + e.ElkjsErr.Error()
	}
	n := min(len(e.Native), len(e.Elkjs))
	i := 0
	for i < n && e.Native[i] == e.Elkjs[i] {
		i++
	}
	lo := max(0, i-60)
	clip := func(b []byte) string { return string(b[min(lo, len(b)):min(len(b), i+60)]) }
	return fmt.Sprintf("elk: check: the native layout differs from elkjs at byte %d (lengths %d and %d)\n  native: …%s…\n  elkjs:  …%s…", i, len(e.Native), len(e.Elkjs), clip(e.Native), clip(e.Elkjs))
}

// Stats counts what an Engine did.
type Stats struct {
	// Layouts is the number of layouts that returned, with or without error.
	Layouts int64
	// Created is the number of runtimes that the engine loaded.
	Created int64
	// Discarded is the number of runtimes that the engine dropped because a
	// layout was interrupted or failed inside the JavaScript engine.
	Discarded int64
	// Idle is the number of warm runtimes that wait for a layout.
	Idle int
	// Native is the number of layouts that the Go port computed, and
	// Fallback the number of layouts of ModeNative and ModeCheck that the Go
	// port did not cover.
	Native   int64
	Fallback int64
}

// Engine runs layouts in a bounded pool of JavaScript runtimes. An Engine is
// safe for concurrent use.
type Engine struct {
	opts        Options
	timeout     time.Duration
	idleTimeout time.Duration
	// sem holds one token for each runtime that is in use.
	sem chan struct{}

	mu     sync.Mutex
	idle   []idleRuntime
	closed bool
	stats  Stats
	trim   *time.Timer
}

type idleRuntime struct {
	rt    *jsRuntime
	since time.Time
}

// New returns an Engine. It does not load the layout engine: the first
// layout or Warm does.
func New(opts Options) *Engine {
	if opts.MaxRuntimes <= 0 {
		opts.MaxRuntimes = min(runtime.GOMAXPROCS(0), 4)
	}
	if opts.Mode == "" {
		switch m := Mode(os.Getenv("MMDG_ELK")); m {
		case ModeGoja, ModeCheck:
			opts.Mode = m
		default:
			opts.Mode = ModeNative
		}
	}
	e := &Engine{opts: opts, sem: make(chan struct{}, opts.MaxRuntimes)}
	switch {
	case opts.Timeout == 0:
		e.timeout = DefaultTimeout
	case opts.Timeout > 0:
		e.timeout = opts.Timeout
	}
	switch {
	case opts.IdleTimeout == 0:
		e.idleTimeout = DefaultIdleTimeout
	case opts.IdleTimeout > 0:
		e.idleTimeout = opts.IdleTimeout
	}
	return e
}

var defaultEngine = New(Options{})

// Layout runs elk.layout(graph) and returns the laid-out graph.
// graphJSON is the ELK JSON graph that mermaid passes to elkjs.
//
// Layout uses a shared Engine with the default options.
func Layout(ctx context.Context, graphJSON []byte) ([]byte, error) {
	return defaultEngine.Layout(ctx, graphJSON)
}

// LayoutGraph is Layout followed by Decode.
func LayoutGraph(ctx context.Context, graphJSON []byte) (*Node, error) {
	return defaultEngine.LayoutGraph(ctx, graphJSON)
}

// Warm prepares n runtimes of the shared Engine. See Engine.Warm.
func Warm(ctx context.Context, n int) error {
	return defaultEngine.Warm(ctx, n)
}

// Layout runs elk.layout(graph) and returns the laid-out graph as JSON, with
// the members in the order of JSON.stringify.
//
// The error is *Error when the engine rejects the graph, ErrInvalidJSON when
// the input is not JSON, and wraps the error of the context when the context
// ends or the time limit passes. After such an interrupt the engine drops the
// runtime, and the next layout gets another one.
func (e *Engine) Layout(ctx context.Context, graphJSON []byte) ([]byte, error) {
	if !json.Valid(graphJSON) {
		return nil, ErrInvalidJSON
	}
	// The option Fresh keeps the "$H" members, which only elkjs makes.
	mode := e.opts.Mode
	if e.opts.Fresh {
		mode = ModeGoja
	}
	switch mode {
	case ModeNative:
		if out, ok, _ := native.Layout(graphJSON); ok {
			e.count(true, true)
			return out, nil
		}
		e.count(false, false)
	case ModeCheck:
		nativeOut, ok, _ := native.Layout(graphJSON)
		e.count(ok, false)
		out, err := e.layoutGoja(ctx, graphJSON)
		if ok {
			if err != nil {
				// An interrupt is not a result of elkjs.
				if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrClosed) {
					return nil, err
				}
				return nil, &CheckError{Native: nativeOut, ElkjsErr: err}
			}
			if !bytes.Equal(nativeOut, out) {
				return nil, &CheckError{Native: nativeOut, Elkjs: out}
			}
		}
		return out, err
	}
	return e.layoutGoja(ctx, graphJSON)
}

// count records a layout of the Go port, or a fallback. layout tells that no
// run of elkjs follows, which would count the layout.
func (e *Engine) count(native, layout bool) {
	e.mu.Lock()
	if native {
		e.stats.Native++
	} else {
		e.stats.Fallback++
	}
	if layout {
		e.stats.Layouts++
	}
	e.mu.Unlock()
}

// elkjsObserver is the function of ObserveElkjs.
var elkjsObserver atomic.Pointer[func(in, out []byte)]

// ObserveElkjs sets a function that each Engine calls with the graph and the
// result of each layout that elkjs computed. A nil function ends the
// observation. The tests use it to record the ELK graphs of real diagrams
// with the results of elkjs. The function must be safe for concurrent use.
func ObserveElkjs(fn func(in, out []byte)) {
	if fn == nil {
		elkjsObserver.Store(nil)
		return
	}
	elkjsObserver.Store(&fn)
}

// layoutGoja lays out the graph with elkjs.
func (e *Engine) layoutGoja(ctx context.Context, graphJSON []byte) ([]byte, error) {
	// A "$H" member in the input is rare. The glue needs to know, because it
	// must keep such a member. "\u00" covers a key with an escaped character.
	hasHash := bytes.Contains(graphJSON, []byte("$H")) || bytes.Contains(graphJSON, []byte(`\u00`))

	rt, err := e.acquire(ctx)
	if err != nil {
		return nil, err
	}
	runCtx := ctx
	if e.timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, e.timeout)
		defer cancel()
	}
	out, err := rt.run(runCtx, string(graphJSON), !e.opts.Fresh, hasHash)
	e.release(rt)
	if err != nil {
		return nil, err
	}
	if fn := elkjsObserver.Load(); fn != nil {
		(*fn)(graphJSON, []byte(out))
	}
	return []byte(out), nil
}

// LayoutGraph is Layout followed by Decode.
func (e *Engine) LayoutGraph(ctx context.Context, graphJSON []byte) (*Node, error) {
	out, err := e.Layout(ctx, graphJSON)
	if err != nil {
		return nil, err
	}
	return Decode(out)
}

// warmGraph is the graph that Warm lays out. It makes the engine initialize
// the classes that a mermaid flowchart needs: the layered algorithm with a
// subgraph, labels and an edge across the border of the subgraph.
const warmGraph = `{"id":"root","layoutOptions":{"elk.hierarchyHandling":"INCLUDE_CHILDREN","elk.algorithm":"elk.layered","elk.layered.nodePlacement.strategy":"BRANDES_KOEPF","elk.layered.nodePlacement.bk.fixedAlignment":"BALANCED","elk.layered.mergeEdges":false,"elk.direction":"DOWN","spacing.baseValue":40,"elk.layered.crossingMinimization.forceNodeModelOrder":false,"elk.layered.considerModelOrder.strategy":"NODES_AND_EDGES","elk.layered.unnecessaryBendpoints":true,"elk.layered.cycleBreaking.strategy":"DEPTH_FIRST","elk.layered.layering.strategy":"NETWORK_SIMPLEX","elk.layered.wrapping.multiEdge.improveCuts":true,"elk.layered.wrapping.multiEdge.improveWrappedEdges":true,"elk.layered.edgeRouting.selfLoopDistribution":"EQUALLY","elk.layered.mergeHierarchyEdges":true,"elk.spacing.portsSurrounding":"[top=12,left=12,bottom=12,right=12]"},"children":[{"id":"a","width":80,"height":54},{"id":"b","width":110,"height":54},{"id":"g","children":[{"id":"c","width":90,"height":54},{"id":"d","width":90,"height":54}],"labels":[{"text":"G","width":20,"height":24}],"layoutOptions":{"nodeSize.constraints":"[MINIMUM_SIZE, NODE_LABELS]","nodeSize.minimum":"(28, 0)","spacing.baseValue":24,"elk.layered.spacing.edgeNodeBetweenLayers":30,"elk.spacing.edgeEdge":20,"spacing.nodeNode":50,"elk.padding":"[top=24,left=24,bottom=24,right=24]","nodeLabels.placement":"[H_CENTER V_TOP, INSIDE]","elk.layered.mergeEdges":false,"elk.layered.nodePlacement.bk.fixedAlignment":"BALANCED","elk.layered.nodePlacement.strategy":"BRANDES_KOEPF","elk.layered.cycleBreaking.strategy":"DEPTH_FIRST","elk.layered.nodePlacement.networkSimplex.nodeFlexibility":"PORT_POSITION","elk.spacing.portsSurrounding":"[top=12,left=12,bottom=12,right=12]","elk.hierarchyHandling":"INCLUDE_CHILDREN"}}],"edges":[{"id":"e1","sources":["a"],"targets":["b"],"labels":[{"width":30,"height":21,"text":"x","layoutOptions":{"edgeLabels.inline":"true","edgeLabels.placement":"CENTER"}}]},{"id":"e2","sources":["b"],"targets":["c"],"labels":[{"width":0,"height":0,"text":"","layoutOptions":{"edgeLabels.inline":"true","edgeLabels.placement":"CENTER"}}]},{"id":"e3","sources":["c"],"targets":["d"],"labels":[{"width":0,"height":0,"text":"","layoutOptions":{"edgeLabels.inline":"true","edgeLabels.placement":"CENTER"}}]},{"id":"e4","sources":["d"],"targets":["a"],"labels":[{"width":0,"height":0,"text":"","layoutOptions":{"edgeLabels.inline":"true","edgeLabels.placement":"CENTER"}}]}]}`

// Warm makes n runtimes ready ahead of use: it compiles the script, loads
// the engine and lays out a small graph in each runtime, so that the first
// real layout does not pay for the start. n is capped at MaxRuntimes. The
// runtimes are prepared in parallel. A program can call Warm in a goroutine
// when it knows that a diagram needs ELK.
//
// Warm does nothing for an Engine with the option Fresh.
func (e *Engine) Warm(ctx context.Context, n int) error {
	if e.opts.Fresh {
		return nil
	}
	n = min(n, e.opts.MaxRuntimes)
	e.mu.Lock()
	n -= len(e.idle)
	e.mu.Unlock()
	if n <= 0 {
		return nil
	}
	if _, _, err := programs(); err != nil {
		return err
	}
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			select {
			case e.sem <- struct{}{}:
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			}
			rt, err := e.create()
			if err == nil {
				_, err = rt.run(ctx, warmGraph, true, false)
			}
			if rt != nil {
				e.release(rt)
			} else {
				<-e.sem
			}
			errs <- err
		}()
	}
	var first error
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Stats returns the counters of the engine.
func (e *Engine) Stats() Stats {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.stats
	s.Idle = len(e.idle)
	return s
}

// Close drops the idle runtimes. A layout that runs ends normally. A later
// layout returns ErrClosed.
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closed = true
	e.idle = nil
	if e.trim != nil {
		e.trim.Stop()
		e.trim = nil
	}
}

// acquire takes a token and returns a runtime: the idle runtime that ran
// last, or a new one.
func (e *Engine) acquire(ctx context.Context) (*jsRuntime, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("elk: layout not started: %w", err)
	}
	select {
	case e.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, fmt.Errorf("elk: layout not started: %w", ctx.Err())
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		<-e.sem
		return nil, ErrClosed
	}
	if n := len(e.idle); n > 0 && !e.opts.Fresh {
		rt := e.idle[n-1].rt
		e.idle[n-1] = idleRuntime{}
		e.idle = e.idle[:n-1]
		e.mu.Unlock()
		return rt, nil
	}
	e.mu.Unlock()
	rt, err := e.create()
	if err != nil {
		<-e.sem
		return nil, err
	}
	return rt, nil
}

func (e *Engine) create() (*jsRuntime, error) {
	rt, err := newRuntime(e.opts.Math)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	e.stats.Created++
	e.mu.Unlock()
	return rt, nil
}

// release returns the token. A runtime that is not broken goes back to the
// pool.
func (e *Engine) release(rt *jsRuntime) {
	e.mu.Lock()
	e.stats.Layouts++
	switch {
	case rt.broken:
		e.stats.Discarded++
	case e.closed || e.opts.Fresh:
	default:
		e.idle = append(e.idle, idleRuntime{rt: rt, since: time.Now()})
		if e.idleTimeout > 0 && e.trim == nil {
			e.trim = time.AfterFunc(e.idleTimeout, e.trimIdle)
		}
	}
	e.mu.Unlock()
	<-e.sem
}

// trimIdle drops the runtimes that were idle for IdleTimeout.
func (e *Engine) trimIdle() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.trim = nil
	if e.closed {
		return
	}
	cutoff := time.Now().Add(-e.idleTimeout)
	kept := e.idle[:0]
	var oldest time.Time
	for _, it := range e.idle {
		if it.since.After(cutoff) {
			if oldest.IsZero() || it.since.Before(oldest) {
				oldest = it.since
			}
			kept = append(kept, it)
		}
	}
	for i := len(kept); i < len(e.idle); i++ {
		e.idle[i] = idleRuntime{}
	}
	e.idle = kept
	if len(kept) > 0 {
		e.trim = time.AfterFunc(time.Until(oldest.Add(e.idleTimeout))+time.Millisecond, e.trimIdle)
	}
}
