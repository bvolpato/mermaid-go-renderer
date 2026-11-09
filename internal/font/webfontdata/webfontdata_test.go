package webfontdata

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/webfonts"
)

//go:embed manifest.json
var manifestJSON []byte

// TestFontsMatchManifest checks that each WOFF2 file of internal/webfonts
// has a decoded file here, and that both are the files that
// tools/reference/gen-webfonts-sfnt.py read and wrote. A WOFF2 file that
// changes without a new run of the script fails here.
func TestFontsMatchManifest(t *testing.T) {
	var manifest map[string]struct {
		Woff2 string `json:"woff2"`
		Sfnt  string `json:"sfnt"`
		Size  int    `json:"size"`
	}
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		t.Fatal(err)
	}
	seen := 0
	err := fs.WalkDir(webfonts.FS, "files", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".woff2") {
			return err
		}
		seen++
		want, ok := manifest[path]
		if !ok {
			t.Errorf("%s: not in the manifest", path)
			return nil
		}
		woff2, err := webfonts.FS.ReadFile(path)
		if err != nil {
			return err
		}
		if sum := sha256.Sum256(woff2); hex.EncodeToString(sum[:]) != want.Woff2 {
			t.Errorf("%s: the WOFF2 file is not the file that was decoded", path)
		}
		data, err := Font(path)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			return nil
		}
		if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != want.Sfnt || len(data) != want.Size {
			t.Errorf("%s: the decoded font is not the font of the manifest", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen != len(manifest) || seen == 0 {
		t.Errorf("%d WOFF2 files, %d manifest entries", seen, len(manifest))
	}
	if _, err := Font("files/missing.woff2"); err == nil {
		t.Error("Font(files/missing.woff2): no error")
	}
}
