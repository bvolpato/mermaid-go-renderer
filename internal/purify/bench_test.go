package purify

import (
	"encoding/json"
	"testing"
)

// BenchmarkSanitizeSVG sanitizes the reference SVGs of the golden with the
// configuration of mermaidAPI.ts.
func BenchmarkSanitizeSVG(b *testing.B) {
	var golden struct {
		Config json.RawMessage `json:"config"`
		Cases  []struct {
			In string `json:"in"`
		} `json:"cases"`
	}
	readGolden(b, "svg_clean.json.gz", &golden)
	cfg, err := ConfigFromJS(parseConfig(b, golden.Config))
	if err != nil {
		b.Fatal(err)
	}
	total := 0
	for _, c := range golden.Cases {
		total += len(c.In)
	}
	b.SetBytes(int64(total))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, c := range golden.Cases {
			Sanitize(c.In, cfg)
		}
	}
}

// BenchmarkSanitizeLabel sanitizes a short label, as sanitizeText does for
// every label of a diagram.
func BenchmarkSanitizeLabel(b *testing.B) {
	cfg := Config{ForbidTags: []string{"style"}}
	p := New()
	b.Run("plain", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			p.Sanitize("Start of the process", cfg)
		}
	})
	b.Run("markup", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			p.Sanitize(`Line 1<br/>Line <b class="x">2</b>`, cfg)
		}
	})
}
