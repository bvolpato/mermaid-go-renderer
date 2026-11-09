package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/perfdata"
)

// TestGeneratedTextIsCurrent checks that the blocks of the README and of
// docs/CLI.md have the numbers of the repository: the manifests with their
// lists, and docs/performance.json. When it fails, run tools/update-docs.sh.
func TestGeneratedTextIsCurrent(t *testing.T) {
	// The paths of the generator are relative to the root of the repository.
	root := filepath.Join("..", "..")
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	perf, err := perfdata.Load(filepath.Join("docs", "performance.json"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := generatedText(perf)
	if err != nil {
		t.Fatal(err)
	}
	for path, blocks := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want, err := replaceBlocks(string(data), blocks)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if want != string(data) {
			t.Errorf("%s does not have the numbers of the repository: run tools/update-docs.sh", path)
		}
	}
}
