package config

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// bundledEsToolkit is the version of es-toolkit inside the mermaid 12.2.0
// bundle. The package in node_modules is newer (1.53.1).
const bundledEsToolkit = "1.45.1"

// The golden files must come from the es-toolkit of the mermaid bundle.
// tools/reference/gen-config.mjs records the version in each file.
func TestGoldenEsToolkitVersion(t *testing.T) {
	for _, name := range []string{"api.json.gz", "corpus.json.gz", "defaults.json.gz", "site.json.gz", "text.json.gz", "yaml.json.gz"} {
		var g struct{ EsToolkit string }
		readGolden(t, name, &g)
		if g.EsToolkit != bundledEsToolkit {
			t.Errorf("%s comes from es-toolkit %q, want %s, the version in the mermaid bundle", name, g.EsToolkit, bundledEsToolkit)
		}
	}
}

// The merge follows es-toolkit 1.45.1 and not 1.53.1. 1.45.1 reads a null
// element of a source array as undefined, so the element does not replace a
// value of the target. The expected values are results of the bundled
// es-toolkit: gen-config.mjs checks the first one before it writes a file, and
// the text cases of the golden file have the same inputs as front matter and
// directives.
func TestCleanAndMergeBundledVersion(t *testing.T) {
	parse := func(text string) any {
		v, err := js.ParseJSON([]byte(text))
		if err != nil {
			t.Fatal(err)
		}
		return revive(v)
	}
	cases := []struct{ a, b, want string }{
		// 1.53.1 gives {"a":[null,3]}.
		{`{"a":[1,2]}`, `{"a":[null,3]}`, `{"a":[1,3]}`},
		// 1.53.1 gives {"a":[null]}.
		{`{}`, `{"a":[null]}`, `{"a":[{"$u":1}]}`},
		// The first source is an array source too. 1.53.1 gives {"a":[null,1]}.
		{`{"a":[null,1]}`, `{}`, `{"a":[{"$u":1},1]}`},
		// 1.53.1 gives {"a":[null,{"b":[null,4]}]}.
		{`{"a":[1,{"b":[2,3]}]}`, `{"a":[null,{"b":[null,4]}]}`, `{"a":[1,{"b":[2,4]}]}`},
		// A null that is not in an array replaces the value in both versions.
		{`{"a":[1]}`, `{"a":null}`, `{"a":null}`},
	}
	for _, c := range cases {
		got, err := cleanAndMerge(parse(c.a), parse(c.b))
		if err != nil {
			t.Errorf("cleanAndMerge(%s, %s): %v", c.a, c.b, err)
			continue
		}
		if d := diff("result", got, parse(c.want)); d != "" {
			t.Errorf("cleanAndMerge(%s, %s): %s", c.a, c.b, d)
		}
	}
}
