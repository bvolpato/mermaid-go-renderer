package mermaid

import (
	"strings"
	"testing"
)

func TestSVGContainsFlowchartLabels(t *testing.T) {
	svg, err := RenderWithOptions(
		"flowchart LR\n  A[Start] --> B{Decision}\n  B -->|yes| C[Done]\n  B -->|no| D[Retry]",
		DefaultRenderOptions(),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	mustContainText(t, svg, "Start")
	mustContainText(t, svg, "Decision")
	mustContainText(t, svg, "Done")
	mustContainText(t, svg, "Retry")
	mustContainTag(t, svg, "<rect")
	mustContainTag(t, svg, "<polygon")
}

func TestSVGContainsSequenceParticipants(t *testing.T) {
	svg, err := RenderWithOptions(
		"sequenceDiagram\n  participant Alice\n  participant Bob\n  Alice->>Bob: Hello\n  Bob-->>Alice: Hi",
		DefaultRenderOptions(),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	mustContainText(t, svg, "Alice")
	mustContainText(t, svg, "Bob")
	mustContainText(t, svg, "Hello")
	mustContainTag(t, svg, "<line")
}

func TestSVGContainsClassDiagramElements(t *testing.T) {
	svg, err := RenderWithOptions(
		"classDiagram\n  class Animal {\n    +int age\n    +eat()\n  }\n  class Dog {\n    +bark()\n  }\n  Animal <|-- Dog",
		DefaultRenderOptions(),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	mustContainText(t, svg, "Animal")
	mustContainText(t, svg, "Dog")
	mustContainTag(t, svg, "<path")
}

func TestSVGContainsStateDiagramElements(t *testing.T) {
	svg, err := RenderWithOptions(
		"stateDiagram-v2\n  [*] --> Idle\n  Idle --> Running\n  Running --> Done\n  Done --> [*]",
		DefaultRenderOptions(),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	mustContainText(t, svg, "Idle")
	mustContainText(t, svg, "Running")
	mustContainText(t, svg, "Done")
}

func TestSVGContainsERDiagramLabels(t *testing.T) {
	svg, err := RenderWithOptions(
		"erDiagram\n  CUSTOMER ||--o{ ORDER : places\n  ORDER ||--|{ LINE_ITEM : contains",
		DefaultRenderOptions(),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	mustContainText(t, svg, "CUSTOMER")
	mustContainText(t, svg, "ORDER")
	mustContainText(t, svg, "LINE_ITEM")
}

func TestSVGContainsPieSlices(t *testing.T) {
	svg, err := RenderWithOptions(
		"pie showData\n  title Pets\n  \"Dogs\" : 10\n  \"Cats\" : 5\n  \"Birds\" : 2",
		DefaultRenderOptions().WithAllowApproximate(true),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	mustContainText(t, svg, "Dogs")
	mustContainText(t, svg, "Cats")
	mustContainText(t, svg, "Birds")
	mustContainText(t, svg, "Pets")
	mustContainTag(t, svg, "<path")
}

func TestSVGContainsMindmapLabels(t *testing.T) {
	svg, err := RenderWithOptions(
		"mindmap\n  root((Root))\n    Branch A\n      Leaf A1\n    Branch B",
		DefaultRenderOptions(),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	mustContainText(t, svg, "Root")
	mustContainText(t, svg, "Branch A")
	mustContainText(t, svg, "Branch B")
}

func TestSVGContainsGanttElements(t *testing.T) {
	svg, err := RenderWithOptions(
		"gantt\n  title Plan\n  section Build\n  Core :done, core, 2026-01-01, 10d\n  QA :active, qa, after core, 5d",
		DefaultRenderOptions().WithAllowApproximate(true),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	mustContainText(t, svg, "Plan")
	mustContainText(t, svg, "Core")
	mustContainTag(t, svg, "<rect")
}

func TestSVGContainsGitGraphElements(t *testing.T) {
	svg, err := RenderWithOptions(
		"gitGraph\n  commit\n  branch feature\n  checkout feature\n  commit\n  checkout main\n  merge feature",
		DefaultRenderOptions().WithAllowApproximate(true),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	mustContainTag(t, svg, "<circle")
	mustContainText(t, svg, "main")
	mustContainText(t, svg, "feature")
}

func TestSVGContainsXYChartAxes(t *testing.T) {
	svg, err := RenderWithOptions(
		"xychart-beta\n  title Revenue\n  x-axis [Q1, Q2, Q3]\n  y-axis 0 --> 100\n  bar [20, 50, 80]\n  line [15, 45, 85]",
		DefaultRenderOptions().WithAllowApproximate(true),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	mustContainText(t, svg, "Revenue")
	mustContainText(t, svg, "Q1")
	mustContainTag(t, svg, "<rect")
}

func TestSVGContainsQuadrantPoints(t *testing.T) {
	svg, err := RenderWithOptions(
		"quadrantChart\n  title Priorities\n  x-axis Low --> High\n  y-axis Low --> High\n  Risk: [0.2, 0.9]\n  Value: [0.8, 0.3]",
		DefaultRenderOptions().WithAllowApproximate(true),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	mustContainText(t, svg, "Priorities")
	mustContainText(t, svg, "Risk")
	mustContainText(t, svg, "Value")
}

func TestSVGContainsTimelineContent(t *testing.T) {
	svg, err := RenderWithOptions(
		"timeline\n  title History\n  2020 : Launch\n  2021 : Growth\n  2022 : IPO",
		DefaultRenderOptions().WithAllowApproximate(true),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	mustContainText(t, svg, "History")
	mustContainText(t, svg, "Launch")
}

func TestSVGContainsJourneySteps(t *testing.T) {
	svg, err := RenderWithOptions(
		"journey\n  title Checkout\n  section Browse\n  View items: 5: Customer",
		DefaultRenderOptions().WithAllowApproximate(true),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	mustContainText(t, svg, "Checkout")
	mustContainText(t, svg, "View items")
}

func TestSVGHasReasonableViewBox(t *testing.T) {
	diagrams := []struct {
		name    string
		diagram string
	}{
		{"flowchart", "flowchart LR\n  A --> B --> C"},
		{"sequence", "sequenceDiagram\n  Alice->>Bob: Hello"},
		{"class", "classDiagram\n  Animal <|-- Dog"},
		{"pie", "pie\n  title X\n  A : 1\n  B : 2"},
		{"state", "stateDiagram-v2\n  [*] --> A --> [*]"},
	}

	for _, d := range diagrams {
		t.Run(d.name, func(t *testing.T) {
			svg, err := RenderWithOptions(d.diagram, DefaultRenderOptions().WithAllowApproximate(true))
			if err != nil {
				t.Fatalf("render error: %v", err)
			}

			w, h := detectSVGSize(svg)
			if w <= 0 || h <= 0 {
				t.Fatalf("invalid SVG dimensions: %dx%d", w, h)
			}
			if w > 10000 || h > 10000 {
				t.Fatalf("SVG dimensions unreasonably large: %dx%d", w, h)
			}
		})
	}
}

func TestSVGAllDiagramKindsProduceWellFormedOutput(t *testing.T) {
	diagrams := map[string]string{
		"flowchart":    "flowchart LR\n  A --> B",
		"sequence":     "sequenceDiagram\n  Alice->>Bob: Hi",
		"class":        "classDiagram\n  Animal <|-- Dog",
		"state":        "stateDiagram-v2\n  [*] --> A --> [*]",
		"er":           "erDiagram\n  CAR ||--o{ DRIVER : allows",
		"pie":          "pie\n  title X\n  A : 1",
		"mindmap":      "mindmap\n  root((R))\n    child",
		"journey":      "journey\n  title J\n  section S\n  Step: 5: Actor",
		"timeline":     "timeline\n  title T\n  2024 : event",
		"gantt":        "gantt\n  title G\n  section S\n  Task A :a, 2026-01-01, 5d",
		"requirement":  "requirementDiagram\n  requirement r1 {\n    id: 1\n    text: req\n  }",
		"gitgraph":     "gitGraph\n  commit\n  branch feat\n  commit",
		"c4":           "C4Context\n  Person(user, \"User\")",
		"sankey":       "sankey-beta\n  A,B,10",
		"quadrant":     "quadrantChart\n  title Q\n  x-axis L --> H\n  y-axis L --> H\n  P: [0.5, 0.5]",
		"zenuml":       "zenuml\n  A->B: call",
		"block":        "block-beta\n  columns 2\n  A B",
		"packet":       "packet-beta\n  packet test {\n  field a\n  }",
		"kanban":       "kanban\n  Todo:\n    task one",
		"architecture": "architecture-beta\n  group api(cloud)[API]",
		"radar":        "radar-beta\n  title R\n  axis A, B, C\n  series You: [8,7,9]",
		"treemap":      "treemap\n  title M\n  A: 10\n  B: 20",
		"xychart":      "xychart-beta\n  title Rev\n  x-axis [Q1, Q2]\n  y-axis 0 --> 100\n  bar [20, 50]",
	}

	for name, diagram := range diagrams {
		t.Run(name, func(t *testing.T) {
			svg, err := RenderWithOptions(diagram, DefaultRenderOptions().WithAllowApproximate(true))
			if err != nil {
				t.Fatalf("render error: %v", err)
			}

			if !strings.Contains(svg, "<svg") {
				t.Fatal("missing <svg tag")
			}
			if !strings.Contains(svg, "</svg>") {
				t.Fatal("missing </svg> closing tag")
			}
			if !strings.Contains(svg, "viewBox=") {
				t.Fatal("missing viewBox attribute")
			}
		})
	}
}

func TestRenderWithMermaidDefaultTheme(t *testing.T) {
	svg, err := RenderWithOptions(
		"flowchart LR\n  A[Start] --> B[End]",
		MermaidDefaultOptions(),
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	mustContainText(t, svg, "Start")
	mustContainText(t, svg, "End")
}

func TestRenderWithModernTheme(t *testing.T) {
	opts := DefaultRenderOptions()
	opts.Theme = ModernTheme()
	svg, err := RenderWithOptions(
		"flowchart LR\n  A[Start] --> B[End]",
		opts,
	)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	mustContainText(t, svg, "Start")
	mustContainText(t, svg, "End")
}

func TestRenderWithCustomSpacing(t *testing.T) {
	svgDefault, err := RenderWithOptions(
		"flowchart TD\n  A --> B --> C",
		DefaultRenderOptions(),
	)
	if err != nil {
		t.Fatalf("default render error: %v", err)
	}

	svgWide, err := RenderWithOptions(
		"flowchart TD\n  A --> B --> C",
		DefaultRenderOptions().WithNodeSpacing(100).WithRankSpacing(100),
	)
	if err != nil {
		t.Fatalf("wide render error: %v", err)
	}

	wDefault, hDefault := detectSVGSize(svgDefault)
	wWide, hWide := detectSVGSize(svgWide)

	if wWide+hWide <= wDefault+hDefault {
		t.Fatalf("wider spacing should produce larger SVG: default=%dx%d wide=%dx%d",
			wDefault, hDefault, wWide, hWide)
	}
}

func mustContainText(t *testing.T, svg, text string) {
	t.Helper()
	if !strings.Contains(svg, text) {
		if len(svg) > 500 {
			svg = svg[:500] + "..."
		}
		t.Fatalf("SVG missing text %q", text)
	}
}

func mustContainTag(t *testing.T, svg, tag string) {
	t.Helper()
	if !strings.Contains(svg, tag) {
		t.Fatalf("SVG missing tag %q", tag)
	}
}
