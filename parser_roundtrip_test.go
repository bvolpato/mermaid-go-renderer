package mermaid

import (
	"testing"
)

func TestParseFlowchartStructure(t *testing.T) {
	input := `flowchart LR
  A[Start] --> B{Decision}
  B -->|yes| C[Done]
  B -->|no| D[Retry]
  D --> B`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramFlowchart {
		t.Fatalf("kind = %s, want flowchart", g.Kind)
	}
	if g.Direction != DirectionLeftRight {
		t.Fatalf("direction = %s, want LR", g.Direction)
	}
	wantNodes := 4
	if len(g.NodeOrder) != wantNodes {
		t.Fatalf("node count = %d, want %d", len(g.NodeOrder), wantNodes)
	}
	wantEdges := 4
	if len(g.Edges) != wantEdges {
		t.Fatalf("edge count = %d, want %d", len(g.Edges), wantEdges)
	}

	assertNodeLabel(t, g, "A", "Start")
	assertNodeLabel(t, g, "B", "Decision")
	assertNodeLabel(t, g, "C", "Done")
	assertNodeLabel(t, g, "D", "Retry")

	assertNodeShape(t, g, "A", ShapeRectangle)
	assertNodeShape(t, g, "B", ShapeDiamond)
}

func TestParseFlowchartAllDirections(t *testing.T) {
	cases := []struct {
		header string
		want   Direction
	}{
		{"flowchart TD", DirectionTopDown},
		{"flowchart TB", DirectionTopDown},
		{"flowchart BT", DirectionBottomTop},
		{"flowchart LR", DirectionLeftRight},
		{"flowchart RL", DirectionRightLeft},
		{"graph LR", DirectionLeftRight},
	}
	for _, tc := range cases {
		t.Run(tc.header, func(t *testing.T) {
			input := tc.header + "\nA --> B"
			parsed, err := ParseMermaid(input)
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if parsed.Graph.Direction != tc.want {
				t.Fatalf("got %s, want %s", parsed.Graph.Direction, tc.want)
			}
		})
	}
}

func TestParseFlowchartShapes(t *testing.T) {
	input := `flowchart LR
  A[Square]
  B(Round)
  C{Diamond}
  D([Stadium])
  E((Circle))
  F[[Subroutine]]`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	assertNodeShape(t, g, "A", ShapeRectangle)
	assertNodeShape(t, g, "B", ShapeRoundRect)
	assertNodeShape(t, g, "C", ShapeDiamond)
	assertNodeShape(t, g, "D", ShapeStadium)
	assertNodeShape(t, g, "E", ShapeCircle)
	assertNodeShape(t, g, "F", ShapeSubroutine)
}

func TestParseFlowchartSubgraphs(t *testing.T) {
	input := `flowchart TB
  subgraph Backend
    A[API]
    B[Worker]
  end
  subgraph Frontend
    C[Web]
  end
  A --> C`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if len(g.FlowSubgraphs) < 2 {
		t.Fatalf("subgraph count = %d, want >= 2", len(g.FlowSubgraphs))
	}
	if len(g.NodeOrder) < 3 {
		t.Fatalf("node count = %d, want >= 3", len(g.NodeOrder))
	}
}

func TestParseFlowchartEdgeStyles(t *testing.T) {
	input := `flowchart TD
  A --> B
  B -.-> C
  C ==> D
  D --- E`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if len(g.Edges) < 4 {
		t.Fatalf("edge count = %d, want >= 4", len(g.Edges))
	}

	foundSolid := false
	foundDotted := false
	foundThick := false
	for _, e := range g.Edges {
		switch e.Style {
		case EdgeSolid:
			foundSolid = true
		case EdgeDotted:
			foundDotted = true
		case EdgeThick:
			foundThick = true
		}
	}
	if !foundSolid {
		t.Error("expected solid edge")
	}
	if !foundDotted {
		t.Error("expected dotted edge")
	}
	if !foundThick {
		t.Error("expected thick edge")
	}
}

func TestParseSequenceDiagramStructure(t *testing.T) {
	input := `sequenceDiagram
  participant Alice
  participant Bob
  participant API
  Alice->>Bob: Hello
  Bob->>API: Fetch data
  API-->>Bob: Response
  Bob-->>Alice: Result`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramSequence {
		t.Fatalf("kind = %s, want sequence", g.Kind)
	}
	if len(g.SequenceParticipants) != 3 {
		t.Fatalf("participant count = %d, want 3", len(g.SequenceParticipants))
	}
	if len(g.SequenceMessages) != 4 {
		t.Fatalf("message count = %d, want 4", len(g.SequenceMessages))
	}

	msg := g.SequenceMessages[0]
	if msg.From != "Alice" || msg.To != "Bob" || msg.Label != "Hello" {
		t.Fatalf("first message: from=%s to=%s label=%s", msg.From, msg.To, msg.Label)
	}
}

func TestParseSequenceWithControlFlow(t *testing.T) {
	input := `sequenceDiagram
  participant Client
  participant Server
  Client->>Server: request
  alt valid
    Server-->>Client: ok
  else invalid
    Server-->>Client: error
  end`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if len(g.SequenceParticipants) != 2 {
		t.Fatalf("participant count = %d, want 2", len(g.SequenceParticipants))
	}
	if len(g.SequenceEvents) == 0 {
		t.Fatal("expected sequence events for alt/else/end")
	}

	foundAltStart := false
	foundAltElse := false
	foundAltEnd := false
	for _, ev := range g.SequenceEvents {
		switch ev.Kind {
		case SequenceEventAltStart:
			foundAltStart = true
		case SequenceEventAltElse:
			foundAltElse = true
		case SequenceEventAltEnd:
			foundAltEnd = true
		}
	}
	if !foundAltStart || !foundAltElse || !foundAltEnd {
		t.Fatalf("missing alt control flow events: start=%v else=%v end=%v", foundAltStart, foundAltElse, foundAltEnd)
	}
}

func TestParseClassDiagramStructure(t *testing.T) {
	input := `classDiagram
  class Animal {
    +int age
    +String name
    +eat()
    +sleep()
  }
  class Dog {
    +bark()
  }
  Animal <|-- Dog`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramClass {
		t.Fatalf("kind = %s, want class", g.Kind)
	}
	if _, ok := g.Nodes["Animal"]; !ok {
		t.Fatal("missing Animal node")
	}
	if _, ok := g.Nodes["Dog"]; !ok {
		t.Fatal("missing Dog node")
	}
	if len(g.Edges) < 1 {
		t.Fatalf("edge count = %d, want >= 1", len(g.Edges))
	}
}

func TestParseStateDiagramStructure(t *testing.T) {
	input := `stateDiagram-v2
  [*] --> Idle
  Idle --> Running
  Running --> Done
  Done --> [*]`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramState {
		t.Fatalf("kind = %s, want state", g.Kind)
	}
	if len(g.Edges) < 4 {
		t.Fatalf("edge count = %d, want >= 4", len(g.Edges))
	}
}

func TestParseERDiagramStructure(t *testing.T) {
	input := `erDiagram
  USER {
    string id
    string email
  }
  ORDER {
    string id
    float total
  }
  USER ||--o{ ORDER : places`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramER {
		t.Fatalf("kind = %s, want er", g.Kind)
	}
	if _, ok := g.Nodes["USER"]; !ok {
		t.Fatal("missing USER node")
	}
	if _, ok := g.Nodes["ORDER"]; !ok {
		t.Fatal("missing ORDER node")
	}
	if len(g.Edges) < 1 {
		t.Fatal("expected at least 1 edge")
	}
}

func TestParsePieChartStructure(t *testing.T) {
	input := `pie showData
  title Pet Adoption
  "Dogs" : 45
  "Cats" : 30
  "Birds" : 15
  "Fish" : 10`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramPie {
		t.Fatalf("kind = %s, want pie", g.Kind)
	}
	if g.PieTitle != "Pet Adoption" {
		t.Fatalf("title = %q, want Pet Adoption", g.PieTitle)
	}
	if !g.PieShowData {
		t.Fatal("expected PieShowData=true")
	}
	if len(g.PieSlices) != 4 {
		t.Fatalf("slice count = %d, want 4", len(g.PieSlices))
	}
	if g.PieSlices[0].Label != "Dogs" || g.PieSlices[0].Value != 45 {
		t.Fatalf("slice[0] = %+v", g.PieSlices[0])
	}
}

func TestParseMindmapStructure(t *testing.T) {
	input := `mindmap
  root((Root))
    Branch A
      Leaf A1
      Leaf A2
    Branch B`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramMindmap {
		t.Fatalf("kind = %s, want mindmap", g.Kind)
	}
	if len(g.MindmapNodes) < 4 {
		t.Fatalf("mindmap node count = %d, want >= 4", len(g.MindmapNodes))
	}
}

func TestParseJourneyStructure(t *testing.T) {
	input := `journey
  title Onboarding
  section Setup
    Create account: 5: User
    Verify email: 3: User
  section First Use
    Explore features: 4: User`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramJourney {
		t.Fatalf("kind = %s, want journey", g.Kind)
	}
	if g.JourneyTitle != "Onboarding" {
		t.Fatalf("title = %q, want Onboarding", g.JourneyTitle)
	}
	if len(g.JourneySteps) != 3 {
		t.Fatalf("step count = %d, want 3", len(g.JourneySteps))
	}
}

func TestParseGanttStructure(t *testing.T) {
	input := `gantt
  title Project Plan
  section Phase 1
  Design :done, des, 2026-01-01, 10d
  Implement :active, impl, after des, 15d
  section Phase 2
  Test :test, after impl, 7d`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramGantt {
		t.Fatalf("kind = %s, want gantt", g.Kind)
	}
	if g.GanttTitle != "Project Plan" {
		t.Fatalf("title = %q, want Project Plan", g.GanttTitle)
	}
	if len(g.GanttTasks) < 3 {
		t.Fatalf("task count = %d, want >= 3", len(g.GanttTasks))
	}
}

func TestParseGitGraphStructure(t *testing.T) {
	input := `gitGraph
  commit
  branch feature
  checkout feature
  commit id:"feat-1"
  commit id:"feat-2"
  checkout main
  merge feature`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramGitGraph {
		t.Fatalf("kind = %s, want gitgraph", g.Kind)
	}
	if len(g.GitBranches) < 2 {
		t.Fatalf("branch count = %d, want >= 2", len(g.GitBranches))
	}
	if len(g.GitCommits) < 3 {
		t.Fatalf("commit count = %d, want >= 3", len(g.GitCommits))
	}
}

func TestParseTimelineStructure(t *testing.T) {
	input := `timeline
  title Company History
  2020 : Founded
  2021 : Series A : Product Launch
  2022 : IPO`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramTimeline {
		t.Fatalf("kind = %s, want timeline", g.Kind)
	}
	if g.TimelineTitle != "Company History" {
		t.Fatalf("title = %q, want Company History", g.TimelineTitle)
	}
	if len(g.TimelineEvents) < 3 {
		t.Fatalf("event count = %d, want >= 3", len(g.TimelineEvents))
	}
}

func TestParseTimelineContinuesEventsForBlankTimeLines(t *testing.T) {
	input := `timeline
  title Product Timeline
  2024 : alpha
  2025 : beta
       : ga`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if len(g.TimelineEvents) != 2 {
		t.Fatalf("event count = %d, want 2", len(g.TimelineEvents))
	}
	if g.TimelineEvents[1].Time != "2025" {
		t.Fatalf("second event time = %q, want 2025", g.TimelineEvents[1].Time)
	}
	if len(g.TimelineEvents[1].Events) != 2 {
		t.Fatalf("second event item count = %d, want 2", len(g.TimelineEvents[1].Events))
	}
	if g.TimelineEvents[1].Events[0] != "beta" || g.TimelineEvents[1].Events[1] != "ga" {
		t.Fatalf("second event items = %#v, want beta/ga", g.TimelineEvents[1].Events)
	}
}

func TestParseQuadrantStructure(t *testing.T) {
	input := `quadrantChart
  title Feature Priorities
  x-axis Low Effort --> High Effort
  y-axis Low Impact --> High Impact
  Quick Win: [0.2, 0.8]
  Big Project: [0.8, 0.9]
  Filler: [0.3, 0.2]`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramQuadrant {
		t.Fatalf("kind = %s, want quadrant", g.Kind)
	}
	if len(g.QuadrantPoints) < 3 {
		t.Fatalf("point count = %d, want >= 3", len(g.QuadrantPoints))
	}
}

func TestParseXYChartStructure(t *testing.T) {
	input := `xychart-beta
  title Revenue
  x-axis [Q1, Q2, Q3, Q4]
  y-axis 0 --> 200
  bar [50, 100, 150, 180]
  line [40, 90, 140, 170]`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramXYChart {
		t.Fatalf("kind = %s, want xychart", g.Kind)
	}
	if len(g.XYSeries) < 2 {
		t.Fatalf("series count = %d, want >= 2", len(g.XYSeries))
	}
	if len(g.XYXCategories) != 4 {
		t.Fatalf("x-axis categories = %d, want 4", len(g.XYXCategories))
	}
}

func TestParseSankeyStructure(t *testing.T) {
	input := `sankey-beta
A,B,10
A,C,5
B,D,8
C,D,3`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramSankey {
		t.Fatalf("kind = %s, want sankey", g.Kind)
	}
	if len(g.SankeyLinks) != 4 {
		t.Fatalf("link count = %d, want 4", len(g.SankeyLinks))
	}
}

func TestParseC4Structure(t *testing.T) {
	input := `C4Context
  title System Context
  Person(user, "End User")
  System(app, "Application")
  System_Ext(ext, "External API")
  Rel(user, app, "Uses")
  Rel(app, ext, "Calls")`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramC4 {
		t.Fatalf("kind = %s, want c4", g.Kind)
	}
	if g.C4Title != "System Context" {
		t.Fatalf("title = %q, want System Context", g.C4Title)
	}
}

func TestParseRequirementStructure(t *testing.T) {
	input := `requirementDiagram
  requirement perf {
    id: 1
    text: must be fast
    risk: low
    verifymethod: test
  }
  element engine {
    type: system
  }
  engine - satisfies -> perf`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramRequirement {
		t.Fatalf("kind = %s, want requirement", g.Kind)
	}
	if len(g.NodeOrder) < 2 {
		t.Fatalf("node count = %d, want >= 2", len(g.NodeOrder))
	}
}

func TestParseArchitectureStructure(t *testing.T) {
	input := `architecture-beta
  group api(cloud)[API Layer]
  service gateway(server)[Gateway] in api
  service db(database)[Database] in api
  gateway:R -- L:db`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramArchitecture {
		t.Fatalf("kind = %s, want architecture", g.Kind)
	}
	if len(g.ArchitectureGroups) < 1 {
		t.Fatalf("group count = %d, want >= 1", len(g.ArchitectureGroups))
	}
	if len(g.ArchitectureServices) < 2 {
		t.Fatalf("service count = %d, want >= 2", len(g.ArchitectureServices))
	}
}

func TestParseRadarStructure(t *testing.T) {
	input := `radar-beta
  title Skills Assessment
  axis speed["Speed"], quality["Quality"], cost["Cost"], time["Time"]
  curve team_a["Team A"]{80, 70, 60, 90}
  curve team_b["Team B"]{65, 85, 75, 55}
  max 100`

	parsed, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}
	g := parsed.Graph

	if g.Kind != DiagramRadar {
		t.Fatalf("kind = %s, want radar", g.Kind)
	}
	if g.RadarTitle != "Skills Assessment" {
		t.Fatalf("title = %q, want Skills Assessment", g.RadarTitle)
	}
	if len(g.RadarAxes) != 4 {
		t.Fatalf("axis count = %d, want 4", len(g.RadarAxes))
	}
	if len(g.RadarCurves) != 2 {
		t.Fatalf("curve count = %d, want 2", len(g.RadarCurves))
	}
}

func TestDetectDiagramKindAll(t *testing.T) {
	cases := []struct {
		prefix string
		want   DiagramKind
	}{
		{"flowchart LR", DiagramFlowchart},
		{"graph TD", DiagramFlowchart},
		{"sequenceDiagram", DiagramSequence},
		{"classDiagram", DiagramClass},
		{"stateDiagram-v2", DiagramState},
		{"erDiagram", DiagramER},
		{"pie", DiagramPie},
		{"mindmap", DiagramMindmap},
		{"journey", DiagramJourney},
		{"timeline", DiagramTimeline},
		{"gantt", DiagramGantt},
		{"requirementDiagram", DiagramRequirement},
		{"gitGraph", DiagramGitGraph},
		{"C4Context", DiagramC4},
		{"sankey-beta", DiagramSankey},
		{"quadrantChart", DiagramQuadrant},
		{"zenuml", DiagramZenUML},
		{"block-beta", DiagramBlock},
		{"packet-beta", DiagramPacket},
		{"kanban", DiagramKanban},
		{"architecture-beta", DiagramArchitecture},
		{"radar-beta", DiagramRadar},
		{"treemap-beta", DiagramTreemap},
		{"xychart-beta", DiagramXYChart},
	}
	for _, tc := range cases {
		t.Run(tc.prefix, func(t *testing.T) {
			got := detectDiagramKind(tc.prefix + "\nA-->B")
			if got != tc.want {
				t.Fatalf("detectDiagramKind(%q) = %s, want %s", tc.prefix, got, tc.want)
			}
		})
	}
}

func assertNodeLabel(t *testing.T, g Graph, id, wantLabel string) {
	t.Helper()
	node, ok := g.Nodes[id]
	if !ok {
		t.Fatalf("node %q not found", id)
	}
	if node.Label != wantLabel {
		t.Fatalf("node %q label = %q, want %q", id, node.Label, wantLabel)
	}
}

func assertNodeShape(t *testing.T, g Graph, id string, wantShape NodeShape) {
	t.Helper()
	node, ok := g.Nodes[id]
	if !ok {
		t.Fatalf("node %q not found", id)
	}
	if node.Shape != wantShape {
		t.Fatalf("node %q shape = %s, want %s", id, node.Shape, wantShape)
	}
}
