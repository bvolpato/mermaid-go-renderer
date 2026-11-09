// This file ports no upstream file. It has the entry point of the native
// layout, which calls the ported code of the other files and packages.

// Package native lays out ELK graphs with a Go port of the layered algorithm
// of Eclipse ELK 0.9.1, as elkjs 0.9.3 runs it. The result is the JSON text
// that elkjs gives, byte for byte, for the graphs that the port covers.
//
// The port covers the graph features, the options and the processors that
// mermaid uses. For each other graph, Layout reports that it did not lay the
// graph out, and the caller runs elkjs (package internal/elk does). The code
// in this directory tree is a derivative work of ELK and is under the Eclipse
// Public License 2.0: see LICENSE and NOTICE.
package native

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/elkgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/layered"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Layout lays out the ELK JSON graph and returns the laid-out graph as JSON:
// the text of JSON.stringify(await elk.layout(graph)) of elkjs 0.9.3, without
// the "$H" members that elkjs adds.
//
// ok is false when the graph uses an option, a feature or a processor that
// is not ported, and when elkjs rejects the graph: the caller then lays the
// graph out with elkjs, which gives the exact result or the exact error. err
// is always nil in this version: each failure is a fallback.
//
// Layout is safe for concurrent use.
func Layout(graphJSON []byte) (out []byte, ok bool, err error) {
	out, reason := LayoutReason(graphJSON)
	return out, reason == "", nil
}

// LayoutReason is Layout with the reason for a fallback. reason is empty
// when out is the layout.
func LayoutReason(graphJSON []byte) (out []byte, reason string) {
	return run(graphJSON, nil)
}

// maxNodes is the largest graph that Layout lays out. See run.
const maxNodes = 2000

// Hooks lets a test observe a layout.
type Hooks struct {
	// Step is called before and after each step of each run.
	Step func(event layered.StepEvent)
	// Imported is called after the import of each run, and Final before the
	// layout of the run goes back to the ELK graph.
	Imported func(elkGraph *elkgraph.Node, graph *lgraph.LGraph)
	Final    func(elkGraph *elkgraph.Node, graph *lgraph.LGraph)
}

// LayoutWithHooks is LayoutReason with observers. It is for tests.
func LayoutWithHooks(graphJSON []byte, hooks *Hooks) (out []byte, reason string) {
	return run(graphJSON, hooks)
}

func run(graphJSON []byte, hooks *Hooks) (out []byte, reason string) {
	defer func() {
		if r := recover(); r != nil {
			out = nil
			var bail *lgraph.BailError
			if e, isErr := r.(error); isErr && errors.As(e, &bail) {
				reason = bail.Reason
				return
			}
			// A defect of the port. The caller falls back, and the check
			// mode of internal/elk reports the reason.
			buf := make([]byte, 2048)
			buf = buf[:runtime.Stack(buf, false)]
			reason = fmt.Sprintf("panic: %v\n%s", r, buf)
		}
	}()

	// A member "$H" is the hash code that elkjs keeps in a node object. With
	// such members in the input, the hash codes of elkjs are not the ones
	// that this port assumes.
	if bytes.Contains(graphJSON, []byte("$H")) || bytes.Contains(graphJSON, []byte(`\u00`)) {
		return nil, "the input has a $H member or an escaped control character in a name"
	}
	// JSON.stringify writes a lone surrogate as an escape. The JSON decoder
	// of Go replaces it.
	if bytes.Contains(graphJSON, []byte(`\ud`)) || bytes.Contains(graphJSON, []byte(`\uD`)) || !utf8.Valid(graphJSON) {
		return nil, "the input has surrogate escapes or is not UTF-8"
	}
	parsed, err := js.ParseJSON(graphJSON)
	if err != nil {
		return nil, "the input is not JSON that this port reads: " + err.Error()
	}
	root, isObj := parsed.(*js.Obj[any])
	if !isObj {
		return nil, "the graph is not an object"
	}

	im := &elkgraph.Importer{}
	rootNode, err := im.Import(root)
	if err != nil {
		return nil, err.Error()
	}

	// elkjs recurses once for each node of a path in several processors, and
	// V8 ends a layout of a path with several thousand nodes with a stack
	// overflow. The Go code has no such limit. So that both paths agree, a
	// graph of that size is left to elkjs.
	if im.NodeCount() > maxNodes {
		return nil, fmt.Sprintf("the graph has more than %d nodes", maxNodes)
	}

	ctx := lgraph.NewContext()
	provider := layered.NewProvider(ctx)
	if hooks != nil {
		provider.ElkLayered().Observer = hooks.Step
		provider.Imported = hooks.Imported
		provider.Final = hooks.Final
	}
	e := &engine{provider: provider}
	e.layout(rootNode, im.AllEdges)

	// ElkJs.layout removes the member logging of the graph. It writes the
	// logs there when the caller asks for them, which this port never does.
	root.Delete("logging")

	im.TransferLayout()
	return []byte(js.StringifyJSON(root)), ""
}
