// These tests cover the reference manifest used by this project.
package parity

import (
	"os"
	"path/filepath"
	"testing"
)

// A partially refreshed corpus must not inherit the version of its first fixture.
func TestBuildManifestRejectsMixedReferences(t *testing.T) {
	const current = `{"mermaid":"12.2.0","mmdc":"12.2.0","page":"mermaid-cli","error":"parse error"}`
	corpus, ref := t.TempDir(), t.TempDir()
	write := func(dir, file, contents string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, file), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(corpus, "a.mmd", "invalid")
	write(corpus, "b.mmd", "invalid")
	write(ref, "b.json", current)
	for _, old := range []string{
		`{"mermaid":"12.2.0","mmdc":"12.0.0","page":"mermaid-cli","error":"parse error"}`,
		`{"mermaid":"12.2.0","error":"parse error"}`,
	} {
		write(ref, "a.json", old)
		if _, err := BuildManifest(corpus, ref); err == nil {
			t.Errorf("accepted a mixed reference directory with a.json = %s", old)
		}
	}
	write(ref, "a.json", current)
	if _, err := BuildManifest(corpus, ref); err != nil {
		t.Fatal(err)
	}
}
