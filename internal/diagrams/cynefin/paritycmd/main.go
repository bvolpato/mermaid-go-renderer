// Command paritycmd is the parity command with only the cynefin diagram
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/cynefin/paritycmd -filter cynefin/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/cynefin"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
