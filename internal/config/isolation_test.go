package config

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// scribble overwrites every value of a config tree, at every depth.
func scribble(v any) {
	switch x := v.(type) {
	case []any:
		for i := range x {
			scribble(x[i])
			x[i] = "scribbled"
		}
	case *js.Obj[any]:
		for _, k := range x.Keys() {
			scribble(x.Get(k))
			x.Set(k, "scribbled")
		}
		x.Set("added", true)
	}
}

// TestIsolation checks that a caller who modifies a result cannot reach the
// shared default config or another State. Upstream copies only the upper
// levels of a config, so deeper objects are shared inside one State.
func TestIsolation(t *testing.T) {
	text := "---\nconfig:\n  theme: forest\n---\nflowchart TD\n  A --> B\n"
	site := js.NewObj[any]()
	site.Set("startOnLoad", false)

	first, err := Resolve(text, site)
	if err != nil {
		t.Fatal(err)
	}
	want := js.Clone(first.Config)
	wantDefault := Default()

	scribble(first.Config)
	scribble(first.Directive)
	scribble(first.State.GetConfig())
	scribble(first.State.GetSiteConfig())
	scribble(first.State.DefaultConfig())
	scribble(Default())

	if d := diff("default", Default(), wantDefault); d != "" {
		t.Errorf("the default config changed: %s", d)
	}
	second, err := Resolve(text, site)
	if err != nil {
		t.Fatal(err)
	}
	if d := diff("config", second.Config, want); d != "" {
		t.Errorf("a second Resolve differs after the first result was modified: %s", d)
	}
}

func TestIsUnknownDiagram(t *testing.T) {
	_, err := Resolve("%%{init: {\"theme\": \"dark\"}}%%\nnot a diagram\n", nil)
	if !IsUnknownDiagram(err) {
		t.Errorf("got %v, want an UnknownDiagramError", err)
	}
	got, err := Resolve("not a diagram\n", nil)
	if err != nil || got.DiagramType != "" {
		t.Errorf("got %v, %v, want no diagram type and no error", got, err)
	}
	if _, err := DetectType("not a diagram\n", nil); !IsUnknownDiagram(err) {
		t.Errorf("DetectType: got %v, want an UnknownDiagramError", err)
	}
	cfg := js.NewObj[any]()
	if typ, err := DetectType("flowchart-elk TD\n", cfg); err != nil || typ != "flowchart-elk" || cfg.Get("layout") != "elk" {
		t.Errorf("DetectType(flowchart-elk) = %q, %v, layout %v", typ, err, cfg.Get("layout"))
	}
}
