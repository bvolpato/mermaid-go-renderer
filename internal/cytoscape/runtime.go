// Runs cytoscape.bundle.js in goja, with the environment of env.js and the
// calls of glue.js. See doc.go.

package cytoscape

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"math"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/dop251/goja"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Versions of the embedded code: what mermaid 12.2.0 bundles.
const (
	CytoscapeVersion   = "3.34.0"
	CoseBilkentVersion = "4.1.0"
	FcoseVersion       = "2.2.0"
)

//go:embed cytoscape.bundle.js
var bundleSource string

//go:embed env.js
var envSource string

//go:embed glue.js
var glueSource string

// DefaultTimeout is the time limit of one layout, from the moment the layout
// has a runtime. The context of the call can set an earlier limit. The
// largest layout of the corpus takes about 3 seconds of CPU time: the limit
// is for a script that does not end, on a machine that is busy.
const DefaultTimeout = 2 * time.Minute

// maxCallStack bounds the JavaScript call stack. A layout that recurses
// without end then fails with a RangeError, as in a browser.
const maxCallStack = 20000

// userAgent is navigator.userAgent of the reference browser. cytoscape tests
// it for Internet Explorer and Edge.
const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/156.0.8078.4 Safari/537.36"

// Error is a JavaScript error that cytoscape or a layout throws.
type Error struct {
	// Name is error.name, for example "TypeError".
	Name string
	// Message is error.message.
	Message string
}

func (e *Error) Error() string {
	if e.Name == "" || e.Name == "Error" {
		return "cytoscape: " + e.Message
	}
	return "cytoscape: " + e.Name + ": " + e.Message
}

// Env is the part of the page that a layout reads. A call uses only the
// functions of its own Env, so calls with different Envs can run at the same
// time.
type Env struct {
	// Random is Math.random of the page: the generator of the render
	// context. A layout that draws from it when Random is nil fails.
	Random func() float64
	// Now is the time of the page clock: Date.now.
	Now time.Time
	// MeasureText is context.measureText(text).width of a canvas 2D context
	// whose font property was set to font. cytoscape sets
	// "<font-style> <font-weight> <font-size>px <font-family>", for example
	// "normal normal 16px Helvetica Neue, Helvetica, sans-serif", and
	// measures each line of a label. A layout that measures a label when
	// MeasureText is nil fails.
	MeasureText func(font, text string) float64
	// Mode selects the Go port of the layout or the JavaScript. The zero
	// value reads the environment variable MMDG_CYTOSCAPE, with the values
	// native, goja and check. See the constants.
	Mode Mode

	// trace receives the state of a native layout at the points that the
	// tests compare with the JavaScript.
	trace func(stage string, values []float64)
}

var (
	compileOnce sync.Once
	programs    struct{ bundle, env, glue *goja.Program }
	compileErr  error
)

// compile compiles the scripts one time for the process, on first use.
func compile() error {
	compileOnce.Do(func() {
		for _, s := range []struct {
			name string
			src  string
			dst  **goja.Program
		}{
			{"cytoscape.bundle.js", bundleSource, &programs.bundle},
			{"env.js", envSource, &programs.env},
			{"glue.js", glueSource, &programs.glue},
		} {
			p, err := goja.Compile(s.name, s.src, false)
			if err != nil {
				compileErr = fmt.Errorf("cytoscape: compile %s: %w", s.name, err)
				return
			}
			*s.dst = p
		}
	})
	return compileErr
}

// Warm compiles the embedded scripts, so that the first layout does not pay
// for it. A program can call it in a goroutine when it knows that a diagram
// needs cytoscape.
func Warm() error { return compile() }

// slots bounds the number of runtimes that exist at the same time. A
// runtime holds some tens of megabytes while a layout runs.
var slots = make(chan struct{}, max(runtime.GOMAXPROCS(0), 1))

// jsRuntime is one JavaScript realm with cytoscape loaded. It runs one call
// and is then dropped: cytoscape and the layouts keep state in module
// variables (the random seed of layout-base is one), and mermaid's reference
// output is the output of a new page.
type jsRuntime struct {
	vm  *goja.Runtime
	env Env
	api *goja.Object
	// hostErr is an error of a host function. The JavaScript error that the
	// host function throws stands for it.
	hostErr error
	// randomCalls, measureCalls and sortCalls count the calls of the host
	// functions.
	randomCalls  int
	measureCalls int
	sortCalls    int
}

// runtimeOptions switch off parts of the runtime. Tests use them to show
// what a part is for.
type runtimeOptions struct {
	// gojaMath keeps the Math functions of goja, which are those of the Go
	// math package.
	gojaMath bool
	// gojaSort keeps Array.prototype.sort of goja.
	gojaSort bool
	// patch runs after cytoscape.bundle.js and before glue.js, with the
	// modules of the bundle. The tests load their debug hooks with it.
	patch func(rt *jsRuntime, modules goja.Value) error
}

// hostFailure is the panic value of a host function that cannot continue.
// goja turns a panic with a JavaScript value into a throw.
func (rt *jsRuntime) fail(err error) {
	if rt.hostErr == nil {
		rt.hostErr = err
	}
	panic(rt.vm.NewGoError(err))
}

func newRuntime(env Env, opts runtimeOptions) (rt *jsRuntime, err error) {
	if err := compile(); err != nil {
		return nil, err
	}
	defer func() {
		if r := recover(); r != nil {
			rt, err = nil, fmt.Errorf("cytoscape: load: %v", r)
		}
	}()
	vm := goja.New()
	vm.SetMaxCallStackSize(maxCallStack)
	rt = &jsRuntime{vm: vm, env: env}

	host := vm.NewObject()
	mathObj := vm.NewObject()
	unary := func(f func(float64) float64) func(goja.FunctionCall) goja.Value {
		return func(call goja.FunctionCall) goja.Value {
			return vm.ToValue(f(call.Argument(0).ToFloat()))
		}
	}
	binary := func(f func(float64, float64) float64) func(goja.FunctionCall) goja.Value {
		return func(call goja.FunctionCall) goja.Value {
			a := call.Argument(0).ToFloat()
			b := call.Argument(1).ToFloat()
			return vm.ToValue(f(a, b))
		}
	}
	// Every function of Math that is not exact by definition. The rest
	// (abs, ceil, floor, round, trunc, sign, sqrt, min, max, fround, imul,
	// clz32) has one correct result, and goja computes it.
	for _, m := range []struct {
		name string
		fn   func(goja.FunctionCall) goja.Value
	}{
		{"sin", unary(js.Sin)}, {"cos", unary(js.Cos)}, {"tan", unary(js.Tan)},
		{"asin", unary(js.Asin)}, {"acos", unary(js.Acos)}, {"atan", unary(js.Atan)},
		{"atan2", binary(js.Atan2)},
		{"exp", unary(js.Exp)}, {"expm1", unary(js.Expm1)},
		{"log", unary(js.Log)}, {"log1p", unary(js.Log1p)}, {"log2", unary(js.Log2)}, {"log10", unary(js.Log10)},
		{"cbrt", unary(js.Cbrt)},
		{"sinh", unary(js.Sinh)}, {"cosh", unary(js.Cosh)}, {"tanh", unary(js.Tanh)},
		{"asinh", unary(js.Asinh)}, {"acosh", unary(js.Acosh)}, {"atanh", unary(js.Atanh)},
		{"pow", binary(js.Pow)},
		{"hypot", func(call goja.FunctionCall) goja.Value {
			// Math.hypot converts every argument before it computes.
			values := make([]float64, len(call.Arguments))
			for i, a := range call.Arguments {
				values[i] = a.ToFloat()
			}
			return vm.ToValue(js.Hypot(values...))
		}},
	} {
		if opts.gojaMath {
			break
		}
		if err := mathObj.Set(m.name, m.fn); err != nil {
			return nil, err
		}
	}
	now := float64(env.Now.UnixMilli())
	members := map[string]any{
		"math":        mathObj,
		"random":      rt.random,
		"now":         func(goja.FunctionCall) goja.Value { return vm.ToValue(now) },
		"measureText": rt.measureText,
		"userAgent":   userAgent,
	}
	if !opts.gojaSort {
		members["sort"] = rt.sort
	}
	for name, v := range members {
		if err := host.Set(name, v); err != nil {
			return nil, err
		}
	}

	call := func(prg *goja.Program, what string, args ...goja.Value) (goja.Value, error) {
		v, err := vm.RunProgram(prg)
		if err != nil {
			return nil, fmt.Errorf("cytoscape: load %s: %w", what, jsError(err))
		}
		fn, ok := goja.AssertFunction(v)
		if !ok {
			return nil, fmt.Errorf("cytoscape: %s is not a function", what)
		}
		res, err := fn(goja.Undefined(), args...)
		if err != nil {
			return nil, fmt.Errorf("cytoscape: start %s: %w", what, jsError(err))
		}
		return res, nil
	}
	envObj, err := call(programs.env, "env.js", host)
	if err != nil {
		return nil, err
	}
	modules, err := call(programs.bundle, "cytoscape.bundle.js")
	if err != nil {
		return nil, err
	}
	if opts.patch != nil {
		if err := opts.patch(rt, modules); err != nil {
			return nil, err
		}
	}
	api, err := call(programs.glue, "glue.js", modules, envObj)
	if err != nil {
		return nil, err
	}
	rt.api = api.ToObject(vm)
	return rt, nil
}

// random is Math.random.
func (rt *jsRuntime) random(goja.FunctionCall) goja.Value {
	if rt.env.Random == nil {
		rt.fail(errors.New("cytoscape: the layout calls Math.random, and Env.Random is nil"))
	}
	rt.randomCalls++
	return rt.vm.ToValue(rt.env.Random())
}

// measureText is context.measureText(text).width.
func (rt *jsRuntime) measureText(call goja.FunctionCall) goja.Value {
	if rt.env.MeasureText == nil {
		rt.fail(errors.New("cytoscape: the layout measures a label, and Env.MeasureText is nil"))
	}
	rt.measureCalls++
	return rt.vm.ToValue(rt.env.MeasureText(call.Argument(0).String(), call.Argument(1).String()))
}

// sort is Array.prototype.sort(comparefn) of V8 15 (Chromium 156) for an
// array and a comparator function: PowerSort, with a binary insertion sort
// for short arrays. goja has another stable sort. All stable sorts agree for
// a consistent comparator, and they differ when the comparator is not one,
// because the result then depends on which pairs the sort compares.
//
// As in V8, a hole and an undefined element are not compared: the undefined
// elements go to the end, and the holes after them.
func (rt *jsRuntime) sort(call goja.FunctionCall) goja.Value {
	vm := rt.vm
	rt.sortCalls++
	arr := call.Argument(0).ToObject(vm)
	cmp, ok := goja.AssertFunction(call.Argument(1))
	if !ok {
		panic(vm.NewTypeError("the comparator must be a function"))
	}
	n := int(arr.Get("length").ToInteger())
	vals := make([]goja.Value, 0, n)
	undefined := 0
	for i := 0; i < n; i++ {
		v := arr.Get(strconv.Itoa(i))
		switch {
		case v == nil:
			// A hole.
		case goja.IsUndefined(v):
			undefined++
		default:
			vals = append(vals, v)
		}
	}
	js.SortV8(js.V8PowerSort, vals, func(a, b goja.Value) float64 {
		res, err := cmp(goja.Undefined(), a, b)
		if err != nil {
			// The comparator threw, or the runtime was interrupted.
			panic(err)
		}
		return res.ToFloat()
	})
	i := 0
	for ; i < len(vals); i++ {
		if err := arr.Set(strconv.Itoa(i), vals[i]); err != nil {
			panic(err)
		}
	}
	for ; i < len(vals)+undefined; i++ {
		if err := arr.Set(strconv.Itoa(i), goja.Undefined()); err != nil {
			panic(err)
		}
	}
	for ; i < n; i++ {
		if err := arr.Delete(strconv.Itoa(i)); err != nil {
			panic(err)
		}
	}
	return arr
}

// invoke calls one function of the glue with one argument and returns its
// result. It ends with an error when the context ends.
func (rt *jsRuntime) invoke(ctx context.Context, name string, arg goja.Value) (res *goja.Object, err error) {
	fn, ok := goja.AssertFunction(rt.api.Get(name))
	if !ok {
		return nil, fmt.Errorf("cytoscape: the glue has no function %s", name)
	}
	defer func() {
		if r := recover(); r != nil {
			res, err = nil, fmt.Errorf("cytoscape: internal error: %v", r)
		}
	}()
	stop := context.AfterFunc(ctx, func() { rt.vm.Interrupt(ctx.Err()) })
	defer stop()
	v, err := fn(goja.Undefined(), arg)
	if err != nil {
		var interrupted *goja.InterruptedError
		var overflow *goja.StackOverflowError
		switch {
		case errors.As(err, &interrupted):
			if cerr := ctx.Err(); cerr != nil {
				return nil, fmt.Errorf("cytoscape: layout interrupted: %w", cerr)
			}
			return nil, errors.New("cytoscape: layout interrupted")
		case errors.As(err, &overflow):
			return nil, &Error{Name: "RangeError", Message: "Maximum call stack size exceeded"}
		case rt.hostErr != nil:
			return nil, rt.hostErr
		}
		return nil, jsError(err)
	}
	obj, ok := v.(*goja.Object)
	if !ok {
		return nil, errors.New("cytoscape: the glue did not return an object")
	}
	return obj, nil
}

// withRuntime makes a runtime, runs fn with it and drops it.
func withRuntime(ctx context.Context, env Env, opts runtimeOptions, fn func(ctx context.Context, rt *jsRuntime) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case slots <- struct{}{}:
	case <-ctx.Done():
		return fmt.Errorf("cytoscape: layout not started: %w", ctx.Err())
	}
	defer func() { <-slots }()
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()
	rt, err := newRuntime(env, opts)
	if err != nil {
		return err
	}
	return fn(ctx, rt)
}

// jsError converts an error of goja to *Error.
func jsError(err error) error {
	var ex *goja.Exception
	if !errors.As(err, &ex) {
		return fmt.Errorf("cytoscape: %w", err)
	}
	v := ex.Value()
	e := &Error{}
	if obj, ok := v.(*goja.Object); ok {
		if m := obj.Get("message"); m != nil && !goja.IsUndefined(m) && !goja.IsNull(m) {
			e.Message = m.String()
		} else {
			e.Message = v.String()
		}
		if n := obj.Get("name"); n != nil && !goja.IsUndefined(n) && !goja.IsNull(n) {
			e.Name = n.String()
		}
	} else if v != nil {
		e.Message = v.String()
	}
	return e
}

// ---- values ----

// PerElement is a layout option that cytoscape reads as a function of an
// element, for example idealEdgeLength of fcose. The key is the element id.
type PerElement map[string]float64

// toValue converts a dynamic value of the package js (nil, js.Null, bool,
// float64, int, string, []any, *js.Obj[any]) or a PerElement to a JavaScript
// value. nil is undefined.
func (rt *jsRuntime) toValue(v any) (goja.Value, error) {
	vm := rt.vm
	switch x := v.(type) {
	case nil:
		return goja.Undefined(), nil
	case bool:
		return vm.ToValue(x), nil
	case float64:
		return vm.ToValue(x), nil
	case int:
		return vm.ToValue(float64(x)), nil
	case string:
		return vm.ToValue(x), nil
	case []any:
		items := make([]any, len(x))
		for i, e := range x {
			ev, err := rt.toValue(e)
			if err != nil {
				return nil, err
			}
			items[i] = ev
		}
		return vm.NewArray(items...), nil
	case []string:
		items := make([]any, len(x))
		for i, e := range x {
			items[i] = vm.ToValue(e)
		}
		return vm.NewArray(items...), nil
	case *js.Obj[any]:
		if x == nil {
			return goja.Undefined(), nil
		}
		obj := vm.NewObject()
		for _, k := range x.Keys() {
			ev, err := rt.toValue(x.Get(k))
			if err != nil {
				return nil, err
			}
			if err := obj.Set(k, ev); err != nil {
				return nil, err
			}
		}
		return obj, nil
	case PerElement:
		table := vm.NewObject()
		for k, f := range x {
			if err := table.Set(k, f); err != nil {
				return nil, err
			}
		}
		obj := vm.NewObject()
		if err := obj.Set("$perElement", table); err != nil {
			return nil, err
		}
		return obj, nil
	}
	if js.IsNullish(v) {
		return goja.Null(), nil
	}
	return nil, fmt.Errorf("cytoscape: a value of type %T cannot be passed to JavaScript", v)
}

// object builds a JavaScript object from key and value pairs, in order. A
// value that is nil stays out: the property is undefined.
func (rt *jsRuntime) object(pairs ...any) *goja.Object {
	obj := rt.vm.NewObject()
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] == nil {
			continue
		}
		if err := obj.Set(pairs[i].(string), pairs[i+1]); err != nil {
			panic(err)
		}
	}
	return obj
}

// optional is a string property that is undefined when the string is empty.
func optional(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// num reads a number property. A property that is undefined or is not a
// number is NaN.
func num(obj *goja.Object, name string) float64 {
	v := obj.Get(name)
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return math.NaN()
	}
	if _, ok := v.Export().(string); ok {
		return math.NaN()
	}
	return v.ToFloat()
}

// index reads a property that is an index. A property that is undefined is -1.
func index(obj *goja.Object, name string) int {
	f := num(obj, name)
	if f != f || f < 0 || f > 1<<31 {
		return -1
	}
	return int(f)
}

func str(obj *goja.Object, name string) string {
	v := obj.Get(name)
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return ""
	}
	return v.String()
}

func boolean(obj *goja.Object, name string) bool {
	v := obj.Get(name)
	return v != nil && v.ToBoolean()
}

// items returns the elements of an array property.
func (rt *jsRuntime) items(obj *goja.Object, name string) []*goja.Object {
	v := obj.Get(name)
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil
	}
	arr := v.ToObject(rt.vm)
	n := int(arr.Get("length").ToInteger())
	out := make([]*goja.Object, n)
	for i := range out {
		out[i] = arr.Get(strconv.Itoa(i)).ToObject(rt.vm)
	}
	return out
}

func point(obj *goja.Object, name string) Point {
	v := obj.Get(name)
	p, ok := v.(*goja.Object)
	if !ok {
		return Point{X: math.NaN(), Y: math.NaN()}
	}
	return Point{X: num(p, "x"), Y: num(p, "y")}
}
