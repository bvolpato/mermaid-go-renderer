package nativetest

import (
	"testing"
)

// TestStateRoundTrip builds the LGraph of each golden state and dumps it
// again. The result must be the stored state: then Build and Dump agree with
// the dump of elkjs, including the order in which the walk finds the
// elements.
func TestStateRoundTrip(t *testing.T) {
	files, err := Files("")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 50 {
		t.Fatalf("only %d golden files", len(files))
	}
	if testing.Short() {
		files = files[:40]
	}
	states, failed := 0, 0
	for _, file := range files {
		c, err := LoadCase(file)
		if err != nil {
			t.Fatal(err)
		}
		for ri, run := range c.Runs {
			for si := range run.States {
				states++
				want, err := run.State(si)
				if err != nil {
					t.Fatal(err)
				}
				built, err := Build(want)
				var got map[string]any
				if err == nil {
					got, err = Dump(built.Root())
				}
				if err != nil {
					failed++
					if failed <= 5 {
						t.Errorf("%s run %d state %d: %v", file, ri, si, err)
					}
					continue
				}
				if d := Diff(got, want, 8); len(d) > 0 {
					failed++
					if failed <= 5 {
						t.Errorf("%s run %d state %d:\n  %v", file, ri, si, d)
					}
				}
			}
		}
	}
	t.Logf("%d states in %d graphs, %d do not survive the round trip", states, len(files), failed)
}
