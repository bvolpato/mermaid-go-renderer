# mermaid-go-renderer

`mermaid-go-renderer` is a native Mermaid renderer written in Go.

- No browser / Chromium dependency
- Works as both a Go library and CLI
- Supports all Mermaid diagram families by native parsing + SVG rendering

## Inspiration

This project is directly inspired by [`1jehuang/mermaid-rs-renderer`](https://github.com/1jehuang/mermaid-rs-renderer), with a Go-first implementation and API for `github.com/bvolpato/mermaid-go-renderer`.

## Performance

Yes, we compared against Mermaid CLI (`mmdc`).

Measured on: Apple M4 Max, Go 1.26.0, macOS darwin 25.1.0.
Method: warm-up + repeated CLI runs (`mmdg`: 20 runs, `mmdc`: 3 runs).

### CLI benchmark (`mmdg` vs `mmdc`)

| Diagram | `mmdg` avg | `mmdc` avg | Speedup |
|:--|--:|--:|--:|
| Flowchart | 12.04 ms | 2085.64 ms | 173.25x |
| Sequence | 11.59 ms | 1957.36 ms | 168.87x |
| Class | 13.67 ms | 1996.32 ms | 146.06x |
| State | 10.69 ms | 2054.83 ms | 192.17x |

Geometric mean speedup: **169.28x**.

### Library microbenchmarks (`go test -bench`)

| Benchmark | Time | Memory | Allocs |
|:--|--:|--:|--:|
| Flowchart | 52,149 ns/op | 21,794 B/op | 336 |
| Sequence | 41,225 ns/op | 26,373 B/op | 394 |
| State | 80,679 ns/op | 20,968 B/op | 306 |
| Class | 61,899 ns/op | 15,494 B/op | 250 |
| Pie | 27,593 ns/op | 10,626 B/op | 156 |
| XY Chart | 28,910 ns/op | 13,957 B/op | 242 |

Reproduce:

```bash
go test -run ^$ -bench BenchmarkRender -benchmem ./...
```

## Install

### From source

```bash
git clone https://github.com/bvolpato/mermaid-go-renderer
cd mermaid-go-renderer
go build ./cmd/mmdg
```

### Homebrew (via GoReleaser)

This repository includes:

- `.goreleaser.yaml`
- `.github/workflows/release.yml`

On every pushed tag (`v*`), GitHub Actions runs GoReleaser to:

- build release binaries
- publish archives + checksums
- update Homebrew formula in `bvolpato/homebrew-tap`

Required GitHub secret:

- `HOMEBREW_TAP_GITHUB_TOKEN` (PAT with permission to push to the tap repository)

## CLI Usage

Render Mermaid from a file:

```bash
mmdg -i diagram.mmd -o diagram.svg -e svg
```

Render Mermaid from stdin:

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

## Library Usage

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

Or use the staged pipeline:

```go
parsed, _ := mermaid.ParseMermaid("flowchart LR\nA-->B")
layout := mermaid.ComputeLayout(&parsed.Graph, mermaid.ModernTheme(), mermaid.DefaultLayoutConfig())
svg := mermaid.RenderSVG(layout, mermaid.ModernTheme(), mermaid.DefaultLayoutConfig())
```

## Testing

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

## Architecture

Native pipeline:

```
.mmd -> parser -> IR graph -> layout -> SVG renderer
```

No browser process is spawned at any point.
