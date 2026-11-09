package purify

import (
	"fmt"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/memo"
)

// settingsConfigs are configurations that differ in one option each. Every
// option of Config that has a content is in the list, so that the list also
// checks that the key of a configuration has every option.
func settingsConfigs() []Config {
	return []Config{
		{},
		{AllowedTags: []string{}},
		{AllowedTags: []string{"b"}},
		{AllowedTags: []string{"b", "i"}},
		{AllowedTags: []string{"bi"}},
		{AllowedAttr: []string{"b"}},
		{AllowedAttr: []string{}},
		{AllowedNamespaces: []string{"http://www.w3.org/1999/xhtml"}},
		{AddURISafeAttr: []string{"b"}},
		{AddDataURITags: []string{"b"}},
		{ForbidContents: []string{"b"}},
		{AddForbidContents: []string{"b"}},
		{ForbidTags: []string{"b"}},
		{ForbidTags: []string{"style"}},
		{ForbidAttr: []string{"b"}},
		{AddTags: []string{"b"}},
		{AddTags: []string{"foreignobject"}},
		{AddAttr: []string{"b"}},
		{AddTags: []string{"foreignobject"}, AddAttr: []string{"dominant-baseline"}, HTMLIntegrationPoints: map[string]bool{"foreignobject": true}},
		{UseProfiles: &Profiles{}},
		{UseProfiles: &Profiles{HTML: true}},
		{UseProfiles: &Profiles{SVG: true}},
		{UseProfiles: &Profiles{SVGFilters: true}},
		{UseProfiles: &Profiles{MathML: true}},
		{AllowAriaAttr: False},
		{AllowDataAttr: False},
		{AllowUnknownProtocols: true},
		{AllowSelfCloseInAttr: False},
		{SafeForTemplates: true},
		{SafeForXML: False},
		{WholeDocument: true},
		{ForceBody: true},
		{SanitizeDOM: False},
		{SanitizeNamedProps: true},
		{KeepContent: False},
		{Namespace: "http://www.w3.org/1999/xhtml"},
		{ParserMediaType: "text/html"},
		{MathMLTextIntegrationPoints: map[string]bool{}},
		{MathMLTextIntegrationPoints: map[string]bool{"mi": true}},
		{MathMLTextIntegrationPoints: map[string]bool{"mi": false}},
		{HTMLIntegrationPoints: map[string]bool{"mi": true}},
		{HTMLIntegrationPoints: map[string]bool{"mi": true, "foreignobject": true}},
		{CustomElementHandling: &CustomElementHandling{}},
		{CustomElementHandling: &CustomElementHandling{AllowCustomizedBuiltInElements: true}},
	}
}

// settingsInputs are texts that the options above treat in different ways.
var settingsInputs = []string{
	`plain text`,
	`<b class="x" data-a="1" aria-label="l" onclick="x()">bold</b><i>it</i><style>p{}</style>`,
	`<svg width="10"><foreignObject><div xmlns="http://www.w3.org/1999/xhtml">a<br>b</div></foreignObject><text dominant-baseline="middle">t</text><script>x</script></svg>`,
	`<a href="javascript:alert(1)" target="_blank" id="location" name="x">link</a><a href="foo:bar">u</a>`,
	`<p>{{ x }} <%= y %> ${z}</p><math><mi>x</mi></math><my-tag is="x">c</my-tag><img src="data:image/png;base64,AA" alt="/>">`,
	`<!DOCTYPE html><html><head><title>t</title></head><body><table><tr><td>c</td></tr></table></body></html>`,
}

// TestSettingsKey checks that configurations that differ have different
// keys, and that a configuration with a function has no key.
func TestSettingsKey(t *testing.T) {
	seen := map[string]int{}
	for i, cfg := range settingsConfigs() {
		key, ok := cfg.appendKey(nil)
		if !ok {
			t.Errorf("configuration %d has no key", i)
			continue
		}
		if j, dup := seen[string(key)]; dup {
			t.Errorf("configurations %d and %d have the same key", j, i)
		}
		seen[string(key)] = i
		// The values that Sanitize sets are in the key too.
		for _, variant := range []Config{{ReturnDOM: true}, {ReturnDOMFragment: true}} {
			c := cfg
			c.ReturnDOM, c.ReturnDOMFragment = variant.ReturnDOM, variant.ReturnDOMFragment
			if k, _ := c.appendKey(nil); string(k) == string(key) {
				t.Errorf("configuration %d: RETURN_DOM is not in the key", i)
			}
		}
	}
	for i, cfg := range []Config{
		{AddTagsFunc: func(string) bool { return true }},
		{AddAttrFunc: func(string, string) bool { return true }},
		{AllowedURI: func(string) bool { return true }},
		{CustomElementHandling: &CustomElementHandling{TagNameCheck: func(string) bool { return true }}},
		{CustomElementHandling: &CustomElementHandling{AttributeNameCheck: func(string, string) bool { return true }}},
	} {
		if _, ok := cfg.appendKey(nil); ok {
			t.Errorf("configuration %d with a function has a key", i)
		}
	}
}

// sanitizeAll sanitizes every input with every configuration, with new
// instances and with instances that have the hooks of mermaid.
func sanitizeAll() []string {
	var out []string
	for _, cfg := range settingsConfigs() {
		plain, hooked := New(), New()
		// Hooks as mermaid installs them: they change nodes, not settings.
		hooked.AddHook(BeforeSanitizeAttributes, func(node *dom.Node, _ *HookEvent, _ *Config) {
			if node.Type == dom.ElementNode && node.HasAttribute("target") {
				_ = node.SetAttribute("data-temp-href-target", "x")
			}
		})
		for _, in := range settingsInputs {
			a, errA := plain.SanitizeErr(in, cfg)
			b, errB := hooked.SanitizeErr(in, cfg)
			node, errC := New().SanitizeDOM(in, cfg)
			tree := "nil"
			if node != nil {
				tree = node.OuterHTMLInert()
			}
			out = append(out, fmt.Sprint(a, "|", errA, "|", b, "|", errB, "|", tree, "|", errC))
		}
	}
	return out
}

// TestSettingsAreTheParsedConfig compares the results with the kept
// settings, on the first use and on later uses, with the results of
// _parseConfig for every call.
func TestSettingsAreTheParsedConfig(t *testing.T) {
	memo.SetEnabled(false)
	want := sanitizeAll()
	memo.SetEnabled(true)
	memo.Purge()
	for round := 0; round < 3; round++ {
		got := sanitizeAll()
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("round %d, case %d (configuration %d, input %d):\n got  %s\n want %s",
					round, i, i/len(settingsInputs), i%len(settingsInputs), got[i], want[i])
			}
		}
	}
	if parsedConfigs.Len() == 0 {
		t.Error("no settings were kept")
	}
}

// TestHookDoesNotChangeKeptSettings lets a hook change the allowed tags and
// attributes of one call, and checks that later calls, of the same instance
// and of others, have the settings of the configuration.
func TestHookDoesNotChangeKeptSettings(t *testing.T) {
	memo.Purge()
	const in = `<foo bar="1">x</foo><b>y</b>`
	cfg := Config{}
	want := Sanitize(in, cfg)
	changing := New()
	changing.AddHook(UponSanitizeElement, func(_ *dom.Node, event *HookEvent, _ *Config) {
		event.AllowedTags["foo"] = true
		event.AllowedTags["b"] = false
	})
	changing.AddHook(UponSanitizeAttribute, func(_ *dom.Node, event *HookEvent, _ *Config) {
		event.AllowedAttributes["bar"] = true
	})
	changed := changing.Sanitize(in, cfg)
	if changed == want {
		t.Fatalf("the hooks changed nothing: %q", changed)
	}
	for i := 0; i < 2; i++ {
		if got := Sanitize(in, cfg); got != want {
			t.Fatalf("after a hook changed the sets: %q, want %q", got, want)
		}
		if got := changing.Sanitize(in, cfg); got != changed {
			t.Fatalf("the instance with hooks: %q, want %q", got, changed)
		}
	}
	// A hook that changes the configuration it gets changes a copy.
	scribbling := New()
	scribbling.AddHook(BeforeSanitizeElements, func(_ *dom.Node, _ *HookEvent, c *Config) {
		c.ForbidTags = []string{"b"}
		c.KeepContent = False
	})
	scribbling.Sanitize(in, cfg)
	if got := Sanitize(in, cfg); got != want {
		t.Fatalf("after a hook changed the configuration: %q, want %q", got, want)
	}
}

// TestSettingsConcurrent sanitizes with the same configurations from many
// goroutines. The race detector finds a shared set that changes.
func TestSettingsConcurrent(t *testing.T) {
	memo.Purge()
	memo.SetEnabled(false)
	want := sanitizeAll()
	memo.SetEnabled(true)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := 0; round < 3; round++ {
				got := sanitizeAll()
				for i := range want {
					if got[i] != want[i] {
						t.Errorf("case %d: got %s, want %s", i, got[i], want[i])
						return
					}
				}
			}
		}()
	}
	wg.Wait()
}
