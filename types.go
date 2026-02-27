package mermaid

import (
	"fmt"
	"strings"
)

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
	From        string
	To          string
	Label       string
	Directed    bool
	ArrowStart  bool
	ArrowEnd    bool
	MarkerStart string
	MarkerEnd   string
	Style       EdgeStyle
}

type SequenceMessage struct {
	From     string
	To       string
	Label    string
	Arrow    string
	Index    string
	IsReturn bool
}

type SequenceEventKind string

const (
	SequenceEventMessage       SequenceEventKind = "message"
	SequenceEventAltStart      SequenceEventKind = "alt_start"
	SequenceEventAltElse       SequenceEventKind = "alt_else"
	SequenceEventAltEnd        SequenceEventKind = "alt_end"
	SequenceEventParStart      SequenceEventKind = "par_start"
	SequenceEventParAnd        SequenceEventKind = "par_and"
	SequenceEventParEnd        SequenceEventKind = "par_end"
	SequenceEventActivateStart SequenceEventKind = "activate_start"
	SequenceEventActivateEnd   SequenceEventKind = "activate_end"
)

type SequenceEvent struct {
	Kind         SequenceEventKind
	MessageIndex int
	Label        string
	Actor        string
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
	After    string
}

type TimelineEvent struct {
	Time    string
	Events  []string
	Section string
}

type JourneyStep struct {
	ID       string
	Label    string
	Score    float64
	HasScore bool
	Actors   []string
	Section  string
}

type SankeyLink struct {
	Source string
	Target string
	Value  float64
}

type RadarAxis struct {
	Name  string
	Label string
}

type RadarCurve struct {
	Name    string
	Label   string
	Entries []float64
}

type PacketField struct {
	Start int
	End   int
	Label string
}

type TreemapItem struct {
	Depth    int
	Label    string
	Value    float64
	HasValue bool
}

type KanbanCard struct {
	ID       string
	Title    string
	Ticket   string
	Assigned string
	Priority string
}

type KanbanColumn struct {
	Title string
	Cards []KanbanCard
}

type MindmapNode struct {
	ID     string
	Label  string
	Level  int
	Parent string
	Shape  NodeShape
}

type GitCommit struct {
	ID            string
	Branch        string
	Label         string
	Message       string
	Seq           int
	CommitType    GitGraphCommitType
	CustomType    GitGraphCommitType
	HasCustomType bool
	Tags          []string
	Parents       []string
	CustomID      bool
}

type GitBranch struct {
	Name           string
	Order          *float64
	InsertionIndex int
}

type GitGraphCommitType string

const (
	GitGraphCommitTypeNormal     GitGraphCommitType = "NORMAL"
	GitGraphCommitTypeMerge      GitGraphCommitType = "MERGE"
	GitGraphCommitTypeReverse    GitGraphCommitType = "REVERSE"
	GitGraphCommitTypeHighlight  GitGraphCommitType = "HIGHLIGHT"
	GitGraphCommitTypeCherryPick GitGraphCommitType = "CHERRY-PICK"
)

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

type ArchitectureGroup struct {
	ID    string
	Label string
	Icon  string
}

type ArchitectureService struct {
	ID      string
	Label   string
	Icon    string
	GroupID string
}

type ArchitectureEndpoint struct {
	ID   string
	Side string
}

type ArchitectureLink struct {
	From ArchitectureEndpoint
	To   ArchitectureEndpoint
}

type ZenUMLAltBlock struct {
	Condition string
	Start     int
	ElseStart int
	End       int
}

type Graph struct {
	Kind      DiagramKind
	Direction Direction
	Source    string

	Nodes     map[string]Node
	NodeOrder []string
	Edges     []Edge

	SequenceParticipants      []string
	SequenceMessages          []SequenceMessage
	SequenceEvents            []SequenceEvent
	SequenceParticipantLabels map[string]string
	ZenUMLTitle               string
	ZenUMLAltBlocks           []ZenUMLAltBlock

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

	SankeyLinks []SankeyLink

	RadarTitle      string
	RadarAxes       []RadarAxis
	RadarCurves     []RadarCurve
	RadarShowLegend bool
	RadarTicks      int
	RadarMax        *float64
	RadarMin        *float64
	RadarGraticule  string

	PacketTitle  string
	PacketFields []PacketField

	TreemapItems []TreemapItem
	KanbanBoard  []KanbanColumn
	BlockColumns int
	BlockRows    [][]string

	MindmapRootID string
	MindmapNodes  []MindmapNode

	GitMainBranch string
	GitBranches   []string
	GitBranchDefs []GitBranch
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

	ClassMembers map[string][]string
	ClassMethods map[string][]string
	ERAttributes map[string][]string

	ArchitectureGroups   []ArchitectureGroup
	ArchitectureServices []ArchitectureService
	ArchitectureLinks    []ArchitectureLink

	GenericLines []string
}

func newGraph(kind DiagramKind) Graph {
	return Graph{
		Kind:                      kind,
		Direction:                 DirectionTopDown,
		Nodes:                     map[string]Node{},
		RadarShowLegend:           true,
		RadarTicks:                5,
		RadarGraticule:            "circle",
		SequenceParticipantLabels: map[string]string{},
		ClassMembers:              map[string][]string{},
		ClassMethods:              map[string][]string{},
		ERAttributes:              map[string][]string{},
	}
}

func (g *Graph) ensureNode(id, label string, shape NodeShape) {
	if id == "" {
		return
	}
	if existing, ok := g.Nodes[id]; ok {
		if strings.TrimSpace(label) == "" || strings.TrimSpace(label) == id {
			if strings.TrimSpace(existing.Label) != "" {
				label = existing.Label
			}
		}
		if shape == "" || shape == ShapeRectangle {
			if existing.Shape != "" {
				shape = existing.Shape
			}
		}
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
