// Command paritycmd is the parity command with only the C4 diagram linked
// in. Run it from the repository root:
//
//	go run ./internal/diagrams/c4/paritycmd -filter c4/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/c4"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
