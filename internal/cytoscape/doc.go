// Package cytoscape runs the graph layouts that mermaid 12 takes from
// cytoscape.js: cose-bilkent for mindmaps and fcose for architecture
// diagrams.
//
// The package has two implementations of each layout, which give the same
// bits:
//
//   - The JavaScript that mermaid bundles (cytoscape 3.34.0,
//     cytoscape-cose-bilkent 4.1.0 with cose-base 1.0.3 and layout-base
//     1.0.2, cytoscape-fcose 2.2.0 with cose-base 2.2.0 and layout-base
//     2.0.1), in goja, a JavaScript engine in Go. The script
//     cytoscape.bundle.js holds that code unchanged.
//     tools/reference/gen-cytoscape-bundle.mjs builds it from the mermaid
//     build, and LICENSES.txt has the licenses.
//   - Go ports of the code paths that the two uses of mermaid reach
//     (internal/cytoscape/native). They take a few hundred times less CPU
//     time than the interpreter.
//
// A port covers the inputs of mermaid diagrams. For each other input it
// reports that it does not cover the input, and the call runs the
// JavaScript: a call never returns a result that is not exact. Env.Mode, or
// the environment variable MMDG_CYTOSCAPE with the values native, goja and
// check, selects the path. The default is native. The mode check runs both
// and fails with a *CheckError on any difference in bits. ReadStats counts
// the layouts by path, with the reasons of the ports.
//
// # What mermaid does, and what the package repeats
//
// mermaid does not use cytoscape in headless mode. It creates the instance
// with a container element that has the style display:none, and removes the
// container from the document right after the constructor. So cytoscape
// runs with styles and with its canvas renderer, and both matter for the
// result:
//
//   - The layouts read the padding of a node from its style.
//   - The renderer computes the geometry of the edges (rscratch, the
//     endpoints and the midpoint), which mermaid reads after the layout. A
//     renderer computes it only when something asks for a bounding box or an
//     endpoint: no frame is drawn before mermaid reads.
//   - The removal of the container makes a mutation observer destroy the
//     renderer. The observer runs as a microtask, after the layout and before
//     mermaid continues. From then on the renderer computes nothing, so a
//     read returns the state of that moment.
//
// env.js is the browser environment for this: a window, a document, the
// container, a canvas that measures text through Env.MeasureText, mutation
// observers, and timers that wait. glue.js has the statements of the mermaid
// functions (createCytoscapeInstance of the cose-bilkent layout,
// layoutArchitecture of the architecture renderer) in their order, the
// microtask, and the reads.
//
// # Exact numbers
//
// The numbers are the numbers of Chromium 156, bit for bit. The golden test
// compares every mindmap and every architecture fixture of the corpus with
// a record of the real layout in the browser, and the tests of the ports
// compare each port with the JavaScript at stage boundaries of the layouts
// (testdata/trace.js has the hooks). For the JavaScript in the interpreter:
//
//   - Each Math function that is not exact by definition is the function of
//     the package js, which has the results of the browser. goja would
//     compute them with the Go math package.
//   - Array.prototype.sort with a comparator is the sort of V8 (js.SortV8).
//   - Math.random is Env.Random and the clock is Env.Now.
//   - Each call runs in a new JavaScript realm. cytoscape and the layouts
//     keep state in module variables (layout-base has a random seed that
//     never resets), and the reference output of mermaid is the output of a
//     new page. A realm is cheap next to a layout: see the benchmarks.
//
// A call is safe for concurrent use. The scripts are compiled one time for
// the process (Warm does it ahead of use). A call ends with an error when
// its context ends, when DefaultTimeout passes, or when the script throws.
// The number of runtimes at one time is bounded by GOMAXPROCS: a runtime
// holds some tens of megabytes while its layout runs.
//
// goja interprets the script, so a layout of the JavaScript costs much more
// CPU time than in V8: about 0.05 s for a mindmap with one node, 0.1 to 0.2 s
// for 10 to 30 elements, and 1 to 3 s for the largest diagrams of the corpus
// (30 to 60 elements). The Go ports take about 0.1 ms to 20 ms for the same
// layouts.
package cytoscape
