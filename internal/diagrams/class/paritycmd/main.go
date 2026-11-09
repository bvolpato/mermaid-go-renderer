// Command paritycmd is the parity command with only the class diagram
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/class/paritycmd -filter class-diagram/
//
// It links the shapes, the edges, the clusters and the layouts of the
// unified renderer for their registration side effects.
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/class"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/dagre"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/elk"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set1"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set2"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set3"
)

func main() { cli.Main() }
