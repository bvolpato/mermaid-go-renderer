// Command paritycmd is the parity command with only the state diagram
// linked in, and the parts of the unified renderer that draw it. Run it
// from the repository root:
//
//	go run ./internal/diagrams/state/paritycmd -filter state
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/state"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"

	// The shapes, the cluster shapes and the layouts register themselves
	// in the unified renderer.
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/dagre"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/elk"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set1"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set2"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set3"
)

func main() { cli.Main() }
