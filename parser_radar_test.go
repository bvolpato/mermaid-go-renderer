package mermaid

import "testing"

func TestParseRadarAxesAndCurves(t *testing.T) {
	input := `radar-beta
title Engineering Capability
axis quality["Quality"], velocity["Velocity"], reliability["Reliability"]
axis security["Security"], operability["Operability"], ux["UX"]
curve teamA["Team A"]{82, 76, 88, 70, 79, 68}
curve teamB["Team B"]{74, 84, 72, 81, 76, 73}
max 100
min 0
`
	out, err := ParseMermaid(input)
	if err != nil {
		t.Fatalf("ParseMermaid returned error: %v", err)
	}
	if out.Graph.Kind != DiagramRadar {
		t.Fatalf("expected radar kind, got %q", out.Graph.Kind)
	}
	if out.Graph.RadarTitle != "Engineering Capability" {
		t.Fatalf("unexpected radar title: %q", out.Graph.RadarTitle)
	}
	if len(out.Graph.RadarAxes) != 6 {
		t.Fatalf("expected 6 axes, got %d", len(out.Graph.RadarAxes))
	}
	if len(out.Graph.RadarCurves) != 2 {
		t.Fatalf("expected 2 curves, got %d", len(out.Graph.RadarCurves))
	}
	if len(out.Graph.RadarCurves[0].Entries) != 6 {
		t.Fatalf("expected 6 entries in first curve, got %d", len(out.Graph.RadarCurves[0].Entries))
	}
	if out.Graph.RadarCurves[0].Label != "Team A" {
		t.Fatalf("unexpected first curve label: %q", out.Graph.RadarCurves[0].Label)
	}
	if out.Graph.RadarMax == nil || *out.Graph.RadarMax != 100 {
		t.Fatalf("unexpected radar max: %+v", out.Graph.RadarMax)
	}
	if out.Graph.RadarMin == nil || *out.Graph.RadarMin != 0 {
		t.Fatalf("unexpected radar min: %+v", out.Graph.RadarMin)
	}
}
