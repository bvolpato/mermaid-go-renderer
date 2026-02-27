package mermaid

import "testing"

func TestParseZenUMLUsesSequenceParticipantsAndMessages(t *testing.T) {
	input := `zenuml
title Demo
A as Alice
J as John
@Actor Bot
A->J: Hello
J->Bot: ping
`

	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}

	if out.Graph.Kind != DiagramZenUML {
		t.Fatalf("expected DiagramZenUML, got %q", out.Graph.Kind)
	}
	if len(out.Graph.SequenceParticipants) != 3 {
		t.Fatalf("expected 3 participants, got %d", len(out.Graph.SequenceParticipants))
	}
	if len(out.Graph.SequenceMessages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(out.Graph.SequenceMessages))
	}
	if out.Graph.SequenceParticipantLabels["A"] != "Alice" {
		t.Fatalf("expected alias label for participant A")
	}
}
