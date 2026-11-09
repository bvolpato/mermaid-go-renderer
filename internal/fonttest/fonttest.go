// Package fonttest tells tests whether this machine has the system fonts
// of the machine that recorded their golden files.
//
// Most golden files of this module come from mermaid in Chromium on one
// macOS machine, with the system fonts of that machine: text widths in them
// depend on those font files. A test that measures text with the system
// fonts and compares with such a golden file can pass only where the same
// fonts are installed. On every other machine it must skip. The proof for
// every machine is TestCorpusParity in the root package, which measures
// with the fonts that are embedded in the module.
package fonttest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
)

// MacOSReferenceFingerprint is the fingerprint of the system fonts of the
// macOS machine that recorded the golden files (macOS 26).
const MacOSReferenceFingerprint = "2f77e7cbfdf534aa"

// macFontFiles are the fonts that the diagrams of the corpus use on macOS:
// the families of the mermaid themes and the defaults of Chromium.
var macFontFiles = []string{
	"/System/Library/Fonts/Helvetica.ttc",
	"/System/Library/Fonts/Times.ttc",
	"/System/Library/Fonts/Courier.ttc",
	"/System/Library/Fonts/Menlo.ttc",
	"/System/Library/Fonts/Supplemental/Arial.ttf",
	"/System/Library/Fonts/Supplemental/Arial Bold.ttf",
	"/System/Library/Fonts/Supplemental/Arial Italic.ttf",
	"/System/Library/Fonts/Supplemental/Arial Bold Italic.ttf",
	"/System/Library/Fonts/Supplemental/Trebuchet MS.ttf",
	"/System/Library/Fonts/Supplemental/Trebuchet MS Bold.ttf",
	"/System/Library/Fonts/Supplemental/Trebuchet MS Italic.ttf",
	"/System/Library/Fonts/Supplemental/Trebuchet MS Bold Italic.ttf",
	"/System/Library/Fonts/Supplemental/Verdana.ttf",
	"/System/Library/Fonts/Supplemental/Verdana Bold.ttf",
	"/System/Library/Fonts/Supplemental/Courier New.ttf",
	"/System/Library/Fonts/Supplemental/Courier New Bold.ttf",
	"/System/Library/Fonts/Supplemental/Times New Roman.ttf",
	"/System/Library/Fonts/Supplemental/Times New Roman Bold.ttf",
}

var (
	fingerprintOnce sync.Once
	fingerprint     string
)

// SystemFontFingerprint returns a value that identifies the system fonts
// that the corpus uses on this machine. It is empty on a system where those
// fonts do not exist, for example on Linux.
func SystemFontFingerprint() string {
	fingerprintOnce.Do(func() {
		h := sha256.New()
		found := 0
		for _, path := range macFontFiles {
			f, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(h, "%s missing\n", path)
				continue
			}
			found++
			fmt.Fprintf(h, "%s\n", path)
			_, _ = io.Copy(h, f)
			_ = f.Close()
		}
		if found > 0 {
			fingerprint = hex.EncodeToString(h.Sum(nil))[:16]
		}
	})
	return fingerprint
}

// SimulateEnv is the name of an environment variable for a test run on the
// reference machine. With the value "skip", IsMacOSReference reports false,
// so the run shows which tests a machine with other fonts skips:
//
//	MMDG_FONTTEST=skip go test ./...
//
// The tests of such a run still measure with the fonts of this machine. A
// comparison that has no guard passes in it. Only a run on a machine with
// other fonts finds such a comparison.
const SimulateEnv = "MMDG_FONTTEST"

// IsMacOSReference reports whether this machine has the system fonts of
// the macOS machine that recorded the golden files.
func IsMacOSReference() bool {
	if os.Getenv(SimulateEnv) == "skip" {
		return false
	}
	return SystemFontFingerprint() == MacOSReferenceFingerprint
}

// RequireMacOSReference skips the test unless this machine has the system
// fonts of the macOS machine that recorded the golden files. Call it first
// in a test that measures text with the system fonts and compares the
// result with a golden file from Chromium on macOS.
//
// A test can also have checks that do not need those fonts: an error of
// mermaid, a dump of a db, an input that must not panic. Such a test keeps
// them on every machine. It makes each comparison that needs the fonts
// conditional on IsMacOSReference, and it calls RequireMacOSReference as its
// last statement. On a machine with other fonts the test then runs the
// other checks and reports that it skipped. A check that failed before the
// call still fails the test.
func RequireMacOSReference(t testing.TB) {
	t.Helper()
	if IsMacOSReference() {
		return
	}
	if os.Getenv(SimulateEnv) == "skip" {
		t.Skipf("the golden file needs the system fonts of the macOS reference machine (%s=skip)", SimulateEnv)
	}
	t.Skipf("the golden file needs the system fonts of the macOS reference machine (fingerprint %s, here %q)", MacOSReferenceFingerprint, SystemFontFingerprint())
}
