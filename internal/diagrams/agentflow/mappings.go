// Ported from mermaid 12.2.0, diagrams/agentflow/agentflowDb.ts: the element
// mappings, the diagnostics and the semantic model (MIT).

package agentflow

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// deriveVertexKind is deriveVertexKind(v, resolvedShape): the vertex kind
// of v0.8.1 for a parsed vertex. resolvedShape is "" when it is undefined.
//
// The kind does not follow from the resolved shape alone: a tool and a task
// can both be a roundedRect. So the function takes the vertex too.
func (d *DB) deriveVertexKind(v *FlowVertex, resolvedShape string) VertexKind {
	if d.IsToolDefinition(v) {
		return "tool"
	}
	if resolvedShape == "hexagon" || resolvedShape == "hex" {
		return "action"
	}
	if resolvedShape == "lean-right" || resolvedShape == "lean_right" {
		return "input"
	}
	if resolvedShape == "lin-doc" || resolvedShape == "lined-document" {
		return "refdoc"
	}
	if resolvedShape == "diamond" {
		return "decision"
	}
	return "task"
}

// semanticMetadata is the metadata of an element for the semantic model:
// the metadata without the presentation keys. The result is nil when no
// key is left.
func semanticMetadata(metadata *js.Obj[any]) *js.Obj[any] {
	if metadata == nil || metadata.Len() == 0 {
		return nil
	}
	// Strip presentation-only keys from metadata passthrough.
	meta := js.NewObj[any]()
	metadata.Each(func(key string, value any) {
		if semanticMetadataSkipKeys[key] {
			return
		}
		meta.Set(key, value)
	})
	if meta.Len() == 0 {
		return nil
	}
	return meta
}

// GetSemanticModel is getSemanticModel(): a view of the diagram without the
// presentation fields, for the tools that read the meaning of the diagram.
// The fields with a meaning stay: the ids, the labels, the shape, the
// domain metadata, the arrow, the stroke and the label of an edge, the
// members of a container, and the diagnostics.
func (d *DB) GetSemanticModel() *SemanticModel {
	// Run the post-parse validators so that the semantic export includes
	// up-to-date diagnostics.
	d.runPostParseValidators()
	// Subgraph ids sometimes also appear in `this.vertices` when metadata
	// (`@{...}`) is attached to a container id: the metadata-attachment
	// path creates a placeholder vertex record. Those are NOT semantic
	// vertices; they're container descriptors and the semantic model
	// exposes them via `subGraphs` instead.
	subGraphIDs := map[string]bool{}
	for _, sg := range d.subGraphs {
		subGraphIDs[sg.ID] = true
	}
	vertices := []*SemanticVertex{}
	connectors := []*SemanticConnector{}
	d.vertices.Each(func(id string, v *FlowVertex) {
		if subGraphIDs[id] {
			return
		}
		if v.IsConnector {
			connector := &SemanticConnector{ID: id}
			if v.HasText && v.Text != id {
				connector.Title, connector.HasTitle = v.Text, true
			}
			connector.Metadata = semanticMetadata(v.Metadata)
			connectors = append(connectors, connector)
			return
		}
		vertex := &SemanticVertex{ID: id}
		if v.HasText {
			vertex.Label, vertex.HasLabel = v.Text, true
		}
		resolvedShape := resolvedType(v)
		if v.HasType {
			vertex.Shape, vertex.HasShape = resolvedShape, true
		}
		// Derived vertex kind per v0.8.1 section 4.
		vertex.VertexKind = d.deriveVertexKind(v, resolvedShape)
		vertex.Metadata = semanticMetadata(v.Metadata)
		vertices = append(vertices, vertex)
	})

	edges := make([]*SemanticEdge, 0, len(d.edges))
	for _, e := range d.edges {
		edge := &SemanticEdge{Start: e.Start, End: e.End, Length: undef}
		edge.ID = e.ID
		if len(e.Text) > 0 {
			edge.Label, edge.HasLabel = e.Text, true
		}
		edge.Type = e.Type
		edge.Stroke = e.Stroke
		edge.EdgeSemantic = e.EdgeSemantic
		edge.Length = e.Length
		if e.Metadata != nil && e.Metadata.Len() > 0 {
			edge.Metadata = spread(e.Metadata)
		}
		edges = append(edges, edge)
	}

	subGraphs := make([]*SemanticSubGraph, 0, len(d.subGraphs))
	for _, sg := range d.subGraphs {
		out := &SemanticSubGraph{
			ID:    sg.ID,
			Nodes: append([]string{}, sg.Nodes...),
		}
		out.Type = sg.Type
		out.Title = sg.Title
		out.Direction = sg.dirJS()
		out.Metadata = semanticMetadata(sg.Metadata)
		subGraphs = append(subGraphs, out)
	}

	model := &SemanticModel{
		Vertices:    vertices,
		Edges:       edges,
		SubGraphs:   subGraphs,
		Connectors:  connectors,
		Diagnostics: d.diagnostics,
	}
	if d.hasDirection {
		model.Direction, model.HasDirection = d.direction, true
	}
	return model
}

// The element mappings. The semantic actions of the grammar call the
// add*Mapping functions beside the add* functions of the structure.

// SetFrontmatterLineOffset is setFrontmatterLineOffset(offset): the number
// of lines of the front matter, which the parser does not see.
func (d *DB) SetFrontmatterLineOffset(offset int) {
	d.frontmatterLineOffset = offset
}

// toElementPosition is toElementPosition(loc). The grammar has no ranges,
// so the indexes are 0. A nil location is the missing location of upstream:
// every value is 0 then.
func (d *DB) toElementPosition(loc *Location) *ElementPosition {
	if loc == nil {
		return &ElementPosition{
			StartLine: d.frontmatterLineOffset,
			EndLine:   d.frontmatterLineOffset,
		}
	}
	return &ElementPosition{
		StartLine:   loc.FirstLine + d.frontmatterLineOffset,
		StartColumn: loc.FirstColumn,
		EndLine:     loc.LastLine + d.frontmatterLineOffset,
		EndColumn:   loc.LastColumn,
		StartIndex:  0,
		EndIndex:    0,
	}
}

// pushMapping is pushMapping(id, type, loc). The result is nil when the id
// is empty.
func (d *DB) pushMapping(id, typ string, loc *Location) *ElementMapping {
	if id == "" {
		return nil
	}
	mapping := &ElementMapping{
		ID:       id,
		Type:     typ,
		Position: d.toElementPosition(loc),
	}
	d.elementMappings = append(d.elementMappings, mapping)
	return mapping
}

// AddVertexMapping is addVertexMapping(id, _text, shape, loc). hasShape is
// false for a bare `id` reference: the declarations with brackets all give
// a shape.
func (d *DB) AddVertexMapping(id string, hasShape bool, loc *Location) {
	mapping := d.pushMapping(id, "vertex", loc)
	// A bare `id` reference carries no shape (the bracketed declaration
	// forms all pass one). Remember it so a trailing `@{ ... }` block can
	// retype it to 'attachment' instead of leaving a second mapping that
	// looks like a redeclaration (issue #75).
	if mapping != nil && !hasShape {
		d.bareVertexMappings[mapping] = true
	}
}

// ExtendVertexMapping is extendVertexMapping(id, loc): it moves the end of
// the mapping of a vertex to the end of a `@{ ... }` block after the vertex
// (`id["..."]@{ ... }`), so that a cursor inside the block finds the node.
// loc is the location of the shape data, so only the end moves. When the
// node has no mapping, the block gets a mapping of its own.
//
// A mapping of a bare `id` reference changes its type to "attachment": the
// statement `id@{ ... }` annotates an element that another statement
// declares.
func (d *DB) ExtendVertexMapping(id string, loc *Location) {
	if id == "" || loc == nil {
		return
	}
	end := d.toElementPosition(loc)
	for i := len(d.elementMappings) - 1; i >= 0; i-- {
		m := d.elementMappings[i]
		if m.Type == "vertex" && m.ID == id {
			if d.bareVertexMappings[m] {
				m.Type = "attachment"
				delete(d.bareVertexMappings, m)
			}
			extendsPastEnd := end.EndLine > m.Position.EndLine ||
				(end.EndLine == m.Position.EndLine && end.EndColumn > m.Position.EndColumn)
			if extendsPastEnd {
				m.Position.EndLine = end.EndLine
				m.Position.EndColumn = end.EndColumn
				m.Position.EndIndex = end.EndIndex
			}
			return
		}
	}
	// No vertex mapping to widen: the `@{ ... }` block stands alone, so
	// record it as an attachment spanning just the block.
	d.pushMapping(id, "attachment", loc)
}

// AddEdgeMapping is addEdgeMapping(_fromStmt, toNodes, _link, loc). The id
// of the mapping is the ids of the destination nodes joined by ">": the
// statement `A --> B & C` is one edge statement with two destinations.
func (d *DB) AddEdgeMapping(toNodes []string, loc *Location) {
	edgeID := ""
	count := 0
	for _, n := range toNodes {
		if n == "" {
			continue
		}
		if count > 0 {
			edgeID += ">"
		}
		edgeID += n
		count++
	}
	if count == 0 {
		edgeID = "edge"
	}
	d.pushMapping(edgeID, "edge", loc)
}

// spanMapping adds a mapping that spans from the start of one location to
// the end of another: the keyword of a container to its `end`.
func (d *DB) spanMapping(id, typ string, startLoc, endLoc *Location) {
	start := d.toElementPosition(startLoc)
	end := start
	if endLoc != nil {
		end = d.toElementPosition(endLoc)
	}
	if id == "" {
		return
	}
	d.elementMappings = append(d.elementMappings, &ElementMapping{
		ID:   id,
		Type: typ,
		// Range spans from the container keyword to its `end`: startLine from
		// the opener, endLine from the closer.
		Position: &ElementPosition{
			StartLine:   start.StartLine,
			StartColumn: start.StartColumn,
			EndLine:     end.EndLine,
			EndColumn:   end.EndColumn,
			StartIndex:  start.StartIndex,
			EndIndex:    end.EndIndex,
		},
	})
}

// AddSubgraphMapping is addSubgraphMapping(_id, _title, startLoc, endLoc).
// _id is the text object of the id, or nil. The id of the mapping is the
// text as it is, with its white space.
func (d *DB) AddSubgraphMapping(_id *FlowText, startLoc, endLoc *Location) {
	id := ""
	if _id != nil {
		id = _id.Text
	}
	d.spanMapping(id, "subgraph", startLoc, endLoc)
}

// AddConnectorMapping is addConnectorMapping(textObj, _titleObj, startLoc,
// endLoc).
func (d *DB) AddConnectorMapping(textObj *FlowText, startLoc, endLoc *Location) {
	id := ""
	if textObj != nil {
		id = js.TrimSpace(textObj.Text)
	}
	if id == "" {
		return
	}
	d.spanMapping(id, "connector", startLoc, endLoc)
}

// GetElementMappings is getElementMappings(): a copy of the list of the
// mappings.
func (d *DB) GetElementMappings() []*ElementMapping {
	return append([]*ElementMapping{}, d.elementMappings...)
}

// GetElementById is getElementById(id): the first mapping of an id, or nil.
func (d *DB) GetElementById(id string) *ElementMapping {
	for _, m := range d.elementMappings {
		if m.ID == id {
			return m
		}
	}
	return nil
}

// GetElementsOnLine is getElementsOnLine(line).
func (d *DB) GetElementsOnLine(line int) []*ElementMapping {
	out := []*ElementMapping{}
	for _, m := range d.elementMappings {
		if line >= m.Position.StartLine && line <= m.Position.EndLine {
			out = append(out, m)
		}
	}
	return out
}

// GetElementAtPosition is getElementAtPosition(line, column): the mapping
// with the smallest span that contains the point, or nil. So a point in
// nested containers finds the deepest container.
func (d *DB) GetElementAtPosition(line, column int) *ElementMapping {
	var smallest *ElementMapping
	for _, m := range d.elementMappings {
		p := m.Position
		if line < p.StartLine || line > p.EndLine {
			continue
		}
		if line == p.StartLine && column < p.StartColumn {
			continue
		}
		if line == p.EndLine && column > p.EndColumn {
			continue
		}
		if smallest == nil {
			smallest = m
			continue
		}
		// Compare [lineSpan, columnSpan] lexicographically. Folding the two into
		// `lineSpan * 1000 + columnSpan` mis-ranked lines wider than 1000 columns,
		// and the column term goes negative whenever a span crosses a line break.
		smallLines := smallest.Position.EndLine - smallest.Position.StartLine
		smallColumns := smallest.Position.EndColumn - smallest.Position.StartColumn
		curLines := p.EndLine - p.StartLine
		curColumns := p.EndColumn - p.StartColumn
		if curLines != smallLines {
			if curLines < smallLines {
				smallest = m
			}
			continue
		}
		if curColumns < smallColumns {
			smallest = m
		}
	}
	return smallest
}

// MappingStats is the result of getMappingStats().
type MappingStats struct {
	Vertices      int
	Edges         int
	Subgraphs     int
	Connectors    int
	Attachments   int
	TotalElements int
}

// GetMappingStats is getMappingStats(): the number of mappings of each
// type.
func (d *DB) GetMappingStats() MappingStats {
	var s MappingStats
	for _, m := range d.elementMappings {
		switch m.Type {
		case "vertex":
			s.Vertices++
		case "edge":
			s.Edges++
		case "subgraph":
			s.Subgraphs++
		case "connector":
			s.Connectors++
		case "attachment":
			s.Attachments++
		}
	}
	s.TotalElements = len(d.elementMappings)
	return s
}

// The diagnostics. emitWarning and emitError record a diagnostic, and
// upstream also writes it to the log. With a nodeId or an edgeId in the
// context, the diagnostic gets the position of the element from the
// element mappings.

// emitDiagnostic is emitDiagnostic(id, severity, message, ctx).
func (d *DB) emitDiagnostic(id, severity, message string, ctx DiagnosticContext) {
	// ctx?.nodeId ?? ctx?.edgeId
	anchorID := ctx.NodeID
	if !ctx.HasNodeID && ctx.NodeID == "" {
		anchorID = ctx.EdgeID
	}
	var mapping *ElementMapping
	if anchorID != "" {
		mapping = d.GetElementById(anchorID)
	}
	diagnostic := &Diagnostic{
		ID:       id,
		Severity: severity,
		Message:  message,
	}
	if ctx.NodeID != "" {
		diagnostic.NodeID = ctx.NodeID
	}
	if ctx.EdgeID != "" && ctx.NodeID == "" {
		diagnostic.EdgeID = ctx.EdgeID
	}
	if mapping != nil {
		diagnostic.Position = mapping.Position
	}
	// `getData()` is idempotent and may be called more than once (parse-then-
	// render, or tooling probing the DB). Re-emitting the same finding each
	// time would inflate the diagnostics list, so record each one once.
	for _, existing := range d.diagnostics {
		if existing.ID == diagnostic.ID &&
			existing.Severity == diagnostic.Severity &&
			existing.Message == diagnostic.Message &&
			existing.NodeID == diagnostic.NodeID &&
			existing.EdgeID == diagnostic.EdgeID {
			return
		}
	}
	d.diagnostics = append(d.diagnostics, diagnostic)
}

// EmitWarning is emitWarning(id, message, ctx).
func (d *DB) EmitWarning(id, message string, ctx DiagnosticContext) {
	d.emitDiagnostic(id, "warning", message, ctx)
}

// EmitError is emitError(id, message, ctx).
func (d *DB) EmitError(id, message string, ctx DiagnosticContext) {
	d.emitDiagnostic(id, "error", message, ctx)
}

// GetDiagnostics is getDiagnostics(): a copy of the list of the
// diagnostics.
func (d *DB) GetDiagnostics() []*Diagnostic {
	return append([]*Diagnostic{}, d.diagnostics...)
}
