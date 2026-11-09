package purify

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The goldens come from DOMPurify 3.4.12 in Chromium. Regenerate them with
// `node tools/reference/gen-purify.mjs`.

// expected is one result of the oracle. See gen-purify.mjs for the encoding.
type expected struct {
	same  bool
	str   string
	throw bool
	node  string // Object.prototype.toString of a node result
	html  string
	null  bool
}

func (e *expected) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	switch {
	case bytes.Equal(data, []byte("0")):
		e.same = true
		return nil
	case len(data) > 0 && data[0] == '"':
		return json.Unmarshal(data, &e.str)
	}
	var obj struct {
		Throw *string `json:"throw"`
		Node  string  `json:"node"`
		HTML  string  `json:"html"`
		Value *string `json:"value"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	switch {
	case obj.Throw != nil:
		e.throw = true
	case obj.Value != nil:
		if *obj.Value != "null" {
			return fmt.Errorf("unexpected value %q", *obj.Value)
		}
		e.null = true
	default:
		e.node, e.html = obj.Node, obj.HTML
	}
	return nil
}

// describe prints an expected result for a failure message.
func (e *expected) describe(input string) string {
	switch {
	case e.same:
		return fmt.Sprintf("%q", input)
	case e.throw:
		return "throw"
	case e.null:
		return "null"
	case e.node != "":
		return fmt.Sprintf("%s %q", e.node, e.html)
	}
	return fmt.Sprintf("%q", e.str)
}

func readGolden(t testing.TB, name string, v any) {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatal(err)
	}
}

// mermaidHooks installs the hooks of setupDompurifyHooks of mermaid
// (diagrams/common/common.ts). internal/sanitize has the same code. It is
// repeated here because the goldens record DOMPurify with these hooks.
func mermaidHooks(p *Purifier) {
	const temporaryAttribute = "data-temp-href-target"
	isAnchor := func(n *dom.Node) bool {
		return n.Type == dom.ElementNode && n.NS == dom.NSHTML && n.Tag == "a"
	}
	p.AddHook(BeforeSanitizeAttributes, func(node *dom.Node, _ *HookEvent, _ *Config) {
		if isAnchor(node) && node.HasAttribute("target") {
			v, _ := node.GetAttribute("target")
			_ = node.SetAttribute(temporaryAttribute, v)
		}
	})
	p.AddHook(AfterSanitizeAttributes, func(node *dom.Node, _ *HookEvent, _ *Config) {
		if isAnchor(node) && node.HasAttribute(temporaryAttribute) {
			v, _ := node.GetAttribute(temporaryAttribute)
			_ = node.SetAttribute("target", v)
			node.RemoveAttribute(temporaryAttribute)
			if v, _ := node.GetAttribute("target"); v == "_blank" {
				_ = node.SetAttribute("rel", "noopener")
			}
		}
	})
}

// run calls the port as the oracle called DOMPurify, and compares.
// It returns "" on a match, or a description of what the port returned.
func run(p *Purifier, config any, input string, want *expected) string {
	cfg, err := ConfigFromJS(config)
	if err != nil {
		if want.throw {
			return ""
		}
		return "error: " + err.Error()
	}
	if cfg.ReturnDOM || cfg.ReturnDOMFragment {
		node, err := p.SanitizeDOM(input, cfg)
		switch {
		case err != nil:
			if want.throw {
				return ""
			}
			return "error: " + err.Error()
		case node == nil:
			if want.null {
				return ""
			}
			return "null"
		}
		var html string
		if cfg.ReturnDOMFragment {
			html = node.InnerHTMLInert()
		} else {
			html = node.OuterHTMLInert()
		}
		if want.node != "" && html == want.html {
			return ""
		}
		return fmt.Sprintf("node %q", html)
	}
	got, err := p.SanitizeErr(input, cfg)
	if err != nil {
		if want.throw {
			return ""
		}
		return "error: " + err.Error()
	}
	switch {
	case want.same:
		if got == input {
			return ""
		}
	case want.throw, want.null, want.node != "":
	default:
		if got == want.str {
			return ""
		}
	}
	return fmt.Sprintf("%q", got)
}

func parseConfig(t testing.TB, raw json.RawMessage) any {
	t.Helper()
	if len(raw) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		// The oracle called sanitize with no config argument.
		return nil
	}
	v, err := js.ParseJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// chromiumTrees maps an input to the tree that DOMParser builds for it in
// Chromium. It is the parser golden of internal/dom.
var chromiumTrees = sync.OnceValue(func() map[string]string {
	f, err := os.Open(filepath.Join("..", "dom", "testdata", "purify_parse.json.gz"))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		panic(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		panic(err)
	}
	var golden struct {
		Inputs []string `json:"inputs"`
		Trees  []string `json:"trees"`
	}
	if err := json.Unmarshal(data, &golden); err != nil {
		panic(err)
	}
	trees := make(map[string]string, len(golden.Inputs))
	for i, input := range golden.Inputs {
		trees[input] = golden.Trees[i]
	}
	return trees
})

// parserDiverges reports whether the HTML parser of the port builds a tree
// for the input that differs from the tree of Chromium. The sanitizer cannot
// give the output of Chromium for such an input. TestGoldenParse of
// internal/dom lists these inputs by cause.
func parserDiverges(input string) bool {
	want, ok := chromiumTrees()[input]
	if !ok {
		return false
	}
	doc, err := dom.ParseHTMLDocument(input)
	return err != nil || doc.Root.DumpTree() != want
}

// report counts a comparison and prints the first failures. A failure on an
// input whose parse tree differs from Chromium is counted apart and is not
// an error of the sanitizer.
type report struct {
	t      *testing.T
	total  int
	failed int
	parser int
}

func (r *report) check(label, input, got string, want *expected) {
	r.total++
	if got == "" {
		return
	}
	if parserDiverges(input) {
		r.parser++
		return
	}
	r.failed++
	dumpFailure(label, input, got, want)
	if r.failed <= 15 {
		r.t.Errorf("%s\n  input: %q\n  got:   %s\n  want:  %s", label, input, got, want.describe(input))
	}
}

func (r *report) done(name string) {
	r.t.Logf("%s: %d of %d cases pass, %d differ because the parse tree differs from Chromium, %d fail",
		name, r.total-r.failed-r.parser, r.total, r.parser, r.failed)
	if r.failed > 15 {
		r.t.Errorf("%s: %d more failures not shown", name, r.failed-15)
	}
}

// dumpFailure appends a failure to the file that PURIFY_FAILURES names, one
// JSON object for each line. It helps to classify failures after a change of
// the parser or of the goldens.
func dumpFailure(label, input, got string, want *expected) {
	name := os.Getenv("PURIFY_FAILURES")
	if name == "" {
		return
	}
	f, err := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	line, _ := json.Marshal(map[string]string{"case": label, "input": input, "got": got, "want": want.describe(input)})
	f.Write(append(line, '\n'))
}

// newPurifier returns an instance in one of the two states that mermaid can
// have: before and after setupDompurifyHooks.
func newPurifier(hooks bool) *Purifier {
	p := New()
	if hooks {
		mermaidHooks(p)
	}
	return p
}

// TestGoldenSVGClean sanitizes reference SVGs of mermaid with the
// configuration of mermaidAPI.ts. It also checks that the parser and the
// serializer of internal/dom round trip real output.
func TestGoldenSVGClean(t *testing.T) {
	var golden struct {
		Config json.RawMessage `json:"config"`
		Cases  []struct {
			Name string   `json:"name"`
			In   string   `json:"in"`
			Out  expected `json:"out"`
		} `json:"cases"`
		OutHooks []*expected `json:"outHooks"`
	}
	readGolden(t, "svg_clean.json.gz", &golden)
	config := parseConfig(t, golden.Config)
	r := &report{t: t}
	for i, c := range golden.Cases {
		for _, hooks := range []bool{false, true} {
			want := &c.Out
			if hooks && golden.OutHooks[i] != nil {
				want = golden.OutHooks[i]
			}
			got := run(newPurifier(hooks), config, c.In, want)
			if got != "" && len(got) > 300 {
				got = firstDiff(c.In, got)
			}
			r.check(fmt.Sprintf("%s hooks=%v", c.Name, hooks), c.Name, got, want)
		}
	}
	r.done("svg_clean")
}

// firstDiff shortens a failure on a large document to the place where the
// output starts to differ from the input.
func firstDiff(input, got string) string {
	// got is quoted by run.
	var unq string
	if _, err := fmt.Sscanf(got, "%q", &unq); err != nil {
		return got[:300]
	}
	i := 0
	for i < len(input) && i < len(unq) && input[i] == unq[i] {
		i++
	}
	lo := max(0, i-60)
	return fmt.Sprintf("differs at byte %d: input %q, got %q", i, input[lo:min(len(input), i+80)], unq[lo:min(len(unq), i+80)])
}

// TestGoldenFinalHostile sanitizes hostile and odd inputs with the
// configuration of mermaidAPI.ts.
func TestGoldenFinalHostile(t *testing.T) {
	var golden struct {
		Config   json.RawMessage `json:"config"`
		Inputs   []string        `json:"inputs"`
		Out      []expected      `json:"out"`
		OutHooks []*expected     `json:"outHooks"`
	}
	readGolden(t, "final_hostile.json.gz", &golden)
	config := parseConfig(t, golden.Config)
	r := &report{t: t}
	for i, input := range golden.Inputs {
		for _, hooks := range []bool{false, true} {
			want := &golden.Out[i]
			if hooks && golden.OutHooks[i] != nil {
				want = golden.OutHooks[i]
			}
			r.check(fmt.Sprintf("#%d hooks=%v", i, hooks), input, run(newPurifier(hooks), config, input, want), want)
		}
	}
	r.done("final_hostile")
}

type goldenRuns struct {
	Inputs []string `json:"inputs"`
	Runs   []struct {
		Config json.RawMessage `json:"config"`
		Hooks  bool            `json:"hooks"`
		Out    []expected      `json:"out"`
	} `json:"runs"`
}

func testRuns(t *testing.T, file string) {
	var golden goldenRuns
	readGolden(t, file, &golden)
	all := &report{t: t}
	for _, run1 := range golden.Runs {
		config := parseConfig(t, run1.Config)
		r := &report{t: t}
		// One instance for the whole run, as in the oracle: this also checks
		// that no state leaks from one call to the next.
		p := newPurifier(run1.Hooks)
		for i, input := range golden.Inputs {
			want := &run1.Out[i]
			r.check(fmt.Sprintf("config %s hooks=%v #%d", run1.Config, run1.Hooks, i), input, run(p, config, input, want), want)
		}
		if r.failed > 0 {
			t.Logf("config %s hooks=%v: %d of %d cases fail", run1.Config, run1.Hooks, r.failed, r.total)
		}
		all.total += r.total
		all.failed += r.failed
		all.parser += r.parser
	}
	t.Logf("%s: %d of %d cases pass, %d differ because the parse tree differs from Chromium, %d fail",
		file, all.total-all.failed-all.parser, all.total, all.parser, all.failed)
}

// TestGoldenLabels sanitizes label texts with the configurations that
// common.ts uses: no config (removeScript) and FORBID_TAGS style
// (sanitizeText), each with and without the hooks of mermaid.
func TestGoldenLabels(t *testing.T) {
	testRuns(t, "labels.json.gz")
}

// TestGoldenConfigs sanitizes hostile inputs with every option of
// _parseConfig that a JSON config can express.
func TestGoldenConfigs(t *testing.T) {
	testRuns(t, "configs.json.gz")
}
