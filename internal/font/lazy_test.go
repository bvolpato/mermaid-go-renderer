package font

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/font/liberation"
)

// TestEmbeddedFiles checks the table of the embedded font files against the
// files: the names in the order of liberation.Names, and for each file the
// family and the style that the parsed font reports. A lookup chooses a
// face from the table before it parses a font.
func TestEmbeddedFiles(t *testing.T) {
	names := liberation.Names()
	if len(names) != len(embeddedFiles) {
		t.Fatalf("the table has %d files, the package has %d", len(embeddedFiles), len(names))
	}
	for i, f := range embeddedFiles {
		if names[i] != f.name {
			t.Errorf("file %d is %s, the package has %s", i, f.name, names[i])
			continue
		}
		data, err := liberation.Font(f.name)
		if err != nil {
			t.Fatal(err)
		}
		faces, err := LoadFor(PlatformLinux, data)
		if err != nil {
			t.Fatal(err)
		}
		if len(faces) != 1 {
			t.Fatalf("%s has %d faces", f.name, len(faces))
		}
		got := faces[0]
		if got.Family != f.family || got.Weight != f.weight || got.Italic != f.italic || got.Index != 0 {
			t.Errorf("%s: the font is %q %d italic=%v index=%d, the table has %q %d italic=%v",
				f.name, got.Family, got.Weight, got.Italic, got.Index, f.family, f.weight, f.italic)
		}
	}
}

// mermaidFamilies is the default font-family list of mermaid.
var mermaidFamilies = []string{`"trebuchet ms"`, "verdana", "arial", "sans-serif"}

// TestEmbeddedFacesAreLazy checks that the embedded library parses one font
// file for a text in one style, that it parses a font one time, and that a
// family that is not installed parses nothing.
func TestEmbeddedFacesAreLazy(t *testing.T) {
	lib := newLibrary(PlatformLinux, embedFonts, nil)
	regular := lib.Resolve(mermaidFamilies, 400, false).Primary()
	if regular == nil || regular.Path != embeddedPrefix+"LiberationSans-Regular.ttf" {
		t.Fatalf("the regular face is %+v", regular)
	}
	if n := lib.ParsedFonts(); n != 1 {
		t.Errorf("a regular text parsed %d fonts, want 1", n)
	}
	// The same lookups from many goroutines, with and without the cache of
	// chains: no font is parsed again.
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if f := lib.Resolve(mermaidFamilies, 400, false).Primary(); f != regular {
				t.Errorf("a later lookup gave another face: %+v", f)
			}
			if f := lib.Match("arial", 400, false); f != regular {
				t.Errorf("Match gave another face: %+v", f)
			}
			if f := lib.Match("Recursive Variable", 400, false); f != nil {
				t.Errorf("a family that is not installed gave %+v", f)
			}
		}()
	}
	wg.Wait()
	if n := lib.ParsedFonts(); n != 1 {
		t.Errorf("repeated lookups parsed %d fonts, want 1", n)
	}
	bold := lib.Resolve(mermaidFamilies, 700, false).Primary()
	if bold == nil || bold.Path != embeddedPrefix+"LiberationSans-Bold.ttf" {
		t.Fatalf("the bold face is %+v", bold)
	}
	if n := lib.ParsedFonts(); n != 2 {
		t.Errorf("a regular and a bold text parsed %d fonts, want 2", n)
	}
}

// TestDirectoryFacesAreLazy checks the same for a library that reads font
// files: a lookup reads the descriptions of the files of a family and
// parses only the face that it gives.
func TestDirectoryFacesAreLazy(t *testing.T) {
	dir := t.TempDir()
	// The scan of a family that is not installed writes its index to the
	// cache directory of the user.
	cache := t.TempDir()
	t.Setenv("HOME", cache)
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("LOCALAPPDATA", cache)
	for _, f := range embeddedFiles {
		data, err := liberation.Font(f.name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f.name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, platform := range []Platform{PlatformDarwin, PlatformLinux} {
		t.Run(platform.String(), func(t *testing.T) {
			lib := NewLibraryFor(platform, dir)
			regular := lib.Match("Liberation Sans", 400, false)
			if regular == nil || filepath.Base(regular.Path) != "LiberationSans-Regular.ttf" {
				t.Fatalf("the regular face is %+v", regular)
			}
			if n := lib.ParsedFonts(); n != 1 {
				t.Errorf("one face parsed %d fonts, want 1", n)
			}
			for i := 0; i < 3; i++ {
				if f := lib.Match("Liberation Sans", 400, false); f != regular {
					t.Errorf("a later lookup gave another face: %+v", f)
				}
				if f := lib.Match("Recursive Variable", 400, false); f != nil {
					t.Errorf("a family that is not installed gave %+v", f)
				}
			}
			if n := lib.ParsedFonts(); n != 1 {
				t.Errorf("repeated lookups parsed %d fonts, want 1", n)
			}
			italic := lib.Match("Liberation Sans", 700, true)
			if italic == nil || filepath.Base(italic.Path) != "LiberationSans-BoldItalic.ttf" {
				t.Fatalf("the bold italic face is %+v", italic)
			}
			if n := lib.ParsedFonts(); n != 2 {
				t.Errorf("two faces parsed %d fonts, want 2", n)
			}
			// The description of a scan is the description of the parsed
			// font.
			for _, face := range []*Face{regular, italic} {
				if face.Family != "Liberation Sans" {
					t.Errorf("%s: family %q", face.Path, face.Family)
				}
			}
		})
	}
}
