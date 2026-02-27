package mermaid

import "fmt"

type Direction string

const (
	DirectionTopDown   Direction = "TD"
	DirectionBottomTop Direction = "BT"
	DirectionLeftRight Direction = "LR"
	DirectionRightLeft Direction = "RL"
)

func directionFromToken(token string) Direction {
	switch upper(token) {
	case "TD", "TB":
		return DirectionTopDown
	case "BT":
		return DirectionBottomTop
	case "LR":
		return DirectionLeftRight
	case "RL":
		return DirectionRightLeft
	default:
		return DirectionTopDown
	}
}

type DiagramKind string

const (
	DiagramFlowchart    DiagramKind = "flowchart"
	DiagramSequence     DiagramKind = "sequence"
	DiagramClass        DiagramKind = "class"
	DiagramState        DiagramKind = "state"
	DiagramER           DiagramKind = "er"
	DiagramPie          DiagramKind = "pie"
	DiagramMindmap      DiagramKind = "mindmap"
	DiagramJourney      DiagramKind = "journey"
	DiagramTimeline     DiagramKind = "timeline"
	DiagramGantt        DiagramKind = "gantt"
	DiagramRequirement  DiagramKind = "requirement"
	DiagramGitGraph     DiagramKind = "gitgraph"
	DiagramC4           DiagramKind = "c4"
	DiagramSankey       DiagramKind = "sankey"
	DiagramQuadrant     DiagramKind = "quadrant"
	DiagramZenUML       DiagramKind = "zenuml"
	DiagramBlock        DiagramKind = "block"
	DiagramPacket       DiagramKind = "packet"
	DiagramKanban       DiagramKind = "kanban"
	DiagramArchitecture DiagramKind = "architecture"
	DiagramRadar        DiagramKind = "radar"
	DiagramTreemap      DiagramKind = "treemap"
	DiagramXYChart      DiagramKind = "xychart"
)

type NodeShape string

const (
	ShapeRectangle     NodeShape = "rectangle"
	ShapeRoundRect     NodeShape = "round-rect"
	ShapeStadium       NodeShape = "stadium"
	ShapeSubroutine    NodeShape = "subroutine"
	ShapeCylinder      NodeShape = "cylinder"
	ShapeCircle        NodeShape = "circle"
	ShapeDoubleCircle  NodeShape = "double-circle"
	ShapeDiamond       NodeShape = "diamond"
	ShapeHexagon       NodeShape = "hexagon"
	ShapeParallelogram NodeShape = "parallelogram"
	ShapeTrapezoid     NodeShape = "trapezoid"
	ShapeAsymmetric    NodeShape = "asymmetric"
)

type EdgeStyle string

const (
	EdgeSolid  EdgeStyle = "solid"
	EdgeDotted EdgeStyle = "dotted"
	EdgeThick  EdgeStyle = "thick"
)

type Node struct {
	ID    string
	Label string
	Shape NodeShape
}

type Edge struct {
	From       string
	To         string
	Label      string
	Directed   bool
	ArrowStart bool
	ArrowEnd   bool
	Style      EdgeStyle
}

type SequenceMessage struct {
	From  string
	To    string
	Label string
	Arrow string
}

type PieSlice struct {
	Label string
	Value float64
}

type GanttTask struct {
	ID       string
	Label    string
	Section  string
	Start    string
	Duration string
	Status   string
}

type TimelineEvent struct {
	Time    string
	Events  []string
	Section string
}

type JourneyStep struct {
	Label   string
	Score   float64
	Actors  []string
	Section string
}

type MindmapNode struct {
	ID     string
	Label  string
	Level  int
	Parent string
}

type GitCommit struct {
	ID     string
	Branch string
	Label  string
}

type XYSeriesKind string

const (
	XYSeriesBar  XYSeriesKind = "bar"
	XYSeriesLine XYSeriesKind = "line"
)

type XYSeries struct {
	Kind   XYSeriesKind
	Label  string
	Values []float64
}

type QuadrantPoint struct {
	Label string
	X     float64
	Y     float64
}

type Graph struct {
	Kind      DiagramKind
	Direction Direction
	Source    string

	Nodes     map[string]Node
	NodeOrder []string
	Edges     []Edge

	SequenceParticipants []string
	SequenceMessages     []SequenceMessage

	PieTitle    string
	PieShowData bool
	PieSlices   []PieSlice

	GanttTitle    string
	GanttSections []string
	GanttTasks    []GanttTask

	TimelineTitle    string
	TimelineSections []string
	TimelineEvents   []TimelineEvent

	JourneyTitle string
	JourneySteps []JourneyStep

	MindmapRootID string
	MindmapNodes  []MindmapNode

	GitMainBranch string
	GitBranches   []string
	GitCommits    []GitCommit

	XYTitle       string
	XYXAxisLabel  string
	XYXCategories []string
	XYYAxisLabel  string
	XYYMin        *float64
	XYYMax        *float64
	XYSeries      []XYSeries

	QuadrantTitle       string
	QuadrantXAxisLeft   string
	QuadrantXAxisRight  string
	QuadrantYAxisBottom string
	QuadrantYAxisTop    string
	QuadrantLabels      [4]string
	QuadrantPoints      []QuadrantPoint

	GenericLines []string
}

func newGraph(kind DiagramKind) Graph {
	return Graph{
		Kind:      kind,
		Direction: DirectionTopDown,
		Nodes:     map[string]Node{},
	}
}

func (g *Graph) ensureNode(id, label string, shape NodeShape) {
	if id == "" {
		return
	}
	if shape == "" {
		shape = ShapeRectangle
	}
	if label == "" {
		label = id
	}
	if _, ok := g.Nodes[id]; !ok {
		g.NodeOrder = append(g.NodeOrder, id)
	}
	g.Nodes[id] = Node{
		ID:    id,
		Label: label,
		Shape: shape,
	}
}

func (g *Graph) addEdge(e Edge) {
	if e.From == "" || e.To == "" {
		return
	}
	if e.Style == "" {
		e.Style = EdgeSolid
	}
	g.Edges = append(g.Edges, e)
}

func (k DiagramKind) IsGraphLike() bool {
	switch k {
	case DiagramFlowchart, DiagramClass, DiagramState, DiagramER, DiagramRequirement,
		DiagramC4, DiagramSankey, DiagramZenUML, DiagramBlock, DiagramPacket,
		DiagramKanban, DiagramArchitecture, DiagramRadar, DiagramTreemap:
		return true
	default:
		return false
	}
}

func (k DiagramKind) String() string {
	return string(k)
}

func mustKindLabel(k DiagramKind) string {
	switch k {
	case DiagramFlowchart:
		return "Flowchart"
	case DiagramSequence:
		return "Sequence Diagram"
	case DiagramClass:
		return "Class Diagram"
	case DiagramState:
		return "State Diagram"
	case DiagramER:
		return "ER Diagram"
	case DiagramPie:
		return "Pie Chart"
	case DiagramMindmap:
		return "Mindmap"
	case DiagramJourney:
		return "Journey"
	case DiagramTimeline:
		return "Timeline"
	case DiagramGantt:
		return "Gantt"
	case DiagramRequirement:
		return "Requirement Diagram"
	case DiagramGitGraph:
		return "Git Graph"
	case DiagramC4:
		return "C4"
	case DiagramSankey:
		return "Sankey"
	case DiagramQuadrant:
		return "Quadrant"
	case DiagramZenUML:
		return "ZenUML"
	case DiagramBlock:
		return "Block"
	case DiagramPacket:
		return "Packet"
	case DiagramKanban:
		return "Kanban"
	case DiagramArchitecture:
		return "Architecture"
	case DiagramRadar:
		return "Radar"
	case DiagramTreemap:
		return "Treemap"
	case DiagramXYChart:
		return "XY Chart"
	default:
		return fmt.Sprintf("%s", k)
	}
}
