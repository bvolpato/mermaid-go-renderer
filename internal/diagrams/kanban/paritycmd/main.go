// Command paritycmd is the parity command with only the kanban diagram
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/kanban/paritycmd -filter kanban/
//
// The kanban renderer draws with parts of the unified renderer. This
// program links the packages that register them: the cluster shapes and the
// node shapes. The kanban package registers the shape kanbanItem itself.
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/kanban"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set1"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set2"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set3"
)

func main() { cli.Main() }
