package raster

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// cpuTime returns the CPU time that the process has used. The benchmarks
// report it next to the wall time: on a busy machine the wall time says
// little.
func cpuTime() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

// benchFiles are the SVGs of the render benchmarks, in testdata/bench:
// diagrams of mermaid at their natural size.
func benchFile(b *testing.B, name string) string {
	b.Helper()
	path := os.Getenv("RASTER_BENCH_FILE")
	if path == "" {
		path = filepath.Join("testdata", "bench", name)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		b.Skipf("no benchmark file: %v", err)
	}
	return string(data)
}

func benchRender(b *testing.B, name string, scale float64) {
	svg := benchFile(b, name)
	// The first render fills the glyph cache.
	img, err := Render(svg, Options{Scale: scale})
	if err != nil {
		b.Fatal(err)
	}
	pixels := float64(img.Bounds().Dx() * img.Bounds().Dy())
	b.ReportAllocs()
	b.ResetTimer()
	start := cpuTime()
	for i := 0; i < b.N; i++ {
		if _, err := Render(svg, Options{Scale: scale}); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(cpuTime()-start)/float64(b.N)/1e6, "cpu-ms/op")
	b.ReportMetric(pixels, "pixels")
}

func BenchmarkRenderFlowchart(b *testing.B)   { benchRender(b, "flowchart.svg", 1) }
func BenchmarkRenderFlowchart2x(b *testing.B) { benchRender(b, "flowchart.svg", 2) }
func BenchmarkRenderSequence(b *testing.B)    { benchRender(b, "sequence.svg", 1) }
func BenchmarkRenderClass(b *testing.B)       { benchRender(b, "class.svg", 1) }
