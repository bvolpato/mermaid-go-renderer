package parity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
)

// A Manifest is a compact form of a reference directory: for every fixture
// of the corpus it has the hash of the SVG that mermaid produced in
// Chromium, or the fact that mermaid failed. With it a test proves
// byte-exact output without the reference files, which are large.
//
// tools/manifest writes a manifest from a reference directory that
// tools/reference/oracle.mjs produced.
type Manifest struct {
	// Mermaid is the version of mermaid that produced the reference.
	Mermaid string `json:"mermaid"`
	// Corpus is the directory of the fixtures under testdata/corpus.
	Corpus string `json:"corpus"`
	// Environment names where the reference comes from, for example
	// "chromium-156-macos".
	Environment string `json:"environment"`
	// Fonts is "system" when the reference depends on the fonts of the
	// machine, and "embedded" when it uses only the fonts in this module.
	Fonts string `json:"fonts"`
	// FontFingerprint identifies the system fonts of the reference
	// machine. It is empty for embedded fonts.
	FontFingerprint string `json:"fontFingerprint,omitempty"`
	// Page is the kind of page of the reference. It is empty for a plain
	// page with only mermaid, where the reference is the string that
	// mermaid.render returned (tools/reference/oracle.mjs). It is
	// "mermaid-cli" for the page of mermaid-cli, where the reference is
	// the .svg file that mermaid-cli writes
	// (tools/reference/mmdc-oracle.mjs).
	Page string `json:"page,omitempty"`
	// MermaidCLI is the version of mermaid-cli of such a reference.
	MermaidCLI string `json:"mermaidCLI,omitempty"`
	// Fixtures maps the path of a fixture, relative to the corpus and
	// without the extension, to its reference.
	Fixtures map[string]ManifestEntry `json:"fixtures"`
}

// ManifestEntry is the reference of one fixture.
type ManifestEntry struct {
	// Type is the diagram type id that mermaid detected.
	Type string `json:"type,omitempty"`
	// Error is true when mermaid failed to render the fixture.
	Error bool `json:"error,omitempty"`
	// SHA256 is the hash of the SVG text that mermaid.render returned. For
	// the page of mermaid-cli it is the hash of the .svg file of
	// mermaid-cli --no-font-embed.
	SHA256 string `json:"sha256,omitempty"`
	// Bytes is the length of that text.
	Bytes int `json:"bytes,omitempty"`
	// EmbedSHA256 is the hash of the .svg file of mermaid-cli with its
	// default options, which embed the fonts of the page that the diagram
	// uses. It is empty when that file is the file of SHA256, and for a
	// plain page.
	EmbedSHA256 string `json:"embedSha256,omitempty"`
}

// PageMermaidCLI is the value of Manifest.Page for the page of mermaid-cli.
const PageMermaidCLI = "mermaid-cli"

// HashSVG returns the hash that a manifest stores for an SVG text.
func HashSVG(svg string) string {
	sum := sha256.Sum256([]byte(svg))
	return hex.EncodeToString(sum[:])
}

// BuildManifest reads a reference directory and returns its manifest.
func BuildManifest(corpus, ref string) (*Manifest, error) {
	m := &Manifest{Fixtures: map[string]ManifestEntry{}}
	err := filepath.WalkDir(corpus, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".mmd") {
			return err
		}
		rel, _ := filepath.Rel(corpus, path)
		base := strings.TrimSuffix(filepath.ToSlash(rel), ".mmd")
		data, err := os.ReadFile(filepath.Join(ref, filepath.FromSlash(base)+".json"))
		if err != nil {
			return fmt.Errorf("%s has no reference: %w", base, err)
		}
		var meta struct {
			Mermaid     string `json:"mermaid"`
			DiagramType string `json:"diagramType"`
			Error       string `json:"error"`
			// The fields of mmdc-oracle.mjs.
			Page        string `json:"page"`
			MermaidCLI  string `json:"mmdc"`
			EmbedSHA256 string `json:"embedSha256"`
		}
		if err := json.Unmarshal(data, &meta); err != nil {
			return fmt.Errorf("%s: %w", base, err)
		}
		if len(m.Fixtures) == 0 {
			m.Mermaid, m.Page, m.MermaidCLI = meta.Mermaid, meta.Page, meta.MermaidCLI
		} else if m.Mermaid != meta.Mermaid || m.Page != meta.Page || m.MermaidCLI != meta.MermaidCLI {
			return fmt.Errorf("%s: mixed references: Mermaid %q, page %q, CLI %q; expected %q, %q, %q",
				base, meta.Mermaid, meta.Page, meta.MermaidCLI, m.Mermaid, m.Page, m.MermaidCLI)
		}
		entry := ManifestEntry{Type: meta.DiagramType}
		if meta.Error != "" {
			entry.Error = true
		} else {
			svg, err := os.ReadFile(filepath.Join(ref, filepath.FromSlash(base)+".svg"))
			if err != nil {
				return fmt.Errorf("%s: %w", base, err)
			}
			entry.SHA256 = HashSVG(string(svg))
			entry.Bytes = len(svg)
			if meta.EmbedSHA256 != entry.SHA256 {
				entry.EmbedSHA256 = meta.EmbedSHA256
			}
		}
		m.Fixtures[base] = entry
		return nil
	})
	if err != nil {
		return nil, err
	}
	return m, nil
}

// LoadManifest reads a manifest file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m := &Manifest{}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// Save writes the manifest as JSON with sorted keys.
func (m *Manifest) Save(path string) error {
	data, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// Names returns the fixture names in order.
func (m *Manifest) Names() []string {
	names := make([]string, 0, len(m.Fixtures))
	for name := range m.Fixtures {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SystemFontFingerprint returns a value that identifies the system fonts
// that the corpus uses on this machine. It is empty on a system where those
// fonts do not exist, for example on Linux.
func SystemFontFingerprint() string {
	return fonttest.SystemFontFingerprint()
}
