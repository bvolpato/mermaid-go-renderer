<div align="center">

<img src="assets/logo.svg" alt="mmdg logo" width="260"/>

# mmdg

**Mermaid diagrams in pure Go. No browser. The same SVG as mermaid, byte for byte.**

[![CI](https://github.com/bvolpato/mermaid-go-renderer/actions/workflows/ci.yml/badge.svg)](https://github.com/bvolpato/mermaid-go-renderer/actions/workflows/ci.yml)

</div>

`mmdg` is a Go library and a command-line tool that render [Mermaid](https://mermaid.js.org) diagrams to SVG and PNG. It is a port of mermaid 12.2.0: the same parsers, the same layout engines and the same markup, with text measurement that reproduces Chromium. It does not start a browser and it has no cgo.

## Why

Mermaid needs a browser. `mermaid-cli` starts headless Chromium for every run. That is slow, heavy in a container and hard to call from a Go service.

A renderer without a browser can replace mermaid only if its output is the same. A diagram that looks similar is not enough, because a small difference in one text width moves nodes and edges. So this project has one measure: the SVG text must have the same bytes as the SVG that `mermaid.render()` returns in Chromium.

## Result

| Corpus | Diagrams | Pass | Byte-exact | Same error as mermaid |
|---|---:|---:|---:|---:|
| The end-to-end tests of mermaid | 2145 | 2141 (99.8%) | 2076 | 65 |
| The examples of the mermaid documentation | 596 | 594 (99.7%) | 576 | 18 |
| Generated large diagrams, 15 to 150 nodes | 70 | 70 (100%) | 70 | 0 |

- "Pass" means byte-exact output, or an error for a diagram that mermaid also rejects.
- The 6 diagrams that do not pass use KaTeX math, which is not supported.
- The ports were developed against the first corpus only. The other two show that the result holds for diagrams that the code never saw.
- Each corpus passes with the same counts against two references: Chromium 156 on Linux with the Liberation fonts, and Chromium 156 on macOS with the system fonts.

You can check this yourself. The repository has the SHA-256 of every reference SVG, and one test compares them:

```sh
go test -run TestCorpusParity .
```

The tables by diagram type are in [docs/PARITY.md](docs/PARITY.md), [docs/PARITY-DOCS.md](docs/PARITY-DOCS.md) and [docs/PARITY-LARGE.md](docs/PARITY-LARGE.md) for the Linux reference, and in [docs/PARITY-MACOS.md](docs/PARITY-MACOS.md) for the macOS reference. [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) explains how the references are made and how you can make them again.

The test also runs in a container with no fonts and no browser, because the default fonts are part of the module.

## Performance

| Measurement | mermaid in Chromium | mmdg | Ratio |
|---|---:|---:|---:|
| One render in a warm process, median of 2076 diagrams | 11.2 ms | 0.89 ms | 13x |
| The same, 90th percentile | 26.9 ms | 2.55 ms | 11x |
| Command-line run, flowchart to SVG (`mmdc` and `mmdg`) | 762 ms | 28.5 ms | 27x |
| Command-line run, flowchart to PNG | 850 ms | 50.3 ms | 17x |

- The first two rows compare `mermaid.render()` in a Chromium page that stays open with `mermaid.Render` in a Go process that stays running. Both sides are warm.
- The last two rows compare whole processes. `mmdc` starts a browser in each run.
- All numbers are from one machine. [docs/PERFORMANCE.md](docs/PERFORMANCE.md) has the numbers for each diagram type and says how they were measured.

## Install

```sh
go get github.com/bvolpato/mermaid-go-renderer
go install github.com/bvolpato/mermaid-go-renderer/cmd/mmdg@latest
```

## Use the library

```go
import mermaid "github.com/bvolpato/mermaid-go-renderer"

svg, err := mermaid.Render("flowchart LR\n  A[Start] --> B{Done?}\n  B -->|yes| C[Stop]")

png, err := mermaid.RenderPNG(src, mermaid.WithScale(2))
```

Config works as in mermaid. Front matter and directives in the diagram text apply, and a site config stands for the argument of `mermaid.initialize`:

```go
svg, err := mermaid.Render(src,
	mermaid.WithConfigJSON([]byte(`{"theme": "dark", "look": "classic"}`)),
	mermaid.WithID("diagram-1"),
)
```

| Option | Effect |
|---|---|
| `WithConfigJSON`, `WithConfig`, `WithTheme` | The site config of mermaid. |
| `WithID` | The id of the `svg` element. The default is `my-svg`, as in mermaid-cli. |
| `WithViewportWidth` | The width of the page that the layout assumes. The default is 800 px. |
| `WithTime`, `WithRandomSeed` | The clock and the random numbers. Both have fixed defaults. |
| `WithSystemFonts`, `WithFontDirs` | The fonts that measure text. See below. |
| `WithImageSize` | The natural size of images that a diagram names by URL. |
| `WithScale`, `WithBackground`, `WithBackgroundColor` | The pixel density and the background of PNG output. `WithBackgroundColor` takes a CSS color and also applies to `StandaloneSVG`. |

For a service:

- `Render` is safe for concurrent use. A test renders the corpus from many goroutines with the race detector.
- The result depends only on the source and the options. The clock and the random numbers are fixed unless you set them.
- `RenderContext` stops when its context is done. Use a deadline for input that you do not trust.
- A defect in the renderer returns as an `*InternalError`. It does not stop your program.
- `StandaloneSVG` turns the result into a file that an image viewer or an XML parser reads. The string that `Render` returns is HTML markup, as in mermaid: a `<br>` in a label has no end tag.
- The renderer never reads from the network. An image in a label needs `WithImageSize`, or a data URL.

## Use the command line

`mmdg` takes the options of `mmdc` (mermaid-cli 12, and the options that mermaid-cli 11 had):

```sh
mmdg -i diagram.mmd -o diagram.svg
mmdg -i diagram.mmd -o diagram.png -s 2
mmdg -i README.md -o out.svg                    # one file for each mermaid block
mmdg -i README.md -o README.out.md -a images    # Markdown with images in place of the blocks
cat diagram.mmd | mmdg -i - -o - -e svg > diagram.svg
```

It writes the files that `mmdc` writes: the SVG as `XMLSerializer` prints it, with the background color, and a PNG with the size of the screenshot of `mmdc`. The messages and the exit codes are those of `mmdc`.

[docs/CLI.md](docs/CLI.md) has every option, what was compared with real `mmdc` runs, and what differs. PDF output and icon packs are not supported.

## Fonts and reproducible output

Mermaid measures text to lay out a diagram, so the fonts decide the result.

By default `mmdg` measures with fonts that are part of the module: Liberation Sans, Serif and Mono, which have the metrics of Arial, Times New Roman and Courier New. It applies the rules of Chromium on Linux. It reads no font file, so it gives the same SVG on a laptop and in a container that has no fonts. The reference for this mode is mermaid in Chromium on Linux with only the Liberation fonts installed.

- `WithFontDirs` adds font files for text that the Liberation fonts have no glyphs for, such as Chinese, Japanese, Korean and emoji.
- `WithSystemFonts` measures with the fonts of the machine, by the rules of Chromium on that operating system. The result then matches mermaid in a browser on the same machine. It is verified on macOS.

## Diagram types

All diagram types of mermaid 12.2.0 render: flowchart, sequence, class, state, entity relationship, requirement, gantt, pie, quadrant, xychart, gitGraph, journey, timeline, mindmap, C4, sankey, block, packet, radar, treemap, kanban, architecture, venn, ishikawa, cynefin, wardley, treeView, usecase, swimlane, agentflow, railroad (four syntaxes), eventmodeling and info.

All layouts are supported: dagre, ELK, cose-bilkent, fcose and swimlanes. All looks (`classic`, `neo`, `handDrawn`) and all themes are supported.

Not supported:

- KaTeX math in labels (`$$...$$`). Mermaid lets the browser lay out MathML.
- The `zenuml` diagram, which is an external plugin of mermaid.
- `securityLevel: sandbox`, which renders into an iframe.
- Click callbacks. They need a live page and do not change the SVG.

## How it works

Each package under `internal/` is a line-by-line port of the code that produces the result in the browser, and each has tests that compare it with that code on recorded inputs, bit for bit. The main parts:

- The diagram code of mermaid and its parsers.
- The layout engines: dagre, the layered algorithm of Eclipse ELK, and cytoscape for mindmap and architecture.
- A small browser engine for measurement: DOM, CSS cascade, font matching, HarfBuzz shaping with the arithmetic of Blink, the float geometry of Skia.
- The JavaScript semantics that decide bits: number formatting, object key order, the sort of V8, and `Math` functions with the results of Chromium.
- A rasterizer for PNG output that ports the software renderer of Skia.

Read [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the details.

## Development

```sh
go test ./...                          # package tests against recorded goldens
go test -run TestCorpusParity .        # the corpus against the reference hashes
go run ./cmd/parity                    # the corpus against reference SVG files, with the first difference
go run ./tools/fmacheck ./internal/... # no fused multiply-add in ported code
```

The reference SVG files come from `tools/reference/oracle.mjs`, which runs mermaid 12.2.0 in headless Chromium, and from `tools/reference/oracle-linux.mjs`, which does the same in a Linux container. Only these tools need node and a browser. The library and its tests do not.

## Licenses

This repository has no license file of its own yet. The owner has to choose one.

The code is a port of mermaid and of the libraries and browser code that produce its output, so most packages are derivative works and keep the terms of their source:

- Most sources are under MIT, ISC, BSD or Apache-2.0 terms: mermaid, d3, dagre, cytoscape, roughjs, DOMPurify, Skia, V8 and most of Chromium.
- `internal/elk` contains elkjs and a port of the layered algorithm of Eclipse ELK, which are under EPL-2.0.
- Parts of `internal/browser`, `internal/raster`, `internal/dom` and `internal/css` follow SVG, layout and CSS source files of Blink that are under LGPL-2.0-or-later. Every render uses this code.
- The embedded Liberation fonts are under the SIL Open Font License 1.1.

[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) has the copyright lines and the license texts. [docs/PROVENANCE.md](docs/PROVENANCE.md) says which package comes from which source. [docs/LICENSE-AUDIT.md](docs/LICENSE-AUDIT.md) has the findings of the audit and the decisions that are open.
