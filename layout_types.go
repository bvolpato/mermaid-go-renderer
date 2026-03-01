package mermaid

type Point struct {
	X float64
	Y float64
}

type NodeLayout struct {
	ID          string
	Label       string
	Shape       NodeShape
	X           float64
	Y           float64
	W           float64
	H           float64
	Fill        string
	Stroke      string
	StrokeWidth float64
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
	Title         string
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
	Class         string
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
	MarkerStart   string
	MarkerEnd     string
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

type SankeyNodeLayout struct {
	ID    string
	Value float64
	X0    float64
	Y0    float64
	X1    float64
	Y1    float64
	Color string
}

type SankeyLinkLayout struct {
	SourceID    string
	TargetID    string
	Value       float64
	Width       float64
	X0          float64
	Y0          float64
	X1          float64
	Y1          float64
	Path        string
	SourceColor string
	TargetColor string
}

type RadarAxisLayout struct {
	Label string
	LineX float64
	LineY float64
	TextX float64
	TextY float64
}

type RadarCurveLayout struct {
	Label   string
	Class   string
	Path    string
	Polygon bool
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

	SankeyNodes []SankeyNodeLayout
	SankeyLinks []SankeyLinkLayout

	MindmapRootID string
	MindmapNodes  []MindmapNode

	RadarTitle            string
	RadarAxes             []RadarAxisLayout
	RadarCurves           []RadarCurveLayout
	RadarLegend           []string
	RadarShowLegend       bool
	RadarTicks            int
	RadarGraticule        string
	RadarGraticuleRadii   []float64
	RadarLegendX          float64
	RadarLegendY          float64
	RadarLegendLineHeight float64

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
