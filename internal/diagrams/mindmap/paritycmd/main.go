// Command paritycmd is the parity command with only the mindmap diagram
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/mindmap/paritycmd -filter mindmap
//
// The mindmap is drawn by the unified renderer. This program links the
// packages that register its parts: the edges and the markers, the cluster
// shapes, the node shapes and the layouts.
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/mindmap"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/cosebilkent"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/dagre"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/elk"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/tidytree"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set1"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set2"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set3"
)

func main() { cli.Main() }
