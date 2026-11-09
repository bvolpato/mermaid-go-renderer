// Command paritycmd is the parity command with only the railroad diagrams
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/railroad/paritycmd -filter railroad/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/railroad"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
