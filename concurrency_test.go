package mermaid

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestConcurrentRendersAreIdentical renders a sample of the corpus one at a
// time, then renders the same sources from many goroutines at once. Every
// concurrent result must equal the serial result. With -race the test also
// finds shared mutable state.
func TestConcurrentRendersAreIdentical(t *testing.T) {
	root := filepath.Join("testdata", "corpus", "mermaid-e2e")
	perFamily := 3
	if testing.Short() {
		perFamily = 1
	}
	type sample struct {
		name, src, want string
	}
	var samples []sample
	taken := map[string]int{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".mmd") {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		family := strings.SplitN(rel, string(filepath.Separator), 2)[0]
		if taken[family] >= perFamily {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		svg, err := Render(string(src))
		if err != nil {
			// A type that is not ported, or a fixture that mermaid rejects.
			return nil
		}
		taken[family]++
		samples = append(samples, sample{rel, string(src), svg})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) == 0 {
		t.Fatal("no fixture rendered")
	}

	const rounds = 4
	var wg sync.WaitGroup
	errs := make(chan string, len(samples)*rounds)
	for round := 0; round < rounds; round++ {
		for _, s := range samples {
			wg.Add(1)
			go func() {
				defer wg.Done()
				got, err := Render(s.src)
				if err != nil {
					errs <- s.name + ": " + err.Error()
				} else if got != s.want {
					errs <- s.name + ": the concurrent result differs from the serial result"
				}
			}()
		}
	}
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Error(msg)
	}
	t.Logf("%d fixtures of %d families, %d concurrent renders", len(samples), len(taken), len(samples)*rounds)
}
