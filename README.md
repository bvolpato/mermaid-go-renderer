<div align="center">

# mmdg (`mermaid-go-renderer`)

**Fast native Mermaid rendering in Go. No browser, no Chromium.**

[![CI](https://github.com/bvolpato/mermaid-go-renderer/actions/workflows/ci.yml/badge.svg)](https://github.com/bvolpato/mermaid-go-renderer/actions/workflows/ci.yml)
[![Release](https://github.com/bvolpato/mermaid-go-renderer/actions/workflows/release.yml/badge.svg)](https://github.com/bvolpato/mermaid-go-renderer/actions/workflows/release.yml)

[Installation](#installation) | [CLI Usage](#cli-usage) | [Fidelity](#fidelity-mmdc-first) | [Performance](#performance) | [Library Usage](#library-usage) | [Release and Homebrew](#release-and-homebrew)

</div>

## Why this project

`mmdg` is a pure Go Mermaid renderer for SVG and PNG output.

- Native execution (no browser process)
- Native SVG and PNG output (PNG rasterized in pure Go)
- Usable both as a library and CLI
- Supports Mermaid diagram families through native parsing and rendering
- Focused on low startup latency for local workflows and CI pipelines

## Inspiration

This project is inspired by the Rust implementation [`1jehuang/mermaid-rs-renderer`](https://github.com/1jehuang/mermaid-rs-renderer).

- **Original Mermaid CLI**: `mmdc` (`@mermaid-js/mermaid-cli`)
- **Rust renderer**: `mmdr` (`mermaid-rs-renderer`)
- **Go renderer**: `mmdg` (this repository)

## Performance

This README includes direct performance and fidelity benchmarking against Mermaid CLI (`mmdc`).

Benchmark host:

- Apple M4 Max
- macOS darwin 25.1.0
- Go 1.26.0
- Date: 2026-02-26

## Fidelity (mmdc-first)

`mmdg` fidelity is measured against **`mmdc` output as ground truth**.

Approach:

1. Render paired outputs for the same fixtures:
   - `mmdc` -> `/tmp/{name}_mmdc.svg`
   - `mmdg` -> `/tmp/{name}_mmdg.svg`
2. Compute XML deltas (not only visual deltas) with:
   - `python3 scripts/svg_xml_delta.py`
3. Track three parity gates per fixture:
   - `tag_delta`
   - `attr_presence_delta`
   - `text_delta`
4. Prioritize fixtures with largest structural deltas first.

Reproduce the full fidelity sweep:

```bash
./scripts/render_all_components_to_tmp.sh
python3 scripts/svg_xml_delta.py
```

Latest XML fidelity snapshot (`/tmp/svg_xml_delta_report.csv`):

- Fixtures compared: **23**
- Exact parity (`tag_delta=0`, `attr_presence_delta=0`, `text_delta=0`): **8 / 23**
- Structural parity (`tag_delta=0`, `attr_presence_delta=0`): **8 / 23**

Exact-parity fixtures:

- `architecture_complex`, `block_complex`, `gantt_complex`, `mindmap_complex`
- `radar_complex`, `sankey_complex`, `sequence_complex`, `treemap_complex`

Current highest-delta fixtures (XML-first backlog):

- `flowchart_complex` (`tag=55`, `attr=272`, `text=5`)
- `gitgraph_complex` (`tag=24`, `attr=268`, `text=2`)
- `c4_context_complex` (`tag=47`, `attr=181`, `text=23`)
- `requirement_complex` (`tag=73`, `attr=149`, `text=18`)

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
| Flowchart | 2450.55 ms | 18.85 ms | 13.94 ms | 129.99x | 175.78x |
| Sequence | 2356.64 ms | 32.60 ms | 15.56 ms | 72.30x | 151.46x |
| Class | 2594.45 ms | 18.47 ms | 15.76 ms | 140.47x | 164.61x |
| State | 2623.20 ms | 20.41 ms | 18.18 ms | 128.51x | 144.27x |

Geometric mean speedup vs `mmdc`:

- `mmdr` (Rust): **114.13x**
- `mmdg` (Go): **158.58x**

On this run, `mmdg` is faster than `mmdr` on these representative fixtures.

### Library microbenchmarks (`go test -bench`)

| Benchmark | Time | Memory | Allocs |
|:--|--:|--:|--:|
| Flowchart | 121,343 ns/op | 44,314 B/op | 418 |
| Sequence | 54,838 ns/op | 46,253 B/op | 206 |
| State | 158,538 ns/op | 62,489 B/op | 302 |
| Class | 70,093 ns/op | 59,870 B/op | 249 |
| Pie | 96,986 ns/op | 50,901 B/op | 390 |
| XY Chart | 104,718 ns/op | 67,371 B/op | 535 |

Reproduce library benchmarks:

```bash
go test -run ^$ -bench BenchmarkRender -benchmem ./...
```

## Installation

### Homebrew (recommended)

```bash
brew tap bvolpato/tap
brew install bvolpato/tap/mmdg
mmdg --help
```

Upgrade later:

```bash
brew update
brew upgrade mmdg
```

### Download prebuilt binary (GitHub Releases)

Pick the archive for your OS/arch from:

- `https://github.com/bvolpato/mermaid-go-renderer/releases/latest`

Example (`darwin_arm64`):

```bash
VERSION="v0.2.0" # replace with the version you want
curl -L "https://github.com/bvolpato/mermaid-go-renderer/releases/download/${VERSION}/mermaid-go-renderer_${VERSION#v}_darwin_arm64.tar.gz" -o mmdg.tar.gz
tar -xzf mmdg.tar.gz
chmod +x mmdg
sudo mv mmdg /usr/local/bin/mmdg
mmdg --help
```

If you have multiple `mmdg` binaries in `PATH`, check with:

```bash
which -a mmdg
```

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

## CLI Usage

Render a Mermaid file to SVG:

```bash
mmdg -i diagram.mmd -o out.svg -e svg
```

Render a Mermaid file to PNG:

```bash
mmdg -i diagram.mmd -o out.png -e png
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

Add the dependency:

```bash
go get github.com/bvolpato/mermaid-go-renderer@latest
```

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

Write PNG directly from Go:

```go
package main

import (
	mermaid "github.com/bvolpato/mermaid-go-renderer"
)

func main() {
	svg, err := mermaid.RenderWithOptions("flowchart LR\nA-->B", mermaid.DefaultRenderOptions())
	if err != nil {
		panic(err)
	}
	if err := mermaid.WriteOutputPNG(svg, "out.png"); err != nil {
		panic(err)
	}
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
