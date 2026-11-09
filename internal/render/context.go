// This file ports no upstream file. It has the render context: the state of
// one render that the ported code shares.

// Package render is the port of the top level of mermaid: mermaidAPI.ts,
// Diagram.ts and the helpers that every diagram renderer uses. A diagram
// type registers a Definition, and Render runs the same steps as
// mermaid.render() in a browser.
package render

import (
	"context"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/browser"
	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

// Version is the mermaid version that this code is a port of.
const Version = "12.2.0"

// These values are shared with the reference oracle (tools/reference/oracle.mjs).
const (
	// DefaultRandomSeed seeds the generator that stands in for Math.random.
	DefaultRandomSeed uint32 = 0x6d6d6467
	// DefaultID is the id of the root svg element, as in mermaid-cli.
	DefaultID = "my-svg"
)

// DefaultNow is the fixed clock of the reference oracle.
var DefaultNow = time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

// Context carries the state of one render. Upstream keeps this state in
// module-level variables; here every render has its own, so that renders
// can run at the same time.
type Context struct {
	// ID is the id of the root svg element.
	ID string
	// Type is the diagram type id, for example "flowchart-v2".
	Type string
	// Config is the effective config: the result of getConfig().
	Config *js.Obj[any]
	// ConfigState lets a diagram change the config, as setConfig does.
	ConfigState *config.State
	// Title is the title from the front matter.
	Title string

	Doc    *dom.Document
	Engine *browser.Engine
	// Body is document.body. Root is the node that upstream selects with
	// select('body'), and Container is the element that holds the svg.
	Body      *dom.Node
	Container *dom.Node
	// SVG is the root svg element.
	SVG *dom.Node

	// Now is the current time, for diagrams that show today.
	Now time.Time
	// Location is the local time zone of the browser.
	Location *time.Location

	// Sanitizer is the DOMPurify instance of the page, with the hooks that
	// mermaid installs. Label sanitizing and the final sanitizing of the
	// SVG share it, as they share one DOMPurify upstream.
	Sanitizer *sanitize.Sanitizer

	// Cancel is the context of the caller. The render stops when it is
	// done. It is never nil.
	Cancel context.Context

	rng uint32
	// counters holds module-level counters of upstream code, by name.
	counters map[string]int
	// values holds other per-render state of ported modules, by name.
	values map[string]any
	// sched is the microtask queue of ctx.All. See async.go.
	sched *asyncScheduler
}

// Random is Math.random(). It is the mulberry32 generator that the
// reference oracle installs in the page, so that both sides draw the same
// numbers in the same order.
func (c *Context) Random() float64 {
	c.rng += 0x6d2b79f5
	t := c.rng
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return float64(t^(t>>14)) / 4294967296
}

// Counter returns the current value of a named counter and then adds one.
// It stands in for a module-level `let cnt = 0; cnt++` of upstream code.
func (c *Context) Counter(name string) int {
	if c.counters == nil {
		c.counters = map[string]int{}
	}
	v := c.counters[name]
	c.counters[name] = v + 1
	return v
}

// Value returns per-render state of a ported module. The state is created
// with init on first use.
func (c *Context) Value(name string, init func() any) any {
	if c.values == nil {
		c.values = map[string]any{}
	}
	v, ok := c.values[name]
	if !ok {
		v = init()
		c.values[name] = v
	}
	return v
}

// SanitizeText is sanitizeText(text, getConfig()).
func (c *Context) SanitizeText(text string) string {
	return c.Sanitizer.SanitizeText(text, c.Config)
}

// Cfg reads a config value by path, as config.a.b does with optional
// chaining. The result is nil when a part of the path is missing.
func (c *Context) Cfg(path ...string) any {
	return js.Path(c.Config, path...)
}

// CfgNum reads a numeric config value. The second result is false when the
// value is missing or is not a number.
func (c *Context) CfgNum(path ...string) (float64, bool) {
	v, ok := js.Path(c.Config, path...).(float64)
	return v, ok
}

// CfgStr reads a string config value, or "" when it is missing.
func (c *Context) CfgStr(path ...string) string {
	v, _ := js.Path(c.Config, path...).(string)
	return v
}

// CfgBool reads a config value with JavaScript truthiness.
func (c *Context) CfgBool(path ...string) bool {
	return js.Truthy(js.Path(c.Config, path...))
}

// canceled is the panic value that stops a render whose context is done.
// Render recovers it and returns the error of the context.
type canceled struct{ err error }

// CheckCanceled stops the render when the context of the caller is done.
// Long steps call it: the measurements, the task queue of All, the layouts.
// It does not return in that case.
func (c *Context) CheckCanceled() {
	if c.Cancel == nil {
		return
	}
	if err := c.Cancel.Err(); err != nil {
		panic(canceled{err})
	}
}
