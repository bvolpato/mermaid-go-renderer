package rendertest

import (
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/flowchart"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/swimlanes"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set1"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set2"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set3"
)

// swimlaneAlphabet has pieces that change the structure of a swimlane
// diagram: lanes, nested groups, edges, labels, cycles.
var swimlaneAlphabet = []string{
	"\n", " ", "-->", "---", "-->|x|", "-- y -->", "==>", "-.->", "&", "end\n", "subgraph ", "subgraph S\n", "A", "B", "L1",
	"A --> A\n", "direction LR\n", "{", "}", "[", "]", "((", "))", "@{ shape: diamond }", ":::", "%%", "é", "😀",
}

// mutateText changes a text at random places: it removes, repeats, moves
// and inserts pieces.
func mutateText(rnd *rand.Rand, text string) string {
	b := []byte(text)
	edits := 1 + rnd.Intn(4)
	for i := 0; i < edits; i++ {
		at := rnd.Intn(len(b) + 1)
		switch rnd.Intn(6) {
		case 0: // remove a piece
			n := min(len(b)-at, 1+rnd.Intn(12))
			b = append(b[:at:at], b[at+n:]...)
		case 1: // repeat a piece
			n := min(len(b)-at, 1+rnd.Intn(40))
			piece := append([]byte{}, b[at:at+n]...)
			b = append(b[:at:at], append(piece, b[at:]...)...)
		case 2, 3: // insert a piece of the alphabet
			piece := swimlaneAlphabet[rnd.Intn(len(swimlaneAlphabet))]
			b = append(b[:at:at], append([]byte(piece), b[at:]...)...)
		case 4: // swap two lines
			lines := strings.Split(string(b), "\n")
			from, to := rnd.Intn(len(lines)), rnd.Intn(len(lines))
			lines[from], lines[to] = lines[to], lines[from]
			b = []byte(strings.Join(lines, "\n"))
		case 5: // remove a line
			lines := strings.Split(string(b), "\n")
			if len(lines) > 3 {
				at := 1 + rnd.Intn(len(lines)-1)
				lines = append(lines[:at:at], lines[at+1:]...)
			}
			b = []byte(strings.Join(lines, "\n"))
		}
	}
	return strings.ToValidUTF8(string(b), "�")
}

// TestMutatedCorpusRender renders mutated swimlane fixtures and hostile
// texts with the swimlane layout. The requirement is: no panic and no
// hang. An error is a good result.
func TestMutatedCorpusRender(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "..", "testdata", "corpus", "mermaid-e2e", "swimlanes")
	var fixtures []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".mmd") {
			fixtures = append(fixtures, path)
		}
		return nil
	})
	sort.Strings(fixtures)
	if len(fixtures) == 0 {
		t.Fatal("no corpus fixtures")
	}
	mutantsPerFixture := 2
	if testing.Short() {
		mutantsPerFixture = 1
		fixtures = fixtures[:len(fixtures)/4]
	}
	rnd := rand.New(rand.NewSource(0x73776c72))
	var texts []string
	for _, f := range fixtures {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < mutantsPerFixture; i++ {
			texts = append(texts, mutateText(rnd, string(data)))
		}
	}
	// Texts that aim at the limits of the layout.
	texts = append(texts,
		"swimlane-beta",
		"swimlane-beta TB",
		"swimlane-beta LR\nA",
		"swimlane-beta TB\nsubgraph L\nA --> A\nend",
		"swimlane-beta TB\nsubgraph L\nA --> A\nA --> B\nB --> A\nend",
		"swimlane-beta LR\nsubgraph L1\nend\nsubgraph L2\nend",
		"swimlane-beta TB\nsubgraph A\nB\nend\nsubgraph B\nA\nend\nX --> Y",
		"swimlane-beta TB\nsubgraph A\nA\nend\nA --> A",
		"swimlane-beta TB\nsubgraph L1\nA\nend\nsubgraph L2\nB\nend\nA --> L2\nL1 --> B\nL1 --> L2",
		"swimlane-beta BT\n"+strings.Repeat("subgraph s\n", 40)+"a --> b\n"+strings.Repeat("end\n", 40),
		"swimlane-beta RL\nsubgraph L\n"+strings.Repeat("a --> b\n", 30)+"end",
		"swimlane-beta TB\nsubgraph L\n"+strings.Repeat("a -->|x| b\nb -->|y| a\n", 6)+"end",
		"---\nconfig:\n  flowchart:\n    nodeSpacing: -50\n    rankSpacing: 0\n---\nswimlane-beta TB\nsubgraph L\nA --> B --> C\nA --> C\nend",
		"---\nconfig:\n  flowchart:\n    nodeSpacing: x\n    rankSpacing: [1]\n  swimlane:\n    lineHops: 5\n    ignoreCrossLaneEdges: no\n---\nswimlane-beta LR\nsubgraph L\nA --> B --> C\nend\nsubgraph M\nD --> A\nend",
		"---\nconfig:\n  swimlane: 5\n---\nswimlane-beta LR\nsubgraph L\nA --> B\nend",
	)

	problems := make([]string, len(texts))
	var wg sync.WaitGroup
	slots := make(chan struct{}, runtime.GOMAXPROCS(0))
	for i, text := range texts {
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			done := make(chan string, 1)
			go func() {
				problem := ""
				defer func() {
					if r := recover(); r != nil {
						problem = "panic"
						if e, ok := r.(error); ok {
							problem = "panic: " + e.Error()
						} else if s, ok := r.(string); ok {
							problem = "panic: " + s
						}
					}
					done <- problem
				}()
				_, _ = render.Render(text, render.Options{})
			}()
			select {
			case problems[i] = <-done:
			case <-time.After(180 * time.Second):
				problems[i] = "no result after 180 seconds"
			}
		}()
	}
	wg.Wait()
	failures := 0
	for i, problem := range problems {
		if problem == "" {
			continue
		}
		failures++
		if failures <= 5 {
			t.Errorf("%s\n--- text %d ---\n%s", problem, i, texts[i])
		}
	}
	t.Logf("%d texts, %d panics or hangs", len(texts), failures)
}
