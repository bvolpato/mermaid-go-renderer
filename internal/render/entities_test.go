package render

import (
	"math/rand"
	"strings"
	"testing"
)

// TestStripMarkerEndURL compares stripMarkerEndURL with the regular
// expression of cleanUpSvgCode, on texts that are made of the parts that the
// expression looks at.
func TestStripMarkerEndURL(t *testing.T) {
	parts := []string{
		`marker-end="url(`, `marker-end="url(#`, `marker-end="url`, `marker-end=`, `marker-`, `m`, `#`, `"`, `(`, `)`,
		`http://localhost:8080/page.html?a=1+2`, `abc`, `-`, `.`, `/`, `:`, `=`, `?`, `+`, `0`, ` `, `_`, `%`, `é`,
		`<path marker-start="url(#x)" `, `id`, "\n",
		`marker-end="url(`, `marker-end="url(`, `marker-end="url(`, `#`, `#`, `marker-end="url(http://a/b#`,
		`marker-end="url(x`, `marker-end="url(marker-end="url(`,
	}
	rng := rand.New(rand.NewSource(1))
	changed := 0
	for i := 0; i < 20000; i++ {
		var sb strings.Builder
		for n := rng.Intn(12); n > 0; n-- {
			sb.WriteString(parts[rng.Intn(len(parts))])
		}
		in := sb.String()
		want := reMarkerEnd.ReplaceAllString(in, `marker-end="url(#`)
		got := stripMarkerEndURL(in)
		if got != want {
			t.Fatalf("stripMarkerEndURL(%q) = %q, want %q", in, got, want)
		}
		if got != in {
			changed++
		}
	}
	if changed < 2000 {
		t.Errorf("only %d texts changed", changed)
	}
}

// TestEncodeEntities checks the case that EncodeEntities returns early: a
// text without '#'.
func TestEncodeEntities(t *testing.T) {
	for _, in := range []string{"", "flowchart TD\n A --> B", "style A fill:red;", "classDef x fill:red;"} {
		if got := EncodeEntities(in); got != in {
			t.Errorf("EncodeEntities(%q) = %q", in, got)
		}
	}
	for in, want := range map[string]string{
		"A[#35;]":               "A[ﬂ°°35¶ß]",
		"A[#quot;]":             "A[ﬂ°quot¶ß]",
		"style A fill:#f9f;":    "style A fill:#f9f",
		"classDef x fill:#f9f;": "classDef x fill:#f9f",
	} {
		if got := EncodeEntities(in); got != want {
			t.Errorf("EncodeEntities(%q) = %q, want %q", in, got, want)
		}
	}
}
