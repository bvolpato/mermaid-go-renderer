// Command paritycmd is the parity command with only the timeline diagram
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/timeline/paritycmd -filter timeline/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/timeline"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
