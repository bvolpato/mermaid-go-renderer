// Runs @zenuml/core 3.49.2, src/svg/renderToSvg.ts and
// src/positioning/WidthProviderFunc.ts, as shipped by
// @mermaid-js/mermaid-zenuml 1.0.1 (MIT). See LICENSES.txt for bundled terms.

package zenuml

import (
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/browser"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/dop251/goja"
)

//go:embed zenuml.bundle.js.gz
var bundle []byte

var compileBundle = sync.OnceValues(func() (*goja.Program, error) {
	r, err := gzip.NewReader(bytes.NewReader(bundle))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	source, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return goja.Compile("zenuml.bundle.js", string(source), false)
})

const bootSource = `
var console = {log(){}, info(){}, warn(){}, error(){}, debug(){}, trace(){}};
function OffscreenCanvas() {}
OffscreenCanvas.prototype.getContext = function() {
  return {font: '', measureText(text) { return {width: __measure(this.font, text)}; }};
};
var document = {
  createElementNS(namespace, name) {
    return {
      attributes: {}, textContent: '',
      setAttribute(name, value) { this.attributes[name] = value; },
      appendChild() {},
      getBBox() { return {width: __measureSVG(this.attributes['font-size'], this.textContent)}; }
    };
  },
  body: {appendChild() {}, removeChild() {}}
};
`

type svgResult struct {
	innerSVG, viewBox string
	width, height     float64
}

type zenumlRuntime struct {
	vm       *goja.Runtime
	renderFn goja.Callable
	fonts    *font.Library
	chain    *font.Chain
	chainLib *font.Library
}

// runtimePools keeps one goja VM pool per *font.Library because @zenuml/core
// memoizes measured text widths inside its JavaScript module state.
var runtimePools sync.Map // *font.Library -> *sync.Pool

func poolForFonts(fonts *font.Library) *sync.Pool {
	if v, ok := runtimePools.Load(fonts); ok {
		return v.(*sync.Pool)
	}
	p, _ := runtimePools.LoadOrStore(fonts, &sync.Pool{})
	return p.(*sync.Pool)
}

func newZenumlRuntime(fonts *font.Library) (*zenumlRuntime, error) {
	program, err := compileBundle()
	if err != nil {
		return nil, fmt.Errorf("zenuml: compile bundle: %w", err)
	}
	vm := goja.New()
	vm.SetMaxCallStackSize(4096)
	rt := &zenumlRuntime{
		vm:       vm,
		fonts:    fonts,
		chain:    fonts.Resolve([]string{"Helvetica", "Verdana", "serif"}, 400, false),
		chainLib: fonts,
	}
	if err := vm.Set("__measure", func(spec, text string) float64 {
		size, _ := strconv.ParseFloat(strings.SplitN(spec, "px", 2)[0], 64)
		// Canvas replaces each ASCII whitespace character with a space.
		// https://html.spec.whatwg.org/multipage/canvas.html#text-preparation-algorithm
		text = canvasWhitespace.Replace(text)
		return font.TextWidth(rt.chain.ShapeText([]rune(text), size))
	}); err != nil {
		return nil, err
	}
	if err := vm.Set("__measureSVG", func(size, text string) float64 {
		doc := browser.NewDocumentWithFonts(rt.fonts)
		node := doc.Body.Append("svg").Append("text").
			Attr("font-family", "Helvetica, Verdana, serif").
			Attr("font-size", size).Text(text)
		return node.BBox().Width
	}); err != nil {
		return nil, err
	}
	if _, err := vm.RunString(bootSource); err != nil {
		return nil, err
	}
	if _, err := vm.RunProgram(program); err != nil {
		return nil, fmt.Errorf("zenuml: load bundle: %w", err)
	}
	api := vm.Get("__zenumlCore").ToObject(vm)
	fn, ok := goja.AssertFunction(api.Get("renderToSvg"))
	if !ok {
		return nil, fmt.Errorf("zenuml: bundle has no renderToSvg function")
	}
	rt.renderFn = fn
	return rt, nil
}

func renderToSVG(ctx context.Context, code string, fonts *font.Library) (result svgResult, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if fonts == nil {
		fonts = font.Embedded()
	}
	pool := poolForFonts(fonts)
	rt, _ := pool.Get().(*zenumlRuntime)
	if rt == nil {
		rt, err = newZenumlRuntime(fonts)
		if err != nil {
			return result, err
		}
	}

	stop := context.AfterFunc(ctx, func() { rt.vm.Interrupt(ctx.Err()) })
	value, callErr := rt.renderFn(goja.Undefined(), rt.vm.ToValue(code))
	clean := stop()
	if callErr != nil {
		if clean {
			pool.Put(rt)
		}
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		return result, fmt.Errorf("zenuml: render: %w", callErr)
	}
	obj := value.ToObject(rt.vm)
	result = svgResult{
		innerSVG: obj.Get("innerSvg").String(), viewBox: obj.Get("viewBox").String(),
		width: obj.Get("width").ToFloat(), height: obj.Get("height").ToFloat(),
	}
	if clean {
		pool.Put(rt)
	}
	return result, nil
}

var canvasWhitespace = strings.NewReplacer("\t", " ", "\n", " ", "\f", " ", "\r", " ")
