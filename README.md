<div align="center">

# mmdg (`mermaid-go-renderer`)

**Fast native Mermaid rendering in Go. No browser, no Chromium.**

[![CI](https://github.com/bvolpato/mermaid-go-renderer/actions/workflows/ci.yml/badge.svg)](https://github.com/bvolpato/mermaid-go-renderer/actions/workflows/ci.yml)
[![Release](https://github.com/bvolpato/mermaid-go-renderer/actions/workflows/release.yml/badge.svg)](https://github.com/bvolpato/mermaid-go-renderer/actions/workflows/release.yml)

[Installation](#installation) | [Quick Start](#quick-start) | [Performance](#performance) | [Library Usage](#library-usage) | [Release and Homebrew](#release-and-homebrew)

</div>

## Why this project

`mmdg` is a pure Go Mermaid renderer for SVG output.

- Native execution (no browser process)
- Usable both as a library and CLI
- Supports Mermaid diagram families through native parsing and rendering
- Focused on low startup latency for local workflows and CI pipelines

## Inspiration

This project is inspired by the Rust implementation [`1jehuang/mermaid-rs-renderer`](https://github.com/1jehuang/mermaid-rs-renderer).

- **Original Mermaid CLI**: `mmdc` (`@mermaid-js/mermaid-cli`)
- **Rust renderer**: `mmdr` (`mermaid-rs-renderer`)
- **Go renderer**: `mmdg` (this repository)

## Performance

Yes - this README now includes a direct comparison against Mermaid CLI (`mmdc`) and the Rust renderer (`mmdr`).

Benchmark host:

- Apple M4 Max
- macOS darwin 25.1.0
- Go 1.26.0

Method:

- warm-up pass for each tool and fixture
- repeated CLI runs (`mmdg`: 20, `mmdr`: 20, `mmdc`: 5)
- measured wall-clock render command time

### Renderer stack comparison

| Tool | Role | Implementation | Runtime stack |
|:--|:--|:--|:--|
| `mmdc` | Original Mermaid CLI | JavaScript | Node.js + Puppeteer + headless Chromium |
| `mmdr` | Rust version | Rust | Native binary |
| `mmdg` | Our version | Go | Native binary |

### CLI render benchmark (`mmdc` vs `mmdr` vs `mmdg`)

| Diagram | `mmdc` avg | `mmdr` avg | `mmdg` avg | `mmdr` vs `mmdc` | `mmdg` vs `mmdc` |
|:--|--:|--:|--:|--:|--:|
| Flowchart | 2190.54 ms | 12.88 ms | 11.22 ms | 170.09x | 195.16x |
| Sequence | 3180.24 ms | 11.42 ms | 11.56 ms | 278.58x | 275.10x |
| Class | 2144.91 ms | 11.01 ms | 11.43 ms | 194.76x | 187.61x |
| State | 2173.91 ms | 10.99 ms | 11.84 ms | 197.72x | 183.66x |

Geometric mean speedup vs `mmdc`:

- `mmdr` (Rust): **206.68x**
- `mmdg` (Go): **207.39x**

On this run, `mmdg` and `mmdr` are effectively near parity overall.

### Library microbenchmarks (`go test -bench`)

| Benchmark | Time | Memory | Allocs |
|:--|--:|--:|--:|
| Flowchart | 52,149 ns/op | 21,794 B/op | 336 |
| Sequence | 41,225 ns/op | 26,373 B/op | 394 |
| State | 80,679 ns/op | 20,968 B/op | 306 |
| Class | 61,899 ns/op | 15,494 B/op | 250 |
| Pie | 27,593 ns/op | 10,626 B/op | 156 |
| XY Chart | 28,910 ns/op | 13,957 B/op | 242 |

Reproduce library benchmarks:

```bash
go test -run ^$ -bench BenchmarkRender -benchmem ./...
```

## Installation

### Build locally

```bash
git clone https://github.com/bvolpato/mermaid-go-renderer
cd mermaid-go-renderer
go build ./cmd/mmdg
```

### Install with `go install`

```bash
go install github.com/bvolpato/mermaid-go-renderer/cmd/mmdg@latest
```

## Quick Start

Render a Mermaid file to SVG:

```bash
mmdg -i diagram.mmd -o out.svg -e svg
```

Render from stdin:

```bash
echo 'flowchart LR; A-->B-->C' | mmdg -e svg
```

Render all Mermaid blocks from Markdown:

```bash
mmdg -i docs.md -o ./out -e svg
```

Useful flags:

- `--nodeSpacing`
- `--rankSpacing`
- `--preferredAspectRatio` (`16:9`, `4/3`, `1.6`)
- `--fastText`
- `--timing`

## Diagram support

Current parser and renderer paths detect and handle Mermaid families including:

- Flowchart, Sequence, Class, State, ER, Pie
- Mindmap, Journey, Timeline, Gantt, Git Graph
- XY Chart, Quadrant
- Requirement, C4, Sankey, ZenUML, Block, Packet, Kanban, Architecture, Radar, Treemap

## Library Usage

Simple API:

```go
package main

import (
	"fmt"

	mermaid "github.com/bvolpato/mermaid-go-renderer"
)

func main() {
	svg, err := mermaid.Render("flowchart LR\nA[Start] --> B{Decision}\nB --> C[Done]")
	if err != nil {
		panic(err)
	}
	fmt.Println(svg)
}
```

Pipeline API:

```go
parsed, _ := mermaid.ParseMermaid("flowchart LR\nA-->B")
layout := mermaid.ComputeLayout(&parsed.Graph, mermaid.ModernTheme(), mermaid.DefaultLayoutConfig())
svg := mermaid.RenderSVG(layout, mermaid.ModernTheme(), mermaid.DefaultLayoutConfig())
```

## Architecture

Native rendering pipeline:

```
.mmd -> parser -> IR graph -> layout -> SVG renderer
```

No browser process is spawned.

## Testing

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

## Release and Homebrew

GoReleaser and CI are configured:

- `.goreleaser.yaml` (schema version 2)
- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`

Release workflow on tag (`v*`) does:

- cross-platform binary builds
- archives + checksums
- GitHub Release publishing
- Homebrew formula updates in `bvolpato/homebrew-tap`

Required GitHub secret:

- `HOMEBREW_TAP_GITHUB_TOKEN` (token with write access to the tap repository)
