// Command paritycmd is the parity command with only the sankey diagram
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/sankey/paritycmd -filter sankey/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/sankey"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
