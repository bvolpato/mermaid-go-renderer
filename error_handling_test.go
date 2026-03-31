package mermaid

import (
	"strings"
	"testing"
)

func TestRenderEmptyInput(t *testing.T) {
	_, err := Render("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestRenderWhitespaceOnly(t *testing.T) {
	_, err := Render("   \n\n  \t  ")
	if err == nil {
		t.Fatal("expected error for whitespace-only input")
	}
}

func TestRenderInvalidDiagramKindFallsBackGracefully(t *testing.T) {
	svg, err := Render("not_a_real_diagram\nA --> B")
	if err != nil {
		t.Fatalf("expected fallback rendering, got error: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Fatal("expected SVG output from fallback")
	}
}

func TestRenderFlowchartWithUnicodeLabels(t *testing.T) {
	input := "flowchart LR\n  A[日本語] --> B[中文]\n  B --> C[한국어]\n  C --> D[Ñoño]"
	svg, err := RenderWithOptions(input, DefaultRenderOptions())
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	mustContainAll(t, svg, "日本語", "中文", "한국어", "Ñoño")
}

func TestRenderFlowchartWithSpecialCharacters(t *testing.T) {
	input := `flowchart LR
  A["Node with <angle> brackets"] --> B["Node & ampersand"]
  B --> C["Quotes 'single' here"]`
	svg, err := RenderWithOptions(input, DefaultRenderOptions())
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Fatal("expected valid SVG output")
	}
}

func TestRenderFlowchartWithLongLabels(t *testing.T) {
	longLabel := strings.Repeat("abcdefgh ", 20)
	input := "flowchart LR\n  A[" + longLabel + "] --> B[Short]"
	svg, err := RenderWithOptions(input, DefaultRenderOptions())
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Fatal("expected valid SVG output")
	}
	w, h := detectSVGSize(svg)
	if w <= 0 || h <= 0 {
		t.Fatalf("invalid dimensions: %dx%d", w, h)
	}
}

func TestRenderFlowchartDeepChain(t *testing.T) {
	parts := make([]string, 0, 22)
	parts = append(parts, "flowchart TD")
	for i := 0; i < 20; i++ {
		parts = append(parts, "  "+string(rune('A'+i))+" --> "+string(rune('A'+i+1)))
	}
	input := strings.Join(parts, "\n")
	svg, err := RenderWithOptions(input, DefaultRenderOptions())
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Fatal("expected valid SVG output")
	}
}

func TestRenderFlowchartNestedSubgraphs(t *testing.T) {
	input := `flowchart TB
  subgraph Outer
    subgraph Inner
      A[Deep]
    end
    B[Mid]
  end
  A --> B`
	svg, err := RenderWithOptions(input, DefaultRenderOptions())
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(svg, "Deep") {
		t.Fatal("expected Deep label in SVG")
	}
}

func TestRenderSequenceWithManyParticipants(t *testing.T) {
	input := `sequenceDiagram
  participant A
  participant B
  participant C
  participant D
  participant E
  participant F
  A->>B: msg1
  B->>C: msg2
  C->>D: msg3
  D->>E: msg4
  E->>F: msg5
  F-->>A: response`
	svg, err := RenderWithOptions(input, DefaultRenderOptions())
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	for _, p := range []string{"A", "B", "C", "D", "E", "F"} {
		if !strings.Contains(svg, ">"+p+"<") && !strings.Contains(svg, p) {
			t.Fatalf("missing participant %s in SVG", p)
		}
	}
}

func TestRenderPieWithSingleSlice(t *testing.T) {
	svg, err := RenderWithOptions(
		"pie\n  title Single\n  \"Only\" : 100",
		DefaultRenderOptions().WithAllowApproximate(true),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(svg, "Only") {
		t.Fatal("expected Only label")
	}
}

func TestRenderPieWithManySlices(t *testing.T) {
	parts := []string{"pie showData", "  title Many Slices"}
	for i := 0; i < 12; i++ {
		parts = append(parts, "  \"Slice "+string(rune('A'+i))+"\" : "+string(rune('1'+i)))
	}
	svg, err := RenderWithOptions(
		strings.Join(parts, "\n"),
		DefaultRenderOptions().WithAllowApproximate(true),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(svg, "Many Slices") {
		t.Fatal("expected title")
	}
}

func TestRenderDirectiveSkipped(t *testing.T) {
	input := `%%{init: {"theme": "dark"}}%%
flowchart LR
  A --> B`
	svg, err := RenderWithOptions(input, DefaultRenderOptions())
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Fatal("expected valid SVG even with directive")
	}
}

func TestRenderFrontMatterSkipped(t *testing.T) {
	input := `---
title: My Diagram
---
flowchart LR
  A --> B`
	svg, err := RenderWithOptions(input, DefaultRenderOptions())
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Fatal("expected valid SVG even with frontmatter")
	}
}

func TestRenderCommentsSkipped(t *testing.T) {
	input := `flowchart LR
  %% This is a comment
  A[Start] --> B[End]
  %% Another comment`
	svg, err := RenderWithOptions(input, DefaultRenderOptions())
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	mustContainAll(t, svg, "Start", "End")
}

func TestRenderWithAllowApproximateFalseRejectsApproximate(t *testing.T) {
	_, err := RenderWithOptions(
		"pie\n  title Test\n  A : 1",
		DefaultRenderOptions().WithAllowApproximate(false),
	)
	if err != nil {
		t.Fatalf("high-fidelity kinds should render without AllowApproximate: %v", err)
	}
}

func TestRenderIdempotent(t *testing.T) {
	input := "flowchart LR\n  A[Start] --> B[End]"
	opts := DefaultRenderOptions()

	svg1, err := RenderWithOptions(input, opts)
	if err != nil {
		t.Fatalf("first render error: %v", err)
	}

	svg2, err := RenderWithOptions(input, opts)
	if err != nil {
		t.Fatalf("second render error: %v", err)
	}

	if svg1 != svg2 {
		t.Fatal("rendering the same input twice produced different SVG output")
	}
}

func TestRenderWithTimingProducesMetrics(t *testing.T) {
	result, err := RenderWithTiming(
		"flowchart LR\n  A --> B --> C",
		DefaultRenderOptions(),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result.ParseUS == 0 && result.LayoutUS == 0 && result.RenderUS == 0 {
		t.Fatal("expected non-zero timing metrics")
	}
	if result.TotalMS() <= 0 {
		t.Fatal("expected positive total milliseconds")
	}
}

func TestRenderWithDetailedTimingProducesStages(t *testing.T) {
	result, err := RenderWithDetailedTiming(
		"flowchart LR\n  A --> B --> C",
		DefaultRenderOptions(),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result.TotalUS() == 0 {
		t.Fatal("expected non-zero total timing")
	}
	if result.LayoutStages.TotalUS() == 0 {
		t.Fatal("expected non-zero layout stages")
	}
}

func mustContainAll(t *testing.T, svg string, texts ...string) {
	t.Helper()
	for _, text := range texts {
		if !strings.Contains(svg, text) {
			t.Fatalf("SVG missing text %q", text)
		}
	}
}
