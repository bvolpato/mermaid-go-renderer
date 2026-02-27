package mermaid

import "testing"

func TestPreprocessInputSkipsFrontMatter(t *testing.T) {
	input := `---
config:
  theme: dark
---
flowchart LR
  A --> B
`

	lines, err := preprocessInput(input)
	if err != nil {
		t.Fatalf("preprocessInput returned error: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 content lines after frontmatter, got %d (%v)", len(lines), lines)
	}
	if lines[0] != "flowchart LR" {
		t.Fatalf("expected first content line to be flowchart header, got %q", lines[0])
	}
}

func TestPreprocessInputSkipsMultilineDirective(t *testing.T) {
	input := `%%{init:
  {"theme":"base","themeVariables":{"fontFamily":"Inter"}}
}%%
flowchart TD
  A --> B
`

	lines, err := preprocessInput(input)
	if err != nil {
		t.Fatalf("preprocessInput returned error: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 content lines after directive, got %d (%v)", len(lines), lines)
	}
	if lines[0] != "flowchart TD" {
		t.Fatalf("expected first content line to be flowchart header, got %q", lines[0])
	}
}

func TestDetectDiagramKindIgnoresMetadataBlocks(t *testing.T) {
	input := `---
config:
  theme: forest
---
%%{init:
  {"sequence":{"showSequenceNumbers":true}}
}%%
sequenceDiagram
  Alice->>Bob: hello
`

	kind := detectDiagramKind(input)
	if kind != DiagramSequence {
		t.Fatalf("expected DiagramSequence, got %q", kind)
	}
}

func TestParseMermaidWithFrontMatterDoesNotCreateMetadataNodes(t *testing.T) {
	input := `---
config:
  theme: neutral
---
flowchart LR
  A --> B
`

	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}
	if _, ok := out.Graph.Nodes["config:"]; ok {
		t.Fatalf("unexpected metadata node parsed from frontmatter")
	}
	if len(out.Graph.Nodes) != 2 {
		t.Fatalf("expected exactly 2 flowchart nodes, got %d", len(out.Graph.Nodes))
	}
}

func TestFlowchartParsesAtMetadataNodeSyntax(t *testing.T) {
	input := `flowchart LR
  A@{ shape: hex, label: "API Gateway" } --> B@{ shape: cyl, label: "DB" }
`

	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}

	nodeA, ok := out.Graph.Nodes["A"]
	if !ok {
		t.Fatalf("expected node A to be parsed")
	}
	if nodeA.Label != "API Gateway" {
		t.Fatalf("unexpected node A label: %q", nodeA.Label)
	}
	if nodeA.Shape != ShapeHexagon {
		t.Fatalf("expected node A shape %q, got %q", ShapeHexagon, nodeA.Shape)
	}

	nodeB, ok := out.Graph.Nodes["B"]
	if !ok {
		t.Fatalf("expected node B to be parsed")
	}
	if nodeB.Shape != ShapeCylinder {
		t.Fatalf("expected node B shape %q, got %q", ShapeCylinder, nodeB.Shape)
	}
}
