// This file ports no upstream file. It gives the web fonts of the page of
// mermaid-cli to the font library as sfnt data.

// Package webfontdata has the web fonts of internal/webfonts as sfnt files.
//
// A browser decodes a WOFF2 font before it reads the font. This module has
// no Brotli decoder, so tools/reference/gen-webfonts-sfnt.py does that step
// one time: each file here is the sfnt data of the WOFF2 file with the same
// path in internal/webfonts, compressed with gzip. manifest.json has the
// SHA-256 of each WOFF2 file and of its sfnt data. The test of this package
// compares the manifest with both sets of files.
//
// The fonts have the licenses of internal/webfonts: the SIL Open Font
// License 1.1 (see the license files in internal/webfonts/files).
package webfontdata

import (
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"strings"
	"sync"
)

//go:embed files
var files embed.FS

type entry struct {
	once sync.Once
	data []byte
	err  error
}

var (
	mu      sync.Mutex
	entries = map[string]*entry{}
)

// Font returns the sfnt data of a web font. path is the path of the WOFF2
// file in webfonts.FS, for example
// "files/recursive/files/recursive-latin-wght-normal.woff2". The file is
// decompressed on the first call, one time for the process. The caller must
// not change the result.
func Font(path string) ([]byte, error) {
	mu.Lock()
	e := entries[path]
	if e == nil {
		e = &entry{}
		entries[path] = e
	}
	mu.Unlock()
	e.once.Do(func() {
		e.data, e.err = inflate(path)
	})
	return e.data, e.err
}

func inflate(path string) ([]byte, error) {
	name, ok := strings.CutSuffix(path, ".woff2")
	if !ok {
		return nil, fmt.Errorf("webfontdata: %s is not a WOFF2 file", path)
	}
	packed, err := files.ReadFile(name + ".sfnt.gz")
	if err != nil {
		return nil, fmt.Errorf("webfontdata: no decoded font for %s", path)
	}
	zr, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		return nil, fmt.Errorf("webfontdata: %s: %w", path, err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("webfontdata: %s: %w", path, err)
	}
	return data, nil
}
