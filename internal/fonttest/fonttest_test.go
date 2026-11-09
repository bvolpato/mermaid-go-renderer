package fonttest

import "testing"

// TestSimulateEnv checks the switch of a simulated run: with
// MMDG_FONTTEST=skip the reference machine is a machine with other fonts.
func TestSimulateEnv(t *testing.T) {
	t.Setenv(SimulateEnv, "skip")
	if IsMacOSReference() {
		t.Fatalf("IsMacOSReference is true with %s=skip", SimulateEnv)
	}
}
