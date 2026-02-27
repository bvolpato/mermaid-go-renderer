package mermaid

import "testing"

func BenchmarkRenderFlowchart(b *testing.B) {
	diagram := "flowchart LR\nA[Start] --> B{Check}\nB -->|Yes| C[OK]\nB -->|No| D[Cancel]\nC --> E[Done]\nD --> E\n"
	benchmarkRender(b, diagram)
}

func BenchmarkRenderSequence(b *testing.B) {
	diagram := "sequenceDiagram\nparticipant Alice\nparticipant Bob\nparticipant API\nAlice->>Bob: Hello\nBob->>API: Fetch\nAPI-->>Bob: Data\nBob-->>Alice: Result\n"
	benchmarkRender(b, diagram)
}

func BenchmarkRenderState(b *testing.B) {
	diagram := "stateDiagram-v2\n[*] --> Init\nInit --> Running\nRunning --> Waiting\nWaiting --> Running\nRunning --> Done\nDone --> [*]\n"
	benchmarkRender(b, diagram)
}

func BenchmarkRenderClass(b *testing.B) {
	diagram := "classDiagram\nclass Animal {\n+int age\n+eat()\n}\nclass Dog {\n+bark()\n}\nAnimal <|-- Dog\n"
	benchmarkRender(b, diagram)
}

func BenchmarkRenderPie(b *testing.B) {
	diagram := "pie showData\ntitle Services\nAPI : 45\nWorker : 30\nDB : 15\nCache : 10\n"
	benchmarkRender(b, diagram)
}

func BenchmarkRenderXYChart(b *testing.B) {
	diagram := "xychart-beta\ntitle Throughput\nx-axis [Q1, Q2, Q3, Q4]\ny-axis 0 --> 200\nbar [50, 110, 160, 180]\nline [40, 90, 140, 170]\n"
	benchmarkRender(b, diagram)
}

func benchmarkRender(b *testing.B, diagram string) {
	options := DefaultRenderOptions().WithAllowApproximate(true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := RenderWithOptions(diagram, options)
		if err != nil {
			b.Fatalf("RenderWithOptions() error = %v", err)
		}
	}
}
