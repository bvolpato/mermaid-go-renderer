package mermaid

import "testing"

func TestSequenceParsesControlAndActivationEvents(t *testing.T) {
	input := `sequenceDiagram
  participant U as User
  participant W as Web
  participant S as Service
  participant D as DB
  U->>W: Submit order
  W->>+S: validateAndCreate()
  alt valid
    par write order
      S->>+D: insert order
      D-->>-S: order id
    and publish event
      S-->>S: enqueue event
    end
    S-->>W: 201 Created
  else invalid
    S-->>W: 400 Bad Request
  end
  W-->>U: response
`

	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}

	if len(out.Graph.SequenceMessages) != 8 {
		t.Fatalf("expected 8 sequence messages, got %d", len(out.Graph.SequenceMessages))
	}

	expectedKinds := []SequenceEventKind{
		SequenceEventMessage,
		SequenceEventMessage,
		SequenceEventActivateStart,
		SequenceEventAltStart,
		SequenceEventParStart,
		SequenceEventMessage,
		SequenceEventActivateStart,
		SequenceEventMessage,
		SequenceEventActivateEnd,
		SequenceEventParAnd,
		SequenceEventMessage,
		SequenceEventParEnd,
		SequenceEventMessage,
		SequenceEventAltElse,
		SequenceEventMessage,
		SequenceEventAltEnd,
		SequenceEventMessage,
	}
	if len(out.Graph.SequenceEvents) != len(expectedKinds) {
		t.Fatalf("expected %d sequence events, got %d", len(expectedKinds), len(out.Graph.SequenceEvents))
	}
	for idx, kind := range expectedKinds {
		if out.Graph.SequenceEvents[idx].Kind != kind {
			t.Fatalf("event[%d] expected kind %q, got %q", idx, kind, out.Graph.SequenceEvents[idx].Kind)
		}
	}

	if out.Graph.SequenceEvents[2].Actor != "S" {
		t.Fatalf("expected first activation start to target S, got %q", out.Graph.SequenceEvents[2].Actor)
	}
	if out.Graph.SequenceEvents[6].Actor != "D" {
		t.Fatalf("expected second activation start to target D, got %q", out.Graph.SequenceEvents[6].Actor)
	}
	if out.Graph.SequenceEvents[8].Actor != "D" {
		t.Fatalf("expected activation end on D, got %q", out.Graph.SequenceEvents[8].Actor)
	}
	if out.Graph.SequenceEvents[3].Label != "valid" {
		t.Fatalf("expected alt label valid, got %q", out.Graph.SequenceEvents[3].Label)
	}
	if out.Graph.SequenceEvents[4].Label != "write order" {
		t.Fatalf("expected par label write order, got %q", out.Graph.SequenceEvents[4].Label)
	}
	if out.Graph.SequenceEvents[9].Label != "publish event" {
		t.Fatalf("expected and label publish event, got %q", out.Graph.SequenceEvents[9].Label)
	}
	if out.Graph.SequenceEvents[13].Label != "invalid" {
		t.Fatalf("expected else label invalid, got %q", out.Graph.SequenceEvents[13].Label)
	}

	if !out.Graph.SequenceMessages[3].IsReturn {
		t.Fatalf("expected D-->>-S to be flagged as return")
	}
	if !out.Graph.SequenceMessages[4].IsReturn {
		t.Fatalf("expected S-->>S to be flagged as return")
	}
}
