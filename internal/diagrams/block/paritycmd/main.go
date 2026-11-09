// Command paritycmd is the parity command with only the block diagram
// linked in, and the parts of the unified renderer that draw it. Run it
// from the repository root:
//
//	go run ./internal/diagrams/block/paritycmd -filter block
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/block"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"

	// The shapes register themselves in the unified renderer. The block
	// package links the edges, the edge labels and the markers itself.
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set1"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set2"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set3"
)

func main() { cli.Main() }
