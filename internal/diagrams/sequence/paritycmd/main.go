// Command paritycmd is the parity command with only the sequence diagram
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/sequence/paritycmd -filter sequence/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/sequence"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
