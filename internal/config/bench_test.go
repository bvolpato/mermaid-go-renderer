package config

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

func BenchmarkResolve(b *testing.B) {
	text := "---\ntitle: Demo\nconfig:\n  theme: forest\n---\nflowchart TD\n  A --> B\n"
	site := js.NewObj[any]()
	site.Set("startOnLoad", false)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Resolve(text, site); err != nil {
			b.Fatal(err)
		}
	}
}
