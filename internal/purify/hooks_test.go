package purify

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

// upperASCII is the uppercase of a tag name as Element.nodeName gives it.
func upperASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 'a' + 'A'
		}
	}
	return string(b)
}

// domNodeName is Node.nodeName.
func domNodeName(n *dom.Node) string {
	if n.Type == dom.ElementNode && n.NS == dom.NSHTML {
		return upperASCII(n.Tag)
	}
	return nodeName(n)
}

// traceHooks installs a hook at every entry point that records the call, as
// the purifyTrace job of tools/reference/purify-page.js does.
func traceHooks(p *Purifier, trace *[]string) {
	points := []struct {
		point HookPoint
		short string
	}{
		{BeforeSanitizeElements, "bE"},
		{UponSanitizeElement, "uE"},
		{AfterSanitizeElements, "aE"},
		{BeforeSanitizeAttributes, "bA"},
		{UponSanitizeAttribute, "uA"},
		{AfterSanitizeAttributes, "aA"},
		{BeforeSanitizeShadowDOM, "bS"},
		{UponSanitizeShadowNode, "uS"},
		{AfterSanitizeShadowDOM, "aS"},
	}
	for _, pt := range points {
		p.AddHook(pt.point, func(node *dom.Node, event *HookEvent, _ *Config) {
			entry := pt.short + " "
			switch pt.point {
			case BeforeSanitizeShadowDOM, AfterSanitizeShadowDOM:
				// The node is the content fragment of a template.
				entry += "#document-fragment"
			default:
				entry += domNodeName(node)
			}
			switch pt.point {
			case UponSanitizeElement:
				entry += " " + event.TagName
			case UponSanitizeAttribute:
				entry += " " + event.AttrName + "=" + event.AttrValue
			}
			*trace = append(*trace, entry)
		})
	}
}

type traceResult struct {
	Out   expected `json:"out"`
	Trace []string `json:"trace"`
}

type hooksGolden struct {
	Inputs     []string      `json:"inputs"`
	Trace      []traceResult `json:"trace"`
	TraceFinal struct {
		Config json.RawMessage `json:"config"`
		Every  int             `json:"every"`
		Out    []traceResult   `json:"out"`
	} `json:"traceFinal"`
	Mutate []struct {
		Out    expected `json:"out"`
		Popped string   `json:"popped"`
		Second expected `json:"second"`
		Third  expected `json:"third"`
		Fourth expected `json:"fourth"`
		First  string   `json:"first"`
	} `json:"mutate"`
}

// TestGoldenHookTrace compares the sequence of hook calls with DOMPurify.
// The sequence shows the order of the node walk, including the nodes that
// the sanitizer moves and removes during the walk.
func TestGoldenHookTrace(t *testing.T) {
	var golden hooksGolden
	readGolden(t, "hooks.json.gz", &golden)
	r := &report{t: t}
	check := func(config any, input string, want *traceResult, label string) {
		p := New()
		var trace []string
		traceHooks(p, &trace)
		got := run(p, config, input, &want.Out)
		if got == "" && strings.Join(trace, "\n") != strings.Join(want.Trace, "\n") {
			got = "trace:\n    " + strings.Join(trace, "\n    ") + "\n  want trace:\n    " + strings.Join(want.Trace, "\n    ")
		}
		r.check(label, input, got, &want.Out)
	}
	for i, input := range golden.Inputs {
		check(nil, input, &golden.Trace[i], "trace, default config")
	}
	config := parseConfig(t, golden.TraceFinal.Config)
	for k := range golden.TraceFinal.Out {
		check(config, golden.Inputs[k*golden.TraceFinal.Every], &golden.TraceFinal.Out[k], "trace, final SVG config")
	}
	r.done("hook trace")
}

// mutateHooks installs the hooks of the purifyMutate job of
// tools/reference/purify-page.js. It returns the id of the last hook.
func mutateHooks(p *Purifier) HookID {
	p.AddHook(UponSanitizeElement, func(node *dom.Node, event *HookEvent, _ *Config) {
		if event.TagName == "foo" {
			event.AllowedTags["foo"] = true
		}
		if event.TagName == "bar" {
			event.AllowedTags["b"] = false
		}
		if event.TagName == "gone" && node.Parent != nil {
			p.RemoveNode(node)
		}
	})
	p.AddHook(UponSanitizeAttribute, func(_ *dom.Node, event *HookEvent, _ *Config) {
		switch event.AttrName {
		case "x-keep":
			event.ForceKeepAttr = true
		case "title":
			// The values of the golden are ASCII.
			event.AttrValue = strings.ToUpper(event.AttrValue)
		case "class":
			event.KeepAttr = false
		case "x-allow":
			event.AllowedAttributes["x-allow"] = true
		case "x-forbid-id":
			event.AllowedAttributes["id"] = false
		}
	})
	p.AddHook(AfterSanitizeAttributes, func(node *dom.Node, _ *HookEvent, _ *Config) {
		if domNodeName(node) == "P" {
			_ = node.SetAttribute("data-seen", "1")
		}
	})
	return p.AddHook(BeforeSanitizeElements, func(node *dom.Node, _ *HookEvent, _ *Config) {
		if node.Type == dom.TextNode {
			node.Data = strings.ReplaceAll(node.Data, "secret", "***")
		}
	})
}

// TestGoldenHookMutate compares hooks that change the result, and the
// removal of hooks, with DOMPurify.
func TestGoldenHookMutate(t *testing.T) {
	var golden hooksGolden
	readGolden(t, "hooks.json.gz", &golden)
	r := &report{t: t}
	for i, input := range golden.Inputs {
		want := &golden.Mutate[i]
		p := New()
		if id := mutateHooks(p); id == 0 {
			t.Fatal("AddHook returned no id")
		}
		r.check("mutate, all hooks", input, run(p, nil, input, &want.Out), &want.Out)
		// removeHook with no function removes the hook that was added last.
		if popped := p.RemoveHook(BeforeSanitizeElements); (popped != nil) != (want.Popped == "function") {
			t.Errorf("RemoveHook returned %v, upstream returned a %s", popped != nil, want.Popped)
		}
		r.check("mutate, one hook removed", input, run(p, nil, input, &want.Second), &want.Second)
		p.RemoveHooks(UponSanitizeAttribute)
		r.check("mutate, attribute hooks removed", input, run(p, nil, input, &want.Third), &want.Third)
		p.RemoveAllHooks()
		r.check("mutate, no hooks", input, run(p, nil, input, &want.Fourth), &want.Fourth)
	}
	r.done("hook mutate")
}

func TestHooksAPI(t *testing.T) {
	p := New()
	var calls []string
	hook := func(name string) Hook {
		return func(*dom.Node, *HookEvent, *Config) { calls = append(calls, name) }
	}
	if id := p.AddHook("noSuchEntryPoint", hook("x")); id != 0 {
		t.Errorf("AddHook accepted an unknown entry point")
	}
	if id := p.AddHook(AfterSanitizeElements, nil); id != 0 {
		t.Errorf("AddHook accepted a nil function")
	}
	a := p.AddHook(AfterSanitizeElements, hook("a"))
	b := p.AddHook(AfterSanitizeElements, hook("b"))
	c := p.AddHook(AfterSanitizeElements, hook("c"))
	if a == 0 || b == 0 || c == 0 || a == b || b == c {
		t.Fatalf("ids %d %d %d", a, b, c)
	}
	// The hooks run in the order of AddHook. The body and the text node pass
	// afterSanitizeElements.
	p.Sanitize("<i>x</i>", Config{})
	if got, want := strings.Join(calls, ""), "abcabcabc"; got != want {
		t.Errorf("calls %q, want %q", got, want)
	}
	// removeHook with a function removes that hook.
	if p.RemoveHook(AfterSanitizeElements, b) == nil {
		t.Errorf("RemoveHook did not find the hook")
	}
	if p.RemoveHook(AfterSanitizeElements, b) != nil {
		t.Errorf("RemoveHook found a removed hook")
	}
	if p.RemoveHook(BeforeSanitizeElements) != nil {
		t.Errorf("RemoveHook returned a hook for an empty entry point")
	}
	calls = calls[:0]
	p.Sanitize("<i></i>", Config{})
	if got, want := strings.Join(calls, ""), "acac"; got != want {
		t.Errorf("calls %q, want %q", got, want)
	}
	// A text with no markup returns before the walk: no hook runs.
	calls = calls[:0]
	if got := p.Sanitize("a & b", Config{}); got != "a & b" || len(calls) != 0 {
		t.Errorf("fast path: %q, calls %v", got, calls)
	}
	p.RemoveAllHooks()
	p.Sanitize("<i></i>", Config{})
	if len(calls) != 0 {
		t.Errorf("a hook ran after RemoveAllHooks")
	}
}

func TestSetConfig(t *testing.T) {
	p := New()
	if err := p.SetConfig(Config{AllowedTags: []string{"b"}}); err != nil {
		t.Fatal(err)
	}
	// The argument of Sanitize is ignored while a config is set.
	if got, want := p.Sanitize("<b>x</b><i>y</i>", Config{AllowedTags: []string{"i"}}), "<b>x</b>y"; got != want {
		t.Errorf("with SetConfig: %q, want %q", got, want)
	}
	p.ClearConfig()
	if got, want := p.Sanitize("<b>x</b><i>y</i>", Config{AllowedTags: []string{"i"}}), "x<i>y</i>"; got != want {
		t.Errorf("after ClearConfig: %q, want %q", got, want)
	}
	if !p.IsValidAttribute("a", "href", "https://example.com") || p.IsValidAttribute("a", "href", "javascript:x") ||
		p.IsValidAttribute("a", "onclick", "x") {
		t.Errorf("IsValidAttribute")
	}
}

func TestUnsupported(t *testing.T) {
	p := New()
	if _, err := p.SanitizeErr("<b>x</b>", Config{ParserMediaType: "application/xhtml+xml"}); err == nil {
		t.Errorf("the XHTML parser must be reported as unsupported")
	}
	if _, err := p.SanitizeErr("<b>x</b>", Config{Namespace: "http://www.w3.org/2000/svg"}); err == nil {
		t.Errorf("a namespace other than HTML must be reported as unsupported")
	}
	// Markup that nests deeper than the parser accepts is an error, and the
	// result of Sanitize is empty.
	deep := strings.Repeat("<div>", 600) + "x"
	if _, err := p.SanitizeErr(deep, Config{}); err == nil {
		t.Errorf("deep nesting must be an error")
	}
	if got := p.Sanitize(deep, Config{}); got != "" {
		t.Errorf("Sanitize of deep nesting = %q", got[:min(len(got), 40)])
	}
}

// TestConcurrent runs many instances at the same time. The package has no
// mutable state at package level, so the race detector must stay quiet.
func TestConcurrent(t *testing.T) {
	const input = `<a href="x" target="_blank" onclick="y">l</a><svg><foreignObject><b>z</b></foreignObject></svg>`
	want := Sanitize(input, Config{AddTags: []string{"foreignobject"}})
	done := make(chan string, 16)
	for i := 0; i < 16; i++ {
		go func() {
			p := New()
			mermaidHooks(p)
			out := ""
			for j := 0; j < 50; j++ {
				p.Sanitize(input, Config{})
				out = New().Sanitize(input, Config{AddTags: []string{"foreignobject"}})
			}
			done <- out
		}()
	}
	for i := 0; i < 16; i++ {
		if got := <-done; got != want {
			t.Errorf("concurrent result %q, want %q", got, want)
		}
	}
}
