// Command paritycmd is the parity command with only the user journey diagram
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/journey/paritycmd -filter journey/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/journey"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
