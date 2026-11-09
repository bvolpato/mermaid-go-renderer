// Command paritycmd is the parity command with only the architecture
// diagram linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/architecture/paritycmd -filter architecture/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/architecture"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
