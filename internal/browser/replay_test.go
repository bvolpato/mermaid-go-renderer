package browser

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

// The replay test puts finished diagrams of mermaid back into the page and
// asks the engine what Chromium was asked: the box of each element, the
// rectangle of each HTML label, the length of each text, and the computed
// font of each label. tools/reference/gen-browser-replay.mjs makes the data
// with tools/reference/probes/replay.js.

// replayFile is one file of replay data.
type replayFile struct {
	Styles []string    `json:"styles"`
	Docs   []replayDoc `json:"docs"`
}

// replayDoc is one diagram: the markup that goes into div#container, and the
// values of Chromium. A record starts with the index of its element in the
// document order of all elements of the container.
type replayDoc struct {
	Name string  `json:"name"`
	Type string  `json:"type"`
	HTML string  `json:"html"`
	N    int     `json:"n"`
	Sig  uint32  `json:"sig"`
	BBox [][]any `json:"bbox"`
	Rect [][]any `json:"rect"`
	TLen [][]any `json:"tlen"`
	Font [][]any `json:"font"`
}

var replayStyleRef = regexp.MustCompile(`<style>/\*@(\d+)\*/</style>`)

func readReplayFile(path string) (*replayFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	var rf replayFile
	if err := json.NewDecoder(zr).Decode(&rf); err != nil {
		return nil, err
	}
	for i := range rf.Docs {
		d := &rf.Docs[i]
		d.HTML = replayStyleRef.ReplaceAllStringFunc(d.HTML, func(m string) string {
			idx, _ := strconv.Atoi(replayStyleRef.FindStringSubmatch(m)[1])
			if idx < 0 || idx >= len(rf.Styles) {
				return m
			}
			return "<style>" + rf.Styles[idx] + "</style>"
		})
	}
	return &rf, nil
}

// replayStat counts the values of one group.
type replayStat struct{ docs, exact, close, diff int }

func (s *replayStat) add(verdict string) {
	switch verdict {
	case "exact":
		s.exact++
	case "close":
		s.close++
	default:
		s.diff++
	}
}

// replayClass is a group of differences with the same form.
type replayClass struct {
	values  int
	docs    map[string]bool
	example string
}

// replayRun collects the results of the documents.
type replayRun struct {
	byType  map[string]*replayStat
	byKind  map[string]*replayStat
	classes map[string]*replayClass
	docs    int
	badTree []string
}

func newReplayRun() *replayRun {
	return &replayRun{byType: map[string]*replayStat{}, byKind: map[string]*replayStat{}, classes: map[string]*replayClass{}}
}

// replayElements lists the elements under n in document order.
func replayElements(n *dom.Node, out []*dom.Node) []*dom.Node {
	for c := n.FirstChild; c != nil; c = c.Next {
		if c.Type != dom.ElementNode {
			continue
		}
		out = append(out, c)
		out = replayElements(c, out)
	}
	return out
}

func replayOpenTag(n *dom.Node) string {
	var sb strings.Builder
	sb.WriteString("<" + n.Tag)
	for _, a := range n.Attrs {
		v := a.Value
		if len(v) > 90 {
			v = v[:90] + "…"
		}
		sb.WriteString(" " + a.QName() + `="` + v + `"`)
	}
	sb.WriteString(">")
	return sb.String()
}

// replayContext describes an element for a report: its ancestors up to the
// foreignObject or the text that holds it, and its markup.
func replayContext(n *dom.Node) string {
	root := n
	for p := n; p != nil && p.Type == dom.ElementNode; p = p.Parent {
		if p.NS == dom.NSSVG && (p.Tag == "foreignObject" || p.Tag == "text") {
			root = p
			break
		}
	}
	markup := root.OuterHTML()
	if len(markup) > 700 {
		markup = markup[:700] + "…"
	}
	var chain []string
	for p := root.Parent; p != nil && p.Type == dom.ElementNode && p.Tag != "svg"; p = p.Parent {
		chain = append([]string{replayOpenTag(p)}, chain...)
	}
	return strings.Join(chain, "") + "\n        " + markup
}

func replayNumbers(rec []any) ([]float64, bool) {
	out := make([]float64, 0, len(rec)-1)
	for _, v := range rec[1:] {
		f, ok := v.(float64)
		if !ok {
			return nil, false
		}
		out = append(out, f)
	}
	return out, true
}

// replayDeltaName names the size of a difference, for grouping.
func replayDeltaName(d float64) string {
	d = math.Abs(d)
	switch {
	case d <= 0.01:
		return "<=0.01"
	case d < 1:
		return "<1"
	case d < 4:
		return strconv.FormatFloat(math.Round(d*2)/2, 'f', -1, 64)
	}
	return ">=4"
}

// run replays one document.
func (r *replayRun) run(d *replayDoc, tol float64) {
	doc := NewDocument()
	container := doc.Body.Append("div").Attr("id", "container")
	container.HTML(d.HTML)
	els := replayElements(container, nil)
	sig := uint32(2166136261)
	for _, el := range els {
		for _, c := range []byte(el.Tag + "|") {
			sig ^= uint32(c)
			sig *= 16777619
		}
	}
	r.docs++
	if len(els) != d.N || sig != d.Sig {
		r.badTree = append(r.badTree, fmt.Sprintf("%s: %d elements (Chromium %d)", d.Name, len(els), d.N))
		return
	}
	typ := r.byType[d.Type]
	if typ == nil {
		typ = &replayStat{}
		r.byType[d.Type] = typ
	}
	typ.docs++
	note := func(kind string, el *dom.Node, verdict, form string, got, want any) {
		typ.add(verdict)
		ks := r.byKind[kind]
		if ks == nil {
			ks = &replayStat{}
			r.byKind[kind] = ks
		}
		ks.add(verdict)
		if verdict == "exact" || (verdict == "close" && os.Getenv("MMDG_REPLAY_CLOSE") == "") {
			return
		}
		if verdict == "close" {
			form = "close " + form
		}
		tag := el.Tag
		if el.NS == dom.NSHTML {
			tag = "html:" + tag + "(" + doc.Engine.ComputedStyle(el, "display") + ")"
		}
		key := kind + " " + tag + " " + form
		cl := r.classes[key]
		if cl == nil {
			gj, _ := json.Marshal(got)
			wj, _ := json.Marshal(want)
			cl = &replayClass{docs: map[string]bool{}, example: fmt.Sprintf("%s\n        got  %s\n        want %s\n        %s\n        %s", d.Name, gj, wj, replayOpenTag(el), replayContext(el))}
			r.classes[key] = cl
		}
		cl.values++
		cl.docs[d.Name] = true
	}
	numbers := func(kind string, recs [][]any, names []string, measure func(el *dom.Node) []float64) {
		for _, rec := range recs {
			idx := int(rec[0].(float64))
			el := els[idx]
			want, ok := replayNumbers(rec)
			if !ok {
				continue
			}
			got := measure(el)
			verdict, form := "exact", ""
			for i := range want {
				if got[i] == want[i] {
					continue
				}
				if math.Abs(got[i]-want[i]) <= tol {
					if verdict == "exact" {
						verdict = "close"
					}
					if verdict == "close" {
						form += " " + names[i]
					}
					continue
				}
				verdict = "diff"
				form += " " + names[i] + ":" + replayDeltaName(got[i]-want[i])
			}
			note(kind, el, verdict, strings.TrimSpace(form), got, want)
		}
	}
	box := []string{"x", "y", "w", "h"}
	numbers("bbox", d.BBox, box, func(el *dom.Node) []float64 {
		b := el.BBox()
		return []float64{b.X, b.Y, b.Width, b.Height}
	})
	numbers("rect", d.Rect, box, func(el *dom.Node) []float64 {
		b := el.BoundingClientRect()
		return []float64{b.X, b.Y, b.Width, b.Height}
	})
	numbers("tlen", d.TLen, []string{"len"}, func(el *dom.Node) []float64 {
		return []float64{el.ComputedTextLength()}
	})
	props := []string{"font-size", "font-family", "font-weight", "font-style", "display"}
	for _, rec := range d.Font {
		el := els[int(rec[0].(float64))]
		verdict, form := "exact", ""
		got := make([]any, 0, len(rec)-1)
		for i, want := range rec[1:] {
			v := el.ComputedStyle(props[i])
			got = append(got, v)
			if v != want {
				verdict = "diff"
				form += " " + props[i]
			}
		}
		note("font", el, verdict, strings.TrimSpace(form), got, rec[1:])
	}
}

// report logs the counts of each diagram type and of each kind of value,
// and the groups of differences, the group with the most documents first.
func (r *replayRun) report(t *testing.T, show int) (total replayStat) {
	t.Helper()
	log := func(title string, m map[string]*replayStat, withDocs bool) {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			s := m[k]
			docs := ""
			if withDocs {
				docs = fmt.Sprintf(" docs=%4d", s.docs)
			}
			t.Logf("%s %-16s%s exact=%7d close=%5d diff=%5d", title, k, docs, s.exact, s.close, s.diff)
		}
	}
	log("type", r.byType, true)
	log("kind", r.byKind, false)
	for _, s := range r.byKind {
		total.exact += s.exact
		total.close += s.close
		total.diff += s.diff
	}
	t.Logf("TOTAL docs=%d exact=%d close(<=0.01)=%d diff=%d", r.docs, total.exact, total.close, total.diff)
	for _, bad := range r.badTree {
		t.Errorf("the tree differs from the tree of Chromium: %s", bad)
	}
	keys := make([]string, 0, len(r.classes))
	for k := range r.classes {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := r.classes[keys[i]], r.classes[keys[j]]
		if len(a.docs) != len(b.docs) {
			return len(a.docs) > len(b.docs)
		}
		if a.values != b.values {
			return a.values > b.values
		}
		return keys[i] < keys[j]
	})
	for i, k := range keys {
		if i >= show {
			t.Logf("... and %d more groups", len(keys)-show)
			break
		}
		cl := r.classes[k]
		t.Logf("docs=%4d values=%5d  %s\n      e.g. %s", len(cl.docs), cl.values, k, cl.example)
	}
	return total
}

func replayShow() int {
	if v, err := strconv.Atoi(os.Getenv("MMDG_REPLAY_SHOW")); err == nil {
		return v
	}
	return 12
}

// TestReplayAgainstChromium replays the documents of testdata/replay.json.gz,
// a sample of the reference corpus that has each diagram type and each
// distinct label. A record that an earlier document has with the same markup
// and style context is not in the file, and neither is a document with
// MathML.
//
// No value can differ from the value of Chromium by more than 0.01, the
// tolerance of the parity command. The rectangles of the HTML labels, the
// text lengths and the computed fonts must be equal to the values of
// Chromium. A few boxes of paths with arcs differ in the last bit of a
// float.
func TestReplayAgainstChromium(t *testing.T) {
	macOSProbe(t)
	rf, err := readReplayFile("testdata/replay.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	filter := os.Getenv("MMDG_REPLAY_FILTER")
	run := newReplayRun()
	for i := range rf.Docs {
		if filter == "" || strings.Contains(rf.Docs[i].Name, filter) {
			run.run(&rf.Docs[i], 0.01)
		}
	}
	total := run.report(t, replayShow())
	if total.diff != 0 {
		t.Errorf("%d values differ from Chromium", total.diff)
	}
	for _, kind := range []string{"rect", "tlen", "font"} {
		if s := run.byKind[kind]; s != nil && s.close != 0 {
			t.Errorf("%d %s values are not equal to the values of Chromium", s.close, kind)
		}
	}
}

// TestReplayCorpusAgainstChromium replays the whole reference corpus from
// .parity/replay, which `gen-browser-replay.mjs measure` writes. It takes
// about half a minute, so it runs only with MMDG_REPLAY_ALL=1, and it is
// skipped without that directory. MMDG_REPLAY_DIR names another directory.
func TestReplayCorpusAgainstChromium(t *testing.T) {
	dir := os.Getenv("MMDG_REPLAY_DIR")
	if dir == "" {
		if os.Getenv("MMDG_REPLAY_ALL") == "" {
			t.Skip("set MMDG_REPLAY_ALL=1 to replay the whole corpus")
		}
		dir = "../../.parity/replay"
	}
	files, _ := filepath.Glob(filepath.Join(dir, "batch-*.json.gz"))
	if len(files) == 0 {
		t.Skip("no replay batches in", dir)
	}
	sort.Strings(files)
	filter := os.Getenv("MMDG_REPLAY_FILTER")
	run := newReplayRun()
	for _, file := range files {
		rf, err := readReplayFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i := range rf.Docs {
			if filter == "" || strings.Contains(rf.Docs[i].Name, filter) {
				run.run(&rf.Docs[i], 0.01)
			}
		}
	}
	run.report(t, replayShow())
}
