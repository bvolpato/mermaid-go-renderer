// Command paritycmd is the parity command with only the venn diagram
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/venn/paritycmd -filter venn/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/venn"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
