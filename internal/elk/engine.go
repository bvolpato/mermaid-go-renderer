// Ported from elkjs 0.9.3, in part: lib/elk-api.js and the FakeWorker of
// lib/elk-worker.js (EPL-2.0).
//
// Runs elkjs/elk-worker.min.js, the layout engine of elkjs 0.9.3, in goja.
// The glue replaces lib/elk-api.js of elkjs (the class ELK and its
// PromisedWorker) and the FakeWorker of the worker script. The seven text
// patches of this file replace functions of the worker script when the
// script is compiled. The patched functions are helpers of the Java runtime
// that GWT emits (GWT has the Apache License 2.0), inside the file that
// elkjs distributes under the Eclipse Public License 2.0.
//
// The notice of lib/elk-api.js of elkjs 0.9.3:
//
//	Copyright (c) 2017 Kiel University and others.
//
//	This program and the accompanying materials are made
//	available under the terms of the Eclipse Public License 2.0
//	which is available at https://www.eclipse.org/legal/epl-2.0/
//
//	SPDX-License-Identifier: EPL-2.0
//
// This file is a derivative work of elkjs. It is made available under the
// Eclipse Public License 2.0. LICENSE-EPL-2.0.txt in this directory has the
// license, and NOTICE in this directory says which files it covers.

package elk

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/dop251/goja"
)

// EngineVersion is the version of the embedded layout engine.
const EngineVersion = "elkjs 0.9.3"

// engineSHA256 is the SHA-256 of elkjs/elk-worker.min.js. The file is
// lib/elk-worker.min.js of the npm package elkjs 0.9.3, unchanged. The file
// lib/elk.bundled.js, which mermaid imports, contains the same bytes.
const engineSHA256 = "114c4f17aea4b297a2d3dd6c86bdfccb45e12040da964ea8f9766345fce4f538"

//go:embed elkjs/elk-worker.min.js
var workerSource string

// A patch replaces one function of the engine with a faster one that has the
// same result. The engine file stays as elkjs ships it. The patches apply to
// the source text when the process compiles the script.
//
// The functions are helpers of the Java runtime that GWT emits, not layout
// code. goja has no compiler, so a helper that runs for each cast or each map
// lookup costs much more than in V8. TestUnpatchedEngine runs the script
// without patches against the same golden results.
//
// The gain is counted in CPU instructions for a warm layout of the benchmark
// graph with 20 nodes: 14.3e9 without patches, 9.4e9 with the first patch,
// 8.3e9 with all of them. V8 needs 0.5e9.
type patch struct {
	name string
	// old is the text of the function in elk-worker.min.js. It must occur
	// exactly one time.
	old string
	// new is the replacement.
	new string
}

var patches = []patch{
	{
		// java.lang.String.hashCode. The engine calls it for each lookup in a
		// property map, because the hash code of a layout option is the hash
		// code of its identifier. The loop calls charCodeAt for each
		// character, and goja wraps the string in an object for each call.
		// $$strHash is the same function in Go.
		name: "String.hashCode",
		old:  `function ohb(a){var b,c;b=0;for(c=0;c<a.length;c++){b=(b<<5)-b+(BFb(c,a.length),a.charCodeAt(c))|0}return b}`,
		new:  `function ohb(a){return $$strHash(a)}`,
	},
	{
		// InternalHashCodeMap.getEntry. When the map has no chain for the
		// hash code, the original makes an empty Java array and searches it.
		// The patch returns null at once, which is the result of that search.
		name: "InternalHashCodeMap.getEntry",
		old:  `function qtb(a,b){return otb(a,b,ptb(a,a.b.Ce(b)))}`,
		new:  `function qtb(a,b){var c=a.a.get(a.b.Ce(b));return c==null?null:otb(a,b,c)}`,
	},
	{
		// Cast.castTo, the check of a Java cast. The patch tests the common
		// case, an object whose class can be cast to the type, without the
		// calls of canCast, instanceOfString and checkCriticalType. Each other
		// case runs the original check.
		name: "Cast.castTo",
		old:  `function RD(a,b){CFb(a==null||QD(a,b));return a}`,
		new:  `function RD(a,b){var c;if(a!=null&&!(typeof a!=='string'&&(c=a.Sm)&&c[b])){CFb(QD(a,b))}return a}`,
	},
	{
		// Cast.instanceOf, in the same way.
		name: "Cast.instanceOf",
		old:  `function ZD(a,b){return a!=null&&QD(a,b)}`,
		new:  `function ZD(a,b){var c;return a!=null&&(typeof a!=='string'&&(c=a.Sm)?!!c[b]:QD(a,b))}`,
	},
	{
		// Object.hashCode with the dispatch on the JavaScript type. The patch
		// reads typeof one time, where the original calls one function for
		// each type that it tests.
		name: "Object.hashCode dispatch",
		old:  `function tb(a){return bE(a)?ohb(a):_D(a)?Nfb(a):$D(a)?Jeb(a):YD(a)?a.Hb():aD(a)?kFb(a):Iz(a)}`,
		new:  `function tb(a){var b=typeof a;return b==='string'?ohb(a):b==='number'?Nfb(a):b==='boolean'?Jeb(a):!Array.isArray(a)&&a.Tm===keb?a.Hb():aD(a)?kFb(a):Iz(a)}`,
	},
	{
		// Object.equals with the dispatch on the JavaScript type.
		name: "Object.equals dispatch",
		old:  `function pb(a,b){return bE(a)?lhb(a,b):_D(a)?Lfb(a,b):$D(a)?(uFb(a),dE(a)===dE(b)):YD(a)?a.Fb(b):aD(a)?mb(a,b):Hz(a,b)}`,
		new:  `function pb(a,b){var c=typeof a;return c==='string'?lhb(a,b):c==='number'?Lfb(a,b):c==='boolean'?(uFb(a),dE(a)===dE(b)):!Array.isArray(a)&&a.Tm===keb?a.Fb(b):aD(a)?mb(a,b):Hz(a,b)}`,
	},
	{
		// The next method of the iterator of ArrayList. The patch reads the
		// index and the array one time. It writes the fields in the order of
		// the original.
		name: "ArrayList iterator next",
		old:  `function ynb(a){sFb(a.a<a.c.c.length);a.b=a.a++;return a.c.c[a.b]}`,
		new:  `function ynb(a){var b=a.a,c=a.c.c;if(!(b<c.length)){sFb(false)}a.a=b+1;a.b=b;return c[b]}`,
	},
}

// The worker script becomes the body of a function. A script at the top
// level keeps its functions in the global object, and goja then looks each
// of them up by name for each call. In a function they are local variables
// with a fixed slot. Measured on a graph with 7 nodes: a warm layout takes
// 13 percent less time.
//
// "self" is the global scope of a web worker. The script takes it as $wnd,
// because goja has no window and no global, and it installs self.onmessage,
// because there is no document. The script starts with 'use strict', which
// stays the first statement of the function.
const (
	workerPrefix = "(function(self,$$strHash){"
	workerSuffix = "\n})"
)

// bootSource connects the worker to Go. It returns the object that Go calls.
const bootSource = `(function (worker, native) {
  'use strict';
  // The default of the constructor of the class ELK in elk-api.js. mermaid
  // uses that default.
  var algorithms = ['layered', 'stress', 'mrtree', 'radial', 'force', 'disco', 'sporeOverlap', 'sporeCompaction', 'rectpacking'];
  // The engine reads Math from $wnd. This copy has Go functions for the
  // members whose results in goja can differ from a browser in the last bit
  // (see Options.Math). The other members that the engine reads are exact.
  var math = {};
  var names = Object.getOwnPropertyNames(Math);
  for (var i = 0; i < names.length; i++) {
    math[names[i]] = Math[names[i]];
  }
  math.sin = native.sin;
  math.cos = native.cos;
  math.acos = native.acos;
  math.atan2 = native.atan2;
  math.log = native.log;
  math.pow = native.pow;
  math.random = native.random;
  Date.now = native.now;

  var message;
  var self = {
    Math: math,
    Array: Array,
    Date: Date,
    Error: Error,
    Function: Function,
    setTimeout: function () { return 0; },
    clearTimeout: function () {},
    postMessage: function (m) { message = m; }
  };
  worker(self, native.strHash);
  var dispatch = self.onmessage;
  if (typeof dispatch !== 'function') {
    throw new Error('the elkjs worker did not install onmessage');
  }

  // send is PromisedWorker.postMessage and PromisedWorker.receive of
  // elk-api.js, without the promise: the dispatcher of the worker answers
  // before it returns.
  function send(data) {
    message = undefined;
    dispatch({ data: data });
    var m = message;
    message = undefined;
    if (m === undefined) {
      throw new Error('the elkjs worker did not answer');
    }
    if (m.error) {
      throw m.error;
    }
    return m.data;
  }

  function isNode(n) {
    return n !== null && typeof n === 'object' && !Array.isArray(n);
  }

  // The engine gives each node object of the graph a member $H, the Java
  // identity hash code. collectHashes finds the nodes that have the member
  // before the layout: those keep it.
  function collectHashes(root, keep) {
    var stack = [root];
    while (stack.length > 0) {
      var node = stack.pop();
      if (Object.prototype.hasOwnProperty.call(node, '$H')) {
        keep.add(node);
      }
      var c = node.children;
      if (Array.isArray(c)) {
        for (var i = 0; i < c.length; i++) {
          if (isNode(c[i])) {
            stack.push(c[i]);
          }
        }
      }
    }
  }

  function stripHashes(root, keep) {
    var stack = [root];
    while (stack.length > 0) {
      var node = stack.pop();
      if (keep === null || !keep.has(node)) {
        delete node.$H;
      }
      var c = node.children;
      if (Array.isArray(c)) {
        for (var i = 0; i < c.length; i++) {
          if (isNode(c[i])) {
            stack.push(c[i]);
          }
        }
      }
    }
  }

  send({ id: 0, cmd: 'register', algorithms: algorithms });

  var api = {
    // lastHash is the largest hash code that a layout gave to a root node.
    lastHash: 0,
    // layout is ELK.layout(graph) of elk-api.js, with the default arguments.
    // strip removes the $H members. hasHash tells that the input may have
    // $H members.
    layout: function (text, strip, hasHash) {
      var graph = JSON.parse(text);
      if (!graph) {
        throw new Error("Missing mandatory parameter 'graph'.");
      }
      var keep = null;
      if (strip && hasHash && isNode(graph)) {
        keep = new Set();
        collectHashes(graph, keep);
      }
      var data = send({
        id: 0,
        cmd: 'layout',
        graph: graph,
        layoutOptions: {},
        options: { logging: false, measureExecutionTime: false }
      });
      if (isNode(data)) {
        var h = data.$H;
        if (typeof h === 'number' && h > api.lastHash && (keep === null || !keep.has(data))) {
          api.lastHash = h;
        }
        if (strip) {
          stripHashes(data, keep);
        }
      }
      return JSON.stringify(data);
    }
  };
  return api;
})`

// Constants of the environment that each runtime gives to the engine. The
// generator tools/reference/gen-elk.mjs installs the same values in node.
const (
	// fixedNow is the base of Date.now.
	fixedNow = 1700000000000
	// randomSeed is the state of Math.random at the start of each layout.
	randomSeed = 0x9e3779b9
	// maxHash retires a runtime. The hash code counter of the engine never
	// goes back, and the engine uses its values as 32-bit integers.
	maxHash = 1 << 30
	// maxCallStack bounds the JavaScript call stack. The stack of V8 has
	// room for about 10000 frames of the engine, so this limit does not
	// stop a layout that V8 completes. ELK recurses without end for a few
	// graphs: V8 then throws a RangeError, and Layout returns one too.
	maxCallStack = 50000
)

var (
	compileOnce   sync.Once
	workerProgram *goja.Program
	bootProgram   *goja.Program
	compileErr    error
)

// patchedSource applies the patches to the worker script.
func patchedSource(enabled bool) (string, error) {
	src := workerSource
	if strings.Contains(src, "$$strHash") {
		return "", errors.New("elk: the engine script uses the name $$strHash")
	}
	if !enabled {
		return src, nil
	}
	for _, p := range patches {
		if n := strings.Count(src, p.old); n != 1 {
			return "", fmt.Errorf("elk: patch %q matches %d times in the engine script, want 1", p.name, n)
		}
		src = strings.Replace(src, p.old, p.new, 1)
	}
	return src, nil
}

func compileSource(enabled bool) (worker, boot *goja.Program, err error) {
	src, err := patchedSource(enabled)
	if err != nil {
		return nil, nil, err
	}
	worker, err = goja.Compile("elkjs/elk-worker.min.js", workerPrefix+src+workerSuffix, false)
	if err != nil {
		return nil, nil, fmt.Errorf("elk: compile the engine: %w", err)
	}
	boot, err = goja.Compile("elk-boot.js", bootSource, false)
	if err != nil {
		return nil, nil, fmt.Errorf("elk: compile the glue: %w", err)
	}
	return worker, boot, nil
}

// programs compiles the engine one time for the process, on first use.
func programs() (worker, boot *goja.Program, err error) {
	compileOnce.Do(func() {
		workerProgram, bootProgram, compileErr = compileSource(true)
	})
	return workerProgram, bootProgram, compileErr
}

// jsRuntime is one JavaScript runtime with a loaded engine. A runtime is not
// safe for concurrent use.
type jsRuntime struct {
	vm     *goja.Runtime
	api    *goja.Object
	layout goja.Callable

	// randState is the state of Math.random (mulberry32).
	randState uint32
	// nowCalls counts the calls of Date.now. nowBase is nowCalls at the start
	// of the current layout.
	nowCalls int64
	nowBase  int64

	// layouts counts the layouts that the runtime ran.
	layouts int
	// broken tells that the runtime must not run another layout.
	broken bool
}

// newRuntime loads the engine into a new runtime and registers the layout
// algorithms.
func newRuntime(m *Math) (*jsRuntime, error) {
	return newRuntimeWith(nil, nil, m)
}

func newRuntimeWith(worker, boot *goja.Program, m *Math) (rt *jsRuntime, err error) {
	if worker == nil {
		worker, boot, err = programs()
		if err != nil {
			return nil, err
		}
	}
	defer func() {
		if r := recover(); r != nil {
			rt, err = nil, fmt.Errorf("elk: load the engine: %v", r)
		}
	}()
	vm := goja.New()
	vm.SetMaxCallStackSize(maxCallStack)
	rt = &jsRuntime{vm: vm, randState: randomSeed}

	// The engine logs through console only when it reports an exception.
	console := vm.NewObject()
	nop := func(goja.FunctionCall) goja.Value { return goja.Undefined() }
	for _, name := range []string{"log", "info", "warn", "error", "debug", "trace", "group", "groupCollapsed", "groupEnd"} {
		if err := console.Set(name, nop); err != nil {
			return nil, err
		}
	}
	if err := vm.Set("console", console); err != nil {
		return nil, err
	}

	fns := m.withDefaults()
	native := vm.NewObject()
	unary := func(f func(float64) float64) func(goja.FunctionCall) goja.Value {
		return func(call goja.FunctionCall) goja.Value {
			return vm.ToValue(f(call.Argument(0).ToFloat()))
		}
	}
	binary := func(f func(float64, float64) float64) func(goja.FunctionCall) goja.Value {
		return func(call goja.FunctionCall) goja.Value {
			return vm.ToValue(f(call.Argument(0).ToFloat(), call.Argument(1).ToFloat()))
		}
	}
	for name, fn := range map[string]any{
		"sin":     unary(fns.Sin),
		"cos":     unary(fns.Cos),
		"acos":    unary(fns.Acos),
		"log":     unary(fns.Log),
		"atan2":   binary(fns.Atan2),
		"pow":     binary(fns.Pow),
		"random":  rt.random,
		"now":     rt.now,
		"strHash": rt.strHash,
	} {
		if err := native.Set(name, fn); err != nil {
			return nil, err
		}
	}

	workerFn, err := vm.RunProgram(worker)
	if err != nil {
		return nil, fmt.Errorf("elk: load the engine: %w", err)
	}
	bootFn, err := vm.RunProgram(boot)
	if err != nil {
		return nil, fmt.Errorf("elk: load the glue: %w", err)
	}
	bootCall, ok := goja.AssertFunction(bootFn)
	if !ok {
		return nil, errors.New("elk: the glue is not a function")
	}
	api, err := bootCall(goja.Undefined(), workerFn, native)
	if err != nil {
		return nil, fmt.Errorf("elk: start the engine: %w", jsError(err))
	}
	rt.api = api.ToObject(vm)
	rt.layout, ok = goja.AssertFunction(rt.api.Get("layout"))
	if !ok {
		return nil, errors.New("elk: the glue has no layout function")
	}
	return rt, nil
}

// random is Math.random: mulberry32, with the same state at the start of
// each layout. The engine calls it only in the overlap removal of the
// algorithms sporeOverlap and sporeCompaction, for nodes with the same centre.
// V8 gives other numbers for each run, so there is no reference result for
// such a graph. The fixed sequence makes the layout a function of the graph.
func (rt *jsRuntime) random(goja.FunctionCall) goja.Value {
	rt.randState += 0x6d2b79f5
	t := rt.randState
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return rt.vm.ToValue(float64(t^(t>>14)) / 4294967296)
}

// now is Date.now. The engine calls it for java.util.Random without a seed
// (the option elk.randomSeed = 0), as uniqueSeed++ + Date.now(), where
// uniqueSeed counts these Random objects since the engine was loaded. The
// result goes down by one for each earlier call, so the k-th Random of a
// layout has the seed fixedNow + k in a new runtime and in a used one.
func (rt *jsRuntime) now(goja.FunctionCall) goja.Value {
	rt.nowCalls++
	return rt.vm.ToValue(float64(fixedNow - rt.nowBase))
}

// strHash is java.lang.String.hashCode: s[0]*31^(n-1) + ... + s[n-1] over
// the UTF-16 code units, as a 32-bit integer.
func (rt *jsRuntime) strHash(call goja.FunctionCall) goja.Value {
	var h int32
	if s, ok := call.Argument(0).(goja.String); ok {
		n := s.Length()
		for i := 0; i < n; i++ {
			h = h*31 + int32(s.CharAt(i))
		}
	} else {
		for _, c := range call.Argument(0).String() {
			h = h*31 + int32(c)
		}
	}
	return rt.vm.ToValue(h)
}

// run lays out one graph. On an interrupt, a stack overflow or a panic it
// marks the runtime as broken.
func (rt *jsRuntime) run(ctx context.Context, graph string, strip, hasHash bool) (out string, err error) {
	if rt.broken {
		return "", errors.New("elk: the runtime is broken")
	}
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("elk: layout not started: %w", err)
	}
	defer func() {
		if r := recover(); r != nil {
			rt.broken = true
			out, err = "", fmt.Errorf("elk: internal error: %v", r)
		}
	}()
	rt.randState = randomSeed
	rt.nowBase = rt.nowCalls
	rt.layouts++

	stop := context.AfterFunc(ctx, func() { rt.vm.Interrupt(ctx.Err()) })
	v, err := rt.layout(goja.Undefined(), rt.vm.ToValue(graph), rt.vm.ToValue(strip), rt.vm.ToValue(hasHash))
	if !stop() {
		// The interrupt ran or runs now. The flag can still be set after this
		// point, so the runtime cannot be used again.
		rt.broken = true
	}
	if err != nil {
		var interrupted *goja.InterruptedError
		var overflow *goja.StackOverflowError
		switch {
		case errors.As(err, &interrupted):
			rt.broken = true
			if cerr := ctx.Err(); cerr != nil {
				return "", fmt.Errorf("elk: layout interrupted: %w", cerr)
			}
			return "", errors.New("elk: layout interrupted")
		case errors.As(err, &overflow):
			rt.broken = true
			return "", &Error{Class: "RangeError", Message: "Maximum call stack size exceeded"}
		}
		return "", jsError(err)
	}
	if h := rt.api.Get("lastHash"); h != nil && h.ToFloat() > maxHash {
		rt.broken = true
	}
	return v.String(), nil
}

var javaClass = regexp.MustCompile(`^((?:[A-Za-z_$][\w$]*\.)+[A-Za-z_$][\w$]*)(?:: |$)`)

// jsError converts an error of goja to *Error.
func jsError(err error) error {
	var ex *goja.Exception
	if !errors.As(err, &ex) {
		return fmt.Errorf("elk: %w", err)
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
			e.Class = n.String()
		}
	} else if v != nil {
		e.Message = v.String()
	}
	// The message of a Java exception starts with its class name.
	if m := javaClass.FindStringSubmatch(e.Message); m != nil {
		e.Class = m[1]
	}
	return e
}
