package mermaid

import "testing"

func TestParseStateDiagramAvoidsColonNodeArtifacts(t *testing.T) {
	input := `stateDiagram-v2
  [*] --> Idle
  Idle --> Validate: submit
  state Validate {
    [*] --> CheckSchema
    CheckSchema --> CheckAuth
    CheckAuth --> [*]
  }
  Validate --> Approved: ok
  Validate --> Rejected: fail
  state Review <<choice>>
  Approved --> Review
`

	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}

	if _, ok := out.Graph.Nodes["Validate:"]; ok {
		t.Fatalf("unexpected state node with trailing colon")
	}
	if _, ok := out.Graph.Nodes["state"]; ok {
		t.Fatalf("unexpected node created from state keyword")
	}
	if _, ok := out.Graph.Nodes["Validate"]; !ok {
		t.Fatalf("expected Validate node to exist")
	}
	if _, ok := out.Graph.Nodes[stateStartNodeID]; !ok {
		t.Fatalf("expected synthesized start node to exist")
	}
	if _, ok := out.Graph.Nodes[stateEndNodeID]; !ok {
		t.Fatalf("expected synthesized end node to exist")
	}
	if out.Graph.Nodes[stateStartNodeID].Label != "" {
		t.Fatalf("expected synthesized start node label to be empty")
	}
	if out.Graph.Nodes[stateEndNodeID].Label != "" {
		t.Fatalf("expected synthesized end node label to be empty")
	}
	if review, ok := out.Graph.Nodes["Review"]; !ok || review.Shape != ShapeDiamond {
		t.Fatalf("expected Review choice state to keep diamond shape")
	}
}
