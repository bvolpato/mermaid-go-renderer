package marked

import "testing"

var benchInputs = map[string]string{
	"plain":    "Hello world",
	"emphasis": "This is **bold** and *italic* text",
	"long":     "The quick brown fox jumps over the lazy dog, again and again, until the label is long enough to wrap twice.",
}

func BenchmarkLexer(b *testing.B) {
	for name, in := range benchInputs {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Lexer(in); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
