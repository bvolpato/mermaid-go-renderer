// Command paritycmd is the parity command with only the treemap diagram
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/treemap/paritycmd -filter treemap/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/treemap"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
