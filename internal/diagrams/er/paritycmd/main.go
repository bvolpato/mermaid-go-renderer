// Command paritycmd is the parity command with only the ER diagram linked
// in. Run it from the repository root:
//
//	go run ./internal/diagrams/er/paritycmd -filter er-diagram/
//
// The ER diagram is drawn by the unified renderer. This program links the
// packages that register its parts: the edges and the markers, the cluster
// shapes, the shape of a subgraph node and the layouts.
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/er"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/dagre"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/elk"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set1"
)

func main() { cli.Main() }
