// This file ports no upstream file. It gives the embedded Liberation fonts
// 2.1.5 (OFL-1.1, the file LICENSE in this directory) to the font library.

// Package liberation has the Liberation fonts 2.1.5 as embedded data: Sans,
// Serif and Mono, each in four styles. They are metric-compatible with
// Arial, Times New Roman and Courier New, and Chromium on Linux uses them
// for those families.
//
// The files are the files of the Ubuntu package fonts-liberation 1:2.1.5-3,
// compressed with gzip. The license is the SIL Open Font License 1.1 (see
// the file LICENSE in this directory, and the variable License).
package liberation

import (
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"sync"
)

//go:embed *.ttf.gz
var files embed.FS

// License is the copyright notice and the license text of the fonts.
//
//go:embed LICENSE
var License string

// names lists the font files in the order in which a directory listing has
// them.
var names = [...]string{
	"LiberationMono-Bold.ttf",
	"LiberationMono-BoldItalic.ttf",
	"LiberationMono-Italic.ttf",
	"LiberationMono-Regular.ttf",
	"LiberationSans-Bold.ttf",
	"LiberationSans-BoldItalic.ttf",
	"LiberationSans-Italic.ttf",
	"LiberationSans-Regular.ttf",
	"LiberationSerif-Bold.ttf",
	"LiberationSerif-BoldItalic.ttf",
	"LiberationSerif-Italic.ttf",
	"LiberationSerif-Regular.ttf",
}

type entry struct {
	once sync.Once
	data []byte
	err  error
}

var entries [len(names)]entry

// Names returns the names of the font files, for example
// "LiberationSans-Regular.ttf".
func Names() []string {
	return append([]string(nil), names[:]...)
}

// Font returns the data of a font file. The file is decompressed on the
// first call. The caller must not change the result.
func Font(name string) ([]byte, error) {
	for i := range names {
		if names[i] != name {
			continue
		}
		e := &entries[i]
		e.once.Do(func() {
			e.data, e.err = inflate(name)
		})
		return e.data, e.err
	}
	return nil, fmt.Errorf("liberation: no font file %q", name)
}

func inflate(name string) ([]byte, error) {
	packed, err := files.ReadFile(name + ".gz")
	if err != nil {
		return nil, err
	}
	zr, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		return nil, fmt.Errorf("liberation: %s: %w", name, err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("liberation: %s: %w", name, err)
	}
	return data, nil
}
