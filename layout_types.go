package mermaid

type Point struct {
	X float64
	Y float64
}

type NodeLayout struct {
	ID    string
	Label string
	Shape NodeShape
	X     float64
	Y     float64
	W     float64
	H     float64
}

type EdgeLayout struct {
	From        string
	To          string
	Label       string
	X1          float64
	Y1          float64
	X2          float64
	Y2          float64
	Style       EdgeStyle
	ArrowStart  bool
	ArrowEnd    bool
	MarkerStart string
	MarkerEnd   string
}

type LayoutRect struct {
	ID              string
	Class           string
	X               float64
	Y               float64
	W               float64
	H               float64
	RX              float64
	RY              float64
	Fill            string
	FillOpacity     float64
	Stroke          string
	StrokeOpacity   float64
	StrokeWidth     float64
	Opacity         float64
	Transform       string
	TransformOrigin string
	StrokeDasharray string
	Dashed          bool
}

type LayoutLine struct {
	ID            string
	Class         string
	X1            float64
	Y1            float64
	X2            float64
	Y2            float64
	Stroke        string
	StrokeOpacity float64
	StrokeWidth   float64
	Opacity       float64
	LineCap       string
	LineJoin      string
	DashArray     string
	Transform     string
	Dashed        bool
	ArrowStart    bool
	ArrowEnd      bool
	MarkerStart   string
	MarkerEnd     string
}

type LayoutCircle struct {
	ID            string
	Class         string
	CX            float64
	CY            float64
	R             float64
	Fill          string
	FillOpacity   float64
	Stroke        string
	StrokeOpacity float64
	StrokeWidth   float64
	Opacity       float64
	Transform     string
}

type LayoutEllipse struct {
	ID            string
	Class         string
	CX            float64
	CY            float64
	RX            float64
	RY            float64
	Fill          string
	FillOpacity   float64
	Stroke        string
	StrokeOpacity float64
	StrokeWidth   float64
	Opacity       float64
	Transform     string
}

type LayoutPolygon struct {
	Points        []Point
	Fill          string
	FillOpacity   float64
	Stroke        string
	StrokeOpacity float64
	StrokeWidth   float64
	Opacity       float64
	Transform     string
}

type LayoutPath struct {
	ID            string
	Class         string
	D             string
	Fill          string
	FillOpacity   float64
	Stroke        string
	StrokeOpacity float64
	StrokeWidth   float64
	Opacity       float64
	Transform     string
	DashArray     string
	LineCap       string
	LineJoin      string
}

type LayoutText struct {
	ID               string
	Class            string
	X                float64
	Y                float64
	BoxX             float64
	BoxY             float64
	BoxW             float64
	BoxH             float64
	Value            string
	Anchor           string
	Size             float64
	Weight           string
	Color            string
	Opacity          float64
	Transform        string
	DominantBaseline string
	FontFamily       string
}

type ArchitectureGroupLayout struct {
	ID    string
	Label string
	Icon  string
	X     float64
	Y     float64
	W     float64
	H     float64
}

type ArchitectureServiceLayout struct {
	ID      string
	Label   string
	Icon    string
	GroupID string
	X       float64
	Y       float64
	W       float64
	H       float64
}

type Layout struct {
	Kind   DiagramKind
	Width  float64
	Height float64

	ViewBoxX      float64
	ViewBoxY      float64
	ViewBoxWidth  float64
	ViewBoxHeight float64
	SVGWidth      string
	SVGHeight     string
	SVGStyle      string

	SequenceParticipants      []string
	SequenceMessages          []SequenceMessage
	SequenceEvents            []SequenceEvent
	SequenceParticipantLabels map[string]string

	ZenUMLTitle        string
	ZenUMLParticipants []string
	ZenUMLMessages     []SequenceMessage
	ZenUMLAltBlocks    []ZenUMLAltBlock

	ArchitectureGroups   []ArchitectureGroupLayout
	ArchitectureServices []ArchitectureServiceLayout

	MindmapRootID string
	MindmapNodes  []MindmapNode

	Nodes []NodeLayout
	Edges []EdgeLayout

	Rects    []LayoutRect
	Lines    []LayoutLine
	Circles  []LayoutCircle
	Ellipses []LayoutEllipse
	Polygons []LayoutPolygon
	Paths    []LayoutPath
	Texts    []LayoutText
}
