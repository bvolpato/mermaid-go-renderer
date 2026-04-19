package mermaid

import "testing"

func TestParseERDiagramCrowFootRelationships(t *testing.T) {
	input := `erDiagram
    CUSTOMER ||--o{ ORDER : places
    ORDER ||--|{ LINE_ITEM : contains
    CUSTOMER }|..|{ DELIVERY_ADDRESS : uses
    CUSTOMER {
      string name
    }
`

	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}

	if _, ok := out.Graph.Nodes["o{"]; ok {
		t.Fatalf("unexpected cardinality marker parsed as node")
	}
	if _, ok := out.Graph.Nodes["|{"]; ok {
		t.Fatalf("unexpected cardinality marker parsed as node")
	}
	if _, ok := out.Graph.Nodes["string"]; ok {
		t.Fatalf("unexpected attribute type parsed as node")
	}

	for _, id := range []string{"CUSTOMER", "ORDER", "LINE_ITEM", "DELIVERY_ADDRESS"} {
		if _, ok := out.Graph.Nodes[id]; !ok {
			t.Fatalf("expected ER entity node %q", id)
		}
	}
	if len(out.Graph.ERAttributes["CUSTOMER"]) != 1 {
		t.Fatalf("expected CUSTOMER to have one parsed ER attribute, got %d", len(out.Graph.ERAttributes["CUSTOMER"]))
	}

	if len(out.Graph.Edges) != 3 {
		t.Fatalf("expected 3 ER relationships, got %d", len(out.Graph.Edges))
	}
}

func TestParseERDiagramWordCardinalities(t *testing.T) {
	input := `erDiagram
    MANUFACTURER only one to zero or more CAR : makes
    CAR many optionally to one OWNER : belongs to
    a many to 1 1 : label
`

	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}

	for _, id := range []string{"MANUFACTURER", "CAR", "OWNER", "a", "1"} {
		if _, ok := out.Graph.Nodes[id]; !ok {
			t.Fatalf("expected ER entity node %q", id)
		}
	}
	if len(out.Graph.Edges) != 3 {
		t.Fatalf("expected 3 ER relationships, got %d", len(out.Graph.Edges))
	}

	if got := out.Graph.Edges[0].MarkerStart; got != "my-svg_er-onlyOneStart" {
		t.Fatalf("unexpected first marker start: %q", got)
	}
	if got := out.Graph.Edges[0].MarkerEnd; got != "my-svg_er-zeroOrMoreEnd" {
		t.Fatalf("unexpected first marker end: %q", got)
	}
	if out.Graph.Edges[1].Style != EdgeDotted {
		t.Fatalf("expected second relationship to be dotted, got %q", out.Graph.Edges[1].Style)
	}
}
