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

func TestFlowchartLayoutUsesMermaidClassicGeometry(t *testing.T) {
	input := `flowchart LR
  A --> B --> C
  A -.-> D
  D ==> E`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	layout := ComputeLayout(&parsed.Graph, MermaidDefaultTheme(), DefaultLayoutConfig())

	findNode := func(id string) *NodeLayout {
		for i := range layout.Nodes {
			if layout.Nodes[i].ID == id {
				return &layout.Nodes[i]
			}
		}
		return nil
	}

	a := findNode("A")
	b := findNode("B")
	c := findNode("C")
	d := findNode("D")
	e := findNode("E")
	if a == nil || b == nil || c == nil || d == nil || e == nil {
		t.Fatalf("expected flowchart nodes in layout, got %#v", layout.Nodes)
	}

	for _, node := range []*NodeLayout{a, b, c, d, e} {
		if node.W < 70 || node.W > 73 {
			t.Fatalf("node %s width = %f, want classic Mermaid width around 71-72", node.ID, node.W)
		}
		if node.H != 54 {
			t.Fatalf("node %s height = %f, want 54", node.ID, node.H)
		}
	}

	if layout.ViewBoxX != 0 || layout.ViewBoxY != 0 {
		t.Fatalf("viewBox origin = (%f,%f), want (0,0)", layout.ViewBoxX, layout.ViewBoxY)
	}
	if layout.ViewBoxWidth < 300 || layout.ViewBoxWidth > 340 {
		t.Fatalf("viewBox width = %f, want near Mermaid classic ~330", layout.ViewBoxWidth)
	}
	if layout.ViewBoxHeight != 174 {
		t.Fatalf("viewBox height = %f, want 174", layout.ViewBoxHeight)
	}
}

func TestRenderFlowchartDefaultRectangleDoesNotRoundCorners(t *testing.T) {
	svg, err := Render(`flowchart LR
  A --> B`)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if strings.Contains(svg, `class="basic label-container" style="" rx="6"`) {
		t.Fatalf("expected default flowchart rectangles to stay square, got: %s", svg)
	}
}

func TestFlowchartSubgraphClustersUseChildBounds(t *testing.T) {
	input := `flowchart TD
  subgraph API
    A[Gateway] --> B[Auth]
  end
  subgraph Data
    C[(DB)] --> D[Cache]
  end
  B --> C`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	layout := ComputeLayout(&parsed.Graph, MermaidDefaultTheme(), DefaultLayoutConfig())

	clusters := make([]LayoutRect, 0, 2)
	for _, rect := range layout.Rects {
		if rect.Class == "cluster" {
			clusters = append(clusters, rect)
		}
	}
	if len(clusters) != 2 {
		t.Fatalf("expected 2 flowchart subgraph clusters, got %#v", layout.Rects)
	}
	if clusters[0].W < 160 || clusters[0].H < 200 {
		t.Fatalf("API cluster bounds = %#v, want Mermaid-like expanded bounds", clusters[0])
	}
	if clusters[1].W < 145 || clusters[1].H < 200 {
		t.Fatalf("Data cluster bounds = %#v, want Mermaid-like expanded bounds", clusters[1])
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

func TestC4TitleOnlyRenders(t *testing.T) {
	input := "C4Container\n    title Model Bank - Container Diagram"
	svg, err := Render(input)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !strings.Contains(svg, "Model Bank") {
		t.Fatalf("expected title in SVG output, got: %s", svg)
	}
}

func TestC4TitleUsesMermaidPlacement(t *testing.T) {
	input := "C4Context\ntitle System Overview\nPerson(user, \"User\")"
	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	layout := ComputeLayout(&parsed.Graph, ModernTheme(), DefaultLayoutConfig())
	// Find the title text
	var titleText *LayoutText
	for i := range layout.Texts {
		if layout.Texts[i].Value == "System Overview" {
			titleText = &layout.Texts[i]
			break
		}
	}
	if titleText == nil {
		t.Fatal("title text not found in layout")
	}
	if titleText.Class != "c4-title" {
		t.Fatalf("expected C4 title class, got %q", titleText.Class)
	}
	if titleText.Anchor != "" {
		t.Fatalf("expected unanchored Mermaid-style C4 title, got %q", titleText.Anchor)
	}
	if titleText.Y != 20 {
		t.Fatalf("expected C4 title Y=20, got %f", titleText.Y)
	}
	if titleText.X >= layout.Width/2 {
		t.Fatalf("expected C4 title X=%f to be left of center for width %f", titleText.X, layout.Width)
	}
}

func TestRenderMindmapUsesPresetEdgePath(t *testing.T) {
	layout := Layout{
		Kind:          DiagramMindmap,
		MindmapRootID: "root",
		MindmapNodes:  []MindmapNode{{ID: "root", Label: "Root"}, {ID: "child", Parent: "root", Label: "Child", Level: 1}},
		Nodes:         []NodeLayout{{ID: "root", Label: "Root", Shape: ShapeCircle, X: 10, Y: 10, W: 80, H: 80}, {ID: "child", Label: "Child", Shape: ShapeRoundRect, X: 10, Y: 120, W: 90, H: 46}},
		Lines:         []LayoutLine{{D: "M1,2C3,4,5,6,7,8", X1: 1, Y1: 2, X2: 7, Y2: 8}},
		ViewBoxX:      0,
		ViewBoxY:      0,
		ViewBoxWidth:  120,
		ViewBoxHeight: 200,
		Width:         120,
		Height:        200,
	}

	svg := RenderSVG(layout, ModernTheme(), DefaultLayoutConfig())
	if !strings.Contains(svg, `d="M1,2C3,4,5,6,7,8"`) {
		t.Fatalf("expected rendered mindmap edge to preserve preset path, got: %s", svg)
	}
}

func TestGitGraphLayoutUsesMermaidDefaultGeometry(t *testing.T) {
	input := `gitGraph
  commit id: "init"
  branch develop
  commit id: "dev-1"
  branch feature
  commit id: "feat-1"
  commit id: "feat-2"
  checkout develop
  merge feature
  checkout main
  merge develop
  commit id: "release"`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	layout := ComputeLayout(&parsed.Graph, MermaidDefaultTheme(), DefaultLayoutConfig())

	findBranchLine := func(class string) *LayoutLine {
		for i := range layout.Lines {
			if strings.TrimSpace(layout.Lines[i].Class) == class {
				return &layout.Lines[i]
			}
		}
		return nil
	}
	findCommitCircle := func(fragment string, radius float64) *LayoutCircle {
		for i := range layout.Circles {
			circle := &layout.Circles[i]
			if strings.Contains(circle.Class, fragment) && circle.R == radius {
				return circle
			}
		}
		return nil
	}

	branch0 := findBranchLine("branch branch0")
	branch1 := findBranchLine("branch branch1")
	branch2 := findBranchLine("branch branch2")
	if branch0 == nil || branch1 == nil || branch2 == nil {
		t.Fatalf("expected gitGraph branch lines in layout, got %#v", layout.Lines)
	}
	if branch0.Y1 != 0 || branch0.Y2 != 0 || branch0.X2 != 350 {
		t.Fatalf("main branch geometry = (%v,%v)->(%v,%v), want (0,0)->(350,0)", branch0.X1, branch0.Y1, branch0.X2, branch0.Y2)
	}
	if branch1.Y1 != 90 || branch1.Y2 != 90 || branch1.X2 != 350 {
		t.Fatalf("develop branch geometry = (%v,%v)->(%v,%v), want (0,90)->(350,90)", branch1.X1, branch1.Y1, branch1.X2, branch1.Y2)
	}
	if branch2.Y1 != 180 || branch2.Y2 != 180 || branch2.X2 != 350 {
		t.Fatalf("feature branch geometry = (%v,%v)->(%v,%v), want (0,180)->(350,180)", branch2.X1, branch2.Y1, branch2.X2, branch2.Y2)
	}

	initCircle := findCommitCircle(" init ", 10)
	devCircle := findCommitCircle(" dev-1 ", 10)
	feat1Circle := findCommitCircle(" feat-1 ", 10)
	feat2Circle := findCommitCircle(" feat-2 ", 10)
	releaseCircle := findCommitCircle(" release ", 10)
	mergeOuter := findCommitCircle(" commit-merge ", 6)
	if initCircle == nil || devCircle == nil || feat1Circle == nil || feat2Circle == nil || releaseCircle == nil || mergeOuter == nil {
		t.Fatalf("expected gitGraph commit circles in layout, got %#v", layout.Circles)
	}
	if initCircle.CX != 10 || initCircle.CY != 0 {
		t.Fatalf("init commit position = (%v,%v), want (10,0)", initCircle.CX, initCircle.CY)
	}
	if devCircle.CX != 60 || devCircle.CY != 90 {
		t.Fatalf("dev-1 commit position = (%v,%v), want (60,90)", devCircle.CX, devCircle.CY)
	}
	if feat1Circle.CX != 110 || feat1Circle.CY != 180 {
		t.Fatalf("feat-1 commit position = (%v,%v), want (110,180)", feat1Circle.CX, feat1Circle.CY)
	}
	if feat2Circle.CX != 160 || feat2Circle.CY != 180 {
		t.Fatalf("feat-2 commit position = (%v,%v), want (160,180)", feat2Circle.CX, feat2Circle.CY)
	}
	if releaseCircle.CX != 310 || releaseCircle.CY != 0 {
		t.Fatalf("release commit position = (%v,%v), want (310,0)", releaseCircle.CX, releaseCircle.CY)
	}
}

func TestRenderGanttUsesMermaidGridOffsets(t *testing.T) {
	diagram := `gantt
  title Delivery Plan
  dateFormat YYYY-MM-DD
  section Build
    Core Engine :done, core, 2026-01-01, 10d
    QA Cycle :active, qa, 2026-01-10, 6d`

	svg, err := RenderWithOptions(diagram, DefaultRenderOptions().WithAllowApproximate(true))
	if err != nil {
		t.Fatalf("RenderWithOptions() error = %v", err)
	}
	if !strings.Contains(svg, `class="grid" transform="translate(75, 98)"`) {
		t.Fatalf("expected Mermaid-like gantt grid transform, got: %s", svg)
	}
	if !strings.Contains(svg, `transform="translate(0.5,0)"`) {
		t.Fatalf("expected first gantt tick to start at 0.5, got: %s", svg)
	}
	if !strings.Contains(svg, `y2="-63"`) {
		t.Fatalf("expected gantt tick lines to follow domain height, got: %s", svg)
	}
}

func TestRenderGanttUsesViewportWidthWhenRequested(t *testing.T) {
	diagram := `gantt
  title Delivery Plan
  dateFormat YYYY-MM-DD
  section Build
    Core Engine :done, core, 2026-01-01, 5d
    QA Cycle :active, qa, 2026-01-05, 3d`

	svg, err := RenderWithOptions(
		diagram,
		DefaultRenderOptions().WithAllowApproximate(true).WithViewportSize(1600, 1200),
	)
	if err != nil {
		t.Fatalf("RenderWithOptions() error = %v", err)
	}
	viewBoxWidth, viewBoxHeight, ok := parseSVGViewBoxSizeForTest(svg)
	if !ok {
		t.Fatalf("expected gantt SVG viewBox, got: %s", svg)
	}
	if viewBoxWidth != 1584 || viewBoxHeight != 148 {
		t.Fatalf("viewport-aware gantt viewBox = %fx%f, want 1584x148", viewBoxWidth, viewBoxHeight)
	}
	if !strings.Contains(svg, `H 1434.5`) {
		t.Fatalf("expected viewport-aware gantt domain width 1434.5, got: %s", svg)
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
