package mermaid

import "testing"

func TestParseTreemap(t *testing.T) {
	input := `treemap-beta
"Revenue"
  "SMB"
    "Self-serve": 140
`

	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}
	if out.Graph.Kind != DiagramTreemap {
		t.Fatalf("expected treemap kind, got %q", out.Graph.Kind)
	}
	if len(out.Graph.TreemapItems) != 3 {
		t.Fatalf("expected 3 treemap items, got %d", len(out.Graph.TreemapItems))
	}
	if out.Graph.TreemapItems[2].Depth != 2 {
		t.Fatalf("expected leaf depth 2, got %d", out.Graph.TreemapItems[2].Depth)
	}
	if !out.Graph.TreemapItems[2].HasValue || out.Graph.TreemapItems[2].Value != 140 {
		t.Fatalf("expected parsed leaf value 140, got %+v", out.Graph.TreemapItems[2])
	}
}
