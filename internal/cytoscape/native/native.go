// Package native lays out graphs with Go ports of the two layouts that
// mermaid 12.2.0 takes from cytoscape: cose-bilkent for a mindmap and fcose
// for an architecture diagram. The result is the result of the JavaScript
// (cytoscape 3.34.0 with cytoscape-cose-bilkent 4.1.0 and cytoscape-fcose
// 2.2.0) in Chromium, bit for bit, for the inputs that the ports cover.
//
// For each other input an entry point returns a reason and no result, and
// the caller runs the JavaScript (package internal/cytoscape does). An entry
// point never returns a result that is not exact.
//
// The ported code is under the MIT license of its sources. Each file names
// the source file, the project and the version that it ports.
package native

// Env is the part of the page that a layout reads.
type Env struct {
	// Random is Math.random.
	Random func() float64
	// MeasureText is context.measureText(text).width of a canvas context
	// with the font property font.
	MeasureText func(font, text string) float64
	// Trace receives the state of the layout at the points that the oracle
	// of the tests records. nil for a normal run.
	Trace func(stage string, values []float64)
	// Cancelled tells that the caller does not wait for the result. A layout
	// asks from time to time and then stops with a reason. nil for a layout
	// that always runs to its end.
	Cancelled func() bool
}
