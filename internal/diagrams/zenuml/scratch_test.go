package zenuml

import (
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/dop251/goja"
)

func TestScratch(t *testing.T) {
	path := os.Getenv("ZEN_BUNDLE")
	if path == "" {
		t.Skip()
	}
	src, _ := os.ReadFile(path)
	t0 := time.Now()
	prg, err := goja.Compile("bundle.js", string(src), false)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("compile", time.Since(t0))
	for round := 0; round < 2; round++ {
		t0 = time.Now()
		vm := goja.New()
		calls := 0
		vm.Set("__measure", func(font, text string) float64 { calls++; return float64(len(text)) * 8.9 })
		_, err = vm.RunString(`
var console = {log(){},info(){},warn(){},error(){},debug(){},trace(){}};
var globalThis = this;
function OffscreenCanvas(){ }
OffscreenCanvas.prototype.getContext = function(){ return { font: '', measureText: function(t){ return {width: __measure(this.font, t)}; } }; };
`)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := vm.RunProgram(prg); err != nil {
			t.Fatal(err)
		}
		fmt.Println("load", time.Since(t0))
		fn, _ := goja.AssertFunction(vm.Get("__zenumlRenderToSvg"))
		for _, f := range []string{"01", "03", "08", "16", "01", "03", "08", "16"} {
			code, _ := os.ReadFile("../../../testdata/corpus/mermaid-docs/syntax/syntax-zenuml-" + f + ".mmd")
			text := regexp.MustCompile(`^\s*zenuml`).ReplaceAllString(string(code), "")
			t0 = time.Now()
			v, err := fn(goja.Undefined(), vm.ToValue(text))
			if err != nil {
				t.Fatal(err)
			}
			o := v.ToObject(vm)
			fmt.Println(f, time.Since(t0), o.Get("viewBox"), len(o.Get("innerSvg").String()), calls)
		}
	}
}
