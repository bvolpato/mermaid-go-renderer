package gantt

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// seedTexts returns the gantt fixtures of the corpus and the extra cases.
func seedTexts(t *testing.T) []string {
	t.Helper()
	var texts []string
	for _, pattern := range []string{
		filepath.Join("..", "..", "..", "testdata", "corpus", "mermaid-e2e", "gantt", "*", "*.mmd"),
		filepath.Join("testdata", "cases", "*.mmd"),
	} {
		files, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			texts = append(texts, string(data))
		}
	}
	if len(texts) < 100 {
		t.Fatalf("only %d seed texts", len(texts))
	}
	return texts
}

var fuzzSnippets = []string{
	"after ", "until ", "crit, ", "milestone, ", "vert, ", "done, ", "active, ", ":", ",", ", ", "\n", ";", "#",
	"excludes weekends\n", "excludes monday tuesday wednesday thursday friday saturday sunday\n", "includes ", "dateFormat x\n",
	"dateFormat YYYY-MM-DD\n", "dateFormat \n", "dateFormat Y\n", "dateFormat Do MMM w z\n", "axisFormat %\n", "axisFormat %Q %s %f\n",
	"tickInterval 1day\n", "tickInterval 1millisecond\n", "tickInterval 7week\n", "todayMarker off\n", "section ", "title ",
	"click ", "href \"", "call ", "weekend friday\n", "weekday monday\n", "topAxis\n", "inclusiveEndDates\n",
	"999999999", "99999999999999999999999", "0", "-1", "d", "ms", "1y", "0.5M", "5000y", "300000d", "accDescr {", "}", "%%", "<br>",
	"__proto__", "constructor", "displayMode: compact\n", "gantt", "\t", " ", "\"", "2024-02-30", "0000-00-00", "275760-09-13",
	"\u00a0", "\u2028", "é", "🚀", "\x00",
}

// mutate changes a text in one to four places.
func mutate(rnd *rand.Rand, text string) string {
	for n := 1 + rnd.Intn(4); n > 0; n-- {
		lines := strings.Split(text, "\n")
		switch rnd.Intn(8) {
		case 0: // delete a range
			if len(text) > 0 {
				i := rnd.Intn(len(text))
				j := min(len(text), i+1+rnd.Intn(12))
				text = text[:i] + text[j:]
			}
		case 1, 2: // insert a snippet
			i := rnd.Intn(len(text) + 1)
			text = text[:i] + fuzzSnippets[rnd.Intn(len(fuzzSnippets))] + text[i:]
		case 3: // replace a byte
			if len(text) > 0 {
				i := rnd.Intn(len(text))
				text = text[:i] + string(rune(32+rnd.Intn(95))) + text[i+1:]
			}
		case 4: // duplicate a line
			i := rnd.Intn(len(lines))
			lines = append(lines[:i+1], lines[i:]...)
			text = strings.Join(lines, "\n")
		case 5: // delete a line
			i := rnd.Intn(len(lines))
			text = strings.Join(append(lines[:i:i], lines[i+1:]...), "\n")
		case 6: // swap two lines
			i, j := rnd.Intn(len(lines)), rnd.Intn(len(lines))
			lines[i], lines[j] = lines[j], lines[i]
			text = strings.Join(lines, "\n")
		case 7: // change a digit
			if i := strings.IndexAny(text, "0123456789"); i >= 0 {
				k := i + rnd.Intn(len(text)-i)
				if j := strings.IndexAny(text[k:], "0123456789"); j >= 0 {
					text = text[:k+j] + string(rune('0'+rnd.Intn(10))) + text[k+j+1:]
				}
			}
		}
	}
	return text
}

// runMutations calls fn for mutated inputs. It fails the test when a call
// panics or when one call takes more than the limit.
func runMutations(t *testing.T, count int, limit time.Duration, fn func(text string)) {
	t.Helper()
	seeds := seedTexts(t)
	rnd := rand.New(rand.NewSource(20260115))
	var current atomic.Pointer[string]
	var started atomic.Int64
	failure := make(chan string, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < count; i++ {
			text := mutate(rnd, seeds[rnd.Intn(len(seeds))])
			current.Store(&text)
			started.Store(time.Now().UnixNano())
			func() {
				defer func() {
					if r := recover(); r != nil {
						select {
						case failure <- fmt.Sprintf("panic: %v\ninput:\n%s", r, text):
						default:
						}
					}
				}()
				fn(text)
			}()
		}
	}()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case msg := <-failure:
			t.Fatal(msg)
		case <-done:
			select {
			case msg := <-failure:
				t.Fatal(msg)
			default:
			}
			return
		case <-ticker.C:
			if s := started.Load(); s != 0 && time.Since(time.Unix(0, s)) > limit {
				t.Fatalf("one input takes more than %v:\n%s", limit, *current.Load())
			}
		}
	}
}

// TestMutatedInputsParse feeds mutated inputs to the parser and the db. It
// requires that no input panics or hangs.
func TestMutatedInputsParse(t *testing.T) {
	count := 1500
	if testing.Short() {
		count = 800
	}
	runMutations(t, count, 60*time.Second, func(text string) {
		db, err := parseLikeMermaid(text)
		if err != nil {
			return
		}
		_, _ = db.GetTasks()
	})
}

// TestMutatedInputsRender feeds mutated inputs to the whole render. It
// requires that no input panics or hangs.
func TestMutatedInputsRender(t *testing.T) {
	count := 200
	if testing.Short() {
		count = 80
	}
	runMutations(t, count, 60*time.Second, func(text string) {
		_, _ = render.Render(text, render.Options{})
	})
}
