package mermaid

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// TestTestDataIsNotInTheModule checks that every testdata directory has a
// go.mod file. The go command leaves such a directory out of the module, so
// a program that depends on this module does not download the test data,
// which is most of the repository.
func TestTestDataIsNotInTheModule(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		switch d.Name() {
		case ".git", ".parity", ".upstream", "node_modules":
			return filepath.SkipDir
		case "testdata":
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err != nil {
				t.Errorf("%s has no go.mod file: copy one from another testdata directory and change the module path", path)
			}
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
