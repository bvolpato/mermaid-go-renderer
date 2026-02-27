package mermaid

import "testing"

func TestParseSankeyCSV(t *testing.T) {
	input := `sankey-beta
"Leads","Qualified",120
"Qualified","Won",45
"Qualified","Lost",75
`
	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}
	if out.Graph.Kind != DiagramSankey {
		t.Fatalf("expected sankey kind, got %q", out.Graph.Kind)
	}
	if len(out.Graph.SankeyLinks) != 3 {
		t.Fatalf("expected 3 sankey links, got %d", len(out.Graph.SankeyLinks))
	}
	first := out.Graph.SankeyLinks[0]
	if first.Source != "Leads" || first.Target != "Qualified" || first.Value != 120 {
		t.Fatalf("unexpected first sankey link: %+v", first)
	}
}
