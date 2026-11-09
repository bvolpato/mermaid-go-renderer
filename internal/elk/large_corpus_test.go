package elk_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	mermaid "github.com/bvolpato/mermaid-go-renderer"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk"
)

// TestRecordLargeCorpus renders the diagrams of testdata/corpus/mermaid-large
// with the Go renderer and writes each ELK graph that the renderer lays out,
// with the result of elkjs in goja, to testdata/mermaid-large.jsonl.gz.
// TestNativeAgainstMermaidLarge compares the Go port with these results.
//
// The graphs are the graphs of the Go glue, and the results are the results
// of the genuine elkjs, so no browser is needed. The test runs only with
// ELK_RECORD_LARGE=1 and MMDG_ELK=goja (or check), so that elkjs computes
// each layout. A layout of 150 nodes takes about 20 s of CPU time in goja.
func TestRecordLargeCorpus(t *testing.T) {
	if os.Getenv("ELK_RECORD_LARGE") == "" {
		t.Skip("set ELK_RECORD_LARGE=1 and MMDG_ELK=goja to record")
	}
	if m := os.Getenv("MMDG_ELK"); m != "goja" && m != "check" {
		t.Fatal("run with MMDG_ELK=goja or MMDG_ELK=check, so that elkjs computes each layout")
	}
	files, err := filepath.Glob("../../testdata/corpus/mermaid-large/*/*.mmd")
	if err != nil || len(files) == 0 {
		t.Fatalf("no diagrams of the large corpus: %v", err)
	}
	sort.Strings(files)

	type record struct {
		Name  string          `json:"name"`
		Nodes int             `json:"nodes"`
		In    json.RawMessage `json:"in"`
		Out   json.RawMessage `json:"out"`
	}
	var mu sync.Mutex
	seen := map[string]bool{}
	var records []record
	elk.ObserveElkjs(func(in, out []byte) {
		mu.Lock()
		defer mu.Unlock()
		if seen[string(in)] {
			return
		}
		seen[string(in)] = true
		records = append(records, record{In: append([]byte(nil), in...), Out: append([]byte(nil), out...)})
	})
	defer elk.ObserveElkjs(nil)

	var wg sync.WaitGroup
	sem := make(chan struct{}, 2)
	var failed []string
	for _, file := range files {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			source, err := os.ReadFile(file)
			if err == nil {
				_, err = mermaid.Render(string(source))
			}
			if err != nil {
				mu.Lock()
				failed = append(failed, file+": "+err.Error())
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	timeouts := 0
	for _, f := range failed {
		// The shared engine stops a layout after 30 s, which elkjs in goja
		// exceeds for the largest graphs on a busy machine. Such a graph is
		// not recorded. With MMDG_ELK=check, a native layout that differs
		// from elkjs is an error of the render: that fails the test.
		if strings.Contains(f, "deadline") || strings.Contains(f, "interrupted") {
			timeouts++
			continue
		}
		t.Errorf("%s", strings.SplitN(f, "\n", 2)[0])
	}
	if timeouts > 0 {
		t.Logf("%d diagrams have a layout that elkjs in goja did not finish in the time limit of the engine; their graphs are not recorded", timeouts)
	}

	sort.Slice(records, func(i, j int) bool { return string(records[i].In) < string(records[j].In) })
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	enc := json.NewEncoder(zw)
	enc.SetEscapeHTML(false)
	large := 0
	for i := range records {
		records[i].Name = "large-" + itoa(i)
		records[i].Nodes = countNodes(records[i].In)
		if records[i].Nodes >= 40 {
			large++
		}
		if err := enc.Encode(records[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if len(records) == 0 {
		t.Fatal("the renderer laid out no ELK graph")
	}
	if err := os.WriteFile("testdata/mermaid-large.jsonl.gz", buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d diagrams, %d ELK graphs recorded (%d with 40 nodes or more), %d bytes", len(files), len(records), large, buf.Len())
}

func itoa(i int) string {
	s := ""
	for {
		s = string(rune('0'+i%10)) + s
		i /= 10
		if i == 0 {
			return s
		}
	}
}

// countNodes counts the nodes of an ELK graph, without the root.
func countNodes(graph []byte) int {
	var node struct {
		Children []json.RawMessage `json:"children"`
	}
	if json.Unmarshal(graph, &node) != nil {
		return 0
	}
	n := 0
	for _, c := range node.Children {
		n += 1 + countNodes(c)
	}
	return n
}
