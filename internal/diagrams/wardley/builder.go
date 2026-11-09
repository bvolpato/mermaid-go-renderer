// Ported from mermaid 12.2.0, diagrams/wardley/wardleyBuilder.ts (MIT).

package wardley

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Node is WardleyNode. A Has field is false for a property that is
// undefined upstream. ClassName and SourceStrategy are "" when they are
// undefined: the parser stores no empty text for them.
type Node struct {
	ID               string
	Label            string
	X                float64
	Y                float64
	ClassName        string
	LabelOffsetX     float64
	HasLabelOffsetX  bool
	LabelOffsetY     float64
	HasLabelOffsetY  bool
	InPipeline       bool
	IsPipelineParent bool
	Inertia          bool
	HasInertia       bool
	// SourceStrategy is 'build', 'buy', 'outsource' or 'market'.
	SourceStrategy string
}

// LinkData is WardleyLink. Flow is 'forward', 'backward' or
// 'bidirectional', or "" when it is undefined.
type LinkData struct {
	Source   string
	Target   string
	Dashed   bool
	Label    string
	HasLabel bool
	Flow     string
}

// Trend is WardleyTrend.
type Trend struct {
	NodeID  string
	TargetX float64
	TargetY float64
}

// PipelineData is WardleyPipeline.
type PipelineData struct {
	NodeID       string
	ComponentIDs []string
}

// Point is a coordinate of an annotation.
type Point struct {
	X, Y float64
}

// AnnotationData is WardleyAnnotation.
type AnnotationData struct {
	Number      float64
	Coordinates []Point
	Text        string
}

// NoteData is WardleyNote.
type NoteData struct {
	Text string
	X, Y float64
}

// AcceleratorData is WardleyAccelerator and WardleyDeaccelerator.
type AcceleratorData struct {
	Name string
	X, Y float64
}

// AxesConfig is WardleyAxesConfig. A nil pointer or slice is a property
// that is undefined.
type AxesConfig struct {
	XLabel *string
	YLabel *string
	Stages []string
	// StageBoundaries are optional custom boundaries for stages (0.0 to
	// 1.0).
	StageBoundaries []float64
}

// SizeData is the size of the canvas.
type SizeData struct {
	Width, Height float64
}

// BuildResult is WardleyBuildResult.
type BuildResult struct {
	Nodes          []*Node
	Links          []LinkData
	Trends         []*Trend
	Pipelines      []*PipelineData
	Annotations    []AnnotationData
	Notes          []NoteData
	Accelerators   []AcceleratorData
	Deaccelerators []AcceleratorData
	AnnotationsBox *Point
	Axes           AxesConfig
	Size           *SizeData
}

// Builder is WardleyBuilder.
type Builder struct {
	nodes          *js.Map[string, *Node]
	links          []LinkData
	trends         *js.Map[string, *Trend]
	pipelines      *js.Map[string, *PipelineData]
	annotations    []AnnotationData
	notes          []NoteData
	accelerators   []AcceleratorData
	deaccelerators []AcceleratorData
	annotationsBox *Point
	axes           AxesConfig
	size           *SizeData
}

func newBuilder() *Builder {
	return &Builder{
		nodes:          js.NewMap[string, *Node](),
		links:          []LinkData{},
		trends:         js.NewMap[string, *Trend](),
		pipelines:      js.NewMap[string, *PipelineData](),
		annotations:    []AnnotationData{},
		notes:          []NoteData{},
		accelerators:   []AcceleratorData{},
		deaccelerators: []AcceleratorData{},
	}
}

// AddNode is addNode(node). A node that exists keeps its place and the
// marks of its pipeline, takes the other properties of the new node, and
// keeps its class name and its label offsets where the new node has none.
func (b *Builder) AddNode(node Node) {
	existing, ok := b.nodes.Lookup(node.ID)
	if !ok {
		existing = &Node{ID: node.ID, Label: node.Label}
	}
	// { ...existing, ...node }: the new node has every property that
	// addNode of the db gives it, also the ones that are undefined. Only
	// the marks of the pipeline come from the existing node.
	merged := node
	merged.InPipeline = existing.InPipeline
	merged.IsPipelineParent = existing.IsPipelineParent
	// className: node.className ?? existing.className
	if node.ClassName == "" {
		merged.ClassName = existing.ClassName
	}
	// labelOffsetX: node.labelOffsetX ?? existing.labelOffsetX
	if !node.HasLabelOffsetX {
		merged.LabelOffsetX, merged.HasLabelOffsetX = existing.LabelOffsetX, existing.HasLabelOffsetX
	}
	// labelOffsetY: node.labelOffsetY ?? existing.labelOffsetY
	if !node.HasLabelOffsetY {
		merged.LabelOffsetY, merged.HasLabelOffsetY = existing.LabelOffsetY, existing.HasLabelOffsetY
	}
	b.nodes.Set(node.ID, &merged)
}

// AddLink is addLink(link).
func (b *Builder) AddLink(link LinkData) {
	b.links = append(b.links, link)
}

// AddTrend is addTrend(trend).
func (b *Builder) AddTrend(trend Trend) {
	b.trends.Set(trend.NodeID, &trend)
}

// StartPipeline is startPipeline(nodeId).
func (b *Builder) StartPipeline(nodeID string) {
	b.pipelines.Set(nodeID, &PipelineData{NodeID: nodeID, ComponentIDs: []string{}})

	// Mark the parent node as a pipeline parent
	if node, ok := b.nodes.Lookup(nodeID); ok {
		node.IsPipelineParent = true
	}
}

// AddPipelineComponent is addPipelineComponent(pipelineNodeId, componentId).
func (b *Builder) AddPipelineComponent(pipelineNodeID, componentID string) {
	if pipeline, ok := b.pipelines.Lookup(pipelineNodeID); ok {
		pipeline.ComponentIDs = append(pipeline.ComponentIDs, componentID)
	}

	// Mark the node as being in a pipeline
	if node, ok := b.nodes.Lookup(componentID); ok {
		node.InPipeline = true
	}
}

// AddAnnotation is addAnnotation(annotation).
func (b *Builder) AddAnnotation(annotation AnnotationData) {
	b.annotations = append(b.annotations, annotation)
}

// AddNote is addNote(note).
func (b *Builder) AddNote(note NoteData) {
	b.notes = append(b.notes, note)
}

// AddAccelerator is addAccelerator(accelerator).
func (b *Builder) AddAccelerator(accelerator AcceleratorData) {
	b.accelerators = append(b.accelerators, accelerator)
}

// AddDeaccelerator is addDeaccelerator(deaccelerator).
func (b *Builder) AddDeaccelerator(deaccelerator AcceleratorData) {
	b.deaccelerators = append(b.deaccelerators, deaccelerator)
}

// SetAnnotationsBox is setAnnotationsBox(x, y).
func (b *Builder) SetAnnotationsBox(x, y float64) {
	b.annotationsBox = &Point{X: x, Y: y}
}

// SetAxes is setAxes(partial): { ...this.axes, ...partial }. A property of
// partial that is nil is not a property of the partial object.
func (b *Builder) SetAxes(partial AxesConfig) {
	if partial.XLabel != nil {
		b.axes.XLabel = partial.XLabel
	}
	if partial.YLabel != nil {
		b.axes.YLabel = partial.YLabel
	}
	if partial.Stages != nil {
		b.axes.Stages = partial.Stages
	}
	if partial.StageBoundaries != nil {
		b.axes.StageBoundaries = partial.StageBoundaries
	}
}

// SetSize is setSize(width, height).
func (b *Builder) SetSize(width, height float64) {
	b.size = &SizeData{Width: width, Height: height}
}

// GetNode is getNode(id). The result is nil for a node that does not exist.
func (b *Builder) GetNode(id string) *Node {
	return b.nodes.Get(id)
}

// ResolveNodeID is resolveNodeId(name): it resolves a name to a node ID. It
// tries an exact ID match first, then falls back to finding a node whose
// label matches the name (this handles pipeline components, which have
// synthetic IDs like "Parent_Child").
func (b *Builder) ResolveNodeID(name string) string {
	if b.nodes.Has(name) {
		return name
	}
	for _, id := range b.nodes.Keys() {
		if b.nodes.Get(id).Label == name {
			return id
		}
	}
	return name
}

// Build is build(). Upstream throws for a node that has no coordinates.
// The db gives coordinates to every node, so that does not happen here.
func (b *Builder) Build() *BuildResult {
	return &BuildResult{
		Nodes:          b.nodes.Values(),
		Links:          append([]LinkData{}, b.links...),
		Trends:         b.trends.Values(),
		Pipelines:      b.pipelines.Values(),
		Annotations:    append([]AnnotationData{}, b.annotations...),
		Notes:          append([]NoteData{}, b.notes...),
		Accelerators:   append([]AcceleratorData{}, b.accelerators...),
		Deaccelerators: append([]AcceleratorData{}, b.deaccelerators...),
		AnnotationsBox: b.annotationsBox,
		Axes:           b.axes,
		Size:           b.size,
	}
}
