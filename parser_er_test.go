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
