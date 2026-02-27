package mermaid

import "testing"

func TestParseBlockRowsAndColumns(t *testing.T) {
	input := `block
  columns 3
  A["Ingress"] B{"Validate"} C["Dispatch"]
  D["Retry Queue"] E[("DB")] F["Workers"]
  A --> B
  B --> C
  B --> D
  C --> E
  C --> F
`

	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}

	if out.Graph.Kind != DiagramBlock {
		t.Fatalf("expected DiagramBlock, got %q", out.Graph.Kind)
	}
	if out.Graph.BlockColumns != 3 {
		t.Fatalf("expected 3 columns, got %d", out.Graph.BlockColumns)
	}
	if len(out.Graph.BlockRows) != 2 {
		t.Fatalf("expected 2 block rows, got %d", len(out.Graph.BlockRows))
	}
	if len(out.Graph.BlockRows[0]) != 3 || len(out.Graph.BlockRows[1]) != 3 {
		t.Fatalf("expected two rows with 3 nodes each, got %v", out.Graph.BlockRows)
	}
	if out.Graph.Nodes["A"].Label != "Ingress" {
		t.Fatalf("expected node A label Ingress, got %q", out.Graph.Nodes["A"].Label)
	}
	if out.Graph.Nodes["B"].Shape != ShapeDiamond {
		t.Fatalf("expected node B to be diamond, got %q", out.Graph.Nodes["B"].Shape)
	}
	if out.Graph.Nodes["E"].Shape != ShapeCylinder {
		t.Fatalf("expected node E to be cylinder, got %q", out.Graph.Nodes["E"].Shape)
	}
	if len(out.Graph.Edges) != 5 {
		t.Fatalf("expected 5 edges, got %d", len(out.Graph.Edges))
	}
}
