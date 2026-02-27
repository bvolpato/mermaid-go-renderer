package mermaid

import "testing"

func TestParseClassDiagramCollectsMembersMethods(t *testing.T) {
	input := `classDiagram
direction LR
class Animal{
  +int age
  +String gender
  +isMammal()
}
class Duck
Animal <|-- Duck
Duck : +swim()
`

	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}

	if out.Graph.Kind != DiagramClass {
		t.Fatalf("expected class diagram kind, got %q", out.Graph.Kind)
	}
	if len(out.Graph.NodeOrder) != 2 {
		t.Fatalf("expected 2 class nodes, got %d", len(out.Graph.NodeOrder))
	}
	if _, ok := out.Graph.Nodes["+int"]; ok {
		t.Fatalf("unexpected member line parsed as node")
	}
	if len(out.Graph.ClassMembers["Animal"]) != 2 {
		t.Fatalf("expected 2 Animal members, got %d", len(out.Graph.ClassMembers["Animal"]))
	}
	if len(out.Graph.ClassMethods["Animal"]) != 1 {
		t.Fatalf("expected 1 Animal method, got %d", len(out.Graph.ClassMethods["Animal"]))
	}
	if len(out.Graph.ClassMethods["Duck"]) != 1 {
		t.Fatalf("expected 1 Duck method, got %d", len(out.Graph.ClassMethods["Duck"]))
	}
	if len(out.Graph.Edges) != 1 {
		t.Fatalf("expected 1 class relation edge, got %d", len(out.Graph.Edges))
	}
}
