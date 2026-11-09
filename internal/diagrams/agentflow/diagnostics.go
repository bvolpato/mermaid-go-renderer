// Ported from mermaid 12.2.0, diagrams/agentflow/diagnostics.ts (MIT).

package agentflow

// The ids of the diagnostics that the agentflow db can emit
// (AgentflowWarning). The ids are part of the public contract of upstream.
const (
	// WarningShapeUnsupported is a shape that the renderer does not
	// recognise. The renderer falls back to roundedRect.
	WarningShapeUnsupported = "SHAPE_UNSUPPORTED"
	// WarningShapeRemoved is a shape that v0.8.1 removed. It is an error.
	WarningShapeRemoved = "SHAPE_REMOVED"
	// WarningEdgeOperatorUnsupported is reserved: this package does not
	// emit it.
	WarningEdgeOperatorUnsupported = "EDGE_OPERATOR_UNSUPPORTED"
	// WarningReferenceEdgeLabelRejected is reserved.
	WarningReferenceEdgeLabelRejected = "REFERENCE_EDGE_LABEL_REJECTED"
	// WarningConnectorRefUnresolved is reserved.
	WarningConnectorRefUnresolved = "CONNECTOR_REF_UNRESOLVED"
	// WarningConnectorRefNotAConnector is reserved.
	WarningConnectorRefNotAConnector = "CONNECTOR_REF_NOT_A_CONNECTOR"
	// WarningMetadataKeyMisapplied is reserved.
	WarningMetadataKeyMisapplied = "METADATA_KEY_MISAPPLIED"
	// WarningDuplicateIDNode is reserved.
	WarningDuplicateIDNode = "DUPLICATE_ID_NODE"
	// WarningReservedSyntheticID is reserved.
	WarningReservedSyntheticID = "RESERVED_SYNTHETIC_ID"
	// WarningContainmentViolation is a child of a container that violates
	// the containment rules: a nesting that would close a loop.
	WarningContainmentViolation = "CONTAINMENT_VIOLATION"
	// WarningEdgeSemanticContradiction is reserved.
	WarningEdgeSemanticContradiction = "EDGE_SEMANTIC_CONTRADICTION"
	// WarningFlowNoInput is reserved.
	WarningFlowNoInput = "FLOW_NO_INPUT"
)

// agentflowWarnings is Object.values(AgentflowWarning), in the order of the
// declaration.
var agentflowWarnings = []string{
	WarningShapeUnsupported,
	WarningShapeRemoved,
	WarningEdgeOperatorUnsupported,
	WarningReferenceEdgeLabelRejected,
	WarningConnectorRefUnresolved,
	WarningConnectorRefNotAConnector,
	WarningMetadataKeyMisapplied,
	WarningDuplicateIDNode,
	WarningReservedSyntheticID,
	WarningContainmentViolation,
	WarningEdgeSemanticContradiction,
	WarningFlowNoInput,
}

// ParserEmittedDiagnostics is PARSER_EMITTED_DIAGNOSTICS: the ids that this
// package raises.
var ParserEmittedDiagnostics = []string{
	WarningShapeUnsupported,
	WarningShapeRemoved,
	WarningContainmentViolation,
}

// ReservedDiagnostics is RESERVED_DIAGNOSTICS: the ids that are reserved
// for the semantics module of upstream. Nothing in this package emits them.
func ReservedDiagnostics() []string {
	out := []string{}
next:
	for _, id := range agentflowWarnings {
		for _, emitted := range ParserEmittedDiagnostics {
			if id == emitted {
				continue next
			}
		}
		out = append(out, id)
	}
	return out
}

// Diagnostic is AgentflowDiagnostic: one warning or error of the db.
type Diagnostic struct {
	ID string
	// Severity is "warning" or "error".
	Severity string
	Message  string
	// NodeID and EdgeID are "" when the property is undefined.
	NodeID string
	EdgeID string
	// Position is the position of the element, or nil.
	Position *ElementPosition
}

// DiagnosticContext is AgentflowDiagnosticContext: the context that the
// caller of emitWarning gives. An empty EdgeID is undefined. HasNodeID says
// that nodeId is defined: upstream takes the position of the node also for
// a node id that is the empty string, and then finds none.
type DiagnosticContext struct {
	NodeID    string
	HasNodeID bool
	EdgeID    string
}

// nodeContext is { nodeId: id }.
func nodeContext(id string) DiagnosticContext {
	return DiagnosticContext{NodeID: id, HasNodeID: true}
}
