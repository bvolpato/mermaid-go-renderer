package stylis

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
)

// bundledVersion is the version of stylis inside the mermaid 12.2.0 bundle.
// The package in node_modules is newer.
const bundledVersion = "4.3.6"

// The golden file comes from tools/reference/gen-stylis.mjs, which runs the
// real stylis and the compileCSS function of the built mermaid. The stylis is
// the bundled version: the script loads its source from the source map of the
// mermaid bundle.
type goldenFile struct {
	// Stylis is the version that made the expected values.
	Stylis string `json:"stylis"`
	// Newer is the version of stylis in node_modules.
	Newer string       `json:"newer"`
	Cases []goldenCase `json:"cases"`
}

type goldenCase struct {
	Group string       `json:"group"`
	NS    string       `json:"ns"`
	CSS   string       `json:"css"`
	AST   []goldenNode `json:"ast"`
	Plain *string      `json:"plain"`
	// Newer is the plain output of the stylis in node_modules. It is set only
	// when that version gives another output than the bundled version.
	Newer *string `json:"newer"`
	Out   *string `json:"out"`
	Error *string `json:"error"`
}

type goldenNode struct {
	Type     string          `json:"t"`
	Value    string          `json:"v"`
	Props    json.RawMessage `json:"p"`
	Children json.RawMessage `json:"c"`
	Length   int             `json:"n"`
	Line     int             `json:"l"`
	Column   int             `json:"k"`
}

func loadGolden(t *testing.T) goldenFile {
	t.Helper()
	f, err := os.Open("testdata/stylis.json.gz")
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
	var g goldenFile
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

// compareTree checks elements against the tree that stylis made. It returns
// a description of the first difference, or "".
func compareTree(got []*Element, want []goldenNode, positions bool, path string) string {
	if len(got) != len(want) {
		return fmt.Sprintf("%s: %d elements, want %d", path, len(got), len(want))
	}
	for i, w := range want {
		g := got[i]
		p := fmt.Sprintf("%s[%d]", path, i)
		if g.Type != w.Type {
			return fmt.Sprintf("%s: type %q, want %q", p, g.Type, w.Type)
		}
		if g.Value != w.Value {
			return fmt.Sprintf("%s: value %q, want %q", p, g.Value, w.Value)
		}
		if g.Length != w.Length {
			return fmt.Sprintf("%s: length %d, want %d", p, g.Length, w.Length)
		}
		if positions && (g.Line != w.Line || g.Column != w.Column) {
			return fmt.Sprintf("%s: position %d:%d, want %d:%d", p, g.Line, g.Column, w.Line, w.Column)
		}
		if bytes.HasPrefix(w.Props, []byte("[")) {
			var props []string
			if err := json.Unmarshal(w.Props, &props); err != nil {
				return p + ": " + err.Error()
			}
			if g.Props == nil {
				return fmt.Sprintf("%s: props is not a list, want %q", p, props)
			}
			if fmt.Sprintf("%q", g.Props) != fmt.Sprintf("%q", props) {
				return fmt.Sprintf("%s: props %q, want %q", p, g.Props, props)
			}
		} else {
			var prop string
			if err := json.Unmarshal(w.Props, &prop); err != nil {
				return p + ": " + err.Error()
			}
			if g.Props != nil || g.Prop != prop {
				return fmt.Sprintf("%s: props %q %q, want the string %q", p, g.Props, g.Prop, prop)
			}
		}
		if bytes.HasPrefix(w.Children, []byte("[")) {
			var children []goldenNode
			if err := json.Unmarshal(w.Children, &children); err != nil {
				return p + ": " + err.Error()
			}
			if g.Children == nil {
				return fmt.Sprintf("%s: children is not a list", p)
			}
			if diff := compareTree(g.Children, children, positions, p); diff != "" {
				return diff
			}
		} else {
			var text string
			if err := json.Unmarshal(w.Children, &text); err != nil {
				return p + ": " + err.Error()
			}
			if g.Children != nil || g.Text != text {
				return fmt.Sprintf("%s: children %q, want the string %q", p, g.Text, text)
			}
		}
	}
	return ""
}

func TestGolden(t *testing.T) {
	golden := loadGolden(t)
	if len(golden.Cases) < 5000 {
		t.Fatalf("the golden file has %d cases, want at least 5000", len(golden.Cases))
	}
	if golden.Stylis != bundledVersion {
		t.Fatalf("the golden file comes from stylis %s, want %s, the version in the mermaid bundle", golden.Stylis, bundledVersion)
	}
	type count struct{ total, failed int }
	groups := map[string]*count{}
	failures := 0
	// versionCases counts the cases that tell the bundled version from the
	// newer one. wrongVersion counts those where the port gives the newer result.
	versionCases, wrongVersion := 0, 0
	for i, c := range golden.Cases {
		if c.Error != nil {
			t.Fatalf("case %d: stylis threw %q, the generator must not keep such a case", i, *c.Error)
		}
		g := groups[c.Group]
		if g == nil {
			g = &count{}
			groups[c.Group] = g
		}
		g.total++

		var problems []string
		if c.AST != nil {
			if diff := compareTree(Compile(c.CSS), c.AST, true, "ast"); diff != "" {
				problems = append(problems, diff)
			}
		}
		if c.Plain != nil {
			got := Serialize(Compile(c.CSS), Stringify)
			if got != *c.Plain {
				problems = append(problems, fmt.Sprintf("serialize:\n got  %q\n want %q", got, *c.Plain))
			}
			if c.Newer != nil {
				versionCases++
				if *c.Newer == *c.Plain {
					t.Fatalf("case %d: the newer output is equal to the bundled output", i)
				}
				if got == *c.Newer {
					wrongVersion++
				}
			}
		}
		if got := CompileCSS(c.NS, c.CSS); got != *c.Out {
			problems = append(problems, fmt.Sprintf("compileCSS:\n got  %q\n want %q", got, *c.Out))
		}
		if len(problems) > 0 {
			g.failed++
			failures++
			if failures <= 15 {
				css := c.CSS
				if len(css) > 300 {
					css = css[:300] + "..."
				}
				t.Errorf("case %d (%s) ns=%q css=%q\n%s", i, c.Group, c.NS, css, problems[0])
			}
		}
	}
	for name, g := range groups {
		t.Logf("%-10s %5d cases, %d failed", name, g.total, g.failed)
	}
	t.Logf("%d cases have another output in stylis %s", versionCases, golden.Newer)
	if golden.Newer != bundledVersion && versionCases < 500 {
		t.Errorf("%d cases tell stylis %s from stylis %s, want at least 500", versionCases, bundledVersion, golden.Newer)
	}
	if wrongVersion > 0 {
		t.Errorf("%d cases have the output of stylis %s: the port must follow stylis %s", wrongVersion, golden.Newer, bundledVersion)
	}
	if failures > 0 {
		t.Errorf("%d of %d cases failed", failures, len(golden.Cases))
	}
}

// The port follows the stylis of the mermaid bundle and not the newer stylis
// of node_modules. Each case gives another result in stylis 4.4.0, which is in
// the comment. The expected values are those of the golden file.
func TestBundledVersion(t *testing.T) {
	cases := []struct{ css, want string }{
		// A custom property that has no value: 4.4.0 gives ".a{--x:;}".
		{".a { --x:; }", ""},
		// 4.4.0 gives ".a{--x:;fill:red;}".
		{".a { --x: ; fill: red }", ".a{fill:red;}"},
		// A comment just before the end makes the declaration in both versions.
		{".a { --x:/**/; }", ".a{--x:;}"},
		// A comment in a pseudo-class function: 4.4.0 gives ".a:not(.b){fill:red;}".
		{".a:not(/* c */.b) { fill: red }", ".a:not(/* c */.b){fill:red;}"},
		// Only the first "&" in the parentheses is the parent selector: 4.4.0
		// gives ".x:not(.x.y, .x.z){fill:red;}".
		{".x { &:not(&.y, &.z) { fill: red } }", ".x:not(.x.y, &.z){fill:red;}"},
		// 4.4.0 gives ".a:is(.x, .x .b){fill:red;}".
		{".x { .a:is(&, & .b) { fill: red } }", ".a:is(.x, & .b){fill:red;}"},
	}
	for _, c := range cases {
		if got := Serialize(Compile(c.css), Stringify); got != c.want {
			t.Errorf("%q:\n got  %q\n want %q", c.css, got, c.want)
		}
	}
}

// Stringify stores its output in Return and joins the selectors into Value,
// as stylis does.
func TestStringifySetsReturn(t *testing.T) {
	elements := Compile(".a,.b{fill:red;.c{stroke:blue}}")
	out := Serialize(elements, Stringify)
	if want := ".a,.b{fill:red;}.a .c,.b .c{stroke:blue;}"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
	if elements[0].Return != ".a,.b{fill:red;}" || elements[0].Value != ".a,.b" {
		t.Errorf("rule: return %q, value %q", elements[0].Return, elements[0].Value)
	}
	if decl := elements[0].Children[0]; decl.Return != "fill:red;" || decl.Prop != "fill" || decl.Text != "red" {
		t.Errorf("declaration: %+v", decl)
	}
}

func BenchmarkCompileCSS(b *testing.B) {
	f, err := os.Open("testdata/stylis.json.gz")
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		b.Fatal(err)
	}
	var g goldenFile
	if err := json.NewDecoder(zr).Decode(&g); err != nil {
		b.Fatal(err)
	}
	var css string
	for _, c := range g.Cases {
		if c.Group == "template" && len(c.CSS) > len(css) {
			css = c.CSS
		}
	}
	b.SetBytes(int64(len(css)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CompileCSS("#my-svg", css)
	}
}
