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
	From       string
	To         string
	Label      string
	X1         float64
	Y1         float64
	X2         float64
	Y2         float64
	Style      EdgeStyle
	ArrowStart bool
	ArrowEnd   bool
}

type LayoutRect struct {
	X           float64
	Y           float64
	W           float64
	H           float64
	RX          float64
	RY          float64
	Fill        string
	Stroke      string
	StrokeWidth float64
	Dashed      bool
}

type LayoutLine struct {
	X1          float64
	Y1          float64
	X2          float64
	Y2          float64
	Stroke      string
	StrokeWidth float64
	Dashed      bool
	ArrowStart  bool
	ArrowEnd    bool
}

type LayoutCircle struct {
	CX          float64
	CY          float64
	R           float64
	Fill        string
	Stroke      string
	StrokeWidth float64
}

type LayoutPolygon struct {
	Points      []Point
	Fill        string
	Stroke      string
	StrokeWidth float64
}

type LayoutPath struct {
	D           string
	Fill        string
	Stroke      string
	StrokeWidth float64
}

type LayoutText struct {
	X      float64
	Y      float64
	Value  string
	Anchor string
	Size   float64
	Weight string
	Color  string
}

type Layout struct {
	Kind   DiagramKind
	Width  float64
	Height float64

	Nodes []NodeLayout
	Edges []EdgeLayout

	Rects    []LayoutRect
	Lines    []LayoutLine
	Circles  []LayoutCircle
	Polygons []LayoutPolygon
	Paths    []LayoutPath
	Texts    []LayoutText
}
