# Architecture

This document explains how the renderer is built and why. Read it before you change a package under `internal/`.

## The goal decides the design

The goal is the same SVG text that `mermaid.render()` returns in Chromium, byte for byte. A renderer that only looks similar cannot replace mermaid in a pipeline, because every small difference in a text width or a rounding step moves nodes and edges.

Two rules follow from that goal.

1. **Port, do not write again.** Each package is a line-by-line port of the JavaScript, Java or C++ code that produces the result in the browser. It keeps the same algorithm, the same order of operations, the same iteration order and the same floating-point expressions. Each ported file names its source in the first comment.
2. **Prove each part against the real code.** Each package has tests that compare it with the code it ports, on recorded inputs and outputs. The comparison is exact: numbers are compared by their bits and strings by their bytes.

## What a render does

`mermaid.Render` calls `internal/render`, which follows `mermaidAPI.render` of mermaid.

1. `internal/config` reads the front matter and the directives, merges them with the site config and the defaults, and finds the diagram type.
2. `internal/render` builds the page that mermaid builds in a browser: `body > div#container > div > svg > g`. The page is a small DOM (`internal/dom`).
3. The package of the diagram type (`internal/diagrams/<type>`) parses the text into its database. Grammars that mermaid writes in jison run on the tables that jison generates (`internal/grammar`, `internal/jison`). Grammars that mermaid writes in Langium have hand-written parsers on a shared lexer (`internal/langium`).
4. `internal/styles` and `internal/theme` produce the style sheet. `internal/stylis` compiles it, as mermaid does.
5. The diagram draws into the DOM. It appends elements, sets attributes and asks for sizes (`getBBox`, `getBoundingClientRect`, `getComputedTextLength`), exactly where the upstream code does.
6. `internal/render` serializes the SVG and passes it through `internal/purify`, the port of DOMPurify.

## The measurement engine

Mermaid measures text and shapes in the browser while it draws. The sizes decide the layout, so the engine must return the numbers that Chromium returns.

- `internal/dom` has the DOM tree, selectors, serialization and the CSSOM behavior of `element.style`. Its methods follow d3-selection, so that renderer code ports line by line.
- `internal/css` parses and prints CSS values as Blink does.
- `internal/font` finds fonts, reads their metrics and shapes text with HarfBuzz (the Go port in `go-text/typesetting`), with the arithmetic of Blink: advances in 16.16 fixed point and layout units of 1/64 px.
- `internal/browser` computes styles (the cascade), lays out SVG text and HTML inside `foreignObject`, and computes bounding boxes with the 32-bit float geometry of Blink and Skia.

Each rule in these packages comes from the Chromium source and has a probe: a set of cases that ran in Chromium, stored under `testdata/`, with a test that replays them.

## JavaScript semantics in Go

`internal/js` has what JavaScript does and Go does differently.

- Number to string conversion, `Math.round`, `toFixed`, `parseInt` and `parseFloat`.
- Objects that list their keys as JavaScript does: integer-like keys first, then insertion order. `Map` and `Set` with insertion order.
- The sort of V8, because some comparators of upstream code are not consistent.
- The `Math` functions with the bits of Chromium (`internal/js/crmath`). Go's `math` package differs in the last bit for 1% to 48% of arguments, depending on the function.
- Go on arm64 fuses `x*y + z` into one instruction, which rounds one time. JavaScript does not. Ported code writes `float64(x*y) + z`, and `tools/fmacheck` asks the compiler for every place where it would fuse.

`internal/render/async.go` reproduces the order in which a browser runs `Promise.all`, `await` and `fastdom.measure`. The order decides which shape gets which numbers of `Math.random`, so it shows in the output.

## Layout

- `internal/dagre` and `internal/graphlib` are ports of dagre-d3-es.
- `internal/elk/native` is a port of the layered algorithm of Eclipse ELK, with the semantics of the GWT build that elkjs ships. It returns the same bytes as elkjs. `internal/elk` also embeds elkjs and runs it in the JavaScript interpreter goja for graphs that the port does not cover: other ELK algorithms and rare options.
- `internal/cytoscape/native` is a port of the cose-bilkent and fcose layouts of cytoscape, for mindmap and architecture diagrams. It returns the same bytes as the JavaScript. `internal/cytoscape` also embeds the genuine cytoscape code and runs it in goja for a graph that the port does not cover.
- The environment variables `MMDG_ELK` and `MMDG_CYTOSCAPE` select the engine: `native` (the default), `goja`, or `check`, which runs both and reports a difference.
- `internal/ru` is the unified renderer of mermaid (`rendering-util`): shapes, clusters, edges, markers, labels and the layout glue for dagre, ELK, cose-bilkent and swimlanes.
- `internal/d3` has the d3 modules that the charts use. `internal/rough` is roughjs, for the hand-drawn look. `internal/venn` is the layout of venn diagrams.

## PNG output

`internal/raster` paints the SVG into a bitmap. It ports the parts of Skia that Chromium uses in software rendering: the analytic anti-aliasing scan converter, the stroker and the blitters. It gets text and HTML layout from `internal/browser`.

## How parity is proven

`tools/reference/oracle.mjs` runs mermaid 12.2.0 in headless Chromium and writes the string that `mermaid.render()` returns for each diagram of a corpus. The page has a fixed clock, a seeded `Math.random` and a fixed viewport, and the Go renderer uses the same values by default.

- `go run ./cmd/parity` renders a corpus and compares each SVG with the reference: byte-exact, equivalent (same tree, numbers within 0.01), different, or an error on both sides.
- `testdata/reference/*.json` are manifests: the SHA-256 of each reference SVG. The test `TestCorpusParity` renders every diagram and compares the hashes, so the proof runs without the reference files. It fails when a diagram that passed stops passing.
- `docs/PARITY*.md` are the tables that `tools/paritydoc` writes from the manifests.

There are two reference environments, and each corpus has a manifest for both.

| Environment | Fonts | Where the test runs |
|---|---|---|
| Chromium 156 on Linux in the Playwright image, with only the Liberation fonts installed (`tools/reference/oracle-linux.mjs --fonts liberation`) | The fonts that are embedded in the module, which is the default | On every machine, also in a container with no fonts |
| Chromium 156 on macOS (`tools/reference/oracle.mjs`) | The system fonts (`WithSystemFonts`) | On a Mac whose fonts are those of the reference machine. On other machines the test skips these manifests. |

There are three corpora under `testdata/corpus`.

| Corpus | Diagrams | Source |
|---|---:|---|
| `mermaid-e2e` | 2145 | The end-to-end tests of mermaid. The ports were developed against it. |
| `mermaid-docs` | 596 | The examples of the mermaid documentation. No port was developed against it. |
| `mermaid-large` | 70 | Generated diagrams with 15 to 150 nodes, for the layout paths that small graphs do not reach. |

## Determinism and concurrency

A render depends only on the source text and the options. The clock, the random numbers and the viewport have fixed defaults. Upstream keeps state in module variables. Here each render has its own `render.Context`, so renders can run at the same time. `TestConcurrentRendersAreIdentical` checks that, also with the race detector.

Renders share bounded caches (`internal/memo`) of values that depend only on their key: config templates, theme variables, compiled CSS, parsed rule sets and shaped texts. A cached value is read-only, and each cache has a size limit. `TestCachesDoNotChangeRenders` renders with the caches off, then from several goroutines with the caches on, and compares the output.

`RenderContext` stops a render when its context is done. A panic in the renderer returns as an `InternalError`.

## What is not ported

- KaTeX math in labels. Mermaid lets the browser lay out MathML, which needs a MathML layout engine.
- The `zenuml` diagram, which is an external plugin of mermaid.
- `securityLevel: sandbox`, which renders into an iframe.
- Click callbacks and tooltips that need a live page. They do not change the SVG.

## Test data and the size of the module

The recorded results of the real code are most of the repository, about 110 MB. Each `testdata` directory has a `go.mod` file, because the go command leaves a directory with a `go.mod` file out of the module that contains it. So a program that depends on this module downloads about 9 MB, without the test data. The tests read their files from a clone of the repository.

A new `testdata` directory needs such a file. Copy one and change the module path. The test `TestTestDataIsNotInTheModule` fails when one is missing. A test cannot embed a file of its test data with `go:embed`. It must read the file.

## Rules for a change

1. Find the upstream code first. Port it, with the source named in the file header.
2. Add cases from the real code to the test data of the package. Compare exactly.
3. Run `go run ./tools/fmacheck ./internal/<package>/...`, and the tests with and without `-gcflags=all=-d=fmahash=n`.
4. Run `go test -run TestCorpusParity .`. No diagram may stop passing.
