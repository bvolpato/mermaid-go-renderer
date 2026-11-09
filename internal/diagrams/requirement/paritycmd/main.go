// Command paritycmd is the parity command with only the requirement
// diagram linked in, and the parts of the unified renderer that draw it.
// Run it from the repository root:
//
//	go run ./internal/diagrams/requirement/paritycmd -filter requirement/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/requirement"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"

	// The edges, the cluster shapes, the layouts and the other shapes
	// register themselves in the unified renderer. The dagre layout draws
	// the label of a relationship from a node to itself with the shape
	// labelRect.
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/dagre"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/elk"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set1"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set2"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set3"
)

func main() { cli.Main() }
