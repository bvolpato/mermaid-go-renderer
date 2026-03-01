package mermaid

import "testing"

func TestParseEdgeLineInlineDottedLabel(t *testing.T) {
	left, label, right, meta, ok := parseEdgeLine("F -. async .-> G[(Audit Log)]")
	if !ok {
		t.Fatalf("expected edge to parse")
	}
	if left != "F" {
		t.Fatalf("unexpected left node: %q", left)
	}
	if label != "async" {
		t.Fatalf("unexpected edge label: %q", label)
	}
	if right != "G[(Audit Log)]" {
		t.Fatalf("unexpected right node: %q", right)
	}
	if meta.style != EdgeDotted {
		t.Fatalf("expected dotted style, got %q", meta.style)
	}
	if !meta.arrowEnd {
		t.Fatalf("expected directed edge with arrow end")
	}
}

func TestSplitEdgeChainSkipsInlineLabelEdge(t *testing.T) {
	if out := splitEdgeChain("F -. async .-> G[(Audit Log)]"); out != nil {
		t.Fatalf("expected inline label edge to remain unsplit, got %#v", out)
	}
}
