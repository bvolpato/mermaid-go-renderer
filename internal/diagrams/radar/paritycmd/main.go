// Command paritycmd is the parity command with only the radar chart linked
// in. Run it from the repository root:
//
//	go run ./internal/diagrams/radar/paritycmd -filter radar/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/radar"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
