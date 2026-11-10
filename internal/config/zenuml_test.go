// Tests the detector of @mermaid-js/mermaid-zenuml 1.0.1,
// src/detector.ts (MIT), with mermaid 12.2.0 preprocessing.

package config

import (
	"fmt"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

func TestZenUMLDetection(t *testing.T) {
	for _, source := range []string{
		"zenuml\nA->B: Hi",
		"\ufeff\u00a0zenuml\nA->B: Hi",
		"%% comment\nzenuml\nA->B: Hi",
		"---\ntitle: Demo\n---\nzenuml\nA->B: Hi",
		"---\rtitle: Demo\r---\rzenuml\rA->B: Hi",
		"%%{init: {'sequence': {'useMaxWidth': false}}}%%\nzenuml\nA->B: Hi",
	} {
		if _, err := DetectType(source, nil); !IsUnknownDiagram(err) {
			t.Errorf("plain detector for %q: got %v", source, err)
		}
		if typ, err := DetectTypeMermaidCLI(source, nil); err != nil || typ != "zenuml" {
			t.Errorf("CLI detector for %q: got %q, %v", source, typ, err)
		}
		resolved, err := ResolveMermaidCLI(source, nil)
		if err != nil || resolved.DiagramType != "zenuml" {
			t.Fatalf("CLI config for %q: got %v, %v", source, resolved, err)
		}
	}
	if _, err := DetectTypeMermaidCLI("ZenUML\nA->B: Hi", nil); !IsUnknownDiagram(err) {
		t.Fatalf("case-sensitive detector: got %v", err)
	}
}

func TestZenUMLResolverPaths(t *testing.T) {
	sites := []string{
		"",
		`{"zenuml":{"theme":"dark"}}`,
		`{"themeVariables":{"custom":[{"x":1}]}}`,
	}
	sources := []string{
		"zenuml\nA->B: Hi",
		"zenuml\nA->B: Hi\n%% C4Container",
		"%%{init: {'sequence': {'useMaxWidth': false}}}%%\nzenuml\nA->B: Hi",
		"---\nconfig:\n  themeVariables:\n    custom: [{x: 1}]\n---\nzenuml\nA->B: Hi",
		"flowchart LR\nA-->B",
	}
	for round := range 2 {
		for _, siteText := range sites {
			for _, source := range sources {
				want, wantErr := resolveStateWithDetector(source, parseSite(t, siteText), true, nil)
				got, gotErr := ResolveMermaidCLI(source, parseSite(t, siteText))
				name := fmt.Sprintf("round %d, site %s, source %q", round, siteText, source)
				if fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
					t.Fatalf("%s: got %v, want %v", name, gotErr, wantErr)
				}
				if gotErr != nil {
					continue
				}
				if got.Code != want.Code || got.Source != want.Source || got.Title != want.Title || got.DisplayMode != want.DisplayMode || got.DiagramType != want.DiagramType {
					t.Fatalf("%s: preprocessed results differ", name)
				}
				if g, w := stateScript(got), stateScript(want); g != w {
					t.Fatalf("%s: config state differs: %s", name, firstDiff(g, w))
				}
			}
		}
	}
}

func TestZenUMLConfig(t *testing.T) {
	source := "%%{init: {'sequence': {'useMaxWidth': false}}}%%\nzenuml\nA->B: Hi"
	resolved, err := ResolveMermaidCLI(source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if js.Path(resolved.Config, "sequence", "useMaxWidth") != false {
		t.Fatal("the sequence.useMaxWidth directive was lost")
	}
	if _, err := Resolve(source, nil); !IsUnknownDiagram(err) {
		t.Fatalf("CLI detector leaked into plain config: %v", err)
	}
}

func TestZenUMLCleanedDetection(t *testing.T) {
	source := "zenuml\nA->B: Hi\n%% C4Container"
	site := js.NewObj[any]()
	zen := js.NewObj[any]()
	zen.Set("theme", "dark")
	site.Set("zenuml", zen)
	resolved, err := ResolveMermaidCLI(source, site)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.DiagramType != "zenuml" || resolved.Config.Get("theme") != "dark" {
		t.Fatalf("type = %q, theme = %v, want zenuml and dark", resolved.DiagramType, resolved.Config.Get("theme"))
	}
}
