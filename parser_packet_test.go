package mermaid

import "testing"

func TestParsePacketFields(t *testing.T) {
	input := `packet
title TCP Header
0-15: "Source Port"
16-31: "Destination Port"
120-135: "Checksum"
`

	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}
	if out.Graph.Kind != DiagramPacket {
		t.Fatalf("expected packet kind, got %q", out.Graph.Kind)
	}
	if out.Graph.PacketTitle != "TCP Header" {
		t.Fatalf("unexpected packet title: %q", out.Graph.PacketTitle)
	}
	if len(out.Graph.PacketFields) != 3 {
		t.Fatalf("expected 3 packet fields, got %d", len(out.Graph.PacketFields))
	}
	if out.Graph.PacketFields[0].Start != 0 || out.Graph.PacketFields[0].End != 15 {
		t.Fatalf("unexpected first packet field range: %d-%d",
			out.Graph.PacketFields[0].Start, out.Graph.PacketFields[0].End)
	}
	if out.Graph.PacketFields[2].Label != "Checksum" {
		t.Fatalf("unexpected last packet field label: %q", out.Graph.PacketFields[2].Label)
	}
}
