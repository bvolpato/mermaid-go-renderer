// Command paritycmd is the parity command with only the wardley diagram
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/wardley/paritycmd -filter wardley/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/wardley"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
