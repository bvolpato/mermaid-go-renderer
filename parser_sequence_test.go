package mermaid

import "testing"

func TestSequenceParticipantAliasExternalSyntax(t *testing.T) {
	input := `sequenceDiagram
participant A as Alice Johnson
participant B as Bob
A->>B: Hello
`

	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}

	if len(out.Graph.SequenceParticipants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(out.Graph.SequenceParticipants))
	}

	if out.Graph.SequenceParticipants[0] != "A" {
		t.Fatalf("expected first participant id A, got %q", out.Graph.SequenceParticipants[0])
	}
	if out.Graph.SequenceParticipantLabels["A"] != "Alice Johnson" {
		t.Fatalf("expected participant A label to be alias, got %q", out.Graph.SequenceParticipantLabels["A"])
	}
	if out.Graph.SequenceParticipantLabels["B"] != "Bob" {
		t.Fatalf("expected participant B label Bob, got %q", out.Graph.SequenceParticipantLabels["B"])
	}
}

func TestSequenceParticipantAliasInlineMetadata(t *testing.T) {
	input := `sequenceDiagram
participant API@{ "type": "boundary", "alias": "Public API" }
participant DB@{ "type": "database" } as User Database
API->>DB: query
`

	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}

	if out.Graph.SequenceParticipantLabels["API"] != "Public API" {
		t.Fatalf("expected inline alias for API, got %q", out.Graph.SequenceParticipantLabels["API"])
	}
	if out.Graph.SequenceParticipantLabels["DB"] != "User Database" {
		t.Fatalf("expected external alias to take precedence for DB, got %q", out.Graph.SequenceParticipantLabels["DB"])
	}
}

func TestSequenceNotePlacements(t *testing.T) {
	input := `sequenceDiagram
participant Alice
participant Bob
Note right of Bob: Thinking
Note over Alice,Bob: Conversation
Note left of Alice: Done
`

	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}

	if len(out.Graph.SequenceMessages) != 3 {
		t.Fatalf("expected 3 sequence note messages, got %d", len(out.Graph.SequenceMessages))
	}

	right := out.Graph.SequenceMessages[0]
	if right.NotePlacement != SequenceNoteRightOf || right.From != "Bob" || right.To != "Bob" {
		t.Fatalf("unexpected right-of note parse: %#v", right)
	}

	over := out.Graph.SequenceMessages[1]
	if over.NotePlacement != SequenceNoteOver || over.From != "Alice" || over.To != "Bob" {
		t.Fatalf("unexpected over note parse: %#v", over)
	}

	left := out.Graph.SequenceMessages[2]
	if left.NotePlacement != SequenceNoteLeftOf || left.From != "Alice" || left.To != "Alice" {
		t.Fatalf("unexpected left-of note parse: %#v", left)
	}
}
