package cytoscape

import (
	"context"
	"errors"
	"math"
	"testing"
)

// smallRecord returns the first record of a layout with at least min and at
// most max elements.
func smallRecord(t *testing.T, layout string, min, max int) *goldenRecord {
	t.Helper()
	for _, rec := range loadGolden(t).Records {
		if rec.Layout == layout && len(rec.Adds) >= min && len(rec.Adds) <= max {
			return rec
		}
	}
	t.Fatalf("no %s record with %d to %d elements", layout, min, max)
	return nil
}

// TestModes runs one layout of each kind in the three modes. The Go port,
// the interpreter and the check of the two give the same bits, and the
// counters tell which one computed the layout.
func TestModes(t *testing.T) {
	mindmap := smallRecord(t, "cose-bilkent", 5, 12)
	arch := smallRecord(t, "fcose", 5, 12)
	run := func(mode Mode) (*CoseBilkentResult, *ArchitectureResult) {
		t.Helper()
		env := newPageEnv(t, mindmap).env()
		env.Mode = mode
		nodes, edges := coseBilkentInput(t, mindmap)
		m, err := CoseBilkent(context.Background(), env, nodes, edges)
		if err != nil {
			t.Fatalf("%s, cose-bilkent: %v", mode, err)
		}
		env = newPageEnv(t, arch).env()
		env.Mode = mode
		a, err := Architecture(context.Background(), env, architectureInput(t, arch))
		if err != nil {
			t.Fatalf("%s, fcose: %v", mode, err)
		}
		return m, a
	}
	before := ReadStats()
	nativeM, nativeA := run(ModeNative)
	afterNative := ReadStats()
	if afterNative.Native-before.Native != 2 || afterNative.Interpreter != before.Interpreter {
		t.Errorf("native mode: %d native layouts and %d runs of the JavaScript, want 2 and 0", afterNative.Native-before.Native, afterNative.Interpreter-before.Interpreter)
	}
	gojaM, gojaA := run(ModeGoja)
	afterGoja := ReadStats()
	if afterGoja.Native != afterNative.Native || afterGoja.Interpreter-afterNative.Interpreter != 2 {
		t.Errorf("goja mode: %d native layouts and %d runs of the JavaScript, want 0 and 2", afterGoja.Native-afterNative.Native, afterGoja.Interpreter-afterNative.Interpreter)
	}
	checkM, checkA := run(ModeCheck)
	for _, other := range []*CoseBilkentResult{gojaM, checkM} {
		if d := diffCoseBilkent(nativeM, other); d != "" {
			t.Errorf("cose-bilkent: %s", d)
		}
	}
	for _, other := range []*ArchitectureResult{gojaA, checkA} {
		if d := diffArchitecture(nativeA, other); d != "" {
			t.Errorf("fcose: %s", d)
		}
	}

	// The comparison of the check mode sees one bit.
	changed := *nativeM
	changed.Nodes = append([]PositionedNode(nil), nativeM.Nodes...)
	changed.Nodes[0].X = math.Nextafter(changed.Nodes[0].X, math.Inf(1))
	if diffCoseBilkent(&changed, gojaM) == "" {
		t.Error("a result that differs in the last bit is no difference")
	}
	var checkErr *CheckError
	if err := error(&CheckError{Layout: "fcose", Difference: "x"}); !errors.As(err, &checkErr) || checkErr.Error() == "" {
		t.Error("CheckError is not an error with a text")
	}
}

// TestPageLog checks the record of what a layout reads from the page: a
// second run reads the same random numbers and then goes on with the
// generator, so a layout that the Go port gives up leaves the page as one
// run of the JavaScript leaves it.
func TestPageLog(t *testing.T) {
	next := 0.0
	env := Env{
		Random:      func() float64 { next++; return next },
		MeasureText: func(font, text string) float64 { return float64(len(font) + len(text)) },
	}
	log := &pageLog{env: env}
	first := log.recording()
	for i := 1; i <= 3; i++ {
		if got := first.Random(); got != float64(i) {
			t.Fatalf("draw %d of the first run: %v", i, got)
		}
	}
	first.MeasureText("f", "abc")
	used := 0
	second := log.replaying(&used)
	for i := 1; i <= 5; i++ {
		if got := second.Random(); got != float64(i) {
			t.Fatalf("draw %d of the second run: %v", i, got)
		}
	}
	if used != 5 || next != 5 {
		t.Errorf("the second run took %d draws, and the generator gave %v", used, next)
	}
	if w := second.MeasureText("f", "abc"); w != 4 {
		t.Errorf("the width of the second run: %v", w)
	}
	// An environment without a generator stays without one.
	empty := (&pageLog{}).recording()
	if empty.Random != nil || empty.MeasureText != nil {
		t.Error("the record of an empty environment has functions")
	}
}
