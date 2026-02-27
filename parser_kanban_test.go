package mermaid

import "testing"

func TestParseKanbanBoard(t *testing.T) {
	input := `kanban
  Backlog
    t1[Design auth flow]@{ ticket: SEC-101, assigned: "alice", priority: "High" }
  Done
    t2[Document MFA rollout]
`

	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}
	if out.Graph.Kind != DiagramKanban {
		t.Fatalf("expected kanban kind, got %q", out.Graph.Kind)
	}
	if len(out.Graph.KanbanBoard) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(out.Graph.KanbanBoard))
	}
	if len(out.Graph.KanbanBoard[0].Cards) != 1 {
		t.Fatalf("expected one card in first column, got %d", len(out.Graph.KanbanBoard[0].Cards))
	}
	card := out.Graph.KanbanBoard[0].Cards[0]
	if card.Ticket != "SEC-101" || card.Assigned != "alice" || card.Priority != "High" {
		t.Fatalf("unexpected card metadata: %+v", card)
	}
}
