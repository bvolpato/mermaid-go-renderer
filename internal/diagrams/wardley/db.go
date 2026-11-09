// Ported from mermaid 12.2.0, diagrams/wardley/wardleyDb.ts (MIT).

package wardley

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// DB is the db of the wardley diagram. Upstream has one builder for the
// page. Here each db has its own.
type DB struct {
	render.CommonDB
	ctx     *render.Context
	builder *Builder
}

func newDB(ctx *render.Context) *DB {
	db := &DB{ctx: ctx, builder: newBuilder()}
	db.Sanitize = ctx.SanitizeText
	return db
}

// GetConfig is getConfig(): the 'wardley-beta' block of the config. It is
// nil when the config has no such block.
func (db *DB) GetConfig() any {
	return db.ctx.Config.Get("wardley-beta")
}

// AddNode is addNode(id, label, x, y, className, labelOffsetX,
// labelOffsetY, inertia, sourceStrategy).
func (db *DB) AddNode(node Node) {
	db.builder.AddNode(node)
}

// AddLink is addLink(sourceId, targetId, dashed, label, flow).
func (db *DB) AddLink(link LinkData) {
	db.builder.AddLink(link)
}

// AddTrend is addTrend(nodeId, targetX, targetY).
func (db *DB) AddTrend(nodeID string, targetX, targetY float64) {
	db.builder.AddTrend(Trend{NodeID: nodeID, TargetX: targetX, TargetY: targetY})
}

// AddAnnotation is addAnnotation(number, coordinates, text).
func (db *DB) AddAnnotation(number float64, coordinates []Point, text string) {
	db.builder.AddAnnotation(AnnotationData{Number: number, Coordinates: coordinates, Text: text})
}

// AddNote is addNote(text, x, y).
func (db *DB) AddNote(text string, x, y float64) {
	db.builder.AddNote(NoteData{Text: text, X: x, Y: y})
}

// AddAccelerator is addAccelerator(name, x, y).
func (db *DB) AddAccelerator(name string, x, y float64) {
	db.builder.AddAccelerator(AcceleratorData{Name: name, X: x, Y: y})
}

// AddDeaccelerator is addDeaccelerator(name, x, y).
func (db *DB) AddDeaccelerator(name string, x, y float64) {
	db.builder.AddDeaccelerator(AcceleratorData{Name: name, X: x, Y: y})
}

// SetAnnotationsBox is setAnnotationsBox(x, y).
func (db *DB) SetAnnotationsBox(x, y float64) {
	db.builder.SetAnnotationsBox(x, y)
}

// SetSize is setSize(width, height).
func (db *DB) SetSize(width, height float64) {
	db.builder.SetSize(width, height)
}

// StartPipeline is startPipeline(nodeId).
func (db *DB) StartPipeline(nodeID string) {
	db.builder.StartPipeline(nodeID)
}

// AddPipelineComponent is addPipelineComponent(pipelineNodeId, componentId).
func (db *DB) AddPipelineComponent(pipelineNodeID, componentID string) {
	db.builder.AddPipelineComponent(pipelineNodeID, componentID)
}

// UpdateAxes is updateAxes(partial).
func (db *DB) UpdateAxes(partial AxesConfig) {
	db.builder.SetAxes(partial)
}

// GetNode is getNode(id).
func (db *DB) GetNode(id string) *Node {
	return db.builder.GetNode(id)
}

// ResolveNodeID is resolveNodeId(name).
func (db *DB) ResolveNodeID(name string) string {
	return db.builder.ResolveNodeID(name)
}

// GetWardleyData is getWardleyData().
func (db *DB) GetWardleyData() *BuildResult {
	return db.builder.Build()
}
