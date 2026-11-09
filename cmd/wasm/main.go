//go:build js && wasm

// This file ports no upstream file.

// Command wasm is the renderer as a WebAssembly module for a web page. The
// playground of the website loads it, so that a visitor can render a diagram
// in the browser with the same Go code that runs on a server.
//
// Build it with:
//
//	GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o mmdg.wasm ./cmd/wasm
//
// The module sets the function mmdgRender(source, page) on the global
// object. page is "mermaid-cli" or "plain". The result is an object with
// the fields svg and type, or with the field error.
package main

import (
	"syscall/js"

	mermaid "github.com/bvolpato/mermaid-go-renderer"
)

func render(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return map[string]any{"error": "mmdgRender needs the diagram text"}
	}
	var opts []mermaid.Option
	if len(args) > 1 && args[1].Type() == js.TypeString && args[1].String() == "mermaid-cli" {
		opts = append(opts, mermaid.WithPage(mermaid.PageMermaidCLI))
	}
	res, err := mermaid.RenderResult(args[0].String(), opts...)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return map[string]any{"svg": res.SVG, "type": res.Type}
}

func main() {
	js.Global().Set("mmdgRender", js.FuncOf(render))
	js.Global().Set("mmdgVersion", mermaid.MermaidVersion)
	// The module stays alive, so that the page can call the function.
	select {}
}
