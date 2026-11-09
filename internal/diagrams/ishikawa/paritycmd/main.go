// Command paritycmd is the parity command with only the ishikawa diagram
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/ishikawa/paritycmd -filter ishikawa/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/ishikawa"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
