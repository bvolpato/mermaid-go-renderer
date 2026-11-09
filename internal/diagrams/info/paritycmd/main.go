// Command paritycmd is the parity command with only the info diagram
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/info/paritycmd -filter info/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/info"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
