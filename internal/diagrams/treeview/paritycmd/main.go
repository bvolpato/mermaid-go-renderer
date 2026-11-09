// Command paritycmd is the parity command with only the treeView diagram
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/treeview/paritycmd -filter tree-view/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/treeview"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
