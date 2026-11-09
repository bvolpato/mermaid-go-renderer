// Command paritycmd is the parity command with only the packet diagram linked
// in. Run it from the repository root:
//
//	go run ./internal/diagrams/packet/paritycmd -filter packet/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/packet"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
