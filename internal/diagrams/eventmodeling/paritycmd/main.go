// Command paritycmd is the parity command with only the eventmodeling
// diagram linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/eventmodeling/paritycmd -filter eventmodeling/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/eventmodeling"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
