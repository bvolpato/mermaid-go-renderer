// This file ports no upstream file. It runs katexjs/katex.mjs, the file
// dist/katex.mjs of KaTeX 0.18.11 (MIT, see katexjs/LICENSE), in goja. The
// glue is the call of katex.renderToString in
// packages/mermaid/src/diagrams/common/common.ts of mermaid 12.2.0 (MIT).

package katex

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/dop251/goja"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// sourceSHA256 is the SHA-256 of katexjs/katex.mjs. The file is
// dist/katex.mjs of the npm package katex 0.18.11, unchanged, as the source
// map of mermaid.min.js of mermaid 12.2.0 has it.
const sourceSHA256 = "b53ef659acd792006f80e32a0fbe16d84668306b2a58ce7fce052121745d5032"

//go:embed katexjs/katex.mjs
var katexSource string

// A patch replaces one text of the script when the process compiles it. The
// file stays as KaTeX ships it. A patch is needed where goja does not give
// the result of V8 for code that is valid in both.
type patch struct {
	name string
	// old is the text in katex.mjs. It must occur exactly one time.
	old string
	// new is the replacement.
	new string
}

var patches = []patch{
	{
		name: "lexer surrogate ranges",
		old:  `[\uD800-\uDBFF][\uDC00-\uDFFF]`,
		// regexp2 gives these two surrogate ranges the same internal key.
		// The complement is the same low-surrogate set with a distinct key.
		new: `[\uD800-\uDBFF][^\u0000-\uDBFF\uE000-\uFFFF]`,
	},
}

// The script is an ES module, and goja has no modules. The module imports
// nothing, and its only export statement is its last line. The script
// becomes the body of a function that returns the default export, the
// object "katex". In a function the declarations of the script are local
// variables with a fixed slot, which goja reads faster than globals.
const (
	exportStart  = "\nexport {"
	exportEnd    = "};"
	scriptPrefix = "(function(){"
	scriptSuffix = "\nreturn katex;\n})"
)

// bootSource connects KaTeX to Go. It returns the function that Go calls.
//
// render is the callback of renderKatexUnsanitized in common.ts of mermaid:
// katex.renderToString(c, {throwOnError: true, displayMode: true, output}).
// The options object is new for each call, as in mermaid. So the member
// "macros" of the settings is a new object for each call, and a \gdef of one
// expression does not reach the next one.
//
// render returns the string, or an array that describes the error. goja
// cannot catch a stack overflow or an interrupt in JavaScript: those reach
// Go as errors.
const bootSource = `(function (katex) {
  'use strict';
  var ParseError = katex.ParseError;
  var renderToString = katex.renderToString;
  function text(v) {
    try {
      return String(v);
    } catch (e) {
      return '';
    }
  }
  return function render(tex, output) {
    try {
      return renderToString(tex, { throwOnError: true, displayMode: true, output: output });
    } catch (e) {
      if (e instanceof ParseError) {
        var pos = typeof e.position === 'number' && typeof e.length === 'number';
        return [true, text(e.name), text(e.message), text(e.rawMessage), pos, pos ? e.position : 0, pos ? e.length : 0];
      }
      if (e !== null && typeof e === 'object') {
        return [false, text(e.name), text(e.message)];
      }
      return [false, '', text(e)];
    }
  };
})`

// maxCallStack bounds the JavaScript call stack. See the comment of
// stackOverflow.
const maxCallStack = 12000

var (
	compileOnce   sync.Once
	katexProgram  *goja.Program
	bootProgram   *goja.Program
	compileErr    error
	started       atomic.Bool
	runtimesMade  atomic.Int64
	runtimePool   sync.Pool
	errNotAString = errors.New("katex: the script returned no string")
)

// scriptSource returns the text that goja compiles: the module without its
// export statement, with the patches, as a function.
func scriptSource(patched bool) (string, error) {
	src := katexSource
	i := strings.LastIndex(src, exportStart)
	if i < 0 || strings.Count(src, "\nexport ") != 1 {
		return "", errors.New("katex: the script must have exactly one export statement")
	}
	tail := strings.TrimRight(src[i+1:], "\n")
	if !strings.HasSuffix(tail, exportEnd) || strings.Contains(tail, "\n") {
		return "", errors.New("katex: the export statement is not the last line of the script")
	}
	if strings.Contains(src, "\nimport ") || strings.HasPrefix(src, "import ") {
		return "", errors.New("katex: the script has an import statement")
	}
	src = src[:i+1]
	if patched {
		for _, p := range patches {
			if n := strings.Count(src, p.old); n != 1 {
				return "", fmt.Errorf("katex: patch %q matches %d times in the script, want 1", p.name, n)
			}
			src = strings.Replace(src, p.old, p.new, 1)
		}
	}
	return scriptPrefix + src + scriptSuffix, nil
}

func compileSource(patched bool) (script, boot *goja.Program, err error) {
	defer func() {
		if r := recover(); r != nil {
			script, boot, err = nil, nil, fmt.Errorf("katex: compile the script: %v", r)
		}
	}()
	src, err := scriptSource(patched)
	if err != nil {
		return nil, nil, err
	}
	script, err = goja.Compile("katexjs/katex.mjs", src, false)
	if err != nil {
		return nil, nil, fmt.Errorf("katex: compile the script: %w", err)
	}
	boot, err = goja.Compile("katex-boot.js", bootSource, false)
	if err != nil {
		return nil, nil, fmt.Errorf("katex: compile the glue: %w", err)
	}
	return script, boot, nil
}

// programs compiles the script one time for the process, on first use.
func programs() (script, boot *goja.Program, err error) {
	compileOnce.Do(func() {
		started.Store(true)
		katexProgram, bootProgram, compileErr = compileSource(true)
	})
	return katexProgram, bootProgram, compileErr
}

// jsRuntime is one JavaScript runtime with KaTeX loaded. A runtime is not
// safe for concurrent use.
type jsRuntime struct {
	vm     *goja.Runtime
	render goja.Callable
}

// newRuntime loads KaTeX into a new runtime.
func newRuntime() (*jsRuntime, error) {
	script, boot, err := programs()
	if err != nil {
		return nil, err
	}
	return newRuntimeWith(script, boot)
}

func newRuntimeWith(script, boot *goja.Program) (rt *jsRuntime, err error) {
	defer func() {
		if r := recover(); r != nil {
			rt, err = nil, fmt.Errorf("katex: load the script: %v", r)
		}
	}()
	vm := goja.New()
	vm.SetMaxCallStackSize(maxCallStack)

	// KaTeX calls console.warn for input that LaTeX does not accept (the
	// setting "strict" is "warn"), and console.log and console.error for the
	// macros \message, \errmessage and \show. A browser prints the text and
	// goes on. The script tests "typeof console" only for console.warn, so
	// without this object the three macros would throw a ReferenceError.
	console := vm.NewObject()
	nop := func(goja.FunctionCall) goja.Value { return goja.Undefined() }
	for _, name := range []string{"log", "info", "warn", "error", "debug", "trace"} {
		if err := console.Set(name, nop); err != nil {
			return nil, err
		}
	}
	if err := vm.Set("console", console); err != nil {
		return nil, err
	}

	scriptFn, err := vm.RunProgram(script)
	if err != nil {
		return nil, fmt.Errorf("katex: load the script: %w", err)
	}
	scriptCall, ok := goja.AssertFunction(scriptFn)
	if !ok {
		return nil, errors.New("katex: the script is not a function")
	}
	api, err := scriptCall(goja.Undefined())
	if err != nil {
		return nil, fmt.Errorf("katex: start the script: %w", err)
	}
	bootFn, err := vm.RunProgram(boot)
	if err != nil {
		return nil, fmt.Errorf("katex: load the glue: %w", err)
	}
	bootCall, ok := goja.AssertFunction(bootFn)
	if !ok {
		return nil, errors.New("katex: the glue is not a function")
	}
	fn, err := bootCall(goja.Undefined(), api)
	if err != nil {
		return nil, fmt.Errorf("katex: start the glue: %w", err)
	}
	rt = &jsRuntime{vm: vm}
	rt.render, ok = goja.AssertFunction(fn)
	if !ok {
		return nil, errors.New("katex: the glue returned no function")
	}
	runtimesMade.Add(1)
	return rt, nil
}

// render runs one expression in a runtime of the pool.
func render(ctx context.Context, tex string, output Output) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("%w: %w", ErrTimeout, err)
	}
	rt, _ := runtimePool.Get().(*jsRuntime)
	if rt == nil {
		var err error
		if rt, err = newRuntime(); err != nil {
			return "", err
		}
	}
	out, reuse, err := rt.run(ctx, tex, output)
	if reuse {
		runtimePool.Put(rt)
	}
	return out, err
}

// stackOverflow is the error for an expression that nests deeper than the
// call stack allows. V8 throws a RangeError with this message, which KaTeX
// does not catch, so katex.renderToString throws it.
//
// The depth where the stack ends is not the same. The stack of V8 has a
// size in bytes (about 1 MB), and a frame of the interpreter has another
// size than a frame of compiled code, so the depth depends on the functions
// of KaTeX that recurse and on how V8 compiled them. goja counts calls.
// maxCallStack is more calls than V8 has room for in the measured cases, so
// an expression that V8 renders is not rejected here.
func stackOverflow() error {
	return &Error{Name: "RangeError", Message: "Maximum call stack size exceeded"}
}

// run renders one expression. reuse tells that the runtime can render
// another expression.
func (rt *jsRuntime) run(ctx context.Context, tex string, output Output) (out string, reuse bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, reuse, err = "", false, fmt.Errorf("katex: internal error: %v", r)
		}
	}()

	// The timer and the context interrupt the JavaScript between two
	// instructions. A runtime that got an interrupt is not used again: the
	// flag can still be set after the call returns.
	timer := time.AfterFunc(MaxDuration, func() { rt.vm.Interrupt(ErrTimeout) })
	stopCtx := context.AfterFunc(ctx, func() { rt.vm.Interrupt(ctx.Err()) })
	v, err := rt.render(goja.Undefined(), toJS(rt.vm, tex), rt.vm.ToValue(string(output)))
	clean := timer.Stop()
	if !stopCtx() {
		clean = false
	}

	if err != nil {
		var interrupted *goja.InterruptedError
		var overflow *goja.StackOverflowError
		switch {
		case errors.As(err, &interrupted):
			if cerr := ctx.Err(); cerr != nil {
				return "", false, fmt.Errorf("%w: %w", ErrTimeout, cerr)
			}
			return "", false, ErrTimeout
		case errors.As(err, &overflow):
			return "", false, stackOverflow()
		}
		// The glue catches each JavaScript exception, so this is an error of
		// the interpreter.
		return "", false, fmt.Errorf("katex: %w", err)
	}

	if s, ok := v.(goja.String); ok {
		return fromJS(s), clean, nil
	}
	e, err := jsError(rt.vm, v)
	if err != nil {
		return "", false, err
	}
	return "", clean, e
}

// jsError converts the array that the glue returns for an exception.
func jsError(vm *goja.Runtime, v goja.Value) (error, error) {
	obj, ok := v.(*goja.Object)
	if !ok {
		return nil, errNotAString
	}
	str := func(i int) string {
		if s, ok := obj.Get(fmt.Sprint(i)).(goja.String); ok {
			return fromJS(s)
		}
		return ""
	}
	first := obj.Get("0")
	if first == nil {
		return nil, errNotAString
	}
	if !first.ToBoolean() {
		return &Error{Name: str(1), Message: str(2)}, nil
	}
	e := &ParseError{Message: str(2), RawMessage: str(3)}
	if p := obj.Get("4"); p != nil && p.ToBoolean() {
		e.HasPosition = true
		e.Position = int(obj.Get("5").ToInteger())
		e.Length = int(obj.Get("6").ToInteger())
	}
	return e, nil
}

// toJS converts a Go string to a JavaScript string. A string of ASCII
// characters needs no conversion. Each other string becomes its UTF-16 code
// units, as internal/js defines them: a lone surrogate in the three-byte
// form becomes one unit, and each other invalid byte becomes U+FFFD.
func toJS(vm *goja.Runtime, s string) goja.Value {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return vm.ToValue(s)
	}
	runes := js.UTF16Units(s)
	units := make([]uint16, len(runes))
	for i, r := range runes {
		units[i] = uint16(r)
	}
	return goja.StringFromUTF16(units)
}

// fromJS converts a JavaScript string to a Go string. goja writes a lone
// surrogate as U+FFFD. When the result has U+FFFD, the function reads the
// code units, so that a lone surrogate keeps its three-byte form.
func fromJS(s goja.String) string {
	out := s.String()
	if !strings.ContainsRune(out, utf8.RuneError) {
		return out
	}
	n := s.Length()
	units := make([]rune, n)
	for i := 0; i < n; i++ {
		units[i] = rune(s.CharAt(i))
	}
	return js.FromUTF16Units(units)
}
