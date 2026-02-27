package mermaid

import (
	"strings"
	"testing"
)

func TestRenderSimpleFlowchart(t *testing.T) {
	input := "flowchart LR\nA[Start] -->|go| B(End)"
	svg, err := Render(input)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !strings.Contains(svg, "<svg") || !strings.Contains(svg, "</svg>") {
		t.Fatalf("expected SVG output, got: %s", svg)
	}
}

func TestRenderAllDiagramKinds(t *testing.T) {
	cases := []struct {
		name    string
		kind    DiagramKind
		diagram string
	}{
		{"flowchart", DiagramFlowchart, "flowchart TD\nA --> B --> C"},
		{"sequence", DiagramSequence, "sequenceDiagram\nparticipant Alice\nparticipant Bob\nAlice->>Bob: Hello"},
		{"class", DiagramClass, "classDiagram\nAnimal <|-- Duck\nDuck : +swim()"},
		{"state", DiagramState, "stateDiagram-v2\n[*] --> Active\nActive --> [*]"},
		{"er", DiagramER, "erDiagram\nCAR ||--o{ DRIVER : allows"},
		{"pie", DiagramPie, "pie showData\ntitle Pets\nDogs : 10\nCats : 5"},
		{"mindmap", DiagramMindmap, "mindmap\n  root((Mindmap))\n    child one\n    child two"},
		{"journey", DiagramJourney, "journey\ntitle Checkout Journey\nsection Happy\nBrowse: 5: Customer"},
		{"timeline", DiagramTimeline, "timeline\ntitle Product\n2024 : MVP\n2025 : GA"},
		{"gantt", DiagramGantt, "gantt\ntitle Roadmap\nsection Build\nCore Engine :done, core, 2026-01-01, 10d"},
		{"requirement", DiagramRequirement, "requirementDiagram\nrequirement req1 {\n id: 1\n text: fast renderer\n}"},
		{"gitgraph", DiagramGitGraph, "gitGraph\ncommit\nbranch feature\ncheckout feature\ncommit\ncheckout main\nmerge feature"},
		{"c4", DiagramC4, "C4Context\nPerson(user, \"User\")"},
		{"sankey", DiagramSankey, "sankey-beta\nA,B,10"},
		{"quadrant", DiagramQuadrant, "quadrantChart\ntitle Priorities\nx-axis Low --> High\ny-axis Low --> High\nRisk: [0.2, 0.9]"},
		{"zenuml", DiagramZenUML, "zenuml\n@startuml\nAlice->Bob: ping"},
		{"block", DiagramBlock, "block-beta\ncolumns 2\nA B"},
		{"packet", DiagramPacket, "packet-beta\npacket test {\nfield a\n}"},
		{"kanban", DiagramKanban, "kanban\nTodo:\n  task one"},
		{"architecture", DiagramArchitecture, "architecture-beta\ngroup api(cloud)[API]"},
		{"radar", DiagramRadar, "radar-beta\ntitle Skills\naxis A, B, C\nseries You: [8,7,9]"},
		{"treemap", DiagramTreemap, "treemap\ntitle Market\nA: 10\nB: 20"},
		{"xychart", DiagramXYChart, "xychart-beta\ntitle Revenue\nx-axis [Q1, Q2, Q3]\ny-axis 0 --> 100\nbar [20, 50, 80]\nline [10, 40, 90]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := ParseMermaid(tc.diagram)
			if err != nil {
				t.Fatalf("ParseMermaid() error = %v", err)
			}
			if parsed.Graph.Kind != tc.kind {
				t.Fatalf("kind mismatch: got %s, want %s", parsed.Graph.Kind, tc.kind)
			}
			layout := ComputeLayout(&parsed.Graph, ModernTheme(), DefaultLayoutConfig())
			if layout.Width <= 0 || layout.Height <= 0 {
				t.Fatalf("invalid layout size: %fx%f", layout.Width, layout.Height)
			}
			svg := RenderSVG(layout, ModernTheme(), DefaultLayoutConfig())
			if !strings.Contains(svg, "<svg") {
				t.Fatalf("expected SVG for kind %s", tc.kind)
			}
		})
	}
}

func TestRenderWithTiming(t *testing.T) {
	result, err := RenderWithTiming("flowchart LR\nA-->B-->C", DefaultRenderOptions())
	if err != nil {
		t.Fatalf("RenderWithTiming() error = %v", err)
	}
	if result.TotalUS() == 0 {
		t.Fatalf("expected non-zero timing, got %d", result.TotalUS())
	}
	if !strings.Contains(result.SVG, "<svg") {
		t.Fatalf("expected SVG output")
	}
}

func TestPreferredAspectRatio(t *testing.T) {
	opts := DefaultRenderOptions().WithPreferredAspectRatio(16.0 / 9.0)
	svg, err := RenderWithOptions("flowchart LR\nA-->B-->C-->D", opts)
	if err != nil {
		t.Fatalf("RenderWithOptions() error = %v", err)
	}
	width, okW := parseSVGAttr(svg, "width")
	height, okH := parseSVGAttr(svg, "height")
	if !okW || !okH || height == 0 {
		vw, vh, okV := parseSVGViewBoxSizeForTest(svg)
		if !okV || vh == 0 {
			t.Fatalf("missing width/height in SVG")
		}
		width = vw
		height = vh
	}
	ratio := width / height
	if ratio < 1.7 || ratio > 1.85 {
		t.Fatalf("expected near 16:9 ratio, got %f", ratio)
	}
}

func TestExtractMermaidBlocks(t *testing.T) {
	markdown := "text\n" +
		"``` mermaid\n" +
		"flowchart LR\n" +
		"  A --> B\n" +
		"```\n" +
		"other\n" +
		"~~~ mermaid\n" +
		"sequenceDiagram\n" +
		"  A->>B: hi\n" +
		"~~~\n"
	blocks := ExtractMermaidBlocks(markdown)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if !strings.Contains(blocks[0], "flowchart") || !strings.Contains(blocks[1], "sequenceDiagram") {
		t.Fatalf("unexpected block extraction: %#v", blocks)
	}
}

func TestRenderPieByDefault(t *testing.T) {
	svg, err := RenderWithOptions(
		"pie showData\ntitle Pets\nDogs: 10\nCats: 5",
		DefaultRenderOptions(),
	)
	if err != nil {
		t.Fatalf("expected default rendering to succeed, got: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Fatalf("expected SVG output")
	}
}

func TestRenderAllowsApproximateWhenEnabled(t *testing.T) {
	svg, err := RenderWithOptions(
		"pie showData\ntitle Pets\nDogs: 10\nCats: 5",
		DefaultRenderOptions().WithAllowApproximate(true),
	)
	if err != nil {
		t.Fatalf("expected approximate rendering to succeed, got: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Fatalf("expected SVG output")
	}
}

func parseSVGAttr(svg, attr string) (float64, bool) {
	marker := attr + "=\""
	start := strings.Index(svg, marker)
	if start < 0 {
		return 0, false
	}
	start += len(marker)
	end := strings.Index(svg[start:], "\"")
	if end < 0 {
		return 0, false
	}
	return parseFloat(svg[start : start+end])
}

func parseSVGViewBoxSizeForTest(svg string) (float64, float64, bool) {
	marker := `viewBox="`
	start := strings.Index(svg, marker)
	if start < 0 {
		return 0, 0, false
	}
	start += len(marker)
	end := strings.Index(svg[start:], `"`)
	if end < 0 {
		return 0, 0, false
	}
	parts := strings.Fields(svg[start : start+end])
	if len(parts) != 4 {
		return 0, 0, false
	}
	w, okW := parseFloat(parts[2])
	h, okH := parseFloat(parts[3])
	if !okW || !okH {
		return 0, 0, false
	}
	return w, h, true
}
