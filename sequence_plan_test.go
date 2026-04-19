package mermaid

import (
	"strings"
	"testing"
)

func TestBuildSequencePlanMatchesMermaidSequenceNotesFixture(t *testing.T) {
	diagram := strings.TrimSpace(`sequenceDiagram
  Alice->>Bob: Hello
  Note right of Bob: Thinking
  Bob-->>Alice: Hi
  Note over Alice,Bob: Conversation`)

	parsed, err := ParseMermaid(diagram)
	if err != nil {
		t.Fatalf("ParseMermaid() error = %v", err)
	}

	plan := buildSequencePlan(
		parsed.Graph.SequenceParticipants,
		parsed.Graph.SequenceParticipantLabels,
		parsed.Graph.SequenceMessages,
		parsed.Graph.SequenceEvents,
		ModernTheme(),
	)

	if plan.ViewBoxWidth != 550 || plan.ViewBoxHeight != 353 {
		t.Fatalf("unexpected viewBox size: %fx%f", plan.ViewBoxWidth, plan.ViewBoxHeight)
	}
	if plan.BottomY != 267 || plan.LifelineEndY != 267 {
		t.Fatalf("unexpected sequence footer positions: bottom=%f lifeline=%f", plan.BottomY, plan.LifelineEndY)
	}
	if len(plan.MessageLayouts) != 4 {
		t.Fatalf("expected 4 message layouts, got %d", len(plan.MessageLayouts))
	}

	first := plan.MessageLayouts[0]
	if first.LineY != 109 || first.TextY != 80 {
		t.Fatalf("unexpected first message positions: line=%f text=%f", first.LineY, first.TextY)
	}

	rightNote := plan.MessageLayouts[1]
	if !rightNote.Note || rightNote.StartX != 300 || rightNote.StopX != 450 || rightNote.LineY != 119 || rightNote.Height != 37 || rightNote.TextY != 124 {
		t.Fatalf("unexpected right note layout: %#v", rightNote)
	}

	second := plan.MessageLayouts[2]
	if second.LineY != 200 || second.TextY != 171 {
		t.Fatalf("unexpected second message positions: line=%f text=%f", second.LineY, second.TextY)
	}

	overNote := plan.MessageLayouts[3]
	if !overNote.Note || overNote.StartX != 50 || overNote.StopX != 300 || overNote.LineY != 210 || overNote.Height != 37 || overNote.TextY != 215 {
		t.Fatalf("unexpected over note layout: %#v", overNote)
	}
}

func TestRenderSequenceNotesIncludesPlacedNotes(t *testing.T) {
	diagram := strings.TrimSpace(`sequenceDiagram
  Alice->>Bob: Hello
  Note right of Bob: Thinking
  Bob-->>Alice: Hi
  Note over Alice,Bob: Conversation`)

	svg, err := RenderWithOptions(diagram, DefaultRenderOptions().WithAllowApproximate(true))
	if err != nil {
		t.Fatalf("RenderWithOptions() error = %v", err)
	}

	for _, want := range []string{
		`viewBox="-50 -10 550 353"`,
		`<rect x="0" y="0" fill="#ECECFF" stroke="#9370DB" width="150" height="65" name="Alice" rx="3" ry="3" class="actor actor-top"/>`,
		`<rect x="200" y="267" fill="#ECECFF" stroke="#9370DB" width="150" height="65" name="Bob" rx="3" ry="3" class="actor actor-bottom"/>`,
		`<rect x="300" y="119" fill="#EDF2AE" stroke="#666" width="150" height="37" class="note"/>`,
		`<rect x="50" y="210" fill="#EDF2AE" stroke="#666" width="250" height="37" class="note"/>`,
		`>Thinking</text>`,
		`>Conversation</text>`,
	} {
		if !strings.Contains(svg, want) {
			t.Fatalf("expected rendered sequence SVG to contain %q, got: %s", want, svg)
		}
	}
}
