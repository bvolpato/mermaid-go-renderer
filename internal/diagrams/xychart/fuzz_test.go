package xychart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// mulberry32 is the generator of the reference scripts.
type mulberry32 uint32

func (m *mulberry32) next() float64 {
	*m += 0x6d2b79f5
	t := uint32(*m)
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return float64(t^(t>>14)) / 4294967296
}

func (m *mulberry32) intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(m.next() * float64(n))
}

var fuzzSnippets = []string{
	"xychart", "xychart-beta horizontal", "title", "x-axis", "y-axis", "line", "bar", "-->", "[", "]", ",", "\"",
	"`", "accTitle:", "accDescr {", "}", "%%", ";", "\n", "  ", "1e999", "-", ".", "+.5", "99999999999999999999999",
	"0 --> 0", "[a, b", "\"unterminated", "<script>", "&amp;", "#35;", "\x00", "é", "日本", "😀", "\r\n", "\t",
	"%%{init: {\"xyChart\": {\"width\": 1e9, \"height\": -5, \"showDataLabel\": true}}}%%\n",
	"%%{init: {\"xyChart\": {\"xAxis\": null, \"yAxis\": 7}}}%%\n",
	"%%{init: {\"xyChart\": {\"xAxis\": {\"labelRotation\": 45, \"labelFontSize\": \"big\"}}}}%%\n",
	"%%{init: {\"xyChart\": {\"chartOrientation\": \"horizontal\", \"showDataLabel\": true, \"plotReservedSpacePercent\": 1e12}}}%%\n",
	"%%{init: {\"themeVariables\": {\"xyChart\": {\"plotColorPalette\": 5}}}}%%\n",
}

// mutate changes a diagram text in a few random places.
func mutate(rnd *mulberry32, text string) string {
	b := []byte(text)
	for edits := 1 + rnd.intn(4); edits > 0; edits-- {
		at := rnd.intn(len(b) + 1)
		switch rnd.intn(6) {
		case 0: // remove a range
			end := min(len(b), at+1+rnd.intn(12))
			b = append(b[:at:at], b[end:]...)
		case 1: // insert a snippet
			s := fuzzSnippets[rnd.intn(len(fuzzSnippets))]
			b = append(b[:at:at], append([]byte(s), b[at:]...)...)
		case 2: // repeat a range
			end := min(len(b), at+1+rnd.intn(40))
			b = append(b[:end:end], append(append([]byte{}, b[at:end]...), b[end:]...)...)
		case 3: // change a byte
			if at < len(b) {
				b[at] = byte(rnd.intn(256))
			}
		case 4: // cut the end
			b = b[:at]
		case 5: // swap two lines
			lines := strings.Split(string(b), "\n")
			i, j := rnd.intn(len(lines)), rnd.intn(len(lines))
			lines[i], lines[j] = lines[j], lines[i]
			b = []byte(strings.Join(lines, "\n"))
		}
	}
	return string(b)
}

// TestMutatedCorpus feeds mutated corpus fixtures to the renderer. The
// renderer must return, with a result or an error: no panic and no hang.
func TestMutatedCorpus(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "corpus", "mermaid-e2e", "xychart")
	var fixtures []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".mmd") {
			fixtures = append(fixtures, path)
		}
		return nil
	})
	if len(fixtures) == 0 {
		t.Fatal("no fixtures")
	}
	mutants := 8
	if testing.Short() {
		mutants = 2
	}
	rnd := mulberry32(0x78796675)
	rendered, failed := 0, 0
	for _, fixture := range fixtures {
		data, err := os.ReadFile(fixture)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < mutants; i++ {
			text := mutate(&rnd, string(data))
			type outcome struct {
				err      error
				panicked any
			}
			done := make(chan outcome, 1)
			go func() {
				var o outcome
				defer func() {
					o.panicked = recover()
					done <- o
				}()
				_, o.err = render.Render(text, render.Options{})
			}()
			select {
			case o := <-done:
				if o.panicked != nil {
					t.Errorf("%s mutant %d: panic: %v\ninput:\n%s", fixture, i, o.panicked, text)
				}
				if o.err != nil {
					failed++
				} else {
					rendered++
				}
			case <-time.After(60 * time.Second):
				t.Fatalf("%s mutant %d: the render did not return in 60 s\ninput:\n%s", fixture, i, text)
			}
		}
	}
	t.Logf("%d mutants rendered, %d returned an error", rendered, failed)
}
