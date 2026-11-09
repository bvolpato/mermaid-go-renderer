// Command manifest writes a reference manifest: the hash of the SVG that
// mermaid produced for every fixture of the corpus.
//
// Usage:
//
//	go run ./tools/manifest -ref .parity/ref/mermaid-e2e -environment chromium-156-macos -fonts system -o testdata/reference/macos.json
//
// The reference directory comes from tools/reference/oracle.mjs.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
)

func main() {
	corpus := flag.String("corpus", "testdata/corpus/mermaid-e2e", "directory of .mmd fixtures")
	ref := flag.String("ref", ".parity/ref/mermaid-e2e", "directory of reference .svg and .json files")
	environment := flag.String("environment", "", "name of the reference environment, for example chromium-156-macos")
	fonts := flag.String("fonts", "system", "fonts of the reference: system or embedded")
	out := flag.String("o", "", "manifest file to write")
	flag.Parse()
	if *out == "" || *environment == "" {
		flag.Usage()
		os.Exit(2)
	}
	m, err := parity.BuildManifest(*corpus, *ref)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	m.Environment = *environment
	m.Corpus = filepath.Base(filepath.Clean(*corpus))
	m.Fonts = *fonts
	if *fonts == "system" {
		m.FontFingerprint = parity.SystemFontFingerprint()
	}
	if err := m.Save(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	failed := 0
	for _, e := range m.Fixtures {
		if e.Error {
			failed++
		}
	}
	fmt.Printf("%s: %d fixtures, %d that mermaid does not render\n", *out, len(m.Fixtures), failed)
}
