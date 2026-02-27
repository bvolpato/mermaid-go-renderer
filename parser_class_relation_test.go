package mermaid

import "testing"

func TestParseClassRelationMarkers(t *testing.T) {
	input := `classDiagram
  A <|-- B
  C --> D
  E o-- F
  G *-- H
  I ..> J
  K -- L
`
	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}
	if out.Graph.Kind != DiagramClass {
		t.Fatalf("expected class kind, got %q", out.Graph.Kind)
	}

	edgeByPair := map[string]Edge{}
	for _, edge := range out.Graph.Edges {
		edgeByPair[edge.From+"->"+edge.To] = edge
	}

	assertEdge := func(pair string) Edge {
		edge, ok := edgeByPair[pair]
		if !ok {
			t.Fatalf("missing expected edge %s", pair)
		}
		return edge
	}

	e := assertEdge("A->B")
	if e.MarkerStart != "my-svg_class-extensionStart" || e.MarkerEnd != "" {
		t.Fatalf("unexpected inheritance markers: %+v", e)
	}

	e = assertEdge("C->D")
	if e.MarkerEnd != "my-svg_class-dependencyEnd" || e.MarkerStart != "" {
		t.Fatalf("unexpected dependency markers: %+v", e)
	}

	e = assertEdge("E->F")
	if e.MarkerStart != "my-svg_class-aggregationStart" || e.MarkerEnd != "" {
		t.Fatalf("unexpected aggregation markers: %+v", e)
	}

	e = assertEdge("G->H")
	if e.MarkerStart != "my-svg_class-compositionStart" || e.MarkerEnd != "" {
		t.Fatalf("unexpected composition markers: %+v", e)
	}

	e = assertEdge("I->J")
	if e.MarkerEnd != "my-svg_class-dependencyEnd" || e.Style != EdgeDotted {
		t.Fatalf("unexpected dotted dependency edge: %+v", e)
	}

	e = assertEdge("K->L")
	if e.MarkerStart != "" || e.MarkerEnd != "" {
		t.Fatalf("plain association should not have markers: %+v", e)
	}
}
